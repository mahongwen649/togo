package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jqcode/portal/backend/internal/coreclient"
	"github.com/jqcode/portal/backend/internal/username"
)

type Service struct {
	core      Core
	usernames *username.Registry
}

type Core interface {
	Login(context.Context, coreclient.LoginRequest) (coreclient.AuthResult, error)
	Register(context.Context, coreclient.RegisterRequest) (coreclient.AuthResult, error)
	UpdateUsername(context.Context, string, string) error
	AdminUser(context.Context, int64) (coreclient.User, error)
	AdminUsers(context.Context, int, int, ...coreclient.AdminUsersOption) (coreclient.UserPage, error)
	Profile(context.Context, string) (coreclient.User, error)
}

type LoginInput struct {
	Identifier     string
	Password       string
	TurnstileToken string
}

type RegisterInput struct {
	Username       string
	Email          string
	Password       string
	VerifyCode     string
	TurnstileToken string
	PromoCode      string
	InvitationCode string
	AffCode        string
}

func New(core Core, usernames *username.Registry) *Service {
	return &Service{core: core, usernames: usernames}
}

func (s *Service) Login(ctx context.Context, input LoginInput) (coreclient.AuthResult, error) {
	identifier := strings.TrimSpace(input.Identifier)
	email := identifier
	if !strings.Contains(identifier, "@") {
		user, err := s.coreUserForUsernameLogin(ctx, identifier)
		if err != nil {
			return coreclient.AuthResult{}, err
		}
		email = user.Email
	}
	return s.core.Login(ctx, coreclient.LoginRequest{
		Email:          email,
		Password:       input.Password,
		TurnstileToken: input.TurnstileToken,
	})
}

func (s *Service) coreUserForUsernameLogin(ctx context.Context, identifier string) (coreclient.User, error) {
	_, requestedKey, err := username.NormalizeExisting(identifier)
	if err != nil {
		return coreclient.User{}, username.ErrNotFound
	}

	userID, err := s.usernames.Resolve(ctx, identifier)
	if err == nil {
		user, err := s.core.AdminUser(ctx, userID)
		if err != nil {
			return coreclient.User{}, fmt.Errorf("resolve Core login email: %w", err)
		}
		_, coreKey, normalizeErr := username.NormalizeExisting(user.Username)
		if normalizeErr == nil && coreKey == requestedKey {
			return user, nil
		}
		if normalizeErr != nil {
			return user, nil
		}
		return s.findAndSyncCoreUsername(ctx, identifier, requestedKey)
	}
	if !errors.Is(err, username.ErrNotFound) {
		return coreclient.User{}, err
	}
	return s.findAndSyncCoreUsername(ctx, identifier, requestedKey)
}

func (s *Service) findAndSyncCoreUsername(ctx context.Context, identifier, requestedKey string) (coreclient.User, error) {
	page, err := s.core.AdminUsers(ctx, 1, 20, coreclient.AdminUsersSearch(identifier))
	if err != nil {
		return coreclient.User{}, fmt.Errorf("search Core username: %w", err)
	}
	for _, candidate := range page.Items {
		_, key, normalizeErr := username.NormalizeExisting(candidate.Username)
		if normalizeErr == nil && key == requestedKey {
			if err := s.usernames.SyncBound(ctx, candidate.Username, candidate.ID); err != nil {
				return coreclient.User{}, fmt.Errorf("sync Portal username registry: %w", err)
			}
			return candidate, nil
		}
	}
	return coreclient.User{}, username.ErrNotFound
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (coreclient.AuthResult, error) {
	reservation, err := s.usernames.Reserve(ctx, input.Username)
	if err != nil {
		return coreclient.AuthResult{}, err
	}
	bound := false
	defer func() {
		if !bound {
			_ = s.usernames.Release(context.WithoutCancel(ctx), reservation)
		}
	}()

	result, err := s.core.Register(ctx, coreclient.RegisterRequest{
		Email:          strings.TrimSpace(input.Email),
		Password:       input.Password,
		VerifyCode:     input.VerifyCode,
		TurnstileToken: input.TurnstileToken,
		PromoCode:      input.PromoCode,
		InvitationCode: input.InvitationCode,
		AffCode:        input.AffCode,
	})
	if err != nil || result.Response.StatusCode < 200 || result.Response.StatusCode >= 300 {
		return result, err
	}
	if err := s.usernames.Bind(ctx, reservation, result.User.ID); err != nil {
		return coreclient.AuthResult{}, fmt.Errorf("bind registered Core user to username: %w", err)
	}
	bound = true

	if err := s.core.UpdateUsername(ctx, result.AccessToken, reservation.Username); err != nil {
		return result, fmt.Errorf("Core user was created but username profile sync failed: %w", err)
	}
	result.User.Username = reservation.Username
	if err := setResponseUsername(&result.Response, reservation.Username); err != nil {
		return result, err
	}
	return result, nil
}

func (s *Service) UsernameAvailable(ctx context.Context, value string) (bool, error) {
	return s.usernames.Available(ctx, value)
}

func (s *Service) RenameCurrentUser(ctx context.Context, accessToken, newUsername string) (coreclient.User, error) {
	current, err := s.core.Profile(ctx, accessToken)
	if err != nil {
		return coreclient.User{}, fmt.Errorf("load current Core user: %w", err)
	}
	if current.ID <= 0 {
		return coreclient.User{}, fmt.Errorf("Core profile is incomplete")
	}
	oldUsername := current.Username
	oldDisplay, oldKey, oldErr := username.Normalize(oldUsername)
	newDisplay, newKey, err := username.Normalize(newUsername)
	if err != nil {
		return coreclient.User{}, err
	}
	if oldErr == nil && oldKey == newKey {
		current.Username = oldDisplay
		return current, nil
	}

	reservation, err := s.usernames.Reserve(ctx, newDisplay)
	if err != nil {
		return coreclient.User{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.usernames.Release(context.WithoutCancel(ctx), reservation)
		}
	}()

	if err := s.core.UpdateUsername(ctx, accessToken, reservation.Username); err != nil {
		return coreclient.User{}, fmt.Errorf("update Core username: %w", err)
	}

	if oldErr != nil {
		if err := s.usernames.Bind(ctx, reservation, current.ID); err != nil {
			_ = s.core.UpdateUsername(context.WithoutCancel(ctx), accessToken, oldUsername)
			return coreclient.User{}, fmt.Errorf("bind renamed username: %w", err)
		}
	} else if err := s.usernames.ReplaceBound(ctx, oldUsername, reservation, current.ID); err != nil {
		_ = s.core.UpdateUsername(context.WithoutCancel(ctx), accessToken, oldUsername)
		return coreclient.User{}, fmt.Errorf("replace renamed username binding: %w", err)
	}

	committed = true
	current.Username = reservation.Username
	return current, nil
}

func setResponseUsername(response *coreclient.Response, value string) error {
	var decoded map[string]any
	if err := json.Unmarshal(response.Body, &decoded); err != nil {
		return fmt.Errorf("decode Core registration response: %w", err)
	}
	data, ok := decoded["data"].(map[string]any)
	if !ok {
		return fmt.Errorf("Core registration response has no data object")
	}
	user, ok := data["user"].(map[string]any)
	if !ok {
		return fmt.Errorf("Core registration response has no user object")
	}
	user["username"] = value
	body, err := json.Marshal(decoded)
	if err != nil {
		return fmt.Errorf("encode registration response: %w", err)
	}
	response.Body = body
	return nil
}
