package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

type marketCoreRaw interface {
	AdminMarketRawRequest(context.Context, string, string, any, string) (coreclient.Response, error)
}

type marketCoreRequest interface {
	AdminMarketRequest(context.Context, string, string, any, string) (coreclient.Response, error)
}

// customerAccount deliberately contains only fields that are useful to the
// customer. Credentials are limited to non-sensitive display metadata by
// sanitizeCustomerCredentials; groups and proxy configuration are never
// decoded or returned by Portal.
type customerAccount struct {
	ID                      int64          `json:"id"`
	Name                    string         `json:"name"`
	Notes                   *string        `json:"notes"`
	Platform                string         `json:"platform"`
	Type                    string         `json:"type"`
	Credentials             map[string]any `json:"credentials,omitempty"`
	Status                  string         `json:"status"`
	ErrorMessage            string         `json:"error_message"`
	Schedulable             bool           `json:"schedulable"`
	RateLimitedAt           *string        `json:"rate_limited_at"`
	RateLimitResetAt        *string        `json:"rate_limit_reset_at"`
	OverloadUntil           *string        `json:"overload_until"`
	TempUnschedulableUntil  *string        `json:"temp_unschedulable_until"`
	TempUnschedulableReason string         `json:"temp_unschedulable_reason"`
	LastUsedAt              *string        `json:"last_used_at"`
	ExpiresAt               *int64         `json:"expires_at"`
	AutoPauseOnExpired      bool           `json:"auto_pause_on_expired"`
	SessionWindowStart      *string        `json:"session_window_start"`
	SessionWindowEnd        *string        `json:"session_window_end"`
	SessionWindowStatus     string         `json:"session_window_status"`
	QuotaLimit              *float64       `json:"quota_limit"`
	QuotaUsed               *float64       `json:"quota_used"`
	QuotaDailyLimit         *float64       `json:"quota_daily_limit"`
	QuotaDailyUsed          *float64       `json:"quota_daily_used"`
	QuotaWeeklyLimit        *float64       `json:"quota_weekly_limit"`
	QuotaWeeklyUsed         *float64       `json:"quota_weekly_used"`
	CurrentWindowCost       *float64       `json:"current_window_cost"`
	ActiveSessions          *int           `json:"active_sessions"`
	CurrentRPM              *float64       `json:"current_rpm"`
	OllamaCloudUsage        map[string]any `json:"ollama_cloud_usage"`
	ParentAccountID         *int64         `json:"parent_account_id"`
	Extra                   map[string]any `json:"extra"`
	Usage                   map[string]any `json:"usage"`
	UsageError              string         `json:"usage_error,omitempty"`
}

type customerPackage struct {
	ID           int64             `json:"id"`
	OrderID      int64             `json:"order_id"`
	Name         string            `json:"name"`
	ProductType  string            `json:"product_type"`
	Mode         string            `json:"mode"`
	ValidityDays int               `json:"validity_days"`
	Quantity     int               `json:"quantity"`
	Status       string            `json:"status"`
	StartsAt     *time.Time        `json:"starts_at"`
	ExpiresAt    *time.Time        `json:"expires_at"`
	AutoRenew    bool              `json:"auto_renew"`
	CoreGroupID  *int64            `json:"core_group_id,omitempty"`
	Accounts     []customerAccount `json:"accounts"`
}

type customerAccountView struct {
	Account customerAccount `json:"account"`
	Usage   map[string]any  `json:"usage"`
}

type customerAccountTestRequest struct {
	ModelID      string `json:"model_id"`
	Prompt       string `json:"prompt"`
	Mode         string `json:"mode"`
	ImageDataURL string `json:"image_data_url"`
	AudioDataURL string `json:"audio_data_url"`
}

func (s *Server) requireCustomerAccount(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	accountID, ok := pathID(w, r, "accountID")
	if !ok {
		return 0, 0, false
	}
	return s.requireCustomerAccountID(w, r, accountID)
}

