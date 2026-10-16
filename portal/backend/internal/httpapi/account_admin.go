package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

type accountAdminUserBrief struct {
	ID       int64   `json:"id"`
	Email    string  `json:"email"`
	Username string  `json:"username"`
	Status   string  `json:"status"`
	Balance  float64 `json:"balance"`
}

type accountAdminTransaction struct {
	ID          int64                  `json:"id"`
	Kind        string                 `json:"kind"`
	UserID      int64                  `json:"user_id"`
	User        *accountAdminUserBrief `json:"user,omitempty"`
	Reference   string                 `json:"reference"`
	BatchName   string                 `json:"batch_name,omitempty"`
	Note        string                 `json:"note,omitempty"`
	OperatorID  *int64                 `json:"operator_id,omitempty"`
	Amount      float64                `json:"amount"`
	PayAmount   float64                `json:"pay_amount,omitempty"`
	Status      string                 `json:"status"`
	CreatedAt   time.Time              `json:"created_at"`
	CompletedAt *time.Time             `json:"completed_at"`
}

func accountAdminBrief(user coreclient.User) accountAdminUserBrief {
	return accountAdminUserBrief{ID: user.ID, Email: user.Email, Username: user.Username, Status: user.Status, Balance: user.Balance}
}

func (s *Server) accountAdminUsersByID(ctx context.Context, ids []int64) map[int64]accountAdminUserBrief {
	result := make(map[int64]accountAdminUserBrief, len(ids))
	seen := make(map[int64]bool, len(ids))
	unique := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		unique = append(unique, id)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, 6)
	for _, id := range unique {
		wg.Add(1)
		go func(userID int64) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			user, err := s.core.AdminUser(ctx, userID)
			if err == nil {
				mu.Lock()
				result[userID] = accountAdminBrief(user)
				mu.Unlock()
			}
		}(id)
	}
	wg.Wait()
	return result
}

func (s *Server) accountAdminSearchUserIDs(ctx context.Context, search string) ([]int64, error) {
	search = strings.TrimSpace(search)
	if search == "" {
		return nil, nil
	}
	ids := make([]int64, 0, 20)
	seen := map[int64]bool{}
	if id, err := strconv.ParseInt(search, 10, 64); err == nil && id > 0 {
		if user, lookupErr := s.core.AdminUser(ctx, id); lookupErr == nil {
			seen[user.ID] = true
			ids = append(ids, user.ID)
		}
	}
	page, err := s.core.AdminUsers(ctx, 1, 100, coreclient.AdminUsersSearch(search))
	if err != nil {
		if len(ids) > 0 {
			return ids, nil
		}
		return nil, err
	}
	for _, user := range page.Items {
		if user.ID > 0 && !seen[user.ID] {
			seen[user.ID] = true
			ids = append(ids, user.ID)
		}
	}
	return ids, nil
}

func addAccountAdminSearchClause(where *[]string, args *[]any, textColumn string, userColumn string, search string, userIDs []int64) {
	search = strings.TrimSpace(search)
	if search == "" {
		return
	}
	parts := make([]string, 0, 2)
	*args = append(*args, "%"+search+"%")
	parts = append(parts, fmt.Sprintf("%s ILIKE $%d", textColumn, len(*args)))
	if len(userIDs) > 0 {
		placeholders := make([]string, 0, len(userIDs))
		for _, id := range userIDs {
			*args = append(*args, id)
			placeholders = append(placeholders, "$"+strconv.Itoa(len(*args)))
		}
		parts = append(parts, fmt.Sprintf("%s IN (%s)", userColumn, strings.Join(placeholders, ",")))
	}
	*where = append(*where, "("+strings.Join(parts, " OR ")+")")
}

