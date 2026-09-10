package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	portalauth "github.com/jqcode/portal/backend/internal/auth"
	"github.com/jqcode/portal/backend/internal/company"
	"github.com/jqcode/portal/backend/internal/coreclient"
	"github.com/jqcode/portal/backend/internal/username"
)

type authStub struct {
	loginInput     portalauth.LoginInput
	loginResult    coreclient.AuthResult
	loginErr       error
	registerInput  portalauth.RegisterInput
	registerResult coreclient.AuthResult
	registerErr    error
	renameToken    string
	renameUsername string
	renameResult   coreclient.User
	renameErr      error
	available      bool
	availableErr   error
}

func (s *authStub) Login(_ context.Context, input portalauth.LoginInput) (coreclient.AuthResult, error) {
	s.loginInput = input
	return s.loginResult, s.loginErr
}

func (s *authStub) Register(_ context.Context, input portalauth.RegisterInput) (coreclient.AuthResult, error) {
	s.registerInput = input
	return s.registerResult, s.registerErr
}

func (s *authStub) RenameCurrentUser(_ context.Context, accessToken, username string) (coreclient.User, error) {
	s.renameToken = accessToken
	s.renameUsername = username
	return s.renameResult, s.renameErr
}

func (s *authStub) UsernameAvailable(context.Context, string) (bool, error) {
	return s.available, s.availableErr
}

type coreStub struct {
	response        coreclient.Response
	err             error
	profileResponse coreclient.Response
	userAPIRequest  coreclient.CoreRequest
	userAPIResponse coreclient.Response
}

type directLoginCoreStub struct {
	coreStub
	users       coreclient.UserPage
	loginInput  coreclient.LoginRequest
	loginResult coreclient.AuthResult
}

func (s *directLoginCoreStub) AdminUsers(context.Context, int, int, ...coreclient.AdminUsersOption) (coreclient.UserPage, error) {
	return s.users, nil
}

func (s *directLoginCoreStub) Login(_ context.Context, input coreclient.LoginRequest) (coreclient.AuthResult, error) {
	s.loginInput = input
	return s.loginResult, nil
}

type verifyCodeCoreStub struct {
	coreStub
	request  coreclient.CoreRequest
	response coreclient.Response
	err      error
}

func (s *verifyCodeCoreStub) SendVerifyCodeRequest(_ context.Context, request coreclient.CoreRequest) (coreclient.Response, error) {
	s.request = request
	return s.response, s.err
}

type passwordResetCoreStub struct {
	coreStub
	request  coreclient.CoreRequest
	response coreclient.Response
}

type passwordResetServiceStub struct {
	sendEmail, sendToken, sendIP         string
	resetEmail, resetCode, resetPassword string
}

func (s *passwordResetServiceStub) SendCode(_ context.Context, email, token, ip string) (int, error) {
	s.sendEmail, s.sendToken, s.sendIP = email, token, ip
	return 60, nil
}

func (s *passwordResetServiceStub) ResetPassword(_ context.Context, email, code, password string) error {
	s.resetEmail, s.resetCode, s.resetPassword = email, code, password
	return nil
}

func (s *passwordResetCoreStub) PasswordResetRequest(_ context.Context, request coreclient.CoreRequest) (coreclient.Response, error) {
	s.request = request
	return s.response, nil
}

type financeAggregationCoreStub struct{ coreStub }

func (financeAggregationCoreStub) AdminUser(context.Context, int64) (coreclient.User, error) {
	return coreclient.User{ID: 99, Email: "member@example.com", Username: "member"}, nil
}

func (financeAggregationCoreStub) UserAPIRequest(_ context.Context, request coreclient.CoreRequest) (coreclient.Response, error) {
	if request.Path == "/api/v1/admin/dashboard/groups" {
		return coreclient.Response{StatusCode: http.StatusOK, Body: []byte(`{"code":0,"data":{"groups":[{"group_id":0,"group_name":""},{"group_id":3,"group_name":"legacy"},{"group_id":12,"group_name":"openai"},{"group_id":13,"group_name":"claude"}]}}`)}, nil
	}
	query, _ := url.ParseQuery(request.RawQuery)
	stats := map[string]string{
		"":   `{"total_requests":123856,"total_input_tokens":808948866,"total_output_tokens":52212622,"total_cache_creation_tokens":30870398,"total_cache_read_tokens":11327376502,"total_image_output_tokens":119153,"total_actual_cost":721.372137}`,
		"3":  `{"total_requests":39805,"total_input_tokens":300000000,"total_output_tokens":20000000,"total_cache_creation_tokens":10000000,"total_cache_read_tokens":3000000000,"total_image_output_tokens":30000,"total_actual_cost":256.727257}`,
		"12": `{"total_requests":74001,"total_input_tokens":450000000,"total_output_tokens":30000000,"total_cache_creation_tokens":18000000,"total_cache_read_tokens":7000000000,"total_image_output_tokens":80000,"total_actual_cost":409.414205}`,
		"13": `{"total_requests":10050,"total_input_tokens":58948866,"total_output_tokens":2212622,"total_cache_creation_tokens":2870398,"total_cache_read_tokens":1327376502,"total_image_output_tokens":9153,"total_actual_cost":55.230675}`,
	}[query.Get("group_id")]
	return coreclient.Response{StatusCode: http.StatusOK, Body: []byte(`{"code":0,"data":` + stats + `}`)}, nil
}

type financeMissingUserCoreStub struct{ coreStub }

func (financeMissingUserCoreStub) AdminUser(context.Context, int64) (coreclient.User, error) {
	return coreclient.User{}, coreclient.ErrNotFound
}

func (financeMissingUserCoreStub) UserAPIRequest(_ context.Context, request coreclient.CoreRequest) (coreclient.Response, error) {
	if request.Path == "/api/v1/admin/dashboard/groups" {
		return coreclient.Response{StatusCode: http.StatusNotFound, Body: []byte(`{"code":"NOT_FOUND"}`)}, nil
	}
	return coreclient.Response{StatusCode: http.StatusOK, Body: []byte(`{"code":0,"data":{"total_requests":42,"total_input_tokens":100,"total_output_tokens":20,"total_cache_creation_tokens":3,"total_cache_read_tokens":4,"total_image_output_tokens":5,"total_actual_cost":1.23}}`)}, nil
}

type dashboardStatsCoreStub struct {
	coreStub
	requests []coreclient.CoreRequest
}

