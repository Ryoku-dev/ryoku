package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"ryoku-cli/internal/host"
)

func TestReadHyakkiEnabledRequiresExplicitTrue(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	path := filepath.Join(config, "hyakki", "hyakki.json")
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
			if got := readHyakkiEnabled(); got != test.want {
				t.Fatalf("readHyakkiEnabled() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestHyakkiManagedUnitsFollowInitSystem(t *testing.T) {
	systemd := hyakkiManagedUnits(host.Systemd)
	if len(systemd) != 2 || systemd[0] != oniSocket || systemd[1] != aiUsageTimer {
		t.Fatalf("systemd units = %v", systemd)
	}
	runit := hyakkiManagedUnits(host.Runit)
	if len(runit) != 1 || runit[0] != aiUsageRunit {
		t.Fatalf("runit units = %v", runit)
	}
}

func TestHyakkiUnitsMatchEnabledSetting(t *testing.T) {
	old := hyakkiService
	t.Cleanup(func() { hyakkiService = old })
	enabled := map[string]bool{oniSocket: true, aiUsageTimer: true}
	hyakkiService = func(args []string) int {
		if enabled[args[len(args)-1]] {
			return host.ExitOK
		}
		return host.ExitFalse
	}
	if !hyakkiUnitsMatch(true, host.Systemd) {
		t.Fatal("enabled units did not match enabled settings")
	}
	if hyakkiUnitsMatch(false, host.Systemd) {
		t.Fatal("enabled units matched disabled settings")
	}
	enabled[aiUsageTimer] = false
	if hyakkiUnitsMatch(true, host.Systemd) {
		t.Fatal("partially enabled units matched enabled settings")
	}
	enabled[oniSocket] = false
	if !hyakkiUnitsMatch(false, host.Systemd) {
		t.Fatal("disabled units did not match disabled settings")
	}
	// the routines wake timer follows the daemon while Hyakki is on, and must
	// be off with it
	enabled[oniSocket], enabled[aiUsageTimer], enabled[routinesTimer] = true, true, true
	if !hyakkiUnitsMatch(true, host.Systemd) {
		t.Fatal("an enabled routines timer broke enabled settings")
	}
	enabled[oniSocket], enabled[aiUsageTimer] = false, false
	if hyakkiUnitsMatch(false, host.Systemd) {
		t.Fatal("an enabled routines timer matched disabled settings")
	}
}

func TestRemoveLegacyAppUnitsSkipsMissingUnits(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	old := hyakkiService
	t.Cleanup(func() { hyakkiService = old })
	disableCalls := 0
	hyakkiService = func(args []string) int {
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
	if err := removeLegacyAppUnits(host.Systemd); err != nil {
		t.Fatalf("missing legacy units caused an error: %v", err)
	}
	if disableCalls != 0 {
		t.Fatalf("disable called %d times for missing units", disableCalls)
	}
}

func TestRemoveLegacyAppUnitsStopsAndRemovesRunitServices(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	root := filepath.Join(config, "service")
	for _, service := range []string{"ryoku-rashin", "ryoku-prowl"} {
		if err := os.MkdirAll(filepath.Join(root, service), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	old := hyakkiService
	t.Cleanup(func() { hyakkiService = old })
	var stopped []string
	hyakkiService = func(args []string) int {
		if len(args) != 3 || args[1] != "stop" {
			t.Fatalf("unexpected service call: %v", args)
		}
		if _, err := os.Stat(filepath.Join(root, args[2])); err != nil {
			t.Fatalf("%s was removed before it was stopped: %v", args[2], err)
		}
		stopped = append(stopped, args[2])
		return host.ExitOK
	}
	if err := removeLegacyAppUnits(host.Runit); err != nil {
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

func TestReconcileAbsentHyakkiStopsServicesAndRemovesLinks(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("PATH", t.TempDir())
	root := filepath.Join(config, "systemd", "user")
	socketLink := filepath.Join(root, "sockets.target.wants", oniSocket)
	timerLink := filepath.Join(root, "timers.target.wants", aiUsageTimer)
	for _, link := range []string{socketLink, timerLink} {
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join("/usr/lib/systemd/user", filepath.Base(link)), link); err != nil {
			t.Fatal(err)
		}
	}

	oldInit, oldService := hyakkiInit, hyakkiService
	oldRunning, oldStop, oldCommand := oniRunning, stopOni, hyakkiCommand
	t.Cleanup(func() {
		hyakkiInit, hyakkiService = oldInit, oldService
		oniRunning, stopOni, hyakkiCommand = oldRunning, oldStop, oldCommand
	})
	hyakkiInit = func() (host.InitSystem, error) { return host.Systemd, nil }
	active := true
	attempted := map[string]map[string]bool{"stop": {}, "disable": {}}
	hyakkiService = func(args []string) int {
		verb := args[1]
		unit := ""
		if len(args) > 2 {
			unit = args[len(args)-1]
		}
		switch verb {
		case "is-active":
			if unit == oniService && active {
				return host.ExitOK
			}
			for _, retired := range retiredAppUnits(host.Systemd) {
				if unit == retired {
					return host.ExitAbsent
				}
			}
			return host.ExitFalse
		case "is-enabled":
			return host.ExitFalse
		case "stop":
			attempted[verb][unit] = true
			if unit == oniService {
				active = false
				return host.ExitOK
			}
			return host.ExitAbsent
		case "disable":
			attempted[verb][unit] = true
			if unit == oniService {
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
	oniRunning = func() bool { return running }
	stopOni = func() error {
		running = false
		return nil
	}
	hyakkiCommand = func(args ...string) error {
		t.Fatalf("absent-package cleanup ran hyakki: %v", args)
		return nil
	}

	if result := reconcileHyakkiDaemon(false); result.status != recFixed {
		t.Fatalf("cleanup result = %+v", result)
	}
	for _, unit := range []string{oniSocket, oniService, aiUsageTimer, routinesTimer} {
		if !attempted["stop"][unit] || !attempted["disable"][unit] {
			t.Fatalf("%s was not stopped and disabled: %v", unit, attempted)
		}
	}
	for _, link := range []string{socketLink, timerLink} {
		if _, err := os.Lstat(link); !os.IsNotExist(err) {
			t.Fatalf("dangling link remains at %s: %v", link, err)
		}
	}
	if result := reconcileHyakkiDaemon(false); result.status != recOK {
		t.Fatalf("second cleanup result = %+v", result)
	}
}
