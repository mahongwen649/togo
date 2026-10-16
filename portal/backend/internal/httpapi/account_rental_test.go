package httpapi

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
	"github.com/jqcode/portal/backend/internal/storage"
)

func TestRentalSelectionValidation(t *testing.T) {
	for _, ids := range [][]int64{nil, {1}, {1, 1}, {1, 0}, {-1, 2}} {
		if validateRentalSelection(ids, 2) == nil {
			t.Fatalf("accepted invalid IDs %v", ids)
		}
	}
	if err := validateRentalSelection([]int64{2, 1}, 2); err != nil {
		t.Fatal(err)
	}
	if !sameRentalSelection([]int64{1, 2}, []int64{2, 1}) {
		t.Fatal("order should not affect selection equality")
	}
}

func TestRentalAccountIsolation(t *testing.T) {
	expires := time.Now().Add(30 * 24 * time.Hour)
	good := rentalAccount{ID: 1, Status: "active", Platform: "openai", Schedulable: true}
	if err := validateRentalAccount(good, 2, expires); err != nil {
		t.Fatal(err)
	}
	cases := []rentalAccount{good, good, good, good, good}
	cases[0].GroupIDs = []int64{99}
	cases[1].Status = "inactive"
	cases[2].Platform = "anthropic"
	early := expires.Add(-time.Hour).Unix()
	cases[3].ExpiresAt = &early
	parent := int64(2)
	cases[4].ParentAccountID = &parent
	for _, a := range cases {
		if validateRentalAccount(a, 2, expires) == nil {
			t.Fatalf("accepted unsafe account: %+v", a)
		}
	}
}

func TestExclusiveAPIKeyAccountIgnoresSubscriptionExpiry(t *testing.T) {
	now := time.Now()
	expired := now.Add(-time.Hour).Unix()
	apiKey := rentalAccount{ID: 1, Status: "active", Platform: "openai", Type: "apikey", Schedulable: true, GroupIDs: []int64{99}, ExpiresAt: &expired}
	if err := validateExclusiveAccount(apiKey, 0, now); err != nil {
		t.Fatalf("API Key account was filtered by subscription expiry or shared groups: %v", err)
	}

	oauth := apiKey
	oauth.Type = "oauth"
	if validateExclusiveAccount(oauth, 0, now) == nil {
		t.Fatal("expired or shared OAuth account should remain ineligible")
	}
}

type rentalFakeCore struct {
	coreStub
	accounts                            map[int64]rentalAccount
	groups                              []rentalGroup
	subscriptions                       []map[string]any
	failAccount                         int64
	lostGroupResponse                   bool
	groupCreates, groupDeletes, assigns int
	lastAssignValidity                  int
	role                                string
}

