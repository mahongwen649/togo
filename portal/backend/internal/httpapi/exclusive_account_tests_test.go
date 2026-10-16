package httpapi

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

type accountTestCore struct {
	pageTestCore
	rawCalls atomic.Int32
}

func (f *accountTestCore) AdminMarketRawRequest(_ context.Context, method, path string, _ any, _ string) (coreclient.Response, error) {
	f.rawCalls.Add(1)
	body := "data: {\"type\":\"content\",\"text\":\"<!doctype html><html><body>account test</body></html>\"}\n\n" +
		"data: {\"type\":\"test_complete\",\"success\":true}\n\n"
	return coreclient.Response{StatusCode: http.StatusOK, Body: []byte(body)}, nil
}

func accountTestFixture(t *testing.T, state string, withLease bool) (*Server, *accountTestCore, *sql.DB, int64) {
	t.Helper()
	db := rentalTestDB(t)
	core := &accountTestCore{}
	s := &Server{portalDB: db, core: core, logger: testLogger(), exclusive: ExclusiveConfig{Enabled: true, Secret: strings.Repeat("s", 32)}}
	var channelID int64
	if err := db.QueryRow(`INSERT INTO portal_exclusive_channels(name,state,account_ids,durations,cooldown_seconds) VALUES('test',$1,'[15]','[1440]',0) RETURNING id`, state).Scan(&channelID); err != nil {
		t.Fatal(err)
	}
	if withLease {
		if _, err := db.Exec(`INSERT INTO portal_exclusive_leases(channel_id,user_id,state,token_hash,token_secret,upstream_secret,expires_at,billing_checked_at) VALUES($1,7,'active','hash','secret','upstream',CURRENT_TIMESTAMP+interval '24 hours',CURRENT_TIMESTAMP)`, channelID); err != nil {
			t.Fatal(err)
		}
	}
	return s, core, db, channelID
}

func accountTestStart(s *Server, channelID, accountID int64, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.SetPathValue("channelID", strconv.FormatInt(channelID, 10))
	r.SetPathValue("accountID", strconv.FormatInt(accountID, 10))
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.exclusiveAccountTestStart(w, r)
	return w
}

