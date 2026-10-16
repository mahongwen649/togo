package coreclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const accountPromptLimit = 2 << 20

var promptAPIVersion = regexp.MustCompile(`(?i)^v[0-9]+(?:\.[0-9]+|(?:alpha|beta|preview)[a-z0-9.-]*)?$`)

type promptAccount struct {
	Platform    string         `json:"platform"`
	Type        string         `json:"type"`
	Credentials map[string]any `json:"credentials"`
	Extra       map[string]any `json:"extra"`
	ProxyKey    *string        `json:"proxy_key"`
}

type promptProxy struct {
	Key      string `json:"proxy_key"`
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// AccountPromptRequest deliberately does not use Core's connectivity test:
// official Core releases can accept prompt but send a hard-coded "hi".
// Fetch only the selected account through the operator's existing permissions.
// Secrets remain in this call and are never returned, persisted or logged.
func (a *HTTPAdapter) AccountPromptRequest(ctx context.Context, id int64, model, prompt, token string) (Response, error) {
	if id <= 0 || strings.TrimSpace(prompt) == "" || len(prompt) > 64000 || len(model) > 256 {
		return Response{}, errors.New("检查题目或账号无效")
	}
	path := fmt.Sprintf("/api/v1/admin/accounts/data?ids=%d&include_proxies=true", id)
	response, err := a.AdminUserMarketRequest(ctx, http.MethodGet, path, nil, token)
	if err != nil {
		// Do not use a server key to bypass a JWT/step-up rejection.
		if response.StatusCode == http.StatusForbidden {
			return Response{}, errors.New("Core 拒绝读取检查配置，请确认管理员权限并完成 Core 二次验证后重试")
		}
		return Response{}, errors.New("无法读取账号检查配置，请确认管理员登录状态和 Core 连接")
	}
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Accounts []promptAccount `json:"accounts"`
			Proxies  []promptProxy   `json:"proxies"`
		} `json:"data"`
	}
	if json.Unmarshal(response.Body, &envelope) != nil || envelope.Code != 0 || len(envelope.Data.Accounts) != 1 {
		return Response{}, errors.New("Core 未返回唯一账号配置；影子账号暂不支持此检查")
	}
	account := envelope.Data.Accounts[0]
	request, actualModel, chat, err := account.promptRequest(ctx, model, prompt)
	if err != nil {
		return Response{}, err
	}
	transport, err := account.promptTransport(envelope.Data.Proxies)
	if err != nil {
		return Response{}, err
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: adminMarketRawTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	upstream, err := client.Do(request)
	if err != nil {
		// net/url errors contain the upstream/proxy URL, which can contain secrets.
		return Response{}, errors.New("账号上游连接失败、超时或已取消")
	}
	defer upstream.Body.Close()
	if upstream.StatusCode < 200 || upstream.StatusCode >= 300 {
		return promptEvents(actualModel, "", fmt.Sprintf("API returned %d: 账号上游请求失败", upstream.StatusCode)), nil
	}
	text, message := readPromptAnswer(upstream.Body, chat)
	// Provider diagnostics occasionally echo credentials. Never persist those.
	for _, key := range []string{"api_key", "access_token", "refresh_token", "id_token"} {
		if secret, _ := account.Credentials[key].(string); secret != "" {
			text = strings.ReplaceAll(text, secret, "[redacted]")
		}
	}
	return promptEvents(actualModel, text, message), nil
}

func (a promptAccount) credential(key string) string {
	value, _ := a.Credentials[key].(string)
	return strings.TrimSpace(value)
}

