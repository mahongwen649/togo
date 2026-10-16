package httpapi

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

type marketCore interface {
	AdminMarketRequest(context.Context, string, string, any, string) (coreclient.Response, error)
}

type rentalAccount struct {
	ID                 int64   `json:"id"`
	Name               string  `json:"name"`
	Platform           string  `json:"platform"`
	Type               string  `json:"type"`
	Status             string  `json:"status"`
	Schedulable        bool    `json:"schedulable"`
	GroupIDs           []int64 `json:"group_ids"`
	ExpiresAt          *int64  `json:"expires_at"`
	AutoPauseOnExpired bool    `json:"auto_pause_on_expired"`
	ParentAccountID    *int64  `json:"parent_account_id"`
	Available          bool    `json:"available"`
}

type rentalGroup struct {
	ImageRateMultiplier       float64                   `json:"image_rate_multiplier"`
	ModelPricing              []exclusiveCoreModelPrice `json:"model_pricing"`
	ImageRateIndependent      bool                      `json:"image_rate_independent"`
	ImagePrice1K              *float64                  `json:"image_price_1k"`
	ImagePrice2K              *float64                  `json:"image_price_2k"`
	ImagePrice4K              *float64                  `json:"image_price_4k"`
	RateMultiplier            float64                   `json:"rate_multiplier"`
	ID                        int64                     `json:"id"`
	Name                      string                    `json:"name"`
	Description               string                    `json:"description"`
	Platform                  string                    `json:"platform"`
	Status                    string                    `json:"status"`
	IsExclusive               bool                      `json:"is_exclusive"`
	SubscriptionType          string                    `json:"subscription_type"`
	AllowImageGeneration      bool                      `json:"allow_image_generation"`
	AllowBatchImageGeneration bool                      `json:"allow_batch_image_generation"`
	FallbackGroupID           *int64                    `json:"fallback_group_id"`
	FallbackInvalidGroupID    *int64                    `json:"fallback_group_id_on_invalid_request"`
}

type rentalOrder struct {
	ID, UserID, ProductID int64
	Quantity              int
	ValidityDays          int
	Mode, Status          string
	AccountIDs            []int64
	GroupID               sql.NullInt64
	StartedAt, ExpiresAt  sql.NullTime
}

type rentalProblem struct {
	code, message string
	status        int
}

func (e *rentalProblem) Error() string    { return e.message }
func rentalConflict(message string) error { return &rentalProblem{"RENTAL_CONFLICT", message, 409} }

func (s *Server) requireMarketDB(w http.ResponseWriter) bool {
	if s.portalDB == nil {
		writeAPIError(w, 503, "ACCOUNT_MARKET_UNAVAILABLE", "账号业务数据库尚未配置")
		return false
	}
	return true
}

func (s *Server) rentalCoreRequest(ctx context.Context, method, path string, payload, out any, key string) error {
	core, ok := s.core.(marketCore)
	if !ok {
		return errors.New("Core account business adapter is unavailable")
	}
	response, err := core.AdminMarketRequest(ctx, method, path, payload, key)
	if err != nil {
		return err
	}
	var envelope struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(response.Body, &envelope); err != nil {
		return err
	}
	if envelope.Code != 0 || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return errors.New("Core returned an unsuccessful account business response")
	}
	if out != nil {
		return json.Unmarshal(envelope.Data, out)
	}
	return nil
}

