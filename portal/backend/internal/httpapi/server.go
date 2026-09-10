package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	portalauth "github.com/jqcode/portal/backend/internal/auth"
	"github.com/jqcode/portal/backend/internal/company"
	"github.com/jqcode/portal/backend/internal/coreclient"
	"github.com/jqcode/portal/backend/internal/lottery"
	"github.com/jqcode/portal/backend/internal/rechargepromo"
	"github.com/jqcode/portal/backend/internal/username"
	"golang.org/x/sync/errgroup"
)

type Server struct {
	core                 coreclient.Adapter
	auth                 AuthService
	logger               *slog.Logger
	company              *company.Service
	usernames            usernameRegistry
	passwordResetService PasswordResetService
	rechargeCampaign     rechargeCampaign
	lottery              *lottery.Service
	corePaymentDB        *sql.DB
	imgtoolKeysMu        sync.Mutex
}

type AuthService interface {
	Login(context.Context, portalauth.LoginInput) (coreclient.AuthResult, error)
	Register(context.Context, portalauth.RegisterInput) (coreclient.AuthResult, error)
	RenameCurrentUser(context.Context, string, string) (coreclient.User, error)
	UsernameAvailable(context.Context, string) (bool, error)
}

type usernameRegistry interface {
	Reserve(context.Context, string) (username.Reservation, error)
	Bind(context.Context, username.Reservation, int64) error
	Release(context.Context, username.Reservation) error
	DeleteBound(context.Context, string, int64) error
}

type PasswordResetService interface {
	SendCode(context.Context, string, string, string) (int, error)
	ResetPassword(context.Context, string, string, string) error
}

