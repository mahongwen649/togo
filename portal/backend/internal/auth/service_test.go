package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
	"github.com/jqcode/portal/backend/internal/username"
)

type coreStub struct {
	loginRequest    coreclient.LoginRequest
	registerRequest coreclient.RegisterRequest
	registerResult  coreclient.AuthResult
	adminUser       coreclient.User
	adminUsers      coreclient.UserPage
	profileUser     coreclient.User
	updatedUsername string
	updateCalls     []string
	updateErr       error
}

func (s *coreStub) PublicSettings(context.Context) (coreclient.Response, error) {
	return coreclient.Response{}, nil
}
func (s *coreStub) Login(_ context.Context, input coreclient.LoginRequest) (coreclient.AuthResult, error) {
	s.loginRequest = input
	return coreclient.AuthResult{}, nil
}
func (s *coreStub) Register(_ context.Context, input coreclient.RegisterRequest) (coreclient.AuthResult, error) {
	s.registerRequest = input
	return s.registerResult, nil
}
func (s *coreStub) UpdateUsername(_ context.Context, _ string, value string) error {
	s.updatedUsername = value
	s.updateCalls = append(s.updateCalls, value)
	return s.updateErr
}
func (s *coreStub) AdminUser(context.Context, int64) (coreclient.User, error) {
	return s.adminUser, nil
}
func (s *coreStub) AdminUsers(context.Context, int, int, ...coreclient.AdminUsersOption) (coreclient.UserPage, error) {
	return s.adminUsers, nil
}
func (s *coreStub) Profile(context.Context, string) (coreclient.User, error) {
	return s.profileUser, nil
}
func (s *coreStub) ProfileResponse(context.Context, string) (coreclient.Response, error) {
	return coreclient.Response{}, nil
}
func (s *coreStub) AdminCreateUser(context.Context, coreclient.AdminCreateUserRequest) (coreclient.User, coreclient.Response, error) {
	return coreclient.User{}, coreclient.Response{}, nil
}
func (s *coreStub) AdminDeleteUser(context.Context, int64) error { return nil }

type usernameStore struct {
	entries map[string]username.Entry
}

func (s *usernameStore) Reserve(_ context.Context, entry username.Entry, _ time.Time) error {
	if s.entries == nil {
		s.entries = map[string]username.Entry{}
	}
	current, exists := s.entries[entry.Key]
	if exists && current.CoreUserID != nil {
		return username.ErrTaken
	}
	s.entries[entry.Key] = entry
	return nil
}
func (s *usernameStore) Bind(_ context.Context, key, lease string, userID int64) error {
	entry, ok := s.entries[key]
	if !ok || entry.LeaseID != lease {
		return username.ErrBadLease
	}
	entry.CoreUserID = &userID
	s.entries[key] = entry
	return nil
}
func (s *usernameStore) SyncBound(_ context.Context, key, display string, userID int64) error {
	if s.entries == nil {
		s.entries = map[string]username.Entry{}
	}
	for currentKey, entry := range s.entries {
		if entry.CoreUserID != nil && *entry.CoreUserID == userID && currentKey != key {
			delete(s.entries, currentKey)
		}
	}
	existing, ok := s.entries[key]
	if ok && existing.CoreUserID != nil && *existing.CoreUserID != userID {
		return username.ErrTaken
	}
	s.entries[key] = username.Entry{Key: key, Username: display, CoreUserID: &userID}
	return nil
}
func (s *usernameStore) Release(_ context.Context, key, lease string) error {
	entry, ok := s.entries[key]
	if ok && entry.CoreUserID == nil && entry.LeaseID == lease {
		delete(s.entries, key)
	}
	return nil
}
func (s *usernameStore) Resolve(_ context.Context, key string) (int64, error) {
	entry, ok := s.entries[key]
	if !ok || entry.CoreUserID == nil {
		return 0, username.ErrNotFound
	}
	return *entry.CoreUserID, nil
}
func (s *usernameStore) Available(_ context.Context, key string, now time.Time) (bool, error) {
	entry, ok := s.entries[key]
	if !ok {
		return true, nil
	}
	return entry.CoreUserID == nil && !entry.ExpiresAt.After(now), nil
}
func (s *usernameStore) DeleteBound(_ context.Context, key string, userID int64) error {
	entry, ok := s.entries[key]
	if !ok || entry.CoreUserID == nil || *entry.CoreUserID != userID {
		return username.ErrNotFound
	}
	delete(s.entries, key)
	return nil
}
func (s *usernameStore) ReplaceBound(_ context.Context, oldKey string, userID int64, newKey, newUsername, newLeaseID string) error {
	newEntry, ok := s.entries[newKey]
	if !ok || newEntry.LeaseID != newLeaseID || newEntry.CoreUserID != nil {
		return username.ErrBadLease
	}
	oldEntry, ok := s.entries[oldKey]
	if !ok || oldEntry.CoreUserID == nil || *oldEntry.CoreUserID != userID {
		return username.ErrNotFound
	}
	newEntry.Username = newUsername
	newEntry.CoreUserID = &userID
	s.entries[newKey] = newEntry
	delete(s.entries, oldKey)
	return nil
}

