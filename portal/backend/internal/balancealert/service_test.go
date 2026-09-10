package balancealert

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

type coreStub struct {
	settings coreclient.BalanceAlertSettings
	users    []coreclient.User
}

func (s *coreStub) BalanceAlertSettings(context.Context) (coreclient.BalanceAlertSettings, error) {
	return s.settings, nil
}

func (s *coreStub) AdminUsers(_ context.Context, page, pageSize int, _ ...coreclient.AdminUsersOption) (coreclient.UserPage, error) {
	if page > 1 {
		return coreclient.UserPage{Page: page, PageSize: pageSize, Pages: 1}, nil
	}
	return coreclient.UserPage{Items: s.users, Page: 1, PageSize: pageSize, Pages: 1, Total: int64(len(s.users))}, nil
}

type storeStub struct {
	observations []Observation
	queue        bool
	disabled     bool
	sent         []int64
	failed       []int64
}

func (s *storeStub) TryRunLock(context.Context) (func(), bool, error) {
	return func() {}, true, nil
}
func (s *storeStub) Observe(_ context.Context, observation Observation) (bool, error) {
	s.observations = append(s.observations, observation)
	return s.queue && observation.Eligible, nil
}
func (s *storeStub) MarkSent(_ context.Context, userID int64) error {
	s.sent = append(s.sent, userID)
	return nil
}
func (s *storeStub) MarkFailed(_ context.Context, userID int64, _ error) error {
	s.failed = append(s.failed, userID)
	return nil
}
func (s *storeStub) DisableAll(context.Context) error {
	s.disabled = true
	return nil
}

type mailerStub struct {
	recipient string
	subject   string
	body      string
	err       error
}

func (m *mailerStub) SendHTML(_ context.Context, recipient, subject, body string) error {
	m.recipient, m.subject, m.body = recipient, subject, body
	return m.err
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRunOnceSendsChinesePrimaryEmailAlert(t *testing.T) {
	threshold := 3.0
	core := &coreStub{
		settings: coreclient.BalanceAlertSettings{Enabled: true, Threshold: 3, RechargeURL: "https://togoapi.com", SiteName: "TogoAPI"},
		users: []coreclient.User{{
			ID: 146, Email: "user@example.com", Username: "测试用户", Status: "active", Balance: 2.5,
			BalanceNotifyEnabled: true, BalanceNotifyThreshold: &threshold,
		}},
	}
	store := &storeStub{queue: true}
	mailer := &mailerStub{}
	service, _ := New(core, store, mailer, testLogger(), Config{})

	if err := service.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if mailer.recipient != "user@example.com" || !strings.Contains(mailer.subject, "余额不足提醒") || !strings.Contains(mailer.body, "当前余额") {
		t.Fatalf("recipient=%q subject=%q body=%q", mailer.recipient, mailer.subject, mailer.body)
	}
	if len(store.sent) != 1 || store.sent[0] != 146 {
		t.Fatalf("sent=%v", store.sent)
	}
}

func TestRunOnceStillSendsPrimaryWhenCoreHasDifferentExtraRecipient(t *testing.T) {
	core := &coreStub{
		settings: coreclient.BalanceAlertSettings{Enabled: true, Threshold: 3},
		users: []coreclient.User{{
			ID: 1, Email: "user@example.com", Status: "active", Balance: 2,
			BalanceNotifyEnabled:     true,
			BalanceNotifyExtraEmails: []coreclient.NotifyEmailEntry{{Email: "alerts@example.com", Verified: true}},
		}},
	}
	store := &storeStub{queue: true}
	mailer := &mailerStub{}
	service, _ := New(core, store, mailer, testLogger(), Config{})

	if err := service.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.observations) != 1 || !store.observations[0].Eligible || mailer.recipient != "user@example.com" {
		t.Fatalf("observations=%+v recipient=%q", store.observations, mailer.recipient)
	}
}

func TestRunOnceLeavesPrimaryRecipientToCoreWithoutDuplicate(t *testing.T) {
	core := &coreStub{
		settings: coreclient.BalanceAlertSettings{Enabled: true, Threshold: 3},
		users: []coreclient.User{{
			ID: 1, Email: "user@example.com", Status: "active", Balance: 2,
			BalanceNotifyEnabled:     true,
			BalanceNotifyExtraEmails: []coreclient.NotifyEmailEntry{{Email: "USER@example.com", Verified: true}},
		}},
	}
	store := &storeStub{queue: true}
	mailer := &mailerStub{}
	service, _ := New(core, store, mailer, testLogger(), Config{})

	if err := service.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.observations) != 1 || store.observations[0].Eligible || mailer.recipient != "" {
		t.Fatalf("observations=%+v recipient=%q", store.observations, mailer.recipient)
	}
}

func TestRunOnceDisablesStateWithGlobalSwitch(t *testing.T) {
	store := &storeStub{}
	service, _ := New(&coreStub{}, store, &mailerStub{}, testLogger(), Config{})
	if err := service.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !store.disabled {
		t.Fatal("expected stored alerts to be disabled")
	}
}

func TestRunOnceKeepsFailedSendPending(t *testing.T) {
	core := &coreStub{
		settings: coreclient.BalanceAlertSettings{Enabled: true, Threshold: 3},
		users:    []coreclient.User{{ID: 7, Email: "user@example.com", Status: "active", Balance: 2, BalanceNotifyEnabled: true}},
	}
	store := &storeStub{queue: true}
	service, _ := New(core, store, &mailerStub{err: errors.New("SMTP unavailable")}, testLogger(), Config{})
	if err := service.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.failed) != 1 || len(store.sent) != 0 {
		t.Fatalf("failed=%v sent=%v", store.failed, store.sent)
	}
}

func TestEffectivePercentageThreshold(t *testing.T) {
	custom := 10.0
	got := effectiveThreshold(3, coreclient.User{BalanceNotifyThreshold: &custom, BalanceNotifyThresholdType: "percentage", TotalRecharged: 80})
	if got != 8 {
		t.Fatalf("threshold=%v", got)
	}
}

func TestTransitionBaselinesThenDetectsDownwardCrossing(t *testing.T) {
	observation := Observation{UserID: 1, Balance: 2.5, Threshold: 3, Eligible: true}
	_, pending := transition(state{}, false, observation)
	if pending {
		t.Fatal("first observation must only establish a baseline")
	}
	below, pending := transition(state{Balance: 3.1, Eligible: true}, true, observation)
	if !below || !pending {
		t.Fatalf("below=%v pending=%v", below, pending)
	}
	_, pending = transition(state{Balance: 2.8, Eligible: true}, true, observation)
	if pending {
		t.Fatal("an account already below the threshold must not retrigger")
	}
}
