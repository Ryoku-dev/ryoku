package hindsight

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"ryoku-rashin/internal/memory/backend"
	"ryoku-rashin/internal/memory/backendtest"
	"ryoku-rashin/internal/protocol"
)

// TestMain doubles as a fake hindsight-api: the script setup writes runs the
// test binary with RASHIN_FAKE_HINDSIGHT set.
func TestMain(m *testing.M) {
	if os.Getenv("RASHIN_FAKE_HINDSIGHT") != "" {
		fakeAPI()
		return
	}
	os.Exit(m.Run())
}

// fakeAPI records its arguments, environment and pid where it runs, serves
// the fake server on the loopback port it was given after FAKE_DELAY, and
// records SIGTERM, ignoring it under FAKE_STUBBORN. FAKE_PG leaves a
// postmaster.pid as pg0's Postgres would.
func fakeAPI() {
	dir, _ := os.Getwd()
	_ = os.WriteFile(filepath.Join(dir, "fake-args"), []byte(strings.Join(os.Args[1:], "\n")), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "fake-env"), []byte(strings.Join(os.Environ(), "\n")), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "fake-pid"), []byte(strconv.Itoa(os.Getpid())), 0o600)
	if os.Getenv("FAKE_PG") != "" {
		pid := filepath.Join(os.Getenv("HOME"), ".pg0", "instances", instance, "data", "postmaster.pid")
		_ = os.MkdirAll(filepath.Dir(pid), 0o700)
		_ = os.WriteFile(pid, []byte("1\n"), 0o600)
	}
	terms := make(chan os.Signal, 1)
	signal.Notify(terms, syscall.SIGTERM)
	go func() {
		for range terms {
			_ = os.WriteFile(filepath.Join(dir, "fake-term"), []byte("term"), 0o600)
			if os.Getenv("FAKE_STUBBORN") == "" {
				os.Exit(0)
			}
		}
	}()
	host, port := "", ""
	for i, arg := range os.Args {
		if arg == "--host" && i+1 < len(os.Args) {
			host = os.Args[i+1]
		}
		if arg == "--port" && i+1 < len(os.Args) {
			port = os.Args[i+1]
		}
	}
	if delay, err := time.ParseDuration(os.Getenv("FAKE_DELAY")); err == nil {
		time.Sleep(delay)
	}
	server := newFakeServer(os.Getenv("HINDSIGHT_API_TENANT_API_KEY"))
	if pending, err := strconv.Atoi(os.Getenv("FAKE_PENDING")); err == nil {
		server.pending = int32(pending)
	}
	_ = http.ListenAndServe(net.JoinHostPort(host, port), server)
	select {}
}

type fakeSecrets struct {
	mu     sync.Mutex
	values map[string]string
}

func (s *fakeSecrets) Secret(name string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.values[name]
	if !ok {
		return "", errors.New("no credential " + name)
	}
	return value, nil
}

func (s *fakeSecrets) Keep(name, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[name] = value
	return nil
}

type fakeGateway struct {
	mu     sync.Mutex
	models []string
}

func (g *fakeGateway) API() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		g.mu.Lock()
		g.models = append(g.models, body.Model)
		g.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})
}

func (g *fakeGateway) APIKey(context.Context, bool) (string, error) { return "gateway-key", nil }

// rig is a local Hindsight whose hindsight-api and pg0 are fakes.
type rig struct {
	layout  Layout
	secrets *fakeSecrets
	proc    *Process
	h       *Hindsight
}

