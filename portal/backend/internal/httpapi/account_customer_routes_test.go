package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

type accountCustomerRouteCore struct {
	coreStub
	userID       int64
	accountExp   map[int64]*int64
	failAccounts map[int64]bool
	calls        []routeCoreCall
}

type routeCoreCall struct {
	method  string
	path    string
	query   string
	payload any
}

func (f *accountCustomerRouteCore) Profile(context.Context, string) (coreclient.User, error) {
	id := f.userID
	if id == 0 {
		id = 7
	}
	return coreclient.User{ID: id, Role: "user", Status: "active"}, nil
}

func (f *accountCustomerRouteCore) AdminMarketRequest(_ context.Context, method, path string, payload any, _ string) (coreclient.Response, error) {
	u, err := url.Parse(path)
	if err != nil {
		return coreclient.Response{}, err
	}
	f.calls = append(f.calls, routeCoreCall{method: method, path: u.Path, query: u.RawQuery, payload: payload})
	var id int64
	_, _ = fmt.Sscanf(u.Path, "/api/v1/admin/accounts/%d", &id)
	if f.failAccounts[id] {
		return coreclient.Response{}, errors.New("simulated Core account lookup failure")
	}
	if u.Path == fmt.Sprintf("/api/v1/admin/accounts/%d", id) && id > 0 {
		expires := f.accountExp[id]
		return routeJSON(map[string]any{
			"id": id, "name": fmt.Sprintf("Account %d", id), "status": "active",
			"expires_at": expires, "credentials": map[string]any{"access_token": "secret"},
		})
	}
	if strings.HasSuffix(u.Path, "/usage") {
		return routeJSON(map[string]any{"used_percent": 1, "credentials": map[string]any{"access_token": "secret"}})
	}
	if strings.HasSuffix(u.Path, "/today-stats") || strings.HasSuffix(u.Path, "/stats") {
		return routeJSON(map[string]any{"stats": map[string]any{"1": map[string]any{"requests": 1}}})
	}
	if strings.HasSuffix(u.Path, "/models") {
		return routeJSON([]map[string]any{{"id": "model"}})
	}
	if strings.Contains(u.Path, "/openai/") || strings.Contains(u.Path, "/grok/") || strings.Contains(u.Path, "/cn-providers/") || strings.Contains(u.Path, "/ollama-cloud-usage/") {
		return routeJSON(map[string]any{"account": map[string]any{"id": id, "credentials": map[string]any{"token": "secret"}}, "ok": true})
	}
	return routeJSON(map[string]any{"ok": true})
}

func (f *accountCustomerRouteCore) AdminMarketRawRequest(_ context.Context, method, path string, payload any, _ string) (coreclient.Response, error) {
	u, err := url.Parse(path)
	if err != nil {
		return coreclient.Response{}, err
	}
	f.calls = append(f.calls, routeCoreCall{method: method, path: u.Path, query: u.RawQuery, payload: payload})
	return coreclient.Response{StatusCode: http.StatusOK, ContentType: "application/json", Body: []byte(`{"code":0,"data":{"ok":true,"credentials":{"token":"secret"}}}`)}, nil
}

func routeJSON(data any) (coreclient.Response, error) {
	body, err := json.Marshal(map[string]any{"code": 0, "data": data})
	return coreclient.Response{StatusCode: http.StatusOK, Body: body}, err
}

func routeServer(t *testing.T, db *sql.DB, core *accountCustomerRouteCore) http.Handler {
	t.Helper()
	return New(core, nil, testLogger(), NewPortalDatabase(db))
}

func seedRouteOrder(t *testing.T, db *sql.DB, userID int64, accountIDs []int64, status string, starts, expires *time.Time) int64 {
	t.Helper()
	orderID := seedRental(t, db, userID, len(accountIDs))
	_, err := db.Exec(`UPDATE portal_account_orders SET core_account_ids=$1::jsonb, status=$2, fulfillment_started_at=$3, expires_at=$4 WHERE id=$5`, mustJSON(accountIDs), status, starts, expires, orderID)
	if err != nil {
		t.Fatal(err)
	}
	return orderID
}

func mustJSON(value any) string {
	body, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(body)
}

func routeRequest(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer test")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func assertNoCorePath(t *testing.T, core *accountCustomerRouteCore, path string) {
	t.Helper()
	for _, call := range core.calls {
		if call.path == path {
			t.Fatalf("unexpected Core request to %s", path)
		}
	}
}

