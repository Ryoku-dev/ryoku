package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// events records the order of profile deletes and daemon requests.
type events struct {
	mu   sync.Mutex
	list []string
}

func (e *events) add(s string) { e.mu.Lock(); e.list = append(e.list, s); e.mu.Unlock() }
func (e *events) get() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.list...)
}

// recordReplaces swaps replaceProfiles for one that logs into ev.
func recordReplaces(t *testing.T, ev *events) {
	t.Helper()
	prev := replaceProfiles
	replaceProfiles = func(ssid string) { ev.add("replace " + ssid) }
	t.Cleanup(func() { replaceProfiles = prev })
}

// fakeDaemon answers one call on a private control socket and hands back the
// request line it read.
func fakeDaemon(t *testing.T, reply string, ev ...*events) <-chan string {
	t.Helper()
	dir := t.TempDir()
	ln, err := net.Listen("unix", filepath.Join(dir, "ryoku-shell.sock"))
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	t.Setenv("XDG_RUNTIME_DIR", dir)
	got := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			got <- "accept: " + err.Error()
			return
		}
		defer conn.Close()
		line, _ := bufio.NewReader(conn).ReadString('\n')
		for _, e := range ev {
			e.add("request")
		}
		got <- strings.TrimSpace(line)
		_, _ = conn.Write([]byte(reply + "\n"))
	}()
	return got
}

// The passphrase travels inside the call's JSON, read from stdin; a space, a
// quote or a colon in it must arrive intact.
func TestWifiConnectSendsTheJoinToTheDaemon(t *testing.T) {
	recordReplaces(t, &events{})
	got := fakeDaemon(t, `{"ok":true,"result":null}`)
	if err := runWifi([]string{"connect", "Cafe: Guest"}, strings.NewReader("p a\"ss:1\n")); err != nil {
		t.Fatalf("runWifi: %v", err)
	}
	req := <-got
	method, raw, _ := strings.Cut(strings.TrimPrefix(req, "call "), " ")
	if method != "network.wifiConnect" {
		t.Fatalf("method = %q, want network.wifiConnect", method)
	}
	var args struct{ Ssid, Password string }
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		t.Fatalf("args %q: %v", raw, err)
	}
	if args.Ssid != "Cafe: Guest" || args.Password != "p a\"ss:1" {
		t.Errorf("args = %+v", args)
	}
}

// A rejected join is the daemon's error, and it must fail the command so the
// page reopens the password row.
func TestWifiConnectSurfacesARejectedJoin(t *testing.T) {
	recordReplaces(t, &events{})
	fakeDaemon(t, `{"ok":false,"error":"could not connect (check the password)"}`)
	err := runWifi([]string{"connect", "Cafe"}, strings.NewReader("wrong\n"))
	if err == nil || !strings.Contains(err.Error(), "check the password") {
		t.Fatalf("err = %v, want the daemon's rejection", err)
	}
	if errors.Is(err, errJoinNotRun) {
		t.Fatal("a rejected passphrase was treated as a join the daemon could not run")
	}
}

func TestWifiConnectNeedsAPassphrase(t *testing.T) {
	if err := runWifi([]string{"connect", "Cafe"}, strings.NewReader("")); err == nil {
		t.Fatal("an empty stdin was sent as a join")
	}
	if err := runWifi([]string{"connect"}, strings.NewReader("pw\n")); err == nil {
		t.Fatal("a join without an SSID was accepted")
	}
	if err := runWifi([]string{"connect", "Cafe", "extra"}, strings.NewReader("pw\n")); err == nil {
		t.Fatal("a stray argument was accepted")
	}
}

// The saved profiles the new passphrase replaces go only once the daemon has
// answered, and before the join: a join that cannot run must not cost the
// user the networks they had.
func TestWifiConnectReplacesProfilesOnlyWithTheDaemonUp(t *testing.T) {
	var ev events
	recordReplaces(t, &ev)
	got := fakeDaemon(t, `{"ok":true,"result":null}`, &ev)
	if err := runWifi([]string{"connect", "Cafe"}, strings.NewReader("pw\n")); err != nil {
		t.Fatalf("runWifi: %v", err)
	}
	<-got
	if want := []string{"replace Cafe", "request"}; strings.Join(ev.get(), "|") != strings.Join(want, "|") {
		t.Errorf("events = %q, want %q", ev.get(), want)
	}
}

func TestWifiConnectKeepsProfilesWhenTheDaemonIsDown(t *testing.T) {
	var ev events
	recordReplaces(t, &ev)
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	err := runWifi([]string{"connect", "Cafe"}, strings.NewReader("pw\n"))
	if !errors.Is(err, errDaemonDown) {
		t.Fatalf("err = %v, want errDaemonDown so the page can fall back", err)
	}
	if len(ev.get()) != 0 {
		t.Errorf("profiles touched with no daemon: %q", ev.get())
	}
}

// Any daemon answer other than a rejected passphrase means the join did not
// run, so the page must be told to join through nmcli instead.
func TestWifiConnectFallsBackWhenTheDaemonCannotJoin(t *testing.T) {
	for _, reply := range []string{
		`{"ok":false,"error":"unknown method: network.wifiConnect"}`,
		`{"ok":false,"error":"no wifi device"}`,
		`{"ok":false,"error":"connection timed out"}`,
		`{"ok":false,"error":"802-11-wireless-security.psk: property is invalid"}`,
	} {
		recordReplaces(t, &events{})
		fakeDaemon(t, reply)
		err := runWifi([]string{"connect", "Cafe"}, strings.NewReader("pw\n"))
		if !errors.Is(err, errJoinNotRun) {
			t.Errorf("reply %s: err = %v, want errJoinNotRun", reply, err)
		}
	}
}
