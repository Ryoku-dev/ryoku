// Package proc is the one place reflex executors start processes. Every
// start goes through a guard that checks the exact argv against the calling
// executor's allowlist first, and nothing here ever runs a shell: argv[0] is
// the program and the rest are its arguments, passed as they are.
package proc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// ErrNotAllowed means an argv is not on the executor's allowlist. Nothing
// was started.
var ErrNotAllowed = errors.New("reflex: command not allowed")

// Runner runs a short command to completion and returns its stdout.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// Spawner starts argv detached and returns at once. exited delivers the
// command's end exactly once (nil for a clean exit), so a caller may watch
// a short grace and then stop listening; the channel is buffered and never
// blocks the waiter.
type Spawner func(argv []string) (exited <-chan error, err error)

// Allow reports whether name and args are allowed, as each executor's
// Allowed does.
type Allow func(name string, args ...string) bool

// Refuse is the error for an argv that is not allowed.
func Refuse(name string, args []string) error {
	return fmt.Errorf("%w: %s %q", ErrNotAllowed, name, args)
}

// Guard wraps run so every call is checked against allow before it runs,
// whatever run is. A nil run is an error, never a silent success.
func Guard(allow Allow, run Runner) Runner {
	return func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if !allow(name, args...) {
			return nil, Refuse(name, args)
		}
		if run == nil {
			return nil, fmt.Errorf("reflex: no runner for %s", name)
		}
		return run(ctx, name, args...)
	}
}

// GuardSpawn is Guard for a Spawner.
func GuardSpawn(allow Allow, spawn Spawner) Spawner {
	return func(argv []string) (<-chan error, error) {
		if len(argv) == 0 {
			return nil, Refuse("", nil)
		}
		if !allow(argv[0], argv[1:]...) {
			return nil, Refuse(argv[0], argv[1:])
		}
		if spawn == nil {
			return nil, fmt.Errorf("reflex: no spawner for %s", argv[0])
		}
		return spawn(argv)
	}
}

// Exec is the real Runner: it runs the command with env and returns its
// stdout. Callers always reach it through Guard.
func Exec(env []string) Runner {
	return func(ctx context.Context, name string, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Env = env
		return cmd.Output()
	}
}

// Detached is the real Spawner. The child gets its own session, stdin,
// stdout and stderr on /dev/null and dir as its working directory, so it
// holds nothing of the daemon's and keeps running when the daemon exits.
// One goroutine reaps it, so no zombie is left behind.
func Detached(env []string, dir string) Spawner {
	return func(argv []string) (<-chan error, error) {
		devnull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
		if err != nil {
			return nil, err
		}
		defer devnull.Close()
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Env = env
		cmd.Dir = dir
		cmd.Stdin, cmd.Stdout, cmd.Stderr = devnull, devnull, devnull
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		exited := make(chan error, 1)
		go func() { exited <- cmd.Wait() }()
		return exited, nil
	}
}
