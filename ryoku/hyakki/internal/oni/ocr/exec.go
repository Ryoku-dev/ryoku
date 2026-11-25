package ocr

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// MaxOutput bounds what tesseract may write before it is stopped: the TSV
// of MaxWords words is a small fraction of it.
const MaxOutput = 4 << 20

// Exec is the real tesseract runner. It starts only argv Allowed admits,
// found on the PATH in env, with env as the whole environment (the
// process's own when env is nil) and "/" as its working directory. The
// picture goes in on stdin; stdout stays in memory and tesseract is stopped
// once it writes more than MaxOutput. A missing program or language is
// ErrUnavailable.
func Exec(env []string) Runner {
	return func(ctx context.Context, name string, args []string, stdin []byte) ([]byte, error) {
		if !Allowed(name, args...) {
			return nil, fmt.Errorf("%w: %s %s", ErrNotAllowed, name, strings.Join(args, " "))
		}
		bin, ok := readerPath(env)
		if !ok {
			return nil, ErrUnavailable
		}
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		out := &bounded{max: MaxOutput, stop: cancel}
		var stderr bytes.Buffer
		cmd := exec.CommandContext(ctx, bin, args...)
		cmd.Env = env
		cmd.Dir = "/"
		cmd.Stdin = bytes.NewReader(stdin)
		cmd.Stdout = out
		cmd.Stderr = &limited{b: &stderr, max: 4096}
		err := cmd.Run()
		if out.over {
			out.clear()
			return nil, ErrTooLarge
		}
		if err != nil {
			out.clear()
			msg := strings.TrimSpace(stderr.String())
			if strings.Contains(msg, "Failed loading language") {
				return nil, fmt.Errorf("%w: language %s", ErrUnavailable, args[3])
			}
			if msg != "" {
				return nil, fmt.Errorf("tesseract: %w: %s", err, msg)
			}
			return nil, fmt.Errorf("tesseract: %w", err)
		}
		return out.buf, nil
	}
}

// Available checks both distribution names without starting the reader.
func Available() bool {
	_, ok := readerPath(nil)
	return ok
}

func readerPath(env []string) (string, bool) {
	if bin, ok := lookPath(Tool, env); ok {
		return bin, true
	}
	return lookPath("tesseract-ocr", env)
}

// lookPath finds name in the PATH env gives, or in the process's PATH when
// env is nil. Relative PATH entries are skipped, so the working directory
// is never searched.
func lookPath(name string, env []string) (string, bool) {
	path := os.Getenv("PATH")
	if env != nil {
		path = ""
		for _, kv := range env {
			if v, ok := strings.CutPrefix(kv, "PATH="); ok {
				path = v
			}
		}
	}
	for _, dir := range filepath.SplitList(path) {
		if !filepath.IsAbs(dir) {
			continue
		}
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() && st.Mode().Perm()&0o111 != 0 {
			return p, true
		}
	}
	return "", false
}

// bounded keeps writes up to max bytes and stops the command past that.
// Growing copies are zeroed, so no stray screen text stays on the heap.
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
		grown := make([]byte, len(b.buf), max(2*cap(b.buf), len(b.buf)+len(p), 64<<10))
		copy(grown, b.buf)
		clear(b.buf)
		b.buf = grown
	}
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *bounded) clear() {
	clear(b.buf[:cap(b.buf)])
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
