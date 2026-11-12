package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	wm "ryoku-wm"
)

// testUtilities are the harmless programs the tests' own scripts and fakes run.
// They are linked from the system directories rather than looked up on PATH, so
// no other program on the machine is reachable: a test that needs a real tool's
// behaviour puts a fake that provides it on PATH.
var testUtilities = []string{
	"sh", "bash", "env", "cat", "cp", "mv", "rm", "mkdir", "chmod", "ln", "touch",
	"dirname", "basename", "awk", "sed", "grep", "sleep", "setsid", "true", "false",
	"printf", "echo", "head", "tail", "tr", "cut", "sort", "wc", "date", "readlink",
	"realpath", "stat", "sha256sum", "flock",
}

// liveOptIns are the switches that ask for a test against the real session. They
// survive the isolation so those tests can tell they were asked for, and
// useLiveSession hands them the session itself.
var liveOptIns = []string{"RYOKU_LIVE_DBUS", "RYOKU_KEYRING_E2E", "RYOKU_KEYRING_INTEROP"}

// liveEnvironment is the environment the suite was started with, kept only for
// the opt-in tests that talk to the real session.
var liveEnvironment []string

// TestMain keeps the suite off the machine that runs it. PATH holds only
// testUtilities, so no test runs a real ryoku tool, quickshell, compositor
// provider, systemctl or pkexec; HOME, every XDG dir and the greeter state dir
// are private; both buses point nowhere; and every handle that reaches the live
// session (the compositor's IPC socket, the Wayland and X displays, audio, the
// keyring and agent sockets) is cleared. A child started to hold a lock skips all
// of this: it only holds the lock its parent asked for.
func TestMain(m *testing.M) {
	if path := os.Getenv(lockHolderEnv); path != "" {
		holdLockForever(path)
		return
	}
	os.Exit(runIsolated(m))
}

func runIsolated(m *testing.M) int {
	liveEnvironment = os.Environ()
	root, err := os.MkdirTemp("", "ryoku-ipc-test-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(root)
	home := filepath.Join(root, "home")
	bin := filepath.Join(root, "bin")
	for _, dir := range []string{home, bin, filepath.Join(root, "runtime"), filepath.Join(root, "share"), filepath.Join(root, "etc-xdg")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			panic(err)
		}
	}
	for _, name := range testUtilities {
		for _, dir := range []string{"/usr/bin", "/bin"} {
			real := filepath.Join(dir, name)
			if info, err := os.Stat(real); err == nil && info.Mode().IsRegular() {
				if err := os.Symlink(real, filepath.Join(bin, name)); err != nil {
					panic(err)
				}
				break
			}
		}
	}
	optIn := map[string]bool{}
	for _, key := range liveOptIns {
		optIn[key] = true
	}
	for _, kv := range liveEnvironment {
		key, _, _ := strings.Cut(kv, "=")
		if (strings.HasPrefix(key, "XDG_") || strings.HasPrefix(key, "RYOKU_")) && !optIn[key] {
			os.Unsetenv(key)
		}
	}
	session := append(wm.SessionHandles(), "WAYLAND_DISPLAY", "WAYLAND_SOCKET", "DISPLAY", "SWAYSOCK",
		"DBUS_LAUNCHD_SESSION_BUS_SOCKET", "PULSE_SERVER", "PIPEWIRE_REMOTE", "PIPEWIRE_RUNTIME_DIR",
		"SSH_AUTH_SOCK", "GNOME_KEYRING_CONTROL", "NOTIFY_SOCKET", "PKEXEC_UID", "SUDO_UID", "SUDO_USER")
	for _, key := range session {
		os.Unsetenv(key)
	}
	noBus := "unix:path=" + filepath.Join(root, "no-bus")
	for key, value := range map[string]string{
		"PATH":                     bin,
		"HOME":                     home,
		"XDG_CONFIG_HOME":          filepath.Join(home, ".config"),
		"XDG_CACHE_HOME":           filepath.Join(home, ".cache"),
		"XDG_DATA_HOME":            filepath.Join(home, ".local", "share"),
		"XDG_STATE_HOME":           filepath.Join(home, ".local", "state"),
		"XDG_RUNTIME_DIR":          filepath.Join(root, "runtime"),
		"XDG_DATA_DIRS":            filepath.Join(root, "share"),
		"XDG_CONFIG_DIRS":          filepath.Join(root, "etc-xdg"),
		"DBUS_SESSION_BUS_ADDRESS": noBus,
		"DBUS_SYSTEM_BUS_ADDRESS":  noBus,
		"RYOKU_GREETER_STATE_DIR":  filepath.Join(root, "greeter"),
	} {
		os.Setenv(key, value)
	}
	return m.Run()
}

// useLiveSession gives an opt-in test back the environment the suite started
// with, for the rest of that test only. Call it after the opt-in check.
func useLiveSession(t *testing.T) {
	t.Helper()
	live := map[string]bool{}
	for _, kv := range liveEnvironment {
		key, value, _ := strings.Cut(kv, "=")
		live[key] = true
		t.Setenv(key, value)
	}
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if !live[key] {
			t.Setenv(key, "")
			os.Unsetenv(key)
		}
	}
}
