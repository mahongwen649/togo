ALTER TABLE portal_exclusive_channels
    ADD COLUMN IF NOT EXISTS rate_multiplier DOUBLE PRECISION NOT NULL DEFAULT 1
        CHECK (rate_multiplier > 0 AND rate_multiplier < 'Infinity'::double precision),
    ADD COLUMN IF NOT EXISTS rate_resume_state TEXT
        CHECK (rate_resume_state IN ('available','maintenance','unpublished'));
