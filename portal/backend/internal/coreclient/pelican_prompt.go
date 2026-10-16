package coreclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// PelicanAccountPromptRequest is a server-owned scheduled probe. Unlike the
// interactive AccountPromptRequest it uses the configured administrator key,
// never a fallback from a rejected user request. Core data is read-only.
func (a *HTTPAdapter) PelicanAccountPromptRequest(ctx context.Context, id int64, model, prompt, requestID string, efforts ...string) (Response, error) {
	if id <= 0 || requestID == "" {
		return Response{}, errors.New("无效的绘图检查账号")
	}
	response, err := a.AdminMarketRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v1/admin/accounts/data?ids=%d&include_proxies=true", id), nil, "")
	if err != nil || response.StatusCode != http.StatusOK {
		return Response{}, errors.New("Core 拒绝读取绘图检查账号配置")
	}
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Accounts []promptAccount `json:"accounts"`
			Proxies  []promptProxy   `json:"proxies"`
		} `json:"data"`
	}
	if json.Unmarshal(response.Body, &envelope) != nil || envelope.Code != 0 || len(envelope.Data.Accounts) != 1 {
		return Response{}, errors.New("Core 未返回唯一绘图账号配置")
	}
	account := envelope.Data.Accounts[0]
	request, actualModel, chat, err := account.promptRequest(ctx, model, prompt)
	if err != nil {
		return Response{}, err
	}
	// Keep the same drawing effort while avoiding affinity to a previous probe.
	var payload map[string]any
	if err = json.NewDecoder(request.Body).Decode(&payload); err != nil {
		return Response{}, errors.New("无法构造绘图请求")
	}
	request.Body.Close()
	effort := "high"
	if len(efforts) > 0 {
		effort = efforts[0]
	}
	if chat {
		payload["reasoning_effort"] = effort
	} else {
		payload["reasoning"] = map[string]string{"effort": effort}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, errors.New("无法构造绘图请求")
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	request.ContentLength = int64(len(body))
	request.Header.Set("Session-Id", requestID)
	request.Header.Set("X-Client-Request-ID", requestID)
	transport, err := account.promptTransport(envelope.Data.Proxies)
	if err != nil {
		return Response{}, err
	}
	defer transport.CloseIdleConnections()
	// The caller owns the entire attempt deadline, including Core lookups and
	// streaming; do not reintroduce the interactive adapter's five-minute cap.
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	upstream, err := client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return Response{}, ctx.Err()
		}
		return Response{}, errors.New("绘图账号上游连接失败")
	}
	defer upstream.Body.Close()
	if upstream.StatusCode < 200 || upstream.StatusCode >= 300 {
		return promptEvents(actualModel, "", fmt.Sprintf("绘图账号返回 HTTP %d", upstream.StatusCode)), nil
	}
	text, message := readPromptAnswerMode(upstream.Body, chat, true)
	if ctx.Err() != nil {
		return Response{}, ctx.Err()
	}
	for _, key := range []string{"api_key", "access_token", "refresh_token", "id_token"} {
		if secret := account.credential(key); secret != "" {
			text = strings.ReplaceAll(text, secret, "[redacted]")
		}
	}
	return promptEvents(actualModel, text, message), nil
}
