package httpapi

import (
	"testing"
	"time"
)

func TestPelicanCandidatesRespectEligibilityAndAttemptBudget(t *testing.T) {
	now := time.Now()
	a := pelicanAccount{ID: 1, Platform: "openai", Type: "apikey", Status: "active", Schedulable: true, GroupIDs: []int64{57}}
	if !a.eligible(57, now) || a.eligible(58, now) {
		t.Fatal("group membership not enforced")
	}
	a.Credentials.ModelMapping = map[string]string{"gpt-image-2": "gpt-image-2"}
	if a.eligible(57, now) {
		t.Fatal("image-only account selected")
	}
	a.Credentials.ModelMapping = map[string]string{"gpt-6-*": "model"}
	if !a.eligible(57, now) {
		t.Fatal("wildcard model not recognized")
	}
	future := now.Add(time.Minute)
	a.RateLimitResetAt = &future
	if a.eligible(57, now) {
		t.Fatal("cooling account selected")
	}
	if pelicanRunTimeout < pelicanMaxAttempts*pelicanAttemptTimeout+30*time.Second {
		t.Fatal("later attempts lose their time budget")
	}
}

func TestPelicanEligibilityUsesConfiguredModel(t *testing.T) {
	a := pelicanAccount{ID: 1, Platform: "openai", Type: "apikey", Status: "active", Schedulable: true, GroupIDs: []int64{57}}
	a.Credentials.ModelMapping = map[string]string{"custom-*": "upstream"}
	if !a.eligible(57, time.Now(), "custom-model") || a.eligible(57, time.Now(), exclusivePageModel) {
		t.Fatal("eligibility ignored configured model")
	}
}
