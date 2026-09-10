package db

import (
	"database/sql"
	"strings"
)

func migrate(database *sql.DB) error {
	_, err := database.Exec(`
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL CHECK (role IN ('admin', 'user')),
  disabled_at INTEGER,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  external_provider TEXT,
  external_subject TEXT
);

CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  expires_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  last_seen_at INTEGER NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS channels (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  name TEXT NOT NULL,
  base_url TEXT NOT NULL,
  encrypted_api_key TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  UNIQUE (user_id, name),
  FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS channel_models (
  id TEXT PRIMARY KEY,
  channel_id TEXT NOT NULL,
  model_id TEXT NOT NULL,
  capabilities_json TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  UNIQUE (channel_id, model_id),
  FOREIGN KEY (channel_id) REFERENCES channels(id)
);

CREATE TABLE IF NOT EXISTS tasks (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('image', 'video', 'text')),
  status TEXT NOT NULL CHECK (status IN ('running', 'completed', 'failed', 'timeout')),
  channel_id TEXT NOT NULL,
  channel_name TEXT NOT NULL,
  base_url TEXT NOT NULL,
  model_id TEXT NOT NULL,
  capability TEXT NOT NULL,
  prompt TEXT NOT NULL,
  parameters_json TEXT NOT NULL,
  upstream_task_id TEXT,
  started_at INTEGER NOT NULL,
  ended_at INTEGER,
  duration_ms INTEGER,
  error_source TEXT,
  failure_reason TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS history_records (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  task_id TEXT NOT NULL UNIQUE,
  kind TEXT NOT NULL CHECK (kind IN ('image', 'video', 'text')),
  status TEXT NOT NULL CHECK (status IN ('完成', '失败', '超时')),
  channel_id TEXT NOT NULL,
  channel_name TEXT NOT NULL,
  base_url TEXT NOT NULL,
  model_id TEXT NOT NULL,
  capability TEXT NOT NULL,
  prompt TEXT NOT NULL,
  parameters_json TEXT NOT NULL,
  started_at INTEGER NOT NULL,
  ended_at INTEGER NOT NULL,
  duration_ms INTEGER NOT NULL,
  error_source TEXT,
  failure_reason TEXT,
  result_text TEXT,
  created_at INTEGER NOT NULL,
  deleted_at INTEGER,
  FOREIGN KEY (user_id) REFERENCES users(id),
  FOREIGN KEY (task_id) REFERENCES tasks(id)
);

CREATE TABLE IF NOT EXISTS result_files (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  history_id TEXT NOT NULL,
  storage_provider TEXT NOT NULL,
  bucket TEXT NOT NULL,
  object_key TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'result' CHECK (role IN ('reference', 'result')),
  media_type TEXT NOT NULL CHECK (media_type = 'image'),
  mime_type TEXT NOT NULL,
  size_bytes INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id),
  FOREIGN KEY (history_id) REFERENCES history_records(id)
);

CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_channels_user ON channels(user_id);
CREATE INDEX IF NOT EXISTS idx_tasks_user_status ON tasks(user_id, status, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_history_user_created ON history_records(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_history_created ON history_records(created_at);
CREATE INDEX IF NOT EXISTS idx_result_files_user ON result_files(user_id);
`)
	if err != nil {
		return err
	}
	_, err = database.Exec(`ALTER TABLE result_files ADD COLUMN role TEXT NOT NULL DEFAULT 'result' CHECK (role IN ('reference', 'result'))`)
	if err != nil && !isDuplicateColumnError(err) {
		return err
	}
	_, err = database.Exec(`ALTER TABLE users ADD COLUMN external_provider TEXT`)
	if err != nil && !isDuplicateColumnError(err) {
		return err
	}
	_, err = database.Exec(`ALTER TABLE users ADD COLUMN external_subject TEXT`)
	if err != nil && !isDuplicateColumnError(err) {
		return err
	}
	_, err = database.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_external_identity ON users(external_provider, external_subject)`)
	if err != nil {
		return err
	}
	return nil
}

func isDuplicateColumnError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate column name:")
}