func (s *Server) accountMarketAdminTransactions(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireActiveAdmin(w, r); !ok || !s.requireMarketDB(w) {
		return
	}
	kind := strings.TrimSpace(r.URL.Query().Get("type"))
	if kind == "" {
		kind = "payment"
	}
	if kind != "payment" && kind != "redemption" && kind != "adjustment" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_TRANSACTION_TYPE", "记录类型无效")
		return
	}
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	if len(search) > 200 {
		writeAPIError(w, http.StatusBadRequest, "INVALID_SEARCH", "搜索内容过长")
		return
	}
	userIDs, err := s.accountAdminSearchUserIDs(r.Context(), search)
	if err != nil {
		writeAPIError(w, http.StatusBadGateway, "CORE_USER_SEARCH_FAILED", "用户搜索暂时不可用")
		return
	}
	if kind == "payment" {
		s.accountMarketAdminPaymentTransactions(w, r, search, userIDs)
		return
	}
	if kind == "redemption" {
		s.accountMarketAdminRedemptionTransactions(w, r, search, userIDs)
		return
	}
	s.accountMarketAdminAdjustmentTransactions(w, r, search, userIDs)
}

func (s *Server) accountMarketAdminPaymentTransactions(w http.ResponseWriter, r *http.Request, search string, userIDs []int64) {
	page, pageSize := parsePageQuery(r.URL.Query())
	if pageSize > 100 {
		pageSize = 100
	}
	where := []string{"1=1"}
	args := make([]any, 0)
	status := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" {
		if status != "PENDING" && status != "COMPLETED" && status != "EXPIRED" {
			writeAPIError(w, http.StatusBadRequest, "INVALID_TRANSACTION_STATUS", "充值状态无效")
			return
		}
		args = append(args, status)
		where = append(where, "CASE WHEN status='PENDING' AND expires_at<CURRENT_TIMESTAMP THEN 'EXPIRED' ELSE status END=$"+strconv.Itoa(len(args)))
	}
	addAccountAdminSearchClause(&where, &args, "(out_trade_no || ' ' || COALESCE(trade_no,''))", "user_id", search, userIDs)
	base := " FROM portal_account_payment_orders WHERE " + strings.Join(where, " AND ")
	var total, completed int
	var totalAmount float64
	if err := s.portalDB.QueryRowContext(r.Context(), "SELECT COUNT(*),COUNT(*) FILTER(WHERE status='COMPLETED'),COALESCE(SUM(amount_cents) FILTER(WHERE status='COMPLETED'),0)/100.0"+base, args...).Scan(&total, &completed, &totalAmount); err != nil {
		writeAPIError(w, 500, "ACCOUNT_TRANSACTION_READ_FAILED", "充值记录加载失败")
		return
	}
	queryArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	query := `SELECT id,user_id,out_trade_no,amount_cents/100.0,pay_cents/100.0,
		CASE WHEN status='PENDING' AND expires_at<CURRENT_TIMESTAMP THEN 'EXPIRED' ELSE status END,
		created_at,completed_at` + base + ` ORDER BY created_at DESC,id DESC LIMIT $` + strconv.Itoa(len(queryArgs)-1) + ` OFFSET $` + strconv.Itoa(len(queryArgs))
	rows, err := s.portalDB.QueryContext(r.Context(), query, queryArgs...)
	if err != nil {
		writeAPIError(w, 500, "ACCOUNT_TRANSACTION_READ_FAILED", "充值记录加载失败")
		return
	}
	defer rows.Close()
	items := make([]accountAdminTransaction, 0, pageSize)
	userList := make([]int64, 0, pageSize)
	for rows.Next() {
		var item accountAdminTransaction
		var completedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.UserID, &item.Reference, &item.Amount, &item.PayAmount, &item.Status, &item.CreatedAt, &completedAt); err != nil {
			writeAPIError(w, 500, "ACCOUNT_TRANSACTION_READ_FAILED", "充值记录解析失败")
			return
		}
		item.Kind = "payment"
		if completedAt.Valid {
			item.CompletedAt = &completedAt.Time
		}
		items = append(items, item)
		userList = append(userList, item.UserID)
	}
	users := s.accountAdminUsersByID(r.Context(), userList)
	for i := range items {
		if user, ok := users[items[i].UserID]; ok {
			value := user
			items[i].User = &value
		}
	}
	writeSuccess(w, map[string]any{"items": items, "total": total, "page": page, "page_size": pageSize, "pages": maxInt(1, (total+pageSize-1)/pageSize), "summary": map[string]any{"total_amount": totalAmount, "completed_count": completed}})
}