func (a promptAccount) promptRequest(ctx context.Context, model, prompt string) (*http.Request, string, bool, error) {
	if a.Platform != "openai" || (a.Type != "apikey" && a.Type != "oauth") {
		return nil, "", false, errors.New("该账号类型暂不支持自定义题目检查")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = "gpt-6-astra"
	}
	model = mapPromptModel(model, a.Credentials["model_mapping"])
	chat := false
	endpoint, secret := "https://chatgpt.com/backend-api/codex/responses", a.credential("access_token")
	if a.Type == "apikey" {
		secret = a.credential("api_key")
		mode, _ := a.Extra["openai_responses_mode"].(string)
		supported, known := a.Extra["openai_responses_supported"].(bool)
		chat = mode == "force_chat_completions" || (mode != "force_responses" && known && !supported)
		base := strings.TrimRight(a.credential("base_url"), "/")
		if base == "" {
			base = "https://api.openai.com"
		}
		u, err := url.Parse(base)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return nil, "", false, errors.New("账号上游地址无效")
		}
		suffix := "/responses"
		if chat {
			suffix = "/chat/completions"
		}
		path := strings.TrimRight(u.Path, "/")
		if !strings.HasSuffix(path, suffix) {
			last := path[strings.LastIndex(path, "/")+1:]
			if !promptAPIVersion.MatchString(last) {
				path += "/v1"
			}
			path += suffix
		}
		u.Path, u.RawPath = path, ""
		endpoint = u.String()
	}
	if secret == "" {
		return nil, "", false, errors.New("账号缺少可用凭据，特殊身份账号暂不支持此检查")
	}
	payload := map[string]any{"model": model, "stream": true}
	if chat {
		payload["messages"] = []any{map[string]any{"role": "user", "content": prompt}}
	} else {
		payload["input"] = []any{map[string]any{"role": "user", "content": []any{map[string]any{"type": "input_text", "text": prompt}}}}
		payload["instructions"] = "Answer the user's question and follow their instructions."
		if a.Type == "oauth" {
			payload["store"] = false
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, "", false, errors.New("无法构造检查请求")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, "", false, errors.New("账号上游地址无效")
	}
	request.Header.Set("Authorization", "Bearer "+secret)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	if a.Type == "oauth" {
		request.Header.Set("OpenAI-Beta", "responses=experimental")
		request.Header.Set("Originator", "codex_cli_rs")
		ua := a.credential("user_agent")
		if ua == "" {
			ua = "codex_cli_rs/0.146.0 (Ubuntu 22.4.0; x86_64) xterm-256color"
		}
		request.Header.Set("User-Agent", ua)
		if org := a.credential("chatgpt_account_id"); org != "" {
			request.Header.Set("ChatGPT-Account-ID", org)
		}
	}
	return request, model, chat, nil
}

func mapPromptModel(model string, raw any) string {
	mapping, _ := raw.(map[string]any)
	if exact, ok := mapping[model].(string); ok && exact != "" {
		return exact
	}
	patterns := make([]string, 0, len(mapping))
	for key := range mapping {
		if strings.HasSuffix(key, "*") {
			patterns = append(patterns, key)
		}
	}
	sort.Slice(patterns, func(i, j int) bool {
		if len(patterns[i]) == len(patterns[j]) {
			return patterns[i] < patterns[j]
		}
		return len(patterns[i]) > len(patterns[j])
	})
	for _, pattern := range patterns {
		if target, ok := mapping[pattern].(string); ok && target != "" && strings.HasPrefix(model, strings.TrimSuffix(pattern, "*")) {
			return target
		}
	}
	return model
}

func (a promptAccount) promptTransport(proxies []promptProxy) (*http.Transport, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil // Only use the account's explicitly configured proxy.
	transport.ResponseHeaderTimeout = 2 * time.Minute
	if a.ProxyKey == nil || *a.ProxyKey == "" {
		return transport, nil
	}
	for _, proxy := range proxies {
		if proxy.Key != *a.ProxyKey {
			continue
		}
		if proxy.Host == "" || proxy.Port < 1 || proxy.Port > 65535 {
			return nil, errors.New("账号代理配置无效")
		}
		switch proxy.Protocol {
		case "http", "https", "socks5", "socks5h":
		default:
			return nil, errors.New("账号代理协议不受支持")
		}
		u := &url.URL{Scheme: proxy.Protocol, Host: net.JoinHostPort(proxy.Host, strconv.Itoa(proxy.Port))}
		if proxy.Username != "" {
			u.User = url.UserPassword(proxy.Username, proxy.Password)
		}
		transport.Proxy = http.ProxyURL(u)
		return transport, nil
	}
	return nil, errors.New("账号代理配置缺失，未发起直连")
}

