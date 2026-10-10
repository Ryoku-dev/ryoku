package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// wayfire's runtime layout and how this provider finds the compositor.
//
// Everything rides the unix socket the ipc plugin exports as WAYFIRE_SOCKET:
// four bytes little-endian of payload length, then one JSON object per
// request. Unlike Hyprland there is nothing to fork, and unlike niri the
// handle is a plain path with no instance pid inside it.

const dialTimeout = 200 * time.Millisecond

// socketPath prefers the handle the session exported, then the newest socket
// in the runtime dir. The fallback matters because the shell daemon restarts
// from the user manager's environment, which can name a socket whose instance
// died.
func socketPath() string {
	if p := os.Getenv("WAYFIRE_SOCKET"); p != "" && socketAlive(p) {
		return p
	}
	if p := liveSocket(); p != "" {
		return p
	}
	return os.Getenv("WAYFIRE_SOCKET")
}

// wayfire names its socket wayfire-<wayland-display>-.socket, so a stale file
// from a crashed instance sits beside a live one. Newest that actually
// answers wins.
func liveSocket() string {
	rt := os.Getenv("XDG_RUNTIME_DIR")
	if rt == "" {
		return ""
	}
	entries, err := os.ReadDir(rt)
	if err != nil {
		return ""
	}
	var candidates []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "wayfire-") && strings.HasSuffix(name, ".socket") {
			candidates = append(candidates, filepath.Join(rt, name))
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return modTime(candidates[i]).After(modTime(candidates[j]))
	})
	for _, p := range candidates {
		if socketAlive(p) {
			return p
		}
	}
	return ""
}

func modTime(path string) time.Time {
	fi, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return fi.ModTime()
}

// aliveCheck is a var so tests can satisfy live() without a compositor.
var aliveCheck = socketAlive

func socketAlive(path string) bool {
	if path == "" {
		return false
	}
	c, err := net.DialTimeout("unix", path, dialTimeout)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

// live lets verbs fail fast instead of hanging on a dead socket.
func live() bool { return aliveCheck(socketPath()) }

// instanceHandle is opaque to consumers, which only string-compare it. The
// socket path carries the wayland display number, so it identifies this
// session and no other.
func instanceHandle() string {
	if !live() {
		return ""
	}
	return socketPath()
}

// wayfireConfigDir is where this provider's generated and user-owned config
// lives.
func wayfireConfigDir() string {
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		return filepath.Join(base, "wayfire")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "wayfire")
}

var sessionEnvironmentRoot = "/proc"

func processEnvironmentValue(data []byte, key string) string {
	prefix := key + "="
	for _, field := range strings.Split(string(data), "\x00") {
		if strings.HasPrefix(field, prefix) {
			return strings.TrimPrefix(field, prefix)
		}
	}
	return ""
}

// runEnvironment exports only this provider's opaque session handle. The
// lifecycle owner can bind an exact login1 scope without learning its name.
func runEnvironment(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("environment: expected one process id")
	}
	pid, err := strconv.Atoi(args[0])
	if err != nil || pid <= 0 {
		return fmt.Errorf("environment: invalid process id %q", args[0])
	}
	data, err := os.ReadFile(filepath.Join(sessionEnvironmentRoot, strconv.Itoa(pid), "environ"))
	if err != nil {
		return fmt.Errorf("environment: read process %d: %w", pid, err)
	}
	if value := processEnvironmentValue(data, "WAYFIRE_SOCKET"); value != "" {
		_, err = fmt.Fprintf(stdout, "WAYFIRE_SOCKET=%s%c", value, byte(0))
	}
	return err
}
