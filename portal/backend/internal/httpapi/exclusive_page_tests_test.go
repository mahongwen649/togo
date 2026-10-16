package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

type pageTestCore struct {
	coreStub
	entered  chan *http.Request
	finish   chan struct{}
	calls    atomic.Int32
	response string
	broken   bool
}

func (f *pageTestCore) Profile(_ context.Context, token string) (coreclient.User, error) {
	id := int64(7)
	role := "user"
	if token == "other" {
		id = 8
	}
	if token == "admin" {
		id = 9
		role = "admin"
	}
	return coreclient.User{ID: id, Role: role, Status: "active", Balance: 20}, nil
}
func (f *pageTestCore) RelayRoundTrip(r *http.Request) (*http.Response, error) {
	f.calls.Add(1)
	f.entered <- r
	<-f.finish
	var body io.ReadCloser = io.NopCloser(strings.NewReader(f.response))
	if f.broken {
		body = io.NopCloser(brokenReader{})
	}
	return &http.Response{StatusCode: 200, Body: body}, nil
}
func pageFixture(t *testing.T) (*Server, *pageTestCore, *sql.DB, int64, int64) {
	t.Helper()
	db := rentalTestDB(t)
	fake := &pageTestCore{entered: make(chan *http.Request, 10), finish: make(chan struct{})}
	s := &Server{portalDB: db, core: fake, logger: testLogger(), exclusive: ExclusiveConfig{Enabled: true, Secret: strings.Repeat("s", 32)}}
	secret, err := s.sealExclusive("pxu_fixture")
	if err != nil {
		t.Fatal(err)
	}
	var channel, lease int64
	err = db.QueryRow(`INSERT INTO portal_exclusive_channels(name,state,account_ids,durations,cooldown_seconds) VALUES('test','active','[]','[1440]',0) RETURNING id`).Scan(&channel)
	if err != nil {
		t.Fatal(err)
	}
	err = db.QueryRow(`INSERT INTO portal_exclusive_leases(channel_id,user_id,state,token_hash,token_secret,upstream_secret,expires_at,billing_checked_at) VALUES($1,7,'active','hash','secret',$2,CURRENT_TIMESTAMP+interval '24 hours',CURRENT_TIMESTAMP) RETURNING id`, channel, secret).Scan(&lease)
	if err != nil {
		t.Fatal(err)
	}
	return s, fake, db, channel, lease
}
func pageStart(s *Server, channel int64, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/", nil)
	r.SetPathValue("channelID", strconv.FormatInt(channel, 10))
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.exclusivePageTestStart(w, r)
	return w
}
func waitPage(t *testing.T, s *Server, channel int64, state string) exclusivePageTest {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		v, err := s.exclusivePageMetadata(context.Background(), channel, true)
		if err != nil {
			t.Fatal(err)
		}
		if v.State == state {
			return v
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("page did not reach " + state)
	return exclusivePageTest{}
}
func pageResponse(html string) string {
	return exclusiveJSON(map[string]any{"status": "completed", "output": []any{map[string]any{"type": "message", "content": []any{map[string]any{"type": "output_text", "text": html}}}}})
}
func TestExclusivePageAuthorization(t *testing.T) {
	s, fake, db, channel, lease := pageFixture(t)
	for _, token := range []string{"other", "admin"} {
		if w := pageStart(s, channel, token); w.Code != 403 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	for _, update := range []string{
		`UPDATE portal_exclusive_leases SET expires_at=CURRENT_TIMESTAMP-interval '1 second'`,
		`UPDATE portal_exclusive_leases SET expires_at=CURRENT_TIMESTAMP+interval '1 day',started_at=CURRENT_TIMESTAMP-interval '31 minutes'`,
		`UPDATE portal_exclusive_leases SET started_at=CURRENT_TIMESTAMP,state='releasing'`,
		`UPDATE portal_exclusive_leases SET state='closed'`,
	} {
		if _, err := db.Exec(update+` WHERE id=$1`, lease); err != nil {
			t.Fatal(err)
		}
		if w := pageStart(s, channel, "holder"); w.Code != 403 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if fake.calls.Load() != 0 {
		t.Fatal("unauthorized inference")
	}
}
func TestExclusivePageGenerationAndRelease(t *testing.T) {
	s, fake, db, channel, lease := pageFixture(t)
	old := "<!doctype html><html><body>old</body></html>"
	if _, err := db.Exec(`INSERT INTO portal_exclusive_page_tests(channel_id,request_id,state,html,generated_at) VALUES($1,'old','succeeded',$2,CURRENT_TIMESTAMP)`, channel, old); err != nil {
		t.Fatal(err)
	}
	fake.response = pageResponse("```html\n<!doctype html><html><body>new</body></html>\n```")
	if w := pageStart(s, channel, "holder"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var upstream *http.Request
	select {
	case upstream = <-fake.entered:
	case <-time.After(time.Second):
		t.Fatal("request not dispatched")
	}
	var payload map[string]json.RawMessage
	if err := json.NewDecoder(upstream.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if string(payload["model"]) != `"gpt-6-astra"` || string(payload["reasoning"]) != `{"effort":"high"}` || string(payload["input"]) != exclusiveJSON(exclusivePagePrompt) || upstream.Header.Get("Authorization") != "Bearer pxu_fixture" || upstream.URL.Path != "/v1/responses" {
		t.Fatal("wrong channel inference", payload)
	}
	if w := pageStart(s, channel, "holder"); w.Code != 409 {
		t.Fatal("duplicate admitted", w.Code)
	}
	v, _ := s.exclusivePageMetadata(context.Background(), channel, true)
	if v.HTML != old {
		t.Fatal("old page removed while generating")
	}
	if _, err := db.Exec(`UPDATE portal_exclusive_leases SET state='releasing' WHERE id=$1`, lease); err != nil {
		t.Fatal(err)
	}
	if err := s.advanceExclusiveLease(context.Background(), lease, true); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := db.QueryRow(`SELECT state FROM portal_exclusive_leases WHERE id=$1`, lease).Scan(&state); err != nil || state != "releasing" {
		t.Fatal("release bypassed running generation", state, err)
	}
	close(fake.finish)
	v = waitPage(t, s, channel, "succeeded")
	if !strings.Contains(v.HTML, ">new<") || v.GeneratedAt == nil {
		t.Fatal(v)
	}
	if err := db.QueryRow(`SELECT state FROM portal_exclusive_requests WHERE lease_id=$1`, lease).Scan(&state); err != nil || state != "finished" {
		t.Fatal("ledger not completed atomically", state, err)
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.SetPathValue("channelID", strconv.FormatInt(channel, 10))
	r.Header.Set("Authorization", "Bearer other")
	w := httptest.NewRecorder()
	s.exclusivePageTestView(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "new") || strings.Contains(w.Body.String(), "pxu_") {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestExclusivePageFailureKeepsPrevious(t *testing.T) {
	for _, broken := range []bool{false, true} {
		t.Run(strconv.FormatBool(broken), func(t *testing.T) {
			s, fake, db, channel, lease := pageFixture(t)
			fake.broken = broken
			fake.response = `{"status":"incomplete"}`
			old := "<html><body>old</body></html>"
			if _, err := db.Exec(`INSERT INTO portal_exclusive_page_tests(channel_id,request_id,state,html,generated_at) VALUES($1,'old','succeeded',$2,CURRENT_TIMESTAMP)`, channel, old); err != nil {
				t.Fatal(err)
			}
			if w := pageStart(s, channel, "holder"); w.Code != 200 {
				t.Fatal(w.Code)
			}
			close(fake.finish)
			expected := "failed"
			ledger := "finished"
			if broken {
				expected = "uncertain"
				ledger = "uncertain"
			}
			v := waitPage(t, s, channel, expected)
			if v.HTML != old || v.GeneratedAt == nil || v.Error == "" {
				t.Fatal(v)
			}
			var state string
			if err := db.QueryRow(`SELECT state FROM portal_exclusive_requests WHERE lease_id=$1`, lease).Scan(&state); err != nil || state != ledger {
				t.Fatal(state, err)
			}
		})
	}
}
func TestExclusivePageParser(t *testing.T) {
	for _, html := range []string{"<html><body>valid</body></html>", "```html\n<!doctype html><html></html>\n```"} {
		if _, err := parseExclusivePage([]byte(pageResponse(html))); err != nil {
			t.Fatal(err)
		}
	}
	for _, html := range []string{"hello", "<svg>partial", "<html>partial", "```js\n<html></html>\n```", strings.Repeat("x", exclusivePageLimit+1)} {
		if _, err := parseExclusivePage([]byte(pageResponse(html))); err == nil {
			t.Fatal("invalid page accepted")
		}
	}
}

func TestExclusivePageAdmissionLimits(t *testing.T) {
	s, _, db, channel, lease := pageFixture(t)
	if _, err := db.Exec(`UPDATE portal_exclusive_leases SET billing_checked_at=CURRENT_TIMESTAMP-interval '31 seconds' WHERE id=$1`, lease); err != nil {
		t.Fatal(err)
	}
	if w := pageStart(s, channel, "holder"); w.Code != 503 {
		t.Fatal("stale billing admitted", w.Code)
	}
	if _, err := db.Exec(`UPDATE portal_exclusive_leases SET billing_checked_at=CURRENT_TIMESTAMP WHERE id=$1`, lease); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"one", "two", "three"} {
		if _, err := db.Exec(`INSERT INTO portal_exclusive_requests(id,lease_id,state) VALUES($1,$2,'running')`, id, lease); err != nil {
			t.Fatal(err)
		}
	}
	if w := pageStart(s, channel, "holder"); w.Code != 429 {
		t.Fatal("concurrency bypassed", w.Code)
	}
	if _, err := db.Exec(`INSERT INTO portal_exclusive_page_tests(channel_id,request_id,state) VALUES($1,'one','running')`, channel); err != nil {
		t.Fatal(err)
	}
	// A new Server process must not replay a durable, unresolved job.
	restarted := &Server{portalDB: db, core: s.core, logger: testLogger(), exclusive: s.exclusive}
	if w := pageStart(restarted, channel, "holder"); w.Code != 409 {
		t.Fatal("restart replayed generation", w.Code)
	}
}

func TestExclusivePageConcurrentAdmission(t *testing.T) {
	s, fake, _, channel, _ := pageFixture(t)
	fake.response = pageResponse("<html><body>done</body></html>")
	results := make(chan int, 2)
	for i := 0; i < 2; i++ {
		go func() { results <- pageStart(s, channel, "holder").Code }()
	}
	a, b := <-results, <-results
	if !((a == 200 && b == 409) || (a == 409 && b == 200)) {
		close(fake.finish)
		t.Fatal("duplicate admission", a, b)
	}
	close(fake.finish)
	waitPage(t, s, channel, "succeeded")
	if fake.calls.Load() != 1 {
		t.Fatal("duplicate upstream calls")
	}
}
