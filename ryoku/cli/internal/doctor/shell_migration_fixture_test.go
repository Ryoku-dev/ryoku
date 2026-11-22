package doctor

import (
	"bytes"
	"encoding/json"
	"os"
	"sort"
	"testing"
)

// The 0.75.5 release predates ryoku/assets/rices/default/rice.json. This fixture
// combines that release's daemon and QML defaults with the stale Atoll, sidebar,
// style, and dock records its doctor tests accepted from an upgraded box.
var shell0755DocumentedDrops = map[string]string{
	"atollVariant":                             "the Atoll skin is retired",
	"barEnabled":                               "frame bars own visibility",
	"barHeight":                                "frame bars own rail geometry",
	"bars.frame.enable_frame":                  "nothing read it; frameEnabled switches the frame",
	"barPosition":                              "frame bars own rail placement",
	"clipboard.bottomPercent":                  "the clipboard palette takes its size and corners from the design",
	"clipboard.cardRadius":                     "the clipboard palette takes its size and corners from the design",
	"clipboard.heightPercent":                  "the clipboard palette takes its size and corners from the design",
	"clipboard.paneRadius":                     "the clipboard palette takes its size and corners from the design",
	"clipboard.panelRadius":                    "the clipboard palette takes its size and corners from the design",
	"clipboard.widthPercent":                   "the clipboard palette takes its size and corners from the design",
	"frameBars.dock":                           "the universal dock owns this store",
	"frameBars.dock.pinned":                    "pins move to dock.pinned",
	"frameBars.menus.quick-settings":           "the global sidebar owns quick settings",
	"frameBars.menus.quick-settings.anchor":    "the global sidebar owns quick settings",
	"frameBars.menus.quick-settings.expansion": "the global sidebar owns quick settings",
	"frameBars.menus.quick-settings.minWidth":  "the global sidebar owns quick settings",
	"frameBars.menus.quick-settings.modules":   "the global sidebar owns quick settings",
	"frameBars.menus.quick-settings.widgets":   "the global sidebar owns quick settings",
	"frameBars.surfaces.stash":                 "the global sidebar owns the stash",
	"frameBars.surfaces.stash.anchor":          "the global sidebar owns the stash",
	"frameBars.surfaces.stash.minWidth":        "the global sidebar owns the stash",
	"frameBars.surfaces.stash.panes":           "the global sidebar owns the stash",
	"frameBorder":                              "the current frame has a fixed border contract",
	"frameRadius":                              "theme sizing owns frame radius",
	"frameSmoothing":                           "the soft frame look is retired",
	"qsbar.dockEnabled":                        "the universal dock owns this value",
	"qsbar.dockFrost":                          "the universal dock owns this value",
	"qsbar.dockMagnify":                        "the universal dock owns this value",
	"qsbar.dockPinned":                         "pins move to dock.pinned",
	"qsbar.dockShadow":                         "the universal dock owns this value",
	"roundness":                                "theme sizing owns roundness",
	"shadowSize":                               "the soft frame look is retired",
	"shadowStrength":                           "the soft frame look is retired",
	"sidebarLeftPanes":                         "the old sidebar is retired",
	"sidebarRightPanes":                        "the old sidebar is retired",
	"sidebarWidth":                             "the old sidebar is retired",
	"surfaceColor":                             "theme tokens own the surface colour",
}

type shellStoreMigration func([]byte) ([]byte, bool, error)

func runShellStoreMigrations(t *testing.T, raw []byte) []byte {
	t.Helper()
	apply := func(name string, migrate shellStoreMigration) {
		t.Helper()
		out, changed, err := migrate(raw)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if changed {
			raw = out
		}
	}

	apply("migrateFrameBarsStyle", migrateFrameBarsStyle)
	migrated, changes, err := migrateShellConfig(raw)
	if err != nil {
		t.Fatalf("migrateShellConfig: %v", err)
	}
	if len(changes) > 0 {
		raw = migrated
	}
	apply("stripLegacyStyleKnobs", stripLegacyStyleKnobs)
	apply("migrateSumiBar", migrateSumiBar)
	apply("migrateDockStore", migrateDockStore)
	apply("stripRetiredMenus", stripRetiredMenus)
	apply("stripRetiredWallpaperKeys", func(in []byte) ([]byte, bool, error) {
		out, _, changed, err := stripRetiredWallpaperKeys(in)
		return out, changed, err
	})
	apply("addCaptureModule", addCaptureModule)
	apply("replaceQuickSettingsModules", func(in []byte) ([]byte, bool, error) {
		return replaceQuickSettingsModules(in, []string{"depth", "parallax"}, "stage")
	})
	apply("migrateSidebarRework", migrateSidebarRework)
	apply("migrateAskKeybind", migrateAskKeybind)
	apply("stripShimaFrameMusicKey", stripShimaFrameMusicKey)
	apply("stripIrisBatteryThresholds", stripIrisBatteryThresholds)
	apply("stripRetiredClipboardLayout", stripRetiredClipboardLayout)
	return raw
}

