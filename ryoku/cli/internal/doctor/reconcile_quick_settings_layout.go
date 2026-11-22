package doctor

import (
	"encoding/json"
	"path/filepath"
	"slices"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

const quickSettingsLayoutMigration = "quick-settings-top-right"

type quickSettingsSection struct {
	ID      string `json:"id"`
	Visible bool   `json:"visible"`
}

var (
	retiredQuickSettingsSections = []quickSettingsSection{
		{"vitals", true}, {"connections", true}, {"powerProfile", true},
		{"media", false}, {"levels", true}, {"bottomControls", true},
	}
	currentQuickSettingsSections = []quickSettingsSection{
		{"connections", true}, {"levels", true}, {"media", false},
		{"powerProfile", true}, {"vitals", true}, {"bottomControls", true},
	}
)

func quickSettingsLayoutMarker() string {
	return filepath.Join(sys.StateDir(), "migrations", quickSettingsLayoutMigration)
}

// the daemon writes the whole controls contract on any change, so most shell.json
// files hold the old defaults as if picked, and the marker lets a later pick stand
func reconcileQuickSettingsLayout(checkOnly bool) recResult {
	marker := quickSettingsLayoutMarker()
	if sys.Exists(marker) {
		return okRes(i18n.T("Quick settings layout already reconciled"))
	}

	path := filepath.Join(sys.ConfigHome(), "ryoku", "shell.json")
	top, exists, err := readRawStore(path)
	if err != nil {
		return warnRes(i18n.T("shell.json cannot be read for the Quick settings layout (%v); it was left untouched"), err).
			withFix(i18n.T("repair %s, then run `ryoku doctor`"), path)
	}
	changed := false
	if exists {
		if changed, err = migrateQuickSettingsLayout(top); err != nil {
			return warnRes(i18n.T("shell.json controls cannot be safely migrated (%v); they were left untouched"), err).
				withFix(i18n.T("repair controls in %s, then run `ryoku doctor`"), path)
		}
	}
	if checkOnly {
		if changed {
			return wouldRes(i18n.T("Quick settings still uses the retired top-left layout"))
		}
		return okRes(i18n.T("Quick settings layout is user-selected or already current"))
	}
	if changed {
		if err := writeRawStore(path, top); err != nil {
			return failRes(i18n.T("could not migrate %s: %v"), path, err).
				withFix(i18n.T("fix the file permissions, then run `ryoku doctor`"))
		}
	}
	if err := markMigration(marker); err != nil {
		return failRes(i18n.T("could not record the Quick settings layout migration: %v"), err).
			withFix(i18n.T("run `ryoku doctor` again"))
	}
	if changed {
		return fixedRes(i18n.T("moved Quick settings to the top right, connections first"))
	}
	return okRes(i18n.T("Quick settings layout is user-selected or already current"))
}

// only the exact shipped list moves, a reordered or partly hidden one was picked
func migrateQuickSettingsLayout(top map[string]json.RawMessage) (bool, error) {
	changed := false

	position, err := rawLeaf(top, "controls", "position")
	if err != nil {
		return false, err
	}
	var pos string
	if position != nil && json.Unmarshal(position, &pos) == nil && pos == "top-left" {
		if err := setRawLeaf(top, json.RawMessage(`"top-right"`), "controls", "position"); err != nil {
			return false, err
		}
		changed = true
	}

	sections, err := rawLeaf(top, "controls", "sections")
	if err != nil {
		return false, err
	}
	var list []quickSettingsSection
	if sections != nil && json.Unmarshal(sections, &list) == nil && slices.Equal(list, retiredQuickSettingsSections) {
		current, err := json.Marshal(currentQuickSettingsSections)
		if err != nil {
			return false, err
		}
		if err := setRawLeaf(top, current, "controls", "sections"); err != nil {
			return false, err
		}
		changed = true
	}
	return changed, nil
}
