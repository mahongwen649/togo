package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type SSOOptions struct {
	Secret   string
	Issuer   string
	Audience string
	Now      func() time.Time
}

type SSOTicketClaims struct {
	Issuer    string `json:"iss"`
	Audience  string `json:"aud"`
	Subject   string `json:"sub"`
	Username  string `json:"username"`
	Email     string `json:"email,omitempty"`
	Role      string `json:"role"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	ID        string `json:"jti"`
}

func VerifySSOTicket(ticket string, opts SSOOptions) (*SSOTicketClaims, error) {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	parts := strings.Split(ticket, ".")
	if len(parts) != 2 {
		return nil, errors.New("invalid ticket format")
	}

	mac := hmac.New(sha256.New, []byte(opts.Secret))
	_, _ = mac.Write([]byte(parts[0]))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return nil, errors.New("invalid ticket signature")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	var claims SSOTicketClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}
	if claims.Issuer != opts.Issuer {
		return nil, errors.New("invalid ticket issuer")
	}
	if claims.Audience != opts.Audience {
		return nil, errors.New("invalid ticket audience")
	}
	if strings.TrimSpace(claims.Subject) == "" {
		return nil, errors.New("missing ticket subject")
	}
	if opts.Now().Unix() > claims.ExpiresAt {
		return nil, errors.New("ticket expired")
	}
	return &claims, nil
}
