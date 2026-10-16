package coreclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const adminMarketRawTimeout = 5 * time.Minute

// AdminMarketRequest is an internal adapter, not a publicly exposed proxy.
// Callers construct fixed Core routes and never forward a client-supplied URL.
func (a *HTTPAdapter) AdminMarketRequest(ctx context.Context, method, path string, payload any, idempotencyKey string) (Response, error) {
	if a.adminAPIKey == "" {
		return Response{}, fmt.Errorf("Core admin API key is not configured")
	}
	var body []byte
	if payload != nil {
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			return Response{}, err
		}
	}
	response, err := a.rawRequestWithHeaders(ctx, method, path, "", "", "application/json", body, map[string]string{"Idempotency-Key": idempotencyKey})
	if err != nil {
		return Response{}, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return response, fmt.Errorf("Core account business request returned HTTP %d", response.StatusCode)
	}
	return response, nil
}

// AdminMarketRawRequest is used for Core admin endpoints that intentionally
// return a non-JSON stream, such as account connection tests. The caller is
// responsible for validating the response body format.
func (a *HTTPAdapter) AdminMarketRawRequest(ctx context.Context, method, path string, payload any, idempotencyKey string) (Response, error) {
	if a.adminAPIKey == "" {
		return Response{}, fmt.Errorf("Core admin API key is not configured")
	}
	var body []byte
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return Response{}, err
		}
		body = encoded
	}
	longClient := &http.Client{
		Transport:     a.client.Transport,
		CheckRedirect: a.client.CheckRedirect,
		Jar:           a.client.Jar,
		Timeout:       adminMarketRawTimeout,
	}
	clone := *a
	clone.client = longClient
	response, err := clone.rawRequestWithHeaders(ctx, method, path, "", "", "application/json", body, map[string]string{"Idempotency-Key": idempotencyKey})
	if err != nil {
		return Response{}, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return response, fmt.Errorf("Core account business request returned HTTP %d", response.StatusCode)
	}
	return response, nil
}

// AdminUserMarketRequest calls a Core admin route with the signed-in
// administrator's JWT. It is used by Portal admin tools that should follow the
// operator's own Core permissions instead of relying on a server API key.
func (a *HTTPAdapter) AdminUserMarketRequest(ctx context.Context, method, path string, payload any, accessToken string) (Response, error) {
	return a.adminUserMarketRequest(ctx, method, path, payload, accessToken, false)
}

// AdminUserMarketRawRequest is the long-running variant for Core account-test
// streams. The response is still bounded by the adapter's response-size limit.
func (a *HTTPAdapter) AdminUserMarketRawRequest(ctx context.Context, method, path string, payload any, accessToken string) (Response, error) {
	return a.adminUserMarketRequest(ctx, method, path, payload, accessToken, true)
}

func (a *HTTPAdapter) adminUserMarketRequest(ctx context.Context, method, path string, payload any, accessToken string, longRunning bool) (Response, error) {
	if strings.TrimSpace(accessToken) == "" {
		return Response{}, fmt.Errorf("Core administrator access token is required")
	}
	var body []byte
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return Response{}, err
		}
		body = encoded
	}
	clone := *a
	clone.adminAPIKey = ""
	if longRunning {
		longClient := &http.Client{Transport: a.client.Transport, CheckRedirect: a.client.CheckRedirect, Jar: a.client.Jar, Timeout: adminMarketRawTimeout}
		clone.client = longClient
	}
	response, err := clone.rawRequestWithHeaders(ctx, method, path, "", accessToken, "application/json", body, nil)
	if err != nil {
		return Response{}, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return response, fmt.Errorf("Core administrator request returned HTTP %d", response.StatusCode)
	}
	return response, nil
}