func New(core coreclient.Adapter, auth AuthService, logger *slog.Logger, dependencies ...any) http.Handler {
	if core == nil {
		panic("Core adapter is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{
		core:   core,
		auth:   auth,
		logger: logger,
	}
	for _, dependency := range dependencies {
		switch value := dependency.(type) {
		case *company.Service:
			s.company = value
		case usernameRegistry:
			s.usernames = value
		case PasswordResetService:
			s.passwordResetService = value
		case rechargeCampaign:
			s.rechargeCampaign = value
		case *lottery.Service:
			s.lottery = value
		case *sql.DB:
			s.corePaymentDB = value
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /api/v1/settings/public", s.publicSettings)
	mux.HandleFunc("POST /api/v1/auth/send-verify-code", s.sendVerifyCode)
	mux.HandleFunc("POST /api/v1/auth/forgot-password", s.passwordReset)
	mux.HandleFunc("POST /api/v1/auth/reset-password", s.passwordReset)
	mux.HandleFunc("GET /api/v1/auth/me", s.authMe)
	mux.HandleFunc("GET /api/v1/keys", s.coreUserAPI)
	mux.HandleFunc("POST /api/v1/keys", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/keys/{keyID}", s.coreUserAPI)
	mux.HandleFunc("PUT /api/v1/keys/{keyID}", s.coreUserAPI)
	mux.HandleFunc("DELETE /api/v1/keys/{keyID}", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/groups/available", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/groups/rates", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/channels/available", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/channel-monitors", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/channel-monitors/{monitorID}/status", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/user/profile", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/user/platform-quotas", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/subscriptions", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/subscriptions/active", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/subscriptions/progress", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/subscriptions/summary", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/subscriptions/{subscriptionID}/progress", s.coreUserAPI)
	mux.HandleFunc("PUT /api/v1/user", s.updateCurrentUser)
	mux.HandleFunc("PUT /api/v1/user/password", s.coreUserAPI)
	mux.HandleFunc("POST /api/v1/user/notify-email/send-code", s.coreUserAPI)
	mux.HandleFunc("POST /api/v1/user/notify-email/verify", s.coreUserAPI)
	mux.HandleFunc("DELETE /api/v1/user/notify-email", s.coreUserAPI)
	mux.HandleFunc("PUT /api/v1/user/notify-email/toggle", s.coreUserAPI)
	mux.HandleFunc("POST /api/v1/user/account-bindings/email/send-code", s.coreUserAPI)
	mux.HandleFunc("POST /api/v1/user/account-bindings/email", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/user/aff", s.coreUserAPI)
	mux.HandleFunc("POST /api/v1/user/aff/transfer", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/user/totp/status", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/user/totp/verification-method", s.coreUserAPI)
	mux.HandleFunc("POST /api/v1/user/totp/send-code", s.coreUserAPI)
	mux.HandleFunc("POST /api/v1/user/totp/setup", s.coreUserAPI)
	mux.HandleFunc("POST /api/v1/user/totp/enable", s.coreUserAPI)
	mux.HandleFunc("POST /api/v1/user/totp/disable", s.coreUserAPI)
	mux.HandleFunc("POST /api/v1/user/totp/step-up", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/redeem/history", s.coreUserAPI)
	mux.HandleFunc("POST /api/v1/redeem", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/payment/config", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/payment/checkout-info", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/payment/limits", s.coreUserAPI)
	mux.HandleFunc("POST /api/v1/payment/orders", s.createPaymentOrder)
	mux.HandleFunc("POST /api/v1/payment/orders/verify", s.verifyPaymentOrder)
	mux.HandleFunc("GET /api/v1/payment/orders/my", s.listPaymentOrders)
	mux.HandleFunc("GET /api/v1/payment/orders/{orderID}", s.getPaymentOrder)
	mux.HandleFunc("POST /api/v1/payment/orders/{orderID}/cancel", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/payment/webhook/easypay", s.paymentWebhook)
	mux.HandleFunc("POST /api/v1/payment/webhook/easypay", s.paymentWebhook)
	mux.HandleFunc("GET /api/v1/announcements", s.coreUserAPI)
	mux.HandleFunc("POST /api/v1/announcements/{announcementID}/read", s.coreUserAPI)
	mux.HandleFunc("POST /api/v1/integrations/imgtool/sso-url", s.imgtoolURL)
	mux.HandleFunc("POST /api/v1/integrations/imgtool/credentials", s.imgtoolCredentials)
	mux.HandleFunc("GET /api/v1/usage", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/usage/stats", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/usage/dashboard/stats", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/usage/dashboard/trend", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/usage/dashboard/models", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/usage/dashboard/snapshot-v2", s.coreUserAPI)
	mux.HandleFunc("POST /api/v1/usage/dashboard/api-keys-usage", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/usage/errors", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/usage/errors/{errorID}", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/usage/{usageID}", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/user/api-keys/{keyID}/usage/daily", s.coreUserAPI)
	mux.HandleFunc("GET /api/v1/admin/groups/all", s.coreAdminAPI)
	mux.HandleFunc("GET /api/v1/admin/users", s.coreAdminAPI)
	mux.HandleFunc("POST /api/v1/admin/users", s.createAdminUser)
	mux.HandleFunc("POST /api/v1/admin/users/batch-limits", s.coreAdminAPI)
	mux.HandleFunc("GET /api/v1/admin/users/{userID}", s.coreAdminAPI)
	mux.HandleFunc("PUT /api/v1/admin/users/{userID}", s.coreAdminAPI)
	mux.HandleFunc("DELETE /api/v1/admin/users/{userID}", s.deleteAdminUser)
	mux.HandleFunc("GET /api/v1/admin/users/{userID}/api-keys", s.coreAdminAPI)
	mux.HandleFunc("GET /api/v1/admin/users/{userID}/usage", s.coreAdminAPI)
	mux.HandleFunc("POST /api/v1/admin/users/{userID}/balance", s.coreAdminAPI)
	mux.HandleFunc("GET /api/v1/admin/users/{userID}/balance-history", s.coreAdminAPI)
	mux.HandleFunc("POST /api/v1/admin/users/{userID}/replace-group", s.coreAdminAPI)
	mux.HandleFunc("GET /api/v1/admin/users/{userID}/rpm-status", s.coreAdminAPI)
	mux.HandleFunc("GET /api/v1/admin/users/{userID}/platform-quotas", s.coreAdminAPI)
	mux.HandleFunc("PUT /api/v1/admin/users/{userID}/platform-quotas", s.coreAdminAPI)
	mux.HandleFunc("POST /api/v1/admin/users/{userID}/platform-quotas/reset", s.coreAdminAPI)
	mux.HandleFunc("GET /api/v1/admin/users/{userID}/attributes", s.coreAdminAPI)
	mux.HandleFunc("PUT /api/v1/admin/users/{userID}/attributes", s.coreAdminAPI)
	mux.HandleFunc("PUT /api/v1/admin/api-keys/{keyID}", s.coreAdminAPI)
	mux.HandleFunc("GET /api/v1/admin/user-attributes", s.coreAdminAPI)
	mux.HandleFunc("POST /api/v1/admin/user-attributes/batch", s.coreAdminAPI)
	mux.HandleFunc("GET /api/v1/admin/announcements", s.coreAdminAPI)
	mux.HandleFunc("POST /api/v1/admin/announcements", s.coreAdminAPI)
	mux.HandleFunc("GET /api/v1/admin/announcements/{announcementID}", s.coreAdminAPI)
	mux.HandleFunc("PUT /api/v1/admin/announcements/{announcementID}", s.coreAdminAPI)
	mux.HandleFunc("DELETE /api/v1/admin/announcements/{announcementID}", s.coreAdminAPI)
	mux.HandleFunc("GET /api/v1/admin/announcements/{announcementID}/read-status", s.coreAdminAPI)
	mux.HandleFunc("GET /api/v1/admin/usage", s.scopedAdminUsageAPI)
	mux.HandleFunc("GET /api/v1/admin/usage/stats", s.scopedAdminUsageAPI)
	mux.HandleFunc("GET /api/v1/admin/usage/search-users", s.scopedAdminUsageAPI)
	mux.HandleFunc("GET /api/v1/admin/usage/search-api-keys", s.scopedAdminUsageAPI)
	mux.HandleFunc("GET /api/v1/admin/usage/cleanup-tasks", s.coreAdminAPI)
	mux.HandleFunc("GET /api/v1/admin/ops/request-errors", s.coreAdminAPI)
	mux.HandleFunc("GET /api/v1/admin/ops/request-errors/{errorID}", s.coreAdminAPI)
	mux.HandleFunc("GET /api/v1/admin/dashboard/snapshot-v2", s.scopedAdminUsageAPI)
	mux.HandleFunc("GET /api/v1/admin/dashboard/stats", s.scopedAdminUsageAPI)
	mux.HandleFunc("GET /api/v1/admin/dashboard/realtime", s.coreAdminAPI)
	mux.HandleFunc("GET /api/v1/admin/dashboard/trend", s.scopedAdminUsageAPI)
	mux.HandleFunc("GET /api/v1/admin/dashboard/models", s.scopedAdminUsageAPI)
	mux.HandleFunc("GET /api/v1/admin/dashboard/groups", s.scopedAdminUsageAPI)
	mux.HandleFunc("GET /api/v1/admin/dashboard/api-keys-trend", s.scopedAdminUsageAPI)
	mux.HandleFunc("GET /api/v1/admin/dashboard/users-trend", s.scopedAdminUsageAPI)
	mux.HandleFunc("GET /api/v1/admin/dashboard/users-ranking", s.scopedAdminUsageAPI)
	mux.HandleFunc("GET /api/v1/admin/dashboard/user-breakdown", s.scopedAdminUsageAPI)
	mux.HandleFunc("POST /api/v1/admin/dashboard/users-usage", s.scopedAdminUsageAPI)
	mux.HandleFunc("POST /api/v1/admin/dashboard/api-keys-usage", s.scopedAdminUsageAPI)
	if auth != nil {
		mux.HandleFunc("POST /api/v1/auth/login", s.login)
		mux.HandleFunc("POST /api/v1/auth/register", s.register)
		mux.HandleFunc("GET /api/v1/auth/username-available", s.usernameAvailable)
	} else {
		mux.HandleFunc("POST /api/v1/auth/login", s.directCoreLogin)
	}
	if s.company != nil {
		mux.HandleFunc("GET /api/v1/admin/management/scope", s.managementScope)
		mux.HandleFunc("GET /api/v1/admin/companies/all", s.listCompanies)
		mux.HandleFunc("POST /api/v1/admin/companies", s.createCompany)
		mux.HandleFunc("PUT /api/v1/admin/companies/{companyID}", s.updateCompany)
		mux.HandleFunc("DELETE /api/v1/admin/companies/{companyID}", s.deactivateCompany)
		mux.HandleFunc("GET /api/v1/admin/companies/{companyID}/members", s.listCompanyMembers)
		mux.HandleFunc("POST /api/v1/admin/companies/{companyID}/members", s.assignCompanyMember)
		mux.HandleFunc("POST /api/v1/admin/companies/{companyID}/members/create", s.createCompanyMember)
		mux.HandleFunc("DELETE /api/v1/admin/companies/{companyID}/members/{userID}", s.removeCompanyMember)
		mux.HandleFunc("POST /api/v1/admin/companies/{companyID}/members/{userID}/balance", s.allocateCompanyMemberBalance)
		mux.HandleFunc("GET /api/v1/admin/companies/{companyID}/managers", s.listCompanyManagers)
		mux.HandleFunc("POST /api/v1/admin/companies/{companyID}/managers", s.addCompanyManager)
		mux.HandleFunc("DELETE /api/v1/admin/companies/{companyID}/managers/{userID}", s.removeCompanyManager)
		mux.HandleFunc("GET /api/v1/admin/finance/summary", s.financeSummary)
		mux.HandleFunc("GET /api/v1/admin/finance/members", s.financeMembers)
		mux.HandleFunc("GET /api/v1/admin/finance/members/{userID}/breakdown", s.financeMemberBreakdown)
		mux.HandleFunc("GET /api/v1/admin/finance/members/{userID}/export", s.financeMemberExport)
		mux.HandleFunc("GET /api/v1/admin/finance/export", s.financeExport)
	}
	mux.HandleFunc("GET /api/v1/admin/recharges", s.adminRecharges)
	mux.HandleFunc("GET /api/v1/lottery/current", s.lotteryCurrent)
	mux.HandleFunc("POST /api/v1/lottery/register", s.lotteryRegister)
	mux.HandleFunc("GET /api/v1/lottery/current/result", s.lotteryCurrentResult)
	mux.HandleFunc("GET /api/v1/lottery/results", s.lotteryResults)
	mux.HandleFunc("GET /api/v1/lottery/history", s.lotteryHistory)
	mux.HandleFunc("GET /api/v1/lottery/history/{id}", s.lotteryHistoryDetail)
	mux.HandleFunc("GET /api/v1/admin/lottery/campaigns", s.lotteryAdminCampaigns)
	mux.HandleFunc("POST /api/v1/admin/lottery/campaigns", s.lotteryAdminCreate)
	mux.HandleFunc("GET /api/v1/admin/lottery/campaigns/{id}", s.lotteryAdminCampaign)
	mux.HandleFunc("PUT /api/v1/admin/lottery/campaigns/{id}", s.lotteryAdminCampaign)
	mux.HandleFunc("DELETE /api/v1/admin/lottery/campaigns/{id}", s.lotteryAdminCampaign)
	mux.HandleFunc("POST /api/v1/admin/lottery/campaigns/{id}/draw", s.lotteryAdminDraw)
	mux.HandleFunc("GET /api/v1/admin/lottery/payouts", s.lotteryAdminPayouts)
	mux.HandleFunc("POST /api/v1/admin/lottery/payouts/{id}/retry", s.lotteryAdminRetry)
	return mux
}

func (s *Server) currentScope(r *http.Request) (company.Scope, error) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(header, "Bearer ") {
		return company.Scope{}, coreclient.ErrUnauthorized
	}
	user, err := s.core.Profile(r.Context(), strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
	if err != nil {
		return company.Scope{}, err
	}
	if user.Status != "active" {
		return company.Scope{}, coreclient.ErrUnauthorized
	}
	return s.company.Scope(r.Context(), user.ID, user.Role)
}

func (s *Server) requireScope(w http.ResponseWriter, r *http.Request) (company.Scope, bool) {
	scope, err := s.currentScope(r)
	if err == nil {
		return scope, true
	}
	if errors.Is(err, coreclient.ErrUnauthorized) {
		writeAPIError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	} else {
		s.logger.Warn("Portal company authentication failed", "error", err)
		writeAPIError(w, http.StatusBadGateway, "CORE_UNAVAILABLE", "Authentication service is unavailable")
	}
	return company.Scope{}, false
}

func (s *Server) managementScope(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.requireScope(w, r)
	if !ok {
		return
	}
	writeSuccess(w, map[string]any{"user_id": scope.UserID, "is_admin": scope.IsAdmin, "is_group_manager": false, "is_company_manager": len(scope.CompanyIDs) > 0, "managed_group_ids": []int64{}, "managed_company_ids": scope.CompanyIDs, "can_access_admin_area": scope.IsAdmin || len(scope.CompanyIDs) > 0})
}

func (s *Server) listCompanies(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.requireScope(w, r)
	if !ok {
		return
	}
	if !requireManagementScope(w, scope) {
		return
	}
	items, err := s.company.List(r.Context(), scope)
	if err != nil {
		s.companyError(w, err)
		return
	}
	writeSuccess(w, items)
}

type companyRequest struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

func (s *Server) createCompany(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.requireScope(w, r)
	if !ok {
		return
	}
	var input companyRequest
	if decodeJSON(r, &input) != nil {
		writeAPIError(w, 400, "INVALID_REQUEST", "Invalid company request")
		return
	}
	item, err := s.company.Create(r.Context(), scope, input.Name, input.Status)
	if err != nil {
		s.companyError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) updateCompany(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.requireScope(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r, "companyID")
	if !ok {
		return
	}
	var input companyRequest
	if decodeJSON(r, &input) != nil {
		writeAPIError(w, 400, "INVALID_REQUEST", "Invalid company request")
		return
	}
	item, err := s.company.Update(r.Context(), scope, id, input.Name, input.Status)
	if err != nil {
		s.companyError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) deactivateCompany(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.requireScope(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r, "companyID")
	if !ok {
		return
	}
	item, err := s.company.Deactivate(r.Context(), scope, id)
	if err != nil {
		s.companyError(w, err)
		return
	}
	writeSuccess(w, item)
}

type userIDRequest struct {
	UserID int64 `json:"user_id"`
}

type allocateCompanyMemberBalanceRequest struct {
	Amount float64 `json:"amount"`
	Notes  string  `json:"notes"`
}

func (s *Server) assignCompanyMember(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.requireScope(w, r)
	if !ok {
		return
	}
	companyID, ok := pathID(w, r, "companyID")
	if !ok {
		return
	}
	var input userIDRequest
	if decodeJSON(r, &input) != nil || input.UserID <= 0 {
		writeAPIError(w, 400, "INVALID_REQUEST", "Invalid member request")
		return
	}
	if _, err := s.core.AdminUser(r.Context(), input.UserID); err != nil {
		writeAPIError(w, 404, "USER_NOT_FOUND", "User not found")
		return
	}
	if err := s.company.AssignMember(r.Context(), scope, companyID, input.UserID); err != nil {
		s.companyError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"company_id": companyID, "user_id": input.UserID, "updated_members": 1})
}

type createCompanyMemberRequest struct {
	Email       string   `json:"email"`
	Password    string   `json:"password"`
	Username    string   `json:"username"`
	Role        string   `json:"role"`
	Notes       string   `json:"notes"`
	Balance     *float64 `json:"balance"`
	Concurrency int      `json:"concurrency"`
	RPMLimit    int      `json:"rpm_limit"`
}

type createAdminUserRequest struct {
	Email       string   `json:"email"`
	Password    string   `json:"password"`
	Username    string   `json:"username"`
	Notes       string   `json:"notes"`
	Role        string   `json:"role"`
	Balance     *float64 `json:"balance"`
	Concurrency int      `json:"concurrency"`
	RPMLimit    int      `json:"rpm_limit"`
}

func (s *Server) createAdminUser(w http.ResponseWriter, r *http.Request) {
	if s.usernames == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "USERNAME_REGISTRY_UNAVAILABLE", "Username registry is unavailable")
		return
	}
	current, ok := s.requireActiveAdmin(w, r)
	if !ok {
		return
	}

	var input createAdminUserRequest
	if err := decodeJSON(r, &input); err != nil || strings.TrimSpace(input.Email) == "" || input.Password == "" || strings.TrimSpace(input.Username) == "" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Email, username, and password are required")
		return
	}
	role := strings.TrimSpace(input.Role)
	if role == "" {
		role = "user"
	}
	if role != "user" && role != "admin" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid user role")
		return
	}

	reservation, err := s.usernames.Reserve(r.Context(), input.Username)
	if err != nil {
		if errors.Is(err, username.ErrTaken) {
			writeAPIError(w, http.StatusConflict, "USERNAME_TAKEN", "Username is already taken")
			return
		}
		if errors.Is(err, username.ErrInvalid) {
			writeAPIError(w, http.StatusBadRequest, "INVALID_USERNAME", "Invalid username")
			return
		}
		s.logger.Warn("Admin user username reservation failed", "admin_user_id", current.ID, "error", err)
		writeAPIError(w, http.StatusServiceUnavailable, "USERNAME_REGISTRY_UNAVAILABLE", "Username registry is unavailable")
		return
	}
	bound := false
	defer func() {
		if !bound {
			_ = s.usernames.Release(context.WithoutCancel(r.Context()), reservation)
		}
	}()

	created, response, err := s.core.AdminCreateUser(r.Context(), coreclient.AdminCreateUserRequest{
		Email:       strings.TrimSpace(input.Email),
		Password:    input.Password,
		Username:    reservation.Username,
		Notes:       input.Notes,
		Role:        role,
		Balance:     input.Balance,
		Concurrency: input.Concurrency,
		RPMLimit:    input.RPMLimit,
	})
	if err != nil {
		s.logger.Warn("Core admin user creation failed", "admin_user_id", current.ID, "error", err)
		writeAPIError(w, http.StatusBadGateway, "CORE_UNAVAILABLE", "Core user creation failed")
		return
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		writeCoreResponse(w, response)
		return
	}
	if created.ID <= 0 {
		writeAPIError(w, http.StatusBadGateway, "CORE_INVALID_RESPONSE", "Core user creation returned an invalid response")
		return
	}
	if err := s.usernames.Bind(r.Context(), reservation, created.ID); err != nil {
		_ = s.core.AdminDeleteUser(context.WithoutCancel(r.Context()), created.ID)
		s.logger.Error("Admin user username bind failed", "admin_user_id", current.ID, "created_core_user_id", created.ID, "error", err)
		writeAPIError(w, http.StatusBadGateway, "USER_CREATION_INCOMPLETE", "User creation could not be completed")
		return
	}
	bound = true
	writeCoreResponse(w, response)
}

func (s *Server) deleteAdminUser(w http.ResponseWriter, r *http.Request) {
	if s.usernames == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "USERNAME_REGISTRY_UNAVAILABLE", "Username registry is unavailable")
		return
	}
	if _, ok := s.requireActiveAdmin(w, r); !ok {
		return
	}
	userID, ok := pathID(w, r, "userID")
	if !ok {
		return
	}
	user, err := s.core.AdminUser(r.Context(), userID)
	if err != nil {
		s.logger.Warn("Core admin user lookup before delete failed", "core_user_id", userID, "error", err)
		writeAPIError(w, http.StatusBadGateway, "CORE_UNAVAILABLE", "Core user lookup failed")
		return
	}
	if err := s.core.AdminDeleteUser(r.Context(), userID); err != nil {
		s.logger.Warn("Core admin user delete failed", "core_user_id", userID, "error", err)
		writeAPIError(w, http.StatusBadGateway, "CORE_UNAVAILABLE", "Core user deletion failed")
		return
	}
	if strings.TrimSpace(user.Username) != "" {
		if err := s.usernames.DeleteBound(context.WithoutCancel(r.Context()), user.Username, userID); err != nil && !errors.Is(err, username.ErrNotFound) {
			s.logger.Warn("Portal username registry cleanup after user delete failed", "core_user_id", userID, "username", user.Username, "error", err)
		}
	}
	if s.company != nil {
		if err := s.company.RemoveUserRelations(context.WithoutCancel(r.Context()), userID); err != nil {
			s.logger.Warn("Portal company relation cleanup after user delete failed", "core_user_id", userID, "error", err)
		}
	}
	writeSuccess(w, map[string]any{"message": "User deleted successfully", "id": userID})
}

func (s *Server) requireActiveAdmin(w http.ResponseWriter, r *http.Request) (coreclient.User, bool) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(header, "Bearer ") {
		writeAPIError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return coreclient.User{}, false
	}
	current, err := s.core.Profile(r.Context(), strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
	if err != nil {
		if errors.Is(err, coreclient.ErrUnauthorized) {
			writeAPIError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
			return coreclient.User{}, false
		}
		s.logger.Warn("Admin authentication failed", "error", err)
		writeAPIError(w, http.StatusBadGateway, "CORE_UNAVAILABLE", "Authentication service is unavailable")
		return coreclient.User{}, false
	}
	if current.Status != "active" || current.Role != "admin" {
		writeAPIError(w, http.StatusForbidden, "FORBIDDEN", "Admin access required")
		return coreclient.User{}, false
	}
	return current, true
}

func (s *Server) createCompanyMember(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.requireScope(w, r)
	if !ok {
		return
	}
	companyID, ok := pathID(w, r, "companyID")
	if !ok {
		return
	}
	if s.usernames == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "USERNAME_REGISTRY_UNAVAILABLE", "Username registry is unavailable")
		return
	}
	if err := s.company.RequireActiveAccess(r.Context(), scope, companyID); err != nil {
		s.companyError(w, err)
		return
	}

	var input createCompanyMemberRequest
	if err := decodeJSON(r, &input); err != nil || strings.TrimSpace(input.Email) == "" || input.Password == "" || strings.TrimSpace(input.Username) == "" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Email, username, and password are required")
		return
	}
	reservation, err := s.usernames.Reserve(r.Context(), input.Username)
	if err != nil {
		if errors.Is(err, username.ErrTaken) {
			writeAPIError(w, http.StatusConflict, "USERNAME_TAKEN", "Username is already taken")
			return
		}
		if errors.Is(err, username.ErrInvalid) {
			writeAPIError(w, http.StatusBadRequest, "INVALID_USERNAME", "Invalid username")
			return
		}
		s.logger.Warn("Company member username reservation failed", "error", err)
		writeAPIError(w, http.StatusServiceUnavailable, "USERNAME_REGISTRY_UNAVAILABLE", "Username registry is unavailable")
		return
	}
	bound := false
	defer func() {
		if !bound {
			_ = s.usernames.Release(context.WithoutCancel(r.Context()), reservation)
		}
	}()

	user, response, err := s.core.AdminCreateUser(r.Context(), coreclient.AdminCreateUserRequest{
		Email: strings.TrimSpace(input.Email), Password: input.Password, Username: reservation.Username,
		Notes: input.Notes, Role: "user", Balance: input.Balance, Concurrency: input.Concurrency, RPMLimit: input.RPMLimit,
	})
	if err != nil {
		s.logger.Warn("Core company member creation failed", "error", err)
		writeAPIError(w, http.StatusBadGateway, "CORE_UNAVAILABLE", "Core user creation failed")
		return
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		writeCoreResponse(w, response)
		return
	}
	if user.ID <= 0 {
		writeAPIError(w, http.StatusBadGateway, "CORE_INVALID_RESPONSE", "Core user creation returned an invalid response")
		return
	}
	if err := s.usernames.Bind(r.Context(), reservation, user.ID); err != nil {
		_ = s.core.AdminDeleteUser(context.WithoutCancel(r.Context()), user.ID)
		s.logger.Error("Company member username bind failed", "error", err, "core_user_id", user.ID)
		writeAPIError(w, http.StatusBadGateway, "MEMBER_CREATION_INCOMPLETE", "Company member creation could not be completed")
		return
	}
	bound = true
	if err := s.company.AssignMember(r.Context(), scope, companyID, user.ID); err != nil {
		deleteCoreErr := s.core.AdminDeleteUser(context.WithoutCancel(r.Context()), user.ID)
		deleteRegistryErr := s.usernames.DeleteBound(context.WithoutCancel(r.Context()), reservation.Username, user.ID)
		if deleteCoreErr != nil || deleteRegistryErr != nil {
			s.logger.Error("Company member compensation failed", "core_user_id", user.ID, "core_error", deleteCoreErr, "registry_error", deleteRegistryErr)
		} else {
			bound = false
		}
		s.companyError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"company_id": companyID, "user_id": user.ID, "updated_members": 1})
}

