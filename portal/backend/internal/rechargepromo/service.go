package rechargepromo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

const applyLease = 30 * time.Second

var ErrApplying = errors.New("recharge promotion is already being applied")

type BalanceCreditor interface {
	AdminAddBalance(context.Context, int64, float64, string, string) error
}

type Promotion struct {
	OrderID    int64
	TradeNo    string
	UserID     int64
	PaidCents  int64
	BonusCents int64
	Status     string
}

type Service struct {
	db     *sql.DB
	credit BalanceCreditor
}

func New(db *sql.DB, credit BalanceCreditor) *Service {
	return &Service{db: db, credit: credit}
}

func BonusCents(amount float64) int64 {
	cents := int64(math.Round(amount * 100))
	if math.Abs(amount*100-float64(cents)) > 0.000001 {
		return 0
	}
	switch cents {
	case 5000:
		return 500
	case 10000:
		return 1200
	case 20000:
		return 2500
	default:
		return 0
	}
}

func (s *Service) Register(ctx context.Context, orderID int64, tradeNo string, userID int64, amount float64) (float64, error) {
	bonusCents := BonusCents(amount)
	if bonusCents == 0 {
		return 0, nil
	}
	paidCents := int64(math.Round(amount * 100))
	if s == nil || s.db == nil || orderID <= 0 || userID <= 0 || strings.TrimSpace(tradeNo) == "" {
		return 0, fmt.Errorf("invalid recharge promotion order")
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO portal_recharge_promotions (
			core_order_id, out_trade_no, core_user_id, paid_amount_cents, bonus_amount_cents
		) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (core_order_id) DO NOTHING
	`, orderID, strings.TrimSpace(tradeNo), userID, paidCents, bonusCents)
	if err != nil {
		return 0, fmt.Errorf("record recharge promotion: %w", err)
	}
	return float64(bonusCents) / 100, nil
}

func (s *Service) LookupByOrderID(ctx context.Context, orderID int64) (Promotion, bool, error) {
	return s.lookup(ctx, "core_order_id", orderID)
}

func (s *Service) LookupByTradeNo(ctx context.Context, tradeNo string) (Promotion, bool, error) {
	return s.lookup(ctx, "out_trade_no", strings.TrimSpace(tradeNo))
}

func (s *Service) lookup(ctx context.Context, column string, value any) (Promotion, bool, error) {
	if s == nil || s.db == nil {
		return Promotion{}, false, nil
	}
	query := `SELECT core_order_id, out_trade_no, core_user_id, paid_amount_cents, bonus_amount_cents, status
		FROM portal_recharge_promotions WHERE ` + column + ` = $1`
	var promotion Promotion
	err := s.db.QueryRowContext(ctx, query, value).Scan(
		&promotion.OrderID, &promotion.TradeNo, &promotion.UserID,
		&promotion.PaidCents, &promotion.BonusCents, &promotion.Status,
	)
	if err == sql.ErrNoRows {
		return Promotion{}, false, nil
	}
	if err != nil {
		return Promotion{}, false, err
	}
	return promotion, true, nil
}

func (s *Service) FulfillByOrderID(ctx context.Context, orderID int64) error {
	return s.fulfill(ctx, "core_order_id", orderID)
}

func (s *Service) FulfillByTradeNo(ctx context.Context, tradeNo string) error {
	return s.fulfill(ctx, "out_trade_no", strings.TrimSpace(tradeNo))
}

func (s *Service) fulfill(ctx context.Context, column string, value any) error {
	if s == nil || s.db == nil || s.credit == nil {
		return nil
	}
	query := `
		UPDATE portal_recharge_promotions
		SET status = 'APPLYING', attempts = attempts + 1,
			lease_expires_at = CURRENT_TIMESTAMP + ($2 * INTERVAL '1 second'),
			last_error = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE ` + column + ` = $1
		  AND (status IN ('PENDING', 'FAILED') OR (status = 'APPLYING' AND lease_expires_at < CURRENT_TIMESTAMP))
		RETURNING core_order_id, out_trade_no, core_user_id, paid_amount_cents, bonus_amount_cents, status
	`
	var promotion Promotion
	err := s.db.QueryRowContext(ctx, query, value, int(applyLease/time.Second)).Scan(
		&promotion.OrderID, &promotion.TradeNo, &promotion.UserID,
		&promotion.PaidCents, &promotion.BonusCents, &promotion.Status,
	)
	if err == sql.ErrNoRows {
		current, found, lookupErr := s.lookup(ctx, column, value)
		if lookupErr != nil || !found || current.Status == "APPLIED" {
			return lookupErr
		}
		return ErrApplying
	}
	if err != nil {
		return fmt.Errorf("claim recharge promotion: %w", err)
	}

	note := fmt.Sprintf("Portal recharge campaign bonus for order %d (%s)", promotion.OrderID, promotion.TradeNo)
	idempotencyKey := fmt.Sprintf("portal-recharge-bonus-%d", promotion.OrderID)
	if err := s.credit.AdminAddBalance(ctx, promotion.UserID, float64(promotion.BonusCents)/100, note, idempotencyKey); err != nil {
		message := err.Error()
		if len(message) > 1000 {
			message = message[:1000]
		}
		_, _ = s.db.ExecContext(ctx, `
			UPDATE portal_recharge_promotions
			SET status = 'FAILED', lease_expires_at = NULL, last_error = $2, updated_at = CURRENT_TIMESTAMP
			WHERE core_order_id = $1 AND status = 'APPLYING'
		`, promotion.OrderID, message)
		return fmt.Errorf("apply recharge promotion: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE portal_recharge_promotions
		SET status = 'APPLIED', lease_expires_at = NULL, last_error = NULL,
			updated_at = CURRENT_TIMESTAMP, applied_at = CURRENT_TIMESTAMP
		WHERE core_order_id = $1 AND status = 'APPLYING'
	`, promotion.OrderID)
	if err != nil {
		return fmt.Errorf("mark recharge promotion applied: %w", err)
	}
	return nil
}