// Only display-safe account fields are decoded; upstream credentials never
// reach the Portal account picker or the customer's order response.
func (s *Server) accountMarketCoreAccounts(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireActiveAdmin(w, r); !ok || !s.requireMarketDB(w) {
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	if len(search) > 200 {
		writeAPIError(w, 400, "INVALID_SEARCH", "搜索条件过长")
		return
	}
	query := url.Values{"page": {strconv.Itoa(page)}, "page_size": {"50"}, "search": {search}, "lite": {"true"}}
	var result struct {
		Items []rentalAccount `json:"items"`
		Total int64           `json:"total"`
		Pages int             `json:"pages"`
		Page  int             `json:"page"`
	}
	if err := s.rentalCoreRequest(r.Context(), "GET", "/api/v1/admin/accounts?"+query.Encode(), nil, &result, ""); err != nil {
		s.writeRentalError(w, err)
		return
	}
	rows, err := s.portalDB.QueryContext(r.Context(), `SELECT core_account_id, order_id FROM portal_account_rental_reservations`)
	if err != nil {
		s.writeRentalError(w, err)
		return
	}
	defer rows.Close()
	reservations := map[int64]int64{}
	for rows.Next() {
		var accountID, orderID int64
		if err := rows.Scan(&accountID, &orderID); err != nil {
			s.writeRentalError(w, err)
			return
		}
		reservations[accountID] = orderID
	}
	if err := rows.Err(); err != nil {
		s.writeRentalError(w, err)
		return
	}
	orderID, _ := strconv.ParseInt(r.URL.Query().Get("order_id"), 10, 64)
	for i := range result.Items {
		a := &result.Items[i]
		reserved := reservations[a.ID]
		a.Available = a.Status == "active" && a.Schedulable && a.Platform == "openai" && a.ParentAccountID == nil && (a.ExpiresAt == nil || *a.ExpiresAt > time.Now().Unix()) && ((reserved == 0 && len(a.GroupIDs) == 0) || (orderID > 0 && reserved == orderID))
	}
	if result.Items == nil {
		result.Items = []rentalAccount{}
	}
	writeSuccess(w, result)
}

// accountMarketRefreshAccountUsage exposes Core's existing active usage query
// only for accounts currently assigned to the authenticated user's package.
// Portal does not calculate quota windows or expose Core's admin quota reset.
func (s *Server) accountMarketRefreshAccountUsage(w http.ResponseWriter, r *http.Request) {
	accessToken, ok := bearerToken(w, r)
	if !ok || !s.requireMarketDB(w) {
		return
	}
	accountID, ok := pathID(w, r, "accountID")
	if !ok {
		return
	}
	user, err := s.core.Profile(r.Context(), accessToken)
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return
	}

	var allowed bool
	if err := s.portalDB.QueryRowContext(r.Context(), `
		SELECT EXISTS (
			SELECT 1
			FROM portal_account_orders
			WHERE user_id = $1
			  AND status = 'completed'
			  AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
			  AND core_account_ids @> jsonb_build_array($2::bigint)
		)`, user.ID, strconv.FormatInt(accountID, 10)).Scan(&allowed); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_USAGE_READ_FAILED", "Failed to verify account access")
		return
	}
	if !allowed {
		writeAPIError(w, http.StatusNotFound, "ACCOUNT_NOT_FOUND", "该账号不在你的有效套餐中")
		return
	}

	var usage map[string]any
	path := fmt.Sprintf("/api/v1/admin/accounts/%d/usage?source=active&force=true", accountID)
	if err := s.rentalCoreRequest(r.Context(), http.MethodGet, path, nil, &usage, ""); err != nil {
		s.writeRentalError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"account_id": accountID, "usage": sanitizeCustomerMap(usage)})
}

func validateRentalSelection(ids []int64, quantity int) error {
	if len(ids) != quantity || quantity < 1 {
		return rentalConflict("所选账号数量必须与订单数量一致")
	}
	seen := map[int64]bool{}
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return rentalConflict("账号 ID 必须是不同的正整数")
		}
		seen[id] = true
	}
	return nil
}

func sameRentalSelection(a, b []int64) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

