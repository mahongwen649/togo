DO $$
DECLARE
    definition TEXT;
BEGIN
    SELECT pg_get_constraintdef(oid) INTO definition
    FROM pg_constraint
    WHERE conrelid = 'portal_admin_account_checks'::regclass
      AND conname = 'portal_admin_account_checks_state_check';

    IF definition IS NULL OR position('cancelled' IN definition) = 0 THEN
        ALTER TABLE portal_admin_account_checks
            DROP CONSTRAINT IF EXISTS portal_admin_account_checks_state_check;
        ALTER TABLE portal_admin_account_checks
            ADD CONSTRAINT portal_admin_account_checks_state_check
            CHECK (state IN ('running', 'succeeded', 'failed', 'cancelled'));
    END IF;
END $$;
