package providers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"imgtool/server/internal/auth"
	"imgtool/server/internal/channels"
)

var (
	ErrNotConfigured = errors.New("image providers are not configured")
	ErrUnavailable   = errors.New("image provider is unavailable")
	ErrUnknownUser   = errors.New("please enter from the main site")
)

type Options struct {
	PortalBaseURL   string
	SSOSecret       string
	CoreBaseURL     string
	OpenAIGroupID   int64
	GrokGroupID     int64
	Client          *http.Client
	Now             func() time.Time
}

type View struct {
	ID             string                `json:"id"`
	Name           string                `json:"name"`
	Models         []channels.ModelEntry `json:"models"`
	DefaultModelID string                `json:"defaultModelId"`
	Available      bool                  `json:"available"`
	Message        string                `json:"message,omitempty"`
}

type credential struct {
	ID        string
	Label     string
	GroupID   int64
	GroupName string
	Key       string
}

type Service struct {
	opts Options
}

type providerSpec struct {
	ID      string
	Label   string
	GroupID int64
}

func NewService(opts Options) *Service {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Client == nil {
		opts.Client = &http.Client{Timeout: 20 * time.Second}
	}
	if opts.OpenAIGroupID <= 0 {
		opts.OpenAIGroupID = 15
	}
	if opts.GrokGroupID <= 0 {
		opts.GrokGroupID = 33
	}
	opts.PortalBaseURL = strings.TrimRight(strings.TrimSpace(opts.PortalBaseURL), "/")
	opts.CoreBaseURL = strings.TrimRight(strings.TrimSpace(opts.CoreBaseURL), "/")
	return &Service{opts: opts}
}

func (s *Service) Enabled() bool {
	return s != nil && s.opts.PortalBaseURL != "" && len(s.opts.SSOSecret) >= 32 && s.opts.CoreBaseURL != ""
}

func (s *Service) List(ctx context.Context, user auth.User) ([]View, error) {
	coreUserID, err := coreUserID(user)
	if err != nil {
		return nil, err
	}
	creds, err := s.credentials(ctx, coreUserID)
	if err != nil {
		return nil, err
	}
	views := make([]View, 0, 2)
	for _, spec := range s.specs() {
		view := View{ID: spec.ID, Name: spec.Label, Models: []channels.ModelEntry{}}
		cred, ok := creds[spec.ID]
		if !ok || cred.GroupID != spec.GroupID || strings.TrimSpace(cred.Key) == "" {
			view.Message = spec.Label + " 生图暂不可用，请确认已开通对应分组后再试。"
			views = append(views, view)
			continue
		}
		if strings.TrimSpace(cred.GroupName) != "" {
			view.Name = cred.GroupName
		}
		ids, err := s.listModelIDs(ctx, cred.Key)
		if err != nil {
			view.Message = "模型列表读取失败，请稍后重试。"
			views = append(views, view)
			continue
		}
		filtered := filterImageModels(spec.ID, ids)
		if len(filtered) == 0 {
			view.Message = spec.Label + " 暂无可用生图模型。"
			views = append(views, view)
			continue
		}
		models := make([]channels.ModelEntry, 0, len(filtered))
		for _, id := range filtered {
			models = append(models, channels.ModelEntry{ID: id, Capabilities: []string{"text-to-image", "image-to-image"}})
		}
		view.Available = true
		view.Models = models
		view.DefaultModelID = filtered[0]
		views = append(views, view)
	}
	return views, nil
}

