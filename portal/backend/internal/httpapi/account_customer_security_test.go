package httpapi

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSanitizeCustomerExtraKeepsOpenAIResetMetadata(t *testing.T) {
	clean := sanitizeCustomerExtra(map[string]any{
		"codex_reset_credit_snapshot": map[string]any{
			"available_count": float64(2),
			"credits":         []any{map[string]any{"expires_at": "2099-01-01T00:00:00Z"}},
		},
		"api_access_token": "must-not-leak",
		"proxy_id":         float64(9),
	})
	if clean["codex_reset_credit_snapshot"] == nil {
		t.Fatal("OpenAI reset-credit snapshot was removed")
	}
	if _, ok := clean["api_access_token"]; ok {
		t.Fatal("access token was not removed")
	}
	if _, ok := clean["proxy_id"]; ok {
		t.Fatal("proxy metadata was not removed")
	}
}

func TestSanitizeCustomerPayloadRemovesRawAccountSecrets(t *testing.T) {
	clean, ok := sanitizeCustomerPayload(map[string]any{
		"account": map[string]any{
			"id": float64(3),
			"credentials": map[string]any{
				"account_mode":            "coding",
				"auth_mode":               "agentIdentity",
				"tier_id":                 "vertex",
				"plan_type":               "plus",
				"subscription_expires_at": "2026-10-13T00:00:00Z",
				"access_token":            "must-not-leak",
			},
			"extra": map[string]any{
				"codex_reset_credit_snapshot": map[string]any{"available_count": float64(1)},
				"proxy_url":                   "http://must-not-leak",
			},
		},
	}).(map[string]any)
	if !ok {
		t.Fatal("sanitized payload has unexpected shape")
	}
	account, ok := clean["account"].(map[string]any)
	if !ok {
		t.Fatal("sanitized account has unexpected shape")
	}
	credentials, ok := account["credentials"].(map[string]any)
	if !ok {
		t.Fatal("safe credential metadata was removed from action response")
	}
	if credentials["account_mode"] != "coding" || credentials["auth_mode"] != "agentIdentity" || credentials["tier_id"] != "vertex" || credentials["plan_type"] != "plus" || credentials["subscription_expires_at"] != "2026-10-13T00:00:00Z" {
		t.Fatalf("safe credential metadata was not preserved: %+v", credentials)
	}
	if _, ok := credentials["access_token"]; ok {
		t.Fatal("access token was not removed from action response")
	}
	extra, ok := account["extra"].(map[string]any)
	if !ok {
		t.Fatal("sanitized extra has unexpected shape")
	}
	if extra["codex_reset_credit_snapshot"] == nil {
		t.Fatal("safe reset-credit metadata was removed from action response")
	}
	if _, ok := extra["proxy_url"]; ok {
		t.Fatal("proxy metadata was not removed from action response")
	}
}

func TestCustomerAccountSerializationOmitsSecretsAndPreservesUsageCounters(t *testing.T) {
	account := sanitizeCustomerAccount(customerAccount{
		Credentials: map[string]any{
			"account_mode":            "coding",
			"auth_mode":               "agentIdentity",
			"tier_id":                 "vertex",
			"plan_type":               "plus",
			"subscription_expires_at": "2026-10-13T00:00:00Z",
			"api_key":                 "must-not-leak",
		},
		Extra: map[string]any{
			"proxy_url": "http://must-not-leak",
			"codex_reset_credit_snapshot": map[string]any{
				"available_count": float64(2),
			},
		},
		Usage: map[string]any{
			"input_tokens": float64(12),
			"total_tokens": float64(18),
			"access_token": "must-not-leak",
			"group_id":     float64(9),
		},
	})
	body, err := json.Marshal(account)
	if err != nil {
		t.Fatal(err)
	}
	serialized := string(body)
	for _, forbidden := range []string{"api_key", "proxy_url", "access_token", "group_id"} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("restricted field %q leaked: %s", forbidden, serialized)
		}
	}
	for _, required := range []string{"account_mode", "auth_mode", "tier_id", "plan_type", "subscription_expires_at"} {
		if !strings.Contains(serialized, required) {
			t.Fatalf("required account metadata %q was removed: %s", required, serialized)
		}
	}
	for _, required := range []string{"input_tokens", "total_tokens", "codex_reset_credit_snapshot"} {
		if !strings.Contains(serialized, required) {
			t.Fatalf("required usage metadata %q was removed: %s", required, serialized)
		}
	}
}

func TestCustomerTestStreamRemovesSecretsAndKeepsEvents(t *testing.T) {
	body := []byte("event: result\r\ndata: {\"credentials\":{\"access_token\":\"secret\"},\r\ndata: \"ok\":true}\r\n\r\ndata: [DONE]\r\n\r\n")
	clean, err := sanitizeCustomerTestBody(body)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(clean), "secret") || strings.Contains(string(clean), "credentials") {
		t.Fatalf("secret leaked: %s", clean)
	}
	if !strings.Contains(string(clean), "event: result\ndata: {\"ok\":true}") || !strings.Contains(string(clean), "data: [DONE]\n\n") {
		t.Fatalf("events damaged: %s", clean)
	}
	if _, err := sanitizeCustomerTestBody([]byte("data: invalid upstream response\n\n")); err == nil {
		t.Fatal("unvalidated data exposed")
	}
}
