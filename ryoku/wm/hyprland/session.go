package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Hyprland's runtime layout: the instance signature, the two sockets, and the
// one way this provider runs hyprctl.
//
// The signature rebind matters because the shell daemon restarts mid-session
// from the user manager's environment, which can still name a compositor that
// exited days ago. hyprctl then dials a dead socket and every call silently
// no-ops. Resolving the live signature into this process fixes every later fork.

func runDir() string {
	if rt := os.Getenv("XDG_RUNTIME_DIR"); rt != "" {
		return filepath.Join(rt, "hypr")
	}
	return filepath.Join("/tmp", "hypr")
}

// A signature whose socket file lingers after the instance exited refuses the
// dial and reads dead.
// aliveCheck is a var so tests can satisfy live() without a compositor.
var aliveCheck = socketAlive

func socketAlive(sig string) bool {
	if sig == "" {
		return false
	}
	c, err := net.DialTimeout("unix", filepath.Join(runDir(), sig, ".socket.sock"), 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

// liveSignature picks the newest instance by socket mtime, so the current login
// wins over stale leftovers. Empty when none answer.
func liveSignature() string {
	entries, err := os.ReadDir(runDir())
	if err != nil {
		return ""
	}
	best := ""
	var bestMod time.Time
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sig := e.Name()
		fi, err := os.Stat(filepath.Join(runDir(), sig, ".socket.sock"))
		if err != nil || !socketAlive(sig) {
			continue
		}
		if best == "" || fi.ModTime().After(bestMod) {
			best, bestMod = sig, fi.ModTime()
		}
	}
	return best
}

// Idempotent and cheap: the alive case is one local dial.
func ensureLiveSignature() {
	if socketAlive(os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")) {
		return
	}
	if live := liveSignature(); live != "" {
		_ = os.Setenv("HYPRLAND_INSTANCE_SIGNATURE", live)
	}
}

// eventSocketPath prefers whichever layout exists, so a login-time race retries
// the right place.
func eventSocketPath() string {
	sig := os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")
	if sig == "" {
		return ""
	}
	var candidates []string
	if rt := os.Getenv("XDG_RUNTIME_DIR"); rt != "" {
		candidates = append(candidates, filepath.Join(rt, "hypr", sig, ".socket2.sock"))
	}
	candidates = append(candidates, filepath.Join("/tmp", "hypr", sig, ".socket2.sock"))
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return candidates[0]
}

// ctl is the single choke point for every compositor call, so the isolation gate
// has one line to allow. A var so tests can pin the emitted argv without a live
// compositor.
var ctl = runCtl

func runCtl(args ...string) ([]byte, error) {
	cmd := exec.Command("hyprctl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("hyprctl %s: %w: %s", strings.Join(args, " "), err, msg)
		}
		return nil, fmt.Errorf("hyprctl %s: %w", strings.Join(args, " "), err)
	}
	return stdout.Bytes(), nil
}

// dispatch runs one Lua dispatcher expression. hyprctl wraps the argument as
// hl.dispatch(<expr>), so the expression is the whole command.
func dispatch(expr string) error {
	_, err := ctl("dispatch", expr)
	return err
}

// evalLua applies a live config change. The Lua parser rejects hyprctl keyword,
// so eval is the only way to change a value without rewriting the config.
func evalLua(expr string) error {
	_, err := ctl("eval", expr)
	return err
}

// live lets verbs fail fast instead of hanging on a dead socket.
func live() bool {
	ensureLiveSignature()
	return aliveCheck(os.Getenv("HYPRLAND_INSTANCE_SIGNATURE"))
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
	if value := processEnvironmentValue(data, "HYPRLAND_INSTANCE_SIGNATURE"); value != "" {
		_, err = fmt.Fprintf(stdout, "HYPRLAND_INSTANCE_SIGNATURE=%s%c", value, byte(0))
	}
	return err
}