func (s *Service) ResolveImage(_ string, coreUserID, providerID, modelID, capability string) (channels.Snapshot, error) {
	if !s.Enabled() {
		return channels.Snapshot{}, ErrNotConfigured
	}
	if strings.TrimSpace(coreUserID) == "" {
		return channels.Snapshot{}, ErrUnknownUser
	}
	providerID = strings.ToLower(strings.TrimSpace(providerID))
	modelID = strings.TrimSpace(modelID)
	capability = strings.TrimSpace(capability)
	if modelID == "" || (capability != "text-to-image" && capability != "image-to-image") {
		return channels.Snapshot{}, ErrUnavailable
	}
	creds, err := s.credentials(context.Background(), coreUserID)
	if err != nil {
		return channels.Snapshot{}, err
	}
	cred, ok := creds[providerID]
	var expected providerSpec
	for _, spec := range s.specs() {
		if spec.ID == providerID {
			expected = spec
			break
		}
	}
	if !ok || expected.ID == "" || expected.GroupID <= 0 || cred.GroupID != expected.GroupID || strings.TrimSpace(cred.Key) == "" {
		return channels.Snapshot{}, ErrUnavailable
	}
	modelIDs, err := s.listModelIDs(context.Background(), cred.Key)
	if err != nil || !containsModel(filterImageModels(providerID, modelIDs), modelID) {
		return channels.Snapshot{}, ErrUnavailable
	}
	label := cred.Label
	if label == "" {
		label = providerID
	}
	return channels.Snapshot{
		ChannelName: label,
		BaseURL:     s.opts.CoreBaseURL,
		APIKey:      cred.Key,
		ModelID:     modelID,
		Capability:  capability,
	}, nil
}

func containsModel(models []string, wanted string) bool {
	wanted = normalizeModelID(wanted)
	for _, model := range models {
		if normalizeModelID(model) == wanted {
			return true
		}
	}
	return false
}

func (s *Service) specs() []providerSpec {
	return []providerSpec{
		{ID: "openai", Label: "OpenAI", GroupID: s.opts.OpenAIGroupID},
		{ID: "grok", Label: "Grok", GroupID: s.opts.GrokGroupID},
	}
}

func (s *Service) credentials(ctx context.Context, coreUserID string) (map[string]credential, error) {
	if !s.Enabled() {
		return nil, ErrNotConfigured
	}
	userID, err := strconv.ParseInt(strings.TrimSpace(coreUserID), 10, 64)
	if err != nil || userID <= 0 {
		return nil, ErrUnknownUser
	}
	payload, err := json.Marshal(map[string]any{"user_id": userID})
	if err != nil {
		return nil, err
	}
	timestamp := s.opts.Now().Unix()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.opts.PortalBaseURL+"/api/v1/integrations/imgtool/credentials", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Imgtool-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-Imgtool-Signature", sign(s.opts.SSOSecret, timestamp, payload))
	res, err := s.opts.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("portal credentials status %d", res.StatusCode)
	}
	var decoded struct {
		Data struct {
			Providers []struct {
				ID        string `json:"id"`
				Label     string `json:"label"`
				GroupID   int64  `json:"group_id"`
				GroupName string `json:"group_name"`
				Key       string `json:"key"`
			} `json:"providers"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, err
	}
	next := map[string]credential{}
	for _, item := range decoded.Data.Providers {
		next[item.ID] = credential{ID: item.ID, Label: item.Label, GroupID: item.GroupID, GroupName: item.GroupName, Key: item.Key}
	}
	return next, nil
}

func (s *Service) listModelIDs(ctx context.Context, apiKey string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.opts.CoreBaseURL+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	res, err := s.opts.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("models status %d", res.StatusCode)
	}
	return parseModelIDs(body), nil
}

func parseModelIDs(body []byte) []string {
	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil
	}
	ids := []string{}
	collectModelIDs(decoded, &ids, 0)
	return ids
}

func collectModelIDs(value any, ids *[]string, depth int) {
	if depth > 8 {
		return
	}
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			collectModelIDs(item, ids, depth+1)
		}
	case map[string]any:
		if id, ok := typed["id"].(string); ok && strings.TrimSpace(id) != "" {
			*ids = append(*ids, id)
		}
		if data, ok := typed["data"]; ok {
			collectModelIDs(data, ids, depth+1)
		}
	}
}

func coreUserID(user auth.User) (string, error) {
	if user.ExternalProvider == "sub2api" && strings.TrimSpace(user.ExternalSubject) != "" {
		return strings.TrimSpace(user.ExternalSubject), nil
	}
	return "", ErrUnknownUser
}

func sign(secret string, timestamp int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strconv.FormatInt(timestamp, 10)))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
