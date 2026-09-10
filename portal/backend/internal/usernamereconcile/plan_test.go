package usernamereconcile

import (
	"testing"

	"github.com/jqcode/portal/backend/internal/coreclient"
	"github.com/jqcode/portal/backend/internal/storage"
)

func TestBuildImportsMatchesAndOrphans(t *testing.T) {
	plan := Build(
		[]coreclient.User{{ID: 1, Username: "Admin"}, {ID: 2, Username: "Alice"}},
		[]storage.BoundUsername{{Key: "admin", Username: "Admin", CoreUserID: 1}, {Key: "gone", Username: "Gone", CoreUserID: 9}},
	)
	if plan.Matched != 1 || len(plan.Imports) != 1 || plan.Imports[0].Key != "alice" || len(plan.Orphans) != 1 {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestBuildReportsConflictsWithoutImports(t *testing.T) {
	plan := Build(
		[]coreclient.User{{ID: 1, Username: "Alice"}, {ID: 2, Username: "ALICE"}, {ID: 3, Username: "Bob"}},
		[]storage.BoundUsername{{Key: "bob", Username: "Bob", CoreUserID: 4}, {Key: "old-bob", Username: "Old-Bob", CoreUserID: 3}},
	)
	if len(plan.Imports) != 0 || len(plan.Conflicts) != 3 {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestBuildReportsInvalidCoreUsername(t *testing.T) {
	plan := Build([]coreclient.User{{ID: 1, Username: ""}}, nil)
	if len(plan.Invalid) != 1 || len(plan.Imports) != 0 {
		t.Fatalf("plan = %+v", plan)
	}
}
