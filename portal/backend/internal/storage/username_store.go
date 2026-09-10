package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jqcode/portal/backend/internal/username"
)

type UsernameStore struct {
	db *sql.DB
}

type BoundUsername struct {
	Key        string
	Username   string
	CoreUserID int64
}

func NewUsernameStore(db *sql.DB) *UsernameStore {
	return &UsernameStore{db: db}
}

func (s *UsernameStore) Reserve(ctx context.Context, entry username.Entry, now time.Time) error {
	result, err := s.db.ExecContext(ctx, `
INSERT INTO portal_username_registry (
    username_key, username, lease_id, lease_expires_at
) VALUES ($1, $2, $3, $4)
ON CONFLICT (username_key) DO UPDATE SET
    username = EXCLUDED.username,
    lease_id = EXCLUDED.lease_id,
    lease_expires_at = EXCLUDED.lease_expires_at,
    updated_at = CURRENT_TIMESTAMP
WHERE portal_username_registry.core_user_id IS NULL
  AND portal_username_registry.lease_expires_at <= $5`,
		entry.Key, entry.Username, entry.LeaseID, entry.ExpiresAt, now)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return username.ErrTaken
	}
	return nil
}

func (s *UsernameStore) Bind(ctx context.Context, key, leaseID string, coreUserID int64) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE portal_username_registry
SET core_user_id = $3,
    lease_id = NULL,
    lease_expires_at = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE username_key = $1 AND lease_id = $2 AND core_user_id IS NULL`, key, leaseID, coreUserID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return username.ErrBadLease
	}
	return nil
}

func (s *UsernameStore) SyncBound(ctx context.Context, key, display string, coreUserID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
DELETE FROM portal_username_registry
WHERE core_user_id = $1 AND username_key <> $2`, coreUserID, key); err != nil {
		return err
	}

	result, err := tx.ExecContext(ctx, `
INSERT INTO portal_username_registry (username_key, username, core_user_id)
VALUES ($1, $2, $3)
ON CONFLICT (username_key) DO UPDATE SET
    username = EXCLUDED.username,
    core_user_id = EXCLUDED.core_user_id,
    lease_id = NULL,
    lease_expires_at = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE portal_username_registry.core_user_id IS NULL
   OR portal_username_registry.core_user_id = EXCLUDED.core_user_id`, key, display, coreUserID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return username.ErrTaken
	}

	return tx.Commit()
}

func (s *UsernameStore) ReplaceBound(ctx context.Context, oldKey string, coreUserID int64, newKey, newUsername, newLeaseID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
DELETE FROM portal_username_registry
WHERE username_key = $1 AND core_user_id = $2`, oldKey, coreUserID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return username.ErrNotFound
	}

	result, err = tx.ExecContext(ctx, `
UPDATE portal_username_registry
SET core_user_id = $3,
    lease_id = NULL,
    lease_expires_at = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE username_key = $1 AND lease_id = $2 AND core_user_id IS NULL`, newKey, newLeaseID, coreUserID)
	if err != nil {
		return err
	}
	affected, err = result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return username.ErrBadLease
	}

	return tx.Commit()
}

func (s *UsernameStore) Release(ctx context.Context, key, leaseID string) error {
	_, err := s.db.ExecContext(ctx, `
DELETE FROM portal_username_registry
WHERE username_key = $1 AND lease_id = $2 AND core_user_id IS NULL`, key, leaseID)
	return err
}

func (s *UsernameStore) Resolve(ctx context.Context, key string) (int64, error) {
	var userID int64
	err := s.db.QueryRowContext(ctx, `
SELECT core_user_id
FROM portal_username_registry
WHERE username_key = $1 AND core_user_id IS NOT NULL`, key).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, username.ErrNotFound
	}
	return userID, err
}

func (s *UsernameStore) Available(ctx context.Context, key string, now time.Time) (bool, error) {
	var occupied bool
	err := s.db.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM portal_username_registry
    WHERE username_key = $1
      AND (core_user_id IS NOT NULL OR lease_expires_at > $2)
)`, key, now).Scan(&occupied)
	return !occupied, err
}

func (s *UsernameStore) DeleteBound(ctx context.Context, key string, coreUserID int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM portal_username_registry WHERE username_key=$1 AND core_user_id=$2`, key, coreUserID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return username.ErrNotFound
	}
	return nil
}

func (s *UsernameStore) ListBound(ctx context.Context) ([]BoundUsername, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT username_key, username, core_user_id
FROM portal_username_registry
WHERE core_user_id IS NOT NULL
ORDER BY username_key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []BoundUsername
	for rows.Next() {
		var item BoundUsername
		if err := rows.Scan(&item.Key, &item.Username, &item.CoreUserID); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *UsernameStore) ApplyReconciliation(ctx context.Context, imports, orphans []BoundUsername) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, item := range orphans {
		result, err := tx.ExecContext(ctx, `
DELETE FROM portal_username_registry
WHERE username_key = $1 AND core_user_id = $2`, item.Key, item.CoreUserID)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil || affected != 1 {
			return fmt.Errorf("orphan username changed during reconciliation")
		}
	}

	for _, item := range imports {
		result, err := tx.ExecContext(ctx, `
INSERT INTO portal_username_registry (username_key, username, core_user_id)
VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING`, item.Key, item.Username, item.CoreUserID)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil || affected != 1 {
			return fmt.Errorf("username conflict appeared during reconciliation")
		}
	}

	return tx.Commit()
}
