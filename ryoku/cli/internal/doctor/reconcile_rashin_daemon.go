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
	rashindSocket    = "rashind.socket"
	rashindService   = "rashind.service"
	aiUsageTimer     = "ryoku-ai-usage.timer"
	aiUsageRunit     = "ryoku-ai-usage"
	legacyRashinUnit = "ryoku-rashin.service"
	legacyProwlUnit  = "ryoku-prowl.service"
)

var (
	rashinInit = func() (host.InitSystem, error) {
		return host.Default().Init()
	}
	rashinService = func(args []string) int {
		return host.Default().Service(args)
	}
	rashinCommand = func(args ...string) error {
		return exec.Command("rashin", args...).Run()
	}
	rashindRunning = func() bool {
		return processRunning("rashind")
	}
	stopRashind = func() error {
		err := exec.Command("pkill", "-x", "rashind").Run()
		if err != nil && rashindRunning() {
			return err
		}
		return nil
	}
)

type rashinSettings struct {
	Enabled bool `json:"enabled"`
}

func readRashinEnabled() bool {
	b, err := os.ReadFile(filepath.Join(sys.ConfigHome(), "ryoku", "rashin.json"))
	if err != nil {
		return false
	}
	var settings rashinSettings
	return json.Unmarshal(b, &settings) == nil && settings.Enabled
}

func rashinManagedUnits(initSystem host.InitSystem) []string {
	if initSystem == host.Runit {
		return []string{aiUsageRunit}
	}
	return []string{rashindSocket, aiUsageTimer}
}

func rashinUnitsMatch(enabled bool, initSystem host.InitSystem) bool {
	for _, unit := range rashinManagedUnits(initSystem) {
		isEnabled := rashinService([]string{"--user", "is-enabled", unit}) == host.ExitOK
		if isEnabled != enabled {
			return false
		}
	}
	return true
}

var migrationMarkerSafe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func rashinMigrationMarker() string {
	version := "unknown"
	if installed, ok := reportPackageVersion("ryoku-rashin"); ok && installed != "" {
		version = installed
	}
	version = strings.Trim(migrationMarkerSafe.ReplaceAllString(version, "-"), "-")
	if version == "" {
		version = "unknown"
	}
	return filepath.Join(sys.StateDir(), "rashin-migrated-"+version)
}

func rashinMigrationPending() bool {
	_, err := os.Stat(rashinMigrationMarker())
	return os.IsNotExist(err)
}

func migrateRashin() error {
	marker := rashinMigrationMarker()
	if _, err := os.Stat(marker); err == nil {
		return nil
	}
	if err := rashinCommand("migrate"); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		return err
	}
	return os.WriteFile(marker, []byte("done\n"), 0o644)
}

func legacyRashinResidue(initSystem host.InitSystem) bool {
	if initSystem == host.Runit {
		for _, service := range []string{"ryoku-rashin", "ryoku-prowl"} {
			if info, err := os.Stat(filepath.Join(sys.ConfigHome(), "service", service)); err == nil && info.IsDir() {
				return true
			}
		}
		return false
	}
	for _, unit := range []string{legacyRashinUnit, legacyProwlUnit} {
		if rashinService([]string{"--user", "is-enabled", unit}) == host.ExitOK ||
			rashinService([]string{"--user", "is-active", unit}) == host.ExitOK {
			return true
		}
		if _, err := os.Lstat(filepath.Join(sys.ConfigHome(), "systemd", "user", unit)); err == nil {
			return true
		}
	}
	return false
}

