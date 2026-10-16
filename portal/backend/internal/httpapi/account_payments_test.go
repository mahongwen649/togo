package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

func TestAccountPaymentMoneyAndFees(t *testing.T) {
	for _, raw := range []string{"", "-1", "NaN", "Infinity", "1e2", "0", "0.001", "1.999", "1000001", " 1", ".5", "1."} {
		if _, err := paymentCents(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	for raw, want := range map[string]int64{"1": 100, "10.01": 1001, "0.01": 1, "1000000.00": 100000000} {
		got, err := paymentCents(raw)
		if err != nil || got != want {
			t.Fatalf("%s: %d %v", raw, got, err)
		}
	}
	for _, c := range []struct {
		cents int64
		rate  string
		want  int64
	}{{10000, "0", 10000}, {10000, "2.5", 10250}, {101, "0.1", 102}, {999, "1.25", 1012}} {
		got, err := accountPaymentTotal(c.cents, c.rate)
		if err != nil || got != c.want {
			t.Fatalf("fee %+v: %d %v", c, got, err)
		}
	}
	for _, rate := range []string{"-1", "NaN", "101"} {
		if _, err := accountPaymentTotal(100, rate); err == nil {
			t.Fatal("invalid fee accepted")
		}
	}
}

func TestAccountPaymentSignatureAndSnapshot(t *testing.T) {
	v := url.Values{"pid": {"42"}, "money": {"10.00"}, "out_trade_no": {"acct_test"}, "name": {"账号套餐 A&B"}}
	v.Set("sign", accountPaymentSign(v, "test-key"))
	v.Set("sign_type", "MD5")
	if !validateAccountPaymentSignature(v, "test-key") {
		t.Fatal("signature rejected")
	}
	v.Set("money", "1.00")
	if validateAccountPaymentSignature(v, "test-key") {
		t.Fatal("tampered amount accepted")
	}
	v.Set("money", "10.00")
	v.Add("money", "10.00")
	if validateAccountPaymentSignature(v, "test-key") {
		t.Fatal("duplicate field accepted")
	}
	s := &Server{accountPayment: AccountPaymentConfig{Secret: strings.Repeat("x", 32)}}
	p := accountPaymentProvider{Key: "test-only-secret", PID: "42"}
	sealed, err := s.sealPayment(p, "acct_a")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(sealed), p.Key) {
		t.Fatal("plaintext key stored")
	}
	opened, err := s.openPayment(sealed, "acct_a")
	if err != nil || opened.Key != p.Key {
		t.Fatal("snapshot roundtrip failed")
	}
	if _, err = s.openPayment(sealed, "acct_b"); err == nil {
		t.Fatal("snapshot swapped between orders")
	}
	sealed[len(sealed)-1] ^= 1
	if _, err = s.openPayment(sealed, "acct_a"); err == nil {
		t.Fatal("tampered snapshot accepted")
	}
}

func accountPaymentTestServer(t *testing.T) *Server {
	t.Helper()
	db := rentalTestDB(t)
	_, err := db.Exec(`CREATE TABLE settings(key TEXT PRIMARY KEY,value TEXT); CREATE TABLE payment_provider_instances(id BIGINT PRIMARY KEY,provider_key TEXT,config TEXT,limits TEXT,supported_types TEXT,enabled BOOLEAN,sort_order INT); INSERT INTO settings VALUES('payment_enabled','true'),('payment_visible_method_alipay_enabled','true'),('payment_visible_method_alipay_source','easypay_alipay'),('RECHARGE_FEE_RATE','2.5'); INSERT INTO payment_provider_instances VALUES(1,'easypay','{"pid":"42","pkey":"test-key","apiBase":"https://gateway.example.test","cidAlipay":"8"}','{"alipay":{"singleMin":1,"singleMax":500}}','alipay',true,0)`)
	if err != nil {
		t.Fatal(err)
	}
	s := fakeRentalServer(db)
	s.corePaymentDB = db
	s.accountPayment = AccountPaymentConfig{Secret: strings.Repeat("x", 32), PublicURL: "https://portal.example.test"}
	return s
}

func newAccountPayment(t *testing.T, s *Server, body string) map[string]any {
	t.Helper()
	r := httptest.NewRequest("POST", "/api/v1/account-market/payments/orders", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	s.accountPaymentCreate(w, r)
	if w.Code != 200 {
		t.Fatalf("create failed %d %s", w.Code, w.Body.String())
	}
	var result struct {
		Data map[string]any `json:"data"`
	}
	if json.Unmarshal(w.Body.Bytes(), &result) != nil {
		t.Fatal("invalid response")
	}
	return result.Data
}

func accountNotification(t *testing.T, s *Server, trade string) url.Values {
	t.Helper()
	var pay int64
	if err := s.portalDB.QueryRow(`SELECT pay_cents FROM portal_account_payment_orders WHERE out_trade_no=$1`, trade).Scan(&pay); err != nil {
		t.Fatal(err)
	}
	v := url.Values{"pid": {"42"}, "out_trade_no": {trade}, "trade_no": {"gateway-" + trade}, "money": {centsText(pay)}, "type": {"alipay"}, "trade_status": {"TRADE_SUCCESS"}, "sign_type": {"MD5"}}
	v.Set("sign", accountPaymentSign(v, "test-key"))
	return v
}

func notifyAccount(s *Server, v url.Values) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/v1/account-market/payments/webhook/easypay", strings.NewReader(v.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	s.accountPaymentWebhook(w, r)
	return w
}

func TestAccountPaymentCheckoutAndIsolation(t *testing.T) {
	s := accountPaymentTestServer(t)
	o := newAccountPayment(t, s, `{"amount":100,"payment_type":"alipay","is_mobile":true}`)
	if o["amount"] != float64(100) || o["pay_amount"] != 102.5 {
		t.Fatalf("wrong pricing %+v", o)
	}
	u, err := url.Parse(o["pay_url"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if u.Host != "gateway.example.test" || u.Query().Get("notify_url") != "https://portal.example.test/api/v1/account-market/payments/webhook/easypay" || u.Query().Get("return_url") != "https://portal.example.test/payment/result" || u.Query().Get("cid") != "8" || len(u.Query().Get("out_trade_no")) > 32 {
		t.Fatal("incorrect gateway URLs")
	}
	if !validateAccountPaymentSignature(u.Query(), "test-key") {
		t.Fatal("invalid outbound signature")
	}
	var balance float64
	if err = s.portalDB.QueryRow(`SELECT balance FROM portal_account_wallets WHERE user_id=7`).Scan(&balance); err != nil || balance != 0 {
		t.Fatal("credited before payment")
	}
	// Core money/order tables do not exist in this isolated schema. Completion
	// must succeed using only the Portal wallet and must not call Core credit APIs.
	v := accountNotification(t, s, o["out_trade_no"].(string))
	w := notifyAccount(s, v)
	if w.Code != 200 || w.Body.String() != "success" {
		t.Fatalf("notify %d %s", w.Code, w.Body.String())
	}
	if err = s.portalDB.QueryRow(`SELECT balance FROM portal_account_wallets WHERE user_id=7`).Scan(&balance); err != nil || balance != 100 {
		t.Fatalf("wrong wallet %v %v", balance, err)
	}
	var ledger int
	if err = s.portalDB.QueryRow(`SELECT count(*) FROM portal_account_wallet_ledger WHERE reference_type='account_payment' AND amount=100`).Scan(&ledger); err != nil || ledger != 1 {
		t.Fatal("missing ledger")
	}
}

func TestAccountPaymentRejectsForgedNotifications(t *testing.T) {
	s := accountPaymentTestServer(t)
	o := newAccountPayment(t, s, `{"amount":10}`)
	trade := o["out_trade_no"].(string)
	for key, value := range map[string]string{"money": "1.00", "pid": "other", "type": "wxpay", "trade_status": "WAIT_BUYER_PAY", "trade_no": "", "out_trade_no": "acct_foreign"} {
		v := accountNotification(t, s, trade)
		v.Set(key, value)
		v.Set("sign", accountPaymentSign(v, "test-key"))
		if w := notifyAccount(s, v); w.Code == 200 {
			t.Fatalf("accepted mismatch %s", key)
		}
	}
	v := accountNotification(t, s, trade)
	v.Set("sign", "fake")
	if notifyAccount(s, v).Code == 200 {
		t.Fatal("accepted forged signature")
	}
	var balance float64
	if err := s.portalDB.QueryRow(`SELECT balance FROM portal_account_wallets WHERE user_id=7`).Scan(&balance); err != nil || balance != 0 {
		t.Fatal("invalid notification credited")
	}
}

func TestAccountPaymentDuplicateConcurrentAndLateCallbacks(t *testing.T) {
	s := accountPaymentTestServer(t)
	o := newAccountPayment(t, s, `{"amount":20}`)
	trade := o["out_trade_no"].(string)
	if _, err := s.portalDB.Exec(`UPDATE portal_account_payment_orders SET status='EXPIRED',expires_at=CURRENT_TIMESTAMP-INTERVAL '1 hour' WHERE out_trade_no=$1`, trade); err != nil {
		t.Fatal(err)
	}
	v := accountNotification(t, s, trade)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if w := notifyAccount(s, v); w.Code != 200 {
				t.Errorf("concurrent notify %d", w.Code)
			}
		}()
	}
	wg.Wait()
	var balance float64
	var ledger int
	if err := s.portalDB.QueryRow(`SELECT balance FROM portal_account_wallets WHERE user_id=7`).Scan(&balance); err != nil || balance != 20 {
		t.Fatalf("balance %v %v", balance, err)
	}
	if err := s.portalDB.QueryRow(`SELECT count(*) FROM portal_account_wallet_ledger`).Scan(&ledger); err != nil || ledger != 1 {
		t.Fatal("duplicate ledger")
	}
	// Configuration rotation does not invalidate the encrypted original snapshot.
	if _, err := s.corePaymentDB.Exec(`UPDATE payment_provider_instances SET config='{}',enabled=false`); err != nil {
		t.Fatal(err)
	}
	if w := notifyAccount(s, v); w.Code != 200 {
		t.Fatal("old order stopped verifying after rotation")
	}
}

func TestAccountPaymentReusedGatewayTradeRollsBack(t *testing.T) {
	s := accountPaymentTestServer(t)
	o1 := newAccountPayment(t, s, `{"amount":10}`)
	o2 := newAccountPayment(t, s, `{"amount":10}`)
	v1 := accountNotification(t, s, o1["out_trade_no"].(string))
	if notifyAccount(s, v1).Code != 200 {
		t.Fatal("first failed")
	}
	v2 := accountNotification(t, s, o2["out_trade_no"].(string))
	v2.Set("trade_no", v1.Get("trade_no"))
	v2.Set("sign", accountPaymentSign(v2, "test-key"))
	if notifyAccount(s, v2).Code == 200 {
		t.Fatal("reused gateway trade accepted")
	}
	var balance float64
	if err := s.portalDB.QueryRow(`SELECT balance FROM portal_account_wallets WHERE user_id=7`).Scan(&balance); err != nil || balance != 10 {
		t.Fatal("failed settlement did not roll back")
	}
}

func TestAccountPaymentOwnershipAndPendingLimit(t *testing.T) {
	s := accountPaymentTestServer(t)
	o := newAccountPayment(t, s, `{"amount":10}`)
	if _, err := s.portalDB.Exec(`UPDATE portal_account_payment_orders SET user_id=8 WHERE id=$1`, o["order_id"]); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/v1/account-market/payments/verify", strings.NewReader(fmt.Sprintf(`{"out_trade_no":%q}`, o["out_trade_no"])))
	r.Header.Set("Authorization", "Bearer test")
	s.accountPaymentVerify(w, r)
	if w.Code != 404 {
		t.Fatal("foreign order exposed")
	}
	for i := 0; i < 3; i++ {
		newAccountPayment(t, s, `{"amount":10}`)
	}
	w = httptest.NewRecorder()
	r = httptest.NewRequest("POST", "/", strings.NewReader(`{"amount":10}`))
	r.Header.Set("Authorization", "Bearer test")
	s.accountPaymentCreate(w, r)
	if w.Code != 429 {
		t.Fatal("pending limit bypassed")
	}
	w = httptest.NewRecorder()
	r = httptest.NewRequest("POST", "/", strings.NewReader(`{"amount":10}`))
	s.accountPaymentCreate(w, r)
	if w.Code != 401 {
		t.Fatal("unauthenticated order accepted")
	}
}

func TestAccountPaymentReconciliationWithoutBrowser(t *testing.T) {
	s := accountPaymentTestServer(t)
	var trade string
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Error("query must follow the gateway GET API contract")
		}
		_ = r.ParseForm()
		if r.Form.Get("key") != "test-key" || r.Form.Get("out_trade_no") != trade {
			t.Error("wrong gateway query")
		}
		_, _ = fmt.Fprintf(w, `{"code":1,"status":1,"money":"10.25","trade_no":"query-test","out_trade_no":%q,"pid":"42","type":"alipay"}`, trade)
	}))
	defer gateway.Close()
	config, _ := json.Marshal(map[string]string{"pid": "42", "pkey": "test-key", "apiBase": gateway.URL})
	if _, err := s.corePaymentDB.Exec(`UPDATE payment_provider_instances SET config=$1`, string(config)); err != nil {
		t.Fatal(err)
	}
	o := newAccountPayment(t, s, `{"amount":10}`)
	trade = o["out_trade_no"].(string)
	if _, err := s.portalDB.Exec(`UPDATE portal_account_payment_orders SET next_check_at=CURRENT_TIMESTAMP-INTERVAL '1 minute'`); err != nil {
		t.Fatal(err)
	}
	if err := s.reconcileAccountPayment(context.Background(), int64(o["order_id"].(float64))); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := s.portalDB.QueryRow(`SELECT status FROM portal_account_payment_orders WHERE out_trade_no=$1`, trade).Scan(&status); err != nil || status != "COMPLETED" {
		t.Fatalf("reconcile %s %v", status, err)
	}
}

