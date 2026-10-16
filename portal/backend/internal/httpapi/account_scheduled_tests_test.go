package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

type scheduledTestRouteCore struct {
	coreStub
	plans   map[int64][]customerScheduledTestPlan
	results map[int64][]customerScheduledTestResult
	calls   []string
	nextID  int64
}

func scheduledRawJSON(data any) (coreclient.Response, error) {
	body, err := json.Marshal(data)
	return coreclient.Response{StatusCode: http.StatusOK, Body: body}, err
}

func (f *scheduledTestRouteCore) Profile(context.Context, string) (coreclient.User, error) {
	return coreclient.User{ID: 7, Role: "user", Status: "active"}, nil
}

func (f *scheduledTestRouteCore) AdminMarketRequest(_ context.Context, method, path string, payload any, _ string) (coreclient.Response, error) {
	u, err := url.Parse(path)
	if err != nil {
		return coreclient.Response{}, err
	}
	f.calls = append(f.calls, method+" "+u.Path+"?"+u.RawQuery)

	if method == http.MethodGet && strings.HasPrefix(u.Path, "/api/v1/admin/accounts/") && strings.HasSuffix(u.Path, "/scheduled-test-plans") {
		var accountID int64
		if _, err := fmt.Sscanf(u.Path, "/api/v1/admin/accounts/%d/scheduled-test-plans", &accountID); err != nil {
			return coreclient.Response{}, err
		}
		return scheduledRawJSON(f.plans[accountID])
	}
	if method == http.MethodGet && strings.HasPrefix(u.Path, "/api/v1/admin/accounts/") {
		var accountID int64
		if _, err := fmt.Sscanf(u.Path, "/api/v1/admin/accounts/%d", &accountID); err != nil {
			return coreclient.Response{}, err
		}
		return routeJSON(map[string]any{"id": accountID, "name": fmt.Sprintf("Account %d", accountID), "status": "active"})
	}
	if method == http.MethodPost && u.Path == "/api/v1/admin/scheduled-test-plans" {
		raw, _ := json.Marshal(payload)
		var input customerCreateScheduledTestPlanRequest
		if err := json.Unmarshal(raw, &input); err != nil {
			return coreclient.Response{}, err
		}
		f.nextID++
		plan := customerScheduledTestPlan{ID: f.nextID, AccountID: input.AccountID, ModelID: input.ModelID, CronExpression: input.CronExpression, MaxResults: input.MaxResults}
		if input.Enabled == nil || *input.Enabled {
			plan.Enabled = true
		}
		if input.AutoRecover != nil {
			plan.AutoRecover = *input.AutoRecover
		}
		f.plans[input.AccountID] = append(f.plans[input.AccountID], plan)
		return scheduledRawJSON(plan)
	}
	if strings.HasPrefix(u.Path, "/api/v1/admin/scheduled-test-plans/") {
		var planID int64
		if _, err := fmt.Sscanf(u.Path, "/api/v1/admin/scheduled-test-plans/%d", &planID); err != nil {
			return coreclient.Response{}, err
		}
		switch method {
		case http.MethodGet:
			return scheduledRawJSON(f.results[planID])
		case http.MethodPut:
			for accountID, plans := range f.plans {
				for index := range plans {
					if plans[index].ID != planID {
						continue
					}
					var input customerUpdateScheduledTestPlanRequest
					raw, _ := json.Marshal(payload)
					if err := json.Unmarshal(raw, &input); err != nil {
						return coreclient.Response{}, err
					}
					if input.ModelID != "" {
						plans[index].ModelID = input.ModelID
					}
					if input.CronExpression != "" {
						plans[index].CronExpression = input.CronExpression
					}
					if input.Enabled != nil {
						plans[index].Enabled = *input.Enabled
					}
					if input.AutoRecover != nil {
						plans[index].AutoRecover = *input.AutoRecover
					}
					if input.MaxResults > 0 {
						plans[index].MaxResults = input.MaxResults
					}
					f.plans[accountID] = plans
					return scheduledRawJSON(plans[index])
				}
			}
		case http.MethodDelete:
				for accountID, plans := range f.plans {
					for index := range plans {
						if plans[index].ID == planID {
							f.plans[accountID] = append(plans[:index], plans[index+1:]...)
							return scheduledRawJSON(map[string]any{"message": "deleted"})
						}
					}
				}
		}
	}
	return coreclient.Response{}, errors.New("unexpected scheduled-test request")
}

