package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
)

// loadProductRegistry reads one canonical category registry from the shared
// extras cache. The envelope is intentionally exact: a source cannot smuggle a
// second catalogue beside the requested category or recover entries from disk.
func loadProductRegistry(ctx context.Context, cache *Cache, category string, refresh bool) ([]ProductEntry, SourceState, error) {
	if cache == nil || !validProductCategory(category) {
		return nil, SourceState{}, fmt.Errorf("invalid %s product registry", category)
	}
	rel := category + "/registry.json"
	raw, state, err := cache.Fetch(ctx, rel, refresh)
	if err != nil {
		return nil, state, err
	}
	var object map[string]json.RawMessage
	if err := decodeOneJSON(raw, &object); err != nil {
		return nil, state, fmt.Errorf("%s: %w", rel, err)
	}
	if object == nil || len(object) != 2 || object["schema"] == nil || object[category] == nil {
		return nil, state, fmt.Errorf("%s: invalid registry envelope", rel)
	}
	var schema int
	if err := json.Unmarshal(object["schema"], &schema); err != nil || schema != 1 {
		return nil, state, fmt.Errorf("%s: unsupported schema", rel)
	}
	var entries []ProductEntry
	if err := json.Unmarshal(object[category], &entries); err != nil || entries == nil {
		return nil, state, fmt.Errorf("%s: invalid %s collection", rel, category)
	}
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if err := validateProductEntry(category, entry); err != nil {
			return nil, state, err
		}
		if entry.Path != path.Join(category, entry.ID) {
			return nil, state, fmt.Errorf("%s/%s: non-canonical product path %q", category, entry.ID, entry.Path)
		}
		if _, duplicate := seen[entry.ID]; duplicate {
			return nil, state, fmt.Errorf("%s/%s: duplicate product id", category, entry.ID)
		}
		seen[entry.ID] = struct{}{}
	}
	return entries, state, nil
}

func productEntryItem(base, category string, entry ProductEntry) (Item, error) {
	item := Item{
		ID: entry.ID, Category: category, Name: entry.Name,
		Summary: entry.Summary, Description: entry.Description,
		Art:    resolveAsset(base, entry.Path, entry.Preview),
		Author: entry.Author, Version: entry.Version,
		Manifest: entry.Manifest, ManifestSHA256: entry.ManifestSHA256,
		Screenshots: resolveAssets(base, entry.Path, entry.Screenshots),
		Tags:        entry.Tags, Accent: entry.Accent, Surface: entry.Surface,
		Upstream: entry.Upstream, Discord: entry.Discord,
		DownloadPaused: entry.DownloadPaused, DownloadPauseReason: entry.DownloadPauseReason,
		RequiredWindowManager: entry.WindowManager,
	}
	if ok, running := productWindowManagerGate(entry.WindowManager); !ok {
		item.Unavailable = true
		item.UnavailableReason = entry.WindowManagerReason
		if item.UnavailableReason == "" {
			item.UnavailableReason = "Written for " + running + "."
		}
	}
	// A missing receipt means not installed. A corrupt or unreadable one is a
	// local problem, not a source outage: leave the product installable
	// (reinstalling rewrites the receipt) rather than erroring the provider,
	// which BuildCatalog would fold into a misleading store-wide "offline".
	if receipt, err := readReceipt(category, entry.ID); err == nil {
		item.Installed = true
		item.InstalledVersion = receipt.Version
		item.UpdateAvailable = receipt.Version != entry.Version
	}
	return item, nil
}

func findProductEntry(entries []ProductEntry, id string) (ProductEntry, error) {
	for _, entry := range entries {
		if entry.ID == id {
			return entry, nil
		}
	}
	return ProductEntry{}, fmt.Errorf("unknown product %q", id)
}

// assertProductInstallable refuses a registry-backed install or update for one of
// two causes: the source has paused the product's downloads, or the product names
// a window manager other than the running one. Either way it reads a fresh,
// authoritative registry so a stale provider cache cannot hide the refusal,
// checks the entry by id alone so a client request carries no bypass, and leaves
// the product listed and removable: only fetching new payload is blocked.
func assertProductInstallable(ctx context.Context, cache *Cache, category, id string) error {
	entries, _, err := loadProductRegistry(ctx, cache, category, true)
	if err != nil {
		return err
	}
	entry, err := findProductEntry(entries, id)
	if err != nil {
		return err
	}
	if ok, running := productWindowManagerGate(entry.WindowManager); !ok {
		if entry.WindowManagerReason != "" {
			return fmt.Errorf("%s/%s: written for %s, not %s: %s",
				category, id, entry.WindowManager, running, entry.WindowManagerReason)
		}
		return fmt.Errorf("%s/%s: written for %s, not %s",
			category, id, entry.WindowManager, running)
	}
	if !entry.DownloadPaused {
		return nil
	}
	if entry.DownloadPauseReason != "" {
		return fmt.Errorf("%s/%s: downloads are paused: %s", category, id, entry.DownloadPauseReason)
	}
	return fmt.Errorf("%s/%s: downloads are paused", category, id)
}
