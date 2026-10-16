package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

type checkAccount struct {
	ID       int64        `json:"id"`
	Name     string       `json:"name"`
	Platform string       `json:"platform"`
	GroupIDs []int64      `json:"group_ids"`
	Groups   []checkGroup `json:"groups"`
}

type checkGroup struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type accountCheckCore interface {
	AdminUserMarketRequest(context.Context, string, string, any, string) (coreclient.Response, error)
	AdminUserMarketRawRequest(context.Context, string, string, any, string) (coreclient.Response, error)
}

func (s *Server) registerAccountChecks(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/admin/account-checks/selection", s.accountCheckSelection)
	mux.HandleFunc("PUT /api/v1/admin/account-checks/selection", s.accountCheckSelection)
	mux.HandleFunc("GET /api/v1/admin/account-checks/history", s.accountCheckHistory)
	mux.HandleFunc("GET /api/v1/admin/account-checks/accounts/{accountID}/history", s.accountCheckHistory)
	mux.HandleFunc("GET /api/v1/admin/account-checks/accounts", s.accountCheckAccounts)
	mux.HandleFunc("GET /api/v1/admin/account-checks/accounts/{accountID}/models", s.accountCheckModels)
	mux.HandleFunc("PUT /api/v1/admin/account-checks/accounts/{accountID}/groups", s.accountCheckUpdateGroups)
	mux.HandleFunc("POST /api/v1/admin/account-checks/accounts/{accountID}/test", s.accountCheckRun)
	mux.HandleFunc("DELETE /api/v1/admin/account-checks/accounts/{accountID}/test", s.accountCheckCancel)
	mux.HandleFunc("GET /api/v1/admin/account-checks/accounts/{accountID}/scheduled-test-plans", s.accountCheckScheduledPlans)
	mux.HandleFunc("POST /api/v1/admin/account-checks/accounts/{accountID}/scheduled-test-plans", s.accountCheckScheduledPlans)
	mux.HandleFunc("PUT /api/v1/admin/account-checks/accounts/{accountID}/scheduled-test-plans/{planID}", s.accountCheckScheduledPlan)
	mux.HandleFunc("DELETE /api/v1/admin/account-checks/accounts/{accountID}/scheduled-test-plans/{planID}", s.accountCheckScheduledPlan)
	mux.HandleFunc("GET /api/v1/admin/account-checks/accounts/{accountID}/scheduled-test-plans/{planID}/results", s.accountCheckScheduledResults)
}

type accountCheckRunKey struct {
	AdminID, AccountID, RunID int64
}

func (s *Server) trackAccountCheck(key accountCheckRunKey, cancel context.CancelFunc) {
	s.accountChecksMu.Lock()
	defer s.accountChecksMu.Unlock()
	if s.accountChecks == nil {
		s.accountChecks = make(map[accountCheckRunKey]context.CancelFunc)
	}
	s.accountChecks[key] = cancel
}

func (s *Server) untrackAccountCheck(key accountCheckRunKey) {
	s.accountChecksMu.Lock()
	defer s.accountChecksMu.Unlock()
	delete(s.accountChecks, key)
}

func (s *Server) cancelTrackedAccountCheck(key accountCheckRunKey) {
	s.accountChecksMu.Lock()
	cancel := s.accountChecks[key]
	s.accountChecksMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Server) accountCheckAccounts(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireActiveAdmin(w, r); !ok {
		return
	}
	core, accessToken, ok := s.accountCheckCore(w, r)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	if len(search) > 200 {
		writeAPIError(w, 400, "INVALID_SEARCH", "搜索条件过长")
		return
	}
	// Decode only display fields. The full Core list includes bound group names.
	query := url.Values{"page": {strconv.Itoa(page)}, "page_size": {"50"}, "search": {search}, "platform": {"openai"}}
	var result struct {
		Items []checkAccount `json:"items"`
		Total int64          `json:"total"`
		Pages int            `json:"pages"`
		Page  int            `json:"page"`
	}
	if err := accountCheckRequest(r.Context(), core, http.MethodGet, "/api/v1/admin/accounts?"+query.Encode(), nil, &result, accessToken); err != nil {
		s.writeRentalError(w, err)
		return
	}
	items := make([]checkAccount, 0, len(result.Items))
	for _, account := range result.Items {
		if account.Platform == "openai" {
			items = append(items, account)
		}
	}
	result.Items = items
	w.Header().Set("Cache-Control", "no-store")
	writeSuccess(w, result)
}

func (s *Server) accountCheckModels(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireActiveAdmin(w, r); !ok {
		return
	}
	id, ok := pathID(w, r, "accountID")
	if !ok {
		return
	}
	core, accessToken, ok := s.accountCheckCore(w, r)
	if !ok {
		return
	}
	if !s.requireOpenAICheckAccount(w, r, core, accessToken, id) {
		return
	}
	var models []struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	}
	if err := accountCheckRequest(r.Context(), core, http.MethodGet, fmt.Sprintf("/api/v1/admin/accounts/%d/models", id), nil, &models, accessToken); err != nil {
		s.writeRentalError(w, err)
		return
	}
	writeSuccess(w, models)
}

