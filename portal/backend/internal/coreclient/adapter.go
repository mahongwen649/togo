package coreclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var ErrUnauthorized = fmt.Errorf("Core session is unauthorized")
var ErrNotFound = fmt.Errorf("Core resource was not found")

// Core admin usage responses can legitimately exceed 2 MiB for established
// production users (for example a 1,000-row usage page is currently about
// 3.5 MiB). Keep a bounded response while allowing those management views to
// render instead of reporting the Core service as unavailable.
const maxResponseBytes = 16 << 20

type Response struct {
	StatusCode  int
	ContentType string
	Body        []byte
}

type Adapter interface {
	PublicSettings(context.Context) (Response, error)
	Login(context.Context, LoginRequest) (AuthResult, error)
	Register(context.Context, RegisterRequest) (AuthResult, error)
	UpdateUsername(context.Context, string, string) error
	AdminUser(context.Context, int64) (User, error)
	Profile(context.Context, string) (User, error)
	ProfileResponse(context.Context, string) (Response, error)
	UserAPIRequest(context.Context, CoreRequest) (Response, error)
	AdminCreateUser(context.Context, AdminCreateUserRequest) (User, Response, error)
	AdminDeleteUser(context.Context, int64) error
	AdminUpdatePassword(context.Context, int64, string) error
	AdminUsers(context.Context, int, int, ...AdminUsersOption) (UserPage, error)
}

type HTTPAdapter struct {
	baseURL     *url.URL
	client      *http.Client
	adminAPIKey string
}

type User struct {
	ID                         int64              `json:"id"`
	Email                      string             `json:"email"`
	Username                   string             `json:"username"`
	Role                       string             `json:"role"`
	Status                     string             `json:"status"`
	Balance                    float64            `json:"balance"`
	BalanceNotifyEnabled       bool               `json:"balance_notify_enabled"`
	BalanceNotifyThresholdType string             `json:"balance_notify_threshold_type"`
	BalanceNotifyThreshold     *float64           `json:"balance_notify_threshold"`
	BalanceNotifyExtraEmails   []NotifyEmailEntry `json:"balance_notify_extra_emails"`
	TotalRecharged             float64            `json:"total_recharged"`
}

type NotifyEmailEntry struct {
	Email    string `json:"email"`
	Verified bool   `json:"verified"`
	Disabled bool   `json:"disabled"`
}

type BalanceAlertSettings struct {
	Enabled     bool
	Threshold   float64
	RechargeURL string
	SiteName    string
}

