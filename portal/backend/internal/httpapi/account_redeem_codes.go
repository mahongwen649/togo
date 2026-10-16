package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const accountRedeemCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

type accountRedeemCode struct {
	ID              int64      `json:"id"`
	BatchID         int64      `json:"batch_id"`
	BatchName       string     `json:"batch_name"`
	Code            string     `json:"code,omitempty"`
	CodeSuffix      string     `json:"code_suffix"`
	Amount          float64    `json:"amount"`
	RemainingAmount float64    `json:"remaining_amount"`
	Status          string     `json:"status"`
	ExpiresAt       *time.Time `json:"expires_at"`
	RedeemedUserID  *int64     `json:"redeemed_user_id"`
	RedeemedAt      *time.Time `json:"redeemed_at"`
	FullyUsedAt     *time.Time `json:"fully_used_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

var errAccountWalletInsufficient = sql.ErrNoRows

func normalizeAccountRedeemCode(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func accountRedeemCodeHash(value string) string {
	sum := sha256.Sum256([]byte(normalizeAccountRedeemCode(value)))
	return hex.EncodeToString(sum[:])
}

func generateAccountRedeemCode() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	code := make([]byte, len(buf))
	for i, value := range buf {
		code[i] = accountRedeemCodeAlphabet[int(value)%len(accountRedeemCodeAlphabet)]
	}
	return fmt.Sprintf("ACCT-%s-%s-%s-%s", string(code[:4]), string(code[4:8]), string(code[8:12]), string(code[12:])), nil
}

func accountRedeemCodeSuffix(code string) string {
	code = normalizeAccountRedeemCode(code)
	if len(code) <= 4 {
		return code
	}
	return code[len(code)-4:]
}

func (s *Server) accountMarketRedeemCode(w http.ResponseWriter, r *http.Request) {
	if !s.requireMarketDB(w) {
		return
	}
	token, ok := bearerToken(w, r)
	if !ok {
		return
	}
	user, err := s.core.Profile(r.Context(), token)
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return
	}
	var input struct {
		Code string `json:"code"`
	}
	if decodeJSON(r, &input) != nil || normalizeAccountRedeemCode(input.Code) == "" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REDEEM_CODE", "请输入账号系统卡密")
		return
	}

	tx, err := s.portalDB.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_REDEEM_FAILED", "兑换失败，请稍后重试")
		return
	}
	defer tx.Rollback()

	var code accountRedeemCode
	var expiresAt, redeemedAt, fullyUsedAt sql.NullTime
	var redeemedUserID sql.NullInt64
	err = tx.QueryRowContext(r.Context(), `
		SELECT c.id, c.batch_id, b.name, c.code_suffix, c.amount, c.remaining_amount,
		       c.status, c.expires_at, c.redeemed_user_id, c.redeemed_at, c.fully_used_at, c.created_at
		FROM portal_account_redeem_codes c
		JOIN portal_account_redeem_batches b ON b.id = c.batch_id
		WHERE c.code_hash = $1
		FOR UPDATE`, accountRedeemCodeHash(input.Code)).Scan(
		&code.ID, &code.BatchID, &code.BatchName, &code.CodeSuffix, &code.Amount,
		&code.RemainingAmount, &code.Status, &expiresAt, &redeemedUserID, &redeemedAt,
		&fullyUsedAt, &code.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, http.StatusNotFound, "INVALID_REDEEM_CODE", "账号卡密不存在或已失效")
		return
	}
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_REDEEM_FAILED", "兑换失败，请稍后重试")
		return
	}
	if expiresAt.Valid {
		code.ExpiresAt = &expiresAt.Time
	}
	if redeemedUserID.Valid {
		code.RedeemedUserID = &redeemedUserID.Int64
	}
	if redeemedAt.Valid {
		code.RedeemedAt = &redeemedAt.Time
	}
	if fullyUsedAt.Valid {
		code.FullyUsedAt = &fullyUsedAt.Time
	}
	if code.Status == "active" && expiresAt.Valid && !expiresAt.Time.After(time.Now()) {
		_, _ = tx.ExecContext(r.Context(), `UPDATE portal_account_redeem_codes SET status='expired' WHERE id=$1`, code.ID)
		writeAPIError(w, http.StatusGone, "REDEEM_CODE_EXPIRED", "账号卡密已过期")
		return
	}
	if code.Status != "active" {
		writeAPIError(w, http.StatusConflict, "REDEEM_CODE_USED", "账号卡密已兑换或不可用")
		return
	}

	if _, err := tx.ExecContext(r.Context(), `INSERT INTO portal_account_wallets(user_id) VALUES($1) ON CONFLICT(user_id) DO NOTHING`, user.ID); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_REDEEM_FAILED", "账号余额初始化失败")
		return
	}
	var balanceBefore, balanceAfter float64
	if err := tx.QueryRowContext(r.Context(), `SELECT balance FROM portal_account_wallets WHERE user_id=$1 FOR UPDATE`, user.ID).Scan(&balanceBefore); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_REDEEM_FAILED", "账号余额读取失败")
		return
	}
	if err := tx.QueryRowContext(r.Context(), `UPDATE portal_account_wallets SET balance=balance+$2,updated_at=CURRENT_TIMESTAMP WHERE user_id=$1 RETURNING balance`, user.ID, code.Amount).Scan(&balanceAfter); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_REDEEM_FAILED", "账号余额更新失败")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE portal_account_redeem_codes SET status='redeemed', remaining_amount=amount, redeemed_user_id=$2, redeemed_at=CURRENT_TIMESTAMP WHERE id=$1`, code.ID, user.ID); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_REDEEM_FAILED", "账号卡密状态更新失败")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO portal_account_wallet_ledger(user_id,kind,amount,balance_before,balance_after,reference_type,reference_id,note) VALUES($1,'redemption',$2,$3,$4,'account_redeem_code',$5,'Account system redeem code')`, user.ID, code.Amount, balanceBefore, balanceAfter, code.ID); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_REDEEM_FAILED", "账号充值记录保存失败")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_REDEEM_FAILED", "兑换失败，请稍后重试")
		return
	}
	writeSuccess(w, map[string]any{"amount": code.Amount, "balance": balanceAfter, "code_suffix": code.CodeSuffix, "status": "redeemed"})
}

