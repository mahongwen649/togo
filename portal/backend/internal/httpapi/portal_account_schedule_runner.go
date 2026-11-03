package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

type scheduledAccountCore interface {
	AdminMarketRequest(context.Context, string, string, any, string) (coreclient.Response, error)
	ScheduledAccountPromptRequest(context.Context, int64, string, string, string, string) (coreclient.Response, error)
}

type AccountScheduleRunner struct {
	db      *sql.DB
	core    scheduledAccountCore
	logger  *slog.Logger
	mu      sync.Mutex
	closing bool
	wg      sync.WaitGroup
}

func NewAccountScheduleRunner(db *sql.DB, core scheduledAccountCore, logger *slog.Logger) *AccountScheduleRunner {
	return &AccountScheduleRunner{db: db, core: core, logger: logger}
}

func (p *AccountScheduleRunner) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		if err := p.tick(ctx); err != nil && ctx.Err() == nil {
			p.logger.Error("account schedule tick failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (p *AccountScheduleRunner) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	p.closing = true
	p.mu.Unlock()
	done := make(chan struct{})
	go func() { p.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type claimedAccountSchedule struct {
	plan     portalAccountSchedule
	resultID int64
}

func claimAccountSchedules(ctx context.Context, db *sql.DB) ([]claimedAccountSchedule, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var locked bool
	if err = tx.QueryRowContext(ctx, `SELECT pg_try_advisory_xact_lock(734210095)`).Scan(&locked); err != nil {
		return nil, err
	}
	if !locked {
		return nil, nil
	}
	// A killed process leaves a visible failure after its bounded lease expires.
	if _, err = tx.ExecContext(ctx, `UPDATE portal_account_check_schedule_results SET status='failed',error_message='定时检查进程中断或超时',finished_at=now() WHERE status='running' AND started_at < now()-interval '6 minutes'`); err != nil {
		return nil, err
	}
	var active int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM portal_account_check_schedules WHERE running_until>now()`).Scan(&active); err != nil {
		return nil, err
	}
	jobs := make([]claimedAccountSchedule, 0)
	for len(jobs)+active < 4 {
		// Serialize per account across plans and instances, while unrelated accounts
		// can execute concurrently. Next execution is reserved before starting I/O.
		plan, err := scanPortalSchedule(tx.QueryRowContext(ctx, `SELECT `+portalScheduleColumns+` FROM portal_account_check_schedules s WHERE enabled AND next_run_at<=now() AND (running_until IS NULL OR running_until<=now()) AND NOT EXISTS(SELECT 1 FROM portal_account_check_schedules busy WHERE busy.account_id=s.account_id AND busy.running_until>now()) ORDER BY next_run_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`))
		if errors.Is(err, sql.ErrNoRows) {
			break
		}
		if err != nil {
			return nil, err
		}
		next, err := nextAccountSchedule(plan.CronExpression, time.Now())
		if err != nil {
			if _, err = tx.ExecContext(ctx, `UPDATE portal_account_check_schedules SET enabled=false,updated_at=now() WHERE id=$1`, plan.ID); err != nil {
				return nil, err
			}
			continue
		}
		if err = tx.QueryRowContext(ctx, `UPDATE portal_account_check_schedules SET last_run_at=now(),next_run_at=$1,running_until=now()+interval '6 minutes' WHERE id=$2 RETURNING running_until`, next, plan.ID).Scan(&plan.RunningUntil); err != nil {
			return nil, err
		}
		var resultID int64
		if err = tx.QueryRowContext(ctx, `INSERT INTO portal_account_check_schedule_results(plan_id,status) VALUES($1,'running') RETURNING id`, plan.ID).Scan(&resultID); err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM portal_account_check_schedule_results WHERE plan_id=$1 AND id NOT IN(SELECT id FROM portal_account_check_schedule_results WHERE plan_id=$1 ORDER BY id DESC LIMIT 3)`, plan.ID); err != nil {
			return nil, err
		}
		jobs = append(jobs, claimedAccountSchedule{plan: plan, resultID: resultID})
	}
	return jobs, tx.Commit()
}

func (p *AccountScheduleRunner) tick(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closing || p.db == nil || p.core == nil {
		return nil
	}
	claimCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	jobs, err := claimAccountSchedules(claimCtx, p.db)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		p.wg.Add(1)
		go func(job claimedAccountSchedule) {
			defer p.wg.Done()
			jobCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			started := time.Now()
			result := performScheduledAccountCheck(jobCtx, p.core, job.plan, job.resultID)
			storeCtx, storeCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer storeCancel()
			if err := finishAccountSchedule(storeCtx, p.db, job, result, time.Since(started).Milliseconds()); err != nil {
				p.logger.Error("save account schedule result failed", "plan_id", job.plan.ID, "error", err)
			}
		}(job)
	}
	return nil
}

func performScheduledAccountCheck(ctx context.Context, core scheduledAccountCore, plan portalAccountSchedule, resultID int64) accountCheckResult {
	// Revoked administrator privileges stop existing plans from using the
	// service credential. These are explicit background tasks, never a fallback
	// after a failed interactive JWT request.
	response, err := core.AdminMarketRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v1/admin/users/%d", plan.AdminID), nil, "")
	var owner struct {
		Code int             `json:"code"`
		Data coreclient.User `json:"data"`
	}
	if err != nil || response.StatusCode != 200 || json.Unmarshal(response.Body, &owner) != nil || owner.Code != 0 || owner.Data.ID != plan.AdminID || owner.Data.Role != "admin" || owner.Data.Status != "active" {
		return accountCheckResult{Error: "创建计划的管理员权限已失效，请检查管理员状态"}
	}
	response, err = core.ScheduledAccountPromptRequest(ctx, plan.AccountID, plan.ModelID, plan.Prompt, scheduledRequestID(plan.ID, resultID), plan.ReasoningEffort)
	if err != nil {
		return accountCheckResult{Model: plan.ModelID, Error: err.Error()}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return accountCheckResult{Model: plan.ModelID, Error: "定时检查请求失败"}
	}
	return parseAccountCheck(response.Body)
}

func finishAccountSchedule(ctx context.Context, db *sql.DB, job claimedAccountSchedule, result accountCheckResult, latency int64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	status := "success"
	if result.Error != "" {
		status = "failed"
	}
	if _, err = tx.ExecContext(ctx, `UPDATE portal_account_check_schedule_results SET status=$1,response_text=$2,html=$3,error_message=$4,latency_ms=$5,finished_at=now() WHERE id=$6 AND status='running'`, status, result.Text, result.HTML, result.Error, latency, job.resultID); err != nil {
		return err
	}
	// Compare the lease so an expired attempt cannot release a newer run.
	if _, err = tx.ExecContext(ctx, `UPDATE portal_account_check_schedules SET running_until=NULL WHERE id=$1 AND running_until=$2`, job.plan.ID, job.plan.RunningUntil); err != nil {
		return err
	}
	return tx.Commit()
}
