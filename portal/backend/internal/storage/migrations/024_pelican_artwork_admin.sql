-- Store the exact Core account that generated an artwork. The correlation ID
-- is resolved through Core's administrator-only usage API.
ALTER TABLE portal_pelican_check_runs ADD COLUMN IF NOT EXISTS client_request_id TEXT NOT NULL DEFAULT '';
ALTER TABLE portal_pelican_check_runs ADD COLUMN IF NOT EXISTS account_id BIGINT;
ALTER TABLE portal_pelican_check_runs ADD COLUMN IF NOT EXISTS account_name TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS portal_pelican_check_runs_client_request
    ON portal_pelican_check_runs(client_request_id)
    WHERE client_request_id <> '';
