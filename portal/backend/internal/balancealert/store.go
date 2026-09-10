package balancealert

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const runnerAdvisoryLock int64 = 7467664291256172

type Observation struct {
	UserID    int64
	Email     string
	Balance   float64
	Threshold float64
	Eligible  bool
}

type state struct {
	Balance  float64
	Eligible bool
	Below    bool
	Pending  bool
}

type StateStore interface {
	TryRunLock(context.Context) (func(), bool, error)
	Observe(context.Context, Observation) (bool, error)
	MarkSent(context.Context, int64) error
	MarkFailed(context.Context, int64, error) error
	DisableAll(context.Context) error
}

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}

func (s *SQLStore) TryRunLock(ctx context.Context) (func(), bool, error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, false, err
	}
	var locked bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, runnerAdvisoryLock).Scan(&locked); err != nil {
		_ = conn.Close()
		return nil, false, err
	}
	if !locked {
		_ = conn.Close()
		return func() {}, false, nil
	}
	return func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(unlockCtx, `SELECT pg_advisory_unlock($1)`, runnerAdvisoryLock)
		_ = conn.Close()
	}, true, nil
}

func (s *SQLStore) Observe(ctx context.Context, observation Observation) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	previous := state{}
	exists := true
	err = tx.QueryRowContext(ctx, `
		SELECT last_balance, eligible, below_threshold, pending
		FROM portal_balance_alert_states
		WHERE core_user_id = $1
		FOR UPDATE
	`, observation.UserID).Scan(&previous.Balance, &previous.Eligible, &previous.Below, &previous.Pending)
	if err == sql.ErrNoRows {
		exists = false
	} else if err != nil {
		return false, err
	}

	below, pending := transition(previous, exists, observation)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO portal_balance_alert_states (
			core_user_id, email, last_balance, last_threshold, eligible,
			below_threshold, pending, last_observed_at, last_send_error
		) VALUES ($1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP, NULL)
		ON CONFLICT (core_user_id) DO UPDATE SET
			email = EXCLUDED.email,
			last_balance = EXCLUDED.last_balance,
			last_threshold = EXCLUDED.last_threshold,
			eligible = EXCLUDED.eligible,
			below_threshold = EXCLUDED.below_threshold,
			pending = EXCLUDED.pending,
			last_observed_at = CURRENT_TIMESTAMP,
			last_send_error = CASE WHEN EXCLUDED.pending THEN portal_balance_alert_states.last_send_error ELSE NULL END
	`, observation.UserID, observation.Email, observation.Balance, observation.Threshold, observation.Eligible, below, pending)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return pending, nil
}

func transition(previous state, exists bool, observation Observation) (below, pending bool) {
	below = observation.Eligible && observation.Balance < observation.Threshold
	pending = previous.Pending
	if !observation.Eligible || !below {
		return below, false
	}
	if exists && previous.Eligible && previous.Balance >= observation.Threshold && observation.Balance < previous.Balance {
		pending = true
	}
	return below, pending
}

func (s *SQLStore) MarkSent(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE portal_balance_alert_states
		SET pending = FALSE, last_sent_at = CURRENT_TIMESTAMP, last_send_error = NULL
		WHERE core_user_id = $1
	`, userID)
	return err
}

func (s *SQLStore) MarkFailed(ctx context.Context, userID int64, sendErr error) error {
	message := fmt.Sprint(sendErr)
	if len(message) > 1000 {
		message = message[:1000]
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE portal_balance_alert_states
		SET pending = TRUE, last_send_error = $2
		WHERE core_user_id = $1
	`, userID, message)
	return err
}

func (s *SQLStore) DisableAll(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE portal_balance_alert_states
		SET eligible = FALSE, pending = FALSE, last_send_error = NULL, last_observed_at = CURRENT_TIMESTAMP
		WHERE eligible = TRUE OR pending = TRUE
	`)
	return err
}
