CREATE TABLE IF NOT EXISTS portal_account_redeem_batches (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    amount NUMERIC(20, 8) NOT NULL CHECK (amount > 0),
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    note TEXT NOT NULL DEFAULT '',
    created_by_admin_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS portal_account_redeem_batches_created_idx
    ON portal_account_redeem_batches (created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS portal_account_redeem_codes (
    id BIGSERIAL PRIMARY KEY,
    batch_id BIGINT NOT NULL REFERENCES portal_account_redeem_batches(id) ON DELETE RESTRICT,
    code_hash TEXT NOT NULL UNIQUE,
    code_suffix TEXT NOT NULL,
    amount NUMERIC(20, 8) NOT NULL CHECK (amount > 0),
    remaining_amount NUMERIC(20, 8) NOT NULL CHECK (remaining_amount >= 0 AND remaining_amount <= amount),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'redeemed', 'partially_used', 'used', 'expired', 'disabled')),
    expires_at TIMESTAMPTZ,
    redeemed_user_id BIGINT,
    redeemed_at TIMESTAMPTZ,
    fully_used_at TIMESTAMPTZ,
    disabled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS portal_account_redeem_codes_batch_idx
    ON portal_account_redeem_codes (batch_id, id);
CREATE INDEX IF NOT EXISTS portal_account_redeem_codes_status_idx
    ON portal_account_redeem_codes (status, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS portal_account_redeem_codes_user_idx
    ON portal_account_redeem_codes (redeemed_user_id, redeemed_at DESC, id DESC);