func TestUsernameLoginResolvesLatestCoreEmail(t *testing.T) {
	store := &usernameStore{}
	registry := username.New(store)
	reservation, _ := registry.Reserve(context.Background(), "Alice")
	_ = registry.Bind(context.Background(), reservation, 42)
	core := &coreStub{adminUser: coreclient.User{ID: 42, Email: "latest@example.com"}}
	service := New(core, registry)
	_, err := service.Login(context.Background(), LoginInput{Identifier: "alice", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if core.loginRequest.Email != "latest@example.com" || core.loginRequest.Password != "secret" {
		t.Fatalf("login request = %+v", core.loginRequest)
	}
}

func TestUsernameLoginAllowsLegacyCharactersWithoutRelaxingReservations(t *testing.T) {
	store := &usernameStore{}
	registry := username.New(store)
	if err := registry.SyncBound(context.Background(), "legacy.user.01", 7); err != nil {
		t.Fatal(err)
	}
	core := &coreStub{adminUser: coreclient.User{ID: 7, Email: "legacy@example.com", Username: "legacy.user.01"}}
	service := New(core, registry)
	_, err := service.Login(context.Background(), LoginInput{Identifier: "Legacy.User.01", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if core.loginRequest.Email != "legacy@example.com" {
		t.Fatalf("login request = %+v", core.loginRequest)
	}
	if _, err := registry.Reserve(context.Background(), "new.user.01"); err != username.ErrInvalid {
		t.Fatalf("legacy characters were accepted for a new reservation: %v", err)
	}
}

func TestUsernameLoginSyncsRegistryWhenCoreUsernameChanged(t *testing.T) {
	store := &usernameStore{}
	registry := username.New(store)
	reservation, _ := registry.Reserve(context.Background(), "old-name")
	_ = registry.Bind(context.Background(), reservation, 11)
	core := &coreStub{
		adminUser: coreclient.User{ID: 11, Username: "user3", Email: "user3@example.com"},
		adminUsers: coreclient.UserPage{Items: []coreclient.User{
			{ID: 11, Username: "user3", Email: "user3@example.com"},
		}},
	}
	service := New(core, registry)
	_, err := service.Login(context.Background(), LoginInput{Identifier: "user3", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if core.loginRequest.Email != "user3@example.com" {
		t.Fatalf("login request = %+v", core.loginRequest)
	}
	if _, err := registry.Resolve(context.Background(), "old-name"); err != username.ErrNotFound {
		t.Fatalf("old name should be removed, err=%v", err)
	}
	if id, err := registry.Resolve(context.Background(), "user3"); err != nil || id != 11 {
		t.Fatalf("new name resolve id=%d err=%v", id, err)
	}
}

func TestUsernameLoginSyncsRegistryWhenUsernameMissingLocally(t *testing.T) {
	store := &usernameStore{}
	core := &coreStub{adminUsers: coreclient.UserPage{Items: []coreclient.User{
		{ID: 8, Username: "scanadmin13940398", Email: "scan@example.com"},
	}}}
	service := New(core, username.New(store))
	_, err := service.Login(context.Background(), LoginInput{Identifier: "scanadmin13940398", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if core.loginRequest.Email != "scan@example.com" {
		t.Fatalf("login request = %+v", core.loginRequest)
	}
}

func TestEmailLoginDoesNotUseUsernameRegistry(t *testing.T) {
	core := &coreStub{}
	service := New(core, username.New(&usernameStore{}))
	_, err := service.Login(context.Background(), LoginInput{Identifier: "User@Example.com", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if core.loginRequest.Email != "User@Example.com" {
		t.Fatalf("email = %q", core.loginRequest.Email)
	}
}

func TestRegisterBindsUsernameAndUpdatesCoreProfile(t *testing.T) {
	store := &usernameStore{}
	core := &coreStub{registerResult: coreclient.AuthResult{
		Response: coreclient.Response{StatusCode: 200, Body: []byte(`{"code":0,"data":{"user":{"id":7,"username":""}}}`)}, User: coreclient.User{ID: 7}, AccessToken: "token",
	}}
	service := New(core, username.New(store))
	result, err := service.Register(context.Background(), RegisterInput{
		Username: "Alice", Email: "a@example.com", Password: "secret",
		PromoCode: "PROMO", InvitationCode: "INVITE", AffCode: "AFF123",
	})
	if err != nil {
		t.Fatal(err)
	}
	entry := store.entries["alice"]
	if result.User.Username != "Alice" || core.updatedUsername != "Alice" || entry.CoreUserID == nil || *entry.CoreUserID != 7 {
		t.Fatalf("result=%+v entry=%+v updated=%q", result, entry, core.updatedUsername)
	}
	if core.registerRequest.PromoCode != "PROMO" || core.registerRequest.InvitationCode != "INVITE" || core.registerRequest.AffCode != "AFF123" {
		t.Fatalf("register request=%+v", core.registerRequest)
	}
}

func TestRegisterKeepsBoundUsernameWhenProfileSyncFails(t *testing.T) {
	store := &usernameStore{}
	core := &coreStub{registerResult: coreclient.AuthResult{
		Response: coreclient.Response{StatusCode: 200, Body: []byte(`{"code":0,"data":{"user":{"id":7,"username":""}}}`)}, User: coreclient.User{ID: 7}, AccessToken: "token",
	}, updateErr: errors.New("temporary failure")}
	service := New(core, username.New(store))
	_, err := service.Register(context.Background(), RegisterInput{Username: "Alice", Email: "a@example.com", Password: "secret"})
	if err == nil {
		t.Fatal("expected profile sync error")
	}
	entry := store.entries["alice"]
	if entry.CoreUserID == nil || *entry.CoreUserID != 7 {
		t.Fatal("bound username must remain reserved for the created Core user")
	}
}

func TestRenameCurrentUserReservesUpdatesCoreAndSwitchesRegistry(t *testing.T) {
	store := &usernameStore{}
	registry := username.New(store)
	oldReservation, _ := registry.Reserve(context.Background(), "Alice")
	if err := registry.Bind(context.Background(), oldReservation, 7); err != nil {
		t.Fatal(err)
	}
	core := &coreStub{profileUser: coreclient.User{ID: 7, Username: "Alice", Email: "a@example.com"}}
	service := New(core, registry)
	user, err := service.RenameCurrentUser(context.Background(), "token", "Bob")
	if err != nil {
		t.Fatal(err)
	}
	if user.Username != "Bob" || core.updatedUsername != "Bob" {
		t.Fatalf("user=%+v updated=%q", user, core.updatedUsername)
	}
	if id, err := registry.Resolve(context.Background(), "bob"); err != nil || id != 7 {
		t.Fatalf("resolve bob=%d %v", id, err)
	}
}

func TestRenameCurrentUserNoopsSameCaseInsensitiveUsername(t *testing.T) {
	core := &coreStub{profileUser: coreclient.User{ID: 7, Username: "Alice", Email: "a@example.com"}}
	service := New(core, username.New(&usernameStore{}))
	user, err := service.RenameCurrentUser(context.Background(), "token", "alice")
	if err != nil {
		t.Fatal(err)
	}
	if user.Username != "Alice" || len(core.updateCalls) != 0 {
		t.Fatalf("user=%+v updateCalls=%v", user, core.updateCalls)
	}
}
