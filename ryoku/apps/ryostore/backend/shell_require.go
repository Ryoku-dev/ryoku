package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// defaultDockDesign is the dock the shell draws when dock.design names nothing
// it can serve (services/Dock.qml).
const defaultDockDesign = "ryoku"

// shellRequirement is one selection that names something neither shipped nor
// installed. Category and ID name the catalogue item that provides it, empty
// when no item does; Available says that item can be installed here now.
type shellRequirement struct {
	Selection string `json:"selection"`
	Value     string `json:"value"`
	Category  string `json:"category"`
	ID        string `json:"id"`
	Available bool   `json:"available"`
}

type shellsRequireResult struct {
	Missing   []shellRequirement `json:"missing"`
	Installed []shellRequirement `json:"installed"`
	FellBack  []shellRequirement `json:"fellBack"`
}

// shellOffer is one catalogue item that may serve a selection.
type shellOffer struct {
	Category    string
	Entry       ProductEntry
	Installable bool
}

// shellCatalogue is what the catalogue offers, indexed by the selection value
// each item serves. Unreachable sources leave it empty.
type shellCatalogue struct {
	products map[string]shellOffer
	byID     map[string]shellOffer
	dock     map[string]shellOffer
	launcher map[string]shellOffer
	widgets  map[string]shellOffer
}

func loadShellCatalogue(ctx context.Context, cache *Cache) shellCatalogue {
	catalogue := shellCatalogue{
		products: map[string]shellOffer{}, byID: map[string]shellOffer{}, dock: map[string]shellOffer{},
		launcher: map[string]shellOffer{}, widgets: map[string]shellOffer{},
	}
	// Subshells last, so an id listed in both categories resolves to the subshell.
	for _, category := range []string{"barstyles", "subshells"} {
		entries, _, err := loadProductRegistry(ctx, cache, category, false)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			offer := shellOffer{Category: category, Entry: entry, Installable: shellEntryInstallable(entry)}
			catalogue.products[category+"/"+entry.ID] = offer
			catalogue.byID[entry.ID] = offer
			if entry.Provides == nil {
				continue
			}
			for _, id := range entry.Provides.Dock {
				catalogue.dock[id] = offer
			}
			for _, id := range entry.Provides.Launcher {
				catalogue.launcher[id] = offer
			}
			for _, id := range entry.Provides.DesktopWidgets {
				catalogue.widgets[id] = offer
			}
		}
	}
	return catalogue
}

// shellEntryInstallable applies the same gates the install transaction does:
// paused downloads, the window manager and the edition.
func shellEntryInstallable(entry ProductEntry) bool {
	if entry.DownloadPaused {
		return false
	}
	if ok, _ := productWindowManagerGate(entry.WindowManager); !ok {
		return false
	}
	ok, _ := productPackageManagerGate(entry.PackageManagers)
	return ok
}

// servedShellSelections is everything this machine can serve right now.
type servedShellSelections struct {
	items    map[string]bool
	dock     map[string]bool
	launcher map[string]bool
	widgets  map[string]bool
}

func loadServedShellSelections() (servedShellSelections, error) {
	served := servedShellSelections{
		items: map[string]bool{}, dock: map[string]bool{}, launcher: map[string]bool{"": true},
		widgets: map[string]bool{},
	}
	for id := range dockProducts.builtins {
		served.dock[id] = true
	}
	for id := range launcherProducts.builtins {
		served.launcher[id] = true
	}
	items, _, err := shellInventory()
	if err != nil {
		return served, err
	}
	for _, item := range items {
		served.items[item.ID] = true
		for _, contribution := range item.Shell.Provides.Dock {
			served.dock[contribution.ID] = true
		}
		for _, contribution := range item.Shell.Provides.Launcher {
			served.launcher[contribution.ID] = true
		}
		for _, contribution := range item.Shell.Provides.DesktopWidgets {
			served.widgets[contribution.ID] = true
		}
	}
	return served, nil
}

// findMissingShellSelections lists every selection the machine cannot serve.
// A desktop widget set is missing when a catalogue item provides it, nothing
// installed does, and one of its widgets is switched on in widgets.json.
func findMissingShellSelections(ctx context.Context, cache *Cache, catalogue shellCatalogue) ([]shellRequirement, error) {
	served, err := loadServedShellSelections()
	if err != nil {
		return nil, err
	}
	selections := readShellSelections()
	missing := []shellRequirement{}
	add := func(selection, value string, offer shellOffer, offered bool) {
		requirement := shellRequirement{Selection: selection, Value: value}
		if offered {
			requirement.Category, requirement.ID = offer.Category, offer.Entry.ID
			requirement.Available = offer.Installable
		}
		missing = append(missing, requirement)
	}
	if !served.items[selections.BarStyle] {
		offer, offered := catalogue.byID[selections.BarStyle]
		add("barStyle", selections.BarStyle, offer, offered)
	}
	if selections.Dock != "" && !served.dock[selections.Dock] && !productInstalled("docks", selections.Dock) {
		offer, offered := catalogue.dock[selections.Dock]
		add("dock.design", selections.Dock, offer, offered)
	}
	if !served.launcher[selections.Launcher] && !productInstalled("launchers", selections.Launcher) {
		offer, offered := catalogue.launcher[selections.Launcher]
		add("launcher.variant", selections.Launcher, offer, offered)
	}
	setIDs := make([]string, 0, len(catalogue.widgets))
	for setID := range catalogue.widgets {
		setIDs = append(setIDs, setID)
	}
	sort.Strings(setIDs)
	for _, setID := range setIDs {
		if served.widgets[setID] {
			continue
		}
		offer := catalogue.widgets[setID]
		contribution, err := offeredWidgetSet(ctx, cache, offer, setID)
		if err != nil || !widgetSetEnabled(selections.Widgets, contribution) {
			continue
		}
		add("desktopWidgets", setID, offer, true)
	}
	return missing, nil
}