func TestAccountCustomerRoutesRejectUnauthorizedAccountStates(t *testing.T) {
	db := rentalTestDB(t)
	core := &accountCustomerRouteCore{accountExp: map[int64]*int64{1: nil, 2: nil, 3: nil, 4: nil, 5: nil}}
	h := routeServer(t, db, core)
	now := time.Now()
	seedRouteOrder(t, db, 7, []int64{1}, "completed", nil, ptrTime(now.Add(time.Hour)))
	seedRouteOrder(t, db, 8, []int64{2}, "completed", nil, ptrTime(now.Add(time.Hour)))
	seedRouteOrder(t, db, 7, []int64{3}, "completed", nil, ptrTime(now.Add(-time.Hour)))
	seedRouteOrder(t, db, 7, []int64{4}, "completed", nil, ptrTime(now.Add(time.Hour)))
	seedRouteOrder(t, db, 7, []int64{5}, "pending", nil, ptrTime(now.Add(2*time.Hour)))
	expired := now.Add(-time.Hour).Unix()
	core.accountExp[4] = &expired
	for _, tc := range []struct{ name, path string }{
		{"unbound", "/api/v1/account-market/accounts/99/usage"},
		{"other user", "/api/v1/account-market/accounts/2/usage"},
		{"expired order", "/api/v1/account-market/accounts/3/usage"},
		{"expired Core account", "/api/v1/account-market/accounts/4/usage"},
		{"not started order", "/api/v1/account-market/accounts/5/usage"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			core.calls = nil
			w := routeRequest(h, http.MethodGet, tc.path, "")
			if w.Code < 400 || w.Code >= 500 {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			assertNoCorePath(t, core, "/api/v1/admin/accounts/"+strings.Split(strings.TrimPrefix(tc.path, "/api/v1/account-market/accounts/"), "/")[0]+"/usage")
		})
	}
}

func TestAccountCustomerOperationsRejectUnauthorizedAccountIDs(t *testing.T) {
	db := rentalTestDB(t)
	now := time.Now()
	core := &accountCustomerRouteCore{accountExp: map[int64]*int64{1: nil, 2: nil, 3: nil, 4: nil, 5: nil}}
	h := routeServer(t, db, core)
	seedRouteOrder(t, db, 7, []int64{1}, "completed", nil, ptrTime(now.Add(time.Hour)))
	seedRouteOrder(t, db, 8, []int64{2}, "completed", nil, ptrTime(now.Add(time.Hour)))
	seedRouteOrder(t, db, 7, []int64{3}, "completed", nil, ptrTime(now.Add(-time.Hour)))
	seedRouteOrder(t, db, 7, []int64{4}, "completed", nil, ptrTime(now.Add(time.Hour)))
	seedRouteOrder(t, db, 7, []int64{5}, "pending", nil, ptrTime(now.Add(time.Hour)))
	expired := now.Add(-time.Hour).Unix()
	core.accountExp[4] = &expired

	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"other user recover", http.MethodPost, "/api/v1/account-market/accounts/2/recover", ""},
		{"expired order scheduling", http.MethodPost, "/api/v1/account-market/accounts/3/schedulable", `{"schedulable":false}`},
		{"expired Core account test", http.MethodPost, "/api/v1/account-market/accounts/4/test", `{"model_id":"m","prompt":"p"}`},
		{"pending order quota query", http.MethodPost, "/api/v1/account-market/accounts/5/core-action/openai-refresh", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			core.calls = nil
			w := routeRequest(h, tc.method, tc.path, tc.body)
			if w.Code < http.StatusBadRequest || w.Code >= http.StatusInternalServerError {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			for _, call := range core.calls {
				// Core expiry must be read to authorize a still-valid Portal order.
				if tc.name == "expired Core account test" && call.method == http.MethodGet && call.path == "/api/v1/admin/accounts/4" {
					continue
				}
				t.Fatalf("unauthorized account operation reached Core: %+v", core.calls)
			}
		})
	}
}

func ptrTime(value time.Time) *time.Time { return &value }

func TestAccountCustomerBatchRoutesFilterUnauthorizedIDsAndResponses(t *testing.T) {
	db := rentalTestDB(t)
	core := &accountCustomerRouteCore{accountExp: map[int64]*int64{1: nil}}
	h := routeServer(t, db, core)
	seedRouteOrder(t, db, 7, []int64{1}, "completed", nil, ptrTime(time.Now().Add(time.Hour)))
	for _, tc := range []struct{ name, path string }{
		{"usage", "/api/v1/account-market/accounts/usage/batch"},
		{"today stats", "/api/v1/account-market/accounts/today-stats/batch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			core.calls = nil
			w := routeRequest(h, http.MethodPost, tc.path, `{"account_ids":[1,999],"force":true}`)
			if w.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "999") {
				t.Fatalf("unauthorized ID in response: %s", w.Body.String())
			}
			for _, call := range core.calls {
				if strings.Contains(call.path, "batch") && strings.Contains(fmt.Sprint(call.payload), "999") {
					t.Fatalf("unauthorized ID forwarded: %+v", call)
				}
			}
		})
	}
}

