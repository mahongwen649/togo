CREATE TABLE IF NOT EXISTS portal_exclusive_channels (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 40),
    state TEXT NOT NULL CHECK (state IN ('provisioning','available','active','releasing','cooling','maintenance','unpublished','error')),
    core_group_id BIGINT UNIQUE,
    account_ids JSONB NOT NULL,
    durations JSONB NOT NULL,
    cooldown_seconds INTEGER NOT NULL CHECK (cooldown_seconds BETWEEN 0 AND 86400),
    cooldown_until TIMESTAMPTZ,
    error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS portal_exclusive_accounts (
    core_account_id BIGINT PRIMARY KEY,
    channel_id BIGINT NOT NULL REFERENCES portal_exclusive_channels(id)
);
CREATE TABLE IF NOT EXISTS portal_exclusive_leases (
    id BIGSERIAL PRIMARY KEY,
    channel_id BIGINT NOT NULL REFERENCES portal_exclusive_channels(id),
    user_id BIGINT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('provisioning','active','releasing','closed')),
    token_hash TEXT NOT NULL UNIQUE,
    token_secret TEXT NOT NULL,
    upstream_secret TEXT NOT NULL,
    core_key_id BIGINT UNIQUE,
    started_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ NOT NULL,
    last_charge_at TIMESTAMPTZ,
    billing_checked_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    reason TEXT NOT NULL DEFAULT '',
    cost NUMERIC(24,12) NOT NULL DEFAULT 0,
    requests BIGINT NOT NULL DEFAULT 0,
    tokens BIGINT NOT NULL DEFAULT 0,
    error TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS portal_exclusive_one_holder ON portal_exclusive_leases(channel_id) WHERE state <> 'closed';
CREATE UNIQUE INDEX IF NOT EXISTS portal_exclusive_one_channel ON portal_exclusive_leases(user_id) WHERE state <> 'closed';
CREATE TABLE IF NOT EXISTS portal_exclusive_requests (
    id TEXT PRIMARY KEY,
    lease_id BIGINT NOT NULL REFERENCES portal_exclusive_leases(id),
    state TEXT NOT NULL CHECK (state IN ('running','finished','uncertain')),
    started_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMPTZ,
    status_code INTEGER,
    error TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS portal_exclusive_requests_open ON portal_exclusive_requests(lease_id) WHERE state <> 'finished';
CREATE TABLE IF NOT EXISTS portal_exclusive_events (
    id BIGSERIAL PRIMARY KEY,
    channel_id BIGINT NOT NULL REFERENCES portal_exclusive_channels(id),
    actor_id BIGINT NOT NULL DEFAULT 0,
    action TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