func (s *Server) removeCompanyMember(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.requireScope(w, r)
	if !ok {
		return
	}
	companyID, ok := pathID(w, r, "companyID")
	if !ok {
		return
	}
	userID, ok := pathID(w, r, "userID")
	if !ok {
		return
	}
	if err := s.company.RemoveMember(r.Context(), scope, companyID, userID); err != nil {
		s.companyError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"company_id": companyID, "user_id": userID, "updated_members": 1})
}

func (s *Server) allocateCompanyMemberBalance(w http.ResponseWriter, r *http.Request) {
	accessToken, ok := bearerToken(w, r)
	if !ok {
		return
	}
	scope, ok := s.requireScope(w, r)
	if !ok {
		return
	}
	companyID, ok := pathID(w, r, "companyID")
	if !ok {
		return
	}
	userID, ok := pathID(w, r, "userID")
	if !ok {
		return
	}
	var input allocateCompanyMemberBalanceRequest
	if err := decodeJSON(r, &input); err != nil || input.Amount <= 0 {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Amount must be greater than zero")
		return
	}
	if err := s.company.RequireMember(r.Context(), scope, companyID, userID); err != nil {
		s.companyError(w, err)
		return
	}
	payload := map[string]any{
		"balance":   input.Amount,
		"operation": "add",
		"notes":     input.Notes,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid balance request")
		return
	}
	response, err := s.core.UserAPIRequest(r.Context(), coreclient.CoreRequest{
		Method:      http.MethodPost,
		Path:        "/api/v1/admin/users/" + strconv.FormatInt(userID, 10) + "/balance",
		AccessToken: accessToken,
		ContentType: "application/json",
		Body:        body,
	})
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return
	}
	writeCoreResponse(w, response)
}

func (s *Server) addCompanyManager(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.requireScope(w, r)
	if !ok {
		return
	}
	companyID, ok := pathID(w, r, "companyID")
	if !ok {
		return
	}
	var input userIDRequest
	if decodeJSON(r, &input) != nil || input.UserID <= 0 {
		writeAPIError(w, 400, "INVALID_REQUEST", "Invalid manager request")
		return
	}
	if err := s.company.AddManager(r.Context(), scope, companyID, input.UserID); err != nil {
		s.companyError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"company_id": companyID, "user_id": input.UserID})
}

func (s *Server) removeCompanyManager(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.requireScope(w, r)
	if !ok {
		return
	}
	companyID, ok := pathID(w, r, "companyID")
	if !ok {
		return
	}
	userID, ok := pathID(w, r, "userID")
	if !ok {
		return
	}
	if err := s.company.RemoveManager(r.Context(), scope, companyID, userID); err != nil {
		s.companyError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"company_id": companyID, "user_id": userID})
}

func (s *Server) listCompanyMembers(w http.ResponseWriter, r *http.Request) {
	accessToken, ok := bearerToken(w, r)
	if !ok {
		return
	}
	scope, ok := s.requireScope(w, r)
	if !ok {
		return
	}
	companyID, ok := pathID(w, r, "companyID")
	if !ok {
		return
	}
	refs, err := s.company.Members(r.Context(), scope, companyID)
	if err != nil {
		s.companyError(w, err)
		return
	}
	items := []map[string]any{}
	for _, ref := range refs {
		user, err := s.core.AdminUser(r.Context(), ref.CoreUserID)
		if err != nil {
			continue
		}
		summary, err := s.companyMemberSummary(r, accessToken, ref.CoreUserID)
		if err != nil {
			s.writeCoreProxyError(w, r, err)
			return
		}
		items = append(items, map[string]any{"user_id": user.ID, "email": user.Email, "username": user.Username, "role": user.Role, "status": user.Status, "balance": user.Balance, "api_key_count": summary.APIKeyCount, "total_cost": roundMoney(summary.TotalCost), "request_count": summary.RequestCount, "last_request_at": summary.LastRequestAt, "company_id": ref.CompanyID, "company_name": ref.CompanyName})
	}
	writeSuccess(w, map[string]any{"items": items, "total": len(items), "page": 1, "page_size": 100, "pages": 1})
}

func (s *Server) listCompanyManagers(w http.ResponseWriter, r *http.Request) {
	scope, ok := s.requireScope(w, r)
	if !ok {
		return
	}
	companyID, ok := pathID(w, r, "companyID")
	if !ok {
		return
	}
	refs, err := s.company.Managers(r.Context(), scope, companyID)
	if err != nil {
		s.companyError(w, err)
		return
	}
	items := []map[string]any{}
	for _, ref := range refs {
		user, err := s.core.AdminUser(r.Context(), ref.CoreUserID)
		if err != nil {
			continue
		}
		items = append(items, map[string]any{"company_id": ref.CompanyID, "user_id": user.ID, "email": user.Email, "username": user.Username, "role": user.Role, "status": user.Status, "created_at": ref.CreatedAt})
	}
	writeSuccess(w, items)
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		writeAPIError(w, 400, "INVALID_ID", "Invalid identifier")
		return 0, false
	}
	return id, true
}

func (s *Server) companyError(w http.ResponseWriter, err error) {
	var problem *company.Error
	if errors.As(err, &problem) {
		writeAPIError(w, problem.Status, problem.Code, problem.Message)
		return
	}
	s.logger.Warn("Portal company request failed", "error", err)
	writeAPIError(w, 500, "COMPANY_ERROR", "Company request failed")
}

func writeSuccess(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": data})
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) publicSettings(w http.ResponseWriter, r *http.Request) {
	response, err := s.core.PublicSettings(r.Context())
	if err != nil {
		s.logger.Warn("Core public settings request failed", "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"code":    "CORE_UNAVAILABLE",
			"message": "Core is unavailable",
		})
		return
	}

	// Password reset is owned by Portal. Report its actual availability and keep
	// Turnstile settings in sync instead of relying on Core's unrelated flow.
	var payload map[string]any
	if json.Unmarshal(response.Body, &payload) == nil {
		if data, ok := payload["data"].(map[string]any); ok {
			data["password_reset_enabled"] = s.passwordResetService != nil
			enabled := strings.TrimSpace(os.Getenv("PORTAL_TURNSTILE_SECRET")) != ""
			data["turnstile_enabled"] = enabled
			if siteKey := strings.TrimSpace(os.Getenv("PORTAL_TURNSTILE_SITE_KEY")); siteKey != "" {
				data["turnstile_site_key"] = siteKey
			} else if !enabled {
				data["turnstile_site_key"] = ""
			}
			if body, err := json.Marshal(payload); err == nil {
				response.Body = body
			}
		}
	}

	contentType := response.ContentType
	if !strings.HasPrefix(strings.ToLower(contentType), "application/json") {
		contentType = "application/json; charset=utf-8"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=15")
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(response.Body)
}

func (s *Server) authMe(w http.ResponseWriter, r *http.Request) {
	accessToken, ok := bearerToken(w, r)
	if !ok {
		return
	}
	response, err := s.core.ProfileResponse(r.Context(), accessToken)
	if err != nil {
		s.logger.Warn("Core profile request failed", "error", err)
		writeAPIError(w, http.StatusBadGateway, "CORE_UNAVAILABLE", "Authentication service is unavailable")
		return
	}
	writeCoreResponse(w, response)
}

type verifyCodeForwarder interface {
	SendVerifyCodeRequest(context.Context, coreclient.CoreRequest) (coreclient.Response, error)
}

func (s *Server) sendVerifyCode(w http.ResponseWriter, r *http.Request) {
	forwarder, ok := s.core.(verifyCodeForwarder)
	if !ok {
		writeAPIError(w, http.StatusServiceUnavailable, "EMAIL_VERIFICATION_UNAVAILABLE", "Email verification is unavailable")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}
	response, err := forwarder.SendVerifyCodeRequest(r.Context(), coreclient.CoreRequest{
		Method: r.Method, Path: r.URL.Path, ContentType: r.Header.Get("Content-Type"), Body: body,
	})
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return
	}
	writeCoreResponse(w, response)
}

