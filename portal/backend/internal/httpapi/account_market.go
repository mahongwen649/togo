package httpapi

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const maxAccountProductImageBytes = 600 * 1024

func normalizeAccountProductValidity(mode string, validityDays int) (int, error) {
	switch mode {
	case "rental":
		if validityDays < 1 || validityDays > 3650 {
			return 0, errors.New("租号有效期必须在 1 到 3650 天之间")
		}
		return validityDays, nil
	case "purchase":
		return 0, nil
	default:
		return 0, errors.New("业务类型必须是租号或账号")
	}
}

func normalizeAccountProductImage(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "data:image/") {
		parts := strings.SplitN(value, ",", 2)
		if len(parts) != 2 || !strings.HasSuffix(strings.ToLower(parts[0]), ";base64") {
			return "", errors.New("广告图格式无效")
		}
		mime := strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(parts[0], "data:"), ";base64"))
		if mime != "image/jpeg" && mime != "image/png" && mime != "image/webp" && mime != "image/gif" {
			return "", errors.New("广告图仅支持 JPG、PNG、WebP 或 GIF")
		}
		decoded, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil || len(decoded) == 0 || len(decoded) > maxAccountProductImageBytes || !matchesAccountProductImage(mime, decoded) {
			return "", errors.New("广告图内容无效或超过 600KB")
		}
		return "data:" + mime + ";base64," + parts[1], nil
	}
	// Existing products may still contain a legacy remote URL. The admin UI
	// only creates uploaded data URLs, but preserving a legacy URL avoids
	// forcing an unrelated image replacement during other product edits.
	if len(value) > 2048 {
		return "", errors.New("广告图地址过长")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("广告图格式无效，请重新上传")
	}
	return value, nil
}

func matchesAccountProductImage(mime string, data []byte) bool {
	switch mime {
	case "image/jpeg":
		return len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff
	case "image/png":
		return len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n"
	case "image/gif":
		return len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a")
	case "image/webp":
		return len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP"
	default:
		return false
	}
}

func (s *Server) accountMarketAdminProducts(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireActiveAdmin(w, r); !ok || !s.requireMarketDB(w) {
		return
	}
	rows, err := s.portalDB.QueryContext(r.Context(), `SELECT id, name, mode, validity_days, unit_price, stock_quantity, locked_quantity, image_url, description, purchase_notice, enabled, sort_order FROM portal_account_products ORDER BY sort_order ASC, id ASC`)
	if err != nil {
		writeAPIError(w, 500, "ACCOUNT_MARKET_READ_FAILED", "Failed to load account products")
		return
	}
	defer rows.Close()
	type item struct {
		ID             int64   `json:"id"`
		Name           string  `json:"name"`
		Mode           string  `json:"mode"`
		ValidityDays   int     `json:"validity_days"`
		UnitPrice      float64 `json:"unit_price"`
		StockQuantity  int     `json:"stock_quantity"`
		LockedQuantity int     `json:"locked_quantity"`
		ImageURL       string  `json:"image_url"`
		Description    string  `json:"description"`
		PurchaseNotice string  `json:"purchase_notice"`
		Enabled        bool    `json:"enabled"`
		SortOrder      int     `json:"sort_order"`
	}
	items := make([]item, 0)
	for rows.Next() {
		var v item
		if err := rows.Scan(&v.ID, &v.Name, &v.Mode, &v.ValidityDays, &v.UnitPrice, &v.StockQuantity, &v.LockedQuantity, &v.ImageURL, &v.Description, &v.PurchaseNotice, &v.Enabled, &v.SortOrder); err != nil {
			writeAPIError(w, 500, "ACCOUNT_MARKET_READ_FAILED", "Failed to decode account products")
			return
		}
		items = append(items, v)
	}
	writeSuccess(w, map[string]any{"items": items})
}

