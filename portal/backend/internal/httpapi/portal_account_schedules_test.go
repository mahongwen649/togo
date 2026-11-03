package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

func portalScheduleRequest(s *Server, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	mux := http.NewServeMux()
	s.registerPortalAccountSchedules(mux)
	mux.ServeHTTP(w, r)
	return w
}

func newTestPortalSchedule(t *testing.T, s *Server, accountID int) portalAccountSchedule {
	t.Helper()
	w := portalScheduleRequest(s, http.MethodPost, fmt.Sprintf("/api/v1/admin/account-checks/accounts/%d/schedules", accountID), `{"model_id":"gpt-test","reasoning_effort":"xhigh","prompt":"生成动画","cron_expression":"CRON_TZ=Asia/Shanghai 30 21 * * *","enabled":true}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var response struct {
		Data portalAccountSchedule `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response.Data
}

func TestPortalScheduleCRUDOwnershipAndPartialUpdate(t *testing.T) {
	db := rentalTestDB(t)
	s := &Server{core: &checksCore{role: "admin", status: "active"}, portalDB: db, logger: testLogger()}
	plan := newTestPortalSchedule(t, s, 42)
	if plan.ReasoningEffort != "xhigh" || plan.Prompt != "生成动画" || plan.MaxResults != 3 {
		t.Fatalf("settings missing: %+v", plan)
	}
	path := fmt.Sprintf("/api/v1/admin/account-checks/accounts/42/schedules/%d", plan.ID)
	w := portalScheduleRequest(s, http.MethodPut, path, `{"enabled":false}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	stored, err := loadOwnedPortalSchedule(context.Background(), db, 1, 42, plan.ID)
	if err != nil || stored.Enabled || stored.ReasoningEffort != "xhigh" || stored.Prompt != plan.Prompt {
		t.Fatal(stored, err)
	}
	w = portalScheduleRequest(s, http.MethodPut, path, `{"reasoning_effort":""}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	stored, _ = loadOwnedPortalSchedule(context.Background(), db, 1, 42, plan.ID)
	if stored.ReasoningEffort != "" {
		t.Fatal("cannot restore model default")
	}
	for _, body := range []string{`{"reasoning_effort":"invalid"}`, `{"prompt":" "}`, `{"cron_expression":"0 0 31 2 *"}`} {
		if w = portalScheduleRequest(s, http.MethodPut, path, body); w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if _, err = db.Exec(`UPDATE portal_account_check_schedules SET admin_id=2 WHERE id=$1`, plan.ID); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		if w = portalScheduleRequest(s, method, path, `{"enabled":true}`); w.Code != 404 {
			t.Fatal("cross-admin access", w.Code, w.Body.String())
		}
	}
	if w = portalScheduleRequest(s, http.MethodGet, path+"/results", ""); w.Code != 404 {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestPortalScheduleClaimsSerializeAccountAndKeepThreeResults(t *testing.T) {
	db := rentalTestDB(t)
	s := &Server{core: &checksCore{role: "admin", status: "active"}, portalDB: db, logger: testLogger()}
	plan := newTestPortalSchedule(t, s, 42)
	other := newTestPortalSchedule(t, s, 42)
	if _, err := db.Exec(`UPDATE portal_account_check_schedules SET next_run_at=now()-interval '1 minute'`); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var all []claimedAccountSchedule
	for n := 0; n < 4; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			jobs, err := claimAccountSchedules(context.Background(), db)
			if err != nil {
				t.Error(err)
			}
			mu.Lock()
			all = append(all, jobs...)
			mu.Unlock()
		}()
	}
	wg.Wait()
	if len(all) != 1 {
		t.Fatalf("duplicate account execution: %d", len(all))
	}
	job := all[0]
	if job.plan.ID != plan.ID {
		t.Fatal("wrong due plan order")
	}
	path := fmt.Sprintf("/api/v1/admin/account-checks/accounts/42/schedules/%d", plan.ID)
	if w := portalScheduleRequest(s, http.MethodDelete, path, ""); w.Code != 409 {
		t.Fatal("deleted an active lease", w.Code)
	}
	if err := finishAccountSchedule(context.Background(), db, job, accountCheckResult{Text: "answer", HTML: "<svg></svg>"}, 123); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE portal_account_check_schedules SET enabled=false WHERE id=$1`, other.ID); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 4; n++ {
		if _, err := db.Exec(`UPDATE portal_account_check_schedules SET next_run_at=now()-interval '1 minute' WHERE id=$1`, plan.ID); err != nil {
			t.Fatal(err)
		}
		jobs, err := claimAccountSchedules(context.Background(), db)
		if err != nil || len(jobs) != 1 {
			t.Fatal(jobs, err)
		}
		if err = finishAccountSchedule(context.Background(), db, jobs[0], accountCheckResult{Text: fmt.Sprint(n)}, 123); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM portal_account_check_schedule_results WHERE plan_id=$1`, plan.ID).Scan(&count); err != nil || count != 3 {
		t.Fatal(count, err)
	}
	if w := portalScheduleRequest(s, http.MethodGet, path+"/results?limit=3", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"response_text":"3"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := portalScheduleRequest(s, http.MethodGet, path+"/results?limit=100", ""); w.Code != 400 {
		t.Fatal(w.Code)
	}
	if w := portalScheduleRequest(s, http.MethodDelete, path, ""); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestPortalScheduleRecoversStaleLeaseAndHonorsPause(t *testing.T) {
	db := rentalTestDB(t)
	s := &Server{core: &checksCore{role: "admin", status: "active"}, portalDB: db, logger: testLogger()}
	plan := newTestPortalSchedule(t, s, 42)
	if _, err := db.Exec(`UPDATE portal_account_check_schedules SET next_run_at=now()-interval '1 minute' WHERE id=$1`, plan.ID); err != nil {
		t.Fatal(err)
	}
	jobs, err := claimAccountSchedules(context.Background(), db)
	if err != nil || len(jobs) != 1 {
		t.Fatal(jobs, err)
	}
	old := jobs[0]
	if _, err = db.Exec(`UPDATE portal_account_check_schedules SET next_run_at=now()-interval '1 minute',running_until=now()-interval '1 minute' WHERE id=$1`, plan.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE portal_account_check_schedule_results SET started_at=now()-interval '7 minutes' WHERE id=$1`, old.resultID); err != nil {
		t.Fatal(err)
	}
	jobs, err = claimAccountSchedules(context.Background(), db)
	if err != nil || len(jobs) != 1 {
		t.Fatal(jobs, err)
	}
	if err = finishAccountSchedule(context.Background(), db, old, accountCheckResult{Text: "late result"}, 100); err != nil {
		t.Fatal(err)
	}
	stored, _ := loadOwnedPortalSchedule(context.Background(), db, 1, 42, plan.ID)
	if stored.RunningUntil == nil {
		t.Fatal("stale attempt released a newer lease")
	}
	var status string
	if err = db.QueryRow(`SELECT status FROM portal_account_check_schedule_results WHERE id=$1`, old.resultID).Scan(&status); err != nil || status != "failed" {
		t.Fatal(status, err)
	}
	if err = finishAccountSchedule(context.Background(), db, jobs[0], accountCheckResult{}, 100); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE portal_account_check_schedules SET enabled=false,next_run_at=now()-interval '1 minute'`); err != nil {
		t.Fatal(err)
	}
	jobs, err = claimAccountSchedules(context.Background(), db)
	if err != nil || len(jobs) != 0 {
		t.Fatal("paused schedule executed", jobs, err)
	}
}

type scheduledCoreProbe struct {
	active bool
	calls  int
	got    portalAccountSchedule
}

func (f *scheduledCoreProbe) AdminMarketRequest(_ context.Context, method, path string, _ any, _ string) (coreclient.Response, error) {
	if method != http.MethodGet || path != "/api/v1/admin/users/1" {
		return coreclient.Response{}, fmt.Errorf("unexpected mutation")
	}
	status := "disabled"
	if f.active {
		status = "active"
	}
	return rentalResponse(coreclient.User{ID: 1, Role: "admin", Status: status})
}
func (f *scheduledCoreProbe) ScheduledAccountPromptRequest(_ context.Context, id int64, model, prompt, requestID, effort string) (coreclient.Response, error) {
	f.calls++
	f.got = portalAccountSchedule{AccountID: id, ModelID: model, Prompt: prompt, ReasoningEffort: effort}
	if requestID != "portal-schedule-9-10" {
		return coreclient.Response{}, fmt.Errorf("no unique identity")
	}
	return coreclient.Response{StatusCode: 200, Body: []byte(checkEvents("answer", true))}, nil
}

func TestPortalScheduleRunnerForwardsSettingsAndChecksCurrentAdmin(t *testing.T) {
	core := &scheduledCoreProbe{active: true}
	plan := portalAccountSchedule{ID: 9, AdminID: 1, AccountID: 42, ModelID: "model", Prompt: "question", ReasoningEffort: "xhigh"}
	result := performScheduledAccountCheck(context.Background(), core, plan, 10)
	if result.Error != "" || core.calls != 1 || core.got.ReasoningEffort != "xhigh" || core.got.Prompt != "question" || core.got.AccountID != 42 {
		t.Fatal(result, core)
	}
	core.active = false
	result = performScheduledAccountCheck(context.Background(), core, plan, 10)
	if result.Error == "" || core.calls != 1 {
		t.Fatal("revoked admin still executed", result)
	}
}

func TestPortalScheduleBeijingTimeAndInvalidDates(t *testing.T) {
	from := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	next, err := nextAccountSchedule("CRON_TZ=Asia/Shanghai 30 9 * * *", from)
	if err != nil || !next.Equal(time.Date(2026, 10, 20, 1, 30, 0, 0, time.UTC)) {
		t.Fatal(next, err)
	}
	if _, err = nextAccountSchedule("0 0 31 2 *", from); err == nil {
		t.Fatal("impossible date accepted")
	}
}
