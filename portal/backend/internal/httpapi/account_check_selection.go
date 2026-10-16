package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Always use the current paginated Core catalog as the source of account identity.
// A failed page is an error, never evidence that an account was deleted.
func currentCheckAccounts(ctx context.Context, core accountCheckCore, token string) (map[int64]checkAccount, error) {
	accounts := map[int64]checkAccount{}
	for page := 1; ; page++ {
		var result struct {
			Items []checkAccount `json:"items"`
			Total int            `json:"total"`
			Pages int            `json:"pages"`
		}
		path := fmt.Sprintf("/api/v1/admin/accounts?platform=openai&page_size=50&page=%d", page)
		if err := accountCheckRequest(ctx, core, http.MethodGet, path, nil, &result, token); err != nil {
			return nil, err
		}
		for _, account := range result.Items {
			if account.Platform == "openai" {
				accounts[account.ID] = account
			}
		}
		pages := result.Pages
		if pages < 1 {
			pages = (result.Total + 49) / 50
		}
		if page >= pages {
			break
		}
		if page >= 10000 || len(result.Items) == 0 {
			return nil, fmt.Errorf("incomplete Core account catalog")
		}
	}
	return accounts, nil
}

func (s *Server) accountCheckSelection(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	admin, ok := s.requireActiveAdmin(w, r)
	if !ok {
		return
	}
	if s.portalDB == nil {
		writeAPIError(w, 503, "HISTORY_UNAVAILABLE", "检查记录存储不可用")
		return
	}
	if r.Method == http.MethodGet {
		var data []byte
		err := s.portalDB.QueryRowContext(r.Context(), `SELECT account_ids FROM portal_admin_account_check_selection WHERE admin_id=$1`, admin.ID).Scan(&data)
		if errors.Is(err, sql.ErrNoRows) {
			writeSuccess(w, map[string]any{"account_ids": nil})
			return
		}
		if err != nil {
			s.writeAccountHistoryError(w, err)
			return
		}
		var ids []int64
		if err = json.Unmarshal(data, &ids); err != nil {
			s.writeAccountHistoryError(w, err)
			return
		}
		writeSuccess(w, map[string]any{"account_ids": ids})
		return
	}
	var input struct {
		AccountIDs []int64 `json:"account_ids"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 128*1024)
	if decodeJSON(r, &input) != nil || input.AccountIDs == nil || len(input.AccountIDs) > 10000 {
		writeAPIError(w, 400, "INVALID_SELECTION", "无效的账号选择")
		return
	}
	ids := []int64{}
	seen := map[int64]bool{}
	for _, id := range input.AccountIDs {
		if id <= 0 {
			writeAPIError(w, 400, "INVALID_SELECTION", "无效的账号 ID")
			return
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	data, _ := json.Marshal(ids)
	if _, err := s.portalDB.ExecContext(r.Context(), `INSERT INTO portal_admin_account_check_selection(admin_id,account_ids) VALUES($1,$2)
        ON CONFLICT(admin_id) DO UPDATE SET account_ids=EXCLUDED.account_ids,updated_at=now()`, admin.ID, data); err != nil {
		s.writeAccountHistoryError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"account_ids": ids})
}
