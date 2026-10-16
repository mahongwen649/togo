CREATE TABLE IF NOT EXISTS portal_account_payment_orders (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    out_trade_no TEXT NOT NULL UNIQUE,
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    pay_cents BIGINT NOT NULL CHECK (pay_cents >= amount_cents),
    fee_rate NUMERIC(10, 6) NOT NULL DEFAULT 0,
    provider_id BIGINT NOT NULL,
    merchant_id TEXT NOT NULL,
    gateway_origin TEXT NOT NULL,
    provider_snapshot BYTEA NOT NULL,
    trade_no TEXT,
    status TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'COMPLETED', 'EXPIRED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    next_check_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP + INTERVAL '30 seconds',
    CHECK ((status = 'COMPLETED') = (completed_at IS NOT NULL))
);
CREATE UNIQUE INDEX IF NOT EXISTS portal_account_payment_trade_unique
    ON portal_account_payment_orders(gateway_origin, merchant_id, trade_no) WHERE trade_no IS NOT NULL;
CREATE INDEX IF NOT EXISTS portal_account_payment_user_idx
    ON portal_account_payment_orders(user_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS portal_account_payment_check_idx
    ON portal_account_payment_orders(next_check_at) WHERE status <> 'COMPLETED';
CREATE UNIQUE INDEX IF NOT EXISTS portal_account_payment_ledger_unique
    ON portal_account_wallet_ledger(reference_type, reference_id) WHERE reference_type = 'account_payment';
