package host

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectionAndOverrides(t *testing.T) {
	dir := t.TempDir()
	runner := &fakeRunner{paths: map[string]bool{"pacman": true, "xbps-install": true}}
	app, _, _ := testApp(runner, map[string]string{})
	app.cfg.SystemdRuntimeDir = filepath.Join(dir, "missing")
	if got, _ := app.Init(); got != Runit {
		t.Fatalf("Init = %q", got)
	}
	if err := os.Mkdir(app.cfg.SystemdRuntimeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, _ := app.Init(); got != Systemd {
		t.Fatalf("Init = %q", got)
	}
	if got, _ := app.PackageManager(); got != Pacman {
		t.Fatalf("PackageManager = %q", got)
	}

	override, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_INIT": "runit", "RYOKU_HOST_PKGMGR": "xbps"})
	if got, _ := override.Init(); got != Runit {
		t.Fatalf("override Init = %q", got)
	}
	if got, _ := override.PackageManager(); got != XBPS {
		t.Fatalf("override PackageManager = %q", got)
	}
}

func TestSystemdServiceArgv(t *testing.T) {
	verbs := []string{"start", "stop", "restart", "try-restart", "reload", "is-active", "is-enabled", "enable", "disable", "kill", "reset-failed"}
	for _, verb := range verbs {
		t.Run(verb, func(t *testing.T) {
			runner := &fakeRunner{}
			app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_INIT": "systemd"})
			args := []string{"--user", verb}
			if verb == "enable" {
				args = append(args, "--now")
			}
			args = append(args, "pipewire", "libvirtd.socket")
			if code := app.Service(args); code != ExitOK {
				t.Fatalf("exit = %d", code)
			}
			want := "systemctl --user " + verb
			if verb == "enable" {
				want += " --now"
			}
			if verb == "is-active" || verb == "is-enabled" {
				want += " --quiet"
			}
			want += " pipewire.service libvirtd.socket"
			if got := argv(runner.commands[0]); got != want {
				t.Fatalf("argv = %q, want %q", got, want)
			}
		})
	}
	runner := &fakeRunner{}
	app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_INIT": "systemd"})
	if code := app.Service([]string{"--system", "daemon-reload"}); code != ExitOK {
		t.Fatal(code)
	}
	if got := argv(runner.commands[0]); got != "systemctl daemon-reload" {
		t.Fatalf("argv = %q", got)
	}
}

func TestRunitUserServiceStateAndReload(t *testing.T) {
	dir := t.TempDir()
	service := filepath.Join(dir, "service", "bluetoothd")
	if err := os.MkdirAll(filepath.Join(service, "supervise"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(service, "supervise", "stat"), []byte("run\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{answer: func(command Command) Result {
		if len(command.Args) > 0 && command.Args[0] == "status" {
			return Result{Output: "run: bluetoothd: (pid 12) 10s\n"}
		}
		return Result{}
	}}
	app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_INIT": "runit"})
	app.cfg.UserServiceDir = filepath.Join(dir, "service")
	if code := app.Service([]string{"--user", "is-active", "bluetooth"}); code != ExitOK {
		t.Fatal(code)
	}
	if code := app.Service([]string{"--user", "disable", "--now", "bluetooth"}); code != ExitOK {
		t.Fatal(code)
	}
	if _, err := os.Stat(filepath.Join(service, "down")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(service, runitUserDisabledMarker)); err != nil {
		t.Fatal(err)
	}
	if code := app.Service([]string{"--user", "is-enabled", "bluetooth"}); code != ExitFalse {
		t.Fatalf("disabled exit = %d", code)
	}
	if code := app.Service([]string{"--user", "enable", "bluetooth"}); code != ExitOK {
		t.Fatal(code)
	}
	for _, name := range []string{"down", runitUserDisabledMarker} {
		if _, err := os.Stat(filepath.Join(service, name)); !os.IsNotExist(err) {
			t.Fatalf("%s remained after enable: %v", name, err)
		}
	}
	if code := app.Service([]string{"--user", "is-enabled", "bluetooth"}); code != ExitOK {
		t.Fatalf("enabled exit = %d", code)
	}
	if err := os.WriteFile(filepath.Join(service, "down"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if code := app.Service([]string{"--user", "is-enabled", "bluetooth"}); code != ExitOK {
		t.Fatalf("committed down marker reported disabled: %d", code)
	}
	if code := app.Service([]string{"--user", "reload", "bluetooth"}); code != ExitOK {
		t.Fatal(code)
	}
	if got := argv(runner.commands[len(runner.commands)-1]); got != "sv hup "+service {
		t.Fatalf("reload argv = %q", got)
	}
	if code := app.Service([]string{"--user", "start", "missing"}); code != ExitAbsent {
		t.Fatalf("missing exit = %d", code)
	}
}

func TestRunitSystemEnableAndInactive(t *testing.T) {
	dir := t.TempDir()
	definition := filepath.Join(dir, "sv", "docker")
	if err := os.MkdirAll(filepath.Join(definition, "supervise"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(definition, "supervise", "stat"), []byte("down\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{answer: func(command Command) Result {
		if len(command.Args) > 0 && command.Args[0] == "status" {
			return Result{Output: "down: docker: 1s\n"}
		}
		return Result{}
	}}
	app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_INIT": "runit"})
	app.cfg.SystemServiceDir = filepath.Join(dir, "sv")
	app.cfg.SystemBootDir = filepath.Join(dir, "boot")
	app.cfg.SystemLiveDir = filepath.Join(dir, "live")
	if code := app.Service([]string{"--system", "enable", "--now", "docker"}); code != ExitOK {
		t.Fatal(code)
	}
	if _, err := os.Lstat(filepath.Join(app.cfg.SystemBootDir, "docker")); err != nil {
		t.Fatal(err)
	}
	if code := app.Service([]string{"--system", "is-active", "docker"}); code != ExitFalse {
		t.Fatalf("inactive exit = %d", code)
	}
	if code := app.Service([]string{"--system", "start", "missing"}); code != ExitAbsent {
		t.Fatalf("missing exit = %d", code)
	}
}