// requireCustomerAccountID is shared by routes whose account ID is carried in
// the request body (for example scheduled-test-plan creation) and routes that
// carry it in the URL. The Core admin key is never accepted as a customer
// credential: the current Portal user and a currently valid completed order
// must both authorize the account.
func (s *Server) requireCustomerAccountID(w http.ResponseWriter, r *http.Request, accountID int64) (int64, int64, bool) {
	if !s.requireMarketDB(w) {
		return 0, 0, false
	}
	if accountID <= 0 {
		writeAPIError(w, http.StatusBadRequest, "INVALID_ID", "Invalid identifier")
		return 0, 0, false
	}
	accessToken, hasToken := bearerToken(w, r)
	if !hasToken {
		return 0, 0, false
	}
	user, err := s.core.Profile(r.Context(), accessToken)
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return 0, 0, false
	}
	var orderID int64
	err = s.portalDB.QueryRowContext(r.Context(), `
		SELECT id
		FROM portal_account_orders
		WHERE user_id = $1
		  AND status = 'completed'
		  AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
		  AND core_account_ids @> jsonb_build_array($2::bigint)
		ORDER BY expires_at DESC NULLS LAST, id DESC
		LIMIT 1`, user.ID, strconv.FormatInt(accountID, 10)).Scan(&orderID)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, http.StatusNotFound, "ACCOUNT_NOT_FOUND", "该账号不在你的有效套餐中")
		return 0, 0, false
	}
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_ACCESS_CHECK_FAILED", "账号权限校验失败")
		return 0, 0, false
	}
	var account customerAccount
	if err := s.rentalCoreRequest(r.Context(), http.MethodGet, fmt.Sprintf("/api/v1/admin/accounts/%d", accountID), nil, &account, ""); err != nil {
		s.writeRentalError(w, err)
		return 0, 0, false
	}
	if account.ExpiresAt != nil && *account.ExpiresAt <= time.Now().Unix() {
		writeAPIError(w, http.StatusNotFound, "ACCOUNT_EXPIRED", "该 Core 账号已过期")
		return 0, 0, false
	}
	return accountID, orderID, true
}

func (s *Server) accountMarketPackages(w http.ResponseWriter, r *http.Request) {
	if !s.requireMarketDB(w) {
		return
	}
	accessToken, ok := bearerToken(w, r)
	if !ok {
		return
	}
	user, err := s.core.Profile(r.Context(), accessToken)
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return
	}
	rows, err := s.portalDB.QueryContext(r.Context(), `
		SELECT id, product_name, mode, validity_days, quantity, status, fulfillment_started_at, expires_at, core_account_ids, auto_renew, core_group_id
		FROM portal_account_orders
		WHERE user_id = $1 AND status IN ('completed', 'expired')
		ORDER BY created_at DESC, id DESC`, user.ID)
	if err != nil {
		s.writeRentalError(w, err)
		return
	}
	defer rows.Close()

	packages := make([]customerPackage, 0)
	for rows.Next() {
		var p customerPackage
		var startedAt, expiresAt sql.NullTime
		var raw []byte
		var status string
		var coreGroupID sql.NullInt64
		if err := rows.Scan(&p.OrderID, &p.Name, &p.Mode, &p.ValidityDays, &p.Quantity, &status, &startedAt, &expiresAt, &raw, &p.AutoRenew, &coreGroupID); err != nil {
			s.writeRentalError(w, err)
			return
		}
		p.ID = p.OrderID
		if coreGroupID.Valid {
			value := coreGroupID.Int64
			p.CoreGroupID = &value
		}
		p.Status = "expired"
		if expiresAt.Valid && expiresAt.Time.After(time.Now()) && status == "completed" {
			p.Status = "active"
		}
		if startedAt.Valid {
			value := startedAt.Time
			p.StartsAt = &value
		}
		if expiresAt.Valid {
			value := expiresAt.Time
			p.ExpiresAt = &value
		}
		var accountIDs []int64
		if err := json.Unmarshal(raw, &accountIDs); err != nil {
			s.writeRentalError(w, err)
			return
		}
		p.ProductType = p.Name
		p.Accounts = make([]customerAccount, 0, len(accountIDs))
		for _, accountID := range accountIDs {
			var account customerAccount
			if err := s.rentalCoreRequest(r.Context(), http.MethodGet, fmt.Sprintf("/api/v1/admin/accounts/%d", accountID), nil, &account, ""); err != nil {
				continue
			}
			account = sanitizeCustomerAccount(account)
			if p.Status != "active" {
				p.Accounts = append(p.Accounts, account)
				continue
			}
			if account.ExpiresAt != nil && *account.ExpiresAt <= time.Now().Unix() {
				continue
			}
			usage := map[string]any{}
			usageErr := s.rentalCoreRequest(r.Context(), http.MethodGet, fmt.Sprintf("/api/v1/admin/accounts/%d/usage?source=active&force=true", accountID), nil, &usage, "")
			account = customerAccountWithUsage(account, usage)
			if usageErr != nil {
				account.UsageError = "Core 使用数据暂时不可用，请点击主动查询重试"
			}
			p.Accounts = append(p.Accounts, account)
		}
		packages = append(packages, p)
	}
	if err := rows.Err(); err != nil {
		s.writeRentalError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"items": packages})
}

