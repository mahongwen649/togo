package username

import (
	"context"
	"sync"
	"testing"
	"time"
)

type memoryStore struct {
	mu      sync.Mutex
	entries map[string]Entry
}

func (s *memoryStore) DeleteBound(_ context.Context, key string, userID int64) error {
	entry, ok := s.entries[key]
	if !ok || entry.CoreUserID == nil || *entry.CoreUserID != userID {
		return ErrNotFound
	}
	delete(s.entries, key)
	return nil
}

func (s *memoryStore) ReplaceBound(_ context.Context, oldKey string, userID int64, newKey, newUsername, newLeaseID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	newEntry, ok := s.entries[newKey]
	if !ok || newEntry.LeaseID != newLeaseID || newEntry.CoreUserID != nil {
		return ErrBadLease
	}
	oldEntry, ok := s.entries[oldKey]
	if !ok || oldEntry.CoreUserID == nil || *oldEntry.CoreUserID != userID {
		return ErrNotFound
	}
	newEntry.Username = newUsername
	newEntry.CoreUserID = &userID
	newEntry.LeaseID = ""
	newEntry.ExpiresAt = time.Time{}
	s.entries[newKey] = newEntry
	delete(s.entries, oldKey)
	return nil
}

func newMemoryStore() *memoryStore {
	return &memoryStore{entries: make(map[string]Entry)}
}

func (s *memoryStore) Reserve(_ context.Context, entry Entry, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, exists := s.entries[entry.Key]
	if exists && (current.CoreUserID != nil || current.ExpiresAt.After(now)) {
		return ErrTaken
	}
	s.entries[entry.Key] = entry
	return nil
}

func (s *memoryStore) Bind(_ context.Context, key, leaseID string, coreUserID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, exists := s.entries[key]
	if !exists || entry.LeaseID != leaseID || entry.CoreUserID != nil {
		return ErrBadLease
	}
	entry.CoreUserID = &coreUserID
	entry.LeaseID = ""
	entry.ExpiresAt = time.Time{}
	s.entries[key] = entry
	return nil
}

func (s *memoryStore) SyncBound(_ context.Context, key, display string, coreUserID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for currentKey, entry := range s.entries {
		if entry.CoreUserID != nil && *entry.CoreUserID == coreUserID && currentKey != key {
			delete(s.entries, currentKey)
		}
	}
	entry, exists := s.entries[key]
	if exists && entry.CoreUserID != nil && *entry.CoreUserID != coreUserID {
		return ErrTaken
	}
	s.entries[key] = Entry{Key: key, Username: display, CoreUserID: &coreUserID}
	return nil
}

func (s *memoryStore) Release(_ context.Context, key, leaseID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, exists := s.entries[key]
	if exists && entry.CoreUserID == nil && entry.LeaseID == leaseID {
		delete(s.entries, key)
	}
	return nil
}

func (s *memoryStore) Resolve(_ context.Context, key string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, exists := s.entries[key]
	if !exists || entry.CoreUserID == nil {
		return 0, ErrNotFound
	}
	return *entry.CoreUserID, nil
}

func (s *memoryStore) Available(_ context.Context, key string, now time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, exists := s.entries[key]
	if !exists {
		return true, nil
	}
	return entry.CoreUserID == nil && !entry.ExpiresAt.After(now), nil
}

func TestNormalizeUsesCaseInsensitiveKey(t *testing.T) {
	display, key, err := Normalize(" User_01 ")
	if err != nil || display != "User_01" || key != "user_01" {
		t.Fatalf("normalize = %q %q %v", display, key, err)
	}
}

func TestNormalizeMatchesFrontendPolicy(t *testing.T) {
	for _, value := range []string{"a", "user@example", "用户😀", "history—name"} {
		if _, _, err := Normalize(value); err == nil {
			t.Fatalf("expected %q to be invalid", value)
		}
	}
}

func TestNormalizeExistingAllowsLegacyCharacters(t *testing.T) {
	display, key, err := NormalizeExisting(" Legacy.User@01 ")
	if err != nil {
		t.Fatalf("NormalizeExisting error = %v", err)
	}
	if display != "Legacy.User@01" || key != "legacy.user@01" {
		t.Fatalf("NormalizeExisting = %q, %q", display, key)
	}
	if _, _, err := Normalize(display); err == nil {
		t.Fatal("strict Normalize unexpectedly accepted legacy characters")
	}
}

func TestConcurrentReservationsHaveOneWinner(t *testing.T) {
	registry := New(newMemoryStore())
	const attempts = 20
	start := make(chan struct{})
	results := make(chan error, attempts)
	var wait sync.WaitGroup
	for range attempts {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := registry.Reserve(context.Background(), "SameName")
			results <- err
		}()
	}
	close(start)
	wait.Wait()
	close(results)

	winners := 0
	for err := range results {
		if err == nil {
			winners++
		} else if err != ErrTaken {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if winners != 1 {
		t.Fatalf("reservation winners = %d", winners)
	}
}

func TestBindAndResolve(t *testing.T) {
	registry := New(newMemoryStore())
	reservation, err := registry.Reserve(context.Background(), "Alice")
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Bind(context.Background(), reservation, 42); err != nil {
		t.Fatal(err)
	}
	userID, err := registry.Resolve(context.Background(), "alice")
	if err != nil || userID != 42 {
		t.Fatalf("resolve = %d %v", userID, err)
	}
}

func TestActiveReservationIsNotAvailable(t *testing.T) {
	registry := New(newMemoryStore())
	if _, err := registry.Reserve(context.Background(), "Alice"); err != nil {
		t.Fatal(err)
	}
	available, err := registry.Available(context.Background(), "alice")
	if err != nil || available {
		t.Fatalf("available = %v, err = %v", available, err)
	}
}

func TestDeleteBoundRequiresExactUser(t *testing.T) {
	registry := New(newMemoryStore())
	reservation, _ := registry.Reserve(context.Background(), "Alice")
	if err := registry.Bind(context.Background(), reservation, 42); err != nil {
		t.Fatal(err)
	}
	if err := registry.DeleteBound(context.Background(), "Alice", 41); err != ErrNotFound {
		t.Fatalf("wrong user delete error = %v", err)
	}
	if err := registry.DeleteBound(context.Background(), "alice", 42); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Resolve(context.Background(), "alice"); err != ErrNotFound {
		t.Fatalf("resolve error = %v", err)
	}
}

func TestReplaceBoundSwitchesUsernameAtomically(t *testing.T) {
	store := newMemoryStore()
	registry := New(store)
	oldReservation, _ := registry.Reserve(context.Background(), "Alice")
	if err := registry.Bind(context.Background(), oldReservation, 42); err != nil {
		t.Fatal(err)
	}
	newReservation, _ := registry.Reserve(context.Background(), "Bob")
	if err := registry.ReplaceBound(context.Background(), "Alice", newReservation, 42); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Resolve(context.Background(), "alice"); err != ErrNotFound {
		t.Fatalf("old resolve error = %v", err)
	}
	userID, err := registry.Resolve(context.Background(), "bob")
	if err != nil || userID != 42 {
		t.Fatalf("new resolve = %d %v", userID, err)
	}
}