func promptEvents(model, text, message string) Response {
	var body bytes.Buffer
	write := func(event any) {
		data, _ := json.Marshal(event)
		fmt.Fprintf(&body, "data: %s\n\n", data)
	}
	write(map[string]any{"type": "test_start", "model": model})
	if text != "" {
		write(map[string]any{"type": "content", "text": text})
	}
	write(map[string]any{"type": "test_complete", "success": message == "", "error": message})
	return Response{StatusCode: http.StatusOK, ContentType: "text/event-stream", Body: body.Bytes()}
}

type promptResponse struct {
	Status string `json:"status"`
	Output []struct {
		Content []struct{ Type, Text string } `json:"content"`
	} `json:"output"`
}

func (r promptResponse) text() string {
	var text strings.Builder
	for _, output := range r.Output {
		for _, content := range output.Content {
			if content.Type == "output_text" {
				text.WriteString(content.Text)
			}
		}
	}
	return text.String()
}

func readPromptAnswer(reader io.Reader, chat bool) (string, string) {
	return readPromptAnswerMode(reader, chat, false)
}

func readPromptAnswerMode(reader io.Reader, chat, stopAtTerminal bool) (string, string) {
	// Bound reasoning/metadata as well as displayed content.
	limited := &io.LimitedReader{R: reader, N: accountPromptLimit*3 + 1}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 4096), accountPromptLimit*3+1)
	var text strings.Builder
	complete, failed := false, false
	var eventData []string
	consume := func() {
		if len(eventData) == 0 {
			return
		}
		data := strings.Join(eventData, "\n")
		eventData = nil
		if data == "[DONE]" {
			return
		}
		var event struct {
			Type     string          `json:"type"`
			Delta    string          `json:"delta"`
			Response promptResponse  `json:"response"`
			Error    json.RawMessage `json:"error"`
			Choices  []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(data), &event) != nil {
			failed = true
			return
		}
		if len(event.Error) > 0 && string(event.Error) != "null" {
			failed = true
		}
		if chat {
			for _, choice := range event.Choices {
				text.WriteString(choice.Delta.Content)
				if choice.FinishReason != nil {
					if *choice.FinishReason == "stop" {
						complete = true
					} else {
						failed = true
					}
				}
			}
		} else {
			switch event.Type {
			case "response.output_text.delta":
				text.WriteString(event.Delta)
			case "response.completed":
				complete = event.Response.Status == "" || event.Response.Status == "completed"
				if text.Len() == 0 {
					text.WriteString(event.Response.text())
				}
			case "error", "response.failed", "response.incomplete":
				failed = true
			}
		}
	}
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			consume()
			if stopAtTerminal && (failed || complete) {
				break
			}
		} else if strings.HasPrefix(line, "data:") {
			eventData = append(eventData, strings.TrimSpace(line[5:]))
		}
		if text.Len() > accountPromptLimit {
			return "", "检查响应过大，请缩短题目后重试"
		}
	}
	consume()
	if text.Len() > accountPromptLimit || limited.N <= 0 {
		return "", "检查响应过大，请缩短题目后重试"
	}
	if scanner.Err() != nil || failed || !complete {
		return text.String(), "账号上游响应失败或中断，未收到完整答案"
	}
	if strings.TrimSpace(text.String()) == "" {
		return "", "模型未返回文案、HTML 或 SVG 内容"
	}
	return text.String(), ""
}
