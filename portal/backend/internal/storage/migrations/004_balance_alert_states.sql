CREATE TABLE IF NOT EXISTS portal_balance_alert_states (
    core_user_id BIGINT PRIMARY KEY,
    email VARCHAR(320) NOT NULL,
    last_balance NUMERIC(20, 8) NOT NULL,
    last_threshold NUMERIC(20, 8) NOT NULL,
    eligible BOOLEAN NOT NULL DEFAULT FALSE,
    below_threshold BOOLEAN NOT NULL DEFAULT FALSE,
    pending BOOLEAN NOT NULL DEFAULT FALSE,
    last_observed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_sent_at TIMESTAMPTZ,
    last_send_error TEXT
);

CREATE INDEX IF NOT EXISTS portal_balance_alert_states_pending_idx
    ON portal_balance_alert_states (pending) WHERE pending = TRUE;

COMMENT ON TABLE portal_balance_alert_states IS
    'Portal-owned low-balance crossing state populated only through official Core APIs.';