// The platform restriction also applies to direct requests, not just the picker.
func (s *Server) requireOpenAICheckAccount(w http.ResponseWriter, r *http.Request, core accountCheckCore, accessToken string, id int64) bool {
	var account checkAccount
	if err := accountCheckRequest(r.Context(), core, http.MethodGet, fmt.Sprintf("/api/v1/admin/accounts/%d", id), nil, &account, accessToken); err != nil {
		s.writeRentalError(w, err)
		return false
	}
	if account.Platform != "openai" {
		writeAPIError(w, http.StatusBadRequest, "ACCOUNT_CHECK_OPENAI_ONLY", "账号降智检查仅支持 OpenAI 账号")
		return false
	}
	return true
}

func (s *Server) accountCheckCore(w http.ResponseWriter, r *http.Request) (accountCheckCore, string, bool) {
	core, ok := s.core.(accountCheckCore)
	if !ok {
		writeAPIError(w, http.StatusServiceUnavailable, "CORE_TEST_UNAVAILABLE", "Core 账号检查接口不可用")
		return nil, "", false
	}
	accessToken, ok := bearerToken(w, r)
	if !ok {
		return nil, "", false
	}
	return core, accessToken, true
}

func accountCheckRequest(ctx context.Context, core accountCheckCore, method, path string, payload, out any, accessToken string) error {
	response, err := core.AdminUserMarketRequest(ctx, method, path, payload, accessToken)
	if err != nil {
		return err
	}
	var envelope struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(response.Body, &envelope); err != nil {
		return err
	}
	if envelope.Code != 0 || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return fmt.Errorf("Core returned an unsuccessful account-check response")
	}
	if out != nil {
		return json.Unmarshal(envelope.Data, out)
	}
	return nil
}

type accountCheckResult struct {
	Text     string `json:"text"`
	HTML     string `json:"html"`
	Model    string `json:"model"`
	Error    string `json:"error,omitempty"`
	Attempts int    `json:"attempts,omitempty"`
}

type accountCheckInput struct {
	ModelID string `json:"model_id"`
	Prompt  string `json:"prompt"`
}

type accountPromptCore interface {
	AccountPromptRequest(context.Context, int64, string, string, string) (coreclient.Response, error)
}

// Retry only an explicit transient service failure before any output. A 1008
// policy closure or a partial generation must not be blindly replayed.
func retryableAccountCheck(result accountCheckResult) bool {
	if result.Text != "" || result.HTML != "" {
		return false
	}
	message := strings.ToLower(result.Error)
	if strings.Contains(message, "1008") {
		return false
	}
	return strings.Contains(message, "api returned 502:") || strings.Contains(message, "api returned 503:") || strings.Contains(message, "api returned 504:")
}

func performAccountCheck(ctx context.Context, core accountCheckCore, id int64, input accountCheckInput, token string, delay time.Duration) accountCheckResult {
	client, ok := core.(accountPromptCore)
	if !ok {
		return accountCheckResult{Error: "账号自定义题目检查接口不可用，请更新 Portal API"}
	}
	for attempt := 1; ; attempt++ {
		response, err := client.AccountPromptRequest(ctx, id, input.ModelID, input.Prompt, token)
		result := accountCheckResult{Error: "Core 账号检查请求失败，请检查账号状态或稍后重试"}
		if err != nil {
			// AccountPromptRequest returns only sanitized operator-facing errors.
			result.Error = err.Error()
		}
		if err == nil && response.StatusCode >= 200 && response.StatusCode < 300 {
			result = parseAccountCheck(response.Body)
		}
		result.Attempts = attempt
		if attempt >= 2 || !retryableAccountCheck(result) || ctx.Err() != nil {
			return result
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return result
		case <-timer.C:
		}
	}
}

func parseAccountCheck(body []byte) accountCheckResult {
	result := accountCheckResult{}
	if len(body) > exclusivePageLimit {
		result.Error = "检查响应过大，请缩短题目后重试"
		return result
	}
	var output strings.Builder
	complete := false
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 1024), exclusivePageLimit+1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			continue
		}
		var event struct {
			Type    string `json:"type"`
			Text    string `json:"text"`
			Model   string `json:"model"`
			Error   string `json:"error"`
			Success bool   `json:"success"`
		}
		if json.Unmarshal([]byte(data), &event) != nil {
			result.Error = "检查响应格式无效"
			continue
		}
		switch event.Type {
		case "test_start":
			result.Model = event.Model
		case "content":
			output.WriteString(event.Text)
		case "test_complete":
			complete = event.Success
			if !event.Success {
				result.Error = event.Error
				if result.Error == "" {
					result.Error = "账号检查失败"
				}
			}
		case "error":
			result.Error = event.Error
			if result.Error == "" {
				result.Error = "账号检查失败"
			}
		}
	}
	result.Text = output.String()
	if scanner.Err() != nil || !complete {
		if result.Error == "" {
			result.Error = "检查响应中断，未收到完成标记，请重新检查"
		}
	}
	if result.Error == "" && strings.TrimSpace(result.Text) == "" {
		result.Error = "模型未返回文案、HTML 或 SVG 内容"
	}
	// Partial output is kept as text, but unfinished scripts are never executed.
	if result.Error == "" {
		for _, candidate := range exclusivePageCandidates(result.Text) {
			if html, ok := extractExclusiveDocument(candidate); ok {
				result.HTML = html
				break
			}
		}
	}
	return result
}

