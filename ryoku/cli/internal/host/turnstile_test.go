package host

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func turnstileTestApp(t *testing.T, init string, runner *fakeRunner) (*App, *bytes.Buffer, *bytes.Buffer, string, string) {
	t.Helper()
	root, home := t.TempDir(), t.TempDir()
	oldRoot, oldNow := turnstileFSRoot, turnstileNow
	oldSockets, oldPeer := turnstileSocketPaths, turnstileSocketPeerPID
	turnstileFSRoot = root
	turnstileNow = func() time.Time { return time.Date(2026, 10, 9, 12, 30, 0, 0, time.UTC) }
	t.Cleanup(func() {
		turnstileFSRoot, turnstileNow = oldRoot, oldNow
		turnstileSocketPaths, turnstileSocketPeerPID = oldSockets, oldPeer
	})
	app, stdout, stderr := testApp(runner, map[string]string{"RYOKU_HOST_INIT": init, "HOME": home})
	app.cfg.UID, app.cfg.UIDSet = 0, true
	return app, stdout, stderr, root, home
}

func writeFixture(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTurnstileSystemEnsureConfigPAMAndIdempotence(t *testing.T) {
	app, stdout, _, root, _ := turnstileTestApp(t, "runit", &fakeRunner{})
	pam := filepath.Join(root, "etc/pam.d/system-login")
	original := "auth required pam_unix.so\nsession required pam_limits.so\nsession optional pam_elogind.so\n"
	writeFixture(t, pam, original)
	conf := filepath.Join(root, "etc/turnstile/turnstiled.conf")
	writeFixture(t, conf, "manage_rundir = yes\nexport_dbus_address = no\nkeep = value\n")

	if code := app.SessionEnsure("--system"); code != ExitOK {
		t.Fatal(code)
	}
	for _, name := range []string{"dbus", "turnstiled"} {
		target, err := os.Readlink(filepath.Join(root, "etc/runit/runsvdir/default", name))
		if err != nil {
			t.Fatal(err)
		}
		if target != filepath.Join(root, "etc/sv", name) {
			t.Fatalf("%s target = %q", name, target)
		}
	}
	body, _ := os.ReadFile(conf)
	if !strings.Contains(string(body), "manage_rundir = no") || !strings.Contains(string(body), "export_dbus_address = yes") || !strings.Contains(string(body), "keep = value") {
		t.Fatalf("conf = %q", body)
	}
	body, _ = os.ReadFile(pam)
	want := "auth required pam_unix.so\nsession required pam_limits.so\n-session optional pam_turnstile.so\nsession optional pam_elogind.so\n"
	if string(body) != want {
		t.Fatalf("pam = %q", body)
	}
	backup, err := os.ReadFile(pam + ".ryoku-bak")
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != original {
		t.Fatalf("backup = %q", backup)
	}

	stdout.Reset()
	if code := app.SessionEnsure("--system"); code != ExitOK {
		t.Fatal(code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("second ensure reported changes: %q", stdout.String())
	}
	backup2, _ := os.ReadFile(pam + ".ryoku-bak")
	if string(backup2) != original {
		t.Fatal("backup was overwritten")
	}
}

func TestTurnstileConfigAddsMissingKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "turnstiled.conf")
	writeFixture(t, path, "keep = value\n")
	changed, err := ensureTurnstileConfig(path)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	body, _ := os.ReadFile(path)
	for _, line := range []string{"keep = value", "manage_rundir = no", "export_dbus_address = yes"} {
		if !strings.Contains(string(body), line) {
			t.Fatalf("missing %q in %q", line, body)
		}
	}
}

func TestTurnstilePAMExistingEntryIsUntouchedAndFallbackPosition(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing")
	body := "session optional pam_turnstile.so\nsession optional pam_elogind.so\n"
	writeFixture(t, existing, body)
	changed, err := ensureTurnstilePAM(existing)
	if err != nil || changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	got, _ := os.ReadFile(existing)
	if string(got) != body {
		t.Fatal("existing PAM file changed")
	}
	if _, err := os.Stat(existing + ".ryoku-bak"); !os.IsNotExist(err) {
		t.Fatal("existing entry created a backup")
	}

	fallback := filepath.Join(dir, "fallback")
	writeFixture(t, fallback, "auth required pam_unix.so\nsession required pam_limits.so\naccount required pam_unix.so\n")
	if changed, err := ensureTurnstilePAM(fallback); err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	got, _ = os.ReadFile(fallback)
	if string(got) != "auth required pam_unix.so\nsession required pam_limits.so\n-session optional pam_turnstile.so\naccount required pam_unix.so\n" {
		t.Fatalf("fallback PAM = %q", got)
	}
}

