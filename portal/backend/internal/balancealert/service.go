package balancealert

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

const defaultPageSize = 100

type Core interface {
	BalanceAlertSettings(context.Context) (coreclient.BalanceAlertSettings, error)
	AdminUsers(context.Context, int, int, ...coreclient.AdminUsersOption) (coreclient.UserPage, error)
}

type Mailer interface {
	SendHTML(context.Context, string, string, string) error
}

type Config struct {
	Interval time.Duration
	PageSize int
}

type Service struct {
	core     Core
	store    StateStore
	mailer   Mailer
	logger   *slog.Logger
	interval time.Duration
	pageSize int
}

func New(core Core, store StateStore, mailer Mailer, logger *slog.Logger, config Config) (*Service, error) {
	if core == nil || store == nil || mailer == nil {
		return nil, fmt.Errorf("balance alert dependencies are required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if config.Interval <= 0 {
		config.Interval = time.Minute
	}
	if config.PageSize <= 0 {
		config.PageSize = defaultPageSize
	}
	return &Service{core: core, store: store, mailer: mailer, logger: logger, interval: config.Interval, pageSize: config.PageSize}, nil
}

func (s *Service) Run(ctx context.Context) {
	s.logger.Info("Portal balance alert monitor started", "interval", s.interval.String())
	s.runWithTimeout(ctx)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runWithTimeout(ctx)
		}
	}
}

func (s *Service) runWithTimeout(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	if err := s.RunOnce(ctx); err != nil {
		s.logger.Error("Portal balance alert scan failed", "error", err)
	}
}

func (s *Service) RunOnce(ctx context.Context) error {
	unlock, locked, err := s.store.TryRunLock(ctx)
	if err != nil {
		return fmt.Errorf("acquire balance alert lock: %w", err)
	}
	if !locked {
		return nil
	}
	defer unlock()

	settings, err := s.core.BalanceAlertSettings(ctx)
	if err != nil {
		return err
	}
	if !settings.Enabled || settings.Threshold <= 0 {
		return s.store.DisableAll(ctx)
	}
	if strings.TrimSpace(settings.SiteName) == "" {
		settings.SiteName = "TogoAPI"
	}

	for pageNumber := 1; ; pageNumber++ {
		page, err := s.core.AdminUsers(ctx, pageNumber, s.pageSize)
		if err != nil {
			return err
		}
		for _, user := range page.Items {
			if err := s.processUser(ctx, settings, user); err != nil {
				s.logger.Error("Portal balance alert user check failed", "user_id", user.ID, "error", err)
			}
		}
		if pageNumber >= page.Pages || len(page.Items) == 0 {
			break
		}
	}
	return nil
}

func (s *Service) processUser(ctx context.Context, settings coreclient.BalanceAlertSettings, user coreclient.User) error {
	threshold := effectiveThreshold(settings.Threshold, user)
	eligible := user.ID > 0 && user.Status == "active" && user.BalanceNotifyEnabled && threshold > 0 && validEmail(user.Email) && !hasCoreRecipientForEmail(user.BalanceNotifyExtraEmails, user.Email)
	pending, err := s.store.Observe(ctx, Observation{
		UserID: user.ID, Email: strings.TrimSpace(user.Email), Balance: user.Balance, Threshold: threshold, Eligible: eligible,
	})
	if err != nil || !pending {
		return err
	}

	alert := Alert{
		SiteName: settings.SiteName, RecipientName: user.Username, Balance: user.Balance,
		Threshold: threshold, RechargeURL: settings.RechargeURL,
	}
	if strings.TrimSpace(alert.RecipientName) == "" {
		alert.RecipientName = user.Email
	}
	if err := s.mailer.SendHTML(ctx, user.Email, alert.Subject(), alert.HTML()); err != nil {
		_ = s.store.MarkFailed(ctx, user.ID, err)
		return fmt.Errorf("send balance alert: %w", err)
	}
	if err := s.store.MarkSent(ctx, user.ID); err != nil {
		return fmt.Errorf("record sent balance alert: %w", err)
	}
	s.logger.Info("Portal balance alert sent", "user_id", user.ID, "balance", user.Balance, "threshold", threshold)
	return nil
}

func effectiveThreshold(systemDefault float64, user coreclient.User) float64 {
	threshold := systemDefault
	if user.BalanceNotifyThreshold != nil {
		threshold = *user.BalanceNotifyThreshold
	}
	if user.BalanceNotifyThresholdType == "percentage" && user.TotalRecharged > 0 {
		return user.TotalRecharged * threshold / 100
	}
	return threshold
}

func hasCoreRecipientForEmail(entries []coreclient.NotifyEmailEntry, primaryEmail string) bool {
	for _, entry := range entries {
		if entry.Verified && !entry.Disabled && strings.EqualFold(strings.TrimSpace(entry.Email), strings.TrimSpace(primaryEmail)) {
			return true
		}
	}
	return false
}

func validEmail(value string) bool {
	value = strings.TrimSpace(value)
	parsed, err := mail.ParseAddress(value)
	return err == nil && strings.EqualFold(parsed.Address, value)
}

type Alert struct {
	SiteName      string
	RecipientName string
	Balance       float64
	Threshold     float64
	RechargeURL   string
}

func (a Alert) Subject() string {
	return fmt.Sprintf("[%s] 余额不足提醒", a.SiteName)
}

func (a Alert) HTML() string {
	button := ""
	if strings.TrimSpace(a.RechargeURL) != "" {
		button = fmt.Sprintf(`<p style="margin:24px 0"><a href="%s" style="display:inline-block;padding:11px 20px;background:#0f9f8f;color:#fff;text-decoration:none;border-radius:6px">立即充值</a></p>`, html.EscapeString(a.RechargeURL))
	}
	return fmt.Sprintf(`<!doctype html><html><body style="margin:0;background:#f4f7f6;font-family:Arial,'Microsoft YaHei',sans-serif;color:#172033"><div style="max-width:600px;margin:32px auto;background:#fff;border:1px solid #dfe7e5;border-top:4px solid #d97706;border-radius:6px;padding:28px"><h2 style="margin:0 0 20px">余额不足提醒</h2><p>%s，您好：</p><p>您当前余额为 <strong>$%.2f</strong>，已低于提醒阈值 <strong>$%.2f</strong>。</p><p>请及时充值，以免服务中断。</p>%s<p style="margin-top:28px;color:#718096;font-size:13px">此邮件由 %s 自动发送。您可以在个人资料中关闭余额不足提醒。</p></div></body></html>`, html.EscapeString(a.RecipientName), a.Balance, a.Threshold, button, html.EscapeString(a.SiteName))
}
