package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"ryoget/internal/paths"
	"ryoget/internal/search"
)

// TestMain doubles as the daemon for the ensure test, which spawns this very
// binary as `serve`.
func TestMain(m *testing.M) {
	if os.Getenv("RYOSEEK_TEST_CHILD") == "1" {
		os.Exit(run(os.Args[1:]))
	}
	root, err := os.MkdirTemp("", "ryoseek-test-")
	if err != nil {
		panic(err)
	}
	for k, v := range map[string]string{
		"HOME":            root,
		"XDG_STATE_HOME":  filepath.Join(root, "state"),
		"XDG_CACHE_HOME":  filepath.Join(root, "cache"),
		"XDG_CONFIG_HOME": filepath.Join(root, "config"),
		"XDG_RUNTIME_DIR": filepath.Join(root, "run"),
	} {
		os.Setenv(k, v)
	}
	os.Unsetenv("RYOSEEK_IDLE_SECONDS")
	code := m.Run()
	os.RemoveAll(root)
	os.Exit(code)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

var offline = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
	return nil, errors.New("offline")
})}

type daemon struct {
	sock   string
	cancel context.CancelFunc
	done   chan error
}

func testOptions(t *testing.T, idle time.Duration) (serverOptions, *atomic.Int32) {
	t.Helper()
	dir := t.TempDir()
	var built atomic.Int32
	return serverOptions{
		Socket: filepath.Join(dir, "ryoseek.sock"),
		Lock:   filepath.Join(dir, "ryoseek.lock"),
		Idle:   idle,
		Log:    log.New(io.Discard, "", 0),
		UID:    os.Getuid(),
		NewService: func() (*search.Service, error) {
			built.Add(1)
			return search.New(search.Options{
				StateFile: filepath.Join(dir, "state", "health.json"),
				CacheDir:  filepath.Join(dir, "cache"),
				Version:   "test",
				Client:    offline,
				Now:       time.Now,
			})
		},
	}, &built
}

func start(t *testing.T, opts serverOptions) *daemon {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	d := &daemon{sock: opts.Socket, cancel: cancel, done: make(chan error, 1)}
	go func() { d.done <- runServer(ctx, opts) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-d.done:
		case <-time.After(15 * time.Second):
			t.Error("daemon did not stop")
		}
	})
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if nc, err := net.Dial("unix", opts.Socket); err == nil {
			nc.Close()
			return d
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("socket never came up")
	return nil
}

func (d *daemon) stop(t *testing.T) error {
	t.Helper()
	d.cancel()
	select {
	case err := <-d.done:
		d.done <- err
		return err
	case <-time.After(15 * time.Second):
		t.Fatal("daemon did not stop")
		return nil
	}
}

type client struct {
	nc net.Conn
	sc *bufio.Scanner
}

func dial(t *testing.T, sock string) *client {
	t.Helper()
	nc, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { nc.Close() })
	sc := bufio.NewScanner(nc)
	sc.Buffer(make([]byte, 64<<10), maxLine)
	return &client{nc: nc, sc: sc}
}

func (c *client) send(t *testing.T, v any) {
	t.Helper()
	b, _ := json.Marshal(v)
	if _, err := c.nc.Write(append(b, '\n')); err != nil {
		t.Fatal(err)
	}
}

func (c *client) next(t *testing.T) map[string]any {
	t.Helper()
	c.nc.SetReadDeadline(time.Now().Add(10 * time.Second))
	if !c.sc.Scan() {
		t.Fatalf("no line: %v", c.sc.Err())
	}
	var m map[string]any
	if err := json.Unmarshal(c.sc.Bytes(), &m); err != nil {
		t.Fatalf("bad line %q: %v", c.sc.Bytes(), err)
	}
	return m
}

func (c *client) hello(t *testing.T) map[string]any {
	t.Helper()
	c.send(t, map[string]any{"hello": map[string]any{"client": "cli", "version": 2, "lang": "en-US"}})
	return c.next(t)
}

