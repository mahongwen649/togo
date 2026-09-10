package httpapi

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"imgtool/server/internal/auth"
	"imgtool/server/internal/channels"
	"imgtool/server/internal/db"
	"imgtool/server/internal/generation"
	"imgtool/server/internal/history"
	"imgtool/server/internal/secure"
)

func TestAuthHTTPLoginMeLogout(t *testing.T) {
	service := newBootstrappedService(t)
	server := NewServer(Dependencies{Auth: service, SecureCookies: false})

	loginBody, _ := json.Marshal(map[string]string{"username": "admin", "password": "secret"})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	loginRes := httptest.NewRecorder()
	server.ServeHTTP(loginRes, loginReq)

	if loginRes.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginRes.Code, loginRes.Body.String())
	}
	cookies := loginRes.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != auth.CookieName || !cookies[0].HttpOnly {
		t.Fatalf("unexpected cookies: %#v", cookies)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	meReq.AddCookie(cookies[0])
	meRes := httptest.NewRecorder()
	server.ServeHTTP(meRes, meReq)
	if meRes.Code != http.StatusOK {
		t.Fatalf("me status = %d, body = %s", meRes.Code, meRes.Body.String())
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutReq.AddCookie(cookies[0])
	logoutRes := httptest.NewRecorder()
	server.ServeHTTP(logoutRes, logoutReq)
	if logoutRes.Code != http.StatusOK {
		t.Fatalf("logout status = %d", logoutRes.Code)
	}

	meAfterLogoutReq := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	meAfterLogoutReq.AddCookie(cookies[0])
	meAfterLogoutRes := httptest.NewRecorder()
	server.ServeHTTP(meAfterLogoutRes, meAfterLogoutReq)
	if meAfterLogoutRes.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout status = %d", meAfterLogoutRes.Code)
	}
}

func TestAuthHTTPMeRequiresCookie(t *testing.T) {
	service := auth.NewService(auth.NewMemoryStore(), auth.Options{
		AdminUsername: "admin",
		AdminPassword: "secret",
		SessionTTL:    time.Hour,
	})
	server := NewServer(Dependencies{Auth: service})

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", res.Code)
	}
}

func TestSSOLoginCreatesSessionForHostUser(t *testing.T) {
	service := newBootstrappedService(t)
	server := NewServer(Dependencies{
		Auth: service,
		SSO: auth.SSOOptions{
			Secret:   "01234567890123456789012345678901",
			Issuer:   "sub2api",
			Audience: "imgtool",
			Now:      func() time.Time { return time.Unix(1001, 0) },
		},
	})
	ticket := signHTTPTestTicket(t, "01234567890123456789012345678901", 42, "alice", "alice@example.com", "user")

	req := httptest.NewRequest(http.MethodGet, "/sso?ticket="+url.QueryEscape(ticket), nil)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusFound {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	cookies := res.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != auth.CookieName || !cookies[0].HttpOnly {
		t.Fatalf("expected imgtool session cookie, got %#v", cookies)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	meReq.AddCookie(cookies[0])
	meRes := httptest.NewRecorder()
	server.ServeHTTP(meRes, meReq)
	if meRes.Code != http.StatusOK {
		t.Fatalf("me status = %d, body = %s", meRes.Code, meRes.Body.String())
	}
}

func TestAdminUsersHTTP(t *testing.T) {
	service := newBootstrappedService(t)
	server := NewServer(Dependencies{Auth: service})
	adminCookie := loginCookie(t, server, "admin", "secret")

	createBody, _ := json.Marshal(map[string]string{"username": "alice", "password": "start-password"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/admin/users", bytes.NewReader(createBody))
	createReq.AddCookie(adminCookie)
	createRes := httptest.NewRecorder()
	server.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", createRes.Code, createRes.Body.String())
	}
	aliceID := readUserID(t, createRes.Body.String())

	listReq := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	listReq.AddCookie(adminCookie)
	listRes := httptest.NewRecorder()
	server.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRes.Code, listRes.Body.String())
	}

	patchBody, _ := json.Marshal(map[string]bool{"disabled": true})
	patchReq := httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+aliceID, bytes.NewReader(patchBody))
	patchReq.AddCookie(adminCookie)
	patchRes := httptest.NewRecorder()
	server.ServeHTTP(patchRes, patchReq)
	if patchRes.Code != http.StatusOK {
		t.Fatalf("patch status = %d, body = %s", patchRes.Code, patchRes.Body.String())
	}

	resetBody, _ := json.Marshal(map[string]string{"password": "new-password"})
	resetReq := httptest.NewRequest(http.MethodPost, "/api/admin/users/"+aliceID+"/reset-password", bytes.NewReader(resetBody))
	resetReq.AddCookie(adminCookie)
	resetRes := httptest.NewRecorder()
	server.ServeHTTP(resetRes, resetReq)
	if resetRes.Code != http.StatusOK {
		t.Fatalf("reset status = %d, body = %s", resetRes.Code, resetRes.Body.String())
	}
}