func (f *rentalFakeCore) Profile(context.Context, string) (coreclient.User, error) {
	role := f.role
	if role == "" {
		role = "admin"
	}
	return coreclient.User{ID: 7, Role: role, Status: "active"}, nil
}
func rentalResponse(data any) (coreclient.Response, error) {
	raw, err := json.Marshal(map[string]any{"code": 0, "data": data})
	return coreclient.Response{StatusCode: 200, Body: raw}, err
}
func (f *rentalFakeCore) AdminMarketRequest(_ context.Context, method, path string, payload any, key string) (coreclient.Response, error) {
	u, _ := url.Parse(path)
	switch {
	case method == "GET" && u.Path == "/api/v1/admin/groups/all":
		return rentalResponse(f.groups)
	case method == "POST" && u.Path == "/api/v1/admin/groups":
		raw, _ := json.Marshal(payload)
		var g rentalGroup
		_ = json.Unmarshal(raw, &g)
		g.ID = int64(len(f.groups) + 100)
		g.Status = "active"
		f.groups = append(f.groups, g)
		f.groupCreates++
		if f.lostGroupResponse {
			f.lostGroupResponse = false
			return coreclient.Response{}, errors.New("lost group response")
		}
		return rentalResponse(g)
	case method == "DELETE" && strings.HasPrefix(u.Path, "/api/v1/admin/groups/"):
		var id int64
		_, _ = fmt.Sscanf(u.Path, "/api/v1/admin/groups/%d", &id)
		for i, g := range f.groups {
			if g.ID == id {
				f.groups = append(f.groups[:i], f.groups[i+1:]...)
				f.groupDeletes++
				return rentalResponse(map[string]any{"id": id})
			}
		}
	case method == "GET" && strings.HasPrefix(u.Path, "/api/v1/admin/groups/"):
		var id int64
		_, _ = fmt.Sscanf(u.Path, "/api/v1/admin/groups/%d", &id)
		for _, g := range f.groups {
			if g.ID == id {
				return rentalResponse(g)
			}
		}
	case strings.HasPrefix(u.Path, "/api/v1/admin/accounts/"):
		var id int64
		_, _ = fmt.Sscanf(u.Path, "/api/v1/admin/accounts/%d", &id)
		a, ok := f.accounts[id]
		if !ok {
			return coreclient.Response{}, errors.New("missing account")
		}
		if method == "PUT" {
			if f.failAccount == id {
				f.failAccount = 0
				return coreclient.Response{}, errors.New("simulated account update failure")
			}
			raw, _ := json.Marshal(payload)
			_ = json.Unmarshal(raw, &a)
			f.accounts[id] = a
		}
		return rentalResponse(a)
	case u.Path == "/api/v1/admin/accounts" && method == "GET":
		items := []map[string]any{}
		for _, a := range f.accounts {
			if group := u.Query().Get("group"); group != "" {
				id, _ := strconv.ParseInt(group, 10, 64)
				if !slices.Contains(a.GroupIDs, id) {
					continue
				}
			}
			raw, _ := json.Marshal(a)
			var v map[string]any
			_ = json.Unmarshal(raw, &v)
			v["credentials"] = map[string]string{"access_token": "must-not-leak"}
			items = append(items, v)
		}
		return rentalResponse(map[string]any{"items": items, "total": len(items), "page": 1, "pages": 1})
	case u.Path == "/api/v1/admin/subscriptions" && method == "GET":
		return rentalResponse(map[string]any{"items": f.subscriptions})
	case u.Path == "/api/v1/admin/subscriptions/assign":
		v := payload.(map[string]any)
		validityDays, _ := v["validity_days"].(int)
		f.lastAssignValidity = validityDays
		v["id"] = int64(500)
		v["status"] = "active"
		v["expires_at"] = time.Now().Add(time.Duration(validityDays) * 24 * time.Hour)
		f.subscriptions = append(f.subscriptions, v)
		f.assigns++
		return rentalResponse(v)
	}
	return coreclient.Response{}, fmt.Errorf("unexpected mock request %s %s", method, path)
}

func rentalTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("PORTAL_RENTAL_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("PORTAL_RENTAL_TEST_DATABASE_URL not set")
	}
	root, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	schema := "rental_test_" + hex.EncodeToString(suffix[:])
	if _, err := root.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close(); _, _ = root.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); _ = root.Close() })
	if err := storage.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}
