package secure

import "testing"

func TestSecretBoxRoundTrip(t *testing.T) {
	box := NewSecretBox("test-secret")
	encrypted, err := box.Encrypt("sk-test")
	if err != nil {
		t.Fatalf("Encrypt returned error: %v", err)
	}
	if encrypted == "sk-test" {
		t.Fatal("encrypted value must not equal plaintext")
	}
	decrypted, err := box.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt returned error: %v", err)
	}
	if decrypted != "sk-test" {
		t.Fatalf("decrypted = %q", decrypted)
	}
}

func TestSecretBoxRejectsWrongSecret(t *testing.T) {
	encrypted, err := NewSecretBox("test-secret").Encrypt("sk-test")
	if err != nil {
		t.Fatalf("Encrypt returned error: %v", err)
	}
	if _, err := NewSecretBox("wrong-secret").Decrypt(encrypted); err == nil {
		t.Fatal("expected decrypt with wrong secret to fail")
	}
}