func (s *Server) accountCheckRun(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireActiveAdmin(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r, "accountID")
	if !ok {
		return
	}
	core, accessToken, ok := s.accountCheckCore(w, r)
	if !ok {
		return
	}
	var input accountCheckInput
	r.Body = http.MaxBytesReader(w, r.Body, 128*1024)
	if decodeJSON(r, &input) != nil || strings.TrimSpace(input.Prompt) == "" || len(input.Prompt) > 64000 || len(input.ModelID) > 256 {
		writeAPIError(w, 400, "INVALID_ACCOUNT_CHECK", "请输入检查题目（最多 64000 字节），并使用有效的模型名称")
		return
	}
	var account checkAccount
	if err := accountCheckRequest(r.Context(), core, http.MethodGet, fmt.Sprintf("/api/v1/admin/accounts/%d", id), nil, &account, accessToken); err != nil {
		s.writeRentalError(w, err)
		return
	}
	if account.Platform != "openai" {
		writeAPIError(w, 400, "ACCOUNT_CHECK_OPENAI_ONLY", "账号降智检查仅支持 OpenAI 账号")
		return
	}
	account.ID = id
	if s.portalDB == nil {
		writeAPIError(w, 503, "HISTORY_UNAVAILABLE", "检查记录存储不可用")
		return
	}
	if err := expireAccountChecks(r.Context(), s.portalDB, admin.ID); err != nil {
		s.writeAccountHistoryError(w, err)
		return
	}
	runID, err := startAccountCheck(r.Context(), s.portalDB, admin.ID, account, input.Prompt, input.ModelID)
	if errors.Is(err, errAccountCheckBusy) {
		writeAPIError(w, 409, "ACCOUNT_CHECK_RUNNING", "该账号正在检查，请等待完成")
		return
	}
	if err != nil {
		s.writeAccountHistoryError(w, err)
		return
	}
	// Accepted checks finish and persist even when a browser refresh closes the SSE connection.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Minute)
	defer cancel()
	key := accountCheckRunKey{AdminID: admin.ID, AccountID: id, RunID: runID}
	s.trackAccountCheck(key, cancel)
	defer s.untrackAccountCheck(key)
	// Keep proxy connections alive while Core buffers its account-test response.
	controller := http.NewResponseController(w)
	_ = controller.SetWriteDeadline(time.Now().Add(6 * time.Minute))
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	_, _ = fmt.Fprint(w, ": checking\n\n")
	_ = controller.Flush()
	finished := make(chan accountCheckResult, 1)
	go func() {
		finished <- performAccountCheck(ctx, core, id, input, accessToken, 2*time.Second)
	}()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	writeResult := func(result accountCheckResult) {
		storeCtx, storeCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer storeCancel()
		if err := finishAccountCheck(storeCtx, s.portalDB, admin.ID, id, runID, result); err != nil {
			s.logger.Error("save account check result failed", "run_id", runID, "error", err)
			result.Error = strings.TrimSpace(result.Error + " 检查结果保存失败，请勿刷新并稍后重试")
		}
		body, _ := json.Marshal(result)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", body)
		_ = controller.Flush()
	}
	for {
		select {
		case result := <-finished:
			writeResult(result)
			return
		case <-ctx.Done():
			writeResult(accountCheckResult{Error: "检查超时或已取消，请重新检查"})
			return
		case <-ticker.C:
			_, _ = fmt.Fprint(w, ": checking\n\n")
			_ = controller.Flush()
		}
	}
}

func (s *Server) accountCheckCancel(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireActiveAdmin(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r, "accountID")
	if !ok {
		return
	}
	if s.portalDB == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "HISTORY_UNAVAILABLE", "检查记录存储不可用")
		return
	}
	runID, cancelled, err := cancelRunningAccountCheck(r.Context(), s.portalDB, admin.ID, id)
	if err != nil {
		s.writeAccountHistoryError(w, err)
		return
	}
	if cancelled {
		s.cancelTrackedAccountCheck(accountCheckRunKey{AdminID: admin.ID, AccountID: id, RunID: runID})
	}
	writeSuccess(w, map[string]any{"cancelled": cancelled, "run_id": runID})
}
