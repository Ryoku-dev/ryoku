package doctor

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// ---- reconciler: colour controls agree --------------------------------------
//
// The Hub's Colours switch (shell.json theme.colours) and the wallpaper
// picker's toggle (Ryogami theme.policy) are two views of the same user choice.
// A value can drift when a setting was written by an older build or by hand.
// The picker's policy is the authority for whether the desktop follows the
// wallpaper, so it decides which palette family the shell should wear.

func reconcileThemeColoursPolicy(checkOnly bool) recResult {
	shellPath := filepath.Join(sys.ConfigHome(), "ryoku", "shell.json")
	raw, err := os.ReadFile(shellPath)
	if err != nil {
		return okRes(i18n.T("no shell settings yet"))
	}
	var shell struct {
		Theme struct {
			Colours string `json:"colours"`
		} `json:"theme"`
	}
	if json.Unmarshal(raw, &shell) != nil {
		return okRes(i18n.T("shell settings are not readable yet"))
	}
	out, err := exec.Command("ryogami", "settings", "get", "theme.policy").Output()
	if err != nil {
		return okRes(i18n.T("Ryogami is not installed yet"))
	}
	policy := strings.Trim(strings.TrimSpace(string(out)), `"`)
	var want string
	switch policy {
	case "wallpaper", "fixed":
		want = "wallpaper"
	case "off":
		want = "ryoku"
	default:
		return warnRes("unknown colour policy: " + policy)
	}
	if shell.Theme.Colours == want {
		return okRes(i18n.T("the colour controls agree"))
	}
	if checkOnly {
		return warnRes("the colour controls disagree: shell=" + shell.Theme.Colours + " policy=" + policy)
	}
	if err := exec.Command("ryoku-shell", "colours", want).Run(); err != nil {
		return warnRes("could not align the colour controls: " + err.Error())
	}
	return okRes(i18n.T("the colour controls now agree"))
}
