package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

type checksCore struct {
	coreStub
	role, status, path, platform string
	payload                      any
	calls                        int
}

func (f *checksCore) Profile(context.Context, string) (coreclient.User, error) {
	return coreclient.User{ID: 1, Role: f.role, Status: f.status}, nil
}
func (f *checksCore) AdminMarketRequest(_ context.Context, _, path string, _ any, _ string) (coreclient.Response, error) {
	f.calls++
	f.path = path
	if !strings.Contains(path, "?") {
		platform := f.platform
		if platform == "" {
			platform = "openai"
		}
		return rentalResponse(map[string]any{"id": 42, "platform": platform})
	}
	return coreclient.Response{StatusCode: 200, Body: []byte(`{"code":0,"data":{"items":[{"id":42,"name":"测试账号","platform":"openai","credentials":{"access_token":"secret"},"group_ids":[7,8],"groups":[{"id":7,"name":"分组一","secret":"hidden"},{"id":8,"name":"分组二"}]}],"total":1,"pages":1,"page":1}}`)}, nil
}
func (f *checksCore) AdminMarketRawRequest(_ context.Context, _, path string, payload any, _ string) (coreclient.Response, error) {
	f.calls++
	f.path = path
	f.payload = payload
	return coreclient.Response{StatusCode: 200, Body: []byte(checkEvents("题目的答案", true))}, nil
}

func (f *checksCore) AdminUserMarketRequest(ctx context.Context, method, path string, payload any, accessToken string) (coreclient.Response, error) {
	if accessToken != "token" {
		return coreclient.Response{}, fmt.Errorf("unexpected access token")
	}
	return f.AdminMarketRequest(ctx, method, path, payload, "")
}

func (f *checksCore) AdminUserMarketRawRequest(ctx context.Context, method, path string, payload any, accessToken string) (coreclient.Response, error) {
	if accessToken != "token" {
		return coreclient.Response{}, fmt.Errorf("unexpected access token")
	}
	return f.AdminMarketRawRequest(ctx, method, path, payload, "")
}

func (f *checksCore) AccountPromptRequest(ctx context.Context, id int64, model, prompt, token string) (coreclient.Response, error) {
	return f.AdminUserMarketRawRequest(ctx, http.MethodPost, fmt.Sprintf("/api/v1/admin/accounts/%d/test", id), accountCheckInput{ModelID: model, Prompt: prompt}, token)
}
func checkEvents(text string, complete bool) string {
	encoded, _ := json.Marshal(map[string]string{"type": "content", "text": text})
	result := "data: {\"type\":\"test_start\",\"model\":\"test-model\"}\r\n\r\ndata:" + string(encoded) + "\r\n\r\n"
	if complete {
		result += "data: {\"type\":\"test_complete\",\"success\":true}"
	}
	return result
}
func checksRequest(s *Server, method, path, body, token string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	s.registerAccountChecks(mux)
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}
func TestAccountChecksAdminOnly(t *testing.T) {
	for _, identity := range []struct {
		role, status, token string
		code                int
	}{
		{"user", "active", "token", 403}, {"group_manager", "active", "token", 403}, {"company_manager", "active", "token", 403}, {"admin", "disabled", "token", 403}, {"admin", "active", "", 401},
	} {
		for _, endpoint := range []struct{ method, path string }{{"GET", "/api/v1/admin/account-checks/accounts"}, {"GET", "/api/v1/admin/account-checks/history"}, {"GET", "/api/v1/admin/account-checks/accounts/42/history"}, {"GET", "/api/v1/admin/account-checks/accounts/42/models"}, {"POST", "/api/v1/admin/account-checks/accounts/42/test"}, {"DELETE", "/api/v1/admin/account-checks/accounts/42/test"}} {
			core := &checksCore{role: identity.role, status: identity.status}
			s := &Server{core: core, logger: testLogger()}
			w := checksRequest(s, endpoint.method, endpoint.path, `{"prompt":"hello"}`, identity.token)
			if w.Code != identity.code || core.calls != 0 {
				t.Fatalf("%+v %s: status=%d calls=%d", identity, endpoint.path, w.Code, core.calls)
			}
		}
	}
}
func TestAccountChecksPickerPreservesGroupsWithoutSecrets(t *testing.T) {
	core := &checksCore{role: "admin", status: "active"}
	s := &Server{core: core, logger: testLogger()}
	w := checksRequest(s, "GET", "/api/v1/admin/account-checks/accounts?page=2&search=hello%20world", "", "token")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, expected := range []string{`"id":42`, "测试账号", "分组一", "分组二"} {
		if !strings.Contains(w.Body.String(), expected) {
			t.Fatal(w.Body.String())
		}
	}
	if strings.Contains(w.Body.String(), "secret") || strings.Contains(w.Body.String(), "credentials") || strings.Contains(w.Body.String(), "hidden") {
		t.Fatal("picker leaked credentials", w.Body.String())
	}
	if !strings.Contains(core.path, "page=2") || !strings.Contains(core.path, "search=hello+world") || !strings.Contains(core.path, "platform=openai") {
		t.Fatal(core.path)
	}
}

