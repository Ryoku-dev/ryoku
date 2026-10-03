package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

func defaultSidebars() map[string]any {
	return map[string]any{
		"width":          float64(380),
		"motion":         "standard",
		"depth":          true,
		"push":           true,
		"wallpaperSlide": 1.15,
		"left": map[string]any{
			"enabled": true,
			"cards":   []any{"system", "notifications", "weather", "media", "capture", "stage"},
		},
		"right": map[string]any{
			"enabled": true,
			"cards":   []any{"usage", "tools", "chat"},
		},
	}
}

func reconcileSidebarRework(checkOnly bool) recResult {
	path := filepath.Join(sys.ConfigHome(), "ryoku", "shell.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return okRes(i18n.T("no shell.json yet (seeded on first shell run)"))
	}
	migrated, changed, err := migrateSidebarRework(raw)
	if err != nil {
		return warnRes(i18n.T("shell.json does not parse (%v); the shell falls back to defaults"), err).
			withFix(i18n.T("delete %s to re-seed it"), path)
	}
	if !changed {
		return okRes(i18n.T("sidebar settings use the current schema"))
	}
	if checkOnly {
		return wouldRes(i18n.T("shell.json still carries retired sidebar records or lacks sidebar settings")).
			withFix(i18n.T("ryoku doctor migrates it in place"))
	}

	tmp := path + ".ryoku-tmp"
	if err := os.WriteFile(tmp, migrated, 0o644); err != nil {
		return failRes(i18n.T("could not write %s: %v"), tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return failRes(i18n.T("could not replace %s: %v"), path, err)
	}
	return fixedRes(i18n.T("migrated shell.json to the sidebar settings schema"))
}

// migrateSidebarRework retires the two frame-bar leaves replaced by sidebars
// and seeds the new top-level settings without disturbing sibling records.
func migrateSidebarRework(raw []byte) ([]byte, bool, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, false, err
	}
	if top == nil {
		return nil, false, fmt.Errorf("top-level shell config is null")
	}

	changed := false
	frameBarsChanged := false
	if frameBarsRaw, present := top["frameBars"]; present {
		var frameBars map[string]json.RawMessage
		if err := json.Unmarshal(frameBarsRaw, &frameBars); err != nil {
			return nil, false, err
		}

		if menusRaw, present := frameBars["menus"]; present {
			var menus map[string]json.RawMessage
			if err := json.Unmarshal(menusRaw, &menus); err != nil {
				return nil, false, err
			}
			if _, present := menus["quick-settings"]; present {
				delete(menus, "quick-settings")
				repacked, err := json.Marshal(menus)
				if err != nil {
					return nil, false, err
				}
				frameBars["menus"] = repacked
				frameBarsChanged = true
			}
		}

		if surfacesRaw, present := frameBars["surfaces"]; present {
			var surfaces map[string]json.RawMessage
			if err := json.Unmarshal(surfacesRaw, &surfaces); err != nil {
				return nil, false, err
			}
			if _, present := surfaces["stash"]; present {
				delete(surfaces, "stash")
				repacked, err := json.Marshal(surfaces)
				if err != nil {
					return nil, false, err
				}
				frameBars["surfaces"] = repacked
				frameBarsChanged = true
			}
		}

		if frameBarsChanged {
			repacked, err := json.Marshal(frameBars)
			if err != nil {
				return nil, false, err
			}
			top["frameBars"] = repacked
			changed = true
		}
	}

	if _, present := top["sidebars"]; !present {
		seed, err := json.Marshal(defaultSidebars())
		if err != nil {
			return nil, false, err
		}
		top["sidebars"] = seed
		changed = true
	}
	if !changed {
		return nil, false, nil
	}

	out, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return append(out, '\n'), true, nil
}