func (s *dashboardStatsCoreStub) UserAPIRequest(_ context.Context, request coreclient.CoreRequest) (coreclient.Response, error) {
	s.requests = append(s.requests, request)
	if strings.HasSuffix(request.Path, "/api-keys") {
		return coreclient.Response{StatusCode: http.StatusOK, Body: []byte(`{"code":0,"data":{"items":[{"status":"active"}],"total":1}}`)}, nil
	}
	query, _ := url.ParseQuery(request.RawQuery)
	today := query.Get("start_date") != "1970-01-01"
	switch query.Get("user_id") {
	case "101":
		if today {
			return coreclient.Response{StatusCode: http.StatusOK, Body: []byte(`{"code":0,"data":{"total_requests":1,"total_input_tokens":70,"total_output_tokens":20,"total_cache_creation_tokens":5,"total_cache_read_tokens":5,"total_tokens":100,"total_cost":0.1,"total_actual_cost":0.08,"total_account_cost":0.09}}`)}, nil
		}
		return coreclient.Response{StatusCode: http.StatusOK, Body: []byte(`{"code":0,"data":{"total_requests":10,"total_input_tokens":700,"total_output_tokens":200,"total_cache_creation_tokens":50,"total_cache_read_tokens":50,"total_tokens":1000,"total_cost":1,"total_actual_cost":0.8,"total_account_cost":0.9}}`)}, nil
	case "202":
		if today {
			return coreclient.Response{StatusCode: http.StatusOK, Body: []byte(`{"code":0,"data":{"total_requests":2,"total_input_tokens":140,"total_output_tokens":40,"total_cache_creation_tokens":10,"total_cache_read_tokens":10,"total_tokens":200,"total_cost":0.2,"total_actual_cost":0.16,"total_account_cost":0.18}}`)}, nil
		}
		return coreclient.Response{StatusCode: http.StatusOK, Body: []byte(`{"code":0,"data":{"total_requests":20,"total_input_tokens":1400,"total_output_tokens":400,"total_cache_creation_tokens":100,"total_cache_read_tokens":100,"total_tokens":2000,"total_cost":2,"total_actual_cost":1.6,"total_account_cost":1.8}}`)}, nil
	default:
		return coreclient.Response{StatusCode: http.StatusBadRequest, Body: []byte(`{"code":"UNEXPECTED_USER"}`)}, nil
	}
}

func (s coreStub) PublicSettings(context.Context) (coreclient.Response, error) {
	return s.response, s.err
}

func (s coreStub) Login(context.Context, coreclient.LoginRequest) (coreclient.AuthResult, error) {
	return coreclient.AuthResult{}, errors.New("not implemented")
}

func (s coreStub) Register(context.Context, coreclient.RegisterRequest) (coreclient.AuthResult, error) {
	return coreclient.AuthResult{}, errors.New("not implemented")
}

func (s coreStub) UpdateUsername(context.Context, string, string) error {
	return errors.New("not implemented")
}

func (s coreStub) AdminUser(context.Context, int64) (coreclient.User, error) {
	return coreclient.User{}, errors.New("not implemented")
}

func (s coreStub) AdminUsers(context.Context, int, int, ...coreclient.AdminUsersOption) (coreclient.UserPage, error) {
	return coreclient.UserPage{}, errors.New("not implemented")
}

func (s coreStub) Profile(context.Context, string) (coreclient.User, error) {
	return coreclient.User{}, errors.New("not implemented")
}

func (s coreStub) ProfileResponse(context.Context, string) (coreclient.Response, error) {
	return s.profileResponse, s.err
}

func (s coreStub) UserAPIRequest(context.Context, coreclient.CoreRequest) (coreclient.Response, error) {
	return s.userAPIResponse, s.err
}

func (s coreStub) AdminCreateUser(context.Context, coreclient.AdminCreateUserRequest) (coreclient.User, coreclient.Response, error) {
	return coreclient.User{}, coreclient.Response{}, errors.New("not implemented")
}

func (s coreStub) AdminDeleteUser(context.Context, int64) error { return errors.New("not implemented") }

func (s coreStub) AdminUpdatePassword(context.Context, int64, string) error {
	return errors.New("not implemented")
}

