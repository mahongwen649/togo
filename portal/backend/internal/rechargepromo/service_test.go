package rechargepromo

import "testing"

func TestBonusCents(t *testing.T) {
	tests := []struct {
		amount float64
		want   int64
	}{
		{50, 500}, {100, 1200}, {200, 2500},
		{10, 0}, {49.99, 0}, {50.01, 0}, {500, 0},
	}
	for _, test := range tests {
		if got := BonusCents(test.amount); got != test.want {
			t.Fatalf("BonusCents(%v) = %d, want %d", test.amount, got, test.want)
		}
	}
}
