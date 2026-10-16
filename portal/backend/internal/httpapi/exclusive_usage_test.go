package httpapi

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

type exclusiveUsageCore struct {
	pageTestCore
	paths       []string
	fail        bool
	accountType string
	expiresAt   *int64
}

func (f *exclusiveUsageCore) AdminMarketRequest(_ context.Context, method, path string, _ any, _ string) (coreclient.Response, error) {
	f.paths = append(f.paths, method+" "+path)
	if path == "/api/v1/admin/accounts/15" {
		accountType := f.accountType
		if accountType == "" {
			accountType = "oauth"
		}
		return rentalResponse(map[string]any{"id": 15, "name": "private-account-name", "platform": "openai", "type": accountType, "status": "active", "schedulable": true, "expires_at": f.expiresAt, "credentials": map[string]string{"plan_type": "prolite"}})
	}
	if strings.HasPrefix(path, "/api/v1/admin/accounts/15/usage?source=active") {
		if f.fail {
			return coreclient.Response{}, fmt.Errorf("upstream unavailable")
		}
		return rentalResponse(map[string]any{"updated_at": "2026-09-16T07:00:00Z", "five_hour": map[string]any{"utilization": 0, "resets_at": nil}, "seven_day": map[string]any{"utilization": 47, "resets_at": "2026-09-20T13:00:00Z"}})
	}
	return coreclient.Response{}, fmt.Errorf("unexpected Core path: %s", path)
}

func TestExclusiveAPIKeyUsageSkipsSubscriptionProbe(t *testing.T) {
	s, _, _, _, _ := pageFixture(t)
	expired := time.Now().Add(-time.Hour).Unix()
	f := &exclusiveUsageCore{accountType: "apikey", expiresAt: &expired, fail: true}
	s.core = f
	a := s.exclusiveAccountView(context.Background(), 15)
	if !a.Available || a.Error != "" || a.Type != "apikey" {
		t.Fatalf("incorrect API Key account state: %+v", a)
	}
	if len(f.paths) != 1 || f.paths[0] != "GET /api/v1/admin/accounts/15" {
		t.Fatalf("API Key account made a subscription usage request: %v", f.paths)
	}
}
func TestExclusiveUsageUsesCoreActiveWindows(t *testing.T) {
	s, _, _, _, _ := pageFixture(t)
	f := &exclusiveUsageCore{}
	s.core = f
	a := s.exclusiveAccountView(context.Background(), 15)
	if a.Error != "" || a.FiveHourRemaining == nil || *a.FiveHourRemaining != 100 || a.WeeklyRemaining == nil || *a.WeeklyRemaining != 53 || a.WeeklyResetsAt == nil || a.ObservedAt == nil {
		t.Fatalf("incorrect windows: %+v", a)
	}
	if f.paths[1] != "GET /api/v1/admin/accounts/15/usage?source=active" {
		t.Fatal(f.paths)
	}
}
func TestExclusiveUsageRefreshScopeAndThrottle(t *testing.T) {
	s, _, db, cid, _ := pageFixture(t)
	if _, err := db.Exec(`UPDATE portal_exclusive_channels SET account_ids='[15]' WHERE id=$1`, cid); err != nil {
		t.Fatal(err)
	}
	f := &exclusiveUsageCore{}
	s.core = f
	request := func(aid int) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/", nil)
		r.Header.Set("Authorization", "Bearer holder")
		r.SetPathValue("channelID", strconv.FormatInt(cid, 10))
		r.SetPathValue("accountID", strconv.Itoa(aid))
		w := httptest.NewRecorder()
		s.exclusiveRefreshUsage(w, r)
		return w
	}
	if w := request(99); w.Code != 404 || len(f.paths) != 0 {
		t.Fatal("foreign account refreshed", w.Code, f.paths)
	}
	if w := request(15); w.Code != 200 || strings.Contains(w.Body.String(), "private-account-name") || strings.Contains(w.Body.String(), "credentials") {
		t.Fatal(w.Code, w.Body.String())
	}
	if f.paths[1] != "GET /api/v1/admin/accounts/15/usage?source=active&force=true" {
		t.Fatal(f.paths)
	}
	if w := request(15); w.Code != 429 {
		t.Fatal("repeat refresh not throttled", w.Code)
	}
	if len(f.paths) != 2 {
		t.Fatal("unnecessary Core requests", f.paths)
	}
	if _, err := db.Exec(`UPDATE portal_exclusive_channels SET state='unpublished' WHERE id=$1`, cid); err != nil {
		t.Fatal(err)
	}
	if w := request(15); w.Code != 404 {
		t.Fatal("hidden channel refresh allowed", w.Code)
	}
}