func (s *Server) accountMarketAdminSaveProduct(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireActiveAdmin(w, r); !ok || !s.requireMarketDB(w) {
		return
	}
	var in struct {
		ID             int64   `json:"id"`
		Name           string  `json:"name"`
		Mode           string  `json:"mode"`
		ValidityDays   int     `json:"validity_days"`
		UnitPrice      float64 `json:"unit_price"`
		StockQuantity  int     `json:"stock_quantity"`
		ImageURL       string  `json:"image_url"`
		Description    string  `json:"description"`
		PurchaseNotice string  `json:"purchase_notice"`
		Enabled        bool    `json:"enabled"`
		SortOrder      int     `json:"sort_order"`
	}
	if decodeJSON(r, &in) != nil || strings.TrimSpace(in.Name) == "" || in.UnitPrice < 0 || in.StockQuantity < 0 {
		writeAPIError(w, 400, "INVALID_REQUEST", "Invalid product configuration")
		return
	}
	validityDays, err := normalizeAccountProductValidity(in.Mode, in.ValidityDays)
	if err != nil {
		writeAPIError(w, 400, "INVALID_VALIDITY", err.Error())
		return
	}
	imageURL, err := normalizeAccountProductImage(in.ImageURL)
	if err != nil {
		writeAPIError(w, 400, "INVALID_PRODUCT_IMAGE", err.Error())
		return
	}
	in.ValidityDays = validityDays
	in.ImageURL = imageURL
	if in.ID > 0 {
		var locked int
		var currentMode string
		if err := s.portalDB.QueryRowContext(r.Context(), `SELECT locked_quantity, mode FROM portal_account_products WHERE id = $1`, in.ID).Scan(&locked, &currentMode); err != nil {
			writeAPIError(w, 404, "PRODUCT_NOT_FOUND", "Product not found")
			return
		}
		if in.StockQuantity < locked {
			writeAPIError(w, 409, "INVALID_STOCK", "Stock cannot be lower than locked quantity")
			return
		}
		if locked > 0 && currentMode != in.Mode {
			writeAPIError(w, 409, "PRODUCT_MODE_LOCKED", "商品有待处理订单时不能修改业务类型")
			return
		}
		_, err := s.portalDB.ExecContext(r.Context(), `UPDATE portal_account_products SET name=$1, mode=$2, validity_days=$3, unit_price=$4, stock_quantity=$5, image_url=$6, description=$7, purchase_notice=$8, enabled=$9, sort_order=$10, updated_at=CURRENT_TIMESTAMP WHERE id=$11`, in.Name, in.Mode, in.ValidityDays, in.UnitPrice, in.StockQuantity, in.ImageURL, in.Description, in.PurchaseNotice, in.Enabled, in.SortOrder, in.ID)
		if err != nil {
			writeAPIError(w, 500, "PRODUCT_SAVE_FAILED", "Failed to save product")
			return
		}
		writeSuccess(w, map[string]any{"id": in.ID})
		return
	}
	var id int64
	err = s.portalDB.QueryRowContext(r.Context(), `INSERT INTO portal_account_products (name, mode, validity_days, unit_price, stock_quantity, image_url, description, purchase_notice, enabled, sort_order) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`, in.Name, in.Mode, in.ValidityDays, in.UnitPrice, in.StockQuantity, in.ImageURL, in.Description, in.PurchaseNotice, in.Enabled, in.SortOrder).Scan(&id)
	if err != nil {
		writeAPIError(w, 500, "PRODUCT_SAVE_FAILED", "Failed to create product")
		return
	}
	writeSuccess(w, map[string]any{"id": id})
}

