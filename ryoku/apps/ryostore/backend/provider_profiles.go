package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

const profilesCategory = "profiles"

type profileStatusFunc func(context.Context) map[string]bool

type profilesProvider struct {
	cache  *Cache
	status profileStatusFunc
}

func newProfilesProvider(cache *Cache) profilesProvider {
	return profilesProvider{cache: cache, status: activeProfiles}
}

func (profilesProvider) Category() Category {
	return Category{
		ID:          profilesCategory,
		Name:        "Profiles",
		Group:       "EXTEND",
		Description: "Install account profiles, then create or update their separate Linux accounts in Ryoku Hub.",
	}
}

func (p profilesProvider) Load(ctx context.Context, refresh bool) ([]Item, SourceState, error) {
	entries, state, err := loadProductRegistry(ctx, p.cache, profilesCategory, refresh)
	if err != nil {
		return nil, state, err
	}
	var active map[string]bool
	if p.status != nil {
		active = p.status(ctx)
	}
	items := make([]Item, 0, len(entries))
	for _, entry := range entries {
		if !entry.Official {
			continue
		}
		item, err := productEntryItem(p.cache.base, profilesCategory, entry)
		if err != nil {
			return nil, state, err
		}
		item.Active = active[entry.ID]
		items = append(items, item)
	}
	return items, state, nil
}

func (p profilesProvider) Install(ctx context.Context, id string) error {
	entries, _, err := loadProductRegistry(ctx, p.cache, profilesCategory, false)
	if err != nil {
		return err
	}
	entry, err := findProductEntry(entries, id)
	if err != nil {
		return err
	}
	if !entry.Official {
		return fmt.Errorf("unknown product %q", id)
	}
	return installProduct(ctx, p.cache, profilesCategory, entry)
}

func activeProfiles(ctx context.Context) map[string]bool {
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(probeCtx, "ryoku-host", "profile", "status").Output()
	if err != nil {
		return nil
	}
	var status struct {
		Accounts []struct {
			Profile string `json:"profile"`
		} `json:"accounts"`
	}
	if json.Unmarshal(out, &status) != nil {
		return nil
	}
	if len(status.Accounts) == 0 {
		return nil
	}
	active := make(map[string]bool, len(status.Accounts))
	for _, account := range status.Accounts {
		if productIDPattern.MatchString(account.Profile) {
			active[account.Profile] = true
		}
	}
	return active
}
