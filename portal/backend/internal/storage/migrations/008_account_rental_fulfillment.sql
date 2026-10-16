ALTER TABLE portal_account_orders ADD COLUMN IF NOT EXISTS fulfillment_started_at TIMESTAMPTZ;
ALTER TABLE portal_account_orders ADD COLUMN IF NOT EXISTS fulfillment_error TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS portal_account_rental_groups (
    user_id BIGINT NOT NULL,
    product_id BIGINT NOT NULL REFERENCES portal_account_products(id),
    core_group_id BIGINT NOT NULL UNIQUE,
    platform TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, product_id)
);

-- Reservations survive an interrupted Core request so another order cannot
-- allocate a partially provisioned account. Order history stays in orders.
CREATE TABLE IF NOT EXISTS portal_account_rental_reservations (
    core_account_id BIGINT PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES portal_account_orders(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS portal_account_rental_reservations_order_idx
    ON portal_account_rental_reservations(order_id);
