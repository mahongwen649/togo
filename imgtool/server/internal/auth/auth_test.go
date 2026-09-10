package auth

import (
	"database/sql"
	"testing"
	"time"

	"imgtool/server/internal/db"
)

func TestAuthServiceWithMemoryStore(t *testing.T) {
	runAuthServiceContract(t, func(t *testing.T) Store {
		t.Helper()
		return NewMemoryStore()
	})
}

func TestAuthServiceWithSQLStore(t *testing.T) {
	runAuthServiceContract(t, func(t *testing.T) Store {
		t.Helper()
		database := openTestDB(t)
		return NewSQLStore(database)
	})
}

func runAuthServiceContract(t *testing.T, newStore func(*testing.T) Store) {
	t.Run("bootstrap creates admin", func(t *testing.T) {
		service := NewService(newStore(t), Options{
			AdminUsername: "admin",
			AdminPassword: "secret",
			SessionTTL:    time.Hour,
		})

		user, err := service.BootstrapAdmin()
		if err != nil {
			t.Fatalf("BootstrapAdmin returned error: %v", err)
		}
		if user.Username != "admin" || user.Role != RoleAdmin {
			t.Fatalf("unexpected admin user: %#v", user)
		}
	})

	t.Run("login accepts correct password", func(t *testing.T) {
		service := newBootstrappedService(t, newStore(t))

		session, user, err := service.Login("admin", "secret")
		if err != nil {
			t.Fatalf("Login returned error: %v", err)
		}
		if session.ID == "" {
			t.Fatal("expected session id")
		}
		if user.Username != "admin" {
			t.Fatalf("username = %q", user.Username)
		}
	})

	t.Run("login rejects wrong password", func(t *testing.T) {
		service := newBootstrappedService(t, newStore(t))

		_, _, err := service.Login("admin", "wrong")
		if err == nil {
			t.Fatal("expected login failure")
		}
	})

	t.Run("disabled user cannot login or use session", func(t *testing.T) {
		store := newStore(t)
		service := NewService(store, Options{
			AdminUsername: "admin",
			AdminPassword: "secret",
			SessionTTL:    time.Hour,
		})
		user, err := service.BootstrapAdmin()
		if err != nil {
			t.Fatalf("BootstrapAdmin returned error: %v", err)
		}
		session, _, err := service.Login("admin", "secret")
		if err != nil {
			t.Fatalf("Login returned error: %v", err)
		}
		if err := store.SetUserDisabled(user.ID, true); err != nil {
			t.Fatalf("SetUserDisabled returned error: %v", err)
		}

		if _, _, err := service.Login("admin", "secret"); err == nil {
			t.Fatal("expected disabled login to fail")
		}
		if _, err := service.UserForSession(session.ID); err == nil {
			t.Fatal("expected disabled session lookup to fail")
		}
	})

	t.Run("user for session returns user", func(t *testing.T) {
		service := newBootstrappedService(t, newStore(t))
		session, loginUser, err := service.Login("admin", "secret")
		if err != nil {
			t.Fatalf("Login returned error: %v", err)
		}

		user, err := service.UserForSession(session.ID)
		if err != nil {
			t.Fatalf("UserForSession returned error: %v", err)
		}
		if user.ID != loginUser.ID {
			t.Fatalf("user id = %q, want %q", user.ID, loginUser.ID)
		}
	})

	t.Run("sso initializes a TogoAPI branded username", func(t *testing.T) {
		service := NewService(newStore(t), Options{SessionTTL: time.Hour})
		_, user, err := service.LoginWithSSO(&SSOTicketClaims{Subject: "3", Username: "mhw"})
		if err != nil {
			t.Fatalf("LoginWithSSO returned error: %v", err)
		}
		if user.Username != "TogoAPI_3_mhw" {
			t.Fatalf("username = %q, want TogoAPI_3_mhw", user.Username)
		}
	})

	t.Run("existing sso usernames are presented with TogoAPI branding", func(t *testing.T) {
		store := newStore(t)
		if _, err := store.CreateUser(User{
			ID: "usr_legacy", Username: "sub2api_3_mhw", Role: RoleUser,
			ExternalProvider: "sub2api", ExternalSubject: "3",
		}); err != nil {
			t.Fatalf("CreateUser returned error: %v", err)
		}
		service := NewService(store, Options{SessionTTL: time.Hour})
		session, _, err := service.LoginWithSSO(&SSOTicketClaims{Subject: "3", Username: "mhw"})
		if err != nil {
			t.Fatalf("LoginWithSSO returned error: %v", err)
		}
		user, err := service.UserForSession(session.ID)
		if err != nil {
			t.Fatalf("UserForSession returned error: %v", err)
		}
		if user.Username != "TogoAPI_3_mhw" {
			t.Fatalf("username = %q, want TogoAPI_3_mhw", user.Username)
		}
	})

	t.Run("admin user management", func(t *testing.T) {
		service := newBootstrappedService(t, newStore(t))

		user, err := service.CreateUser("alice", "start-password", RoleUser)
		if err != nil {
			t.Fatalf("CreateUser returned error: %v", err)
		}
		if user.Username != "alice" || user.Role != RoleUser || user.PasswordHash != "" {
			t.Fatalf("unexpected user: %#v", user)
		}

		if _, _, err := service.Login("alice", "start-password"); err != nil {
			t.Fatalf("created user could not log in: %v", err)
		}

		if err := service.SetUserDisabled(user.ID, true); err != nil {
			t.Fatalf("SetUserDisabled returned error: %v", err)
		}
		if _, _, err := service.Login("alice", "start-password"); err == nil {
			t.Fatal("disabled user logged in")
		}

		if err := service.SetUserDisabled(user.ID, false); err != nil {
			t.Fatalf("enable user returned error: %v", err)
		}
		if err := service.ResetPassword(user.ID, "new-password"); err != nil {
			t.Fatalf("ResetPassword returned error: %v", err)
		}
		if _, _, err := service.Login("alice", "start-password"); err == nil {
			t.Fatal("old password still worked")
		}
		if _, _, err := service.Login("alice", "new-password"); err != nil {
			t.Fatalf("new password did not work: %v", err)
		}
	})

	t.Run("list users omits password hash", func(t *testing.T) {
		service := newBootstrappedService(t, newStore(t))
		if _, err := service.CreateUser("alice", "start-password", RoleUser); err != nil {
			t.Fatalf("CreateUser returned error: %v", err)
		}

		users, err := service.ListUsers()
		if err != nil {
			t.Fatalf("ListUsers returned error: %v", err)
		}
		if len(users) != 2 {
			t.Fatalf("user count = %d", len(users))
		}
		for _, user := range users {
			if user.PasswordHash != "" {
				t.Fatalf("password hash leaked for %s", user.Username)
			}
		}
	})
}

func newBootstrappedService(t *testing.T, store Store) *Service {
	t.Helper()
	service := NewService(store, Options{
		AdminUsername: "admin",
		AdminPassword: "secret",
		SessionTTL:    time.Hour,
	})
	if _, err := service.BootstrapAdmin(); err != nil {
		t.Fatalf("BootstrapAdmin returned error: %v", err)
	}
	return service
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(t.TempDir() + "/imgtool.sqlite")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}
