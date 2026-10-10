package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

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
		return okRes(i18n.T("shell.json carries no retired sidebar settings"))
	}
	if checkOnly {
		return wouldRes(i18n.T("shell.json still carries retired sidebar settings")).
			withFix(i18n.T("ryoku doctor removes them in place"))
	}

	tmp := path + ".ryoku-tmp"
	if err := os.WriteFile(tmp, migrated, 0o644); err != nil {
		return failRes(i18n.T("could not write %s: %v"), tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return failRes(i18n.T("could not replace %s: %v"), path, err)
	}
	return fixedRes(i18n.T("removed retired sidebar settings from shell.json"))
}

// migrateSidebarRework removes the retired global sidebars key and the frame-bar
// records that held the original surfaces. The current controls key and every
// sibling setting remain byte-equivalent after decoding; a clean store is a no-op.
func migrateSidebarRework(raw []byte) ([]byte, bool, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, false, err
	}
	if top == nil {
		return nil, false, fmt.Errorf("top-level shell config is null")
	}

	changed := false
	if _, present := top["sidebars"]; present {
		delete(top, "sidebars")
		changed = true
	}

	frameBarsRaw, present := top["frameBars"]
	if present {
		var frameBars map[string]json.RawMessage
		if err := json.Unmarshal(frameBarsRaw, &frameBars); err != nil {
			return nil, false, err
		}
		frameBarsChanged := false

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
			surfacesChanged := false
			for _, key := range []string{"stash", "system"} {
				if _, present := surfaces[key]; present {
					delete(surfaces, key)
					surfacesChanged = true
				}
			}
			if surfacesChanged {
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

	if !changed {
		return nil, false, nil
	}
	out, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return append(out, '\n'), true, nil
}
