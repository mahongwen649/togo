CREATE TABLE IF NOT EXISTS portal_lottery_campaigns (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    subtitle TEXT NOT NULL DEFAULT '',
    registration_start TIMESTAMPTZ NOT NULL,
    draw_at TIMESTAMPTZ NOT NULL,
    participant_limit INTEGER NOT NULL CHECK (participant_limit > 0),
    random_limit INTEGER NOT NULL CHECK (random_limit > 0 AND random_limit <= participant_limit),
    random_min NUMERIC(20,8) NOT NULL CHECK (random_min > 0),
    random_max NUMERIC(20,8) NOT NULL CHECK (random_max >= random_min),
    random_budget NUMERIC(20,8) NOT NULL CHECK (random_budget > 0),
    guarantee_amount NUMERIC(20,8) NOT NULL CHECK (guarantee_amount > 0),
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','scheduled','registering','drawing','completed','cancelled')),
    published BOOLEAN NOT NULL DEFAULT FALSE,
    drawn_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (draw_at > registration_start),
    CHECK (random_budget / random_limit >= random_min AND random_budget / random_limit <= random_max)
);

CREATE UNIQUE INDEX IF NOT EXISTS portal_lottery_one_published_window
    ON portal_lottery_campaigns ((published))
    WHERE published = TRUE AND status IN ('scheduled','registering','drawing');

CREATE INDEX IF NOT EXISTS portal_lottery_campaigns_public_idx
    ON portal_lottery_campaigns (published, status, registration_start, draw_at);

CREATE TABLE IF NOT EXISTS portal_lottery_participants (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES portal_lottery_campaigns(id) ON DELETE CASCADE,
    core_user_id BIGINT NOT NULL,
    email TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (campaign_id, core_user_id)
);

CREATE INDEX IF NOT EXISTS portal_lottery_participants_campaign_idx
    ON portal_lottery_participants (campaign_id, created_at, id);

CREATE TABLE IF NOT EXISTS portal_lottery_payouts (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES portal_lottery_campaigns(id) ON DELETE CASCADE,
    participant_id BIGINT NOT NULL REFERENCES portal_lottery_participants(id) ON DELETE CASCADE,
    core_user_id BIGINT NOT NULL,
    email TEXT NOT NULL,
    prize_type TEXT NOT NULL CHECK (prize_type IN ('random','guarantee')),
    amount NUMERIC(20,8) NOT NULL CHECK (amount > 0),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','processing','credited','failed')),
    idempotency_key TEXT NOT NULL UNIQUE,
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    credited_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS portal_lottery_payouts_status_idx
    ON portal_lottery_payouts (status, updated_at);
