package httpapi

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Shares the allocator lock with account-package rental provisioning.
type exclusiveCoreAccount struct {
	rentalAccount
	Credentials struct {
		PlanType string `json:"plan_type"`
	} `json:"credentials"`
}

func exclusiveAccountCanJoinGroup(a rentalAccount) bool {
	return isAPIKeyAccount(a) || len(a.GroupIDs) == 0
}

func exclusiveAccountBoundToGroup(a rentalAccount, groupID int64) bool {
	if !slices.Contains(a.GroupIDs, groupID) {
		return false
	}
	return isAPIKeyAccount(a) || len(a.GroupIDs) == 1
}

func exclusiveAccountBindGroups(a rentalAccount, groupID int64) []int64 {
	if !isAPIKeyAccount(a) {
		return []int64{groupID}
	}
	groups := slices.Clone(a.GroupIDs)
	if !slices.Contains(groups, groupID) {
		groups = append(groups, groupID)
	}
	return groups
}

func exclusiveAccountUnbindGroups(a rentalAccount, groupID int64) []int64 {
	if !isAPIKeyAccount(a) {
		return []int64{}
	}
	groups := make([]int64, 0, len(a.GroupIDs))
	for _, id := range a.GroupIDs {
		if id != groupID {
			groups = append(groups, id)
		}
	}
	return groups
}

