# Imgtool Web Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first runnable foundation for the Vite React + Go Imgtool web app: project scaffold, backend config/database/auth/session basics, and frontend login shell.

**Architecture:** The project is split into `web/` for the Vite React SPA and `server/` for the Go API/static server. The first increment establishes secure configuration loading, SQLite migrations, Argon2id password hashing, session cookies, `/api/auth/login`, `/api/auth/logout`, `/api/me`, and a minimal frontend login/app shell that uses those APIs.

**Tech Stack:** Go, `net/http`, `modernc.org/sqlite`, `golang.org/x/crypto/argon2`, Vite, React, TypeScript, Vitest.

---

## File Structure

- Create `server/go.mod` and `server/go.sum` for Go dependencies.
- Create `server/cmd/imgtool/main.go` for server startup.
- Create `server/internal/config/config.go` and `server/internal/config/config_test.go` for environment configuration.
- Create `server/internal/crypto/password.go` and `server/internal/crypto/password_test.go` for Argon2id password hashing.
- Create `server/internal/db/db.go`, `server/internal/db/migrations.go`, and `server/internal/db/db_test.go` for SQLite setup.
- Create `server/internal/auth/store.go`, `server/internal/auth/service.go`, `server/internal/auth/http.go`, and `server/internal/auth/auth_test.go` for user/session logic and auth endpoints.
- Create `server/internal/httpapi/response.go` and `server/internal/httpapi/server.go` for API envelope and routing.
- Create `web/package.json`, `web/index.html`, `web/vite.config.ts`, `web/tsconfig.json`, and `web/tsconfig.node.json`.
- Create `web/src/main.tsx`, `web/src/App.tsx`, `web/src/api.ts`, `web/src/auth.tsx`, `web/src/styles.css`, and `web/src/App.test.tsx`.
- Create root `.env.example` with non-secret placeholders.

## Task 1: Go Module And Config

**Files:**
- Create: `server/go.mod`
- Create: `server/internal/config/config.go`
- Test: `server/internal/config/config_test.go`

- [ ] **Step 1: Write failing config tests**

```go
package config

import "testing"

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("IMGTOOL_HTTP_ADDR", "")
	t.Setenv("IMGTOOL_DATABASE_PATH", "")
	t.Setenv("IMGTOOL_ENV", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.HTTPAddr != "127.0.0.1:8080" {
		t.Fatalf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if cfg.DatabasePath != "./data/imgtool.sqlite" {
		t.Fatalf("DatabasePath = %q", cfg.DatabasePath)
	}
	if cfg.Env != "development" {
		t.Fatalf("Env = %q", cfg.Env)
	}
}

func TestLoadRequiresSecret(t *testing.T) {
	t.Setenv("IMGTOOL_SECRET", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected missing secret error")
	}
}
```

- [ ] **Step 2: Run config tests and verify they fail**

Run: `cd server && go test ./internal/config`

Expected: FAIL because `Load` and `Config` are not implemented.

- [ ] **Step 3: Implement minimal config loader**

```go
package config

import (
	"errors"
	"os"
	"strconv"
)

type Config struct {
	Env                 string
	HTTPAddr            string
	DatabasePath        string
	Secret              string
	AdminUsername       string
	AdminPassword       string
	StorageProvider     string
	OSSBucket           string
	OSSEndpoint         string
	OSSPrefix           string
	OSSSignedURLExpires int
	OSSAccessKeyID      string
	OSSAccessKeySecret  string
}

func Load() (Config, error) {
	cfg := Config{
		Env:                 getenv("IMGTOOL_ENV", "development"),
		HTTPAddr:            getenv("IMGTOOL_HTTP_ADDR", "127.0.0.1:8080"),
		DatabasePath:        getenv("IMGTOOL_DATABASE_PATH", "./data/imgtool.sqlite"),
		Secret:              os.Getenv("IMGTOOL_SECRET"),
		AdminUsername:       getenv("IMGTOOL_ADMIN_USERNAME", "admin"),
		AdminPassword:       os.Getenv("IMGTOOL_ADMIN_PASSWORD"),
		StorageProvider:     getenv("IMGTOOL_STORAGE_PROVIDER", "aliyun-oss"),
		OSSBucket:           os.Getenv("ALIYUN_OSS_BUCKET"),
		OSSEndpoint:         os.Getenv("ALIYUN_OSS_ENDPOINT"),
		OSSPrefix:           getenv("ALIYUN_OSS_PREFIX", "aiImg"),
		OSSSignedURLExpires: getenvInt("ALIYUN_OSS_SIGNED_URL_EXPIRES", 600),
		OSSAccessKeyID:      os.Getenv("ALIYUN_OSS_ACCESS_KEY_ID"),
		OSSAccessKeySecret:  os.Getenv("ALIYUN_OSS_ACCESS_KEY_SECRET"),
	}
	if cfg.Secret == "" {
		return Config{}, errors.New("IMGTOOL_SECRET is required")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
```

- [ ] **Step 4: Run config tests and verify they pass**

Run: `cd server && go test ./internal/config`

Expected: PASS.

## Task 2: Password Hashing

**Files:**
- Create: `server/internal/crypto/password.go`
- Test: `server/internal/crypto/password_test.go`

- [ ] **Step 1: Write failing password tests**