func TestAccountPaymentMaximumAmountWithFee(t *testing.T) {
	s := accountPaymentTestServer(t)
	if _, err := s.corePaymentDB.Exec(`UPDATE payment_provider_instances SET limits='{}'; UPDATE settings SET value='100' WHERE key='RECHARGE_FEE_RATE'`); err != nil {
		t.Fatal(err)
	}
	o := newAccountPayment(t, s, `{"amount":1000000}`)
	if o["pay_amount"] != float64(2000000) {
		t.Fatal("maximum fee calculated incorrectly")
	}
	if w := notifyAccount(s, accountNotification(t, s, o["out_trade_no"].(string))); w.Code != 200 {
		t.Fatalf("maximum amount with fee rejected: %s", w.Body.String())
	}
	var balance float64
	if err := s.portalDB.QueryRow(`SELECT balance FROM portal_account_wallets WHERE user_id=7`).Scan(&balance); err != nil || balance != 1000000 {
		t.Fatalf("wrong credited face value: %v %v", balance, err)
	}
}

func TestAccountPaymentResumeOwnershipAndStatus(t *testing.T) {
	s := accountPaymentTestServer(t)
	o := newAccountPayment(t, s, `{"amount":10}`)
	get := func() *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("Authorization", "Bearer test-token")
		r.SetPathValue("orderID", fmt.Sprint(o["order_id"]))
		s.accountPaymentGet(w, r)
		return w
	}
	// Keep this read test independent of the external gateway query.
	if _, err := s.portalDB.Exec(`UPDATE portal_account_payment_orders SET next_check_at=CURRENT_TIMESTAMP+INTERVAL '1 hour'`); err != nil {
		t.Fatal(err)
	}
	w := get()
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"pay_url"`) || !strings.Contains(w.Body.String(), o["out_trade_no"].(string)) {
		t.Fatalf("pending resume missing: %s", w.Body.String())
	}
	if _, err := s.portalDB.Exec(`UPDATE portal_account_payment_orders SET user_id=8`); err != nil {
		t.Fatal(err)
	}
	if w = get(); w.Code != 404 || strings.Contains(w.Body.String(), "pay_url") {
		t.Fatal("foreign order payment URL exposed")
	}
	if _, err := s.portalDB.Exec(`UPDATE portal_account_payment_orders SET user_id=7,expires_at=CURRENT_TIMESTAMP-INTERVAL '1 hour'`); err != nil {
		t.Fatal(err)
	}
	if w = get(); w.Code != 200 || strings.Contains(w.Body.String(), "pay_url") {
		t.Fatal("expired order resumable")
	}
	if notifyAccount(s, accountNotification(t, s, o["out_trade_no"].(string))).Code != 200 {
		t.Fatal("settlement failed")
	}
	if w = get(); w.Code != 200 || strings.Contains(w.Body.String(), "pay_url") {
		t.Fatal("completed order resumable")
	}
}

func TestAccountPaymentQueryStatusConsistency(t *testing.T) {
	for _, test := range []struct {
		body string
		paid bool
	}{
		{`"status":1`, true},
		{`"trade_status":"TRADE_SUCCESS"`, true},
		{`"status":1,"trade_status":"TRADE_SUCCESS"`, true},
		{`"status":1,"trade_status":"TRADE_CLOSED"`, false},
		{`"status":0,"trade_status":"TRADE_SUCCESS"`, false},
		{`"status":0`, false},
		{`"status":null`, false},
	} {
		t.Run(test.body, func(t *testing.T) {
			gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = fmt.Fprintf(w, `{"code":1,"money":"10.00","trade_no":"gateway-test",%s}`, test.body)
			}))
			defer gateway.Close()
			_, paid, err := queryAccountPayment(context.Background(), accountPaymentProvider{Base: gateway.URL, PID: "42", Key: "test-key"}, "acct_test")
			if err != nil || paid != test.paid {
				t.Fatalf("paid=%v error=%v", paid, err)
			}
		})
	}
}

// Exercise public HTTP routes rather than invoking handlers directly.
func TestAccountPaymentHTTPCheckoutToWallet(t *testing.T) {
	s := accountPaymentTestServer(t)
	server := httptest.NewServer(New(s.core, nil, s.logger, NewPortalDatabase(s.portalDB), s.corePaymentDB, s.accountPayment))
	defer server.Close()
	request := func(method, path, body, contentType string, authenticated bool, want int) []byte {
		t.Helper()
		r, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if authenticated {
			r.Header.Set("Authorization", "Bearer local-test-token")
		}
		if contentType != "" {
			r.Header.Set("Content-Type", contentType)
		}
		response, err := server.Client().Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		data, err := io.ReadAll(response.Body)
		if err != nil || response.StatusCode != want {
			t.Fatalf("%s %s: status=%d body=%s error=%v", method, path, response.StatusCode, data, err)
		}
		return data
	}
	dataOf := func(raw []byte) map[string]any {
		t.Helper()
		var envelope struct {
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Data == nil {
			t.Fatalf("invalid API response: %s", raw)
		}
		return envelope.Data
	}
	const root = "/api/v1/account-market"
	request("GET", root+"/payments/checkout-info", "", "", false, http.StatusUnauthorized)
	checkout := dataOf(request("GET", root+"/payments/checkout-info", "", "", true, http.StatusOK))
	if checkout["wallet"] != "account" || checkout["balance_recharge_multiplier"] != float64(1) {
		t.Fatalf("wrong checkout destination: %+v", checkout)
	}
	order := dataOf(request("POST", root+"/payments/orders", `{"amount":20,"payment_type":"alipay"}`, "application/json", true, http.StatusOK))
	id := fmt.Sprint(order["order_id"])
	trade := order["out_trade_no"].(string)
	if order["pay_amount"] != float64(20.5) {
		t.Fatal("wrong payment total")
	}
	readWallet := func() float64 {
		return dataOf(request("GET", root+"/wallet", "", "", true, http.StatusOK))["balance"].(float64)
	}
	if readWallet() != 0 {
		t.Fatal("order creation credited the wallet")
	}
	request("GET", root+"/payments/orders/"+id, "", "", false, http.StatusUnauthorized)
	got := dataOf(request("GET", root+"/payments/orders/"+id, "", "", true, http.StatusOK))
	if got["status"] != "PENDING" || got["pay_url"] == nil {
		t.Fatal("pending order cannot be resumed")
	}
	verifyBody := fmt.Sprintf(`{"out_trade_no":%q}`, trade)
	got = dataOf(request("POST", root+"/payments/verify", verifyBody, "application/json", true, http.StatusOK))
	if got["status"] != "PENDING" {
		t.Fatal("unpaid order marked complete")
	}
	notification := accountNotification(t, s, trade)
	// ZPAY's documented callback has no pid and arrives via GET.
	notification.Del("pid")
	notification.Set("sign", accountPaymentSign(notification, "test-key"))
	validSignature := notification.Get("sign")
	notification.Set("sign", "invalid")
	request("POST", root+"/payments/webhook/easypay", notification.Encode(), "application/x-www-form-urlencoded", false, http.StatusBadRequest)
	if readWallet() != 0 {
		t.Fatal("invalid callback credited the wallet")
	}
	notification.Set("sign", validSignature)
	for i := 0; i < 2; i++ {
		body := request("GET", root+"/payments/webhook/easypay?"+notification.Encode(), "", "", false, http.StatusOK)
		if string(body) != "success" {
			t.Fatalf("gateway acknowledgment: %s", body)
		}
	}
	if readWallet() != 20 {
		t.Fatal("payment did not credit exactly once")
	}
	got = dataOf(request("POST", root+"/payments/verify", verifyBody, "application/json", true, http.StatusOK))
	if got["status"] != "COMPLETED" || got["pay_url"] != nil {
		t.Fatal("settled order not reflected in verification")
	}
	history := dataOf(request("GET", root+"/payments/orders", "", "", true, http.StatusOK))
	if history["total"] != float64(1) || history["items"].([]any)[0].(map[string]any)["status"] != "COMPLETED" {
		t.Fatal("settled order missing from history")
	}
	if _, err := s.corePaymentDB.Exec(`UPDATE payment_provider_instances SET enabled=false`); err != nil {
		t.Fatal(err)
	}
	request("GET", root+"/payments/checkout-info", "", "", true, http.StatusServiceUnavailable)
	request("POST", root+"/payments/orders", `{"amount":20}`, "application/json", true, http.StatusServiceUnavailable)
	if readWallet() != 20 {
		t.Fatal("disabling payments affected existing balance")
	}
}

func TestAccountPaymentLegacyProviderRouting(t *testing.T) {
	s := accountPaymentTestServer(t)
	if _, err := s.corePaymentDB.Exec(`UPDATE settings SET value='' WHERE key='payment_visible_method_alipay_source'; UPDATE settings SET value='false' WHERE key='payment_visible_method_alipay_enabled'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.loadAccountPaymentProvider(context.Background()); err != nil {
		t.Fatalf("legacy sole EasyPay provider unavailable: %v", err)
	}
	if _, err := s.corePaymentDB.Exec(`INSERT INTO payment_provider_instances VALUES(2,'alipay','{}','{}','alipay',true,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.loadAccountPaymentProvider(context.Background()); err == nil {
		t.Fatal("ambiguous legacy providers accepted")
	}
	if _, err := s.corePaymentDB.Exec(`UPDATE settings SET value='easypay_alipay' WHERE key='payment_visible_method_alipay_source'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.loadAccountPaymentProvider(context.Background()); err == nil {
		t.Fatal("explicit disabled source accepted")
	}
	if _, err := s.corePaymentDB.Exec(`UPDATE settings SET value='true' WHERE key='payment_visible_method_alipay_enabled'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.loadAccountPaymentProvider(context.Background()); err != nil {
		t.Fatal("explicit EasyPay source unavailable")
	}
}
