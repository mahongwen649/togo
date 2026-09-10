package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jqcode/portal/backend/internal/coreclient"
	"github.com/jqcode/portal/backend/internal/rechargepromo"
)

type rechargeCampaign interface {
	Register(context.Context, int64, string, int64, float64) (float64, error)
	FulfillByOrderID(context.Context, int64) error
	FulfillByTradeNo(context.Context, string) error
	LookupByOrderID(context.Context, int64) (rechargepromo.Promotion, bool, error)
	LookupByTradeNo(context.Context, string) (rechargepromo.Promotion, bool, error)
}

type paymentOrderSnapshot struct {
	ID       int64
	TradeNo  string
	Amount   float64
	Status   string
	Response map[string]any
	Data     map[string]any
}

func (s *Server) createPaymentOrder(w http.ResponseWriter, r *http.Request) {
	accessToken, ok := bearerToken(w, r)
	if !ok {
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid payment order")
		return
	}
	var request struct {
		Amount    float64 `json:"amount"`
		OrderType string  `json:"order_type"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid payment order")
		return
	}

	bonusCents := rechargepromo.BonusCents(request.Amount)
	var userID int64
	if bonusCents > 0 && strings.EqualFold(strings.TrimSpace(request.OrderType), "balance") {
		user, profileErr := s.core.Profile(r.Context(), accessToken)
		if profileErr != nil {
			s.writeCoreProxyError(w, r, profileErr)
			return
		}
		userID = user.ID
	}

	response, err := s.callCoreUserAPI(r, accessToken, r.URL.RawQuery, body)
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return
	}
	if bonusCents == 0 || userID == 0 || response.StatusCode < 200 || response.StatusCode >= 300 || s.rechargeCampaign == nil {
		writeCoreResponse(w, response)
		return
	}

	order, err := decodePaymentOrder(response)
	if err != nil || order.ID <= 0 || order.TradeNo == "" {
		s.logger.Error("Core campaign order response is incomplete", "error", err)
		writeAPIError(w, http.StatusBadGateway, "CAMPAIGN_ORDER_INCOMPLETE", "Recharge campaign order could not be recorded")
		return
	}
	bonus, err := s.rechargeCampaign.Register(r.Context(), order.ID, order.TradeNo, userID, request.Amount)
	if err != nil {
		s.logger.Error("Recharge campaign order persistence failed", "order_id", order.ID, "error", err)
		s.cancelUnrecordedPaymentOrder(r, accessToken, order.ID)
		writeAPIError(w, http.StatusBadGateway, "CAMPAIGN_ORDER_INCOMPLETE", "Recharge campaign order could not be recorded")
		return
	}
	writeCampaignOrderResponse(w, response, order, bonus, "PENDING")
}

func (s *Server) getPaymentOrder(w http.ResponseWriter, r *http.Request) {
	accessToken, ok := bearerToken(w, r)
	if !ok {
		return
	}
	response, err := s.callCoreUserAPI(r, accessToken, r.URL.RawQuery, nil)
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return
	}
	s.processCampaignOrderResponse(r.Context(), response, "order_id", w)
}

func (s *Server) verifyPaymentOrder(w http.ResponseWriter, r *http.Request) {
	accessToken, ok := bearerToken(w, r)
	if !ok {
		return
	}
	response, err := s.callCoreUserAPI(r, accessToken, r.URL.RawQuery, nil)
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return
	}
	s.processCampaignOrderResponse(r.Context(), response, "trade_no", w)
}

func (s *Server) listPaymentOrders(w http.ResponseWriter, r *http.Request) {
	accessToken, ok := bearerToken(w, r)
	if !ok {
		return
	}
	response, err := s.callCoreUserAPI(r, accessToken, r.URL.RawQuery, nil)
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || s.rechargeCampaign == nil {
		writeCoreResponse(w, response)
		return
	}

	var payload map[string]any
	decoder := json.NewDecoder(bytes.NewReader(response.Body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		writeCoreResponse(w, response)
		return
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		writeCoreResponse(w, response)
		return
	}
	items, ok := data["items"].([]any)
	if !ok {
		writeCoreResponse(w, response)
		return
	}
	for _, rawItem := range items {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		order := paymentOrderFromData(item)
		promotion, found, campaignErr := s.resolveCampaignPromotion(r.Context(), order, "order_id")
		if campaignErr != nil && !errors.Is(campaignErr, rechargepromo.ErrApplying) {
			s.logger.Warn("Recharge campaign order list annotation failed", "order_id", order.ID, "error", campaignErr)
		}
		if found {
			item["campaign_bonus"] = float64(promotion.BonusCents) / 100
			item["credited_amount"] = order.Amount + float64(promotion.BonusCents)/100
			item["campaign_bonus_status"] = promotion.Status
		}
	}
	body, err := json.Marshal(payload)
	if err == nil {
		response.Body = body
	}
	writeCoreResponse(w, response)
}

func (s *Server) processCampaignOrderResponse(ctx context.Context, response coreclient.Response, lookup string, w http.ResponseWriter) {
	if response.StatusCode < 200 || response.StatusCode >= 300 || s.rechargeCampaign == nil {
		writeCoreResponse(w, response)
		return
	}
	order, err := decodePaymentOrder(response)
	if err != nil {
		writeCoreResponse(w, response)
		return
	}
	promotion, found, err := s.resolveCampaignPromotion(ctx, order, lookup)
	if err != nil && !errors.Is(err, rechargepromo.ErrApplying) {
		s.logger.Warn("Recharge campaign lookup failed", "order_id", order.ID, "error", err)
	}
	if !found {
		writeCoreResponse(w, response)
		return
	}
	writeCampaignOrderResponse(w, response, order, float64(promotion.BonusCents)/100, promotion.Status)
}

func (s *Server) resolveCampaignPromotion(ctx context.Context, order paymentOrderSnapshot, lookup string) (rechargepromo.Promotion, bool, error) {
	var promotion rechargepromo.Promotion
	var found bool
	var err error
	if lookup == "trade_no" {
		promotion, found, err = s.rechargeCampaign.LookupByTradeNo(ctx, order.TradeNo)
	} else {
		promotion, found, err = s.rechargeCampaign.LookupByOrderID(ctx, order.ID)
	}
	if err != nil || !found || order.Status != "COMPLETED" || promotion.Status == "APPLIED" {
		return promotion, found, err
	}
	if lookup == "trade_no" {
		err = s.rechargeCampaign.FulfillByTradeNo(ctx, order.TradeNo)
	} else {
		err = s.rechargeCampaign.FulfillByOrderID(ctx, order.ID)
	}
	if lookup == "trade_no" {
		promotion, found, _ = s.rechargeCampaign.LookupByTradeNo(ctx, order.TradeNo)
	} else {
		promotion, found, _ = s.rechargeCampaign.LookupByOrderID(ctx, order.ID)
	}
	return promotion, found, err
}

func decodePaymentOrder(response coreclient.Response) (paymentOrderSnapshot, error) {
	var payload map[string]any
	decoder := json.NewDecoder(bytes.NewReader(response.Body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return paymentOrderSnapshot{}, err
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		return paymentOrderSnapshot{}, fmt.Errorf("missing payment order data")
	}
	order := paymentOrderFromData(data)
	order.Response = payload
	order.Data = data
	return order, nil
}

func paymentOrderFromData(data map[string]any) paymentOrderSnapshot {
	order := paymentOrderSnapshot{Data: data}
	order.ID = int64(numberValue(firstValue(data, "id", "order_id")))
	order.TradeNo = stringValue(data["out_trade_no"])
	order.Amount = numberValue(data["amount"])
	order.Status = strings.ToUpper(stringValue(data["status"]))
	return order
}

func writeCampaignOrderResponse(w http.ResponseWriter, response coreclient.Response, order paymentOrderSnapshot, bonus float64, status string) {
	order.Data["campaign_bonus"] = bonus
	order.Data["credited_amount"] = order.Amount + bonus
	order.Data["campaign_bonus_status"] = status
	body, err := json.Marshal(order.Response)
	if err != nil {
		writeCoreResponse(w, response)
		return
	}
	response.Body = body
	writeCoreResponse(w, response)
}

func (s *Server) cancelUnrecordedPaymentOrder(source *http.Request, accessToken string, orderID int64) {
	request := source.Clone(source.Context())
	request.Method = http.MethodPost
	request.URL.Path = "/api/v1/payment/orders/" + strconv.FormatInt(orderID, 10) + "/cancel"
	request.URL.RawQuery = ""
	request.Body = io.NopCloser(strings.NewReader(""))
	if _, err := s.callCoreUserAPI(request, accessToken, "", []byte{}); err != nil {
		s.logger.Warn("Unrecorded campaign order cancellation failed", "order_id", orderID, "error", err)
	}
}

func firstValue(values map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			return value
		}
	}
	return nil
}

func numberValue(value any) float64 {
	switch value := value.(type) {
	case json.Number:
		result, _ := value.Float64()
		return result
	case float64:
		return value
	default:
		return 0
	}
}

func stringValue(value any) string {
	result, _ := value.(string)
	return strings.TrimSpace(result)
}

func paymentWebhookTradeNo(rawQuery string, body []byte) string {
	query, _ := url.ParseQuery(rawQuery)
	if value := strings.TrimSpace(query.Get("out_trade_no")); value != "" {
		return value
	}
	form, _ := url.ParseQuery(string(body))
	return strings.TrimSpace(form.Get("out_trade_no"))
}