func TestAccountScheduledTestRoutesPreserveCoreCRUDAndResults(t *testing.T) {
	db := rentalTestDB(t)
	core := &scheduledTestRouteCore{
		plans:   map[int64][]customerScheduledTestPlan{1: {}},
		results: map[int64][]customerScheduledTestResult{},
		nextID:  100,
	}
	h := New(core, nil, testLogger(), NewPortalDatabase(db))
	seedRouteOrder(t, db, 7, []int64{1}, "completed", nil, ptrTime(timeNowPlusHour()))

	list := routeRequest(h, http.MethodGet, "/api/v1/account-market/accounts/1/scheduled-test-plans", "")
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"data":[]`) {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}

	create := routeRequest(h, http.MethodPost, "/api/v1/account-market/accounts/1/scheduled-test-plans", `{"account_id":1,"model_id":"gpt-test","cron_expression":"*/30 * * * *","enabled":true,"max_results":20,"auto_recover":true}`)
	if create.Code != http.StatusOK {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}
	planID := core.plans[1][0].ID

	update := routeRequest(h, http.MethodPut, fmt.Sprintf("/api/v1/account-market/accounts/1/scheduled-test-plans/%d", planID), `{"cron_expression":"0 * * * *","enabled":false}`)
	if update.Code != http.StatusOK || core.plans[1][0].Enabled {
		t.Fatalf("update status=%d body=%s plans=%+v", update.Code, update.Body.String(), core.plans[1])
	}

	core.results[planID] = []customerScheduledTestResult{{ID: 1, PlanID: planID, Status: "success", ResponseText: "ok"}}
	results := routeRequest(h, http.MethodGet, fmt.Sprintf("/api/v1/account-market/accounts/1/scheduled-test-plans/%d/results?limit=20", planID), "")
	if results.Code != http.StatusOK || !strings.Contains(results.Body.String(), `"response_text":"ok"`) {
		t.Fatalf("results status=%d body=%s", results.Code, results.Body.String())
	}

	deleted := routeRequest(h, http.MethodDelete, fmt.Sprintf("/api/v1/account-market/accounts/1/scheduled-test-plans/%d", planID), "")
	if deleted.Code != http.StatusOK || len(core.plans[1]) != 0 {
		t.Fatalf("delete status=%d body=%s plans=%+v", deleted.Code, deleted.Body.String(), core.plans[1])
	}
}

func TestAccountScheduledTestRoutesRejectPlanIDsOutsideAccount(t *testing.T) {
	db := rentalTestDB(t)
	core := &scheduledTestRouteCore{
		plans: map[int64][]customerScheduledTestPlan{1: {{ID: 101, AccountID: 1, ModelID: "model", CronExpression: "* * * * *", Enabled: true}}},
		results: map[int64][]customerScheduledTestResult{},
	}
	h := New(core, nil, testLogger(), NewPortalDatabase(db))
	seedRouteOrder(t, db, 7, []int64{1}, "completed", nil, ptrTime(timeNowPlusHour()))

	w := routeRequest(h, http.MethodPut, "/api/v1/account-market/accounts/1/scheduled-test-plans/999", `{"enabled":false}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	for _, call := range core.calls {
		if strings.Contains(call, "PUT /api/v1/admin/scheduled-test-plans/999") {
			t.Fatalf("unauthorized plan forwarded to Core: %s", call)
		}
	}
}

func TestAccountScheduledTestLegacyUnscopedRoutesAreNotRegistered(t *testing.T) {
	db := rentalTestDB(t)
	core := &scheduledTestRouteCore{plans: map[int64][]customerScheduledTestPlan{1: {}}}
	h := New(core, nil, testLogger(), NewPortalDatabase(db))
	seedRouteOrder(t, db, 7, []int64{1}, "completed", nil, ptrTime(timeNowPlusHour()))

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/v1/account-market/scheduled-test-plans", "{\"account_id\":1}"},
		{http.MethodPut, "/api/v1/account-market/scheduled-test-plans/1", "{\"enabled\":false}"},
		{http.MethodDelete, "/api/v1/account-market/scheduled-test-plans/1", ""},
		{http.MethodGet, "/api/v1/account-market/scheduled-test-plans/1/results", ""},
	} {
		w := routeRequest(h, tc.method, tc.path, tc.body)
		if w.Code != http.StatusNotFound {
			t.Fatalf("legacy route %s %s returned status=%d body=%s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}

func timeNowPlusHour() time.Time {
	return time.Now().Add(time.Hour)
}
