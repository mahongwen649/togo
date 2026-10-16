package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Account-package records reuse Core's existing usage and ops endpoints. Core
// accepts one account_id per request, so Portal aggregates the allowed account
// slices and returns the same response shapes as the original Core pages.
func (s *Server) accountMarketUsageAccountIDs(r *http.Request) ([]int64, bool) {
	if s.portalDB == nil {
		return nil, false
	}
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if !strings.HasPrefix(header, "Bearer ") || token == "" {
		return nil, false
	}
	user, err := s.core.Profile(r.Context(), token)
	if err != nil {
		return nil, false
	}
	rows, err := s.portalDB.QueryContext(r.Context(), `
		SELECT core_account_ids
		FROM portal_account_orders
		WHERE user_id = $1 AND status = 'completed'
		  AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)`, user.ID)
	if err != nil {
		return nil, false
	}
	defer rows.Close()
	seen := map[int64]bool{}
	ids := make([]int64, 0)
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, false
		}
		var values []int64
		if err := json.Unmarshal(raw, &values); err != nil {
			return nil, false
		}
		for _, id := range values {
			if id > 0 && !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, false
	}
	ids, err = s.filterLiveCustomerAccountIDs(r, ids)
	if err != nil {
		return nil, false
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, true
}

func accountMarketErrorCategory(phase, errType string) string {
	switch phase {
	case "auth":
		return "auth"
	case "routing":
		return "service_unavailable"
	case "account_auth", "upstream", "network":
		return "upstream"
	case "internal":
		return "internal"
	case "request":
		switch errType {
		case "rate_limit_error":
			return "rate_limit"
		case "billing_error", "subscription_error":
			return "quota"
		case "invalid_request_error":
			return "invalid_request"
		case "cyber_policy":
			return "cyber"
		}
	}
	return "other"
}

func accountMarketErrorView(item map[string]any) map[string]any {
	model := accountUsageRowValue(item, "requested_model")
	if model == "<nil>" || model == "" {
		model = accountUsageRowValue(item, "model")
	}
	return map[string]any{
		"id": item["id"], "created_at": item["created_at"], "model": model,
		"inbound_endpoint": item["inbound_endpoint"], "status_code": item["status_code"],
		"category": accountMarketErrorCategory(accountUsageRowValue(item, "phase"), accountUsageRowValue(item, "type")),
		"platform": item["platform"], "message": item["message"],
		"key_name": item["api_key_name"], "key_deleted": item["api_key_deleted"],
		"client_ip":    item["client_ip"],
		"request_type": item["request_type"], "stream": item["stream"], "user_agent": item["user_agent"],
	}
}

func (s *Server) accountMarketUsageErrors(w http.ResponseWriter, r *http.Request) {
	ids, ok := s.accountMarketUsageIDs(w, r)
	if !ok {
		return
	}
	page, pageSize := parsePageQuery(r.URL.Query())
	rows := make([]map[string]any, 0)
	for _, id := range ids {
		query := customerUsageQuery(r, id, minInt(page*pageSize, 1000))
		var result corePage[map[string]any]
		if err := s.rentalCoreRequest(r.Context(), http.MethodGet, "/api/v1/admin/ops/request-errors?"+query, nil, &result, ""); err != nil {
			writeAPIError(w, http.StatusBadGateway, "CORE_INVALID_RESPONSE", "Core 错误记录响应无效")
			return
		}
		for _, item := range result.Items {
			rows = append(rows, accountMarketErrorView(item))
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		left, right := accountUsageRowValue(rows[i], "created_at"), accountUsageRowValue(rows[j], "created_at")
		if strings.EqualFold(r.URL.Query().Get("sort_order"), "asc") {
			return left < right
		}
		return left > right
	})
	total := len(rows)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := minInt(start+pageSize, total)
	pages := (total + pageSize - 1) / pageSize
	if pages == 0 {
		pages = 1
	}
	writeSuccess(w, map[string]any{"items": rows[start:end], "total": total, "page": page, "page_size": pageSize, "pages": pages})
}

