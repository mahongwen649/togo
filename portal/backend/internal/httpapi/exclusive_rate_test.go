package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

type exclusiveRateCore struct {
	*rentalFakeCore
	loseUpdate bool
	updates    int
}

func (f *exclusiveRateCore) AdminMarketRequest(ctx context.Context, method, path string, payload any, key string) (coreclient.Response, error) {
	if method == "PUT" && strings.HasPrefix(path, "/api/v1/admin/groups/") {
		var id int64
		fmt.Sscanf(path, "/api/v1/admin/groups/%d", &id)
		for i := range f.groups {
			if f.groups[i].ID == id {
				f.updates++
				raw, _ := json.Marshal(payload)
				json.Unmarshal(raw, &f.groups[i])
				if f.loseUpdate {
					f.loseUpdate = false
					return coreclient.Response{}, errors.New("lost update response")
				}
				return rentalResponse(f.groups[i])
			}
		}
	}
	return f.rentalFakeCore.AdminMarketRequest(ctx, method, path, payload, key)
}
func rateServer(t *testing.T) (*Server, *exclusiveRateCore, int64) {
	t.Helper()
	s := fakeRentalServer(rentalTestDB(t))
	s.exclusive = ExclusiveConfig{Enabled: true}
	f := &exclusiveRateCore{rentalFakeCore: s.core.(*rentalFakeCore)}
	s.core = f
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"rated","account_ids":[1],"durations":[1440],"cooldown_seconds":0,"rate_multiplier":1.25}`))
	r.Header.Set("Authorization", "Bearer admin")
	w := httptest.NewRecorder()
	s.exclusivePublishChannel(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var id int64
	if err := s.portalDB.QueryRow(`SELECT id FROM portal_exclusive_channels`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return s, f, id
}
func rateUpdate(s *Server, id int64, rate string) *httptest.ResponseRecorder {
	body := `{"name":"rated","durations":[1440],"cooldown_seconds":0`
	if rate != "" {
		body += `,"rate_multiplier":` + rate
	}
	body += `}`
	r := httptest.NewRequest("PUT", "/", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer admin")
	r.SetPathValue("channelID", strconv.FormatInt(id, 10))
	w := httptest.NewRecorder()
	s.exclusiveUpdateChannel(w, r)
	return w
}
func TestExclusiveRateCreateAndUpdate(t *testing.T) {
	for _, state := range []string{"available", "maintenance", "unpublished"} {
		t.Run(state, func(t *testing.T) {
			s, f, id := rateServer(t)
			if f.groups[0].RateMultiplier != 1.25 {
				t.Fatal("rate was not sent to Core", f.groups)
			}
			if _, err := s.portalDB.Exec(`UPDATE portal_exclusive_channels SET state=$1 WHERE id=$2`, state, id); err != nil {
				t.Fatal(err)
			}
			if w := rateUpdate(s, id, "2.5"); w.Code != 200 {
				t.Fatal(w.Code, w.Body.String())
			}
			c, err := scanExclusiveChannel(s.portalDB.QueryRow(`SELECT `+exclusiveChannelColumns+` FROM portal_exclusive_channels WHERE id=$1`, id))
			if err != nil || c.RateMultiplier != 2.5 || c.State != state || c.RateResumeState != nil || f.groups[0].RateMultiplier != 2.5 {
				t.Fatal(c, f.groups, err)
			}
			if w := rateUpdate(s, id, ""); w.Code != 200 {
				t.Fatal(w.Code)
			}
			if f.updates != 1 {
				t.Fatal("unchanged rate updated Core", f.updates)
			}
		})
	}
}
func TestExclusiveRateUpdateFailureRetry(t *testing.T) {
	s, f, id := rateServer(t)
	f.loseUpdate = true
	if _, err := s.portalDB.Exec(`UPDATE portal_exclusive_channels SET state='unpublished' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if w := rateUpdate(s, id, "3"); w.Code != 503 {
		t.Fatal(w.Code, w.Body.String())
	}
	c, err := scanExclusiveChannel(s.portalDB.QueryRow(`SELECT `+exclusiveChannelColumns+` FROM portal_exclusive_channels WHERE id=$1`, id))
	if err != nil || c.State != "error" || c.RateMultiplier != 3 || c.RateResumeState == nil || *c.RateResumeState != "unpublished" {
		t.Fatal(c, err)
	}
	if err = s.provisionExclusive(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if err = s.portalDB.QueryRow(`SELECT state FROM portal_exclusive_channels WHERE id=$1`, id).Scan(&c.State); err != nil || c.State != "unpublished" || f.updates != 1 {
		t.Fatal(c.State, f.updates, err)
	}
}
func TestExclusiveRateValidationAndOccupied(t *testing.T) {
	s, f, id := rateServer(t)
	for _, rate := range []string{"0", "-1", "1e999", "\"bad\""} {
		if w := rateUpdate(s, id, rate); w.Code != 400 {
			t.Fatal(rate, w.Code)
		}
	}
	for _, state := range []string{"active", "releasing", "cooling"} {
		if _, err := s.portalDB.Exec(`UPDATE portal_exclusive_channels SET state=$1 WHERE id=$2`, state, id); err != nil {
			t.Fatal(err)
		}
		if w := rateUpdate(s, id, "2"); w.Code != 200 {
			t.Fatal(state, w.Code, w.Body.String())
		}
	}
	for _, state := range []string{"provisioning", "error"} {
		if _, err := s.portalDB.Exec(`UPDATE portal_exclusive_channels SET state=$1 WHERE id=$2`, state, id); err != nil {
			t.Fatal(err)
		}
		if w := rateUpdate(s, id, "3"); w.Code != 409 {
			t.Fatal(state, w.Code, w.Body.String())
		}
	}
	if f.updates != 0 {
		t.Fatal("blocked rate reached Core")
	}
	for _, rate := range []float64{math.NaN(), math.Inf(1), 0, -1} {
		if validateExclusiveConfig(exclusivePublish{RateMultiplier: &rate}, true) == nil {
			t.Fatal(rate)
		}
	}
}
func TestExclusiveClaimRejectsChangedRate(t *testing.T) {
	s, _, db, id, lease := pageFixture(t)
	if _, err := db.Exec(`UPDATE portal_exclusive_leases SET state='closed' WHERE id=$1`, lease); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE portal_exclusive_channels SET state='available',core_group_id=123,rate_multiplier=2 WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"duration":1440,"rate_multiplier":1}`))
	r.Header.Set("Authorization", "Bearer holder")
	r.SetPathValue("channelID", strconv.FormatInt(id, 10))
	w := httptest.NewRecorder()
	s.exclusiveClaim(w, r)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "倍率已调整") {
		t.Fatal(w.Code, w.Body.String())
	}
}
