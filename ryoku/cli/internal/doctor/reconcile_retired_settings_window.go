package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
	wm "ryoku-wm"
)

// The Settings window size rows left the Hub with no replacement: every
// compositor opens Ryoku Settings at its default page fit now. Their
// wm.<provider>.settingsWindow leaves stay in desktop.json, and the module a
// provider's apply rendered from them stays in the live tree and in the overlay
// that re-lays it on every update. This strips the leaves and removes the
// modules. Surgical and idempotent: a clean box is untouched.
func reconcileRetiredSettingsWindow(checkOnly bool) recResult {
	cfg := sys.ConfigHome()
	store := filepath.Join(cfg, "ryoku", "desktop.json")
	var stripped []byte
	if raw, err := os.ReadFile(store); err == nil {
		out, changed, err := stripSettingsWindow(raw)
		if err != nil {
			return failRes(i18n.T("desktop.json does not parse, so the retired Settings window size cannot be dropped: %v"), err).
				withFix(i18n.T("fix or delete %s, then re-run ryoku doctor"), store)
		}
		if changed {
			stripped = out
		}
	}
	var leftovers []string
	for _, name := range wm.Providers() {
		for _, rel := range wm.RetiredGeneratedConfig(name) {
			if sys.PathPresent(filepath.Join(cfg, rel)) {
				leftovers = append(leftovers, rel)
			}
		}
	}
	if stripped == nil && len(leftovers) == 0 {
		return okRes(i18n.T("no retired Settings window size in the store or config"))
	}
	if checkOnly {
		return wouldRes(i18n.T("the retired Settings window size is still stored")).
			withFix(i18n.T("ryoku doctor drops it from desktop.json and removes its generated module"))
	}
	if stripped != nil {
		if err := writeStore(store, stripped); err != nil {
			return failRes(i18n.T("could not write desktop.json: %v"), err)
		}
	}
	for _, rel := range leftovers {
		if err := os.Remove(filepath.Join(cfg, rel)); err != nil && !os.IsNotExist(err) {
			return failRes(i18n.T("could not remove %s: %v"), rel, err)
		}
	}
	return fixedRes(i18n.T("dropped the retired Settings window size"))
}

// stripSettingsWindow drops settingsWindow from every wm.<provider> object,
// keeping every other key as its own raw bytes. A store without one is a no-op;
// a provider object left empty stays, since an empty object is what the Hub
// writes for a provider with no exclusives.
func stripSettingsWindow(raw []byte) (out []byte, changed bool, err error) {
	var top map[string]json.RawMessage
	if err = json.Unmarshal(raw, &top); err != nil {
		return nil, false, err
	}
	wmRaw, ok := top["wm"]
	if !ok {
		return nil, false, nil
	}
	var providers map[string]json.RawMessage
	if err = json.Unmarshal(wmRaw, &providers); err != nil {
		return nil, false, err
	}
	for name, provRaw := range providers {
		var prov map[string]json.RawMessage
		if json.Unmarshal(provRaw, &prov) != nil {
			continue
		}
		if _, ok := prov["settingsWindow"]; !ok {
			continue
		}
		delete(prov, "settingsWindow")
		repacked, err := json.Marshal(prov)
		if err != nil {
			return nil, false, err
		}
		providers[name] = repacked
		changed = true
	}
	if !changed {
		return nil, false, nil
	}
	repacked, err := json.Marshal(providers)
	if err != nil {
		return nil, false, err
	}
	top["wm"] = repacked
	out, err = json.MarshalIndent(top, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return append(out, '\n'), true, nil
}
