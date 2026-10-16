package coreclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPelicanPromptStopsOnStreamRateLimitAndPreservesAccount(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer selected-key" || r.Header.Get("Session-Id") != "unique-probe" || r.Header.Get("x-api-key") != "" {
			t.Error("incorrect upstream identity")
		}
		var payload map[string]any
		if json.NewDecoder(r.Body).Decode(&payload) != nil || payload["model"] != "mapped-model" {
			t.Error("model mapping missing")
		}
		if payload["reasoning"].(map[string]any)["effort"] != "low" {
			t.Error("reasoning effort missing")
		}
		fmt.Fprint(w, "data: {\"type\":\"response.failed\",\"response\":{\"error\":{\"code\":\"rate_limit_exceeded\",\"message\":\"selected-key\"}}}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer upstream.Close()
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/v1/admin/accounts/data" || r.URL.Query().Get("ids") != "42" || r.Header.Get("x-api-key") != "server-key" {
			t.Error("incorrect Core export authorization")
		}
		json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"accounts": []promptAccount{{Platform: "openai", Type: "apikey", Credentials: map[string]any{"base_url": upstream.URL, "api_key": "selected-key", "model_mapping": map[string]any{"requested-model": "mapped-model"}}}}}})
	}))
	defer core.Close()
	adapter, _ := NewHTTPAdapter(core.URL, core.Client())
	adapter = adapter.WithAdminAPIKey("server-key")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	response, err := adapter.PelicanAccountPromptRequest(ctx, 42, "requested-model", "draw a pelican", "unique-probe", "low")
	if err != nil {
		t.Fatalf("waited for stream EOF instead of terminal error: %v", err)
	}
	if !strings.Contains(string(response.Body), `"success":false`) || strings.Contains(string(response.Body), "selected-key") {
		t.Fatalf("unsafe or successful failure result: %s", response.Body)
	}
}

func TestPelicanPromptExportDenialDoesNotContactUpstream(t *testing.T) {
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) }))
	defer core.Close()
	adapter, _ := NewHTTPAdapter(core.URL, core.Client())
	_, err := adapter.WithAdminAPIKey("server-key").PelicanAccountPromptRequest(context.Background(), 42, "model", "draw", "probe")
	if err == nil {
		t.Fatal("export denial was ignored")
	}
}