func (s *Server) passwordReset(w http.ResponseWriter, r *http.Request) {
	if s.passwordResetService == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "PASSWORD_RESET_UNAVAILABLE", "Password reset is unavailable")
		return
	}
	if r.URL.Path == "/api/v1/auth/forgot-password" {
		var input struct {
			Email          string `json:"email"`
			TurnstileToken string `json:"turnstile_token"`
		}
		if err := decodeJSON(r, &input); err != nil {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
			return
		}
		countdown, err := s.passwordResetService.SendCode(r.Context(), input.Email, input.TurnstileToken, clientIP(r))
		if err != nil {
			s.writePasswordResetError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "data": map[string]any{"message": "If the account exists, a verification code has been sent.", "countdown": countdown}})
		return
	}

	var input struct {
		Email       string `json:"email"`
		VerifyCode  string `json:"verify_code"`
		NewPassword string `json:"new_password"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}
	if err := s.passwordResetService.ResetPassword(r.Context(), input.Email, input.VerifyCode, input.NewPassword); err != nil {
		s.writePasswordResetError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "data": map[string]string{"message": "Password reset successful"}})
}

func (s *Server) writePasswordResetError(w http.ResponseWriter, err error) {
	if typed, ok := err.(interface {
		error
		StatusCode() int
		ErrorCode() string
	}); ok {
		writeAPIError(w, typed.StatusCode(), typed.ErrorCode(), typed.Error())
		return
	}
	s.logger.Error("password reset failed", "error", err)
	writeAPIError(w, http.StatusServiceUnavailable, "PASSWORD_RESET_UNAVAILABLE", "Password reset is temporarily unavailable")
}

func clientIP(r *http.Request) string {
	if value := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); value != "" {
		return value
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return ""
}

func bearerToken(w http.ResponseWriter, r *http.Request) (string, bool) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if !strings.HasPrefix(header, "Bearer ") || token == "" {
		writeAPIError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return "", false
	}
	return token, true
}

type updateCurrentUserRequest struct {
	Username string `json:"username"`
}

func (s *Server) updateCurrentUser(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "API route is not available")
		return
	}
	accessToken, ok := bearerToken(w, r)
	if !ok {
		return
	}
	var request updateCurrentUserRequest
	if err := decodeJSON(r, &request); err != nil || strings.TrimSpace(request.Username) == "" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Username is required")
		return
	}
	user, err := s.auth.RenameCurrentUser(r.Context(), accessToken, request.Username)
	if err != nil {
		if errors.Is(err, username.ErrTaken) {
			writeAPIError(w, http.StatusConflict, "USERNAME_TAKEN", "Username is already taken")
			return
		}
		if errors.Is(err, username.ErrInvalid) {
			writeAPIError(w, http.StatusBadRequest, "INVALID_USERNAME", "Invalid username")
			return
		}
		s.logger.Warn("Portal username update failed", "error", err)
		writeAPIError(w, http.StatusBadGateway, "USERNAME_UPDATE_INCOMPLETE", "Username could not be updated")
		return
	}
	writeSuccess(w, user)
}

func (s *Server) coreUserAPI(w http.ResponseWriter, r *http.Request) {
	accessToken, ok := bearerToken(w, r)
	if !ok {
		return
	}
	if !allowedCoreUserAPI(r) {
		writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "API route is not available")
		return
	}
	response, err := s.callCoreUserAPI(r, accessToken, r.URL.RawQuery, nil)
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return
	}
	writeCoreResponse(w, response)
}

type paymentWebhookForwarder interface {
	PaymentWebhookRequest(context.Context, coreclient.CoreRequest) (coreclient.Response, error)
}

func (s *Server) paymentWebhook(w http.ResponseWriter, r *http.Request) {
	forwarder, ok := s.core.(paymentWebhookForwarder)
	if !ok {
		writeAPIError(w, http.StatusServiceUnavailable, "PAYMENT_WEBHOOK_UNAVAILABLE", "Payment webhook is unavailable")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_WEBHOOK", "Invalid payment webhook")
		return
	}
	response, err := forwarder.PaymentWebhookRequest(r.Context(), coreclient.CoreRequest{
		Method: r.Method, Path: r.URL.Path, RawQuery: r.URL.RawQuery,
		ContentType: r.Header.Get("Content-Type"), Body: body,
	})
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return
	}
	if s.rechargeCampaign != nil && response.StatusCode >= 200 && response.StatusCode < 300 && strings.EqualFold(strings.TrimSpace(string(response.Body)), "success") {
		if tradeNo := paymentWebhookTradeNo(r.URL.RawQuery, body); tradeNo != "" {
			if err := s.rechargeCampaign.FulfillByTradeNo(r.Context(), tradeNo); err != nil && !errors.Is(err, rechargepromo.ErrApplying) {
				s.logger.Error("Recharge campaign webhook bonus failed", "out_trade_no", tradeNo, "error", err)
			}
		}
	}
	if response.ContentType != "" {
		w.Header().Set("Content-Type", response.ContentType)
	}
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(response.Body)
}

func (s *Server) imgtoolURL(w http.ResponseWriter, r *http.Request) {
	accessToken, ok := bearerToken(w, r)
	if !ok {
		return
	}
	if s.core == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "IMGTOOL_UNAVAILABLE", "Image generation is not configured")
		return
	}
	user, err := s.core.Profile(r.Context(), accessToken)
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return
	}
	s.ensureImgtoolKeys(r.Context(), accessToken)
	redirectURL, err := buildImgtoolSSORedirectURL(user)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "IMGTOOL_UNAVAILABLE", "Image generation is not configured")
		return
	}
	writeSuccess(w, map[string]string{"redirect_url": redirectURL})
}

func (s *Server) coreAdminAPI(w http.ResponseWriter, r *http.Request) {
	accessToken, ok := bearerToken(w, r)
	if !ok {
		return
	}
	if !allowedCoreUserAPI(r) {
		writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "API route is not available")
		return
	}
	if !s.requireCoreAdmin(w, r, accessToken) {
		return
	}
	response, err := s.callCoreUserAPI(r, accessToken, r.URL.RawQuery, nil)
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return
	}
	writeCoreResponse(w, response)
}

func (s *Server) requireCoreAdmin(w http.ResponseWriter, r *http.Request, accessToken string) bool {
	user, err := s.core.Profile(r.Context(), accessToken)
	if err != nil {
		if errors.Is(err, coreclient.ErrUnauthorized) {
			writeAPIError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		} else {
			s.logger.Warn("Core admin authorization failed", "error", err)
			writeAPIError(w, http.StatusBadGateway, "CORE_UNAVAILABLE", "Authentication service is unavailable")
		}
		return false
	}
	if user.Status != "active" {
		writeAPIError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return false
	}
	if user.Role != "admin" {
		writeAPIError(w, http.StatusForbidden, "FULL_ADMIN_REQUIRED", "Full administrator access required")
		return false
	}
	return true
}

func (s *Server) callCoreUserAPI(r *http.Request, accessToken, rawQuery string, bodyOverride []byte) (coreclient.Response, error) {
	body := bodyOverride
	if body == nil {
		var err error
		body, err = io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return coreclient.Response{}, err
		}
	}
	return s.core.UserAPIRequest(r.Context(), coreclient.CoreRequest{
		Method:      r.Method,
		Path:        r.URL.Path,
		RawQuery:    rawQuery,
		AccessToken: accessToken,
		ContentType: r.Header.Get("Content-Type"),
		Body:        body,
		Host:        r.Host,
		Referer:     r.Referer(),
	})
}

func (s *Server) writeCoreProxyError(w http.ResponseWriter, r *http.Request, err error) {
	s.logger.Warn("Core user API proxy failed", "path", r.URL.Path, "method", r.Method, "error", err)
	writeAPIError(w, http.StatusBadGateway, "CORE_UNAVAILABLE", "Core API is unavailable")
}

func (s *Server) scopedAdminUsageAPI(w http.ResponseWriter, r *http.Request) {
	accessToken, ok := bearerToken(w, r)
	if !ok {
		return
	}
	if !allowedCoreUserAPI(r) {
		writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "API route is not available")
		return
	}
	if s.company == nil {
		response, err := s.callCoreUserAPI(r, accessToken, r.URL.RawQuery, nil)
		if err != nil {
			s.writeCoreProxyError(w, r, err)
			return
		}
		writeCoreResponse(w, response)
		return
	}
	scope, ok := s.requireScope(w, r)
	if !ok {
		return
	}
	if scope.IsAdmin {
		response, err := s.callCoreUserAPI(r, accessToken, r.URL.RawQuery, nil)
		if err != nil {
			s.writeCoreProxyError(w, r, err)
			return
		}
		writeCoreResponse(w, response)
		return
	}
	if !requireManagementScope(w, scope) {
		return
	}
	members, err := s.company.ScopedMembers(r.Context(), scope, nil)
	if err != nil {
		s.companyError(w, err)
		return
	}
	memberIDs := scopedMemberIDSet(members)
	if len(memberIDs) == 0 {
		writeSuccess(w, emptyScopedAdminPayload(r.URL.Path))
		return
	}
	if r.Method == http.MethodPost {
		s.scopedAdminUsagePost(w, r, accessToken, memberIDs)
		return
	}
	if r.URL.Path == "/api/v1/admin/usage/search-users" {
		s.scopedSearchUsers(w, r, members)
		return
	}
	query := r.URL.Query()
	if requested := strings.TrimSpace(query.Get("user_id")); requested != "" {
		userID, err := strconv.ParseInt(requested, 10, 64)
		if err != nil || !memberIDs[userID] {
			writeSuccess(w, emptyScopedAdminPayload(r.URL.Path))
			return
		}
	} else if pathSupportsSingleUserScope(r.URL.Path) && len(memberIDs) == 1 {
		for id := range memberIDs {
			query.Set("user_id", strconv.FormatInt(id, 10))
		}
	} else if r.URL.Path == "/api/v1/admin/usage" {
		s.scopedUsageListForMembers(w, r, accessToken, memberIDs)
		return
	} else if r.URL.Path == "/api/v1/admin/usage/stats" {
		s.scopedUsageStatsForMembers(w, r, accessToken, memberIDs)
		return
	} else if r.URL.Path == "/api/v1/admin/dashboard/snapshot-v2" {
		s.scopedDashboardSnapshotForMembers(w, r, accessToken, memberIDs)
		return
	} else if r.URL.Path == "/api/v1/admin/dashboard/models" {
		models, ok := s.aggregateDashboardSeries(w, r, accessToken, memberIDs, "/api/v1/admin/dashboard/models", "models", "model")
		if !ok {
			return
		}
		writeSuccess(w, map[string]any{"models": models, "start_date": r.URL.Query().Get("start_date"), "end_date": r.URL.Query().Get("end_date")})
		return
	} else if r.URL.Path == "/api/v1/admin/dashboard/groups" {
		groups, ok := s.aggregateDashboardSeries(w, r, accessToken, memberIDs, "/api/v1/admin/dashboard/groups", "groups", "group_id")
		if !ok {
			return
		}
		writeSuccess(w, map[string]any{"groups": groups, "start_date": r.URL.Query().Get("start_date"), "end_date": r.URL.Query().Get("end_date")})
		return
	} else if r.URL.Path == "/api/v1/admin/dashboard/users-trend" {
		s.scopedDashboardUsersTrendForMembers(w, r, accessToken, memberIDs)
		return
	} else if r.URL.Path == "/api/v1/admin/dashboard/users-ranking" {
		s.scopedDashboardUsersRankingForMembers(w, r, accessToken, memberIDs)
		return
	} else {
		writeSuccess(w, emptyScopedAdminPayload(r.URL.Path))
		return
	}
	response, err := s.callCoreUserAPI(r, accessToken, query.Encode(), nil)
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return
	}
	writeCoreResponse(w, response)
}

func (s *Server) scopedUsageListForMembers(w http.ResponseWriter, r *http.Request, accessToken string, memberIDs map[int64]bool) {
	page, pageSize := parsePageQuery(r.URL.Query())
	all := []map[string]any{}
	for userID := range memberIDs {
		query := cloneQuery(r.URL.Query())
		query.Set("user_id", strconv.FormatInt(userID, 10))
		query.Set("page", "1")
		query.Set("page_size", "1000")
		response, err := s.core.UserAPIRequest(r.Context(), coreclient.CoreRequest{Method: http.MethodGet, Path: "/api/v1/admin/usage", RawQuery: query.Encode(), AccessToken: accessToken})
		if err != nil {
			s.writeCoreProxyError(w, r, err)
			return
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			writeCoreResponse(w, response)
			return
		}
		var decoded apiEnvelope[corePage[map[string]any]]
		if err := json.Unmarshal(response.Body, &decoded); err != nil {
			writeAPIError(w, http.StatusBadGateway, "CORE_INVALID_RESPONSE", "Core usage response is invalid")
			return
		}
		all = append(all, decoded.Data.Items...)
	}
	sortUsageRows(all, strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort_order"))) != "asc")
	total := len(all)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	pages := int(math.Ceil(float64(total) / float64(pageSize)))
	if pages == 0 {
		pages = 1
	}
	writeSuccess(w, map[string]any{"items": all[start:end], "total": total, "page": page, "page_size": pageSize, "pages": pages})
}

func (s *Server) scopedUsageStatsForMembers(w http.ResponseWriter, r *http.Request, accessToken string, memberIDs map[int64]bool) {
	total := emptyUsageStats()
	for userID := range memberIDs {
		query := cloneQuery(r.URL.Query())
		query.Set("user_id", strconv.FormatInt(userID, 10))
		response, err := s.core.UserAPIRequest(r.Context(), coreclient.CoreRequest{Method: http.MethodGet, Path: "/api/v1/admin/usage/stats", RawQuery: query.Encode(), AccessToken: accessToken})
		if err != nil {
			s.writeCoreProxyError(w, r, err)
			return
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			writeCoreResponse(w, response)
			return
		}
		var decoded apiEnvelope[map[string]any]
		if err := json.Unmarshal(response.Body, &decoded); err != nil {
			writeAPIError(w, http.StatusBadGateway, "CORE_INVALID_RESPONSE", "Core usage stats response is invalid")
			return
		}
		addNumericStats(total, decoded.Data)
	}
	writeSuccess(w, total)
}

func (s *Server) scopedDashboardSnapshotForMembers(w http.ResponseWriter, r *http.Request, accessToken string, memberIDs map[int64]bool) {
	includeStats := parseBoolQuery(r.URL.Query().Get("include_stats"), true)
	includeTrend := parseBoolQuery(r.URL.Query().Get("include_trend"), true)
	includeModels := parseBoolQuery(r.URL.Query().Get("include_model_stats"), true)
	includeGroups := parseBoolQuery(r.URL.Query().Get("include_group_stats"), true)
	payload := map[string]any{
		"generated_at": "",
		"start_date":   r.URL.Query().Get("start_date"),
		"end_date":     r.URL.Query().Get("end_date"),
		"granularity":  r.URL.Query().Get("granularity"),
	}
	if includeStats {
		stats, ok := s.aggregateDashboardStats(w, r, accessToken, memberIDs)
		if !ok {
			return
		}
		payload["stats"] = stats
	}
	if includeTrend {
		trend, ok := s.aggregateDashboardSeries(w, r, accessToken, memberIDs, "/api/v1/admin/dashboard/trend", "trend", "date")
		if !ok {
			return
		}
		payload["trend"] = trend
	}
	if includeModels {
		models, ok := s.aggregateDashboardSeries(w, r, accessToken, memberIDs, "/api/v1/admin/dashboard/models", "models", "model")
		if !ok {
			return
		}
		payload["models"] = models
	}
	if includeGroups {
		groups, ok := s.aggregateDashboardSeries(w, r, accessToken, memberIDs, "/api/v1/admin/dashboard/groups", "groups", "group_id")
		if !ok {
			return
		}
		payload["groups"] = groups
	}
	writeSuccess(w, payload)
}

func (s *Server) scopedDashboardUsersTrendForMembers(w http.ResponseWriter, r *http.Request, accessToken string, memberIDs map[int64]bool) {
	items, ok := s.usageRowsForMembers(w, r, accessToken, memberIDs)
	if !ok {
		return
	}
	granularity := r.URL.Query().Get("granularity")
	grouped := map[string]map[string]any{}
	for _, row := range items {
		userID := int64(numericStat(row["user_id"]))
		date := usageTrendBucket(usageCreatedAt(row), granularity)
		key := strconv.FormatInt(userID, 10) + "|" + date
		item := grouped[key]
		if item == nil {
			item = map[string]any{"date": date, "user_id": userID, "email": "", "username": "", "requests": 0.0, "tokens": 0.0, "cost": 0.0, "actual_cost": 0.0}
			if user, ok := row["user"].(map[string]any); ok {
				item["email"] = stringStat(user["email"])
				item["username"] = stringStat(user["username"])
			}
			grouped[key] = item
		}
		item["requests"] = numericStat(item["requests"]) + 1
		item["tokens"] = numericStat(item["tokens"]) + numericStat(row["input_tokens"]) + numericStat(row["output_tokens"]) + numericStat(row["cache_creation_tokens"]) + numericStat(row["cache_read_tokens"]) + numericStat(row["image_output_tokens"])
		item["cost"] = numericStat(item["cost"]) + numericStat(row["total_cost"])
		item["actual_cost"] = numericStat(item["actual_cost"]) + numericStat(row["actual_cost"])
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit <= 0 {
		limit = 12
	} else if limit > 50 {
		limit = 50
	}
	result := rankedUserTrend(grouped, limit)
	writeSuccess(w, map[string]any{"trend": result, "start_date": r.URL.Query().Get("start_date"), "end_date": r.URL.Query().Get("end_date"), "granularity": r.URL.Query().Get("granularity")})
}

func rankedUserTrend(grouped map[string]map[string]any, limit int) []map[string]any {
	totals := make(map[int64]float64)
	for _, item := range grouped {
		userID := int64(numericStat(item["user_id"]))
		totals[userID] += numericStat(item["tokens"])
	}

	userIDs := make([]int64, 0, len(totals))
	for userID := range totals {
		userIDs = append(userIDs, userID)
	}
	sort.Slice(userIDs, func(i, j int) bool {
		if totals[userIDs[i]] == totals[userIDs[j]] {
			return userIDs[i] < userIDs[j]
		}
		return totals[userIDs[i]] > totals[userIDs[j]]
	})
	if limit > 0 && len(userIDs) > limit {
		userIDs = userIDs[:limit]
	}

	rank := make(map[int64]int, len(userIDs))
	for index, userID := range userIDs {
		rank[userID] = index
	}
	result := make([]map[string]any, 0, len(grouped))
	for _, item := range grouped {
		if _, ok := rank[int64(numericStat(item["user_id"]))]; ok {
			result = append(result, item)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		leftID := int64(numericStat(result[i]["user_id"]))
		rightID := int64(numericStat(result[j]["user_id"]))
		if rank[leftID] != rank[rightID] {
			return rank[leftID] < rank[rightID]
		}
		return stringStat(result[i]["date"]) < stringStat(result[j]["date"])
	})
	return result
}

func usageTrendBucket(createdAt, granularity string) string {
	if len(createdAt) < len("2006-01-02") {
		return createdAt
	}
	if granularity != "hour" || len(createdAt) < len("2006-01-02T15") {
		return createdAt[:len("2006-01-02")]
	}
	return strings.Replace(createdAt[:len("2006-01-02T15")], "T", " ", 1) + ":00"
}

func (s *Server) scopedDashboardUsersRankingForMembers(w http.ResponseWriter, r *http.Request, accessToken string, memberIDs map[int64]bool) {
	items, ok := s.usageRowsForMembers(w, r, accessToken, memberIDs)
	if !ok {
		return
	}
	grouped := map[int64]map[string]any{}
	for _, row := range items {
		userID := int64(numericStat(row["user_id"]))
		item := grouped[userID]
		if item == nil {
			item = map[string]any{"user_id": userID, "email": "", "username": "", "actual_cost": 0.0, "requests": 0.0, "tokens": 0.0}
			if user, ok := row["user"].(map[string]any); ok {
				item["email"] = stringStat(user["email"])
				item["username"] = stringStat(user["username"])
			}
			grouped[userID] = item
		}
		item["requests"] = numericStat(item["requests"]) + 1
		item["tokens"] = numericStat(item["tokens"]) + numericStat(row["input_tokens"]) + numericStat(row["output_tokens"]) + numericStat(row["cache_creation_tokens"]) + numericStat(row["cache_read_tokens"]) + numericStat(row["image_output_tokens"])
		item["actual_cost"] = numericStat(item["actual_cost"]) + numericStat(row["actual_cost"])
	}
	ranking := mapValuesInt(grouped)
	sort.SliceStable(ranking, func(i, j int) bool {
		return numericStat(ranking[i]["actual_cost"]) > numericStat(ranking[j]["actual_cost"])
	})
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 12
	}
	if len(ranking) > limit {
		ranking = ranking[:limit]
	}
	totalCost := 0.0
	totalRequests := 0.0
	totalTokens := 0.0
	for _, item := range grouped {
		totalCost += numericStat(item["actual_cost"])
		totalRequests += numericStat(item["requests"])
		totalTokens += numericStat(item["tokens"])
	}
	writeSuccess(w, map[string]any{"ranking": ranking, "total_actual_cost": totalCost, "total_requests": totalRequests, "total_tokens": totalTokens, "start_date": r.URL.Query().Get("start_date"), "end_date": r.URL.Query().Get("end_date")})
}

func (s *Server) aggregateDashboardStats(w http.ResponseWriter, r *http.Request, accessToken string, memberIDs map[int64]bool) (map[string]any, bool) {
	stats := emptyDashboardStats(len(memberIDs))
	activeUserIDs := map[int64]bool{}
	today := time.Now().In(time.FixedZone("Asia/Shanghai", 8*60*60)).Format("2006-01-02")
	for userID := range memberIDs {
		total, ok := s.dashboardUsageStats(w, r, accessToken, dashboardStatsQuery(userID, "1970-01-01", today))
		if !ok {
			return nil, false
		}
		currentDay, ok := s.dashboardUsageStats(w, r, accessToken, dashboardStatsQuery(userID, today, today))
		if !ok {
			return nil, false
		}
		addNumericStats(stats, total)
		addTodayNumericStats(stats, currentDay)
		if numericStat(currentDay["total_requests"]) > 0 {
			activeUserIDs[userID] = true
		}
	}
	keyStats, ok := s.aggregateScopedAPIKeyStats(w, r, accessToken, memberIDs)
	if !ok {
		return nil, false
	}
	stats["total_api_keys"] = keyStats.Total
	stats["active_api_keys"] = keyStats.Active
	stats["active_users"] = len(activeUserIDs)
	return stats, true
}

func dashboardStatsQuery(userID int64, startDate, endDate string) url.Values {
	return url.Values{
		"user_id":      {strconv.FormatInt(userID, 10)},
		"start_date":   {startDate},
		"end_date":     {endDate},
		"timezone":     {"Asia/Shanghai"},
		"summary_only": {"true"},
		"nocache":      {"true"},
	}
}

func (s *Server) dashboardUsageStats(w http.ResponseWriter, r *http.Request, accessToken string, query url.Values) (map[string]any, bool) {
	response, err := s.core.UserAPIRequest(r.Context(), coreclient.CoreRequest{
		Method: http.MethodGet, Path: "/api/v1/admin/usage/stats", RawQuery: query.Encode(), AccessToken: accessToken,
	})
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return nil, false
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		writeCoreResponse(w, response)
		return nil, false
	}
	var decoded apiEnvelope[map[string]any]
	if err := json.Unmarshal(response.Body, &decoded); err != nil {
		writeAPIError(w, http.StatusBadGateway, "CORE_INVALID_RESPONSE", "Core usage stats response is invalid")
		return nil, false
	}
	return decoded.Data, true
}

type scopedAPIKeyStats struct {
	Total  int
	Active int
}

type companyMemberSummary struct {
	APIKeyCount   int
	TotalCost     float64
	RequestCount  int64
	LastRequestAt string
}

func (s *Server) companyMemberSummary(r *http.Request, accessToken string, userID int64) (companyMemberSummary, error) {
	summary := companyMemberSummary{}
	keyResponse, err := s.core.UserAPIRequest(r.Context(), coreclient.CoreRequest{Method: http.MethodGet, Path: "/api/v1/admin/users/" + strconv.FormatInt(userID, 10) + "/api-keys", RawQuery: "page=1&page_size=1&exact_total=true", AccessToken: accessToken})
	if err != nil {
		return summary, err
	}
	if keyResponse.StatusCode < 200 || keyResponse.StatusCode >= 300 {
		return summary, errors.New("Core API key query returned non-success status")
	}
	var decodedKeys apiEnvelope[corePage[map[string]any]]
	if err := json.Unmarshal(keyResponse.Body, &decodedKeys); err != nil {
		return summary, err
	}
	summary.APIKeyCount = int(decodedKeys.Data.Total)

	statsQuery := url.Values{
		"user_id":      {strconv.FormatInt(userID, 10)},
		"start_date":   {"1970-01-01"},
		"end_date":     {time.Now().In(time.FixedZone("CST", 8*60*60)).Format("2006-01-02")},
		"timezone":     {"Asia/Shanghai"},
		"summary_only": {"true"},
		"nocache":      {"true"},
	}
	stats, err := s.financeUsageStats(r.Context(), accessToken, statsQuery)
	if err != nil {
		return summary, err
	}
	summary.RequestCount = stats.TotalRequests
	summary.TotalCost = stats.TotalActualCost

	latestQuery := url.Values{
		"user_id": {strconv.FormatInt(userID, 10)}, "page": {"1"}, "page_size": {"1"}, "exact_total": {"false"},
	}
	latestResponse, err := s.core.UserAPIRequest(r.Context(), coreclient.CoreRequest{Method: http.MethodGet, Path: "/api/v1/admin/usage", RawQuery: latestQuery.Encode(), AccessToken: accessToken})
	if err != nil {
		return summary, err
	}
	if latestResponse.StatusCode < 200 || latestResponse.StatusCode >= 300 {
		return summary, errors.New("Core latest usage query returned non-success status")
	}
	var latest apiEnvelope[corePage[map[string]any]]
	if err := json.Unmarshal(latestResponse.Body, &latest); err != nil {
		return summary, err
	}
	if len(latest.Data.Items) > 0 {
		summary.LastRequestAt = stringStat(latest.Data.Items[0]["created_at"])
	}
	return summary, nil
}

func (s *Server) aggregateScopedAPIKeyStats(w http.ResponseWriter, r *http.Request, accessToken string, memberIDs map[int64]bool) (scopedAPIKeyStats, bool) {
	result := scopedAPIKeyStats{}
	for userID := range memberIDs {
		response, err := s.core.UserAPIRequest(r.Context(), coreclient.CoreRequest{Method: http.MethodGet, Path: "/api/v1/admin/users/" + strconv.FormatInt(userID, 10) + "/api-keys", RawQuery: "page=1&page_size=1000", AccessToken: accessToken})
		if err != nil {
			s.writeCoreProxyError(w, r, err)
			return result, false
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			writeCoreResponse(w, response)
			return result, false
		}
		var decoded apiEnvelope[corePage[map[string]any]]
		if err := json.Unmarshal(response.Body, &decoded); err != nil {
			writeAPIError(w, http.StatusBadGateway, "CORE_INVALID_RESPONSE", "Core API key response is invalid")
			return result, false
		}
		result.Total += len(decoded.Data.Items)
		for _, item := range decoded.Data.Items {
			if strings.EqualFold(stringStat(item["status"]), "active") {
				result.Active++
			}
		}
	}
	return result, true
}

func (s *Server) aggregateDashboardSeries(w http.ResponseWriter, r *http.Request, accessToken string, memberIDs map[int64]bool, path, dataKey, groupKey string) ([]map[string]any, bool) {
	grouped := map[string]map[string]any{}
	for userID := range memberIDs {
		query := cloneQuery(r.URL.Query())
		query.Set("user_id", strconv.FormatInt(userID, 10))
		response, err := s.core.UserAPIRequest(r.Context(), coreclient.CoreRequest{Method: http.MethodGet, Path: path, RawQuery: query.Encode(), AccessToken: accessToken})
		if err != nil {
			s.writeCoreProxyError(w, r, err)
			return nil, false
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			writeCoreResponse(w, response)
			return nil, false
		}
		var decoded apiEnvelope[map[string]any]
		if err := json.Unmarshal(response.Body, &decoded); err != nil {
			writeAPIError(w, http.StatusBadGateway, "CORE_INVALID_RESPONSE", "Core dashboard response is invalid")
			return nil, false
		}
		rows, _ := decoded.Data[dataKey].([]any)
		for _, raw := range rows {
			row, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			key := dashboardSeriesGroupKey(row, groupKey)
			if key == "" {
				key = "unknown"
			}
			item := grouped[key]
			if item == nil {
				item = cloneMap(row)
				grouped[key] = item
			} else {
				for _, field := range []string{"requests", "input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens", "total_tokens", "cost", "actual_cost", "account_cost"} {
					item[field] = numericStat(item[field]) + numericStat(row[field])
				}
			}
		}
	}
	result := mapValues(grouped)
	return result, true
}

func dashboardSeriesGroupKey(row map[string]any, groupKey string) string {
	if groupKey == "group_id" {
		return strconv.FormatInt(int64(numericStat(row[groupKey])), 10)
	}
	return stringStat(row[groupKey])
}

func (s *Server) usageRowsForMembers(w http.ResponseWriter, r *http.Request, accessToken string, memberIDs map[int64]bool) ([]map[string]any, bool) {
	all := []map[string]any{}
	for userID := range memberIDs {
		query := cloneQuery(r.URL.Query())
		query.Set("user_id", strconv.FormatInt(userID, 10))
		query.Set("page", "1")
		query.Set("page_size", "1000")
		response, err := s.core.UserAPIRequest(r.Context(), coreclient.CoreRequest{Method: http.MethodGet, Path: "/api/v1/admin/usage", RawQuery: query.Encode(), AccessToken: accessToken})
		if err != nil {
			s.writeCoreProxyError(w, r, err)
			return nil, false
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			writeCoreResponse(w, response)
			return nil, false
		}
		var decoded apiEnvelope[corePage[map[string]any]]
		if err := json.Unmarshal(response.Body, &decoded); err != nil {
			writeAPIError(w, http.StatusBadGateway, "CORE_INVALID_RESPONSE", "Core usage response is invalid")
			return nil, false
		}
		all = append(all, decoded.Data.Items...)
	}
	return all, true
}

func (s *Server) scopedAdminUsagePost(w http.ResponseWriter, r *http.Request, accessToken string, memberIDs map[int64]bool) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}
	if r.URL.Path == "/api/v1/admin/dashboard/users-usage" {
		var payload struct {
			UserIDs []int64 `json:"user_ids"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
			return
		}
		filtered := []int64{}
		for _, id := range payload.UserIDs {
			if memberIDs[id] {
				filtered = append(filtered, id)
			}
		}
		body, _ = json.Marshal(map[string]any{"user_ids": filtered})
		response, err := s.callCoreUserAPI(r, accessToken, r.URL.RawQuery, body)
		if err != nil {
			s.writeCoreProxyError(w, r, err)
			return
		}
		writeCoreResponse(w, response)
		return
	}
	writeSuccess(w, emptyScopedAdminPayload(r.URL.Path))
}