func customerAccountWithUsage(account customerAccount, usage map[string]any) customerAccount {
	account = sanitizeCustomerAccount(account)
	if usage != nil {
		account.Usage = sanitizeCustomerMap(usage)
	}
	return account
}

func sanitizeCustomerAccount(account customerAccount) customerAccount {
	account.Credentials = sanitizeCustomerCredentials(account.Credentials)
	account.Extra = sanitizeCustomerExtra(account.Extra)
	account.OllamaCloudUsage = sanitizeCustomerMap(account.OllamaCloudUsage)
	account.Usage = sanitizeCustomerMap(account.Usage)
	return account
}

// sanitizeCustomerCredentials keeps only the non-secret fields required by
// Core's account usage components to select their platform-specific views.
// It intentionally does not expose Core's credential-status map or any raw
// authentication material.
func sanitizeCustomerCredentials(credentials map[string]any) map[string]any {
	if credentials == nil {
		return nil
	}
	allowed := map[string]bool{
		"account_mode":            true,
		"api_protocol":            true,
		"auth_mode":               true,
		"oauth_type":              true,
		"tier_id":                 true,
		"project_id":              true,
		"plan_type":               true,
		"subscription_expires_at": true,
	}
	clean := make(map[string]any, len(allowed))
	for key, value := range credentials {
		if allowed[strings.ToLower(strings.TrimSpace(key))] {
			clean[key] = value
		}
	}
	if len(clean) == 0 {
		return nil
	}
	return clean
}

func sanitizeCustomerExtra(extra map[string]any) map[string]any {
	if extra == nil {
		return nil
	}
	safeMetadata := map[string]bool{
		// This is a non-secret Core snapshot containing only reset-credit counts
		// and expiry timestamps. Keep it so the customer sees the same cached
		// "次数" state as the Core account page; the key name contains "token"
		// but it never contains an access token.
		"codex_reset_credit_snapshot":   true,
		"codex_auto_reset_credit_state": true,
		"auto_reset_credit_enabled":     true,
	}
	deny := func(key string) bool { return !safeMetadata[strings.ToLower(key)] && isCustomerSensitiveKey(key) }
	var clean func(any) any
	clean = func(value any) any {
		switch v := value.(type) {
		case map[string]any:
			out := make(map[string]any, len(v))
			for key, item := range v {
				if !deny(key) {
					out[key] = clean(item)
				}
			}
			return out
		case []any:
			out := make([]any, len(v))
			for i, item := range v {
				out[i] = clean(item)
			}
			return out
		default:
			return value
		}
	}
	return clean(extra).(map[string]any)
}

func sanitizeCustomerMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	clean, ok := sanitizeCustomerPayload(value).(map[string]any)
	if !ok {
		return nil
	}
	return clean
}

