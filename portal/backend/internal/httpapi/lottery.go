package httpapi

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jqcode/portal/backend/internal/lottery"
)

type lotteryRateState struct {
	Window time.Time
	Count  int
}

var lotteryRateLimiter = struct {
	sync.Mutex
	items map[string]lotteryRateState
}{items: make(map[string]lotteryRateState)}

func allowLotteryRequest(r *http.Request, bucket string, limit int) bool {
	client := strings.TrimSpace(r.Header.Get("CF-Connecting-IP"))
	if client == "" {
		client = strings.TrimSpace(r.Header.Get("X-Real-IP"))
	}
	if client == "" {
		client = r.RemoteAddr
		if host, _, err := net.SplitHostPort(client); err == nil {
			client = host
		}
	}
	now := time.Now()
	window := now.Truncate(time.Minute)
	key := bucket + ":" + client

	lotteryRateLimiter.Lock()
	defer lotteryRateLimiter.Unlock()
	if len(lotteryRateLimiter.items) > 10000 {
		for key, state := range lotteryRateLimiter.items {
			if now.Sub(state.Window) >= time.Minute {
				delete(lotteryRateLimiter.items, key)
			}
		}
	}
	state := lotteryRateLimiter.items[key]
	if state.Window != window {
		state = lotteryRateState{Window: window}
	}
	if state.Count >= limit {
		lotteryRateLimiter.items[key] = state
		return false
	}
	state.Count++
	lotteryRateLimiter.items[key] = state
	return true
}

func requireLotteryRate(w http.ResponseWriter, r *http.Request, bucket string, limit int) bool {
	if allowLotteryRequest(r, bucket, limit) {
		return true
	}
	w.Header().Set("Retry-After", "60")
	writeAPIError(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many lottery requests")
	return false
}

func (s *Server) lotteryAvailable(w http.ResponseWriter) bool {
	if s.lottery == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "LOTTERY_UNAVAILABLE", "Lottery service is not configured")
		return false
	}
	return true
}

func (s *Server) lotteryCurrent(w http.ResponseWriter, r *http.Request) {
	if !requireLotteryRate(w, r, "current", 60) || !s.lotteryAvailable(w) {
		return
	}
	item, err := s.lottery.Current(r.Context())
	if err != nil {
		s.writeLotteryError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) lotteryRegister(w http.ResponseWriter, r *http.Request) {
	if !requireLotteryRate(w, r, "register", 5) || !s.lotteryAvailable(w) {
		return
	}
	var input struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}
	result, err := s.lottery.Register(r.Context(), input.Email)
	if err != nil {
		s.writeLotteryError(w, err)
		return
	}
	writeSuccess(w, result)
}

func (s *Server) lotteryCurrentResult(w http.ResponseWriter, r *http.Request) {
	if !requireLotteryRate(w, r, "result", 10) || !s.lotteryAvailable(w) {
		return
	}
	item, err := s.lottery.CurrentResult(r.Context(), r.URL.Query().Get("email"))
	if err != nil {
		s.writeLotteryError(w, err)
		return
	}
	item.Email = maskLotteryEmail(item.Email)
	writeSuccess(w, publicLotteryPayout(item))
}

func (s *Server) lotteryResults(w http.ResponseWriter, r *http.Request) {
	if !requireLotteryRate(w, r, "results", 10) || !s.lotteryAvailable(w) {
		return
	}
	items, err := s.lottery.ResultsByEmail(r.Context(), r.URL.Query().Get("email"))
	if err != nil {
		s.writeLotteryError(w, err)
		return
	}
	publicItems := make([]map[string]any, 0, len(items))
	for _, item := range items {
		publicItems = append(publicItems, publicLotteryPayout(item))
	}
	writeSuccess(w, map[string]any{"items": publicItems, "total": len(publicItems)})
}

func (s *Server) lotteryHistory(w http.ResponseWriter, r *http.Request) {
	if !requireLotteryRate(w, r, "history", 30) || !s.lotteryAvailable(w) {
		return
	}
	page := parsePositive(r.URL.Query().Get("page"), 1)
	pageSize := parsePositive(r.URL.Query().Get("page_size"), 10)
	items, total, err := s.lottery.History(r.Context(), page, pageSize)
	if err != nil {
		s.writeLotteryError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"items": items, "page": page, "page_size": pageSize, "total": total, "pages": (total + pageSize - 1) / pageSize})
}

func (s *Server) lotteryHistoryDetail(w http.ResponseWriter, r *http.Request) {
	if !s.lotteryAvailable(w) {
		return
	}
	id, ok := lotteryPathID(w, r, "id")
	if !ok {
		return
	}
	campaign, err := s.lottery.GetCampaign(r.Context(), id)
	if err != nil {
		s.writeLotteryError(w, err)
		return
	}
	if campaign.Status != "completed" {
		writeAPIError(w, http.StatusNotFound, "RESULT_NOT_AVAILABLE", "Campaign results are not available yet")
		return
	}
	items, err := s.lottery.Payouts(r.Context(), "", id)
	if err != nil {
		s.writeLotteryError(w, err)
		return
	}
	publicItems := make([]map[string]any, 0, len(items))
	for _, item := range items {
		publicItems = append(publicItems, publicLotteryPayout(item))
	}
	writeSuccess(w, map[string]any{"campaign": campaign, "items": publicItems})
}

