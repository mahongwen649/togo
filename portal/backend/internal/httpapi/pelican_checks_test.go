package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

type pelicanFakeCore struct {
	rentalFakeCore
	entered      chan struct{}
	finish       chan struct{}
	calls        atomic.Int32
	failFirst    bool
	accountCalls []int64
	callMu       sync.Mutex
	probeError   error
	models       []string
	prompts      []string
}

func (f *pelicanFakeCore) AdminMarketRequest(_ context.Context, _ string, path string, _ any, _ string) (coreclient.Response, error) {
	if strings.HasPrefix(path, "/api/v1/admin/accounts?") {
		return coreclient.Response{StatusCode: 200, Body: []byte(`{"code":0,"data":{"items":[{"id":42,"name":"绘图账号","platform":"openai","type":"apikey","status":"active","schedulable":true,"group_ids":[1]},{"id":43,"name":"绘图账号","platform":"openai","type":"apikey","status":"active","schedulable":true,"group_ids":[1]}],"pages":1}}`)}, nil
	}
	if strings.HasPrefix(path, "/api/v1/admin/usage?") {
		return coreclient.Response{StatusCode: 200, Body: []byte(`{"code":0,"data":{"items":[{"account_id":42,"account":{"id":42,"name":"绘图账号"}}]}}`)}, nil
	}
	return coreclient.Response{StatusCode: 200, Body: []byte(`{"code":0,"data":{"items":[{"id":2,"key":"test-key","name":"new-key","group_id":1,"status":"active"}],"pages":1}}`)}, nil
}

func (f *pelicanFakeCore) PelicanAccountPromptRequest(ctx context.Context, id int64, model, prompt, requestID string, efforts ...string) (coreclient.Response, error) {
	f.callMu.Lock()
	f.accountCalls = append(f.accountCalls, id)
	f.models = append(f.models, model)
	f.prompts = append(f.prompts, prompt)
	f.callMu.Unlock()
	if f.probeError != nil {
		return coreclient.Response{}, f.probeError
	}
	response, err := f.RelayRoundTrip(httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx))
	if err != nil {
		return coreclient.Response{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return coreclient.Response{}, err
	}
	page, err := parsePelicanStream(body)
	if err != nil {
		return coreclient.Response{}, err
	}
	content, _ := json.Marshal(map[string]any{"type": "content", "text": page})
	return coreclient.Response{StatusCode: 200, Body: []byte("data: " + string(content) + "\n\ndata: {\"type\":\"test_complete\",\"success\":true}\n\n")}, nil
}

func (f *pelicanFakeCore) RelayRoundTrip(r *http.Request) (*http.Response, error) {
	call := f.calls.Add(1)
	if f.entered != nil {
		f.entered <- struct{}{}
	}
	if f.finish != nil {
		select {
		case <-f.finish:
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
	}
	if f.failFirst && call == 1 {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`data: {"type":"error","error":{"code":"stream_read_error","message":"stream_read_error"}}`))}, nil
	}
	text, _ := json.Marshal(map[string]any{"type": "response.output_text.done", "text": `<!doctype html><html><body><svg xmlns="http://www.w3.org/2000/svg"><circle r="5"/></svg></body></html>`})
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("data: " + string(text) + "\n\ndata: {\"type\":\"response.completed\"}\n\n"))}, nil
}

