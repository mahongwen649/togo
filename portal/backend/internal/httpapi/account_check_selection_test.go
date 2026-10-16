package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jqcode/portal/backend/internal/coreclient"
	"strings"
	"testing"
)

type catalogCheckCore struct {
	checksCore
	failSecond bool
}

func (f *catalogCheckCore) AdminUserMarketRequest(_ context.Context, _, path string, _ any, _ string) (coreclient.Response, error) {
	if strings.Contains(path, "page=2") {
		if f.failSecond {
			return coreclient.Response{}, fmt.Errorf("temporary upstream failure")
		}
		return rentalResponse(map[string]any{"items": []checkAccount{{ID: 43, Name: "current second page", Platform: "openai"}}, "total": 51, "pages": 2})
	}
	return rentalResponse(map[string]any{"items": []checkAccount{{ID: 42, Name: "current name", Platform: "openai", GroupIDs: []int64{10}, Groups: []checkGroup{{ID: 10, Name: "current group"}}}}, "total": 51, "pages": 2})
}

func TestAccountCheckHistoryUsesCurrentCatalog(t *testing.T) {
	db := rentalTestDB(t)
	for _, id := range []int64{42, 43, 99} {
		run, err := startAccountCheck(context.Background(), db, 1, checkAccount{ID: id, Name: "old name", Platform: "openai"}, "prompt", "model")
		if err != nil {
			t.Fatal(err)
		}
		if err = finishAccountCheck(context.Background(), db, 1, id, run, accountCheckResult{Text: "history kept"}); err != nil {
			t.Fatal(err)
		}
	}
	core := &catalogCheckCore{checksCore: checksCore{role: "admin", status: "active"}}
	s := &Server{portalDB: db, core: core, logger: testLogger()}
	w := checksRequest(s, "GET", "/api/v1/admin/account-checks/history", "", "token")
	var response struct {
		Data []accountCheckRecord `json:"data"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &response) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	if len(response.Data) != 2 || response.Data[0].Account.Name != "current name" || response.Data[0].Account.Groups[0].Name != "current group" || response.Data[1].Account.ID != 43 {
		t.Fatal(w.Body.String())
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM portal_admin_account_checks`).Scan(&count); err != nil || count != 3 {
		t.Fatal(count, err)
	}
	core.failSecond = true
	w = checksRequest(s, "GET", "/api/v1/admin/account-checks/history", "", "token")
	if w.Code == 200 {
		t.Fatal("partial catalog must not be treated as deletions")
	}
}

func TestAccountCheckSelectionPersistsExplicitEmptyAndIsolatesAdmins(t *testing.T) {
	db := rentalTestDB(t)
	s := &Server{portalDB: db, core: &checksCore{role: "admin", status: "active"}, logger: testLogger()}
	path := "/api/v1/admin/account-checks/selection"
	w := checksRequest(s, "GET", path, "", "token")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"account_ids":null`) {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, body := range []string{`{"account_ids":[42,43,42]}`, `{"account_ids":[]}`} {
		w = checksRequest(s, "PUT", path, body, "token")
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	w = checksRequest(s, "GET", path, "", "token")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"account_ids":[]`) {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err := db.Exec(`INSERT INTO portal_admin_account_check_selection(admin_id,account_ids) VALUES(2,'[99]')`); err != nil {
		t.Fatal(err)
	}
	w = checksRequest(s, "GET", path, "", "token")
	if strings.Contains(w.Body.String(), "99") {
		t.Fatal("leaked other admin selection")
	}
	for _, method := range []string{"GET", "PUT"} {
		s.core = &checksCore{role: "user", status: "active"}
		w = checksRequest(s, method, path, `{"account_ids":[]}`, "token")
		if w.Code != 403 {
			t.Fatal(w.Code)
		}
	}
}
