package host

import (
	"os"
	"path/filepath"
	"testing"
)

func TestServiceEnvironmentBackends(t *testing.T) {
	runner := &fakeRunner{}
	app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_INIT": "systemd"})
	if code := app.Service([]string{"env", "DISPLAY=:1", "WAYLAND_DISPLAY"}); code != ExitOK {
		t.Fatal(code)
	}
	if got := argv(runner.commands[0]); got != "dbus-update-activation-environment --systemd DISPLAY=:1 WAYLAND_DISPLAY" {
		t.Fatalf("argv = %q", got)
	}

	dir := t.TempDir()
	runner = &fakeRunner{}
	app, _, _ = testApp(runner, map[string]string{"RYOKU_HOST_INIT": "runit", "WAYLAND_DISPLAY": "wayland-2"})
	app.cfg.TurnstileEnvDir = dir
	if code := app.Service([]string{"env", "DISPLAY=:2", "WAYLAND_DISPLAY"}); code != ExitOK {
		t.Fatal(code)
	}
	for path, want := range map[string]string{"DISPLAY": ":2", "WAYLAND_DISPLAY": "wayland-2"} {
		body, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != want {
			t.Fatalf("%s = %q", path, body)
		}
	}
	if got := argv(runner.commands[0]); got != "dbus-update-activation-environment DISPLAY=:2 WAYLAND_DISPLAY" {
		t.Fatalf("argv = %q", got)
	}
}

func TestInhibitPreferenceAndAbsence(t *testing.T) {
	for _, tc := range []struct {
		name  string
		paths map[string]bool
		want  string
		code  int
	}{
		{"systemd preferred", map[string]bool{"systemd-inhibit": true, "elogind-inhibit": true}, "systemd-inhibit --what=sleep cat", ExitOK},
		{"elogind fallback", map[string]bool{"elogind-inhibit": true}, "elogind-inhibit --what=sleep cat", ExitOK},
		{"neither", map[string]bool{}, "", ExitAbsent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &fakeRunner{paths: tc.paths}
			app, _, _ := testApp(runner, nil)
			if code := app.Inhibit([]string{"--what=sleep", "cat"}); code != tc.code {
				t.Fatal(code)
			}
			if tc.want != "" && argv(runner.commands[0]) != tc.want {
				t.Fatalf("argv = %q", argv(runner.commands[0]))
			}
		})
	}
}

func TestSystemdTransientAllFlags(t *testing.T) {
	runner := &fakeRunner{}
	app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_INIT": "systemd"})
	args := []string{"start", "job", "--slice", "background.slice", "--prop", "Type=exec", "--prop", "TimeoutStopSec=5s", "--env", "A=B", "--", "echo", "ok"}
	if code := app.Transient(args); code != ExitOK {
		t.Fatal(code)
	}
	want := "systemd-run --user --unit=job --slice=background.slice -p Type=exec -p TimeoutStopSec=5s -E A=B --collect --quiet -- echo ok"
	if got := argv(runner.commands[0]); got != want {
		t.Fatalf("argv = %q, want %q", got, want)
	}

	runner.commands = nil
	app.cfg.RuntimeDir = t.TempDir()
	if code := app.Transient([]string{"start", "foreground", "--scope", "--prop", "P=V", "--", "sleep", "1"}); code != ExitOK {
		t.Fatal(code)
	}
	if got := argv(runner.commands[0]); got != "systemd-run --user --unit=foreground --scope -p P=V --quiet -- sleep 1" {
		t.Fatalf("scope argv = %q", got)
	}
	runner.commands = nil
	if code := app.Transient([]string{"is-active", "foreground"}); code != ExitOK {
		t.Fatal(code)
	}
	if got := argv(runner.commands[0]); got != "systemctl --user is-active --quiet foreground.scope" {
		t.Fatalf("state argv = %q", got)
	}
}

func TestRunitTransientLifecycle(t *testing.T) {
	dir := t.TempDir()
	runner := &fakeRunner{results: []Result{{PID: 42}}, alive: map[int]bool{42: true}}
	app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_INIT": "runit"})
	app.cfg.RuntimeDir = dir
	if code := app.Transient([]string{"start", "job", "--env", "A=B", "--", "worker", "x"}); code != ExitOK {
		t.Fatal(code)
	}
	if !runner.commands[0].Detached || runner.commands[0].AppendPath == "" {
		t.Fatal("job was not detached with a log")
	}
	if code := app.Transient([]string{"is-active", "job"}); code != ExitOK {
		t.Fatal(code)
	}
	if code := app.Transient([]string{"stop", "job"}); code != ExitOK {
		t.Fatal(code)
	}
	if len(runner.signals) != 1 || runner.signals[0].pid != 42 {
		t.Fatalf("signals = %#v", runner.signals)
	}
}

func TestSessionStartChainAndFailureSemantics(t *testing.T) {
	runner := &fakeRunner{results: []Result{{Code: 1}, {Code: 1}, {Code: 1}, {}, {}}}
	app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_INIT": "systemd"})
	if code := app.Session([]string{"start", "xdg-desktop-portal-gtk.service"}); code != ExitOK {
		t.Fatal(code)
	}
	want := []string{
		"dbus-update-activation-environment --systemd --all",
		"systemctl --user daemon-reload",
		"ryoku-reload-cover begin boot",
		"ryoku-power-cutover session-start-logged",
		"systemctl --user try-restart xdg-desktop-portal.service xdg-desktop-portal-gtk.service",
	}
	for index := range want {
		if got := argv(runner.commands[index]); got != want[index] {
			t.Fatalf("call %d = %q", index, got)
		}
	}

	runner = &fakeRunner{results: []Result{{}, {}, {}, {Code: 1}}}
	app, _, _ = testApp(runner, map[string]string{"RYOKU_HOST_INIT": "systemd"})
	if code := app.Session([]string{"start"}); code != ExitFailure {
		t.Fatalf("cutover exit = %d", code)
	}
	if len(runner.commands) != 4 {
		t.Fatalf("portal restart ran after failed cutover")
	}
}

func TestRunitSessionExecsTranslatedEntrypoint(t *testing.T) {
	runner := &fakeRunner{}
	app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_INIT": "runit", "RYOKU_INIT_LIB": "/test/runit"})
	if code := app.Session([]string{"start", "ignored-portal.service"}); code != ExitOK {
		t.Fatal(code)
	}
	command := runner.commands[0]
	if command.Name != "/test/runit/session-start" || !command.Replace || len(command.Args) != 0 {
		t.Fatalf("session command = %#v", command)
	}
}
