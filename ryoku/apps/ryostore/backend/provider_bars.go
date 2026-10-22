// The bar-style provider keeps Sumi built into the shell and adapts optional
// styles from the canonical extras registry into receipt-owned Store products.
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

const barStyleTransactionKey = "ryoStoreBarStyleTransaction"
const barStyleScene = "Scene.qml"

type barStyleIndexRow struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Scene   string `json:"scene"`
	View    string `json:"view"`
}

type barProvider struct {
	cache       *Cache
	shellConfig string
}

var barStyleShellPatch = func(id string) error {
	return exec.Command("ryoku-shell", "barstyle", id).Run()
}

func newBarProvider(cache *Cache) barProvider {
	if cache == nil {
		cache = newCache()
	}
	return barProvider{
		cache:       cache,
		shellConfig: defaultShellConfigPath(),
	}
}

func (barProvider) Category() Category {
	return Category{
		ID:          "barstyles",
		Name:        "Bar styles",
		Group:       "wear",
		Description: "Complete bar compositions for the Ryoku shell.",
	}
}

func (p barProvider) Load(ctx context.Context, refresh bool) ([]Item, SourceState, error) {
	local, err := rebuildBarStyleIndex()
	if err != nil {
		return nil, SourceState{}, err
	}

	active := p.activeStyle()
	items := builtinBarStyles()
	for i := range items {
		items[i].Active = items[i].ID != "sumi" && items[i].ID == active
	}

	entries, state, registryErr := loadProductRegistry(ctx, p.cache, "barstyles", refresh)
	if registryErr != nil && !productRegistryUnavailable(registryErr) {
		return nil, state, registryErr
	}
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		item, err := productEntryItem(p.cache.base, "barstyles", entry)
		if err != nil {
			return nil, state, fmt.Errorf("barstyles/%s: installed state: %w", entry.ID, err)
		}
		item.Active = item.Installed && entry.ID == active
		if item.Metadata == nil {
			item.Metadata = map[string]any{}
		}
		item.Metadata["scene"] = barStyleScene
		items = append(items, item)
		seen[entry.ID] = true
	}
	for _, row := range local {
		if seen[row.ID] {
			continue
		}
		items = append(items, Item{
			ID: row.ID, Category: "barstyles", Name: row.ID,
			Version: row.Version, InstalledVersion: row.Version,
			Installed: true, Active: row.ID == active,
			Metadata: map[string]any{"scene": barStyleScene},
		})
	}
	if registryErr != nil {
		state.Offline = true
	}
	// A selection the shell cannot render falls back to the Sumi frame.
	fallback := 0
	for i := range items {
		if items[i].ID == "sumi" {
			fallback = i
			break
		}
	}
	for i := range items {
		if i != fallback && items[i].Active {
			items[fallback].Active = false
			return items, state, nil
		}
	}
	items[fallback].Active = true
	return items, state, nil
}

// builtinBarStyles are the styles this binary ships; their text lives here, so a
// catalogue snapshot built by an older binary is checked against it.
func builtinBarStyles() []Item {
	return []Item{{
		ID: "orpheus", Category: "barstyles", Name: "Orpheus",
		Summary:     "Liquid wave",
		Description: "A liquid top bar that hugs what it shows, ripples along its whole edge when music plays, and grows each group into a card.",
		Tags:        []string{"top", "liquid", "built-in"},
		Installed:   true,
		Metadata:    map[string]any{"scene": "Scene.qml", "core": true},
	}, {
		ID: "sumi", Category: "barstyles", Name: "Sumi",
		Summary:     "Ink spine",
		Description: "The built-in left rail: paper, ink, and a vertical working edge.",
		Tags:        []string{"rail", "vertical", "built-in"},
		Installed:   true,
		Metadata:    map[string]any{"scene": "", "core": true},
	}, {
		ID: "qsbar", Category: "barstyles", Name: "QS Bar",
		Summary:     "Hancore top bar",
		Description: "Hancore's Quickshell Rise top bar, ported to Ryoku.",
		Tags:        []string{"top", "horizontal", "built-in"},
		Installed:   true,
		Metadata:    map[string]any{"scene": "Scene.qml", "core": true},
	}, {
		ID: "kairos", Category: "barstyles", Name: "Kairos",
		Summary:     "Island clock",
		Description: "One floating island at the top of the screen: the clock that grows on hover.",
		Tags:        []string{"top", "island", "built-in"},
		Installed:   true,
		Metadata:    map[string]any{"scene": "Scene.qml", "core": true},
	}, {
		ID: "iris", Category: "barstyles", Name: "Shima",
		Summary:     "Based on iNiR by snowarch",
		Description: "An island on any screen edge that grows into whatever you clicked: morphing glass, bubbles, a dock, a studio and its own settings.",
		Tags:        []string{"island", "frame", "built-in"},
		Installed:   true,
		Metadata:    map[string]any{"scene": "Scene.qml", "core": true},
	}, {
		ID: "python", Category: "barstyles", Name: "Python",
		Summary:     "Based on Serpantinum by ilyamiro",
		Description: "Serpantinum's orientation-aware bar, liquid-glass stage, popups, quick actions, dock design, settings guide and desktop face library on Ryoku's shared services.",
		Tags:        []string{"top", "stage", "dock", "built-in"},
		Installed:   true,
		Metadata:    map[string]any{"scene": "Scene.qml", "core": true},
	}, {
		ID: "nomarchy", Category: "barstyles", Name: "Nomarchy",
		Summary:     "Based on Omarchy by 37signals",
		Description: "Omarchy's configurable bar, panels, menu, notifications and plugin host, adapted to Ryoku's window-manager seam.",
		Tags:        []string{"bar", "panels", "plugins", "built-in"},
		Installed:   true,
		Metadata:    map[string]any{"scene": "Scene.qml", "core": true},
	}, {
		ID: "caelestia", Category: "barstyles", Name: "Caelestia",
		Summary:     "Material bar",
		Description: "The Caelestia Material bar on the bottom edge with workspaces, a dock, the clock, status icons and power, drawn with the Ryoku shell's own services.",
		Tags:        []string{"bottom", "material", "built-in"},
		Installed:   true,
		Metadata:    map[string]any{"scene": "Scene.qml", "core": true},
	}}
}

