package httpapi

import (
	"context"
	"errors"
	"math"
	"regexp"
	"slices"
	"sort"
	"strings"
)

const exclusiveDefaultImagePrice = 0.05

// Prices cross the Core API in balance units per token, not per million tokens.
type exclusiveModelPrice struct {
	Model          string   `json:"model"`
	InputPrice     *float64 `json:"input_price"`
	OutputPrice    *float64 `json:"output_price"`
	CacheReadPrice *float64 `json:"cache_read_price"`
}

type exclusiveCoreModelPrice struct {
	Platform        string   `json:"platform"`
	Models          []string `json:"models"`
	BillingMode     string   `json:"billing_mode"`
	InputPrice      *float64 `json:"input_price"`
	OutputPrice     *float64 `json:"output_price"`
	CacheReadPrice  *float64 `json:"cache_read_price"`
	PerRequestPrice *float64 `json:"per_request_price"`
}

var exclusiveModelName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)

func validateExclusivePricing(p exclusivePublish) error {
	if p.ImageRateMultiplier != nil && (*p.ImageRateMultiplier <= 0 || math.IsNaN(*p.ImageRateMultiplier) || math.IsInf(*p.ImageRateMultiplier, 0)) {
		return errors.New("图片倍率必须是大于 0 的有效数字")
	}
	valid := func(v *float64) bool { return v == nil || (*v >= 0 && !math.IsNaN(*v) && !math.IsInf(*v, 0)) }
	if !valid(p.ImagePrice) {
		return errors.New("图片单价须为大于等于 0 的有效数字")
	}
	if p.ModelPricing == nil {
		return nil
	}
	if len(*p.ModelPricing) > 100 {
		return errors.New("最多设置 100 个模型的独立价格")
	}
	seen := map[string]bool{}
	for i := range *p.ModelPricing {
		v := &(*p.ModelPricing)[i]
		v.Model = strings.ToLower(strings.TrimSpace(v.Model))
		if !exclusiveModelName.MatchString(v.Model) || strings.HasPrefix(v.Model, "gpt-image") || strings.HasPrefix(v.Model, "dall-e") {
			return errors.New("请输入准确的文本模型名称，不支持通配符；图片模型使用单独的按张定价")
		}
		if seen[v.Model] {
			return errors.New("模型名称不能重复")
		}
		seen[v.Model] = true
		if !valid(v.InputPrice) || !valid(v.OutputPrice) || !valid(v.CacheReadPrice) {
			return errors.New("模型单价须为大于等于 0 的有效数字，留空使用默认价")
		}
	}
	*p.ModelPricing = normalizeExclusivePricing(*p.ModelPricing)
	return nil
}

func normalizeExclusivePricing(values []exclusiveModelPrice) []exclusiveModelPrice {
	result := make([]exclusiveModelPrice, 0, len(values))
	for _, v := range values {
		if v.InputPrice != nil || v.OutputPrice != nil || v.CacheReadPrice != nil {
			result = append(result, v)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Model < result[j].Model })
	return result
}

func exclusiveCorePricing(c exclusiveChannel) []exclusiveCoreModelPrice {
	result := make([]exclusiveCoreModelPrice, 0, len(c.ModelPricing)+1)
	for _, v := range c.ModelPricing {
		result = append(result, exclusiveCoreModelPrice{Platform: "openai", Models: []string{v.Model}, BillingMode: "token", InputPrice: v.InputPrice, OutputPrice: v.OutputPrice, CacheReadPrice: v.CacheReadPrice})
	}
	// Explicit image entries keep generated images out of the token override path.
	return append(result, exclusiveCoreModelPrice{Platform: "openai", Models: []string{"gpt-image-*", "dall-e-*"}, BillingMode: "image", PerRequestPrice: &c.ImagePrice})
}

func exclusivePricingMatches(c exclusiveChannel, g rentalGroup) bool {
	if !g.ImageRateIndependent || g.ImageRateMultiplier != c.ImageRateMultiplier || g.RateMultiplier != c.RateMultiplier {
		return false
	}
	for _, price := range []*float64{g.ImagePrice1K, g.ImagePrice2K, g.ImagePrice4K} {
		if price == nil || *price != c.ImagePrice {
			return false
		}
	}
	return exclusiveJSON(g.ModelPricing) == exclusiveJSON(exclusiveCorePricing(c))
}