func removeLegacyRashinUnits(initSystem host.InitSystem) error {
	if initSystem == host.Runit {
		for _, service := range []string{"ryoku-rashin", "ryoku-prowl"} {
			path := filepath.Join(sys.ConfigHome(), "service", service)
			if info, err := os.Stat(path); err != nil || !info.IsDir() {
				continue
			}
			code := rashinService([]string{"--user", "stop", service})
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
	for _, unit := range []string{legacyRashinUnit, legacyProwlUnit} {
		path := filepath.Join(sys.ConfigHome(), "systemd", "user", unit)
		_, pathErr := os.Lstat(path)
		active := rashinService([]string{"--user", "is-active", unit})
		if os.IsNotExist(pathErr) && active == host.ExitAbsent {
			continue
		}
		code := rashinService([]string{"--user", "disable", "--now", unit})
		if code != host.ExitOK && code != host.ExitAbsent && code != host.ExitFalse {
			return fmt.Errorf("disable %s: service exited %d", unit, code)
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		reload = true
	}
	if reload && rashinService([]string{"--user", "daemon-reload"}) != host.ExitOK {
		return fmt.Errorf("reload user services after removing retired units")
	}
	return nil
}

func absentRashinUnits(initSystem host.InitSystem) []string {
	if initSystem == host.Runit {
		return []string{aiUsageRunit}
	}
	return []string{rashindSocket, rashindService, aiUsageTimer}
}

func rashinUnitLinks(units []string) []string {
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

func absentRashinResidue(initSystem host.InitSystem) bool {
	if rashindRunning() || legacyRashinResidue(initSystem) {
		return true
	}
	units := absentRashinUnits(initSystem)
	if initSystem == host.Systemd && len(rashinUnitLinks(units)) > 0 {
		return true
	}
	for _, unit := range units {
		if rashinService([]string{"--user", "is-enabled", unit}) == host.ExitOK ||
			rashinService([]string{"--user", "is-active", unit}) == host.ExitOK {
			return true
		}
	}
	return false
}

func cleanAbsentRashin(initSystem host.InitSystem) error {
	units := absentRashinUnits(initSystem)
	for _, unit := range units {
		active := rashinService([]string{"--user", "is-active", unit})
		enabled := rashinService([]string{"--user", "is-enabled", unit})
		code := rashinService([]string{"--user", "stop", unit})
		if code != host.ExitOK && code != host.ExitAbsent && code != host.ExitFalse &&
			active != host.ExitAbsent && active != host.ExitFalse {
			return fmt.Errorf("stop %s: service exited %d", unit, code)
		}
		code = rashinService([]string{"--user", "disable", unit})
		if code != host.ExitOK && code != host.ExitAbsent && code != host.ExitFalse &&
			enabled == host.ExitOK {
			return fmt.Errorf("disable %s: service exited %d", unit, code)
		}
	}
	if initSystem == host.Systemd {
		for _, link := range rashinUnitLinks(units) {
			if err := os.Remove(link); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		if rashinService([]string{"--user", "daemon-reload"}) != host.ExitOK {
			return fmt.Errorf("reload user services after removing Rashin")
		}
	}
	if err := removeLegacyRashinUnits(initSystem); err != nil {
		return err
	}
	if rashindRunning() {
		if err := stopRashind(); err != nil {
			return fmt.Errorf("stop rashind: %w", err)
		}
	}
	return nil
}

func convergeRashinUnits(enabled bool, initSystem host.InitSystem) error {
	verb := "enable"
	if !enabled {
		verb = "disable"
	}
	for _, unit := range rashinManagedUnits(initSystem) {
		code := rashinService([]string{"--user", verb, "--now", unit})
		if code != host.ExitOK && !enabled && (code == host.ExitAbsent || code == host.ExitFalse) {
			continue
		}
		if code != host.ExitOK {
			return fmt.Errorf("%s %s: service exited %d", verb, unit, code)
		}
	}
	if !enabled {
		if err := rashinCommand("off"); err != nil {
			return fmt.Errorf("stop rashind: %w", err)
		}
		if initSystem == host.Systemd {
			code := rashinService([]string{"--user", "stop", rashindService})
			if code != host.ExitOK && code != host.ExitAbsent && code != host.ExitFalse {
				return fmt.Errorf("stop %s: service exited %d", rashindService, code)
			}
		}
	}
	return nil
}

func reconcileRashinDaemon(checkOnly bool) recResult {
	installed := sys.Has("rashin")
	initSystem, err := rashinInit()
	if err != nil {
		return noteRes(i18n.T("could not identify the session supervisor; Rashin was not changed"))
	}
	if !installed {
		if !absentRashinResidue(initSystem) {
			return okRes(i18n.T("Rashin is not installed and no service or daemon remains"))
		}
		if checkOnly {
			return wouldRes(i18n.T("Rashin is not installed, but its services or daemon are still present")).
				withFix(i18n.T("ryoku doctor stops the daemon, disables its user services and removes dangling unit links"))
		}
		if err := cleanAbsentRashin(initSystem); err != nil {
			return failRes(i18n.T("could not remove Rashin's remaining services: %v"), err).
				withFix(i18n.T("stop the Rashin user services and daemon, then run `ryoku doctor` again"))
		}
		return fixedRes(i18n.T("stopped Rashin and removed its remaining user services"))
	}
	enabled := readRashinEnabled()
	migrationPending := rashinMigrationPending()
	legacy := legacyRashinResidue(initSystem)
	unitsMatch := rashinUnitsMatch(enabled, initSystem) && (enabled || !rashindRunning())
	if !migrationPending && !legacy && unitsMatch {
		if enabled {
			return okRes(i18n.T("Rashin is on and its socket and usage collector follow its settings"))
		}
		return okRes(i18n.T("Rashin is off with no socket, timer or daemon running"))
	}
	if checkOnly {
		return wouldRes(i18n.T("Rashin's migration and service state do not match its enabled setting")).
			withFix(i18n.T("ryoku doctor migrates Rashin and converges its socket and usage collector without turning it on"))
	}
	if migrationPending {
		if err := migrateRashin(); err != nil {
			return failRes(i18n.T("could not migrate Rashin: %v"), err).
				withFix(i18n.T("run `rashin migrate`, then run `ryoku doctor` again"))
		}
	}
	if err := removeLegacyRashinUnits(initSystem); err != nil {
		return failRes(i18n.T("could not remove the old Rashin and Prowl units: %v"), err).
			withFix(i18n.T("remove the old user units, then run `ryoku doctor` again"))
	}
	if err := convergeRashinUnits(enabled, initSystem); err != nil {
		return failRes(i18n.T("could not converge Rashin's services: %v"), err).
			withFix(i18n.T("run `rashin on` or `rashin off`, then run `ryoku doctor` again"))
	}
	if enabled {
		return fixedRes(i18n.T("migrated Rashin and enabled its socket and usage collector"))
	}
	return fixedRes(i18n.T("migrated Rashin and left it off with its services disabled"))
}
