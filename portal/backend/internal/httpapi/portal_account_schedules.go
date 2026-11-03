package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

var accountScheduleParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

type portalAccountSchedule struct {
	ID              int64      `json:"id"`
	AdminID         int64      `json:"-"`
	AccountID       int64      `json:"account_id"`
	ModelID         string     `json:"model_id"`
	ReasoningEffort string     `json:"reasoning_effort"`
	Prompt          string     `json:"prompt"`
	CronExpression  string     `json:"cron_expression"`
	Enabled         bool       `json:"enabled"`
	MaxResults      int        `json:"max_results"`
	LastRunAt       *time.Time `json:"last_run_at"`
	NextRunAt       time.Time  `json:"next_run_at"`
	RunningUntil    *time.Time `json:"-"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type portalScheduleInput struct {
	ModelID         *string `json:"model_id"`
	ReasoningEffort *string `json:"reasoning_effort"`
	Prompt          *string `json:"prompt"`
	CronExpression  *string `json:"cron_expression"`
	Enabled         *bool   `json:"enabled"`
}

const portalScheduleColumns = `id,admin_id,account_id,model_id,reasoning_effort,prompt,cron_expression,enabled,last_run_at,next_run_at,running_until,created_at,updated_at`

type scheduleScanner interface{ Scan(...any) error }

func scanPortalSchedule(row scheduleScanner) (portalAccountSchedule, error) {
	var plan portalAccountSchedule
	plan.MaxResults = 3
	err := row.Scan(&plan.ID, &plan.AdminID, &plan.AccountID, &plan.ModelID, &plan.ReasoningEffort, &plan.Prompt, &plan.CronExpression, &plan.Enabled, &plan.LastRunAt, &plan.NextRunAt, &plan.RunningUntil, &plan.CreatedAt, &plan.UpdatedAt)
	return plan, err
}

func nextAccountSchedule(expression string, from time.Time) (time.Time, error) {
	if len(expression) > 200 {
		return time.Time{}, errors.New("执行时间无效")
	}
	schedule, err := accountScheduleParser.Parse(expression)
	if err != nil {
		return time.Time{}, errors.New("执行时间无效，请重新选择时间或检查 Cron 表达式")
	}
	next := schedule.Next(from)
	if next.IsZero() {
		return time.Time{}, errors.New("所设时间没有可执行的日期")
	}
	return next, nil
}

func applyPortalScheduleInput(plan *portalAccountSchedule, input portalScheduleInput) error {
	if input.ModelID != nil {
		plan.ModelID = strings.TrimSpace(*input.ModelID)
	}
	if input.ReasoningEffort != nil {
		plan.ReasoningEffort = strings.TrimSpace(*input.ReasoningEffort)
	}
	if input.Prompt != nil {
		plan.Prompt = strings.TrimSpace(*input.Prompt)
	}
	if input.CronExpression != nil {
		plan.CronExpression = strings.TrimSpace(*input.CronExpression)
	}
	if input.Enabled != nil {
		plan.Enabled = *input.Enabled
	}
	if plan.ModelID == "" || len(plan.ModelID) > 256 || plan.Prompt == "" || len(plan.Prompt) > 64000 {
		return errors.New("请选择模型并填写检查题目（最多 64000 字节）")
	}
	switch plan.ReasoningEffort {
	case "", "none", "minimal", "low", "medium", "high", "xhigh":
	default:
		return errors.New("思考程度无效")
	}
	next, err := nextAccountSchedule(plan.CronExpression, time.Now())
	plan.NextRunAt = next
	return err
}

func (s *Server) registerPortalAccountSchedules(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/admin/account-checks/accounts/{accountID}/schedules", s.portalAccountSchedules)
	mux.HandleFunc("POST /api/v1/admin/account-checks/accounts/{accountID}/schedules", s.portalAccountSchedules)
	mux.HandleFunc("PUT /api/v1/admin/account-checks/accounts/{accountID}/schedules/{planID}", s.portalAccountSchedule)
	mux.HandleFunc("DELETE /api/v1/admin/account-checks/accounts/{accountID}/schedules/{planID}", s.portalAccountSchedule)
	mux.HandleFunc("GET /api/v1/admin/account-checks/accounts/{accountID}/schedules/{planID}/results", s.portalAccountScheduleResults)
}

func (s *Server) portalScheduleContext(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	admin, ok := s.requireActiveAdmin(w, r)
	if !ok {
		return 0, 0, false
	}
	accountID, ok := pathID(w, r, "accountID")
	if !ok {
		return 0, 0, false
	}
	if s.portalDB == nil {
		writeAPIError(w, 503, "SCHEDULE_UNAVAILABLE", "定时检查存储不可用")
		return 0, 0, false
	}
	return admin.ID, accountID, true
}

func (s *Server) portalAccountSchedules(w http.ResponseWriter, r *http.Request) {
	adminID, accountID, ok := s.portalScheduleContext(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		rows, err := s.portalDB.QueryContext(r.Context(), `SELECT `+portalScheduleColumns+` FROM portal_account_check_schedules WHERE admin_id=$1 AND account_id=$2 ORDER BY id DESC`, adminID, accountID)
		if err != nil {
			s.writeAccountHistoryError(w, err)
			return
		}
		defer rows.Close()
		plans := make([]portalAccountSchedule, 0)
		for rows.Next() {
			plan, err := scanPortalSchedule(rows)
			if err != nil {
				s.writeAccountHistoryError(w, err)
				return
			}
			plans = append(plans, plan)
		}
		if err = rows.Err(); err != nil {
			s.writeAccountHistoryError(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeSuccess(w, plans)
		return
	}
	core, token, ok := s.accountCheckCore(w, r)
	if !ok || !s.requireOpenAICheckAccount(w, r, core, token, accountID) {
		return
	}
	var input portalScheduleInput
	r.Body = http.MaxBytesReader(w, r.Body, 128*1024)
	if decodeJSON(r, &input) != nil {
		writeAPIError(w, 400, "INVALID_SCHEDULE", "定时检查参数无效")
		return
	}
	plan := portalAccountSchedule{AdminID: adminID, AccountID: accountID, Enabled: true, Prompt: "hi"}
	if err := applyPortalScheduleInput(&plan, input); err != nil {
		writeAPIError(w, 400, "INVALID_SCHEDULE", err.Error())
		return
	}
	created, err := scanPortalSchedule(s.portalDB.QueryRowContext(r.Context(), `INSERT INTO portal_account_check_schedules(admin_id,account_id,model_id,reasoning_effort,prompt,cron_expression,enabled,next_run_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING `+portalScheduleColumns, adminID, accountID, plan.ModelID, plan.ReasoningEffort, plan.Prompt, plan.CronExpression, plan.Enabled, plan.NextRunAt))
	if err != nil {
		s.writeAccountHistoryError(w, err)
		return
	}
	writeSuccess(w, created)
}

func loadOwnedPortalSchedule(ctx context.Context, db *sql.DB, adminID, accountID, planID int64) (portalAccountSchedule, error) {
	return scanPortalSchedule(db.QueryRowContext(ctx, `SELECT `+portalScheduleColumns+` FROM portal_account_check_schedules WHERE id=$1 AND admin_id=$2 AND account_id=$3`, planID, adminID, accountID))
}

func (s *Server) portalScheduleError(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, 404, "SCHEDULE_NOT_FOUND", "定时检查计划不存在")
		return
	}
	s.writeAccountHistoryError(w, err)
}

func (s *Server) portalAccountSchedule(w http.ResponseWriter, r *http.Request) {
	adminID, accountID, ok := s.portalScheduleContext(w, r)
	if !ok {
		return
	}
	planID, ok := pathID(w, r, "planID")
	if !ok {
		return
	}
	if r.Method == http.MethodDelete {
		var id int64
		err := s.portalDB.QueryRowContext(r.Context(), `DELETE FROM portal_account_check_schedules WHERE id=$1 AND admin_id=$2 AND account_id=$3 AND (running_until IS NULL OR running_until<=now()) RETURNING id`, planID, adminID, accountID).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			if _, lookupErr := loadOwnedPortalSchedule(r.Context(), s.portalDB, adminID, accountID, planID); lookupErr == nil {
				writeAPIError(w, 409, "SCHEDULE_RUNNING", "该计划正在检查，请先暂停并等待本次完成后删除")
				return
			}
		}
		if err != nil {
			s.portalScheduleError(w, err)
			return
		}
		writeSuccess(w, map[string]bool{"deleted": true})
		return
	}
	var input portalScheduleInput
	r.Body = http.MaxBytesReader(w, r.Body, 128*1024)
	if decodeJSON(r, &input) != nil {
		writeAPIError(w, 400, "INVALID_SCHEDULE", "定时检查参数无效")
		return
	}
	// Lock through the update so a concurrent pause cannot be lost to an edit.
	tx, err := s.portalDB.BeginTx(r.Context(), nil)
	if err != nil {
		s.writeAccountHistoryError(w, err)
		return
	}
	defer tx.Rollback()
	plan, err := scanPortalSchedule(tx.QueryRowContext(r.Context(), `SELECT `+portalScheduleColumns+` FROM portal_account_check_schedules WHERE id=$1 AND admin_id=$2 AND account_id=$3 FOR UPDATE`, planID, adminID, accountID))
	if err != nil {
		s.portalScheduleError(w, err)
		return
	}
	if err = applyPortalScheduleInput(&plan, input); err != nil {
		writeAPIError(w, 400, "INVALID_SCHEDULE", err.Error())
		return
	}
	plan, err = scanPortalSchedule(tx.QueryRowContext(r.Context(), `UPDATE portal_account_check_schedules SET model_id=$1,reasoning_effort=$2,prompt=$3,cron_expression=$4,enabled=$5,next_run_at=$6,updated_at=now() WHERE id=$7 RETURNING `+portalScheduleColumns, plan.ModelID, plan.ReasoningEffort, plan.Prompt, plan.CronExpression, plan.Enabled, plan.NextRunAt, planID))
	if err != nil {
		s.writeAccountHistoryError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		s.writeAccountHistoryError(w, err)
		return
	}
	writeSuccess(w, plan)
}

type portalScheduleResult struct {
	ID           int64      `json:"id"`
	PlanID       int64      `json:"plan_id"`
	Status       string     `json:"status"`
	ResponseText string     `json:"response_text"`
	HTML         string     `json:"html"`
	ErrorMessage string     `json:"error_message"`
	LatencyMs    int64      `json:"latency_ms"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

func (s *Server) portalAccountScheduleResults(w http.ResponseWriter, r *http.Request) {
	adminID, accountID, ok := s.portalScheduleContext(w, r)
	if !ok {
		return
	}
	planID, ok := pathID(w, r, "planID")
	if !ok {
		return
	}
	if _, err := loadOwnedPortalSchedule(r.Context(), s.portalDB, adminID, accountID, planID); err != nil {
		s.portalScheduleError(w, err)
		return
	}
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "1"
	}
	if limit != "1" && limit != "3" {
		writeAPIError(w, 400, "INVALID_LIMIT", "limit 必须为 1 或 3")
		return
	}
	rows, err := s.portalDB.QueryContext(r.Context(), `SELECT id,plan_id,status,response_text,html,error_message,latency_ms,started_at,finished_at,created_at FROM portal_account_check_schedule_results WHERE plan_id=$1 ORDER BY id DESC LIMIT `+limit, planID)
	if err != nil {
		s.writeAccountHistoryError(w, err)
		return
	}
	defer rows.Close()
	results := make([]portalScheduleResult, 0)
	for rows.Next() {
		var result portalScheduleResult
		if err = rows.Scan(&result.ID, &result.PlanID, &result.Status, &result.ResponseText, &result.HTML, &result.ErrorMessage, &result.LatencyMs, &result.StartedAt, &result.FinishedAt, &result.CreatedAt); err != nil {
			s.writeAccountHistoryError(w, err)
			return
		}
		results = append(results, result)
	}
	if err = rows.Err(); err != nil {
		s.writeAccountHistoryError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeSuccess(w, results)
}

func scheduledRequestID(planID, resultID int64) string {
	return fmt.Sprintf("portal-schedule-%d-%d", planID, resultID)
}
