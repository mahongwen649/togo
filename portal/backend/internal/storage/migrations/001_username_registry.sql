CREATE TABLE IF NOT EXISTS portal_username_registry (
    username_key VARCHAR(100) PRIMARY KEY,
    username VARCHAR(100) NOT NULL,
    core_user_id BIGINT UNIQUE,
    lease_id VARCHAR(64),
    lease_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT portal_username_registry_state_check CHECK (
        (core_user_id IS NOT NULL AND lease_id IS NULL AND lease_expires_at IS NULL)
        OR
        (core_user_id IS NULL AND lease_id IS NOT NULL AND lease_expires_at IS NOT NULL)
    )
);

COMMENT ON TABLE portal_username_registry IS
    'Portal-owned unique username index; Core remains the user and credential source of truth.';
