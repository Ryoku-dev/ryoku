package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"ryoku-cli/internal/host"
)

func TestReadRashinEnabledRequiresExplicitTrue(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	path := filepath.Join(config, "ryoku", "rashin.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		body string
		want bool
	}{
		{name: "missing", want: false},
		{name: "enabled", body: `{"enabled":true}`, want: true},
		{name: "disabled", body: `{"enabled":false}`, want: false},
		{name: "old opt out", body: `{"enabled":true,"optedOut":true}`, want: true},
		{name: "invalid", body: `not json`, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			_ = os.Remove(path)
			if test.body != "" {
				if err := os.WriteFile(path, []byte(test.body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if got := readRashinEnabled(); got != test.want {
				t.Fatalf("readRashinEnabled() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestRashinManagedUnitsFollowInitSystem(t *testing.T) {
	systemd := rashinManagedUnits(host.Systemd)
	if len(systemd) != 2 || systemd[0] != rashindSocket || systemd[1] != aiUsageTimer {
		t.Fatalf("systemd units = %v", systemd)
	}
	runit := rashinManagedUnits(host.Runit)
	if len(runit) != 1 || runit[0] != aiUsageRunit {
		t.Fatalf("runit units = %v", runit)
	}
}

func TestRashinUnitsMatchEnabledSetting(t *testing.T) {
	old := rashinService
	t.Cleanup(func() { rashinService = old })
	enabled := map[string]bool{rashindSocket: true, aiUsageTimer: true}
	rashinService = func(args []string) int {
		if enabled[args[len(args)-1]] {
			return host.ExitOK
		}
		return host.ExitFalse
	}
	if !rashinUnitsMatch(true, host.Systemd) {
		t.Fatal("enabled units did not match enabled settings")
	}
	if rashinUnitsMatch(false, host.Systemd) {
		t.Fatal("enabled units matched disabled settings")
	}
	enabled[aiUsageTimer] = false
	if rashinUnitsMatch(true, host.Systemd) {
		t.Fatal("partially enabled units matched enabled settings")
	}
	enabled[rashindSocket] = false
	if !rashinUnitsMatch(false, host.Systemd) {
		t.Fatal("disabled units did not match disabled settings")
	}
}

func TestRemoveLegacyRashinUnitsSkipsMissingUnits(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	old := rashinService
	t.Cleanup(func() { rashinService = old })
	disableCalls := 0
	rashinService = func(args []string) int {
		switch args[1] {
		case "is-active":
			return host.ExitAbsent
		case "disable":
			disableCalls++
			return host.ExitFailure
		default:
			t.Fatalf("unexpected service call: %v", args)
			return host.ExitFailure
		}
	}
	if err := removeLegacyRashinUnits(host.Systemd); err != nil {
		t.Fatalf("missing legacy units caused an error: %v", err)
	}
	if disableCalls != 0 {
		t.Fatalf("disable called %d times for missing units", disableCalls)
	}
}

func TestRemoveLegacyRashinUnitsStopsAndRemovesRunitServices(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	root := filepath.Join(config, "service")
	for _, service := range []string{"ryoku-rashin", "ryoku-prowl"} {
		if err := os.MkdirAll(filepath.Join(root, service), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	old := rashinService
	t.Cleanup(func() { rashinService = old })
	var stopped []string
	rashinService = func(args []string) int {
		if len(args) != 3 || args[1] != "stop" {
			t.Fatalf("unexpected service call: %v", args)
		}
		if _, err := os.Stat(filepath.Join(root, args[2])); err != nil {
			t.Fatalf("%s was removed before it was stopped: %v", args[2], err)
		}
		stopped = append(stopped, args[2])
		return host.ExitOK
	}
	if err := removeLegacyRashinUnits(host.Runit); err != nil {
		t.Fatal(err)
	}
	if len(stopped) != 2 || stopped[0] != "ryoku-rashin" || stopped[1] != "ryoku-prowl" {
		t.Fatalf("stopped services = %v", stopped)
	}
	for _, service := range stopped {
		if _, err := os.Stat(filepath.Join(root, service)); !os.IsNotExist(err) {
			t.Fatalf("%s remains after cleanup: %v", service, err)
		}
	}
}

func TestReconcileAbsentRashinStopsServicesAndRemovesLinks(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("PATH", t.TempDir())
	root := filepath.Join(config, "systemd", "user")
	socketLink := filepath.Join(root, "sockets.target.wants", rashindSocket)
	timerLink := filepath.Join(root, "timers.target.wants", aiUsageTimer)
	for _, link := range []string{socketLink, timerLink} {
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join("/usr/lib/systemd/user", filepath.Base(link)), link); err != nil {
			t.Fatal(err)
		}
	}

	oldInit, oldService := rashinInit, rashinService
	oldRunning, oldStop, oldCommand := rashindRunning, stopRashind, rashinCommand
	t.Cleanup(func() {
		rashinInit, rashinService = oldInit, oldService
		rashindRunning, stopRashind, rashinCommand = oldRunning, oldStop, oldCommand
	})
	rashinInit = func() (host.InitSystem, error) { return host.Systemd, nil }
	active := true
	attempted := map[string]map[string]bool{"stop": {}, "disable": {}}
	rashinService = func(args []string) int {
		verb := args[1]
		unit := ""
		if len(args) > 2 {
			unit = args[len(args)-1]
		}
		switch verb {
		case "is-active":
			if unit == rashindService && active {
				return host.ExitOK
			}
			if unit == legacyRashinUnit || unit == legacyProwlUnit {
				return host.ExitAbsent
			}
			return host.ExitFalse
		case "is-enabled":
			return host.ExitFalse
		case "stop":
			attempted[verb][unit] = true
			if unit == rashindService {
				active = false
				return host.ExitOK
			}
			return host.ExitAbsent
		case "disable":
			attempted[verb][unit] = true
			if unit == rashindService {
				return host.ExitOK
			}
			return host.ExitFailure
		case "daemon-reload":
			return host.ExitOK
		default:
			t.Fatalf("unexpected service call: %v", args)
			return host.ExitFailure
		}
	}
	running := true
	rashindRunning = func() bool { return running }
	stopRashind = func() error {
		running = false
		return nil
	}
	rashinCommand = func(args ...string) error {
		t.Fatalf("absent-package cleanup ran rashin: %v", args)
		return nil
	}

	if result := reconcileRashinDaemon(false); result.status != recFixed {
		t.Fatalf("cleanup result = %+v", result)
	}
	for _, unit := range []string{rashindSocket, rashindService, aiUsageTimer} {
		if !attempted["stop"][unit] || !attempted["disable"][unit] {
			t.Fatalf("%s was not stopped and disabled: %v", unit, attempted)
		}
	}
	for _, link := range []string{socketLink, timerLink} {
		if _, err := os.Lstat(link); !os.IsNotExist(err) {
			t.Fatalf("dangling link remains at %s: %v", link, err)
		}
	}
	if result := reconcileRashinDaemon(false); result.status != recOK {
		t.Fatalf("second cleanup result = %+v", result)
	}
}
