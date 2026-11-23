package doctor

import (
	"encoding/json"
	"path/filepath"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

const launcherOrpheusDefaultMigration = "launcher-orpheus-default"

func launcherOrpheusDefaultMarker() string {
	return filepath.Join(sys.StateDir(), "migrations", launcherOrpheusDefaultMigration)
}

// Hero was the shipped launcher and the Settings factory wrote it into
// launcher.json as if picked, so only that exact value moves to Orpheus, once.
// The marker lets a later pick of Hero stand.
func reconcileLauncherOrpheusDefault(checkOnly bool) recResult {
	marker := launcherOrpheusDefaultMarker()
	if sys.Exists(marker) {
		return okRes(i18n.T("launcher default already reconciled"))
	}

	path := filepath.Join(sys.ConfigHome(), "ryoku", "launcher.json")
	top, exists, err := readRawStore(path)
	if err != nil {
		return warnRes(i18n.T("launcher.json cannot be read for the launcher default (%v); it was left untouched"), err).
			withFix(i18n.T("repair %s, then run `ryoku doctor`"), path)
	}
	changed := exists && migrateLauncherOrpheusDefault(top)
	if checkOnly {
		if changed {
			return wouldRes(i18n.T("the launcher still uses the retired Hero default"))
		}
		return okRes(i18n.T("launcher is user-selected or already current"))
	}
	if changed {
		out, err := json.MarshalIndent(top, "", "  ")
		if err == nil {
			// keeps a symlinked launcher.json and its mode, like the local-frost move
			err = replaceLauncherConfig(path, append(out, '\n'))
		}
		if err != nil {
			return failRes(i18n.T("could not migrate %s: %v"), path, err).
				withFix(i18n.T("fix the file permissions, then run `ryoku doctor`"))
		}
	}
	if err := markMigration(marker); err != nil {
		return failRes(i18n.T("could not record the launcher default migration: %v"), err).
			withFix(i18n.T("run `ryoku doctor` again"))
	}
	if changed {
		return fixedRes(i18n.T("moved the launcher from Hero to Orpheus, the new default"))
	}
	return okRes(i18n.T("launcher is user-selected or already current"))
}

// only the exact retired default moves, any other value was picked
func migrateLauncherOrpheusDefault(top map[string]json.RawMessage) bool {
	var variant string
	if v, ok := top["variant"]; !ok || json.Unmarshal(v, &variant) != nil || variant != "hero" {
		return false
	}
	top["variant"] = json.RawMessage(`"orpheus"`)
	return true
}
