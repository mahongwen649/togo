package storage

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jqcode/portal/backend/internal/username"
)

func TestUsernameStoreReplaceBoundSwitchesUniqueCoreUserID(t *testing.T) {
	databaseURL := os.Getenv("PORTAL_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PORTAL_DATABASE_URL is not set")
	}

	ctx := context.Background()
	db, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}

	store := NewUsernameStore(db)
	now := time.Now()
	oldKey := "replace_old_" + time.Now().Format("20060102150405000000000")
	newKey := oldKey + "_new"
	coreUserID := time.Now().UnixNano()

	oldReservation := username.Entry{
		Key:       oldKey,
		Username:  oldKey,
		LeaseID:   oldKey + "_lease",
		ExpiresAt: now.Add(time.Minute),
	}
	if err := store.Reserve(ctx, oldReservation, now); err != nil {
		t.Fatal(err)
	}
	if err := store.Bind(ctx, oldReservation.Key, oldReservation.LeaseID, coreUserID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = store.DeleteBound(context.Background(), oldKey, coreUserID)
		_ = store.DeleteBound(context.Background(), newKey, coreUserID)
	})

	newReservation := username.Entry{
		Key:       newKey,
		Username:  newKey,
		LeaseID:   newKey + "_lease",
		ExpiresAt: now.Add(time.Minute),
	}
	if err := store.Reserve(ctx, newReservation, now); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceBound(ctx, oldKey, coreUserID, newKey, newKey, newReservation.LeaseID); err != nil {
		t.Fatal(err)
	}

	if _, err := store.Resolve(ctx, oldKey); err != username.ErrNotFound {
		t.Fatalf("old username resolve error = %v", err)
	}
	resolved, err := store.Resolve(ctx, newKey)
	if err != nil || resolved != coreUserID {
		t.Fatalf("new username resolve = %d, %v", resolved, err)
	}
}
