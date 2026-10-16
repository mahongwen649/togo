package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const exclusiveAccountTestTimeout = 5 * time.Minute
const exclusiveAccountTestStaleAfter = exclusiveAccountTestTimeout + time.Minute
const exclusiveAccountTestInterrupted = "测试进程中断或超过运行时限，请重新测试"

type exclusiveAccountTestStore interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// exclusiveAccountTestView returns the latest result for one Core account.
func (s *Server) exclusiveAccountTestView(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.exclusiveUser(w, r)
	if !ok {
		return
	}
	channelID, accountID, ok := exclusiveAccountPathIDs(w, r)
	if !ok {
		return
	}
	var visible bool
	err := s.portalDB.QueryRowContext(r.Context(), `SELECT ($2 OR state NOT IN ('unpublished','provisioning','error')) AND account_ids @> jsonb_build_array($3::bigint) FROM portal_exclusive_channels WHERE id=$1`, channelID, user.Role == "admin", accountID).Scan(&visible)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	if !visible {
		writeAPIError(w, http.StatusNotFound, "EXCLUSIVE_NOT_FOUND", "通道或账号不存在")
		return
	}
	v, err := s.exclusiveAccountTestMetadata(r.Context(), accountID, true)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeSuccess(w, v)
}

func (s *Server) exclusiveAccountTestStart(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.exclusiveUser(w, r)
	if !ok {
		return
	}
	channelID, accountID, ok := exclusiveAccountPathIDs(w, r)
	if !ok {
		return
	}
	core, ok := s.core.(marketCoreRaw)
	if !ok {
		writeAPIError(w, http.StatusServiceUnavailable, "CORE_TEST_UNAVAILABLE", "Core 账号测试接口不可用")
		return
	}
	tx, err := s.portalDB.BeginTx(r.Context(), nil)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	defer tx.Rollback()
	var channelState string
	var linked bool
	if err = tx.QueryRowContext(r.Context(), `SELECT state,account_ids @> jsonb_build_array($2::bigint) FROM portal_exclusive_channels WHERE id=$1 FOR UPDATE`, channelID, accountID).Scan(&channelState, &linked); err != nil {
		s.exclusiveError(w, err)
		return
	}
	if !linked {
		writeAPIError(w, http.StatusNotFound, "ACCOUNT_NOT_IN_CHANNEL", "账号不属于该通道")
		return
	}
	if user.Role == "admin" {
		s.admitExclusiveAccountTest(w, r, tx, core, channelID, accountID, 0)
		return
	}
	l, err := scanExclusiveLease(tx.QueryRowContext(r.Context(), `SELECT `+exclusiveLeaseColumns+` FROM portal_exclusive_leases WHERE channel_id=$1 AND state<>'closed' FOR UPDATE`, channelID))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, http.StatusForbidden, "CHANNEL_NOT_OWNED", "只有当前领取人可以进行降智测试")
		return
	}
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	now := time.Now()
	if l.UserID != user.ID || l.State != "active" || channelState != "active" || !now.Before(exclusiveDeadline(l)) {
		writeAPIError(w, http.StatusForbidden, "CHANNEL_NOT_OWNED", "只有未到期且未回收通道的当前领取人可以进行降智测试")
		return
	}
	if l.BillingCheckedAt == nil || now.Sub(*l.BillingCheckedAt) > 30*time.Second {
		writeAPIError(w, http.StatusServiceUnavailable, "BILLING_STATE_UNAVAILABLE", "暂时无法确认 Core 消费状态，请稍后重试")
		return
	}
	if err = expireExclusiveAccountTest(r.Context(), tx, accountID); err != nil {
		s.exclusiveError(w, err)
		return
	}
	var blocked bool
	if err = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM portal_exclusive_account_tests WHERE account_id=$1 AND state IN ('running','uncertain'))`, accountID).Scan(&blocked); err != nil {
		s.exclusiveError(w, err)
		return
	}
	if blocked {
		writeAPIError(w, http.StatusConflict, "TEST_IN_PROGRESS", "该账号已有降智测试未完成，请勿重复提交")
		return
	}
	var count int
	if err = tx.QueryRowContext(r.Context(), `SELECT count(*) FROM portal_exclusive_requests WHERE lease_id=$1 AND state<>'finished'`, l.ID).Scan(&count); err != nil {
		s.exclusiveError(w, err)
		return
	}
	if count >= 3 {
		writeAPIError(w, http.StatusTooManyRequests, "CHANNEL_CONCURRENCY", "通道并发已达上限")
		return
	}
	s.admitExclusiveAccountTest(w, r, tx, core, channelID, accountID, l.ID)
}

func (s *Server) admitExclusiveAccountTest(w http.ResponseWriter, r *http.Request, tx *sql.Tx, core marketCoreRaw, channelID, accountID, leaseID int64) bool {
	if err := expireExclusiveAccountTest(r.Context(), tx, accountID); err != nil {
		s.exclusiveError(w, err)
		return false
	}
	var blocked bool
	if err := tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM portal_exclusive_account_tests WHERE account_id=$1 AND state IN ('running','uncertain'))`, accountID).Scan(&blocked); err != nil {
		s.exclusiveError(w, err)
		return false
	}
	if blocked {
		writeAPIError(w, http.StatusConflict, "TEST_IN_PROGRESS", "该账号已有降智测试未完成，请勿重复提交")
		return false
	}
	requestID, err := exclusiveRandom("")
	if err != nil {
		s.exclusiveError(w, err)
		return false
	}
	if leaseID != 0 {
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO portal_exclusive_requests(id,lease_id,state) VALUES($1,$2,'running')`, requestID, leaseID); err != nil {
			s.exclusiveError(w, err)
			return false
		}
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO portal_exclusive_account_tests(account_id,channel_id,request_id,state) VALUES($1,$2,$3,'running') ON CONFLICT(account_id) DO UPDATE SET channel_id=$2,request_id=$3,state='running',started_at=CURRENT_TIMESTAMP,error=''`, accountID, channelID, requestID); err != nil {
		s.exclusiveError(w, err)
		return false
	}
	if err = tx.Commit(); err != nil {
		s.exclusiveError(w, err)
		return false
	}
	workerContext := s.exclusive.Context
	if workerContext == nil {
		workerContext = context.Background()
	}
	go s.generateExclusiveAccountTest(workerContext, core, channelID, accountID, requestID)
	writeSuccess(w, map[string]any{"state": "running", "account_id": accountID})
	return true
}