func exclusivePricingPayload(c exclusiveChannel) map[string]any {
	return map[string]any{"rate_multiplier": c.RateMultiplier, "model_pricing": exclusiveCorePricing(c), "image_rate_independent": true, "image_rate_multiplier": c.ImageRateMultiplier, "image_price_1k": c.ImagePrice, "image_price_2k": c.ImagePrice, "image_price_4k": c.ImagePrice}
}

// Existing leases retain their current Core configuration. Deferred administrator
// edits are promoted only after the lease drains and the channel is available.
func (s *Server) syncPendingExclusivePricing(ctx context.Context) {
	_, unlock, err := s.exclusiveLock(ctx, 734210091)
	if err != nil {
		return
	}
	defer unlock()
	pendingRows, err := s.portalDB.QueryContext(ctx, `SELECT id FROM portal_exclusive_channels c WHERE pending_config IS NOT NULL AND state IN ('available','maintenance','unpublished') AND NOT EXISTS(SELECT 1 FROM portal_exclusive_leases l WHERE l.channel_id=c.id AND l.state<>'closed') ORDER BY id`)
	if err != nil {
		return
	}
	pendingIDs := []int64{}
	for pendingRows.Next() {
		var id int64
		if err = pendingRows.Scan(&id); err != nil {
			break
		}
		pendingIDs = append(pendingIDs, id)
	}
	pendingRows.Close()
	for _, id := range pendingIDs {
		tx, txErr := s.portalDB.BeginTx(ctx, nil)
		if txErr != nil {
			continue
		}
		c, scanErr := scanExclusiveChannel(tx.QueryRowContext(ctx, `SELECT `+exclusiveChannelColumns+` FROM portal_exclusive_channels WHERE id=$1 FOR UPDATE`, id))
		if scanErr != nil || c.PendingConfig == nil || !slices.Contains([]string{"available", "maintenance", "unpublished"}, c.State) {
			tx.Rollback()
			continue
		}
		var occupied bool
		if scanErr = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM portal_exclusive_leases WHERE channel_id=$1 AND state<>'closed')`, id).Scan(&occupied); scanErr != nil || occupied {
			tx.Rollback()
			continue
		}
		p := c.PendingConfig
		_, txErr = tx.ExecContext(ctx, `UPDATE portal_exclusive_channels SET account_ids=$1::jsonb,rate_multiplier=$2,model_pricing=$3::jsonb,image_price=$4,image_rate_multiplier=$5,rate_resume_state=state,state='provisioning',pricing_revision=pricing_revision+1,pricing_synced=false,pending_config=NULL,updated_at=CURRENT_TIMESTAMP WHERE id=$6`, exclusiveJSON(p.AccountIDs), p.RateMultiplier, exclusiveJSON(p.ModelPricing), p.ImagePrice, p.ImageRateMultiplier, id)
		if txErr == nil {
			txErr = tx.Commit()
		} else {
			tx.Rollback()
		}
		if txErr != nil {
			continue
		}
		_ = s.exclusiveEvent(ctx, id, 0, "待生效配置开始同步")
		if txErr = s.provisionExclusive(ctx, id); txErr != nil {
			s.markExclusiveError(id, txErr)
		}
	}
	rows, err := s.portalDB.QueryContext(ctx, `UPDATE portal_exclusive_channels c SET rate_resume_state=state,state='provisioning',pricing_revision=pricing_revision+1,updated_at=CURRENT_TIMESTAMP WHERE NOT pricing_synced AND core_group_id IS NOT NULL AND state IN ('available','maintenance','unpublished') AND NOT EXISTS(SELECT 1 FROM portal_exclusive_leases l WHERE l.channel_id=c.id AND l.state<>'closed') RETURNING id`)
	if err != nil {
		return
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			break
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		if err := s.provisionExclusive(ctx, id); err != nil {
			s.markExclusiveError(id, err)
		}
	}
}
