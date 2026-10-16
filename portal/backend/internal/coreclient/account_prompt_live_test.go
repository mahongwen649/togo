package coreclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

// Opt-in deployment smoke test. The operator supplies the existing server admin
// key over stdin (never flags/files/logs). Its normal Core export permissions,
// including step-up denial, still apply. Production uses only the caller JWT.
func TestAccountPromptLive(t *testing.T) {
	if os.Getenv("PORTAL_ACCOUNT_PROMPT_LIVE") != "1" {
		t.Skip("live smoke test is opt-in")
	}
	var config struct{ BaseURL, AdminKey string }
	if err := json.NewDecoder(os.Stdin).Decode(&config); err != nil {
		t.Fatal("missing smoke-test configuration")
	}
	if config.AdminKey == "" {
		t.Fatal("existing administrator key is not configured")
	}
	real, err := NewHTTPAdapter(config.BaseURL, &http.Client{})
	if err != nil {
		t.Fatal("invalid Core origin")
	}
	real = real.WithAdminAPIKey(config.AdminKey)
	fixtureTransport := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/admin/accounts/data" || r.Header.Get("Authorization") != "Bearer smoke-fixture" {
			t.Fatal("unexpected smoke-test route")
		}
		response, err := real.AdminMarketRequest(r.Context(), http.MethodGet, r.URL.RequestURI(), nil, "")
		if err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: response.StatusCode, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(response.Body)))}, nil
	})
	adapter, _ := NewHTTPAdapter("http://smoke-fixture.invalid", &http.Client{Transport: fixtureTransport})
	for _, tc := range []struct {
		name         string
		id           int64
		prompt, want string
	}{
		{"shared-746", 746, "只回答下面算式的数字结果，不要解释：17 + 26 = ?", "43"},
		{"shared-747", 747, "只回答下面算式的数字结果，不要解释：17 + 26 = ?", "43"},
		{"individual-746", 746, "只回答下面算式的数字结果，不要解释：21 * 4 = ?", "84"},
		{"individual-747", 747, "只回答下面算式的数字结果，不要解释：9 * 8 = ?", "72"},
		{"html-svg-735", 735, "生成一个简短的完整 HTML，使用内联 SVG 绘制鹈鹕骑自行车的 2D 动画。不用进行测试，只输出 HTML。", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response, err := adapter.AccountPromptRequest(context.Background(), tc.id, "gpt-6-astra", tc.prompt, "smoke-fixture")
			if err != nil {
				t.Fatal(err)
			}
			text, complete := "", false
			for _, line := range strings.Split(string(response.Body), "\n") {
				if !strings.HasPrefix(line, "data:") {
					continue
				}
				var event struct {
					Type, Text, Error string
					Success           bool
				}
				if json.Unmarshal([]byte(strings.TrimSpace(line[5:])), &event) != nil {
					t.Fatal("invalid result")
				}
				if event.Error != "" {
					t.Fatal(event.Error)
				}
				if event.Type == "content" {
					text += event.Text
				}
				if event.Type == "test_complete" {
					complete = event.Success
				}
			}
			if !complete {
				t.Fatal("missing completion")
			}
			if tc.want != "" {
				if strings.Trim(text, "`* \r\n。.") != tc.want {
					t.Fatal("answer did not match the arithmetic question")
				}
				t.Logf("account %d answered %s", tc.id, tc.want)
			} else {
				if !strings.Contains(strings.ToLower(text), "<svg") || !strings.Contains(strings.ToLower(text), "<html") {
					t.Fatal("answer is not HTML/SVG")
				}
				t.Logf("account %d generated HTML/SVG (%d bytes)", tc.id, len(text))
			}
		})
	}
}