func TestAdminUsersHTTPRequiresAdmin(t *testing.T) {
	service := newBootstrappedService(t)
	user, err := service.CreateUser("alice", "start-password", auth.RoleUser)
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	if user.ID == "" {
		t.Fatal("expected user id")
	}
	server := NewServer(Dependencies{Auth: service})
	userCookie := loginCookie(t, server, "alice", "start-password")

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	req.AddCookie(userCookie)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
}

func TestChannelsHTTPAreScopedToCurrentUser(t *testing.T) {
	authService := newBootstrappedService(t)
	alice, err := authService.CreateUser("alice", "alice-password", auth.RoleUser)
	if err != nil {
		t.Fatalf("CreateUser alice: %v", err)
	}
	bob, err := authService.CreateUser("bob", "bob-password", auth.RoleUser)
	if err != nil {
		t.Fatalf("CreateUser bob: %v", err)
	}
	if alice.ID == bob.ID {
		t.Fatal("expected distinct users")
	}
	channelService := channels.NewService(channels.NewMemoryStore(), secure.NewSecretBox("test-secret"))
	server := NewServer(Dependencies{Auth: authService, Channels: channelService})
	aliceCookie := loginCookie(t, server, "alice", "alice-password")
	bobCookie := loginCookie(t, server, "bob", "bob-password")

	createBody, _ := json.Marshal(map[string]any{
		"name":    "OpenAI",
		"baseUrl": "https://api.openai.com",
		"apiKey":  "sk-alice",
		"models": []map[string]any{{
			"id":           "gpt-image-1",
			"capabilities": []string{"text-to-image"},
		}},
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/channels", bytes.NewReader(createBody))
	createReq.AddCookie(aliceCookie)
	createRes := httptest.NewRecorder()
	server.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", createRes.Code, createRes.Body.String())
	}
	if bytes.Contains(createRes.Body.Bytes(), []byte("sk-alice")) {
		t.Fatalf("api key leaked in create response: %s", createRes.Body.String())
	}

	aliceListReq := httptest.NewRequest(http.MethodGet, "/api/channels", nil)
	aliceListReq.AddCookie(aliceCookie)
	aliceListRes := httptest.NewRecorder()
	server.ServeHTTP(aliceListRes, aliceListReq)
	if aliceListRes.Code != http.StatusOK {
		t.Fatalf("alice list status = %d", aliceListRes.Code)
	}
	if !bytes.Contains(aliceListRes.Body.Bytes(), []byte("OpenAI")) || bytes.Contains(aliceListRes.Body.Bytes(), []byte("sk-alice")) {
		t.Fatalf("unexpected alice list response: %s", aliceListRes.Body.String())
	}

	bobListReq := httptest.NewRequest(http.MethodGet, "/api/channels", nil)
	bobListReq.AddCookie(bobCookie)
	bobListRes := httptest.NewRecorder()
	server.ServeHTTP(bobListRes, bobListReq)
	if bobListRes.Code != http.StatusOK {
		t.Fatalf("bob list status = %d", bobListRes.Code)
	}
	if bytes.Contains(bobListRes.Body.Bytes(), []byte("OpenAI")) {
		t.Fatalf("bob saw alice channel: %s", bobListRes.Body.String())
	}
}

func TestHistoryHTTPRequiresUserAndListsOwnRecords(t *testing.T) {
	authService := newBootstrappedService(t)
	historyService := history.NewService(openHTTPTestDB(t))
	server := NewServer(Dependencies{Auth: authService, History: historyService})
	adminCookie := loginCookie(t, server, "admin", "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/history", nil)
	req.AddCookie(adminCookie)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("history status = %d, body = %s", res.Code, res.Body.String())
	}

	anonReq := httptest.NewRequest(http.MethodGet, "/api/history", nil)
	anonRes := httptest.NewRecorder()
	server.ServeHTTP(anonRes, anonReq)
	if anonRes.Code != http.StatusUnauthorized {
		t.Fatalf("anon status = %d", anonRes.Code)
	}
}

