package history

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"time"
)

var ErrNotFound = errors.New("history item not found")

const runningTaskTimeoutMs = int64(30 * time.Minute / time.Millisecond)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) CreateTask(userID string, input TaskInput) (Task, error) {
	now := nowMillis()
	parameters, err := json.Marshal(input.Parameters)
	if err != nil {
		return Task{}, err
	}
	task := Task{
		ID:          newID("tsk"),
		UserID:      userID,
		Kind:        input.Kind,
		Status:      "running",
		ChannelID:   input.ChannelID,
		ChannelName: input.ChannelName,
		BaseURL:     input.BaseURL,
		ModelID:     input.ModelID,
		Capability:  input.Capability,
		Prompt:      input.Prompt,
		Parameters:  input.Parameters,
		StartedAt:   now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	_, err = s.db.Exec(
		`INSERT INTO tasks (
			id, user_id, kind, status, channel_id, channel_name, base_url, model_id,
			capability, prompt, parameters_json, started_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID,
		task.UserID,
		task.Kind,
		task.Status,
		task.ChannelID,
		task.ChannelName,
		task.BaseURL,
		task.ModelID,
		task.Capability,
		task.Prompt,
		string(parameters),
		task.StartedAt,
		task.CreatedAt,
		task.UpdatedAt,
	)
	if err != nil {
		return Task{}, err
	}
	return task, nil
}

func (s *Service) CompleteTask(userID string, taskID string, input CompleteInput) (Record, error) {
	task, err := s.GetTask(userID, taskID)
	if err != nil {
		return Record{}, err
	}
	if input.Status == "" {
		input.Status = "完成"
	}
	endedAt := nowMillis()
	duration := int64(math.Max(0, float64(endedAt-task.StartedAt)))
	taskStatus := toTaskStatus(input.Status)
	tx, err := s.db.Begin()
	if err != nil {
		return Record{}, err
	}
	defer rollback(tx)

	_, err = tx.Exec(
		`UPDATE tasks
		 SET status = ?, ended_at = ?, duration_ms = ?, error_source = NULLIF(?, ''), failure_reason = NULLIF(?, ''), updated_at = ?
		 WHERE id = ? AND user_id = ?`,
		taskStatus,
		endedAt,
		duration,
		input.ErrorSource,
		input.FailureReason,
		endedAt,
		taskID,
		userID,
	)
	if err != nil {
		return Record{}, err
	}
	record := Record{
		ID:            newID("his"),
		UserID:        userID,
		TaskID:        task.ID,
		Kind:          task.Kind,
		Status:        input.Status,
		ChannelID:     task.ChannelID,
		ChannelName:   task.ChannelName,
		BaseURL:       task.BaseURL,
		ModelID:       task.ModelID,
		Capability:    task.Capability,
		Prompt:        task.Prompt,
		Parameters:    task.Parameters,
		StartedAt:     task.StartedAt,
		EndedAt:       endedAt,
		DurationMs:    duration,
		ErrorSource:   input.ErrorSource,
		FailureReason: input.FailureReason,
		ResultText:    input.ResultText,
		CreatedAt:     endedAt,
	}
	parameters, err := json.Marshal(record.Parameters)
	if err != nil {
		return Record{}, err
	}
	_, err = tx.Exec(
		`INSERT INTO history_records (
			id, user_id, task_id, kind, status, channel_id, channel_name, base_url, model_id,
			capability, prompt, parameters_json, started_at, ended_at, duration_ms,
			error_source, failure_reason, result_text, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?)`,
		record.ID,
		record.UserID,
		record.TaskID,
		record.Kind,
		record.Status,
		record.ChannelID,
		record.ChannelName,
		record.BaseURL,
		record.ModelID,
		record.Capability,
		record.Prompt,
		string(parameters),
		record.StartedAt,
		record.EndedAt,
		record.DurationMs,
		record.ErrorSource,
		record.FailureReason,
		record.ResultText,
		record.CreatedAt,
	)
	if err != nil {
		return Record{}, err
	}
	for _, file := range input.Files {
		role := file.Role
		if role == "" {
			role = "result"
		}
		stored := File{
			ID:              newID("fil"),
			StorageProvider: file.StorageProvider,
			Bucket:          file.Bucket,
			ObjectKey:       file.ObjectKey,
			Role:            role,
			MediaType:       file.MediaType,
			MimeType:        file.MimeType,
			SizeBytes:       file.SizeBytes,
			CreatedAt:       endedAt,
		}
		_, err := tx.Exec(
			`INSERT INTO result_files (
				id, user_id, history_id, storage_provider, bucket, object_key, role, media_type, mime_type, size_bytes, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			stored.ID,
			userID,
			record.ID,
			stored.StorageProvider,
			stored.Bucket,
			stored.ObjectKey,
			stored.Role,
			stored.MediaType,
			stored.MimeType,
			stored.SizeBytes,
			stored.CreatedAt,
		)
		if err != nil {
			return Record{}, err
		}
		record.Files = append(record.Files, stored)
	}
	if err := tx.Commit(); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s *Service) CancelTask(userID string, taskID string) (Record, error) {
	task, err := s.GetTask(userID, taskID)
	if err != nil {
		return Record{}, err
	}
	if task.Status != "running" {
		return Record{}, ErrNotFound
	}
	endedAt := nowMillis()
	duration := int64(math.Max(0, float64(endedAt-task.StartedAt)))
	tx, err := s.db.Begin()
	if err != nil {
		return Record{}, err
	}
	defer rollback(tx)

	result, err := tx.Exec(
		`UPDATE tasks
		 SET status = 'failed', ended_at = ?, duration_ms = ?, error_source = 'user', failure_reason = '用户取消', updated_at = ?
		 WHERE id = ? AND user_id = ? AND status = 'running'`,
		endedAt,
		duration,
		endedAt,
		taskID,
		userID,
	)
	if err != nil {
		return Record{}, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return Record{}, err
	}
	if rowsAffected == 0 {
		return Record{}, ErrNotFound
	}

	record := Record{
		ID:            newID("his"),
		UserID:        userID,
		TaskID:        task.ID,
		Kind:          task.Kind,
		Status:        "失败",
		ChannelID:     task.ChannelID,
		ChannelName:   task.ChannelName,
		BaseURL:       task.BaseURL,
		ModelID:       task.ModelID,
		Capability:    task.Capability,
		Prompt:        task.Prompt,
		Parameters:    task.Parameters,
		StartedAt:     task.StartedAt,
		EndedAt:       endedAt,
		DurationMs:    duration,
		ErrorSource:   "user",
		FailureReason: "用户取消",
		CreatedAt:     endedAt,
	}
	parameters, err := json.Marshal(record.Parameters)
	if err != nil {
		return Record{}, err
	}
	_, err = tx.Exec(
		`INSERT INTO history_records (
			id, user_id, task_id, kind, status, channel_id, channel_name, base_url, model_id,
			capability, prompt, parameters_json, started_at, ended_at, duration_ms,
			error_source, failure_reason, result_text, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?)`,
		record.ID,
		record.UserID,
		record.TaskID,
		record.Kind,
		record.Status,
		record.ChannelID,
		record.ChannelName,
		record.BaseURL,
		record.ModelID,
		record.Capability,
		record.Prompt,
		string(parameters),
		record.StartedAt,
		record.EndedAt,
		record.DurationMs,
		record.ErrorSource,
		record.FailureReason,
		record.ResultText,
		record.CreatedAt,
	)
	if err != nil {
		return Record{}, err
	}
	if err := tx.Commit(); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s *Service) GetTask(userID string, taskID string) (Task, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, kind, status, channel_id, channel_name, base_url, model_id, capability,
			prompt, parameters_json, started_at, COALESCE(ended_at, 0), COALESCE(duration_ms, 0),
			COALESCE(error_source, ''), COALESCE(failure_reason, ''), created_at, updated_at
		 FROM tasks
		 WHERE id = ? AND user_id = ?`,
		taskID,
		userID,
	)
	return scanTask(row)
}

func (s *Service) ListTasks(userID string, status string, limit int) ([]Task, error) {
	if status == "" {
		status = "running"
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	if status == "running" {
		if err := s.expireStaleRunningTasks(userID, nowMillis()-runningTaskTimeoutMs); err != nil {
			return nil, err
		}
	}
	rows, err := s.db.Query(
		`SELECT id, user_id, kind, status, channel_id, channel_name, base_url, model_id, capability,
			prompt, parameters_json, started_at, COALESCE(ended_at, 0), COALESCE(duration_ms, 0),
			COALESCE(error_source, ''), COALESCE(failure_reason, ''), created_at, updated_at
		 FROM tasks
		 WHERE user_id = ? AND status = ?
		 ORDER BY started_at ASC
		 LIMIT ?`,
		userID,
		status,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []Task{}
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (s *Service) expireStaleRunningTasks(userID string, cutoffStartedAt int64) error {
	rows, err := s.db.Query(
		`SELECT id
		 FROM tasks
		 WHERE user_id = ? AND status = 'running' AND started_at <= ?
		 ORDER BY started_at ASC`,
		userID,
		cutoffStartedAt,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	taskIDs := []string{}
	for rows.Next() {
		var taskID string
		if err := rows.Scan(&taskID); err != nil {
			return err
		}
		taskIDs = append(taskIDs, taskID)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, taskID := range taskIDs {
		if _, err := s.CompleteTask(userID, taskID, CompleteInput{
			Status:        "超时",
			ErrorSource:   "system",
			FailureReason: "生成任务超时",
		}); err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
	}
	return nil
}

func (s *Service) ListHistory(userID string, limit int) ([]Record, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.Query(
		`SELECT id, user_id, task_id, kind, status, channel_id, channel_name, base_url, model_id,
			capability, prompt, parameters_json, started_at, ended_at, duration_ms,
			COALESCE(error_source, ''), COALESCE(failure_reason, ''), COALESCE(result_text, ''), created_at
		 FROM history_records
		 WHERE user_id = ? AND deleted_at IS NULL AND status = '完成'
		 ORDER BY created_at DESC
		 LIMIT ?`,
		userID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []Record{}
	for rows.Next() {
		record, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		files, err := s.filesForRecord(userID, record.ID)
		if err != nil {
			return nil, err
		}
		record.Files = files
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Service) DeleteHistory(userID string, historyID string) error {
	now := nowMillis()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer rollback(tx)

	result, err := tx.Exec(
		`UPDATE history_records
		 SET deleted_at = ?
		 WHERE id = ? AND user_id = ? AND deleted_at IS NULL`,
		now,
		historyID,
		userID,
	)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(
		`DELETE FROM result_files
		 WHERE user_id = ? AND history_id = ?`,
		userID,
		historyID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) FilesForHistory(userID string, historyID string) ([]File, error) {
	var id string
	err := s.db.QueryRow(
		`SELECT id
		 FROM history_records
		 WHERE user_id = ? AND id = ? AND deleted_at IS NULL`,
		userID,
		historyID,
	).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return s.filesForRecord(userID, historyID)
}

func (s *Service) ExpiredHistory(cutoffMs int64, limit int) ([]ExpiredRecord, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(
		`SELECT user_id, id
		 FROM history_records
		 WHERE deleted_at IS NULL AND created_at < ?
		 ORDER BY created_at ASC
		 LIMIT ?`,
		cutoffMs,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	expired := []ExpiredRecord{}
	for rows.Next() {
		var record ExpiredRecord
		if err := rows.Scan(&record.UserID, &record.HistoryID); err != nil {
			return nil, err
		}
		files, err := s.filesForRecord(record.UserID, record.HistoryID)
		if err != nil {
			return nil, err
		}
		record.Files = files
		expired = append(expired, record)
	}
	return expired, rows.Err()
}

func (s *Service) GetFile(userID string, fileID string) (File, error) {
	row := s.db.QueryRow(
		`SELECT id, storage_provider, bucket, object_key, role, media_type, mime_type, size_bytes, created_at
		 FROM result_files
		 WHERE user_id = ? AND id = ?`,
		userID,
		fileID,
	)
	var file File
	if err := row.Scan(
		&file.ID,
		&file.StorageProvider,
		&file.Bucket,
		&file.ObjectKey,
		&file.Role,
		&file.MediaType,
		&file.MimeType,
		&file.SizeBytes,
		&file.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return File{}, ErrNotFound
		}
		return File{}, err
	}
	return file, nil
}

func (s *Service) filesForRecord(userID string, historyID string) ([]File, error) {
	rows, err := s.db.Query(
		`SELECT id, storage_provider, bucket, object_key, role, media_type, mime_type, size_bytes, created_at
		 FROM result_files
		 WHERE user_id = ? AND history_id = ?
		 ORDER BY CASE role WHEN 'reference' THEN 0 ELSE 1 END, created_at ASC`,
		userID,
		historyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	files := []File{}
	for rows.Next() {
		var file File
		if err := rows.Scan(
			&file.ID,
			&file.StorageProvider,
			&file.Bucket,
			&file.ObjectKey,
			&file.Role,
			&file.MediaType,
			&file.MimeType,
			&file.SizeBytes,
			&file.CreatedAt,
		); err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanTask(row scanner) (Task, error) {
	var task Task
	var parametersJSON string
	err := row.Scan(
		&task.ID,
		&task.UserID,
		&task.Kind,
		&task.Status,
		&task.ChannelID,
		&task.ChannelName,
		&task.BaseURL,
		&task.ModelID,
		&task.Capability,
		&task.Prompt,
		&parametersJSON,
		&task.StartedAt,
		&task.EndedAt,
		&task.DurationMs,
		&task.ErrorSource,
		&task.FailureReason,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Task{}, ErrNotFound
		}
		return Task{}, err
	}
	if err := json.Unmarshal([]byte(parametersJSON), &task.Parameters); err != nil {
		return Task{}, err
	}
	return task, nil
}

func scanRecord(row scanner) (Record, error) {
	var record Record
	var parametersJSON string
	err := row.Scan(
		&record.ID,
		&record.UserID,
		&record.TaskID,
		&record.Kind,
		&record.Status,
		&record.ChannelID,
		&record.ChannelName,
		&record.BaseURL,
		&record.ModelID,
		&record.Capability,
		&record.Prompt,
		&parametersJSON,
		&record.StartedAt,
		&record.EndedAt,
		&record.DurationMs,
		&record.ErrorSource,
		&record.FailureReason,
		&record.ResultText,
		&record.CreatedAt,
	)
	if err != nil {
		return Record{}, err
	}
	if err := json.Unmarshal([]byte(parametersJSON), &record.Parameters); err != nil {
		return Record{}, err
	}
	return record, nil
}

func toTaskStatus(status string) string {
	if status == "完成" {
		return "completed"
	}
	if status == "超时" {
		return "timeout"
	}
	return "failed"
}

func rollback(tx *sql.Tx) {
	_ = tx.Rollback()
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