func TestAccountChecksRejectsNonOpenAIAccount(t *testing.T) {
	for _, endpoint := range []struct{ method, path string }{{"GET", "/api/v1/admin/account-checks/accounts/42/models"}, {"POST", "/api/v1/admin/account-checks/accounts/42/test"}} {
		core := &checksCore{role: "admin", status: "active", platform: "anthropic"}
		s := &Server{core: core, logger: testLogger()}
		w := checksRequest(s, endpoint.method, endpoint.path, `{"prompt":"question"}`, "token")
		if w.Code != 400 || core.calls != 1 || !strings.Contains(w.Body.String(), "ACCOUNT_CHECK_OPENAI_ONLY") {
			t.Fatal(w.Code, core.calls, w.Body.String())
		}
	}
}
func TestAccountChecksRunsSelectedAccountAndPromptWithoutRental(t *testing.T) {
	core := &checksCore{role: "admin", status: "active"}
	s := &Server{core: core, logger: testLogger(), portalDB: rentalTestDB(t)}
	w := checksRequest(s, "POST", "/api/v1/admin/account-checks/accounts/42/test", `{"prompt":"请证明这个定理","model_id":"chosen-model"}`, "token")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "题目的答案") || !strings.Contains(w.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatal(w.Code, w.Body.String())
	}
	payload, _ := json.Marshal(core.payload)
	if core.path != "/api/v1/admin/accounts/42/test" || !strings.Contains(string(payload), "请证明这个定理") || !strings.Contains(string(payload), "chosen-model") {
		t.Fatal(core.path, string(payload))
	}
}
func TestAccountChecksRejectsInvalidInput(t *testing.T) {
	for _, body := range []string{`{`, `{}`, `{"prompt":"   "}`, `{"prompt":"` + strings.Repeat("a", 64001) + `"}`} {
		core := &checksCore{role: "admin", status: "active"}
		s := &Server{core: core, logger: testLogger()}
		w := checksRequest(s, "POST", "/api/v1/admin/account-checks/accounts/42/test", body, "token")
		if w.Code != 400 || core.calls != 0 {
			t.Fatal(w.Code, core.calls)
		}
	}
}
func TestParseAccountCheck(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		preview    bool
	}{
		{"text", "普通文案：答案是 42。", false},
		{"html", "说明\n```html\n<!doctype html><html><body><script>requestAnimationFrame(()=>{})</script></body></html>\n```", true},
		{"svg", "```svg\n<svg xmlns=\"http://www.w3.org/2000/svg\"><circle><animate attributeName=\"r\" values=\"1;10;1\" dur=\"2s\" repeatCount=\"indefinite\"/></circle></svg>\n```", true},
		{"code", "```js\nconst text = '<html><body>example</body></html>'\n```", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := parseAccountCheck([]byte(checkEvents(tc.text, true)))
			if result.Error != "" || result.Text != tc.text || (result.HTML != "") != tc.preview || result.Model != "test-model" {
				t.Fatalf("%+v", result)
			}
		})
	}
	for _, body := range []string{checkEvents("<html><body>partial</body></html>", false), checkEvents("", true), checkEvents("partial", true) + "\ndata: {\"type\":\"error\"}", checkEvents("partial", false) + "\ndata: {\"type\":\"test_complete\",\"success\":false}", "data: malformed"} {
		result := parseAccountCheck([]byte(body))
		if result.Error == "" || result.HTML != "" {
			t.Fatalf("accepted failure: %+v", result)
		}
	}
}

type accountCheckManagementCore struct {
	coreStub
	paths   []string
	payload any
}

func (f *accountCheckManagementCore) Profile(context.Context, string) (coreclient.User, error) {
	return coreclient.User{ID: 1, Role: "admin", Status: "active"}, nil
}

func (f *accountCheckManagementCore) AdminUserMarketRequest(_ context.Context, method, path string, payload any, accessToken string) (coreclient.Response, error) {
	if accessToken != "token" {
		return coreclient.Response{}, fmt.Errorf("unexpected access token")
	}
	f.paths = append(f.paths, method+" "+path)
	f.payload = payload
	switch {
	case method == http.MethodGet && path == "/api/v1/admin/accounts/42":
		return rentalResponse(checkAccount{ID: 42, Name: "测试账号", Platform: "openai", GroupIDs: []int64{7}})
	case method == http.MethodPut && path == "/api/v1/admin/accounts/42":
		return rentalResponse(checkAccount{ID: 42, Name: "测试账号", Platform: "openai", GroupIDs: []int64{7, 8}})
	case method == http.MethodGet && path == "/api/v1/admin/accounts/42/scheduled-test-plans":
		body, _ := json.Marshal([]customerScheduledTestPlan{{ID: 9, AccountID: 42, ModelID: "gpt-6-astra", CronExpression: "0 * * * *", Enabled: true, MaxResults: 3}})
		return coreclient.Response{StatusCode: 200, Body: body}, nil
	case method == http.MethodPost && path == "/api/v1/admin/scheduled-test-plans":
		body, _ := json.Marshal(customerScheduledTestPlan{ID: 9, AccountID: 42, ModelID: "gpt-6-astra", CronExpression: "0 * * * *", Enabled: true, MaxResults: 3})
		return coreclient.Response{StatusCode: 200, Body: body}, nil
	case method == http.MethodGet && path == "/api/v1/admin/scheduled-test-plans/9/results?limit=3":
		body, _ := json.Marshal([]customerScheduledTestResult{{ID: 3, PlanID: 9, Status: "success", ResponseText: "latest"}})
		return coreclient.Response{StatusCode: 200, Body: body}, nil
	default:
		return coreclient.Response{}, fmt.Errorf("unexpected request %s %s", method, path)
	}
}

