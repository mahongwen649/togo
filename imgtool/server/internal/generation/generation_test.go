package generation

import (
	"database/sql"
	"testing"

	"imgtool/server/internal/channels"
	"imgtool/server/internal/db"
	"imgtool/server/internal/history"
	"imgtool/server/internal/secure"
)

func TestGenerateTextCreatesHistoryRecord(t *testing.T) {
	database := openTestDB(t)
	channelService := channels.NewService(channels.NewSQLStore(database), secure.NewSecretBox("test-secret"))
	historyService := history.NewService(database)
	provider := &fakeProvider{text: "caption result"}
	service := NewService(channelService, historyService, provider)

	channel, err := channelService.Create("alice", channels.CreateInput{
		Name:    "OpenAI",
		BaseURL: "https://api.openai.com",
		APIKey:  "sk-alice",
		Models:  []channels.ModelInput{{ID: "gpt-4o", Capabilities: []string{"image-to-text"}}},
	})
	if err != nil {
		t.Fatalf("Create channel: %v", err)
	}

	result, err := service.GenerateText("alice", TextRequest{
		ChannelID: channel.ID,
		ModelID:   "gpt-4o",
		Prompt:    "describe this",
		Image:     []byte("fake-image"),
		MimeType:  "image/png",
	})
	if err != nil {
		t.Fatalf("GenerateText returned error: %v", err)
	}
	if result.ResultText != "caption result" || result.HistoryID == "" || result.TaskID == "" || result.DurationMs < 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if provider.lastSnapshot.APIKey != "sk-alice" {
		t.Fatalf("provider APIKey = %q", provider.lastSnapshot.APIKey)
	}

	records, err := historyService.ListHistory("alice", 10)
	if err != nil {
		t.Fatalf("ListHistory returned error: %v", err)
	}
	if len(records) != 1 || records[0].ResultText != "caption result" {
		t.Fatalf("unexpected records: %#v", records)
	}
}

func TestGenerateTextRejectsOtherUsersChannel(t *testing.T) {
	database := openTestDB(t)
	channelService := channels.NewService(channels.NewSQLStore(database), secure.NewSecretBox("test-secret"))
	service := NewService(channelService, history.NewService(database), &fakeProvider{text: "caption result"})

	channel, err := channelService.Create("alice", channels.CreateInput{
		Name:    "OpenAI",
		BaseURL: "https://api.openai.com",
		APIKey:  "sk-alice",
		Models:  []channels.ModelInput{{ID: "gpt-4o", Capabilities: []string{"image-to-text"}}},
	})
	if err != nil {
		t.Fatalf("Create channel: %v", err)
	}

	if _, err := service.GenerateText("bob", TextRequest{
		ChannelID: channel.ID,
		ModelID:   "gpt-4o",
		Prompt:    "describe this",
		Image:     []byte("fake-image"),
		MimeType:  "image/png",
	}); err == nil {
		t.Fatal("bob generated with alice channel")
	}
}

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
	text         string
	images       []ImageResult
	lastSnapshot channels.Snapshot
}

func (p *fakeProvider) GenerateText(snapshot channels.Snapshot, request TextRequest) (string, error) {
	p.lastSnapshot = snapshot
	return p.text, nil
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