func (s *Server) scopedSearchUsers(w http.ResponseWriter, r *http.Request, members []company.ScopedMemberRef) {
	keyword := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	result := []map[string]any{}
	for _, member := range members {
		user, err := s.core.AdminUser(r.Context(), member.CoreUserID)
		if err != nil {
			s.logger.Warn("Core admin user lookup failed for scoped search", "user_id", member.CoreUserID, "error", err)
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(user.Email), keyword) && !strings.Contains(strings.ToLower(user.Username), keyword) {
			continue
		}
		result = append(result, map[string]any{"id": user.ID, "email": user.Email, "username": user.Username, "deleted": false})
	}
	writeSuccess(w, result)
}

func scopedMemberIDSet(members []company.ScopedMemberRef) map[int64]bool {
	result := map[int64]bool{}
	for _, member := range members {
		if member.CoreUserID > 0 {
			result[member.CoreUserID] = true
		}
	}
	return result
}

func pathSupportsSingleUserScope(path string) bool {
	switch path {
	case "/api/v1/admin/usage", "/api/v1/admin/usage/stats",
		"/api/v1/admin/dashboard/snapshot-v2", "/api/v1/admin/dashboard/trend",
		"/api/v1/admin/dashboard/models", "/api/v1/admin/dashboard/groups",
		"/api/v1/admin/dashboard/user-breakdown":
		return true
	default:
		return false
	}
}

func emptyScopedAdminPayload(path string) any {
	switch path {
	case "/api/v1/admin/usage":
		return map[string]any{"items": []any{}, "total": 0, "page": 1, "page_size": 20, "pages": 0}
	case "/api/v1/admin/usage/stats":
		return emptyUsageStats()
	case "/api/v1/admin/usage/search-users", "/api/v1/admin/usage/search-api-keys":
		return []any{}
	case "/api/v1/admin/dashboard/snapshot-v2":
		return map[string]any{"stats": emptyUsageStats(), "trend": []any{}, "models": []any{}, "groups": []any{}, "users_trend": []any{}}
	case "/api/v1/admin/dashboard/trend", "/api/v1/admin/dashboard/api-keys-trend", "/api/v1/admin/dashboard/users-trend":
		return map[string]any{"trend": []any{}}
	case "/api/v1/admin/dashboard/models":
		return map[string]any{"models": []any{}}
	case "/api/v1/admin/dashboard/groups":
		return map[string]any{"groups": []any{}}
	case "/api/v1/admin/dashboard/users-ranking":
		return map[string]any{"ranking": []any{}, "total_actual_cost": 0, "total_requests": 0, "total_tokens": 0}
	case "/api/v1/admin/dashboard/user-breakdown":
		return map[string]any{"users": []any{}}
	case "/api/v1/admin/dashboard/users-usage", "/api/v1/admin/dashboard/api-keys-usage":
		return map[string]any{"stats": map[string]any{}}
	default:
		return map[string]any{}
	}
}

func emptyUsageStats() map[string]any {
	return map[string]any{
		"total_requests": 0, "total_input_tokens": 0, "total_output_tokens": 0,
		"total_cache_tokens": 0, "total_cache_creation_tokens": 0, "total_cache_read_tokens": 0,
		"total_tokens": 0, "total_cost": 0, "total_actual_cost": 0, "total_account_cost": 0,
		"average_duration_ms": 0, "endpoints": []any{}, "upstream_endpoints": []any{}, "endpoint_paths": []any{},
	}
}

