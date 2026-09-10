package channels

import (
	"database/sql"
	"testing"

	"imgtool/server/internal/db"
	"imgtool/server/internal/secure"
)

func TestChannelServiceWithMemoryStore(t *testing.T) {
	runChannelServiceContract(t, func(t *testing.T) Store {
		t.Helper()
		return NewMemoryStore()
	})
}

func TestChannelServiceWithSQLStore(t *testing.T) {
	runChannelServiceContract(t, func(t *testing.T) Store {
		t.Helper()
		return NewSQLStore(openTestDB(t))
	})
}

func runChannelServiceContract(t *testing.T, newStore func(*testing.T) Store) {
	t.Run("channels are scoped by user", func(t *testing.T) {
		service := NewService(newStore(t), secure.NewSecretBox("test-secret"))
		aliceChannel, err := service.Create("alice", CreateInput{
			Name:    "OpenAI",
			BaseURL: "https://api.openai.com",
			APIKey:  "sk-alice",
			Models:  []ModelInput{{ID: "gpt-image-1", Capabilities: []string{"text-to-image"}}},
		})
		if err != nil {
			t.Fatalf("Create alice channel: %v", err)
		}
		if _, err := service.Create("bob", CreateInput{
			Name:    "OpenAI",
			BaseURL: "https://api.openai.com",
			APIKey:  "sk-bob",
			Models:  []ModelInput{{ID: "gpt-image-1", Capabilities: []string{"text-to-image"}}},
		}); err != nil {
			t.Fatalf("Create bob channel: %v", err)
		}

		aliceChannels, err := service.List("alice")
		if err != nil {
			t.Fatalf("List alice: %v", err)
		}
		if len(aliceChannels) != 1 || aliceChannels[0].ID != aliceChannel.ID {
			t.Fatalf("unexpected alice channels: %#v", aliceChannels)
		}
		if aliceChannels[0].APIKey != "" || !aliceChannels[0].HasAPIKey {
			t.Fatalf("api key leaked or missing flag: %#v", aliceChannels[0])
		}

		if _, err := service.ResolveSnapshot("bob", aliceChannel.ID, "gpt-image-1", "text-to-image"); err == nil {
			t.Fatal("bob resolved alice channel")
		}
	})

	t.Run("resolve snapshot decrypts api key", func(t *testing.T) {
		service := NewService(newStore(t), secure.NewSecretBox("test-secret"))
		channel, err := service.Create("alice", CreateInput{
			Name:    "OpenAI",
			BaseURL: "https://api.openai.com",
			APIKey:  "sk-alice",
			Models:  []ModelInput{{ID: "gpt-image-1", Capabilities: []string{"text-to-image", "image-to-image"}}},
		})
		if err != nil {
			t.Fatalf("Create channel: %v", err)
		}

		snapshot, err := service.ResolveSnapshot("alice", channel.ID, "gpt-image-1", "image-to-image")
		if err != nil {
			t.Fatalf("ResolveSnapshot returned error: %v", err)
		}
		if snapshot.APIKey != "sk-alice" {
			t.Fatalf("APIKey = %q", snapshot.APIKey)
		}
		if snapshot.ChannelName != "OpenAI" || snapshot.ModelID != "gpt-image-1" || snapshot.Capability != "image-to-image" {
			t.Fatalf("unexpected snapshot: %#v", snapshot)
		}
	})

	t.Run("update channel can replace key and models", func(t *testing.T) {
		service := NewService(newStore(t), secure.NewSecretBox("test-secret"))
		channel, err := service.Create("alice", CreateInput{
			Name:    "OpenAI",
			BaseURL: "https://api.openai.com",
			APIKey:  "sk-old",
			Models:  []ModelInput{{ID: "old-model", Capabilities: []string{"text-to-image"}}},
		})
		if err != nil {
			t.Fatalf("Create channel: %v", err)
		}

		updated, err := service.Update("alice", channel.ID, UpdateInput{
			Name:    "Router",
			BaseURL: "https://router.example.com",
			APIKey:  "sk-new",
			Models:  []ModelInput{{ID: "new-model", Capabilities: []string{"image-to-text"}}},
		})
		if err != nil {
			t.Fatalf("Update channel: %v", err)
		}
		if updated.Name != "Router" || len(updated.Models) != 1 || updated.Models[0].ID != "new-model" {
			t.Fatalf("unexpected updated channel: %#v", updated)
		}

		snapshot, err := service.ResolveSnapshot("alice", channel.ID, "new-model", "image-to-text")
		if err != nil {
			t.Fatalf("ResolveSnapshot returned error: %v", err)
		}
		if snapshot.APIKey != "sk-new" {
			t.Fatalf("APIKey = %q", snapshot.APIKey)
		}
	})

	t.Run("channel can be imported without api key before later completion", func(t *testing.T) {
		service := NewService(newStore(t), secure.NewSecretBox("test-secret"))
		channel, err := service.Create("alice", CreateInput{
			Name:    "Imported Router",
			BaseURL: "https://router.example.com",
			APIKey:  "",
			Models:  []ModelInput{{ID: "gpt-image-1", Capabilities: []string{"text-to-image"}}},
		})
		if err != nil {
			t.Fatalf("Create imported channel: %v", err)
		}
		if channel.HasAPIKey {
			t.Fatalf("imported channel unexpectedly has api key: %#v", channel)
		}

		listed, err := service.List("alice")
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(listed) != 1 || listed[0].HasAPIKey {
			t.Fatalf("unexpected listed imported channel: %#v", listed)
		}
		if _, err := service.ResolveSnapshot("alice", channel.ID, "gpt-image-1", "text-to-image"); err == nil {
			t.Fatal("resolved channel without api key")
		}
	})
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
