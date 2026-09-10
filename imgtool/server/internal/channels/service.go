package channels

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/url"
	"sort"
	"strings"
	"time"

	"imgtool/server/internal/secure"
)

var ErrInvalidChannel = errors.New("invalid channel")
var ErrMissingAPIKey = errors.New("missing api key")

type Service struct {
	store Store
	box   secure.SecretBox
}

func NewService(store Store, box secure.SecretBox) *Service {
	return &Service{store: store, box: box}
}

func (s *Service) Create(userID string, input CreateInput) (Channel, error) {
	channel, err := s.channelFromInput(userID, "", input.Name, input.BaseURL, input.APIKey, input.Models)
	if err != nil {
		return Channel{}, err
	}
	now := nowMillis()
	channel.ID = newID("chn")
	channel.CreatedAt = now
	channel.UpdatedAt = now
	created, err := s.store.Create(channel)
	if err != nil {
		return Channel{}, err
	}
	return publicChannel(created), nil
}

func (s *Service) Update(userID string, id string, input UpdateInput) (Channel, error) {
	existing, err := s.store.ByID(userID, id)
	if err != nil {
		return Channel{}, err
	}
	apiKey := input.APIKey
	if strings.TrimSpace(apiKey) == "" {
		apiKey = existing.APIKey
	}
	if apiKey == "" && existing.EncryptedAPIKey != "" {
		apiKey, err = s.box.Decrypt(existing.EncryptedAPIKey)
		if err != nil {
			return Channel{}, err
		}
	}
	channel, err := s.channelFromInput(userID, id, input.Name, input.BaseURL, apiKey, input.Models)
	if err != nil {
		return Channel{}, err
	}
	channel.CreatedAt = existing.CreatedAt
	channel.UpdatedAt = nowMillis()
	updated, err := s.store.Update(channel)
	if err != nil {
		return Channel{}, err
	}
	return publicChannel(updated), nil
}

func (s *Service) Delete(userID string, id string) error {
	return s.store.Delete(userID, id)
}

func (s *Service) List(userID string) ([]Channel, error) {
	channels, err := s.store.List(userID)
	if err != nil {
		return nil, err
	}
	sort.Slice(channels, func(i, j int) bool {
		return channels[i].CreatedAt < channels[j].CreatedAt
	})
	for index := range channels {
		channels[index] = publicChannel(channels[index])
	}
	return channels, nil
}

func (s *Service) ResolveSnapshot(userID string, channelID string, modelID string, capability string) (Snapshot, error) {
	channel, err := s.store.ByID(userID, channelID)
	if err != nil {
		return Snapshot{}, err
	}
	model, ok := findModel(channel.Models, modelID)
	if !ok || !hasCapability(model, capability) {
		return Snapshot{}, ErrInvalidChannel
	}
	if strings.TrimSpace(channel.EncryptedAPIKey) == "" {
		return Snapshot{}, ErrMissingAPIKey
	}
	apiKey, err := s.box.Decrypt(channel.EncryptedAPIKey)
	if err != nil {
		return Snapshot{}, err
	}
	if strings.TrimSpace(apiKey) == "" {
		return Snapshot{}, ErrMissingAPIKey
	}
	return Snapshot{
		ChannelName: channel.Name,
		BaseURL:     channel.BaseURL,
		APIKey:      apiKey,
		ModelID:     model.ID,
		Capability:  capability,
	}, nil
}

func (s *Service) channelFromInput(userID string, id string, name string, baseURL string, apiKey string, models []ModelInput) (Channel, error) {
	name = strings.TrimSpace(name)
	baseURL = strings.TrimSpace(baseURL)
	apiKey = strings.TrimSpace(apiKey)
	if userID == "" || name == "" || baseURL == "" || len(models) == 0 {
		return Channel{}, ErrInvalidChannel
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return Channel{}, ErrInvalidChannel
	}
	encrypted := ""
	if apiKey != "" {
		var err error
		encrypted, err = s.box.Encrypt(apiKey)
		if err != nil {
			return Channel{}, err
		}
	}
	return Channel{
		ID:              id,
		UserID:          userID,
		Name:            name,
		BaseURL:         baseURL,
		APIKey:          apiKey,
		EncryptedAPIKey: encrypted,
		HasAPIKey:       encrypted != "",
		Models:          normalizeModels(models),
	}, nil
}

func normalizeModels(inputs []ModelInput) []ModelEntry {
	models := []ModelEntry{}
	seen := map[string]bool{}
	for _, input := range inputs {
		id := strings.TrimSpace(input.ID)
		if id == "" || seen[id] {
			continue
		}
		capabilities := normalizeCapabilities(input.Capabilities)
		if len(capabilities) == 0 {
			capabilities = []string{"text-to-image", "image-to-image", "image-to-text"}
		}
		models = append(models, ModelEntry{ID: id, Capabilities: capabilities})
		seen[id] = true
	}
	return models
}

func normalizeCapabilities(inputs []string) []string {
	allowed := map[string]bool{
		"text-to-image": true,
		"image-to-image": true,
		"image-to-text": true,
	}
	capabilities := []string{}
	seen := map[string]bool{}
	for _, input := range inputs {
		value := strings.TrimSpace(input)
		if allowed[value] && !seen[value] {
			capabilities = append(capabilities, value)
			seen[value] = true
		}
	}
	return capabilities
}

func findModel(models []ModelEntry, modelID string) (ModelEntry, bool) {
	for _, model := range models {
		if model.ID == modelID {
			return model, true
		}
	}
	return ModelEntry{}, false
}

func hasCapability(model ModelEntry, capability string) bool {
	for _, item := range model.Capabilities {
		if item == capability {
			return true
		}
	}
	return false
}

func publicChannel(channel Channel) Channel {
	channel.APIKey = ""
	channel.EncryptedAPIKey = ""
	channel.UserID = ""
	return channel
}

func newID(prefix string) string {
	return prefix + "_" + randomToken(18)
}

func randomToken(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes)
}

func nowMillis() int64 {
	return time.Now().UnixMilli()
}