func requireManagementScope(w http.ResponseWriter, scope company.Scope) bool {
	if scope.IsAdmin || len(scope.CompanyIDs) > 0 {
		return true
	}
	writeAPIError(w, http.StatusForbidden, "MANAGEMENT_SCOPE_REQUIRED", "Management access required")
	return false
}

type financeUsageRow struct {
	UserID              int64     `json:"user_id"`
	RequestCount        int64     `json:"-"`
	InputTokens         int64     `json:"input_tokens"`
	OutputTokens        int64     `json:"output_tokens"`
	CacheCreationTokens int64     `json:"cache_creation_tokens"`
	CacheReadTokens     int64     `json:"cache_read_tokens"`
	ImageOutputTokens   int64     `json:"image_output_tokens"`
	ActualCost          float64   `json:"actual_cost"`
	GroupID             *int64    `json:"group_id"`
	Group               *coreName `json:"group"`
	User                *coreName `json:"user"`
}

type financeUsageStats struct {
	TotalRequests            int64   `json:"total_requests"`
	TotalInputTokens         int64   `json:"total_input_tokens"`
	TotalOutputTokens        int64   `json:"total_output_tokens"`
	TotalCacheCreationTokens int64   `json:"total_cache_creation_tokens"`
	TotalCacheReadTokens     int64   `json:"total_cache_read_tokens"`
	TotalImageOutputTokens   int64   `json:"total_image_output_tokens"`
	TotalActualCost          float64 `json:"total_actual_cost"`
}

type financeGroupStat struct {
	GroupID   int64  `json:"group_id"`
	GroupName string `json:"group_name"`
}

type financeGroupStatsData struct {
	Groups []financeGroupStat `json:"groups"`
}

func (s *financeUsageStats) add(other financeUsageStats) {
	s.TotalRequests += other.TotalRequests
	s.TotalInputTokens += other.TotalInputTokens
	s.TotalOutputTokens += other.TotalOutputTokens
	s.TotalCacheCreationTokens += other.TotalCacheCreationTokens
	s.TotalCacheReadTokens += other.TotalCacheReadTokens
	s.TotalImageOutputTokens += other.TotalImageOutputTokens
	s.TotalActualCost += other.TotalActualCost
}

func (s financeUsageStats) subtract(other financeUsageStats) financeUsageStats {
	return financeUsageStats{
		TotalRequests:            max(s.TotalRequests-other.TotalRequests, 0),
		TotalInputTokens:         max(s.TotalInputTokens-other.TotalInputTokens, 0),
		TotalOutputTokens:        max(s.TotalOutputTokens-other.TotalOutputTokens, 0),
		TotalCacheCreationTokens: max(s.TotalCacheCreationTokens-other.TotalCacheCreationTokens, 0),
		TotalCacheReadTokens:     max(s.TotalCacheReadTokens-other.TotalCacheReadTokens, 0),
		TotalImageOutputTokens:   max(s.TotalImageOutputTokens-other.TotalImageOutputTokens, 0),
		TotalActualCost:          math.Max(s.TotalActualCost-other.TotalActualCost, 0),
	}
}

type coreName struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Username  string `json:"username"`
	DeletedAt string `json:"deleted_at,omitempty"`
}

type corePage[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Pages    int   `json:"pages"`
}

