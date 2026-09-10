CREATE TABLE IF NOT EXISTS portal_companies (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS portal_companies_name_active
    ON portal_companies (name) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS portal_company_members (
    company_id BIGINT NOT NULL REFERENCES portal_companies(id) ON DELETE CASCADE,
    core_user_id BIGINT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (company_id, core_user_id)
);

CREATE TABLE IF NOT EXISTS portal_company_managers (
    company_id BIGINT NOT NULL,
    core_user_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (company_id, core_user_id),
    FOREIGN KEY (company_id, core_user_id)
        REFERENCES portal_company_members(company_id, core_user_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS portal_company_members_company_idx
    ON portal_company_members(company_id);
CREATE INDEX IF NOT EXISTS portal_company_managers_user_idx
    ON portal_company_managers(core_user_id);