func (s *Server) accountMarketAdminRedemptionTransactions(w http.ResponseWriter, r *http.Request, search string, userIDs []int64) {
	page, pageSize := parsePageQuery(r.URL.Query())
	if pageSize > 100 {
		pageSize = 100
	}
	where := []string{"c.redeemed_user_id IS NOT NULL"}
	args := make([]any, 0)
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" {
		if status != "redeemed" && status != "partially_used" && status != "used" {
			writeAPIError(w, http.StatusBadRequest, "INVALID_TRANSACTION_STATUS", "兑换状态无效")
			return
		}
		args = append(args, status)
		where = append(where, "c.status=$"+strconv.Itoa(len(args)))
	}
	addAccountAdminSearchClause(&where, &args, "(b.name || ' ' || c.code_suffix)", "c.redeemed_user_id", search, userIDs)
	base := ` FROM portal_account_redeem_codes c JOIN portal_account_redeem_batches b ON b.id=c.batch_id WHERE ` + strings.Join(where, " AND ")
	var total int
	var totalAmount float64
	if err := s.portalDB.QueryRowContext(r.Context(), "SELECT COUNT(*),COALESCE(SUM(c.amount),0)"+base, args...).Scan(&total, &totalAmount); err != nil {
		writeAPIError(w, 500, "ACCOUNT_TRANSACTION_READ_FAILED", "兑换记录加载失败")
		return
	}
	queryArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	query := `SELECT c.id,c.redeemed_user_id,c.code_suffix,b.name,c.amount,c.status,c.redeemed_at` + base + ` ORDER BY c.redeemed_at DESC NULLS LAST,c.id DESC LIMIT $` + strconv.Itoa(len(queryArgs)-1) + ` OFFSET $` + strconv.Itoa(len(queryArgs))
	rows, err := s.portalDB.QueryContext(r.Context(), query, queryArgs...)
	if err != nil {
		writeAPIError(w, 500, "ACCOUNT_TRANSACTION_READ_FAILED", "兑换记录加载失败")
		return
	}
	defer rows.Close()
	items := make([]accountAdminTransaction, 0, pageSize)
	userList := make([]int64, 0, pageSize)
	for rows.Next() {
		var item accountAdminTransaction
		var redeemedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.UserID, &item.Reference, &item.BatchName, &item.Amount, &item.Status, &redeemedAt); err != nil {
			writeAPIError(w, 500, "ACCOUNT_TRANSACTION_READ_FAILED", "兑换记录解析失败")
			return
		}
		item.Kind = "redemption"
		if redeemedAt.Valid {
			item.CreatedAt = redeemedAt.Time
			item.CompletedAt = &redeemedAt.Time
		}
		items = append(items, item)
		userList = append(userList, item.UserID)
	}
	users := s.accountAdminUsersByID(r.Context(), userList)
	for i := range items {
		if user, ok := users[items[i].UserID]; ok {
			value := user
			items[i].User = &value
		}
	}
	writeSuccess(w, map[string]any{"items": items, "total": total, "page": page, "page_size": pageSize, "pages": maxInt(1, (total+pageSize-1)/pageSize), "summary": map[string]any{"total_amount": totalAmount, "completed_count": total}})
}