type apiEnvelope[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type financeSummaryRow struct {
	CompanyID            int64   `json:"company_id"`
	CompanyName          string  `json:"company_name"`
	Platform             string  `json:"platform,omitempty"`
	GroupID              int64   `json:"group_id"`
	GroupName            string  `json:"group_name"`
	MemberCount          int     `json:"member_count"`
	RequestCount         int64   `json:"request_count"`
	InputTokens          int64   `json:"input_tokens"`
	OutputTokens         int64   `json:"output_tokens"`
	CacheCreationTokens  int64   `json:"cache_creation_tokens"`
	CacheReadTokens      int64   `json:"cache_read_tokens"`
	ImageOutputTokens    int64   `json:"image_output_tokens"`
	ActualChargedAmount  float64 `json:"actual_charged_amount"`
	seenMemberCoreUserID map[int64]bool
}

type financeMemberRow struct {
	CompanyID           int64   `json:"company_id"`
	CompanyName         string  `json:"company_name"`
	Platform            string  `json:"platform,omitempty"`
	GroupID             int64   `json:"group_id"`
	GroupName           string  `json:"group_name"`
	UserID              int64   `json:"user_id"`
	Username            string  `json:"username"`
	UserEmail           string  `json:"user_email,omitempty"`
	UserDeletedAt       string  `json:"user_deleted_at,omitempty"`
	RequestCount        int64   `json:"request_count"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	ImageOutputTokens   int64   `json:"image_output_tokens"`
	ActualChargedAmount float64 `json:"actual_charged_amount"`
}

func (s *Server) financeSummary(w http.ResponseWriter, r *http.Request) {
	accessToken, ok := bearerToken(w, r)
	if !ok {
		return
	}
	scope, ok := s.requireScope(w, r)
	if !ok {
		return
	}
	if !requireManagementScope(w, scope) {
		return
	}
	members, err := s.financeScopedMembers(r, scope)
	if err != nil {
		s.companyError(w, err)
		return
	}
	rows, err := s.financeUsageRows(r, accessToken, members)
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return
	}
	byUser := financeMembersByUser(members)
	result := aggregateFinanceSummaryRows(rows, byUser, strings.TrimSpace(r.URL.Query().Get("platform")))
	writeSuccess(w, result)
}

func (s *Server) financeMembers(w http.ResponseWriter, r *http.Request) {
	accessToken, ok := bearerToken(w, r)
	if !ok {
		return
	}
	scope, ok := s.requireScope(w, r)
	if !ok {
		return
	}
	if !requireManagementScope(w, scope) {
		return
	}
	members, err := s.financeScopedMembers(r, scope)
	if err != nil {
		s.companyError(w, err)
		return
	}
	rows, err := s.financeUsageRows(r, accessToken, members)
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return
	}
	byUser := financeMembersByUser(members)
	all := aggregateFinanceMemberRows(rows, byUser, strings.TrimSpace(r.URL.Query().Get("platform")), false)
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].ActualChargedAmount == all[j].ActualChargedAmount {
			if all[i].RequestCount == all[j].RequestCount {
				if all[i].CompanyID == all[j].CompanyID {
					return all[i].UserID < all[j].UserID
				}
				return all[i].CompanyID < all[j].CompanyID
			}
			return all[i].RequestCount > all[j].RequestCount
		}
		return all[i].ActualChargedAmount > all[j].ActualChargedAmount
	})
	page, pageSize := parsePageQuery(r.URL.Query())
	total := len(all)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	pages := int(math.Ceil(float64(total) / float64(pageSize)))
	if pages == 0 {
		pages = 1
	}
	writeSuccess(w, map[string]any{"items": all[start:end], "total": total, "page": page, "page_size": pageSize, "pages": pages})
}

func (s *Server) financeMemberBreakdown(w http.ResponseWriter, r *http.Request) {
	accessToken, ok := bearerToken(w, r)
	if !ok {
		return
	}
	scope, ok := s.requireScope(w, r)
	if !ok {
		return
	}
	if !requireManagementScope(w, scope) {
		return
	}
	userID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("userID")), 10, 64)
	if err != nil || userID <= 0 {
		writeAPIError(w, http.StatusBadRequest, "INVALID_USER", "Invalid user")
		return
	}
	members, err := s.financeScopedMembers(r, scope)
	if err != nil {
		s.companyError(w, err)
		return
	}
	members = filterFinanceMembersByUser(members, userID)
	if len(members) == 0 {
		writeAPIError(w, http.StatusNotFound, "COMPANY_MEMBER_NOT_FOUND", "Company member not found")
		return
	}
	rows, err := s.financeUsageRows(r, accessToken, members)
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return
	}
	byUser := financeMembersByUser(members)
	result := aggregateFinanceMemberRows(rows, byUser, strings.TrimSpace(r.URL.Query().Get("platform")), true)
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].ActualChargedAmount == result[j].ActualChargedAmount {
			if result[i].CompanyID == result[j].CompanyID {
				return result[i].GroupID < result[j].GroupID
			}
			return result[i].CompanyID < result[j].CompanyID
		}
		return result[i].ActualChargedAmount > result[j].ActualChargedAmount
	})
	writeSuccess(w, result)
}

func (s *Server) financeExport(w http.ResponseWriter, r *http.Request) {
	accessToken, ok := bearerToken(w, r)
	if !ok {
		return
	}
	scope, ok := s.requireScope(w, r)
	if !ok {
		return
	}
	if !requireManagementScope(w, scope) {
		return
	}
	members, err := s.financeScopedMembers(r, scope)
	if err != nil {
		s.companyError(w, err)
		return
	}
	rows, err := s.financeUsageRows(r, accessToken, members)
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return
	}
	byUser := financeMembersByUser(members)
	all := aggregateFinanceMemberRows(rows, byUser, strings.TrimSpace(r.URL.Query().Get("platform")), false)
	buffer := &bytes.Buffer{}
	buffer.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(buffer)
	_ = writer.Write([]string{"company_id", "company_name", "platform", "user_id", "username", "user_email", "user_deleted_at", "request_count", "input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens", "image_output_tokens", "actual_charged_amount"})
	for _, row := range all {
		_ = writer.Write([]string{
			strconv.FormatInt(row.CompanyID, 10),
			row.CompanyName,
			row.Platform,
			strconv.FormatInt(row.UserID, 10),
			row.Username,
			row.UserEmail,
			row.UserDeletedAt,
			strconv.FormatInt(row.RequestCount, 10),
			strconv.FormatInt(row.InputTokens, 10),
			strconv.FormatInt(row.OutputTokens, 10),
			strconv.FormatInt(row.CacheCreationTokens, 10),
			strconv.FormatInt(row.CacheReadTokens, 10),
			strconv.FormatInt(row.ImageOutputTokens, 10),
			strconv.FormatFloat(roundMoney(row.ActualChargedAmount), 'f', 6, 64),
		})
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "FINANCE_EXPORT_FAILED", "Finance export failed")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="finance-report.csv"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buffer.Bytes())
}

func (s *Server) financeMemberExport(w http.ResponseWriter, r *http.Request) {
	accessToken, ok := bearerToken(w, r)
	if !ok {
		return
	}
	scope, ok := s.requireScope(w, r)
	if !ok {
		return
	}
	if !requireManagementScope(w, scope) {
		return
	}
	userID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("userID")), 10, 64)
	if err != nil || userID <= 0 {
		writeAPIError(w, http.StatusBadRequest, "INVALID_USER", "Invalid user")
		return
	}
	members, err := s.financeScopedMembers(r, scope)
	if err != nil {
		s.companyError(w, err)
		return
	}
	members = filterFinanceMembersByUser(members, userID)
	if len(members) == 0 {
		writeAPIError(w, http.StatusNotFound, "COMPANY_MEMBER_NOT_FOUND", "Company member not found")
		return
	}
	rows, err := s.financeUsageRows(r, accessToken, members)
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return
	}
	byUser := financeMembersByUser(members)
	all := aggregateFinanceMemberRows(rows, byUser, strings.TrimSpace(r.URL.Query().Get("platform")), true)
	buffer := &bytes.Buffer{}
	buffer.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(buffer)
	_ = writer.Write([]string{"company_id", "company_name", "platform", "group_id", "group_name", "user_id", "username", "user_email", "user_deleted_at", "request_count", "input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens", "image_output_tokens", "actual_charged_amount"})
	for _, row := range all {
		_ = writer.Write([]string{
			strconv.FormatInt(row.CompanyID, 10),
			row.CompanyName,
			row.Platform,
			strconv.FormatInt(row.GroupID, 10),
			row.GroupName,
			strconv.FormatInt(row.UserID, 10),
			row.Username,
			row.UserEmail,
			row.UserDeletedAt,
			strconv.FormatInt(row.RequestCount, 10),
			strconv.FormatInt(row.InputTokens, 10),
			strconv.FormatInt(row.OutputTokens, 10),
			strconv.FormatInt(row.CacheCreationTokens, 10),
			strconv.FormatInt(row.CacheReadTokens, 10),
			strconv.FormatInt(row.ImageOutputTokens, 10),
			strconv.FormatFloat(roundMoney(row.ActualChargedAmount), 'f', 6, 64),
		})
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "FINANCE_EXPORT_FAILED", "Finance export failed")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="finance-member-detail.csv"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buffer.Bytes())
}

func (s *Server) financeScopedMembers(r *http.Request, scope company.Scope) ([]company.ScopedMemberRef, error) {
	return s.company.ScopedMembers(r.Context(), scope, parseIDListQuery(r.URL.Query(), "company_id", "company_ids"))
}

func (s *Server) financeUsageRows(r *http.Request, accessToken string, members []company.ScopedMemberRef) ([]financeUsageRow, error) {
	result := []financeUsageRow{}
	var resultMu sync.Mutex
	group, ctx := errgroup.WithContext(r.Context())
	group.SetLimit(8)
	for _, member := range members {
		member := member
		group.Go(func() error {
			rows, err := s.financeUsageRowsForMember(ctx, accessToken, r.URL.Query(), member.CoreUserID)
			if err != nil {
				return err
			}
			resultMu.Lock()
			result = append(result, rows...)
			resultMu.Unlock()
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Server) financeUsageRowsForMember(ctx context.Context, accessToken string, source url.Values, userID int64) ([]financeUsageRow, error) {
	user, err := s.core.AdminUser(ctx, userID)
	missingUser := false
	if err != nil {
		if errors.Is(err, coreclient.ErrNotFound) {
			missingUser = true
			user = coreclient.User{ID: userID}
		} else {
			return nil, err
		}
	}
	total, err := s.financeUsageStats(ctx, accessToken, financeCoreStatsQuery(source, userID, 0))
	if err != nil {
		return nil, err
	}
	userName := &coreName{ID: userID, Email: user.Email, Username: user.Username}
	if missingUser {
		userName.DeletedAt = "missing"
	}

	groupResponse, err := s.core.UserAPIRequest(ctx, coreclient.CoreRequest{
		Method: http.MethodGet, Path: "/api/v1/admin/dashboard/groups",
		RawQuery: financeGroupDiscoveryQuery(source, userID).Encode(), AccessToken: accessToken,
	})
	if err != nil {
		return nil, err
	}
	if groupResponse.StatusCode < 200 || groupResponse.StatusCode >= 300 {
		if missingUser && total.TotalRequests > 0 {
			return []financeUsageRow{{
				UserID: userID, InputTokens: total.TotalInputTokens, OutputTokens: total.TotalOutputTokens,
				RequestCount:        total.TotalRequests,
				CacheCreationTokens: total.TotalCacheCreationTokens, CacheReadTokens: total.TotalCacheReadTokens,
				ImageOutputTokens: total.TotalImageOutputTokens, ActualCost: total.TotalActualCost,
				User: userName,
			}}, nil
		}
		return nil, errors.New("Core group statistics query returned non-success status")
	}
	var decodedGroups apiEnvelope[financeGroupStatsData]
	if err := json.Unmarshal(groupResponse.Body, &decodedGroups); err != nil {
		return nil, err
	}

	rows := make([]financeUsageRow, 0, len(decodedGroups.Data.Groups)+1)
	grouped := financeUsageStats{}
	for _, group := range decodedGroups.Data.Groups {
		// Core reports ungrouped usage as group 0. It is derived from total minus
		// named groups below because group_id=0 means "no filter" to the stats API.
		if group.GroupID == 0 {
			continue
		}
		stats, err := s.financeUsageStats(ctx, accessToken, financeCoreStatsQuery(source, userID, group.GroupID))
		if err != nil {
			return nil, err
		}
		if stats.TotalRequests == 0 {
			continue
		}
		groupID := group.GroupID
		rows = append(rows, financeUsageRow{
			UserID: userID, InputTokens: stats.TotalInputTokens, OutputTokens: stats.TotalOutputTokens,
			RequestCount:        stats.TotalRequests,
			CacheCreationTokens: stats.TotalCacheCreationTokens, CacheReadTokens: stats.TotalCacheReadTokens,
			ImageOutputTokens: stats.TotalImageOutputTokens, ActualCost: stats.TotalActualCost,
			GroupID: &groupID, Group: &coreName{ID: groupID, Name: group.GroupName},
			User: userName,
		})
		grouped.add(stats)
	}

	ungrouped := total.subtract(grouped)
	if ungrouped.TotalRequests > 0 {
		row := financeUsageRow{
			UserID: userID, InputTokens: ungrouped.TotalInputTokens, OutputTokens: ungrouped.TotalOutputTokens,
			RequestCount:        ungrouped.TotalRequests,
			CacheCreationTokens: ungrouped.TotalCacheCreationTokens, CacheReadTokens: ungrouped.TotalCacheReadTokens,
			ImageOutputTokens: ungrouped.TotalImageOutputTokens, ActualCost: ungrouped.TotalActualCost,
			User: userName,
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (s *Server) financeUsageStats(ctx context.Context, accessToken string, query url.Values) (financeUsageStats, error) {
	response, err := s.core.UserAPIRequest(ctx, coreclient.CoreRequest{
		Method: http.MethodGet, Path: "/api/v1/admin/usage/stats", RawQuery: query.Encode(), AccessToken: accessToken,
	})
	if err != nil {
		return financeUsageStats{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return financeUsageStats{}, errors.New("Core usage statistics query returned non-success status")
	}
	var decoded apiEnvelope[financeUsageStats]
	if err := json.Unmarshal(response.Body, &decoded); err != nil {
		return financeUsageStats{}, err
	}
	return decoded.Data, nil
}

func financeCoreStatsQuery(source url.Values, userID, groupID int64) url.Values {
	query := url.Values{}
	for _, key := range []string{"start_date", "end_date", "group_id", "model", "request_type", "billing_type", "billing_mode", "timezone"} {
		if value := strings.TrimSpace(source.Get(key)); value != "" {
			query.Set(key, value)
		}
	}
	query.Set("user_id", strconv.FormatInt(userID, 10))
	if groupID > 0 {
		query.Set("group_id", strconv.FormatInt(groupID, 10))
	}
	query.Set("summary_only", "true")
	query.Set("nocache", "true")
	return query
}

func financeGroupDiscoveryQuery(source url.Values, userID int64) url.Values {
	query := url.Values{}
	for _, key := range []string{"start_date", "end_date", "group_id", "request_type", "billing_type", "timezone"} {
		if value := strings.TrimSpace(source.Get(key)); value != "" {
			query.Set(key, value)
		}
	}
	query.Set("user_id", strconv.FormatInt(userID, 10))
	return query
}

func financeMembersByUser(members []company.ScopedMemberRef) map[int64]company.ScopedMemberRef {
	result := map[int64]company.ScopedMemberRef{}
	for _, member := range members {
		result[member.CoreUserID] = member
	}
	return result
}

func filterFinanceMembersByUser(members []company.ScopedMemberRef, userID int64) []company.ScopedMemberRef {
	result := []company.ScopedMemberRef{}
	for _, member := range members {
		if member.CoreUserID == userID {
			result = append(result, member)
		}
	}
	return result
}

func aggregateFinanceSummaryRows(rows []financeUsageRow, byUser map[int64]company.ScopedMemberRef, platform string) []financeSummaryRow {
	grouped := map[string]*financeSummaryRow{}
	for _, row := range rows {
		member := byUser[row.UserID]
		key := strconv.FormatInt(member.CompanyID, 10) + "|" + platform
		item := grouped[key]
		if item == nil {
			item = &financeSummaryRow{CompanyID: member.CompanyID, CompanyName: member.CompanyName, Platform: platform, seenMemberCoreUserID: map[int64]bool{}}
			grouped[key] = item
		}
		item.RequestCount += row.RequestCount
		item.InputTokens += row.InputTokens
		item.OutputTokens += row.OutputTokens
		item.CacheCreationTokens += row.CacheCreationTokens
		item.CacheReadTokens += row.CacheReadTokens
		item.ImageOutputTokens += row.ImageOutputTokens
		item.ActualChargedAmount += row.ActualCost
		item.seenMemberCoreUserID[row.UserID] = true
		item.MemberCount = len(item.seenMemberCoreUserID)
	}
	result := make([]financeSummaryRow, 0, len(grouped))
	for _, item := range grouped {
		item.ActualChargedAmount = roundMoney(item.ActualChargedAmount)
		item.seenMemberCoreUserID = nil
		result = append(result, *item)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].ActualChargedAmount == result[j].ActualChargedAmount {
			if result[i].RequestCount == result[j].RequestCount {
				if result[i].CompanyID == result[j].CompanyID {
					return result[i].Platform < result[j].Platform
				}
				return result[i].CompanyID < result[j].CompanyID
			}
			return result[i].RequestCount > result[j].RequestCount
		}
		return result[i].ActualChargedAmount > result[j].ActualChargedAmount
	})
	return result
}

func aggregateFinanceMemberRows(rows []financeUsageRow, byUser map[int64]company.ScopedMemberRef, platform string, includeGroup bool) []financeMemberRow {
	grouped := map[string]*financeMemberRow{}
	for _, row := range rows {
		member := byUser[row.UserID]
		groupID, groupName := int64(0), ""
		if includeGroup {
			groupID, groupName = financeGroup(row)
		}
		key := strconv.FormatInt(member.CompanyID, 10) + "|" + platform + "|" + strconv.FormatInt(row.UserID, 10)
		if includeGroup {
			key += "|" + strconv.FormatInt(groupID, 10)
		}
		item := grouped[key]
		if item == nil {
			item = &financeMemberRow{
				CompanyID: member.CompanyID, CompanyName: member.CompanyName, Platform: platform,
				GroupID: groupID, GroupName: groupName, UserID: row.UserID, Username: financeUsername(row),
				UserEmail: financeUserEmail(row), UserDeletedAt: financeUserDeletedAt(row),
			}
			grouped[key] = item
		}
		item.RequestCount += row.RequestCount
		item.InputTokens += row.InputTokens
		item.OutputTokens += row.OutputTokens
		item.CacheCreationTokens += row.CacheCreationTokens
		item.CacheReadTokens += row.CacheReadTokens
		item.ImageOutputTokens += row.ImageOutputTokens
		item.ActualChargedAmount += row.ActualCost
	}
	return mapFinanceMemberRows(grouped)
}

func mapFinanceMemberRows(source map[string]*financeMemberRow) []financeMemberRow {
	result := make([]financeMemberRow, 0, len(source))
	for _, item := range source {
		item.ActualChargedAmount = roundMoney(item.ActualChargedAmount)
		result = append(result, *item)
	}
	return result
}

func financeGroup(row financeUsageRow) (int64, string) {
	if row.GroupID != nil {
		name := "未分组"
		if row.Group != nil && strings.TrimSpace(row.Group.Name) != "" {
			name = row.Group.Name
		}
		return *row.GroupID, name
	}
	return 0, "未分组"
}

func financeUsername(row financeUsageRow) string {
	if row.User != nil {
		if strings.TrimSpace(row.User.Username) != "" {
			return row.User.Username
		}
		if strings.TrimSpace(row.User.Email) != "" {
			return row.User.Email
		}
	}
	return strconv.FormatInt(row.UserID, 10)
}

func financeUserEmail(row financeUsageRow) string {
	if row.User != nil {
		return strings.TrimSpace(row.User.Email)
	}
	return ""
}

func financeUserDeletedAt(row financeUsageRow) string {
	if row.User != nil {
		return strings.TrimSpace(row.User.DeletedAt)
	}
	return ""
}

func parseIDListQuery(query url.Values, singleKey, listKey string) []int64 {
	ids := []int64{}
	for _, raw := range append(query[singleKey], strings.Split(query.Get(listKey), ",")...) {
		if id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

func parsePageQuery(query url.Values) (int, int) {
	page, _ := strconv.Atoi(query.Get("page"))
	pageSize, _ := strconv.Atoi(query.Get("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func roundMoney(value float64) float64 {
	return math.Round(value*1_000_000) / 1_000_000
}

func cloneQuery(source url.Values) url.Values {
	query := url.Values{}
	for key, values := range source {
		for _, value := range values {
			query.Add(key, value)
		}
	}
	return query
}

func sortUsageRows(rows []map[string]any, desc bool) {
	sort.SliceStable(rows, func(i, j int) bool {
		left := usageCreatedAt(rows[i])
		right := usageCreatedAt(rows[j])
		if desc {
			return left > right
		}
		return left < right
	})
}

func usageCreatedAt(row map[string]any) string {
	if value, ok := row["created_at"].(string); ok {
		return value
	}
	return ""
}

func addNumericStats(target, source map[string]any) {
	for _, key := range []string{
		"total_requests", "total_input_tokens", "total_output_tokens", "total_cache_tokens",
		"total_cache_creation_tokens", "total_cache_read_tokens", "total_tokens",
		"total_cost", "total_actual_cost", "total_account_cost",
	} {
		target[key] = numericStat(target[key]) + numericStat(source[key])
	}
	if endpoints, ok := source["endpoints"].([]any); ok && len(endpoints) > 0 {
		target["endpoints"] = append(asAnySlice(target["endpoints"]), endpoints...)
	}
	if endpoints, ok := source["upstream_endpoints"].([]any); ok && len(endpoints) > 0 {
		target["upstream_endpoints"] = append(asAnySlice(target["upstream_endpoints"]), endpoints...)
	}
	if endpoints, ok := source["endpoint_paths"].([]any); ok && len(endpoints) > 0 {
		target["endpoint_paths"] = append(asAnySlice(target["endpoint_paths"]), endpoints...)
	}
}

func addTodayNumericStats(target, source map[string]any) {
	for _, fields := range [][2]string{
		{"today_requests", "total_requests"},
		{"today_input_tokens", "total_input_tokens"},
		{"today_output_tokens", "total_output_tokens"},
		{"today_cache_creation_tokens", "total_cache_creation_tokens"},
		{"today_cache_read_tokens", "total_cache_read_tokens"},
		{"today_tokens", "total_tokens"},
		{"today_cost", "total_cost"},
		{"today_actual_cost", "total_actual_cost"},
		{"today_account_cost", "total_account_cost"},
	} {
		target[fields[0]] = numericStat(target[fields[0]]) + numericStat(source[fields[1]])
	}
}

func numericStat(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}

func asAnySlice(value any) []any {
	if result, ok := value.([]any); ok {
		return result
	}
	return []any{}
}

func parseBoolQuery(raw string, fallback bool) bool {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return parsed
}

func emptyDashboardStats(memberCount int) map[string]any {
	stats := emptyUsageStats()
	for _, key := range []string{
		"total_users", "today_new_users", "active_users", "hourly_active_users",
		"total_api_keys", "active_api_keys", "total_accounts", "normal_accounts",
		"error_accounts", "ratelimit_accounts", "overload_accounts", "today_requests",
		"today_input_tokens", "today_output_tokens", "today_cache_creation_tokens",
		"today_cache_read_tokens", "today_tokens", "today_cost", "today_actual_cost",
		"today_account_cost", "uptime", "rpm", "tpm",
	} {
		stats[key] = 0
	}
	stats["total_users"] = memberCount
	stats["stats_updated_at"] = ""
	stats["stats_stale"] = false
	return stats
}

func stringStat(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func cloneMap(source map[string]any) map[string]any {
	result := map[string]any{}
	for key, value := range source {
		result[key] = value
	}
	return result
}

func mapValues(source map[string]map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(source))
	for _, item := range source {
		result = append(result, item)
	}
	return result
}

func mapValuesInt(source map[int64]map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(source))
	for _, item := range source {
		result = append(result, item)
	}
	return result
}

func allowedCoreUserAPI(r *http.Request) bool {
	switch r.URL.Path {
	case "/api/v1/keys":
		return r.Method == http.MethodGet || r.Method == http.MethodPost
	case "/api/v1/groups/available", "/api/v1/groups/rates", "/api/v1/channels/available",
		"/api/v1/channel-monitors":
		return r.Method == http.MethodGet
	case "/api/v1/user/profile", "/api/v1/user/platform-quotas", "/api/v1/user/aff",
		"/api/v1/subscriptions", "/api/v1/subscriptions/active", "/api/v1/subscriptions/progress",
		"/api/v1/subscriptions/summary",
		"/api/v1/user/totp/status", "/api/v1/user/totp/verification-method",
		"/api/v1/redeem/history", "/api/v1/announcements":
		return r.Method == http.MethodGet
	case "/api/v1/user/password", "/api/v1/user/notify-email/send-code",
		"/api/v1/user/notify-email/verify", "/api/v1/user/account-bindings/email/send-code",
		"/api/v1/user/account-bindings/email", "/api/v1/user/aff/transfer",
		"/api/v1/user/totp/send-code", "/api/v1/user/totp/setup", "/api/v1/user/totp/enable",
		"/api/v1/user/totp/disable", "/api/v1/user/totp/step-up", "/api/v1/redeem":
		return r.Method == http.MethodPut || r.Method == http.MethodPost
	case "/api/v1/user/notify-email":
		return r.Method == http.MethodDelete
	case "/api/v1/user/notify-email/toggle":
		return r.Method == http.MethodPut
	case "/api/v1/payment/config", "/api/v1/payment/checkout-info", "/api/v1/payment/limits",
		"/api/v1/payment/orders/my":
		return r.Method == http.MethodGet
	case "/api/v1/payment/orders", "/api/v1/payment/orders/verify":
		return r.Method == http.MethodPost
	case "/api/v1/integrations/imgtool/sso-url", "/api/v1/integrations/imgtool/credentials":
		return r.Method == http.MethodPost
	case "/api/v1/usage", "/api/v1/usage/stats", "/api/v1/usage/dashboard/stats",
		"/api/v1/usage/dashboard/trend", "/api/v1/usage/dashboard/models",
		"/api/v1/usage/dashboard/snapshot-v2", "/api/v1/usage/errors":
		return r.Method == http.MethodGet
	case "/api/v1/usage/dashboard/api-keys-usage":
		return r.Method == http.MethodPost
	case "/api/v1/admin/groups/all", "/api/v1/admin/announcements", "/api/v1/admin/usage",
		"/api/v1/admin/usage/stats", "/api/v1/admin/usage/search-users",
		"/api/v1/admin/usage/search-api-keys", "/api/v1/admin/usage/cleanup-tasks",
		"/api/v1/admin/ops/request-errors",
		"/api/v1/admin/users", "/api/v1/admin/user-attributes":
		return r.Method == http.MethodGet || (r.URL.Path == "/api/v1/admin/announcements" && r.Method == http.MethodPost)
	case "/api/v1/admin/user-attributes/batch":
		return r.Method == http.MethodPost
	case "/api/v1/admin/users/batch-limits":
		return r.Method == http.MethodPost
	case "/api/v1/admin/dashboard/snapshot-v2", "/api/v1/admin/dashboard/stats",
		"/api/v1/admin/dashboard/realtime", "/api/v1/admin/dashboard/trend",
		"/api/v1/admin/dashboard/models", "/api/v1/admin/dashboard/groups",
		"/api/v1/admin/dashboard/api-keys-trend", "/api/v1/admin/dashboard/users-trend",
		"/api/v1/admin/dashboard/users-ranking", "/api/v1/admin/dashboard/user-breakdown":
		return r.Method == http.MethodGet
	case "/api/v1/admin/dashboard/users-usage", "/api/v1/admin/dashboard/api-keys-usage":
		return r.Method == http.MethodPost
	default:
		if strings.HasPrefix(r.URL.Path, "/api/v1/subscriptions/") && strings.HasSuffix(r.URL.Path, "/progress") {
			rest := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/subscriptions/"), "/progress")
			if rest == "" || strings.Contains(rest, "/") {
				return false
			}
			_, err := strconv.ParseInt(rest, 10, 64)
			return err == nil && r.Method == http.MethodGet
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/channel-monitors/") && strings.HasSuffix(r.URL.Path, "/status") {
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/channel-monitors/"), "/status")
			if id == "" || strings.Contains(id, "/") {
				return false
			}
			_, err := strconv.ParseInt(id, 10, 64)
			return err == nil && r.Method == http.MethodGet
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/payment/orders/") {
			rest := strings.TrimPrefix(r.URL.Path, "/api/v1/payment/orders/")
			method := http.MethodGet
			if strings.HasSuffix(rest, "/cancel") {
				rest = strings.TrimSuffix(rest, "/cancel")
				method = http.MethodPost
			}
			if rest == "" || strings.Contains(rest, "/") {
				return false
			}
			_, err := strconv.ParseInt(rest, 10, 64)
			return err == nil && r.Method == method
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/keys/") {
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/keys/")
			if id == "" || strings.Contains(id, "/") {
				return false
			}
			if _, err := strconv.ParseInt(id, 10, 64); err != nil {
				return false
			}
			return r.Method == http.MethodGet || r.Method == http.MethodPut || r.Method == http.MethodDelete
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/usage/errors/") {
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/usage/errors/")
			if id == "" || strings.Contains(id, "/") {
				return false
			}
			_, err := strconv.ParseInt(id, 10, 64)
			return err == nil && r.Method == http.MethodGet
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/usage/") {
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/usage/")
			if id == "" || strings.Contains(id, "/") {
				return false
			}
			_, err := strconv.ParseInt(id, 10, 64)
			return err == nil && r.Method == http.MethodGet
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/admin/ops/request-errors/") {
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/ops/request-errors/")
			if id == "" || strings.Contains(id, "/") {
				return false
			}
			_, err := strconv.ParseInt(id, 10, 64)
			return err == nil && r.Method == http.MethodGet
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/user/api-keys/") && strings.HasSuffix(r.URL.Path, "/usage/daily") {
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/user/api-keys/"), "/usage/daily")
			if id == "" || strings.Contains(id, "/") {
				return false
			}
			_, err := strconv.ParseInt(id, 10, 64)
			return err == nil && r.Method == http.MethodGet
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/announcements/") && strings.HasSuffix(r.URL.Path, "/read") {
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/announcements/"), "/read")
			if id == "" || strings.Contains(id, "/") {
				return false
			}
			_, err := strconv.ParseInt(id, 10, 64)
			return err == nil && r.Method == http.MethodPost
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/admin/announcements/") {
			rest := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/announcements/")
			if rest == "" {
				return false
			}
			if strings.HasSuffix(rest, "/read-status") {
				id := strings.TrimSuffix(rest, "/read-status")
				if id == "" || strings.Contains(id, "/") {
					return false
				}
				_, err := strconv.ParseInt(id, 10, 64)
				return err == nil && r.Method == http.MethodGet
			}
			if strings.Contains(rest, "/") {
				return false
			}
			_, err := strconv.ParseInt(rest, 10, 64)
			return err == nil && (r.Method == http.MethodGet || r.Method == http.MethodPut || r.Method == http.MethodDelete)
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/admin/users/") {
			rest := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/users/")
			if rest == "" {
				return false
			}
			suffixMethods := []struct {
				suffix  string
				methods []string
			}{
				{suffix: "/platform-quotas/reset", methods: []string{http.MethodPost}},
				{suffix: "/balance-history", methods: []string{http.MethodGet}},
				{suffix: "/platform-quotas", methods: []string{http.MethodGet, http.MethodPut}},
				{suffix: "/replace-group", methods: []string{http.MethodPost}},
				{suffix: "/attributes", methods: []string{http.MethodGet, http.MethodPut}},
				{suffix: "/api-keys", methods: []string{http.MethodGet}},
				{suffix: "/rpm-status", methods: []string{http.MethodGet}},
				{suffix: "/balance", methods: []string{http.MethodPost}},
				{suffix: "/usage", methods: []string{http.MethodGet}},
			}
			for _, route := range suffixMethods {
				if strings.HasSuffix(rest, route.suffix) {
					id := strings.TrimSuffix(rest, route.suffix)
					if id == "" || strings.Contains(id, "/") {
						return false
					}
					_, err := strconv.ParseInt(id, 10, 64)
					if err != nil {
						return false
					}
					for _, method := range route.methods {
						if r.Method == method {
							return true
						}
					}
					return false
				}
			}
			if strings.Contains(rest, "/") {
				return false
			}
			_, err := strconv.ParseInt(rest, 10, 64)
			return err == nil && (r.Method == http.MethodGet || r.Method == http.MethodPut)
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/admin/api-keys/") {
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/api-keys/")
			if id == "" || strings.Contains(id, "/") {
				return false
			}
			_, err := strconv.ParseInt(id, 10, 64)
			return err == nil && r.Method == http.MethodPut
		}
		return false
	}
}

type loginRequest struct {
	Identifier     string `json:"identifier"`
	Password       string `json:"password"`
	TurnstileToken string `json:"turnstile_token"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if err := decodeJSON(r, &request); err != nil || strings.TrimSpace(request.Identifier) == "" || request.Password == "" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid login request")
		return
	}
	result, err := s.auth.Login(r.Context(), portalauth.LoginInput{
		Identifier: request.Identifier, Password: request.Password, TurnstileToken: request.TurnstileToken,
	})
	if err != nil {
		if errors.Is(err, username.ErrNotFound) {
			writeAPIError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid account or password")
			return
		}
		s.logger.Warn("Portal login failed", "error", err)
		writeAPIError(w, http.StatusBadGateway, "CORE_UNAVAILABLE", "Authentication service is unavailable")
		return
	}
	if result.Response.StatusCode == http.StatusUnauthorized {
		writeAPIError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid account or password")
		return
	}
	writeCoreResponse(w, result.Response)
}

