package httpapi

import (
	"context"
	"github.com/jqcode/portal/backend/internal/coreclient"
	"testing"
	"time"
)

type retryCheckCore struct {
	checksCore
	responses []string
}

func (f *retryCheckCore) AccountPromptRequest(context.Context, int64, string, string, string) (coreclient.Response, error) {
	f.calls++
	index := f.calls - 1
	if index >= len(f.responses) {
		index = len(f.responses) - 1
	}
	return coreclient.Response{StatusCode: 200, Body: []byte(f.responses[index])}, nil
}
func TestAccountCheckTransientRetry(t *testing.T) {
	transient := "data: {\"type\":\"error\",\"error\":\"API returned 503: upstream unavailable\"}\n"
	for _, tc := range []struct {
		name    string
		events  []string
		calls   int
		success bool
	}{
		{"recovers", []string{transient, checkEvents("answer", true)}, 2, true},
		{"bounded", []string{transient}, 2, false},
		{"no partial replay", []string{checkEvents("partial", false) + "\n" + transient}, 1, false},
		{"policy close", []string{"data: {\"type\":\"error\",\"error\":\"websocket closed before terminal event (code 1008)\"}\n"}, 1, false},
		{"authentication", []string{"data: {\"type\":\"error\",\"error\":\"API returned 401: invalid credentials\"}\n"}, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			core := &retryCheckCore{responses: tc.events}
			result := performAccountCheck(context.Background(), core, 42, accountCheckInput{Prompt: "unchanged"}, "token", 0)
			if core.calls != tc.calls || result.Attempts != tc.calls || (result.Error == "") != tc.success {
				t.Fatalf("calls=%d result=%+v", core.calls, result)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	core := &retryCheckCore{responses: []string{transient}}
	performAccountCheck(ctx, core, 42, accountCheckInput{Prompt: "question"}, "token", time.Hour)
	if core.calls != 1 {
		t.Fatal("retried cancelled request")
	}
}
