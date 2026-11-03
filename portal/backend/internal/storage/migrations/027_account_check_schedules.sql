CREATE TABLE IF NOT EXISTS portal_account_check_schedules (
    id BIGSERIAL PRIMARY KEY,
    admin_id BIGINT NOT NULL,
    account_id BIGINT NOT NULL,
    model_id TEXT NOT NULL,
    reasoning_effort TEXT NOT NULL DEFAULT '',
    prompt TEXT NOT NULL DEFAULT 'hi',
    cron_expression TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ NOT NULL,
    running_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS portal_account_check_schedules_owner ON portal_account_check_schedules(admin_id,account_id);
CREATE INDEX IF NOT EXISTS portal_account_check_schedules_due ON portal_account_check_schedules(next_run_at) WHERE enabled;
CREATE TABLE IF NOT EXISTS portal_account_check_schedule_results (
    id BIGSERIAL PRIMARY KEY,
    plan_id BIGINT NOT NULL REFERENCES portal_account_check_schedules(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    response_text TEXT NOT NULL DEFAULT '',
    html TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    latency_ms BIGINT NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS portal_account_check_schedule_results_plan ON portal_account_check_schedule_results(plan_id,id DESC);
