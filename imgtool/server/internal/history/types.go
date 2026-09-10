package history

type TaskInput struct {
	Kind        string
	ChannelID   string
	ChannelName string
	BaseURL     string
	ModelID     string
	Capability  string
	Prompt      string
	Parameters  map[string]any
}

type CompleteInput struct {
	Status        string
	ErrorSource   string
	FailureReason string
	ResultText    string
	Files         []FileInput
}

type FileInput struct {
	StorageProvider string
	Bucket          string
	ObjectKey       string
	Role            string
	MediaType       string
	MimeType        string
	SizeBytes       int64
}

type Task struct {
	ID            string         `json:"id"`
	UserID        string         `json:"-"`
	Kind          string         `json:"kind"`
	Status        string         `json:"status"`
	ChannelID     string         `json:"channelId"`
	ChannelName   string         `json:"channelName"`
	BaseURL       string         `json:"baseUrl"`
	ModelID       string         `json:"modelId"`
	Capability    string         `json:"capability"`
	Prompt        string         `json:"prompt"`
	Parameters    map[string]any `json:"parameters"`
	StartedAt     int64          `json:"startedAt"`
	EndedAt       int64          `json:"endedAt,omitempty"`
	DurationMs    int64          `json:"durationMs,omitempty"`
	ErrorSource   string         `json:"errorSource,omitempty"`
	FailureReason string         `json:"failureReason,omitempty"`
	CreatedAt     int64          `json:"createdAt"`
	UpdatedAt     int64          `json:"updatedAt"`
}

type Record struct {
	ID            string         `json:"id"`
	UserID        string         `json:"-"`
	TaskID        string         `json:"taskId"`
	Kind          string         `json:"kind"`
	Status        string         `json:"status"`
	ChannelID     string         `json:"channelId"`
	ChannelName   string         `json:"channelName"`
	BaseURL       string         `json:"baseUrl"`
	ModelID       string         `json:"modelId"`
	Capability    string         `json:"capability"`
	Prompt        string         `json:"prompt"`
	Parameters    map[string]any `json:"parameters"`
	StartedAt     int64          `json:"startedAt"`
	EndedAt       int64          `json:"endedAt"`
	DurationMs    int64          `json:"durationMs"`
	ErrorSource   string         `json:"errorSource,omitempty"`
	FailureReason string         `json:"failureReason,omitempty"`
	ResultText    string         `json:"resultText,omitempty"`
	Files         []File         `json:"files"`
	CreatedAt     int64          `json:"createdAt"`
}

type File struct {
	ID              string `json:"id"`
	StorageProvider string `json:"storageProvider"`
	Bucket          string `json:"bucket"`
	ObjectKey       string `json:"objectKey"`
	Role            string `json:"role"`
	MediaType       string `json:"mediaType"`
	MimeType        string `json:"mimeType"`
	SizeBytes       int64  `json:"sizeBytes"`
	CreatedAt       int64  `json:"createdAt"`
}

type ExpiredRecord struct {
	UserID    string
	HistoryID string
	Files     []File
}
