package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// the pinned answer bubble left with the assistant it pinned, so ask.bubble is dead data in shell.json
func reconcileRetiredAskBubble(checkOnly bool) recResult {
	path := filepath.Join(sys.ConfigHome(), "ryoku", "shell.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return okRes(i18n.T("no shell.json yet (seeded on first shell run)"))
	}
	migrated, changed, err := stripRetiredAskBubble(raw)
	if err != nil {
		return warnRes(i18n.T("shell.json does not parse (%v); the shell falls back to defaults"), err).
			withFix(i18n.T("delete %s to re-seed it"), path)
	}
	if !changed {
		return okRes(i18n.T("shell.json carries no retired answer bubble keys"))
	}
	if checkOnly {
		return wouldRes(i18n.T("shell.json still carries the retired answer bubble keys")).
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
	return fixedRes(i18n.T("stripped the retired answer bubble keys from shell.json"))
}

// a malformed ask errors rather than being rewritten, and one left empty is dropped
func stripRetiredAskBubble(raw []byte) (out []byte, changed bool, err error) {
	var top map[string]json.RawMessage
	if err = json.Unmarshal(raw, &top); err != nil {
		return nil, false, err
	}
	askRaw, ok := top["ask"]
	if !ok {
		return nil, false, nil
	}
	var ask map[string]json.RawMessage
	if err = json.Unmarshal(askRaw, &ask); err != nil {
		return nil, false, err
	}
	if _, ok := ask["bubble"]; !ok {
		return nil, false, nil
	}
	delete(ask, "bubble")
	if len(ask) == 0 {
		delete(top, "ask")
	} else {
		repacked, err := json.Marshal(ask)
		if err != nil {
			return nil, false, err
		}
		top["ask"] = repacked
	}
	out, err = json.MarshalIndent(top, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return append(out, '\n'), true, nil
}
