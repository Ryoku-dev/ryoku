package main

import (
	"encoding/json"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The Hub reads the daemon's storage report over the control socket, so the wire
// contract is the thing that can break silently: a request line the daemon does
// not parse, or a reply shape the backend mis-reads, would show up as a wrong
// number in Settings rather than as an error. Both halves are pinned here.
func TestDaemonStatsReadsTheReport(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "ryoku-shell.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	t.Setenv("XDG_RUNTIME_DIR", dir)

	got := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			got <- "accept: " + err.Error()
			return
		}
		defer conn.Close()
		buf := make([]byte, 128)
		n, _ := conn.Read(buf)
		got <- strings.TrimSpace(string(buf[:n]))
		_, _ = conn.Write([]byte(`{"ok":true,"result":{"items":4,"starred":1,"bytes":300,"textBytes":100,"imageBytes":200,"lastPrune":1700000000}}` + "\n"))
	}()

	var st clipboardStats
	if err := daemonCall("clipboard.stats", nil, &st); err != nil {
		t.Fatalf("daemonCall: %v", err)
	}
	if req := <-got; req != "call clipboard.stats" {
		t.Errorf("request line = %q, want %q", req, "call clipboard.stats")
	}
	want := clipboardStats{Items: 4, Starred: 1, Bytes: 300, TextBytes: 100, ImageBytes: 200, LastPrune: 1700000000}
	if st != want {
		t.Errorf("stats = %+v, want %+v", st, want)
	}
}

// An error reply must surface as an error, not as a zeroed report: a prune that
// the daemon refused cannot look like "0 B used, nothing to do".
func TestDaemonCallSurfacesErrors(t *testing.T) {
	var out clipboardStats
	err := decodeCallReply("clipboard", `{"ok":false,"error":"unknown method: clipboard.stats"}`, &out)
	if err == nil || !strings.Contains(err.Error(), "unknown method") {
		t.Errorf("decodeCallReply error = %v, want the daemon's message", err)
	}
	if err := decodeCallReply("clipboard", `{"ok":true,"result":{"items":2,"bytes":10}}`, &out); err != nil {
		t.Fatalf("valid reply: %v", err)
	}
	if out.Items != 2 || out.Bytes != 10 {
		t.Errorf("decoded = %+v, want items 2 and 10 bytes", out)
	}
}

// A daemon that is not running is reported, never silently ignored: the page has
// to be able to say why the number is missing.
func TestDaemonCallWithoutDaemon(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir()) // no socket in there
	if err := daemonCall("clipboard.stats", nil, nil); err == nil {
		t.Error("daemonCall with no daemon returned nil, want an error")
	}
}

// The prune prints the same report it just refreshed, so the page's readout
// updates from one command instead of two racing ones.
func TestClipboardPrunePrintsPostPruneStats(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "ryoku-shell.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	t.Setenv("XDG_RUNTIME_DIR", dir)

	methods := make(chan []string, 1)
	go func() {
		var seen []string
		for i := 0; i < 2; i++ {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			buf := make([]byte, 128)
			n, _ := conn.Read(buf)
			seen = append(seen, strings.TrimSpace(string(buf[:n])))
			if i == 0 {
				_, _ = conn.Write([]byte(`{"ok":true}` + "\n"))
			} else {
				_, _ = conn.Write([]byte(`{"ok":true,"result":{"items":3,"starred":1,"bytes":42}}` + "\n"))
			}
			conn.Close()
		}
		methods <- seen
	}()

	out := captureStdout(t, func() {
		if err := runClipboard([]string{"prune"}); err != nil {
			t.Errorf("runClipboard(prune): %v", err)
		}
	})
	var st clipboardStats
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &st); err != nil {
		t.Fatalf("prune output %q is not JSON: %v", out, err)
	}
	if st.Items != 3 || st.Bytes != 42 {
		t.Errorf("post-prune report = %+v, want 3 items and 42 bytes", st)
	}
	select {
	case seen := <-methods:
		want := []string{"call clipboard.clear", "call clipboard.stats"}
		if len(seen) != 2 || seen[0] != want[0] || seen[1] != want[1] {
			t.Errorf("daemon calls = %v, want %v", seen, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the backend never talked to the fake daemon")
	}
}
