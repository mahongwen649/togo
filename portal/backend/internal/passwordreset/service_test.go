package passwordreset

import (
	"testing"
)

func TestNormalizeEmail(t *testing.T) {
	got, err := normalizeEmail(" User@Example.COM ")
	if err != nil || got != "user@example.com" {
		t.Fatalf("email=%q err=%v", got, err)
	}
	if _, err := normalizeEmail("not-an-email"); err == nil {
		t.Fatal("expected invalid email")
	}
}

func TestGenerateCodeIsSixDigits(t *testing.T) {
	for range 100 {
		code, err := generateCode()
		if err != nil || len(code) != 6 {
			t.Fatalf("code=%q err=%v", code, err)
		}
		for _, digit := range code {
			if digit < '0' || digit > '9' {
				t.Fatalf("non-digit code=%q", code)
			}
		}
	}
}

func TestDigestBindsCodeToEmail(t *testing.T) {
	service := &Service{secret: []byte("01234567890123456789012345678901")}
	first := service.digest("one@example.com", "123456")
	if string(first) == string(service.digest("two@example.com", "123456")) || string(first) == string(service.digest("one@example.com", "654321")) {
		t.Fatal("digest must bind both email and code")
	}
}
