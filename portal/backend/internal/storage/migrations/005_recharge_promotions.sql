CREATE TABLE IF NOT EXISTS portal_recharge_promotions (
    core_order_id BIGINT PRIMARY KEY,
    out_trade_no VARCHAR(128) NOT NULL UNIQUE,
    core_user_id BIGINT NOT NULL,
    paid_amount_cents BIGINT NOT NULL CHECK (paid_amount_cents > 0),
    bonus_amount_cents BIGINT NOT NULL CHECK (bonus_amount_cents > 0),
    status VARCHAR(16) NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'APPLYING', 'APPLIED', 'FAILED')),
    attempts INTEGER NOT NULL DEFAULT 0,
    lease_expires_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    applied_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS portal_recharge_promotions_status_idx
    ON portal_recharge_promotions (status, lease_expires_at)
    WHERE status <> 'APPLIED';

COMMENT ON TABLE portal_recharge_promotions IS
    'Portal-owned recharge bonus state; Core payment amounts remain unchanged.';
