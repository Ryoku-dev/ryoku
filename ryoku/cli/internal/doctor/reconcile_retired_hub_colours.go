package doctor

import (
	"encoding/json"
	"path/filepath"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// Settings' own colours (shell.json hub.colours, hub.contrast, hub.tint) became the
// desktop's: theme.colours, theme.contrast and theme.tint, chosen on Appearance > Style
// and worn by every Ryoku surface in every design. A value the user set is carried
// while its new home still holds the shipped one; then the hub keys are dropped, and
// the hub object with them once it is empty. hub.colours "light" was Ryoku's colours
// on white: it carries as ryoku, since light or dark is the desktop's now. Idempotent.

var retiredHubColours = []struct{ from, to, shipped string }{
	{"colours", "colours", "ryoku"},
	{"contrast", "contrast", "standard"},
	{"tint", "tint", "off"},
}

func reconcileRetiredHubColours(checkOnly bool) recResult {
	path := filepath.Join(sys.ConfigHome(), "ryoku", "shell.json")
	shell, exists, err := readRawStore(path)
	if err != nil {
		return warnRes(i18n.T("shell.json does not parse (%v); the shell falls back to defaults"), err)
	}
	if !exists {
		return okRes(i18n.T("no shell.json yet"))
	}
	var hub map[string]json.RawMessage
	if raw, ok := shell["hub"]; !ok || json.Unmarshal(raw, &hub) != nil || !carriesHubColours(hub) {
		return okRes(i18n.T("shell.json carries no retired Settings colours"))
	}
	if checkOnly {
		return wouldRes(i18n.T("shell.json still carries Settings' own colours")).
			withFix(i18n.T("ryoku doctor carries them into the desktop's colours on Style, then drops them"))
	}
	for _, k := range retiredHubColours {
		v, ok := hub[k.from]
		if !ok {
			continue
		}
		delete(hub, k.from)
		var word string
		if json.Unmarshal(v, &word) != nil {
			continue
		}
		if k.from == "colours" && word == "light" {
			word = "ryoku"
		}
		cur, err := rawLeaf(shell, "theme", k.to)
		if err != nil {
			return failRes(i18n.T("could not read theme.%s in shell.json: %v"), k.to, err)
		}
		var now string
		if cur != nil && json.Unmarshal(cur, &now) == nil && now != k.shipped {
			continue
		}
		b, _ := json.Marshal(word)
		if err := setRawLeaf(shell, b, "theme", k.to); err != nil {
			return failRes(i18n.T("could not write theme.%s in shell.json: %v"), k.to, err)
		}
	}
	if len(hub) == 0 {
		delete(shell, "hub")
	} else {
		b, _ := json.Marshal(hub)
		shell["hub"] = b
	}
	if err := writeRawStore(path, shell); err != nil {
		return failRes(i18n.T("could not write %s: %v"), path, err)
	}
	return fixedRes(i18n.T("moved Settings' own colours into the desktop's colours on Style"))
}

func carriesHubColours(hub map[string]json.RawMessage) bool {
	for _, k := range retiredHubColours {
		if _, ok := hub[k.from]; ok {
			return true
		}
	}
	return false
}
