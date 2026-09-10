package httpapi

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type rechargeOrderRow struct {
	ID             int64   `json:"id"`
	UserID         int64   `json:"user_id"`
	UserName       string  `json:"user_name"`
	UserEmail      string  `json:"user_email"`
	CurrentBalance float64 `json:"current_balance"`
	TotalRecharged float64 `json:"total_recharged"`
	Amount         float64 `json:"amount"`
	PayAmount      float64 `json:"pay_amount"`
	PaymentType    string  `json:"payment_type"`
	Status         string  `json:"status"`
	OrderType      string  `json:"order_type"`
	CreatedAt      string  `json:"created_at"`
	PaidAt         *string `json:"paid_at"`
	CompletedAt    *string `json:"completed_at"`
	EffectiveTime  string  `json:"effective_time"`
	TradeNo        string  `json:"payment_trade_no"`
	OutTradeNo     string  `json:"out_trade_no,omitempty"`
}

type rechargeSummary struct {
	TotalAmount           float64           `json:"total_amount"`
	TotalPayAmount        float64           `json:"total_pay_amount"`
	AllTimeTotalRecharged float64           `json:"all_time_total_recharged"`
	RechargeUsersBalance  float64           `json:"recharge_users_balance"`
	OrderCount            int64             `json:"order_count"`
	UserCount             int64             `json:"user_count"`
	AveragePayAmount      float64           `json:"average_pay_amount"`
	MaxPayAmount          float64           `json:"max_pay_amount"`
	Latest                *rechargeOrderRow `json:"latest"`
	StartTime             string            `json:"start_time"`
	EndTime               string            `json:"end_time"`
	Granularity           string            `json:"granularity"`
}

type rechargeUserStat struct {
	UserID           int64   `json:"user_id"`
	UserName         string  `json:"user_name"`
	UserEmail        string  `json:"user_email"`
	CurrentBalance   float64 `json:"current_balance"`
	TotalAmount      float64 `json:"total_amount"`
	TotalPayAmount   float64 `json:"total_pay_amount"`
	OrderCount       int64   `json:"order_count"`
	AveragePayAmount float64 `json:"average_pay_amount"`
	MaxPayAmount     float64 `json:"max_pay_amount"`
	LatestTime       string  `json:"latest_time"`
}

type rechargeTimeStat struct {
	Bucket         string  `json:"bucket"`
	TotalAmount    float64 `json:"total_amount"`
	TotalPayAmount float64 `json:"total_pay_amount"`
	OrderCount     int64   `json:"order_count"`
	UserCount      int64   `json:"user_count"`
}

type rechargeQuery struct {
	Where       string
	Args        []any
	Start       time.Time
	End         time.Time
	Location    *time.Location
	Granularity string
	Page        int
	PageSize    int
}

const rechargeEffectiveExpr = "COALESCE(paid_at, completed_at, created_at)"

func (s *Server) adminRecharges(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireActiveAdmin(w, r); !ok {
		return
	}
	if s.corePaymentDB == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "CORE_DATABASE_UNAVAILABLE", "Core payment database is not configured")
		return
	}
	query, err := parseRechargeQuery(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	type result struct {
		items           []rechargeOrderRow
		total           int64
		summary         rechargeSummary
		users           []rechargeUserStat
		usersTotal      int64
		timeseries      []rechargeTimeStat
		timeseriesTotal int64
		err             error
	}
	payload := result{}
	payload.items, payload.total, payload.err = s.rechargeItems(r.Context(), query)
	if payload.err == nil {
		payload.summary, payload.err = s.rechargeSummary(r.Context(), query)
	}
	if payload.err == nil {
		payload.users, payload.usersTotal, payload.err = s.rechargeUsers(r.Context(), query)
	}
	if payload.err == nil {
		payload.timeseries, payload.timeseriesTotal, payload.err = s.rechargeTimeseries(r.Context(), query)
	}
	if payload.err != nil {
		s.logger.Warn("Admin recharge query failed", "error", payload.err)
		writeAPIError(w, http.StatusBadGateway, "RECHARGE_QUERY_FAILED", "Recharge records could not be loaded")
		return
	}

	writeSuccess(w, map[string]any{
		"items":            payload.items,
		"total":            payload.total,
		"page":             query.Page,
		"page_size":        query.PageSize,
		"pages":            rechargePageCount(payload.total, query.PageSize),
		"summary":          payload.summary,
		"users":            payload.users,
		"users_total":      payload.usersTotal,
		"users_pages":      rechargePageCount(payload.usersTotal, query.PageSize),
		"timeseries":       payload.timeseries,
		"timeseries_total": payload.timeseriesTotal,
		"timeseries_pages": rechargePageCount(payload.timeseriesTotal, query.PageSize),
	})
}

