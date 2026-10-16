package coreclient

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestAdminUserMarketRequestUsesAdministratorJWTWithoutServerKey(t *testing.T) {
	transport := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/v1/admin/accounts" || r.Header.Get("Authorization") != "Bearer administrator-token" {
			t.Fatalf("request=%s authorization=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		if value := r.Header.Get("x-api-key"); value != "" {
			t.Fatalf("unexpected server key %q", value)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"code":0,"data":[]}`))}, nil
	})
	baseURL, err := url.Parse("https://core.example")
	if err != nil {
		t.Fatal(err)
	}
	adapter := &HTTPAdapter{baseURL: baseURL, client: &http.Client{Transport: transport}, adminAPIKey: "wrong-server-key"}
	response, err := adapter.AdminUserMarketRequest(context.Background(), http.MethodGet, "/api/v1/admin/accounts", nil, "administrator-token")
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("response=%+v err=%v", response, err)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
