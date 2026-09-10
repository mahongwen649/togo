package history

import (
	"database/sql"
	"testing"

	"imgtool/server/internal/db"
)

func TestHistoryServiceCompletesTextTaskForOwner(t *testing.T) {
	service := NewService(openTestDB(t))
	task, err := service.CreateTask("alice", TaskInput{
		Kind:        "text",
		ChannelID:   "chn_alice",
		ChannelName: "OpenAI",
		BaseURL:     "https://api.openai.com",
		ModelID:     "gpt-4o",
		Capability:  "image-to-text",
		Prompt:      "describe image",
		Parameters:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("CreateTask returned error: %v", err)
	}

	record, err := service.CompleteTask("alice", task.ID, CompleteInput{
		Status:     "完成",
		ResultText: "caption",
	})
	if err != nil {
		t.Fatalf("CompleteTask returned error: %v", err)
	}
	if record.UserID != "alice" || record.TaskID != task.ID || record.ResultText != "caption" {
		t.Fatalf("unexpected record: %#v", record)
	}
}

func TestHistoryIsScopedByUser(t *testing.T) {
	service := NewService(openTestDB(t))
	aliceTask, err := service.CreateTask("alice", TaskInput{
		Kind:        "text",
		ChannelID:   "chn_alice",
		ChannelName: "OpenAI",
		BaseURL:     "https://api.openai.com",
		ModelID:     "gpt-4o",
		Capability:  "image-to-text",
		Prompt:      "alice prompt",
		Parameters:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("CreateTask alice returned error: %v", err)
	}
	if _, err := service.CompleteTask("alice", aliceTask.ID, CompleteInput{Status: "完成", ResultText: "alice result"}); err != nil {
		t.Fatalf("CompleteTask alice returned error: %v", err)
	}
	bobTask, err := service.CreateTask("bob", TaskInput{
		Kind:        "text",
		ChannelID:   "chn_bob",
		ChannelName: "OpenAI",
		BaseURL:     "https://api.openai.com",
		ModelID:     "gpt-4o",
		Capability:  "image-to-text",
		Prompt:      "bob prompt",
		Parameters:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("CreateTask bob returned error: %v", err)
	}
	if _, err := service.CompleteTask("bob", bobTask.ID, CompleteInput{Status: "完成", ResultText: "bob result"}); err != nil {
		t.Fatalf("CompleteTask bob returned error: %v", err)
	}

	aliceRecords, err := service.ListHistory("alice", 20)
	if err != nil {
		t.Fatalf("ListHistory alice returned error: %v", err)
	}
	if len(aliceRecords) != 1 || aliceRecords[0].Prompt != "alice prompt" {
		t.Fatalf("unexpected alice records: %#v", aliceRecords)
	}
	if _, err := service.GetTask("bob", aliceTask.ID); err == nil {
		t.Fatal("bob read alice task")
	}
}

func TestListHistoryExcludesFailedRecords(t *testing.T) {
	service := NewService(openTestDB(t))
	doneTask, err := service.CreateTask("alice", TaskInput{
		Kind:        "image",
		ChannelID:   "chn_alice",
		ChannelName: "OpenAI",
		BaseURL:     "https://api.openai.com",
		ModelID:     "gpt-image-1",
		Capability:  "text-to-image",
		Prompt:      "finished prompt",
		Parameters:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("CreateTask done returned error: %v", err)
	}
	if _, err := service.CompleteTask("alice", doneTask.ID, CompleteInput{Status: "完成"}); err != nil {
		t.Fatalf("CompleteTask done returned error: %v", err)
	}
	failedTask, err := service.CreateTask("alice", TaskInput{
		Kind:        "image",
		ChannelID:   "chn_alice",
		ChannelName: "OpenAI",
		BaseURL:     "https://api.openai.com",
		ModelID:     "gpt-image-1",
		Capability:  "text-to-image",
		Prompt:      "failed prompt",
		Parameters:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("CreateTask failed returned error: %v", err)
	}
	if _, err := service.CompleteTask("alice", failedTask.ID, CompleteInput{Status: "失败", FailureReason: "upstream error"}); err != nil {
		t.Fatalf("CompleteTask failed returned error: %v", err)
	}

	records, err := service.ListHistory("alice", 20)
	if err != nil {
		t.Fatalf("ListHistory returned error: %v", err)
	}
	if len(records) != 1 || records[0].Prompt != "finished prompt" {
		t.Fatalf("unexpected records: %#v", records)
	}
}

func TestListRunningTasksIsScopedByUser(t *testing.T) {
	service := NewService(openTestDB(t))
	aliceRunning, err := service.CreateTask("alice", TaskInput{
		Kind:        "image",
		ChannelID:   "chn_alice",
		ChannelName: "OpenAI",
		BaseURL:     "https://api.openai.com",
		ModelID:     "gpt-image-1",
		Capability:  "text-to-image",
		Prompt:      "alice running",
		Parameters:  map[string]any{"size": "1024x1024"},
	})
	if err != nil {
		t.Fatalf("CreateTask alice running returned error: %v", err)
	}
	aliceDone, err := service.CreateTask("alice", TaskInput{
		Kind:        "image",
		ChannelID:   "chn_alice",
		ChannelName: "OpenAI",
		BaseURL:     "https://api.openai.com",
		ModelID:     "gpt-image-1",
		Capability:  "text-to-image",
		Prompt:      "alice done",
		Parameters:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("CreateTask alice done returned error: %v", err)
	}
	if _, err := service.CompleteTask("alice", aliceDone.ID, CompleteInput{Status: "完成"}); err != nil {
		t.Fatalf("CompleteTask alice done returned error: %v", err)
	}
	if _, err := service.CreateTask("bob", TaskInput{
		Kind:        "image",
		ChannelID:   "chn_bob",
		ChannelName: "OpenAI",
		BaseURL:     "https://api.openai.com",
		ModelID:     "gpt-image-1",
		Capability:  "text-to-image",
		Prompt:      "bob running",
		Parameters:  map[string]any{},
	}); err != nil {
		t.Fatalf("CreateTask bob running returned error: %v", err)
	}

	tasks, err := service.ListTasks("alice", "running", 20)
	if err != nil {
		t.Fatalf("ListTasks returned error: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != aliceRunning.ID || tasks[0].Prompt != "alice running" {
		t.Fatalf("unexpected running tasks: %#v", tasks)
	}
}

func TestListRunningTasksExpiresStaleTasks(t *testing.T) {
	service := NewService(openTestDB(t))
	task, err := service.CreateTask("alice", TaskInput{
		Kind:        "image",
		ChannelID:   "chn_alice",
		ChannelName: "OpenAI",
		BaseURL:     "https://api.openai.com",
		ModelID:     "gpt-image-1",
		Capability:  "text-to-image",
		Prompt:      "stale running",
		Parameters:  map[string]any{"size": "1024x1024"},
	})
	if err != nil {
		t.Fatalf("CreateTask returned error: %v", err)
	}
	staleStartedAt := nowMillis() - int64(31*60*1000)
	if _, err := service.db.Exec(
		`UPDATE tasks SET started_at = ?, created_at = ?, updated_at = ? WHERE id = ?`,
		staleStartedAt,
		staleStartedAt,
		staleStartedAt,
		task.ID,
	); err != nil {
		t.Fatalf("age task: %v", err)
	}

	tasks, err := service.ListTasks("alice", "running", 20)
	if err != nil {
		t.Fatalf("ListTasks returned error: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("stale task still listed as running: %#v", tasks)
	}
	records, err := service.ListHistory("alice", 20)
	if err != nil {
		t.Fatalf("ListHistory returned error: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("timeout history should not be listed: %#v", records)
	}
}

func TestCancelTaskCompletesRunningTaskAsFailedForOwner(t *testing.T) {
	service := NewService(openTestDB(t))
	task, err := service.CreateTask("alice", TaskInput{
		Kind:        "image",
		ChannelID:   "chn_alice",
		ChannelName: "OpenAI",
		BaseURL:     "https://api.openai.com",
		ModelID:     "gpt-image-1",
		Capability:  "text-to-image",
		Prompt:      "cancel me",
		Parameters:  map[string]any{"size": "1024x1024"},
	})
	if err != nil {
		t.Fatalf("CreateTask returned error: %v", err)
	}

	record, err := service.CancelTask("alice", task.ID)
	if err != nil {
		t.Fatalf("CancelTask returned error: %v", err)
	}
	if record.Status != "失败" || record.FailureReason != "用户取消" || record.TaskID != task.ID {
		t.Fatalf("unexpected cancel record: %#v", record)
	}
	running, err := service.ListTasks("alice", "running", 20)
	if err != nil {
		t.Fatalf("ListTasks returned error: %v", err)
	}
	if len(running) != 0 {
		t.Fatalf("cancelled task still running: %#v", running)
	}
	if _, err := service.CancelTask("bob", task.ID); err == nil {
		t.Fatal("bob cancelled alice task")
	}
	if _, err := service.CancelTask("alice", task.ID); err == nil {
		t.Fatal("cancelled task was cancelled twice")
	}
}

func TestCompleteTaskDoesNotOverwriteCancelledTask(t *testing.T) {
	service := NewService(openTestDB(t))
	task, err := service.CreateTask("alice", TaskInput{
		Kind:        "image",
		ChannelID:   "chn_alice",
		ChannelName: "OpenAI",
		BaseURL:     "https://api.openai.com",
		ModelID:     "gpt-image-1",
		Capability:  "text-to-image",
		Prompt:      "cancel race",
		Parameters:  map[string]any{"size": "1024x1024"},
	})
	if err != nil {
		t.Fatalf("CreateTask returned error: %v", err)
	}
	if _, err := service.CancelTask("alice", task.ID); err != nil {
		t.Fatalf("CancelTask returned error: %v", err)
	}

	if _, err := service.CompleteTask("alice", task.ID, CompleteInput{Status: "完成"}); err == nil {
		t.Fatal("CompleteTask completed a cancelled task")
	}
	records, err := service.ListHistory("alice", 20)
	if err != nil {
		t.Fatalf("ListHistory returned error: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("cancelled history should not be listed: %#v", records)
	}
}

func TestGetFileIsScopedByUser(t *testing.T) {
	service := NewService(openTestDB(t))
	task, err := service.CreateTask("alice", TaskInput{
		Kind:        "image",
		ChannelID:   "chn_alice",
		ChannelName: "OpenAI",
		BaseURL:     "https://api.openai.com",
		ModelID:     "gpt-image-1",
		Capability:  "text-to-image",
		Prompt:      "draw",
		Parameters:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("CreateTask returned error: %v", err)
	}
	record, err := service.CompleteTask("alice", task.ID, CompleteInput{
		Status: "完成",
		Files: []FileInput{{
			StorageProvider: "aliyun-oss",
			Bucket:          "bucket",
			ObjectKey:       "aiImg/users/alice/result.png",
			MediaType:       "image",
			MimeType:        "image/png",
			SizeBytes:       3,
		}},
	})
	if err != nil {
		t.Fatalf("CompleteTask returned error: %v", err)
	}
	file, err := service.GetFile("alice", record.Files[0].ID)
	if err != nil {
		t.Fatalf("GetFile alice returned error: %v", err)
	}
	if file.ObjectKey != "aiImg/users/alice/result.png" {
		t.Fatalf("file = %#v", file)
	}
	if _, err := service.GetFile("bob", record.Files[0].ID); err == nil {
		t.Fatal("bob read alice file")
	}
}

func TestDeleteHistoryIsScopedByUserAndRemovesFiles(t *testing.T) {
	service := NewService(openTestDB(t))
	task, err := service.CreateTask("alice", TaskInput{
		Kind:        "image",
		ChannelID:   "chn_alice",
		ChannelName: "OpenAI",
		BaseURL:     "https://api.openai.com",
		ModelID:     "gpt-image-1",
		Capability:  "text-to-image",
		Prompt:      "draw",
		Parameters:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("CreateTask returned error: %v", err)
	}
	record, err := service.CompleteTask("alice", task.ID, CompleteInput{
		Status: "完成",
		Files: []FileInput{{
			StorageProvider: "aliyun-oss",
			Bucket:          "bucket",
			ObjectKey:       "aiImg/users/alice/result.png",
			MediaType:       "image",
			MimeType:        "image/png",
			SizeBytes:       3,
		}},
	})
	if err != nil {
		t.Fatalf("CompleteTask returned error: %v", err)
	}

	if err := service.DeleteHistory("bob", record.ID); err == nil {
		t.Fatal("bob deleted alice history")
	}
	if err := service.DeleteHistory("alice", record.ID); err != nil {
		t.Fatalf("DeleteHistory returned error: %v", err)
	}
	records, err := service.ListHistory("alice", 20)
	if err != nil {
		t.Fatalf("ListHistory returned error: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("deleted record still listed: %#v", records)
	}
	if _, err := service.GetFile("alice", record.Files[0].ID); err == nil {
		t.Fatal("deleted history file is still readable")
	}
}

func TestExpiredHistoryReturnsRecordsAndFilesOlderThanCutoff(t *testing.T) {
	service := NewService(openTestDB(t))
	oldTask, err := service.CreateTask("alice", TaskInput{
		Kind:        "image",
		ChannelID:   "chn_alice",
		ChannelName: "OpenAI",
		BaseURL:     "https://api.openai.com",
		ModelID:     "gpt-image-1",
		Capability:  "image-to-image",
		Prompt:      "old image",
		Parameters:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("CreateTask old returned error: %v", err)
	}
	oldRecord, err := service.CompleteTask("alice", oldTask.ID, CompleteInput{
		Status: "完成",
		Files: []FileInput{
			{
				StorageProvider: "aliyun-oss",
				Bucket:          "bucket",
				ObjectKey:       "aiImg/users/alice/old-reference.png",
				Role:            "reference",
				MediaType:       "image",
				MimeType:        "image/png",
				SizeBytes:       9,
			},
			{
				StorageProvider: "aliyun-oss",
				Bucket:          "bucket",
				ObjectKey:       "aiImg/users/alice/old-result.png",
				Role:            "result",
				MediaType:       "image",
				MimeType:        "image/png",
				SizeBytes:       3,
			},
		},
	})
	if err != nil {
		t.Fatalf("CompleteTask old returned error: %v", err)
	}
	if _, err := service.db.Exec(`UPDATE history_records SET created_at = ? WHERE id = ?`, int64(1000), oldRecord.ID); err != nil {
		t.Fatalf("age old record: %v", err)
	}

	newTask, err := service.CreateTask("alice", TaskInput{
		Kind:        "image",
		ChannelID:   "chn_alice",
		ChannelName: "OpenAI",
		BaseURL:     "https://api.openai.com",
		ModelID:     "gpt-image-1",
		Capability:  "text-to-image",
		Prompt:      "new image",
		Parameters:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("CreateTask new returned error: %v", err)
	}
	newRecord, err := service.CompleteTask("alice", newTask.ID, CompleteInput{Status: "完成"})
	if err != nil {
		t.Fatalf("CompleteTask new returned error: %v", err)
	}
	if _, err := service.db.Exec(`UPDATE history_records SET created_at = ? WHERE id = ?`, int64(3000), newRecord.ID); err != nil {
		t.Fatalf("age new record: %v", err)
	}

	expired, err := service.ExpiredHistory(2000, 100)
	if err != nil {
		t.Fatalf("ExpiredHistory returned error: %v", err)
	}
	if len(expired) != 1 || expired[0].HistoryID != oldRecord.ID || expired[0].UserID != "alice" {
		t.Fatalf("expired records = %#v", expired)
	}
	if len(expired[0].Files) != 2 || expired[0].Files[0].Role != "reference" || expired[0].Files[1].Role != "result" {
		t.Fatalf("expired files = %#v", expired[0].Files)
	}
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
