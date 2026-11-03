package riskcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jqcode/portal/backend/internal/coreclient"
)

type fakeCore struct {
	call func(string, string, any) (coreclient.Response, error)
}

func (f fakeCore) AdminMarketRequest(_ context.Context, m, p string, b any, _ string) (coreclient.Response, error) {
	return f.call(m, p, b)
}
func response(data any) coreclient.Response {
	b, _ := json.Marshal(map[string]any{"data": data})
	return coreclient.Response{StatusCode: 200, Body: b}
}
func newTestGuard(t *testing.T) (*Guard, sqlmock.Sqlmock) {
	t.Helper()
	db, m, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := m.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
		db.Close()
	})
	return &Guard{DB: db, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}, m
}

func TestOnlyExplicitRiskTypesBan(t *testing.T) {
	for _, tt := range []struct {
		kind, phase string
		user        int64
		want        bool
	}{
		{"cyber_policy", "request", 8, true}, {"cyber_policy_session_blocked", "request", 8, true},
		{"permission_error", "request", 8, false}, {"rate_limit_error", "request", 8, false},
		{"billing_error", "request", 8, false}, {"cyber_policy", "upstream", 8, false}, {"cyber_policy", "request", 0, false},
	} {
		if got := (event{ID: 1, UserID: tt.user, Type: tt.kind, Phase: tt.phase}).actionable(); got != tt.want {
			t.Errorf("%+v: %v", tt, got)
		}
	}
}

func TestMiddlewareCredentialPrecedenceAndFailure(t *testing.T) {
	for _, tt := range []struct {
		name, auth, api, goog, key string
		banned, fail               bool
		status                     int
	}{
		{name: "bearer", auth: "bEaReR sk-a", api: "sk-b", key: "sk-a", banned: true, status: 403},
		{name: "anthropic", api: "sk-b", key: "sk-b", banned: true, status: 403},
		{name: "gemini", goog: "sk-c", key: "sk-c", banned: true, status: 403},
		{name: "ordinary", auth: "Bearer sk-ok", key: "sk-ok", status: 204},
		{name: "missing", status: 204},
		{name: "storage failure", auth: "Bearer sk-a", key: "sk-a", fail: true, status: 503},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g, m := newTestGuard(t)
			if tt.key != "" {
				q := m.ExpectQuery("SELECT EXISTS").WithArgs(keyHash(tt.key))
				if tt.fail {
					q.WillReturnError(errors.New("db offline"))
				} else {
					q.WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(tt.banned))
				}
			}
			r := httptest.NewRequest("POST", "/v1/responses", nil)
			r.Header.Set("Authorization", tt.auth)
			r.Header.Set("X-Api-Key", tt.api)
			r.Header.Set("X-Goog-Api-Key", tt.goog)
			w := httptest.NewRecorder()
			g.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })).ServeHTTP(w, r)
			if w.Code != tt.status {
				t.Fatalf("status %d", w.Code)
			}
			if tt.banned && (!strings.Contains(w.Body.String(), Message) || !strings.Contains(w.Body.String(), "ACCOUNT_RISK_BANNED")) {
				t.Fatal(w.Body.String())
			}
		})
	}
}

func TestBanPersistsAllKeysBeforeDisablingAndRetriesFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "core failure"}[fail], func(t *testing.T) {
			g, m := newTestGuard(t)
			e := event{ID: 12, UserID: 8, Type: "cyber_policy", Phase: "request"}
			m.ExpectQuery("SELECT EXISTS").WithArgs(e.ID).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
			m.ExpectBegin()
			for _, key := range []string{"sk-a", "sk-b"} {
				m.ExpectExec("INSERT INTO portal_risk_keys").WithArgs(keyHash(key), int64(8), int64(12)).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			m.ExpectCommit()
			if !fail {
				m.ExpectExec("INSERT INTO portal_risk_events").WithArgs(e.ID, e.UserID, e.Type).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			writes := 0
			g.Core = fakeCore{call: func(method, path string, body any) (coreclient.Response, error) {
				if method == "PUT" {
					writes++
					if err := m.ExpectationsWereMet(); fail && err != nil {
						t.Fatal("keys must be committed before Core disable:", err)
					}
					if body.(map[string]string)["status"] != "disabled" {
						t.Fatal(body)
					}
					if fail {
						return coreclient.Response{}, errors.New("Core unavailable")
					}
					return response(nil), nil
				}
				if strings.Contains(path, "api-keys") {
					return response(map[string]any{"items": []map[string]any{{"id": 1, "key": "sk-a"}, {"id": 2, "key": "sk-b"}}, "total": 2}), nil
				}
				return response(map[string]any{"id": 8, "role": "user", "status": "active"}), nil
			}}
			err := g.ban(context.Background(), e)
			if (err != nil) != fail {
				t.Fatalf("error %v", err)
			}
			if writes != 1 {
				t.Fatalf("writes %d", writes)
			}
		})
	}
}

