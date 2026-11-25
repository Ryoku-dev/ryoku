package stt

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/spec"
)

// fakeClock fires idle timers only when the test says so.
type fakeClock struct {
	mu     sync.Mutex
	timers []*fakeTimer
}

type fakeTimer struct {
	d       time.Duration
	f       func()
	mu      sync.Mutex
	stopped bool
}

func (c *fakeClock) Now() time.Time { return time.Unix(0, 0) }

func (c *fakeClock) AfterFunc(d time.Duration, f func()) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{d: d, f: f}
	c.timers = append(c.timers, t)
	return t
}

func (t *fakeTimer) Stop() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	was := !t.stopped
	t.stopped = true
	return was
}

func (c *fakeClock) armed() []*fakeTimer {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []*fakeTimer
	for _, t := range c.timers {
		t.mu.Lock()
		if !t.stopped {
			out = append(out, t)
		}
		t.mu.Unlock()
	}
	return out
}

func (c *fakeClock) all() []*fakeTimer {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]*fakeTimer(nil), c.timers...)
}

// fakeEngine stands in for whisper-server: Start binds, after load, an
// HTTP server in this process on the argv's --host/--port that answers
// only on --request-path + --inference-path.
type fakeEngine struct {
	t       *testing.T
	handler http.HandlerFunc
	// deaf makes the child exit at once without listening.
	deaf bool
	// load delays the listen, as a model load does.
	load time.Duration

	mu    sync.Mutex
	argvs [][]string
	procs []*fakeProc
	hits  []string
}

type fakeProc struct {
	stop    func()
	once    sync.Once
	done    chan struct{}
	stderr  string
	mu      sync.Mutex
	signals []os.Signal
}

func flagValue(argv []string, name string) string {
	for i := 0; i+1 < len(argv); i++ {
		if argv[i] == name {
			return argv[i+1]
		}
	}
	return ""
}

func (f *fakeEngine) Start(argv []string) (Process, error) {
	f.mu.Lock()
	f.argvs = append(f.argvs, append([]string(nil), argv...))
	f.mu.Unlock()
	p := &fakeProc{done: make(chan struct{})}
	f.mu.Lock()
	f.procs = append(f.procs, p)
	f.mu.Unlock()
	if f.deaf {
		p.stderr = "couldn't bind to server socket"
		p.stop = func() {}
		p.once.Do(func() { close(p.done) })
		return p, nil
	}
	host, port := flagValue(argv, "--host"), flagValue(argv, "--port")
	route := flagValue(argv, "--request-path") + flagValue(argv, "--inference-path")
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.hits = append(f.hits, r.Method+" "+r.URL.Path)
		f.mu.Unlock()
		if r.Method != http.MethodPost || r.URL.Path != route {
			http.NotFound(w, r)
			return
		}
		f.handler(w, r)
	})
	srv := &http.Server{Handler: mux}
	var stopped sync.Once
	p.stop = func() { stopped.Do(func() { srv.Close() }) }
	// like the real engine, listen only after loading the model
	go func() {
		time.Sleep(f.load)
		ln, err := net.Listen("tcp4", net.JoinHostPort(host, port))
		if err != nil {
			f.t.Error(err)
			return
		}
		_ = srv.Serve(ln)
	}()
	return p, nil
}

func (f *fakeEngine) started() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.argvs)
}

func (f *fakeEngine) proc(i int) *fakeProc {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.procs[i]
}

// Pid is the test process: the fake engine's socket is this process's
// own, so the adapter's real /proc ownership probe applies.
func (p *fakeProc) Pid() int { return os.Getpid() }
func (p *fakeProc) StderrTail() string {
	return p.stderr
}

func (p *fakeProc) Signal(s os.Signal) error {
	p.mu.Lock()
	p.signals = append(p.signals, s)
	p.mu.Unlock()
	if s == syscall.SIGTERM || s == syscall.SIGKILL {
		p.once.Do(func() {
			p.stop()
			close(p.done)
		})
	}
	return nil
}

func (p *fakeProc) Wait() error { <-p.done; return nil }

func (p *fakeProc) exited() bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

func (p *fakeProc) gotSignals() []os.Signal {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]os.Signal(nil), p.signals...)
}

// fixture writes a fake engine binary and a model with its real sha256.
func fixture(t *testing.T, binName string) (bin, model, sum string) {
	t.Helper()
	dir := t.TempDir()
	bin = filepath.Join(dir, binName)
	if err := os.WriteFile(bin, []byte("#!/bin/false\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	model = filepath.Join(dir, "model.bin")
	data := []byte("not really a model")
	if err := os.WriteFile(model, data, 0o600); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(data)
	return bin, model, hex.EncodeToString(h[:])
}

func specLimits(t *testing.T) Limits {
	t.Helper()
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	return LimitsFromSpec(sp)
}

// speech is a second of a tone; content does not matter to the fakes.
func speech(d time.Duration) []int16 {
	pcm := make([]int16, int(d.Seconds()*SampleRate))
	for i := range pcm {
		pcm[i] = int16((i % 40) * 400)
	}
	return pcm
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}