func (s *Server) accountMarketAdminOrders(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireActiveAdmin(w, r); !ok || !s.requireMarketDB(w) {
		return
	}
	rows, err := s.portalDB.QueryContext(r.Context(), `SELECT id, user_id, product_name, mode, validity_days, quantity, total_price, status, core_account_ids, expires_at, created_at,core_group_id,fulfillment_started_at,fulfillment_error FROM portal_account_orders ORDER BY created_at DESC, id DESC`)
	if err != nil {
		writeAPIError(w, 500, "ACCOUNT_ORDER_READ_FAILED", "Failed to load account orders")
		return
	}
	defer rows.Close()
	type order struct {
		ID                   int64      `json:"id"`
		UserID               int64      `json:"user_id"`
		ProductName          string     `json:"product_name"`
		Mode                 string     `json:"mode"`
		ValidityDays         int        `json:"validity_days"`
		Quantity             int        `json:"quantity"`
		TotalPrice           float64    `json:"total_price"`
		Status               string     `json:"status"`
		CoreAccountIDs       []int64    `json:"core_account_ids"`
		ExpiresAt            *time.Time `json:"expires_at"`
		CreatedAt            time.Time  `json:"created_at"`
		CoreGroupID          *int64     `json:"core_group_id"`
		FulfillmentStartedAt *time.Time `json:"fulfillment_started_at"`
		FulfillmentError     string     `json:"fulfillment_error"`
	}
	items := make([]order, 0)
	for rows.Next() {
		var v order
		var raw []byte
		if err := rows.Scan(&v.ID, &v.UserID, &v.ProductName, &v.Mode, &v.ValidityDays, &v.Quantity, &v.TotalPrice, &v.Status, &raw, &v.ExpiresAt, &v.CreatedAt, &v.CoreGroupID, &v.FulfillmentStartedAt, &v.FulfillmentError); err != nil {
			writeAPIError(w, 500, "ACCOUNT_ORDER_READ_FAILED", "Failed to decode account orders")
			return
		}
		if err := json.Unmarshal(raw, &v.CoreAccountIDs); err != nil {
			s.writeRentalError(w, err)
			return
		}
		items = append(items, v)
	}
	if err := rows.Err(); err != nil {
		s.writeRentalError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"items": items})
}

func (s *Server) accountMarketAdminUpdateOrder(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireActiveAdmin(w, r); !ok || !s.requireMarketDB(w) {
		return
	}
	id, ok := pathID(w, r, "orderID")
	if !ok {
		return
	}
	var in struct {
		Status         string  `json:"status"`
		CoreAccountIDs []int64 `json:"core_account_ids"`
	}
	if decodeJSON(r, &in) != nil || (in.Status != "completed" && in.Status != "cancelled") {
		writeAPIError(w, 400, "INVALID_STATUS", "Only completed or cancelled is supported")
		return
	}
	if in.Status == "completed" {
		s.fulfillRental(w, r, id, in.CoreAccountIDs)
		return
	}
	tx, err := s.portalDB.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, 500, "ORDER_UPDATE_FAILED", "Failed to start order update")
		return
	}
	defer tx.Rollback()
	var userID, productID int64
	var quantity int
	var total string
	var current string
	var started sql.NullTime
	err = tx.QueryRowContext(r.Context(), `SELECT user_id, product_id, quantity, total_price, status, fulfillment_started_at FROM portal_account_orders WHERE id=$1 FOR UPDATE`, id).Scan(&userID, &productID, &quantity, &total, &current, &started)
	if err == sql.ErrNoRows {
		writeAPIError(w, 404, "ORDER_NOT_FOUND", "Order not found")
		return
	}
	if err != nil {
		writeAPIError(w, 500, "ORDER_UPDATE_FAILED", "Failed to load order")
		return
	}
	if current != "pending" {
		writeAPIError(w, 409, "ORDER_ALREADY_PROCESSED", "Order has already been processed")
		return
	}
	if started.Valid {
		writeAPIError(w, 409, "ORDER_SYNC_IN_PROGRESS", "订单已开始同步，不能直接取消，请先等待或联系管理员")
		return
	}
	if in.Status == "cancelled" {
		// Match checkout's product-then-wallet lock order.
		if _, err = tx.ExecContext(r.Context(), `UPDATE portal_account_products SET locked_quantity=locked_quantity-$1,updated_at=CURRENT_TIMESTAMP WHERE id=$2`, quantity, productID); err != nil {
			writeAPIError(w, 500, "ORDER_UPDATE_FAILED", "Failed to release stock")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO portal_account_wallets(user_id,balance) VALUES($1,$2::numeric) ON CONFLICT(user_id) DO UPDATE SET balance=portal_account_wallets.balance+EXCLUDED.balance,updated_at=CURRENT_TIMESTAMP`, userID, total); err != nil {
			writeAPIError(w, 500, "ORDER_UPDATE_FAILED", "Failed to refund balance")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO portal_account_wallet_ledger(user_id,kind,amount,balance_before,balance_after,reference_type,reference_id,note) SELECT user_id,'refund',$2::numeric,balance-$2::numeric,balance,'account_order',$3,'Account order cancelled' FROM portal_account_wallets WHERE user_id=$1`, userID, total, id); err != nil {
			writeAPIError(w, 500, "ORDER_UPDATE_FAILED", "Failed to record refund")
			return
		}
	}
	_, err = tx.ExecContext(r.Context(), `UPDATE portal_account_orders SET status='cancelled',completed_at=NULL,updated_at=CURRENT_TIMESTAMP WHERE id=$1`, id)
	if err != nil {
		writeAPIError(w, 500, "ORDER_UPDATE_FAILED", "Failed to update order")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, 500, "ORDER_UPDATE_FAILED", "Failed to commit order update")
		return
	}
	writeSuccess(w, map[string]any{"id": id, "status": "cancelled"})
}

