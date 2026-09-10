package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
	"github.com/jqcode/portal/backend/internal/storage"
	"github.com/jqcode/portal/backend/internal/usernamereconcile"
)

func main() {
	apply := flag.Bool("apply", false, "apply conflict-free imports and orphan removals")
	flag.Parse()
	if err := run(*apply); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(apply bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	adapter, err := coreclient.NewHTTPAdapter(envOrDefault("CORE_BASE_URL", "http://127.0.0.1:18080"), &http.Client{Timeout: 10 * time.Second})
	if err != nil {
		return err
	}
	adminKey := os.Getenv("CORE_ADMIN_API_KEY")
	databaseURL := os.Getenv("PORTAL_DATABASE_URL")
	if adminKey == "" || databaseURL == "" {
		return fmt.Errorf("CORE_ADMIN_API_KEY and PORTAL_DATABASE_URL are required")
	}
	adapter = adapter.WithAdminAPIKey(adminKey)

	var coreUsers []coreclient.User
	for page := 1; ; page++ {
		result, err := adapter.AdminUsers(ctx, page, 100)
		if err != nil {
			return err
		}
		coreUsers = append(coreUsers, result.Items...)
		if page >= result.Pages {
			break
		}
	}

	db, err := storage.Open(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	store := storage.NewUsernameStore(db)
	registry, err := store.ListBound(ctx)
	if err != nil {
		return err
	}
	plan := usernamereconcile.Build(coreUsers, registry)
	if apply {
		if len(plan.Conflicts) > 0 {
			return fmt.Errorf("refusing to apply reconciliation with %d conflicts", len(plan.Conflicts))
		}
		if err := store.ApplyReconciliation(ctx, plan.Imports, plan.Orphans); err != nil {
			return err
		}
	}

	output := struct {
		Applied bool `json:"applied"`
		usernamereconcile.Plan
	}{Applied: apply, Plan: plan}
	return json.NewEncoder(os.Stdout).Encode(output)
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