func (s *Server) accountMarketAdminAdjustmentTransactions(w http.ResponseWriter, r *http.Request, search string, userIDs []int64) {
	page, pageSize := parsePageQuery(r.URL.Query())
	if pageSize > 100 {
		pageSize = 100
	}
	status := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" && status != "COMPLETED" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_TRANSACTION_STATUS", "手动调整状态无效")
		return
	}
	where := []string{"kind='adjustment'", "reference_type IN ('admin_account_recharge','admin_account_debit')"}
	args := make([]any, 0)
	addAccountAdminSearchClause(&where, &args, "note", "user_id", search, userIDs)
	base := " FROM portal_account_wallet_ledger WHERE " + strings.Join(where, " AND ")
	var total int
	var totalAmount float64
	if err := s.portalDB.QueryRowContext(r.Context(), "SELECT COUNT(*),COALESCE(SUM(amount),0)"+base, args...).Scan(&total, &totalAmount); err != nil {
		writeAPIError(w, 500, "ACCOUNT_TRANSACTION_READ_FAILED", "手动调整记录加载失败")
		return
	}
	queryArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	query := `SELECT id,user_id,amount,reference_id,note,created_at` + base + ` ORDER BY created_at DESC,id DESC LIMIT $` + strconv.Itoa(len(queryArgs)-1) + ` OFFSET $` + strconv.Itoa(len(queryArgs))
	rows, err := s.portalDB.QueryContext(r.Context(), query, queryArgs...)
	if err != nil {
		writeAPIError(w, 500, "ACCOUNT_TRANSACTION_READ_FAILED", "手动调整记录加载失败")
		return
	}
	defer rows.Close()
	items := make([]accountAdminTransaction, 0, pageSize)
	userList := make([]int64, 0, pageSize)
	for rows.Next() {
		var item accountAdminTransaction
		var operatorID sql.NullInt64
		if err := rows.Scan(&item.ID, &item.UserID, &item.Amount, &operatorID, &item.Note, &item.CreatedAt); err != nil {
			writeAPIError(w, 500, "ACCOUNT_TRANSACTION_READ_FAILED", "手动调整记录解析失败")
			return
		}
		item.Kind = "adjustment"
		item.Reference = fmt.Sprintf("ADJ-%d", item.ID)
		item.Status = "COMPLETED"
		item.CompletedAt = &item.CreatedAt
		if operatorID.Valid {
			item.OperatorID = &operatorID.Int64
		}
		items = append(items, item)
		userList = append(userList, item.UserID)
	}
	if err := rows.Err(); err != nil {
		writeAPIError(w, 500, "ACCOUNT_TRANSACTION_READ_FAILED", "手动调整记录加载失败")
		return
	}
	users := s.accountAdminUsersByID(r.Context(), userList)
	for i := range items {
		if user, ok := users[items[i].UserID]; ok {
			value := user
			items[i].User = &value
		}
	}
	writeSuccess(w, map[string]any{"items": items, "total": total, "page": page, "page_size": pageSize, "pages": maxInt(1, (total+pageSize-1)/pageSize), "summary": map[string]any{"total_amount": totalAmount, "completed_count": total}})
}