func waitAccountTest(t *testing.T, s *Server, accountID int64, state string) exclusivePageTest {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		v, err := s.exclusiveAccountTestMetadata(context.Background(), accountID, true)
		if err != nil {
			t.Fatal(err)
		}
		if v.State == state {
			return v
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("account test did not reach " + state)
	return exclusivePageTest{}
}

func TestExclusiveAccountTestAdminCanStartWithoutLease(t *testing.T) {
	s, core, db, channelID := accountTestFixture(t, "available", false)
	if w := accountTestStart(s, channelID, 15, "admin"); w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	v := waitAccountTest(t, s, 15, "succeeded")
	if !strings.Contains(v.HTML, "account test") || core.rawCalls.Load() != 1 {
		t.Fatal(v, core.rawCalls.Load())
	}
	var requests int
	if err := db.QueryRow(`SELECT count(*) FROM portal_exclusive_requests`).Scan(&requests); err != nil || requests != 0 {
		t.Fatal("admin test should not create a lease request", requests, err)
	}
}

func TestExclusiveAccountTestAdminCanStartWhileChannelIsHeld(t *testing.T) {
	s, core, _, channelID := accountTestFixture(t, "active", true)
	if w := accountTestStart(s, channelID, 15, "admin"); w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	waitAccountTest(t, s, 15, "succeeded")
	if core.rawCalls.Load() != 1 {
		t.Fatal("missing administrator test request")
	}
}

func TestExclusiveAccountTestStillRejectsNonHolder(t *testing.T) {
	s, core, _, channelID := accountTestFixture(t, "active", true)
	if w := accountTestStart(s, channelID, 15, "other"); w.Code != http.StatusForbidden {
		t.Fatal(w.Code, w.Body.String())
	}
	if core.rawCalls.Load() != 0 {
		t.Fatal("unauthorized account test reached Core")
	}
}

func TestExclusiveAccountTestDuplicatePrecedesLeaseConcurrencyLimit(t *testing.T) {
	s, core, db, channelID := accountTestFixture(t, "active", true)
	var leaseID int64
	if err := db.QueryRow(`SELECT id FROM portal_exclusive_leases WHERE channel_id=$1`, channelID).Scan(&leaseID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO portal_exclusive_account_tests(account_id,channel_id,request_id,state) VALUES(15,$1,'existing','running')`, channelID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"one", "two", "three"} {
		if _, err := db.Exec(`INSERT INTO portal_exclusive_requests(id,lease_id,state) VALUES($1,$2,'running')`, id, leaseID); err != nil {
			t.Fatal(err)
		}
	}
	if w := accountTestStart(s, channelID, 15, "holder"); w.Code != http.StatusConflict {
		t.Fatal(w.Code, w.Body.String())
	}
	if core.rawCalls.Load() != 0 {
		t.Fatal("duplicate holder test reached Core")
	}
}

func TestExclusiveAccountTestAdminRejectsUnresolvedOrUnboundAccount(t *testing.T) {
	for _, state := range []string{"running", "uncertain"} {
		t.Run(state, func(t *testing.T) {
			s, core, db, channelID := accountTestFixture(t, "available", false)
			if _, err := db.Exec(`INSERT INTO portal_exclusive_account_tests(account_id,channel_id,request_id,state) VALUES(15,$1,'existing',$2)`, channelID, state); err != nil {
				t.Fatal(err)
			}
			if w := accountTestStart(s, channelID, 15, "admin"); w.Code != http.StatusConflict {
				t.Fatal(w.Code, w.Body.String())
			}
			if core.rawCalls.Load() != 0 {
				t.Fatal("duplicate account test reached Core")
			}
		})
	}

	s, core, db, channelID := accountTestFixture(t, "available", false)
	if _, err := db.Exec(`INSERT INTO portal_exclusive_accounts(core_account_id,channel_id) VALUES(99,$1)`, channelID); err != nil {
		t.Fatal(err)
	}
	if w := accountTestStart(s, channelID, 99, "admin"); w.Code != http.StatusNotFound {
		t.Fatal(w.Code, w.Body.String())
	}
	if core.rawCalls.Load() != 0 {
		t.Fatal("account outside the active channel configuration reached Core")
	}
}

func TestExclusiveAccountTestExpiresStaleRunAndReleasesLedger(t *testing.T) {
	s, _, db, channelID := accountTestFixture(t, "active", true)
	var leaseID int64
	if err := db.QueryRow(`SELECT id FROM portal_exclusive_leases WHERE channel_id=$1`, channelID).Scan(&leaseID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO portal_exclusive_requests(id,lease_id,state) VALUES('stale-test',$1,'running')`, leaseID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO portal_exclusive_account_tests(account_id,channel_id,request_id,state,started_at) VALUES(15,$1,'stale-test','running',CURRENT_TIMESTAMP-INTERVAL '7 minutes')`, channelID); err != nil {
		t.Fatal(err)
	}
	v, err := s.exclusiveAccountTestMetadata(context.Background(), 15, true)
	if err != nil {
		t.Fatal(err)
	}
	if v.State != "failed" || v.Error != exclusiveAccountTestInterrupted {
		t.Fatalf("stale test was not expired: %+v", v)
	}
	var state, message string
	if err := db.QueryRow(`SELECT state,error FROM portal_exclusive_requests WHERE id='stale-test'`).Scan(&state, &message); err != nil {
		t.Fatal(err)
	}
	if state != "finished" || message != exclusiveAccountTestInterrupted {
		t.Fatalf("stale ledger was not released: state=%q error=%q", state, message)
	}
}

func TestExclusiveAccountTestAllowsRetryAfterStaleRun(t *testing.T) {
	s, core, db, channelID := accountTestFixture(t, "available", false)
	if _, err := db.Exec(`INSERT INTO portal_exclusive_account_tests(account_id,channel_id,request_id,state,started_at) VALUES(15,$1,'stale-test','running',CURRENT_TIMESTAMP-INTERVAL '7 minutes')`, channelID); err != nil {
		t.Fatal(err)
	}
	if w := accountTestStart(s, channelID, 15, "admin"); w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	waitAccountTest(t, s, 15, "succeeded")
	if core.rawCalls.Load() != 1 {
		t.Fatal("retry did not reach Core")
	}
}
