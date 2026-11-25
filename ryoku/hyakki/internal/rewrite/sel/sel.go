// Package sel reads the text the person has selected, the Wayland PRIMARY
// selection, for a rewrite in place. It is the one place Hyakki names
// wl-paste, and it runs exactly one argv:
//
//	wl-paste --primary --no-newline --type text/plain;charset=utf-8
//
// started without a shell, with no stdin and an environment that names one
// display and nothing else. It never reads or writes the clipboard, never
// asks an app to copy (there is no Ctrl+C fallback), and never logs or
// returns the text in an error: when nothing is selected, or the selection
// cannot be read, the read fails and the caller does nothing.
package sel

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// Tool is the one program this package starts.
const Tool = "wl-paste"

// Args is the only argv Tool is started with: the PRIMARY selection, as
// UTF-8 plain text, without a newline added.
var Args = []string{"--primary", "--no-newline", "--type", "text/plain;charset=utf-8"}

// DefaultTimeout bounds one read.
const DefaultTimeout = 2 * time.Second

var (
	// ErrNotAllowed is returned for any argv but Tool with Args.
	ErrNotAllowed = errors.New("sel: command not in the selection allowlist")
	// ErrEnv is returned for an environment that names more than one display
	// or anything besides it.
	ErrEnv = errors.New("sel: the environment must name one display and nothing else")
	// ErrEmpty: nothing is selected, or only white space.
	ErrEmpty = errors.New("sel: nothing is selected")
	// ErrUnreadable: the selection could not be read as UTF-8 text.
	ErrUnreadable = errors.New("sel: the selection cannot be read")
	// ErrTooLong: the selection is longer than the caller takes.
	ErrTooLong = errors.New("sel: the selection is too long")
)

// Runner starts name with args and returns at most limit bytes of its
// stdout; a longer output is cut to limit bytes. Exec is the real one; tests
// pass a fake, so nothing is started.
type Runner func(ctx context.Context, limit int, name string, args ...string) ([]byte, error)

// Allowed reports whether name and args are exactly Tool and Args.
func Allowed(name string, args ...string) bool {
	return name == Tool && slices.Equal(args, Args)
}

// Env is the whole environment of a read on the display named display in
// runtimeDir: PATH to find the tool, the runtime dir and the display.
func Env(runtimeDir, display string) ([]string, error) {
	if runtimeDir == "" || !filepath.IsAbs(runtimeDir) || filepath.Clean(runtimeDir) != runtimeDir || strings.ContainsAny(runtimeDir, "\x00\n=") {
		return nil, ErrEnv
	}
	if display == "" || strings.ContainsAny(display, "/\x00\n=") || display == "." || display == ".." {
		return nil, ErrEnv
	}
	return []string{"PATH=" + os.Getenv("PATH"), "XDG_RUNTIME_DIR=" + runtimeDir, "WAYLAND_DISPLAY=" + display}, nil
}

// envOK reports whether env is exactly what Env builds.
func envOK(env []string) bool {
	if len(env) != 3 || !strings.HasPrefix(env[0], "PATH=") {
		return false
	}
	dir, ok1 := strings.CutPrefix(env[1], "XDG_RUNTIME_DIR=")
	display, ok2 := strings.CutPrefix(env[2], "WAYLAND_DISPLAY=")
	if !ok1 || !ok2 {
		return false
	}
	_, err := Env(dir, display)
	return err == nil
}

// Exec is the real Runner for the display env names. It refuses an env Env
// would not build and any argv Allowed refuses, before starting anything.
func Exec(env []string) Runner {
	env = slices.Clone(env)
	return func(ctx context.Context, limit int, name string, args ...string) ([]byte, error) {
		if !envOK(env) {
			return nil, ErrEnv
		}
		if !Allowed(name, args...) {
			return nil, ErrNotAllowed
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
		var buf bytes.Buffer
		_, readErr := io.Copy(&buf, io.LimitReader(out, int64(limit)))
		// drain the rest so the tool is not blocked writing, keeping none of it
		_, _ = io.Copy(io.Discard, out)
		waitErr := cmd.Wait()
		if readErr != nil {
			clear(buf.Bytes())
			return nil, readErr
		}
		if waitErr != nil {
			clear(buf.Bytes())
			// wl-paste exits non-zero when no selection, or no text type,
			// is offered: the person sees the same as an empty selection
			var exit *exec.ExitError
			if errors.As(waitErr, &exit) && ctx.Err() == nil {
				return nil, ErrEmpty
			}
			return nil, waitErr
		}
		return buf.Bytes(), nil
	}
}

// Read returns the selected text, at most max characters. It fails with
// ErrEmpty when nothing (or only white space) is selected or the tool says
// so, ErrUnreadable when the tool fails or the bytes are not UTF-8, and
// ErrTooLong past max. The bytes read are zeroed before Read returns.
func Read(ctx context.Context, run Runner, max int) (string, error) {
	if run == nil || max <= 0 {
		return "", ErrUnreadable
	}
	ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()
	limit := max*utf8.UTFMax + 1
	raw, err := run(ctx, limit, Tool, Args...)
	defer clear(raw)
	if err != nil {
		if errors.Is(err, ErrEmpty) && ctx.Err() == nil {
			return "", ErrEmpty
		}
		return "", ErrUnreadable
	}
	if len(raw) >= limit {
		return "", ErrTooLong
	}
	if !utf8.Valid(raw) {
		return "", ErrUnreadable
	}
	text := string(raw)
	if strings.TrimSpace(text) == "" {
		return "", ErrEmpty
	}
	if utf8.RuneCountInString(text) > max {
		return "", ErrTooLong
	}
	return text, nil
}
