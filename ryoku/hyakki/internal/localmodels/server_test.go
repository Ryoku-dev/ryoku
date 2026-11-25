package localmodels

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"ryoku-rashin/internal/childproc"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

type localBroadcastCapture struct{ states chan string }

func (b localBroadcastCapture) Broadcast(event string, data any) {
	if event == protocol.EvtLocalChanged {
		b.states <- data.(protocol.LocalResult).Runtime.State
	}
}

func TestStopBroadcastsProgressAndWaitsForChildExit(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	broadcast := localBroadcastCapture{states: make(chan string, 8)}
	manager := New(func() protocol.Settings { return protocol.Settings{} }, broadcast)
	cmd := exec.Command("/bin/sleep", "60")
	cmd.SysProcAttr = childproc.Attr()
	exited := make(chan struct{})
	if err := childproc.LaunchLocked(cmd, exited); err != nil {
		t.Fatal(err)
	}
	manager.mu.Lock()
	manager.server = &server{cmd: cmd, exited: exited}
	manager.mu.Unlock()
	manager.Stop()
	if first, second := <-broadcast.states, <-broadcast.states; first != protocol.RuntimeStopping || second == protocol.RuntimeStopping {
		t.Fatalf("stop states %q, %q", first, second)
	}
	select {
	case <-exited:
	default:
		t.Fatal("stop returned before the child exited")
	}
	if err := syscall.Kill(cmd.Process.Pid, 0); err != syscall.ESRCH {
		t.Fatalf("stop left process %d: %v", cmd.Process.Pid, err)
	}
}

func TestIdleStopPreservesLeasedChildAndReapsProcess(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	manager := New(func() protocol.Settings { return protocol.Settings{} }, nil)
	cmd := exec.Command("/bin/sleep", "60")
	cmd.SysProcAttr = childproc.Attr()
	exited := make(chan struct{})
	if err := childproc.LaunchLocked(cmd, exited); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		select {
		case <-exited:
		default:
			childproc.Terminate(cmd, exited, time.Second)
		}
	})
	manager.mu.Lock()
	manager.server = &server{cmd: cmd, exited: exited}
	manager.mu.Unlock()
	release := manager.Acquire()
	manager.stopIfIdle()
	select {
	case <-exited:
		t.Fatal("idle stop killed an active request")
	default:
	}
	release()
	manager.stopIfIdle()
	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatal("idle stop left the child running")
	}
	manager.mu.Lock()
	still := manager.server != nil
	manager.mu.Unlock()
	if still {
		t.Fatal("idle stop kept a stopped child")
	}
}

func TestServerKeyOverridesInheritedLlamaKey(t *testing.T) {
	t.Setenv("LLAMA_API_KEY", "userskey")
	t.Setenv("LLAMA_ARG_API_KEY", "usersarg")
	env := serverEnv("rashinkey")
	keys := 0
	for _, value := range env {
		if strings.HasPrefix(value, "LLAMA_API_KEY=") {
			keys++
			if value != "LLAMA_API_KEY=rashinkey" {
				t.Fatalf("router child got inherited key %q", value)
			}
		}
		if strings.HasPrefix(value, "LLAMA_ARG_API_KEY=") {
			t.Fatalf("router child got inherited key override %q", value)
		}
	}
	if keys != 1 {
		t.Fatalf("router child has %d keys, want exactly one per-start key", keys)
	}
}

