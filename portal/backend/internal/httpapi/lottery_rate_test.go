package httpapi

import (
	"net/http/httptest"
	"testing"
)

func TestAllowLotteryRequestEnforcesPerClientBucketLimit(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/v1/lottery/current", nil)
	request.RemoteAddr = "192.0.2.10:12345"
	if !allowLotteryRequest(request, "test-limit", 2) {
		t.Fatal("first request was unexpectedly limited")
	}
	if !allowLotteryRequest(request, "test-limit", 2) {
		t.Fatal("second request was unexpectedly limited")
	}
	if allowLotteryRequest(request, "test-limit", 2) {
		t.Fatal("third request was not limited")
	}

	otherClient := httptest.NewRequest("GET", "/api/v1/lottery/current", nil)
	otherClient.RemoteAddr = "192.0.2.11:12345"
	if !allowLotteryRequest(otherClient, "test-limit", 2) {
		t.Fatal("separate client shared another client's limit")
	}
}
