package coreclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAccountPromptRequestForwardsSelectedQuestion(t *testing.T) {
	for _, chat := range []bool{false, true} {
		t.Run(fmt.Sprintf("chat=%v", chat), func(t *testing.T) {
			questions := map[int64]string{42: "生成 html，内容是 svg 绘制鹈鹕骑自行车 2D 动画", 43: "只回答 17 + 26 的结果\n不用解释"}
			seen := make(map[int64]string)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				id := int64(42)
				if r.Header.Get("Authorization") == "Bearer key-43" {
					id = 43
				}
				if r.Method != "POST" || r.Header.Get("Authorization") != fmt.Sprintf("Bearer key-%d", id) || r.Header.Get("x-api-key") != "" {
					t.Error("wrong upstream authentication or method")
				}
				var body struct {
					Model string `json:"model"`
					Input []struct {
						Content []struct {
							Text string `json:"text"`
						} `json:"content"`
					} `json:"input"`
					Messages []struct {
						Content string `json:"content"`
					} `json:"messages"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body.Model != "upstream-model" {
					t.Errorf("model=%s", body.Model)
				}
				if chat {
					if r.URL.Path != "/v1/chat/completions" || len(body.Messages) != 1 {
						t.Error("wrong chat request")
						return
					}
					seen[id] = body.Messages[0].Content
					fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"answer\"},\"finish_reason\":null}]}\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
				} else {
					if r.URL.Path != "/v1/responses" || len(body.Input) != 1 || len(body.Input[0].Content) != 1 {
						t.Error("wrong responses request")
						return
					}
					seen[id] = body.Input[0].Content[0].Text
					fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"answer\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n")
				}
			}))
			defer upstream.Close()
			core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Path != "/api/v1/admin/accounts/data" || r.URL.Query().Get("include_proxies") != "true" || r.Header.Get("Authorization") != "Bearer admin-jwt" || r.Header.Get("x-api-key") != "" {
					t.Error("unexpected Core request; must not use connectivity test or server key")
				}
				id := r.URL.Query().Get("ids")
				if id != "42" && id != "43" {
					t.Fatal("not a single selected account")
				}
				json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"accounts": []promptAccount{{
					Platform: "openai", Type: "apikey", Credentials: map[string]any{"base_url": upstream.URL, "api_key": "key-" + id, "model_mapping": map[string]any{"selected-model": "upstream-model"}},
					Extra: map[string]any{"openai_responses_supported": !chat},
				}}}})
			}))
			defer core.Close()
			adapter, _ := NewHTTPAdapter(core.URL, core.Client())
			adapter = adapter.WithAdminAPIKey("must-not-be-used")
			for id, question := range questions {
				response, err := adapter.AccountPromptRequest(context.Background(), id, "selected-model", question, "admin-jwt")
				if err != nil || !strings.Contains(string(response.Body), `"success":true`) || !strings.Contains(string(response.Body), "answer") {
					t.Fatalf("response=%s err=%v", response.Body, err)
				}
				if seen[id] != question {
					t.Fatalf("account %d received %q, want %q", id, seen[id], question)
				}
			}
			// Shared mode sends the same question independently to both accounts.
			for _, id := range []int64{42, 43} {
				_, err := adapter.AccountPromptRequest(context.Background(), id, "selected-model", questions[42], "admin-jwt")
				if err != nil || seen[id] != questions[42] {
					t.Fatalf("shared question lost for %d: %v", id, err)
				}
			}
		})
	}
}

func TestAccountPromptOAuthRequest(t *testing.T) {
	a := promptAccount{Platform: "openai", Type: "oauth", Credentials: map[string]any{"access_token": "private-token", "chatgpt_account_id": "organization", "user_agent": "configured-agent"}}
	r, model, chat, err := a.promptRequest(context.Background(), "gpt-6-astra", "原样保留\n这道题")
	if err != nil {
		t.Fatal(err)
	}
	if chat || model != "gpt-6-astra" || r.URL.String() != "https://chatgpt.com/backend-api/codex/responses" || r.Header.Get("Authorization") != "Bearer private-token" || r.Header.Get("ChatGPT-Account-ID") != "organization" || r.Header.Get("User-Agent") != "configured-agent" {
		t.Fatal("wrong OAuth request")
	}
	data, _ := io.ReadAll(r.Body)
	var body map[string]any
	json.Unmarshal(data, &body)
	if body["store"] != false || body["stream"] != true || !strings.Contains(string(data), `原样保留\n这道题`) || strings.Contains(string(data), "private-token") {
		t.Fatalf("body=%s", data)
	}
}

func TestAccountPromptRejectsDeniedExportWithoutFallback(t *testing.T) {
	calls := 0
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("x-api-key") != "" {
			t.Error("used server key")
		}
		w.WriteHeader(403)
		fmt.Fprint(w, `{"code":"STEP_UP_REQUIRED","message":"private diagnostic"}`)
	}))
	defer core.Close()
	a, _ := NewHTTPAdapter(core.URL, core.Client())
	a = a.WithAdminAPIKey("server-secret")
	_, err := a.AccountPromptRequest(context.Background(), 42, "model", "question", "jwt")
	if calls != 1 || err == nil || !strings.Contains(err.Error(), "二次验证") || strings.Contains(err.Error(), "private diagnostic") {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
	_, err = a.AccountPromptRequest(context.Background(), 42, "model", " \n ", "jwt")
	if calls != 1 || err == nil {
		t.Fatal("blank question sent upstream")
	}
}

func TestAccountPromptDoesNotFollowRedirectOrExposeProviderErrors(t *testing.T) {
	for _, status := range []int{302, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			redirected := false
			destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected = true }))
			defer destination.Close()
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", destination.URL)
				w.WriteHeader(status)
				fmt.Fprint(w, "provider echoed private-api-key and internal details")
			}))
			defer upstream.Close()
			core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"accounts": []promptAccount{{Platform: "openai", Type: "apikey", Credentials: map[string]any{"base_url": upstream.URL, "api_key": "private-api-key"}}}}})
			}))
			defer core.Close()
			a, _ := NewHTTPAdapter(core.URL, core.Client())
			response, err := a.AccountPromptRequest(context.Background(), 42, "model", "question", "jwt")
			if err != nil || redirected || strings.Contains(string(response.Body), "private-api-key") || !strings.Contains(string(response.Body), fmt.Sprintf("API returned %d:", status)) {
				t.Fatalf("response=%s err=%v redirected=%v", response.Body, err, redirected)
			}
		})
	}
}

func TestAccountPromptCancellationStopsUpstream(t *testing.T) {
	started, cancelled := make(chan struct{}), make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
		close(cancelled)
	}))
	defer upstream.Close()
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"accounts": []promptAccount{{Platform: "openai", Type: "apikey", Credentials: map[string]any{"base_url": upstream.URL, "api_key": "secret"}}}}})
	}))
	defer core.Close()
	a, _ := NewHTTPAdapter(core.URL, core.Client())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := a.AccountPromptRequest(ctx, 42, "model", "question", "jwt"); done <- err }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("upstream not started")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled call succeeded")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("request not cancelled")
	}
	select {
	case <-cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("upstream remained active")
	}
}

func TestAccountPromptProxyAndModelMapping(t *testing.T) {
	key := "configured-proxy"
	a := promptAccount{ProxyKey: &key}
	if _, err := a.promptTransport(nil); err == nil {
		t.Fatal("silently used direct connection")
	}
	transport, err := a.promptTransport([]promptProxy{{Key: key, Protocol: "socks5", Host: "127.0.0.1", Port: 1080, Username: "u", Password: "p"}})
	if err != nil {
		t.Fatal(err)
	}
	defer transport.CloseIdleConnections()
	u, err := transport.Proxy(&http.Request{})
	if err != nil || u.String() != "socks5://u:p@127.0.0.1:1080" {
		t.Fatal("wrong proxy")
	}
	mapping := map[string]any{"*": "fallback", "gpt-*": "gpt", "gpt-6*": "six", "gpt-6-astra": "exact"}
	for input, want := range map[string]string{"other": "fallback", "gpt-5": "gpt", "gpt-6-sol": "six", "gpt-6-astra": "exact"} {
		if got := mapPromptModel(input, mapping); got != want {
			t.Fatalf("%s -> %s", input, got)
		}
	}
}

func TestReadPromptAnswer(t *testing.T) {
	for _, tc := range []struct {
		name, body, text string
		chat, success    bool
	}{
		{"responses", "data: {\"type\":\"response.output_text.delta\",\"delta\":\"答案\"}\r\n\r\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}", "答案", false, true},
		{"final only", "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"output\":[{\"content\":[{\"type\":\"output_text\",\"text\":\"answer\"}]}]}}\n\n", "answer", false, true},
		{"partial", "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\n", "partial", false, false},
		{"failure", "data: {\"type\":\"response.failed\",\"error\":{\"message\":\"secret\"}}\n\n", "", false, false},
		{"malformed", "data: nope\n\n", "", false, false},
		{"empty", "data: {\"type\":\"response.completed\"}\n\n", "", false, false},
		{"chat truncated", "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"},\"finish_reason\":\"length\"}]}\n\n", "partial", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text, message := readPromptAnswer(strings.NewReader(tc.body), tc.chat)
			if text != tc.text || (message == "") != tc.success || strings.Contains(message, "secret") {
				t.Fatalf("text=%q error=%q", text, message)
			}
		})
	}
	_, message := readPromptAnswer(strings.NewReader(strings.Repeat(": keepalive\n\n", accountPromptLimit)), false)
	if !strings.Contains(message, "过大") {
		t.Fatal(message)
	}
}
