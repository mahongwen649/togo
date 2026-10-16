package httpapi

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

const exclusiveIdle = 30 * time.Minute
const exclusiveUpstreamPrefix = "pxu_"
const exclusiveTokenPrefix = "pxc_"

// Isolation must be established at deployment, not inferred from a monitoring API.
type ExclusiveConfig struct {
	Enabled         bool
	IngressIsolated bool
	PublicURL       string
	Secret          string
	Context         context.Context
	DisableWorker   bool
}

func (c ExclusiveConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	u, err := url.Parse(c.PublicURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return errors.New("exclusive public URL must be an absolute HTTP(S) origin")
	}
	if !c.IngressIsolated || len(c.Secret) < 32 {
		return errors.New("exclusive channels require isolated ingress and a secret of at least 32 bytes")
	}
	return nil
}

type exclusiveChannel struct {
	ImageRateMultiplier float64                 `json:"image_rate_multiplier"`
	ModelPricing        []exclusiveModelPrice   `json:"model_pricing"`
	ImagePrice          float64                 `json:"image_price"`
	PricingRevision     int64                   `json:"pricing_revision"`
	PricingSynced       bool                    `json:"pricing_synced"`
	RateMultiplier      float64                 `json:"rate_multiplier"`
	RateResumeState     *string                 `json:"-"`
	PageTest            exclusivePageTest       `json:"page_test"`
	ID                  int64                   `json:"id"`
	Name                string                  `json:"name"`
	State               string                  `json:"state"`
	GroupID             *int64                  `json:"core_group_id,omitempty"`
	AccountIDs          []int64                 `json:"-"`
	Durations           []int                   `json:"durations"`
	CooldownSeconds     int                     `json:"cooldown_seconds"`
	CooldownUntil       *time.Time              `json:"cooldown_until"`
	Error               string                  `json:"error,omitempty"`
	Accounts            []exclusiveAccount      `json:"accounts"`
	PendingAccounts     []exclusiveAccount      `json:"pending_accounts,omitempty"`
	PendingConfig       *exclusivePendingConfig `json:"pending_config,omitempty"`
	Lease               *exclusiveLease         `json:"lease,omitempty"`
}
type exclusivePendingConfig struct {
	AccountIDs          []int64               `json:"account_ids"`
	RateMultiplier      float64               `json:"rate_multiplier"`
	ModelPricing        []exclusiveModelPrice `json:"model_pricing"`
	ImagePrice          float64               `json:"image_price"`
	ImageRateMultiplier float64               `json:"image_rate_multiplier"`
}
type exclusiveAccount struct {
	PageTest          exclusivePageTest `json:"page_test"`
	FiveHourResetsAt  *time.Time        `json:"five_hour_resets_at"`
	WeeklyResetsAt    *time.Time        `json:"weekly_resets_at"`
	Platform          string            `json:"platform"`
	Type              string            `json:"type"`
	PlanType          string            `json:"plan_type,omitempty"`
	ID                int64             `json:"id"`
	Name              string            `json:"name"`
	Available         bool              `json:"available"`
	FiveHourRemaining *float64          `json:"five_hour_remaining"`
	WeeklyRemaining   *float64          `json:"weekly_remaining"`
	ObservedAt        *time.Time        `json:"observed_at"`
	Error             string            `json:"error,omitempty"`
}
type exclusiveLease struct {
	ID               int64      `json:"id"`
	ChannelID        int64      `json:"channel_id"`
	UserID           int64      `json:"user_id,omitempty"`
	State            string     `json:"state"`
	CoreKeyID        *int64     `json:"-"`
	TokenSecret      string     `json:"-"`
	UpstreamSecret   string     `json:"-"`
	StartedAt        time.Time  `json:"started_at"`
	ExpiresAt        time.Time  `json:"expires_at"`
	LastChargeAt     *time.Time `json:"last_charge_at"`
	BillingCheckedAt *time.Time `json:"billing_checked_at"`
	EndedAt          *time.Time `json:"ended_at"`
	Reason           string     `json:"reason"`
	Cost             float64    `json:"cost"`
	Requests         int64      `json:"requests"`
	Tokens           int64      `json:"tokens"`
	Error            string     `json:"error,omitempty"`
	Inflight         int        `json:"inflight"`
	Uncertain        int        `json:"uncertain"`
	Mine             bool       `json:"mine"`
	Key              string     `json:"key,omitempty"`
	BaseURL          string     `json:"base_url,omitempty"`
	ChannelName      string     `json:"channel_name,omitempty"`
}
type exclusivePublish struct {
	ImageRateMultiplier *float64               `json:"image_rate_multiplier"`
	ModelPricing        *[]exclusiveModelPrice `json:"model_pricing"`
	ImagePrice          *float64               `json:"image_price"`
	RateMultiplier      *float64               `json:"rate_multiplier"`
	Name                string                 `json:"name"`
	AccountIDs          []int64                `json:"account_ids"`
	Durations           []int                  `json:"durations"`
	CooldownSeconds     int                    `json:"cooldown_seconds"`
}