func TestAccountCustomerSingleRoutesRequireAuthorizationAndUseExpectedMethods(t *testing.T) {
	db := rentalTestDB(t)
	core := &accountCustomerRouteCore{accountExp: map[int64]*int64{1: nil}}
	h := routeServer(t, db, core)
	seedRouteOrder(t, db, 7, []int64{1}, "completed", nil, ptrTime(time.Now().Add(time.Hour)))
	routes := []struct {
		name, method, path, body, wantPath string
	}{
		{"account refresh", http.MethodGet, "/api/v1/account-market/accounts/1", "", "/api/v1/admin/accounts/1"},
		{"recover", http.MethodPost, "/api/v1/account-market/accounts/1/recover", "", "/api/v1/admin/accounts/1/recover-state"},
		{"schedulable", http.MethodPost, "/api/v1/account-market/accounts/1/schedulable", `{"schedulable":false}`, "/api/v1/admin/accounts/1/schedulable"},
		{"test", http.MethodPost, "/api/v1/account-market/accounts/1/test", `{"model_id":"m","prompt":"p","mode":"sync","image_data_url":"data:image/png;base64,abc","audio_data_url":"data:audio/wav;base64,def"}`, "/api/v1/admin/accounts/1/test"},
		{"usage", http.MethodGet, "/api/v1/account-market/accounts/1/usage?source=passive", "", "/api/v1/admin/accounts/1/usage"},
		{"stats", http.MethodGet, "/api/v1/account-market/accounts/1/stats?days=7", "", "/api/v1/admin/accounts/1/stats"},
		{"today stats", http.MethodGet, "/api/v1/account-market/accounts/1/today-stats", "", "/api/v1/admin/accounts/1/today-stats"},
		{"models", http.MethodGet, "/api/v1/account-market/accounts/1/models", "", "/api/v1/admin/accounts/1/models"},
		{"openai refresh", http.MethodPost, "/api/v1/account-market/accounts/1/core-action/openai-refresh", "", "/api/v1/admin/openai/accounts/1/quota/refresh"},
		{"openai reset", http.MethodPost, "/api/v1/account-market/accounts/1/core-action/openai-reset", "", "/api/v1/admin/openai/accounts/1/reset-quota"},
		{"account reset quota", http.MethodPost, "/api/v1/account-market/accounts/1/core-action/account-reset-quota", "", "/api/v1/admin/accounts/1/reset-quota"},
		{"grok quota GET", http.MethodGet, "/api/v1/account-market/accounts/1/core-action/grok-quota", "", "/api/v1/admin/grok/accounts/1/quota"},
		{"grok reset POST", http.MethodPost, "/api/v1/account-market/accounts/1/core-action/grok-reset", "", "/api/v1/admin/grok/accounts/1/reset-quota"},
		{"cn quota GET", http.MethodGet, "/api/v1/account-market/accounts/1/core-action/cn-quota", "", "/api/v1/admin/cn-providers/accounts/1/quota"},
		{"cn balance GET", http.MethodGet, "/api/v1/account-market/accounts/1/core-action/cn-balance", "", "/api/v1/admin/cn-providers/accounts/1/balance"},
		{"ollama refresh POST", http.MethodPost, "/api/v1/account-market/accounts/1/core-action/ollama-refresh", "", "/api/v1/admin/accounts/1/ollama-cloud-usage/refresh"},
	}
	for _, tc := range routes {
		t.Run(tc.name, func(t *testing.T) {
			core.calls = nil
			w := routeRequest(h, tc.method, tc.path, tc.body)
			if w.Code >= 400 {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			var found *routeCoreCall
			for i := range core.calls {
				if core.calls[i].path == tc.wantPath {
					found = &core.calls[i]
				}
			}
			if found == nil {
				t.Fatalf("Core path %s not called; calls=%+v", tc.wantPath, core.calls)
			}
			wantMethod := tc.method
			if strings.Contains(tc.name, "quota GET") || strings.Contains(tc.name, "balance GET") {
				wantMethod = http.MethodGet
			}
			if found.method != wantMethod {
				t.Fatalf("Core method=%s want %s", found.method, wantMethod)
			}
			if strings.Contains(w.Body.String(), "secret") || strings.Contains(w.Body.String(), "credentials") {
				t.Fatalf("sensitive response leaked: %s", w.Body.String())
			}
		})
	}
}

func TestAccountCustomerCoreLookupFailureIsNotSuccessful(t *testing.T) {
	db := rentalTestDB(t)
	core := &accountCustomerRouteCore{accountExp: map[int64]*int64{1: nil}, failAccounts: map[int64]bool{1: true}}
	h := routeServer(t, db, core)
	seedRouteOrder(t, db, 7, []int64{1}, "completed", nil, ptrTime(time.Now().Add(time.Hour)))
	w := routeRequest(h, http.MethodGet, "/api/v1/account-market/accounts/1/usage", "")
	if w.Code < 400 {
		t.Fatalf("Core account lookup failure returned success: %d %s", w.Code, w.Body.String())
	}
}
