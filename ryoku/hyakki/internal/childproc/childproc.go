// Package childproc holds what rashind needs to run a helper server as its
// child: a launch that ties the child's life to the daemon, a health wait, a
// process-group stop, a free loopback port and a per-start secret. Hindsight
// and Rashin's llama.cpp router share it.
package childproc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"syscall"
	"time"
)

// ErrExited is WaitHealthy's error when the child exits before it answers.
var ErrExited = errors.New("the child exited while starting")

// ErrTimeout is WaitHealthy's error when the wait passes without an answer.
var ErrTimeout = errors.New("the child did not answer in time")

// Attr puts the child in its own process group, so a stop reaches what it
// spawns, and sends it SIGTERM when the thread that started it dies.
func Attr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
}

// LaunchLocked starts cmd from a goroutine that keeps its OS thread until the
// child exits, then closes exited. Pdeathsig follows the thread that started
// the child, not the process, so the child must not outlive that thread by
// accident.
func LaunchLocked(cmd *exec.Cmd, exited chan struct{}) error {
	started := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		if err := cmd.Start(); err != nil {
			started <- err
			return
		}
		started <- nil
		_ = cmd.Wait()
		close(exited)
	}()
	return <-started
}

// Health is what WaitHealthy polls: URL with Bearer, every Period, for at most
// Wait, giving up when Exited closes.
type Health struct {
	URL    string
	Bearer string
	Exited <-chan struct{}
	Wait   time.Duration
	Period time.Duration
}

// WaitHealthy polls h.URL until it answers 200. It fails with ErrExited when
// the child exits, ErrTimeout when the wait passes, or ctx's error.
func WaitHealthy(ctx context.Context, h Health) error {
	period := h.Period
	if period <= 0 {
		period = 250 * time.Millisecond
	}
	deadline := time.NewTimer(h.Wait)
	defer deadline.Stop()
	tick := time.NewTicker(period)
	defer tick.Stop()
	client := &http.Client{Timeout: 2 * time.Second}
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.URL, nil)
		if err != nil {
			return err
		}
		if h.Bearer != "" {
			req.Header.Set("Authorization", "Bearer "+h.Bearer)
		}
		if resp, err := client.Do(req); err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case <-tick.C:
		case <-h.Exited:
			return ErrExited
		case <-deadline.C:
			return ErrTimeout
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Terminate sends SIGTERM to cmd's process group and SIGKILL after wait,
// returning once the child has exited. A command that never started is left
// alone.
func Terminate(cmd *exec.Cmd, exited <-chan struct{}, wait time.Duration) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	select {
	case <-exited:
		return
	default:
	}
	group := -cmd.Process.Pid
	_ = syscall.Kill(group, syscall.SIGTERM)
	select {
	case <-exited:
	case <-time.After(wait):
		_ = syscall.Kill(group, syscall.SIGKILL)
		<-exited
	}
}

// FreePort asks the kernel for a loopback port nothing holds.
func FreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// NewSecret is 256 random bits in hex, safe in a URL, a header and an
// environment.
func NewSecret() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}
