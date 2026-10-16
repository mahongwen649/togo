package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

type accountAdminCore struct {
	*rentalFakeCore
	users map[int64]coreclient.User
}

func (f *accountAdminCore) AdminUser(_ context.Context, userID int64) (coreclient.User, error) {
	user, ok := f.users[userID]
	if !ok {
		return coreclient.User{}, coreclient.ErrNotFound
	}
	return user, nil
}

func (f *accountAdminCore) AdminUsers(_ context.Context, page, pageSize int, options ...coreclient.AdminUsersOption) (coreclient.UserPage, error) {
	query := url.Values{}
	for _, option := range options {
		option(query)
	}
	search := strings.ToLower(query.Get("search"))
	items := make([]coreclient.User, 0)
	for _, user := range f.users {
		text := strings.ToLower(fmt.Sprintf("%d %s %s", user.ID, user.Email, user.Username))
		if search == "" || strings.Contains(text, search) {
			items = append(items, user)
		}
	}
	return coreclient.UserPage{Items: items, Page: page, PageSize: pageSize, Pages: 1, Total: int64(len(items))}, nil
}

func (f *accountAdminCore) AdminMarketRequest(ctx context.Context, method, path string, payload any, key string) (coreclient.Response, error) {
	if method == http.MethodGet && strings.HasPrefix(path, "/api/v1/admin/users/") && strings.Contains(path, "/usage") {
		return rentalResponse(map[string]any{"total_requests": 12, "total_cost": 3.5, "total_tokens": 456})
	}
	return f.rentalFakeCore.AdminMarketRequest(ctx, method, path, payload, key)
}

func accountAdminTestHandler(t *testing.T) http.Handler {
	t.Helper()
	db := rentalTestDB(t)
	_, err := db.Exec(`
		INSERT INTO portal_account_products(id,name,mode,validity_days,unit_price,stock_quantity,locked_quantity) VALUES(1,'30天套餐','rental',30,20,10,1);
		INSERT INTO portal_account_wallets(user_id,balance) VALUES(8,88.5);
		INSERT INTO portal_account_orders(user_id,product_id,product_name,mode,validity_days,quantity,unit_price,total_price,status,expires_at,core_account_ids,auto_renew,fulfillment_started_at)
		VALUES(8,1,'30天套餐','rental',30,1,20,20,'completed',CURRENT_TIMESTAMP+INTERVAL '20 days','[11]'::jsonb,true,CURRENT_TIMESTAMP);
		INSERT INTO portal_account_payment_orders(user_id,out_trade_no,amount_cents,pay_cents,fee_rate,provider_id,merchant_id,gateway_origin,provider_snapshot,trade_no,status,expires_at,completed_at)
		VALUES(8,'acct_admin_test',5000,5100,2,1,'42','https://pay.test',decode('00','hex'),'gateway-1','COMPLETED',CURRENT_TIMESTAMP+INTERVAL '1 hour',CURRENT_TIMESTAMP);
		INSERT INTO portal_account_redeem_batches(id,name,amount,quantity,note,created_by_admin_id) VALUES(1,'测试批次',25,1,'',7);
		INSERT INTO portal_account_redeem_codes(batch_id,code_hash,code_suffix,amount,remaining_amount,status,redeemed_user_id,redeemed_at)
		VALUES(1,'hash-admin-test','ABCD',25,25,'redeemed',8,CURRENT_TIMESTAMP);
		INSERT INTO portal_account_wallet_ledger(user_id,kind,amount,balance_before,balance_after,reference_type,note)
		VALUES(8,'purchase',-20,108.5,88.5,'account_order','Account product purchase');`)
	if err != nil {
		t.Fatal(err)
	}
	base := &rentalFakeCore{accounts: map[int64]rentalAccount{}, groups: []rentalGroup{}}
	core := &accountAdminCore{rentalFakeCore: base, users: map[int64]coreclient.User{
		8: {ID: 8, Email: "member@example.com", Username: "member", Role: "user", Status: "active", Balance: 17.25},
	}}
	return New(core, nil, testLogger(), NewPortalDatabase(db))
}

