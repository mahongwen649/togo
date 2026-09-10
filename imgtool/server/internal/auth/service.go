package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sort"
	"strings"
	"time"

	"imgtool/server/internal/crypto"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrDisabledUser       = errors.New("user disabled")
	ErrExpiredSession     = errors.New("session expired")
)

type Options struct {
	AdminUsername string
	AdminPassword string
	SessionTTL    time.Duration
}

type Service struct {
	store Store
	opts  Options
}

func NewService(store Store, opts Options) *Service {
	if opts.AdminUsername == "" {
		opts.AdminUsername = "admin"
	}
	if opts.AdminPassword == "" {
		opts.AdminPassword = "admin"
	}
	if opts.SessionTTL == 0 {
		opts.SessionTTL = 24 * time.Hour
	}
	return &Service{store: store, opts: opts}
}

func (s *Service) BootstrapAdmin() (User, error) {
	count, err := s.store.CountAdmins()
	if err != nil {
		return User{}, err
	}
	if count > 0 {
		return s.store.UserByUsername(s.opts.AdminUsername)
	}
	hash, err := crypto.HashPassword(s.opts.AdminPassword)
	if err != nil {
		return User{}, err
	}
	now := nowMillis()
	return s.store.CreateUser(User{
		ID:           newID("usr"),
		Username:     s.opts.AdminUsername,
		PasswordHash: hash,
		Role:         RoleAdmin,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
}

func (s *Service) SessionMaxAgeSeconds() int {
	return int(s.opts.SessionTTL.Seconds())
}

func (s *Service) Login(username string, password string) (Session, User, error) {
	user, err := s.store.UserByUsername(strings.TrimSpace(username))
	if err != nil {
		return Session{}, User{}, ErrInvalidCredentials
	}
	if user.DisabledAt != 0 {
		return Session{}, User{}, ErrDisabledUser
	}
	if !crypto.VerifyPassword(user.PasswordHash, password) {
		return Session{}, User{}, ErrInvalidCredentials
	}
	now := nowMillis()
	session, err := s.store.CreateSession(Session{
		ID:         newSessionID(),
		UserID:     user.ID,
		ExpiresAt:  now + s.opts.SessionTTL.Milliseconds(),
		CreatedAt:  now,
		LastSeenAt: now,
	})
	if err != nil {
		return Session{}, User{}, err
	}
	return session, user, nil
}

func (s *Service) LoginWithSSO(claims *SSOTicketClaims) (Session, User, error) {
	if claims == nil || strings.TrimSpace(claims.Subject) == "" {
		return Session{}, User{}, ErrInvalidCredentials
	}
	user, err := s.store.UserByExternalIdentity("sub2api", strings.TrimSpace(claims.Subject))
	if err != nil {
		user, err = s.createSSOUser(claims)
		if err != nil {
			return Session{}, User{}, err
		}
	}
	if user.DisabledAt != 0 {
		return Session{}, User{}, ErrDisabledUser
	}
	now := nowMillis()
	session, err := s.store.CreateSession(Session{
		ID:         newSessionID(),
		UserID:     user.ID,
		ExpiresAt:  now + s.opts.SessionTTL.Milliseconds(),
		CreatedAt:  now,
		LastSeenAt: now,
	})
	if err != nil {
		return Session{}, User{}, err
	}
	return session, user, nil
}

func (s *Service) createSSOUser(claims *SSOTicketClaims) (User, error) {
	subject := strings.TrimSpace(claims.Subject)
	username := strings.TrimSpace(claims.Username)
	if username == "" {
		username = "TogoAPI_" + subject
	} else {
		username = "TogoAPI_" + subject + "_" + username
	}
	username = normalizeSSOUsername(username)
	hash, err := crypto.HashPassword(randomToken(32))
	if err != nil {
		return User{}, err
	}
	now := nowMillis()
	user, err := s.store.CreateUser(User{
		ID:               newID("usr"),
		Username:         username,
		PasswordHash:     hash,
		Role:             RoleUser,
		CreatedAt:        now,
		UpdatedAt:        now,
		ExternalProvider: "sub2api",
		ExternalSubject:  subject,
	})
	if err != nil {
		if errors.Is(err, ErrConflict) {
			return s.store.UserByExternalIdentity("sub2api", subject)
		}
		return User{}, err
	}
	return publicUser(user), nil
}

func (s *Service) CreateUser(username string, password string, role string) (User, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return User{}, ErrInvalidCredentials
	}
	if role != RoleAdmin && role != RoleUser {
		role = RoleUser
	}
	hash, err := crypto.HashPassword(password)
	if err != nil {
		return User{}, err
	}
	now := nowMillis()
	user, err := s.store.CreateUser(User{
		ID:           newID("usr"),
		Username:     username,
		PasswordHash: hash,
		Role:         role,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		return User{}, err
	}
	return publicUser(user), nil
}

func (s *Service) ListUsers() ([]User, error) {
	users, err := s.store.ListUsers()
	if err != nil {
		return nil, err
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].CreatedAt < users[j].CreatedAt
	})
	for index := range users {
		users[index] = publicUser(users[index])
	}
	return users, nil
}

func (s *Service) SetUserDisabled(id string, disabled bool) error {
	return s.store.SetUserDisabled(id, disabled)
}

func (s *Service) ResetPassword(id string, password string) error {
	if password == "" {
		return ErrInvalidCredentials
	}
	hash, err := crypto.HashPassword(password)
	if err != nil {
		return err
	}
	return s.store.UpdatePasswordHash(id, hash)
}

func (s *Service) UserForSession(sessionID string) (User, error) {
	session, err := s.store.SessionByID(sessionID)
	if err != nil {
		return User{}, err
	}
	if session.ExpiresAt <= nowMillis() {
		_ = s.store.DeleteSession(session.ID)
		return User{}, ErrExpiredSession
	}
	user, err := s.store.UserByID(session.UserID)
	if err != nil {
		return User{}, err
	}
	if user.DisabledAt != 0 {
		return User{}, ErrDisabledUser
	}
	return publicUser(user), nil
}

func (s *Service) Logout(sessionID string) error {
	return s.store.DeleteSession(sessionID)
}

func newID(prefix string) string {
	return prefix + "_" + randomToken(18)
}

func newSessionID() string {
	return "ses_" + randomToken(32)
}

func randomToken(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes)
}

func nowMillis() int64 {
	return time.Now().UnixMilli()
}

func publicUser(user User) User {
	if user.ExternalProvider == "sub2api" && strings.HasPrefix(strings.ToLower(user.Username), "sub2api_") {
		user.Username = "TogoAPI_" + user.Username[len("sub2api_"):]
	}
	user.PasswordHash = ""
	return user
}

func normalizeSSOUsername(username string) string {
	var b strings.Builder
	for _, r := range username {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	normalized := strings.Trim(b.String(), "_-")
	if normalized == "" {
		return "TogoAPI_user"
	}
	return normalized
}
