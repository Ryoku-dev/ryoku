package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// niri's runtime layout and the one way this provider talks to the compositor.
//
// Everything rides a single unix socket: one JSON value per line in, one per
// line out. Actions, queries and the event stream are all the same protocol, so
// unlike the Hyprland provider there is nothing to fork. That also means one
// choke point for the isolation gate: request below.

const dialTimeout = 200 * time.Millisecond

// socketPath prefers the handle the session exported, then the newest socket in
// the runtime dir. The fallback matters because the shell daemon restarts from
// the user manager's environment, which can name a socket whose instance died.
func socketPath() string {
	if p := os.Getenv("NIRI_SOCKET"); p != "" && socketAlive(p) {
		return p
	}
	if p := liveSocket(); p != "" {
		return p
	}
	return os.Getenv("NIRI_SOCKET")
}

// niri names its socket niri.<wayland-display>.<pid>.sock, so a stale file from
// a crashed instance sits beside a live one. Newest that actually answers wins.
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
		if strings.HasPrefix(name, "niri.") && strings.HasSuffix(name, ".sock") {
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
// socket path carries the instance pid, so it identifies this session and no
// other.
func instanceHandle() string {
	if !live() {
		return ""
	}
	return socketPath()
}

// reply is niri's response envelope. Exactly one side is set.
type reply struct {
	Ok  json.RawMessage `json:"Ok"`
	Err string          `json:"Err"`
}

// request is the single choke point for every compositor call, so the isolation
// gate has one line to allow. A var so tests can pin the emitted request
// without a live compositor.
var request = sendRequest

func sendRequest(req any) (json.RawMessage, error) {
	conn, payload, err := open(req)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("niri %s: %w", payload, err)
	}
	return unwrap(payload, line)
}

// open dials and sends, returning the connection so the event stream can keep
// reading from it. The request text comes back for error messages.
func open(req any) (net.Conn, string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, "", fmt.Errorf("encode request: %w", err)
	}
	path := socketPath()
	if path == "" {
		return nil, string(body), fmt.Errorf("niri %s: no socket", body)
	}
	conn, err := net.Dial("unix", path)
	if err != nil {
		return nil, string(body), fmt.Errorf("niri %s: %w", body, err)
	}
	if _, err := conn.Write(append(body, '\n')); err != nil {
		_ = conn.Close()
		return nil, string(body), fmt.Errorf("niri %s: %w", body, err)
	}
	return conn, string(body), nil
}

// niri answers Err with its own wording; pass it through rather than
// paraphrasing, so a protocol change reads as itself.
func unwrap(payload string, line []byte) (json.RawMessage, error) {
	var r reply
	if err := json.Unmarshal(line, &r); err != nil {
		return nil, fmt.Errorf("niri %s: bad reply %q", payload, strings.TrimSpace(string(line)))
	}
	if r.Err != "" {
		return nil, fmt.Errorf("niri %s: %s", payload, r.Err)
	}
	return r.Ok, nil
}

// niriConfigDir is where this provider's generated and user-owned KDL lives.
func niriConfigDir() string {
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		return filepath.Join(base, "niri")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "niri")
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
	if value := processEnvironmentValue(data, "NIRI_SOCKET"); value != "" {
		_, err = fmt.Fprintf(stdout, "NIRI_SOCKET=%s%c", value, byte(0))
	}
	return err
}