func TestTurnstileUserEnsureMergesCoreServices(t *testing.T) {
	app, stdout, _, _, home := turnstileTestApp(t, "runit", &fakeRunner{})
	ready := filepath.Join(home, ".config/service/turnstile-ready/conf")
	writeFixture(t, ready, "core_services=\"pipewire custom\"\n")
	if code := app.SessionEnsure("--user"); code != ExitOK {
		t.Fatal(code)
	}
	body, _ := os.ReadFile(ready)
	if string(body) != "core_services=\"pipewire custom dbus\"\n" {
		t.Fatalf("core services = %q", body)
	}
	for _, name := range []string{"run", "check"} {
		target, err := os.Readlink(filepath.Join(home, ".config/service/dbus", name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(target, "dbus."+name) {
			t.Fatalf("%s target = %q", name, target)
		}
	}
	if info, err := os.Stat(filepath.Join(home, ".config/service-env")); err != nil || !info.IsDir() {
		t.Fatalf("envdir: %v", err)
	}
	stdout.Reset()
	if code := app.SessionEnsure("--user"); code != ExitOK {
		t.Fatal(code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("second ensure = %q", stdout.String())
	}
}

func TestTurnstileCheckReportsEveryProblemClass(t *testing.T) {
	runner := &fakeRunner{answer: func(command Command) Result {
		if command.Name == "pgrep" {
			return Result{Code: 1}
		}
		return Result{}
	}}
	app, stdout, _, root, home := turnstileTestApp(t, "runit", runner)
	writeFixture(t, filepath.Join(root, "etc/turnstile/turnstiled.conf"), "manage_rundir = yes\nexport_dbus_address = no\n")
	writeFixture(t, filepath.Join(root, "etc/pam.d/system-login"), "session optional pam_elogind.so\n")
	writeFixture(t, filepath.Join(home, ".profile"), "exec dbus-run-session compositor-x --session\n")
	runtime := filepath.Join(home, "runtime")
	writeFixture(t, filepath.Join(runtime, "bus"), "")
	app.cfg.Getenv = func(key string) string {
		if key == "HOME" {
			return home
		}
		if key == "RYOKU_HOST_INIT" {
			return "runit"
		}
		if key == "XDG_RUNTIME_DIR" {
			return runtime
		}
		return ""
	}
	turnstileSocketPaths = func(string, string) []string { return []string{filepath.Join(runtime, "wayland-9")} }
	turnstileSocketPeerPID = func(string) (int, error) { return 103, nil }
	for pid, fixture := range map[string]struct{ comm, cmd, env string }{
		"101": {"dbus-daemon", "dbus-daemon\x00--session\x00", ""},
		"102": {"dbus-daemon", "dbus-daemon\x00--session\x00", ""},
		"103": {"compositor-x", "compositor-x\x00--session\x00", "DBUS_SESSION_BUS_ADDRESS=unix:path=/wrong\x00"},
	} {
		base := filepath.Join(root, "proc", pid)
		writeFixture(t, filepath.Join(base, "comm"), fixture.comm+"\n")
		writeFixture(t, filepath.Join(base, "cmdline"), fixture.cmd)
		writeFixture(t, filepath.Join(base, "environ"), fixture.env)
	}
	if code := app.SessionCheck(); code != ExitFalse {
		t.Fatalf("check exit = %d", code)
	}
	for _, id := range []string{"turnstile-boot-dbus", "turnstile-boot-turnstiled", "turnstile-not-running", "turnstile-rundir", "turnstile-dbus-export", "turnstile-pam", "turnstile-user-dbus-run", "turnstile-user-dbus-check", "turnstile-ready-dbus", "turnstile-wrapper", "turnstile-extra-bus", "turnstile-compositor-bus"} {
		if !strings.Contains(stdout.String(), id+":") {
			t.Errorf("missing finding %s in %q", id, stdout.String())
		}
	}
}

func TestStripSessionWrappersInsideShellGuards(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		changed bool
	}{
		{
			name:    "tty guard",
			input:   `[ "$(tty)" = /dev/tty1 ] && exec dbus-launch --exit-with-session compositor-x --session`,
			want:    `[ "$(tty)" = /dev/tty1 ] && exec compositor-x --session`,
			changed: true,
		},
		{
			name:    "if then guard",
			input:   `if [ -z "$WAYLAND_DISPLAY" ] && [ "$XDG_VTNR" -eq 1 ]; then exec dbus-run-session compositor-x; fi`,
			want:    `if [ -z "$WAYLAND_DISPLAY" ] && [ "$XDG_VTNR" -eq 1 ]; then exec compositor-x; fi`,
			changed: true,
		},
		{
			name:    "test guard",
			input:   `test -z "$DISPLAY" && dbus-run-session compositor-x`,
			want:    `test -z "$DISPLAY" && compositor-x`,
			changed: true,
		},
		{
			name:    "any command is unwrapped",
			input:   `test -z "$DISPLAY" && dbus-run-session session-tool`,
			want:    `test -z "$DISPLAY" && session-tool`,
			changed: true,
		},
		{
			name:    "wrapper is an argument",
			input:   `printf '%s\n' dbus-run-session compositor-x`,
			want:    `printf '%s\n' dbus-run-session compositor-x`,
			changed: false,
		},
		{
			name:    "wrapper is quoted mid-command",
			input:   `echo "dbus-run-session compositor-x"`,
			want:    `echo "dbus-run-session compositor-x"`,
			changed: false,
		},
		{
			name:    "comment",
			input:   `  # test -z "$DISPLAY" && dbus-run-session compositor-x`,
			want:    `  # test -z "$DISPLAY" && dbus-run-session compositor-x`,
			changed: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, changed := stripSessionWrappers(test.input)
			if got != test.want || changed != test.changed {
				t.Fatalf("strip = %q, %v; want %q, %v", got, changed, test.want, test.changed)
			}
		})
	}
}

func TestSessionWrapperStrippingAndBackup(t *testing.T) {
	app, stdout, _, root, home := turnstileTestApp(t, "runit", &fakeRunner{})
	profile := filepath.Join(home, ".profile")
	original := "# dbus-run-session compositor-x is documented here\nexec dbus-run-session compositor-x --session\ndbus-launch --exit-with-session session-tool\n"
	writeFixture(t, profile, original)
	desktop := filepath.Join(home, ".local/share/wayland-sessions/custom.desktop")
	writeFixture(t, desktop, "[Desktop Entry]\nExec=dbus-run-session compositor-x\n")
	packaged := filepath.Join(root, "usr/share/wayland-sessions/vendor.desktop")
	writeFixture(t, packaged, "[Desktop Entry]\nExec=dbus-run-session compositor-x\n")
	backup := filepath.Join(home, "backup")
	if code := app.SessionFixWrappers([]string{"--backup-dir", backup}); code != ExitOK {
		t.Fatal(code)
	}
	body, _ := os.ReadFile(profile)
	want := "# dbus-run-session compositor-x is documented here\nexec compositor-x --session\nsession-tool\n"
	if string(body) != want {
		t.Fatalf("profile = %q", body)
	}
	body, _ = os.ReadFile(desktop)
	if !strings.Contains(string(body), "Exec=compositor-x") {
		t.Fatalf("desktop = %q", body)
	}
	body, _ = os.ReadFile(packaged)
	if !strings.Contains(string(body), "dbus-run-session") {
		t.Fatal("packaged desktop was edited")
	}
	for _, path := range []string{profile, desktop} {
		copyPath := filepath.Join(backup, strings.TrimPrefix(path, "/"))
		if _, err := os.Stat(copyPath); err != nil {
			t.Fatalf("backup %s: %v", copyPath, err)
		}
	}
	if !strings.Contains(stdout.String(), profile) || !strings.Contains(stdout.String(), desktop) {
		t.Fatalf("changes = %q", stdout.String())
	}
	stdout.Reset()
	if code := app.SessionCheck(); code != ExitFalse {
		t.Fatalf("check exit = %d", code)
	}
	if !strings.Contains(stdout.String(), packaged+" starts the session") {
		t.Fatalf("packaged wrapper not reported: %q", stdout.String())
	}
}

func TestSessionWrapperDefaultBackupDirectory(t *testing.T) {
	app, _, _, _, home := turnstileTestApp(t, "runit", &fakeRunner{})
	profile := filepath.Join(home, ".zprofile")
	writeFixture(t, profile, "dbus-launch --sh-syntax compositor-x\n")
	if code := app.SessionFixWrappers(nil); code != ExitOK {
		t.Fatal(code)
	}
	backup := filepath.Join(home, ".local/state/ryoku/session-backup/20261009T123000Z", strings.TrimPrefix(profile, "/"))
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("default backup: %v", err)
	}
}

