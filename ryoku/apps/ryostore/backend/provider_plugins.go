// The plugins provider adapts the canonical external product registry into the
// Store contract. Browse metadata comes from the registry; receipt ownership,
// installed version, and update state come from the common product transaction
// layer. Placement remains user state in plugins.json: installing honours the
// product's own defaults.desktopWidget.autoEnable declaration, otherwise the
// plugin stays disabled until the user places it.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type pluginProvider struct {
	cache *Cache
}

func (pluginProvider) Category() Category {
	return Category{
		ID:          "plugins",
		Name:        "Plugins",
		Group:       "EXTEND",
		Description: "Shell plugins that mount as widgets or frame popouts.",
	}
}

func (p pluginProvider) Load(ctx context.Context, refresh bool) ([]Item, SourceState, error) {
	if _, err := rebuildPluginIndex(); err != nil {
		return nil, SourceState{}, fmt.Errorf("plugins: rebuild runtime index: %w", err)
	}
	entries, state, err := loadProductRegistry(ctx, p.cache, "plugins", refresh)
	if err != nil {
		return nil, state, err
	}
	placements := readPluginPlacements()
	items := make([]Item, 0, len(entries))
	for _, entry := range entries {
		item, err := productEntryItem(p.cache.base, "plugins", entry)
		if err != nil {
			return nil, state, fmt.Errorf("plugins/%s: installed state: %w", entry.ID, err)
		}
		placement, placed := placements[entry.ID]
		item.Enabled = item.Installed && placed && placement.Enabled
		metadata := map[string]any{}
		if entry.Official {
			metadata["official"] = true
		} else {
			metadata["community"] = true
		}
		if len(entry.Hosts) > 0 {
			metadata["hosts"] = entry.Hosts
		}
		if entry.Icon != "" {
			metadata["icon"] = entry.Icon
		}
		if item.InstalledVersion != "" {
			metadata["installedVersion"] = item.InstalledVersion
		}
		if placed && placement.Host != "" {
			metadata["placement"] = placement.Host
		}
		if len(metadata) > 0 {
			item.Metadata = metadata
		}
		items = append(items, item)
	}
	return items, state, nil
}

func snapshotPluginPlacement(id string) (json.RawMessage, bool, bool, error) {
	path := filepath.Join(configHome(), "ryoku", "plugins.json")
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, false, nil
	}
	if err != nil {
		return nil, false, false, err
	}
	var placements map[string]json.RawMessage
	if err := decodeOneJSON(raw, &placements); err != nil {
		return nil, false, true, fmt.Errorf("plugins.json: %w", err)
	}
	entry, present := placements[id]
	return append(json.RawMessage(nil), entry...), present, true, nil
}

func setPluginPlacementEnabled(id string, enabled bool) error {
	if err := exec.Command("ryoku-plugins-place", id, "enabled", fmt.Sprintf("%t", enabled)).Run(); err != nil {
		return fmt.Errorf("set fresh plugin placement: %w", err)
	}
	return nil
}

// pluginAutoEnable reports whether a freshly installed plugin ships a manifest
// that asks to be placed on install. Only a desktop widget may auto-enable, so a
// missing or unreadable manifest, or one without the desktopWidget host or with
// the flag unset, leaves the plugin disabled.
func pluginAutoEnable(installDir string) bool {
	raw, err := os.ReadFile(filepath.Join(installDir, "manifest.json"))
	if err != nil {
		return false
	}
	var manifest struct {
		Hosts    []string `json:"hosts"`
		Defaults struct {
			DesktopWidget struct {
				AutoEnable bool `json:"autoEnable"`
			} `json:"desktopWidget"`
		} `json:"defaults"`
	}
	if decodeOneJSON(raw, &manifest) != nil {
		return false
	}
	if !manifest.Defaults.DesktopWidget.AutoEnable {
		return false
	}
	for _, host := range manifest.Hosts {
		if host == "desktopWidget" {
			return true
		}
	}
	return false
}

func restorePluginPlacement(id string, entry json.RawMessage, present, filePresent bool) error {
	value := "null"
	if present {
		value = string(entry)
	}
	if err := exec.Command(
		"ryoku-plugins-place", id, "restore", value, fmt.Sprintf("%t", filePresent),
	).Run(); err != nil {
		return fmt.Errorf("restore plugin placement: %w", err)
	}
	return nil
}

func (p pluginProvider) Install(ctx context.Context, id string) error {
	entries, _, err := loadProductRegistry(ctx, p.cache, "plugins", false)
	if err != nil {
		return err
	}
	entry, err := findProductEntry(entries, id)
	if err != nil {
		return err
	}
	return installProduct(ctx, p.cache, "plugins", entry)
}

func configHome() string {
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		return base
	}
	return filepath.Join(os.Getenv("HOME"), ".config")
}

type pluginPlacement struct {
	Enabled bool   `json:"enabled"`
	Host    string `json:"host"`
}

func readPluginPlacements() map[string]pluginPlacement {
	raw, err := os.ReadFile(filepath.Join(configHome(), "ryoku", "plugins.json"))
	if err != nil {
		return nil
	}
	var placements map[string]pluginPlacement
	if decodeOneJSON(raw, &placements) != nil {
		return nil
	}
	return placements
}

// resolveAsset turns a registry-relative asset path into an absolute source URL,
// passing an already-absolute HTTP URL through unchanged.
func resolveAsset(base, path, asset string) string {
	if asset == "" {
		return ""
	}
	if strings.HasPrefix(asset, "http://") || strings.HasPrefix(asset, "https://") {
		return asset
	}
	return base + "/" + path + "/" + strings.TrimLeft(asset, "/")
}

func resolveAssets(base, path string, assets []string) []string {
	if len(assets) == 0 {
		return nil
	}
	resolved := make([]string, len(assets))
	for index, asset := range assets {
		resolved[index] = resolveAsset(base, path, asset)
	}
	return resolved
}