func pelicanFixture(t *testing.T) (*sql.DB, *pelicanFakeCore, *PelicanRunner) {
	t.Helper()
	db := rentalTestDB(t)
	if _, err := db.Exec(`UPDATE portal_pelican_check_config SET enabled=true,group_id=1,group_name='group',api_key_id=2,api_key_name='new-key',next_run_at=CURRENT_TIMESTAMP - INTERVAL '1 minute' WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	fake := &pelicanFakeCore{}
	p := NewPelicanRunner(db, fake, testLogger())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = p.Shutdown(ctx)
	})
	return db, fake, p
}

func waitPelican(t *testing.T, db *sql.DB, id int64) string {
	t.Helper()
	for end := time.Now().Add(8 * time.Second); time.Now().Before(end); {
		var state string
		if err := db.QueryRow(`SELECT state FROM portal_pelican_check_runs WHERE id=$1`, id).Scan(&state); err != nil {
			t.Fatal(err)
		}
		if state != "running" {
			return state
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not finish")
	return ""
}

func TestPelicanSchedulerIndependentLockAndPersistentSchedule(t *testing.T) {
	db, _, p := pelicanFixture(t)
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, `SELECT pg_advisory_lock(734210092)`); err != nil {
		t.Fatal(err)
	}
	defer conn.ExecContext(ctx, `SELECT pg_advisory_unlock(734210092)`)
	id, err := p.start(ctx, false)
	if err != nil || id == 0 {
		t.Fatalf("scheduler blocked by unrelated lock: %d %v", id, err)
	}
	if got := waitPelican(t, db, id); got != "succeeded" {
		t.Fatal(got)
	}
	config, err := loadPelicanCheckConfig(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if config.SchedulerSeenAt == nil || config.NextRunAt == nil || time.Until(*config.NextRunAt) < 29*time.Minute {
		t.Fatalf("missing heartbeat/schedule: %+v", config)
	}
	if _, err = db.Exec(`DELETE FROM portal_pelican_check_runs WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if id, err = p.start(ctx, false); err != nil || id != 0 {
		t.Fatalf("history cleanup triggered unexpected run: %d %v", id, err)
	}
}

func TestPelicanConcurrentManualScheduledAndShutdownDrain(t *testing.T) {
	db, fake, p := pelicanFixture(t)
	fake.entered = make(chan struct{}, 20)
	fake.finish = make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	id, err := p.start(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	<-fake.entered
	cancel() // HTTP caller can leave without cancelling an accepted job.
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			other := NewPelicanRunner(db, fake, testLogger())
			if _, err := other.start(context.Background(), i%2 == 0); !errors.Is(err, errPelicanCheckInProgress) {
				t.Errorf("duplicate admitted: %v", err)
			}
		}(i)
	}
	wg.Wait()
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer drainCancel()
	done := make(chan error, 1)
	go func() { done <- p.Shutdown(drainCtx) }()
	select {
	case err := <-done:
		t.Fatalf("shutdown returned before generation finished: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(fake.finish)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := waitPelican(t, db, id); got != "succeeded" {
		t.Fatal(got)
	}
	if fake.calls.Load() != 1 {
		t.Fatalf("calls=%d", fake.calls.Load())
	}
	if _, err = p.start(context.Background(), true); err == nil {
		t.Fatal("accepted job during shutdown")
	}
}

func TestPelicanTransientRetryAndStaleRecovery(t *testing.T) {
	db, fake, p := pelicanFixture(t)
	fake.failFirst = true
	if _, err := db.Exec(`INSERT INTO portal_pelican_check_runs(state,started_at) VALUES('running',CURRENT_TIMESTAMP-INTERVAL '21 minutes')`); err != nil {
		t.Fatal(err)
	}
	id, err := p.start(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if got := waitPelican(t, db, id); got != "succeeded" {
		t.Fatal(got)
	}
	var attempt, stale int
	if err = db.QueryRow(`SELECT attempt FROM portal_pelican_check_runs WHERE id=$1`, id).Scan(&attempt); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT count(*) FROM portal_pelican_check_runs WHERE state='failed'`).Scan(&stale); err != nil {
		t.Fatal(err)
	}
	if len(fake.accountCalls) != 2 || fake.accountCalls[0] == fake.accountCalls[1] {
		t.Fatalf("did not switch accounts: %v", fake.accountCalls)
	}
	if fake.calls.Load() != 2 || attempt != 2 || stale != 1 {
		t.Fatalf("calls=%d attempt=%d stale=%d", fake.calls.Load(), attempt, stale)
	}
	if pelicanRetryable(errors.New("invalid_api_key")) {
		t.Fatal("invalid Key must not retry")
	}
}

func TestPelicanGalleryOmitsFailuresAndRates(t *testing.T) {
	db, fake, _ := pelicanFixture(t)
	if _, err := db.Exec(`INSERT INTO portal_pelican_check_runs(group_id,api_key_id,state,error,started_at) VALUES (1,1,'succeeded','',CURRENT_TIMESTAMP),(1,2,'failed','private upstream failure',CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	s := &Server{portalDB: db, core: fake, logger: testLogger()}
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer admin")
	w := httptest.NewRecorder()
	s.pelicanChecks(w, r)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	for _, hidden := range []string{"private upstream failure", "pass_rate", "failed", "timeline"} {
		if strings.Contains(w.Body.String(), hidden) {
			t.Fatalf("gallery leaked %s: %s", hidden, w.Body)
		}
	}
	if !strings.Contains(w.Body.String(), `"latest_run_id":2`) || !strings.Contains(w.Body.String(), `"active_run_id":0`) {
		t.Fatalf("missing completion cursor: %s", w.Body)
	}
}

func TestPelicanMonitorContinuesPollingAfterSuccess(t *testing.T) {
	db, fake, p := pelicanFixture(t)
	fake.entered = make(chan struct{}, 4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { p.Run(ctx); close(done) }()
	select {
	case <-fake.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("startup did not schedule due run")
	}
	var id int64
	if err := db.QueryRow(`SELECT max(id) FROM portal_pelican_check_runs`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if state := waitPelican(t, db, id); state != "succeeded" {
		t.Fatal(state)
	}
	// Advance the persistent due time in this isolated database, without
	// waiting thirty minutes or touching any live check configuration.
	if _, err := db.Exec(`UPDATE portal_pelican_check_config SET next_run_at=CURRENT_TIMESTAMP-INTERVAL '1 second'`); err != nil {
		t.Fatal(err)
	}
	select {
	case <-fake.entered:
	case <-time.After(7 * time.Second):
		t.Fatal("scheduler stopped after its first completed run")
	}
	cancel()
	<-done
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer drainCancel()
	if err := p.Shutdown(drainCtx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM portal_pelican_check_runs WHERE state='succeeded'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("completed %d runs", count)
	}
}

func TestPelicanArtworkAccountIsAdminOnlyAndAdminCanDelete(t *testing.T) {
	db, fake, p := pelicanFixture(t)
	id, err := p.start(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if state := waitPelican(t, db, id); state != "succeeded" {
		t.Fatal(state)
	}
	var requestID, accountName string
	var accountID int64
	if err = db.QueryRow(`SELECT client_request_id,account_id,account_name FROM portal_pelican_check_runs WHERE id=$1`, id).Scan(&requestID, &accountID, &accountName); err != nil {
		t.Fatal(err)
	}
	if requestID == "" || (accountID != 42 && accountID != 43) || accountName != "绘图账号" {
		t.Fatalf("missing account correlation: request=%q account=%d/%q", requestID, accountID, accountName)
	}

	s := &Server{portalDB: db, core: fake, logger: testLogger()}
	get := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/admin/pelican-checks", nil)
		r.Header.Set("Authorization", "Bearer token")
		w := httptest.NewRecorder()
		s.pelicanChecks(w, r)
		return w
	}
	adminView := get()
	if adminView.Code != http.StatusOK || !strings.Contains(adminView.Body.String(), `"account_name":"绘图账号"`) || !strings.Contains(adminView.Body.String(), `"account_id":`) {
		t.Fatalf("admin account identity missing: %d %s", adminView.Code, adminView.Body.String())
	}
	fake.role = "user"
	memberView := get()
	if memberView.Code != http.StatusOK || strings.Contains(memberView.Body.String(), `"account_name"`) || strings.Contains(memberView.Body.String(), `"account_id"`) {
		t.Fatalf("member received administrator account identity: %d %s", memberView.Code, memberView.Body.String())
	}

	deleteRequest := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/pelican-checks/artworks/1", nil)
		r.SetPathValue("artworkID", strconv.FormatInt(id, 10))
		r.Header.Set("Authorization", "Bearer token")
		w := httptest.NewRecorder()
		s.deletePelicanArtwork(w, r)
		return w
	}
	if denied := deleteRequest(); denied.Code != http.StatusForbidden {
		t.Fatalf("member deletion status=%d body=%s", denied.Code, denied.Body.String())
	}
	fake.role = "admin"
	if deleted := deleteRequest(); deleted.Code != http.StatusOK {
		t.Fatalf("admin deletion status=%d body=%s", deleted.Code, deleted.Body.String())
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM portal_pelican_check_runs WHERE id=$1`, id).Scan(&count); err != nil || count != 0 {
		t.Fatalf("artwork was not deleted: count=%d err=%v", count, err)
	}
}

func TestPelicanFailedAccountsAreNotReusedOrPublished(t *testing.T) {
	db, fake, runner := pelicanFixture(t)
	fake.probeError = context.DeadlineExceeded
	id, err := runner.start(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if state := waitPelican(t, db, id); state != "failed" {
		t.Fatal(state)
	}
	if len(fake.accountCalls) != 2 || fake.accountCalls[0] == fake.accountCalls[1] {
		t.Fatalf("reused failed account: %v", fake.accountCalls)
	}
	var html string
	if err := db.QueryRow(`SELECT html FROM portal_pelican_check_runs WHERE id=$1`, id).Scan(&html); err != nil || html != "" {
		t.Fatalf("published failed output: %q %v", html, err)
	}
}

func TestPelicanCustomPromptReachesUpstream(t *testing.T) {
	fake := &pelicanFakeCore{}
	_, err := requestPelicanAccountPage(context.Background(), fake, 42, "request-id", "custom-model", "绘制太阳系动画")
	if err != nil {
		t.Fatal(err)
	}
	if fake.models[0] != "custom-model" || !strings.HasPrefix(fake.prompts[0], "绘制太阳系动画\n") {
		t.Fatalf("custom configuration lost: %v %v", fake.models, fake.prompts)
	}
}

func TestPelicanConfigValidationAndDisabledPersistence(t *testing.T) {
	db, fake, _ := pelicanFixture(t)
	s := &Server{portalDB: db, core: fake, logger: testLogger()}
	save := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer token")
		w := httptest.NewRecorder()
		s.updatePelicanCheckConfig(w, r)
		return w
	}
	for _, body := range []string{`{"model":" "}`, `{"prompt":" "}`, `{"reasoning_effort":"invalid"}`} {
		if w := save(body); w.Code != 400 {
			t.Fatalf("invalid config accepted: %d %s", w.Code, w.Body)
		}
	}
	if w := save(`{"enabled":false,"model":" custom-model ","prompt":" 太阳系 ","reasoning_effort":"low"}`); w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body)
	}
	config, err := loadPelicanCheckConfig(context.Background(), db)
	if err != nil || config.Enabled || config.Model != "custom-model" || config.Prompt != "太阳系" || config.ReasoningEffort != "low" {
		t.Fatalf("config not persisted: %+v %v", config, err)
	}
	fake.role = "user"
	if w := save(`{"model":"other"}`); w.Code != 403 {
		t.Fatalf("member changed config: %d", w.Code)
	}
}