func (s *Server) accountMarketUsageErrorDetail(w http.ResponseWriter, r *http.Request) {
	ids, ok := s.accountMarketUsageIDs(w, r)
	if !ok {
		return
	}
	errorID, ok := pathID(w, r, "errorID")
	if !ok {
		return
	}
	var item map[string]any
	if err := s.rentalCoreRequest(r.Context(), http.MethodGet, fmt.Sprintf("/api/v1/admin/ops/request-errors/%d", errorID), nil, &item, ""); err != nil {
		writeAPIError(w, http.StatusBadGateway, "CORE_INVALID_RESPONSE", "Core 错误详情暂时不可用")
		return
	}
	field := "account_id"
	if _, ok := r.Context().Value(customerUsageScopeKey{}).(customerUsageScope); ok {
		field = "api_key_id"
	}
	accountID := int64(numericStat(item[field]))
	allowed := false
	for _, id := range ids {
		if id == accountID {
			allowed = true
			break
		}
	}
	if !allowed {
		writeAPIError(w, http.StatusNotFound, "ERROR_NOT_FOUND", "错误记录不存在")
		return
	}
	view := accountMarketErrorView(item)
	view["error_body"] = sanitizeCustomerPayload(item["error_body"])
	view["upstream_status_code"] = item["upstream_status_code"]
	writeSuccess(w, view)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *Server) accountMarketUsageIDs(w http.ResponseWriter, r *http.Request) ([]int64, bool) {
	if scope, ok := r.Context().Value(customerUsageScopeKey{}).(customerUsageScope); ok {
		return scope.IDs, true
	}
	ids, ok := s.accountMarketUsageAccountIDs(r)
	if !ok {
		writeAPIError(w, http.StatusBadGateway, "ACCOUNT_USAGE_READ_FAILED", "账号使用记录暂时不可用")
		return nil, false
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("account_id")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			writeAPIError(w, http.StatusBadRequest, "INVALID_ACCOUNT_ID", "账号 ID 无效")
			return nil, false
		}
		for _, allowed := range ids {
			if allowed == id {
				return []int64{id}, true
			}
		}
		writeAPIError(w, http.StatusNotFound, "ACCOUNT_NOT_FOUND", "该账号不在你的有效套餐中")
		return nil, false
	}
	return ids, true
}

func accountMarketUsageQuery(source map[string][]string, accountID int64, pageSize int) string {
	query := cloneQuery(source)
	query.Del("account_id")
	query.Del("page")
	query.Del("page_size")
	query.Del("exact_total")
	query.Set("account_id", strconv.FormatInt(accountID, 10))
	query.Set("page", "1")
	query.Set("page_size", strconv.Itoa(pageSize))
	query.Set("exact_total", "true")
	return query.Encode()
}