func newRig(t *testing.T, env ...string) *rig {
	t.Helper()
	root := t.TempDir()
	layout := Layout{Data: filepath.Join(root, "data"), State: filepath.Join(root, "state"), Cache: filepath.Join(root, "cache")}
	if err := os.MkdirAll(filepath.Dir(layout.program()), 0o700); err != nil {
		t.Fatal(err)
	}
	test, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	script := fmt.Sprintf("#!/bin/sh\nexec env RASHIN_FAKE_HINDSIGHT=1 %s %q \"$@\"\n", strings.Join(env, " "), test)
	python := "#!/bin/sh\necho \"$@\" >>\"$HOME/pg0-calls\"\nrm -f \"$HOME/.pg0/instances/rashin/data/postmaster.pid\"\n"
	for path, body := range map[string]string{layout.program(): script, layout.python(): python, layout.installed(): lockSum() + "\n"} {
		if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	secrets := &fakeSecrets{values: map[string]string{KeyName: backendtest.Canary, DatabaseName: "db-password-canary"}}
	proc := NewProcess(Config{Layout: layout, Secrets: secrets, Gateway: &fakeGateway{}, Model: func(context.Context, string) string { return "ollama/qwen3" }})
	r := &rig{layout: layout, secrets: secrets, proc: proc, h: Local(proc)}
	t.Cleanup(proc.Stop)
	return r
}

func (r *rig) file(name string) string {
	raw, _ := os.ReadFile(filepath.Join(r.layout.run(), name))
	return string(raw)
}

func (r *rig) pid(t *testing.T) int {
	t.Helper()
	pid, err := strconv.Atoi(r.file("fake-pid"))
	if err != nil {
		t.Fatalf("no fake pid: %v", err)
	}
	return pid
}

func alive(pid int) bool { return syscall.Kill(pid, 0) == nil }

func ctxFor(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestStartsOnlyWhenNeededAndHardened(t *testing.T) {
	r := newRig(t)
	if status := r.h.Status(ctxFor(t)); status.State != protocol.EngineStopped {
		t.Fatalf("Status before any call = %+v", status)
	}
	r.h.Settle(ctxFor(t))
	if r.file("fake-args") != "" {
		t.Fatal("Status or Settle started Hindsight")
	}
	if _, _, err := r.h.List(ctxFor(t), protocol.MemoryYou, 0, 10); err != nil {
		t.Fatal(err)
	}
	args := strings.Split(r.file("fake-args"), "\n")
	if at := slices.Index(args, "--host"); at < 0 || args[at+1] != "127.0.0.1" {
		t.Fatalf("args %q do not bind loopback", args)
	}
	for _, secret := range []string{backendtest.Canary, "db-password-canary", "gateway-key"} {
		if strings.Contains(r.file("fake-args"), secret) {
			t.Fatalf("a secret is on the command line: %q", args)
		}
	}
	env := strings.Split(r.file("fake-env"), "\n")
	for _, want := range []string{
		"HINDSIGHT_API_TENANT_EXTENSION=hindsight_api.extensions.builtin.tenant:ApiKeyTenantExtension",
		"HINDSIGHT_API_TENANT_API_KEY=" + backendtest.Canary,
		"HINDSIGHT_API_LLM_TRACE_ENABLED=false",
		"HINDSIGHT_API_STORE_DOCUMENT_TEXT=false",
		"HINDSIGHT_API_MCP_ENABLED=false",
		"HINDSIGHT_API_ENABLE_FILE_UPLOAD_API=false",
		"HINDSIGHT_API_ENABLE_DRY_RUN_EXTRACT=false",
		"HINDSIGHT_API_RERANKER_PROVIDER=rrf",
		"HINDSIGHT_API_LLM_API_KEY=gateway-key",
		"HF_HUB_OFFLINE=1",
		"HOME=" + r.layout.home(),
	} {
		if !slices.Contains(env, want) {
			t.Errorf("the child's environment lacks %s", want)
		}
	}
	for _, line := range env {
		if value, ok := strings.CutPrefix(line, "HINDSIGHT_API_DATABASE_URL="); ok {
			if !strings.HasPrefix(value, "pg0://rashin:db-password-canary@rashin:") {
				t.Errorf("the database URL is %q", value)
			}
		}
		if value, ok := strings.CutPrefix(line, "HINDSIGHT_API_LLM_BASE_URL="); ok && !strings.HasPrefix(value, "http://127.0.0.1:") {
			t.Errorf("the model listener is at %q", value)
		}
	}
	info, err := os.Stat(r.layout.run())
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("the folder it runs in is %v, %v", info.Mode(), err)
	}
	if r.h.Busy() {
		t.Fatal("Busy with nothing in flight")
	}
	if status := r.h.Status(ctxFor(t)); status.State != protocol.EngineReady {
		t.Fatalf("Status while running = %+v", status)
	}
	pid := r.pid(t)
	r.h.Release()
	if r.file("fake-term") == "" || alive(pid) {
		t.Fatal("Release did not stop Hindsight with SIGTERM")
	}
	if status := r.h.Status(ctxFor(t)); status.State != protocol.EngineStopped {
		t.Fatalf("Status after Release = %+v", status)
	}
	// the next call starts it again
	if _, _, err := r.h.List(ctxFor(t), protocol.MemoryYou, 0, 10); err != nil {
		t.Fatal(err)
	}
	if r.pid(t) == pid {
		t.Fatal("the next call did not start a new Hindsight")
	}
}

func TestBusyWhileStarting(t *testing.T) {
	r := newRig(t, "FAKE_DELAY=700ms")
	done := make(chan error)
	go func() {
		_, _, err := r.h.List(ctxFor(t), protocol.MemoryYou, 0, 10)
		done <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	for r.h.Status(ctxFor(t)).State != protocol.EngineStarting && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !r.h.Busy() {
		t.Fatal("not Busy while a call waits for Hindsight to start")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if r.h.Busy() {
		t.Fatal("Busy after the call returned")
	}
}

func TestShortCallerStopsWaitingButStartGoesOn(t *testing.T) {
	r := newRig(t, "FAKE_DELAY=700ms")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := r.h.Recall(ctx, backend.Query{Text: "anything", Limit: 3}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a short caller got %v", err)
	}
	if _, _, err := r.h.List(ctxFor(t), protocol.MemoryYou, 0, 1); err != nil {
		t.Fatal(err)
	}
}

func TestStubbornChildIsKilled(t *testing.T) {
	r := newRig(t, "FAKE_STUBBORN=1", "FAKE_PG=1")
	r.proc.stopWait = 300 * time.Millisecond
	if _, _, err := r.h.List(ctxFor(t), protocol.MemoryYou, 0, 1); err != nil {
		t.Fatal(err)
	}
	pid := r.pid(t)
	start := time.Now()
	r.proc.Stop()
	if r.file("fake-term") == "" {
		t.Fatal("no SIGTERM came first")
	}
	if alive(pid) {
		t.Fatal("a child that ignored SIGTERM is still alive")
	}
	if waited := time.Since(start); waited < 300*time.Millisecond {
		t.Fatalf("SIGKILL came after %s, before the wait", waited)
	}
	calls, _ := os.ReadFile(filepath.Join(r.layout.home(), "pg0-calls"))
	if strings.TrimSpace(string(calls)) != "-I -c from pg0 import Pg0; Pg0(name='rashin').stop()" {
		t.Fatalf("pg0 was stopped with %q", calls)
	}
	if _, err := os.Stat(r.layout.postmaster()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("Postgres was left running")
	}
}

func TestSettleDrainsOnce(t *testing.T) {
	r := newRig(t, "FAKE_PENDING=2")
	now := time.Now()
	r.h.now = func() time.Time { return now }
	if _, _, err := r.h.List(ctxFor(t), protocol.MemoryYou, 0, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := r.h.Remember(ctxFor(t), []protocol.Memory{{Scope: protocol.MemoryYou, Text: "The user likes tea."}}); err != nil {
		t.Fatal(err)
	}
	r.h.Settle(ctxFor(t))
	if !r.h.Busy() {
		t.Fatal("not Busy while Hindsight has work queued")
	}
	now = now.Add(drainWait + time.Second)
	if r.h.Busy() {
		t.Fatal("still Busy after the drain window")
	}
	r.h.Settle(ctxFor(t))
	if r.h.Busy() {
		t.Fatal("a second Settle drained again")
	}
}

func TestNotInstalledAsksForSetup(t *testing.T) {
	r := newRig(t)
	if err := os.Remove(r.layout.installed()); err != nil {
		t.Fatal(err)
	}
	if status := r.h.Status(ctxFor(t)); status.State != protocol.EngineSetup {
		t.Fatalf("Status = %+v", status)
	}
	if _, _, err := r.h.List(ctxFor(t), protocol.MemoryYou, 0, 1); err == nil {
		t.Fatal("a call started Hindsight that was never set up")
	}
	if r.file("fake-args") != "" {
		t.Fatal("Hindsight started without setup")
	}
}

func TestModelListenerChoosesTheModel(t *testing.T) {
	gateway := &fakeGateway{}
	model := "ollama/qwen3"
	server := httptest.NewServer(chooseModel(gateway.API(), func(context.Context) string { return model }))
	defer server.Close()
	post := func() int {
		resp, err := http.Post(server.URL+chatPath, "application/json", strings.NewReader(`{"model":"rashin-memory","messages":[]}`))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if status := post(); status != http.StatusOK {
		t.Fatalf("status %d", status)
	}
	model = ""
	if status := post(); status != http.StatusServiceUnavailable {
		t.Fatalf("with no model allowed, status %d", status)
	}
	if !slices.Equal(gateway.models, []string{"ollama/qwen3"}) {
		t.Fatalf("the gateway saw models %q", gateway.models)
	}
}

func TestContract(t *testing.T) {
	backendtest.Run(t, func(t *testing.T) backend.Backend {
		server := httptest.NewServer(newFakeServer(backendtest.Canary))
		t.Cleanup(server.Close)
		return Remote(server.URL, backendtest.Canary)
	})
}

func TestContractLocal(t *testing.T) {
	backendtest.Run(t, func(t *testing.T) backend.Backend { return newRig(t).h })
}

func TestRemoteSettleNeverHoldsTheDaemon(t *testing.T) {
	fake := newFakeServer(backendtest.Canary)
	fake.pending = 5
	server := httptest.NewServer(fake)
	defer server.Close()
	h := Remote(server.URL, backendtest.Canary)
	if _, err := h.Remember(ctxFor(t), []protocol.Memory{{Scope: protocol.MemoryYou, Text: "The user likes tea."}}); err != nil {
		t.Fatal(err)
	}
	h.Settle(ctxFor(t))
	if h.Busy() {
		t.Fatal("a remote Hindsight with work queued keeps the daemon up")
	}
}

func TestEditTextStaysOutOfAddresses(t *testing.T) {
	fake := newFakeServer(backendtest.Canary)
	var mu sync.Mutex
	var queries []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		queries = append(queries, r.URL.RawQuery)
		mu.Unlock()
		fake.ServeHTTP(w, r)
	}))
	defer server.Close()
	h := Remote(server.URL, backendtest.Canary)
	kept, err := h.Remember(ctxFor(t), []protocol.Memory{{Scope: protocol.MemoryYou, Text: "The user likes tea."}})
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	queries = nil
	mu.Unlock()
	if _, err := h.Edit(ctxFor(t), kept[0].ID, "The user likes oolong."); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, query := range queries {
		if strings.Contains(query, "oolong") {
			t.Fatalf("the edited text went in an address: %q", query)
		}
	}
}

func TestTransportErrorsHoldNoAddress(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	h := Remote(server.URL, backendtest.Canary)
	server.Close()
	_, err := h.Recall(ctxFor(t), backend.Query{Text: "s-private", Limit: 1})
	if err == nil || strings.Contains(err.Error(), "/v1/") || strings.Contains(err.Error(), "s-private") {
		t.Fatalf("a transport error reads %v", err)
	}
}

func TestRemoteKeyNeverShows(t *testing.T) {
	server := httptest.NewServer(newFakeServer("another-key"))
	h := Remote(server.URL, backendtest.Canary)
	_, _, err := h.List(ctxFor(t), "", 0, 10)
	if err == nil || strings.Contains(err.Error(), backendtest.Canary) {
		t.Fatalf("a refused key gave %v", err)
	}
	server.Close()
	status := h.Status(ctxFor(t))
	if status.State != protocol.EngineUnreachable || strings.Contains(status.Detail, backendtest.Canary) {
		t.Fatalf("a gone server gave %+v", status)
	}
}