// isCustomerSensitiveKey blocks account secrets and internal routing metadata
// without removing legitimate usage counters such as input_tokens and
// total_tokens from Core's usage responses.
func isCustomerSensitiveKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return false
	}
	for _, part := range []string{"credential", "proxy", "group", "authorization", "cookie", "password", "secret"} {
		if strings.Contains(key, part) {
			return true
		}
	}
	switch key {
	case "token", "access_token", "refresh_token", "id_token", "api_token", "api_access_token", "setup_token", "temp_token":
		return true
	}
	return strings.HasPrefix(key, "token_") || strings.HasSuffix(key, "_token")
}

// sanitizeCustomerPayload protects raw Core action responses as well as the
// normal account DTO. Some Core actions include an updated account object in
// their data payload; returning that raw object would bypass the account
// response sanitizer above and could expose credentials or routing metadata.
func sanitizeCustomerPayload(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			lower := strings.ToLower(strings.TrimSpace(key))
			if lower == "credentials" {
				if credentials, ok := item.(map[string]any); ok {
					if clean := sanitizeCustomerCredentials(credentials); clean != nil {
						out[key] = clean
					}
				}
				continue
			}
			if isCustomerSensitiveKey(key) {
				continue
			}
			if lower == "extra" {
				if extra, ok := item.(map[string]any); ok {
					out[key] = sanitizeCustomerExtra(extra)
					continue
				}
			}
			out[key] = sanitizeCustomerPayload(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = sanitizeCustomerPayload(item)
		}
		return out
	default:
		return value
	}
}

func (s *Server) accountMarketAccount(w http.ResponseWriter, r *http.Request) {
	accountID, _, ok := s.requireCustomerAccount(w, r)
	if !ok {
		return
	}
	var account customerAccount
	if err := s.rentalCoreRequest(r.Context(), http.MethodGet, fmt.Sprintf("/api/v1/admin/accounts/%d", accountID), nil, &account, ""); err != nil {
		s.writeRentalError(w, err)
		return
	}
	usage := map[string]any{}
	usageErr := s.rentalCoreRequest(r.Context(), http.MethodGet, fmt.Sprintf("/api/v1/admin/accounts/%d/usage?source=active&force=true", accountID), nil, &usage, "")
	if usageErr != nil {
		account.UsageError = "Core 使用数据暂时不可用，请点击主动查询重试"
	}
	account = customerAccountWithUsage(account, usage)
	writeSuccess(w, customerAccountView{Account: account, Usage: account.Usage})
}

func (s *Server) accountMarketRecoverAccount(w http.ResponseWriter, r *http.Request) {
	accountID, _, ok := s.requireCustomerAccount(w, r)
	if !ok {
		return
	}
	var account customerAccount
	if err := s.rentalCoreRequest(r.Context(), http.MethodPost, fmt.Sprintf("/api/v1/admin/accounts/%d/recover-state", accountID), nil, &account, ""); err != nil {
		s.writeRentalError(w, err)
		return
	}
	writeSuccess(w, sanitizeCustomerAccount(account))
}

func (s *Server) accountMarketAccountModels(w http.ResponseWriter, r *http.Request) {
	accountID, _, ok := s.requireCustomerAccount(w, r)
	if !ok {
		return
	}
	var models []any
	if err := s.rentalCoreRequest(r.Context(), http.MethodGet, fmt.Sprintf("/api/v1/admin/accounts/%d/models", accountID), nil, &models, ""); err != nil {
		s.writeRentalError(w, err)
		return
	}
	writeSuccess(w, sanitizeCustomerPayload(models))
}