func (s *Server) accountMarketAdminCreditUserBalance(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireActiveAdmin(w, r)
	if !ok || !s.requireMarketDB(w) {
		return
	}
	userID, ok := pathID(w, r, "userID")
	if !ok {
		return
	}
	var input struct {
		Operation string  `json:"operation"`
		Amount    float64 `json:"amount"`
		Note      string  `json:"note"`
	}
	input.Operation = "credit"
	if err := decodeJSON(r, &input); err != nil || input.Amount <= 0 || input.Amount > 1_000_000 {
		writeAPIError(w, http.StatusBadRequest, "INVALID_ACCOUNT_ADJUSTMENT", "调整金额必须大于 0 且不超过 100 万元")
		return
	}
	operation := strings.ToLower(strings.TrimSpace(input.Operation))
	if operation != "credit" && operation != "debit" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_ACCOUNT_ADJUSTMENT", "余额调整类型无效")
		return
	}
	amount := math.Round(input.Amount*100) / 100
	if math.Abs(amount-input.Amount) > 0.000000001 {
		writeAPIError(w, http.StatusBadRequest, "INVALID_ACCOUNT_ADJUSTMENT", "调整金额最多保留两位小数")
		return
	}
	note := strings.TrimSpace(input.Note)
	if len([]rune(note)) > 200 {
		writeAPIError(w, http.StatusBadRequest, "INVALID_ACCOUNT_ADJUSTMENT", "调整备注不能超过 200 个字符")
		return
	}
	if _, err := s.core.AdminUser(r.Context(), userID); errors.Is(err, coreclient.ErrNotFound) {
		writeAPIError(w, http.StatusNotFound, "USER_NOT_FOUND", "用户不存在")
		return
	} else if err != nil {
		writeAPIError(w, http.StatusBadGateway, "CORE_USER_READ_FAILED", "用户信息暂时不可用")
		return
	}

	tx, err := s.portalDB.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_ADJUSTMENT_FAILED", "账号套餐余额调整失败")
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO portal_account_wallets(user_id) VALUES($1) ON CONFLICT(user_id) DO NOTHING`, userID); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_ADJUSTMENT_FAILED", "账号套餐余额初始化失败")
		return
	}
	var balanceBefore, balanceAfter float64
	if err := tx.QueryRowContext(r.Context(), `SELECT balance FROM portal_account_wallets WHERE user_id=$1 FOR UPDATE`, userID).Scan(&balanceBefore); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_ADJUSTMENT_FAILED", "账号套餐余额读取失败")
		return
	}
	if operation == "debit" && amount > balanceBefore {
		writeAPIError(w, http.StatusConflict, "INSUFFICIENT_ACCOUNT_BALANCE", "扣款金额不能超过当前账号套餐余额")
		return
	}
	amountParam := strconv.FormatFloat(amount, 'f', 2, 64)
	signedAmountParam := amountParam
	referenceType := "admin_account_recharge"
	auditNote := "管理员手动充值账号套餐余额"
	if operation == "debit" {
		signedAmountParam = "-" + amountParam
		referenceType = "admin_account_debit"
		auditNote = "管理员手动扣减账号套餐余额"
	}
	if err := tx.QueryRowContext(r.Context(), `UPDATE portal_account_wallets SET balance=balance+$2::numeric,updated_at=CURRENT_TIMESTAMP WHERE user_id=$1 RETURNING balance`, userID, signedAmountParam).Scan(&balanceAfter); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_ADJUSTMENT_FAILED", "账号套餐余额更新失败")
		return
	}
	if note != "" {
		auditNote += "：" + note
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO portal_account_wallet_ledger(user_id,kind,amount,balance_before,balance_after,reference_type,reference_id,note) VALUES($1,'adjustment',$2::numeric,$3,$4,$5,$6,$7)`, userID, signedAmountParam, balanceBefore, balanceAfter, referenceType, admin.ID, auditNote); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_ADJUSTMENT_FAILED", "账号套餐余额记录保存失败")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_ADJUSTMENT_FAILED", "账号套餐余额调整失败")
		return
	}
	writeSuccess(w, map[string]any{"user_id": userID, "operation": operation, "amount": amount, "balance_before": balanceBefore, "balance_after": balanceAfter, "operator_id": admin.ID, "note": note})
}

