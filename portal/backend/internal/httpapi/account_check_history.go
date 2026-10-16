package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

type accountCheckRecord struct {
	ID         int64              `json:"id"`
	Account    checkAccount       `json:"account"`
	Prompt     string             `json:"prompt"`
	Model      string             `json:"model"`
	State      string             `json:"state"`
	Result     accountCheckResult `json:"result"`
	StartedAt  time.Time          `json:"started_at"`
	FinishedAt *time.Time         `json:"finished_at"`
}

var errAccountCheckBusy = errors.New("account check already running")

func expireAccountChecks(ctx context.Context, db *sql.DB, adminID int64) error {
	_, err := db.ExecContext(ctx, `UPDATE portal_admin_account_checks SET state='failed', finished_at=now(),
        result='{"text":"","html":"","model":"","error":"检查中断或超时，请重新检查"}'
        WHERE admin_id=$1 AND state='running' AND started_at < now()-interval '6 minutes'`, adminID)
	return err
}

func startAccountCheck(ctx context.Context, db *sql.DB, adminID int64, account checkAccount, prompt, model string) (int64, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 734210094))`, fmt.Sprintf("%d:%d", adminID, account.ID)); err != nil {
		return 0, err
	}
	var busy bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM portal_admin_account_checks WHERE admin_id=$1 AND account_id=$2 AND state='running' AND started_at > now()-interval '6 minutes')`, adminID, account.ID).Scan(&busy); err != nil {
		return 0, err
	}
	if busy {
		return 0, errAccountCheckBusy
	}
	data, err := json.Marshal(account)
	if err != nil {
		return 0, err
	}
	var id int64
	err = tx.QueryRowContext(ctx, `INSERT INTO portal_admin_account_checks(admin_id,account_id,account,prompt,model,state) VALUES($1,$2,$3,$4,$5,'running') RETURNING id`, adminID, account.ID, data, prompt, model).Scan(&id)
	if err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM portal_admin_account_checks WHERE admin_id=$1 AND account_id=$2 AND id NOT IN
        (SELECT id FROM portal_admin_account_checks WHERE admin_id=$1 AND account_id=$2 ORDER BY id DESC LIMIT 3)`, adminID, account.ID); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func finishAccountCheck(ctx context.Context, db *sql.DB, adminID, accountID, id int64, result accountCheckResult) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 734210094))`, fmt.Sprintf("%d:%d", adminID, accountID)); err != nil {
		return err
	}
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	state := "succeeded"
	if result.Error != "" {
		state = "failed"
	}
	if _, err = tx.ExecContext(ctx, `UPDATE portal_admin_account_checks SET state=$1,result=$2,finished_at=now() WHERE id=$3 AND admin_id=$4 AND state='running'`, state, data, id, adminID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM portal_admin_account_checks WHERE admin_id=$1 AND account_id=$2 AND id NOT IN
        (SELECT id FROM portal_admin_account_checks WHERE admin_id=$1 AND account_id=$2 ORDER BY id DESC LIMIT 3)`, adminID, accountID); err != nil {
		return err
	}
	return tx.Commit()
}

func cancelRunningAccountCheck(ctx context.Context, db *sql.DB, adminID, accountID int64) (int64, bool, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 734210094))`, fmt.Sprintf("%d:%d", adminID, accountID)); err != nil {
		return 0, false, err
	}
	result, err := json.Marshal(accountCheckResult{Error: "检查已由管理员终止"})
	if err != nil {
		return 0, false, err
	}
	var id int64
	err = tx.QueryRowContext(ctx, `UPDATE portal_admin_account_checks SET state='cancelled',result=$1,finished_at=now()
        WHERE id=(SELECT id FROM portal_admin_account_checks WHERE admin_id=$2 AND account_id=$3 AND state='running' ORDER BY id DESC LIMIT 1)
        RETURNING id`, result, adminID, accountID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if err = tx.Commit(); err != nil {
		return 0, false, err
	}
	return id, true, nil
}

func (s *Server) accountCheckHistory(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireActiveAdmin(w, r)
	if !ok {
		return
	}
	if s.portalDB == nil {
		writeAPIError(w, 503, "HISTORY_UNAVAILABLE", "检查记录存储不可用")
		return
	}
	var accountID int64
	if r.PathValue("accountID") != "" {
		accountID, ok = pathID(w, r, "accountID")
		if !ok {
			return
		}
	}
	if err := expireAccountChecks(r.Context(), s.portalDB, admin.ID); err != nil {
		s.writeAccountHistoryError(w, err)
		return
	}
	// The overview restores cards without downloading every historical HTML document.
	query := `SELECT DISTINCT ON (account_id) id,account,prompt,model,state,'{"text":"","html":"","model":""}'::jsonb,started_at,finished_at
        FROM portal_admin_account_checks WHERE admin_id=$1 ORDER BY account_id,id DESC`
	args := []any{admin.ID}
	if accountID != 0 {
		query = `SELECT id,account,prompt,model,state,result,started_at,finished_at FROM portal_admin_account_checks WHERE admin_id=$1 AND account_id=$2 ORDER BY id DESC LIMIT 3`
		args = append(args, accountID)
	}
	rows, err := s.portalDB.QueryContext(r.Context(), query, args...)
	if err != nil {
		s.writeAccountHistoryError(w, err)
		return
	}
	defer rows.Close()
	records := []accountCheckRecord{}
	for rows.Next() {
		var record accountCheckRecord
		var account, result []byte
		if err = rows.Scan(&record.ID, &account, &record.Prompt, &record.Model, &record.State, &result, &record.StartedAt, &record.FinishedAt); err != nil {
			s.writeAccountHistoryError(w, err)
			return
		}
		if err = json.Unmarshal(account, &record.Account); err != nil {
			s.writeAccountHistoryError(w, err)
			return
		}
		if err = json.Unmarshal(result, &record.Result); err != nil {
			s.writeAccountHistoryError(w, err)
			return
		}
		records = append(records, record)
	}
	if err = rows.Err(); err != nil {
		s.writeAccountHistoryError(w, err)
		return
	}
	if accountID == 0 && len(records) > 0 {
		core, token, ok := s.accountCheckCore(w, r)
		if !ok {
			return
		}
		current, err := currentCheckAccounts(r.Context(), core, token)
		if err != nil {
			s.writeRentalError(w, err)
			return
		}
		visible := make([]accountCheckRecord, 0, len(records))
		for _, record := range records {
			if account, exists := current[record.Account.ID]; exists {
				record.Account = account
				visible = append(visible, record)
			}
		}
		records = visible
	}
	w.Header().Set("Cache-Control", "no-store")
	writeSuccess(w, records)
}

func (s *Server) writeAccountHistoryError(w http.ResponseWriter, err error) {
	s.logger.Error("account check history storage failed", "error", err)
	writeAPIError(w, 503, "HISTORY_UNAVAILABLE", "检查记录读写失败，请稍后重试")
}
