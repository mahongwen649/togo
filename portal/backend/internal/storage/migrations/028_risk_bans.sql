CREATE TABLE IF NOT EXISTS portal_risk_events (
    event_id BIGINT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    event_type TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS portal_risk_keys (
    key_hash TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    event_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS portal_risk_keys_user ON portal_risk_keys(user_id);
CREATE TABLE IF NOT EXISTS portal_risk_cursor (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    checked_until TIMESTAMPTZ NOT NULL
);
