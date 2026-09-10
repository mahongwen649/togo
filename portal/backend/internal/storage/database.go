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
	return nil
}