func (s *Server) accountMarketRedeemHistory(w http.ResponseWriter, r *http.Request) {
	if !s.requireMarketDB(w) {
		return
	}
	token, ok := bearerToken(w, r)
	if !ok {
		return
	}
	user, err := s.core.Profile(r.Context(), token)
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return
	}
	rows, err := s.portalDB.QueryContext(r.Context(), `
		SELECT c.id, c.batch_id, b.name, c.code_suffix, c.amount, c.remaining_amount,
		       CASE WHEN c.status='active' AND c.expires_at IS NOT NULL AND c.expires_at <= CURRENT_TIMESTAMP THEN 'expired' ELSE c.status END,
		       c.expires_at, c.redeemed_at, c.fully_used_at, c.created_at
		FROM portal_account_redeem_codes c
		JOIN portal_account_redeem_batches b ON b.id=c.batch_id
		WHERE c.redeemed_user_id=$1
		ORDER BY c.redeemed_at DESC NULLS LAST, c.id DESC`, user.ID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_REDEEM_HISTORY_FAILED", "充值记录加载失败")
		return
	}
	defer rows.Close()
	items := make([]accountRedeemCode, 0)
	for rows.Next() {
		var item accountRedeemCode
		var expiresAt, redeemedAt, fullyUsedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.BatchID, &item.BatchName, &item.CodeSuffix, &item.Amount, &item.RemainingAmount, &item.Status, &expiresAt, &redeemedAt, &fullyUsedAt, &item.CreatedAt); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_REDEEM_HISTORY_FAILED", "充值记录解析失败")
			return
		}
		if expiresAt.Valid {
			item.ExpiresAt = &expiresAt.Time
		}
		if redeemedAt.Valid {
			item.RedeemedAt = &redeemedAt.Time
		}
		if fullyUsedAt.Valid {
			item.FullyUsedAt = &fullyUsedAt.Time
		}
		items = append(items, item)
	}
	writeSuccess(w, map[string]any{"items": items})
}