var exclusiveDurations = []int{30, 60, 120, 240, 480, 720, 1440}

func validateExclusiveConfig(p exclusivePublish, publishing bool) error {
	if err := validateExclusivePricing(p); err != nil {
		return err
	}
	if p.RateMultiplier != nil && (*p.RateMultiplier <= 0 || math.IsNaN(*p.RateMultiplier) || math.IsInf(*p.RateMultiplier, 0)) {
		return errors.New("通道倍率必须是大于 0 的有效数字")
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(p.Name)); n < 1 || n > 40 {
		return errors.New("通道名称须为 1 到 40 个字符")
	}
	if p.CooldownSeconds < 0 || p.CooldownSeconds > 86400 {
		return errors.New("冷却时间须为 0 到 86400 秒")
	}
	if !slices.Contains(p.Durations, 1440) || len(p.Durations) > len(exclusiveDurations) {
		return errors.New("可选时长须包含默认的 24 小时")
	}
	seen := map[int]bool{}
	for _, d := range p.Durations {
		if !slices.Contains(exclusiveDurations, d) || seen[d] {
			return errors.New("可选时长无效或重复")
		}
		seen[d] = true
	}
	if publishing || p.AccountIDs != nil {
		if len(p.AccountIDs) < 1 || len(p.AccountIDs) > 50 {
			return errors.New("请选择 1 到 50 个可用账号")
		}
		ids := map[int64]bool{}
		for _, id := range p.AccountIDs {
			if id <= 0 || ids[id] {
				return errors.New("账号无效或重复")
			}
			ids[id] = true
		}
	}
	return nil
}
func exclusiveDeadline(l exclusiveLease) time.Time {
	last := l.StartedAt
	if l.LastChargeAt != nil && l.LastChargeAt.After(last) {
		last = *l.LastChargeAt
	}
	deadline := last.Add(exclusiveIdle)
	if l.ExpiresAt.Before(deadline) {
		deadline = l.ExpiresAt
	}
	return deadline
}
func exclusiveReason(l exclusiveLease, now time.Time) string {
	if !now.Before(l.ExpiresAt) {
		return "expired"
	}
	if !now.Before(exclusiveDeadline(l)) {
		return "idle"
	}
	return ""
}
func exclusiveRandom(prefix string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(b), nil
}
func exclusiveHash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func (s *Server) exclusiveAEAD() (cipher.AEAD, error) {
	h := sha256.Sum256([]byte(s.exclusive.Secret))
	block, err := aes.NewCipher(h[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func (s *Server) sealExclusive(value string) (string, error) {
	a, err := s.exclusiveAEAD()
	if err != nil {
		return "", err
	}
	n := make([]byte, a.NonceSize())
	if _, err = rand.Read(n); err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(a.Seal(n, n, []byte(value), []byte("portal-exclusive-v1"))), nil
}
func (s *Server) openExclusive(value string) (string, error) {
	a, err := s.exclusiveAEAD()
	if err != nil {
		return "", err
	}
	b, err := base64.RawStdEncoding.DecodeString(value)
	if err != nil || len(b) < a.NonceSize() {
		return "", errors.New("invalid encrypted exclusive credential")
	}
	p, err := a.Open(nil, b[:a.NonceSize()], b[a.NonceSize():], []byte("portal-exclusive-v1"))
	return string(p), err
}

type exclusiveRow interface{ Scan(...any) error }

const exclusiveChannelColumns = `id,name,state,core_group_id,account_ids,durations,cooldown_seconds,cooldown_until,error,rate_multiplier,rate_resume_state,model_pricing,image_price,pricing_revision,pricing_synced,image_rate_multiplier,pending_config`

func scanExclusiveChannel(row exclusiveRow) (exclusiveChannel, error) {
	var c exclusiveChannel
	var ids, ds, pricing []byte
	var pending []byte
	err := row.Scan(&c.ID, &c.Name, &c.State, &c.GroupID, &ids, &ds, &c.CooldownSeconds, &c.CooldownUntil, &c.Error, &c.RateMultiplier, &c.RateResumeState, &pricing, &c.ImagePrice, &c.PricingRevision, &c.PricingSynced, &c.ImageRateMultiplier, &pending)
	if err == nil {
		err = json.Unmarshal(ids, &c.AccountIDs)
	}
	if err == nil {
		err = json.Unmarshal(ds, &c.Durations)
	}
	if err == nil {
		err = json.Unmarshal(pricing, &c.ModelPricing)
	}
	if err == nil && len(pending) > 0 {
		c.PendingConfig = &exclusivePendingConfig{}
		err = json.Unmarshal(pending, c.PendingConfig)
	}
	c.Accounts = []exclusiveAccount{}
	c.PendingAccounts = []exclusiveAccount{}
	return c, err
}

const exclusiveLeaseColumns = `id,channel_id,user_id,state,core_key_id,token_secret,upstream_secret,started_at,expires_at,last_charge_at,billing_checked_at,ended_at,reason,cost,requests,tokens,error`

func scanExclusiveLease(row exclusiveRow) (exclusiveLease, error) {
	var l exclusiveLease
	err := row.Scan(&l.ID, &l.ChannelID, &l.UserID, &l.State, &l.CoreKeyID, &l.TokenSecret, &l.UpstreamSecret, &l.StartedAt, &l.ExpiresAt, &l.LastChargeAt, &l.BillingCheckedAt, &l.EndedAt, &l.Reason, &l.Cost, &l.Requests, &l.Tokens, &l.Error)
	return l, err
}
func exclusiveJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
func (s *Server) exclusiveAvailable(w http.ResponseWriter) bool {
	if !s.exclusive.Enabled || s.portalDB == nil {
		writeAPIError(w, 503, "EXCLUSIVE_UNAVAILABLE", "独享通道尚未启用或入口隔离未配置")
		return false
	}
	return true
}
func (s *Server) exclusiveError(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, 404, "EXCLUSIVE_NOT_FOUND", "通道或使用记录不存在")
		return
	}
	var p *rentalProblem
	if errors.As(err, &p) {
		writeAPIError(w, p.status, p.code, p.message)
		return
	}
	s.logger.Error("exclusive operation failed", "error", err)
	writeAPIError(w, 503, "EXCLUSIVE_SYNC_FAILED", "Core 同步或数据库操作未完成，通道保持锁定，请刷新或重试同步")
}
func (s *Server) exclusiveEvent(ctx context.Context, channelID, actor int64, action string) error {
	_, err := s.portalDB.ExecContext(ctx, `INSERT INTO portal_exclusive_events(channel_id,actor_id,action) VALUES($1,$2,$3)`, channelID, actor, action)
	return err
}
func exclusiveConflict(message string) error {
	return &rentalProblem{"EXCLUSIVE_CONFLICT", message, 409}
}
func exclusiveGroupName(id int64) string { return fmt.Sprintf("portal-exclusive-%d", id) }
