package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

func TestAccountCheckHistoryRetentionAndOwnership(t *testing.T) {
	db := rentalTestDB(t)
	ctx := context.Background()
	account := checkAccount{ID: 42, Name: "saved account", Platform: "openai"}
	for n := 0; n < 5; n++ {
		id, err := startAccountCheck(ctx, db, 1, account, "question", "model")
		if err != nil {
			t.Fatal(err)
		}
		if err = finishAccountCheck(ctx, db, 1, 42, id, accountCheckResult{Text: "answer", HTML: "<html><body>saved</body></html>", Model: "model"}); err != nil {
			t.Fatal(err)
		}
	}
	id, err := startAccountCheck(ctx, db, 2, account, "private question", "model")
	if err != nil {
		t.Fatal(err)
	}
	if err = finishAccountCheck(ctx, db, 2, 42, id, accountCheckResult{Text: "other admin"}); err != nil {
		t.Fatal(err)
	}
	s := &Server{portalDB: db, core: &checksCore{role: "admin", status: "active"}, logger: testLogger()}
	w := checksRequest(s, "GET", "/api/v1/admin/account-checks/accounts/42/history", "", "token")
	var envelope struct {
		Data []accountCheckRecord `json:"data"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &envelope) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	if len(envelope.Data) != 3 {
		t.Fatal(w.Body.String())
	}
	for i, r := range envelope.Data {
		if r.Result.Text != "answer" || r.Result.HTML == "" || r.FinishedAt == nil || r.Prompt != "question" {
			t.Fatal(r)
		}
		if i > 0 && r.ID >= envelope.Data[i-1].ID {
			t.Fatal("history order")
		}
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM portal_admin_account_checks WHERE admin_id=1`).Scan(&count); err != nil || count != 3 {
		t.Fatal(count, err)
	}
	w = checksRequest(s, "GET", "/api/v1/admin/account-checks/history", "", "token")
	if w.Code != 200 || strings.Contains(w.Body.String(), "<html>") || strings.Contains(w.Body.String(), "private question") {
		t.Fatal(w.Body.String())
	}
	for _, role := range []string{"user", "group_manager", "company_manager"} {
		s.core = &checksCore{role: role, status: "active"}
		if w = checksRequest(s, "GET", "/api/v1/admin/account-checks/accounts/42/history", "", "token"); w.Code != 403 {
			t.Fatal(role, w.Code)
		}
	}
}

func TestAccountCheckHistoryConcurrentAdmissionAndStaleRecovery(t *testing.T) {
	db := rentalTestDB(t)
	ctx := context.Background()
	account := checkAccount{ID: 42, Platform: "openai"}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := startAccountCheck(ctx, db, 1, account, "prompt", "model")
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success, busy := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, errAccountCheckBusy) {
			busy++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || busy != 1 {
		t.Fatal(success, busy)
	}
	if _, err := db.Exec(`UPDATE portal_admin_account_checks SET started_at=now()-interval '7 minutes'`); err != nil {
		t.Fatal(err)
	}
	if err := expireAccountChecks(ctx, db, 1); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := db.QueryRow(`SELECT state FROM portal_admin_account_checks`).Scan(&state); err != nil || state != "failed" {
		t.Fatal(state, err)
	}
	if _, err := startAccountCheck(ctx, db, 1, account, "retry", "model"); err != nil {
		t.Fatal(err)
	}
}

type disconnectChecksCore struct {
	checksCore
	started, release, cancelled chan struct{}
}

func (f *disconnectChecksCore) AccountPromptRequest(ctx context.Context, id int64, model, prompt, token string) (coreclient.Response, error) {
	close(f.started)
	select {
	case <-ctx.Done():
		if f.cancelled != nil {
			close(f.cancelled)
		}
		return coreclient.Response{}, ctx.Err()
	case <-f.release:
		return coreclient.Response{StatusCode: 200, Body: []byte(checkEvents("saved after refresh", true))}, nil
	}
}

func TestAccountCheckCanBeExplicitlyCancelled(t *testing.T) {
	db := rentalTestDB(t)
	core := &disconnectChecksCore{checksCore: checksCore{role: "admin", status: "active"}, started: make(chan struct{}), release: make(chan struct{}), cancelled: make(chan struct{})}
	s := &Server{portalDB: db, core: core, logger: testLogger()}
	mux := http.NewServeMux()
	s.registerAccountChecks(mux)
	request := httptest.NewRequest("POST", "/api/v1/admin/account-checks/accounts/42/test", strings.NewReader(`{"prompt":"question"}`))
	request.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { mux.ServeHTTP(w, request); close(done) }()
	select {
	case <-core.started:
	case <-time.After(5 * time.Second):
		t.Fatal("check not started")
	}
	cancelResponse := checksRequest(s, "DELETE", "/api/v1/admin/account-checks/accounts/42/test", "", "token")
	if cancelResponse.Code != 200 || !strings.Contains(cancelResponse.Body.String(), `"cancelled":true`) {
		t.Fatal(cancelResponse.Code, cancelResponse.Body.String())
	}
	select {
	case <-core.cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("Core request context was not cancelled")
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("check handler did not stop")
	}
	var state, message string
	if err := db.QueryRow(`SELECT state,result->>'error' FROM portal_admin_account_checks`).Scan(&state, &message); err != nil || state != "cancelled" || message != "检查已由管理员终止" {
		t.Fatal(state, message, err)
	}
	cancelResponse = checksRequest(s, "DELETE", "/api/v1/admin/account-checks/accounts/42/test", "", "token")
	if cancelResponse.Code != 200 || !strings.Contains(cancelResponse.Body.String(), `"cancelled":false`) {
		t.Fatal(cancelResponse.Code, cancelResponse.Body.String())
	}
}
func TestAccountCheckPersistsAfterBrowserDisconnect(t *testing.T) {
	db := rentalTestDB(t)
	core := &disconnectChecksCore{checksCore: checksCore{role: "admin", status: "active"}, started: make(chan struct{}), release: make(chan struct{})}
	s := &Server{portalDB: db, core: core, logger: testLogger()}
	mux := http.NewServeMux()
	s.registerAccountChecks(mux)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := httptest.NewRequest("POST", "/api/v1/admin/account-checks/accounts/42/test", strings.NewReader(`{"prompt":"question"}`)).WithContext(ctx)
	request.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { mux.ServeHTTP(w, request); close(done) }()
	select {
	case <-core.started:
	case <-time.After(5 * time.Second):
		t.Fatal("check not started")
	}
	cancel()
	close(core.release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("check did not finish")
	}
	var state, text string
	if err := db.QueryRow(`SELECT state,result->>'text' FROM portal_admin_account_checks`).Scan(&state, &text); err != nil || state != "succeeded" || text != "saved after refresh" {
		t.Fatal(state, text, err)
	}
}
