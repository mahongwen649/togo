package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseCodes(t *testing.T) {
	codes, err := parseCodes("\uFEFFcode,label\r\nAAA,普通\r\nBBB,幸运\r\nAAA,重复\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != 3 || codes[0] != "AAA" || codes[1] != "BBB" || codes[2] != "AAA" {
		t.Fatalf("parseCodes = %#v", codes)
	}
}

func TestRealHTTPFlow(t *testing.T) {
	app := &App{store: testStore(t), adminPassword: "test-admin-password"}
	handler := app.routes()

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/gift/api/admin/stats", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized stats status = %d", unauthorized.Code)
	}

	login := httptest.NewRecorder()
	handler.ServeHTTP(login, jsonRequest(http.MethodPost, "/gift/api/admin/login", `{"password":"test-admin-password"}`))
	if login.Code != http.StatusOK || len(login.Result().Cookies()) == 0 {
		t.Fatalf("login status = %d, cookies = %d", login.Code, len(login.Result().Cookies()))
	}
	authCookie := login.Result().Cookies()[0]

	importRequest := jsonRequest(http.MethodPost, "/gift/api/admin/codes/import", `{"content":"REAL-CODE-A\nREAL-CODE-B"}`)
	importRequest.AddCookie(authCookie)
	importResponse := httptest.NewRecorder()
	handler.ServeHTTP(importResponse, importRequest)
	if importResponse.Code != http.StatusOK {
		t.Fatalf("import status = %d, body = %s", importResponse.Code, importResponse.Body.String())
	}

	claimResponse := httptest.NewRecorder()
	handler.ServeHTTP(claimResponse, jsonRequest(http.MethodPost, "/gift/api/claim", `{}`))
	if claimResponse.Code != http.StatusOK {
		t.Fatalf("claim status = %d, body = %s", claimResponse.Code, claimResponse.Body.String())
	}
	var first struct {
		Claim    Claim `json:"claim"`
		Existing bool  `json:"existing"`
	}
	if err := json.NewDecoder(claimResponse.Body).Decode(&first); err != nil {
		t.Fatal(err)
	}
	if first.Claim.Code == "" {
		t.Fatalf("first claim = %+v", first)
	}
	deviceCookie := claimResponse.Result().Cookies()[0]

	repeatResponse := httptest.NewRecorder()
	repeatRequest := jsonRequest(http.MethodPost, "/gift/api/claim", `{}`)
	repeatRequest.AddCookie(deviceCookie)
	handler.ServeHTTP(repeatResponse, repeatRequest)
	if repeatResponse.Code != http.StatusTooManyRequests {
		t.Fatalf("repeat claim status = %d, body = %s", repeatResponse.Code, repeatResponse.Body.String())
	}

	statsRequest := httptest.NewRequest(http.MethodGet, "/gift/api/admin/stats", nil)
	statsRequest.AddCookie(authCookie)
	statsResponse := httptest.NewRecorder()
	handler.ServeHTTP(statsResponse, statsRequest)
	var stats Stats
	if err := json.NewDecoder(statsResponse.Body).Decode(&stats); err != nil {
		t.Fatal(err)
	}
	if stats != (Stats{Total: 2, Claimed: 1, Remaining: 1}) {
		t.Fatalf("stats = %+v", stats)
	}
}

func jsonRequest(method, target, body string) *http.Request {
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	return request
}