func rechargePageCount(total int64, pageSize int) int64 {
	if total == 0 {
		return 0
	}
	return int64(math.Ceil(float64(total) / float64(pageSize)))
}

func parseRechargeQuery(r *http.Request) (rechargeQuery, error) {
	values := r.URL.Query()
	location := rechargeLocation(values.Get("timezone"))
	now := time.Now().In(location)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	end := start.Add(24 * time.Hour)

	if values.Get("start_date") != "" || values.Get("end_date") != "" {
		var err error
		start, err = parseRechargeDate(values.Get("start_date"), start, location)
		if err != nil {
			return rechargeQuery{}, fmt.Errorf("invalid start date")
		}
		endDate, err := parseRechargeDate(values.Get("end_date"), start, location)
		if err != nil {
			return rechargeQuery{}, fmt.Errorf("invalid end date")
		}
		end = endDate.Add(24 * time.Hour)
	}
	if near := strings.TrimSpace(values.Get("near_time")); near != "" {
		center, err := parseRechargeTime(near, location)
		if err != nil {
			return rechargeQuery{}, fmt.Errorf("invalid near time")
		}
		minutes, _ := strconv.Atoi(values.Get("near_minutes"))
		if minutes <= 0 {
			minutes = 5
		}
		if minutes > 1440 {
			minutes = 1440
		}
		delta := time.Duration(minutes) * time.Minute
		start = center.Add(-delta)
		end = center.Add(delta)
	}
	if !end.After(start) {
		return rechargeQuery{}, fmt.Errorf("end time must be after start time")
	}

	page, pageSize := parsePageQuery(values)
	args := []any{start, end}
	conditions := []string{rechargeEffectiveExpr + " >= $1", rechargeEffectiveExpr + " < $2"}
	nextArg := 3
	status := strings.TrimSpace(values.Get("status"))
	if status == "" {
		status = "COMPLETED"
	}
	if !strings.EqualFold(status, "all") {
		conditions = append(conditions, fmt.Sprintf("status = $%d", nextArg))
		args = append(args, status)
		nextArg++
	}
	if keyword := strings.TrimSpace(values.Get("keyword")); keyword != "" {
		if id, err := strconv.ParseInt(keyword, 10, 64); err == nil && id > 0 {
			conditions = append(conditions, fmt.Sprintf("(user_id = $%d OR user_name ILIKE $%d OR user_email ILIKE $%d)", nextArg, nextArg+1, nextArg+1))
			args = append(args, id, "%"+keyword+"%")
			nextArg += 2
		} else {
			conditions = append(conditions, fmt.Sprintf("(user_name ILIKE $%d OR user_email ILIKE $%d)", nextArg, nextArg))
			args = append(args, "%"+keyword+"%")
			nextArg++
		}
	}
	if paymentType := strings.TrimSpace(values.Get("payment_type")); paymentType != "" && !strings.EqualFold(paymentType, "all") {
		conditions = append(conditions, fmt.Sprintf("payment_type = $%d", nextArg))
		args = append(args, paymentType)
		nextArg++
	}
	if orderType := strings.TrimSpace(values.Get("order_type")); orderType != "" && !strings.EqualFold(orderType, "all") {
		conditions = append(conditions, fmt.Sprintf("order_type = $%d", nextArg))
		args = append(args, orderType)
	}

	granularity := strings.TrimSpace(values.Get("granularity"))
	if granularity == "" {
		if end.Sub(start) <= 48*time.Hour {
			granularity = "hour"
		} else if end.Sub(start) <= 92*24*time.Hour {
			granularity = "day"
		} else {
			granularity = "month"
		}
	}
	if granularity != "hour" && granularity != "day" && granularity != "month" {
		granularity = "day"
	}

	return rechargeQuery{
		Where: strings.Join(conditions, " AND "), Args: args, Start: start, End: end,
		Location: location, Granularity: granularity, Page: page, PageSize: pageSize,
	}, nil
}

