package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

type exclusiveRecordsCore struct {
	pageTestCore
	paths []string
}

func (f *exclusiveRecordsCore) AdminMarketRequest(_ context.Context, _, path string, _ any, _ string) (coreclient.Response, error) {
	f.paths = append(f.paths, path)
	u, _ := url.Parse(path)
	if strings.HasPrefix(u.Path, "/api/v1/admin/ops/request-errors/") {
		key := 11
		if strings.HasSuffix(u.Path, "/902") {
			key = 22
		}
		return rentalResponse(map[string]any{"id": 901, "api_key_id": key, "account_id": 99, "message": "failure"})
	}
	q := u.Query()
	if (q.Get("api_key_id") != "11" && q.Get("api_key_id") != "12") || q.Get("user_id") != "7" || q.Get("account_id") != "" {
		return coreclient.Response{}, fmt.Errorf("unscoped request: %s", path)
	}
	switch u.Path {
	case "/api/v1/admin/usage", "/api/v1/admin/ops/request-errors":
		return rentalResponse(map[string]any{"items": []any{map[string]any{"id": 1, "model": "gpt-test", "created_at": "2026-09-17T00:00:00Z"}}, "total": 1})
	case "/api/v1/admin/usage/stats":
		return rentalResponse(map[string]any{"total_requests": 1})
	case "/api/v1/admin/dashboard/models":
		return rentalResponse(map[string]any{"models": []any{map[string]any{"model": "gpt-test", "requests": 1}}})
	case "/api/v1/admin/dashboard/snapshot-v2":
		return rentalResponse(map[string]any{"trend": []any{map[string]any{"date": "2026-09-17", "requests": 1}}})
	}
	return coreclient.Response{}, fmt.Errorf("unexpected path: %s", path)
}

func TestExclusiveRequestRecordsReusePackageHandlers(t *testing.T) {
	s, _, db, channel, lease := pageFixture(t)
	fake := &exclusiveRecordsCore{}
	s.core = fake
	if _, err := db.Exec(`UPDATE portal_exclusive_leases SET core_key_id=11 WHERE id=$1`, lease); err != nil {
		t.Fatal(err)
	}
	// Closed leases remain visible. A different user of this same channel is excluded.
	for _, row := range []struct {
		user, key int
		hash      string
	}{{7, 12, "closed"}, {8, 22, "other"}} {
		if _, err := db.Exec(`INSERT INTO portal_exclusive_leases(channel_id,user_id,state,core_key_id,token_hash,token_secret,upstream_secret,expires_at) VALUES($1,$2,'closed',$3,$4,'secret','secret',CURRENT_TIMESTAMP)`, channel, row.user, row.key, row.hash); err != nil {
			t.Fatal(err)
		}
	}
	mux := http.NewServeMux()
	s.registerExclusive(mux)
	for _, suffix := range []string{"", "/stats", "/dashboard/models", "/dashboard/snapshot-v2", "/errors"} {
		t.Run(suffix, func(t *testing.T) {
			fake.paths = nil
			r := httptest.NewRequest("GET", "/api/v1/exclusive-channels/usage"+suffix+"?user_id=8&account_id=99", nil)
			r.Header.Set("Authorization", "Bearer user")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != 200 || len(fake.paths) != 2 {
				t.Fatalf("status=%d body=%s paths=%v", w.Code, w.Body.String(), fake.paths)
			}
		})
	}
	for _, tc := range []struct {
		path, token   string
		status, calls int
	}{
		{"?api_key_id=22", "user", 404, 0},
		{"?api_key_id=bad", "user", 400, 0},
		{"?api_key_id=12", "user", 200, 1},
		{"", "admin", 200, 0},
		{"", "", 401, 0},
		{"/errors/901", "user", 200, 1},
		{"/errors/902", "user", 404, 1},
	} {
		fake.paths = nil
		r := httptest.NewRequest("GET", "/api/v1/exclusive-channels/usage"+tc.path, nil)
		if tc.token != "" {
			r.Header.Set("Authorization", "Bearer "+tc.token)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != tc.status || len(fake.paths) != tc.calls {
			t.Fatalf("%+v: status=%d body=%s paths=%v", tc, w.Code, w.Body.String(), fake.paths)
		}
	}
}
