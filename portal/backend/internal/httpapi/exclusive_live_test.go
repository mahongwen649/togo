package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

// Opt-in integration against the canonical local Core. Only generated resources
// are changed; all inference terminates at this test's synthetic upstream.
func TestExclusiveAgainstLocalCore(t *testing.T) {
	testExclusiveAgainstLocalCore(t, false)
}

func TestExclusivePricingAgainstLocalCore(t *testing.T) {
	testExclusiveAgainstLocalCore(t, true)
}

func testExclusiveAgainstLocalCore(t *testing.T, pricing bool) {
	base, adminKey := os.Getenv("PORTAL_RENTAL_LIVE_CORE_URL"), os.Getenv("PORTAL_RENTAL_LIVE_CORE_KEY")
	if base == "" || adminKey == "" {
		t.Skip("local Core integration is not configured")
	}
	u, err := url.Parse(base)
	if err != nil || (u.Hostname() != "sub2api" && u.Hostname() != "host.docker.internal" && u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") {
		t.Fatal("integration is restricted to local Core")
	}
	upstreamBase := os.Getenv("PORTAL_EXCLUSIVE_TEST_UPSTREAM_URL")
	if upstreamBase != "http://portal-api:18181" {
		t.Fatal("synthetic upstream must use the test listener on portal-api:18181")
	}
	publicBase := os.Getenv("PORTAL_EXCLUSIVE_TEST_PUBLIC_URL")
	if pricing && publicBase != "" {
		t.Skip("pricing acceptance uses an isolated Portal database")
	}
	var db *sql.DB
	if publicBase != "" {
		if publicBase != "http://portal-web:8080" {
			t.Fatal("public integration must use canonical local Portal Web")
		}
		db, err = sql.Open("pgx", os.Getenv("PORTAL_RENTAL_TEST_DATABASE_URL"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
	} else {
		db = rentalTestDB(t)
	}
	adapter, err := coreclient.NewHTTPAdapter(base, &http.Client{Timeout: 15 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	adapter = adapter.WithAdminAPIKey(adminKey)
	s := &Server{portalDB: db, core: adapter, logger: testLogger(), exclusive: ExclusiveConfig{Enabled: true, IngressIsolated: true, PublicURL: "http://localhost", Secret: strings.Repeat("test-secret-", 4), DisableWorker: true}}
	if publicBase != "" {
		s.exclusive.Secret = os.Getenv("PORTAL_EXCLUSIVE_TEST_SECRET")
		if len(s.exclusive.Secret) < 32 {
			t.Fatal("public integration requires the local Portal encryption secret")
		}
	}
	ctx := context.Background()
	suffix, err := exclusiveRandom("")
	if err != nil {
		t.Fatal(err)
	}
	name := "exclusive-test-" + suffix[:16]
	password := suffix + "Test9!"
	var user coreclient.User
	if err = s.rentalCoreRequest(ctx, "POST", "/api/v1/admin/users", map[string]any{"email": name + "@example.invalid", "username": name, "password": password, "role": "user", "balance": 10, "concurrency": 3}, &user, ""); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := adapter.AdminDeleteUser(ctx, user.ID); err != nil {
			t.Error(err)
		}
	})
	login, err := adapter.Login(ctx, coreclient.LoginRequest{Email: name + "@example.invalid", Password: password})
	if err != nil || login.AccessToken == "" {
		t.Fatal("test user login failed", err)
	}

	entered, finish := make(chan struct{}), make(chan struct{})
	disconnectEntered := make(chan struct{})
	var enteredOnce, finishOnce, disconnectOnce sync.Once
	listener, err := net.Listen("tcp", ":18181")
	if err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-exclusive-synthetic-only" {
			http.Error(w, "invalid synthetic credential", 401)
			return
		}
		if r.URL.Path == "/v1/responses" {
			var input struct {
				Stream bool            `json:"stream"`
				Input  json.RawMessage `json:"input"`
			}
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				http.Error(w, "invalid request", 400)
				return
			}
			output := map[string]any{"id": "msg_exclusive_test", "type": "message", "role": "assistant", "status": "completed", "content": []any{map[string]any{"type": "output_text", "text": "test", "annotations": []any{}}}}
			response := map[string]any{"id": "resp_exclusive_test", "object": "response", "status": "completed", "model": "gpt-4o-mini", "output": []any{output}, "usage": map[string]any{"input_tokens": 100, "output_tokens": 50, "total_tokens": 150}}
			if pricing && strings.Contains(string(input.Input), "pricing-images") {
				images := []any{}
				for i, size := range []string{"1024x1024", "2048x2048", "4096x4096"} {
					images = append(images, map[string]any{"id": fmt.Sprintf("ig_test_%d", i), "type": "image_generation_call", "status": "completed", "result": fmt.Sprintf("synthetic-image-%d", i), "size": size})
				}
				response["id"], response["output"] = "resp_pricing_images", images
			}
			if !input.Stream {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			event := func(kind string, data map[string]any) {
				data["type"] = kind
				fmt.Fprintf(w, "event: %s\ndata: %s\n\n", kind, exclusiveJSON(data))
			}
			event("response.created", map[string]any{"response": map[string]any{"id": "resp_exclusive_test", "object": "response", "status": "in_progress", "model": "gpt-4o-mini", "output": []any{}}})
			event("response.output_item.added", map[string]any{"output_index": 0, "item": map[string]any{"id": "msg_exclusive_test", "type": "message", "role": "assistant", "status": "in_progress", "content": []any{}}})
			event("response.content_part.added", map[string]any{"item_id": "msg_exclusive_test", "output_index": 0, "content_index": 0, "part": map[string]any{"type": "output_text", "text": "", "annotations": []any{}}})
			event("response.output_text.delta", map[string]any{"item_id": "msg_exclusive_test", "output_index": 0, "content_index": 0, "delta": "test"})
			if pricing && strings.Contains(string(input.Input), "drain-test") {
				w.(http.Flusher).Flush()
				enteredOnce.Do(func() { close(entered) })
				select {
				case <-finish:
				case <-r.Context().Done():
					return
				}
			}
			event("response.output_text.done", map[string]any{"item_id": "msg_exclusive_test", "output_index": 0, "content_index": 0, "text": "test"})
			event("response.output_item.done", map[string]any{"output_index": 0, "item": output})
			event("response.completed", map[string]any{"response": response})
			return
		}
		if r.URL.Path != "/v1/chat/completions" {
			http.Error(w, "unexpected upstream route", 404)
			return
		}
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		var chat struct {
			Stream bool `json:"stream"`
		}
		if json.Unmarshal(payload, &chat) != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		if !chat.Stream {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"id":"chatcmpl-exclusive-test","object":"chat.completion","model":"gpt-4o-mini","choices":[{"index":0,"message":{"role":"assistant","content":"test"},"finish_reason":"stop"}],"usage":{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150}}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"id\":\"chatcmpl-exclusive-test\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-4o-mini\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"test\"},\"finish_reason\":null}]}\n\n")
		w.(http.Flusher).Flush()
		if strings.Contains(string(payload), "drain-test") || strings.Contains(string(payload), "disconnect-test") {
			if strings.Contains(string(payload), "disconnect-test") {
				disconnectOnce.Do(func() { close(disconnectEntered) })
			} else {
				enteredOnce.Do(func() { close(entered) })
			}
			select {
			case <-finish:
			case <-r.Context().Done():
				return
			}
		}
		fmt.Fprint(w, "data: {\"id\":\"chatcmpl-exclusive-test\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-4o-mini\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":100,\"completion_tokens\":50,\"total_tokens\":150}}\n\ndata: [DONE]\n\n")
	}))
	upstream.Listener.Close()
	upstream.Listener = listener
	upstream.Start()
	t.Cleanup(func() { finishOnce.Do(func() { close(finish) }); upstream.Close() })
	var account rentalAccount
	accountPayload := map[string]any{"name": name, "platform": "openai", "type": "apikey", "credentials": map[string]any{"api_key": "sk-exclusive-synthetic-only", "base_url": upstreamBase}, "concurrency": 3, "priority": 1, "group_ids": []int64{}}
	if pricing {
		accountPayload["extra"] = map[string]any{"openai_responses_mode": "force_responses"}
	}
	if err = s.rentalCoreRequest(ctx, "POST", "/api/v1/admin/accounts", accountPayload, &account, ""); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := adapter.AdminMarketRequest(ctx, "DELETE", fmt.Sprintf("/api/v1/admin/accounts/%d", account.ID), nil, ""); err != nil {
			t.Error(err)
		}
	})
	channelID := time.Now().UnixMilli()
	if publicBase == "" {
		if _, err = db.Exec(`INSERT INTO portal_exclusive_channels(id,name,state,account_ids,durations,cooldown_seconds) VALUES($1,$2,'provisioning',$3::jsonb,'[30,1440]',0)`, channelID, name, exclusiveJSON([]int64{account.ID})); err != nil {
			t.Fatal(err)
		}
		if pricing {
			if _, err = db.Exec(`UPDATE portal_exclusive_channels SET rate_multiplier=2,model_pricing='[{"model":"gpt-4o-mini","input_price":0.000002,"output_price":0.000004,"cache_read_price":0}]'::jsonb WHERE id=$1`, channelID); err != nil {
				t.Fatal(err)
			}
		}
	} else {
		var admin coreclient.User
		if err = s.rentalCoreRequest(ctx, "POST", "/api/v1/admin/users", map[string]any{"email": name + "a@example.invalid", "username": name + "a", "password": password, "role": "admin", "balance": 0, "concurrency": 1}, &admin, ""); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := s.rentalCoreRequest(ctx, "PUT", fmt.Sprintf("/api/v1/admin/users/%d", admin.ID), map[string]any{"role": "user"}, nil, ""); err != nil {
				t.Error(err)
				return
			}
			if err := adapter.AdminDeleteUser(ctx, admin.ID); err != nil {
				t.Error(err)
			}
		})
		adminLogin, err := adapter.Login(ctx, coreclient.LoginRequest{Email: name + "a@example.invalid", Password: password})
		if err != nil || adminLogin.AccessToken == "" {
			t.Fatal("test administrator login failed", err)
		}
		body := exclusiveJSON(map[string]any{"name": name, "account_ids": []int64{account.ID}, "durations": []int{30, 1440}, "cooldown_seconds": 0})
		response := exclusiveLivePublicRequest(publicBase, "POST", "/api/v1/admin/exclusive-channels", adminLogin.AccessToken, body)
		if err = db.QueryRow(`SELECT id FROM portal_exclusive_channels WHERE name=$1`, name).Scan(&channelID); err != nil {
			t.Fatalf("public publish HTTP %d, no channel: %v", response.Code, err)
		}
	}
	if publicBase != "" {
		t.Cleanup(func() {
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Error(err)
				return
			}
			defer tx.Rollback()
			var owned bool
			if err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM portal_exclusive_channels WHERE id=$1 AND name=$2)`, channelID, name).Scan(&owned); err != nil || !owned {
				t.Error("cannot verify generated channel ownership", err)
				return
			}
			for _, query := range []string{
				`DELETE FROM portal_exclusive_requests WHERE lease_id IN (SELECT id FROM portal_exclusive_leases WHERE channel_id=$1)`,
				`DELETE FROM portal_exclusive_events WHERE channel_id=$1`,
				`DELETE FROM portal_exclusive_leases WHERE channel_id=$1`,
				`DELETE FROM portal_exclusive_accounts WHERE channel_id=$1`,
				`DELETE FROM portal_exclusive_channels WHERE id=$1`,
			} {
				if _, err = tx.Exec(query, channelID); err != nil {
					t.Error(err)
					return
				}
			}
			if err = tx.Commit(); err != nil {
				t.Error(err)
			}
		})
	}
	if publicBase == "" {
		if _, err = db.Exec(`INSERT INTO portal_exclusive_accounts(core_account_id,channel_id) VALUES($1,$2)`, account.ID, channelID); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		var groups []rentalGroup
		if err := s.rentalCoreRequest(ctx, "GET", "/api/v1/admin/groups/all?include_inactive=true", nil, &groups, ""); err != nil {
			t.Error(err)
			return
		}
		for _, g := range groups {
			if g.Name == exclusiveGroupName(channelID) {
				if _, err := adapter.AdminMarketRequest(ctx, "DELETE", fmt.Sprintf("/api/v1/admin/groups/%d", g.ID), nil, ""); err != nil {
					t.Error(err)
				}
			}
		}
	})
	if publicBase == "" {
		if err = s.provisionExclusive(ctx, channelID); err != nil {
			t.Fatal("publish:", err)
		}
	} else {
		var state string
		if err = db.QueryRow(`SELECT state FROM portal_exclusive_channels WHERE id=$1`, channelID).Scan(&state); err != nil || state != "available" {
			t.Fatal("public publication did not finish", err)
		}
	}
	mux := http.NewServeMux()
	s.registerExclusive(mux)
	request := func(method, path, token, body string) *httptest.ResponseRecorder {
		if publicBase != "" {
			return exclusiveLivePublicRequest(publicBase, method, path, token, body)
		}
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	w := request("POST", fmt.Sprintf("/api/v1/exclusive-channels/%d/claim", channelID), login.AccessToken, `{"pricing_revision":1}`)
	if w.Code != 200 {
		t.Fatalf("claim HTTP %d: %s", w.Code, w.Body.String())
	}
	l, err := scanExclusiveLease(db.QueryRow(`SELECT `+exclusiveLeaseColumns+` FROM portal_exclusive_leases WHERE channel_id=$1`, channelID))
	if err != nil || l.State != "active" {
		t.Fatal("lease activation", err)
	}
	if d := l.ExpiresAt.Sub(l.StartedAt); d < 23*time.Hour || d > 25*time.Hour {
		t.Fatal("default duration changed")
	}
	clientKey, err := s.openExclusive(l.TokenSecret)
	if err != nil {
		t.Fatal(err)
	}
	internalKey, err := s.openExclusive(l.UpstreamSecret)
	if err != nil {
		t.Fatal(err)
	}
	if publicBase != "" {
		keys := request("GET", "/api/v1/keys", login.AccessToken, "")
		if keys.Code != 200 || strings.Contains(keys.Body.String(), internalKey) {
			t.Fatal("public key list leaked the internal credential")
		}
		var groupID int64
		if err = db.QueryRow(`SELECT core_group_id FROM portal_exclusive_channels WHERE id=$1`, channelID).Scan(&groupID); err != nil {
			t.Fatal(err)
		}
		if request("POST", "/api/v1/keys", login.AccessToken, exclusiveJSON(map[string]any{"name": "forbidden", "group_id": groupID})).Code != 403 {
			t.Fatal("user could create an alternate exclusive-group key")
		}
	}
	for _, key := range []string{clientKey, internalKey} {
		if request("GET", "/core-ingress/v1/models", key, "").Code != 403 {
			t.Fatal("managed credential bypassed ordinary ingress")
		}
	}
	keyRequest := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/keys/%d", *l.CoreKeyID), strings.NewReader(`{"group_id":0}`))
	if s.protectExclusiveUserAPI(httptest.NewRecorder(), keyRequest, login.AccessToken) {
		t.Fatal("user could alter managed Core key")
	}
	for _, sample := range []struct{ path, body, marker string }{
		{"/exclusive/v1/responses", `{"model":"gpt-4o-mini","input":"test","stream":false}`, `"completed"`},
		{"/exclusive/v1/responses", `{"model":"gpt-4o-mini","input":"test","stream":true}`, "response.completed"},
	} {
		response := request("POST", sample.path, clientKey, sample.body)
		if response.Code != 200 || !strings.Contains(response.Body.String(), sample.marker) {
			t.Fatalf("Responses contract failed: HTTP %d", response.Code)
		}
	}
	if pricing {
		response := request("POST", "/exclusive/v1/responses", clientKey, `{"model":"gpt-4o-mini","input":"pricing-images","tools":[{"type":"image_generation","model":"gpt-image-2"}],"stream":false}`)
		if response.Code != 200 || !strings.Contains(response.Body.String(), "ig_test_2") {
			t.Fatalf("image acceptance HTTP %d: %s", response.Code, response.Body.String())
		}
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	requestFinished := make(chan struct{})
	go func() {
		defer close(requestFinished)
		done <- request("POST", "/exclusive/v1/chat/completions", clientKey, `{"model":"gpt-4o-mini","stream":true,"messages":[{"role":"user","content":"drain-test"}]}`)
	}()
	t.Cleanup(func() {
		finishOnce.Do(func() { close(finish) })
		select {
		case <-requestFinished:
		case <-time.After(35 * time.Second):
			t.Error("test request did not finish during cleanup")
		}
	})
	select {
	case <-entered:
	case response := <-done:
		t.Fatalf("upstream not reached, HTTP %d: %s", response.Code, response.Body.String())
	case <-time.After(20 * time.Second):
		t.Fatal("synthetic upstream not reached")
	}
	wantTokens := int64(450)
	if pricing {
		wantTokens += 150
	}
	if publicBase != "" {
		r, err := http.NewRequest("POST", publicBase+"/exclusive/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o-mini","stream":true,"messages":[{"role":"user","content":"disconnect-test"}]}`))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Authorization", "Bearer "+clientKey)
		r.Header.Set("Content-Type", "application/json")
		response, err := (&http.Client{Timeout: 10 * time.Second}).Do(r)
		if err != nil {
			t.Fatal("disconnect request", err)
		}
		if response.StatusCode != 200 {
			response.Body.Close()
			t.Fatalf("disconnect request HTTP %d", response.StatusCode)
		}
		var firstByte [1]byte
		_, err = response.Body.Read(firstByte[:])
		response.Body.Close()
		if err != nil {
			t.Fatal("stream did not start before disconnect", err)
		}
		select {
		case <-disconnectEntered:
		case <-time.After(5 * time.Second):
			t.Fatal("disconnected request did not reach upstream")
		}
		wantTokens += 150
	}
	w = request("POST", fmt.Sprintf("/api/v1/exclusive-channels/%d/release", channelID), login.AccessToken, `{}`)
	if w.Code != 200 {
		t.Fatalf("release HTTP %d", w.Code)
	}
	if err = s.advanceExclusiveLease(ctx, l.ID, true); err != nil {
		t.Fatal(err)
	}
	var state string
	if err = db.QueryRow(`SELECT state FROM portal_exclusive_leases WHERE id=$1`, l.ID).Scan(&state); err != nil || state != "releasing" {
		t.Fatal("released before stream ended", err)
	}
	if request("POST", "/exclusive/v1/chat/completions", clientKey, `{"model":"gpt-4o-mini"}`).Code != 403 {
		t.Fatal("new request admitted during release")
	}
	finishOnce.Do(func() { close(finish) })
	select {
	case response := <-done:
		if response.Code != 200 || !strings.Contains(response.Body.String(), "[DONE]") {
			t.Fatalf("stream did not finish, HTTP %d", response.Code)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("stream did not drain")
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if err = s.syncExclusiveBilling(ctx, l); err != nil {
			t.Fatal(err)
		}
		l, err = scanExclusiveLease(db.QueryRow(`SELECT `+exclusiveLeaseColumns+` FROM portal_exclusive_leases WHERE id=$1`, l.ID))
		if err != nil {
			t.Fatal(err)
		}
		if l.Cost > 0 && l.Tokens == wantTokens {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if l.Cost <= 0 || l.LastChargeAt == nil || l.Tokens != wantTokens {
		t.Fatalf("Core actual billing missing: cost=%g tokens=%d", l.Cost, l.Tokens)
	}
	if err = s.advanceExclusiveLease(ctx, l.ID, true); err != nil {
		t.Fatal("cleanup:", err)
	}
	if err = db.QueryRow(`SELECT state FROM portal_exclusive_leases WHERE id=$1`, l.ID).Scan(&state); err != nil || state != "closed" {
		t.Fatal("cleanup incomplete", err)
	}
	profile, err := adapter.AdminUser(ctx, user.ID)
	if err != nil || profile.Balance >= 10 {
		t.Fatal("original user's relay balance was not charged", err)
	}
	if pricing {
		// Text uses rate 2; images keep the independent default rate 1.
		wantCost := 3*(100*.000002+50*.000004)*2 + 3*.05
		if math.Abs(l.Cost-wantCost) > 1e-8 || math.Abs((10-profile.Balance)-wantCost) > 1e-8 {
			t.Fatalf("pricing mismatch: usage=%g balance-delta=%g want=%g", l.Cost, 10-profile.Balance, wantCost)
		}
		t.Logf("verified Core settlement: 3 images * 0.05 * 1 + text at 2x = %.6f", wantCost)
	}
	var walletRows int
	if err = db.QueryRow(`SELECT count(*) FROM portal_account_wallets WHERE user_id=$1`, user.ID).Scan(&walletRows); err != nil || walletRows != 0 {
		t.Fatal("account-package wallet was used", err)
	}
	resp, err := adapter.UserAPIRequest(ctx, coreclient.CoreRequest{Method: "GET", Path: fmt.Sprintf("/api/v1/keys/%d", *l.CoreKeyID), AccessToken: login.AccessToken})
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Data coreclient.APIKey `json:"data"`
	}
	if json.Unmarshal(resp.Body, &data) != nil || pointerID(data.Data.GroupID) != 0 {
		t.Fatal("old Core key remained bound")
	}
	t.Log("real Core: publish, claim, guarded ingress, streaming drain, relay billing and cleanup passed")
}

func exclusiveLivePublicRequest(base, method, path, token, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r, err := http.NewRequest(method, base+path, strings.NewReader(body))
	if err != nil {
		w.WriteHeader(599)
		return w
	}
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(r)
	if err != nil {
		w.WriteHeader(599)
		return w
	}
	defer response.Body.Close()
	w.WriteHeader(response.StatusCode)
	if _, err = io.Copy(w, response.Body); err != nil {
		w.Code = 599
	}
	return w
}