func validateRentalAccount(a rentalAccount, groupID int64, expiresAt time.Time) error {
	if a.ID <= 0 || a.Status != "active" || !a.Schedulable || a.ParentAccountID != nil {
		return rentalConflict(fmt.Sprintf("账号 #%d 不可租赁，请选择正常的独立账号", a.ID))
	}
	if a.Platform != "openai" {
		return rentalConflict("当前账号套餐仅支持 OpenAI 账号")
	}
	for _, id := range a.GroupIDs {
		if groupID == 0 || id != groupID {
			return rentalConflict(fmt.Sprintf("账号 #%d 仍属于其他分组，请先在 Core 确认用途并移出其他分组", a.ID))
		}
	}
	if a.ExpiresAt != nil && *a.ExpiresAt < expiresAt.Unix() {
		return rentalConflict(fmt.Sprintf("账号 #%d 的到期时间不足本次租赁周期", a.ID))
	}
	return nil
}

func validateExclusiveAccount(a rentalAccount, groupID int64, now time.Time) error {
	if isAPIKeyAccount(a) {
		a.ExpiresAt = nil
		a.GroupIDs = nil
	}
	return validateRentalAccount(a, groupID, now)
}

func isAPIKeyAccount(a rentalAccount) bool {
	return strings.EqualFold(strings.TrimSpace(a.Type), "apikey")
}

func (s *Server) writeRentalError(w http.ResponseWriter, err error) {
	var p *rentalProblem
	if errors.As(err, &p) {
		writeAPIError(w, p.status, p.code, p.message)
		return
	}
	s.logger.Warn("Account rental operation failed", "error", err)
	writeAPIError(w, 502, "RENTAL_SYNC_FAILED", "账号同步未完成，订单及所选账号已保留，请稍后重试")
}

func (s *Server) fulfillRental(w http.ResponseWriter, r *http.Request, id int64, accountIDs []int64) {
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(115 * time.Second))
	ctx, cancel := context.WithTimeout(r.Context(), 100*time.Second)
	defer cancel()
	order, err := s.provisionRental(ctx, id, accountIDs)
	if err != nil {
		// The detailed diagnostic is logged server-side, never copied from a
		// Core response (which may contain sensitive account data).
		saveCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		_, _ = s.portalDB.ExecContext(saveCtx, `UPDATE portal_account_orders SET fulfillment_error='Core 同步未完成，请重试', updated_at=CURRENT_TIMESTAMP WHERE id=$1 AND status='pending' AND fulfillment_started_at IS NOT NULL`, id)
		s.writeRentalError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"id": id, "status": "completed", "core_group_id": order.GroupID.Int64, "expires_at": order.ExpiresAt.Time})
}

