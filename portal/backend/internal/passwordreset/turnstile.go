package passwordreset

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

type TurnstileVerifier struct {
	secret string
	client *http.Client
}

// DisabledTurnstileVerifier is used when Portal Turnstile protection is not enabled.
type DisabledTurnstileVerifier struct{}

func (DisabledTurnstileVerifier) Verify(context.Context, string, string) error { return nil }

func NewTurnstileVerifier(secret string, client *http.Client) (*TurnstileVerifier, error) {
	if strings.TrimSpace(secret) == "" || client == nil {
		return nil, &Error{Status: 503, Code: "TURNSTILE_NOT_CONFIGURED", Message: "Human verification is unavailable"}
	}
	return &TurnstileVerifier{secret: strings.TrimSpace(secret), client: client}, nil
}

func (v *TurnstileVerifier) Verify(ctx context.Context, token, remoteIP string) error {
	if strings.TrimSpace(token) == "" {
		return &Error{Status: 400, Code: "TURNSTILE_REQUIRED", Message: "Please complete human verification"}
	}
	form := url.Values{"secret": {v.secret}, "response": {token}}
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://challenges.cloudflare.com/turnstile/v0/siteverify", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var result struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	if !result.Success {
		return &Error{Status: 400, Code: "TURNSTILE_INVALID", Message: "Human verification failed"}
	}
	return nil
}
