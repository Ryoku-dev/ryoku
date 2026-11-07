package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// shellItemProvider serves one of the two shell categories, subshells or
// barstyles, from the revamped catalogue. Subshells also list the ones Ryoku
// ships: installed, never removable.
type shellItemProvider struct {
	category string
	cache    *Cache
}

func newShellItemProvider(category string, cache *Cache) shellItemProvider {
	if cache == nil {
		cache = newRevampedCache()
	}
	return shellItemProvider{category: category, cache: cache}
}

func (p shellItemProvider) Category() Category {
	if p.category == "subshells" {
		return Category{
			ID: "subshells", Name: "Subshells", Group: "wear",
			Description: "Complete shells with their own bar, panels, quick settings and tools.",
		}
	}
	return Category{
		ID: "barstyles", Name: "Bar styles", Group: "wear",
		Description: "Bar designs for the Ryoku shell.",
	}
}

func (p shellItemProvider) Load(ctx context.Context, refresh bool) ([]Item, SourceState, error) {
	// A failed migration is retried on the next load; shells-require reports it.
	_ = migrateLegacyShellProducts(ctx, p.cache)
	active := readShellSelections().BarStyle
	items := []Item{}
	seen := map[string]bool{}
	if p.category == "subshells" {
		shipped, _, err := discoverShippedShells()
		if err != nil {
			return nil, SourceState{}, err
		}
		for _, shell := range shipped {
			items = append(items, shippedShellCatalogItem(shell, active))
			seen[shell.ID] = true
		}
	}

	entries, state, registryErr := loadProductRegistry(ctx, p.cache, p.category, refresh)
	if productRegistryAbsent(registryErr) {
		entries, registryErr = nil, nil
	}
	if registryErr != nil && !productRegistryUnavailable(registryErr) {
		return nil, state, registryErr
	}
	for _, entry := range entries {
		if seen[entry.ID] {
			continue
		}
		item, err := productEntryItem(p.cache.base, p.category, entry)
		if err != nil {
			return nil, state, fmt.Errorf("%s/%s: installed state: %w", p.category, entry.ID, err)
		}
		item.Provides = entry.Provides
		if manifest, err := loadProductManifestFrom(ctx, p.cache, p.category, entry, refresh); err == nil {
			provides := shellProvidesSummary(manifest.Shell.Provides)
			settings := manifest.Shell.Settings
			item.Provides, item.Settings = &provides, &settings
		}
		item.Active = item.Installed && entry.ID == active
		items = append(items, item)
		seen[entry.ID] = true
	}

	// Installed items the source no longer lists stay visible and removable.
	installed, _, err := installedShellItems()
	if err != nil {
		return nil, state, err
	}
	for _, shell := range installed {
		if shell.Category != p.category || seen[shell.ID] {
			continue
		}
		provides := shellProvidesSummary(shell.Shell.Provides)
		settings := shell.Shell.Settings
		items = append(items, Item{
			ID: shell.ID, Category: shell.Category, Name: shell.Name,
			Summary: shell.Summary, Version: shell.Version, InstalledVersion: shell.Version,
			Installed: true, Active: shell.ID == active, Provides: &provides, Settings: &settings,
		})
	}
	if registryErr != nil {
		state.Offline = true
	}
	return items, state, nil
}

// shippedShellCatalogItem presents a shipped subshell from its item.toml, with
// its preview read straight from the shell config root.
func shippedShellCatalogItem(shell shellInventoryItem, active string) Item {
	provides := shellProvidesSummary(shell.Shell.Provides)
	settings := shell.Shell.Settings
	item := Item{
		ID: shell.ID, Category: "subshells", Name: shell.Name, Summary: shell.Summary,
		Version: shell.Version, InstalledVersion: shell.Version, Installed: true,
		Active: shell.ID == active, Provides: &provides, Settings: &settings,
		// core marks an item that ships with Ryoku: always installed, never removable.
		Metadata: map[string]any{"core": true},
	}
	if meta := shell.Metadata; meta != nil {
		item.Description = meta.Description
		item.Author = strings.Join(meta.Authors, ", ")
		item.Tags = nonNilStrings(meta.Tags)
		item.Accent, item.Surface = meta.Accent, meta.Surface
		item.Upstream = meta.Upstream
		item.License = meta.License
		root := shellItemFilesystemRoot(shell)
		if preview := filepath.Join(root, filepath.FromSlash(meta.Preview)); fileExists(preview) {
			item.Art = "file://" + preview
		}
		for _, screenshot := range meta.Screenshots {
			if file := filepath.Join(root, filepath.FromSlash(screenshot)); fileExists(file) {
				item.Screenshots = append(item.Screenshots, "file://"+file)
			}
		}
	}
	return item
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func (p shellItemProvider) Install(ctx context.Context, id string) error {
	if p.category == "subshells" && shippedShellIDs()[id] {
		return fmt.Errorf("the %s subshell ships with Ryoku", id)
	}
	entries, _, err := loadProductRegistry(ctx, p.cache, p.category, false)
	if err != nil {
		return err
	}
	entry, err := findProductEntry(entries, id)
	if err != nil {
		return err
	}
	if err := installProduct(ctx, p.cache, p.category, entry); err != nil {
		return err
	}
	if _, err := runShellsSync(false); err != nil {
		return fmt.Errorf("%s/%s is installed, but the shells index was not rebuilt: %w", p.category, id, err)
	}
	return nil
}

func (p shellItemProvider) Remove(ctx context.Context, id string) error {
	if p.category == "subshells" && shippedShellIDs()[id] {
		return fmt.Errorf("the %s subshell ships with Ryoku and cannot be removed", id)
	}
	if err := removeProduct(ctx, p.category, id); err != nil {
		return err
	}
	if err := removeShellItemArtifacts(p.category, id); err != nil {
		return err
	}
	if _, err := runShellsSync(false); err != nil {
		return fmt.Errorf("%s/%s is removed, but the shells index was not rebuilt: %w", p.category, id, err)
	}
	return nil
}

func shellProvidesSummary(provides ShellProvides) ProductProvides {
	summary := ProductProvides{Dock: []string{}, Launcher: []string{}, DesktopWidgets: []string{}}
	for _, item := range provides.Dock {
		summary.Dock = append(summary.Dock, item.ID)
	}
	for _, item := range provides.Launcher {
		summary.Launcher = append(summary.Launcher, item.ID)
	}
	for _, item := range provides.DesktopWidgets {
		summary.DesktopWidgets = append(summary.DesktopWidgets, item.ID)
	}
	return summary
}