func loadRentalOrder(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id int64, lock bool) (rentalOrder, error) {
	var o rentalOrder
	var raw []byte
	query := `SELECT id,user_id,product_id,quantity,validity_days,mode,status,core_account_ids,core_group_id,fulfillment_started_at,expires_at FROM portal_account_orders WHERE id=$1`
	if lock {
		query += " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, query, id).Scan(&o.ID, &o.UserID, &o.ProductID, &o.Quantity, &o.ValidityDays, &o.Mode, &o.Status, &raw, &o.GroupID, &o.StartedAt, &o.ExpiresAt)
	if err == sql.ErrNoRows {
		return o, &rentalProblem{"ORDER_NOT_FOUND", "订单不存在", 404}
	}
	if err != nil {
		return o, err
	}
	if err := json.Unmarshal(raw, &o.AccountIDs); err != nil {
		return o, err
	}
	return o, nil
}

func (s *Server) provisionRental(ctx context.Context, id int64, ids []int64) (rentalOrder, error) {
	// A session advisory lock serializes Portal allocation across processes.
	// Checkpoints are committed independently of remote HTTP requests.
	conn, err := s.portalDB.Conn(ctx)
	if err != nil {
		return rentalOrder{}, err
	}
	defer conn.Close()
	var locked bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(734210091)`).Scan(&locked); err != nil {
		return rentalOrder{}, err
	}
	if !locked {
		return rentalOrder{}, rentalConflict("另一个账号订单正在同步，请稍后重试")
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if _, err := conn.ExecContext(unlockCtx, `SELECT pg_advisory_unlock(734210091)`); err != nil {
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
	}()

	o, err := loadRentalOrder(ctx, conn, id, false)
	if err != nil {
		return o, err
	}
	if o.Mode != "rental" {
		return o, rentalConflict("购买订单应通过账号信息交付，不能使用租赁分组交付")
	}
	if o.ValidityDays < 1 || o.ValidityDays > 3650 {
		return o, rentalConflict("租号订单有效期配置无效")
	}
	if err := validateRentalSelection(ids, o.Quantity); err != nil {
		return o, err
	}
	if o.Status == "completed" && sameRentalSelection(ids, o.AccountIDs) && o.GroupID.Valid {
		return o, nil
	}
	if o.Status != "pending" {
		return o, rentalConflict("订单已处理，不能重复分配")
	}
	if o.StartedAt.Valid && !sameRentalSelection(ids, o.AccountIDs) {
		return o, rentalConflict("同步已开始，重试时不能更换已锁定的账号")
	}
	expiresAt := time.Now().UTC().Truncate(time.Second).Add(time.Duration(o.ValidityDays) * 24 * time.Hour)
	if o.ExpiresAt.Valid {
		expiresAt = o.ExpiresAt.Time
	}
	if !expiresAt.After(time.Now()) {
		return o, rentalConflict("该订单同步周期已过期，需要管理员核查")
	}
	var groupID int64
	err = conn.QueryRowContext(ctx, `SELECT core_group_id FROM portal_account_rental_groups WHERE user_id=$1 AND product_id=$2`, o.UserID, o.ProductID).Scan(&groupID)
	if err != nil && err != sql.ErrNoRows {
		return o, err
	}
	for _, id := range ids {
		var exclusiveReserved bool
		if err := conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM portal_exclusive_accounts WHERE core_account_id=$1)`, id).Scan(&exclusiveReserved); err != nil {
			return o, err
		}
		if exclusiveReserved {
			return o, rentalConflict("账号已被独享通道预留")
		}
		var account rentalAccount
		if err := s.rentalCoreRequest(ctx, "GET", fmt.Sprintf("/api/v1/admin/accounts/%d", id), nil, &account, ""); err != nil {
			return o, err
		}
		if account.ID != id {
			return o, errors.New("Core returned an unexpected account ID")
		}
		if err := validateRentalAccount(account, groupID, expiresAt); err != nil {
			return o, err
		}
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return o, err
	}
	defer tx.Rollback()
	o, err = loadRentalOrder(ctx, tx, o.ID, true)
	if err != nil {
		return o, err
	}
	if o.Status != "pending" {
		return o, rentalConflict("订单状态已变化，请刷新")
	}
	for _, accountID := range ids {
		var owner int64
		err := tx.QueryRowContext(ctx, `INSERT INTO portal_account_rental_reservations(core_account_id,order_id) VALUES($1,$2) ON CONFLICT(core_account_id) DO UPDATE SET order_id=portal_account_rental_reservations.order_id RETURNING order_id`, accountID, o.ID).Scan(&owner)
		if err != nil {
			return o, err
		}
		if owner != o.ID {
			return o, rentalConflict(fmt.Sprintf("账号 #%d 已被其他订单锁定", accountID))
		}
	}
	raw, err := json.Marshal(ids)
	if err != nil {
		return o, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE portal_account_orders SET core_account_ids=$1::jsonb,fulfillment_started_at=COALESCE(fulfillment_started_at,CURRENT_TIMESTAMP),expires_at=$2,fulfillment_error='',updated_at=CURRENT_TIMESTAMP WHERE id=$3`, string(raw), expiresAt, o.ID); err != nil {
		return o, err
	}
	if err := tx.Commit(); err != nil {
		return o, err
	}
	o.AccountIDs = ids
	o.ExpiresAt = sql.NullTime{Time: expiresAt, Valid: true}

	group, err := s.ensureRentalGroup(ctx, o, groupID)
	if err != nil {
		return o, err
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO portal_account_rental_groups(user_id,product_id,core_group_id,platform) VALUES($1,$2,$3,$4) ON CONFLICT(user_id,product_id) DO NOTHING`, o.UserID, o.ProductID, group.ID, group.Platform); err != nil {
		return o, err
	}
	o.GroupID = sql.NullInt64{Int64: group.ID, Valid: true}
	if _, err := conn.ExecContext(ctx, `UPDATE portal_account_orders SET core_group_id=$1 WHERE id=$2`, group.ID, o.ID); err != nil {
		return o, err
	}
	for _, accountID := range ids {
		var account rentalAccount
		path := fmt.Sprintf("/api/v1/admin/accounts/%d", accountID)
		// Re-read immediately before updating to detect intervening Core edits.
		if err := s.rentalCoreRequest(ctx, "GET", path, nil, &account, ""); err != nil {
			return o, err
		}
		if err := validateRentalAccount(account, group.ID, expiresAt); err != nil {
			return o, err
		}
		payload := map[string]any{"group_ids": []int64{group.ID}, "expires_at": expiresAt.Unix(), "auto_pause_on_expired": true}
		if err := s.rentalCoreRequest(ctx, "PUT", path, payload, &account, ""); err != nil {
			return o, err
		}
		if len(account.GroupIDs) != 1 || account.GroupIDs[0] != group.ID || account.ExpiresAt == nil || *account.ExpiresAt != expiresAt.Unix() || !account.AutoPauseOnExpired {
			return o, errors.New("Core did not apply the requested rental account restrictions")
		}
	}
	if err := s.verifyRentalGroupAccounts(ctx, o, group); err != nil {
		return o, err
	}
	if err := s.ensureRentalSubscription(ctx, o, group); err != nil {
		return o, err
	}

	done, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return o, err
	}
	defer done.Rollback()
	if _, err := done.ExecContext(ctx, `UPDATE portal_account_products SET stock_quantity=stock_quantity-$1,locked_quantity=locked_quantity-$1,updated_at=CURRENT_TIMESTAMP WHERE id=$2`, o.Quantity, o.ProductID); err != nil {
		return o, err
	}
	if _, err := done.ExecContext(ctx, `UPDATE portal_account_orders SET status='completed',completed_at=CURRENT_TIMESTAMP,fulfillment_error='',updated_at=CURRENT_TIMESTAMP WHERE id=$1 AND status='pending'`, o.ID); err != nil {
		return o, err
	}
	if err := done.Commit(); err != nil {
		return o, err
	}
	o.Status = "completed"
	return o, nil
}