func (s *Server) lotteryAdminCampaigns(w http.ResponseWriter, r *http.Request) {
	if !s.requireLotteryAdmin(w, r) || !s.lotteryAvailable(w) {
		return
	}
	items, err := s.lottery.ListCampaigns(r.Context(), r.URL.Query().Get("status"))
	if err != nil {
		s.writeLotteryError(w, err)
		return
	}
	writeSuccess(w, items)
}

func (s *Server) lotteryAdminCreate(w http.ResponseWriter, r *http.Request) {
	if !s.requireLotteryAdmin(w, r) || !s.lotteryAvailable(w) {
		return
	}
	var input lottery.CampaignInput
	if err := decodeJSON(r, &input); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}
	item, err := s.lottery.CreateCampaign(r.Context(), input)
	if err != nil {
		s.writeLotteryError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"code": 0, "message": "success", "data": item})
}

func (s *Server) lotteryAdminCampaign(w http.ResponseWriter, r *http.Request) {
	if !s.requireLotteryAdmin(w, r) || !s.lotteryAvailable(w) {
		return
	}
	id, ok := lotteryPathID(w, r, "id")
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, err := s.lottery.GetCampaign(r.Context(), id)
		if err != nil {
			s.writeLotteryError(w, err)
			return
		}
		writeSuccess(w, item)
	case http.MethodPut:
		var input lottery.CampaignInput
		if err := decodeJSON(r, &input); err != nil {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
			return
		}
		item, err := s.lottery.UpdateCampaign(r.Context(), id, input)
		if err != nil {
			s.writeLotteryError(w, err)
			return
		}
		writeSuccess(w, item)
	case http.MethodDelete:
		if err := s.lottery.CancelCampaign(r.Context(), id); err != nil {
			s.writeLotteryError(w, err)
			return
		}
		writeSuccess(w, map[string]any{"cancelled": true})
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
	}
}

func (s *Server) lotteryAdminDraw(w http.ResponseWriter, r *http.Request) {
	if !s.requireLotteryAdmin(w, r) || !s.lotteryAvailable(w) {
		return
	}
	id, ok := lotteryPathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.lottery.DrawCampaign(r.Context(), id); err != nil {
		s.writeLotteryError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"drawn": true})
}

func (s *Server) lotteryAdminPayouts(w http.ResponseWriter, r *http.Request) {
	if !s.requireLotteryAdmin(w, r) || !s.lotteryAvailable(w) {
		return
	}
	items, err := s.lottery.Payouts(r.Context(), r.URL.Query().Get("status"), parsePositive64(r.URL.Query().Get("campaign_id")))
	if err != nil {
		s.writeLotteryError(w, err)
		return
	}
	writeSuccess(w, items)
}

func (s *Server) lotteryAdminRetry(w http.ResponseWriter, r *http.Request) {
	if !s.requireLotteryAdmin(w, r) || !s.lotteryAvailable(w) {
		return
	}
	id, ok := lotteryPathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.lottery.RetryPayout(r.Context(), id); err != nil {
		s.writeLotteryError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"queued": true})
}

func (s *Server) requireLotteryAdmin(w http.ResponseWriter, r *http.Request) bool {
	token, ok := bearerToken(w, r)
	if !ok {
		return false
	}
	return s.requireCoreAdmin(w, r, token)
}

func (s *Server) writeLotteryError(w http.ResponseWriter, err error) {
	var problem *lottery.Problem
	if errors.As(err, &problem) {
		writeAPIError(w, problem.Status, problem.Code, problem.Message)
		return
	}
	s.logger.Warn("lottery request failed", "error", err)
	writeAPIError(w, http.StatusBadGateway, "LOTTERY_ERROR", "Lottery request failed")
}

func lotteryPathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue(name)), 10, 64)
	if err != nil || id <= 0 {
		writeAPIError(w, http.StatusBadRequest, "INVALID_ID", "Invalid identifier")
		return 0, false
	}
	return id, true
}

func parsePositive(value string, fallback int) int {
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}
func parsePositive64(value string) int64 {
	n, _ := strconv.ParseInt(value, 10, 64)
	if n < 1 {
		return 0
	}
	return n
}
func maskLotteryEmail(value string) string {
	parts := strings.SplitN(strings.ToLower(strings.TrimSpace(value)), "@", 2)
	if len(parts) != 2 {
		return value
	}
	prefix := parts[0]
	if len(prefix) > 2 {
		prefix = prefix[:2]
	}
	return prefix + "***@" + parts[1]
}

func publicLotteryPayout(item lottery.Payout) map[string]any {
	return map[string]any{"id": item.ID, "campaign_id": item.CampaignID, "campaign_name": item.CampaignName, "email": item.Email, "prize_type": item.PrizeType, "amount": item.Amount, "status": item.Status, "created_at": item.CreatedAt, "credited_at": item.CreditedAt}
}
