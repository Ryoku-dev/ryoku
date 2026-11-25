// Package insert types dictated text into the captured target window. It is
// the only dictation code that drives input, and it drives exactly one tool:
// `wtype -s Settle -- TEXT`, started without a shell. There is no clipboard
// path. Text is typed in chunks of at most MaxChunk characters and the target
// is verified before every chunk; at the first refusal or failure typing
// stops and the untyped rest goes back to the caller, which keeps it on
// screen for Copy and never places it on the clipboard or types it anywhere
// else.
//
// How wtype reads its arguments (wtype 0.4, main.c parse_args): "-s MS" is a
// sleep run before the keys, after the keymap has been sent; everything
// after a "--" argument is text, so text that starts with a dash is never
// read as an option; several text arguments are joined with a space, so a
// chunk is always exactly one argument; "\n" is typed as the Return key and
// "\t" as Tab, while ESC and the other control characters would be typed as
// their keys, so Check refuses them. wtype decodes its text with the C
// library's locale, so Exec runs it under a UTF-8 locale.
package insert

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/dictate/target"
)

// Tool is the one command this package starts.
const Tool = "wtype"

// MaxChunk is the most characters (runes) one wtype call types.
const MaxChunk = 40

// Settle is how long, in milliseconds, each wtype call waits (its -s
// command) after giving the compositor its keymap and before the first key.
// Hyprland applies a new virtual keyboard's keymap a moment later and drops
// a key sent at once: in the nested Hyprland lab a bare `wtype -- TEXT` lost
// its first character in 14 of 15 calls, and none of 15 with a 5 ms wait.
const Settle = "50"

// DefaultChunkTimeout bounds one wtype call.
const DefaultChunkTimeout = 5 * time.Second

// Runner starts one command and waits for it. The real one is Exec; tests
// pass a fake, so nothing is ever typed.
type Runner func(ctx context.Context, name string, args ...string) error

var (
	// ErrNotAllowed is returned for any argv other than `wtype -s Settle -- CHUNK`.
	// Its text never includes the arguments, which hold dictated words.
	ErrNotAllowed = errors.New("insert: command not in the typing allowlist")
	// ErrText is returned before anything is typed when the text holds
	// invalid UTF-8 or a control character other than a newline or tab.
	ErrText = errors.New("insert: text holds characters that would be typed as keys")
	// ErrTool wraps a wtype failure.
	ErrTool = errors.New("insert: wtype failed")
	// ErrNoRunner is returned when the Inserter has no Runner.
	ErrNoRunner = errors.New("insert: no runner")
)

// Message is the user copy for an insert error, falling back to the
// target's refusal copy. It never names the dictated text.
func Message(err error) string {
	var r *target.Refusal
	switch {
	case err == nil:
		return ""
	case errors.As(err, &r):
		return r.Message()
	case errors.Is(err, ErrText):
		return i18n.T("The text holds characters that would press keys rather than type, so nothing was typed. The text stays here to copy.")
	case errors.Is(err, ErrTool), errors.Is(err, ErrNoRunner), errors.Is(err, ErrNotAllowed):
		return i18n.T("Typing failed, so dictation stopped. Check that wtype is installed. The text stays here to copy.")
	default:
		return i18n.T("Typing stopped. The text stays here to copy.")
	}
}

// Check returns ErrText unless text is valid UTF-8 whose only control
// characters are newlines and tabs.
func Check(text string) error {
	if !utf8.ValidString(text) {
		return ErrText
	}
	for _, r := range text {
		if r != '\n' && r != '\t' && unicode.IsControl(r) {
			return ErrText
		}
	}
	return nil
}

// Allowed reports whether name and args are `wtype -s Settle -- CHUNK`, with
// CHUNK one non-empty argument of at most MaxChunk characters that passes
// Check.
func Allowed(name string, args ...string) bool {
	if name != Tool || len(args) != 4 || args[0] != "-s" || args[1] != Settle || args[2] != "--" {
		return false
	}
	c := args[3]
	return c != "" && utf8.RuneCountInString(c) <= MaxChunk && Check(c) == nil
}

// Exec is the real Runner. It refuses anything Allowed refuses before
// starting a process, runs wtype directly with no shell and no stdin, and
// sets a UTF-8 character locale so wtype decodes multi-byte text.
func Exec(env []string) Runner {
	return func(ctx context.Context, name string, args ...string) error {
		if !Allowed(name, args...) {
			return ErrNotAllowed
		}
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Env = utf8Env(env)
		var stderr bytes.Buffer
		cmd.Stderr = &limited{buf: &stderr, max: 512}
		if err := cmd.Run(); err != nil {
			if msg := strings.TrimSpace(stderr.String()); msg != "" {
				return fmt.Errorf("%w (%s)", err, msg)
			}
			return err
		}
		return nil
	}
}