func (s *Server) ensureRentalGroup(ctx context.Context, o rentalOrder, id int64) (rentalGroup, error) {
	name := fmt.Sprintf("portal-rental-u%d-p%d", o.UserID, o.ProductID)
	description := fmt.Sprintf("Portal rental owner=%d product=%d", o.UserID, o.ProductID)
	var group rentalGroup
	if id > 0 {
		if err := s.rentalCoreRequest(ctx, "GET", fmt.Sprintf("/api/v1/admin/groups/%d", id), nil, &group, ""); err != nil {
			return group, err
		}
	} else {
		var groups []rentalGroup
		if err := s.rentalCoreRequest(ctx, "GET", "/api/v1/admin/groups/all?include_inactive=true", nil, &groups, ""); err != nil {
			return group, err
		}
		for _, candidate := range groups {
			if candidate.Name == name {
				if group.ID != 0 {
					return group, rentalConflict("发现重复的租赁分组，请管理员核查")
				}
				group = candidate
			}
		}
		if group.ID == 0 {
			payload := map[string]any{
				"name":                         name,
				"description":                  description,
				"platform":                     "openai",
				"subscription_type":            "subscription",
				"is_exclusive":                 true,
				"rate_multiplier":              1,
				"allow_image_generation":       true,
				"allow_batch_image_generation": false,
			}
			if err := s.rentalCoreRequest(ctx, "POST", "/api/v1/admin/groups", payload, &group, ""); err != nil {
				return group, err
			}
		}
	}
	if group.ID <= 0 || group.Name != name || group.Description != description || group.Platform != "openai" || group.Status != "active" || !group.IsExclusive || group.SubscriptionType != "subscription" || !group.AllowImageGeneration || group.AllowBatchImageGeneration || group.FallbackGroupID != nil || group.FallbackInvalidGroupID != nil {
		return group, rentalConflict("租赁分组配置与专属账号权限不一致，请管理员核查 Core 分组")
	}
	return group, nil
}