type UserPage struct {
	Items    []User `json:"items"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Pages    int    `json:"pages"`
	Total    int64  `json:"total"`
}

type APIKeyGroup struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	RateMultiplier float64 `json:"rate_multiplier"`
}

type APIKey struct {
	ID        int64        `json:"id"`
	Key       string       `json:"key"`
	Name      string       `json:"name"`
	GroupID   *int64       `json:"group_id"`
	Group     *APIKeyGroup `json:"group"`
	Status    string       `json:"status"`
	Quota     float64      `json:"quota"`
	QuotaUsed float64      `json:"quota_used"`
	ExpiresAt *time.Time   `json:"expires_at"`
	CreatedAt time.Time    `json:"created_at"`
}

type APIKeyPage struct {
	Items []APIKey `json:"items"`
	Total int64    `json:"total"`
}

type AdminUsersOption func(url.Values)

func AdminUsersSearch(value string) AdminUsersOption {
	return func(query url.Values) {
		if strings.TrimSpace(value) != "" {
			query.Set("search", strings.TrimSpace(value))
		}
	}
}

type LoginRequest struct {
	Email          string `json:"email"`
	Password       string `json:"password"`
	TurnstileToken string `json:"turnstile_token,omitempty"`
}

type RegisterRequest struct {
	Email          string `json:"email"`
	Password       string `json:"password"`
	VerifyCode     string `json:"verify_code,omitempty"`
	TurnstileToken string `json:"turnstile_token,omitempty"`
	PromoCode      string `json:"promo_code,omitempty"`
	InvitationCode string `json:"invitation_code,omitempty"`
	AffCode        string `json:"aff_code,omitempty"`
}

type AdminCreateUserRequest struct {
	Email       string   `json:"email"`
	Password    string   `json:"password"`
	Username    string   `json:"username"`
	Notes       string   `json:"notes,omitempty"`
	Role        string   `json:"role"`
	Balance     *float64 `json:"balance,omitempty"`
	Concurrency int      `json:"concurrency,omitempty"`
	RPMLimit    int      `json:"rpm_limit,omitempty"`
}

type AuthResult struct {
	Response    Response
	User        User
	AccessToken string
}

type CoreRequest struct {
	Method      string
	Path        string
	RawQuery    string
	AccessToken string
	ContentType string
	Body        []byte
	Host        string
	Referer     string
}

type envelope[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type authData struct {
	AccessToken string `json:"access_token"`
	User        User   `json:"user"`
}

func NewHTTPAdapter(rawBaseURL string, client *http.Client) (*HTTPAdapter, error) {
	baseURL, err := url.Parse(strings.TrimRight(rawBaseURL, "/"))
	if err != nil || baseURL.Scheme == "" || baseURL.Host == "" {
		return nil, fmt.Errorf("invalid Core base URL")
	}
	if client == nil {
		return nil, fmt.Errorf("Core HTTP client is required")
	}
	return &HTTPAdapter{baseURL: baseURL, client: client}, nil
}

func (a *HTTPAdapter) WithAdminAPIKey(key string) *HTTPAdapter {
	clone := *a
	clone.adminAPIKey = strings.TrimSpace(key)
	return &clone
}

func (a *HTTPAdapter) PublicSettings(ctx context.Context) (Response, error) {
	return a.get(ctx, "/api/v1/settings/public")
}

func (a *HTTPAdapter) BalanceAlertSettings(ctx context.Context) (BalanceAlertSettings, error) {
	response, err := a.PublicSettings(ctx)
	if err != nil {
		return BalanceAlertSettings{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return BalanceAlertSettings{}, fmt.Errorf("Core public settings returned status %d", response.StatusCode)
	}
	var decoded struct {
		Data struct {
			Enabled     bool    `json:"balance_low_notify_enabled"`
			Threshold   float64 `json:"balance_low_notify_threshold"`
			RechargeURL string  `json:"balance_low_notify_recharge_url"`
			SiteName    string  `json:"site_name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body, &decoded); err != nil {
		return BalanceAlertSettings{}, fmt.Errorf("decode Core balance alert settings: %w", err)
	}
	return BalanceAlertSettings{
		Enabled:     decoded.Data.Enabled,
		Threshold:   decoded.Data.Threshold,
		RechargeURL: decoded.Data.RechargeURL,
		SiteName:    decoded.Data.SiteName,
	}, nil
}

func (a *HTTPAdapter) Login(ctx context.Context, input LoginRequest) (AuthResult, error) {
	return a.auth(ctx, "/api/v1/auth/login", input)
}

func (a *HTTPAdapter) Register(ctx context.Context, input RegisterRequest) (AuthResult, error) {
	return a.auth(ctx, "/api/v1/auth/register", input)
}

func (a *HTTPAdapter) auth(ctx context.Context, path string, input any) (AuthResult, error) {
	response, err := a.jsonRequest(ctx, http.MethodPost, path, "", input)
	if err != nil {
		return AuthResult{}, err
	}
	result := AuthResult{Response: response}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return result, nil
	}
	var decoded envelope[authData]
	if err := json.Unmarshal(response.Body, &decoded); err != nil {
		return AuthResult{}, fmt.Errorf("decode Core auth response: %w", err)
	}
	result.User = decoded.Data.User
	result.AccessToken = decoded.Data.AccessToken
	if result.User.ID <= 0 || result.AccessToken == "" {
		return AuthResult{}, fmt.Errorf("Core auth response is incomplete")
	}
	return result, nil
}