func (s *Server) accountMarketCoreAction(w http.ResponseWriter, r *http.Request) {
	accountID, _, ok := s.requireCustomerAccount(w, r)
	if !ok {
		return
	}
	action := r.PathValue("action")
	paths := map[string]string{
		"account-reset-quota": fmt.Sprintf("/api/v1/admin/accounts/%d/reset-quota", accountID),
		"openai-refresh":      fmt.Sprintf("/api/v1/admin/openai/accounts/%d/quota/refresh", accountID),
		"openai-reset":        fmt.Sprintf("/api/v1/admin/openai/accounts/%d/reset-quota", accountID),
		"grok-quota":          fmt.Sprintf("/api/v1/admin/grok/accounts/%d/quota", accountID),
		"grok-reset":          fmt.Sprintf("/api/v1/admin/grok/accounts/%d/reset-quota", accountID),
		"cn-quota":            fmt.Sprintf("/api/v1/admin/cn-providers/accounts/%d/quota", accountID),
		"cn-balance":          fmt.Sprintf("/api/v1/admin/cn-providers/accounts/%d/balance", accountID),
		"ollama-refresh":      fmt.Sprintf("/api/v1/admin/accounts/%d/ollama-cloud-usage/refresh", accountID),
	}
	path, exists := paths[action]
	if !exists {
		writeAPIError(w, http.StatusNotFound, "CORE_ACTION_NOT_FOUND", "账号操作不存在")
		return
	}
	core, ok := s.core.(marketCoreRequest)
	if !ok {
		writeAPIError(w, http.StatusServiceUnavailable, "CORE_ACTION_UNAVAILABLE", "Core 账号操作接口不可用")
		return
	}
	method := http.MethodGet
	if action == "account-reset-quota" || action == "openai-refresh" || action == "openai-reset" || action == "grok-reset" || action == "ollama-refresh" {
		method = http.MethodPost
	}
	response, err := core.AdminMarketRequest(r.Context(), method, path, nil, "")
	if err != nil {
		s.writeRentalError(w, err)
		return
	}
	var payload any
	if err := json.Unmarshal(response.Body, &payload); err != nil {
		writeAPIError(w, http.StatusBadGateway, "CORE_INVALID_RESPONSE", "Core 账号操作响应无效")
		return
	}
	payload = sanitizeCustomerPayload(payload)
	body, err := json.Marshal(payload)
	if err != nil {
		writeAPIError(w, http.StatusBadGateway, "CORE_INVALID_RESPONSE", "Core 账号操作响应无效")
		return
	}
	response.Body = body
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(response.Body)
}

func (s *Server) accountMarketTestAccount(w http.ResponseWriter, r *http.Request) {
	accountID, _, ok := s.requireCustomerAccount(w, r)
	if !ok {
		return
	}
	var input customerAccountTestRequest
	if r.Body != nil {
		if err := decodeJSON(r, &input); err != nil && !errors.Is(err, io.EOF) {
			writeAPIError(w, http.StatusBadRequest, "INVALID_ACCOUNT_TEST", "测试参数无效")
			return
		}
	}
	core, ok := s.core.(marketCoreRaw)
	if !ok {
		writeAPIError(w, http.StatusServiceUnavailable, "CORE_TEST_UNAVAILABLE", "Core 测试接口不可用")
		return
	}
	response, err := core.AdminMarketRawRequest(r.Context(), http.MethodPost, fmt.Sprintf("/api/v1/admin/accounts/%d/test", accountID), input, "")
	if err != nil {
		s.writeRentalError(w, err)
		return
	}
	contentType := response.ContentType
	if contentType == "" {
		contentType = "text/event-stream; charset=utf-8"
	}
	body, err := sanitizeCustomerTestBody(response.Body)
	if err != nil {
		writeAPIError(w, http.StatusBadGateway, "CORE_INVALID_RESPONSE", "Core 测试响应无效")
		return
	}
	response.Body = body
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(response.Body)
}

func (s *Server) accountMarketAccountUsage(w http.ResponseWriter, r *http.Request) {
	accountID, _, ok := s.requireCustomerAccount(w, r)
	if !ok {
		return
	}
	source := r.URL.Query().Get("source")
	if source == "" {
		source = "active"
	}
	if source != "active" && source != "passive" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_USAGE_SOURCE", "source 必须是 active 或 passive")
		return
	}
	var usage map[string]any
	path := fmt.Sprintf("/api/v1/admin/accounts/%d/usage?source=%s&force=%t", accountID, source, r.URL.Query().Get("force") == "true")
	if err := s.rentalCoreRequest(r.Context(), http.MethodGet, path, nil, &usage, ""); err != nil {
		s.writeRentalError(w, err)
		return
	}
	writeSuccess(w, sanitizeCustomerMap(usage))
}
func (s *Server) accountMarketTodayStats(w http.ResponseWriter, r *http.Request) {
	accountID, _, ok := s.requireCustomerAccount(w, r)
	if !ok {
		return
	}
	var stats map[string]any
	if err := s.rentalCoreRequest(r.Context(), http.MethodGet, fmt.Sprintf("/api/v1/admin/accounts/%d/today-stats", accountID), nil, &stats, ""); err != nil {
		s.writeRentalError(w, err)
		return
	}
	writeSuccess(w, sanitizeCustomerMap(stats))
}