func TestTwoClientsShareOneDaemon(t *testing.T) {
	opts, built := testOptions(t, time.Minute)
	d := start(t, opts)
	if fi, err := os.Stat(d.sock); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("socket mode: %v %v", fi, err)
	}
	a, b := dial(t, d.sock), dial(t, d.sock)
	for name, c := range map[string]*client{"a": a, "b": b} {
		r := c.hello(t)
		if r["ready"] != true || r["version"] != float64(2) {
			t.Fatalf("%s: ready line %v", name, r)
		}
		if srcs, _ := r["sources"].([]any); len(srcs) == 0 {
			t.Fatalf("%s: no sources in %v", name, r)
		}
	}
	if n := built.Load(); n != 1 {
		t.Fatalf("built %d services, want one shared", n)
	}
	if err := d.stop(t); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(d.sock); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket left behind: %v", err)
	}
}

func TestQuerySettles(t *testing.T) {
	opts, _ := testOptions(t, time.Minute)
	d := start(t, opts)
	c := dial(t, d.sock)
	c.hello(t)
	c.send(t, map[string]any{"id": 7, "q": "what does tbh mean", "scope": "all", "mode": "suggest"})
	for {
		m := c.next(t)
		if m["id"] == float64(7) && m["source"] == nil && m["state"] == "done" {
			return
		}
	}
}

func TestSecondDaemonRefused(t *testing.T) {
	opts, built := testOptions(t, time.Minute)
	start(t, opts)
	err := runServer(context.Background(), opts)
	if !errors.Is(err, errRunning) {
		t.Fatalf("second daemon: %v", err)
	}
	if n := built.Load(); n != 1 {
		t.Fatalf("the refused daemon built a service (%d)", n)
	}
}

func TestIdleExit(t *testing.T) {
	opts, _ := testOptions(t, 300*time.Millisecond)
	d := start(t, opts)
	c := dial(t, d.sock)
	c.hello(t)
	select {
	case err := <-d.done:
		t.Fatalf("exited with a client connected: %v", err)
	case <-time.After(time.Second):
	}
	c.nc.Close()
	select {
	case err := <-d.done:
		d.done <- err
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("did not exit once idle")
	}
}

func TestIdleFromEnv(t *testing.T) {
	t.Setenv("RYOSEEK_IDLE_SECONDS", "0.5")
	if got := idleFromEnv(); got != 500*time.Millisecond {
		t.Fatalf("idle = %v", got)
	}
	t.Setenv("RYOSEEK_IDLE_SECONDS", "nonsense")
	if got := idleFromEnv(); got != defaultIdle {
		t.Fatalf("idle = %v", got)
	}
}

func TestPeerCheck(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "p.sock")
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: sock, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		if nc, err := net.Dial("unix", sock); err == nil {
			defer nc.Close()
			time.Sleep(time.Second)
		}
	}()
	nc, err := ln.AcceptUnix()
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	if !peerIs(nc, os.Getuid()) {
		t.Fatal("own uid refused")
	}
	if peerIs(nc, os.Getuid()+1) {
		t.Fatal("other uid admitted")
	}

	opts, _ := testOptions(t, time.Minute)
	opts.UID = os.Getuid() + 1
	d := start(t, opts)
	c := dial(t, d.sock)
	// the daemon may hang up before the hello lands; either way no answer
	c.nc.Write([]byte(`{"hello":{"client":"cli","version":2}}` + "\n"))
	c.nc.SetReadDeadline(time.Now().Add(5 * time.Second))
	if c.sc.Scan() {
		t.Fatalf("a foreign peer got an answer: %s", c.sc.Bytes())
	}
}