func (s *Server) rechargeItems(ctx context.Context, query rechargeQuery) ([]rechargeOrderRow, int64, error) {
	var total int64
	if err := s.corePaymentDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM payment_orders WHERE "+query.Where, query.Args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args := append([]any{}, query.Args...)
	args = append(args, query.PageSize, (query.Page-1)*query.PageSize)
	sqlText := fmt.Sprintf(`
SELECT id, user_id, user_name, user_email,
       COALESCE((SELECT u.balance FROM users u WHERE u.id = payment_orders.user_id), 0)::float8 AS current_balance,
       COALESCE((
           SELECT SUM(all_orders.pay_amount)
           FROM payment_orders all_orders
           WHERE all_orders.user_id = payment_orders.user_id
             AND all_orders.status = 'COMPLETED'
             AND all_orders.order_type = 'balance'
       ), 0)::float8 AS total_recharged,
       amount::float8, pay_amount::float8, payment_type, status, order_type,
       to_char(created_at AT TIME ZONE $%d, 'YYYY-MM-DD HH24:MI:SS'),
       to_char(paid_at AT TIME ZONE $%d, 'YYYY-MM-DD HH24:MI:SS'),
       to_char(completed_at AT TIME ZONE $%d, 'YYYY-MM-DD HH24:MI:SS'),
       to_char(%s AT TIME ZONE $%d, 'YYYY-MM-DD HH24:MI:SS'),
       payment_trade_no,
       COALESCE(out_trade_no, '')
FROM payment_orders
WHERE %s
ORDER BY %s DESC
LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+1, len(args)+1, rechargeEffectiveExpr, len(args)+1, query.Where, rechargeEffectiveExpr, len(args)-1, len(args))
	args = append(args, query.Location.String())
	rows, err := s.corePaymentDB.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items, err := scanRechargeRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *Server) rechargeSummary(ctx context.Context, query rechargeQuery) (rechargeSummary, error) {
	sqlText := `
SELECT COALESCE(SUM(amount), 0)::float8, COALESCE(SUM(pay_amount), 0)::float8,
       COUNT(*)::bigint, COUNT(DISTINCT user_id)::bigint,
       COALESCE(AVG(pay_amount), 0)::float8, COALESCE(MAX(pay_amount), 0)::float8
FROM payment_orders WHERE ` + query.Where
	var summary rechargeSummary
	if err := s.corePaymentDB.QueryRowContext(ctx, sqlText, query.Args...).Scan(
		&summary.TotalAmount, &summary.TotalPayAmount, &summary.OrderCount, &summary.UserCount,
		&summary.AveragePayAmount, &summary.MaxPayAmount,
	); err != nil {
		return rechargeSummary{}, err
	}
	if err := s.corePaymentDB.QueryRowContext(ctx, `
SELECT COALESCE((
           SELECT SUM(pay_amount)
           FROM payment_orders
           WHERE status = 'COMPLETED' AND order_type = 'balance'
       ), 0)::float8,
       COALESCE((
           SELECT SUM(u.balance)
           FROM users u
           WHERE EXISTS (
               SELECT 1
               FROM payment_orders completed_orders
               WHERE completed_orders.user_id = u.id
                 AND completed_orders.status = 'COMPLETED'
                 AND completed_orders.order_type = 'balance'
           )
       ), 0)::float8`).Scan(&summary.AllTimeTotalRecharged, &summary.RechargeUsersBalance); err != nil {
		return rechargeSummary{}, err
	}
	latest, _, err := s.rechargeItems(ctx, rechargeQuery{Where: query.Where, Args: query.Args, Page: 1, PageSize: 1, Location: query.Location})
	if err != nil {
		return rechargeSummary{}, err
	}
	if len(latest) > 0 {
		summary.Latest = &latest[0]
	}
	summary.StartTime = query.Start.In(query.Location).Format("2006-01-02 15:04:05")
	summary.EndTime = query.End.In(query.Location).Format("2006-01-02 15:04:05")
	summary.Granularity = query.Granularity
	return summary, nil
}

func (s *Server) rechargeUsers(ctx context.Context, query rechargeQuery) ([]rechargeUserStat, int64, error) {
	var total int64
	countSQL := `SELECT COUNT(DISTINCT user_id) FROM payment_orders WHERE ` + query.Where
	if err := s.corePaymentDB.QueryRowContext(ctx, countSQL, query.Args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args := append([]any{}, query.Args...)
	timezoneArg := len(args) + 1
	args = append(args, query.Location.String(), query.PageSize, (query.Page-1)*query.PageSize)
	sqlText := fmt.Sprintf(`
SELECT user_id,
       COALESCE(NULLIF((array_agg(user_name ORDER BY %s DESC))[1], ''), '') AS user_name,
       COALESCE(NULLIF((array_agg(user_email ORDER BY %s DESC))[1], ''), '') AS user_email,
       COALESCE((SELECT u.balance FROM users u WHERE u.id = payment_orders.user_id), 0)::float8 AS current_balance,
       COALESCE(SUM(amount), 0)::float8, COALESCE(SUM(pay_amount), 0)::float8,
       COUNT(*)::bigint, COALESCE(AVG(pay_amount), 0)::float8, COALESCE(MAX(pay_amount), 0)::float8,
       to_char(MAX(%s) AT TIME ZONE $%d, 'YYYY-MM-DD HH24:MI:SS')
FROM payment_orders
WHERE %s
GROUP BY user_id
ORDER BY SUM(pay_amount) DESC, COUNT(*) DESC, MAX(%s) DESC
LIMIT $%d OFFSET $%d`, rechargeEffectiveExpr, rechargeEffectiveExpr, rechargeEffectiveExpr, timezoneArg, query.Where, rechargeEffectiveExpr, timezoneArg+1, timezoneArg+2)
	rows, err := s.corePaymentDB.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var result []rechargeUserStat
	for rows.Next() {
		var item rechargeUserStat
		if err := rows.Scan(&item.UserID, &item.UserName, &item.UserEmail, &item.CurrentBalance, &item.TotalAmount, &item.TotalPayAmount, &item.OrderCount, &item.AveragePayAmount, &item.MaxPayAmount, &item.LatestTime); err != nil {
			return nil, 0, err
		}
		result = append(result, item)
	}
	return result, total, rows.Err()
}

func (s *Server) rechargeTimeseries(ctx context.Context, query rechargeQuery) ([]rechargeTimeStat, int64, error) {
	args := append([]any{}, query.Args...)
	args = append(args, query.Granularity, query.Location.String())
	granularityArg := len(args) - 1
	timezoneArg := len(args)
	countSQL := fmt.Sprintf(`
SELECT COUNT(*) FROM (
    SELECT date_trunc($%d, %s AT TIME ZONE $%d) AS bucket
    FROM payment_orders
    WHERE %s
    GROUP BY bucket
) buckets`, granularityArg, rechargeEffectiveExpr, timezoneArg, query.Where)
	var total int64
	if err := s.corePaymentDB.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, query.PageSize, (query.Page-1)*query.PageSize)
	sqlText := fmt.Sprintf(`
SELECT to_char(date_trunc($%d, %s AT TIME ZONE $%d), 'YYYY-MM-DD HH24:MI:SS') AS bucket,
       COALESCE(SUM(amount), 0)::float8, COALESCE(SUM(pay_amount), 0)::float8,
       COUNT(*)::bigint, COUNT(DISTINCT user_id)::bigint
FROM payment_orders
WHERE %s
GROUP BY bucket
ORDER BY bucket ASC
LIMIT $%d OFFSET $%d`, granularityArg, rechargeEffectiveExpr, timezoneArg, query.Where, timezoneArg+1, timezoneArg+2)
	rows, err := s.corePaymentDB.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var result []rechargeTimeStat
	for rows.Next() {
		var item rechargeTimeStat
		if err := rows.Scan(&item.Bucket, &item.TotalAmount, &item.TotalPayAmount, &item.OrderCount, &item.UserCount); err != nil {
			return nil, 0, err
		}
		result = append(result, item)
	}
	return result, total, rows.Err()
}

func scanRechargeRows(rows *sql.Rows) ([]rechargeOrderRow, error) {
	var items []rechargeOrderRow
	for rows.Next() {
		var item rechargeOrderRow
		var paidAt, completedAt sql.NullString
		if err := rows.Scan(
			&item.ID, &item.UserID, &item.UserName, &item.UserEmail, &item.CurrentBalance, &item.TotalRecharged, &item.Amount, &item.PayAmount,
			&item.PaymentType, &item.Status, &item.OrderType, &item.CreatedAt, &paidAt, &completedAt,
			&item.EffectiveTime, &item.TradeNo, &item.OutTradeNo,
		); err != nil {
			return nil, err
		}
		if paidAt.Valid {
			item.PaidAt = &paidAt.String
		}
		if completedAt.Valid {
			item.CompletedAt = &completedAt.String
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func rechargeLocation(value string) *time.Location {
	if value == "" {
		value = "Asia/Shanghai"
	}
	loc, err := time.LoadLocation(value)
	if err != nil {
		return time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return loc
}

func parseRechargeDate(raw string, fallback time.Time, loc *time.Location) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", raw, loc)
	if err != nil {
		return time.Time{}, err
	}
	return parsed, nil
}

func parseRechargeTime(raw string, loc *time.Location) (time.Time, error) {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "T", " "))
	layouts := []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"}
	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, raw, loc); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time")
}
