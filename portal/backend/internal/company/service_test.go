package company

import "testing"

func TestAllowsAdminAndManagedCompany(t *testing.T) {
	if !allows(Scope{IsAdmin: true}, 99) {
		t.Fatal("admin should access every company")
	}
	if !allows(Scope{CompanyIDs: []int64{7}}, 7) || allows(Scope{CompanyIDs: []int64{7}}, 8) {
		t.Fatal("company manager scope mismatch")
	}
}

func TestNormalizeCompany(t *testing.T) {
	name, status, err := normalize(" Acme ", "")
	if err != nil || name != "Acme" || status != "active" {
		t.Fatalf("normalize = %q %q %v", name, status, err)
	}
	if _, _, err := normalize("", "active"); err == nil {
		t.Fatal("empty company name should fail")
	}
	if _, _, err := normalize("Acme", "deleted"); err == nil {
		t.Fatal("invalid status should fail")
	}
}
