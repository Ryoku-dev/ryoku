package doctor

import (
	"path/filepath"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// shell.json theme.matugen (preference, scheme_type, mode, contrast) came over
// with the reference schema and nothing reads it: Ryogami owns the palette in
// matugen.json. The daemon no longer models it; this drops a stored copy.
func reconcileRetiredShellMatugen(checkOnly bool) recResult {
	path := filepath.Join(sys.ConfigHome(), "ryoku", "shell.json")
	shell, exists, err := readRawStore(path)
	if err != nil {
		return warnRes(i18n.T("shell.json does not parse (%v); the shell falls back to defaults"), err).
			withFix(i18n.T("delete %s to re-seed it"), path)
	}
	if !exists {
		return okRes(i18n.T("no shell.json yet (seeded on first shell run)"))
	}
	cur, err := rawLeaf(shell, "theme", "matugen")
	if err != nil {
		return warnRes(i18n.T("shell.json theme does not parse (%v)"), err)
	}
	if cur == nil {
		return okRes(i18n.T("shell.json carries no retired palette settings"))
	}
	if checkOnly {
		return wouldRes(i18n.T("shell.json still carries the unused theme.matugen settings")).
			withFix(i18n.T("ryoku doctor drops them; the palette is set in Ryogami"))
	}
	if _, err := dropRawLeaf(shell, "theme", "matugen"); err != nil {
		return failRes(i18n.T("could not drop theme.matugen: %v"), err)
	}
	if err := writeRawStore(path, shell); err != nil {
		return failRes(i18n.T("could not write %s: %v"), path, err)
	}
	return fixedRes(i18n.T("dropped the unused theme.matugen settings from shell.json"))
}
