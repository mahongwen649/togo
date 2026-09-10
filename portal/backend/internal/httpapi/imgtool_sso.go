package httpapi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

type imgtoolSSOOptions struct {
	Secret   string
	Issuer   string
	Audience string
	TTL      time.Duration
	Now      func() time.Time
}

type imgtoolSSOUser struct {
	ID       int64
	Username string
	Email    string
	Role     string
}

type imgtoolSSOTicket struct {
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

func buildImgtoolSSORedirectURL(user coreclient.User) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("IMGTOOL_BASE_URL")), "/")
	if baseURL == "" {
		return "", errors.New("missing imgtool base url")
	}
	signer := newImgtoolSSOSigner(imgtoolSSOOptions{
		Secret:   firstNonEmpty(strings.TrimSpace(os.Getenv("IMGTOOL_SSO_SECRET")), strings.TrimSpace(os.Getenv("SUB2API_IMGTOOL_SSO_SECRET"))),
		Issuer:   "sub2api",
		Audience: "imgtool",
		TTL:      2 * time.Minute,
		Now:      time.Now,
	})
	ticket, err := signer.Sign(imgtoolSSOUser{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
	})
	if err != nil {
		return "", err
	}
	u, err := url.Parse(baseURL + "/sso")
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("ticket", ticket)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func newImgtoolSSOSigner(opts imgtoolSSOOptions) *imgtoolSSOSigner {
	if opts.Issuer == "" {
		opts.Issuer = "sub2api"
	}
	if opts.Audience == "" {
		opts.Audience = "imgtool"
	}
	if opts.TTL == 0 {
		opts.TTL = 2 * time.Minute
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &imgtoolSSOSigner{opts: opts}
}

type imgtoolSSOSigner struct {
	opts imgtoolSSOOptions
}

func (s *imgtoolSSOSigner) Sign(user imgtoolSSOUser) (string, error) {
	if len([]byte(s.opts.Secret)) < 32 {
		return "", errors.New("imgtool sso secret must be at least 32 bytes")
	}
	now := s.opts.Now().UTC()
	payload := imgtoolSSOTicket{
		Issuer:    s.opts.Issuer,
		Audience:  s.opts.Audience,
		Subject:   strconv.FormatInt(user.ID, 10),
		Username:  user.Username,
		Email:     user.Email,
		Role:      user.Role,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(s.opts.TTL).Unix(),
		ID:        randomImgtoolSSOJTI(),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	encodedBody := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, []byte(s.opts.Secret))
	_, _ = mac.Write([]byte(encodedBody))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encodedBody + "." + signature, nil
}

func randomImgtoolSSOJTI() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf[:])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
