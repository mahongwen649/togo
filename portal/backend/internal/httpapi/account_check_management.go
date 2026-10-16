package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const accountCheckScheduledResultLimit = 3

func (s *Server) accountCheckUpdateGroups(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireActiveAdmin(w, r); !ok {
		return
	}
	accountID, ok := pathID(w, r, "accountID")
	if !ok {
		return
	}
	core, accessToken, ok := s.accountCheckCore(w, r)
	if !ok || !s.requireOpenAICheckAccount(w, r, core, accessToken, accountID) {
		return
	}
	var input struct {
		GroupIDs []int64 `json:"group_ids"`
	}
	if err := decodeJSON(r, &input); err != nil || len(input.GroupIDs) > 500 {
		writeAPIError(w, http.StatusBadRequest, "INVALID_GROUPS", "账号分组参数无效")
		return
	}
	seen := make(map[int64]struct{}, len(input.GroupIDs))
	groupIDs := make([]int64, 0, len(input.GroupIDs))
	for _, id := range input.GroupIDs {
		if id <= 0 {
			writeAPIError(w, http.StatusBadRequest, "INVALID_GROUPS", "账号分组参数无效")
			return
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		groupIDs = append(groupIDs, id)
	}
	var account checkAccount
	if err := accountCheckRequest(r.Context(), core, http.MethodPut, fmt.Sprintf("/api/v1/admin/accounts/%d", accountID), map[string]any{"group_ids": groupIDs}, &account, accessToken); err != nil {
		s.writeRentalError(w, err)
		return
	}
	writeSuccess(w, account)
}

func accountCheckScheduledRequest(ctx context.Context, core accountCheckCore, method, path string, payload, out any, accessToken string) error {
	response, err := core.AdminUserMarketRequest(ctx, method, path, payload, accessToken)
	if err != nil {
		return err
	}
	body := strings.TrimSpace(string(response.Body))
	if body == "" {
		return fmt.Errorf("Core returned an empty scheduled-test response")
	}
	if out != nil {
		if err := json.Unmarshal(response.Body, out); err != nil {
			return fmt.Errorf("invalid Core scheduled-test response: %w", err)
		}
	}
	return nil
}

func (s *Server) accountCheckScheduledContext(w http.ResponseWriter, r *http.Request) (int64, accountCheckCore, string, bool) {
	if _, ok := s.requireActiveAdmin(w, r); !ok {
		return 0, nil, "", false
	}
	accountID, ok := pathID(w, r, "accountID")
	if !ok {
		return 0, nil, "", false
	}
	core, accessToken, ok := s.accountCheckCore(w, r)
	if !ok || !s.requireOpenAICheckAccount(w, r, core, accessToken, accountID) {
		return 0, nil, "", false
	}
	return accountID, core, accessToken, true
}

func (s *Server) accountCheckScheduledPlans(w http.ResponseWriter, r *http.Request) {
	accountID, core, accessToken, ok := s.accountCheckScheduledContext(w, r)
	if !ok {
		return
	}
	path := customerScheduledTestPlansPath(accountID)
	if r.Method == http.MethodGet {
		plans := make([]customerScheduledTestPlan, 0)
		if err := accountCheckScheduledRequest(r.Context(), core, http.MethodGet, path, nil, &plans, accessToken); err != nil {
			s.writeRentalError(w, err)
			return
		}
		if plans == nil {
			plans = make([]customerScheduledTestPlan, 0)
		}
		writeSuccess(w, plans)
		return
	}
	var input customerCreateScheduledTestPlanRequest
	if err := decodeJSON(r, &input); err != nil || strings.TrimSpace(input.CronExpression) == "" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_SCHEDULED_TEST_PLAN", "定时检查参数无效")
		return
	}
	input.AccountID = accountID
	input.MaxResults = accountCheckScheduledResultLimit
	autoRecover := false
	input.AutoRecover = &autoRecover
	var plan customerScheduledTestPlan
	if err := accountCheckScheduledRequest(r.Context(), core, http.MethodPost, "/api/v1/admin/scheduled-test-plans", input, &plan, accessToken); err != nil {
		s.writeRentalError(w, err)
		return
	}
	writeSuccess(w, plan)
}

func (s *Server) requireAccountCheckScheduledPlan(w http.ResponseWriter, r *http.Request, core accountCheckCore, accessToken string, accountID, planID int64) bool {
	var plans []customerScheduledTestPlan
	if err := accountCheckScheduledRequest(r.Context(), core, http.MethodGet, customerScheduledTestPlansPath(accountID), nil, &plans, accessToken); err != nil {
		s.writeRentalError(w, err)
		return false
	}
	for _, plan := range plans {
		if plan.ID == planID && plan.AccountID == accountID {
			return true
		}
	}
	writeAPIError(w, http.StatusNotFound, "SCHEDULED_TEST_PLAN_NOT_FOUND", "定时检查计划不存在")
	return false
}

func (s *Server) accountCheckScheduledPlan(w http.ResponseWriter, r *http.Request) {
	accountID, core, accessToken, ok := s.accountCheckScheduledContext(w, r)
	if !ok {
		return
	}
	planID, ok := pathID(w, r, "planID")
	if !ok || !s.requireAccountCheckScheduledPlan(w, r, core, accessToken, accountID, planID) {
		return
	}
	path := customerScheduledTestPlanPath(planID)
	if r.Method == http.MethodDelete {
		var result map[string]any
		if err := accountCheckScheduledRequest(r.Context(), core, http.MethodDelete, path, nil, &result, accessToken); err != nil {
			s.writeRentalError(w, err)
			return
		}
		writeSuccess(w, result)
		return
	}
	var input customerUpdateScheduledTestPlanRequest
	if err := decodeJSON(r, &input); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_SCHEDULED_TEST_PLAN", "定时检查参数无效")
		return
	}
	input.MaxResults = accountCheckScheduledResultLimit
	input.AutoRecover = nil
	var plan customerScheduledTestPlan
	if err := accountCheckScheduledRequest(r.Context(), core, http.MethodPut, path, input, &plan, accessToken); err != nil {
		s.writeRentalError(w, err)
		return
	}
	writeSuccess(w, plan)
}

func (s *Server) accountCheckScheduledResults(w http.ResponseWriter, r *http.Request) {
	accountID, core, accessToken, ok := s.accountCheckScheduledContext(w, r)
	if !ok {
		return
	}
	planID, ok := pathID(w, r, "planID")
	if !ok || !s.requireAccountCheckScheduledPlan(w, r, core, accessToken, accountID, planID) {
		return
	}
	limit := 1
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > accountCheckScheduledResultLimit {
			writeAPIError(w, http.StatusBadRequest, "INVALID_LIMIT", "limit 必须是 1 到 3 之间的整数")
			return
		}
		limit = parsed
	}
	var results []customerScheduledTestResult
	path := fmt.Sprintf("%s/results?limit=%d", customerScheduledTestPlanPath(planID), limit)
	if err := accountCheckScheduledRequest(r.Context(), core, http.MethodGet, path, nil, &results, accessToken); err != nil {
		s.writeRentalError(w, err)
		return
	}
	writeSuccess(w, results)
}
