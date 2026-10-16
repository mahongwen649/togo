package storage

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/001_username_registry.sql
var usernameRegistryMigration string

//go:embed migrations/002_companies.sql
var companiesMigration string

//go:embed migrations/003_password_reset_codes.sql
var passwordResetCodesMigration string

//go:embed migrations/004_balance_alert_states.sql
var balanceAlertStatesMigration string

//go:embed migrations/005_recharge_promotions.sql
var rechargePromotionsMigration string

//go:embed migrations/006_lottery_campaign.sql
var lotteryCampaignMigration string

//go:embed migrations/007_account_market.sql
var accountMarketMigration string

//go:embed migrations/008_account_rental_fulfillment.sql
var accountRentalFulfillmentMigration string

//go:embed migrations/009_account_auto_renew.sql
var accountAutoRenewMigration string

//go:embed migrations/010_account_redeem_codes.sql
var accountRedeemCodesMigration string

//go:embed migrations/011_account_product_validity.sql
var accountProductValidityMigration string

//go:embed migrations/012_account_payments.sql
var accountPaymentsMigration string

//go:embed migrations/013_exclusive_channels.sql
var exclusiveChannelsMigration string

//go:embed migrations/014_exclusive_page_tests.sql
var exclusivePageTestsMigration string

//go:embed migrations/015_exclusive_page_diagnostic.sql
var exclusivePageDiagnosticMigration string

//go:embed migrations/016_exclusive_channel_rate.sql
var exclusiveChannelRateMigration string

//go:embed migrations/017_exclusive_channel_pricing.sql
var exclusiveChannelPricingMigration string

//go:embed migrations/018_exclusive_account_downgrade_tests.sql
var exclusiveAccountDowngradeTestsMigration string

//go:embed migrations/019_exclusive_pending_config.sql
var exclusivePendingConfigMigration string

//go:embed migrations/020_pelican_checks.sql
var pelicanChecksMigration string

//go:embed migrations/021_admin_account_check_history.sql
var adminAccountCheckHistoryMigration string

//go:embed migrations/022_admin_account_check_selection.sql
var adminAccountCheckSelectionMigration string

//go:embed migrations/023_admin_account_check_cancellation.sql
var adminAccountCheckCancellationMigration string

//go:embed migrations/024_pelican_artwork_admin.sql
var pelicanArtworkAdminMigration string

//go:embed migrations/025_pelican_check_prompt.sql
var pelicanCheckPromptMigration string

//go:embed migrations/026_pelican_reasoning_effort.sql
var pelicanReasoningEffortMigration string

func Open(ctx context.Context, databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open Portal database: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping Portal database: %w", err)
	}
	return db, nil
}

func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, usernameRegistryMigration); err != nil {
		return fmt.Errorf("apply Portal username registry migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, companiesMigration); err != nil {
		return fmt.Errorf("apply Portal company migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, passwordResetCodesMigration); err != nil {
		return fmt.Errorf("apply Portal password reset migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, balanceAlertStatesMigration); err != nil {
		return fmt.Errorf("apply Portal balance alert migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, rechargePromotionsMigration); err != nil {
		return fmt.Errorf("apply Portal recharge promotions migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, lotteryCampaignMigration); err != nil {
		return fmt.Errorf("apply Portal lottery campaign migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, accountMarketMigration); err != nil {
		return fmt.Errorf("apply Portal account market migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, accountRentalFulfillmentMigration); err != nil {
		return fmt.Errorf("apply Portal account rental fulfillment migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, accountAutoRenewMigration); err != nil {
		return fmt.Errorf("apply Portal account auto renew migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, accountRedeemCodesMigration); err != nil {
		return fmt.Errorf("apply Portal account redeem codes migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, accountProductValidityMigration); err != nil {
		return fmt.Errorf("apply Portal account product validity migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, accountPaymentsMigration); err != nil {
		return fmt.Errorf("apply Portal account payments migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, exclusiveChannelsMigration); err != nil {
		return fmt.Errorf("apply Portal exclusive channels migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, exclusivePageTestsMigration); err != nil {
		return fmt.Errorf("apply Portal exclusive page tests migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, exclusivePageDiagnosticMigration); err != nil {
		return fmt.Errorf("apply Portal exclusive page diagnostic migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, exclusiveChannelRateMigration); err != nil {
		return fmt.Errorf("apply Portal exclusive channel rate migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, exclusiveChannelPricingMigration); err != nil {
		return fmt.Errorf("apply Portal exclusive channel pricing migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, exclusiveAccountDowngradeTestsMigration); err != nil {
		return fmt.Errorf("apply Portal exclusive account downgrade tests migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, exclusivePendingConfigMigration); err != nil {
		return fmt.Errorf("apply Portal exclusive pending config migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, pelicanChecksMigration); err != nil {
		return fmt.Errorf("apply Portal pelican checks migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, adminAccountCheckHistoryMigration); err != nil {
		return fmt.Errorf("apply Portal admin account check history migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, adminAccountCheckSelectionMigration); err != nil {
		return fmt.Errorf("apply Portal admin account check selection migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, adminAccountCheckCancellationMigration); err != nil {
		return fmt.Errorf("apply Portal admin account check cancellation migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, pelicanArtworkAdminMigration); err != nil {
		return fmt.Errorf("apply Portal pelican artwork administrator migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, pelicanCheckPromptMigration); err != nil {
		return fmt.Errorf("apply Portal pelican prompt migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, pelicanReasoningEffortMigration); err != nil {
		return fmt.Errorf("apply pelican reasoning effort migration: %w", err)
	}
	return nil
}