func (s *Server) accountMarketAdminGenerateRedeemCodes(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireActiveAdmin(w, r)
	if !ok || !s.requireMarketDB(w) {
		return
	}
	var input struct {
		Name          string  `json:"name"`
		Amount        float64 `json:"amount"`
		Count         int     `json:"count"`
		ExpiresInDays int     `json:"expires_in_days"`
		Note          string  `json:"note"`
	}
	if decodeJSON(r, &input) != nil || input.Amount <= 0 || input.Count <= 0 || input.Count > 5000 {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REDEEM_BATCH", "请填写有效的面值和生成数量")
		return
	}
	if input.ExpiresInDays < 0 || input.ExpiresInDays > 3650 {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REDEEM_BATCH", "卡密有效期范围无效")
		return
	}
	var expiresAt *time.Time
	if input.ExpiresInDays > 0 {
		value := time.Now().Add(time.Duration(input.ExpiresInDays) * 24 * time.Hour)
		expiresAt = &value
	}
	tx, err := s.portalDB.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, 500, "REDEEM_BATCH_CREATE_FAILED", "卡密批次创建失败")
		return
	}
	defer tx.Rollback()
	var batchID int64
	if err := tx.QueryRowContext(r.Context(), `INSERT INTO portal_account_redeem_batches(name,amount,quantity,note,created_by_admin_id) VALUES($1,$2,$3,$4,$5) RETURNING id`, strings.TrimSpace(input.Name), input.Amount, input.Count, strings.TrimSpace(input.Note), admin.ID).Scan(&batchID); err != nil {
		writeAPIError(w, 500, "REDEEM_BATCH_CREATE_FAILED", "卡密批次创建失败")
		return
	}
	codes := make([]accountRedeemCode, 0, input.Count)
	for i := 0; i < input.Count; i++ {
		var plain string
		for {
			plain, err = generateAccountRedeemCode()
			if err != nil {
				writeAPIError(w, 500, "REDEEM_CODE_CREATE_FAILED", "卡密生成失败")
				return
			}
			var exists bool
			if err := tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM portal_account_redeem_codes WHERE code_hash=$1)`, accountRedeemCodeHash(plain)).Scan(&exists); err != nil {
				writeAPIError(w, 500, "REDEEM_CODE_CREATE_FAILED", "卡密生成失败")
				return
			}
			if !exists {
				break
			}
		}
		var id int64
		if err := tx.QueryRowContext(r.Context(), `INSERT INTO portal_account_redeem_codes(batch_id,code_hash,code_suffix,amount,remaining_amount,expires_at) VALUES($1,$2,$3,$4,$4,$5) RETURNING id`, batchID, accountRedeemCodeHash(plain), accountRedeemCodeSuffix(plain), input.Amount, expiresAt).Scan(&id); err != nil {
			writeAPIError(w, 500, "REDEEM_CODE_CREATE_FAILED", "卡密生成失败")
			return
		}
		codes = append(codes, accountRedeemCode{ID: id, BatchID: batchID, BatchName: strings.TrimSpace(input.Name), Code: plain, CodeSuffix: accountRedeemCodeSuffix(plain), Amount: input.Amount, RemainingAmount: input.Amount, Status: "active", ExpiresAt: expiresAt, CreatedAt: time.Now()})
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, 500, "REDEEM_BATCH_CREATE_FAILED", "卡密批次保存失败")
		return
	}
	writeSuccess(w, map[string]any{"batch_id": batchID, "name": strings.TrimSpace(input.Name), "amount": input.Amount, "count": input.Count, "expires_at": expiresAt, "codes": codes})
}

func (s *Server) accountMarketAdminRedeemCodes(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireActiveAdmin(w, r); !ok || !s.requireMarketDB(w) {
		return
	}
	page, pageSize := parsePageQuery(r.URL.Query())
	if pageSize > 100 {
		pageSize = 100
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	statusExpr := "CASE WHEN c.status='active' AND c.expires_at IS NOT NULL AND c.expires_at <= CURRENT_TIMESTAMP THEN 'expired' ELSE c.status END"
	where := []string{"1=1"}
	args := make([]any, 0)
	if status != "" {
		args = append(args, status)
		where = append(where, statusExpr+"=$"+strconv.Itoa(len(args)))
	}
	if search != "" {
		args = append(args, "%"+search+"%")
		n := strconv.Itoa(len(args))
		where = append(where, "(b.name ILIKE $"+n+" OR c.code_suffix ILIKE $"+n+")")
	}
	base := `FROM portal_account_redeem_codes c JOIN portal_account_redeem_batches b ON b.id=c.batch_id WHERE ` + strings.Join(where, " AND ")
	var total int
	if err := s.portalDB.QueryRowContext(r.Context(), "SELECT COUNT(*) "+base, args...).Scan(&total); err != nil {
		writeAPIError(w, 500, "REDEEM_CODE_READ_FAILED", "卡密记录加载失败")
		return
	}
	args = append(args, pageSize, (page-1)*pageSize)
	query := `SELECT c.id,c.batch_id,b.name,c.code_suffix,c.amount,c.remaining_amount,` + statusExpr + `,c.expires_at,c.redeemed_user_id,c.redeemed_at,c.fully_used_at,c.created_at ` + base + ` ORDER BY c.created_at DESC,c.id DESC LIMIT $` + strconv.Itoa(len(args)-1) + ` OFFSET $` + strconv.Itoa(len(args))
	rows, err := s.portalDB.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeAPIError(w, 500, "REDEEM_CODE_READ_FAILED", "卡密记录加载失败")
		return
	}
	defer rows.Close()
	items := make([]accountRedeemCode, 0)
	for rows.Next() {
		var item accountRedeemCode
		var expiresAt, redeemedAt, fullyUsedAt sql.NullTime
		var redeemedUserID sql.NullInt64
		if err := rows.Scan(&item.ID, &item.BatchID, &item.BatchName, &item.CodeSuffix, &item.Amount, &item.RemainingAmount, &item.Status, &expiresAt, &redeemedUserID, &redeemedAt, &fullyUsedAt, &item.CreatedAt); err != nil {
			writeAPIError(w, 500, "REDEEM_CODE_READ_FAILED", "卡密记录解析失败")
			return
		}
		if expiresAt.Valid {
			item.ExpiresAt = &expiresAt.Time
		}
		if redeemedUserID.Valid {
			item.RedeemedUserID = &redeemedUserID.Int64
		}
		if redeemedAt.Valid {
			item.RedeemedAt = &redeemedAt.Time
		}
		if fullyUsedAt.Valid {
			item.FullyUsedAt = &fullyUsedAt.Time
		}
		items = append(items, item)
	}
	writeSuccess(w, map[string]any{"items": items, "total": total, "page": page, "page_size": pageSize, "pages": maxInt(1, (total+pageSize-1)/pageSize)})
}

func (s *Server) accountMarketAdminDisableRedeemCode(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireActiveAdmin(w, r); !ok || !s.requireMarketDB(w) {
		return
	}
	id, ok := pathID(w, r, "codeID")
	if !ok {
		return
	}
	result, err := s.portalDB.ExecContext(r.Context(), `UPDATE portal_account_redeem_codes SET status='disabled',disabled_at=CURRENT_TIMESTAMP WHERE id=$1 AND status IN ('active','expired')`, id)
	if err != nil {
		writeAPIError(w, 500, "REDEEM_CODE_UPDATE_FAILED", "卡密状态更新失败")
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		writeAPIError(w, 409, "REDEEM_CODE_UPDATE_FAILED", "卡密不存在或已兑换")
		return
	}
	writeSuccess(w, map[string]any{"id": id, "status": "disabled"})
}

// debitAccountWalletTx debits the account wallet and consumes redeemed-code
// credit in FIFO order so administrators can see whether a card is unused,
// partially used, or fully used.
func debitAccountWalletTx(ctx context.Context, tx *sql.Tx, userID int64, amount float64) (float64, error) {
	amountParam := strconv.FormatFloat(amount, 'f', 8, 64)
	if _, err := tx.ExecContext(ctx, `INSERT INTO portal_account_wallets(user_id) VALUES($1) ON CONFLICT(user_id) DO NOTHING`, userID); err != nil {
		return 0, fmt.Errorf("initialize account wallet: %w", err)
	}
	var balance float64
	if err := tx.QueryRowContext(ctx, `SELECT balance FROM portal_account_wallets WHERE user_id=$1 FOR UPDATE`, userID).Scan(&balance); err != nil {
		return 0, fmt.Errorf("read account wallet: %w", err)
	}
	if balance < amount {
		return 0, errAccountWalletInsufficient
	}
	var balanceAfter float64
	if err := tx.QueryRowContext(ctx, `UPDATE portal_account_wallets SET balance=balance-$2::numeric,updated_at=CURRENT_TIMESTAMP WHERE user_id=$1 RETURNING balance`, userID, amountParam).Scan(&balanceAfter); err != nil {
		return 0, fmt.Errorf("debit account wallet: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,remaining_amount FROM portal_account_redeem_codes WHERE redeemed_user_id=$1 AND status IN ('redeemed','partially_used') AND remaining_amount>0 ORDER BY redeemed_at ASC,id ASC FOR UPDATE`, userID)
	if err != nil {
		return 0, fmt.Errorf("lock redeemed account credits: %w", err)
	}
	left := amount
	type consumed struct {
		id        int64
		remaining float64
	}
	consumedCodes := make([]consumed, 0)
	for left > 0 && rows.Next() {
		var id int64
		var remaining float64
		if err := rows.Scan(&id, &remaining); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("read redeemed account credit: %w", err)
		}
		use := remaining
		if use > left {
			use = left
		}
		consumedCodes = append(consumedCodes, consumed{id: id, remaining: remaining - use})
		left -= use
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, fmt.Errorf("iterate redeemed account credits: %w", err)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("close redeemed account credits: %w", err)
	}
	for _, item := range consumedCodes {
		remainingParam := strconv.FormatFloat(item.remaining, 'f', 8, 64)
		var err error
		if item.remaining <= 0 {
			_, err = tx.ExecContext(ctx, `UPDATE portal_account_redeem_codes SET remaining_amount=$2::numeric,status='used',fully_used_at=CURRENT_TIMESTAMP WHERE id=$1`, item.id, remainingParam)
		} else {
			_, err = tx.ExecContext(ctx, `UPDATE portal_account_redeem_codes SET remaining_amount=$2::numeric,status='partially_used',fully_used_at=NULL WHERE id=$1`, item.id, remainingParam)
		}
		if err != nil {
			return 0, fmt.Errorf("update redeemed account credit %d: %w", item.id, err)
		}
	}
	return balanceAfter, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