func seedRental(t *testing.T, db *sql.DB, user int64, quantity int) int64 {
	t.Helper()
	_, err := db.Exec(`INSERT INTO portal_account_products(id,name,mode,validity_days,unit_price,stock_quantity,locked_quantity) VALUES(1,'Plus','rental',30,10,20,$1) ON CONFLICT(id) DO UPDATE SET locked_quantity=portal_account_products.locked_quantity+EXCLUDED.locked_quantity`, quantity)
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	err = db.QueryRow(`INSERT INTO portal_account_orders(user_id,product_id,product_name,mode,validity_days,quantity,unit_price,total_price) VALUES($1,1,'Plus','rental',30,$2,10,$2*10) RETURNING id`, user, quantity).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func fakeRentalServer(db *sql.DB) *Server {
	return &Server{portalDB: db, logger: testLogger(), core: &rentalFakeCore{accounts: map[int64]rentalAccount{1: {ID: 1, Name: "Account 1", Platform: "openai", Status: "active", Schedulable: true}, 2: {ID: 2, Name: "Account 2", Platform: "openai", Status: "active", Schedulable: true}, 3: {ID: 3, Platform: "openai", Status: "active", Schedulable: true}}, groups: []rentalGroup{}}}
}

func TestEnsureRentalGroupEnablesImageGeneration(t *testing.T) {
	core := &rentalFakeCore{accounts: map[int64]rentalAccount{}, groups: []rentalGroup{}}
	s := &Server{core: core}

	group, err := s.ensureRentalGroup(context.Background(), rentalOrder{UserID: 7, ProductID: 11}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !group.AllowImageGeneration {
		t.Fatal("subscription rental group did not enable image generation")
	}
	if group.AllowBatchImageGeneration {
		t.Fatal("OpenAI rental group unexpectedly enabled Gemini-only batch image generation")
	}
}

func TestRentalFulfillmentRetryAndReuse(t *testing.T) {
	db := rentalTestDB(t)
	s := fakeRentalServer(db)
	core := s.core.(*rentalFakeCore)
	id := seedRental(t, db, 7, 2)
	core.failAccount = 2
	if _, err := s.provisionRental(context.Background(), id, []int64{1, 2}); err == nil {
		t.Fatal("expected upstream failure")
	}
	o, err := loadRentalOrder(context.Background(), db, id, false)
	if err != nil {
		t.Fatal(err)
	}
	if o.Status != "pending" || !o.StartedAt.Valid || !o.GroupID.Valid {
		t.Fatalf("missing durable checkpoint: %+v", o)
	}
	if core.assigns != 0 {
		t.Fatal("authorized user before all account updates succeeded")
	}
	if _, err := s.provisionRental(context.Background(), id, []int64{1, 3}); err == nil {
		t.Fatal("allowed changing a partial allocation")
	}
	o, err = s.provisionRental(context.Background(), id, []int64{2, 1})
	if err != nil {
		t.Fatal(err)
	}
	if o.Status != "completed" || core.groupCreates != 1 || core.assigns != 1 {
		t.Fatalf("bad completion %+v", o)
	}
	for _, id := range []int64{1, 2} {
		a := core.accounts[id]
		if a.ExpiresAt == nil || *a.ExpiresAt != o.ExpiresAt.Time.Unix() || !a.AutoPauseOnExpired || len(a.GroupIDs) != 1 || a.GroupIDs[0] != o.GroupID.Int64 {
			t.Fatalf("missing restrictions: %+v", a)
		}
	}
	if _, err = s.provisionRental(context.Background(), id, []int64{1, 2}); err != nil {
		t.Fatal(err)
	}
	var stock, locked int
	if err := db.QueryRow(`SELECT stock_quantity,locked_quantity FROM portal_account_products WHERE id=1`).Scan(&stock, &locked); err != nil {
		t.Fatal(err)
	}
	if stock != 18 || locked != 0 {
		t.Fatalf("stock consumed more than once: %d/%d", stock, locked)
	}
	next := seedRental(t, db, 7, 1)
	// Existing authorization covers this test order, so no new subscription.
	core.subscriptions[0]["expires_at"] = time.Now().Add(31 * 24 * time.Hour)
	nextOrder, err := s.provisionRental(context.Background(), next, []int64{3})
	if err != nil {
		t.Fatal(err)
	}
	if nextOrder.GroupID.Int64 != o.GroupID.Int64 || core.groupCreates != 1 || core.assigns != 1 {
		t.Fatal("did not reuse user/type group")
	}
}

func TestRentalLostGroupResponseRecovery(t *testing.T) {
	db := rentalTestDB(t)
	s := fakeRentalServer(db)
	core := s.core.(*rentalFakeCore)
	core.lostGroupResponse = true
	id := seedRental(t, db, 7, 1)
	if _, err := s.provisionRental(context.Background(), id, []int64{1}); err == nil {
		t.Fatal("expected lost response")
	}
	if _, err := s.provisionRental(context.Background(), id, []int64{1}); err != nil {
		t.Fatal(err)
	}
	if core.groupCreates != 1 {
		t.Fatal("created duplicate group on retry")
	}
}

func TestRentalCustomValidityIsApplied(t *testing.T) {
	db := rentalTestDB(t)
	s := fakeRentalServer(db)
	core := s.core.(*rentalFakeCore)
	id := seedRental(t, db, 7, 1)
	if _, err := db.Exec(`UPDATE portal_account_orders SET validity_days=15 WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	before := time.Now().UTC()
	order, err := s.provisionRental(context.Background(), id, []int64{1})
	if err != nil {
		t.Fatal(err)
	}
	if core.lastAssignValidity != 15 {
		t.Fatalf("Core validity=%d want 15", core.lastAssignValidity)
	}
	remaining := order.ExpiresAt.Time.Sub(before)
	if remaining < 15*24*time.Hour-time.Minute || remaining > 15*24*time.Hour+time.Minute {
		t.Fatalf("custom expiry remaining=%s", remaining)
	}
}

func TestAutoRenewOnlyAllowsThirtyDayRental(t *testing.T) {
	db := rentalTestDB(t)
	s := fakeRentalServer(db)
	id := seedRental(t, db, 7, 1)
	if _, err := db.Exec(`UPDATE portal_account_orders SET status='completed', validity_days=15, expires_at=CURRENT_TIMESTAMP + INTERVAL '15 days' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	handler := New(s.core, nil, testLogger(), NewPortalDatabase(db))
	request := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/account-market/orders/%d/auto-renew", id), strings.NewReader(`{"enabled":true}`))
		req.Header.Set("Authorization", "Bearer test")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		return recorder
	}
	if recorder := request(); recorder.Code != http.StatusNotFound {
		t.Fatalf("custom validity enabled auto renew: %d %s", recorder.Code, recorder.Body.String())
	}
	if _, err := db.Exec(`UPDATE portal_account_orders SET validity_days=30 WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if recorder := request(); recorder.Code != http.StatusOK {
		t.Fatalf("30-day auto renew rejected: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestRentalRejectsReservedAccountAndCancellation(t *testing.T) {
	db := rentalTestDB(t)
	s := fakeRentalServer(db)
	id := seedRental(t, db, 7, 1)
	other := seedRental(t, db, 8, 1)
	if _, err := db.Exec(`INSERT INTO portal_account_rental_reservations(core_account_id,order_id) VALUES(1,$1)`, other); err != nil {
		t.Fatal(err)
	}
	if _, err := s.provisionRental(context.Background(), id, []int64{1}); err == nil {
		t.Fatal("accepted another order's reservation")
	}
	if _, err := db.Exec(`UPDATE portal_account_orders SET fulfillment_started_at=CURRENT_TIMESTAMP WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	handler := New(s.core, nil, testLogger(), NewPortalDatabase(db))
	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/admin/account-market/orders/%d", id), strings.NewReader(`{"status":"cancelled"}`))
	req.Header.Set("Authorization", "Bearer test")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != 409 {
		t.Fatalf("cancelled partial allocation: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestRentalAccountPickerAuthAndRedaction(t *testing.T) {
	db := rentalTestDB(t)
	s := fakeRentalServer(db)
	core := s.core.(*rentalFakeCore)
	handler := New(core, nil, testLogger(), NewPortalDatabase(db))
	req := httptest.NewRequest("GET", "/api/v1/admin/account-market/core-accounts", nil)
	req.Header.Set("Authorization", "Bearer test")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != 200 || strings.Contains(recorder.Body.String(), "must-not-leak") || strings.Contains(recorder.Body.String(), "credentials") {
		t.Fatalf("unsafe picker: %s", recorder.Body.String())
	}
	core.role = "user"
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("non-admin accepted: %d", recorder.Code)
	}
}

func TestAccountCheckoutAndRefundUseExactPortalBalance(t *testing.T) {
	db := rentalTestDB(t)
	s := fakeRentalServer(db)
	if _, err := db.Exec(`INSERT INTO portal_account_products(id,name,mode,unit_price,stock_quantity) VALUES(1,'Plus','purchase',0.1,3)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO portal_account_wallets(user_id,balance) VALUES(7,0.3)`); err != nil {
		t.Fatal(err)
	}
	handler := New(s.core, nil, testLogger(), NewPortalDatabase(db))
	request := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer test")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	w := request("POST", "/api/v1/account-market/orders", `{"product_id":1,"quantity":3}`)
	if w.Code != 200 {
		t.Fatalf("exact balance rejected: %d %s", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	var amount string
	if err := db.QueryRow(`SELECT balance::text FROM portal_account_wallets WHERE user_id=7`).Scan(&amount); err != nil {
		t.Fatal(err)
	}
	if amount != "0.00000000" {
		t.Fatalf("incorrect debit %s", amount)
	}
	path := fmt.Sprintf("/api/v1/admin/account-market/orders/%d", body.Data.ID)
	w = request("PUT", path, `{"status":"cancelled"}`)
	if w.Code != 200 {
		t.Fatalf("refund failed %s", w.Body.String())
	}
	w = request("PUT", path, `{"status":"cancelled"}`)
	if w.Code != 409 {
		t.Fatal("duplicate refund accepted")
	}
	if err := db.QueryRow(`SELECT balance::text FROM portal_account_wallets WHERE user_id=7`).Scan(&amount); err != nil {
		t.Fatal(err)
	}
	if amount != "0.30000000" {
		t.Fatalf("incorrect refund %s", amount)
	}
}

func TestAccountMarketWithoutPortalDBIsUnavailable(t *testing.T) {
	s := fakeRentalServer(nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/account-market/wallet", nil)
	r.Header.Set("Authorization", "Bearer test")
	New(s.core, nil, testLogger()).ServeHTTP(w, r)
	if w.Code != 503 {
		t.Fatalf("missing Portal DB silently succeeded: %d", w.Code)
	}
}
