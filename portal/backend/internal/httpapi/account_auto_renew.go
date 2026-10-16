package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

// RunAccountRenewalWorker renews enabled rental packages during the final
// three days. It is intentionally independent from Core's own subscription
// billing and uses the Portal wallet only.
func RunAccountRenewalWorker(ctx context.Context, db *sql.DB, adapter coreclient.Adapter, logger *slog.Logger, interval time.Duration) {
	if db == nil || adapter == nil {
		return
	}
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	s := &Server{portalDB: db, core: adapter, logger: logger}
	run := func() {
		rows, err := db.QueryContext(ctx, `SELECT id FROM portal_account_orders WHERE auto_renew=TRUE AND mode='rental' AND validity_days=30 AND status IN ('completed','expired') AND expires_at IS NOT NULL AND expires_at <= CURRENT_TIMESTAMP + INTERVAL '3 days' AND expires_at >= CURRENT_TIMESTAMP - INTERVAL '3 days' ORDER BY expires_at ASC LIMIT 50`)
		if err != nil {
			if logger != nil {
				logger.Warn("account auto renewal scan failed", "error", err)
			}
			return
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			if rows.Scan(&id) == nil {
				if err := s.renewAccountOrder(ctx, id); err != nil && logger != nil {
					logger.Warn("account auto renewal failed", "order_id", id, "error", err)
				}
			}
		}
	}
	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

func (s *Server) renewAccountOrder(ctx context.Context, orderID int64) error {
	tx, err := s.portalDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var userID, groupID sql.NullInt64
	var quantity int
	var validityDays int
	var mode, status string
	var totalPrice float64
	var expiresAt time.Time
	var autoRenew bool
	var raw []byte
	err = tx.QueryRowContext(ctx, `SELECT user_id, mode, quantity, validity_days, total_price, status, expires_at, core_group_id, core_account_ids, auto_renew FROM portal_account_orders WHERE id=$1 FOR UPDATE`, orderID).Scan(&userID, &mode, &quantity, &validityDays, &totalPrice, &status, &expiresAt, &groupID, &raw, &autoRenew)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil || !userID.Valid || mode != "rental" || validityDays != 30 || !autoRenew || status == "cancelled" {
		return err
	}
	now := time.Now().UTC()
	if expiresAt.Before(now.Add(-3*24*time.Hour)) || expiresAt.After(now.Add(3*24*time.Hour)) {
		return nil
	}
	start := expiresAt
	if start.Before(now) {
		start = now
	}
	newExpiry := start.Add(time.Duration(validityDays) * 24 * time.Hour)
	balanceAfter, debitErr := debitAccountWalletTx(ctx, tx, userID.Int64, totalPrice)
	if errors.Is(debitErr, sql.ErrNoRows) {
		_, _ = tx.ExecContext(ctx, `UPDATE portal_account_orders SET renewal_error='账号套餐余额不足，请充值后自动续订',updated_at=CURRENT_TIMESTAMP WHERE id=$1`, orderID)
		_ = tx.Commit()
		return nil
	} else if debitErr != nil {
		return debitErr
	}
	var accountIDs []int64
	if err := json.Unmarshal(raw, &accountIDs); err != nil {
		return err
	}
	o := rentalOrder{ID: orderID, UserID: userID.Int64, Quantity: quantity, ValidityDays: validityDays, AccountIDs: accountIDs, ExpiresAt: sql.NullTime{Time: newExpiry, Valid: true}}
	if groupID.Valid {
		if err := s.ensureRentalSubscription(ctx, o, rentalGroup{ID: groupID.Int64}); err != nil {
			return err
		}
	}
	for _, accountID := range accountIDs {
		var account customerAccount
		if err := s.rentalCoreRequest(ctx, "PUT", "/api/v1/admin/accounts/"+strconv.FormatInt(accountID, 10), map[string]any{"expires_at": newExpiry.Unix(), "auto_pause_on_expired": true}, &account, "portal-auto-renew-"+strconv.FormatInt(orderID, 10)); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE portal_account_orders SET status='completed',expires_at=$2,last_renewed_at=CURRENT_TIMESTAMP,renewal_error='',updated_at=CURRENT_TIMESTAMP WHERE id=$1`, orderID, newExpiry); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO portal_account_wallet_ledger(user_id,kind,amount,balance_before,balance_after,reference_type,reference_id,note) VALUES($1,'purchase',$2,$3,$4,'account_order',$5,'Account package automatic renewal')`, userID.Int64, -totalPrice, balanceAfter+totalPrice, balanceAfter, orderID); err != nil {
		return err
	}
	return tx.Commit()
}