func TestTurnstileVerbsAreNoopsOnSystemd(t *testing.T) {
	app, stdout, stderr, _, home := turnstileTestApp(t, "systemd", &fakeRunner{})
	if code := app.SessionEnsure("--system"); code != ExitOK {
		t.Fatal(code)
	}
	if code := app.SessionEnsure("--user"); code != ExitOK {
		t.Fatal(code)
	}
	if code := app.SessionCheck(); code != ExitOK {
		t.Fatal(code)
	}
	if code := app.SessionFixWrappers(nil); code != ExitOK {
		t.Fatal(code)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".config")); !os.IsNotExist(err) {
		t.Fatal("systemd no-op wrote user files")
	}
}

func TestTurnstileCommandWiringAndHelp(t *testing.T) {
	app, stdout, _, _, _ := turnstileTestApp(t, "systemd", &fakeRunner{})
	for _, args := range [][]string{
		{"session", "ensure", "--system"},
		{"session", "ensure", "--user"},
		{"session", "check"},
		{"session", "fix-wrappers", "--backup-dir", "/unused"},
	} {
		if code := app.Execute(args); code != ExitOK {
			t.Fatalf("%v exit = %d", args, code)
		}
	}
	stdout.Reset()
	if code := app.Execute([]string{"--help"}); code != ExitOK {
		t.Fatal(code)
	}
	for _, text := range []string{"session ensure --system", "session ensure --user", "session check", "session fix-wrappers [--backup-dir DIR]"} {
		if !strings.Contains(stdout.String(), text) {
			t.Fatalf("help misses %q", text)
		}
	}
}
