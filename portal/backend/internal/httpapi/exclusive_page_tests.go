package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const exclusivePageModel = "gpt-6-astra"
const exclusivePagePrompt = "生成 html，内容是 svg 绘制鹈鹕骑自行车 2D 动画，不用进行测试"
const exclusivePageLimit = 2 << 20

type exclusivePageTest struct {
	State       string     `json:"state"`
	StartedAt   *time.Time `json:"started_at"`
	GeneratedAt *time.Time `json:"generated_at"`
	HasPage     bool       `json:"has_page"`
	HTML        string     `json:"html,omitempty"`
	Error       string     `json:"error,omitempty"`
}

func (s *Server) exclusivePageMetadata(ctx context.Context, id int64, content bool) (exclusivePageTest, error) {
	v := exclusivePageTest{State: "none"}
	// Avoid transferring page bodies in lobby responses.
	err := s.portalDB.QueryRowContext(ctx, `SELECT state,started_at,generated_at,html<>'',CASE WHEN $2 THEN html ELSE '' END,error FROM portal_exclusive_page_tests WHERE channel_id=$1`, id, content).Scan(&v.State, &v.StartedAt, &v.GeneratedAt, &v.HasPage, &v.HTML, &v.Error)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return v, err
}

func (s *Server) exclusivePageTestView(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.exclusiveUser(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	if err != nil || id <= 0 {
		writeAPIError(w, 400, "INVALID_CHANNEL", "通道无效")
		return
	}
	var visible bool
	err = s.portalDB.QueryRowContext(r.Context(), `SELECT ($2 OR state NOT IN ('unpublished','provisioning','error')) FROM portal_exclusive_channels WHERE id=$1`, id, user.Role == "admin").Scan(&visible)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	if !visible {
		writeAPIError(w, 404, "EXCLUSIVE_NOT_FOUND", "通道不存在")
		return
	}
	v, err := s.exclusivePageMetadata(r.Context(), id, true)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeSuccess(w, v)
}

func (s *Server) exclusivePageTestStart(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.exclusiveUser(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	if err != nil || id <= 0 {
		writeAPIError(w, 400, "INVALID_CHANNEL", "通道无效")
		return
	}
	relay, ok := s.core.(exclusiveRelay)
	if !ok {
		writeAPIError(w, 503, "RELAY_UNAVAILABLE", "Core 转发适配器不可用")
		return
	}
	tx, err := s.portalDB.BeginTx(r.Context(), nil)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	defer tx.Rollback()
	var state string
	if err = tx.QueryRowContext(r.Context(), `SELECT state FROM portal_exclusive_channels WHERE id=$1 FOR UPDATE`, id).Scan(&state); err != nil {
		s.exclusiveError(w, err)
		return
	}
	l, err := scanExclusiveLease(tx.QueryRowContext(r.Context(), `SELECT `+exclusiveLeaseColumns+` FROM portal_exclusive_leases WHERE channel_id=$1 AND state<>'closed' FOR UPDATE`, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, 403, "CHANNEL_NOT_OWNED", "只有当前领取人可以测试")
		return
	}
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	now := time.Now()
	if l.UserID != user.ID || l.State != "active" || state != "active" || !now.Before(exclusiveDeadline(l)) {
		writeAPIError(w, 403, "CHANNEL_NOT_OWNED", "只有未到期且未回收通道的当前领取人可以测试")
		return
	}
	if l.BillingCheckedAt == nil || now.Sub(*l.BillingCheckedAt) > 30*time.Second {
		writeAPIError(w, 503, "BILLING_STATE_UNAVAILABLE", "暂时无法确认 Core 消费状态，请稍后重试")
		return
	}
	var blocked bool
	if err = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM portal_exclusive_page_tests WHERE channel_id=$1 AND state IN ('running','uncertain'))`, id).Scan(&blocked); err != nil {
		s.exclusiveError(w, err)
		return
	}
	if blocked {
		writeAPIError(w, 409, "TEST_IN_PROGRESS", "该通道已有生成请求未完成，请勿重复提交")
		return
	}
	var count int
	if err = tx.QueryRowContext(r.Context(), `SELECT count(*) FROM portal_exclusive_requests WHERE lease_id=$1 AND state<>'finished'`, l.ID).Scan(&count); err != nil {
		s.exclusiveError(w, err)
		return
	}
	if count >= 3 {
		writeAPIError(w, 429, "CHANNEL_CONCURRENCY", "通道并发已达上限")
		return
	}
	upstream, err := s.openExclusive(l.UpstreamSecret)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	requestID, err := exclusiveRandom("")
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO portal_exclusive_requests(id,lease_id,state) VALUES($1,$2,'running')`, requestID, l.ID); err != nil {
		s.exclusiveError(w, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO portal_exclusive_page_tests(channel_id,request_id,state) VALUES($1,$2,'running') ON CONFLICT(channel_id) DO UPDATE SET request_id=$2,state='running',started_at=CURRENT_TIMESTAMP,error=''`, id, requestID); err != nil {
		s.exclusiveError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		s.exclusiveError(w, err)
		return
	}
	// The durable ledger owns this operation, not the HTTP client or the preview modal.
	go s.generateExclusivePage(context.WithoutCancel(r.Context()), relay, id, requestID, upstream)
	writeSuccess(w, map[string]any{"state": "running"})
}

func (s *Server) generateExclusivePage(ctx context.Context, relay exclusiveRelay, channel int64, id, key string) {
	body, _ := json.Marshal(map[string]any{"model": exclusivePageModel, "reasoning": map[string]string{"effort": "high"}, "input": exclusivePagePrompt, "instructions": "仅返回完整、自包含的 HTML 页面，包含内联 SVG、CSS 和必要的 JavaScript。不要解释、不要 Markdown 代码围栏，不引用外部资源。", "stream": false})
	request := (&http.Request{Method: "POST", URL: &url.URL{Path: "/v1/responses"}, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body))}).WithContext(ctx)
	request.Header.Set("Authorization", "Bearer "+key)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := relay.RelayRoundTrip(request)
	status, clean, html, message := 0, false, "", "Core 连接中断，无法确认请求已结束；通道保持锁定等待核查"
	diagnostic := ""
	if err == nil {
		status = response.StatusCode
		data, readErr := io.ReadAll(io.LimitReader(response.Body, exclusivePageLimit+1))
		_, drainErr := io.Copy(io.Discard, response.Body)
		response.Body.Close()
		clean = readErr == nil && drainErr == nil
		if clean {
			switch {
			case status < 200 || status >= 300:
				message = "Core 生成失败，请检查通道模型可用性和中转余额"
			case len(data) > exclusivePageLimit:
				message = "生成响应过大，请重试"
			default:
				html, err = parseExclusivePage(data)
				if err != nil {
					message = err.Error()
					diagnostic, err = s.sealExclusive(exclusivePageDiagnostic(data))
					if err != nil {
						s.logger.Error("exclusive page diagnostic persistence failed", "request_id", id, "error", err)
					}
					s.logger.Warn("exclusive page parsing failed", "channel_id", channel, "request_id", id, "response_bytes", len(data), "reason", message)
				} else {
					message = ""
				}
			}
		}
	}
	state, ledger := "failed", "finished"
	if !clean {
		state, ledger = "uncertain", "uncertain"
	} else if html != "" {
		state = "succeeded"
	}
	// Publish the result and finish the in-flight request together, before cleanup can proceed.
	saveCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, err := s.portalDB.BeginTx(saveCtx, nil)
	if err == nil {
		defer tx.Rollback()
		var result sql.Result
		result, err = tx.ExecContext(saveCtx, `UPDATE portal_exclusive_page_tests SET state=$1,error=$2,html=CASE WHEN $3<>'' THEN $3 ELSE html END,generated_at=CASE WHEN $3<>'' THEN CURRENT_TIMESTAMP ELSE generated_at END,response_secret=$6 WHERE channel_id=$4 AND request_id=$5`, state, message, html, channel, id, diagnostic)
		if err == nil {
			var n int64
			n, err = result.RowsAffected()
			if err == nil && n != 1 {
				err = errors.New("page test ownership lost")
			}
		}
		if err == nil {
			_, err = tx.ExecContext(saveCtx, `UPDATE portal_exclusive_requests SET state=$1,ended_at=CURRENT_TIMESTAMP,status_code=$2,error=$3 WHERE id=$4`, ledger, status, message, id)
		}
		if err == nil {
			err = tx.Commit()
		}
	}
	if err != nil {
		s.logger.Error("exclusive page result could not be persisted", "request_id", id, "error", err)
	}
}
