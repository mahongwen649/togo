package auth

import (
	"errors"
	"sync"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

type Store interface {
	CountAdmins() (int, error)
	CreateUser(user User) (User, error)
	ListUsers() ([]User, error)
	UserByUsername(username string) (User, error)
	UserByExternalIdentity(provider string, subject string) (User, error)
	UserByID(id string) (User, error)
	SetUserDisabled(id string, disabled bool) error
	UpdatePasswordHash(id string, passwordHash string) error
	CreateSession(session Session) (Session, error)
	SessionByID(id string) (Session, error)
	DeleteSession(id string) error
}

type MemoryStore struct {
	mu       sync.RWMutex
	users    map[string]User
	sessions map[string]Session
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users:    map[string]User{},
		sessions: map[string]Session{},
	}
}

func (s *MemoryStore) CountAdmins() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, user := range s.users {
		if user.Role == RoleAdmin {
			count += 1
		}
	}
	return count, nil
}

func (s *MemoryStore) CreateUser(user User) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.users {
		if existing.Username == user.Username {
			return User{}, ErrConflict
		}
		if user.ExternalProvider != "" && user.ExternalSubject != "" &&
			existing.ExternalProvider == user.ExternalProvider && existing.ExternalSubject == user.ExternalSubject {
			return User{}, ErrConflict
		}
	}
	s.users[user.ID] = user
	return user, nil
}

func (s *MemoryStore) ListUsers() ([]User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	users := make([]User, 0, len(s.users))
	for _, user := range s.users {
		users = append(users, user)
	}
	return users, nil
}

func (s *MemoryStore) UserByUsername(username string) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, user := range s.users {
		if user.Username == username {
			return user, nil
		}
	}
	return User{}, ErrNotFound
}

func (s *MemoryStore) UserByExternalIdentity(provider string, subject string) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, user := range s.users {
		if user.ExternalProvider == provider && user.ExternalSubject == subject {
			return user, nil
		}
	}
	return User{}, ErrNotFound
}

func (s *MemoryStore) UserByID(id string) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return user, nil
}

func (s *MemoryStore) SetUserDisabled(id string, disabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[id]
	if !ok {
		return ErrNotFound
	}
	if disabled {
		user.DisabledAt = nowMillis()
	} else {
		user.DisabledAt = 0
	}
	user.UpdatedAt = nowMillis()
	s.users[id] = user
	return nil
}

func (s *MemoryStore) UpdatePasswordHash(id string, passwordHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[id]
	if !ok {
		return ErrNotFound
	}
	user.PasswordHash = passwordHash
	user.UpdatedAt = nowMillis()
	s.users[id] = user
	return nil
}

func (s *MemoryStore) CreateSession(session Session) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = session
	return session, nil
}

func (s *MemoryStore) SessionByID(id string) (Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[id]
	if !ok {
		return Session{}, ErrNotFound
	}
	return session, nil
}

func (s *MemoryStore) DeleteSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
	return nil
}
