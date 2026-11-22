package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const dialTimeout = 200 * time.Millisecond

func socketPath() string {
	if p := os.Getenv("UMBRIEL_SOCKET"); p != "" && alive(p) {
		return p
	}
	if p := liveSocket(); p != "" {
		return p
	}
	return os.Getenv("UMBRIEL_SOCKET")
}

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
		n := e.Name()
		if strings.HasPrefix(n, "umbriel-") && strings.HasSuffix(n, ".sock") {
			candidates = append(candidates, filepath.Join(rt, n))
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		a, _ := os.Stat(candidates[i])
		b, _ := os.Stat(candidates[j])
		if a == nil || b == nil {
			return false
		}
		return a.ModTime().After(b.ModTime())
	})
	for _, p := range candidates {
		if alive(p) {
			return p
		}
	}
	return ""
}

func alive(path string) bool {
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

func live() bool { return alive(socketPath()) }

func instanceHandle() string {
	if !live() {
		return ""
	}
	return socketPath()
}

type reply struct {
	Ok  json.RawMessage `json:"ok"`
	Err string          `json:"err"`
}

var request = sendRequest

func sendRequest(req any) (json.RawMessage, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	path := socketPath()
	if path == "" {
		return nil, fmt.Errorf("umbriel %s: no socket", body)
	}
	conn, err := net.Dial("unix", path)
	if err != nil {
		return nil, fmt.Errorf("umbriel %s: %w", body, err)
	}
	defer conn.Close()
	// A compositor that accepts a connection but never answers must not hang a
	// CLI call forever; the Hub has its own timeout, ryoku wm does not.
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write(append(body, '\n')); err != nil {
		return nil, fmt.Errorf("umbriel %s: %w", body, err)
	}
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("umbriel %s: %w", body, err)
	}
	var r reply
	if err := json.Unmarshal(line, &r); err != nil {
		return nil, fmt.Errorf("umbriel %s: bad reply %q", body, strings.TrimSpace(string(line)))
	}
	if r.Err != "" {
		return nil, fmt.Errorf("umbriel %s: %s", body, r.Err)
	}
	return r.Ok, nil
}

func query(cmd string) (json.RawMessage, error) {
	return request(map[string]any{"cmd": cmd})
}

// action sends the action form this Umbriel binary actually accepts. The newer
// typed {"cmd":"action",...} request is not supported by 0.1.0: its `msg`
// command sends {"cmd":"msg","arg":"<action>:<args>"} over the same socket.
// Colon-separated args follow `umbriel msg --help` for each action.
func action(name, value string) error {
	arg := name
	if value != "" {
		arg = name + ":" + value
	}
	_, err := request(map[string]any{"cmd": "msg", "arg": arg})
	return err
}

func subscribe(events []string) (net.Conn, error) {
	path := socketPath()
	if path == "" {
		return nil, fmt.Errorf("umbriel subscribe: no socket")
	}
	body, err := json.Marshal(map[string]any{"cmd": "subscribe", "events": events})
	if err != nil {
		return nil, err
	}
	conn, err := net.Dial("unix", path)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Write(append(body, '\n')); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func processEnvironmentValue(data []byte, key string) string {
	prefix := key + "="
	for _, field := range strings.Split(string(data), "\x00") {
		if strings.HasPrefix(field, prefix) {
			return strings.TrimPrefix(field, prefix)
		}
	}
	return ""
}
