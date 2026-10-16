CREATE TABLE IF NOT EXISTS portal_admin_account_checks (
    id BIGSERIAL PRIMARY KEY,
    admin_id BIGINT NOT NULL,
    account_id BIGINT NOT NULL,
    account JSONB NOT NULL,
    prompt TEXT NOT NULL,
    model TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL CHECK (state IN ('running', 'succeeded', 'failed')),
    result JSONB NOT NULL DEFAULT '{"text":"","html":"","model":""}',
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS portal_admin_account_checks_owner_idx
    ON portal_admin_account_checks (admin_id, account_id, id DESC);