func (f *accountCheckManagementCore) AdminUserMarketRawRequest(context.Context, string, string, any, string) (coreclient.Response, error) {
	return coreclient.Response{}, fmt.Errorf("unexpected raw request")
}

func TestAccountCheckUpdatesOnlyGroupIDs(t *testing.T) {
	core := &accountCheckManagementCore{}
	s := &Server{core: core, logger: testLogger()}
	w := checksRequest(s, http.MethodPut, "/api/v1/admin/account-checks/accounts/42/groups", `{"group_ids":[7,8,8]}`, "token")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"group_ids":[7,8]`) {
		t.Fatal(w.Code, w.Body.String())
	}
	payload, ok := core.payload.(map[string]any)
	if !ok || fmt.Sprint(payload["group_ids"]) != "[7 8]" {
		t.Fatalf("unexpected group update payload: %#v", core.payload)
	}
}

func TestAccountCheckScheduledPlansForceThreeResults(t *testing.T) {
	core := &accountCheckManagementCore{}
	s := &Server{core: core, logger: testLogger()}
	w := checksRequest(s, http.MethodPost, "/api/v1/admin/account-checks/accounts/42/scheduled-test-plans", `{"account_id":999,"model_id":"gpt-6-astra","cron_expression":"0 * * * *","max_results":100,"auto_recover":true}`, "token")
	if w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	payload, ok := core.payload.(customerCreateScheduledTestPlanRequest)
	if !ok || payload.AccountID != 42 || payload.MaxResults != 3 || payload.AutoRecover == nil || *payload.AutoRecover {
		t.Fatalf("scheduled plan was not constrained: %#v", core.payload)
	}
}

func TestAccountCheckScheduledResultsAreLimitedToThree(t *testing.T) {
	core := &accountCheckManagementCore{}
	s := &Server{core: core, logger: testLogger()}
	w := checksRequest(s, http.MethodGet, "/api/v1/admin/account-checks/accounts/42/scheduled-test-plans/9/results?limit=3", "", "token")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "latest") {
		t.Fatal(w.Code, w.Body.String())
	}
	if core.paths[len(core.paths)-1] != "GET /api/v1/admin/scheduled-test-plans/9/results?limit=3" {
		t.Fatal(core.paths)
	}
	w = checksRequest(s, http.MethodGet, "/api/v1/admin/account-checks/accounts/42/scheduled-test-plans/9/results?limit=4", "", "token")
	if w.Code != http.StatusBadRequest {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestAccountCheckScheduledRequestAcceptsNullAsAnEmptyList(t *testing.T) {
	core := &accountCheckNullPlansCore{}
	s := &Server{core: core, logger: testLogger()}
	w := checksRequest(s, http.MethodGet, "/api/v1/admin/account-checks/accounts/42/scheduled-test-plans", "", "token")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"data":[]`) {
		t.Fatal(w.Code, w.Body.String())
	}
}

type accountCheckNullPlansCore struct {
	coreStub
}

func (f *accountCheckNullPlansCore) Profile(context.Context, string) (coreclient.User, error) {
	return coreclient.User{ID: 1, Role: "admin", Status: "active"}, nil
}

func (f *accountCheckNullPlansCore) AdminUserMarketRequest(_ context.Context, method, path string, _ any, _ string) (coreclient.Response, error) {
	if method == http.MethodGet && path == "/api/v1/admin/accounts/42" {
		return rentalResponse(checkAccount{ID: 42, Platform: "openai"})
	}
	if method == http.MethodGet && path == "/api/v1/admin/accounts/42/scheduled-test-plans" {
		return coreclient.Response{StatusCode: 200, Body: []byte("null")}, nil
	}
	return coreclient.Response{}, fmt.Errorf("unexpected request %s %s", method, path)
}

func (f *accountCheckNullPlansCore) AdminUserMarketRawRequest(context.Context, string, string, any, string) (coreclient.Response, error) {
	return coreclient.Response{}, fmt.Errorf("unexpected raw request")
}
