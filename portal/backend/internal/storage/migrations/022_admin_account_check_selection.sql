CREATE TABLE IF NOT EXISTS portal_admin_account_check_selection (
    admin_id BIGINT PRIMARY KEY,
    account_ids JSONB NOT NULL DEFAULT '[]',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
