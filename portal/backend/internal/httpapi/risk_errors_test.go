package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

func TestLegacyRiskCheckAllowsAllCredentials(t *testing.T) {
	for _, headers := range []http.Header{
		{},
		{"Authorization": {"Bearer sk-user"}},
		{"X-Api-Key": {"sk-user"}},
		{"X-Goog-Api-Key": {"sk-user"}},
	} {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			t.Run(fmt.Sprintf("%s/%v", method, headers), func(t *testing.T) {
				// The compatibility endpoint must work without a database or Core
				// lookup, including when Nginx inherits the original POST method.
				r := httptest.NewRequest(method, "/api/v1/internal/risk-check", nil)
				r.Header = headers.Clone()
				w := httptest.NewRecorder()
				New(coreStub{}, nil, testLogger()).ServeHTTP(w, r)
				if w.Code != http.StatusNoContent || w.Body.Len() != 0 {
					t.Fatalf("response = %d %s", w.Code, w.Body.String())
				}
			})
		}
	}
}

func TestCoreRiskErrorsDoNotBlockSubsequentRequests(t *testing.T) {
	for _, kind := range []string{"permission_error", "cyber_policy", "cyber_policy_session_blocked"} {
		t.Run(kind, func(t *testing.T) {
			errorBody := fmt.Sprintf(`{"error":{"type":%q,"message":"request denied"}}`, kind)
			successBody := `{"code":0,"data":{"id":8,"status":"active"}}`
			requests := 0
			core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Method != http.MethodGet || r.URL.Path != "/api/v1/user/profile" || r.Header.Get("Authorization") != "Bearer user-token" {
					t.Errorf("unexpected Core request: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if requests == 1 {
					w.WriteHeader(http.StatusForbidden)
					_, _ = w.Write([]byte(errorBody))
					return
				}
				_, _ = w.Write([]byte(successBody))
			}))
			defer core.Close()
			adapter, err := coreclient.NewHTTPAdapter(core.URL, core.Client())
			if err != nil {
				t.Fatal(err)
			}
			handler := New(adapter, nil, testLogger())
			for i, wantBody := range []string{errorBody, successBody} {
				r := httptest.NewRequest(http.MethodGet, "/api/v1/user/profile", nil)
				r.Header.Set("Authorization", "Bearer user-token")
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, r)
				wantStatus := http.StatusOK
				if i == 0 {
					wantStatus = http.StatusForbidden
				}
				if w.Code != wantStatus || w.Body.String() != wantBody || w.Header().Get("Content-Type") != "application/json" {
					t.Fatalf("request %d: response = %d %s", i+1, w.Code, w.Body.String())
				}
			}
			if requests != 2 {
				t.Fatalf("Core received %d requests, want exactly two forwarded requests", requests)
			}
		})
	}
}
