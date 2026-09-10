package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestOpenMigratesCoreTables(t *testing.T) {
	database, err := Open(t.TempDir() + "/imgtool.sqlite")
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	for _, table := range []string{"users", "sessions", "channels", "channel_models", "tasks", "history_records", "result_files"} {
		if !tableExists(t, database, table) {
			t.Fatalf("expected table %s to exist", table)
		}
	}
}

func TestOpenMigratesLegacyUsersTableBeforeCreatingExternalIdentityIndex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "imgtool.sqlite")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	_, err = legacy.Exec(`
CREATE TABLE users (
  id TEXT PRIMARY KEY,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL CHECK (role IN ('admin', 'user')),
  disabled_at INTEGER,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);`)
	if err != nil {
		t.Fatalf("create legacy users table: %v", err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}

	database, err := Open(path)
	if err != nil {
		t.Fatalf("Open returned error for legacy database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	for _, column := range []string{"external_provider", "external_subject"} {
		if !columnExists(t, database, "users", column) {
			t.Fatalf("expected users.%s to exist", column)
		}
	}
	if !indexExists(t, database, "idx_users_external_identity") {
		t.Fatal("expected external identity index to exist")
	}
}

func tableExists(t *testing.T, database *sql.DB, name string) bool {
	t.Helper()
	var found string
	err := database.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&found)
	return err == nil && found == name
}

func columnExists(t *testing.T, database *sql.DB, table string, column string) bool {
	t.Helper()
	rows, err := database.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err == nil && name == column {
			return true
		}
	}
	return false
}

func indexExists(t *testing.T, database *sql.DB, name string) bool {
	t.Helper()
	var found string
	err := database.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'index' AND name = ?`, name).Scan(&found)
	return err == nil && found == name
}