// utf8Env returns env with every locale override replaced by LC_ALL=C.UTF-8.
// wtype only uses LC_CTYPE, and C.UTF-8 is built into glibc.
func utf8Env(env []string) []string {
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if strings.HasPrefix(kv, "LC_ALL=") || strings.HasPrefix(kv, "LC_CTYPE=") {
			continue
		}
		out = append(out, kv)
	}
	return append(out, "LC_ALL=C.UTF-8")
}

type limited struct {
	buf *bytes.Buffer
	max int
}

func (l *limited) Write(p []byte) (int, error) {
	if room := l.max - l.buf.Len(); room > 0 {
		l.buf.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}

// Inserter types text into a target. Run and Windows are required.
type Inserter struct {
	Run     Runner
	Windows target.Source
	// ChunkTimeout bounds each wtype call; 0 means DefaultChunkTimeout.
	ChunkTimeout time.Duration
}

// Result is what Insert typed.
type Result struct {
	// Landed is the exact number of characters (runes) in chunks wtype
	// finished.
	Landed int
	// Rest is the text not confirmed typed, byte for byte: text minus its
	// first Landed characters. The caller shows it for Copy.
	Rest string
	// Unsure is the size, in characters, of the chunk wtype failed on: some
	// of it may have reached the window. It is 0 unless the error is ErrTool.
	// Rest still starts at that chunk.
	Unsure int
}

// Insert verifies t, types one chunk, and repeats until text is typed or a
// step fails. It returns the first failure: a *target.Refusal, ErrText,
// ErrTool, ErrNoRunner or the context's error. Nothing is typed after a
// failure, and the untyped text is only ever returned in Result.Rest.
func (in Inserter) Insert(ctx context.Context, t target.Target, text string) (Result, error) {
	res := Result{Rest: text}
	if text == "" {
		return res, nil
	}
	if err := Check(text); err != nil {
		return res, err
	}
	if in.Run == nil {
		return res, ErrNoRunner
	}
	timeout := in.ChunkTimeout
	if timeout <= 0 {
		timeout = DefaultChunkTimeout
	}
	off := 0
	for _, c := range Chunks(text, MaxChunk) {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		if err := t.Verify(ctx, in.Windows); err != nil {
			return res, err
		}
		if !Allowed(Tool, "-s", Settle, "--", c) {
			return res, ErrNotAllowed
		}
		cctx, cancel := context.WithTimeout(ctx, timeout)
		err := in.Run(cctx, Tool, "-s", Settle, "--", c)
		cancel()
		n := utf8.RuneCountInString(c)
		if err != nil {
			res.Unsure = n
			return res, fmt.Errorf("%w: %w", ErrTool, err)
		}
		off += len(c)
		res.Landed += n
		res.Rest = text[off:]
	}
	return res, nil
}

// Chunks splits text into pieces of at most max runes whose concatenation is
// text. A piece never ends inside a rune and, where the cluster fits, never
// inside a user-perceived character: combining marks, variation selectors,
// emoji modifiers, tag sequences, zero-width-joiner sequences and flag pairs
// stay with the character before them. A newline always starts a new piece,
// so the target is checked right before every Return key.
func Chunks(text string, max int) []string {
	if max < 1 {
		max = 1
	}
	var out []string
	start, n := 0, 0
	for i := 0; i < len(text); {
		end, runes := cluster(text, i)
		newline := text[i] == '\n' && n > 0 && text[i-1] != '\n'
		if newline || (n > 0 && n+runes > max) {
			out = append(out, text[start:i])
			start, n = i, 0
		}
		if runes > max {
			// A cluster longer than a chunk is split on rune boundaries.
			// The chunk is empty here: the check above flushed it.
			k := 0
			for j := i; j < end; {
				if k == max {
					out = append(out, text[start:j])
					start, k = j, 0
				}
				_, size := utf8.DecodeRuneInString(text[j:])
				j += size
				k++
			}
			n, i = k, end
			continue
		}
		n += runes
		i = end
	}
	if start < len(text) {
		out = append(out, text[start:])
	}
	return out
}

const zwj = '\u200d'

// cluster returns the end of the cluster starting at byte i and its length
// in runes.
func cluster(s string, i int) (int, int) {
	r, size := utf8.DecodeRuneInString(s[i:])
	end, n := i+size, 1
	prev := r
	flags := 0
	if isRegional(r) {
		flags = 1
	}
	for end < len(s) {
		next, size := utf8.DecodeRuneInString(s[end:])
		switch {
		case next == '\n':
			return end, n
		case prev == zwj, extends(next):
		case isRegional(next) && flags == 1:
			flags = 2
		default:
			return end, n
		}
		end += size
		n++
		prev = next
	}
	return end, n
}

func extends(r rune) bool {
	switch {
	case r == zwj,
		unicode.In(r, unicode.Mn, unicode.Me, unicode.Mc),
		r >= 0xfe00 && r <= 0xfe0f,
		r >= 0xe0100 && r <= 0xe01ef,
		r >= 0x1f3fb && r <= 0x1f3ff,
		r >= 0xe0020 && r <= 0xe007f:
		return true
	}
	return false
}

func isRegional(r rune) bool { return r >= 0x1f1e6 && r <= 0x1f1ff }