func (a *HTTPAdapter) UpdateUsername(ctx context.Context, accessToken, value string) error {
	response, err := a.jsonRequest(ctx, http.MethodPut, "/api/v1/user", accessToken, map[string]string{"username": value})
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Core username update returned status %d", response.StatusCode)
	}
	return nil
}

func (a *HTTPAdapter) AdminUser(ctx context.Context, userID int64) (User, error) {
	if a.adminAPIKey == "" {
		return User{}, fmt.Errorf("Core admin API key is not configured")
	}
	response, err := a.jsonRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v1/admin/users/%d", userID), "", nil)
	if err != nil {
		return User{}, err
	}
	if response.StatusCode == http.StatusNotFound {
		return User{}, ErrNotFound
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return User{}, fmt.Errorf("Core admin user lookup returned status %d", response.StatusCode)
	}
	var decoded envelope[User]
	if err := json.Unmarshal(response.Body, &decoded); err != nil {
		return User{}, fmt.Errorf("decode Core admin user: %w", err)
	}
	return decoded.Data, nil
}

func (a *HTTPAdapter) Profile(ctx context.Context, accessToken string) (User, error) {
	response, err := a.ProfileResponse(ctx, accessToken)
	if err != nil {
		return User{}, err
	}
	if response.StatusCode == http.StatusUnauthorized {
		return User{}, ErrUnauthorized
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return User{}, fmt.Errorf("Core profile returned status %d", response.StatusCode)
	}
	var decoded envelope[User]
	if err := json.Unmarshal(response.Body, &decoded); err != nil {
		return User{}, fmt.Errorf("decode Core profile: %w", err)
	}
	return decoded.Data, nil
}

func (a *HTTPAdapter) ProfileResponse(ctx context.Context, accessToken string) (Response, error) {
	if strings.TrimSpace(accessToken) == "" {
		return Response{}, fmt.Errorf("Core access token is required")
	}
	return a.jsonRequest(ctx, http.MethodGet, "/api/v1/user/profile", accessToken, nil)
}

func (a *HTTPAdapter) UserAPIRequest(ctx context.Context, input CoreRequest) (Response, error) {
	if strings.TrimSpace(input.AccessToken) == "" {
		return Response{}, fmt.Errorf("Core access token is required")
	}
	return a.rawRequestWithSource(ctx, input.Method, input.Path, input.RawQuery, input.AccessToken, input.ContentType, input.Body, input.Host, input.Referer)
}

// SendVerifyCodeRequest forwards only the public registration email endpoint.
// Keeping this separate from UserAPIRequest avoids a general anonymous proxy.
func (a *HTTPAdapter) SendVerifyCodeRequest(ctx context.Context, input CoreRequest) (Response, error) {
	if input.Method != http.MethodPost || input.Path != "/api/v1/auth/send-verify-code" {
		return Response{}, fmt.Errorf("unsupported anonymous auth request")
	}
	return a.rawRequest(ctx, input.Method, input.Path, "", "", input.ContentType, input.Body)
}

// PasswordResetRequest forwards only the two public password-reset endpoints.
// Keeping this separate from UserAPIRequest avoids a general anonymous proxy.
func (a *HTTPAdapter) PasswordResetRequest(ctx context.Context, input CoreRequest) (Response, error) {
	if input.Method != http.MethodPost || (input.Path != "/api/v1/auth/forgot-password" && input.Path != "/api/v1/auth/reset-password") {
		return Response{}, fmt.Errorf("unsupported anonymous password reset request")
	}
	return a.rawRequest(ctx, input.Method, input.Path, "", "", input.ContentType, input.Body)
}

// PaymentWebhookRequest forwards only the public EasyPay callback. Keeping this
// separate from UserAPIRequest avoids exposing a general unauthenticated proxy.
func (a *HTTPAdapter) PaymentWebhookRequest(ctx context.Context, input CoreRequest) (Response, error) {
	if input.Path != "/api/v1/payment/webhook/easypay" || (input.Method != http.MethodGet && input.Method != http.MethodPost) {
		return Response{}, fmt.Errorf("unsupported payment webhook request")
	}
	return a.rawRequest(ctx, input.Method, input.Path, input.RawQuery, "", input.ContentType, input.Body)
}

