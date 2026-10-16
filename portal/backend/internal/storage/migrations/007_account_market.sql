CREATE TABLE IF NOT EXISTS portal_account_market_settings (
    id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    recharge_embed_url TEXT NOT NULL DEFAULT '',
    recharge_external_url TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO portal_account_market_settings (id)
VALUES (1)
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS portal_account_products (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    mode TEXT NOT NULL CHECK (mode IN ('rental', 'purchase')),
    unit_price NUMERIC(20, 8) NOT NULL CHECK (unit_price >= 0),
    stock_quantity INTEGER NOT NULL DEFAULT 0 CHECK (stock_quantity >= 0),
    locked_quantity INTEGER NOT NULL DEFAULT 0 CHECK (locked_quantity >= 0),
    image_url TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    purchase_notice TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (locked_quantity <= stock_quantity)
);

CREATE INDEX IF NOT EXISTS portal_account_products_public_idx
    ON portal_account_products (enabled, sort_order, id);

CREATE TABLE IF NOT EXISTS portal_account_wallets (
    user_id BIGINT PRIMARY KEY,
    balance NUMERIC(20, 8) NOT NULL DEFAULT 0 CHECK (balance >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS portal_account_wallet_ledger (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('redemption', 'purchase', 'refund', 'adjustment')),
    amount NUMERIC(20, 8) NOT NULL,
    balance_before NUMERIC(20, 8) NOT NULL,
    balance_after NUMERIC(20, 8) NOT NULL,
    reference_type TEXT NOT NULL DEFAULT '',
    reference_id BIGINT,
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS portal_account_wallet_ledger_user_idx
    ON portal_account_wallet_ledger (user_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS portal_account_orders (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    product_id BIGINT NOT NULL REFERENCES portal_account_products(id),
    product_name TEXT NOT NULL,
    mode TEXT NOT NULL CHECK (mode IN ('rental', 'purchase')),
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    unit_price NUMERIC(20, 8) NOT NULL CHECK (unit_price >= 0),
    total_price NUMERIC(20, 8) NOT NULL CHECK (total_price >= 0),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'completed', 'cancelled', 'expired')),
    expires_at TIMESTAMPTZ,
    core_group_id BIGINT,
    core_account_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    delivery_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS portal_account_orders_user_idx
    ON portal_account_orders (user_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS portal_account_orders_status_idx
    ON portal_account_orders (status, created_at DESC);
