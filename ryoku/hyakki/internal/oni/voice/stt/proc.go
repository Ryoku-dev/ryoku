package stt

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Starter starts a resident engine child; tests inject one.
type Starter interface {
	Start(argv []string) (Process, error)
}

// Process is a started engine child. The adapter signals it to stop and
// calls Wait exactly once.
type Process interface {
	Pid() int
	Signal(os.Signal) error
	Wait() error
	// StderrTail is the end of what the child wrote to stderr.
	StderrTail() string
}

// Runner runs a one-shot engine child to completion and returns its
// stdout; tests inject one.
type Runner interface {
	Run(ctx context.Context, argv []string) ([]byte, error)
}

// childEnv is the environment an engine gets: the search path, the
// locale and the compute library knobs, nothing of the user's secrets or
// proxies.
func childEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		switch {
		case k == "PATH", k == "LANG", k == "LC_ALL", k == "LD_LIBRARY_PATH", k == "CUDA_VISIBLE_DEVICES",
			strings.HasPrefix(k, "GGML_"):
			env = append(env, kv)
		}
	}
	return env
}

func childAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
}

// tail keeps the last n bytes written.
type tail struct {
	mu  sync.Mutex
	n   int
	buf []byte
}

func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.n {
		t.buf = append([]byte(nil), t.buf[len(t.buf)-t.n:]...)
	}
	return len(p), nil
}

func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.TrimSpace(string(t.buf))
}

// ExecStarter starts the child as its own process group, in /, with
// childEnv and stdout discarded (whisper-server prints what it heard there),
// killed if Hyakki dies.
type ExecStarter struct{}

type execProcess struct {
	cmd    *exec.Cmd
	stderr *tail
}

func (ExecStarter) Start(argv []string) (Process, error) {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = "/"
	cmd.Env = childEnv()
	cmd.SysProcAttr = childAttr()
	stderr := &tail{n: 4096}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &execProcess{cmd: cmd, stderr: stderr}, nil
}

func (p *execProcess) Pid() int                 { return p.cmd.Process.Pid }
func (p *execProcess) Signal(s os.Signal) error { return p.cmd.Process.Signal(s) }
func (p *execProcess) Wait() error              { return p.cmd.Wait() }
func (p *execProcess) StderrTail() string       { return p.stderr.String() }

// ExecRunner runs the child as its own process group, in /, with childEnv;
// a cancelled context kills the whole group.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, argv []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = "/"
	cmd.Env = childEnv()
	cmd.SysProcAttr = childAttr()
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 2 * time.Second
	var stdout bytes.Buffer
	stderr := &tail{n: 2048}
	cmd.Stdout, cmd.Stderr = &limited{w: &stdout, n: 1 << 16}, stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if s := stderr.String(); s != "" {
			return nil, fmt.Errorf("%s: %w: %s", argv[0], err, s)
		}
		return nil, fmt.Errorf("%s: %w", argv[0], err)
	}
	return stdout.Bytes(), nil
}

// limited keeps the first n bytes and discards the rest.
type limited struct {
	w io.Writer
	n int
}

func (l *limited) Write(p []byte) (int, error) {
	keep := min(len(p), l.n)
	if keep > 0 {
		if _, err := l.w.Write(p[:keep]); err != nil {
			return 0, err
		}
		l.n -= keep
	}
	return len(p), nil
}
