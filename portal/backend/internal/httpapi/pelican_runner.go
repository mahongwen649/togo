package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sync"
	"time"
)

const pelicanRunTimeout = pelicanMaxAttempts*pelicanAttemptTimeout + time.Minute

// PelicanRunner owns both manual and scheduled jobs, including shutdown draining.
type PelicanRunner struct {
	db      *sql.DB
	core    pelicanCore
	logger  *slog.Logger
	mu      sync.Mutex
	closing bool
	wg      sync.WaitGroup
}

func NewPelicanRunner(db *sql.DB, core pelicanCore, logger *slog.Logger) *PelicanRunner {
	return &PelicanRunner{db: db, core: core, logger: logger}
}

func (p *PelicanRunner) Run(ctx context.Context) {
	p.logger.Info("pelican scheduler started", "interval", "30m", "poll", "5s")
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		tickCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		_, err := p.start(tickCtx, false)
		cancel()
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, errPelicanCheckInProgress) {
			p.logger.Error("pelican scheduler tick failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (p *PelicanRunner) Shutdown(ctx context.Context) error {
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

func (p *PelicanRunner) start(ctx context.Context, manual bool) (int64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closing {
		return 0, errors.New("检查服务正在重启，请稍后再试")
	}
	if p.db == nil || p.core == nil {
		return 0, errors.New("检查服务未配置")
	}
	// Transaction-scoped locking cannot leak through the connection pool, and
	// this lock is separate from the exclusive-channel reclamation lock.
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var locked bool
	if err = tx.QueryRowContext(ctx, `SELECT pg_try_advisory_xact_lock($1)`, pelicanCheckLockID).Scan(&locked); err != nil {
		return 0, err
	}
	if !locked {
		return 0, errPelicanCheckInProgress
	}
	if !manual {
		if _, err = tx.ExecContext(ctx, `UPDATE portal_pelican_check_config SET scheduler_seen_at=CURRENT_TIMESTAMP WHERE id=1`); err != nil {
			return 0, err
		}
	}
	config, err := loadPelicanConfig(ctx, tx)
	if err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE portal_pelican_check_runs SET state='failed',finished_at=CURRENT_TIMESTAMP,error='检查进程中断或超过运行时限',progress='' WHERE state='running' AND started_at < CURRENT_TIMESTAMP - ($1 * INTERVAL '1 second')`, int((pelicanRunTimeout+time.Minute)/time.Second)); err != nil {
		return 0, err
	}
	if !config.Enabled || config.GroupID == nil || config.APIKeyID == nil {
		if manual {
			return 0, errors.New("请先保存配置并开启检测")
		}
		return 0, tx.Commit()
	}
	var active bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM portal_pelican_check_runs WHERE state='running')`).Scan(&active); err != nil {
		return 0, err
	}
	if active {
		if err = tx.Commit(); err != nil {
			return 0, err
		}
		return 0, errPelicanCheckInProgress
	}
	if !manual && config.NextRunAt != nil && time.Now().Before(*config.NextRunAt) {
		return 0, tx.Commit()
	}
	var runID int64
	if err = tx.QueryRowContext(ctx, `INSERT INTO portal_pelican_check_runs(group_id,group_name,api_key_id,api_key_name,state,progress) VALUES($1,$2,$3,$4,'running','正在校验检查用 Key') RETURNING id`, *config.GroupID, config.GroupName, *config.APIKeyID, config.APIKeyName).Scan(&runID); err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE portal_pelican_check_config SET next_run_at=CURRENT_TIMESTAMP + INTERVAL '30 minutes' WHERE id=1`); err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		// Accepted jobs outlive HTTP disconnects and scheduler cancellation.
		// Shutdown waits for them before the database is closed.
		jobCtx, cancel := context.WithTimeout(context.Background(), pelicanRunTimeout)
		defer cancel()
		p.logger.Info("pelican check started", "run_id", runID, "manual", manual)
		keyCtx, keyCancel := context.WithTimeout(jobCtx, 15*time.Second)
		key, keyErr := findPelicanCoreKey(keyCtx, p.core, *config.GroupID, *config.APIKeyID)
		keyCancel()
		if err := executePelicanCheck(jobCtx, p.db, p.core, p.logger, config, runID, key, keyErr); err != nil {
			p.logger.Warn("pelican check finished with error", "run_id", runID, "error", err)
		} else {
			p.logger.Info("pelican check succeeded", "run_id", runID)
		}
	}()
	return runID, nil
}

type pelicanConfigReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func loadPelicanConfig(ctx context.Context, db pelicanConfigReader) (pelicanCheckConfig, error) {
	var config pelicanCheckConfig
	err := db.QueryRowContext(ctx, `SELECT reasoning_effort,model,prompt,enabled,group_id,group_name,api_key_id,api_key_name,updated_at,next_run_at,scheduler_seen_at FROM portal_pelican_check_config WHERE id=1`).Scan(&config.ReasoningEffort, &config.Model, &config.Prompt, &config.Enabled, &config.GroupID, &config.GroupName, &config.APIKeyID, &config.APIKeyName, &config.UpdatedAt, &config.NextRunAt, &config.SchedulerSeenAt)
	return config, err
}
