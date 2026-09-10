package generation

import (
	"database/sql"
	"testing"

	"imgtool/server/internal/channels"
	"imgtool/server/internal/db"
	"imgtool/server/internal/history"
	"imgtool/server/internal/secure"
)

func TestGenerateImageCreatesHistoryWithFile(t *testing.T) {
	database := openTestDB(t)
	channelService := channels.NewService(channels.NewSQLStore(database), secure.NewSecretBox("test-secret"))
	historyService := history.NewService(database)
	provider := &fakeProvider{images: []ImageResult{{Bytes: []byte("png"), MimeType: "image/png"}}}
	storage := &fakeStorage{}
	service := NewService(channelService, historyService, provider, storage)

	channel, err := channelService.Create("alice", channels.CreateInput{
		Name:    "OpenAI",
		BaseURL: "https://api.openai.com",
		APIKey:  "sk-alice",
		Models:  []channels.ModelInput{{ID: "gpt-image-1", Capabilities: []string{"text-to-image"}}},
	})
	if err != nil {
		t.Fatalf("Create channel: %v", err)
	}

	result, err := service.GenerateImage("alice", ImageRequest{
		ChannelID: channel.ID,
		ModelID:   "gpt-image-1",
		Prompt:    "draw a whale",
		Size:      "1024x1024",
		Count:     1,
	})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if result.TaskID == "" || result.HistoryID == "" || result.DurationMs < 0 || len(result.Files) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Files[0].ObjectKey != "aiImg/result.png" {
		t.Fatalf("object key = %q", result.Files[0].ObjectKey)
	}

	records, err := historyService.ListHistory("alice", 10)
	if err != nil {
		t.Fatalf("ListHistory returned error: %v", err)
	}
	if len(records) != 1 || len(records[0].Files) != 1 {
		t.Fatalf("unexpected records: %#v", records)
	}
}

func TestGenerateImageToImageCreatesHistoryWithReferenceAndResultFiles(t *testing.T) {
	database := openTestDB(t)
	channelService := channels.NewService(channels.NewSQLStore(database), secure.NewSecretBox("test-secret"))
	historyService := history.NewService(database)
	provider := &fakeProvider{images: []ImageResult{{Bytes: []byte("png"), MimeType: "image/png"}}}
	storage := &fakeStorage{}
	service := NewService(channelService, historyService, provider, storage)

	channel, err := channelService.Create("alice", channels.CreateInput{
		Name:    "OpenAI",
		BaseURL: "https://api.openai.com",
		APIKey:  "sk-alice",
		Models:  []channels.ModelInput{{ID: "gpt-image-1", Capabilities: []string{"image-to-image"}}},
	})
	if err != nil {
		t.Fatalf("Create channel: %v", err)
	}

	result, err := service.GenerateImage("alice", ImageRequest{
		ChannelID:  channel.ID,
		ModelID:    "gpt-image-1",
		Prompt:     "change colors",
		Size:       "1024x1024",
		Count:      1,
		Capability: "image-to-image",
		Image:      []byte("reference-bytes"),
		MimeType:   "image/webp",
	})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if len(result.Files) != 2 {
		t.Fatalf("result files = %#v", result.Files)
	}
	if result.Files[0].Role != "reference" || result.Files[0].MimeType != "image/webp" || result.Files[0].SizeBytes != int64(len("reference-bytes")) {
		t.Fatalf("reference file = %#v", result.Files[0])
	}
	if result.Files[1].Role != "result" || result.Files[1].MimeType != "image/png" || result.Files[1].SizeBytes != 3 {
		t.Fatalf("result file = %#v", result.Files[1])
	}

	records, err := historyService.ListHistory("alice", 10)
	if err != nil {
		t.Fatalf("ListHistory returned error: %v", err)
	}
	if len(records) != 1 || len(records[0].Files) != 2 || records[0].Files[0].Role != "reference" || records[0].Files[1].Role != "result" {
		t.Fatalf("unexpected records: %#v", records)
	}
}

type fakeProvider struct {
	images       []ImageResult
	lastSnapshot channels.Snapshot
}

func (p *fakeProvider) GenerateImage(snapshot channels.Snapshot, request ImageRequest) ([]ImageResult, error) {
	p.lastSnapshot = snapshot
	return p.images, nil
}

type fakeStorage struct{}

func (s *fakeStorage) SaveGeneratedImage(input StoreImageInput) (StoredImage, error) {
	role := input.Role
	if role == "" {
		role = "result"
	}
	return StoredImage{
		StorageProvider: "fake",
		Bucket:          "bucket",
		ObjectKey:       "aiImg/" + role + ".png",
		Role:            role,
		MediaType:       "image",
		MimeType:        input.MimeType,
		SizeBytes:       int64(len(input.Bytes)),
	}, nil
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(t.TempDir() + "/imgtool.sqlite")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	for _, id := range []string{"alice", "bob"} {
		_, err := database.Exec(
			`INSERT INTO users (id, username, password_hash, role, created_at, updated_at)
			 VALUES (?, ?, ?, 'user', 1, 1)`,
			id,
			id,
			"test-hash",
		)
		if err != nil {
			t.Fatalf("seed user %s: %v", id, err)
		}
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}
