CREATE TABLE IF NOT EXISTS portal_pelican_check_config (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    group_id BIGINT,
    group_name TEXT NOT NULL DEFAULT '',
    api_key_id BIGINT,
    api_key_name TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (group_id IS NULL OR group_id > 0),
    CHECK (api_key_id IS NULL OR api_key_id > 0)
);

INSERT INTO portal_pelican_check_config(id) VALUES(1)
ON CONFLICT(id) DO NOTHING;

CREATE TABLE IF NOT EXISTS portal_pelican_check_runs (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT,
    group_name TEXT NOT NULL DEFAULT '',
    api_key_id BIGINT,
    api_key_name TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL CHECK (state IN ('running', 'succeeded', 'failed')),
    started_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at TIMESTAMPTZ,
    latency_ms BIGINT NOT NULL DEFAULT 0,
    html TEXT NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT ''
);

-- Compatibility with the brief pre-release account-based schema.
ALTER TABLE portal_pelican_check_runs ADD COLUMN IF NOT EXISTS group_id BIGINT;
ALTER TABLE portal_pelican_check_runs ADD COLUMN IF NOT EXISTS group_name TEXT NOT NULL DEFAULT '';
ALTER TABLE portal_pelican_check_runs ADD COLUMN IF NOT EXISTS api_key_id BIGINT;
ALTER TABLE portal_pelican_check_runs ADD COLUMN IF NOT EXISTS api_key_name TEXT NOT NULL DEFAULT '';
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'portal_pelican_check_runs'
          AND column_name = 'channel_id'
    ) THEN
        ALTER TABLE portal_pelican_check_runs ALTER COLUMN channel_id DROP NOT NULL;
    END IF;
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'portal_pelican_check_runs'
          AND column_name = 'account_id'
    ) THEN
        ALTER TABLE portal_pelican_check_runs ALTER COLUMN account_id DROP NOT NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS portal_pelican_check_runs_started
    ON portal_pelican_check_runs(started_at DESC);

CREATE INDEX IF NOT EXISTS portal_pelican_check_runs_key_started
    ON portal_pelican_check_runs(group_id, api_key_id, started_at DESC);

-- Scheduling is independent of history retention and administrator cleanup.
ALTER TABLE portal_pelican_check_config ADD COLUMN IF NOT EXISTS next_run_at TIMESTAMPTZ;
ALTER TABLE portal_pelican_check_config ADD COLUMN IF NOT EXISTS scheduler_seen_at TIMESTAMPTZ;
ALTER TABLE portal_pelican_check_runs ADD COLUMN IF NOT EXISTS attempt INTEGER NOT NULL DEFAULT 1;
ALTER TABLE portal_pelican_check_runs ADD COLUMN IF NOT EXISTS progress TEXT NOT NULL DEFAULT '';
