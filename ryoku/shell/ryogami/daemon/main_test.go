package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	wm "ryoku-wm"
)

// TestMain points HOME, the XDG dirs and the runtime dir at a throwaway tree, so
// no test can write into the developer's or CI runner's real config, cache,
// data, or trash, or reach a running shell through its socket. It also drops
// every live session handle and re-resolves the window-manager client the
// package opened at init, so no test can query or drive the compositor, bus or
// display of the session the tests were started from, and shadows pkill, which
// a full video stop runs user-wide against the live wallpaper player.
func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "ryogami-test-home-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "ryogami tests: temp home:", err)
		os.Exit(1)
	}
	runtime := filepath.Join(root, "run")
	bin := filepath.Join(root, "bin")
	for _, dir := range []string{runtime, bin} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			fmt.Fprintln(os.Stderr, "ryogami tests:", err)
			os.Exit(1)
		}
	}
	if err := os.WriteFile(filepath.Join(bin, "pkill"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "ryogami tests: fake pkill:", err)
		os.Exit(1)
	}
	env := map[string]string{
		"HOME":            root,
		"XDG_CONFIG_HOME": filepath.Join(root, ".config"),
		"XDG_CACHE_HOME":  filepath.Join(root, ".cache"),
		"XDG_DATA_HOME":   filepath.Join(root, ".local", "share"),
		"XDG_STATE_HOME":  filepath.Join(root, ".local", "state"),
		"XDG_RUNTIME_DIR": runtime,
		"PATH":            bin + string(os.PathListSeparator) + os.Getenv("PATH"),
	}
	for k, v := range env {
		os.Setenv(k, v)
	}
	live := append(wm.SessionHandles(), "RYOKU_WM", "XDG_CURRENT_DESKTOP", "WAYLAND_DISPLAY", "DISPLAY", "DBUS_SESSION_BUS_ADDRESS")
	for _, k := range live {
		os.Unsetenv(k)
	}
	wmClient = wm.Open()
	code := m.Run()
	os.RemoveAll(root)
	os.Exit(code)
}