// snapshotStaleBuiltins reports a snapshot whose built-in bar styles read
// differently from the ones this binary ships: the snapshot is only refreshed on
// demand, so an update that renames a built-in would otherwise never show.
func snapshotStaleBuiltins(data []byte) bool {
	var s struct {
		Items []Item `json:"items"`
	}
	if json.Unmarshal(data, &s) != nil {
		return false
	}
	shipped := make(map[string]Item)
	for _, it := range builtinBarStyles() {
		shipped[it.ID] = it
	}
	for _, it := range s.Items {
		want, ok := shipped[it.ID]
		if !ok || it.Category != "barstyles" {
			continue
		}
		if it.Name != want.Name || it.Summary != want.Summary || it.Description != want.Description {
			return true
		}
	}
	return false
}

func (p barProvider) Install(ctx context.Context, id string) error {
	if id == "sumi" || id == "qsbar" || id == "kairos" || id == "orpheus" || id == "iris" || id == "python" || id == "nomarchy" || id == "caelestia" {
		return fmt.Errorf("the built-in %s bar style is already installed", id)
	}
	entries, _, err := loadProductRegistry(ctx, p.cache, "barstyles", false)
	if err != nil {
		return err
	}
	entry, err := findProductEntry(entries, id)
	if err != nil {
		return err
	}
	return installProduct(ctx, p.cache, "barstyles", entry)
}

func (p barProvider) Remove(ctx context.Context, id string) error {
	if id == "sumi" || id == "qsbar" || id == "kairos" || id == "orpheus" || id == "iris" || id == "python" || id == "nomarchy" || id == "caelestia" {
		return fmt.Errorf("the built-in %s bar style is not removable", id)
	}
	return removeProduct(ctx, "barstyles", id)
}

func barStyleIndexPath() string {
	return filepath.Join(storeStateDir(), "barstyles.json")
}

func installedBarStyleRows() ([]barStyleIndexRow, error) {
	directory := filepath.Join(storeStateDir(), "barstyles")
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return []barStyleIndexRow{}, nil
	}
	if err != nil {
		return nil, err
	}
	rows := make([]barStyleIndexRow, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		// A broken barstyle receipt (bad name, unreadable, missing Scene.qml, or
		// drifted from disk) is quarantined: skipped so it can't error the
		// provider and report the store offline. It reads as installable again
		// and a reinstall heals it.
		if !productIDPattern.MatchString(id) {
			continue
		}
		receipt, err := readReceipt("barstyles", id)
		if err != nil {
			continue
		}
		dst, _, err := productDestination("barstyles", id)
		if err != nil {
			continue
		}
		if !receiptOwnsFile(receipt, barStyleScene) {
			continue
		}
		if err := verifyInstalledReceipt(dst, receipt); err != nil {
			continue
		}
		row := barStyleIndexRow{ID: id, Version: receipt.Version, Scene: barStyleScene}
		row.View = barStyleViews.path(id, receipt)
		rows = append(rows, row)
	}
	return rows, nil
}

func rebuildBarStyleIndex() ([]barStyleIndexRow, error) {
	unlock, err := lockTree(storeTransactionLockPath())
	if err != nil {
		return nil, err
	}
	defer unlock()
	if err := recoverStoreTransactions(); err != nil {
		return nil, err
	}
	return rebuildBarStyleIndexLocked()
}

func writeBarStyleIndexLocked() error {
	_, err := rebuildBarStyleIndexLocked()
	return err
}

func rebuildBarStyleIndexLocked() ([]barStyleIndexRow, error) {
	rows, err := installedBarStyleRows()
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		receipt, err := readReceipt("barstyles", row.ID)
		if err != nil {
			return nil, err
		}
		if err := barStyleViews.ensure(row.ID, row.View, receipt); err != nil {
			return nil, err
		}
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		return nil, err
	}
	if err := atomicWrite(barStyleIndexPath(), append(raw, '\n'), 0o600); err != nil {
		return nil, err
	}
	return rows, nil
}

func receiptOwnsFile(receipt Receipt, destination string) bool {
	for _, file := range receipt.Files {
		if file.Destination == destination {
			return true
		}
	}
	return false
}

func defaultShellConfigPath() string {
	return filepath.Join(configHome(), "ryoku", "shell.json")
}

func setBarStyleSelection(path, id string) error {
	if path == defaultShellConfigPath() && barStyleShellPatch(id) == nil {
		return nil
	}
	unlock, err := lockConfigFile(path)
	if err != nil {
		return err
	}
	defer unlock()
	return writeBarStyleSelection(path, id)
}

func writeBarStyleSelection(path, id string) error {
	state, err := readConfigState(path)
	if err != nil {
		return err
	}
	state["barStyle"] = id
	return writeConfigState(path, state)
}

// activeStyle is the style the shell renders: an absent or unreadable
// selection draws the shipped Orpheus bar, so the catalogue marks that one.
func (p barProvider) activeStyle() string {
	if id := readBarStyleSelection(p.shellConfig); id != "" {
		return id
	}
	return "orpheus"
}

func readBarStyleSelection(path string) string {
	return readConfigString(path, "barStyle")
}