func adminAccountRequest(handler http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer admin")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func adminAccountPost(handler http.Handler, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer admin")
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func TestAccountAdminTransactionsAndUserOverview(t *testing.T) {
	handler := accountAdminTestHandler(t)

	payment := adminAccountRequest(handler, "/api/v1/admin/account-market/transactions?type=payment&search=member%40example.com")
	if payment.Code != http.StatusOK || !strings.Contains(payment.Body.String(), "acct_admin_test") || !strings.Contains(payment.Body.String(), "member@example.com") {
		t.Fatalf("payment records failed: %d %s", payment.Code, payment.Body.String())
	}

	redemption := adminAccountRequest(handler, "/api/v1/admin/account-market/transactions?type=redemption&search=member")
	if redemption.Code != http.StatusOK || !strings.Contains(redemption.Body.String(), "ABCD") || !strings.Contains(redemption.Body.String(), "测试批次") {
		t.Fatalf("redemption records failed: %d %s", redemption.Code, redemption.Body.String())
	}

	credit := adminAccountPost(handler, "/api/v1/admin/account-market/users/8/balance", `{"amount":12.5,"note":"售后补偿"}`)
	if credit.Code != http.StatusOK || !strings.Contains(credit.Body.String(), `"balance_after":101`) {
		t.Fatalf("manual credit failed: %d %s", credit.Code, credit.Body.String())
	}
	invalidCredit := adminAccountPost(handler, "/api/v1/admin/account-market/users/8/balance", `{"amount":1.001}`)
	if invalidCredit.Code != http.StatusBadRequest {
		t.Fatalf("invalid manual credit accepted: %d %s", invalidCredit.Code, invalidCredit.Body.String())
	}
	debit := adminAccountPost(handler, "/api/v1/admin/account-market/users/8/balance", `{"operation":"debit","amount":11,"note":"退款冲正"}`)
	if debit.Code != http.StatusOK || !strings.Contains(debit.Body.String(), `"operation":"debit"`) || !strings.Contains(debit.Body.String(), `"balance_after":90`) {
		t.Fatalf("manual debit failed: %d %s", debit.Code, debit.Body.String())
	}
	overdraft := adminAccountPost(handler, "/api/v1/admin/account-market/users/8/balance", `{"operation":"debit","amount":100}`)
	if overdraft.Code != http.StatusConflict || !strings.Contains(overdraft.Body.String(), "INSUFFICIENT_ACCOUNT_BALANCE") {
		t.Fatalf("manual debit overdraft accepted: %d %s", overdraft.Code, overdraft.Body.String())
	}
	adjustment := adminAccountRequest(handler, "/api/v1/admin/account-market/transactions?type=adjustment&search=member%40example.com")
	if adjustment.Code != http.StatusOK || !strings.Contains(adjustment.Body.String(), "售后补偿") || !strings.Contains(adjustment.Body.String(), "退款冲正") || !strings.Contains(adjustment.Body.String(), `"amount":-11`) || !strings.Contains(adjustment.Body.String(), `"operator_id":7`) {
		t.Fatalf("manual credit records failed: %d %s", adjustment.Code, adjustment.Body.String())
	}

	overview := adminAccountRequest(handler, "/api/v1/admin/account-market/users/8/overview")
	if overview.Code != http.StatusOK {
		t.Fatalf("overview failed: %d %s", overview.Code, overview.Body.String())
	}
	var body struct {
		Data struct {
			AccountBalance float64 `json:"account_balance"`
			UsageAvailable bool    `json:"usage_available"`
			Packages       []struct {
				PackageStatus string `json:"package_status"`
			} `json:"packages"`
			Summary struct {
				TotalPayments    float64 `json:"total_payments"`
				TotalRedemptions float64 `json:"total_redemptions"`
				TotalAdjustments float64 `json:"total_adjustments"`
				TotalDeductions  float64 `json:"total_deductions"`
				TotalSpent       float64 `json:"total_spent"`
				ActivePackages   int     `json:"active_packages"`
			} `json:"summary"`
		} `json:"data"`
	}
	if err := json.Unmarshal(overview.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.AccountBalance != 90 || !body.Data.UsageAvailable || len(body.Data.Packages) != 1 || body.Data.Packages[0].PackageStatus != "active" {
		t.Fatalf("unexpected overview: %+v", body.Data)
	}
	if body.Data.Summary.TotalPayments != 50 || body.Data.Summary.TotalRedemptions != 25 || body.Data.Summary.TotalAdjustments != 12.5 || body.Data.Summary.TotalDeductions != 11 || body.Data.Summary.TotalSpent != 20 || body.Data.Summary.ActivePackages != 1 {
		t.Fatalf("unexpected summary: %+v", body.Data.Summary)
	}
}

func TestAccountAdminRoutesRequireAdmin(t *testing.T) {
	handler := accountAdminTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/account-market/transactions", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated request accepted: %d %s", recorder.Code, recorder.Body.String())
	}
	creditRequest := httptest.NewRequest(http.MethodPost, "/api/v1/admin/account-market/users/8/balance", strings.NewReader(`{"amount":10}`))
	creditRecorder := httptest.NewRecorder()
	handler.ServeHTTP(creditRecorder, creditRequest)
	if creditRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated manual credit accepted: %d %s", creditRecorder.Code, creditRecorder.Body.String())
	}
}
