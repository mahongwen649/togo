package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

func TestExclusiveSelection(t *testing.T) {
	good := exclusivePublish{Name: "通道", AccountIDs: []int64{1, 2}, Durations: []int{30, 1440}, CooldownSeconds: 0}
	if err := validateExclusiveConfig(good, true); err != nil {
		t.Fatal(err)
	}
	for _, ids := range [][]int64{nil, {1, 1}, {0}, {-1}} {
		p := good
		p.AccountIDs = ids
		if validateExclusiveConfig(p, true) == nil {
			t.Fatalf("accepted %v", ids)
		}
	}
	p := good
	p.Durations = []int{30}
	if validateExclusiveConfig(p, true) == nil {
		t.Fatal("missing default accepted")
	}
	p = good
	p.CooldownSeconds = -1
	if validateExclusiveConfig(p, true) == nil {
		t.Fatal("negative cooldown")
	}
	if validateExclusiveConfig(good, false) != nil {
		t.Fatal("account append rejected")
	}
}
func TestExclusiveIdle(t *testing.T) {
	now := time.Now().UTC()
	l := exclusiveLease{StartedAt: now, ExpiresAt: now.Add(24 * time.Hour)}
	if exclusiveReason(l, now.Add(exclusiveIdle-time.Nanosecond)) != "" {
		t.Fatal("early idle")
	}
	if exclusiveReason(l, now.Add(exclusiveIdle)) != "idle" {
		t.Fatal("missing idle")
	}
	charge := now.Add(20 * time.Minute)
	l.LastChargeAt = &charge
	if exclusiveReason(l, now.Add(exclusiveIdle)) != "" {
		t.Fatal("charge did not reset")
	}
	l.ExpiresAt = now.Add(25 * time.Minute)
	if exclusiveReason(l, l.ExpiresAt) != "expired" {
		t.Fatal("expiry must win")
	}
}
func TestExclusiveBodyValidation(t *testing.T) {
	for _, body := range []string{`{"model":"gpt","stream":true}`, `{"background":false,"messages":[{"role":"user","content":"hi"}]}`} {
		if !validExclusiveBody([]byte(body)) {
			t.Fatalf("rejected %s", body)
		}
	}
	for _, body := range []string{`{"background":true}`, `{"background":null}`, `{"background":false,"background":true}`, `{"messages":[{"role":"user","role":"assistant"}]}`, `{} {}`, `[]`, `null`, `{`} {
		if validExclusiveBody([]byte(body)) {
			t.Fatalf("accepted %s", body)
		}
	}
}
func TestExclusiveCredentialGuardAndEncryption(t *testing.T) {
	key := exclusiveUpstreamPrefix + strings.Repeat("a", 64)
	for _, header := range []string{"Authorization", "x-api-key", "x-goog-api-key"} {
		r := httptest.NewRequest("POST", "/core-ingress/v1/responses", nil)
		v := key
		if header == "Authorization" {
			v = "bEaReR " + key
		}
		r.Header.Set(header, v)
		if !exclusiveManagedCredential(r) {
			t.Fatal(header)
		}
	}
	body := []byte(`{"data":{"api_key":{"key":"` + key + `"}},"message":"token ` + key + ` rejected"}`)
	if strings.Contains(string(redactExclusiveCredentials(body)), key) {
		t.Fatal("nested credential leaked")
	}
	s := &Server{exclusive: ExclusiveConfig{Secret: strings.Repeat("s", 32)}}
	encrypted, err := s.sealExclusive(key)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := s.openExclusive(encrypted)
	if err != nil || plain != key {
		t.Fatal("roundtrip", err)
	}
	s.exclusive.Secret = strings.Repeat("x", 32)
	if _, err = s.openExclusive(encrypted); err == nil {
		t.Fatal("wrong secret accepted")
	}
}

type disconnectedWriter struct{}

func (disconnectedWriter) Header() http.Header       { return make(http.Header) }
func (disconnectedWriter) WriteHeader(int)           {}
func (disconnectedWriter) Write([]byte) (int, error) { return 0, errors.New("client gone") }

type observedReader struct{ remaining int }

func (r *observedReader) Read(b []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	r.remaining--
	b[0] = 'x'
	return 1, nil
}
func TestExclusiveDrainsAfterDisconnect(t *testing.T) {
	r := &observedReader{remaining: 20}
	if !drainExclusiveResponse(disconnectedWriter{}, r) || r.remaining != 0 {
		t.Fatal("upstream was not drained")
	}
}

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func TestExclusiveBrokenUpstreamIsUncertain(t *testing.T) {
	if drainExclusiveResponse(httptest.NewRecorder(), brokenReader{}) {
		t.Fatal("broken upstream marked complete")
	}
}

