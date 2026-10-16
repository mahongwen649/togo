ALTER TABLE portal_account_orders ADD COLUMN IF NOT EXISTS auto_renew BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE portal_account_orders ADD COLUMN IF NOT EXISTS last_renewed_at TIMESTAMPTZ;
ALTER TABLE portal_account_orders ADD COLUMN IF NOT EXISTS renewal_error TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS portal_account_orders_auto_renew_idx
    ON portal_account_orders (expires_at)
    WHERE auto_renew = TRUE AND status IN ('completed', 'expired');
