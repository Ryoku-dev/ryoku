package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

const (
	oniSocket       = "oni.socket"
	oniService      = "oni.service"
	aiUsageTimer    = "hyakki-usage.timer"
	aiUsageRunit    = "hyakki-usage"
	routinesTimer   = "oni-routines.timer"
	routinesRunit   = "oni-routines"
	legacyAppUnit   = "ryoku-rashin.service"
	legacyProwlUnit = "ryoku-prowl.service"
)

var (
	hyakkiInit = func() (host.InitSystem, error) {
		return host.Default().Init()
	}
	hyakkiService = func(args []string) int {
		return host.Default().Service(args)
	}
	hyakkiCommand = func(args ...string) error {
		return exec.Command("hyakki", args...).Run()
	}
	hyakkiMigration = func() error {
		return host.Default().MigrateHyakki("")
	}
	oniRunning = func() bool {
		return processRunning("oni")
	}
	stopOni = func() error {
		err := exec.Command("pkill", "-x", "oni").Run()
		if err != nil && oniRunning() {
			return err
		}
		return nil
	}
)

type hyakkiSettings struct {
	Enabled bool `json:"enabled"`
}

func readHyakkiEnabled() bool {
	b, err := os.ReadFile(filepath.Join(sys.ConfigHome(), "hyakki", "hyakki.json"))
	if err != nil {
		return false
	}
	var settings hyakkiSettings
	return json.Unmarshal(b, &settings) == nil && settings.Enabled
}

func hyakkiManagedUnits(initSystem host.InitSystem) []string {
	if initSystem == host.Runit {
		return []string{aiUsageRunit}
	}
	return []string{oniSocket, aiUsageTimer}
}

// These are never armed just because Hyakki is on, but must be parked while off.
func hyakkiParkedUnits(initSystem host.InitSystem) []string {
	if initSystem == host.Runit {
		return []string{"oni", routinesRunit}
	}
	return []string{oniService, "hyakki-usage.service", routinesTimer, "oni-routines.service"}
}

func hyakkiUnitsMatch(enabled bool, initSystem host.InitSystem) bool {
	for _, unit := range hyakkiManagedUnits(initSystem) {
		isEnabled := hyakkiService([]string{"--user", "is-enabled", unit}) == host.ExitOK
		if isEnabled != enabled {
			return false
		}
	}
	if !enabled {
		for _, unit := range hyakkiParkedUnits(initSystem) {
			if hyakkiService([]string{"--user", "is-enabled", unit}) == host.ExitOK ||
				hyakkiService([]string{"--user", "is-active", unit}) == host.ExitOK {
				return false
			}
		}
	}
	return true
}

var migrationMarkerSafe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func hyakkiMigrationMarker() string {
	version := "unknown"
	if installed, ok := reportPackageVersion("hyakki"); ok && installed != "" {
		version = installed
	}
	version = strings.Trim(migrationMarkerSafe.ReplaceAllString(version, "-"), "-")
	if version == "" {
		version = "unknown"
	}
	return filepath.Join(sys.StateDir(), "hyakki-migrated-"+version)
}

func hyakkiMigrationPending() bool {
	_, err := os.Stat(hyakkiMigrationMarker())
	return os.IsNotExist(err)
}

func migrateHyakki() error {
	marker := hyakkiMigrationMarker()
	if err := hyakkiMigration(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		return err
	}
	return os.WriteFile(marker, []byte("done\n"), 0o644)
}

func retiredAppUnits(initSystem host.InitSystem) []string {
	if initSystem == host.Runit {
		return []string{"ryoku-rashin", "ryoku-prowl", "rashind", "rashind-routines", "ryoku-ai-usage"}
	}
	return []string{legacyAppUnit, legacyProwlUnit, "rashind.socket", "rashind.service",
		"rashind-routines.timer", "rashind-routines.service", "ryoku-ai-usage.timer", "ryoku-ai-usage.service"}
}

func legacyUnitMasked(path string) bool {
	target, err := os.Readlink(path)
	return err == nil && target == "/dev/null"
}