func (s *Server) directCoreLogin(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if err := decodeJSON(r, &request); err != nil || strings.TrimSpace(request.Identifier) == "" || request.Password == "" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid login request")
		return
	}
	if upstream := strings.TrimSpace(os.Getenv("PORTAL_AUTH_UPSTREAM_URL")); upstream != "" {
		s.proxyPortalLogin(w, r, upstream, request)
		return
	}
	identifier := strings.TrimSpace(request.Identifier)
	email := identifier
	if !strings.Contains(identifier, "@") {
		page, err := s.core.AdminUsers(r.Context(), 1, 20, coreclient.AdminUsersSearch(identifier))
		if err != nil {
			s.logger.Warn("Direct Core username lookup failed", "error", err)
			writeAPIError(w, http.StatusBadGateway, "CORE_UNAVAILABLE", "Authentication service is unavailable")
			return
		}
		email = ""
		for _, user := range page.Items {
			if strings.EqualFold(strings.TrimSpace(user.Username), identifier) {
				email = strings.TrimSpace(user.Email)
				break
			}
		}
		if email == "" {
			writeAPIError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid account or password")
			return
		}
	}
	result, err := s.core.Login(r.Context(), coreclient.LoginRequest{
		Email: email, Password: request.Password, TurnstileToken: request.TurnstileToken,
	})
	if err != nil {
		s.logger.Warn("Direct Core login failed", "error", err)
		writeAPIError(w, http.StatusBadGateway, "CORE_UNAVAILABLE", "Authentication service is unavailable")
		return
	}
	if result.Response.StatusCode == http.StatusUnauthorized {
		writeAPIError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid account or password")
		return
	}
	writeCoreResponse(w, result.Response)
}

func (s *Server) proxyPortalLogin(w http.ResponseWriter, r *http.Request, upstream string, request loginRequest) {
	baseURL, err := url.Parse(upstream)
	if err != nil || (baseURL.Scheme != "https" && baseURL.Scheme != "http") || baseURL.Host == "" {
		writeAPIError(w, http.StatusServiceUnavailable, "CORE_UNAVAILABLE", "Authentication service is unavailable")
		return
	}
	baseURL.Path = strings.TrimRight(baseURL.Path, "/") + "/api/v1/auth/login"
	baseURL.RawQuery = ""
	baseURL.Fragment = ""
	body, err := json.Marshal(request)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid login request")
		return
	}
	upstreamRequest, err := http.NewRequestWithContext(r.Context(), http.MethodPost, baseURL.String(), bytes.NewReader(body))
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "CORE_UNAVAILABLE", "Authentication service is unavailable")
		return
	}
	upstreamRequest.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 15 * time.Second}).Do(upstreamRequest)
	if err != nil {
		s.logger.Warn("Portal auth upstream login failed", "error", err)
		writeAPIError(w, http.StatusBadGateway, "CORE_UNAVAILABLE", "Authentication service is unavailable")
		return
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		writeAPIError(w, http.StatusBadGateway, "CORE_UNAVAILABLE", "Authentication service is unavailable")
		return
	}
	writeCoreResponse(w, coreclient.Response{
		StatusCode: response.StatusCode, ContentType: response.Header.Get("Content-Type"), Body: responseBody,
	})
}

type registerRequest struct {
	Username       string `json:"username"`
	Email          string `json:"email"`
	Password       string `json:"password"`
	VerifyCode     string `json:"verify_code"`
	TurnstileToken string `json:"turnstile_token"`
	PromoCode      string `json:"promo_code"`
	InvitationCode string `json:"invitation_code"`
	AffCode        string `json:"aff_code"`
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var request registerRequest
	if err := decodeJSON(r, &request); err != nil || request.Username == "" || request.Email == "" || request.Password == "" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid registration request")
		return
	}
	result, err := s.auth.Register(r.Context(), portalauth.RegisterInput{
		Username: request.Username, Email: request.Email, Password: request.Password,
		VerifyCode: request.VerifyCode, TurnstileToken: request.TurnstileToken,
		PromoCode: request.PromoCode, InvitationCode: request.InvitationCode, AffCode: request.AffCode,
	})
	if err != nil {
		if errors.Is(err, username.ErrTaken) {
			writeAPIError(w, http.StatusConflict, "USERNAME_TAKEN", "Username is already taken")
			return
		}
		if errors.Is(err, username.ErrInvalid) {
			writeAPIError(w, http.StatusBadRequest, "INVALID_USERNAME", "Invalid username")
			return
		}
		s.logger.Warn("Portal registration failed", "error", err)
		writeAPIError(w, http.StatusBadGateway, "REGISTRATION_INCOMPLETE", "Registration could not be completed")
		return
	}
	writeCoreResponse(w, result.Response)
}

func (s *Server) usernameAvailable(w http.ResponseWriter, r *http.Request) {
	available, err := s.auth.UsernameAvailable(r.Context(), r.URL.Query().Get("username"))
	if err != nil {
		if errors.Is(err, username.ErrInvalid) {
			writeAPIError(w, http.StatusBadRequest, "INVALID_USERNAME", "Invalid username")
			return
		}
		s.logger.Warn("Username availability check failed", "error", err)
		writeAPIError(w, http.StatusServiceUnavailable, "USERNAME_CHECK_UNAVAILABLE", "Username check is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": map[string]bool{"available": available}})
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeCoreResponse(w http.ResponseWriter, response coreclient.Response) {
	contentType := response.ContentType
	if !strings.HasPrefix(strings.ToLower(contentType), "application/json") {
		contentType = "application/json; charset=utf-8"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(response.Body)
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"code": code, "message": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