type accountAdminPackage struct {
	ID             int64      `json:"id"`
	Name           string     `json:"name"`
	Mode           string     `json:"mode"`
	ValidityDays   int        `json:"validity_days"`
	Quantity       int        `json:"quantity"`
	TotalPrice     float64    `json:"total_price"`
	Status         string     `json:"status"`
	PackageStatus  string     `json:"package_status"`
	AutoRenew      bool       `json:"auto_renew"`
	CoreAccountIDs []int64    `json:"core_account_ids"`
	StartsAt       *time.Time `json:"starts_at"`
	ExpiresAt      *time.Time `json:"expires_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (s *Server) accountMarketAdminUserOverview(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireActiveAdmin(w, r); !ok || !s.requireMarketDB(w) {
		return
	}
	userID, ok := pathID(w, r, "userID")
	if !ok {
		return
	}
	user, err := s.core.AdminUser(r.Context(), userID)
	if errors.Is(err, coreclient.ErrNotFound) {
		writeAPIError(w, http.StatusNotFound, "USER_NOT_FOUND", "用户不存在")
		return
	}
	if err != nil {
		writeAPIError(w, http.StatusBadGateway, "CORE_USER_READ_FAILED", "用户信息暂时不可用")
		return
	}
	var accountBalance float64
	err = s.portalDB.QueryRowContext(r.Context(), `SELECT balance FROM portal_account_wallets WHERE user_id=$1`, userID).Scan(&accountBalance)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, 500, "ACCOUNT_USER_READ_FAILED", "套餐余额读取失败")
		return
	}
	packages, err := s.accountAdminUserPackages(r.Context(), userID)
	if err != nil {
		writeAPIError(w, 500, "ACCOUNT_USER_READ_FAILED", "套餐记录读取失败")
		return
	}
	payments, err := s.accountAdminRecentPayments(r.Context(), userID)
	if err != nil {
		writeAPIError(w, 500, "ACCOUNT_USER_READ_FAILED", "充值记录读取失败")
		return
	}
	redemptions, err := s.accountAdminRecentRedemptions(r.Context(), userID)
	if err != nil {
		writeAPIError(w, 500, "ACCOUNT_USER_READ_FAILED", "兑换记录读取失败")
		return
	}
	var totalPayments, totalRedemptions, totalAdjustments, totalDeductions, totalSpent float64
	var paymentCount, redemptionCount, adjustmentCount, deductionCount, totalPackages, activePackages int
	if err := s.portalDB.QueryRowContext(r.Context(), `SELECT COALESCE(SUM(amount_cents) FILTER(WHERE status='COMPLETED'),0)/100.0,COUNT(*) FILTER(WHERE status='COMPLETED') FROM portal_account_payment_orders WHERE user_id=$1`, userID).Scan(&totalPayments, &paymentCount); err != nil {
		writeAPIError(w, 500, "ACCOUNT_USER_READ_FAILED", "充值汇总读取失败")
		return
	}
	if err := s.portalDB.QueryRowContext(r.Context(), `SELECT COALESCE(SUM(amount),0),COUNT(*) FROM portal_account_redeem_codes WHERE redeemed_user_id=$1`, userID).Scan(&totalRedemptions, &redemptionCount); err != nil {
		writeAPIError(w, 500, "ACCOUNT_USER_READ_FAILED", "兑换汇总读取失败")
		return
	}
	if err := s.portalDB.QueryRowContext(r.Context(), `SELECT COALESCE(SUM(amount),0),COUNT(*) FROM portal_account_wallet_ledger WHERE user_id=$1 AND kind='adjustment' AND reference_type='admin_account_recharge' AND amount>0`, userID).Scan(&totalAdjustments, &adjustmentCount); err != nil {
		writeAPIError(w, 500, "ACCOUNT_USER_READ_FAILED", "手动充值汇总读取失败")
		return
	}
	if err := s.portalDB.QueryRowContext(r.Context(), `SELECT COALESCE(-SUM(amount),0),COUNT(*) FROM portal_account_wallet_ledger WHERE user_id=$1 AND kind='adjustment' AND reference_type='admin_account_debit' AND amount<0`, userID).Scan(&totalDeductions, &deductionCount); err != nil {
		writeAPIError(w, 500, "ACCOUNT_USER_READ_FAILED", "手动扣款汇总读取失败")
		return
	}
	if err := s.portalDB.QueryRowContext(r.Context(), `SELECT COALESCE(-SUM(amount) FILTER(WHERE kind='purchase'),0) FROM portal_account_wallet_ledger WHERE user_id=$1`, userID).Scan(&totalSpent); err != nil {
		writeAPIError(w, 500, "ACCOUNT_USER_READ_FAILED", "消费汇总读取失败")
		return
	}
	if err := s.portalDB.QueryRowContext(r.Context(), `SELECT COUNT(*),COUNT(*) FILTER(WHERE status='completed' AND (expires_at IS NULL OR expires_at>CURRENT_TIMESTAMP)) FROM portal_account_orders WHERE user_id=$1`, userID).Scan(&totalPackages, &activePackages); err != nil {
		writeAPIError(w, 500, "ACCOUNT_USER_READ_FAILED", "套餐汇总读取失败")
		return
	}
	usage := map[string]any{}
	usageAvailable := true
	if err := s.rentalCoreRequest(r.Context(), http.MethodGet, fmt.Sprintf("/api/v1/admin/users/%d/usage?period=month", userID), nil, &usage, ""); err != nil {
		usageAvailable = false
		usage = map[string]any{}
	}
	writeSuccess(w, map[string]any{
		"user": accountAdminBrief(user), "account_balance": accountBalance, "usage": usage, "usage_available": usageAvailable,
		"summary":  map[string]any{"total_payments": totalPayments, "payment_count": paymentCount, "total_redemptions": totalRedemptions, "redemption_count": redemptionCount, "total_adjustments": totalAdjustments, "adjustment_count": adjustmentCount, "total_deductions": totalDeductions, "deduction_count": deductionCount, "total_spent": totalSpent, "total_packages": totalPackages, "active_packages": activePackages},
		"packages": packages, "payments": payments, "redemptions": redemptions,
	})
}

func (s *Server) accountAdminUserPackages(ctx context.Context, userID int64) ([]accountAdminPackage, error) {
	rows, err := s.portalDB.QueryContext(ctx, `SELECT id,product_name,mode,validity_days,quantity,total_price,status,
		CASE WHEN status='completed' AND (expires_at IS NULL OR expires_at>CURRENT_TIMESTAMP) THEN 'active' WHEN status='completed' THEN 'expired' ELSE status END,
		auto_renew,core_account_ids,fulfillment_started_at,expires_at,created_at
		FROM portal_account_orders WHERE user_id=$1 ORDER BY created_at DESC,id DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]accountAdminPackage, 0)
	for rows.Next() {
		var item accountAdminPackage
		var raw []byte
		var startsAt, expiresAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Name, &item.Mode, &item.ValidityDays, &item.Quantity, &item.TotalPrice, &item.Status, &item.PackageStatus, &item.AutoRenew, &raw, &startsAt, &expiresAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &item.CoreAccountIDs); err != nil {
			return nil, err
		}
		if startsAt.Valid {
			item.StartsAt = &startsAt.Time
		}
		if expiresAt.Valid {
			item.ExpiresAt = &expiresAt.Time
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Server) accountAdminRecentPayments(ctx context.Context, userID int64) ([]accountPaymentOrder, error) {
	rows, err := s.portalDB.QueryContext(ctx, `SELECT `+accountPaymentColumns+` FROM portal_account_payment_orders WHERE user_id=$1 ORDER BY created_at DESC,id DESC LIMIT 10`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]accountPaymentOrder, 0)
	for rows.Next() {
		item, err := scanAccountPayment(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Server) accountAdminRecentRedemptions(ctx context.Context, userID int64) ([]accountRedeemCode, error) {
	rows, err := s.portalDB.QueryContext(ctx, `SELECT c.id,c.batch_id,b.name,c.code_suffix,c.amount,c.remaining_amount,c.status,c.expires_at,c.redeemed_user_id,c.redeemed_at,c.fully_used_at,c.created_at FROM portal_account_redeem_codes c JOIN portal_account_redeem_batches b ON b.id=c.batch_id WHERE c.redeemed_user_id=$1 ORDER BY c.redeemed_at DESC NULLS LAST,c.id DESC LIMIT 10`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]accountRedeemCode, 0)
	for rows.Next() {
		var item accountRedeemCode
		var expiresAt, redeemedAt, fullyUsedAt sql.NullTime
		var redeemedUserID sql.NullInt64
		if err := rows.Scan(&item.ID, &item.BatchID, &item.BatchName, &item.CodeSuffix, &item.Amount, &item.RemainingAmount, &item.Status, &expiresAt, &redeemedUserID, &redeemedAt, &fullyUsedAt, &item.CreatedAt); err != nil {
			return nil, err
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
	return items, rows.Err()
}