func TestTasksHTTPListsCurrentUsersRunningTasks(t *testing.T) {
	authService := newBootstrappedService(t)
	database := openHTTPTestDB(t)
	historyService := history.NewService(database)
	server := NewServer(Dependencies{Auth: authService, History: historyService})
	adminCookie := loginCookie(t, server, "admin", "secret")
	adminID := firstAdminID(t, authService)
	seedUserID(t, database, adminID)
	seedUserID(t, database, "bob")

	adminTask, err := historyService.CreateTask(adminID, history.TaskInput{
		Kind:        "image",
		ChannelID:   "chn_admin",
		ChannelName: "OpenAI",
		BaseURL:     "https://api.openai.com",
		ModelID:     "gpt-image-1",
		Capability:  "text-to-image",
		Prompt:      "admin running",
		Parameters:  map[string]any{"size": "1024x1024"},
	})
	if err != nil {
		t.Fatalf("CreateTask admin: %v", err)
	}
	doneTask, err := historyService.CreateTask(adminID, history.TaskInput{
		Kind:        "image",
		ChannelID:   "chn_admin",
		ChannelName: "OpenAI",
		BaseURL:     "https://api.openai.com",
		ModelID:     "gpt-image-1",
		Capability:  "text-to-image",
		Prompt:      "admin done",
		Parameters:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("CreateTask done: %v", err)
	}
	if _, err := historyService.CompleteTask(adminID, doneTask.ID, history.CompleteInput{Status: "完成"}); err != nil {
		t.Fatalf("CompleteTask done: %v", err)
	}
	if _, err := historyService.CreateTask("bob", history.TaskInput{
		Kind:        "image",
		ChannelID:   "chn_bob",
		ChannelName: "OpenAI",
		BaseURL:     "https://api.openai.com",
		ModelID:     "gpt-image-1",
		Capability:  "text-to-image",
		Prompt:      "bob running",
		Parameters:  map[string]any{},
	}); err != nil {
		t.Fatalf("CreateTask bob: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	req.AddCookie(adminCookie)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("tasks status = %d, body = %s", res.Code, res.Body.String())
	}
	if !bytes.Contains(res.Body.Bytes(), []byte(adminTask.ID)) || !bytes.Contains(res.Body.Bytes(), []byte("admin running")) {
		t.Fatalf("missing admin running task: %s", res.Body.String())
	}
	if bytes.Contains(res.Body.Bytes(), []byte("admin done")) || bytes.Contains(res.Body.Bytes(), []byte("bob running")) {
		t.Fatalf("tasks leaked completed or other user task: %s", res.Body.String())
	}

	anonReq := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	anonRes := httptest.NewRecorder()
	server.ServeHTTP(anonRes, anonReq)
	if anonRes.Code != http.StatusUnauthorized {
		t.Fatalf("anon status = %d", anonRes.Code)
	}
}

func TestCancelTaskHTTPCompletesCurrentUsersRunningTask(t *testing.T) {
	authService := newBootstrappedService(t)
	database := openHTTPTestDB(t)
	historyService := history.NewService(database)
	server := NewServer(Dependencies{Auth: authService, History: historyService})
	adminCookie := loginCookie(t, server, "admin", "secret")
	adminID := firstAdminID(t, authService)
	seedUserID(t, database, adminID)

	task, err := historyService.CreateTask(adminID, history.TaskInput{
		Kind:        "image",
		ChannelID:   "chn_admin",
		ChannelName: "OpenAI",
		BaseURL:     "https://api.openai.com",
		ModelID:     "gpt-image-1",
		Capability:  "text-to-image",
		Prompt:      "cancel from http",
		Parameters:  map[string]any{"size": "1024x1024"},
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/tasks/"+task.ID+"/cancel", nil)
	req.AddCookie(adminCookie)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("cancel status = %d, body = %s", res.Code, res.Body.String())
	}
	if !bytes.Contains(res.Body.Bytes(), []byte(`"cancelled":true`)) || !bytes.Contains(res.Body.Bytes(), []byte("用户取消")) {
		t.Fatalf("unexpected cancel response: %s", res.Body.String())
	}
	running, err := historyService.ListTasks(adminID, "running", 20)
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(running) != 0 {
		t.Fatalf("task still running: %#v", running)
	}

	anonReq := httptest.NewRequest(http.MethodPost, "/api/tasks/"+task.ID+"/cancel", nil)
	anonRes := httptest.NewRecorder()
	server.ServeHTTP(anonRes, anonReq)
	if anonRes.Code != http.StatusUnauthorized {
		t.Fatalf("anon status = %d", anonRes.Code)
	}
}

func TestGenerateImageHTTP(t *testing.T) {
	authService := newBootstrappedService(t)
	database := openHTTPTestDB(t)
	channelService := channels.NewService(channels.NewSQLStore(database), secure.NewSecretBox("test-secret"))
	historyService := history.NewService(database)
	provider := &fakeTextProvider{images: []generation.ImageResult{{Bytes: []byte("png"), MimeType: "image/png"}}}
	storage := &fakeImageStorage{}
	generationService := generation.NewService(channelService, historyService, provider, storage)
	server := NewServer(Dependencies{Auth: authService, Channels: channelService, History: historyService, Generation: generationService})
	adminCookie := loginCookie(t, server, "admin", "secret")
	adminID := firstAdminID(t, authService)
	seedUserID(t, database, adminID)

	channel, err := channelService.Create(adminID, channels.CreateInput{
		Name:    "OpenAI",
		BaseURL: "https://api.openai.com",
		APIKey:  "sk-admin",
		Models:  []channels.ModelInput{{ID: "gpt-image-1", Capabilities: []string{"text-to-image"}}},
	})
	if err != nil {
		t.Fatalf("Create channel: %v", err)
	}

	body, _ := json.Marshal(map[string]any{
		"channelId": channel.ID,
		"modelId":   "gpt-image-1",
		"prompt":    "draw a whale",
		"size":      "1024x1024",
		"count":     1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/generate/image", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(adminCookie)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("generate image status = %d, body = %s", res.Code, res.Body.String())
	}
	if !bytes.Contains(res.Body.Bytes(), []byte(`"status":"running"`)) {
		t.Fatalf("expected running image task: %s", res.Body.String())
	}
	awaitHistoryCount(t, historyService, adminID, 1)
	records, err := historyService.ListHistory(adminID, 10)
	if err != nil {
		t.Fatalf("ListHistory returned error: %v", err)
	}
	if len(records) != 1 || len(records[0].Files) != 1 || records[0].Files[0].ObjectKey != "aiImg/http-test.png" {
		t.Fatalf("missing generated image file: %#v", records)
	}
}

func TestGenerateImageHTTPReturnsRunningTaskBeforeProviderCompletes(t *testing.T) {
	authService := newBootstrappedService(t)
	database := openHTTPTestDB(t)
	channelService := channels.NewService(channels.NewSQLStore(database), secure.NewSecretBox("test-secret"))
	historyService := history.NewService(database)
	provider := newBlockingImageProvider()
	storage := &fakeImageStorage{}
	generationService := generation.NewService(channelService, historyService, provider, storage)
	server := NewServer(Dependencies{Auth: authService, Channels: channelService, History: historyService, Generation: generationService})
	adminCookie := loginCookie(t, server, "admin", "secret")
	adminID := firstAdminID(t, authService)
	seedUserID(t, database, adminID)

	channel, err := channelService.Create(adminID, channels.CreateInput{
		Name:    "OpenAI",
		BaseURL: "https://api.openai.com",
		APIKey:  "sk-admin",
		Models:  []channels.ModelInput{{ID: "gpt-image-1", Capabilities: []string{"text-to-image"}}},
	})
	if err != nil {
		t.Fatalf("Create channel: %v", err)
	}

	body, _ := json.Marshal(map[string]any{
		"channelId": channel.ID,
		"modelId":   "gpt-image-1",
		"prompt":    "draw a whale",
		"size":      "1024x1024",
		"count":     1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/generate/image", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(adminCookie)
	res := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		server.ServeHTTP(res, req)
		close(done)
	}()

	<-provider.started
	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		provider.release()
		<-done
		t.Fatal("generate image HTTP request waited for provider completion")
	}
	defer provider.release()

	if res.Code != http.StatusOK {
		t.Fatalf("generate image status = %d, body = %s", res.Code, res.Body.String())
	}
	if !bytes.Contains(res.Body.Bytes(), []byte(`"status":"running"`)) {
		t.Fatalf("expected running task response, body = %s", res.Body.String())
	}
	if !bytes.Contains(res.Body.Bytes(), []byte(`"historyId":""`)) {
		t.Fatalf("response should not include completed history yet: %s", res.Body.String())
	}

	tasks, err := historyService.ListTasks(adminID, "running", 20)
	if err != nil {
		t.Fatalf("ListTasks returned error: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Prompt != "draw a whale" {
		t.Fatalf("unexpected running tasks: %#v", tasks)
	}
}

func TestGenerateImageToImageHTTP(t *testing.T) {
	authService := newBootstrappedService(t)
	database := openHTTPTestDB(t)
	channelService := channels.NewService(channels.NewSQLStore(database), secure.NewSecretBox("test-secret"))
	historyService := history.NewService(database)
	provider := &fakeTextProvider{images: []generation.ImageResult{{Bytes: []byte("png"), MimeType: "image/png"}}}
	storage := &fakeImageStorage{}
	generationService := generation.NewService(channelService, historyService, provider, storage)
	server := NewServer(Dependencies{Auth: authService, Channels: channelService, History: historyService, Generation: generationService})
	adminCookie := loginCookie(t, server, "admin", "secret")
	adminID := firstAdminID(t, authService)
	seedUserID(t, database, adminID)

	channel, err := channelService.Create(adminID, channels.CreateInput{
		Name:    "OpenAI",
		BaseURL: "https://api.openai.com",
		APIKey:  "sk-admin",
		Models:  []channels.ModelInput{{ID: "gpt-image-1", Capabilities: []string{"image-to-image"}}},
	})
	if err != nil {
		t.Fatalf("Create channel: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("channelId", channel.ID)
	_ = writer.WriteField("modelId", "gpt-image-1")
	_ = writer.WriteField("prompt", "make it cinematic")
	_ = writer.WriteField("size", "1024x1024")
	_ = writer.WriteField("count", "1")
	fileHeader := make(textproto.MIMEHeader)
	fileHeader.Set("Content-Disposition", `form-data; name="inputImage"; filename="ref.png"`)
	fileHeader.Set("Content-Type", "application/octet-stream")
	fileWriter, err := writer.CreatePart(fileHeader)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	_, _ = fileWriter.Write([]byte("\x89PNG\r\n\x1a\nfake-image"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/generate/image", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.AddCookie(adminCookie)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("generate image-to-image status = %d, body = %s", res.Code, res.Body.String())
	}
	if !bytes.Contains(res.Body.Bytes(), []byte(`"status":"running"`)) {
		t.Fatalf("expected running image-to-image task: %s", res.Body.String())
	}
	awaitHistoryCount(t, historyService, adminID, 1)
	if provider.lastImageRequest.Capability != "image-to-image" {
		t.Fatalf("capability = %q", provider.lastImageRequest.Capability)
	}
	if string(provider.lastImageRequest.Image) != "\x89PNG\r\n\x1a\nfake-image" || provider.lastImageRequest.MimeType != "image/png" {
		t.Fatalf("image request = %#v", provider.lastImageRequest)
	}
}

func TestSignedFileURLHTTPRequiresOwner(t *testing.T) {
	authService := newBootstrappedService(t)
	database := openHTTPTestDB(t)
	historyService := history.NewService(database)
	server := NewServer(Dependencies{
		Auth:       authService,
		History:    historyService,
		FileSigner: fakeFileSigner{url: "https://signed.example/file.png"},
	})
	adminCookie := loginCookie(t, server, "admin", "secret")
	adminID := firstAdminID(t, authService)
	seedUserID(t, database, adminID)

	task, err := historyService.CreateTask(adminID, history.TaskInput{
		Kind:        "image",
		ChannelID:   "chn_admin",
		ChannelName: "OpenAI",
		BaseURL:     "https://api.openai.com",
		ModelID:     "gpt-image-1",
		Capability:  "text-to-image",
		Prompt:      "draw",
		Parameters:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	record, err := historyService.CompleteTask(adminID, task.ID, history.CompleteInput{
		Status: "完成",
		Files: []history.FileInput{{
			StorageProvider: "aliyun-oss",
			Bucket:          "bucket",
			ObjectKey:       "aiImg/users/admin/result.png",
			MediaType:       "image",
			MimeType:        "image/png",
			SizeBytes:       3,
		}},
	})
	if err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/files/"+record.Files[0].ID+"/signed-url", nil)
	req.AddCookie(adminCookie)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("signed url status = %d, body = %s", res.Code, res.Body.String())
	}
	if !bytes.Contains(res.Body.Bytes(), []byte("https://signed.example/file.png")) {
		t.Fatalf("missing signed url: %s", res.Body.String())
	}
}

func TestSignedFileDownloadURLHTTPRequiresOwner(t *testing.T) {
	authService := newBootstrappedService(t)
	database := openHTTPTestDB(t)
	historyService := history.NewService(database)
	server := NewServer(Dependencies{
		Auth:       authService,
		History:    historyService,
		FileSigner: fakeFileSigner{url: "https://signed.example/file.png", downloadURL: "https://signed.example/file.png?download=1"},
	})
	adminCookie := loginCookie(t, server, "admin", "secret")
	adminID := firstAdminID(t, authService)
	seedUserID(t, database, adminID)

	task, err := historyService.CreateTask(adminID, history.TaskInput{
		Kind:        "image",
		ChannelID:   "chn_admin",
		ChannelName: "OpenAI",
		BaseURL:     "https://api.openai.com",
		ModelID:     "gpt-image-1",
		Capability:  "text-to-image",
		Prompt:      "draw",
		Parameters:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	record, err := historyService.CompleteTask(adminID, task.ID, history.CompleteInput{
		Status: "完成",
		Files: []history.FileInput{{
			StorageProvider: "aliyun-oss",
			Bucket:          "bucket",
			ObjectKey:       "aiImg/users/admin/result.png",
			MediaType:       "image",
			MimeType:        "image/png",
			SizeBytes:       3,
		}},
	})
	if err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/files/"+record.Files[0].ID+"/download-url", nil)
	req.AddCookie(adminCookie)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("download url status = %d, body = %s", res.Code, res.Body.String())
	}
	if !bytes.Contains(res.Body.Bytes(), []byte("https://signed.example/file.png?download=1")) {
		t.Fatalf("missing download url: %s", res.Body.String())
	}
}

func TestFileContentHTTPRequiresOwner(t *testing.T) {
	authService := newBootstrappedService(t)
	database := openHTTPTestDB(t)
	historyService := history.NewService(database)
	server := NewServer(Dependencies{
		Auth:       authService,
		History:    historyService,
		FileReader: fakeFileReader{content: []byte("png-bytes")},
	})
	adminCookie := loginCookie(t, server, "admin", "secret")
	adminID := firstAdminID(t, authService)
	seedUserID(t, database, adminID)

	task, err := historyService.CreateTask(adminID, history.TaskInput{
		Kind:        "image",
		ChannelID:   "chn_admin",
		ChannelName: "OpenAI",
		BaseURL:     "https://api.openai.com",
		ModelID:     "gpt-image-1",
		Capability:  "text-to-image",
		Prompt:      "draw",
		Parameters:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	record, err := historyService.CompleteTask(adminID, task.ID, history.CompleteInput{
		Status: "完成",
		Files: []history.FileInput{{
			StorageProvider: "aliyun-oss",
			Bucket:          "bucket",
			ObjectKey:       "aiImg/users/admin/result.png",
			MediaType:       "image",
			MimeType:        "image/png",
			SizeBytes:       9,
		}},
	})
	if err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/files/"+record.Files[0].ID+"/content", nil)
	req.AddCookie(adminCookie)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("file content status = %d, body = %s", res.Code, res.Body.String())
	}
	if res.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("content type = %q", res.Header().Get("Content-Type"))
	}
	if res.Body.String() != "png-bytes" {
		t.Fatalf("body = %q", res.Body.String())
	}
}

func TestDeleteHistoryHTTPDeletesCurrentUsersRecord(t *testing.T) {
	authService := newBootstrappedService(t)
	database := openHTTPTestDB(t)
	historyService := history.NewService(database)
	server := NewServer(Dependencies{Auth: authService, History: historyService})
	adminCookie := loginCookie(t, server, "admin", "secret")
	adminID := firstAdminID(t, authService)
	seedUserID(t, database, adminID)

	task, err := historyService.CreateTask(adminID, history.TaskInput{
		Kind:        "text",
		ChannelID:   "chn_admin",
		ChannelName: "OpenAI",
		BaseURL:     "https://api.openai.com",
		ModelID:     "gpt-4o",
		Capability:  "image-to-text",
		Prompt:      "describe",
		Parameters:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	record, err := historyService.CompleteTask(adminID, task.ID, history.CompleteInput{
		Status:     "完成",
		ResultText: "caption",
	})
	if err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/history/"+record.ID, nil)
	req.AddCookie(adminCookie)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("delete history status = %d, body = %s", res.Code, res.Body.String())
	}
	records, err := historyService.ListHistory(adminID, 20)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("deleted record still listed: %#v", records)
	}
}

func TestDeleteHistoryHTTPDeletesSourceFiles(t *testing.T) {
	authService := newBootstrappedService(t)
	database := openHTTPTestDB(t)
	historyService := history.NewService(database)
	fileDeleter := &fakeFileDeleter{}
	server := NewServer(Dependencies{Auth: authService, History: historyService, FileDeleter: fileDeleter})
	adminCookie := loginCookie(t, server, "admin", "secret")
	adminID := firstAdminID(t, authService)
	seedUserID(t, database, adminID)

	task, err := historyService.CreateTask(adminID, history.TaskInput{
		Kind:        "image",
		ChannelID:   "chn_admin",
		ChannelName: "OpenAI",
		BaseURL:     "https://api.openai.com",
		ModelID:     "gpt-image-1",
		Capability:  "image-to-image",
		Prompt:      "edit",
		Parameters:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	record, err := historyService.CompleteTask(adminID, task.ID, history.CompleteInput{
		Status: "完成",
		Files: []history.FileInput{
			{
				StorageProvider: "aliyun-oss",
				Bucket:          "bucket",
				ObjectKey:       "aiImg/reference.png",
				Role:            "reference",
				MediaType:       "image",
				MimeType:        "image/png",
				SizeBytes:       9,
			},
			{
				StorageProvider: "aliyun-oss",
				Bucket:          "bucket",
				ObjectKey:       "aiImg/result.png",
				Role:            "result",
				MediaType:       "image",
				MimeType:        "image/png",
				SizeBytes:       3,
			},
		},
	})
	if err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/history/"+record.ID, nil)
	req.AddCookie(adminCookie)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("delete history status = %d, body = %s", res.Code, res.Body.String())
	}
	want := []string{"aiImg/reference.png", "aiImg/result.png"}
	if !reflect.DeepEqual(fileDeleter.deleted, want) {
		t.Fatalf("deleted source files = %#v, want %#v", fileDeleter.deleted, want)
	}
}

func newBootstrappedService(t *testing.T) *auth.Service {
	t.Helper()
	service := auth.NewService(auth.NewMemoryStore(), auth.Options{
		AdminUsername: "admin",
		AdminPassword: "secret",
		SessionTTL:    time.Hour,
	})
	if _, err := service.BootstrapAdmin(); err != nil {
		t.Fatalf("BootstrapAdmin returned error: %v", err)
	}
	return service
}

func openHTTPTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(t.TempDir() + "/imgtool.sqlite")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	_, err = database.Exec(
		`INSERT INTO users (id, username, password_hash, role, created_at, updated_at)
		 VALUES (?, ?, ?, 'admin', 1, 1)`,
		"usr_admin",
		"admin",
		"test-hash",
	)
	if err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func loginCookie(t *testing.T, server http.Handler, username string, password string) *http.Cookie {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", res.Code, res.Body.String())
	}
	cookies := res.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie")
	}
	return cookies[0]
}

func readUserID(t *testing.T, body string) string {
	t.Helper()
	var payload struct {
		Data struct {
			User struct {
				ID string `json:"id"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.NopCloser(bytes.NewBufferString(body))).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Data.User.ID == "" {
		t.Fatalf("missing user id in response: %s", body)
	}
	return payload.Data.User.ID
}

func firstAdminID(t *testing.T, service *auth.Service) string {
	t.Helper()
	users, err := service.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	for _, user := range users {
		if user.Role == auth.RoleAdmin {
			return user.ID
		}
	}
	t.Fatal("admin user not found")
	return ""
}

func seedUserID(t *testing.T, database *sql.DB, id string) {
	t.Helper()
	_, err := database.Exec(
		`INSERT OR IGNORE INTO users (id, username, password_hash, role, created_at, updated_at)
		 VALUES (?, ?, ?, 'admin', 1, 1)`,
		id,
		"seed_"+id,
		"test-hash",
	)
	if err != nil {
		t.Fatalf("seed user id %s: %v", id, err)
	}
}

func awaitHistoryCount(t *testing.T, service *history.Service, userID string, count int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		records, err := service.ListHistory(userID, 20)
		if err != nil {
			t.Fatalf("ListHistory returned error: %v", err)
		}
		if len(records) >= count {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("history count = %d, want at least %d", len(records), count)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type fakeTextProvider struct {
	images           []generation.ImageResult
	lastImageRequest generation.ImageRequest
}

func (p *fakeTextProvider) GenerateImage(_ channels.Snapshot, request generation.ImageRequest) ([]generation.ImageResult, error) {
	p.lastImageRequest = request
	return p.images, nil
}

type fakeImageStorage struct{}

func (s *fakeImageStorage) SaveGeneratedImage(input generation.StoreImageInput) (generation.StoredImage, error) {
	return generation.StoredImage{
		StorageProvider: "fake",
		Bucket:          "bucket",
		ObjectKey:       "aiImg/http-test.png",
		MediaType:       "image",
		MimeType:        input.MimeType,
		SizeBytes:       int64(len(input.Bytes)),
	}, nil
}

type blockingImageProvider struct {
	started  chan struct{}
	released chan struct{}
	once     sync.Once
}

func newBlockingImageProvider() *blockingImageProvider {
	return &blockingImageProvider{
		started:  make(chan struct{}),
		released: make(chan struct{}),
	}
}

func (p *blockingImageProvider) GenerateImage(_ channels.Snapshot, _ generation.ImageRequest) ([]generation.ImageResult, error) {
	p.once.Do(func() { close(p.started) })
	<-p.released
	return []generation.ImageResult{{Bytes: []byte("png"), MimeType: "image/png"}}, nil
}

func (p *blockingImageProvider) release() {
	select {
	case <-p.released:
	default:
		close(p.released)
	}
}

type fakeFileSigner struct {
	url         string
	downloadURL string
}

func (s fakeFileSigner) SignedGetURL(_ string) (string, error) {
	return s.url, nil
}

func (s fakeFileSigner) SignedDownloadURL(_, _ string) (string, error) {
	if s.downloadURL != "" {
		return s.downloadURL, nil
	}
	return s.url, nil
}

type fakeFileReader struct {
	content []byte
}

func (r fakeFileReader) ReadObject(_ string) ([]byte, error) {
	return r.content, nil
}

type fakeFileDeleter struct {
	deleted []string
}

func (d *fakeFileDeleter) DeleteObject(objectKey string) error {
	d.deleted = append(d.deleted, objectKey)
	return nil
}

func signHTTPTestTicket(t *testing.T, secret string, userID int64, username string, email string, role string) string {
	t.Helper()
	payload := auth.SSOTicketClaims{
		Issuer:    "sub2api",
		Audience:  "imgtool",
		Subject:   strconv.FormatInt(userID, 10),
		Username:  username,
		Email:     email,
		Role:      role,
		IssuedAt:  time.Unix(1000, 0).Unix(),
		ExpiresAt: time.Unix(1120, 0).Unix(),
		ID:        "http-test-ticket",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal ticket: %v", err)
	}
	encodedBody := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(encodedBody))
	return encodedBody + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
