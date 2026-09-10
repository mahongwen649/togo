package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

type imgtoolKeysCoreStub struct {
	recordingCoreStub
	keys    coreclient.APIKeyPage
	keysErr error
	calls   []coreclient.CoreRequest
}

func (s *imgtoolKeysCoreStub) UserAPIRequest(_ context.Context, input coreclient.CoreRequest) (coreclient.Response, error) {
	s.calls = append(s.calls, input)
	s.request = input
	if s.userAPIResponse.StatusCode == 0 && len(s.userAPIResponse.Body) == 0 {
		return coreclient.Response{StatusCode: http.StatusOK, Body: []byte(`{"code":0,"data":{"items":[]}}`)}, nil
	}
	return s.userAPIResponse, s.err
}

func (s *imgtoolKeysCoreStub) AdminUserAPIKeys(_ context.Context, userID int64) (coreclient.APIKeyPage, error) {
	if s.keysErr != nil {
		return coreclient.APIKeyPage{}, s.keysErr
	}
	if userID != s.profile.ID && s.profile.ID != 0 {
		return coreclient.APIKeyPage{}, nil
	}
	return s.keys, nil
}

func TestImgtoolURLEnsuresMissingGroupKeys(t *testing.T) {
	t.Setenv("IMGTOOL_BASE_URL", "https://img.togoapi.com/")
	t.Setenv("IMGTOOL_SSO_SECRET", "0123456789abcdef0123456789abcdef")
	core := &imgtoolKeysCoreStub{}
	core.profile = coreclient.User{ID: 7, Username: "alice", Email: "alice@example.com", Role: "user", Status: "active"}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/integrations/imgtool/sso-url", nil)
	request.Header.Set("Authorization", "Bearer user-token")
	New(core, nil, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"redirect_url":"https://img.togoapi.com/sso?ticket=`) {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
	if len(core.calls) < 3 {
		t.Fatalf("expected list plus two creates, got %#v", core.calls)
	}
	if core.calls[0].Path != "/api/v1/keys" || core.calls[0].Method != http.MethodGet {
		t.Fatalf("first call = %+v", core.calls[0])
	}
	created := map[int64]bool{}
	for _, call := range core.calls[1:] {
		if call.Method != http.MethodPost || call.Path != "/api/v1/keys" {
			t.Fatalf("create call = %+v", call)
		}
		var payload struct {
			Name    string `json:"name"`
			GroupID int64  `json:"group_id"`
		}
		if err := json.Unmarshal(call.Body, &payload); err != nil {
			t.Fatalf("decode create body: %v", err)
		}
		created[payload.GroupID] = true
	}
	if !created[15] || !created[33] {
		t.Fatalf("created groups = %#v", created)
	}
}

func TestImgtoolCredentialsReturnsMatchingGroupKeys(t *testing.T) {
	t.Setenv("IMGTOOL_SSO_SECRET", "0123456789abcdef0123456789abcdef")
	openaiID := int64(15)
	grokID := int64(33)
	core := &imgtoolKeysCoreStub{
		keys: coreclient.APIKeyPage{Items: []coreclient.APIKey{
			{ID: 11, Key: "sk-openai", Name: "manual", GroupID: &openaiID, Status: "active", Group: &coreclient.APIKeyGroup{ID: 15, Name: "gpt-image-2"}},
			{ID: 22, Key: "sk-grok", Name: "生图-Grok", GroupID: &grokID, Status: "active", Group: &coreclient.APIKeyGroup{ID: 33, Name: "Grok Heavy"}},
		}},
	}
	core.profile = coreclient.User{ID: 7, Status: "active"}
	body := []byte(`{"user_id":7}`)
	timestamp := time.Now().Unix()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/integrations/imgtool/credentials", strings.NewReader(string(body)))
	request.Header.Set("X-Imgtool-Timestamp", strconv.FormatInt(timestamp, 10))
	request.Header.Set("X-Imgtool-Signature", signImgtoolServiceRequest("0123456789abcdef0123456789abcdef", timestamp, body))
	recorder := httptest.NewRecorder()
	New(core, nil, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"sk-openai"`) || !strings.Contains(recorder.Body.String(), `"sk-grok"`) {
		t.Fatalf("missing keys: %s", recorder.Body.String())
	}
}

func TestImgtoolCredentialsRejectsBadSignature(t *testing.T) {
	t.Setenv("IMGTOOL_SSO_SECRET", "0123456789abcdef0123456789abcdef")
	request := httptest.NewRequest(http.MethodPost, "/api/v1/integrations/imgtool/credentials", strings.NewReader(`{"user_id":7}`))
	request.Header.Set("X-Imgtool-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	request.Header.Set("X-Imgtool-Signature", "deadbeef")
	recorder := httptest.NewRecorder()
	New(&imgtoolKeysCoreStub{}, nil, testLogger()).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
