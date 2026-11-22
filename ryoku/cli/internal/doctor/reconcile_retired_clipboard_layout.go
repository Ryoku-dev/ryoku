package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// the palette takes its size and corners from the design, so these keys are dead data in shell.json
func reconcileRetiredClipboardLayout(checkOnly bool) recResult {
	path := filepath.Join(sys.ConfigHome(), "ryoku", "shell.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return okRes(i18n.T("no shell.json yet (seeded on first shell run)"))
	}
	migrated, changed, err := stripRetiredClipboardLayout(raw)
	if err != nil {
		return warnRes(i18n.T("shell.json does not parse (%v); the shell falls back to defaults"), err).
			withFix(i18n.T("delete %s to re-seed it"), path)
	}
	if !changed {
		return okRes(i18n.T("shell.json carries no retired clipboard panel keys"))
	}
	if checkOnly {
		return wouldRes(i18n.T("shell.json still carries the retired clipboard panel size and corner keys")).
			withFix(i18n.T("ryoku doctor strips them in place"))
	}
	tmp := path + ".ryoku-tmp"
	if err := os.WriteFile(tmp, migrated, 0o644); err != nil {
		return failRes(i18n.T("could not write %s: %v"), tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return failRes(i18n.T("could not replace %s: %v"), path, err)
	}
	return fixedRes(i18n.T("stripped the retired clipboard panel size and corner keys from shell.json"))
}

var retiredClipboardLayoutKeys = []string{"widthPercent", "heightPercent", "bottomPercent", "panelRadius", "paneRadius", "cardRadius"}

// a malformed clipboard errors rather than being rewritten, and one left empty is dropped
func stripRetiredClipboardLayout(raw []byte) (out []byte, changed bool, err error) {
	var top map[string]json.RawMessage
	if err = json.Unmarshal(raw, &top); err != nil {
		return nil, false, err
	}
	clipRaw, ok := top["clipboard"]
	if !ok {
		return nil, false, nil
	}
	var clip map[string]json.RawMessage
	if err = json.Unmarshal(clipRaw, &clip); err != nil {
		return nil, false, err
	}
	for _, key := range retiredClipboardLayoutKeys {
		if _, ok := clip[key]; ok {
			delete(clip, key)
			changed = true
		}
	}
	if !changed {
		return nil, false, nil
	}
	if len(clip) == 0 {
		delete(top, "clipboard")
	} else {
		repacked, err := json.Marshal(clip)
		if err != nil {
			return nil, false, err
		}
		top["clipboard"] = repacked
	}
	out, err = json.MarshalIndent(top, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return append(out, '\n'), true, nil
}