func (s *Server) accountMarketUsageRows(r *http.Request, ids []int64, limit int) ([]map[string]any, int, bool) {
	if len(ids) == 0 {
		return []map[string]any{}, 0, true
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 1000 {
		limit = 1000
	}
	rows := make([]map[string]any, 0)
	total := 0
	for _, id := range ids {
		var page corePage[map[string]any]
		if err := s.rentalCoreRequest(r.Context(), http.MethodGet, "/api/v1/admin/usage?"+customerUsageQuery(r, id, limit), nil, &page, ""); err != nil {
			return nil, 0, false
		}
		total += int(page.Total)
		for _, item := range page.Items {
			if clean, ok := sanitizeCustomerPayload(item).(map[string]any); ok {
				rows = append(rows, clean)
			}
		}
	}
	return rows, total, true
}

func accountUsageRowValue(row map[string]any, key string) string {
	if value, ok := row[key].(string); ok {
		return value
	}
	return fmt.Sprint(row[key])
}

func (s *Server) accountMarketUsageRecords(w http.ResponseWriter, r *http.Request) {
	ids, ok := s.accountMarketUsageIDs(w, r)
	if !ok {
		return
	}
	page, pageSize := parsePageQuery(r.URL.Query())
	fetchSize := page * pageSize
	if fetchSize > 1000 {
		fetchSize = 1000
	}
	rows, total, ok := s.accountMarketUsageRows(r, ids, fetchSize)
	if !ok {
		writeAPIError(w, http.StatusBadGateway, "CORE_INVALID_RESPONSE", "Core 使用记录响应无效")
		return
	}
	sort.SliceStable(rows, func(i, j int) bool {
		by := strings.TrimSpace(r.URL.Query().Get("sort_by"))
		if by == "model" {
			return accountUsageRowValue(rows[i], "model") < accountUsageRowValue(rows[j], "model")
		}
		left, right := accountUsageRowValue(rows[i], "created_at"), accountUsageRowValue(rows[j], "created_at")
		if strings.EqualFold(r.URL.Query().Get("sort_order"), "asc") {
			return left < right
		}
		return left > right
	})
	start := (page - 1) * pageSize
	if start > len(rows) {
		start = len(rows)
	}
	end := start + pageSize
	if end > len(rows) {
		end = len(rows)
	}
	pages := (total + pageSize - 1) / pageSize
	if pages == 0 {
		pages = 1
	}
	writeSuccess(w, map[string]any{"items": rows[start:end], "total": total, "page": page, "page_size": pageSize, "pages": pages})
}

func mergeUsageSeries(target map[string]map[string]any, source []any, key string) {
	for _, raw := range source {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name := accountUsageRowValue(item, key)
		if _, ok := target[name]; !ok {
			target[name] = map[string]any{}
			for k, v := range item {
				target[name][k] = v
			}
			continue
		}
		for _, field := range []string{"requests", "input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens", "total_tokens", "tokens", "cost", "actual_cost"} {
			target[name][field] = numericStat(target[name][field]) + numericStat(item[field])
		}
	}
}

func mergeUsageMapStats(target map[string]any, source map[string]any) {
	for _, key := range []string{"total_requests", "total_input_tokens", "total_output_tokens", "total_cache_tokens", "total_cache_creation_tokens", "total_cache_read_tokens", "total_tokens", "total_cost", "total_actual_cost"} {
		target[key] = numericStat(target[key]) + numericStat(source[key])
	}
	for _, pair := range [][2]string{{"endpoints", "endpoint"}, {"upstream_endpoints", "endpoint"}, {"endpoint_paths", "endpoint"}} {
		series := map[string]map[string]any{}
		if existing, ok := target[pair[0]].([]any); ok {
			mergeUsageSeries(series, existing, pair[1])
		}
		if incoming, ok := source[pair[0]].([]any); ok {
			mergeUsageSeries(series, incoming, pair[1])
		}
		items := make([]any, 0, len(series))
		for _, item := range series {
			items = append(items, item)
		}
		target[pair[0]] = items
	}
}

func (s *Server) accountMarketUsageStats(w http.ResponseWriter, r *http.Request) {
	ids, ok := s.accountMarketUsageIDs(w, r)
	if !ok {
		return
	}
	stats := emptyUsageStats()
	for _, id := range ids {
		var item map[string]any
		if err := s.rentalCoreRequest(r.Context(), http.MethodGet, "/api/v1/admin/usage/stats?"+customerUsageQuery(r, id, 100), nil, &item, ""); err != nil {
			writeAPIError(w, http.StatusBadGateway, "CORE_INVALID_RESPONSE", "Core 使用统计响应无效")
			return
		}
		mergeUsageMapStats(stats, item)
	}
	delete(stats, "total_account_cost")
	delete(stats, "upstream_endpoints")
	delete(stats, "endpoint_paths")
	writeSuccess(w, sanitizeCustomerMap(stats))
}

func (s *Server) accountMarketUsageModels(w http.ResponseWriter, r *http.Request) {
	ids, ok := s.accountMarketUsageIDs(w, r)
	if !ok {
		return
	}
	series := map[string]map[string]any{}
	for _, id := range ids {
		var result struct {
			Models []any `json:"models"`
		}
		if err := s.rentalCoreRequest(r.Context(), http.MethodGet, "/api/v1/admin/dashboard/models?"+customerUsageQuery(r, id, 100), nil, &result, ""); err != nil {
			writeAPIError(w, http.StatusBadGateway, "CORE_INVALID_RESPONSE", "Core 模型统计响应无效")
			return
		}
		mergeUsageSeries(series, result.Models, "model")
	}
	items := make([]any, 0, len(series))
	for _, item := range series {
		items = append(items, item)
	}
	writeSuccess(w, map[string]any{"models": sanitizeCustomerPayload(items), "start_date": r.URL.Query().Get("start_date"), "end_date": r.URL.Query().Get("end_date")})
}

func (s *Server) accountMarketUsageSnapshot(w http.ResponseWriter, r *http.Request) {
	ids, ok := s.accountMarketUsageIDs(w, r)
	if !ok {
		return
	}
	response := map[string]any{"generated_at": time.Now().UTC().Format(time.RFC3339), "start_date": r.URL.Query().Get("start_date"), "end_date": r.URL.Query().Get("end_date"), "granularity": r.URL.Query().Get("granularity")}
	trend := map[string]map[string]any{}
	for _, id := range ids {
		query := customerUsageQuery(r, id, 100)
		var item map[string]any
		if err := s.rentalCoreRequest(r.Context(), http.MethodGet, "/api/v1/admin/dashboard/snapshot-v2?"+query, nil, &item, ""); err != nil {
			writeAPIError(w, http.StatusBadGateway, "CORE_INVALID_RESPONSE", "Core 使用趋势响应无效")
			return
		}
		if values, ok := item["trend"].([]any); ok {
			mergeUsageSeries(trend, values, "date")
		}
	}
	trendItems := make([]any, 0, len(trend))
	for _, item := range trend {
		trendItems = append(trendItems, item)
	}
	response["trend"] = sanitizeCustomerPayload(trendItems)
	writeSuccess(w, response)
}