func (a *HTTPAdapter) AdminUsers(ctx context.Context, page, pageSize int, options ...AdminUsersOption) (UserPage, error) {
	if a.adminAPIKey == "" {
		return UserPage{}, fmt.Errorf("Core admin API key is not configured")
	}
	if page < 1 || pageSize < 1 {
		return UserPage{}, fmt.Errorf("invalid Core user page request")
	}
	query := url.Values{}
	query.Set("page", fmt.Sprint(page))
	query.Set("page_size", fmt.Sprint(pageSize))
	query.Set("include_subscriptions", "false")
	for _, option := range options {
		if option != nil {
			option(query)
		}
	}
	response, err := a.jsonRequest(ctx, http.MethodGet, "/api/v1/admin/users?"+query.Encode(), "", nil)
	if err != nil {
		return UserPage{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return UserPage{}, fmt.Errorf("Core admin user list returned status %d", response.StatusCode)
	}
	var decoded envelope[UserPage]
	if err := json.Unmarshal(response.Body, &decoded); err != nil {
		return UserPage{}, fmt.Errorf("decode Core admin users: %w", err)
	}
	return decoded.Data, nil
}

func (a *HTTPAdapter) AdminUserAPIKeys(ctx context.Context, userID int64) (APIKeyPage, error) {
	if a.adminAPIKey == "" {
		return APIKeyPage{}, fmt.Errorf("Core admin API key is not configured")
	}
	if userID <= 0 {
		return APIKeyPage{}, fmt.Errorf("invalid Core user ID")
	}
	path := fmt.Sprintf("/api/v1/admin/users/%d/api-keys?page=1&page_size=100&sort_by=created_at&sort_order=desc", userID)
	response, err := a.jsonRequest(ctx, http.MethodGet, path, "", nil)
	if err != nil {
		return APIKeyPage{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return APIKeyPage{}, fmt.Errorf("Core admin API key list returned status %d", response.StatusCode)
	}
	var decoded envelope[APIKeyPage]
	if err := json.Unmarshal(response.Body, &decoded); err != nil {
		return APIKeyPage{}, fmt.Errorf("decode Core admin API keys: %w", err)
	}
	return decoded.Data, nil
}

func (a *HTTPAdapter) AdminCreateUser(ctx context.Context, input AdminCreateUserRequest) (User, Response, error) {
	if a.adminAPIKey == "" {
		return User{}, Response{}, fmt.Errorf("Core admin API key is not configured")
	}
	response, err := a.jsonRequest(ctx, http.MethodPost, "/api/v1/admin/users", "", input)
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return User{}, response, err
	}
	var decoded envelope[User]
	if err := json.Unmarshal(response.Body, &decoded); err != nil {
		return User{}, response, fmt.Errorf("decode Core created user: %w", err)
	}
	return decoded.Data, response, nil
}

func (a *HTTPAdapter) AdminDeleteUser(ctx context.Context, userID int64) error {
	if a.adminAPIKey == "" {
		return fmt.Errorf("Core admin API key is not configured")
	}
	response, err := a.jsonRequest(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/admin/users/%d", userID), "", nil)
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Core admin user delete returned status %d", response.StatusCode)
	}
	return nil
}

func (a *HTTPAdapter) AdminUpdatePassword(ctx context.Context, userID int64, password string) error {
	if a.adminAPIKey == "" {
		return fmt.Errorf("Core admin API key is not configured")
	}
	response, err := a.jsonRequest(ctx, http.MethodPut, fmt.Sprintf("/api/v1/admin/users/%d", userID), "", map[string]string{"password": password})
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Core admin password update returned status %d", response.StatusCode)
	}
	return nil
}