func (s *Server) accountMarketAccountStats(w http.ResponseWriter, r *http.Request) {
	accountID, _, ok := s.requireCustomerAccount(w, r)
	if !ok {
		return
	}
	days := 30
	if value, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && value > 0 && value <= 90 {
		days = value
	}
	var stats map[string]any
	path := fmt.Sprintf("/api/v1/admin/accounts/%d/stats?days=%d", accountID, days)
	if err := s.rentalCoreRequest(r.Context(), http.MethodGet, path, nil, &stats, ""); err != nil {
		s.writeRentalError(w, err)
		return
	}
	writeSuccess(w, sanitizeCustomerMap(stats))
}

type customerBatchAccountIDs struct {
	AccountIDs []int64 `json:"account_ids"`
	Force      bool    `json:"force"`
}

func (s *Server) accountMarketBatchTodayStats(w http.ResponseWriter, r *http.Request) {
	var input customerBatchAccountIDs
	if err := decodeJSON(r, &input); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_ACCOUNT_IDS", "账号列表参数无效")
		return
	}
	allowed, ok := s.accountMarketUsageAllowedIDs(w, r)
	if !ok {
		return
	}
	ids := filterCustomerIDs(input.AccountIDs, allowed)
	stats := map[string]any{}
	if len(ids) > 0 {
		var response struct {
			Stats map[string]any `json:"stats"`
		}
		if err := s.rentalCoreRequest(r.Context(), http.MethodPost, "/api/v1/admin/accounts/today-stats/batch", map[string]any{"account_ids": ids}, &response, ""); err != nil {
			s.writeRentalError(w, err)
			return
		}
		for id, value := range response.Stats {
			if parsed, err := strconv.ParseInt(id, 10, 64); err == nil && allowed[parsed] {
				stats[id] = sanitizeCustomerPayload(value)
			}
		}
	}
	writeSuccess(w, map[string]any{"stats": sanitizeCustomerPayload(stats)})
}

func (s *Server) accountMarketBatchUsage(w http.ResponseWriter, r *http.Request) {
	var input customerBatchAccountIDs
	if err := decodeJSON(r, &input); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_ACCOUNT_IDS", "账号列表参数无效")
		return
	}
	allowed, ok := s.accountMarketUsageAllowedIDs(w, r)
	if !ok {
		return
	}
	ids := filterCustomerIDs(input.AccountIDs, allowed)
	usage := map[string]any{}
	errorsByAccount := map[string]string{}
	if len(ids) > 0 {
		var response struct {
			Usage  map[string]any    `json:"usage"`
			Errors map[string]string `json:"errors"`
		}
		if err := s.rentalCoreRequest(r.Context(), http.MethodPost, "/api/v1/admin/accounts/usage/batch", map[string]any{"account_ids": ids, "force": input.Force}, &response, ""); err != nil {
			s.writeRentalError(w, err)
			return
		}
		for id, value := range response.Usage {
			if parsed, err := strconv.ParseInt(id, 10, 64); err == nil && allowed[parsed] {
				usage[id] = sanitizeCustomerPayload(value)
			}
		}
		for id, value := range response.Errors {
			if parsed, err := strconv.ParseInt(id, 10, 64); err == nil && allowed[parsed] {
				errorsByAccount[id] = value
			}
		}
	}
	writeSuccess(w, map[string]any{"usage": sanitizeCustomerPayload(usage), "errors": errorsByAccount})
}