func exclusiveAccountPathIDs(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	channelID, err := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	if err != nil || channelID <= 0 {
		writeAPIError(w, http.StatusBadRequest, "INVALID_CHANNEL", "通道无效")
		return 0, 0, false
	}
	accountID, err := strconv.ParseInt(r.PathValue("accountID"), 10, 64)
	if err != nil || accountID <= 0 {
		writeAPIError(w, http.StatusBadRequest, "INVALID_ACCOUNT", "账号无效")
		return 0, 0, false
	}
	return channelID, accountID, true
}

func (s *Server) exclusiveAccountTestMetadata(ctx context.Context, accountID int64, content bool) (exclusivePageTest, error) {
	v := exclusivePageTest{State: "none"}
	tx, err := s.portalDB.BeginTx(ctx, nil)
	if err != nil {
		return v, err
	}
	defer tx.Rollback()
	if err = expireExclusiveAccountTest(ctx, tx, accountID); err != nil {
		return v, err
	}
	err = tx.QueryRowContext(ctx, `SELECT state,started_at,generated_at,html<>'',CASE WHEN $2 THEN html ELSE '' END,error FROM portal_exclusive_account_tests WHERE account_id=$1`, accountID, content).Scan(&v.State, &v.StartedAt, &v.GeneratedAt, &v.HasPage, &v.HTML, &v.Error)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	if err == nil {
		err = tx.Commit()
	}
	return v, err
}

func expireExclusiveAccountTest(ctx context.Context, store exclusiveAccountTestStore, accountID int64) error {
	var requestID string
	err := store.QueryRowContext(ctx, `UPDATE portal_exclusive_account_tests SET state='failed',error=$1 WHERE account_id=$2 AND state='running' AND started_at<$3 RETURNING request_id`, exclusiveAccountTestInterrupted, accountID, time.Now().Add(-exclusiveAccountTestStaleAfter)).Scan(&requestID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = store.ExecContext(ctx, `UPDATE portal_exclusive_requests SET state='finished',ended_at=CURRENT_TIMESTAMP,error=$1 WHERE id=$2 AND state='running'`, exclusiveAccountTestInterrupted, requestID)
	return err
}

func (s *Server) generateExclusiveAccountTest(ctx context.Context, core marketCoreRaw, channelID, accountID int64, requestID string) {
	ctx, cancel := context.WithTimeout(ctx, exclusiveAccountTestTimeout)
	defer cancel()
	payload := map[string]string{"model_id": exclusivePageModel, "prompt": exclusivePagePrompt}
	response, err := core.AdminMarketRawRequest(ctx, http.MethodPost, fmt.Sprintf("/api/v1/admin/accounts/%d/test", accountID), payload, "")
	state, message, html := "failed", "Core 账号测试失败，请检查账号模型可用性", ""
	status := 0
	if err == nil {
		status = response.StatusCode
		html, message, err = parseExclusiveAccountTest(response.Body)
		if err == nil {
			state, message = "succeeded", ""
		}
	} else {
		message = "Core 账号测试请求失败，请稍后重试"
		var urlErr *url.Error
		if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &urlErr) && urlErr.Timeout()) {
			message = "Core 账号测试超时，请稍后重试"
		}
		s.logger.Error("exclusive account test Core request failed", "account_id", accountID, "request_id", requestID, "error", err)
	}
	if err != nil && message == "" {
		message = err.Error()
	}
	saveCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, txErr := s.portalDB.BeginTx(saveCtx, nil)
	if txErr == nil {
		defer tx.Rollback()
		var result sql.Result
		result, txErr = tx.ExecContext(saveCtx, `UPDATE portal_exclusive_account_tests SET state=$1,error=$2,html=CASE WHEN $3<>'' THEN $3 ELSE html END,generated_at=CASE WHEN $3<>'' THEN CURRENT_TIMESTAMP ELSE generated_at END WHERE account_id=$4 AND request_id=$5 AND state='running'`, state, message, html, accountID, requestID)
		if txErr == nil {
			var updated int64
			updated, txErr = result.RowsAffected()
			if txErr == nil && updated == 1 {
				_, txErr = tx.ExecContext(saveCtx, `UPDATE portal_exclusive_requests SET state='finished',ended_at=CURRENT_TIMESTAMP,status_code=$1,error=$2 WHERE id=$3`, status, message, requestID)
			}
		}
		if txErr == nil {
			txErr = tx.Commit()
		}
	}
	if txErr != nil {
		s.logger.Error("exclusive account test result could not be persisted", "account_id", accountID, "request_id", requestID, "error", txErr)
	}
}