func legacyAppResidue(initSystem host.InitSystem) bool {
	if initSystem == host.Runit {
		for _, service := range retiredAppUnits(initSystem) {
			if info, err := os.Stat(filepath.Join(sys.ConfigHome(), "service", service)); err == nil && info.IsDir() {
				return true
			}
		}
		return false
	}
	units := retiredAppUnits(initSystem)
	if len(hyakkiUnitLinks(units)) > 0 {
		return true
	}
	for _, unit := range units {
		if hyakkiService([]string{"--user", "is-enabled", unit}) == host.ExitOK ||
			hyakkiService([]string{"--user", "is-active", unit}) == host.ExitOK {
			return true
		}
		path := filepath.Join(sys.ConfigHome(), "systemd", "user", unit)
		if _, err := os.Lstat(path); err == nil && !legacyUnitMasked(path) {
			return true
		}
	}
	return false
}

func removeLegacyAppUnits(initSystem host.InitSystem) error {
	if initSystem == host.Runit {
		for _, service := range retiredAppUnits(initSystem) {
			path := filepath.Join(sys.ConfigHome(), "service", service)
			if info, err := os.Stat(path); err != nil || !info.IsDir() {
				continue
			}
			code := hyakkiService([]string{"--user", "stop", service})
			if code != host.ExitOK && code != host.ExitAbsent && code != host.ExitFalse {
				return fmt.Errorf("stop %s: service exited %d", service, code)
			}
			if err := os.RemoveAll(path); err != nil {
				return err
			}
		}
		return nil
	}

	reload := false
	units := retiredAppUnits(initSystem)
	for _, unit := range units {
		path := filepath.Join(sys.ConfigHome(), "systemd", "user", unit)
		if legacyUnitMasked(path) {
			continue
		}
		_, pathErr := os.Lstat(path)
		active := hyakkiService([]string{"--user", "is-active", unit})
		if os.IsNotExist(pathErr) && active == host.ExitAbsent && len(hyakkiUnitLinks([]string{unit})) == 0 {
			continue
		}
		code := hyakkiService([]string{"--user", "disable", "--now", unit})
		if code != host.ExitOK && code != host.ExitAbsent && code != host.ExitFalse {
			return fmt.Errorf("disable %s: service exited %d", unit, code)
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		reload = true
		for _, link := range hyakkiUnitLinks([]string{unit}) {
			if err := os.Remove(link); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	if reload && hyakkiService([]string{"--user", "daemon-reload"}) != host.ExitOK {
		return fmt.Errorf("reload user services after removing retired units")
	}
	return nil
}

func absentHyakkiUnits(initSystem host.InitSystem) []string {
	if initSystem == host.Runit {
		return []string{"oni", aiUsageRunit, routinesRunit}
	}
	return []string{oniSocket, oniService, aiUsageTimer, "hyakki-usage.service", routinesTimer, "oni-routines.service"}
}

func hyakkiUnitLinks(units []string) []string {
	root := filepath.Join(sys.ConfigHome(), "systemd", "user")
	var links []string
	for _, unit := range units {
		matches, _ := filepath.Glob(filepath.Join(root, "*.wants", unit))
		for _, match := range matches {
			info, err := os.Lstat(match)
			if err == nil && info.Mode()&os.ModeSymlink != 0 {
				links = append(links, match)
			}
		}
	}
	return links
}

func absentHyakkiResidue(initSystem host.InitSystem) bool {
	if oniRunning() {
		return true
	}
	units := absentHyakkiUnits(initSystem)
	if initSystem == host.Systemd && len(hyakkiUnitLinks(units)) > 0 {
		return true
	}
	for _, unit := range units {
		if hyakkiService([]string{"--user", "is-enabled", unit}) == host.ExitOK ||
			hyakkiService([]string{"--user", "is-active", unit}) == host.ExitOK {
			return true
		}
	}
	return false
}

func cleanAbsentHyakki(initSystem host.InitSystem) error {
	units := absentHyakkiUnits(initSystem)
	for _, unit := range units {
		active := hyakkiService([]string{"--user", "is-active", unit})
		enabled := hyakkiService([]string{"--user", "is-enabled", unit})
		code := hyakkiService([]string{"--user", "stop", unit})
		if code != host.ExitOK && code != host.ExitAbsent && code != host.ExitFalse &&
			active != host.ExitAbsent && active != host.ExitFalse {
			return fmt.Errorf("stop %s: service exited %d", unit, code)
		}
		code = hyakkiService([]string{"--user", "disable", unit})
		if code != host.ExitOK && code != host.ExitAbsent && code != host.ExitFalse &&
			enabled == host.ExitOK {
			return fmt.Errorf("disable %s: service exited %d", unit, code)
		}
	}
	if initSystem == host.Systemd {
		for _, link := range hyakkiUnitLinks(units) {
			if err := os.Remove(link); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		if hyakkiService([]string{"--user", "daemon-reload"}) != host.ExitOK {
			return fmt.Errorf("reload user services after removing Hyakki")
		}
	}
	if oniRunning() {
		if err := stopOni(); err != nil {
			return fmt.Errorf("stop oni: %w", err)
		}
	}
	return nil
}

func convergeHyakkiUnits(enabled bool, initSystem host.InitSystem) error {
	verb := "enable"
	if !enabled {
		verb = "disable"
	}
	for _, unit := range hyakkiManagedUnits(initSystem) {
		code := hyakkiService([]string{"--user", verb, "--now", unit})
		if code != host.ExitOK && !enabled && (code == host.ExitAbsent || code == host.ExitFalse) {
			continue
		}
		if code != host.ExitOK {
			return fmt.Errorf("%s %s: service exited %d", verb, unit, code)
		}
	}
	if !enabled {
		for _, unit := range hyakkiParkedUnits(initSystem) {
			code := hyakkiService([]string{"--user", "disable", "--now", unit})
			if code != host.ExitOK && code != host.ExitAbsent && code != host.ExitFalse {
				return fmt.Errorf("disable %s: service exited %d", unit, code)
			}
		}
		if err := hyakkiCommand("off"); err != nil {
			return fmt.Errorf("stop oni: %w", err)
		}
		if initSystem == host.Systemd {
			code := hyakkiService([]string{"--user", "stop", oniService})
			if code != host.ExitOK && code != host.ExitAbsent && code != host.ExitFalse {
				return fmt.Errorf("stop %s: service exited %d", oniService, code)
			}
		}
	}
	return nil
}

func reconcileHyakkiDaemon(checkOnly bool) recResult {
	installed := sys.Has("hyakki")
	initSystem, err := hyakkiInit()
	if err != nil {
		return noteRes(i18n.T("could not identify the session supervisor; Hyakki was not changed"))
	}
	if !installed {
		if !absentHyakkiResidue(initSystem) {
			return okRes(i18n.T("Hyakki is not installed and no service or daemon remains"))
		}
		if checkOnly {
			return wouldRes(i18n.T("Hyakki is not installed, but its services or daemon are still present")).
				withFix(i18n.T("ryoku doctor stops the daemon, disables its user services and removes dangling unit links"))
		}
		if err := cleanAbsentHyakki(initSystem); err != nil {
			return failRes(i18n.T("could not remove Hyakki's remaining services: %v"), err).
				withFix(i18n.T("stop the Hyakki user services and daemon, then run `ryoku doctor` again"))
		}
		return fixedRes(i18n.T("stopped Hyakki and removed its remaining user services"))
	}
	enabled := readHyakkiEnabled()
	migrationPending := hyakkiMigrationPending()
	legacy := legacyAppResidue(initSystem)
	unitsMatch := hyakkiUnitsMatch(enabled, initSystem) && (enabled || !oniRunning())
	if !migrationPending && !legacy && unitsMatch {
		if enabled {
			return okRes(i18n.T("Hyakki is on and its socket and usage collector follow its settings"))
		}
		return okRes(i18n.T("Hyakki is off with no socket, timer or daemon running"))
	}
	if checkOnly {
		return wouldRes(i18n.T("Hyakki's migration and service state do not match its enabled setting")).
			withFix(i18n.T("ryoku doctor migrates Hyakki and converges its socket and usage collector without turning it on"))
	}
	if migrationPending || legacy {
		if err := migrateHyakki(); err != nil {
			return failRes(i18n.T("could not migrate Hyakki: %v"), err).
				withFix(i18n.T("run `ryoku-host hyakki migrate` as your desktop user, then run `ryoku doctor` again"))
		}
		enabled = readHyakkiEnabled()
	}
	if err := removeLegacyAppUnits(initSystem); err != nil {
		return failRes(i18n.T("could not retire the old app services: %v"), err).
			withFix(i18n.T("stop the old app services, then run `ryoku doctor` again"))
	}
	if err := convergeHyakkiUnits(enabled, initSystem); err != nil {
		return failRes(i18n.T("could not converge Hyakki's services: %v"), err).
			withFix(i18n.T("run `hyakki on` or `hyakki off`, then run `ryoku doctor` again"))
	}
	if enabled {
		return fixedRes(i18n.T("migrated Hyakki and enabled its socket and usage collector"))
	}
	return fixedRes(i18n.T("migrated Hyakki and left it off with its services disabled"))
}