func (s *Server) accountMarketAdminDeleteProduct(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireActiveAdmin(w, r); !ok || !s.requireMarketDB(w) {
		return
	}
	id, ok := pathID(w, r, "productID")
	if !ok {
		return
	}
	var locked int
	if err := s.portalDB.QueryRowContext(r.Context(), `SELECT locked_quantity FROM portal_account_products WHERE id=$1`, id).Scan(&locked); err != nil {
		writeAPIError(w, 404, "PRODUCT_NOT_FOUND", "Product not found")
		return
	}
	if locked > 0 {
		writeAPIError(w, 409, "PRODUCT_IN_USE", "Product has pending orders")
		return
	}
	if _, err := s.portalDB.ExecContext(r.Context(), `UPDATE portal_account_products SET enabled=FALSE,updated_at=CURRENT_TIMESTAMP WHERE id=$1`, id); err != nil {
		writeAPIError(w, 500, "PRODUCT_DELETE_FAILED", fmt.Sprintf("Failed to archive product: %v", err))
		return
	}
	writeSuccess(w, map[string]any{"id": id})
}

type accountMarketProduct struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	Mode           string  `json:"mode"`
	ValidityDays   int     `json:"validity_days"`
	UnitPrice      float64 `json:"unit_price"`
	StockRemaining int     `json:"stock_remaining"`
	ImageURL       string  `json:"image_url"`
	Description    string  `json:"description"`
	PurchaseNotice string  `json:"purchase_notice"`
}

func (s *Server) accountMarketRecharge(w http.ResponseWriter, r *http.Request) {
	url := strings.TrimSpace(os.Getenv("ACCOUNT_RECHARGE_URL"))
	writeSuccess(w, map[string]string{"url": url})
}

func (s *Server) accountMarketProducts(w http.ResponseWriter, r *http.Request) {
	if s.portalDB == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "ACCOUNT_MARKET_UNAVAILABLE", "Account market is not configured")
		return
	}
	rows, err := s.portalDB.QueryContext(r.Context(), `
		SELECT id, name, mode, validity_days, unit_price, (stock_quantity - locked_quantity), image_url, description, purchase_notice
		FROM portal_account_products
		WHERE enabled = TRUE AND stock_quantity > locked_quantity
		ORDER BY sort_order ASC, id ASC`)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_MARKET_READ_FAILED", "Failed to load account products")
		return
	}
	defer rows.Close()
	items := make([]accountMarketProduct, 0)
	for rows.Next() {
		var item accountMarketProduct
		if err := rows.Scan(&item.ID, &item.Name, &item.Mode, &item.ValidityDays, &item.UnitPrice, &item.StockRemaining, &item.ImageURL, &item.Description, &item.PurchaseNotice); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_MARKET_READ_FAILED", "Failed to decode account products")
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_MARKET_READ_FAILED", "Failed to load account products")
		return
	}
	writeSuccess(w, map[string]any{"items": items})
}