func (a *HTTPAdapter) AdminAddBalance(ctx context.Context, userID int64, amount float64, notes, idempotencyKey string) error {
	if a.adminAPIKey == "" {
		return fmt.Errorf("Core admin API key is not configured")
	}
	body, err := json.Marshal(map[string]any{
		"balance": amount, "operation": "add", "notes": notes,
	})
	if err != nil {
		return fmt.Errorf("encode Core balance update: %w", err)
	}
	response, err := a.rawRequestWithHeaders(
		ctx, http.MethodPost, fmt.Sprintf("/api/v1/admin/users/%d/balance", userID), "", "", "application/json", body,
		map[string]string{"Idempotency-Key": idempotencyKey},
	)
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Core admin balance update returned status %d: %s", response.StatusCode, strings.TrimSpace(string(response.Body)))
	}
	return nil
}

func (a *HTTPAdapter) get(ctx context.Context, path string) (Response, error) {
	return a.jsonRequest(ctx, http.MethodGet, path, "", nil)
}

func (a *HTTPAdapter) jsonRequest(ctx context.Context, method, path, accessToken string, payload any) (Response, error) {
	var body []byte
	contentType := ""
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return Response{}, fmt.Errorf("encode Core request: %w", err)
		}
		body = encoded
		contentType = "application/json"
	}
	return a.rawRequest(ctx, method, path, "", accessToken, contentType, body)
}

func (a *HTTPAdapter) rawRequest(ctx context.Context, method, path, rawQuery, accessToken, contentType string, body []byte) (Response, error) {
	return a.rawRequestWithSource(ctx, method, path, rawQuery, accessToken, contentType, body, "", "")
}

func (a *HTTPAdapter) rawRequestWithSource(ctx context.Context, method, path, rawQuery, accessToken, contentType string, body []byte, sourceHost, referer string) (Response, error) {
	return a.rawRequestWithSourceAndHeaders(ctx, method, path, rawQuery, accessToken, contentType, body, sourceHost, referer, nil)
}

func (a *HTTPAdapter) rawRequestWithHeaders(ctx context.Context, method, path, rawQuery, accessToken, contentType string, body []byte, headers map[string]string) (Response, error) {
	return a.rawRequestWithSourceAndHeaders(ctx, method, path, rawQuery, accessToken, contentType, body, "", "", headers)
}

func (a *HTTPAdapter) rawRequestWithSourceAndHeaders(ctx context.Context, method, path, rawQuery, accessToken, contentType string, body []byte, sourceHost, referer string, headers map[string]string) (Response, error) {
	requestURL, err := url.Parse(path)
	if err != nil {
		return Response{}, fmt.Errorf("build Core URL: %w", err)
	}
	if rawQuery != "" {
		requestURL.RawQuery = rawQuery
	}
	target := a.baseURL.ResolveReference(requestURL)
	var requestBody io.Reader
	if len(body) > 0 {
		requestBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target.String(), requestBody)
	if err != nil {
		return Response{}, fmt.Errorf("build Core request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	// Internal HTTP Core relies on the Portal host for payment return URL
	// validation. Public HTTPS Core endpoints must keep their own Host so TLS
	// gateways and virtual hosts route the request correctly.
	if strings.TrimSpace(sourceHost) != "" && !strings.EqualFold(a.baseURL.Scheme, "https") {
		req.Host = strings.TrimSpace(sourceHost)
	}
	if strings.TrimSpace(referer) != "" {
		req.Header.Set("Referer", strings.TrimSpace(referer))
	}
	if strings.TrimSpace(contentType) != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	if strings.HasPrefix(path, "/api/v1/admin/") {
		req.Header.Set("x-api-key", a.adminAPIKey)
	}
	for key, value := range headers {
		if strings.TrimSpace(value) != "" {
			req.Header.Set(key, value)
		}
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("call Core: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return Response{}, fmt.Errorf("read Core response: %w", err)
	}
	if len(responseBody) > maxResponseBytes {
		return Response{}, fmt.Errorf("Core response exceeds limit")
	}

	return Response{
		StatusCode:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		Body:        responseBody,
	}, nil
}
