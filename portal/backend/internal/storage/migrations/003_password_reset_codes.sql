CREATE TABLE IF NOT EXISTS portal_password_reset_codes (
    email VARCHAR(320) PRIMARY KEY,
    core_user_id BIGINT NOT NULL,
    code_digest BYTEA NOT NULL,
    attempts SMALLINT NOT NULL DEFAULT 0,
    send_count SMALLINT NOT NULL DEFAULT 1,
    expires_at TIMESTAMPTZ NOT NULL,
    last_sent_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS portal_password_reset_codes_expiry_idx
    ON portal_password_reset_codes (expires_at);

ALTER TABLE portal_password_reset_codes
    ADD COLUMN IF NOT EXISTS send_count SMALLINT NOT NULL DEFAULT 1;