func (s *Server) accountMarketWallet(w http.ResponseWriter, r *http.Request) {
	accessToken, ok := bearerToken(w, r)
	if !ok || !s.requireMarketDB(w) {
		return
	}
	user, err := s.core.Profile(r.Context(), accessToken)
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return
	}
	var balance float64
	err = s.portalDB.QueryRowContext(r.Context(), `SELECT balance FROM portal_account_wallets WHERE user_id = $1`, user.ID).Scan(&balance)
	if err == sql.ErrNoRows {
		balance = 0
	} else if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_WALLET_READ_FAILED", "Failed to load account balance")
		return
	}
	writeSuccess(w, map[string]any{"user_id": user.ID, "balance": balance})
}

func (s *Server) accountMarketOrders(w http.ResponseWriter, r *http.Request) {
	accessToken, ok := bearerToken(w, r)
	if !ok || !s.requireMarketDB(w) {
		return
	}
	user, err := s.core.Profile(r.Context(), accessToken)
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return
	}
	rows, err := s.portalDB.QueryContext(r.Context(), `
		SELECT id, product_name, mode, validity_days, quantity, unit_price, total_price,
		       CASE WHEN status = 'completed' AND expires_at IS NOT NULL AND expires_at <= CURRENT_TIMESTAMP THEN 'expired' ELSE status END,
		       expires_at, core_group_id, core_account_ids, created_at, auto_renew, last_renewed_at, renewal_error
		FROM portal_account_orders WHERE user_id = $1 ORDER BY created_at DESC, id DESC`, user.ID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_ORDER_READ_FAILED", "Failed to load account orders")
		return
	}
	defer rows.Close()
	type order struct {
		ID             int64      `json:"id"`
		ProductName    string     `json:"product_name"`
		Mode           string     `json:"mode"`
		ValidityDays   int        `json:"validity_days"`
		Quantity       int        `json:"quantity"`
		UnitPrice      float64    `json:"unit_price"`
		TotalPrice     float64    `json:"total_price"`
		Status         string     `json:"status"`
		ExpiresAt      *time.Time `json:"expires_at"`
		CoreGroupID    *int64     `json:"core_group_id"`
		CoreAccountIDs []int64    `json:"core_account_ids"`
		CreatedAt      time.Time  `json:"created_at"`
		AutoRenew      bool       `json:"auto_renew"`
		LastRenewedAt  *time.Time `json:"last_renewed_at"`
		RenewalError   string     `json:"renewal_error"`
	}
	items := make([]order, 0)
	for rows.Next() {
		var item order
		var rawAccountIDs []byte
		if err := rows.Scan(&item.ID, &item.ProductName, &item.Mode, &item.ValidityDays, &item.Quantity, &item.UnitPrice, &item.TotalPrice, &item.Status, &item.ExpiresAt, &item.CoreGroupID, &rawAccountIDs, &item.CreatedAt, &item.AutoRenew, &item.LastRenewedAt, &item.RenewalError); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_ORDER_READ_FAILED", "Failed to decode account orders")
			return
		}
		if len(rawAccountIDs) > 0 {
			if err := json.Unmarshal(rawAccountIDs, &item.CoreAccountIDs); err != nil {
				writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_ORDER_READ_FAILED", "Failed to decode account accounts")
				return
			}
		}
		if item.CoreAccountIDs == nil {
			item.CoreAccountIDs = []int64{}
		}
		items = append(items, item)
	}
	writeSuccess(w, map[string]any{"items": items})
}

func (s *Server) accountMarketSetAutoRenew(w http.ResponseWriter, r *http.Request) {
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
	orderID, ok := pathID(w, r, "orderID")
	if !ok {
		return
	}
	var input struct {
		Enabled *bool `json:"enabled"`
	}
	if err := decodeJSON(r, &input); err != nil || input.Enabled == nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_AUTO_RENEW", "请提供自动续订开关状态")
		return
	}
	var expiresAt *time.Time
	err = s.portalDB.QueryRowContext(r.Context(), `
		UPDATE portal_account_orders
		SET auto_renew=$1, renewal_error='', updated_at=CURRENT_TIMESTAMP
		WHERE id=$2 AND user_id=$3 AND mode='rental' AND validity_days=30 AND status IN ('completed','expired')
		RETURNING expires_at`, *input.Enabled, orderID, user.ID).Scan(&expiresAt)
	if err == sql.ErrNoRows {
		writeAPIError(w, http.StatusNotFound, "ORDER_NOT_FOUND", "套餐订单不存在或不支持自动续订")
		return
	}
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "AUTO_RENEW_UPDATE_FAILED", "自动续订设置保存失败")
		return
	}
	writeSuccess(w, map[string]any{"id": orderID, "auto_renew": *input.Enabled, "expires_at": expiresAt})
}