func TestExclusiveDatabaseDrainAndUniqueness(t *testing.T) {
	db := rentalTestDB(t)
	ctx := context.Background()
	s := &Server{portalDB: db, logger: slog.Default()}
	var channel, lease int64
	if err := db.QueryRow(`INSERT INTO portal_exclusive_channels(name,state,account_ids,durations,cooldown_seconds) VALUES('test','active','[1]','[1440]',0) RETURNING id`).Scan(&channel); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO portal_exclusive_leases(channel_id,user_id,state,token_hash,token_secret,upstream_secret,expires_at,billing_checked_at) VALUES($1,7,'active','hash','secret','upstream',CURRENT_TIMESTAMP+interval '1 day',CURRENT_TIMESTAMP) RETURNING id`, channel).Scan(&lease); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO portal_exclusive_leases(channel_id,user_id,state,token_hash,token_secret,upstream_secret,expires_at) VALUES($1,8,'active','other','secret','upstream',CURRENT_TIMESTAMP+interval '1 day')`, channel); err == nil {
		t.Fatal("second holder allowed")
	}
	if _, err := db.Exec(`UPDATE portal_exclusive_leases SET started_at=CURRENT_TIMESTAMP-interval '31 minutes' WHERE id=$1`, lease); err != nil {
		t.Fatal(err)
	}
	if err := s.advanceExclusiveLease(ctx, lease, false); err != nil {
		t.Fatal(err)
	}
	var state string
	db.QueryRow(`SELECT state FROM portal_exclusive_leases WHERE id=$1`, lease).Scan(&state)
	if state != "active" {
		t.Fatal("billing outage must not infer idle")
	}
	if err := s.advanceExclusiveLease(ctx, lease, true); err != nil {
		t.Fatal(err)
	}
	db.QueryRow(`SELECT state FROM portal_exclusive_leases WHERE id=$1`, lease).Scan(&state)
	if state != "releasing" {
		t.Fatal(state)
	}
	if _, err := db.Exec(`INSERT INTO portal_exclusive_requests(id,lease_id,state) VALUES('request',$1,'uncertain')`, lease); err != nil {
		t.Fatal(err)
	}
	if err := s.advanceExclusiveLease(ctx, lease, true); err != nil {
		t.Fatal(err)
	}
	db.QueryRow(`SELECT state FROM portal_exclusive_leases WHERE id=$1`, lease).Scan(&state)
	if state != "releasing" {
		t.Fatal("uncertain request released")
	}
}

func TestExclusivePublicResponsesRedactCredentials(t *testing.T) {
	key := exclusiveUpstreamPrefix + strings.Repeat("a", 64)
	w := httptest.NewRecorder()
	writeSuccess(w, map[string]any{"usage": map[string]any{"key": key}, "balance": 12.25})
	var result map[string]any
	if json.Unmarshal(w.Body.Bytes(), &result) != nil || strings.Contains(w.Body.String(), key) {
		t.Fatal("public response leaked a credential or became invalid JSON")
	}
	escaped := `{"key":"\u0070xu_` + strings.Repeat("a", 64) + `"}`
	if strings.Contains(string(redactExclusiveCredentials([]byte(escaped))), strings.Repeat("a", 64)) {
		t.Fatal("escaped credential leaked")
	}
	r := httptest.NewRequest("GET", "/core-ingress/v1/models?key=ordinary&key="+key, nil)
	r.Header.Add("Authorization", "Bearer ordinary")
	r.Header.Add("Authorization", "Bearer "+key)
	if !exclusiveManagedCredential(r) {
		t.Fatal("duplicate credential fields bypassed ingress guard")
	}
}

func TestExclusivePublishRetryKeepsReservations(t *testing.T) {
	db := rentalTestDB(t)
	s := fakeRentalServer(db)
	s.exclusive = ExclusiveConfig{Enabled: true, IngressIsolated: true, PublicURL: "http://localhost", Secret: strings.Repeat("s", 32)}
	fake := s.core.(*rentalFakeCore)
	fake.failAccount = 2
	r := httptest.NewRequest("POST", "/api/v1/admin/exclusive-channels", strings.NewReader(`{"name":"test","account_ids":[1,2],"durations":[1440],"cooldown_seconds":0}`))
	r.Header.Set("Authorization", "Bearer admin")
	w := httptest.NewRecorder()
	s.exclusivePublishChannel(w, r)
	var channel int64
	var state string
	if err := db.QueryRow(`SELECT id,state FROM portal_exclusive_channels`).Scan(&channel, &state); err != nil {
		t.Fatal(err, w.Body.String())
	}
	if state != "error" {
		t.Fatalf("partial publish became %s", state)
	}
	var reserved int
	if err := db.QueryRow(`SELECT count(*) FROM portal_exclusive_accounts WHERE channel_id=$1`, channel).Scan(&reserved); err != nil || reserved != 2 {
		t.Fatal("partial publish lost account reservations", err)
	}
	if err := s.provisionExclusive(context.Background(), channel); err != nil {
		t.Fatal(err)
	}
	if fake.groupCreates != 1 {
		t.Fatal("retry created a duplicate group")
	}
	if len(fake.groups) != 1 || !fake.groups[0].AllowImageGeneration || fake.groups[0].AllowBatchImageGeneration {
		t.Fatal("new channel groups must enable image generation without enabling batch generation")
	}
	if err := db.QueryRow(`SELECT state FROM portal_exclusive_channels WHERE id=$1`, channel).Scan(&state); err != nil || state != "available" {
		t.Fatal("retry did not finish publication", state, err)
	}
}

