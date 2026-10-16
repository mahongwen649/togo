ALTER TABLE portal_exclusive_channels
    ADD COLUMN IF NOT EXISTS pending_config JSONB;
