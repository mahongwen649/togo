package lottery

import (
	"math"
	"testing"
)

func TestRandomAmountsStayWithinRangeAndBudget(t *testing.T) {
	campaign := Campaign{RandomLimit: 4, RandomMin: 5, RandomMax: 20, RandomBudget: 50}
	min, max := cents(campaign.RandomMin), cents(campaign.RandomMax)
	target := int64(math.Round(campaign.RandomBudget * 100 * 3 / float64(campaign.RandomLimit)))

	for run := 0; run < 200; run++ {
		amounts, err := randomAmounts(campaign, 3)
		if err != nil {
			t.Fatalf("randomAmounts() error = %v", err)
		}
		var total int64
		for _, amount := range amounts {
			value := cents(amount)
			if value < min || value > max {
				t.Fatalf("amount %v is outside [%v, %v]", amount, campaign.RandomMin, campaign.RandomMax)
			}
			total += value
		}
		if total != target {
			t.Fatalf("total = %d cents, want %d", total, target)
		}
	}
}

func TestRandomAmountsWithNoWinners(t *testing.T) {
	amounts, err := randomAmounts(Campaign{RandomLimit: 1, RandomMin: 1, RandomMax: 2, RandomBudget: 1}, 0)
	if err != nil {
		t.Fatalf("randomAmounts() error = %v", err)
	}
	if len(amounts) != 0 {
		t.Fatalf("len(amounts) = %d, want 0", len(amounts))
	}
}

func TestRandomAmountsRejectsImpossibleBudget(t *testing.T) {
	_, err := randomAmounts(Campaign{RandomLimit: 2, RandomMin: 10, RandomMax: 20, RandomBudget: 5}, 2)
	if err == nil {
		t.Fatal("randomAmounts() error = nil, want invalid budget error")
	}
	problem, ok := err.(*Problem)
	if !ok || problem.Code != "INVALID_BUDGET" {
		t.Fatalf("error = %#v, want INVALID_BUDGET", err)
	}
}
