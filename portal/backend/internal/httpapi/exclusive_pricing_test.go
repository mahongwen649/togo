package httpapi

import (
	"context"
	"math"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

type pricingUserCore struct{ *exclusiveRateCore }

func (f *pricingUserCore) Profile(context.Context, string) (coreclient.User, error) {
	return coreclient.User{ID: 7, Role: "admin", Status: "active", Balance: 10}, nil
}

func pricingUpdate(s *Server, id int64, fields string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("PUT", "/", strings.NewReader(`{"name":"priced","durations":[1440],"cooldown_seconds":0`+fields+`}`))
	r.Header.Set("Authorization", "Bearer admin")
	r.SetPathValue("channelID", strconv.FormatInt(id, 10))
	w := httptest.NewRecorder()
	s.exclusiveUpdateChannel(w, r)
	return w
}

func pricingChannel(t *testing.T, s *Server, id int64) exclusiveChannel {
	t.Helper()
	c, err := scanExclusiveChannel(s.portalDB.QueryRow(`SELECT `+exclusiveChannelColumns+` FROM portal_exclusive_channels WHERE id=$1`, id))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestExclusiveAccountReplacementAndDelete(t *testing.T) {
	s, f, id := rateServer(t)
	if w := pricingUpdate(s, id, `,"account_ids":[2]`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	c := pricingChannel(t, s, id)
	if !sameRentalSelection(c.AccountIDs, []int64{2}) || len(f.accounts[1].GroupIDs) != 0 || len(f.accounts[2].GroupIDs) != 1 || f.accounts[2].GroupIDs[0] != *c.GroupID {
		t.Fatal(c.AccountIDs, f.accounts)
	}
	var reserved []int64
	rows, err := s.portalDB.Query(`SELECT core_account_id FROM portal_exclusive_accounts WHERE channel_id=$1 ORDER BY core_account_id`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var accountID int64
		if err = rows.Scan(&accountID); err != nil {
			t.Fatal(err)
		}
		reserved = append(reserved, accountID)
	}
	if !sameRentalSelection(reserved, []int64{2}) {
		t.Fatal("stale reservations", reserved)
	}

	r := httptest.NewRequest("DELETE", "/", nil)
	r.Header.Set("Authorization", "Bearer admin")
	r.SetPathValue("channelID", strconv.FormatInt(id, 10))
	w := httptest.NewRecorder()
	s.exclusiveDeleteChannel(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if len(f.accounts[2].GroupIDs) != 0 || f.groupDeletes != 1 {
		t.Fatal(f.accounts[2], f.groupDeletes)
	}
	var count int
	if err = s.portalDB.QueryRow(`SELECT count(*) FROM portal_exclusive_channels WHERE id=$1`, id).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}

func TestExclusiveAPIKeyPreservesSharedGroups(t *testing.T) {
	db := rentalTestDB(t)
	fake := &rentalFakeCore{
		accounts: map[int64]rentalAccount{
			1: {ID: 1, Name: "API Key 1", Platform: "openai", Type: "apikey", Status: "active", Schedulable: true, GroupIDs: []int64{71}},
			2: {ID: 2, Name: "API Key 2", Platform: "openai", Type: "apikey", Status: "active", Schedulable: true, GroupIDs: []int64{72}},
		},
		groups: []rentalGroup{},
	}
	core := &exclusiveRateCore{rentalFakeCore: fake}
	s := &Server{portalDB: db, core: core, logger: testLogger(), exclusive: ExclusiveConfig{Enabled: true}}
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"shared-api-key","account_ids":[1],"durations":[1440],"cooldown_seconds":0}`))
	r.Header.Set("Authorization", "Bearer admin")
	w := httptest.NewRecorder()
	s.exclusivePublishChannel(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var channelID int64
	if err := db.QueryRow(`SELECT id FROM portal_exclusive_channels`).Scan(&channelID); err != nil {
		t.Fatal(err)
	}
	c := pricingChannel(t, s, channelID)
	if c.GroupID == nil || !sameRentalSelection(core.accounts[1].GroupIDs, []int64{71, *c.GroupID}) {
		t.Fatal("publish replaced API Key groups", c, core.accounts[1])
	}

	account := core.accounts[1]
	account.GroupIDs = []int64{71}
	core.accounts[1] = account
	if _, err := db.Exec(`UPDATE portal_exclusive_channels SET state='error' WHERE id=$1`, channelID); err != nil {
		t.Fatal(err)
	}
	if err := s.provisionExclusive(context.Background(), channelID); err != nil {
		t.Fatal("retry failed for shared API Key", err)
	}
	if !sameRentalSelection(core.accounts[1].GroupIDs, []int64{71, *c.GroupID}) {
		t.Fatal("retry replaced API Key groups", core.accounts[1])
	}

	r = httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer admin")
	w = httptest.NewRecorder()
	s.exclusiveCandidates(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"id":2`) {
		t.Fatal("shared API Key missing from candidates", w.Code, w.Body.String())
	}

	if w = pricingUpdate(s, channelID, `,"account_ids":[2]`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if !sameRentalSelection(core.accounts[1].GroupIDs, []int64{71}) || !sameRentalSelection(core.accounts[2].GroupIDs, []int64{72, *c.GroupID}) {
		t.Fatal("replacement changed unrelated API Key groups", core.accounts)
	}

	r = httptest.NewRequest("DELETE", "/", nil)
	r.Header.Set("Authorization", "Bearer admin")
	r.SetPathValue("channelID", strconv.FormatInt(channelID, 10))
	w = httptest.NewRecorder()
	s.exclusiveDeleteChannel(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if !sameRentalSelection(core.accounts[2].GroupIDs, []int64{72}) {
		t.Fatal("delete removed unrelated API Key groups", core.accounts[2])
	}
}

func TestExclusivePricingDefaultsOverridesAndReset(t *testing.T) {
	s, f, id := rateServer(t)
	c := pricingChannel(t, s, id)
	if c.ImagePrice != .05 || !c.PricingSynced || !exclusivePricingMatches(c, f.groups[0]) {
		t.Fatal(c, f.groups)
	}
	if c.PricingRevision != 1 {
		t.Fatal(c.PricingRevision)
	}
	if w := pricingUpdate(s, id, `,"image_price":0.08,"model_pricing":[{"model":" GPT-6-ASTRA ","input_price":0.0000025,"output_price":null,"cache_read_price":0}]`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	c = pricingChannel(t, s, id)
	if c.PricingRevision != 2 || len(c.ModelPricing) != 1 || c.ModelPricing[0].Model != "gpt-6-astra" || c.ModelPricing[0].OutputPrice != nil || *c.ModelPricing[0].CacheReadPrice != 0 || c.ImagePrice != .08 || !exclusivePricingMatches(c, f.groups[0]) {
		t.Fatal(c, f.groups)
	}
	if *f.groups[0].ModelPricing[0].InputPrice != .0000025 || f.groups[0].ModelPricing[1].BillingMode != "image" || *f.groups[0].ModelPricing[1].PerRequestPrice != .08 {
		t.Fatal(f.groups)
	}
	// Omitted fields leave all configured prices unchanged.
	if w := pricingUpdate(s, id, ""); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if got := pricingChannel(t, s, id); got.PricingRevision != 2 || got.ImagePrice != .08 {
		t.Fatal(got)
	}
	if w := pricingUpdate(s, id, `,"image_price":0.05,"model_pricing":[]`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	c = pricingChannel(t, s, id)
	if c.PricingRevision != 3 || len(c.ModelPricing) != 0 || c.ImagePrice != .05 || len(f.groups[0].ModelPricing) != 1 {
		t.Fatal(c, f.groups)
	}
	if w := pricingUpdate(s, id, `,"model_pricing":[{"model":"gpt-6-astra"}]`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if got := pricingChannel(t, s, id); got.PricingRevision != 3 {
		t.Fatal("empty overrides should inherit", got)
	}
}

func TestExclusivePricingValidation(t *testing.T) {
	s, f, id := rateServer(t)
	for _, fields := range []string{
		`,"image_rate_multiplier":0`, `,"image_rate_multiplier":-1`, `,"image_rate_multiplier":1e999`,
		`,"image_price":-1`, `,"image_price":1e999`, `,"image_price":"bad"`,
		`,"model_pricing":[{"model":"gpt-6-astra","input_price":-1}]`,
		`,"model_pricing":[{"model":"gpt-6-astra","output_price":"bad"}]`,
		`,"model_pricing":[{"model":"gpt-*","input_price":1}]`,
		`,"model_pricing":[{"model":"gpt-image-2","input_price":1}]`,
		`,"model_pricing":[{"model":"dall-e-3","input_price":1}]`,
		`,"model_pricing":[{"model":""}]`,
		`,"model_pricing":[{"model":"gpt-6-astra"},{"model":" GPT-6-ASTRA "}]`,
	} {
		if w := pricingUpdate(s, id, fields); w.Code != 400 {
			t.Fatal(fields, w.Code, w.Body.String())
		}
	}
	for _, n := range []float64{math.NaN(), math.Inf(1), -1} {
		if validateExclusivePricing(exclusivePublish{ImagePrice: &n}) == nil {
			t.Fatal(n)
		}
	}
	if f.updates != 0 {
		t.Fatal("invalid prices reached Core")
	}
}

func TestExclusiveImageRateIndependent(t *testing.T) {
	s, f, id := rateServer(t)
	c := pricingChannel(t, s, id)
	if c.ImageRateMultiplier != 1 || !f.groups[0].ImageRateIndependent || f.groups[0].ImageRateMultiplier != 1 {
		t.Fatal(c, f.groups)
	}
	if w := rateUpdate(s, id, "3"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if f.groups[0].ImageRateMultiplier != 1 {
		t.Fatal("image rate followed text rate")
	}
	if w := pricingUpdate(s, id, `,"image_rate_multiplier":1.5`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	c = pricingChannel(t, s, id)
	if c.ImageRateMultiplier != 1.5 || c.RateMultiplier != 3 || !exclusivePricingMatches(c, f.groups[0]) {
		t.Fatal(c, f.groups)
	}
	if _, err := s.portalDB.Exec(`UPDATE portal_exclusive_channels SET state='active' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if w := pricingUpdate(s, id, `,"image_rate_multiplier":2`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	c = pricingChannel(t, s, id)
	if c.ImageRateMultiplier != 1.5 || c.PendingConfig == nil || c.PendingConfig.ImageRateMultiplier != 2 {
		t.Fatal(c)
	}
}

func TestExclusivePricingOccupiedAndSyncRecovery(t *testing.T) {
	s, f, id := rateServer(t)
	for _, state := range []string{"active", "releasing", "cooling"} {
		if _, err := s.portalDB.Exec(`UPDATE portal_exclusive_channels SET state=$1 WHERE id=$2`, state, id); err != nil {
			t.Fatal(err)
		}
		if w := pricingUpdate(s, id, `,"image_price":0.08`); w.Code != 200 {
			t.Fatal(state, w.Code, w.Body.String())
		}
		if w := pricingUpdate(s, id, `,"model_pricing":[{"model":"gpt-6-astra","output_price":0}]`); w.Code != 200 {
			t.Fatal(state, w.Code, w.Body.String())
		}
	}
	for _, state := range []string{"provisioning", "error"} {
		if _, err := s.portalDB.Exec(`UPDATE portal_exclusive_channels SET state=$1 WHERE id=$2`, state, id); err != nil {
			t.Fatal(err)
		}
		if w := pricingUpdate(s, id, `,"image_price":0.09`); w.Code != 409 {
			t.Fatal(state, w.Code, w.Body.String())
		}
	}
	if _, err := s.portalDB.Exec(`UPDATE portal_exclusive_channels SET state='maintenance' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	f.loseUpdate = true
	if w := pricingUpdate(s, id, `,"image_price":0.08`); w.Code != 503 {
		t.Fatal(w.Code, w.Body.String())
	}
	c := pricingChannel(t, s, id)
	if c.PricingSynced || c.State != "error" || c.ImagePrice != .08 {
		t.Fatal(c)
	}
	if err := s.provisionExclusive(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	c = pricingChannel(t, s, id)
	if !c.PricingSynced || c.State != "maintenance" || f.updates != 1 {
		t.Fatal(c, f.updates)
	}
}

func TestExclusiveOccupiedEditAppliesAfterLeaseCloses(t *testing.T) {
	s, f, id := rateServer(t)
	if _, err := s.portalDB.Exec(`UPDATE portal_exclusive_channels SET state='active' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	var leaseID int64
	if err := s.portalDB.QueryRow(`INSERT INTO portal_exclusive_leases(channel_id,user_id,state,token_hash,token_secret,upstream_secret,expires_at) VALUES($1,7,'active','pending-edit','secret','upstream',CURRENT_TIMESTAMP+interval '1 day') RETURNING id`, id).Scan(&leaseID); err != nil {
		t.Fatal(err)
	}
	if w := pricingUpdate(s, id, `,"account_ids":[2],"rate_multiplier":1.5,"image_price":0.08`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	c := pricingChannel(t, s, id)
	if c.PendingConfig == nil || !sameRentalSelection(c.PendingConfig.AccountIDs, []int64{2}) || c.PendingConfig.RateMultiplier != 1.5 || c.PendingConfig.ImagePrice != .08 {
		t.Fatal(c)
	}
	if !sameRentalSelection(c.AccountIDs, []int64{1}) || c.RateMultiplier != 1.25 || len(f.accounts[1].GroupIDs) != 1 || len(f.accounts[2].GroupIDs) != 0 {
		t.Fatal(c, f.accounts)
	}
	if _, err := s.portalDB.Exec(`UPDATE portal_exclusive_leases SET state='closed',ended_at=CURRENT_TIMESTAMP WHERE id=$1`, leaseID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.portalDB.Exec(`UPDATE portal_exclusive_channels SET state='available' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	s.syncPendingExclusivePricing(context.Background())
	c = pricingChannel(t, s, id)
	if c.PendingConfig != nil || c.State != "available" || !sameRentalSelection(c.AccountIDs, []int64{2}) || c.RateMultiplier != 1.5 || c.ImagePrice != .08 || len(f.accounts[1].GroupIDs) != 0 || len(f.accounts[2].GroupIDs) != 1 {
		t.Fatal(c, f.accounts)
	}
}

func TestExclusivePricingLegacySyncAndClaimGuard(t *testing.T) {
	s, f, id := rateServer(t)
	s.core = &pricingUserCore{f}
	if _, err := s.portalDB.Exec(`UPDATE portal_exclusive_channels SET pricing_synced=false,state='active' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	s.syncPendingExclusivePricing(context.Background())
	if c := pricingChannel(t, s, id); c.PricingSynced || c.State != "active" {
		t.Fatal("changed occupied pricing", c)
	}
	if _, err := s.portalDB.Exec(`UPDATE portal_exclusive_channels SET state='unpublished' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	s.syncPendingExclusivePricing(context.Background())
	if c := pricingChannel(t, s, id); !c.PricingSynced || c.State != "unpublished" || c.PricingRevision != 2 {
		t.Fatal(c)
	}
	// Quote validation happens before issuing any user credential.
	if _, err := s.portalDB.Exec(`UPDATE portal_exclusive_channels SET state='available' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	for _, fields := range []string{``, `,"pricing_revision":1`} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(`{"duration":1440`+fields+`}`))
		r.Header.Set("Authorization", "Bearer admin")
		r.SetPathValue("channelID", strconv.FormatInt(id, 10))
		w := httptest.NewRecorder()
		s.exclusiveClaim(w, r)
		if w.Code != 409 || !strings.Contains(w.Body.String(), "价格已调整") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
