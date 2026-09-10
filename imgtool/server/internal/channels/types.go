package channels

type Channel struct {
	ID              string       `json:"id"`
	UserID          string       `json:"-"`
	Name            string       `json:"name"`
	BaseURL         string       `json:"baseUrl"`
	APIKey          string       `json:"-"`
	EncryptedAPIKey string       `json:"-"`
	HasAPIKey       bool         `json:"hasApiKey"`
	Models          []ModelEntry `json:"models"`
	CreatedAt       int64        `json:"createdAt"`
	UpdatedAt       int64        `json:"updatedAt"`
}

type ModelEntry struct {
	ID           string   `json:"id"`
	Capabilities []string `json:"capabilities"`
}

type ModelInput struct {
	ID           string   `json:"id"`
	Capabilities []string `json:"capabilities"`
}

type CreateInput struct {
	Name    string       `json:"name"`
	BaseURL string       `json:"baseUrl"`
	APIKey  string       `json:"apiKey"`
	Models  []ModelInput `json:"models"`
}

type UpdateInput struct {
	Name    string       `json:"name"`
	BaseURL string       `json:"baseUrl"`
	APIKey  string       `json:"apiKey"`
	Models  []ModelInput `json:"models"`
}

type Snapshot struct {
	ChannelName string
	BaseURL     string
	APIKey      string
	ModelID     string
	Capability  string
}