type exclusiveCleanupCore struct {
	coreStub
	unbound int
	keys    []coreclient.APIKey
}

func (f *exclusiveCleanupCore) AdminMarketRequest(_ context.Context, method, path string, payload any, _ string) (coreclient.Response, error) {
	if method == "GET" && strings.HasPrefix(path, "/api/v1/admin/users/7/api-keys?") {
		return rentalResponse(map[string]any{"items": f.keys, "total": len(f.keys)})
	}
	if method != "PUT" || path != "/api/v1/admin/api-keys/19" || payload.(map[string]any)["group_id"] != 0 {
		return coreclient.Response{}, errors.New("unexpected cleanup operation")
	}
	f.unbound++
	return rentalResponse(map[string]any{"api_key": coreclient.APIKey{ID: 19}})
}

func TestExclusiveFailedClaimExpiryCleanup(t *testing.T) {
	for _, created := range []bool{false, true} {
		t.Run(map[bool]string{false: "before_create", true: "lost_create_response"}[created], func(t *testing.T) {
			db := rentalTestDB(t)
			fake := &exclusiveCleanupCore{}
			s := &Server{portalDB: db, core: fake, logger: testLogger(), exclusive: ExclusiveConfig{Secret: strings.Repeat("s", 32)}}
			key := exclusiveUpstreamPrefix + strings.Repeat("a", 64)
			secret, err := s.sealExclusive(key)
			if err != nil {
				t.Fatal(err)
			}
			if created {
				group := int64(3)
				fake.keys = []coreclient.APIKey{{ID: 19, Key: key, GroupID: &group}}
			}
			var channel, lease int64
			if err = db.QueryRow(`INSERT INTO portal_exclusive_channels(name,state,account_ids,durations,cooldown_seconds) VALUES('test','active','[1]','[1440]',0) RETURNING id`).Scan(&channel); err != nil {
				t.Fatal(err)
			}
			if err = db.QueryRow(`INSERT INTO portal_exclusive_leases(channel_id,user_id,state,token_hash,token_secret,upstream_secret,expires_at) VALUES($1,7,'provisioning','hash','secret',$2,CURRENT_TIMESTAMP-interval '1 second') RETURNING id`, channel, secret).Scan(&lease); err != nil {
				t.Fatal(err)
			}
			if err = s.advanceExclusiveLease(context.Background(), lease, false); err != nil {
				t.Fatal(err)
			}
			var state string
			if err = db.QueryRow(`SELECT state FROM portal_exclusive_leases WHERE id=$1`, lease).Scan(&state); err != nil || state != "closed" {
				t.Fatal(state, err)
			}
			if (fake.unbound == 1) != created {
				t.Fatal("lost create response was not reconciled")
			}
		})
	}
}

func TestExclusiveCleanupWaitsAndFreezesCooldown(t *testing.T) {
	db := rentalTestDB(t)
	fake := &exclusiveCleanupCore{}
	s := &Server{portalDB: db, core: fake, logger: testLogger()}
	var channel, lease int64
	if err := db.QueryRow(`INSERT INTO portal_exclusive_channels(name,state,account_ids,durations,cooldown_seconds) VALUES('test','releasing','[1]','[1440]',60) RETURNING id`).Scan(&channel); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO portal_exclusive_leases(channel_id,user_id,state,core_key_id,token_hash,token_secret,upstream_secret,expires_at) VALUES($1,7,'releasing',19,'hash','secret','upstream',CURRENT_TIMESTAMP) RETURNING id`, channel).Scan(&lease); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO portal_exclusive_requests(id,lease_id,state) VALUES('draining',$1,'running')`, lease); err != nil {
		t.Fatal(err)
	}
	if err := s.advanceExclusiveLease(context.Background(), lease, true); err != nil || fake.unbound != 0 {
		t.Fatal("revoked key before admitted request ended", err)
	}
	s.finishExclusiveRequest("draining", 200, true)
	if err := s.advanceExclusiveLease(context.Background(), lease, true); err != nil || fake.unbound != 1 {
		t.Fatal("cleanup did not revoke the old key", err)
	}
	var state string
	var deadline time.Time
	if err := db.QueryRow(`SELECT state,cooldown_until FROM portal_exclusive_channels WHERE id=$1`, channel).Scan(&state, &deadline); err != nil || state != "cooling" || time.Until(deadline) < 55*time.Second {
		t.Fatal("cooldown did not start after cleanup", state, err)
	}
	if _, err := db.Exec(`UPDATE portal_exclusive_channels SET cooldown_seconds=0 WHERE id=$1`, channel); err != nil {
		t.Fatal(err)
	}
	var unchanged time.Time
	if err := db.QueryRow(`SELECT cooldown_until FROM portal_exclusive_channels WHERE id=$1`, channel).Scan(&unchanged); err != nil || !unchanged.Equal(deadline) {
		t.Fatal("editing cooldown shortened current cooldown", err)
	}
}
