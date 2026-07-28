package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var ErrPoolEmpty = errors.New("code pool is empty")

const claimCooldown = time.Minute

type ClaimCooldownError struct {
	RetryAfter time.Duration
}

func (e *ClaimCooldownError) Error() string { return "device is in claim cooldown" }

type Store struct {
	db *sql.DB
}

type Claim struct {
	Email     string    `json:"email"`
	Code      string    `json:"code"`
	ClaimedAt time.Time `json:"claimedAt"`
}

type Stats struct {
	Total     int `json:"total"`
	Claimed   int `json:"claimed"`
	Remaining int `json:"remaining"`
}

func OpenStore(path string) (*Store, error) {
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(8)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS codes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			code TEXT NOT NULL UNIQUE,
			created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
		);
		CREATE TABLE IF NOT EXISTS claims (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL COLLATE NOCASE,
			code_id INTEGER NOT NULL UNIQUE,
			claimed_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
			FOREIGN KEY (code_id) REFERENCES codes(id)
		);
		CREATE INDEX IF NOT EXISTS idx_claims_claimed_at ON claims(claimed_at DESC);
	`); err != nil {
		return err
	}

	// Older deployments used email as the primary key. Rebuild the table in
	// place so previous claims remain visible while repeat claims are allowed.
	rows, err := s.db.Query(`PRAGMA table_info(claims)`)
	if err != nil {
		return err
	}
	legacySchema := false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, dataType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return err
		}
		if name == "email" && primaryKey > 0 {
			legacySchema = true
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if !legacySchema {
		return nil
	}

	_, err = s.db.Exec(`
		ALTER TABLE claims RENAME TO claims_legacy;
		CREATE TABLE claims (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL COLLATE NOCASE,
			code_id INTEGER NOT NULL UNIQUE,
			claimed_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
			FOREIGN KEY (code_id) REFERENCES codes(id)
		);
		INSERT INTO claims(email, code_id, claimed_at)
		SELECT email, code_id, claimed_at FROM claims_legacy;
		DROP TABLE claims_legacy;
		CREATE INDEX idx_claims_claimed_at ON claims(claimed_at DESC);
	`)
	return err
}

func (s *Store) ImportCodes(ctx context.Context, codes []string) (added, duplicates int, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	seen := make(map[string]struct{}, len(codes))
	for _, raw := range codes {
		code := strings.TrimSpace(raw)
		if code == "" {
			continue
		}
		if len(code) > 512 {
			return 0, 0, fmt.Errorf("兑换码长度不能超过 512 个字符")
		}
		if _, ok := seen[code]; ok {
			duplicates++
			continue
		}
		seen[code] = struct{}{}

		result, execErr := tx.ExecContext(ctx, "INSERT OR IGNORE INTO codes(code) VALUES (?)", code)
		if execErr != nil {
			return 0, 0, execErr
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return 0, 0, rowsErr
		}
		if rows == 1 {
			added++
		} else {
			duplicates++
		}
	}
	if added == 0 && duplicates == 0 {
		return 0, 0, errors.New("没有找到可导入的兑换码")
	}
	return added, duplicates, tx.Commit()
}

func (s *Store) ClaimRandom(ctx context.Context, email string) (Claim, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Claim{}, false, err
	}
	defer tx.Rollback()

	var lastClaimedAt time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT claimed_at FROM claims
		WHERE email = ? COLLATE NOCASE
		ORDER BY claimed_at DESC LIMIT 1
	`, email).Scan(&lastClaimedAt)
	if err == nil {
		retryAfter := time.Until(lastClaimedAt.Add(claimCooldown))
		if retryAfter > 0 {
			return Claim{}, false, &ClaimCooldownError{RetryAfter: retryAfter}
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Claim{}, false, err
	}

	var codeID int64
	var code string
	err = tx.QueryRowContext(ctx, `
		SELECT codes.id, codes.code
		FROM codes
		LEFT JOIN claims ON claims.code_id = codes.id
		WHERE claims.code_id IS NULL
		ORDER BY RANDOM()
		LIMIT 1
	`).Scan(&codeID, &code)
	if errors.Is(err, sql.ErrNoRows) {
		return Claim{}, false, ErrPoolEmpty
	}
	if err != nil {
		return Claim{}, false, err
	}

	claimedAt := time.Now().UTC().Truncate(time.Millisecond)
	_, err = tx.ExecContext(ctx, "INSERT INTO claims(email, code_id, claimed_at) VALUES (?, ?, ?)", email, codeID, claimedAt)
	if err != nil {
		return Claim{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Claim{}, false, err
	}
	return Claim{Email: email, Code: code, ClaimedAt: claimedAt}, false, nil
}

func (s *Store) Stats(ctx context.Context) (Stats, error) {
	var stats Stats
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COUNT(claims.code_id), COUNT(*) - COUNT(claims.code_id)
		FROM codes LEFT JOIN claims ON claims.code_id = codes.id
	`).Scan(&stats.Total, &stats.Claimed, &stats.Remaining)
	return stats, err
}

func (s *Store) Claims(ctx context.Context, query string, limit, offset int) ([]Claim, int, error) {
	pattern := "%" + strings.TrimSpace(query) + "%"
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM claims WHERE email LIKE ? OR EXISTS (SELECT 1 FROM codes WHERE codes.id = claims.code_id AND codes.code LIKE ?)", pattern, pattern).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT claims.email, codes.code, claims.claimed_at
		FROM claims JOIN codes ON codes.id = claims.code_id
		WHERE claims.email LIKE ? OR codes.code LIKE ?
		ORDER BY claims.claimed_at DESC
		LIMIT ? OFFSET ?
	`, pattern, pattern, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	claims := make([]Claim, 0)
	for rows.Next() {
		var claim Claim
		if err := rows.Scan(&claim.Email, &claim.Code, &claim.ClaimedAt); err != nil {
			return nil, 0, err
		}
		claims = append(claims, claim)
	}
	return claims, total, rows.Err()
}
