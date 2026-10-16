package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"strings"
)

// Reuse package usage records and statistics with lease keys as the scope.
// Account IDs alone would include requests from other holders of the channel.
type customerUsageScopeKey struct{}
type customerUsageScope struct {
	IDs    []int64
	UserID int64
}

func customerUsageQuery(r *http.Request, id int64, pageSize int) string {
	scope, ok := r.Context().Value(customerUsageScopeKey{}).(customerUsageScope)
	if !ok {
		return accountMarketUsageQuery(r.URL.Query(), id, pageSize)
	}
	query := cloneQuery(r.URL.Query())
	query.Del("account_id")
	query.Set("user_id", strconv.FormatInt(scope.UserID, 10))
	query.Set("api_key_id", strconv.FormatInt(id, 10))
	query.Set("page", "1")
	query.Set("page_size", strconv.Itoa(pageSize))
	query.Set("exact_total", "true")
	return query.Encode()
}

func (s *Server) exclusiveUsageRecords(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := s.exclusiveUser(w, r)
		if !ok {
			return
		}
		rows, err := s.portalDB.QueryContext(r.Context(), `SELECT DISTINCT core_key_id FROM portal_exclusive_leases WHERE user_id=$1 AND core_key_id IS NOT NULL ORDER BY core_key_id`, user.ID)
		if err != nil {
			s.exclusiveError(w, err)
			return
		}
		ids := []int64{}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				s.exclusiveError(w, err)
				return
			}
			if id > 0 {
				ids = append(ids, id)
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			s.exclusiveError(w, err)
			return
		}
		if raw := strings.TrimSpace(r.URL.Query().Get("api_key_id")); raw != "" {
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || id <= 0 {
				writeAPIError(w, http.StatusBadRequest, "INVALID_API_KEY_ID", "Key ID 无效")
				return
			}
			found := false
			for _, allowed := range ids {
				if allowed == id {
					found = true
					break
				}
			}
			if !found {
				writeAPIError(w, http.StatusNotFound, "KEY_NOT_FOUND", "独享通道 Key 不存在")
				return
			}
			ids = []int64{id}
		}
		w.Header().Set("Cache-Control", "no-store")
		scope := customerUsageScope{IDs: ids, UserID: user.ID}
		next(w, r.WithContext(context.WithValue(r.Context(), customerUsageScopeKey{}, scope)))
	}
}
