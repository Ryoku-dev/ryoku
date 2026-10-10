package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// The session target was renamed, and an existing box still wants its units
// under the old name. Every user unit now declares WantedBy the new target, but
// the enablement symlinks a prior install created live in the old target's
// .wants dir; after an update the new target owns nothing, so starting it starts
// no shell and the desktop comes up black. This moves each stale want onto the
// new target. Idempotent; retired once every box has migrated.

const (
	// retiredSessionTarget is named only to migrate off it.
	retiredSessionTarget = "hyprland-session.target"
	sessionTarget        = "ryoku-session.target"
)

func userUnitDir() string {
	return filepath.Join(sys.ConfigHome(), "systemd", "user")
}

func retiredWantsDir() string {
	return filepath.Join(userUnitDir(), retiredSessionTarget+".wants")
}

// staleSessionWants lists the unit names still symlinked under the retired
// target's .wants dir, sorted; empty when the dir is absent or holds nothing.
func staleSessionWants() []string {
	ents, err := os.ReadDir(retiredWantsDir())
	if err != nil {
		return nil
	}
	var units []string
	for _, e := range ents {
		units = append(units, e.Name())
	}
	sort.Strings(units)
	return units
}

var sessionTargetInit = func() (host.InitSystem, error) {
	return host.Default().Init()
}

// enableUserUnit re-enables a unit so it lands in the new target's .wants dir
// (the unit's own [Install] WantedBy names it). A var so the migration is
// testable without a live user manager.
var enableUserUnit = func(unit string) error {
	if code := host.Default().Service([]string{"--user", "enable", unit}); code != host.ExitOK {
		return fmt.Errorf("service enable exited %d", code)
	}
	return nil
}

// reloadUserDaemon makes systemd pick up the relocated symlinks. A var so the
// migration test never touches the real user manager.
var reloadUserDaemon = func() {
	_ = host.Default().Service([]string{"--user", "daemon-reload"})
}

func reconcileSessionTarget(checkOnly bool) recResult {
	initSystem, err := sessionTargetInit()
	if err != nil {
		return noteRes(i18n.T("could not identify the session supervisor; retired systemd targets were not checked"))
	}
	if initSystem != host.Systemd {
		return okRes(i18n.T("the runit session roster replaces systemd session targets"))
	}
	stale := staleSessionWants()
	if len(stale) == 0 {
		return okRes(i18n.T("session units are wanted by the current target"))
	}
	list := strings.Join(stale, ", ")
	if checkOnly {
		return wouldRes(i18n.T("session units are still wanted by the retired target, so the desktop comes up with no shell: %s"), list).
			withFix(i18n.T("ryoku doctor re-enables them under the current session target"))
	}
	dir := retiredWantsDir()
	var migrated, failed []string
	for _, unit := range stale {
		if err := os.Remove(filepath.Join(dir, unit)); err != nil && !os.IsNotExist(err) {
			failed = append(failed, unit)
			continue
		}
		if err := enableUserUnit(unit); err != nil {
			failed = append(failed, unit)
			continue
		}
		migrated = append(migrated, unit)
	}
	// Drop the now-empty retired .wants dir so a second run is a clean no-op.
	_ = os.Remove(dir)
	reloadUserDaemon()
	if len(failed) > 0 {
		return failRes(i18n.T("could not migrate every session unit onto %s: %s"), sessionTarget, strings.Join(failed, ", ")).
			withFix(i18n.T("ryoku-host svc --user enable %s"), strings.Join(failed, " "))
	}
	return fixedRes(i18n.T("re-enabled session units under %s: %s"), sessionTarget, strings.Join(migrated, ", "))
}
