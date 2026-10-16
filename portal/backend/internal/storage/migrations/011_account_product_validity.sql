ALTER TABLE portal_account_products
    ADD COLUMN IF NOT EXISTS validity_days INTEGER NOT NULL DEFAULT 0;

UPDATE portal_account_products
SET validity_days = 30
WHERE mode = 'rental' AND validity_days <= 0;

ALTER TABLE portal_account_orders
    ADD COLUMN IF NOT EXISTS validity_days INTEGER NOT NULL DEFAULT 0;

UPDATE portal_account_orders
SET validity_days = 30
WHERE mode = 'rental' AND validity_days <= 0;

UPDATE portal_account_orders
SET auto_renew = FALSE
WHERE validity_days <> 30 AND auto_renew = TRUE;

ALTER TABLE portal_account_products
    DROP CONSTRAINT IF EXISTS portal_account_products_validity_days_check;
ALTER TABLE portal_account_products
    ADD CONSTRAINT portal_account_products_validity_days_check
    CHECK (
        (mode = 'rental' AND validity_days BETWEEN 1 AND 3650)
        OR (mode = 'purchase' AND validity_days = 0)
    );

ALTER TABLE portal_account_orders
    DROP CONSTRAINT IF EXISTS portal_account_orders_validity_days_check;
ALTER TABLE portal_account_orders
    ADD CONSTRAINT portal_account_orders_validity_days_check
    CHECK (
        (mode = 'rental' AND validity_days BETWEEN 1 AND 3650)
        OR (mode = 'purchase' AND validity_days = 0)
    );
