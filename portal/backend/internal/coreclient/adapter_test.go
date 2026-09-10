package coreclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestPublicSettingsUsesOfficialCoreRoute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/settings/public" {
			t.Fatalf("unexpected Core request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"registration_enabled":true}`))
	}))
	defer server.Close()

	adapter, err := NewHTTPAdapter(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	response, err := adapter.PublicSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	if string(response.Body) != `{"registration_enabled":true}` {
		t.Fatalf("body = %s", response.Body)
	}
}

func TestBalanceAlertSettingsUsesPublicSettings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"data":{"balance_low_notify_enabled":true,"balance_low_notify_threshold":3,"balance_low_notify_recharge_url":"https://example.com/recharge","site_name":"TogoAPI"}}`))
	}))
	defer server.Close()

	adapter, _ := NewHTTPAdapter(server.URL, server.Client())
	settings, err := adapter.BalanceAlertSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !settings.Enabled || settings.Threshold != 3 || settings.RechargeURL != "https://example.com/recharge" || settings.SiteName != "TogoAPI" {
		t.Fatalf("settings = %+v", settings)
	}
}

func TestNewHTTPAdapterRejectsInvalidBaseURL(t *testing.T) {
	if _, err := NewHTTPAdapter("localhost:18080", http.DefaultClient); err == nil {
		t.Fatal("expected invalid Core base URL error")
	}
}

func TestUpdateUsernameUsesOfficialCoreUserRoute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v1/user" {
			t.Fatalf("unexpected Core request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
			t.Fatalf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":{"username":"Alice"}}`))
	}))
	defer server.Close()

	adapter, err := NewHTTPAdapter(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.UpdateUsername(context.Background(), "access-token", "Alice"); err != nil {
		t.Fatal(err)
	}
}

func TestAdminAddBalanceUsesCampaignIdempotencyKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/admin/users/7/balance" {
			t.Fatalf("unexpected Core request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "admin-key" || r.Header.Get("Idempotency-Key") != "portal-recharge-bonus-42" {
			t.Fatalf("missing Core admin or idempotency header")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["balance"] != float64(5) || body["operation"] != "add" {
			t.Fatalf("body=%v", body)
		}
		_, _ = w.Write([]byte(`{"code":0,"data":{"id":7,"balance":55}}`))
	}))
	defer server.Close()

	adapter, _ := NewHTTPAdapter(server.URL, server.Client())
	err := adapter.WithAdminAPIKey("admin-key").AdminAddBalance(
		context.Background(), 7, 5, "Portal campaign", "portal-recharge-bonus-42",
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAdminUsersUsesOfficialPaginationContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/admin/users" || r.URL.Query().Get("page") != "2" || r.URL.Query().Get("page_size") != "50" {
			t.Fatalf("unexpected Core request: %s", r.URL.String())
		}
		if r.URL.Query().Get("include_subscriptions") != "false" || r.Header.Get("x-api-key") != "admin-key" {
			t.Fatalf("missing Core admin contract")
		}
		_, _ = w.Write([]byte(`{"code":0,"data":{"items":[{"id":7,"username":"Alice","balance":2.5,"balance_notify_enabled":true,"balance_notify_threshold_type":"fixed","balance_notify_threshold":3,"total_recharged":10,"balance_notify_extra_emails":[{"email":"alerts@example.com","verified":true,"disabled":false}]}],"page":2,"page_size":50,"pages":3,"total":101}}`))
	}))
	defer server.Close()

	adapter, err := NewHTTPAdapter(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	page, err := adapter.WithAdminAPIKey("admin-key").AdminUsers(context.Background(), 2, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != 7 || page.Pages != 3 || !page.Items[0].BalanceNotifyEnabled || page.Items[0].BalanceNotifyThreshold == nil || len(page.Items[0].BalanceNotifyExtraEmails) != 1 {
		t.Fatalf("page = %+v", page)
	}
}

func TestProfileUsesOfficialCoreRoute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/user/profile" || r.Header.Get("Authorization") != "Bearer user-token" {
			t.Fatalf("unexpected Core profile request")
		}
		_, _ = w.Write([]byte(`{"code":0,"data":{"id":7,"role":"admin","status":"active"}}`))
	}))
	defer server.Close()
	adapter, _ := NewHTTPAdapter(server.URL, server.Client())
	user, err := adapter.Profile(context.Background(), "user-token")
	if err != nil {
		t.Fatal(err)
	}
	if user.ID != 7 || user.Role != "admin" {
		t.Fatalf("user = %+v", user)
	}
}

func TestProfileResponsePreservesAllCoreFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/user/profile" || r.Header.Get("Authorization") != "Bearer user-token" {
			t.Fatalf("unexpected Core profile request")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":{"id":7,"balance":12.5,"allowed_groups":[1]}}`))
	}))
	defer server.Close()
	adapter, _ := NewHTTPAdapter(server.URL, server.Client())
	response, err := adapter.ProfileResponse(context.Background(), "user-token")
	if err != nil {
		t.Fatal(err)
	}
	if string(response.Body) != `{"code":0,"data":{"id":7,"balance":12.5,"allowed_groups":[1]}}` {
		t.Fatalf("body = %s", response.Body)
	}
}

func TestUserAPIRequestPreservesPortalSourceForPaymentReturnValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "togoapi.com" || r.Referer() != "https://togoapi.com/recharge" {
			t.Fatalf("source host=%q referer=%q", r.Host, r.Referer())
		}
		if r.Header.Get("Authorization") != "Bearer user-token" {
			t.Fatalf("authorization=%q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"code":0}`))
	}))
	defer server.Close()

	adapter, _ := NewHTTPAdapter(server.URL, server.Client())
	_, err := adapter.UserAPIRequest(context.Background(), CoreRequest{
		Method: http.MethodPost, Path: "/api/v1/payment/orders", AccessToken: "user-token",
		Host: "togoapi.com", Referer: "https://togoapi.com/recharge",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUserAPIRequestKeepsHTTPSUpstreamHostForPublicCore(t *testing.T) {
	var upstreamHost string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != upstreamHost || r.Referer() != "http://127.0.0.1:3000/recharge" {
			t.Fatalf("source host=%q referer=%q", r.Host, r.Referer())
		}
		if r.Header.Get("Authorization") != "Bearer user-token" {
			t.Fatalf("authorization=%q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"code":0}`))
	}))
	defer server.Close()
	upstreamURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	upstreamHost = upstreamURL.Host

	adapter, _ := NewHTTPAdapter(server.URL, server.Client())
	_, err = adapter.UserAPIRequest(context.Background(), CoreRequest{
		Method: http.MethodGet, Path: "/api/v1/payment/checkout-info", AccessToken: "user-token",
		Host: "127.0.0.1:3000", Referer: "http://127.0.0.1:3000/recharge",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAdminCreateAndDeleteUserUseOfficialRoutes(t *testing.T) {
	var deleted bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "admin-key" {
			t.Fatal("missing admin API key")
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/admin/users":
			_, _ = w.Write([]byte(`{"code":0,"data":{"id":8,"username":"Alice","role":"user"}}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/admin/users/8":
			deleted = true
			_, _ = w.Write([]byte(`{"code":0,"data":{"message":"deleted"}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	adapter, _ := NewHTTPAdapter(server.URL, server.Client())
	adapter = adapter.WithAdminAPIKey("admin-key")
	user, response, err := adapter.AdminCreateUser(context.Background(), AdminCreateUserRequest{Email: "a@example.com", Password: "secret", Username: "Alice", Role: "user"})
	if err != nil || response.StatusCode != http.StatusOK || user.ID != 8 {
		t.Fatalf("user=%+v response=%+v err=%v", user, response, err)
	}
	if err := adapter.AdminDeleteUser(context.Background(), 8); err != nil || !deleted {
		t.Fatalf("deleted=%v err=%v", deleted, err)
	}
}

func TestAdminUpdatePasswordUsesScopedCoreAdminRoute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v1/admin/users/42" || r.Header.Get("x-api-key") != "admin-key" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		defer r.Body.Close()
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload["password"] != "new-secret" || len(payload) != 1 {
			t.Fatalf("payload=%v err=%v", payload, err)
		}
		_, _ = w.Write([]byte(`{"code":0,"data":{}}`))
	}))
	defer server.Close()
	adapter, _ := NewHTTPAdapter(server.URL, server.Client())
	if err := adapter.WithAdminAPIKey("admin-key").AdminUpdatePassword(context.Background(), 42, "new-secret"); err != nil {
		t.Fatal(err)
	}
}
