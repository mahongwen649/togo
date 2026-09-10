package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"testing"
	"time"
)

func TestVerifySSOTicketAcceptsValidTicket(t *testing.T) {
	ticket := mustSignTestTicket(t, "01234567890123456789012345678901", 42, time.Unix(1000, 0), time.Unix(1120, 0))

	claims, err := VerifySSOTicket(ticket, SSOOptions{
		Secret:   "01234567890123456789012345678901",
		Issuer:   "sub2api",
		Audience: "imgtool",
		Now:      func() time.Time { return time.Unix(1001, 0) },
	})
	if err != nil {
		t.Fatalf("VerifySSOTicket returned error: %v", err)
	}
	if claims.Subject != "42" {
		t.Fatalf("subject = %q, want 42", claims.Subject)
	}
}

func TestVerifySSOTicketRejectsExpiredTicket(t *testing.T) {
	ticket := mustSignTestTicket(t, "01234567890123456789012345678901", 42, time.Unix(1000, 0), time.Unix(1120, 0))

	_, err := VerifySSOTicket(ticket, SSOOptions{
		Secret:   "01234567890123456789012345678901",
		Issuer:   "sub2api",
		Audience: "imgtool",
		Now:      func() time.Time { return time.Unix(1200, 0) },
	})
	if err == nil {
		t.Fatal("expected expired ticket error")
	}
}

func TestVerifySSOTicketRejectsBadSignature(t *testing.T) {
	ticket := mustSignTestTicket(t, "01234567890123456789012345678901", 42, time.Unix(1000, 0), time.Unix(1120, 0))

	_, err := VerifySSOTicket(ticket, SSOOptions{
		Secret:   "badbadbadbadbadbadbadbadbadbadbadbad",
		Issuer:   "sub2api",
		Audience: "imgtool",
		Now:      func() time.Time { return time.Unix(1001, 0) },
	})
	if err == nil {
		t.Fatal("expected bad signature error")
	}
}

func mustSignTestTicket(t *testing.T, secret string, userID int64, issuedAt time.Time, expiresAt time.Time) string {
	t.Helper()
	payload := SSOTicketClaims{
		Issuer:    "sub2api",
		Audience:  "imgtool",
		Subject:   strconv.FormatInt(userID, 10),
		Username:  "alice",
		Email:     "alice@example.com",
		Role:      "user",
		IssuedAt:  issuedAt.Unix(),
		ExpiresAt: expiresAt.Unix(),
		ID:        "test-ticket-id",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal ticket: %v", err)
	}
	encodedBody := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(encodedBody))
	return encodedBody + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
