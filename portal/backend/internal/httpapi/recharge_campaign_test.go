package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jqcode/portal/backend/internal/coreclient"
	"github.com/jqcode/portal/backend/internal/rechargepromo"
)

type campaignStub struct {
	promotion  rechargepromo.Promotion
	registered int
	fulfilled  int
}

func (s *campaignStub) Register(_ context.Context, orderID int64, tradeNo string, userID int64, amount float64) (float64, error) {
	s.registered++
	s.promotion = rechargepromo.Promotion{
		OrderID: orderID, TradeNo: tradeNo, UserID: userID,
		PaidCents: int64(amount * 100), BonusCents: rechargepromo.BonusCents(amount), Status: "PENDING",
	}
	return float64(s.promotion.BonusCents) / 100, nil
}

func (s *campaignStub) FulfillByOrderID(context.Context, int64) error {
	s.fulfilled++
	s.promotion.Status = "APPLIED"
	return nil
}

func (s *campaignStub) FulfillByTradeNo(context.Context, string) error {
	s.fulfilled++
	s.promotion.Status = "APPLIED"
	return nil
}

func (s *campaignStub) LookupByOrderID(_ context.Context, orderID int64) (rechargepromo.Promotion, bool, error) {
	return s.promotion, s.promotion.OrderID == orderID, nil
}

func (s *campaignStub) LookupByTradeNo(_ context.Context, tradeNo string) (rechargepromo.Promotion, bool, error) {
	return s.promotion, s.promotion.TradeNo == tradeNo, nil
}

func TestCampaignOrderKeepsCorePaymentAmountAndRecordsBonus(t *testing.T) {
	core := &recordingCoreStub{coreStub: coreStub{userAPIResponse: coreclient.Response{
		StatusCode: http.StatusOK, ContentType: "application/json",
		Body: []byte(`{"code":0,"data":{"order_id":42,"out_trade_no":"sub2_42","amount":50,"pay_amount":50,"status":"PENDING"}}`),
	}}, profile: coreclient.User{ID: 7, Status: "active"}}
	campaign := &campaignStub{}
	recorder := httptest.NewRecorder()
	requestBody := `{"amount":50,"payment_type":"alipay","order_type":"balance"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/payment/orders", strings.NewReader(requestBody))
	request.Header.Set("Authorization", "Bearer user-token")
	request.Header.Set("Content-Type", "application/json")

	New(core, nil, testLogger(), campaign).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
	if string(core.request.Body) != requestBody {
		t.Fatalf("Core body=%s, want original %s", core.request.Body, requestBody)
	}
	if campaign.registered != 1 || campaign.promotion.PaidCents != 5000 || campaign.promotion.BonusCents != 500 {
		t.Fatalf("campaign=%+v registered=%d", campaign.promotion, campaign.registered)
	}
	var response struct {
		Data struct {
			Amount         float64 `json:"amount"`
			Bonus          float64 `json:"campaign_bonus"`
			CreditedAmount float64 `json:"credited_amount"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Data.Amount != 50 || response.Data.Bonus != 5 || response.Data.CreditedAmount != 55 {
		t.Fatalf("response data=%+v", response.Data)
	}
}

func TestCompletedCampaignOrderFulfillsOnlyOnce(t *testing.T) {
	core := &recordingCoreStub{coreStub: coreStub{userAPIResponse: coreclient.Response{
		StatusCode: http.StatusOK, ContentType: "application/json",
		Body: []byte(`{"code":0,"data":{"id":42,"out_trade_no":"sub2_42","amount":50,"pay_amount":50,"status":"COMPLETED"}}`),
	}}}
	campaign := &campaignStub{promotion: rechargepromo.Promotion{
		OrderID: 42, TradeNo: "sub2_42", UserID: 7, PaidCents: 5000, BonusCents: 500, Status: "PENDING",
	}}
	handler := New(core, nil, testLogger(), campaign)
	for range 2 {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/payment/orders/42", nil)
		request.Header.Set("Authorization", "Bearer user-token")
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"credited_amount":55`) {
			t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
		}
	}
	if campaign.fulfilled != 1 {
		t.Fatalf("fulfillment calls=%d, want 1", campaign.fulfilled)
	}
}

func TestCampaignOrderListIncludesBonusRecordFields(t *testing.T) {
	core := &recordingCoreStub{coreStub: coreStub{userAPIResponse: coreclient.Response{
		StatusCode: http.StatusOK, ContentType: "application/json",
		Body: []byte(`{"code":0,"data":{"items":[{"id":42,"out_trade_no":"sub2_42","amount":50,"pay_amount":50,"status":"COMPLETED"}],"total":1,"page":1,"page_size":20,"pages":1}}`),
	}}}
	campaign := &campaignStub{promotion: rechargepromo.Promotion{
		OrderID: 42, TradeNo: "sub2_42", UserID: 7, PaidCents: 5000, BonusCents: 500, Status: "APPLIED",
	}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/payment/orders/my?page=1&page_size=20", nil)
	request.Header.Set("Authorization", "Bearer user-token")

	New(core, nil, testLogger(), campaign).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
	if core.request.RawQuery != "page=1&page_size=20" {
		t.Fatalf("query=%q", core.request.RawQuery)
	}
	var response struct {
		Data struct {
			Items []struct {
				Amount         float64 `json:"amount"`
				Bonus          float64 `json:"campaign_bonus"`
				CreditedAmount float64 `json:"credited_amount"`
				BonusStatus    string  `json:"campaign_bonus_status"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data.Items) != 1 || response.Data.Items[0].Amount != 50 || response.Data.Items[0].Bonus != 5 || response.Data.Items[0].CreditedAmount != 55 || response.Data.Items[0].BonusStatus != "APPLIED" {
		t.Fatalf("items=%+v", response.Data.Items)
	}
}
