package capture

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"ryoku-rashin/internal/oni/window"
)

// MaxStdout bounds what grim may write before it is stopped: the largest
// admitted picture, uncompressed, plus PNG framing.
const MaxStdout = MaxPixels*4 + 8<<20

// Exec is the real grim runner. It starts only argv the allowlist admits,
// with env as the whole environment, keeps stdout in memory and stops grim
// once it writes more than MaxStdout.
func Exec(env []string) window.Runner {
	return func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if !Allowed(name, args...) {
			return nil, fmt.Errorf("%w: %s %s", ErrNotAllowed, name, strings.Join(args, " "))
		}
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		out := &bounded{max: MaxStdout, stop: cancel}
		var stderr bytes.Buffer
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Env = env
		cmd.Stdout = out
		cmd.Stderr = &limited{b: &stderr, max: 4096}
		err := cmd.Run()
		if out.over {
			out.clear()
			return nil, ErrTooLarge
		}
		if err != nil {
			out.clear()
			if msg := strings.TrimSpace(stderr.String()); msg != "" {
				return nil, fmt.Errorf("grim: %w: %s", err, msg)
			}
			return nil, fmt.Errorf("grim: %w", err)
		}
		return out.buf, nil
	}
}

// bounded keeps writes up to max bytes and stops the command past that.
// Growing copies are zeroed so no stray picture bytes stay on the heap.
type bounded struct {
	buf  []byte
	max  int
	over bool
	stop func()
}

func (b *bounded) Write(p []byte) (int, error) {
	if b.over || len(b.buf)+len(p) > b.max {
		b.over = true
		b.stop()
		return 0, ErrTooLarge
	}
	if len(b.buf)+len(p) > cap(b.buf) {
		grown := make([]byte, len(b.buf), max(2*cap(b.buf), len(b.buf)+len(p), 1<<20))
		copy(grown, b.buf)
		zero(b.buf)
		b.buf = grown
	}
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *bounded) clear() {
	zero(b.buf[:cap(b.buf)])
	b.buf = nil
}

type limited struct {
	b   *bytes.Buffer
	max int
}

func (l *limited) Write(p []byte) (int, error) {
	if room := l.max - l.b.Len(); room > 0 {
		l.b.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}