func (s *Server) exclusiveLock(ctx context.Context, id int64) (*sql.Conn, func(), error) {
	conn, err := s.portalDB.Conn(ctx)
	if err != nil {
		return nil, nil, err
	}
	var locked bool
	if err = conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, id).Scan(&locked); err != nil || !locked {
		conn.Close()
		if err == nil {
			err = exclusiveConflict("另一个同步正在处理，请稍后重试")
		}
		return nil, nil, err
	}
	return conn, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_unlock($1)`, id); err != nil {
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
		conn.Close()
	}, nil
}
func (s *Server) exclusiveCandidates(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireActiveAdmin(w, r); !ok || !s.exclusiveAvailable(w) {
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	if len(search) > 200 {
		writeAPIError(w, 400, "INVALID_SEARCH", "搜索内容过长")
		return
	}
	q := url.Values{"page": {strconv.Itoa(page)}, "page_size": {"50"}, "platform": {"openai"}, "search": {search}, "lite": {"true"}}
	var data struct {
		Items []exclusiveCoreAccount `json:"items"`
		Total int64                  `json:"total"`
		Pages int                    `json:"pages"`
	}
	if err := s.rentalCoreRequest(r.Context(), "GET", "/api/v1/admin/accounts?"+q.Encode(), nil, &data, ""); err != nil {
		s.exclusiveError(w, err)
		return
	}
	result := []exclusiveAccount{}
	for _, a := range data.Items {
		if validateExclusiveAccount(a.rentalAccount, 0, time.Now()) != nil || !exclusiveAccountCanJoinGroup(a.rentalAccount) {
			continue
		}
		var reserved bool
		err := s.portalDB.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM portal_exclusive_accounts WHERE core_account_id=$1 UNION ALL SELECT 1 FROM portal_account_rental_reservations WHERE core_account_id=$1)`, a.ID).Scan(&reserved)
		if err != nil {
			s.exclusiveError(w, err)
			return
		}
		if reserved {
			continue
		}
		result = append(result, exclusiveAccount{ID: a.ID, Name: a.Name, Platform: a.Platform, Type: a.Type, PlanType: a.Credentials.PlanType, Available: true})
	}
	writeSuccess(w, map[string]any{"items": result, "page": page, "pages": data.Pages, "total": data.Total})
}
func (s *Server) exclusivePublishChannel(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireActiveAdmin(w, r)
	if !ok || !s.exclusiveAvailable(w) {
		return
	}
	var p exclusivePublish
	if err := decodeJSON(r, &p); err != nil {
		writeAPIError(w, 400, "INVALID_CHANNEL", "通道配置无效")
		return
	}
	if err := validateExclusiveConfig(p, true); err != nil {
		writeAPIError(w, 400, "INVALID_CHANNEL", err.Error())
		return
	}
	conn, unlock, err := s.exclusiveLock(r.Context(), 734210091)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	defer unlock()
	for _, id := range p.AccountIDs {
		var a rentalAccount
		if err = s.rentalCoreRequest(r.Context(), "GET", fmt.Sprintf("/api/v1/admin/accounts/%d", id), nil, &a, ""); err != nil {
			s.exclusiveError(w, err)
			return
		}
		if a.ID != id || validateExclusiveAccount(a, 0, time.Now()) != nil || !exclusiveAccountCanJoinGroup(a) {
			s.exclusiveError(w, exclusiveConflict("所选账号已绑定或不可用"))
			return
		}
	}
	tx, err := conn.BeginTx(r.Context(), nil)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	defer tx.Rollback()
	var id int64
	rate := 1.0
	if p.RateMultiplier != nil {
		rate = *p.RateMultiplier
	}
	prices := []exclusiveModelPrice{}
	if p.ModelPricing != nil {
		prices = *p.ModelPricing
	}
	imagePrice := exclusiveDefaultImagePrice
	imageRate := 1.0
	if p.ImageRateMultiplier != nil {
		imageRate = *p.ImageRateMultiplier
	}
	if p.ImagePrice != nil {
		imagePrice = *p.ImagePrice
	}
	err = tx.QueryRowContext(r.Context(), `INSERT INTO portal_exclusive_channels(name,state,account_ids,durations,cooldown_seconds,rate_multiplier,model_pricing,image_price,image_rate_multiplier) VALUES($1,'provisioning',$2::jsonb,$3::jsonb,$4,$5,$6::jsonb,$7,$8) RETURNING id`, strings.TrimSpace(p.Name), exclusiveJSON(p.AccountIDs), exclusiveJSON(p.Durations), p.CooldownSeconds, rate, exclusiveJSON(prices), imagePrice, imageRate).Scan(&id)
	if err == nil {
		for _, aid := range p.AccountIDs {
			var busy bool
			err = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM portal_account_rental_reservations WHERE core_account_id=$1)`, aid).Scan(&busy)
			if err != nil {
				break
			}
			if busy {
				err = exclusiveConflict("所选账号已被账号套餐预留")
				break
			}
			_, err = tx.ExecContext(r.Context(), `INSERT INTO portal_exclusive_accounts(core_account_id,channel_id) VALUES($1,$2)`, aid, id)
			if err != nil {
				err = exclusiveConflict("所选账号已被其他通道预留")
				break
			}
		}
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	_ = s.exclusiveEvent(r.Context(), id, admin.ID, "发布通道：已预留账号")
	if err = s.provisionExclusive(r.Context(), id); err != nil {
		s.markExclusiveError(id, err)
		writeSuccess(w, map[string]any{"id": id, "state": "error", "error": "账号已预留，Core 同步未完成，请重试同步"})
		return
	}
	writeSuccess(w, map[string]any{"id": id, "state": "available"})
}
func (s *Server) markExclusiveError(id int64, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.logger.Error("exclusive provisioning failed", "channel_id", id, "error", err)
	_, _ = s.portalDB.ExecContext(ctx, `UPDATE portal_exclusive_channels SET state='error',error='Core 配置同步未完成，请重试同步',updated_at=CURRENT_TIMESTAMP WHERE id=$1 AND state IN ('provisioning','error')`, id)
}
func (s *Server) provisionExclusive(ctx context.Context, id int64) error {
	c, err := scanExclusiveChannel(s.portalDB.QueryRowContext(ctx, `SELECT `+exclusiveChannelColumns+` FROM portal_exclusive_channels WHERE id=$1`, id))
	if err != nil {
		return err
	}
	if c.State != "provisioning" && c.State != "error" {
		return exclusiveConflict("通道已完成发布")
	}
	name := exclusiveGroupName(id)
	description := "Portal exclusive channel " + strconv.FormatInt(id, 10)
	var g rentalGroup
	if c.GroupID != nil {
		err = s.rentalCoreRequest(ctx, "GET", fmt.Sprintf("/api/v1/admin/groups/%d", *c.GroupID), nil, &g, "")
	} else {
		var groups []rentalGroup
		err = s.rentalCoreRequest(ctx, "GET", "/api/v1/admin/groups/all?include_inactive=true", nil, &groups, "")
		if err != nil {
			return err
		}
		for _, candidate := range groups {
			if candidate.Name == name {
				if g.ID != 0 {
					return exclusiveConflict("发现重复的自动分组，请管理员核查")
				}
				g = candidate
			}
		}
		if g.ID == 0 {
			payload := exclusivePricingPayload(c)
			payload["name"], payload["description"], payload["platform"] = name, description, "openai"
			payload["is_exclusive"], payload["subscription_type"] = true, "standard"
			payload["allow_image_generation"], payload["allow_batch_image_generation"] = true, false
			err = s.rentalCoreRequest(ctx, "POST", "/api/v1/admin/groups", payload, &g, fmt.Sprintf("exclusive-group-%d", id))
		}
	}
	if err != nil {
		return err
	}
	if g.ID <= 0 || g.Name != name || g.Description != description || g.Platform != "openai" || !g.IsExclusive || g.SubscriptionType != "standard" || g.FallbackGroupID != nil || g.FallbackInvalidGroupID != nil {
		return exclusiveConflict("Core 分组不符合独享标准分组配置")
	}
	if _, err = s.portalDB.ExecContext(ctx, `UPDATE portal_exclusive_channels SET core_group_id=$1 WHERE id=$2`, g.ID, id); err != nil {
		return err
	}
	if !exclusivePricingMatches(c, g) {
		var updated rentalGroup
		if err = s.rentalCoreRequest(ctx, "PUT", fmt.Sprintf("/api/v1/admin/groups/%d", g.ID), exclusivePricingPayload(c), &updated, ""); err != nil {
			return err
		}
		if updated.ID != g.ID || !exclusivePricingMatches(c, updated) {
			return exclusiveConflict("Core 分组定价同步确认失败")
		}
	}
	var grouped struct {
		Items []rentalAccount `json:"items"`
	}
	if err = s.rentalCoreRequest(ctx, "GET", fmt.Sprintf("/api/v1/admin/accounts?group=%d&page=1&page_size=100&lite=true", g.ID), nil, &grouped, ""); err != nil {
		return err
	}
	for _, a := range grouped.Items {
		if slices.Contains(c.AccountIDs, a.ID) {
			continue
		}
		if !exclusiveAccountBoundToGroup(a, g.ID) {
			return exclusiveConflict("Core 分组包含无法自动解绑的账号，请先在 Core 核查")
		}
		groups := exclusiveAccountUnbindGroups(a, g.ID)
		var cleared rentalAccount
		if err = s.rentalCoreRequest(ctx, "PUT", fmt.Sprintf("/api/v1/admin/accounts/%d", a.ID), map[string]any{"group_ids": groups}, &cleared, fmt.Sprintf("exclusive-unbind-%d-%d", id, a.ID)); err != nil {
			return err
		}
		if cleared.ID != a.ID || !sameRentalSelection(cleared.GroupIDs, groups) {
			return exclusiveConflict("账号解绑确认失败")
		}
	}
	for _, aid := range c.AccountIDs {
		var a rentalAccount
		if err = s.rentalCoreRequest(ctx, "GET", fmt.Sprintf("/api/v1/admin/accounts/%d", aid), nil, &a, ""); err != nil {
			return err
		}
		if a.ID != aid || validateExclusiveAccount(a, g.ID, time.Now()) != nil {
			return exclusiveConflict("账号已被修改或不可用，请管理员核查")
		}
		groups := exclusiveAccountBindGroups(a, g.ID)
		if err = s.rentalCoreRequest(ctx, "PUT", fmt.Sprintf("/api/v1/admin/accounts/%d", aid), map[string]any{"group_ids": groups}, &a, fmt.Sprintf("exclusive-bind-%d-%d", id, aid)); err != nil {
			return err
		}
		if a.ID != aid || !sameRentalSelection(a.GroupIDs, groups) || !exclusiveAccountBoundToGroup(a, g.ID) {
			return exclusiveConflict("账号分组绑定确认失败")
		}
	}
	if err = s.verifyExclusiveGroup(ctx, c, g.ID); err != nil {
		return err
	}
	if _, err = s.portalDB.ExecContext(ctx, `DELETE FROM portal_exclusive_accounts WHERE channel_id=$1 AND NOT EXISTS (SELECT 1 FROM jsonb_array_elements_text($2::jsonb) item WHERE item::bigint=core_account_id)`, id, exclusiveJSON(c.AccountIDs)); err != nil {
		return err
	}
	_, err = s.portalDB.ExecContext(ctx, `UPDATE portal_exclusive_channels SET state=COALESCE(rate_resume_state,'available'),rate_resume_state=NULL,pricing_synced=true,error='',updated_at=CURRENT_TIMESTAMP WHERE id=$1 AND state IN ('provisioning','error')`, id)
	if err == nil {
		_ = s.exclusiveEvent(ctx, id, 0, fmt.Sprintf("Core 分组同步完成，通道倍率 %g，图片单价 %g，%d 个文本模型独立定价", c.RateMultiplier, c.ImagePrice, len(c.ModelPricing)))
	}
	return err
}
func (s *Server) verifyExclusiveGroup(ctx context.Context, c exclusiveChannel, groupID int64) error {
	var data struct {
		Items []rentalAccount `json:"items"`
		Total int             `json:"total"`
	}
	if err := s.rentalCoreRequest(ctx, "GET", fmt.Sprintf("/api/v1/admin/accounts?group=%d&page=1&page_size=100&lite=true", groupID), nil, &data, ""); err != nil {
		return err
	}
	if len(data.Items) != len(c.AccountIDs) || data.Total != len(c.AccountIDs) {
		return exclusiveConflict("Core 分组账号数量不一致")
	}
	seen := map[int64]bool{}
	for _, a := range data.Items {
		if !slices.Contains(c.AccountIDs, a.ID) || seen[a.ID] || !exclusiveAccountBoundToGroup(a, groupID) {
			return exclusiveConflict("Core 分组包含意外账号或共享绑定")
		}
		seen[a.ID] = true
	}
	return nil
}
func (s *Server) exclusiveRetryChannel(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireActiveAdmin(w, r); !ok || !s.exclusiveAvailable(w) {
		return
	}
	id, ok := pathID(w, r, "channelID")
	if !ok {
		return
	}
	_, unlock, err := s.exclusiveLock(r.Context(), 734210091)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	defer unlock()
	if err = s.provisionExclusive(r.Context(), id); err != nil {
		s.markExclusiveError(id, err)
		s.exclusiveError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"id": id})
}
func (s *Server) exclusiveUpdateChannel(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireActiveAdmin(w, r)
	if !ok || !s.exclusiveAvailable(w) {
		return
	}
	id, ok := pathID(w, r, "channelID")
	if !ok {
		return
	}
	var p exclusivePublish
	if err := decodeJSON(r, &p); err != nil {
		writeAPIError(w, 400, "INVALID_CHANNEL", "通道配置无效")
		return
	}
	if err := validateExclusiveConfig(p, false); err != nil {
		writeAPIError(w, 400, "INVALID_CHANNEL", err.Error())
		return
	}
	conn, unlock, err := s.exclusiveLock(r.Context(), 734210091)
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
	desired := exclusivePendingConfig{AccountIDs: c.AccountIDs, RateMultiplier: c.RateMultiplier, ModelPricing: c.ModelPricing, ImagePrice: c.ImagePrice, ImageRateMultiplier: c.ImageRateMultiplier}
	if c.PendingConfig != nil {
		desired = *c.PendingConfig
	}
	configTouched := p.AccountIDs != nil || p.RateMultiplier != nil || p.ModelPricing != nil || p.ImagePrice != nil || p.ImageRateMultiplier != nil
	if p.AccountIDs != nil {
		desired.AccountIDs = p.AccountIDs
	}
	if p.RateMultiplier != nil {
		desired.RateMultiplier = *p.RateMultiplier
	}
	if p.ImageRateMultiplier != nil {
		desired.ImageRateMultiplier = *p.ImageRateMultiplier
	}
	if p.ModelPricing != nil {
		desired.ModelPricing = *p.ModelPricing
	}
	if p.ImagePrice != nil {
		desired.ImagePrice = *p.ImagePrice
	}
	accountChanged := configTouched && !sameRentalSelection(desired.AccountIDs, c.AccountIDs)
	pricingChanged := configTouched && (desired.RateMultiplier != c.RateMultiplier || desired.ImagePrice != c.ImagePrice || desired.ImageRateMultiplier != c.ImageRateMultiplier || exclusiveJSON(desired.ModelPricing) != exclusiveJSON(c.ModelPricing))
	changed := accountChanged || pricingChanged
	var occupied bool
	if err = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM portal_exclusive_leases WHERE channel_id=$1 AND state<>'closed')`, id).Scan(&occupied); err != nil {
		s.exclusiveError(w, err)
		return
	}
	applyNow := !occupied && slices.Contains([]string{"available", "maintenance", "unpublished"}, c.State)
	deferConfig := occupied || slices.Contains([]string{"active", "releasing", "cooling"}, c.State)
	if configTouched && !applyNow && !deferConfig && (changed || c.PendingConfig != nil) {
		s.exclusiveError(w, exclusiveConflict("Core 配置正在同步，请同步完成后再修改账号或定价"))
		return
	}
	if accountChanged {
		if c.GroupID == nil {
			s.exclusiveError(w, exclusiveConflict("Core 分组尚未同步完成，暂不能修改绑定账号"))
			return
		}
		for _, aid := range desired.AccountIDs {
			var a rentalAccount
			if err = s.rentalCoreRequest(r.Context(), "GET", fmt.Sprintf("/api/v1/admin/accounts/%d", aid), nil, &a, ""); err != nil {
				s.exclusiveError(w, err)
				return
			}
			if a.ID != aid {
				s.exclusiveError(w, exclusiveConflict("账号不存在或已被删除"))
				return
			}
			if slices.Contains(c.AccountIDs, aid) {
				if validateExclusiveAccount(a, *c.GroupID, time.Now()) != nil || !exclusiveAccountBoundToGroup(a, *c.GroupID) {
					s.exclusiveError(w, exclusiveConflict("已有账号已不可用，请先在 Core 核查"))
					return
				}
				continue
			}
			if validateExclusiveAccount(a, 0, time.Now()) != nil || !exclusiveAccountCanJoinGroup(a) {
				s.exclusiveError(w, exclusiveConflict("新增账号已绑定或不可用"))
				return
			}
			var owner int64
			ownerErr := tx.QueryRowContext(r.Context(), `SELECT channel_id FROM portal_exclusive_accounts WHERE core_account_id=$1`, aid).Scan(&owner)
			if ownerErr != nil && !errors.Is(ownerErr, sql.ErrNoRows) {
				s.exclusiveError(w, ownerErr)
				return
			}
			if ownerErr == nil && owner != id {
				s.exclusiveError(w, exclusiveConflict("新增账号已被其他通道预留"))
				return
			}
			var reserved bool
			if err = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM portal_account_rental_reservations WHERE core_account_id=$1)`, aid).Scan(&reserved); err != nil {
				s.exclusiveError(w, err)
				return
			}
			if reserved {
				s.exclusiveError(w, exclusiveConflict("新增账号已被账号套餐预留"))
				return
			}
		}
	}
	if configTouched && deferConfig {
		if changed {
			for _, aid := range desired.AccountIDs {
				if slices.Contains(c.AccountIDs, aid) {
					continue
				}
				if _, err = tx.ExecContext(r.Context(), `INSERT INTO portal_exclusive_accounts(core_account_id,channel_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, aid, id); err != nil {
					s.exclusiveError(w, exclusiveConflict("新增账号预留失败，请重试"))
					return
				}
			}
			_, err = tx.ExecContext(r.Context(), `UPDATE portal_exclusive_channels SET name=$1,durations=$2::jsonb,cooldown_seconds=$3,pending_config=$4::jsonb,updated_at=CURRENT_TIMESTAMP WHERE id=$5`, strings.TrimSpace(p.Name), exclusiveJSON(p.Durations), p.CooldownSeconds, exclusiveJSON(desired), id)
		} else {
			_, err = tx.ExecContext(r.Context(), `UPDATE portal_exclusive_channels SET name=$1,durations=$2::jsonb,cooldown_seconds=$3,pending_config=NULL,updated_at=CURRENT_TIMESTAMP WHERE id=$4`, strings.TrimSpace(p.Name), exclusiveJSON(p.Durations), p.CooldownSeconds, id)
		}
		if err == nil {
			_, err = tx.ExecContext(r.Context(), `DELETE FROM portal_exclusive_accounts WHERE channel_id=$1 AND NOT (core_account_id=ANY($2) OR core_account_id=ANY($3))`, id, c.AccountIDs, desired.AccountIDs)
		}
		if err != nil {
			s.exclusiveError(w, err)
			return
		}
		if err = tx.Commit(); err != nil {
			s.exclusiveError(w, err)
			return
		}
		if changed {
			_ = s.exclusiveEvent(r.Context(), id, admin.ID, "保存待生效配置：当前使用结束后自动同步")
		} else if c.PendingConfig != nil {
			_ = s.exclusiveEvent(r.Context(), id, admin.ID, "取消待生效配置")
		}
		_ = s.exclusiveEvent(r.Context(), id, admin.ID, "更新通道配置")
		writeSuccess(w, map[string]any{"id": id, "pending": changed})
		return
	}
	_, err = tx.ExecContext(r.Context(), `UPDATE portal_exclusive_channels SET name=$1,account_ids=$2::jsonb,durations=$3::jsonb,cooldown_seconds=$4,rate_multiplier=$6,rate_resume_state=CASE WHEN $7 THEN state ELSE rate_resume_state END,state=CASE WHEN $7 THEN 'provisioning' ELSE state END,model_pricing=$8::jsonb,image_price=$9,image_rate_multiplier=$10,pricing_revision=pricing_revision+CASE WHEN $7 THEN 1 ELSE 0 END,pricing_synced=CASE WHEN $7 THEN false ELSE pricing_synced END,pending_config=NULL,updated_at=CURRENT_TIMESTAMP WHERE id=$5`, strings.TrimSpace(p.Name), exclusiveJSON(desired.AccountIDs), exclusiveJSON(p.Durations), p.CooldownSeconds, id, desired.RateMultiplier, changed, exclusiveJSON(desired.ModelPricing), desired.ImagePrice, desired.ImageRateMultiplier)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	if accountChanged {
		for _, aid := range desired.AccountIDs {
			if slices.Contains(c.AccountIDs, aid) {
				continue
			}
			if _, err = tx.ExecContext(r.Context(), `INSERT INTO portal_exclusive_accounts(core_account_id,channel_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, aid, id); err != nil {
				s.exclusiveError(w, exclusiveConflict("新增账号预留失败，请重试"))
				return
			}
		}
	}
	if err = tx.Commit(); err != nil {
		s.exclusiveError(w, err)
		return
	}
	if changed {
		if pricingChanged {
			_ = s.exclusiveEvent(r.Context(), id, admin.ID, fmt.Sprintf("修改通道定价：倍率 %g → %g，图片 %g → %g，图片倍率 %g → %g，文本模型 %s，等待 Core 同步", c.RateMultiplier, desired.RateMultiplier, c.ImagePrice, desired.ImagePrice, c.ImageRateMultiplier, desired.ImageRateMultiplier, exclusiveJSON(desired.ModelPricing)))
		}
		if accountChanged {
			_ = s.exclusiveEvent(r.Context(), id, admin.ID, fmt.Sprintf("调整绑定账号：%d 个，等待 Core 同步", len(desired.AccountIDs)))
		}
		if err = s.provisionExclusive(r.Context(), id); err != nil {
			s.markExclusiveError(id, err)
			s.exclusiveError(w, err)
			return
		}
	}
	_ = s.exclusiveEvent(r.Context(), id, admin.ID, "更新通道配置")
	writeSuccess(w, map[string]any{"id": id})
}

func (s *Server) exclusiveDeleteChannel(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireActiveAdmin(w, r)
	if !ok || !s.exclusiveAvailable(w) {
		return
	}
	id, ok := pathID(w, r, "channelID")
	if !ok {
		return
	}
	conn, unlock, err := s.exclusiveLock(r.Context(), 734210091)
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
	var occupied bool
	if err = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM portal_exclusive_leases WHERE channel_id=$1 AND state<>'closed')`, id).Scan(&occupied); err != nil {
		s.exclusiveError(w, err)
		return
	}
	if occupied || !slices.Contains([]string{"available", "maintenance", "unpublished", "error"}, c.State) {
		s.exclusiveError(w, exclusiveConflict("占用、回收、冷却或同步期间不能删除通道"))
		return
	}
	if c.GroupID != nil {
		var grouped struct {
			Items []rentalAccount `json:"items"`
		}
		if err = s.rentalCoreRequest(r.Context(), "GET", fmt.Sprintf("/api/v1/admin/accounts?group=%d&page=1&page_size=100&lite=true", *c.GroupID), nil, &grouped, ""); err != nil {
			s.exclusiveError(w, err)
			return
		}
		for _, a := range grouped.Items {
			if !slices.Contains(c.AccountIDs, a.ID) || !exclusiveAccountBoundToGroup(a, *c.GroupID) {
				s.exclusiveError(w, exclusiveConflict("Core 分组包含意外账号，拒绝自动删除"))
				return
			}
			groups := exclusiveAccountUnbindGroups(a, *c.GroupID)
			var cleared rentalAccount
			if err = s.rentalCoreRequest(r.Context(), "PUT", fmt.Sprintf("/api/v1/admin/accounts/%d", a.ID), map[string]any{"group_ids": groups}, &cleared, fmt.Sprintf("exclusive-delete-unbind-%d-%d", id, a.ID)); err != nil || cleared.ID != a.ID || !sameRentalSelection(cleared.GroupIDs, groups) {
				if err == nil {
					err = exclusiveConflict("删除前账号解绑确认失败")
				}
				s.exclusiveError(w, err)
				return
			}
		}
		if err = s.rentalCoreRequest(r.Context(), "DELETE", fmt.Sprintf("/api/v1/admin/groups/%d", *c.GroupID), nil, nil, fmt.Sprintf("exclusive-delete-group-%d", id)); err != nil {
			s.exclusiveError(w, err)
			return
		}
	}
	for _, query := range []string{
		`DELETE FROM portal_exclusive_requests WHERE lease_id IN (SELECT id FROM portal_exclusive_leases WHERE channel_id=$1)`,
		`DELETE FROM portal_exclusive_leases WHERE channel_id=$1`,
		`DELETE FROM portal_exclusive_events WHERE channel_id=$1`,
		`DELETE FROM portal_exclusive_accounts WHERE channel_id=$1`,
		`DELETE FROM portal_exclusive_channels WHERE id=$1`,
	} {
		if _, err = tx.ExecContext(r.Context(), query, id); err != nil {
			s.exclusiveError(w, err)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		s.exclusiveError(w, err)
		return
	}
	s.logger.Info("exclusive channel deleted", "channel_id", id, "admin_id", admin.ID, "core_group_id", c.GroupID)
	writeSuccess(w, map[string]any{"id": id})
}
func (s *Server) exclusiveAvailability(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireActiveAdmin(w, r)
	if !ok || !s.exclusiveAvailable(w) {
		return
	}
	id, ok := pathID(w, r, "channelID")
	if !ok {
		return
	}
	var p struct {
		State string `json:"state"`
	}
	if decodeJSON(r, &p) != nil || !slices.Contains([]string{"available", "maintenance", "unpublished"}, p.State) {
		writeAPIError(w, 400, "INVALID_STATE", "发布状态无效")
		return
	}
	result, err := s.portalDB.ExecContext(r.Context(), `UPDATE portal_exclusive_channels SET state=$1,updated_at=CURRENT_TIMESTAMP WHERE id=$2 AND state IN ('available','maintenance','unpublished') AND NOT EXISTS(SELECT 1 FROM portal_exclusive_leases WHERE channel_id=$2 AND state<>'closed')`, p.State, id)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		s.exclusiveError(w, exclusiveConflict("占用、同步或冷却期间不能更改发布状态"))
		return
	}
	_ = s.exclusiveEvent(r.Context(), id, admin.ID, "发布状态："+p.State)
	writeSuccess(w, map[string]any{"id": id})
}
func remainingExclusive(v *float64) *float64 {
	if v == nil || math.IsNaN(*v) || math.IsInf(*v, 0) || *v < 0 || *v > 100 {
		return nil
	}
	r := 100 - *v
	return &r
}
func (s *Server) exclusiveAccountView(ctx context.Context, id int64) exclusiveAccount {
	return s.exclusiveAccountUsage(ctx, id, false)
}
func (s *Server) exclusiveAccountUsage(ctx context.Context, id int64, force bool) exclusiveAccount {
	out := exclusiveAccount{ID: id, Name: fmt.Sprintf("账号 #%d", id)}
	var a exclusiveCoreAccount
	if err := s.rentalCoreRequest(ctx, "GET", fmt.Sprintf("/api/v1/admin/accounts/%d", id), nil, &a, ""); err != nil {
		out.Error = "账号状态暂不可用"
		return out
	}
	out.Name = a.Name
	out.Platform = a.Platform
	out.Type = a.Type
	out.PlanType = a.Credentials.PlanType
	isAPIKey := strings.EqualFold(strings.TrimSpace(a.Type), "apikey")
	out.Available = a.Status == "active" && a.Schedulable && a.ParentAccountID == nil && (isAPIKey || a.ExpiresAt == nil || *a.ExpiresAt > time.Now().Unix())
	if isAPIKey {
		return out
	}
	var usage struct {
		UpdatedAt *time.Time `json:"updated_at"`
		FiveHour  *struct {
			Utilization *float64   `json:"utilization"`
			ResetsAt    *time.Time `json:"resets_at"`
		} `json:"five_hour"`
		SevenDay *struct {
			Utilization *float64   `json:"utilization"`
			ResetsAt    *time.Time `json:"resets_at"`
		} `json:"seven_day"`
	}
	path := fmt.Sprintf("/api/v1/admin/accounts/%d/usage?source=active", id)
	if force {
		path += "&force=true"
	}
	if err := s.rentalCoreRequest(ctx, "GET", path, nil, &usage, ""); err != nil {
		out.Error = "额度数据暂不可用"
		return out
	}
	if usage.FiveHour != nil {
		out.FiveHourRemaining = remainingExclusive(usage.FiveHour.Utilization)
		out.FiveHourResetsAt = usage.FiveHour.ResetsAt
	}
	if usage.SevenDay != nil {
		out.WeeklyRemaining = remainingExclusive(usage.SevenDay.Utilization)
		out.WeeklyResetsAt = usage.SevenDay.ResetsAt
	}
	out.ObservedAt = usage.UpdatedAt
	if out.FiveHourRemaining == nil || out.WeeklyRemaining == nil {
		out.Error = "部分额度尚无数据"
	}
	if (out.FiveHourRemaining != nil && *out.FiveHourRemaining <= 0) || (out.WeeklyRemaining != nil && *out.WeeklyRemaining <= 0) {
		out.Available = false
	}
	return out
}
