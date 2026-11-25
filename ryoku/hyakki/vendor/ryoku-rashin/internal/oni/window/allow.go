package window

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"slices"
)

// allowlist is every command this package may start. Each is a read verb: a
// list, a capability report or an event stream. Nothing here can focus, move,
// type or click, and Allowed refuses anything not spelled out.
var allowlist = map[string][][]string{
	ProviderNiri: {
		{"msg", "--json", "windows"},
		{"msg", "--json", "outputs"},
		{"msg", "--json", "workspaces"},
		{"msg", "--json", "event-stream"},
	},
	CommandHyprland: {
		{"-j", "clients"},
		{"-j", "monitors"},
		{"-j", "cursorpos"},
		{"-j", "layers"},
		// the snapshot batch: exactly the two list reads above, nothing else
		{"--batch", hyprSnapshotBatch},
	},
	"ryoku": {
		{"wm", "state"},
		{"wm", "caps"},
	},
}

// seamWatchKinds are the frame kinds `ryoku wm watch` may be narrowed to.
var seamWatchKinds = []string{"windows", "outputs", "workspaces", "focus"}

// Allowed reports whether name and args are one of the read verbs above.
func Allowed(name string, args ...string) bool {
	if name == "ryoku" && len(args) >= 2 && args[0] == "wm" && args[1] == "watch" {
		seen := map[string]bool{}
		for _, k := range args[2:] {
			if !slices.Contains(seamWatchKinds, k) || seen[k] {
				return false
			}
			seen[k] = true
		}
		return true
	}
	for _, argv := range allowlist[name] {
		if slices.Equal(argv, args) {
			return true
		}
	}
	return false
}

func refuse(name string, args []string) error {
	return fmt.Errorf("%w: %s %q", ErrNotAllowed, name, args)
}

// guard wraps a Runner so every call is checked before it runs, whatever the
// Runner is. Providers only ever call through it.
func guard(run Runner) Runner {
	return func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if !Allowed(name, args...) {
			return nil, refuse(name, args)
		}
		if run == nil {
			return nil, fmt.Errorf("window: no runner for %s", name)
		}
		return run(ctx, name, args...)
	}
}

func guardStream(stream Stream) Stream {
	return func(ctx context.Context, name string, args ...string) (io.ReadCloser, error) {
		if !Allowed(name, args...) {
			return nil, refuse(name, args)
		}
		if stream == nil {
			return nil, ErrNoStream
		}
		return stream(ctx, name, args...)
	}
}

// Exec is the real Runner: it starts an allowed command with env and returns
// its stdout.
func Exec(env []string) Runner {
	return func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if !Allowed(name, args...) {
			return nil, refuse(name, args)
		}
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Env = env
		return cmd.Output()
	}
}

// ExecStream is the real Stream for event-stream commands. Closing the reader
// ends the command.
func ExecStream(env []string) Stream {
	return func(ctx context.Context, name string, args ...string) (io.ReadCloser, error) {
		if !Allowed(name, args...) {
			return nil, refuse(name, args)
		}
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Env = env
		out, err := cmd.StdoutPipe()
		if err != nil {
			return nil, err
		}
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		return &execReader{ReadCloser: out, cmd: cmd}, nil
	}
}

type execReader struct {
	io.ReadCloser
	cmd *exec.Cmd
}

func (r *execReader) Close() error {
	err := r.ReadCloser.Close()
	if r.cmd.Process != nil {
		_ = r.cmd.Process.Kill()
	}
	_ = r.cmd.Wait()
	return err
}
