CREATE TABLE IF NOT EXISTS portal_exclusive_page_tests (
    channel_id BIGINT PRIMARY KEY REFERENCES portal_exclusive_channels(id) ON DELETE CASCADE,
    request_id TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('running','succeeded','failed','uncertain')),
    started_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    generated_at TIMESTAMPTZ,
    html TEXT NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT ''
);
