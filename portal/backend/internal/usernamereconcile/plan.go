package usernamereconcile

import (
	"sort"

	"github.com/jqcode/portal/backend/internal/coreclient"
	"github.com/jqcode/portal/backend/internal/storage"
	"github.com/jqcode/portal/backend/internal/username"
)

type Conflict struct {
	Kind       string `json:"kind"`
	Username   string `json:"username"`
	CoreUserID int64  `json:"core_user_id"`
}

type Invalid struct {
	CoreUserID int64  `json:"core_user_id"`
	Username   string `json:"username"`
}

type Plan struct {
	Matched   int                     `json:"matched"`
	Imports   []storage.BoundUsername `json:"imports"`
	Orphans   []storage.BoundUsername `json:"orphans"`
	Conflicts []Conflict              `json:"conflicts"`
	Invalid   []Invalid               `json:"invalid"`
}

func Build(coreUsers []coreclient.User, registry []storage.BoundUsername) Plan {
	plan := Plan{
		Imports:   []storage.BoundUsername{},
		Orphans:   []storage.BoundUsername{},
		Conflicts: []Conflict{},
		Invalid:   []Invalid{},
	}
	coreIDs := make(map[int64]struct{}, len(coreUsers))
	registryByKey := make(map[string]storage.BoundUsername, len(registry))
	registryByID := make(map[int64]storage.BoundUsername, len(registry))
	for _, item := range registry {
		registryByKey[item.Key] = item
		registryByID[item.CoreUserID] = item
	}

	type candidate struct {
		user coreclient.User
		key  string
	}
	candidates := make([]candidate, 0, len(coreUsers))
	coreByKey := make(map[string][]coreclient.User)
	for _, user := range coreUsers {
		coreIDs[user.ID] = struct{}{}
		display, key, err := username.Normalize(user.Username)
		if err != nil {
			plan.Invalid = append(plan.Invalid, Invalid{CoreUserID: user.ID, Username: user.Username})
			continue
		}
		user.Username = display
		coreByKey[key] = append(coreByKey[key], user)
		candidates = append(candidates, candidate{user: user, key: key})
	}

	for _, item := range candidates {
		if len(coreByKey[item.key]) > 1 {
			plan.Conflicts = append(plan.Conflicts, Conflict{Kind: "duplicate_core_username", Username: item.user.Username, CoreUserID: item.user.ID})
			continue
		}
		byKey, keyExists := registryByKey[item.key]
		byID, idExists := registryByID[item.user.ID]
		switch {
		case keyExists && byKey.CoreUserID != item.user.ID:
			plan.Conflicts = append(plan.Conflicts, Conflict{Kind: "username_owned_by_other_core_user", Username: item.user.Username, CoreUserID: item.user.ID})
		case idExists && byID.Key != item.key:
			plan.Conflicts = append(plan.Conflicts, Conflict{Kind: "core_user_mapped_to_other_username", Username: item.user.Username, CoreUserID: item.user.ID})
		case keyExists && idExists:
			plan.Matched++
		default:
			plan.Imports = append(plan.Imports, storage.BoundUsername{Key: item.key, Username: item.user.Username, CoreUserID: item.user.ID})
		}
	}

	for _, item := range registry {
		if _, exists := coreIDs[item.CoreUserID]; !exists {
			plan.Orphans = append(plan.Orphans, item)
		}
	}
	sort.Slice(plan.Imports, func(i, j int) bool { return plan.Imports[i].Key < plan.Imports[j].Key })
	sort.Slice(plan.Orphans, func(i, j int) bool { return plan.Orphans[i].Key < plan.Orphans[j].Key })
	return plan
}
