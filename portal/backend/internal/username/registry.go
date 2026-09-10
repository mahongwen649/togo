package username

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalid  = errors.New("invalid username")
	ErrTaken    = errors.New("username is already taken")
	ErrNotFound = errors.New("username not found")
	ErrBadLease = errors.New("username reservation is invalid")
)

const reservationTTL = 2 * time.Minute

type Entry struct {
	Key        string
	Username   string
	CoreUserID *int64
	LeaseID    string
	ExpiresAt  time.Time
}

type Store interface {
	Reserve(context.Context, Entry, time.Time) error
	Bind(context.Context, string, string, int64) error
	SyncBound(context.Context, string, string, int64) error
	ReplaceBound(context.Context, string, int64, string, string, string) error
	Release(context.Context, string, string) error
	Resolve(context.Context, string) (int64, error)
	Available(context.Context, string, time.Time) (bool, error)
	DeleteBound(context.Context, string, int64) error
}

type Registry struct {
	store Store
	now   func() time.Time
}

type Reservation struct {
	Key      string
	Username string
	LeaseID  string
}

func New(store Store) *Registry {
	return &Registry{store: store, now: time.Now}
}

func Normalize(value string) (display string, key string, err error) {
	display = strings.TrimSpace(value)
	length := utf8.RuneCountInString(display)
	if length < 2 || length > 30 {
		return "", "", ErrInvalid
	}
	for _, r := range display {
		if !isAllowedRune(r) {
			return "", "", ErrInvalid
		}
	}
	return display, strings.ToLower(display), nil
}

// NormalizeExisting accepts legacy Core usernames for lookup and migration.
// New reservations and renames must continue to use Normalize.
func NormalizeExisting(value string) (display string, key string, err error) {
	display = strings.TrimSpace(value)
	if display == "" || utf8.RuneCountInString(display) > 100 {
		return "", "", ErrInvalid
	}
	return display, strings.ToLower(display), nil
}

func isAllowedRune(r rune) bool {
	return r >= 'a' && r <= 'z' ||
		r >= 'A' && r <= 'Z' ||
		r >= '0' && r <= '9' ||
		r == '_' || r == '-' ||
		r >= '\u4e00' && r <= '\u9fff'
}

func (r *Registry) Reserve(ctx context.Context, value string) (Reservation, error) {
	display, key, err := Normalize(value)
	if err != nil {
		return Reservation{}, err
	}
	leaseID, err := newLeaseID()
	if err != nil {
		return Reservation{}, fmt.Errorf("create username reservation: %w", err)
	}
	entry := Entry{
		Key:       key,
		Username:  display,
		LeaseID:   leaseID,
		ExpiresAt: r.now().Add(reservationTTL),
	}
	if err := r.store.Reserve(ctx, entry, r.now()); err != nil {
		return Reservation{}, err
	}
	return Reservation{Key: key, Username: display, LeaseID: leaseID}, nil
}

func (r *Registry) Bind(ctx context.Context, reservation Reservation, coreUserID int64) error {
	if coreUserID <= 0 {
		return ErrBadLease
	}
	return r.store.Bind(ctx, reservation.Key, reservation.LeaseID, coreUserID)
}

func (r *Registry) SyncBound(ctx context.Context, value string, coreUserID int64) error {
	display, key, err := NormalizeExisting(value)
	if err != nil || coreUserID <= 0 {
		return ErrInvalid
	}
	return r.store.SyncBound(ctx, key, display, coreUserID)
}

func (r *Registry) ReplaceBound(ctx context.Context, oldValue string, reservation Reservation, coreUserID int64) error {
	if coreUserID <= 0 {
		return ErrBadLease
	}
	_, oldKey, err := NormalizeExisting(oldValue)
	if err != nil {
		return ErrNotFound
	}
	if oldKey == reservation.Key {
		return r.store.Bind(ctx, reservation.Key, reservation.LeaseID, coreUserID)
	}
	return r.store.ReplaceBound(ctx, oldKey, coreUserID, reservation.Key, reservation.Username, reservation.LeaseID)
}

func (r *Registry) Release(ctx context.Context, reservation Reservation) error {
	return r.store.Release(ctx, reservation.Key, reservation.LeaseID)
}

func (r *Registry) Resolve(ctx context.Context, value string) (int64, error) {
	_, key, err := NormalizeExisting(value)
	if err != nil {
		return 0, ErrNotFound
	}
	return r.store.Resolve(ctx, key)
}

func (r *Registry) Available(ctx context.Context, value string) (bool, error) {
	_, key, err := Normalize(value)
	if err != nil {
		return false, err
	}
	return r.store.Available(ctx, key, r.now())
}

func (r *Registry) DeleteBound(ctx context.Context, value string, coreUserID int64) error {
	_, key, err := NormalizeExisting(value)
	if err != nil || coreUserID <= 0 {
		return ErrInvalid
	}
	return r.store.DeleteBound(ctx, key, coreUserID)
}

func newLeaseID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}