func (s *Server) accountMarketCreateOrder(w http.ResponseWriter, r *http.Request) {
	accessToken, ok := bearerToken(w, r)
	if !ok || !s.requireMarketDB(w) {
		return
	}
	user, err := s.core.Profile(r.Context(), accessToken)
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return
	}
	var input struct {
		ProductID int64 `json:"product_id"`
		Quantity  int   `json:"quantity"`
	}
	if err := decodeJSON(r, &input); err != nil || input.ProductID <= 0 || input.Quantity <= 0 {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Product and quantity are required")
		return
	}
	tx, err := s.portalDB.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_ORDER_FAILED", "Failed to start order")
		return
	}
	defer tx.Rollback()
	var name, mode string
	var validityDays int
	var unitPrice, total string
	var available int
	err = tx.QueryRowContext(r.Context(), `SELECT name, mode, validity_days, unit_price::text, (unit_price*$2)::text, stock_quantity - locked_quantity FROM portal_account_products WHERE id = $1 AND enabled = TRUE FOR UPDATE`, input.ProductID, input.Quantity).Scan(&name, &mode, &validityDays, &unitPrice, &total, &available)
	if err == sql.ErrNoRows {
		writeAPIError(w, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Product is not available")
		return
	}
	if err != nil {
		writeAPIError(w, 500, "ACCOUNT_ORDER_FAILED", "Failed to load product")
		return
	}
	if input.Quantity > available {
		writeAPIError(w, http.StatusConflict, "INSUFFICIENT_STOCK", "Not enough stock")
		return
	}
	amount, parseErr := strconv.ParseFloat(total, 64)
	if parseErr != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_ORDER_FAILED", "Failed to calculate order amount")
		return
	}
	balanceAfter, debitErr := debitAccountWalletTx(r.Context(), tx, user.ID, amount)
	if errors.Is(debitErr, sql.ErrNoRows) {
		writeAPIError(w, http.StatusPaymentRequired, "INSUFFICIENT_ACCOUNT_BALANCE", "Please recharge your account balance first")
		return
	}
	if debitErr != nil {
		s.logger.Error("account wallet debit failed", "user_id", user.ID, "product_id", input.ProductID, "quantity", input.Quantity, "amount", total, "error", debitErr)
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_ORDER_FAILED", "Failed to debit account balance")
		return
	}
	var orderID int64
	if err = tx.QueryRowContext(r.Context(), `INSERT INTO portal_account_orders (user_id, product_id, product_name, mode, validity_days, quantity, unit_price, total_price) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, user.ID, input.ProductID, name, mode, validityDays, input.Quantity, unitPrice, total).Scan(&orderID); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_ORDER_FAILED", "Failed to create account order")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE portal_account_products SET locked_quantity = locked_quantity + $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, input.Quantity, input.ProductID); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_ORDER_FAILED", "Failed to lock product stock")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO portal_account_wallet_ledger (user_id, kind, amount, balance_before, balance_after, reference_type, reference_id, note) SELECT user_id,'purchase',-$2::numeric,balance+$2::numeric,balance,'account_order',$3,'Account product purchase' FROM portal_account_wallets WHERE user_id=$1`, user.ID, total, orderID); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_ORDER_FAILED", "Failed to record account balance")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ACCOUNT_ORDER_FAILED", "Failed to complete account order")
		return
	}
	writeSuccess(w, map[string]any{"id": orderID, "status": "pending", "balance": balanceAfter})
}

func (s *Server) accountMarketPublicConfig(w http.ResponseWriter, r *http.Request) {
	writeSuccess(w, map[string]string{"recharge_url": strings.TrimSpace(os.Getenv("ACCOUNT_RECHARGE_URL"))})
}
