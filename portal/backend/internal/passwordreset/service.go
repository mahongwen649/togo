package passwordreset

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

const (
	CodeTTL      = 10 * time.Minute
	SendCooldown = 60 * time.Second
	MaxAttempts  = 5
)

var ErrInvalidCode = &Error{Status: 400, Code: "INVALID_VERIFY_CODE", Message: "Invalid or expired verification code"}

type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string     { return e.Message }
func (e *Error) StatusCode() int   { return e.Status }
func (e *Error) ErrorCode() string { return e.Code }

type Core interface {
	AdminUsers(context.Context, int, int, ...coreclient.AdminUsersOption) (coreclient.UserPage, error)
	AdminUpdatePassword(context.Context, int64, string) error
}

type Mailer interface {
	SendPasswordResetCode(context.Context, string, string, time.Duration) error
}

type HumanVerifier interface {
	Verify(context.Context, string, string) error
}

type Service struct {
	db       *sql.DB
	core     Core
	mailer   Mailer
	verifier HumanVerifier
	secret   []byte
	now      func() time.Time
}

func New(db *sql.DB, core Core, mailer Mailer, verifier HumanVerifier, secret []byte) (*Service, error) {
	if db == nil || core == nil || mailer == nil || verifier == nil || len(secret) < 32 {
		return nil, errors.New("password reset requires database, Core, mailer, verifier, and a 32-byte secret")
	}
	return &Service{db: db, core: core, mailer: mailer, verifier: verifier, secret: secret, now: time.Now}, nil
}

func (s *Service) SendCode(ctx context.Context, email, turnstileToken, remoteIP string) (int, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return 0, &Error{Status: 400, Code: "INVALID_REQUEST", Message: "A valid email address is required"}
	}
	now := s.now().UTC()
	var lastSent, expiresAt time.Time
	var sendCount int
	err = s.db.QueryRowContext(ctx, `SELECT last_sent_at,expires_at,send_count FROM portal_password_reset_codes WHERE email=$1`, email).Scan(&lastSent, &expiresAt, &sendCount)
	activeRequest := err == nil && now.Before(expiresAt)
	if activeRequest && now.Sub(lastSent) < SendCooldown {
		return int(lastSent.Add(SendCooldown).Sub(now).Seconds()) + 1, &Error{Status: 429, Code: "RESET_CODE_TOO_FREQUENT", Message: "Please wait before requesting another code"}
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("read reset cooldown: %w", err)
	}

	if activeRequest && sendCount >= 5 {
		return 0, &Error{Status: 429, Code: "RESET_CODE_LIMIT_REACHED", Message: "Please wait before requesting another code"}
	}
	if !activeRequest {
		if err := s.verifier.Verify(ctx, turnstileToken, remoteIP); err != nil {
			return 0, err
		}
	}

	user, found, err := s.findUser(ctx, email)
	if err != nil {
		return 0, err
	}
	if !found {
		return int(SendCooldown / time.Second), nil
	}

	code, err := generateCode()
	if err != nil {
		return 0, fmt.Errorf("generate reset code: %w", err)
	}
	digest := s.digest(email, code)
	_, err = s.db.ExecContext(ctx, `
INSERT INTO portal_password_reset_codes(email,core_user_id,code_digest,attempts,send_count,expires_at,last_sent_at,created_at)
VALUES($1,$2,$3,0,1,$4,$5,$5)
ON CONFLICT(email) DO UPDATE SET core_user_id=EXCLUDED.core_user_id,code_digest=EXCLUDED.code_digest,
attempts=0,send_count=CASE WHEN portal_password_reset_codes.expires_at > EXCLUDED.last_sent_at THEN portal_password_reset_codes.send_count+1 ELSE 1 END,
expires_at=EXCLUDED.expires_at,last_sent_at=EXCLUDED.last_sent_at`, email, user.ID, digest, now.Add(CodeTTL), now)
	if err != nil {
		return 0, fmt.Errorf("save reset code: %w", err)
	}
	if err := s.mailer.SendPasswordResetCode(ctx, email, code, CodeTTL); err != nil {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM portal_password_reset_codes WHERE email=$1`, email)
		slog.Error("failed to send Portal password reset code", "error", err)
		return int(SendCooldown / time.Second), nil
	}
	return int(SendCooldown / time.Second), nil
}

func (s *Service) ResetPassword(ctx context.Context, email, code, password string) error {
	email, err := normalizeEmail(email)
	if err != nil || len(code) != 6 || len(password) < 8 {
		return &Error{Status: 400, Code: "INVALID_REQUEST", Message: "Email, 6-digit code, and an 8-character password are required"}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin reset transaction: %w", err)
	}
	defer tx.Rollback()

	var userID int64
	var digest []byte
	var attempts int
	var expiresAt time.Time
	err = tx.QueryRowContext(ctx, `SELECT core_user_id,code_digest,attempts,expires_at FROM portal_password_reset_codes WHERE email=$1 FOR UPDATE`, email).
		Scan(&userID, &digest, &attempts, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) || attempts >= MaxAttempts || !s.now().Before(expiresAt) {
		return ErrInvalidCode
	}
	if err != nil {
		return fmt.Errorf("read reset code: %w", err)
	}
	if !hmac.Equal(digest, s.digest(email, code)) {
		_, _ = tx.ExecContext(ctx, `UPDATE portal_password_reset_codes SET attempts=attempts+1 WHERE email=$1`, email)
		_ = tx.Commit()
		return ErrInvalidCode
	}
	if err := s.core.AdminUpdatePassword(ctx, userID, password); err != nil {
		return fmt.Errorf("update Core password: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM portal_password_reset_codes WHERE email=$1`, email); err != nil {
		return fmt.Errorf("consume reset code: %w", err)
	}
	return tx.Commit()
}

func (s *Service) findUser(ctx context.Context, email string) (coreclient.User, bool, error) {
	page, err := s.core.AdminUsers(ctx, 1, 20, coreclient.AdminUsersSearch(email))
	if err != nil {
		return coreclient.User{}, false, fmt.Errorf("find Core user: %w", err)
	}
	for _, user := range page.Items {
		if strings.EqualFold(strings.TrimSpace(user.Email), email) && user.Status == "active" {
			return user, true, nil
		}
	}
	return coreclient.User{}, false, nil
}

func (s *Service) digest(email, code string) []byte {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(email + "\n" + code))
	return mac.Sum(nil)
}

func normalizeEmail(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value || len(value) > 320 {
		return "", errors.New("invalid email")
	}
	return value, nil
}

func generateCode() (string, error) {
	var raw [4]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", binary.BigEndian.Uint32(raw[:])%1000000), nil
}
