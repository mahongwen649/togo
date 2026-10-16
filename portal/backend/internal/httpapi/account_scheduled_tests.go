package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// These shapes intentionally mirror Core's scheduled-test API. Core returns
// plain JSON for this handler rather than Portal's {code,data} envelope.
type customerScheduledTestPlan struct {
	ID             int64  `json:"id"`
	AccountID      int64  `json:"account_id"`
	ModelID        string `json:"model_id"`
	CronExpression string `json:"cron_expression"`
	Enabled        bool   `json:"enabled"`
	MaxResults     int    `json:"max_results"`
	AutoRecover    bool   `json:"auto_recover"`
	LastRunAt      *string `json:"last_run_at"`
	NextRunAt      *string `json:"next_run_at"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type customerScheduledTestResult struct {
	ID           int64  `json:"id"`
	PlanID       int64  `json:"plan_id"`
	Status       string `json:"status"`
	ResponseText string `json:"response_text"`
	ErrorMessage string `json:"error_message"`
	LatencyMs    int64  `json:"latency_ms"`
	StartedAt    string `json:"started_at"`
	FinishedAt   string `json:"finished_at"`
	CreatedAt    string `json:"created_at"`
}

type customerCreateScheduledTestPlanRequest struct {
	AccountID      int64  `json:"account_id"`
	ModelID        string `json:"model_id"`
	CronExpression string `json:"cron_expression"`
	Enabled        *bool  `json:"enabled"`
	MaxResults     int    `json:"max_results"`
	AutoRecover    *bool  `json:"auto_recover"`
}

type customerUpdateScheduledTestPlanRequest struct {
	ModelID        string `json:"model_id"`
	CronExpression string `json:"cron_expression"`
	Enabled        *bool  `json:"enabled"`
	MaxResults     int    `json:"max_results"`
	AutoRecover    *bool  `json:"auto_recover"`
}

func customerScheduledTestPlansPath(accountID int64) string {
	return fmt.Sprintf("/api/v1/admin/accounts/%d/scheduled-test-plans", accountID)
}

func customerScheduledTestPlanPath(planID int64) string {
	return fmt.Sprintf("/api/v1/admin/scheduled-test-plans/%d", planID)
}

func (s *Server) customerScheduledTestRawRequest(r *http.Request, method, path string, payload any) (json.RawMessage, error) {
	core, ok := s.core.(marketCore)
	if !ok {
		return nil, fmt.Errorf("Core account business adapter is unavailable")
	}
	response, err := core.AdminMarketRequest(r.Context(), method, path, payload, "")
	if err != nil {
		return nil, err
	}
	body := json.RawMessage(strings.TrimSpace(string(response.Body)))
	if len(body) == 0 || string(body) == "null" {
		return nil, fmt.Errorf("Core returned an empty scheduled-test response")
	}
	return body, nil
}

func decodeCustomerScheduledTestResponse(body json.RawMessage, out any) error {
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("invalid Core scheduled-test response: %w", err)
	}
	return nil
}

func (s *Server) accountMarketScheduledTestPlans(w http.ResponseWriter, r *http.Request) {
	accountID, _, ok := s.requireCustomerAccount(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		body, err := s.customerScheduledTestRawRequest(r, http.MethodGet, customerScheduledTestPlansPath(accountID), nil)
		if err != nil {
			s.writeRentalError(w, err)
			return
		}
		var plans []customerScheduledTestPlan
		if err := decodeCustomerScheduledTestResponse(body, &plans); err != nil {
			s.writeRentalError(w, err)
			return
		}
		writeSuccess(w, plans)
	case http.MethodPost:
		var input customerCreateScheduledTestPlanRequest
		if err := decodeJSON(r, &input); err != nil {
			writeAPIError(w, http.StatusBadRequest, "INVALID_SCHEDULED_TEST_PLAN", "定时测试计划参数无效")
			return
		}
		if input.AccountID != 0 && input.AccountID != accountID {
			writeAPIError(w, http.StatusForbidden, "ACCOUNT_ACCESS_DENIED", "不能为其他账号创建定时测试计划")
			return
		}
		input.AccountID = accountID
		if err := s.createCustomerScheduledTestPlan(w, r, input); err != nil {
			return
		}
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "不支持的请求方法")
	}
}

func (s *Server) createCustomerScheduledTestPlan(w http.ResponseWriter, r *http.Request, input customerCreateScheduledTestPlanRequest) error {
	body, err := s.customerScheduledTestRawRequest(r, http.MethodPost, "/api/v1/admin/scheduled-test-plans", input)
	if err != nil {
		s.writeRentalError(w, err)
		return err
	}
	var plan customerScheduledTestPlan
	if err := decodeCustomerScheduledTestResponse(body, &plan); err != nil {
		s.writeRentalError(w, err)
		return err
	}
	writeSuccess(w, plan)
	return nil
}

func (s *Server) requireCustomerScheduledTestPlan(w http.ResponseWriter, r *http.Request, accountID, planID int64) bool {
	body, err := s.customerScheduledTestRawRequest(r, http.MethodGet, customerScheduledTestPlansPath(accountID), nil)
	if err != nil {
		s.writeRentalError(w, err)
		return false
	}
	var plans []customerScheduledTestPlan
	if err := decodeCustomerScheduledTestResponse(body, &plans); err != nil {
		s.writeRentalError(w, err)
		return false
	}
	for _, plan := range plans {
		if plan.ID == planID && plan.AccountID == accountID {
			return true
		}
	}
	writeAPIError(w, http.StatusNotFound, "SCHEDULED_TEST_PLAN_NOT_FOUND", "定时测试计划不存在")
	return false
}

func (s *Server) accountMarketScheduledTestPlan(w http.ResponseWriter, r *http.Request) {
	accountID, _, ok := s.requireCustomerAccount(w, r)
	if !ok {
		return
	}
	planID, ok := pathID(w, r, "planID")
	if !ok || !s.requireCustomerScheduledTestPlan(w, r, accountID, planID) {
		return
	}
	switch r.Method {
	case http.MethodPut:
		s.updateCustomerScheduledTestPlan(w, r, planID)
	case http.MethodDelete:
		s.deleteCustomerScheduledTestPlan(w, r, planID)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "不支持的请求方法")
	}
}

func (s *Server) updateCustomerScheduledTestPlan(w http.ResponseWriter, r *http.Request, planID int64) {
	var input customerUpdateScheduledTestPlanRequest
	if err := decodeJSON(r, &input); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_SCHEDULED_TEST_PLAN", "定时测试计划参数无效")
		return
	}
	body, err := s.customerScheduledTestRawRequest(r, http.MethodPut, customerScheduledTestPlanPath(planID), input)
	if err != nil {
		s.writeRentalError(w, err)
		return
	}
	var plan customerScheduledTestPlan
	if err := decodeCustomerScheduledTestResponse(body, &plan); err != nil {
		s.writeRentalError(w, err)
		return
	}
	writeSuccess(w, plan)
}

func (s *Server) deleteCustomerScheduledTestPlan(w http.ResponseWriter, r *http.Request, planID int64) {
	body, err := s.customerScheduledTestRawRequest(r, http.MethodDelete, customerScheduledTestPlanPath(planID), nil)
	if err != nil {
		s.writeRentalError(w, err)
		return
	}
	var result map[string]any
	if err := decodeCustomerScheduledTestResponse(body, &result); err != nil {
		s.writeRentalError(w, err)
		return
	}
	writeSuccess(w, result)
}

func (s *Server) accountMarketScheduledTestResults(w http.ResponseWriter, r *http.Request) {
	accountID, _, ok := s.requireCustomerAccount(w, r)
	if !ok {
		return
	}
	planID, ok := pathID(w, r, "planID")
	if !ok {
		return
	}
	if !s.requireCustomerScheduledTestPlan(w, r, accountID, planID) {
		return
	}

	limit := 20
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 || parsed > 100 {
			writeAPIError(w, http.StatusBadRequest, "INVALID_LIMIT", "limit 必须是 1 到 100 之间的整数")
			return
		}
		limit = parsed
	}
	body, err := s.customerScheduledTestRawRequest(r, http.MethodGet, fmt.Sprintf("%s/results?limit=%d", customerScheduledTestPlanPath(planID), limit), nil)
	if err != nil {
		s.writeRentalError(w, err)
		return
	}
	var results []customerScheduledTestResult
	if err := decodeCustomerScheduledTestResponse(body, &results); err != nil {
		s.writeRentalError(w, err)
		return
	}
	writeSuccess(w, results)
}
