package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

func (s *Server) exclusiveUser(w http.ResponseWriter, r *http.Request) (coreclient.User, string, bool) {
	if !s.exclusiveAvailable(w) {
		return coreclient.User{}, "", false
	}
	token, ok := bearerToken(w, r)
	if !ok {
		return coreclient.User{}, "", false
	}
	user, err := s.core.Profile(r.Context(), token)
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return user, "", false
	}
	if user.Status != "active" {
		writeAPIError(w, 403, "USER_INACTIVE", "账号已停用")
		return user, "", false
	}
	return user, token, true
}
func (s *Server) exclusiveUserCore(ctx context.Context, token, method, path string, payload, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := s.core.UserAPIRequest(ctx, coreclient.CoreRequest{AccessToken: token, Method: method, Path: path, ContentType: "application/json", Body: body})
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Core exclusive credential operation returned HTTP %d", resp.StatusCode)
	}
	var envelope struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(resp.Body, &envelope) != nil || envelope.Code != 0 || len(envelope.Data) == 0 {
		return errors.New("invalid Core exclusive credential response")
	}
	if out != nil {
		return json.Unmarshal(envelope.Data, out)
	}
	return nil
}
func (s *Server) exclusiveClaim(w http.ResponseWriter, r *http.Request) {
	user, token, ok := s.exclusiveUser(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r, "channelID")
	if !ok {
		return
	}
	var p struct {
		Duration        int      `json:"duration"`
		RateMultiplier  *float64 `json:"rate_multiplier"`
		PricingRevision *int64   `json:"pricing_revision"`
	}
	if decodeJSON(r, &p) != nil {
		writeAPIError(w, 400, "INVALID_DURATION", "请选择使用时长")
		return
	}
	if p.Duration == 0 {
		p.Duration = 1440
	}
	if user.Balance <= 0 {
		writeAPIError(w, 409, "INSUFFICIENT_RELAY_BALANCE", "中转余额不足")
		return
	}
	conn, unlock, err := s.exclusiveLock(r.Context(), 734210092)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	defer unlock()
	tx, err := conn.BeginTx(r.Context(), nil)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	defer tx.Rollback()
	c, err := scanExclusiveChannel(tx.QueryRowContext(r.Context(), `SELECT `+exclusiveChannelColumns+` FROM portal_exclusive_channels WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	l, err := scanExclusiveLease(tx.QueryRowContext(r.Context(), `SELECT `+exclusiveLeaseColumns+` FROM portal_exclusive_leases WHERE user_id=$1 AND state<>'closed'`, user.ID))
	if err == nil {
		if l.ChannelID != id || (l.State != "provisioning" && l.State != "active") {
			s.exclusiveError(w, exclusiveConflict("你已有占用中的通道"))
			return
		}
		if err = tx.Commit(); err != nil {
			s.exclusiveError(w, err)
			return
		}
		if l.State == "provisioning" {
			if err = s.provisionExclusiveLease(r.Context(), token, l.ID); err != nil {
				s.exclusiveLeaseError(l.ID, err)
				s.exclusiveError(w, err)
				return
			}
		}
		writeSuccess(w, map[string]any{"id": l.ID})
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		s.exclusiveError(w, err)
		return
	}
	if c.State != "available" || c.GroupID == nil {
		s.exclusiveError(w, exclusiveConflict("通道当前不可领取"))
		return
	}
	if p.RateMultiplier != nil && *p.RateMultiplier != c.RateMultiplier {
		s.exclusiveError(w, exclusiveConflict("通道倍率已调整，请刷新后重新确认领取"))
		return
	}
	if !c.PricingSynced {
		s.exclusiveError(w, exclusiveConflict("通道定价正在同步，请稍后刷新"))
		return
	}
	if (p.PricingRevision != nil && *p.PricingRevision != c.PricingRevision) || (p.PricingRevision == nil && (c.PricingRevision > 1 || len(c.ModelPricing) > 0 || c.ImagePrice != exclusiveDefaultImagePrice || c.ImageRateMultiplier != 1)) {
		s.exclusiveError(w, exclusiveConflict("通道价格已调整，请刷新后重新确认领取"))
		return
	}
	if !slices.Contains(c.Durations, p.Duration) {
		writeAPIError(w, 400, "INVALID_DURATION", "请选择管理员允许的使用时长")
		return
	}
	// Verify membership before issuing any credential; account configuration must remain exclusive.
	if err = s.verifyExclusiveGroup(r.Context(), c, *c.GroupID); err != nil {
		s.exclusiveError(w, err)
		return
	}
	available := false
	for _, aid := range c.AccountIDs {
		var a rentalAccount
		if err = s.rentalCoreRequest(r.Context(), "GET", fmt.Sprintf("/api/v1/admin/accounts/%d", aid), nil, &a, ""); err != nil {
			s.exclusiveError(w, err)
			return
		}
		if validateExclusiveAccount(a, *c.GroupID, time.Now()) == nil {
			available = true
		}
	}
	if !available {
		s.exclusiveError(w, exclusiveConflict("分组暂无可用账号"))
		return
	}
	clientKey, err := exclusiveRandom(exclusiveTokenPrefix)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	upstreamKey, err := exclusiveRandom(exclusiveUpstreamPrefix)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	secret, err := s.sealExclusive(clientKey)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	upstream, err := s.sealExclusive(upstreamKey)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	err = tx.QueryRowContext(r.Context(), `INSERT INTO portal_exclusive_leases(channel_id,user_id,state,token_hash,token_secret,upstream_secret,expires_at) VALUES($1,$2,'provisioning',$3,$4,$5,CURRENT_TIMESTAMP+($6*interval '1 minute')) RETURNING id`, id, user.ID, exclusiveHash(clientKey), secret, upstream, p.Duration).Scan(&l.ID)
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `UPDATE portal_exclusive_channels SET state='active',updated_at=CURRENT_TIMESTAMP WHERE id=$1`, id)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		s.exclusiveError(w, exclusiveConflict("通道或用户已被另一项领取操作锁定，请刷新"))
		return
	}
	_ = s.exclusiveEvent(r.Context(), id, user.ID, "领取通道：正在创建专用凭据")
	if err = s.provisionExclusiveLease(r.Context(), token, l.ID); err != nil {
		s.exclusiveLeaseError(l.ID, err)
		s.exclusiveError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"id": l.ID})
}
func (s *Server) exclusiveLeaseError(id int64, err error) {
	s.logger.Error("exclusive lease synchronization failed", "lease_id", id, "error", err)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = s.portalDB.ExecContext(ctx, `UPDATE portal_exclusive_leases SET error='凭据同步未完成，请重试领取同一通道' WHERE id=$1 AND state='provisioning'`, id)
}
func (s *Server) findExclusiveCoreKey(ctx context.Context, l exclusiveLease, key string) (coreclient.APIKey, error) {
	for page := 1; page <= 1000; page++ {
		var data struct {
			Items []coreclient.APIKey `json:"items"`
			Total int                 `json:"total"`
		}
		if err := s.rentalCoreRequest(ctx, "GET", fmt.Sprintf("/api/v1/admin/users/%d/api-keys?page=%d&page_size=100", l.UserID, page), nil, &data, ""); err != nil {
			return coreclient.APIKey{}, err
		}
		for _, k := range data.Items {
			if k.Key == key {
				return k, nil
			}
		}
		if page*100 >= data.Total || len(data.Items) == 0 {
			return coreclient.APIKey{}, sql.ErrNoRows
		}
	}
	return coreclient.APIKey{}, errors.New("Core key inventory exceeds safe pagination bound")
}
func (s *Server) provisionExclusiveLease(ctx context.Context, token string, id int64) error {
	l, err := scanExclusiveLease(s.portalDB.QueryRowContext(ctx, `SELECT `+exclusiveLeaseColumns+` FROM portal_exclusive_leases WHERE id=$1`, id))
	if err != nil {
		return err
	}
	if l.State != "provisioning" {
		return exclusiveConflict("使用记录已变化")
	}
	if !time.Now().Before(l.ExpiresAt) {
		return exclusiveConflict("领取同步已超时，请管理员核查")
	}
	c, err := scanExclusiveChannel(s.portalDB.QueryRowContext(ctx, `SELECT `+exclusiveChannelColumns+` FROM portal_exclusive_channels WHERE id=$1`, l.ChannelID))
	if err != nil || c.GroupID == nil {
		return errors.New("exclusive Core group is not ready")
	}
	key, err := s.openExclusive(l.UpstreamSecret)
	if err != nil {
		return err
	}
	k, err := s.findExclusiveCoreKey(ctx, l, key)
	if errors.Is(err, sql.ErrNoRows) {
		err = s.exclusiveUserCore(ctx, token, "POST", "/api/v1/keys", map[string]any{"name": fmt.Sprintf("Portal exclusive lease %d", id), "custom_key": key, "expires_in_days": 1}, &k)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if k.ID <= 0 || k.Key != key {
		return errors.New("Core returned a different exclusive credential")
	}
	if _, err = s.portalDB.ExecContext(ctx, `UPDATE portal_exclusive_leases SET core_key_id=$1 WHERE id=$2`, k.ID, id); err != nil {
		return err
	}
	if err = s.exclusiveUserCore(ctx, token, "PUT", fmt.Sprintf("/api/v1/keys/%d", k.ID), map[string]any{"expires_at": l.ExpiresAt.UTC().Format(time.RFC3339), "status": "active"}, &k); err != nil {
		return err
	}
	var bound struct {
		APIKey coreclient.APIKey `json:"api_key"`
	}
	if err = s.rentalCoreRequest(ctx, "PUT", fmt.Sprintf("/api/v1/admin/api-keys/%d", k.ID), map[string]any{"group_id": *c.GroupID}, &bound, ""); err != nil {
		return err
	}
	if bound.APIKey.ID != k.ID || bound.APIKey.GroupID == nil || *bound.APIKey.GroupID != *c.GroupID {
		return errors.New("Core exclusive key binding was not confirmed")
	}
	_, err = s.portalDB.ExecContext(ctx, `UPDATE portal_exclusive_leases SET state='active',billing_checked_at=CURRENT_TIMESTAMP,error='' WHERE id=$1 AND state='provisioning'`, id)
	if err == nil {
		_ = s.exclusiveEvent(ctx, c.ID, l.UserID, "领取成功")
	}
	return err
}
func (s *Server) exclusiveReturn(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.exclusiveUser(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r, "channelID")
	if !ok {
		return
	}
	admin := strings.HasPrefix(r.URL.Path, "/api/v1/admin/")
	if admin && user.Role != "admin" {
		writeAPIError(w, 403, "FORBIDDEN", "需要管理员权限")
		return
	}
	_, unlock, err := s.exclusiveLock(r.Context(), 734210092)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	defer unlock()
	tx, err := s.portalDB.BeginTx(r.Context(), nil)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	defer tx.Rollback()
	var cid int64
	if err = tx.QueryRowContext(r.Context(), `SELECT id FROM portal_exclusive_channels WHERE id=$1 FOR UPDATE`, id).Scan(&cid); err != nil {
		s.exclusiveError(w, err)
		return
	}
	l, err := scanExclusiveLease(tx.QueryRowContext(r.Context(), `SELECT `+exclusiveLeaseColumns+` FROM portal_exclusive_leases WHERE channel_id=$1 AND state<>'closed' FOR UPDATE`, id))
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	if !admin && l.UserID != user.ID {
		writeAPIError(w, 403, "FORBIDDEN", "不能归还其他用户的通道")
		return
	}
	reason := "manual"
	if admin {
		reason = "admin"
	}
	if l.State == "provisioning" {
		if err = s.closeExclusiveProvisioning(r.Context(), tx, l, reason); err == nil {
			err = tx.Commit()
		}
		if err != nil {
			s.exclusiveError(w, err)
			return
		}
		_ = s.exclusiveEvent(r.Context(), id, user.ID, "取消未完成领取："+reason)
		writeSuccess(w, map[string]any{"state": "closed"})
		return
	}
	if l.State != "releasing" {
		_, err = tx.ExecContext(r.Context(), `UPDATE portal_exclusive_leases SET state='releasing',reason=$1 WHERE id=$2`, reason, l.ID)
		if err == nil {
			_, err = tx.ExecContext(r.Context(), `UPDATE portal_exclusive_channels SET state='releasing',updated_at=CURRENT_TIMESTAMP WHERE id=$1`, id)
		}
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	_ = s.exclusiveEvent(r.Context(), id, user.ID, "开始回收："+reason)
	writeSuccess(w, map[string]any{"state": "releasing"})
}

// The client credential is never exposed or admitted while provisioning. Under
// the lease allocator lock, reconcile any lost create response before releasing.
func (s *Server) closeExclusiveProvisioning(ctx context.Context, tx *sql.Tx, l exclusiveLease, reason string) error {
	key, err := s.openExclusive(l.UpstreamSecret)
	if err != nil {
		return err
	}
	k, err := s.findExclusiveCoreKey(ctx, l, key)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil {
		var unbound struct {
			APIKey coreclient.APIKey `json:"api_key"`
		}
		if err = s.rentalCoreRequest(ctx, "PUT", fmt.Sprintf("/api/v1/admin/api-keys/%d", k.ID), map[string]any{"group_id": 0}, &unbound, ""); err != nil {
			return err
		}
		if unbound.APIKey.ID != k.ID || pointerID(unbound.APIKey.GroupID) != 0 {
			return errors.New("Core provisioning cleanup was not confirmed")
		}
		if _, err = tx.ExecContext(ctx, `UPDATE portal_exclusive_leases SET core_key_id=$1 WHERE id=$2`, k.ID, l.ID); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE portal_exclusive_leases SET state='closed',reason=$1,ended_at=CURRENT_TIMESTAMP,error='' WHERE id=$2 AND state='provisioning'`, reason, l.ID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE portal_exclusive_channels SET state='cooling',cooldown_until=CURRENT_TIMESTAMP+(cooldown_seconds*interval '1 second'),updated_at=CURRENT_TIMESTAMP WHERE id=$1`, l.ChannelID)
	return err
}
func (s *Server) syncExclusiveBilling(ctx context.Context, l exclusiveLease) error {
	if l.CoreKeyID == nil {
		return errors.New("exclusive credential has not been confirmed")
	}
	var stats struct {
		Cost     float64 `json:"total_actual_cost"`
		Requests int64   `json:"total_requests"`
		Tokens   int64   `json:"total_tokens"`
	}
	if err := s.rentalCoreRequest(ctx, "GET", fmt.Sprintf("/api/v1/admin/usage/stats?api_key_id=%d&user_id=%d", *l.CoreKeyID, l.UserID), nil, &stats, ""); err != nil {
		return err
	}
	var last *time.Time
	for page := 1; page <= 1000; page++ {
		var data struct {
			Items []struct {
				UserID int64     `json:"user_id"`
				KeyID  int64     `json:"api_key_id"`
				Cost   float64   `json:"actual_cost"`
				At     time.Time `json:"created_at"`
			} `json:"items"`
			Total int `json:"total"`
		}
		err := s.rentalCoreRequest(ctx, "GET", fmt.Sprintf("/api/v1/admin/usage?api_key_id=%d&user_id=%d&page=%d&page_size=100&sort_by=created_at&sort_order=desc", *l.CoreKeyID, l.UserID, page), nil, &data, "")
		if err != nil {
			return err
		}
		stop := false
		for _, row := range data.Items {
			if row.KeyID != *l.CoreKeyID || row.UserID != l.UserID {
				return errors.New("Core usage response violated lease scope")
			}
			if row.Cost > 0 {
				t := row.At
				if t.IsZero() || t.Before(l.StartedAt) || t.After(time.Now().Add(time.Minute)) {
					return errors.New("Core usage timestamp is outside the lease")
				}
				last = &t
				stop = true
				break
			}
			if l.LastChargeAt != nil && !row.At.After(*l.LastChargeAt) {
				stop = true
				break
			}
		}
		if stop || page*100 >= data.Total || len(data.Items) == 0 {
			break
		}
		if page == 1000 {
			return errors.New("Core billing scan exceeds safe pagination bound")
		}
	}
	_, err := s.portalDB.ExecContext(ctx, `UPDATE portal_exclusive_leases SET last_charge_at=GREATEST(last_charge_at,$1),billing_checked_at=CURRENT_TIMESTAMP,cost=GREATEST(cost,$2),requests=GREATEST(requests,$3),tokens=GREATEST(tokens,$4) WHERE id=$5`, last, stats.Cost, stats.Requests, stats.Tokens, l.ID)
	return err
}
func (s *Server) runExclusiveWorker(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		s.exclusiveWorkerTick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (s *Server) exclusiveWorkerTick(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	_, unlock, err := s.exclusiveLock(ctx, 734210092)
	if err != nil {
		return
	}
	defer unlock()
	rows, err := s.portalDB.QueryContext(ctx, `SELECT `+exclusiveLeaseColumns+` FROM portal_exclusive_leases WHERE state IN ('provisioning','active','releasing') OR (state='closed' AND core_key_id IS NOT NULL AND ended_at>CURRENT_TIMESTAMP-interval '1 day') ORDER BY billing_checked_at ASC NULLS FIRST,id LIMIT 200`)
	if err != nil {
		return
	}
	leases := []exclusiveLease{}
	for rows.Next() {
		l, e := scanExclusiveLease(rows)
		if e != nil {
			rows.Close()
			return
		}
		leases = append(leases, l)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return
	}
	for _, l := range leases {
		if l.State == "provisioning" {
			if err := s.advanceExclusiveLease(ctx, l.ID, false); err != nil {
				s.logger.Warn("exclusive provisioning cleanup waiting", "lease_id", l.ID, "error", err)
			}
			continue
		}
		billingErr := s.syncExclusiveBilling(ctx, l)
		if billingErr != nil {
			s.logger.Warn("exclusive billing sync unavailable", "lease_id", l.ID, "error", billingErr)
		}
		if l.State == "closed" {
			continue
		}
		if err := s.advanceExclusiveLease(ctx, l.ID, billingErr == nil); err != nil {
			s.logger.Warn("exclusive reclamation waiting", "lease_id", l.ID, "error", err)
		}
	}
	_, _ = s.portalDB.ExecContext(ctx, `UPDATE portal_exclusive_channels SET state='available',cooldown_until=NULL,updated_at=CURRENT_TIMESTAMP WHERE state='cooling' AND cooldown_until<=CURRENT_TIMESTAMP AND NOT EXISTS(SELECT 1 FROM portal_exclusive_leases l WHERE l.channel_id=portal_exclusive_channels.id AND l.state<>'closed')`)
	s.syncPendingExclusivePricing(ctx)
}
func (s *Server) advanceExclusiveLease(ctx context.Context, id int64, billingOK bool) error {
	l, err := scanExclusiveLease(s.portalDB.QueryRowContext(ctx, `SELECT `+exclusiveLeaseColumns+` FROM portal_exclusive_leases WHERE id=$1`, id))
	if err != nil {
		return err
	}
	tx, err := s.portalDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	c, err := scanExclusiveChannel(tx.QueryRowContext(ctx, `SELECT `+exclusiveChannelColumns+` FROM portal_exclusive_channels WHERE id=$1 FOR UPDATE`, l.ChannelID))
	if err != nil {
		return err
	}
	l, err = scanExclusiveLease(tx.QueryRowContext(ctx, `SELECT `+exclusiveLeaseColumns+` FROM portal_exclusive_leases WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return err
	}
	if l.State == "provisioning" {
		reason := exclusiveReason(l, time.Now())
		if reason == "" {
			return nil
		}
		if err = s.closeExclusiveProvisioning(ctx, tx, l, reason); err != nil {
			return err
		}
		if err = tx.Commit(); err == nil {
			_ = s.exclusiveEvent(ctx, c.ID, 0, "未完成领取自动回收："+reason)
		}
		return err
	}
	if l.State == "active" {
		reason := exclusiveReason(l, time.Now())
		if reason == "idle" && !billingOK {
			return nil
		}
		if reason == "" {
			return nil
		}
		if _, err = tx.ExecContext(ctx, `UPDATE portal_exclusive_leases SET state='releasing',reason=$1 WHERE id=$2`, reason, id); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE portal_exclusive_channels SET state='releasing',updated_at=CURRENT_TIMESTAMP WHERE id=$1`, c.ID); err != nil {
			return err
		}
		if err = tx.Commit(); err == nil {
			_ = s.exclusiveEvent(ctx, c.ID, 0, "自动回收："+reason)
		}
		return err
	}
	if l.State != "releasing" {
		return nil
	}
	var open int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM portal_exclusive_requests WHERE lease_id=$1 AND state<>'finished'`, id).Scan(&open); err != nil {
		return err
	}
	if open != 0 || !billingOK {
		return nil
	}
	// All admitted HTTP responses reached EOF. Revoke the immutable lease key's group before handoff.
	var unbound struct {
		APIKey coreclient.APIKey `json:"api_key"`
	}
	if l.CoreKeyID == nil {
		return errors.New("exclusive Core key ID missing")
	}
	if err = s.rentalCoreRequest(ctx, "PUT", fmt.Sprintf("/api/v1/admin/api-keys/%d", *l.CoreKeyID), map[string]any{"group_id": 0}, &unbound, ""); err != nil {
		return err
	}
	if unbound.APIKey.ID != *l.CoreKeyID || (unbound.APIKey.GroupID != nil && *unbound.APIKey.GroupID != 0) {
		return errors.New("Core key revocation was not confirmed")
	}
	if _, err = tx.ExecContext(ctx, `UPDATE portal_exclusive_leases SET state='closed',ended_at=CURRENT_TIMESTAMP,error='' WHERE id=$1`, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE portal_exclusive_channels SET state='cooling',cooldown_until=CURRENT_TIMESTAMP+(cooldown_seconds*interval '1 second'),updated_at=CURRENT_TIMESTAMP WHERE id=$1`, c.ID); err != nil {
		return err
	}
	if err = tx.Commit(); err == nil {
		_ = s.exclusiveEvent(ctx, c.ID, 0, "在途请求结束，凭据解绑确认，开始冷却")
	}
	return err
}