```go
package crypto

import "testing"

func TestPasswordHashVerifies(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if hash == "correct horse battery staple" {
		t.Fatal("hash must not equal plaintext")
	}
	if !VerifyPassword(hash, "correct horse battery staple") {
		t.Fatal("expected password to verify")
	}
	if VerifyPassword(hash, "wrong") {
		t.Fatal("wrong password verified")
	}
}
```

- [ ] **Step 2: Run password tests and verify they fail**

Run: `cd server && go test ./internal/crypto`

Expected: FAIL because password functions do not exist.

- [ ] **Step 3: Implement Argon2id password hashing**

Use encoded hashes shaped like:

```text
$argon2id$v=19$m=65536,t=3,p=1$base64salt$base64hash
```

- [ ] **Step 4: Run password tests and verify they pass**

Run: `cd server && go test ./internal/crypto`

Expected: PASS.

## Task 3: SQLite Migrations

**Files:**
- Create: `server/internal/db/db.go`
- Create: `server/internal/db/migrations.go`
- Test: `server/internal/db/db_test.go`

- [ ] **Step 1: Write failing migration test**

```go
package db

import (
	"database/sql"
	"testing"
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

func tableExists(t *testing.T, database *sql.DB, name string) bool {
	t.Helper()
	var found string
	err := database.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&found)
	return err == nil && found == name
}
```

- [ ] **Step 2: Run migration test and verify it fails**

Run: `cd server && go test ./internal/db`

Expected: FAIL because `Open` is not implemented.

- [ ] **Step 3: Implement SQLite open and migrations**

Use `database/sql` with `modernc.org/sqlite`. Ensure parent directory creation, WAL mode, foreign keys, and the core tables from the spec.

- [ ] **Step 4: Run migration test and verify it passes**

Run: `cd server && go test ./internal/db`

Expected: PASS.

## Task 4: Auth Store And Service

**Files:**
- Create: `server/internal/auth/store.go`
- Create: `server/internal/auth/service.go`
- Test: `server/internal/auth/auth_test.go`

- [ ] **Step 1: Write failing auth service tests**

Cover:

- Bootstrap creates admin when no users exist.
- Login accepts correct password.
- Login rejects wrong password.
- Disabled users cannot log in.
- Session lookup returns the user.

- [ ] **Step 2: Run auth tests and verify they fail**

Run: `cd server && go test ./internal/auth`

Expected: FAIL because auth package is not implemented.

- [ ] **Step 3: Implement auth store and service**

Implement user creation, admin bootstrap, password verification, session creation, session lookup, logout, and disabled-user checks.

- [ ] **Step 4: Run auth tests and verify they pass**

Run: `cd server && go test ./internal/auth`

Expected: PASS.

## Task 5: Auth HTTP API

**Files:**
- Create: `server/internal/httpapi/response.go`
- Create: `server/internal/httpapi/server.go`
- Create: `server/internal/auth/http.go`
- Modify: `server/cmd/imgtool/main.go`
- Test: `server/internal/auth/http_test.go`

- [ ] **Step 1: Write failing HTTP tests**

Cover:

- `POST /api/auth/login` sets an HttpOnly cookie.
- `GET /api/me` returns current user when cookie is present.
- `POST /api/auth/logout` clears the cookie and deletes the session.
- `GET /api/me` returns 401 when unauthenticated.

- [ ] **Step 2: Run HTTP tests and verify they fail**

Run: `cd server && go test ./internal/auth`

Expected: FAIL because HTTP handlers are not implemented.

- [ ] **Step 3: Implement API envelope and auth routes**

Use `net/http`. Return `{ "ok": true, "data": ... }` and `{ "ok": false, "error": ... }`.

- [ ] **Step 4: Run HTTP tests and verify they pass**

Run: `cd server && go test ./internal/auth ./internal/httpapi`

Expected: PASS.

## Task 6: Vite React Scaffold And Login Shell

**Files:**
- Create: `web/package.json`
- Create: `web/index.html`
- Create: `web/vite.config.ts`
- Create: `web/tsconfig.json`
- Create: `web/tsconfig.node.json`
- Create: `web/src/main.tsx`
- Create: `web/src/App.tsx`
- Create: `web/src/api.ts`
- Create: `web/src/auth.tsx`
- Create: `web/src/styles.css`
- Test: `web/src/App.test.tsx`

- [ ] **Step 1: Write failing frontend tests**

Cover:

- Login form renders when `/api/me` returns 401.
- Submitting credentials calls `/api/auth/login`.
- App shell renders after `/api/me` returns a user.

- [ ] **Step 2: Run frontend tests and verify they fail**

Run: `cd web && npm test -- --run`

Expected: FAIL because Vite app does not exist.

- [ ] **Step 3: Implement minimal Vite React app**

Implement API client, auth provider, login form, and placeholder authenticated shell with links for workspace/settings/history/admin.

- [ ] **Step 4: Run frontend tests and verify they pass**

Run: `cd web && npm test -- --run`

Expected: PASS.

## Task 7: Full Verification

**Files:**
- Modify only if verification exposes issues.

- [ ] **Step 1: Run backend tests**

Run: `cd server && go test ./...`

Expected: PASS.

- [ ] **Step 2: Run frontend tests**

Run: `cd web && npm test -- --run`

Expected: PASS.

- [ ] **Step 3: Run build checks**

Run:

```bash
cd web && npm run build
cd ../server && go build ./cmd/imgtool
```

Expected: both commands exit 0.
