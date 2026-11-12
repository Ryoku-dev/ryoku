// Package testenv keeps a package's tests off the machine that runs them. A
// package's TestMain hands its run to Run, so no test can exec a real ryoku
// tool, compositor provider, systemctl, sudo or pkexec, reach a session or
// system bus or a compositor socket, or write into the developer's home.
package testenv

import (
	"os"
	"path/filepath"
	"testing"

	wm "ryoku-wm"
)

// utilities are the harmless programs the tests' own scripts and fakes run. They
// are linked from the system directories rather than looked up on PATH, so
// nothing else on the machine is reachable; a test that needs a real tool's
// output puts a fake that prints it on PATH.
var utilities = []string{
	"sh", "bash", "env", "cat", "cp", "mv", "rm", "mkdir", "install", "chmod", "ln",
	"touch", "dirname", "basename", "awk", "sed", "grep", "sleep", "true", "false",
	"printf", "echo", "head", "tail", "tr", "cut", "sort", "wc", "date", "seq",
	"readlink", "realpath", "stat", "find", "git",
}

// Run gives the tests a private PATH of utilities, a private HOME, XDG and
// runtime tree, both D-Bus addresses pointing nowhere, and no session or
// compositor handle, then runs them and removes the tree.
func Run(m *testing.M) int {
	root, err := os.MkdirTemp("", "ryoku-cli-test-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(root)
	bin := filepath.Join(root, "bin")
	home := filepath.Join(root, "home")
	runtime := filepath.Join(root, "runtime")
	for _, dir := range []string{bin, home, runtime} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			panic(err)
		}
	}
	for _, name := range utilities {
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
	noBus := "unix:path=" + filepath.Join(root, "no-bus")
	for key, value := range map[string]string{
		"PATH":                     bin,
		"HOME":                     home,
		"XDG_CONFIG_HOME":          filepath.Join(home, ".config"),
		"XDG_CACHE_HOME":           filepath.Join(home, ".cache"),
		"XDG_DATA_HOME":            filepath.Join(home, ".local", "share"),
		"XDG_STATE_HOME":           filepath.Join(home, ".local", "state"),
		"XDG_RUNTIME_DIR":          runtime,
		"DBUS_SESSION_BUS_ADDRESS": noBus,
		"DBUS_SYSTEM_BUS_ADDRESS":  noBus,
	} {
		os.Setenv(key, value)
	}
	session := append(wm.SessionHandles(), "WAYLAND_DISPLAY", "DISPLAY", "SWAYSOCK", "XDG_CURRENT_DESKTOP",
		"XDG_SESSION_TYPE", "XDG_SESSION_ID", "RYOKU_WM", "RYOKU_REPO", "RYOKU_CHANNEL", "RYOKU_CONFIG_BASE",
		"PKEXEC_UID", "SUDO_UID", "SUDO_USER")
	for _, key := range session {
		os.Unsetenv(key)
	}
	return m.Run()
}
