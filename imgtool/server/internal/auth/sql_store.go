package auth

import (
	"database/sql"
	"errors"
)

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}

func (s *SQLStore) CountAdmins() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&count)
	return count, err
}

func (s *SQLStore) CreateUser(user User) (User, error) {
	_, err := s.db.Exec(
		`INSERT INTO users (id, username, password_hash, role, disabled_at, created_at, updated_at, external_provider, external_subject)
		 VALUES (?, ?, ?, ?, NULLIF(?, 0), ?, ?, NULLIF(?, ''), NULLIF(?, ''))`,
		user.ID,
		user.Username,
		user.PasswordHash,
		user.Role,
		user.DisabledAt,
		user.CreatedAt,
		user.UpdatedAt,
		user.ExternalProvider,
		user.ExternalSubject,
	)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *SQLStore) ListUsers() ([]User, error) {
	rows, err := s.db.Query(
		`SELECT id, username, password_hash, role, COALESCE(disabled_at, 0), created_at, updated_at, COALESCE(external_provider, ''), COALESCE(external_subject, '')
		 FROM users
		 ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []User{}
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *SQLStore) UserByUsername(username string) (User, error) {
	row := s.db.QueryRow(
		`SELECT id, username, password_hash, role, COALESCE(disabled_at, 0), created_at, updated_at, COALESCE(external_provider, ''), COALESCE(external_subject, '')
		 FROM users
		 WHERE username = ?`,
		username,
	)
	return scanUser(row)
}

func (s *SQLStore) UserByExternalIdentity(provider string, subject string) (User, error) {
	row := s.db.QueryRow(
		`SELECT id, username, password_hash, role, COALESCE(disabled_at, 0), created_at, updated_at, COALESCE(external_provider, ''), COALESCE(external_subject, '')
		 FROM users
		 WHERE external_provider = ? AND external_subject = ?`,
		provider,
		subject,
	)
	return scanUser(row)
}

func (s *SQLStore) UserByID(id string) (User, error) {
	row := s.db.QueryRow(
		`SELECT id, username, password_hash, role, COALESCE(disabled_at, 0), created_at, updated_at, COALESCE(external_provider, ''), COALESCE(external_subject, '')
		 FROM users
		 WHERE id = ?`,
		id,
	)
	return scanUser(row)
}

func (s *SQLStore) SetUserDisabled(id string, disabled bool) error {
	var disabledAt any
	if disabled {
		disabledAt = nowMillis()
	}
	result, err := s.db.Exec(`UPDATE users SET disabled_at = ?, updated_at = ? WHERE id = ?`, disabledAt, nowMillis(), id)
	if err != nil {
		return err
	}
	return requireAffected(result)
}

func (s *SQLStore) UpdatePasswordHash(id string, passwordHash string) error {
	result, err := s.db.Exec(`UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`, passwordHash, nowMillis(), id)
	if err != nil {
		return err
	}
	return requireAffected(result)
}

func (s *SQLStore) CreateSession(session Session) (Session, error) {
	_, err := s.db.Exec(
		`INSERT INTO sessions (id, user_id, expires_at, created_at, last_seen_at)
		 VALUES (?, ?, ?, ?, ?)`,
		session.ID,
		session.UserID,
		session.ExpiresAt,
		session.CreatedAt,
		session.LastSeenAt,
	)
	if err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s *SQLStore) SessionByID(id string) (Session, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, expires_at, created_at, last_seen_at
		 FROM sessions
		 WHERE id = ?`,
		id,
	)
	var session Session
	if err := row.Scan(&session.ID, &session.UserID, &session.ExpiresAt, &session.CreatedAt, &session.LastSeenAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrNotFound
		}
		return Session{}, err
	}
	return session, nil
}

func (s *SQLStore) DeleteSession(id string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

type userScanner interface {
	Scan(dest ...any) error
}

func scanUser(scanner userScanner) (User, error) {
	var user User
	if err := scanner.Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Role,
		&user.DisabledAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.ExternalProvider,
		&user.ExternalSubject,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	return user, nil
}

func requireAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}
