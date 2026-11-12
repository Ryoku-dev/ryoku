package doctor

import (
	"path/filepath"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// shell.json general.temperature_unit ("Metric"/"Imperial") came over with the
// reference schema and nothing ever read it: the weather unit is the top-level
// weatherUnit. The daemon no longer models it; this drops a stored copy.
func reconcileRetiredTemperatureUnit(checkOnly bool) recResult {
	path := filepath.Join(sys.ConfigHome(), "ryoku", "shell.json")
	shell, exists, err := readRawStore(path)
	if err != nil {
		return warnRes(i18n.T("shell.json does not parse (%v); the shell falls back to defaults"), err).
			withFix(i18n.T("delete %s to re-seed it"), path)
	}
	if !exists {
		return okRes(i18n.T("no shell.json yet (seeded on first shell run)"))
	}
	cur, err := rawLeaf(shell, "general", "temperature_unit")
	if err != nil {
		return warnRes(i18n.T("shell.json general does not parse (%v)"), err)
	}
	if cur == nil {
		return okRes(i18n.T("shell.json carries no retired temperature unit"))
	}
	if checkOnly {
		return wouldRes(i18n.T("shell.json still carries the unused general.temperature_unit")).
			withFix(i18n.T("ryoku doctor drops it; the weather unit is set in Time & language"))
	}
	if _, err := dropRawLeaf(shell, "general", "temperature_unit"); err != nil {
		return failRes(i18n.T("could not drop general.temperature_unit: %v"), err)
	}
	if err := writeRawStore(path, shell); err != nil {
		return failRes(i18n.T("could not write %s: %v"), path, err)
	}
	return fixedRes(i18n.T("dropped the unused general.temperature_unit from shell.json"))
}
