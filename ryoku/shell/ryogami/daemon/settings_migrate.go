package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// migrateLegacyConfig folds the old picker's config in once, then renames it so it never runs again.
func (d *daemon) migrateLegacyConfig() {
	oldPath := filepath.Join(ryogamiWallConfigDir(), "config.json")
	b, err := os.ReadFile(oldPath)
	if err != nil {
		return
	}
	var old map[string]interface{}
	if json.Unmarshal(b, &old) != nil {
		return // leave a malformed file for a hand fix rather than lose it
	}

	tree := loadRyogamiTree()
	if tree == nil {
		tree = map[string]interface{}{}
	}
	changed := migrateMerge(tree, old, "")
	if migrateDayNight(tree, old) {
		changed = true
	}

	if changed {
		out, err := json.MarshalIndent(tree, "", "  ")
		if err != nil {
			return
		}
		_ = os.MkdirAll(ryokuConfigDir(), 0o755)
		if err := settingsAtomicWrite(configPath(), out); err != nil {
			return // keep the source so the next boot retries
		}
		setSettingsBaseline(flattenSettings(tree))
		d.reloadConfig()
	}
	_ = os.Rename(oldPath, oldPath+".migrated")
}

func (d *daemon) migratePickerState() {
	if d.store == nil {
		return
	}
	tree := loadRyogamiTree()
	if tree == nil {
		tree = map[string]interface{}{}
	}
	legacyKeys := []string{
		"filterBar.last.type",
		"filterBar.last.folder",
		"filterBar.last.color",
		"filterBar.last.sort",
		"filterBar.last.favourites",
	}
	values := map[string]interface{}{}
	for _, key := range []string{"type", "folder", "sort"} {
		if value, ok := d.store.stateGet("filterBar.last." + key); ok {
			values["filterBar.last."+key] = value
		}
	}
	if value, ok := d.store.stateGet("filterBar.last.color"); ok {
		if parsed, err := strconv.Atoi(value); err == nil {
			values["filterBar.last.color"] = parsed
		}
	}
	if value, ok := d.store.stateGet("filterBar.last.favourites"); ok {
		values["filterBar.last.favourites"] = value == "1" || value == "true"
	}

	changed := false
	for key, value := range values {
		if _, exists := lookupDotted(tree, key); exists || validateSetting(key, value) != nil {
			continue
		}
		setDotted(tree, key, value)
		changed = true
	}
	if changed {
		out, err := json.MarshalIndent(tree, "", "  ")
		if err != nil {
			return
		}
		if err := os.MkdirAll(ryokuConfigDir(), 0o755); err != nil {
			return
		}
		if err := settingsAtomicWrite(configPath(), out); err != nil {
			return
		}
		setSettingsBaseline(flattenSettings(tree))
		d.reloadConfig()
	}
	for _, key := range legacyKeys {
		d.store.stateSet(key, nil)
	}
}

// The old strip's card shape and saved presets stay behind so the rebuilt picker opens on its own look.
var legacyLook = []string{
	"components.wallpaperSelector.corner",
	"components.wallpaperSelector.roundCorners",
	"components.wallpaperSelector.skewOffset",
	"components.wallpaperSelector.slice",
	"components.wallpaperSelector.visibleCount",
	"components.wallpaperSelector.expandedWidth",
	"components.wallpaperSelector.presets",
	"components.wallpaperSelector.customPresets",
}

func isLegacyLook(path string) bool {
	for _, p := range legacyLook {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// Retired keys no longer have a schema entry, so migration must drop them rather
// than carry them forward as foreign data. The skwd-paper image/video engine is
// gone: Ryoku paints stills and video itself, and skwd-paper stays only as the
// Wallpaper Engine scene renderer.
var retiredKeys = []string{
	"paper.engine",
	"paper.videoEngine",
	"paper.wallpaperLayer",
	"paper.videoMultiProcess",
	// The skwd filter bar gave way to the masthead: one layout, pinned to the top,
	// so its style, orientation and offsets have nothing left to choose.
	"filterBar.visualStyle",
	"filterBar.orientation",
	"filterBar.offsetX",
	"filterBar.offsetY",
	// custom outputs follow theme.mode, the one light-or-dark choice
	"matugen.mode",
}

func isRetiredKey(path string) bool {
	for _, p := range retiredKeys {
		if path == p {
			return true
		}
	}
	return false
}

// Never overwrites a value dst holds; an old value the schema rejects falls back to the default.
func migrateMerge(dst, src map[string]interface{}, prefix string) bool {
	changed := false
	for k, sv := range src {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		if path == "daynight" || strings.HasPrefix(path, "daynight.") || isLegacyLook(path) || isRetiredKey(path) {
			continue
		}
		if spec, ok := specFor(path); ok {
			if isDocStore(spec.Store) || spec.Store == "shell" || spec.validate(sv) != nil {
				continue
			}
			if _, exists := dst[k]; !exists {
				dst[k] = sv
				changed = true
			}
			continue
		}
		if sm, ok := sv.(map[string]interface{}); ok {
			dm, ok := dst[k].(map[string]interface{})
			if !ok {
				if _, occupied := dst[k]; occupied {
					continue
				}
				dm = map[string]interface{}{}
				dst[k] = dm
				changed = true
			}
			if migrateMerge(dm, sm, path) {
				changed = true
			}
			continue
		}
		if _, exists := dst[k]; !exists {
			dst[k] = sv
			changed = true
		}
	}
	return changed
}

// The two windows never overlap, so both rules share one priority.
func migrateDayNight(tree, old map[string]interface{}) bool {
	raw, ok := old["daynight"].(map[string]interface{})
	if !ok {
		return false
	}
	interval := 60
	if v, ok := toNumber(raw["rotateIntervalMinutes"]); ok && v >= 1 {
		interval = int(v)
	}
	noRepeat := true
	if v, ok := raw["noRepeatWithinDay"].(bool); ok {
		noRepeat = v
	}
	enabled := false
	if v, ok := raw["enabled"].(bool); ok {
		enabled = v
	}
	dayDir, _ := raw["dayDir"].(string)
	nightDir, _ := raw["nightDir"].(string)

	changed := false
	if _, exists := lookupDotted(tree, "schedule.rules"); !exists {
		setDotted(tree, "schedule.rules", []interface{}{
			dayNightRule("Day", dayDir, "sunrise", "sunset", interval, noRepeat),
			dayNightRule("Night", nightDir, "sunset", "sunrise", interval, noRepeat),
		})
		changed = true
	}
	if _, exists := lookupDotted(tree, "schedule.enabled"); !exists {
		setDotted(tree, "schedule.enabled", enabled)
		changed = true
	}
	return changed
}

func dayNightRule(name, dir, from, to string, interval int, noRepeat bool) map[string]interface{} {
	target := map[string]interface{}{"type": "random"}
	if dir != "" {
		target = map[string]interface{}{"type": "folder", "value": dir}
	}
	return map[string]interface{}{
		"name":     name,
		"enabled":  true,
		"priority": float64(100),
		"target":   target,
		"theme":    "keep",
		"condition": map[string]interface{}{
			"op":      "all",
			"negated": false,
			"children": []interface{}{
				map[string]interface{}{
					"block":   "timewindow",
					"negated": false,
					"from":    from,
					"to":      to,
				},
			},
		},
		"rotateMinutes": float64(interval),
		"noRepeat":      noRepeat,
	}
}