func productInstalled(category, id string) bool {
	if !productIDPattern.MatchString(id) {
		return false
	}
	_, err := readReceipt(category, id)
	return err == nil
}

// offeredWidgetSet reads one desktop widget set, with its roster, from the
// offering item's manifest.
func offeredWidgetSet(ctx context.Context, cache *Cache, offer shellOffer, setID string) (ShellDesktopWidgetContribution, error) {
	manifest, err := loadProductManifestFrom(ctx, cache, offer.Category, offer.Entry, false)
	if err != nil {
		return ShellDesktopWidgetContribution{}, err
	}
	for _, contribution := range manifest.Shell.Provides.DesktopWidgets {
		if contribution.ID == setID {
			return contribution, nil
		}
	}
	return ShellDesktopWidgetContribution{}, fmt.Errorf("%s/%s does not provide desktop widgets %s", offer.Category, offer.Entry.ID, setID)
}

// runShellsRequire is `ryostore internal shells-require`. install adds the
// missing items the catalogue offers, as the calling user; fallback then
// resets every selection that still cannot be served.
func runShellsRequire(install, fallback bool) (shellsRequireResult, error) {
	ctx := context.Background()
	cache := newRevampedCache()
	result := shellsRequireResult{
		Missing: []shellRequirement{}, Installed: []shellRequirement{}, FellBack: []shellRequirement{},
	}
	var failures []error
	if install {
		if err := migrateLegacyShellProducts(ctx, cache); err != nil {
			failures = append(failures, err)
		}
	}
	catalogue := loadShellCatalogue(ctx, cache)
	missing, err := findMissingShellSelections(ctx, cache, catalogue)
	if err != nil {
		return result, err
	}
	result.Missing = missing
	if install {
		attempted := map[string]bool{}
		for _, requirement := range missing {
			key := requirement.Category + "/" + requirement.ID
			if !requirement.Available || attempted[key] {
				continue
			}
			attempted[key] = true
			offer := catalogue.products[key]
			if err := installProduct(ctx, cache, requirement.Category, offer.Entry); err != nil {
				failures = append(failures, fmt.Errorf("install %s: %w", key, err))
				continue
			}
			result.Installed = append(result.Installed, requirement)
		}
	}
	if fallback {
		remaining, err := findMissingShellSelections(ctx, cache, catalogue)
		if err != nil {
			return result, err
		}
		for _, requirement := range remaining {
			if err := fallBackShellSelection(ctx, cache, catalogue, requirement); err != nil {
				return result, err
			}
			result.FellBack = append(result.FellBack, requirement)
		}
		// Everything unserved has a fallback now, so failed installs are settled.
		failures = nil
	}
	// An install pass may also have migrated legacy installs, so it always syncs.
	if install || len(result.FellBack) > 0 {
		if _, err := runShellsSync(false); err != nil {
			return result, err
		}
	}
	return result, errors.Join(failures...)
}

func fallBackShellSelection(ctx context.Context, cache *Cache, catalogue shellCatalogue, requirement shellRequirement) error {
	switch requirement.Selection {
	case "barStyle":
		return writeBarStyleSelection(defaultShellConfigPath(), shellFallbackID)
	case "dock.design":
		return mutateConfigState(defaultShellConfigPath(), func(state map[string]any) bool {
			section, _ := state["dock"].(map[string]any)
			if section == nil || section["design"] == defaultDockDesign {
				return false
			}
			section["design"] = defaultDockDesign
			return true
		})
	case "launcher.variant":
		fallback := defaultLauncherVariant()
		return mutateConfigState(defaultLauncherConfigPath(), func(state map[string]any) bool {
			if state["variant"] == fallback {
				return false
			}
			state["variant"] = fallback
			return true
		})
	case "desktopWidgets":
		offer, offered := catalogue.widgets[requirement.Value]
		if !offered {
			return nil
		}
		contribution, err := offeredWidgetSet(ctx, cache, offer, requirement.Value)
		if err != nil {
			return err
		}
		return mutateConfigState(widgetsConfigPath(), func(state map[string]any) bool {
			changed := false
			for _, record := range contribution.RosterRecords {
				if prefix, ok := record["prefix"].(string); ok && prefix != "" {
					changed = disableConfigBoolAnywhere(state, prefix+"Enabled") || changed
				}
			}
			return changed
		})
	default:
		return fmt.Errorf("unknown shell selection %q", requirement.Selection)
	}
}

// defaultLauncherVariant is the default the shell's launcher catalogue names
// (modules/launcher/catalog.json), or "" (the shell's own default) when it
// cannot be read.
func defaultLauncherVariant() string {
	raw, err := os.ReadFile(filepath.Join(shellConfigRoot(), "modules", "launcher", "catalog.json"))
	if err != nil {
		return ""
	}
	var catalog struct {
		Default string `json:"default"`
	}
	if decodeOneJSON(raw, &catalog) != nil {
		return ""
	}
	return catalog.Default
}

func disableConfigBoolAnywhere(value any, key string) bool {
	changed := false
	switch current := value.(type) {
	case map[string]any:
		if enabled, ok := current[key].(bool); ok && enabled {
			current[key] = false
			changed = true
		}
		for _, child := range current {
			changed = disableConfigBoolAnywhere(child, key) || changed
		}
	case []any:
		for _, child := range current {
			changed = disableConfigBoolAnywhere(child, key) || changed
		}
	}
	return changed
}
