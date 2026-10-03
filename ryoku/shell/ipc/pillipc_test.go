package main

import (
	"bufio"
	"net"
	"strings"
	"sync"
	"testing"
)

// startPillServer runs a fake pill command socket at pillSockPath that records
// the line of each connection and answers with reply. Returns a reader for the
// last received line and a stop function.
func startPillServer(t *testing.T, reply string) (func() string, func()) {
	t.Helper()
	ln, err := net.Listen("unix", pillSockPath())
	if err != nil {
		t.Fatalf("listen %s: %v", pillSockPath(), err)
	}
	var mu sync.Mutex
	var last string
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			line, _ := bufio.NewReader(conn).ReadString('\n')
			mu.Lock()
			last = strings.TrimRight(line, "\n")
			mu.Unlock()
			if reply != "" {
				_, _ = conn.Write([]byte(reply))
			}
			_ = conn.Close()
		}
	}()
	get := func() string { mu.Lock(); defer mu.Unlock(); return last }
	return get, func() { _ = ln.Close() }
}

// an "ok" ack means the pill handled the command, so the daemon skips the qs
// client entirely.
func TestPillSocketCallAck(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	get, stop := startPillServer(t, "ok\n")
	defer stop()
	if !pillSocketCall("openSurface eDP-1 sidebar-left") {
		t.Fatal("pillSocketCall = false, want true on ok ack")
	}
	if got := get(); got != "openSurface eDP-1 sidebar-left" {
		t.Fatalf("server received %q, want %q", got, "openSurface eDP-1 sidebar-left")
	}
}

// a non-ok answer (unknown command on the pill side) must fail so the caller
// falls back to the qs client.
func TestPillSocketCallErrAck(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	_, stop := startPillServer(t, "err\n")
	defer stop()
	if pillSocketCall("bogus") {
		t.Fatal("pillSocketCall = true, want false on err ack")
	}
}

// no socket (pill down or restarting) must fail so the caller falls back.
func TestPillSocketCallNoServer(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	if pillSocketCall("openSurface eDP-1 sidebar-left") {
		t.Fatal("pillSocketCall = true with no server, want false")
	}
}

// pillIpc must use the socket when it is up, and drop empty args from the line.
func TestPillIpcPrefersSocket(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	get, stop := startPillServer(t, "ok\n")
	defer stop()
	if got := pillIpc("openSurface", "eDP-1", "sidebar-left"); got != "ok" {
		t.Fatalf("pillIpc(openSurface) = %q, want ok", got)
	}
	if got := get(); got != "openSurface eDP-1 sidebar-left" {
		t.Fatalf("server received %q, want %q", got, "openSurface eDP-1 sidebar-left")
	}
	if got := pillIpc("closeAllMenus"); got != "ok" {
		t.Fatalf("pillIpc(closeAllMenus without monitor) = %q, want ok", got)
	}
	if got := get(); got != "closeAllMenus" {
		t.Fatalf("server received %q, want %q", got, "closeAllMenus")
	}
}

func BenchmarkPillSocketCall(b *testing.B) {
	b.Setenv("XDG_RUNTIME_DIR", b.TempDir())
	ln, err := net.Listen("unix", pillSockPath())
	if err != nil {
		b.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_, _ = bufio.NewReader(conn).ReadString('\n')
			_, _ = conn.Write([]byte("ok\n"))
			_ = conn.Close()
		}
	}()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if !pillSocketCall("openSurface eDP-1 sidebar-left") {
			b.Fatal("not ok")
		}
	}
}
