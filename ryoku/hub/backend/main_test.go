package main

import (
	"os"
	"path/filepath"
	"testing"

	wm "ryoku-wm"
)

// harmless utilities the tests' own scripts run, linked from the system directories
// rather than looked up on PATH, so nothing else on the machine is reachable
var testUtilities = []string{
	"sh", "bash", "env", "cat", "cp", "mv", "rm", "mkdir", "install", "chmod", "ln",
	"touch", "dirname", "basename", "awk", "sed", "grep", "sleep", "luac",
}

// TestMain keeps the suite off this machine: PATH holds only testUtilities, so no
// test reaches a real ryoku-host (snapper), ryoku, pkexec, systemctl, ryogami, nmcli
// or compositor provider; HOME, the XDG dirs, the session sockets and both buses are private
func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "hub-test-")
	if err != nil {
		panic(err)
	}
	bin := filepath.Join(root, "bin")
	for _, dir := range []string{bin, filepath.Join(root, "home"), filepath.Join(root, "runtime")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			panic(err)
		}
	}
	for _, name := range testUtilities {
		for _, dir := range []string{"/usr/bin", "/bin"} {
			if real := filepath.Join(dir, name); isFile(real) {
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
		"HOME":                     filepath.Join(root, "home"),
		"XDG_CONFIG_HOME":          filepath.Join(root, "home", ".config"),
		"XDG_CACHE_HOME":           filepath.Join(root, "home", ".cache"),
		"XDG_DATA_HOME":            filepath.Join(root, "home", ".local", "share"),
		"XDG_STATE_HOME":           filepath.Join(root, "home", ".local", "state"),
		"XDG_RUNTIME_DIR":          filepath.Join(root, "runtime"),
		"DBUS_SESSION_BUS_ADDRESS": noBus,
		"DBUS_SYSTEM_BUS_ADDRESS":  noBus,
	} {
		os.Setenv(key, value)
	}
	for _, key := range append(wm.SessionHandles(), "WAYLAND_DISPLAY", "DISPLAY", "SWAYSOCK",
		"XDG_CURRENT_DESKTOP", "RYOKU_WM", "PKEXEC_UID", "SUDO_UID", "SUDO_USER") {
		os.Unsetenv(key)
	}
	code := m.Run()
	os.RemoveAll(root)
	os.Exit(code)
}
