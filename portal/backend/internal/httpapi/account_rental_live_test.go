package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

// Opt-in local contract smoke test. Test resources have generated names and
// invalid upstream credentials; no gateway request is sent.
func TestRentalAgainstLocalCore(t *testing.T) {
	base, key := os.Getenv("PORTAL_RENTAL_LIVE_CORE_URL"), os.Getenv("PORTAL_RENTAL_LIVE_CORE_KEY")
	if base == "" || key == "" {
		t.Skip("local Core smoke test not configured")
	}
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	if u.Hostname() != "host.docker.internal" && u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" {
		t.Fatal("live smoke test is restricted to local Core")
	}
	db := rentalTestDB(t)
	adapter, err := coreclient.NewHTTPAdapter(base, &http.Client{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	adapter = adapter.WithAdminAPIKey(key)
	s := &Server{portalDB: db, core: adapter, logger: testLogger()}
	var token [12]byte
	if _, err := rand.Read(token[:]); err != nil {
		t.Fatal(err)
	}
	name := "portaltest" + hex.EncodeToString(token[:])
	ctx := context.Background()
	var user coreclient.User
	if err := s.rentalCoreRequest(ctx, "POST", "/api/v1/admin/users", map[string]any{"email": name + "@example.invalid", "username": name, "password": hex.EncodeToString(token[:]) + "Test9!", "role": "user", "balance": 0, "concurrency": 2}, &user, ""); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := adapter.AdminDeleteUser(ctx, user.ID); err != nil {
			t.Error(err)
		}
	})
	var account rentalAccount
	if err := s.rentalCoreRequest(ctx, "POST", "/api/v1/admin/accounts", map[string]any{"name": name, "platform": "openai", "type": "apikey", "credentials": map[string]string{"api_key": "sk-invalid-portal-local-test"}, "concurrency": 1, "priority": 1, "group_ids": []int64{}}, &account, name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := adapter.AdminMarketRequest(ctx, "DELETE", fmt.Sprintf("/api/v1/admin/accounts/%d", account.ID), nil, ""); err != nil {
			t.Error(err)
		}
	})
	orderID := seedRental(t, db, user.ID, 1)
	t.Cleanup(func() {
		var group int64
		if db.QueryRow(`SELECT core_group_id FROM portal_account_rental_groups WHERE user_id=$1`, user.ID).Scan(&group) == nil {
			if _, err := adapter.AdminMarketRequest(ctx, "DELETE", fmt.Sprintf("/api/v1/admin/groups/%d", group), nil, ""); err != nil {
				t.Error(err)
			}
		}
	})
	o, err := s.provisionRental(ctx, orderID, []int64{account.ID})
	if err != nil {
		t.Fatal(err)
	}
	if o.Status != "completed" || !o.GroupID.Valid || !o.ExpiresAt.Valid {
		t.Fatalf("incomplete live fulfillment: %+v", o)
	}
	if _, err := s.provisionRental(ctx, orderID, []int64{account.ID}); err != nil {
		t.Fatal(err)
	}
	profile, err := adapter.AdminUser(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Balance != 0 {
		t.Fatal("rental changed Core relay balance")
	}
}
