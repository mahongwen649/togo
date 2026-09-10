package httpapi

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRechargeOrderRowIncludesCurrentBalance(t *testing.T) {
	payload, err := json.Marshal(rechargeOrderRow{ID: 121, UserID: 165, CurrentBalance: 12.34, TotalRecharged: 125.5})
	if err != nil {
		t.Fatalf("marshal recharge order: %v", err)
	}
	if !strings.Contains(string(payload), `"current_balance":12.34`) {
		t.Fatalf("current balance missing from recharge order response: %s", payload)
	}
	if !strings.Contains(string(payload), `"total_recharged":125.5`) {
		t.Fatalf("total recharged missing from recharge order response: %s", payload)
	}
}

func TestRechargeSummaryIncludesAllTimeUserTotals(t *testing.T) {
	payload, err := json.Marshal(rechargeSummary{AllTimeTotalRecharged: 204, RechargeUsersBalance: 83.75})
	if err != nil {
		t.Fatalf("marshal recharge summary: %v", err)
	}
	if !strings.Contains(string(payload), `"all_time_total_recharged":204`) {
		t.Fatalf("all-time total missing from recharge summary response: %s", payload)
	}
	if !strings.Contains(string(payload), `"recharge_users_balance":83.75`) {
		t.Fatalf("recharge user balance missing from recharge summary response: %s", payload)
	}
}

func TestRechargePageCount(t *testing.T) {
	tests := []struct {
		total    int64
		pageSize int
		want     int64
	}{
		{total: 0, pageSize: 50, want: 0},
		{total: 1, pageSize: 50, want: 1},
		{total: 50, pageSize: 50, want: 1},
		{total: 51, pageSize: 50, want: 2},
	}
	for _, test := range tests {
		if got := rechargePageCount(test.total, test.pageSize); got != test.want {
			t.Fatalf("rechargePageCount(%d, %d)=%d, want %d", test.total, test.pageSize, got, test.want)
		}
	}
}