func (s *Server) accountMarketUsageAllowedIDs(w http.ResponseWriter, r *http.Request) (map[int64]bool, bool) {
	ids, ok := s.accountMarketUsageAccountIDs(r)
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "ACCOUNT_ACCESS_CHECK_FAILED", "账号权限校验失败")
		return nil, false
	}
	allowed := make(map[int64]bool, len(ids))
	for _, id := range ids {
		allowed[id] = true
	}
	return allowed, true
}

func filterCustomerIDs(ids []int64, allowed map[int64]bool) []int64 {
	seen := map[int64]bool{}
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 && allowed[id] && !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}

func (s *Server) filterLiveCustomerAccountIDs(r *http.Request, ids []int64) ([]int64, error) {
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		var account customerAccount
		if err := s.rentalCoreRequest(r.Context(), http.MethodGet, fmt.Sprintf("/api/v1/admin/accounts/%d", id), nil, &account, ""); err != nil {
			return nil, err
		}
		if account.ExpiresAt != nil && *account.ExpiresAt <= time.Now().Unix() {
			continue
		}
		result = append(result, id)
	}
	return result, nil
}
func (s *Server) accountMarketSetSchedulable(w http.ResponseWriter, r *http.Request) {
	accountID, _, ok := s.requireCustomerAccount(w, r)
	if !ok {
		return
	}
	var input struct {
		Schedulable *bool `json:"schedulable"`
	}
	if err := decodeJSON(r, &input); err != nil || input.Schedulable == nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_SCHEDULABLE", "schedulable 必须是布尔值")
		return
	}
	var account customerAccount
	if err := s.rentalCoreRequest(r.Context(), http.MethodPost, fmt.Sprintf("/api/v1/admin/accounts/%d/schedulable", accountID), map[string]bool{"schedulable": *input.Schedulable}, &account, ""); err != nil {
		s.writeRentalError(w, err)
		return
	}
	writeSuccess(w, sanitizeCustomerAccount(account))
}
func (s *Server) accountMarketTempUnschedulable(w http.ResponseWriter, r *http.Request) {
	accountID, _, ok := s.requireCustomerAccount(w, r)
	if !ok {
		return
	}
	var state map[string]any
	if err := s.rentalCoreRequest(r.Context(), http.MethodGet, fmt.Sprintf("/api/v1/admin/accounts/%d/temp-unschedulable", accountID), nil, &state, ""); err != nil {
		s.writeRentalError(w, err)
		return
	}
	writeSuccess(w, sanitizeCustomerPayload(state))
}
func (s *Server) accountMarketClearTempUnschedulable(w http.ResponseWriter, r *http.Request) {
	accountID, _, ok := s.requireCustomerAccount(w, r)
	if !ok {
		return
	}
	var state map[string]any
	if err := s.rentalCoreRequest(r.Context(), http.MethodDelete, fmt.Sprintf("/api/v1/admin/accounts/%d/temp-unschedulable", accountID), nil, &state, ""); err != nil {
		s.writeRentalError(w, err)
		return
	}
	writeSuccess(w, sanitizeCustomerPayload(state))
}

// Sanitize JSON responses and each JSON SSE event before exposing admin tests.
func sanitizeCustomerTestBody(body []byte) ([]byte, error) {
	cleanJSON := func(raw []byte) ([]byte, error) {
		var payload any
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, err
		}
		return json.Marshal(sanitizeCustomerPayload(payload))
	}
	if json.Valid(body) {
		return cleanJSON(body)
	}
	events := strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n\n")
	for i, event := range events {
		lines := strings.Split(event, "\n")
		fields, data := []string{}, []string{}
		for _, line := range lines {
			if strings.HasPrefix(line, "data:") {
				data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
			} else {
				fields = append(fields, line)
			}
		}
		if len(data) == 0 {
			continue
		}
		raw := strings.Join(data, "\n")
		if raw == "[DONE]" {
			continue
		}
		clean, err := cleanJSON([]byte(raw))
		if err != nil {
			return nil, err
		}
		fields = append(fields, "data: "+string(clean))
		events[i] = strings.Join(fields, "\n")
	}
	return []byte(strings.Join(events, "\n\n")), nil
}
