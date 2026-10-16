package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

type exclusiveRelay interface {
	RelayRoundTrip(*http.Request) (*http.Response, error)
}
type exclusiveRoundTripper func(*http.Request) (*http.Response, error)

func (f exclusiveRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func exclusiveBearer(header string) string {
	parts := strings.Fields(header)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	return ""
}
func exclusiveManagedCredential(r *http.Request) bool {
	values := append([]string{}, r.URL.Query()["key"]...)
	values = append(values, r.URL.Query()["api_key"]...)
	for _, name := range []string{"Authorization", "x-api-key", "x-goog-api-key"} {
		for _, value := range r.Header.Values(name) {
			if name == "Authorization" {
				value = exclusiveBearer(value)
			}
			values = append(values, value)
		}
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, exclusiveUpstreamPrefix) || strings.HasPrefix(value, exclusiveTokenPrefix) {
			return true
		}
	}
	return false
}
func validExclusiveBody(body []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var walk func() bool
	walk = func() bool {
		token, err := dec.Token()
		if err != nil {
			return false
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			return true
		}
		if delimiter == '{' {
			seen := map[string]bool{}
			for dec.More() {
				key, err := dec.Token()
				name, ok := key.(string)
				if err != nil || !ok || seen[name] {
					return false
				}
				seen[name] = true
				if !walk() {
					return false
				}
			}
			end, err := dec.Token()
			return err == nil && end == json.Delim('}')
		}
		if delimiter == '[' {
			for dec.More() {
				if !walk() {
					return false
				}
			}
			end, err := dec.Token()
			return err == nil && end == json.Delim(']')
		}
		return false
	}
	if !walk() {
		return false
	}
	if _, err := dec.Token(); err != io.EOF {
		return false
	}
	var data map[string]json.RawMessage
	if json.Unmarshal(body, &data) != nil || data == nil {
		return false
	}
	if background, ok := data["background"]; ok && string(bytes.TrimSpace(background)) != "false" {
		return false
	}
	return true
}
func (s *Server) exclusiveGateway(w http.ResponseWriter, r *http.Request) {
	if !s.exclusiveAvailable(w) {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/exclusive")
	inference := r.Method == "POST" && (path == "/v1/responses" || path == "/v1/chat/completions" || path == "/v1/messages")
	if (!inference && !(r.Method == "GET" && path == "/v1/models")) || r.Header.Get("Upgrade") != "" || r.URL.RawQuery != "" {
		writeAPIError(w, 400, "EXCLUSIVE_PROTOCOL_UNSUPPORTED", "独享入口支持 HTTP/SSE Responses、Chat Completions、Messages 和模型列表；不支持后台任务或 WebSocket")
		return
	}
	token := exclusiveBearer(r.Header.Get("Authorization"))
	if !strings.HasPrefix(token, exclusiveTokenPrefix) || len(token) != len(exclusiveTokenPrefix)+64 {
		writeAPIError(w, 401, "INVALID_CHANNEL_KEY", "通道凭据无效")
		return
	}
	var body []byte
	var err error
	if inference {
		body, err = io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<20))
		if err != nil {
			writeAPIError(w, 413, "REQUEST_TOO_LARGE", "请求体过大")
			return
		}
		if !validExclusiveBody(body) {
			writeAPIError(w, 400, "INVALID_REQUEST", "请求必须为有效 JSON，且不支持重复字段或后台任务")
			return
		}
	}
	relay, ok := s.core.(exclusiveRelay)
	if !ok {
		writeAPIError(w, 503, "RELAY_UNAVAILABLE", "Core 转发适配器不可用")
		return
	}
	l, err := scanExclusiveLease(s.portalDB.QueryRowContext(r.Context(), `SELECT `+exclusiveLeaseColumns+` FROM portal_exclusive_leases WHERE token_hash=$1`, exclusiveHash(token)))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, 401, "INVALID_CHANNEL_KEY", "通道凭据无效")
		} else {
			s.exclusiveError(w, err)
		}
		return
	}
	upstream, err := s.openExclusive(l.UpstreamSecret)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	requestID, err := exclusiveRandom("")
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	tx, err := s.portalDB.BeginTx(r.Context(), nil)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	defer tx.Rollback()
	var channelState string
	if err = tx.QueryRowContext(r.Context(), `SELECT state FROM portal_exclusive_channels WHERE id=$1 FOR UPDATE`, l.ChannelID).Scan(&channelState); err != nil {
		s.exclusiveError(w, err)
		return
	}
	l, err = scanExclusiveLease(tx.QueryRowContext(r.Context(), `SELECT `+exclusiveLeaseColumns+` FROM portal_exclusive_leases WHERE id=$1 FOR UPDATE`, l.ID))
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	now := time.Now()
	if l.State != "active" || channelState != "active" || !now.Before(exclusiveDeadline(l)) {
		writeAPIError(w, 403, "CHANNEL_RELEASED", "通道已到期或正在回收，不再接收新请求")
		return
	}
	if l.BillingCheckedAt == nil || now.Sub(*l.BillingCheckedAt) > 30*time.Second {
		writeAPIError(w, 503, "BILLING_STATE_UNAVAILABLE", "暂时无法确认 Core 消费状态，请稍后重试")
		return
	}
	var count int
	if err = tx.QueryRowContext(r.Context(), `SELECT count(*) FROM portal_exclusive_requests WHERE lease_id=$1 AND state<>'finished'`, l.ID).Scan(&count); err != nil {
		s.exclusiveError(w, err)
		return
	}
	if count >= 3 {
		w.Header().Set("Retry-After", "1")
		writeAPIError(w, 429, "CHANNEL_CONCURRENCY", "通道并发已达上限")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO portal_exclusive_requests(id,lease_id,state) VALUES($1,$2,'running')`, requestID, l.ID); err != nil {
		s.exclusiveError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		s.exclusiveError(w, err)
		return
	}
	// Once admitted, downstream cancellation must not cancel the Core request.
	// Drain its response even after the client disconnects; only a clean EOF closes the ledger row.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	upstreamRequest := &http.Request{Method: r.Method, URL: &url.URL{Path: path}, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body))}
	upstreamRequest = upstreamRequest.WithContext(context.WithoutCancel(r.Context()))
	for _, name := range []string{"Content-Type", "Accept", "User-Agent", "OpenAI-Beta", "Anthropic-Version", "Anthropic-Beta", "Session_id", "X-Session-Id"} {
		if value := r.Header.Get(name); value != "" {
			upstreamRequest.Header.Set(name, value)
		}
	}
	upstreamRequest.Header.Set("Authorization", "Bearer "+upstream)
	response, err := relay.RelayRoundTrip(upstreamRequest)
	if err != nil {
		s.finishExclusiveRequest(requestID, 0, false)
		writeAPIError(w, 502, "UPSTREAM_UNCERTAIN", "Core 请求状态不确定，通道将保持锁定等待核查")
		return
	}
	defer response.Body.Close()
	for _, name := range []string{"Content-Type", "X-Request-Id", "Retry-After", "OpenAI-Processing-Ms"} {
		if v := response.Header.Get(name); v != "" {
			w.Header().Set(name, v)
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(response.StatusCode)
	clean := drainExclusiveResponse(w, response.Body)
	s.finishExclusiveRequest(requestID, response.StatusCode, clean)
}
func drainExclusiveResponse(w http.ResponseWriter, body io.Reader) bool {
	buffer := make([]byte, 32<<10)
	connected := true
	for {
		n, err := body.Read(buffer)
		if n > 0 && connected {
			// Bound a stalled downstream write, not the admitted Core operation.
			_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(30 * time.Second))
			if _, e := w.Write(buffer[:n]); e != nil {
				connected = false
			} else if e = http.NewResponseController(w).Flush(); e != nil && e != http.ErrNotSupported {
				connected = false
			}
		}
		if err != nil {
			return err == io.EOF
		}
	}
}
func (s *Server) finishExclusiveRequest(id string, status int, clean bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	state, message := "finished", ""
	if !clean {
		state = "uncertain"
		message = "Core 连接异常，无法确认旧请求已完成"
	}
	_, err := s.portalDB.ExecContext(ctx, `UPDATE portal_exclusive_requests SET state=$1,ended_at=CURRENT_TIMESTAMP,status_code=$2,error=$3 WHERE id=$4`, state, status, message, id)
	if err != nil {
		s.logger.Error("exclusive request completion could not be persisted", "request_id", id, "error", err)
	}
}

// The public Core host must route inference through this guard and /api/ through
// Portal's existing allowlisted management routes. The Core origin stays private.
func (s *Server) exclusiveCoreIngress(w http.ResponseWriter, r *http.Request) {
	if !s.exclusive.Enabled {
		writeAPIError(w, 503, "INGRESS_DISABLED", "独享入口未启用")
		return
	}
	if exclusiveManagedCredential(r) {
		writeAPIError(w, 403, "EXCLUSIVE_DIRECT_ACCESS_DENIED", "独享凭据只能用于专用通道入口")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/core-ingress")
	if !(strings.HasPrefix(path, "/v1/") || strings.HasPrefix(path, "/v1beta/") || strings.HasPrefix(path, "/openai/v1/") || strings.HasPrefix(path, "/anthropic/v1/")) || strings.Contains(path, "..") || strings.Contains(path, "\\") {
		writeAPIError(w, 404, "NOT_FOUND", "该入口不提供管理接口")
		return
	}
	// Ordinary relay traffic preserves its own credentials and billing; it is not an exclusive lease.
	relay, ok := s.core.(exclusiveRelay)
	if !ok {
		writeAPIError(w, 503, "RELAY_UNAVAILABLE", "转发不可用")
		return
	}
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	proxy := &httputil.ReverseProxy{
		Rewrite: func(p *httputil.ProxyRequest) {
			p.Out.URL = &url.URL{Scheme: "http", Host: "core.internal", Path: path, RawQuery: r.URL.RawQuery}
			for _, h := range []string{"Cookie", "X-Forwarded-For", "Forwarded", "X-Real-IP", "CF-Connecting-IP", "Proxy-Authorization"} {
				p.Out.Header.Del(h)
			}
		},
		Transport: exclusiveRoundTripper(relay.RelayRoundTrip), FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			writeAPIError(w, 502, "CORE_UNAVAILABLE", "Core 暂不可用")
		},
	}
	proxy.ServeHTTP(w, r)
}