func (s *Server) verifyRentalGroupAccounts(ctx context.Context, o rentalOrder, g rentalGroup) error {
	rows, err := s.portalDB.QueryContext(ctx, `SELECT r.core_account_id FROM portal_account_rental_reservations r JOIN portal_account_orders o ON o.id=r.order_id WHERE o.user_id=$1 AND o.product_id=$2`, o.UserID, o.ProductID)
	if err != nil {
		return err
	}
	defer rows.Close()
	allowed := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		allowed[id] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	seen := map[int64]bool{}
	for page := 1; ; page++ {
		var result struct {
			Items []rentalAccount `json:"items"`
			Pages int             `json:"pages"`
		}
		// Core's admin account list uses `group` for the group filter.
		// `group_id` is ignored by Core and would return every account,
		// causing the isolation check below to report a false conflict.
		path := fmt.Sprintf("/api/v1/admin/accounts?group=%d&page=%d&page_size=100&lite=true", g.ID, page)
		if err := s.rentalCoreRequest(ctx, "GET", path, nil, &result, ""); err != nil {
			return err
		}
		for _, a := range result.Items {
			if !allowed[a.ID] {
				return rentalConflict("专属分组中存在不属于该用户的账号，请先在 Core 核查")
			}
			seen[a.ID] = true
		}
		if page >= result.Pages {
			break
		}
	}
	for _, id := range o.AccountIDs {
		if !seen[id] {
			return errors.New("Core group account membership verification failed")
		}
	}
	return nil
}

func (s *Server) ensureRentalSubscription(ctx context.Context, o rentalOrder, g rentalGroup) error {
	var page struct {
		Items []struct {
			ID        int64     `json:"id"`
			UserID    int64     `json:"user_id"`
			GroupID   int64     `json:"group_id"`
			Status    string    `json:"status"`
			ExpiresAt time.Time `json:"expires_at"`
		} `json:"items"`
	}
	path := fmt.Sprintf("/api/v1/admin/subscriptions?user_id=%d&group_id=%d&page=1&page_size=100", o.UserID, g.ID)
	if err := s.rentalCoreRequest(ctx, "GET", path, nil, &page, ""); err != nil {
		return err
	}
	for _, sub := range page.Items {
		if sub.UserID != o.UserID || sub.GroupID != g.ID {
			return errors.New("Core returned an unexpected rental subscription")
		}
		if sub.Status == "suspended" {
			return rentalConflict("该租赁分组授权已被暂停，请先在 Core 核查")
		}
		if sub.Status == "active" && sub.ExpiresAt.After(time.Now()) {
			if !sub.ExpiresAt.Before(o.ExpiresAt.Time) {
				return nil
			}
			days := int(math.Ceil(o.ExpiresAt.Time.Sub(sub.ExpiresAt).Hours() / 24))
			// Core only accepts whole days; account expiry still enforces the
			// product's exact boundary even if authorization rounds up a partial day.
			return s.rentalCoreRequest(ctx, "POST", fmt.Sprintf("/api/v1/admin/subscriptions/%d/extend", sub.ID), map[string]int{"days": days}, nil, fmt.Sprintf("portal-rental-extend-%d", o.ID))
		}
	}
	return s.rentalCoreRequest(ctx, "POST", "/api/v1/admin/subscriptions/assign", map[string]any{"user_id": o.UserID, "group_id": g.ID, "validity_days": o.ValidityDays, "notes": g.Description}, nil, "")
}
