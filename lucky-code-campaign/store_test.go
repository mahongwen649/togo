package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestImportAndClaim(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	added, duplicates, err := store.ImportCodes(ctx, []string{"CODE-A", "CODE-B", "CODE-A", " "})
	if err != nil {
		t.Fatalf("ImportCodes: %v", err)
	}
	if added != 2 || duplicates != 1 {
		t.Fatalf("ImportCodes counts = %d, %d; want 2, 1", added, duplicates)
	}

	first, existing, err := store.ClaimRandom(ctx, "first@example.com")
	if err != nil || existing {
		t.Fatalf("first claim = %+v, %v, %v", first, existing, err)
	}
	repeated, existing, err := store.ClaimRandom(ctx, "FIRST@example.com")
	var cooldownErr *ClaimCooldownError
	if !errors.As(err, &cooldownErr) || existing || repeated.Code != "" {
		t.Fatalf("immediate repeat = %+v, %v, %v; first = %+v", repeated, existing, err, first)
	}
	if _, err := store.db.Exec("UPDATE claims SET claimed_at = ?", time.Now().Add(-claimCooldown-time.Minute)); err != nil {
		t.Fatal(err)
	}
	repeated, existing, err = store.ClaimRandom(ctx, "FIRST@example.com")
	if err != nil || existing || repeated.Code == first.Code {
		t.Fatalf("repeat after cooldown = %+v, %v, %v; first = %+v", repeated, existing, err, first)
	}

	_, _, err = store.ClaimRandom(ctx, "third@example.com")
	if !errors.Is(err, ErrPoolEmpty) {
		t.Fatalf("empty pool error = %v; want ErrPoolEmpty", err)
	}

	stats, err := store.Stats(ctx)
	if err != nil || stats != (Stats{Total: 2, Claimed: 2, Remaining: 0}) {
		t.Fatalf("Stats = %+v, %v", stats, err)
	}
}

func TestLegacySchemaMigrationAllowsRepeatClaims(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE codes (id INTEGER PRIMARY KEY AUTOINCREMENT, code TEXT NOT NULL UNIQUE, created_at DATETIME NOT NULL);
		CREATE TABLE claims (email TEXT NOT NULL PRIMARY KEY COLLATE NOCASE, code_id INTEGER NOT NULL UNIQUE, claimed_at DATETIME NOT NULL, FOREIGN KEY (code_id) REFERENCES codes(id));
		INSERT INTO codes(code, created_at) VALUES ('OLD-CODE', CURRENT_TIMESTAMP), ('NEW-CODE', CURRENT_TIMESTAMP);
		INSERT INTO claims(email, code_id, claimed_at) VALUES ('repeat@example.com', 1, datetime('now', '-11 minutes'));
	`)
	if closeErr := db.Close(); err != nil || closeErr != nil {
		t.Fatalf("create legacy database: %v; close: %v", err, closeErr)
	}

	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("migrate legacy database: %v", err)
	}
	defer store.Close()
	claim, existing, err := store.ClaimRandom(context.Background(), "repeat@example.com")
	if err != nil || existing || claim.Code != "NEW-CODE" {
		t.Fatalf("claim after migration = %+v, %v, %v", claim, existing, err)
	}
}

func TestConcurrentClaimsAreUnique(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	const count = 24
	codes := make([]string, count)
	for i := range codes {
		codes[i] = fmt.Sprintf("CODE-%02d", i)
	}
	if _, _, err := store.ImportCodes(ctx, codes); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	results := make(chan Claim, count)
	errs := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			claim, _, err := store.ClaimRandom(ctx, fmt.Sprintf("user%02d@example.com", i))
			if err != nil {
				errs <- err
				return
			}
			results <- claim
		}(i)
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Errorf("concurrent claim: %v", err)
	}

	seen := make(map[string]bool, count)
	for claim := range results {
		if seen[claim.Code] {
			t.Errorf("duplicate code allocated: %s", claim.Code)
		}
		seen[claim.Code] = true
	}
	if len(seen) != count {
		t.Fatalf("received %d unique codes; want %d", len(seen), count)
	}
}
