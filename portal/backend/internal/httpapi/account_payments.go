package httpapi

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type accountPaymentOrder struct {
	ID          int64      `json:"id"`
	Trade       string     `json:"out_trade_no"`
	Amount      float64    `json:"amount"`
	PayAmount   float64    `json:"pay_amount"`
	FeeRate     float64    `json:"fee_rate"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	CompletedAt *time.Time `json:"completed_at"`
	OrderType   string     `json:"order_type"`
	PaymentType string     `json:"payment_type"`
	PayURL      string     `json:"pay_url,omitempty"`
}

const accountPaymentColumns = `id,out_trade_no,amount_cents/100.0,pay_cents/100.0,fee_rate,
 CASE WHEN status='PENDING' AND expires_at<CURRENT_TIMESTAMP THEN 'EXPIRED' ELSE status END,created_at,expires_at,completed_at`

func scanAccountPayment(row interface{ Scan(...any) error }) (accountPaymentOrder, error) {
	o := accountPaymentOrder{OrderType: "account_balance", PaymentType: "alipay"}
	err := row.Scan(&o.ID, &o.Trade, &o.Amount, &o.PayAmount, &o.FeeRate, &o.Status, &o.CreatedAt, &o.ExpiresAt, &o.CompletedAt)
	return o, err
}

func (s *Server) accountPaymentUser(w http.ResponseWriter, r *http.Request) (int64, bool) {
	token, ok := bearerToken(w, r)
	if !ok || !s.requireMarketDB(w) {
		return 0, false
	}
	user, err := s.core.Profile(r.Context(), token)
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return 0, false
	}
	if user.ID <= 0 || user.Status != "active" {
		writeAPIError(w, 403, "ACCOUNT_DISABLED", "账号不可用")
		return 0, false
	}
	return user.ID, true
}

func (s *Server) accountPaymentCheckout(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.accountPaymentUser(w, r); !ok {
		return
	}
	p, err := s.loadAccountPaymentProvider(r.Context())
	if err != nil {
		writeAPIError(w, 503, "ACCOUNT_PAYMENT_UNAVAILABLE", errAccountPaymentUnavailable.Error())
		return
	}
	fee, _ := strconv.ParseFloat(p.Fee, 64)
	writeSuccess(w, map[string]any{"methods": map[string]any{"alipay": map[string]any{"single_min": float64(p.Min) / 100, "single_max": float64(p.Max) / 100, "fee_rate": fee}}, "global_min": float64(p.Min) / 100, "global_max": float64(p.Max) / 100, "balance_disabled": false, "balance_recharge_multiplier": 1, "recharge_fee_rate": fee, "wallet": "account", "help_text": "支付宝付款后直接计入账号套餐余额"})
}

func (s *Server) accountPaymentCreate(w http.ResponseWriter, r *http.Request) {
	user, ok := s.accountPaymentUser(w, r)
	if !ok {
		return
	}
	var input struct {
		Amount      json.Number `json:"amount"`
		PaymentType string      `json:"payment_type"`
		IsMobile    bool        `json:"is_mobile"`
	}
	if decodeJSON(r, &input) != nil || (input.PaymentType != "" && input.PaymentType != "alipay") {
		writeAPIError(w, 400, "INVALID_PAYMENT", "充值参数无效")
		return
	}
	cents, err := paymentCents(string(input.Amount))
	if err != nil {
		writeAPIError(w, 400, "INVALID_AMOUNT", err.Error())
		return
	}
	p, err := s.loadAccountPaymentProvider(r.Context())
	if err != nil {
		writeAPIError(w, 503, "ACCOUNT_PAYMENT_UNAVAILABLE", errAccountPaymentUnavailable.Error())
		return
	}
	if cents < p.Min || cents > p.Max {
		writeAPIError(w, 400, "INVALID_AMOUNT", "充值金额不符合单笔限额")
		return
	}
	pay, err := accountPaymentTotal(cents, p.Fee)
	if err != nil {
		writeAPIError(w, 503, "ACCOUNT_PAYMENT_UNAVAILABLE", errAccountPaymentUnavailable.Error())
		return
	}
	// Gateway merchant order numbers are limited to 32 characters including the prefix.
	var random [13]byte
	if _, err = rand.Read(random[:]); err != nil {
		writeAPIError(w, 500, "PAYMENT_CREATE_FAILED", "创建充值订单失败")
		return
	}
	trade := "acct_" + hex.EncodeToString(random[:])
	snapshot, err := s.sealPayment(p, trade)
	if err != nil {
		writeAPIError(w, 500, "PAYMENT_CREATE_FAILED", "创建充值订单失败")
		return
	}
	tx, err := s.portalDB.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, 500, "PAYMENT_CREATE_FAILED", "创建充值订单失败")
		return
	}
	defer tx.Rollback()
	fail := func() { writeAPIError(w, 500, "PAYMENT_CREATE_FAILED", "创建充值订单失败，请稍后重试") }
	// Serialize per-user limits without touching any Core balance or Core order.
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO portal_account_wallets(user_id) VALUES($1) ON CONFLICT DO NOTHING`, user); err != nil {
		fail()
		return
	}
	var locked int64
	if err = tx.QueryRowContext(r.Context(), `SELECT user_id FROM portal_account_wallets WHERE user_id=$1 FOR UPDATE`, user).Scan(&locked); err != nil {
		fail()
		return
	}
	var pending int
	var used int64
	err = tx.QueryRowContext(r.Context(), `SELECT count(*) FILTER(WHERE status='PENDING' AND expires_at>CURRENT_TIMESTAMP),COALESCE(sum(amount_cents) FILTER(WHERE created_at>CURRENT_TIMESTAMP-INTERVAL '24 hours'),0) FROM portal_account_payment_orders WHERE user_id=$1`, user).Scan(&pending, &used)
	if err != nil {
		fail()
		return
	}
	if pending >= p.Pending {
		writeAPIError(w, 429, "PAYMENT_PENDING_LIMIT", "待支付订单过多，请先完成已有订单或等待过期")
		return
	}
	if p.Daily > 0 && used+cents > p.Daily {
		writeAPIError(w, 429, "PAYMENT_DAILY_LIMIT", "已达到账号套餐每日充值限额")
		return
	}
	if p.ChannelDaily > 0 {
		// Shared provider usage is read-only; gateway limits remain authoritative.
		var coreUsed, portalUsed float64
		err = s.corePaymentDB.QueryRowContext(r.Context(), `SELECT COALESCE(sum(pay_amount),0) FROM payment_orders WHERE provider_instance_id=$1 AND created_at>=CURRENT_DATE AND status IN ('PENDING','PAID','RECHARGING','COMPLETED')`, strconv.FormatInt(p.ID, 10)).Scan(&coreUsed)
		if err != nil {
			fail()
			return
		}
		if _, err = tx.ExecContext(r.Context(), `SELECT pg_advisory_xact_lock(7412,$1::int)`, p.ID); err != nil {
			fail()
			return
		}
		err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(sum(pay_cents),0)/100.0 FROM portal_account_payment_orders WHERE provider_id=$1 AND created_at>=CURRENT_DATE`, p.ID).Scan(&portalUsed)
		if err != nil {
			fail()
			return
		}
		if coreUsed+portalUsed+float64(pay)/100 > float64(p.ChannelDaily)/100 {
			writeAPIError(w, 429, "PAYMENT_CHANNEL_LIMIT", "支付通道当日额度不足")
			return
		}
	}
	row := tx.QueryRowContext(r.Context(), `INSERT INTO portal_account_payment_orders(user_id,out_trade_no,amount_cents,pay_cents,fee_rate,provider_id,merchant_id,gateway_origin,provider_snapshot,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,CURRENT_TIMESTAMP+($10*INTERVAL '1 minute')) RETURNING `+accountPaymentColumns, user, trade, cents, pay, p.Fee, p.ID, p.PID, p.Base, snapshot, p.Timeout)
	o, err := scanAccountPayment(row)
	if err != nil {
		fail()
		return
	}
	if err = tx.Commit(); err != nil {
		fail()
		return
	}
	writeSuccess(w, map[string]any{"order_id": o.ID, "out_trade_no": o.Trade, "amount": o.Amount, "pay_amount": o.PayAmount, "fee_rate": o.FeeRate, "expires_at": o.ExpiresAt, "order_type": o.OrderType, "pay_url": s.accountPaymentURL(p, trade, pay, input.IsMobile)})
}

func (s *Server) accountPaymentList(w http.ResponseWriter, r *http.Request) {
	user, ok := s.accountPaymentUser(w, r)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	if page > 100000 {
		page = 100000
	}
	size := 20
	var total int
	if err := s.portalDB.QueryRowContext(r.Context(), `SELECT count(*) FROM portal_account_payment_orders WHERE user_id=$1`, user).Scan(&total); err != nil {
		writeAPIError(w, 500, "PAYMENT_READ_FAILED", "读取充值记录失败")
		return
	}
	rows, err := s.portalDB.QueryContext(r.Context(), `SELECT `+accountPaymentColumns+` FROM portal_account_payment_orders WHERE user_id=$1 ORDER BY id DESC LIMIT $2 OFFSET $3`, user, size, (page-1)*size)
	if err != nil {
		writeAPIError(w, 500, "PAYMENT_READ_FAILED", "读取充值记录失败")
		return
	}
	defer rows.Close()
	items := make([]accountPaymentOrder, 0)
	for rows.Next() {
		o, err := scanAccountPayment(rows)
		if err != nil {
			writeAPIError(w, 500, "PAYMENT_READ_FAILED", "读取充值记录失败")
			return
		}
		items = append(items, o)
	}
	if rows.Err() != nil {
		writeAPIError(w, 500, "PAYMENT_READ_FAILED", "读取充值记录失败")
		return
	}
	writeSuccess(w, map[string]any{"items": items, "total": total, "page": page, "page_size": size, "pages": maxInt(1, (total+size-1)/size)})
}

func (s *Server) accountPaymentGet(w http.ResponseWriter, r *http.Request) {
	user, ok := s.accountPaymentUser(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r, "orderID")
	if !ok {
		return
	}
	s.respondAccountPayment(w, r, user, "id", id)
}

func (s *Server) accountPaymentVerify(w http.ResponseWriter, r *http.Request) {
	user, ok := s.accountPaymentUser(w, r)
	if !ok {
		return
	}
	var input struct {
		Trade string `json:"out_trade_no"`
	}
	if decodeJSON(r, &input) != nil || !strings.HasPrefix(input.Trade, "acct_") || len(input.Trade) > 64 {
		writeAPIError(w, 400, "INVALID_PAYMENT", "充值订单号无效")
		return
	}
	s.respondAccountPayment(w, r, user, "out_trade_no", input.Trade)
}

func (s *Server) respondAccountPayment(w http.ResponseWriter, r *http.Request, user int64, column string, value any) {
	o, err := scanAccountPayment(s.portalDB.QueryRowContext(r.Context(), `SELECT `+accountPaymentColumns+` FROM portal_account_payment_orders WHERE user_id=$1 AND `+column+`=$2`, user, value))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, 404, "PAYMENT_NOT_FOUND", "充值订单不存在")
		return
	}
	if err != nil {
		writeAPIError(w, 500, "PAYMENT_READ_FAILED", "读取充值订单失败")
		return
	}
	if o.Status != "COMPLETED" {
		// A durable lease rate-limits gateway queries across tabs and workers.
		if err := s.reconcileAccountPayment(r.Context(), o.ID); err != nil {
			s.logger.Warn("Account payment verification deferred", "order_id", o.ID)
		}
		o, err = scanAccountPayment(s.portalDB.QueryRowContext(r.Context(), `SELECT `+accountPaymentColumns+` FROM portal_account_payment_orders WHERE id=$1 AND user_id=$2`, o.ID, user))
		if err != nil {
			writeAPIError(w, 500, "PAYMENT_READ_FAILED", "读取充值订单失败")
			return
		}
	}
	if o.Status == "PENDING" {
		var sealed []byte
		if err := s.portalDB.QueryRowContext(r.Context(), `SELECT provider_snapshot FROM portal_account_payment_orders WHERE id=$1 AND user_id=$2`, o.ID, user).Scan(&sealed); err == nil {
			if p, err := s.openPayment(sealed, o.Trade); err == nil {
				if _, err := paymentURL(s.accountPayment.PublicURL); err == nil {
					pay, err := paymentCentsLimit(strconv.FormatFloat(o.PayAmount, 'f', 2, 64), 200000000)
					if err == nil {
						o.PayURL = s.accountPaymentURL(p, o.Trade, pay, false)
					}
				}
			}
		}
	}
	writeSuccess(w, o)
}

func (s *Server) accountPaymentWebhook(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fail := func(code int) { w.WriteHeader(code); _, _ = io.WriteString(w, "fail") }
	if s.portalDB == nil {
		fail(503)
		return
	}
	var values url.Values
	var err error
	if r.Method == http.MethodPost {
		body, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
		if e != nil {
			fail(400)
			return
		}
		values, err = url.ParseQuery(string(body))
		if r.URL.RawQuery != "" {
			fail(400)
			return
		}
	} else {
		values, err = url.ParseQuery(r.URL.RawQuery)
	}
	if err != nil || !strings.HasPrefix(values.Get("out_trade_no"), "acct_") {
		fail(400)
		return
	}
	var sealed []byte
	err = s.portalDB.QueryRowContext(r.Context(), `SELECT provider_snapshot FROM portal_account_payment_orders WHERE out_trade_no=$1`, values.Get("out_trade_no")).Scan(&sealed)
	if err != nil {
		fail(404)
		return
	}
	p, err := s.openPayment(sealed, values.Get("out_trade_no"))
	if err != nil {
		fail(503)
		return
	}
	if !validateAccountPaymentSignature(values, p.Key) {
		fail(400)
		return
	}
	if err = s.settleAccountPayment(r.Context(), p, values); err != nil {
		fail(400)
		return
	}
	_, _ = io.WriteString(w, "success")
}

func (s *Server) settleAccountPayment(ctx context.Context, p accountPaymentProvider, values url.Values) error {
	// The gateway total includes a fee of up to 100% above the face-value cap.
	paid, err := paymentCentsLimit(values.Get("money"), 200000000)
	// Some gateways omit pid; the verified signature and encrypted order snapshot bind the merchant.
	if err != nil || (values.Get("pid") != "" && values.Get("pid") != p.PID) || values.Get("type") != "alipay" || values.Get("trade_status") != "TRADE_SUCCESS" || strings.TrimSpace(values.Get("trade_no")) == "" || len(values.Get("trade_no")) > 128 {
		return errors.New("invalid payment notification")
	}
	tx, err := s.portalDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id, user, amount, expected, providerID int64
	var status, merchant, base string
	var existing sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT id,user_id,amount_cents,pay_cents,status,trade_no,provider_id,merchant_id,gateway_origin FROM portal_account_payment_orders WHERE out_trade_no=$1 FOR UPDATE`, values.Get("out_trade_no")).Scan(&id, &user, &amount, &expected, &status, &existing, &providerID, &merchant, &base)
	if err != nil {
		return err
	}
	if expected != paid || providerID != p.ID || merchant != p.PID || base != p.Base {
		return errors.New("payment identity or amount mismatch")
	}
	if status == "COMPLETED" {
		if existing.String != values.Get("trade_no") {
			return errors.New("payment trade mismatch")
		}
		return nil
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO portal_account_wallets(user_id) VALUES($1) ON CONFLICT DO NOTHING`, user); err != nil {
		return err
	}
	// Ledger, wallet and completion are committed together; duplicates cannot credit twice.
	var after, before string
	err = tx.QueryRowContext(ctx, `UPDATE portal_account_wallets SET balance=balance+$2::numeric,updated_at=CURRENT_TIMESTAMP WHERE user_id=$1 RETURNING balance::text,(balance-$2::numeric)::text`, user, centsText(amount)).Scan(&after, &before)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO portal_account_wallet_ledger(user_id,kind,amount,balance_before,balance_after,reference_type,reference_id,note) VALUES($1,'adjustment',$2::numeric,$3::numeric,$4::numeric,'account_payment',$5,'支付宝充值账号套餐余额')`, user, centsText(amount), before, after, id)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE portal_account_payment_orders SET status='COMPLETED',completed_at=CURRENT_TIMESTAMP,trade_no=$2 WHERE id=$1`, id, values.Get("trade_no"))
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Server) reconcileAccountPayment(ctx context.Context, id int64) error {
	var trade string
	var sealed []byte
	err := s.portalDB.QueryRowContext(ctx, `UPDATE portal_account_payment_orders SET next_check_at=CURRENT_TIMESTAMP+INTERVAL '60 seconds',status=CASE WHEN expires_at<CURRENT_TIMESTAMP THEN 'EXPIRED' ELSE status END WHERE id=$1 AND status<>'COMPLETED' AND next_check_at<=CURRENT_TIMESTAMP RETURNING out_trade_no,provider_snapshot`, id).Scan(&trade, &sealed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	p, err := s.openPayment(sealed, trade)
	if err != nil {
		return err
	}
	values, paid, err := queryAccountPayment(ctx, p, trade)
	if err != nil {
		return err
	}
	if !paid {
		return nil
	}
	return s.settleAccountPayment(ctx, p, values)
}

func RunAccountPaymentWorker(ctx context.Context, db *sql.DB, logger *slog.Logger, config AccountPaymentConfig) {
	if db == nil || len(config.Secret) < 32 {
		return
	}
	s := &Server{portalDB: db, logger: logger, accountPayment: config}
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rows, err := db.QueryContext(ctx, `SELECT id FROM portal_account_payment_orders WHERE status<>'COMPLETED' AND created_at>CURRENT_TIMESTAMP-INTERVAL '30 days' AND next_check_at<=CURRENT_TIMESTAMP ORDER BY next_check_at LIMIT 25`)
			if err != nil {
				logger.Warn("Account payment reconciliation scan failed")
				continue
			}
			var ids []int64
			for rows.Next() {
				var id int64
				if rows.Scan(&id) == nil {
					ids = append(ids, id)
				}
			}
			rows.Close()
			for _, id := range ids {
				if ctx.Err() != nil {
					return
				}
				if err := s.reconcileAccountPayment(ctx, id); err != nil {
					logger.Warn("Account payment reconciliation deferred", "order_id", id)
				}
			}
		}
	}
}