func testModel(t *testing.T) Entry {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	data := []byte("model")
	hash := sha256.Sum256(data)
	e := Entry{ID: "test-model", Repo: "Unit/model", Revision: "abcdef123456789012345678901234567890abcd",
		Name: "Test model", Context: 8192, Files: []File{{Name: "model.gguf", Role: protocol.LocalFileModel,
			Size: int64(len(data)), SHA256: hex.EncodeToString(hash[:])}}}
	if err := os.MkdirAll(repoDir(e.Repo), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath(e, "model.gguf"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeManifest(e, manifest{ID: e.ID, Repo: e.Repo, Revision: e.Revision, Files: e.Files}); err != nil {
		t.Fatal(err)
	}
	if err := writeCustom([]Entry{e}); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestEnsureRefreshesNewPresetWithoutStoppingRouter(t *testing.T) {
	e := testModel(t)
	var reloads atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer key" {
			http.Error(w, "wrong key", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/models":
			if r.URL.Query().Get("reload") != "" {
				reloads.Add(1)
				data, err := os.ReadFile(paths.LlamaPresets())
				if err != nil || !strings.Contains(string(data), "["+e.ID+"]") {
					t.Errorf("reloaded presets without newly installed model: %v", err)
				}
			}
			fmt.Fprintf(w, `{"data":[{"id":%q,"status":{"value":"loaded"}}]}`, e.ID)
		case "/props":
			fmt.Fprint(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()
	manager := New(func() protocol.Settings { return protocol.Settings{} }, nil)
	s := &server{base: api.URL, key: "key", build: 12521, router: true, exited: make(chan struct{}),
		loaded: map[string]bool{}, loading: map[string]chan struct{}{}, presets: map[string]bool{}}
	manager.server = s
	t.Cleanup(func() { manager.Stop() })
	if _, _, err := manager.Ensure(context.Background(), e.ID); err != nil {
		t.Fatal(err)
	}
	if reloads.Load() != 1 || !s.presets[e.ID] {
		t.Fatalf("new preset not activated: reloads=%d presets=%v", reloads.Load(), s.presets)
	}
	if _, _, err := manager.Ensure(context.Background(), e.ID); err != nil || reloads.Load() != 1 {
		t.Fatalf("preset unexpectedly reloaded again: err=%v reloads=%d", err, reloads.Load())
	}
}

func TestEnsureLoadedResumesExistingRouterLoad(t *testing.T) {
	e := testModel(t)
	for _, initial := range []string{"loading", "loaded", "already running"} {
		t.Run(initial, func(t *testing.T) {
			var posts atomic.Int32
			var reads atomic.Int32
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/models":
					n := reads.Add(1)
					value := initial
					if initial == "already running" {
						value = "unloaded"
					}
					if n > 1 {
						value = "loaded"
					}
					fmt.Fprintf(w, `{"data":[{"id":%q,"status":{"value":%q}}]}`, e.ID, value)
				case "/models/load":
					posts.Add(1)
					http.Error(w, `{"message":"model is already running"}`, http.StatusBadRequest)
				case "/props":
					fmt.Fprint(w, `{}`)
				default:
					http.NotFound(w, r)
				}
			}))
			defer api.Close()
			manager := New(func() protocol.Settings { return protocol.Settings{} }, nil)
			s := &server{base: api.URL, key: "key", exited: make(chan struct{}),
				loaded: map[string]bool{}, loading: map[string]chan struct{}{}}
			if err := manager.ensureLoaded(context.Background(), s, e); err != nil {
				t.Fatal(err)
			}
			want := int32(0)
			if initial == "already running" {
				want = 1
			}
			if posts.Load() != want {
				t.Fatalf("load posts = %d, want %d", posts.Load(), want)
			}
		})
	}
}

func TestRemoveWaitsForOtherModelLease(t *testing.T) {
	e := testModel(t)
	manager := New(func() protocol.Settings { return protocol.Settings{} }, nil)
	cmd := exec.Command("/bin/sleep", "60")
	cmd.SysProcAttr = childproc.Attr()
	exited := make(chan struct{})
	if err := childproc.LaunchLocked(cmd, exited); err != nil {
		t.Fatal(err)
	}
	manager.server = &server{cmd: cmd, exited: exited}
	t.Cleanup(func() { childproc.Terminate(cmd, exited, time.Second) })
	release := manager.Acquire()
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := manager.Remove(ctx, e.ID); err != context.DeadlineExceeded {
		t.Fatalf("removal while another session is streaming: %v", err)
	}
	select {
	case <-exited:
		t.Fatal("removal killed a leased stream")
	default:
	}
	release()
	if _, err := manager.Remove(context.Background(), e.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-exited:
	case <-time.After(time.Second):
		t.Fatal("removed model left router running")
	}
}