func shellConfigKeyPaths(raw []byte) (map[string]bool, error) {
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, err
	}
	paths := map[string]bool{}
	var walk func(string, map[string]any)
	walk = func(prefix string, object map[string]any) {
		for key, value := range object {
			path := key
			if prefix != "" {
				path = prefix + "." + key
			}
			paths[path] = true
			if child, ok := value.(map[string]any); ok {
				walk(path, child)
			}
		}
	}
	walk("", root)
	return paths, nil
}

func assertDocumentedShellDrops(t *testing.T, before, after []byte) {
	t.Helper()
	oldPaths, err := shellConfigKeyPaths(before)
	if err != nil {
		t.Fatalf("parse fixture paths: %v", err)
	}
	newPaths, err := shellConfigKeyPaths(after)
	if err != nil {
		t.Fatalf("parse migrated paths: %v", err)
	}

	removed := make([]string, 0)
	for path := range oldPaths {
		if !newPaths[path] {
			removed = append(removed, path)
		}
	}
	sort.Strings(removed)
	for _, path := range removed {
		if _, documented := shell0755DocumentedDrops[path]; !documented {
			t.Fatalf("shell migration removed unlisted key %q", path)
		}
	}
	for path := range shell0755DocumentedDrops {
		if oldPaths[path] && !newPaths[path] {
			continue
		}
		t.Errorf("documented shell drop %q was not removed from the fixture", path)
	}
}

func TestShellMigrationFrom0755Fixture(t *testing.T) {
	fixture, err := os.ReadFile("testdata/shell-0.75.5.json")
	if err != nil {
		t.Fatal(err)
	}

	migrated := runShellStoreMigrations(t, fixture)
	var current map[string]any
	if err := json.Unmarshal(migrated, &current); err != nil {
		t.Fatalf("migrated shell fixture does not parse: %v", err)
	}
	if got := current["barStyle"]; got != "orpheus" {
		t.Errorf("retired 0.75.5 bar style reached %v, want current default orpheus", got)
	}
	if got := current["fontScale"]; got != 1.17 {
		t.Errorf("custom fontScale = %v, want 1.17", got)
	}
	assertDocumentedShellDrops(t, fixture, migrated)
	if second := runShellStoreMigrations(t, migrated); !bytes.Equal(second, migrated) {
		t.Errorf("second migration changed the converged fixture:\nfirst:  %s\nsecond: %s", migrated, second)
	}

	var deliberate map[string]any
	if err := json.Unmarshal(fixture, &deliberate); err != nil {
		t.Fatal(err)
	}
	deliberate["barStyle"] = "qsbar"
	deliberateFixture, err := json.Marshal(deliberate)
	if err != nil {
		t.Fatal(err)
	}
	deliberateMigrated := runShellStoreMigrations(t, deliberateFixture)
	if err := json.Unmarshal(deliberateMigrated, &deliberate); err != nil {
		t.Fatalf("migrated deliberate-choice fixture does not parse: %v", err)
	}
	if got := deliberate["barStyle"]; got != "qsbar" {
		t.Errorf("deliberate barStyle = %v, want qsbar", got)
	}
	if got := deliberate["fontScale"]; got != 1.17 {
		t.Errorf("deliberate fixture fontScale = %v, want 1.17", got)
	}
	if second := runShellStoreMigrations(t, deliberateMigrated); !bytes.Equal(second, deliberateMigrated) {
		t.Errorf("second deliberate-choice migration changed the converged fixture")
	}
}
