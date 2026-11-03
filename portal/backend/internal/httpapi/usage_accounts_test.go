package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

type usageAccountsCore struct {
	checksCore
	response coreclient.Response
	err      error
}

func (f *usageAccountsCore) AdminUserMarketRequest(_ context.Context, method, path string, _ any, token string) (coreclient.Response, error) {
	f.calls++
	f.path = path
	if method != http.MethodGet || token != "token" {
		return coreclient.Response{}, errors.New("unexpected request credentials or method")
	}
	return f.response, f.err
}

func usageAccountsRequest(core *usageAccountsCore, query, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/admin/usage/search-accounts?"+query, nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	New(core, nil, testLogger()).ServeHTTP(w, r)
	return w
}

func TestUsageSearchAccountsUsesAllPlatformsAndOnlyReturnsDisplayFields(t *testing.T) {
	core := &usageAccountsCore{
		checksCore: checksCore{role: "admin", status: "active"},
		response:   coreclient.Response{StatusCode: 200, Body: []byte(`{"code":0,"data":{"items":[{"id":42,"name":"账号 & 一号","platform":"openai","credentials":{"access_token":"secret"}},{"id":43,"name":"Claude","platform":"anthropic","proxy":{"password":"private"}}]}}`)},
	}
	keyword := "账号 & 一号"
	w := usageAccountsRequest(core, url.Values{"q": {"  " + keyword + "  "}}.Encode(), "token")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	upstream, err := url.Parse(core.path)
	if err != nil {
		t.Fatal(err)
	}
	q := upstream.Query()
	if upstream.Path != "/api/v1/admin/accounts" || q.Get("search") != keyword || q.Get("page_size") != "20" || q.Get("page") != "1" || q.Has("platform") || len(q) != 3 {
		t.Fatal(core.path)
	}
	var result struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Data) != 2 {
		t.Fatal(w.Body.String())
	}
	for _, account := range result.Data {
		if len(account) != 2 || account["id"] == nil || account["name"] == nil {
			t.Fatal(account)
		}
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Header())
	}
}

func TestUsageSearchAccountsRequiresActiveAdministrator(t *testing.T) {
	for _, tc := range []struct {
		role, status, token string
		want                int
	}{
		{"admin", "active", "", 401}, {"user", "active", "token", 403},
		{"group_manager", "active", "token", 403}, {"company_manager", "active", "token", 403},
		{"admin", "disabled", "token", 403},
	} {
		core := &usageAccountsCore{checksCore: checksCore{role: tc.role, status: tc.status}}
		w := usageAccountsRequest(core, "q=test", tc.token)
		if w.Code != tc.want || core.calls != 0 {
			t.Fatalf("%+v: status=%d calls=%d", tc, w.Code, core.calls)
		}
	}
}

func TestUsageSearchAccountsEmptyInvalidAndUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name, keyword, body string
		err                 error
		want, calls         int
	}{
		{name: "empty", keyword: "  ", want: 200},
		{name: "too long", keyword: strings.Repeat("x", 201), want: 400},
		{name: "no results", keyword: "missing", body: `{"code":0,"data":{"items":null}}`, want: 200, calls: 1},
		{name: "invalid response", keyword: "test", body: `invalid`, want: 502, calls: 1},
		{name: "unavailable", keyword: "test", err: errors.New("unavailable"), want: 502, calls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			core := &usageAccountsCore{checksCore: checksCore{role: "admin", status: "active"}, response: coreclient.Response{StatusCode: 200, Body: []byte(tc.body)}, err: tc.err}
			w := usageAccountsRequest(core, url.Values{"q": {tc.keyword}}.Encode(), "token")
			if w.Code != tc.want || core.calls != tc.calls {
				t.Fatalf("status=%d calls=%d body=%s", w.Code, core.calls, w.Body.String())
			}
			if tc.want == 200 && !strings.Contains(w.Body.String(), `"data":[]`) {
				t.Fatal(w.Body.String())
			}
		})
	}
}

func TestUsageProxyPreservesCoreFilters(t *testing.T) {
	query := url.Values{"account_id": {"42"}, "native_compaction_v2": {"true"}, "upstream_model_mismatch": {"false"}, "request_type": {"live"}, "billing_type": {"0"}, "billing_mode": {"token"}}.Encode()
	for _, path := range []string{"/api/v1/admin/usage", "/api/v1/admin/usage/stats", "/api/v1/admin/dashboard/models", "/api/v1/admin/dashboard/snapshot-v2", "/api/v1/admin/dashboard/user-breakdown", "/api/v1/admin/ops/request-errors"} {
		t.Run(path, func(t *testing.T) {
			core := &recordingCoreStub{coreStub: coreStub{userAPIResponse: coreclient.Response{StatusCode: 200, Body: []byte(`{"code":0,"data":{}}`)}}}
			r := httptest.NewRequest(http.MethodGet, path+"?"+query, nil)
			r.Header.Set("Authorization", "Bearer token")
			w := httptest.NewRecorder()
			New(core, nil, testLogger()).ServeHTTP(w, r)
			if w.Code != 200 || core.request.RawQuery != query || core.request.Path != path {
				t.Fatalf("status=%d request=%+v", w.Code, core.request)
			}
		})
	}
}
