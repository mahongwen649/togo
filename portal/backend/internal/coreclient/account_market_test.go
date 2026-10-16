package coreclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAdminMarketRequestUsesServerCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "server-only-key" || r.Header.Get("Authorization") != "" || r.Header.Get("Idempotency-Key") != "rental-order-1" {
			t.Error("incorrect admin authentication")
		}
		if r.URL.Path != "/api/v1/admin/accounts/1" || r.Method != "PUT" {
			t.Error("incorrect route")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body) != 1 || body["group_ids"] == nil {
			t.Error("unexpected account update fields")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":{"id":1}}`))
	}))
	defer server.Close()
	adapter, err := NewHTTPAdapter(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.AdminMarketRequest(context.Background(), "PUT", "/api/v1/admin/accounts/1", nil, ""); err == nil {
		t.Fatal("accepted missing admin credential")
	}
	_, err = adapter.WithAdminAPIKey("server-only-key").AdminMarketRequest(context.Background(), "PUT", "/api/v1/admin/accounts/1", map[string]any{"group_ids": []int64{2}}, "rental-order-1")
	if err != nil {
		t.Fatal(err)
	}
}

func TestAdminMarketRawRequestUsesDedicatedLongTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(30 * time.Millisecond)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"test_complete\",\"success\":true}\n\n"))
	}))
	defer server.Close()

	client := server.Client()
	client.Timeout = 5 * time.Millisecond
	adapter, err := NewHTTPAdapter(server.URL, client)
	if err != nil {
		t.Fatal(err)
	}
	response, err := adapter.WithAdminAPIKey("server-only-key").AdminMarketRawRequest(context.Background(), http.MethodPost, "/api/v1/admin/accounts/1/test", map[string]string{"prompt": "html"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || client.Timeout != 5*time.Millisecond {
		t.Fatalf("unexpected response=%d original_timeout=%s", response.StatusCode, client.Timeout)
	}
}