func TestHealth(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	New(coreStub{}, nil, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestUsageTrendBucketHonorsGranularity(t *testing.T) {
	tests := []struct {
		name        string
		createdAt   string
		granularity string
		want        string
	}{
		{name: "hour RFC3339", createdAt: "2026-07-23T15:42:18+08:00", granularity: "hour", want: "2026-07-23 15:00"},
		{name: "hour SQL timestamp", createdAt: "2026-07-23 15:42:18", granularity: "hour", want: "2026-07-23 15:00"},
		{name: "day", createdAt: "2026-07-23T15:42:18+08:00", granularity: "day", want: "2026-07-23"},
		{name: "invalid granularity defaults to day", createdAt: "2026-07-23T15:42:18+08:00", granularity: "week", want: "2026-07-23"},
		{name: "short value is preserved", createdAt: "invalid", granularity: "hour", want: "invalid"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := usageTrendBucket(test.createdAt, test.granularity); got != test.want {
				t.Fatalf("usageTrendBucket(%q, %q) = %q, want %q", test.createdAt, test.granularity, got, test.want)
			}
		})
	}
}

type dashboardTrendCoreStub struct{ coreStub }

func (dashboardTrendCoreStub) UserAPIRequest(_ context.Context, request coreclient.CoreRequest) (coreclient.Response, error) {
	query, _ := url.ParseQuery(request.RawQuery)
	items := map[string]string{
		"101": `[{"created_at":"2026-07-23T08:00:00Z","user_id":101,"input_tokens":20,"user":{"username":"user-101"}}]`,
		"202": `[{"created_at":"2026-07-23T09:00:00Z","user_id":202,"input_tokens":30,"user":{"username":"user-202"}},{"created_at":"2026-07-23T08:00:00Z","user_id":202,"input_tokens":20,"user":{"username":"user-202"}}]`,
		"303": `[{"created_at":"2026-07-23T08:00:00Z","user_id":303,"input_tokens":50,"user":{"username":"user-303"}}]`,
		"404": `[{"created_at":"2026-07-23T08:00:00Z","user_id":404,"input_tokens":5,"user":{"username":"user-404"}}]`,
	}[query.Get("user_id")]
	return coreclient.Response{StatusCode: http.StatusOK, Body: []byte(`{"code":0,"data":{"items":` + items + `,"total":1}}`)}, nil
}

func TestScopedDashboardUsersTrendRanksByTotalTokensAndHonorsLimit(t *testing.T) {
	server := &Server{core: dashboardTrendCoreStub{}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/users-trend?start_date=2026-07-23&end_date=2026-07-23&granularity=hour&limit=2", nil)
	server.scopedDashboardUsersTrendForMembers(recorder, request, "token", map[int64]bool{101: true, 202: true, 303: true, 404: true})

	if recorder.Code != http.StatusOK {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
	var response apiEnvelope[struct {
		Trend []map[string]any `json:"trend"`
	}]
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data.Trend) != 3 {
		t.Fatalf("trend=%v, want two users and three data points", response.Data.Trend)
	}
	wantUserIDs := []int64{202, 202, 303}
	wantDates := []string{"2026-07-23 08:00", "2026-07-23 09:00", "2026-07-23 08:00"}
	for i, point := range response.Data.Trend {
		if userID := int64(numericStat(point["user_id"])); userID != wantUserIDs[i] || stringStat(point["date"]) != wantDates[i] {
			t.Fatalf("trend[%d]=%v, want user_id=%d date=%s", i, point, wantUserIDs[i], wantDates[i])
		}
	}
}

func TestAggregateDashboardStatsUsesOnlyCompanyMembersAndNaturalDay(t *testing.T) {
	core := &dashboardStatsCoreStub{}
	server := &Server{core: core, logger: testLogger()}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/snapshot-v2?user_id=999&start_date=2020-01-01&end_date=2020-01-02", nil)

	stats, ok := server.aggregateDashboardStats(recorder, request, "token", map[int64]bool{101: true, 202: true})
	if !ok {
		t.Fatalf("aggregate failed: %d %s", recorder.Code, recorder.Body.String())
	}
	if got := numericStat(stats["total_tokens"]); got != 3000 {
		t.Fatalf("total_tokens=%v, want 3000", got)
	}
	if got := numericStat(stats["today_tokens"]); got != 300 {
		t.Fatalf("today_tokens=%v, want 300", got)
	}
	if got := numericStat(stats["today_requests"]); got != 3 {
		t.Fatalf("today_requests=%v, want 3", got)
	}
	if got := numericStat(stats["active_users"]); got != 2 {
		t.Fatalf("active_users=%v, want 2", got)
	}

	usageRequests := 0
	for _, sent := range core.requests {
		if sent.Path != "/api/v1/admin/usage/stats" {
			continue
		}
		usageRequests++
		query, err := url.ParseQuery(sent.RawQuery)
		if err != nil {
			t.Fatal(err)
		}
		if query.Get("user_id") != "101" && query.Get("user_id") != "202" {
			t.Fatalf("usage query escaped company scope: %s", sent.RawQuery)
		}
		if query.Get("timezone") != "Asia/Shanghai" || query.Get("start_date") == "2020-01-01" || query.Get("end_date") == "2020-01-02" {
			t.Fatalf("dashboard KPI inherited chart range instead of fixed dates: %s", sent.RawQuery)
		}
		if query.Get("start_date") != "1970-01-01" && query.Get("start_date") != query.Get("end_date") {
			t.Fatalf("unexpected dashboard KPI range: %s", sent.RawQuery)
		}
	}
	if usageRequests != 4 {
		t.Fatalf("usage stats requests=%d, want 4 (total and today for each member)", usageRequests)
	}
}

func TestPublicSettingsPreservesCoreStatusAndBody(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/settings/public", nil)
	New(coreStub{response: coreclient.Response{
		StatusCode:  http.StatusLocked,
		ContentType: "application/json",
		Body:        []byte(`{"code":"LOCKED"}`),
	}}, nil, testLogger()).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusLocked || recorder.Body.String() != `{"code":"LOCKED"}` {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Cache-Control"); got != "private, max-age=15" {
		t.Fatalf("cache control = %q", got)
	}
}

func TestPublicSettingsMapsTransportFailure(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/settings/public", nil)
	New(coreStub{err: errors.New("connection refused")}, nil, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestPublicSettingsDisablesTurnstileWhenPortalVerifierDisabled(t *testing.T) {
	t.Setenv("PORTAL_TURNSTILE_SECRET", "")
	t.Setenv("PORTAL_TURNSTILE_SITE_KEY", "")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/settings/public", nil)
	New(coreStub{response: coreclient.Response{
		StatusCode:  http.StatusOK,
		ContentType: "application/json",
		Body:        []byte(`{"code":0,"data":{"turnstile_enabled":true,"turnstile_site_key":"core-site-key","site_name":"TogoAPI"}}`),
	}}, nil, testLogger()).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	var payload struct {
		Data struct {
			TurnstileEnabled bool   `json:"turnstile_enabled"`
			TurnstileSiteKey string `json:"turnstile_site_key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.TurnstileEnabled {
		t.Fatalf("turnstile_enabled = true, want false")
	}
	if payload.Data.TurnstileSiteKey != "" {
		t.Fatalf("turnstile_site_key = %q, want empty", payload.Data.TurnstileSiteKey)
	}
}

func TestPublicSettingsReportsPortalPasswordResetAvailability(t *testing.T) {
	tests := []struct {
		name        string
		coreEnabled bool
		service     PasswordResetService
		wantEnabled bool
	}{
		{
			name:        "enabled when Portal service is configured",
			coreEnabled: false,
			service:     &passwordResetServiceStub{},
			wantEnabled: true,
		},
		{
			name:        "disabled when Portal service is unavailable",
			coreEnabled: true,
			wantEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/v1/settings/public", nil)
			body := fmt.Sprintf(`{"code":0,"data":{"password_reset_enabled":%t}}`, tt.coreEnabled)
			dependencies := []any{}
			if tt.service != nil {
				dependencies = append(dependencies, tt.service)
			}
			New(coreStub{response: coreclient.Response{
				StatusCode:  http.StatusOK,
				ContentType: "application/json",
				Body:        []byte(body),
			}}, nil, testLogger(), dependencies...).ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d", recorder.Code)
			}
			var payload struct {
				Data struct {
					PasswordResetEnabled bool `json:"password_reset_enabled"`
				} `json:"data"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if payload.Data.PasswordResetEnabled != tt.wantEnabled {
				t.Fatalf("password_reset_enabled = %t, want %t", payload.Data.PasswordResetEnabled, tt.wantEnabled)
			}
		})
	}
}

func TestAuthMePreservesCoreProfileResponse(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	request.Header.Set("Authorization", "Bearer user-token")
	New(coreStub{profileResponse: coreclient.Response{
		StatusCode: http.StatusOK, ContentType: "application/json", Body: []byte(`{"code":0,"data":{"id":7,"balance":12.5,"role":"admin"}}`),
	}}, nil, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != `{"code":0,"data":{"id":7,"balance":12.5,"role":"admin"}}` {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
}

func TestAuthMeRequiresBearerToken(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	New(coreStub{}, nil, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", recorder.Code)
	}
}

type recordingCoreStub struct {
	coreStub
	request    coreclient.CoreRequest
	profile    coreclient.User
	profileErr error
}

func (s *recordingCoreStub) Profile(context.Context, string) (coreclient.User, error) {
	if s.profileErr != nil {
		return coreclient.User{}, s.profileErr
	}
	if s.profile.ID == 0 {
		return coreclient.User{ID: 1, Role: "admin", Status: "active"}, nil
	}
	return s.profile, nil
}

func (s *recordingCoreStub) UserAPIRequest(_ context.Context, input coreclient.CoreRequest) (coreclient.Response, error) {
	s.request = input
	return s.userAPIResponse, s.err
}

func TestCoreUserAPIProxiesAllowedKeysRequest(t *testing.T) {
	core := &recordingCoreStub{coreStub: coreStub{userAPIResponse: coreclient.Response{
		StatusCode: http.StatusOK, ContentType: "application/json", Body: []byte(`{"code":0,"data":{"items":[]}}`),
	}}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/keys?page=1&page_size=20", nil)
	request.Header.Set("Authorization", "Bearer user-token")
	New(core, nil, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != `{"code":0,"data":{"items":[]}}` {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
	if core.request.Path != "/api/v1/keys" || core.request.RawQuery != "page=1&page_size=20" || core.request.AccessToken != "user-token" {
		t.Fatalf("proxied request=%+v", core.request)
	}
}

func TestCoreUserAPIProxiesPaymentCheckoutRequest(t *testing.T) {
	core := &recordingCoreStub{coreStub: coreStub{userAPIResponse: coreclient.Response{
		StatusCode: http.StatusOK, ContentType: "application/json", Body: []byte(`{"code":0,"data":{"methods":{"alipay":{"available":true}}}}`),
	}}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/payment/checkout-info", nil)
	request.Header.Set("Authorization", "Bearer user-token")
	New(core, nil, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
	if core.request.Path != "/api/v1/payment/checkout-info" || core.request.AccessToken != "user-token" {
		t.Fatalf("proxied request=%+v", core.request)
	}
}

type paymentWebhookCoreStub struct {
	coreStub
	request coreclient.CoreRequest
}

func (s *paymentWebhookCoreStub) PaymentWebhookRequest(_ context.Context, input coreclient.CoreRequest) (coreclient.Response, error) {
	s.request = input
	return coreclient.Response{StatusCode: http.StatusOK, ContentType: "text/plain; charset=utf-8", Body: []byte("success")}, nil
}

func TestEasyPayWebhookPreservesPlainTextResponse(t *testing.T) {
	core := &paymentWebhookCoreStub{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/payment/webhook/easypay?out_trade_no=order-1&trade_status=TRADE_SUCCESS", nil)
	New(core, nil, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "success" {
		t.Fatalf("response=%d %q", recorder.Code, recorder.Body.String())
	}
	if core.request.RawQuery != "out_trade_no=order-1&trade_status=TRADE_SUCCESS" || core.request.Path != "/api/v1/payment/webhook/easypay" {
		t.Fatalf("proxied request=%+v", core.request)
	}
}

func TestImgtoolURLReturnsConfiguredExternalLink(t *testing.T) {
	t.Setenv("IMGTOOL_BASE_URL", "http://8.222.223.187/")
	t.Setenv("IMGTOOL_SSO_SECRET", "0123456789abcdef0123456789abcdef")
	core := &recordingCoreStub{}
	core.profile = coreclient.User{ID: 7, Username: "alice", Email: "alice@example.com", Role: "user"}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/integrations/imgtool/sso-url", nil)
	request.Header.Set("Authorization", "Bearer user-token")
	New(core, nil, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"redirect_url":"http://8.222.223.187/sso?ticket=`) {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
	if core.request.Path != "" && core.request.Path != "/api/v1/keys" {
		t.Fatalf("imgtool route should only prepare keys, got %+v", core.request)
	}
}

func TestAggregateFinanceSummaryRowsByCompanyOnly(t *testing.T) {
	byUser := map[int64]company.ScopedMemberRef{
		1: {CompanyID: 10, CompanyName: "鲸奇", CoreUserID: 1},
		2: {CompanyID: 10, CompanyName: "鲸奇", CoreUserID: 2},
		3: {CompanyID: 20, CompanyName: "音跃互娱", CoreUserID: 3},
	}
	rows := []financeUsageRow{
		{UserID: 1, RequestCount: 10, ActualCost: 1.2},
		{UserID: 2, RequestCount: 20, ActualCost: 3.4},
		{UserID: 3, RequestCount: 30, ActualCost: 2.3},
	}
	got := aggregateFinanceSummaryRows(rows, byUser, "")
	if len(got) != 2 {
		t.Fatalf("len=%d, want 2", len(got))
	}
	if got[0].CompanyID != 10 || got[0].MemberCount != 2 || got[0].RequestCount != 30 || got[0].ActualChargedAmount != 4.6 {
		t.Fatalf("unexpected first row: %+v", got[0])
	}
	if got[1].CompanyID != 20 || got[1].MemberCount != 1 || got[1].RequestCount != 30 || got[1].ActualChargedAmount != 2.3 {
		t.Fatalf("unexpected second row: %+v", got[1])
	}
}

func TestAggregateFinanceMemberRowsCanGroupByMemberOrDetail(t *testing.T) {
	byUser := map[int64]company.ScopedMemberRef{
		1: {CompanyID: 10, CompanyName: "鲸奇", CoreUserID: 1},
	}
	groupA := int64(3)
	groupB := int64(12)
	rows := []financeUsageRow{
		{UserID: 1, GroupID: &groupA, Group: &coreName{ID: groupA, Name: "legacy"}, User: &coreName{ID: 1, Username: "member"}, RequestCount: 10, ActualCost: 1.2},
		{UserID: 1, GroupID: &groupB, Group: &coreName{ID: groupB, Name: "openai"}, User: &coreName{ID: 1, Username: "member"}, RequestCount: 20, ActualCost: 3.4},
	}
	summary := aggregateFinanceMemberRows(rows, byUser, "openai", false)
	if len(summary) != 1 {
		t.Fatalf("summary len=%d, want 1", len(summary))
	}
	if summary[0].GroupID != 0 || summary[0].GroupName != "" || summary[0].RequestCount != 30 || summary[0].ActualChargedAmount != 4.6 {
		t.Fatalf("unexpected summary row: %+v", summary[0])
	}
	detail := aggregateFinanceMemberRows(rows, byUser, "openai", true)
	if len(detail) != 2 {
		t.Fatalf("detail len=%d, want 2", len(detail))
	}
}

func TestCoreUserAPIProxiesDashboardAPIKeyUsage(t *testing.T) {
	core := &recordingCoreStub{coreStub: coreStub{userAPIResponse: coreclient.Response{
		StatusCode: http.StatusOK, ContentType: "application/json", Body: []byte(`{"code":0,"data":{"stats":{}}}`),
	}}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/usage/dashboard/api-keys-usage", strings.NewReader(`{"api_key_ids":[1]}`))
	request.Header.Set("Authorization", "Bearer user-token")
	request.Header.Set("Content-Type", "application/json")
	New(core, nil, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != `{"code":0,"data":{"stats":{}}}` {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
	if core.request.Path != "/api/v1/usage/dashboard/api-keys-usage" || string(core.request.Body) != `{"api_key_ids":[1]}` || core.request.ContentType != "application/json" {
		t.Fatalf("proxied request=%+v body=%s", core.request, string(core.request.Body))
	}
}

func TestCoreUserAPIProxiesRedeemHistory(t *testing.T) {
	core := &recordingCoreStub{coreStub: coreStub{userAPIResponse: coreclient.Response{
		StatusCode: http.StatusOK, ContentType: "application/json", Body: []byte(`{"code":0,"data":[]}`),
	}}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/redeem/history", nil)
	request.Header.Set("Authorization", "Bearer user-token")
	New(core, nil, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != `{"code":0,"data":[]}` {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
	if core.request.Path != "/api/v1/redeem/history" || core.request.AccessToken != "user-token" {
		t.Fatalf("proxied request=%+v", core.request)
	}
}

func TestCoreUserAPIProxiesAnnouncementRead(t *testing.T) {
	core := &recordingCoreStub{coreStub: coreStub{userAPIResponse: coreclient.Response{
		StatusCode: http.StatusOK, ContentType: "application/json", Body: []byte(`{"code":0,"message":"success"}`),
	}}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/announcements/42/read", nil)
	request.Header.Set("Authorization", "Bearer user-token")
	New(core, nil, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != `{"code":0,"message":"success"}` {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
	if core.request.Path != "/api/v1/announcements/42/read" || core.request.Method != http.MethodPost {
		t.Fatalf("proxied request=%+v", core.request)
	}
}

func TestUpdateCurrentUserRenamesThroughPortalAuthService(t *testing.T) {
	auth := &authStub{renameResult: coreclient.User{ID: 7, Username: "new-name", Email: "a@example.com"}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/user", strings.NewReader(`{"username":"new-name"}`))
	request.Header.Set("Authorization", "Bearer user-token")
	New(coreStub{}, auth, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"username":"new-name"`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if auth.renameToken != "user-token" || auth.renameUsername != "new-name" {
		t.Fatalf("rename token=%q username=%q", auth.renameToken, auth.renameUsername)
	}
}

func TestUpdateCurrentUserRejectsTakenUsername(t *testing.T) {
	auth := &authStub{renameErr: username.ErrTaken}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/user", strings.NewReader(`{"username":"taken"}`))
	request.Header.Set("Authorization", "Bearer user-token")
	New(coreStub{}, auth, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "USERNAME_TAKEN") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestCoreUserAPIProxiesAdminAnnouncementsReadStatus(t *testing.T) {
	core := &recordingCoreStub{coreStub: coreStub{userAPIResponse: coreclient.Response{
		StatusCode: http.StatusOK, ContentType: "application/json", Body: []byte(`{"code":0,"data":{"items":[]}}`),
	}}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/announcements/7/read-status?page=1", nil)
	request.Header.Set("Authorization", "Bearer admin-token")
	New(core, nil, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != `{"code":0,"data":{"items":[]}}` {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
	if core.request.Path != "/api/v1/admin/announcements/7/read-status" || core.request.RawQuery != "page=1" {
		t.Fatalf("proxied request=%+v", core.request)
	}
}

func TestCoreUserAPIProxiesReadOnlyAdminUsage(t *testing.T) {
	core := &recordingCoreStub{coreStub: coreStub{userAPIResponse: coreclient.Response{
		StatusCode: http.StatusOK, ContentType: "application/json", Body: []byte(`{"code":0,"data":{"items":[]}}`),
	}}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/usage?period=24h", nil)
	request.Header.Set("Authorization", "Bearer admin-token")
	New(core, nil, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != `{"code":0,"data":{"items":[]}}` {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
	if core.request.Path != "/api/v1/admin/usage" || core.request.RawQuery != "period=24h" {
		t.Fatalf("proxied request=%+v", core.request)
	}
}

func TestCoreUserAPIProxiesAdminRequestErrors(t *testing.T) {
	core := &recordingCoreStub{coreStub: coreStub{userAPIResponse: coreclient.Response{
		StatusCode: http.StatusOK, ContentType: "application/json", Body: []byte(`{"code":0,"data":{"items":[]}}`),
	}}}
	server := New(core, nil, testLogger())

	for _, path := range []string{
		"/api/v1/admin/ops/request-errors?page=2&status_codes=500",
		"/api/v1/admin/ops/request-errors/88",
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer admin-token")
		server.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("path=%s response=%d %s", path, recorder.Code, recorder.Body.String())
		}
	}
	if core.request.Path != "/api/v1/admin/ops/request-errors/88" || core.request.AccessToken != "admin-token" {
		t.Fatalf("proxied request=%+v", core.request)
	}
}

func TestCoreUserAPIDoesNotProxyMalformedAdminRequestErrorID(t *testing.T) {
	core := &recordingCoreStub{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/ops/request-errors/not-a-number", nil)
	request.Header.Set("Authorization", "Bearer admin-token")
	New(core, nil, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound || core.request.Path != "" {
		t.Fatalf("status=%d request=%+v", recorder.Code, core.request)
	}
}

func TestCoreUserAPIDoesNotProxyUsageCleanupWrites(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/usage/cleanup-tasks", nil)
	request.Header.Set("Authorization", "Bearer admin-token")
	New(coreStub{}, nil, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestCoreUserAPIProxiesAdminDashboardModels(t *testing.T) {
	core := &recordingCoreStub{coreStub: coreStub{userAPIResponse: coreclient.Response{
		StatusCode: http.StatusOK, ContentType: "application/json", Body: []byte(`{"code":0,"data":{"models":[]}}`),
	}}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/models?period=24h", nil)
	request.Header.Set("Authorization", "Bearer admin-token")
	New(core, nil, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != `{"code":0,"data":{"models":[]}}` {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
	if core.request.Path != "/api/v1/admin/dashboard/models" || core.request.RawQuery != "period=24h" {
		t.Fatalf("proxied request=%+v", core.request)
	}
}

func TestCoreUserAPIDoesNotProxyAdminDashboardBackfill(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/dashboard/aggregation/backfill", nil)
	request.Header.Set("Authorization", "Bearer admin-token")
	New(coreStub{}, nil, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestCoreUserAPIProxiesReadOnlyAdminUsers(t *testing.T) {
	core := &recordingCoreStub{coreStub: coreStub{userAPIResponse: coreclient.Response{
		StatusCode: http.StatusOK, ContentType: "application/json", Body: []byte(`{"code":0,"data":{"items":[]}}`),
	}}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/9/platform-quotas", nil)
	request.Header.Set("Authorization", "Bearer admin-token")
	New(core, nil, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != `{"code":0,"data":{"items":[]}}` {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
	if core.request.Path != "/api/v1/admin/users/9/platform-quotas" {
		t.Fatalf("proxied request=%+v", core.request)
	}
}

func TestCoreAdminAPIProxiesUserLimitUpdate(t *testing.T) {
	core := &recordingCoreStub{coreStub: coreStub{userAPIResponse: coreclient.Response{
		StatusCode: http.StatusOK, ContentType: "application/json", Body: []byte(`{"code":0,"data":{"id":9,"concurrency":10,"rpm_limit":0}}`),
	}}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/9", strings.NewReader(`{"concurrency":10,"rpm_limit":0}`))
	request.Header.Set("Authorization", "Bearer admin-token")
	request.Header.Set("Content-Type", "application/json")

	New(core, nil, testLogger()).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || recorder.Body.String() != `{"code":0,"data":{"id":9,"concurrency":10,"rpm_limit":0}}` {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
	if core.request.Method != http.MethodPut || core.request.Path != "/api/v1/admin/users/9" || core.request.AccessToken != "admin-token" || core.request.ContentType != "application/json" || string(core.request.Body) != `{"concurrency":10,"rpm_limit":0}` {
		t.Fatalf("proxied request=%+v body=%s", core.request, string(core.request.Body))
	}
}

func TestCoreAdminAPIProxiesRetainedUserManagementWrites(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "batch limits", method: http.MethodPost, path: "/api/v1/admin/users/batch-limits"},
		{name: "replace group", method: http.MethodPost, path: "/api/v1/admin/users/9/replace-group"},
		{name: "update platform quotas", method: http.MethodPut, path: "/api/v1/admin/users/9/platform-quotas"},
		{name: "reset platform quota", method: http.MethodPost, path: "/api/v1/admin/users/9/platform-quotas/reset"},
		{name: "update attributes", method: http.MethodPut, path: "/api/v1/admin/users/9/attributes"},
		{name: "change API key group", method: http.MethodPut, path: "/api/v1/admin/api-keys/17"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core := &recordingCoreStub{coreStub: coreStub{userAPIResponse: coreclient.Response{
				StatusCode: http.StatusOK, ContentType: "application/json", Body: []byte(`{"code":0,"data":{}}`),
			}}}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"value":1}`))
			request.Header.Set("Authorization", "Bearer admin-token")
			request.Header.Set("Content-Type", "application/json")

			New(core, nil, testLogger()).ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
			}
			if core.request.Method != tt.method || core.request.Path != tt.path || string(core.request.Body) != `{"value":1}` {
				t.Fatalf("proxied request=%+v body=%s", core.request, string(core.request.Body))
			}
		})
	}
}

func TestCoreAdminAPIRejectsUserLimitUpdateFromNonAdmin(t *testing.T) {
	core := &recordingCoreStub{profile: coreclient.User{ID: 11, Role: "user", Status: "active"}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/9", strings.NewReader(`{"concurrency":10}`))
	request.Header.Set("Authorization", "Bearer user-token")
	request.Header.Set("Content-Type", "application/json")

	New(core, nil, testLogger()).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "FULL_ADMIN_REQUIRED") {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
	if core.request.Path != "" {
		t.Fatalf("core admin update should not be proxied, request=%+v", core.request)
	}
}

func TestCoreAdminAPIRejectsNonAdminProfile(t *testing.T) {
	core := &recordingCoreStub{
		coreStub: coreStub{userAPIResponse: coreclient.Response{
			StatusCode: http.StatusOK, ContentType: "application/json", Body: []byte(`{"code":0,"data":{"items":[]}}`),
		}},
		profile: coreclient.User{ID: 11, Role: "user", Status: "active"},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?page=1", nil)
	request.Header.Set("Authorization", "Bearer user-token")
	New(core, nil, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "FULL_ADMIN_REQUIRED") {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
	if core.request.Path != "" {
		t.Fatalf("core admin route should not be proxied, request=%+v", core.request)
	}
}

type usernameRegistryStub struct {
	reservation username.Reservation
	reserveErr  error
	bindErr     error
	released    bool
	boundUserID int64
	deletedID   int64
}

func (s *usernameRegistryStub) Reserve(_ context.Context, value string) (username.Reservation, error) {
	if s.reserveErr != nil {
		return username.Reservation{}, s.reserveErr
	}
	if s.reservation.Key == "" {
		s.reservation = username.Reservation{Key: strings.ToLower(value), Username: value, LeaseID: "lease"}
	}
	return s.reservation, nil
}

func (s *usernameRegistryStub) Bind(_ context.Context, reservation username.Reservation, coreUserID int64) error {
	if s.bindErr != nil {
		return s.bindErr
	}
	s.reservation = reservation
	s.boundUserID = coreUserID
	return nil
}

func (s *usernameRegistryStub) Release(context.Context, username.Reservation) error {
	s.released = true
	return nil
}

func (s *usernameRegistryStub) DeleteBound(_ context.Context, _ string, coreUserID int64) error {
	s.deletedID = coreUserID
	return nil
}

type adminCreateCoreStub struct {
	coreStub
	profile coreclient.User
	input   coreclient.AdminCreateUserRequest
	user    coreclient.User
	resp    coreclient.Response
	err     error
	deleted int64
}

func (s *adminCreateCoreStub) Profile(context.Context, string) (coreclient.User, error) {
	return s.profile, nil
}

func (s *adminCreateCoreStub) AdminCreateUser(_ context.Context, input coreclient.AdminCreateUserRequest) (coreclient.User, coreclient.Response, error) {
	s.input = input
	return s.user, s.resp, s.err
}

func (s *adminCreateCoreStub) AdminDeleteUser(_ context.Context, userID int64) error {
	s.deleted = userID
	return nil
}

func TestCreateAdminUserUsesRegistryAndCoreAdminCreate(t *testing.T) {
	core := &adminCreateCoreStub{
		profile: coreclient.User{ID: 1, Role: "admin", Status: "active"},
		user:    coreclient.User{ID: 9, Email: "new@example.com", Username: "NewUser", Role: "user", Status: "active"},
		resp:    coreclient.Response{StatusCode: http.StatusOK, ContentType: "application/json", Body: []byte(`{"code":0,"data":{"id":9,"email":"new@example.com","username":"NewUser"}}`)},
	}
	registry := &usernameRegistryStub{reservation: username.Reservation{Key: "newuser", Username: "NewUser", LeaseID: "lease"}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", strings.NewReader(`{"email":" new@example.com ","password":"secret123","username":"NewUser","role":"user","balance":10,"concurrency":2,"rpm_limit":5}`))
	request.Header.Set("Authorization", "Bearer admin-token")
	request.Header.Set("Content-Type", "application/json")

	New(core, nil, testLogger(), registry).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || recorder.Body.String() != `{"code":0,"data":{"id":9,"email":"new@example.com","username":"NewUser"}}` {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
	if core.input.Email != "new@example.com" || core.input.Username != "NewUser" || core.input.Password != "secret123" || core.input.Role != "user" || core.input.Concurrency != 2 || core.input.RPMLimit != 5 {
		t.Fatalf("core input=%+v", core.input)
	}
	if core.input.Balance == nil || *core.input.Balance != 10 {
		t.Fatalf("core balance=%v", core.input.Balance)
	}
	if registry.boundUserID != 9 || registry.released {
		t.Fatalf("registry bound=%d released=%v", registry.boundUserID, registry.released)
	}
}

func TestCreateAdminUserRejectsTakenUsername(t *testing.T) {
	core := &adminCreateCoreStub{profile: coreclient.User{ID: 1, Role: "admin", Status: "active"}}
	registry := &usernameRegistryStub{reserveErr: username.ErrTaken}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", strings.NewReader(`{"email":"new@example.com","password":"secret123","username":"taken"}`))
	request.Header.Set("Authorization", "Bearer admin-token")

	New(core, nil, testLogger(), registry).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "USERNAME_TAKEN") {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
	if core.input.Email != "" {
		t.Fatalf("core should not be called, input=%+v", core.input)
	}
}

func TestCreateAdminUserRequiresAdminRole(t *testing.T) {
	core := &adminCreateCoreStub{profile: coreclient.User{ID: 2, Role: "user", Status: "active"}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", strings.NewReader(`{"email":"new@example.com","password":"secret123","username":"new"}`))
	request.Header.Set("Authorization", "Bearer user-token")

	New(core, nil, testLogger(), &usernameRegistryStub{}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "FORBIDDEN") {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
	if core.input.Email != "" {
		t.Fatalf("core should not be called, input=%+v", core.input)
	}
}

type adminDeleteCoreStub struct {
	coreStub
	profile coreclient.User
	user    coreclient.User
	userErr error
	deleted int64
	delErr  error
}

func (s *adminDeleteCoreStub) Profile(context.Context, string) (coreclient.User, error) {
	return s.profile, nil
}

func (s *adminDeleteCoreStub) AdminUser(context.Context, int64) (coreclient.User, error) {
	return s.user, s.userErr
}

func (s *adminDeleteCoreStub) AdminDeleteUser(_ context.Context, userID int64) error {
	s.deleted = userID
	return s.delErr
}

func TestDeleteAdminUserDeletesCoreUserAndPortalUsername(t *testing.T) {
	core := &adminDeleteCoreStub{
		profile: coreclient.User{ID: 1, Role: "admin", Status: "active"},
		user:    coreclient.User{ID: 9, Username: "OldUser", Role: "user", Status: "active"},
	}
	registry := &usernameRegistryStub{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/9", nil)
	request.Header.Set("Authorization", "Bearer admin-token")

	New(core, nil, testLogger(), registry).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"id":9`) {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
	if core.deleted != 9 {
		t.Fatalf("deleted=%d", core.deleted)
	}
	if registry.deletedID != 9 {
		t.Fatalf("registry deleted=%d", registry.deletedID)
	}
}

func TestDeleteAdminUserRequiresAdminRole(t *testing.T) {
	core := &adminDeleteCoreStub{profile: coreclient.User{ID: 2, Role: "user", Status: "active"}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/9", nil)
	request.Header.Set("Authorization", "Bearer user-token")

	New(core, nil, testLogger(), &usernameRegistryStub{}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "FORBIDDEN") {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
	if core.deleted != 0 {
		t.Fatalf("core should not be called, deleted=%d", core.deleted)
	}
}

func TestCoreUserAPIProxiesAdminUserBalanceWrites(t *testing.T) {
	core := &recordingCoreStub{coreStub: coreStub{userAPIResponse: coreclient.Response{
		StatusCode: http.StatusOK, ContentType: "application/json", Body: []byte(`{"code":0,"data":{"id":9,"balance":101}}`),
	}}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/9/balance", strings.NewReader(`{"balance":1,"operation":"add","notes":"manual recharge"}`))
	request.Header.Set("Authorization", "Bearer admin-token")
	request.Header.Set("Content-Type", "application/json")

	New(core, nil, testLogger()).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || recorder.Body.String() != `{"code":0,"data":{"id":9,"balance":101}}` {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
	if core.request.Path != "/api/v1/admin/users/9/balance" || core.request.Method != http.MethodPost || string(core.request.Body) != `{"balance":1,"operation":"add","notes":"manual recharge"}` {
		t.Fatalf("proxied request=%+v body=%s", core.request, string(core.request.Body))
	}
}

func TestCoreUserAPIDoesNotProxyUnlistedAdminUserWrites(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/9/auth-identities", nil)
	request.Header.Set("Authorization", "Bearer admin-token")
	New(coreStub{}, nil, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestCoreUserAPIRejectsUnlistedRoute(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/groups/rates", nil)
	request.Header.Set("Authorization", "Bearer user-token")
	New(coreStub{}, nil, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestCoreUserAPIProxiesAvailableChannels(t *testing.T) {
	core := &recordingCoreStub{coreStub: coreStub{userAPIResponse: coreclient.Response{
		StatusCode: http.StatusOK, ContentType: "application/json", Body: []byte(`{"code":0,"data":[]}`),
	}}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/channels/available", nil)
	request.Header.Set("Authorization", "Bearer user-token")
	request.Header.Set("Referer", "https://togoapi.com/recharge")
	request.Host = "togoapi.com"

	New(core, nil, testLogger()).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || recorder.Body.String() != `{"code":0,"data":[]}` {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
	if core.request.Path != "/api/v1/channels/available" || core.request.Method != http.MethodGet || core.request.AccessToken != "user-token" {
		t.Fatalf("proxied request=%+v", core.request)
	}
	if core.request.Host != "togoapi.com" || core.request.Referer != "https://togoapi.com/recharge" {
		t.Fatalf("proxied source host=%q referer=%q", core.request.Host, core.request.Referer)
	}
}

func TestCoreUserAPIProxiesChannelMonitorReads(t *testing.T) {
	tests := []string{
		"/api/v1/channel-monitors",
		"/api/v1/channel-monitors/42/status",
	}
	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			core := &recordingCoreStub{coreStub: coreStub{userAPIResponse: coreclient.Response{
				StatusCode: http.StatusOK, ContentType: "application/json", Body: []byte(`{"code":0,"data":{}}`),
			}}}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.Header.Set("Authorization", "Bearer user-token")

			New(core, nil, testLogger()).ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
			}
			if core.request.Path != path || core.request.Method != http.MethodGet || core.request.AccessToken != "user-token" {
				t.Fatalf("proxied request=%+v", core.request)
			}
		})
	}
}

func TestCoreUserAPIRejectsInvalidChannelMonitorDetailPath(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/channel-monitors/not-a-number/status", nil)
	request.Header.Set("Authorization", "Bearer user-token")

	New(coreStub{}, nil, testLogger()).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestLoginAcceptsUnifiedIdentifierContract(t *testing.T) {
	auth := &authStub{loginResult: coreclient.AuthResult{Response: coreclient.Response{
		StatusCode: http.StatusOK, ContentType: "application/json", Body: []byte(`{"code":0,"data":{"access_token":"redacted"}}`),
	}}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"identifier":"Alice","password":"secret"}`))
	New(coreStub{}, auth, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || auth.loginInput.Identifier != "Alice" || auth.loginInput.Password != "secret" {
		t.Fatalf("status=%d input=%+v", recorder.Code, auth.loginInput)
	}
}

func TestDirectCoreLoginResolvesUsernameToEmail(t *testing.T) {
	core := &directLoginCoreStub{
		users: coreclient.UserPage{Items: []coreclient.User{{ID: 7, Username: "Admin", Email: "admin@example.com"}}},
		loginResult: coreclient.AuthResult{Response: coreclient.Response{
			StatusCode: http.StatusOK, ContentType: "application/json", Body: []byte(`{"code":0,"data":{"access_token":"redacted"}}`),
		}},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"identifier":"admin","password":"secret"}`))
	New(core, nil, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || core.loginInput.Email != "admin@example.com" || core.loginInput.Password != "secret" {
		t.Fatalf("status=%d input=%+v body=%s", recorder.Code, core.loginInput, recorder.Body.String())
	}
}

func TestDirectLoginCanProxyPortalUpstream(t *testing.T) {
	var received loginRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/login" || r.Method != http.MethodPost {
			t.Fatalf("request=%s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":0,"data":{"access_token":"redacted"}}`)
	}))
	defer upstream.Close()
	t.Setenv("PORTAL_AUTH_UPSTREAM_URL", upstream.URL)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"identifier":"admin","password":"secret"}`))
	New(coreStub{}, nil, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || received.Identifier != "admin" || received.Password != "secret" {
		t.Fatalf("status=%d received=%+v body=%s", recorder.Code, received, recorder.Body.String())
	}
}

func TestSendVerifyCodeProxiesAnonymousRequestToCore(t *testing.T) {
	core := &verifyCodeCoreStub{response: coreclient.Response{
		StatusCode: http.StatusOK, ContentType: "application/json", Body: []byte(`{"code":0,"data":{"countdown":60}}`),
	}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/send-verify-code", strings.NewReader(`{"email":"user@example.com"}`))
	request.Header.Set("Content-Type", "application/json")

	New(core, nil, testLogger()).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || recorder.Body.String() != `{"code":0,"data":{"countdown":60}}` {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
	if core.request.Method != http.MethodPost || core.request.Path != "/api/v1/auth/send-verify-code" || core.request.AccessToken != "" {
		t.Fatalf("proxied request=%+v", core.request)
	}
	if core.request.ContentType != "application/json" || string(core.request.Body) != `{"email":"user@example.com"}` {
		t.Fatalf("proxied content-type=%q body=%s", core.request.ContentType, core.request.Body)
	}
}

func TestPasswordResetUsesPortalVerificationCodeService(t *testing.T) {
	service := &passwordResetServiceStub{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", strings.NewReader(`{"email":"user@example.com","turnstile_token":"human"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Forwarded-For", "203.0.113.8")
	New(coreStub{}, nil, testLogger(), service).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.sendEmail != "user@example.com" || service.sendToken != "human" || service.sendIP != "203.0.113.8" {
		t.Fatalf("response=%d service=%+v", recorder.Code, service)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", strings.NewReader(`{"email":"user@example.com","verify_code":"123456","new_password":"new-secret"}`))
	request.Header.Set("Content-Type", "application/json")
	New(coreStub{}, nil, testLogger(), service).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.resetEmail != "user@example.com" || service.resetCode != "123456" || service.resetPassword != "new-secret" {
		t.Fatalf("response=%d service=%+v", recorder.Code, service)
	}
}

func TestUnknownUsernameDoesNotRevealExistence(t *testing.T) {
	auth := &authStub{loginErr: username.ErrNotFound}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"identifier":"missing","password":"secret"}`))
	New(coreStub{}, auth, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized || !strings.Contains(recorder.Body.String(), "INVALID_CREDENTIALS") {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
}

func TestCoreInvalidPasswordUsesSamePortalError(t *testing.T) {
	auth := &authStub{loginResult: coreclient.AuthResult{Response: coreclient.Response{
		StatusCode:  http.StatusUnauthorized,
		ContentType: "application/json",
		Body:        []byte(`{"code":401,"message":"invalid email or password"}`),
	}}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"identifier":"user@example.com","password":"wrong"}`))
	New(coreStub{}, auth, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized || recorder.Body.String() != "{\"code\":\"INVALID_CREDENTIALS\",\"message\":\"Invalid account or password\"}\n" {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
}

func TestRegisterRejectsTakenUsername(t *testing.T) {
	auth := &authStub{registerErr: username.ErrTaken}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"username":"Alice","email":"a@example.com","password":"secret"}`))
	New(coreStub{}, auth, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "USERNAME_TAKEN") {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
}

func TestRegisterForwardsReferralFields(t *testing.T) {
	auth := &authStub{registerResult: coreclient.AuthResult{Response: coreclient.Response{
		StatusCode: http.StatusOK, ContentType: "application/json", Body: []byte(`{"code":0}`),
	}}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(
		`{"username":"Alice","email":"a@example.com","password":"secret123","verify_code":"123456","promo_code":"PROMO","invitation_code":"INVITE","aff_code":"AFF123"}`,
	))
	New(coreStub{}, auth, testLogger()).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
	if auth.registerInput.PromoCode != "PROMO" || auth.registerInput.InvitationCode != "INVITE" || auth.registerInput.AffCode != "AFF123" {
		t.Fatalf("input=%+v", auth.registerInput)
	}
}

func TestUsernameAvailabilityIsAdvisory(t *testing.T) {
	auth := &authStub{available: false}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/username-available?username=Alice", nil)
	New(coreStub{}, auth, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"available":false`) {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
}

func TestFinanceUsageRowsUsesExactAggregatesAndDoesNotDoubleCountUngrouped(t *testing.T) {
	server := &Server{core: financeAggregationCoreStub{}}
	rows, err := server.financeUsageRowsForMember(context.Background(), "token", url.Values{
		"start_date": {"2026-07-01"}, "end_date": {"2026-07-31"}, "timezone": {"Asia/Shanghai"},
	}, 99)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows=%d, want 3 named groups", len(rows))
	}
	var requests, input, output, cacheCreation, cacheRead, image int64
	var cost float64
	for _, row := range rows {
		requests += row.RequestCount
		input += row.InputTokens
		output += row.OutputTokens
		cacheCreation += row.CacheCreationTokens
		cacheRead += row.CacheReadTokens
		image += row.ImageOutputTokens
		cost += row.ActualCost
	}
	if requests != 123856 || input != 808948866 || output != 52212622 || cacheCreation != 30870398 || cacheRead != 11327376502 || image != 119153 {
		t.Fatalf("unexpected totals: requests=%d input=%d output=%d cache_creation=%d cache_read=%d image=%d", requests, input, output, cacheCreation, cacheRead, image)
	}
	if math.Abs(cost-721.372137) > 0.0000001 {
		t.Fatalf("cost=%f", cost)
	}
}

func TestFinanceUsageRowsKeepsSpendingForMissingCoreUser(t *testing.T) {
	server := &Server{core: financeMissingUserCoreStub{}}
	rows, err := server.financeUsageRowsForMember(context.Background(), "token", url.Values{}, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows=%d, want 1", len(rows))
	}
	if rows[0].RequestCount != 42 || rows[0].ActualCost != 1.23 || financeUsername(rows[0]) != "30" || financeUserDeletedAt(rows[0]) == "" {
		t.Fatalf("unexpected row=%+v username=%q deleted_at=%q", rows[0], financeUsername(rows[0]), financeUserDeletedAt(rows[0]))
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