func TestProcessedEventIsNotReapplied(t *testing.T) {
	g, m := newTestGuard(t)
	m.ExpectQuery("SELECT EXISTS").WithArgs(int64(12)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	if err := g.ban(context.Background(), event{ID: 12}); err != nil {
		t.Fatal(err)
	}
}

func TestAdministratorIsNotDisabled(t *testing.T) {
	g, m := newTestGuard(t)
	e := event{ID: 12, UserID: 8, Type: "cyber_policy"}
	m.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	m.ExpectExec("INSERT INTO portal_risk_events").WithArgs(e.ID, e.UserID, e.Type).WillReturnResult(sqlmock.NewResult(0, 1))
	g.Core = fakeCore{call: func(method, path string, body any) (coreclient.Response, error) {
		if method != "GET" || strings.Contains(path, "api-keys") {
			t.Fatal("unexpected admin mutation")
		}
		return response(map[string]any{"id": 8, "role": "admin", "status": "active"}), nil
	}}
	if err := g.ban(context.Background(), e); err != nil {
		t.Fatal(err)
	}
}

func TestAdminReenableAndDeletedUserReleaseIngress(t *testing.T) {
	for _, status := range []string{"active", "disabled", "deleted"} {
		t.Run(status, func(t *testing.T) {
			g, m := newTestGuard(t)
			m.ExpectQuery("SELECT DISTINCT").WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(8))
			if status != "disabled" {
				m.ExpectExec("DELETE FROM portal_risk_keys").WithArgs(int64(8)).WillReturnResult(sqlmock.NewResult(0, 2))
			}
			g.Core = fakeCore{call: func(_, _ string, _ any) (coreclient.Response, error) {
				if status == "deleted" {
					return coreclient.Response{StatusCode: 404}, errors.New("not found")
				}
				return response(map[string]any{"id": 8, "status": status}), nil
			}}
			if err := g.releaseReenabled(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestScanPaginationAndFailedScanKeepsCursor(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "second page fails"}[fail], func(t *testing.T) {
			g, m := newTestGuard(t)
			now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
			start := now.Add(-time.Minute)
			m.ExpectQuery("SELECT pg_try_advisory_lock").WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(true))
			m.ExpectQuery("SELECT DISTINCT").WillReturnRows(sqlmock.NewRows([]string{"user_id"}))
			m.ExpectQuery("SELECT checked_until").WillReturnRows(sqlmock.NewRows([]string{"checked_until"}).AddRow(start))
			if !fail {
				m.ExpectExec("INSERT INTO portal_risk_cursor").WithArgs(now).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			m.ExpectExec("SELECT pg_advisory_unlock").WillReturnResult(sqlmock.NewResult(0, 1))
			pages := 0
			g.Core = fakeCore{call: func(method, path string, body any) (coreclient.Response, error) {
				pages++
				u, _ := url.Parse(path)
				q := u.Query()
				if method != "GET" || q.Get("view") != "all" || q.Get("start_time") != start.Add(-5*time.Minute).Format(time.RFC3339Nano) || q.Get("end_time") != now.Format(time.RFC3339Nano) {
					t.Fatalf("wrong scan window: %s", path)
				}
				if pages == 2 && fail {
					return coreclient.Response{}, errors.New("unavailable")
				}
				count := 500
				if pages == 2 {
					count = 1
				}
				items := make([]event, count)
				// Ordinary errors returned by broad text search must never ban.
				for i := range items {
					items[i] = event{ID: int64(i + 1), UserID: 8, Phase: "request", Type: "permission_error"}
				}
				return response(map[string]any{"items": items, "total": 501}), nil
			}}
			if err := g.tick(context.Background(), now); (err != nil) != fail {
				t.Fatalf("tick error: %v", err)
			}
			if pages != 2 {
				t.Fatalf("pages = %d", pages)
			}
		})
	}
}
