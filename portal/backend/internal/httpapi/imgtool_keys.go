package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

const (
	defaultImgtoolOpenAIGroupID = int64(15)
	defaultImgtoolGrokGroupID   = int64(33)
	imgtoolKeyListPageSize      = 100
	imgtoolServiceSkew          = 5 * time.Minute
)

type imgtoolProviderSpec struct {
	ID      string
	Label   string
	GroupID int64
	KeyName string
}

type imgtoolProviderCredential struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	GroupID   int64  `json:"group_id"`
	GroupName string `json:"group_name,omitempty"`
	KeyID     int64  `json:"key_id,omitempty"`
	Key       string `json:"key,omitempty"`
}

type imgtoolAdminKeySource interface {
	AdminUserAPIKeys(context.Context, int64) (coreclient.APIKeyPage, error)
}

func imgtoolProviderSpecs() []imgtoolProviderSpec {
	return []imgtoolProviderSpec{
		{ID: "openai", Label: "OpenAI", GroupID: imgtoolGroupID("IMGTOOL_OPENAI_GROUP_ID", defaultImgtoolOpenAIGroupID), KeyName: "生图-OpenAI"},
		{ID: "grok", Label: "Grok", GroupID: imgtoolGroupID("IMGTOOL_GROK_GROUP_ID", defaultImgtoolGrokGroupID), KeyName: "生图-Grok"},
	}
}

func imgtoolGroupID(key string, fallback int64) int64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func imgtoolServiceSecret() string {
	return firstNonEmpty(strings.TrimSpace(os.Getenv("IMGTOOL_SSO_SECRET")), strings.TrimSpace(os.Getenv("SUB2API_IMGTOOL_SSO_SECRET")))
}

func (s *Server) ensureImgtoolKeys(ctx context.Context, accessToken string) {
	if s == nil || s.core == nil || strings.TrimSpace(accessToken) == "" {
		return
	}
	s.imgtoolKeysMu.Lock()
	defer s.imgtoolKeysMu.Unlock()
	keys, err := listUserAPIKeys(ctx, s.core, accessToken)
	if err != nil {
		s.logger.Warn("imgtool key list failed", "error", err)
		keys = nil
	}
	for _, spec := range imgtoolProviderSpecs() {
		if selectImgtoolKey(keys, spec.GroupID) != nil {
			continue
		}
		if err := createUserAPIKey(ctx, s.core, accessToken, spec.KeyName, spec.GroupID); err != nil {
			s.logger.Warn("imgtool key create failed", "provider", spec.ID, "group_id", spec.GroupID, "error", err)
		}
	}
}

func (s *Server) imgtoolCredentials(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to read request body")
		return
	}
	if err := verifyImgtoolServiceRequest(r, body, imgtoolServiceSecret()); err != nil {
		writeAPIError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Image generation service authentication failed")
		return
	}
	var input struct {
		UserID int64 `json:"user_id"`
	}
	if err := json.Unmarshal(body, &input); err != nil || input.UserID <= 0 {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "user_id is required")
		return
	}
	source, ok := s.core.(imgtoolAdminKeySource)
	if !ok {
		writeAPIError(w, http.StatusServiceUnavailable, "IMGTOOL_UNAVAILABLE", "Image generation is not configured")
		return
	}
	page, err := source.AdminUserAPIKeys(r.Context(), input.UserID)
	if err != nil {
		s.logger.Warn("imgtool credential lookup failed", "user_id", input.UserID, "error", err)
		writeAPIError(w, http.StatusBadGateway, "CORE_UNAVAILABLE", "Core API is unavailable")
		return
	}
	providers := make([]imgtoolProviderCredential, 0, 2)
	for _, spec := range imgtoolProviderSpecs() {
		item := imgtoolProviderCredential{ID: spec.ID, Label: spec.Label, GroupID: spec.GroupID}
		if key := selectImgtoolKey(page.Items, spec.GroupID); key != nil {
			item.KeyID = key.ID
			item.Key = key.Key
			if key.Group != nil {
				item.GroupName = key.Group.Name
			}
		}
		providers = append(providers, item)
	}
	writeSuccess(w, map[string]any{"providers": providers})
}

func listUserAPIKeys(ctx context.Context, core coreclient.Adapter, accessToken string) ([]coreclient.APIKey, error) {
	response, err := core.UserAPIRequest(ctx, coreclient.CoreRequest{
		Method:      http.MethodGet,
		Path:        "/api/v1/keys",
		RawQuery:    "page=1&page_size=" + strconv.Itoa(imgtoolKeyListPageSize) + "&sort_by=created_at&sort_order=desc",
		AccessToken: accessToken,
	})
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, errors.New("list api keys returned status " + strconv.Itoa(response.StatusCode))
	}
	var decoded struct {
		Data struct {
			Items []coreclient.APIKey `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body, &decoded); err != nil {
		return nil, err
	}
	return decoded.Data.Items, nil
}

func createUserAPIKey(ctx context.Context, core coreclient.Adapter, accessToken, name string, groupID int64) error {
	payload, err := json.Marshal(map[string]any{"name": name, "group_id": groupID})
	if err != nil {
		return err
	}
	response, err := core.UserAPIRequest(ctx, coreclient.CoreRequest{
		Method:      http.MethodPost,
		Path:        "/api/v1/keys",
		AccessToken: accessToken,
		ContentType: "application/json",
		Body:        payload,
	})
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return errors.New("create api key returned status " + strconv.Itoa(response.StatusCode))
	}
	return nil
}

func selectImgtoolKey(keys []coreclient.APIKey, groupID int64) *coreclient.APIKey {
	var fallback *coreclient.APIKey
	now := time.Now()
	for i := range keys {
		key := &keys[i]
		if key.GroupID == nil || *key.GroupID != groupID {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(key.Status), "active") {
			continue
		}
		if key.ExpiresAt != nil && !key.ExpiresAt.After(now) {
			continue
		}
		if strings.TrimSpace(key.Key) == "" {
			continue
		}
		if fallback == nil {
			fallback = key
		}
		if strings.HasPrefix(key.Name, "生图-") {
			return key
		}
	}
	return fallback
}

func verifyImgtoolServiceRequest(r *http.Request, body []byte, secret string) error {
	if len(secret) < 32 {
		return errors.New("imgtool sso secret must be at least 32 bytes")
	}
	timestampRaw := strings.TrimSpace(r.Header.Get("X-Imgtool-Timestamp"))
	signature := strings.TrimSpace(r.Header.Get("X-Imgtool-Signature"))
	timestamp, err := strconv.ParseInt(timestampRaw, 10, 64)
	if err != nil || timestamp <= 0 {
		return errors.New("missing timestamp")
	}
	if absDuration(time.Since(time.Unix(timestamp, 0))) > imgtoolServiceSkew {
		return errors.New("expired timestamp")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestampRaw))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(strings.ToLower(signature))) && !hmac.Equal([]byte(expected), []byte(signature)) {
		return errors.New("invalid signature")
	}
	return nil
}

func signImgtoolServiceRequest(secret string, timestamp int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strconv.FormatInt(timestamp, 10)))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func absDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}

