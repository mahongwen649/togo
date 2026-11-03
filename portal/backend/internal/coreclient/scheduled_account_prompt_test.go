package coreclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestScheduledAccountPromptUsesReadOnlyCoreAndSelectedEffort(t *testing.T) {
	for _, chat := range []bool{false, true} {
		for _, effort := range []string{"", "none", "high", "xhigh"} {
			t.Run(fmt.Sprintf("chat=%v/%s", chat, effort), func(t *testing.T) {
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("Authorization") != "Bearer selected-key" || r.Header.Get("Session-Id") != "scheduled-run" {
						t.Error("wrong account or missing run identity")
					}
					var payload map[string]any
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Error(err)
					}
					if payload["model"] != "mapped-model" {
						t.Error("model mapping lost")
					}
					if !strings.Contains(fmt.Sprint(payload), "saved question") {
						t.Error("saved question lost")
					}
					key := "reasoning"
					var got any
					if chat {
						key = "reasoning_effort"
						got = payload[key]
					} else if value, ok := payload[key].(map[string]any); ok {
						got = value["effort"]
					}
					if effort == "" {
						if _, ok := payload[key]; ok {
							t.Error("model default overridden")
						}
					} else if got != effort {
						t.Errorf("effort = %v", got)
					}
					if chat {
						fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"answer\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
					} else {
						fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"answer\"}\n\ndata: {\"type\":\"response.completed\"}\n\n")
					}
				}))
				defer upstream.Close()
				core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method != "GET" || r.URL.Path != "/api/v1/admin/accounts/data" || r.URL.Query().Get("ids") != "42" || r.Header.Get("x-api-key") != "server-key" {
						t.Error("Core was mutated or wrong credentials used")
					}
					extra := map[string]any{}
					if chat {
						extra["openai_chat_completions_only"] = true
						extra["openai_responses_supported"] = false
					}
					json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"accounts": []promptAccount{{Platform: "openai", Type: "apikey", Credentials: map[string]any{"base_url": upstream.URL, "api_key": "selected-key", "model_mapping": map[string]any{"model": "mapped-model"}}, Extra: extra}}}})
				}))
				defer core.Close()
				adapter, _ := NewHTTPAdapter(core.URL, core.Client())
				response, err := adapter.WithAdminAPIKey("server-key").ScheduledAccountPromptRequest(context.Background(), 42, "model", "saved question", "scheduled-run", effort)
				if err != nil || !strings.Contains(string(response.Body), "answer") {
					t.Fatal(err, string(response.Body))
				}
			})
		}
	}
}