func TestEnsureSpawnsDetachedDaemon(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(root, "run"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("RYOSEEK_TEST_CHILD", "1")
	t.Setenv("RYOSEEK_IDLE_SECONDS", "30")
	// a ryoku-host whose unit never answers, so ensure falls back to the spawn
	bin := filepath.Join(root, "bin")
	calls := filepath.Join(root, "host-calls")
	os.MkdirAll(bin, 0o755)
	script := "#!/bin/sh\necho \"$@\" >> " + calls + "\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "ryoku-host"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	if code := ensure(context.Background()); code != exitOK {
		t.Fatalf("ensure = %d", code)
	}
	got, _ := os.ReadFile(calls)
	if strings.TrimSpace(string(got)) != "svc --user start ryoseek" {
		t.Fatalf("ryoku-host called with %q", got)
	}

	nc, err := net.Dial("unix", paths.SeekSocket())
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := nc.(*net.UnixConn).SyscallConn()
	var cred *syscall.Ucred
	raw.Control(func(fd uintptr) {
		cred, _ = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	})
	nc.Close()
	if cred == nil {
		t.Fatal("no peer credentials")
	}
	pid := int(cred.Pid)
	t.Cleanup(func() { syscall.Kill(pid, syscall.SIGKILL) })
	if sid := sessionOf(t, pid); sid != pid {
		t.Fatalf("daemon session %d, want its own (%d)", sid, pid)
	}
	if code := ensure(context.Background()); code != exitOK {
		t.Fatalf("second ensure = %d", code)
	}
	if got2, _ := os.ReadFile(calls); !bytes.Equal(got, got2) {
		t.Fatal("ensure started something although the daemon answered")
	}

	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	exited := make(chan syscall.WaitStatus, 1)
	go func() {
		var ws syscall.WaitStatus
		syscall.Wait4(pid, &ws, 0, nil)
		exited <- ws
	}()
	select {
	case ws := <-exited:
		if ws.ExitStatus() != 0 {
			t.Fatalf("daemon exit status %d", ws.ExitStatus())
		}
	case <-time.After(15 * time.Second):
		t.Fatal("daemon ignored SIGTERM")
	}
	if _, err := os.Stat(paths.SeekSocket()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket left behind: %v", err)
	}
	logb, _ := os.ReadFile(paths.SeekLog())
	if !strings.Contains(string(logb), "listening on") || !strings.Contains(string(logb), "stopped") {
		t.Fatalf("daemon log: %q", logb)
	}
}

func TestEnsureNothingToStart(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	old := selfPath
	selfPath = func() (string, error) { return "", errors.New("gone") }
	defer func() { selfPath = old }()
	if code := ensure(context.Background()); code != exitMissing {
		t.Fatalf("ensure = %d, want %d", code, exitMissing)
	}
}

// sessionOf reads field 6 of /proc/<pid>/stat, after the parenthesised name.
func sessionOf(t *testing.T, pid int) int {
	t.Helper()
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	f := strings.Fields(s[strings.LastIndexByte(s, ')')+1:])
	sid, _ := strconv.Atoi(f[3])
	return sid
}

func TestParseQuery(t *testing.T) {
	q, err := parseQuery([]string{"what", "does", "--mode", "suggest", "tbh", "mean", "--json", "--scope", "web"})
	if err != nil {
		t.Fatal(err)
	}
	if q.Text != "what does tbh mean" || q.Mode != "suggest" || q.Scope != "web" || !q.JSON {
		t.Fatalf("%+v", q)
	}
	for _, bad := range [][]string{{}, {"--mode"}, {"x", "--mode", "loud"}} {
		if _, err := parseQuery(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestLocaleTag(t *testing.T) {
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "pt_BR.UTF-8")
	if got := localeTag(); got != "pt-BR" {
		t.Fatalf("locale %q", got)
	}
}

func TestPrintLanes(t *testing.T) {
	id := int64(1)
	lanes := map[string]*lane{
		"web": {reply: reply{ID: &id, Source: "web", State: "done", Rank: 1, Items: []item{{Title: "Result", URL: "https://example.org", Snippet: "a\nsnippet"}}}, order: 0},
		"dictionary": {reply: reply{ID: &id, Source: "dictionary", State: "done", Rank: 6, Items: []item{{
			Kind: "definition", Title: "tbh", Snippet: "To be honest.",
			Meta: json.RawMessage(`{"partOfSpeech":"phrase","attribution":"Wiktionary, CC BY-SA 4.0"}`),
		}}}, order: 1},
		"wikipedia": {reply: reply{ID: &id, Source: "wikipedia", State: "paused", Until: "2026-11-18T15:20:00Z"}, order: 2},
	}
	var buf bytes.Buffer
	printLanes(&buf, lanes, map[string]string{"web": "Web", "dictionary": "Dictionary"})
	out := buf.String()
	di, wi := strings.Index(out, "Dictionary"), strings.Index(out, "Web")
	if di < 0 || wi < 0 || di > wi {
		t.Fatalf("higher rank must lead:\n%s", out)
	}
	for _, want := range []string{"tbh (phrase): To be honest.", "Wiktionary, CC BY-SA 4.0", "1. Result", "https://example.org", "a snippet", "wikipedia: paused until"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
