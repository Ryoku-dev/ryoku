package host

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTimeSetZoneValidationAndWrite(t *testing.T) {
	dir := t.TempDir()
	zoneRoot := filepath.Join(dir, "zoneinfo")
	if err := os.MkdirAll(filepath.Join(zoneRoot, "Europe"), 0o755); err != nil {
		t.Fatal(err)
	}
	zone := filepath.Join(zoneRoot, "Europe", "Paris")
	if err := os.WriteFile(zone, []byte("TZif"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_INIT": "runit"})
	app.cfg.ZoneinfoDir, app.cfg.LocaltimePath = zoneRoot, filepath.Join(dir, "localtime")
	app.cfg.UID, app.cfg.UIDSet = 0, true
	for _, invalid := range []string{"../etc/passwd", "/Europe/Paris", "Europe/Missing", "Europe/../Paris"} {
		if code := app.Time([]string{"set-zone", invalid}); code != ExitUsage {
			t.Fatalf("%q exit = %d", invalid, code)
		}
	}
	if code := app.Time([]string{"set-zone", "Europe/Paris"}); code != ExitOK {
		t.Fatal(code)
	}
	target, err := os.Readlink(app.cfg.LocaltimePath)
	if err != nil {
		t.Fatal(err)
	}
	if target != zone {
		t.Fatalf("target = %q", target)
	}
	if code := app.Time([]string{"zone"}); code != ExitOK {
		t.Fatal(code)
	}
}

func TestTimeAndKeymapSystemdArgv(t *testing.T) {
	runner := &fakeRunner{}
	app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_INIT": "systemd"})
	if code := app.Time([]string{"set-zone", "Europe/Paris"}); code != ExitOK {
		t.Fatal(code)
	}
	if got := argv(runner.commands[0]); got != "timedatectl set-timezone Europe/Paris" {
		t.Fatalf("argv = %q", got)
	}
	if code := app.Keymap([]string{"set", "de", "nodeadkeys", "ctrl:nocaps"}); code != ExitOK {
		t.Fatal(code)
	}
	if got := argv(runner.commands[1]); got != "localectl set-x11-keymap de  nodeadkeys ctrl:nocaps" {
		t.Fatalf("argv = %q", got)
	}
}

func TestRunitKeymapWritesRCConfAndEscalates(t *testing.T) {
	dir := t.TempDir()
	rc := filepath.Join(dir, "rc.conf")
	if err := os.WriteFile(rc, []byte("KEYMAP=\"us\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_INIT": "runit"})
	app.cfg.RCConfPath, app.cfg.UID, app.cfg.UIDSet = rc, 0, true
	if code := app.Keymap([]string{"set", "fr"}); code != ExitOK {
		t.Fatal(code)
	}
	body, err := os.ReadFile(rc)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "KEYMAP=\"fr\"\n" {
		t.Fatalf("rc.conf = %q", body)
	}

	app.cfg.UID = 1000
	if code := app.Keymap([]string{"set", "gb", "", "ctrl:nocaps"}); code != ExitOK {
		t.Fatal(code)
	}
	if got := argv(runner.commands[0]); got != "pkexec ryoku-host keymap set gb  ctrl:nocaps" {
		t.Fatalf("pkexec argv = %q", got)
	}
}

func TestUsageListsEveryTopLevelVerbAndFlag(t *testing.T) {
	for _, text := range []string{"init", "pkgmgr", "svc", "reload", "--now", "env", "--all", "inhibit", "transient", "--scope", "--slice", "--prop", "--env", "session", "pkg", "--upgrade", "--aur", "--foreign", "time", "keymap"} {
		if !strings.Contains(Usage, text) {
			t.Fatalf("usage misses %q", text)
		}
	}
}
