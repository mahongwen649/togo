-- Account-scoped downgrade tests. The legacy channel page-test table remains
-- available for rolling upgrades, but new tests are keyed by Core account.
CREATE TABLE IF NOT EXISTS portal_exclusive_account_tests (
    account_id BIGINT PRIMARY KEY,
    channel_id BIGINT NOT NULL REFERENCES portal_exclusive_channels(id) ON DELETE CASCADE,
    request_id TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('running','succeeded','failed','uncertain')),
    started_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    generated_at TIMESTAMPTZ,
    html TEXT NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT '',
    response_secret TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS portal_exclusive_account_tests_channel ON portal_exclusive_account_tests(channel_id);
