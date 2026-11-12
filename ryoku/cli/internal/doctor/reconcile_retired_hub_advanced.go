package doctor

import (
	"bytes"
	"os"
	"path/filepath"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// The Settings rail's global Advanced switch left with the old Hub, and the
// rebuilt Settings has none. Its `advanced` value under hub.toml's [ui] table
// has no reader left, so this drops that line and keeps every other byte.
// Idempotent: a hub.toml without it is untouched.
func reconcileRetiredHubAdvanced(checkOnly bool) recResult {
	path := filepath.Join(sys.ConfigHome(), "ryoku", "hub.toml")
	raw, err := os.ReadFile(path)
	if err != nil {
		return okRes(i18n.T("no hub.toml yet"))
	}
	out, changed := stripHubAdvanced(raw)
	if !changed {
		return okRes(i18n.T("hub.toml carries no retired Advanced switch"))
	}
	if checkOnly {
		return wouldRes(i18n.T("hub.toml still carries the retired Advanced switch")).
			withFix(i18n.T("ryoku doctor drops it in place"))
	}
	if err := writeStore(path, out); err != nil {
		return failRes(i18n.T("could not write hub.toml: %v"), err)
	}
	return fixedRes(i18n.T("dropped the retired Advanced switch from hub.toml"))
}

// stripHubAdvanced removes the `advanced = ...` key line from the [ui] table,
// the only place the Hub ever wrote it.
func stripHubAdvanced(raw []byte) ([]byte, bool) {
	lines := bytes.SplitAfter(raw, []byte("\n"))
	out := make([]byte, 0, len(raw))
	table := ""
	changed := false
	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)
		if bytes.HasPrefix(trimmed, []byte("[")) {
			table = string(bytes.TrimSpace(bytes.Trim(trimmed, "[]")))
		} else if table == "ui" && isTOMLKey(trimmed, "advanced") {
			changed = true
			continue
		}
		out = append(out, line...)
	}
	return out, changed
}

// isTOMLKey reports whether a trimmed TOML line assigns the bare key.
func isTOMLKey(line []byte, key string) bool {
	rest, ok := bytes.CutPrefix(line, []byte(key))
	if !ok {
		return false
	}
	return bytes.HasPrefix(bytes.TrimLeft(rest, " \t"), []byte("="))
}
