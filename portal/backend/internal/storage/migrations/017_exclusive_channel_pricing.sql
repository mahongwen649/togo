ALTER TABLE portal_exclusive_channels
    ADD COLUMN IF NOT EXISTS model_pricing JSONB NOT NULL DEFAULT '[]'::jsonb
        CHECK (jsonb_typeof(model_pricing) = 'array'),
    ADD COLUMN IF NOT EXISTS image_price DOUBLE PRECISION NOT NULL DEFAULT 0.05
        CHECK (image_price >= 0 AND image_price < 'Infinity'::double precision),
    ADD COLUMN IF NOT EXISTS image_rate_multiplier DOUBLE PRECISION NOT NULL DEFAULT 1
        CHECK (image_rate_multiplier > 0 AND image_rate_multiplier < 'Infinity'::double precision),
    ADD COLUMN IF NOT EXISTS pricing_revision BIGINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS pricing_synced BOOLEAN NOT NULL DEFAULT false;
