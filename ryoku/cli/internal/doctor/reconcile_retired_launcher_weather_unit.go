package doctor

import (
	"encoding/json"
	"path/filepath"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// The launcher had its own temperature unit (launcher.json weatherUnit, "C",
// "F" or "auto") over the desktop's; Time & language's shell.json weatherUnit
// is now the only one. A launcher unit is carried across while the desktop's is
// still Auto, then the launcher key is dropped. Idempotent.
func reconcileRetiredLauncherWeatherUnit(checkOnly bool) recResult {
	dir := filepath.Join(sys.ConfigHome(), "ryoku")
	launcherPath := filepath.Join(dir, "launcher.json")
	launcher, exists, err := readRawStore(launcherPath)
	if err != nil {
		return warnRes(i18n.T("launcher.json does not parse (%v); the launcher falls back to defaults"), err).
			withFix(i18n.T("delete %s to start fresh"), launcherPath)
	}
	v, ok := launcher["weatherUnit"]
	if !exists || !ok {
		return okRes(i18n.T("launcher.json carries no weather unit of its own"))
	}
	if checkOnly {
		return wouldRes(i18n.T("launcher.json still carries its own weather unit")).
			withFix(i18n.T("ryoku doctor moves it to Time & language, then drops it"))
	}
	if unit := launcherWeatherUnit(v); unit != "" {
		if err := carryWeatherUnit(filepath.Join(dir, "shell.json"), unit); err != nil {
			return failRes(i18n.T("could not carry the launcher's weather unit into shell.json: %v"), err)
		}
	}
	delete(launcher, "weatherUnit")
	if err := writeRawStore(launcherPath, launcher); err != nil {
		return failRes(i18n.T("could not write %s: %v"), launcherPath, err)
	}
	return fixedRes(i18n.T("moved the launcher's weather unit to Time & language"))
}

// launcherWeatherUnit maps a stored launcher unit to the shell's words; Auto and
// anything unknown map to "".
func launcherWeatherUnit(v json.RawMessage) string {
	var s string
	if json.Unmarshal(v, &s) != nil {
		return ""
	}
	switch s {
	case "C", "celsius":
		return "celsius"
	case "F", "fahrenheit":
		return "fahrenheit"
	}
	return ""
}

// carryWeatherUnit sets shell.json weatherUnit while it is absent or Auto. A
// box with no shell.json has never run the shell, so nothing is carried there.
func carryWeatherUnit(path, unit string) error {
	shell, exists, err := readRawStore(path)
	if err != nil || !exists {
		return err
	}
	var cur string
	if v, ok := shell["weatherUnit"]; ok && (json.Unmarshal(v, &cur) != nil || cur != "auto") {
		return nil
	}
	b, _ := json.Marshal(unit)
	shell["weatherUnit"] = b
	return writeRawStore(path, shell)
}
