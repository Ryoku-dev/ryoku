// Package input is the one place Hyakki drives a pointer or a keyboard on a
// Wayland display: a virtual pointer spoken over the Wayland wire, and the
// wtype keyboard tool, started without a shell. No other package names an
// input tool; the agent-session screen, the separate nested desktop and the
// live selected-app path all come here.
//
// Every process starts from an exact argv allowlist (Allowed) with an
// environment that names one display and nothing else (Env): no D-Bus
// address, no compositor socket, no X display. Text goes in chunks of at
// most MaxChunk characters and holds no control character, so typing never
// presses Return, Tab or Escape; a key is a separate, named chord. There is
// no clipboard path.
//
// How wtype reads its arguments (wtype 0.4, main.c parse_args): "-s MS" is a
// sleep run in order, so placed first it waits after the keymap is sent and
// before the first key; everything after a "--" argument is text, so text
// that starts with a dash is never read as an option; several text
// arguments are joined with a space, so a chunk is always exactly one
// argument; "-M", "-k" and "-m" press a modifier, type a key and release a
// modifier, each naming an XKB keysym. wtype decodes its text with the C
// library's locale, so Exec runs it under a UTF-8 locale.
package input

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Tool is the keyboard command this package starts, and the only one.
const Tool = "wtype"

// MaxChunk is the most characters (runes) one Tool call types.
const MaxChunk = 40

// Settle is how long, in milliseconds, each Tool call waits after giving the
// compositor its keymap and before the first key. Hyprland applies a new
// virtual keyboard's keymap a moment later and drops a key sent at once.
const Settle = "50"

// Runner starts one command and waits for it. The real one is Exec; tests
// pass a fake, so nothing is ever typed.
type Runner func(ctx context.Context, name string, args ...string) error

var (
	// ErrNotAllowed is returned for any argv outside the allowlist. Its text
	// never includes the arguments, which may hold typed words.
	ErrNotAllowed = errors.New("input: command not in the input allowlist")
	// ErrText is returned before anything is typed when the text holds
	// invalid UTF-8 or a character that would press a key rather than type.
	ErrText = errors.New("input: text holds characters that would be typed as keys")
	// ErrEnv is returned for an environment that names more than one display
	// or anything besides it.
	ErrEnv = errors.New("input: the environment must name one display and nothing else")
	// ErrKey is returned for a chord outside the key vocabulary.
	ErrKey = errors.New("input: not a key")
	// ErrTool wraps a Tool failure.
	ErrTool = errors.New("input: the keyboard tool failed")
	// ErrNoRunner is returned when a Keyboard has no Runner.
	ErrNoRunner = errors.New("input: no runner")
	// ErrNoCheck is returned when checked typing has no check.
	ErrNoCheck = errors.New("input: checked typing needs a check")
)

// Check returns ErrText unless text is valid UTF-8 with no control
// character and no line or paragraph separator: newlines and tabs are keys,
// pressed only as named chords.
func Check(text string) error {
	if !utf8.ValidString(text) {
		return ErrText
	}
	for _, r := range text {
		if unicode.IsControl(r) || unicode.In(r, unicode.Zl, unicode.Zp) {
			return ErrText
		}
	}
	return nil
}

// Env is the whole environment of an input process for the display named
// display in runtimeDir: PATH to find the tool, the runtime dir and display,
// and a UTF-8 locale. runtimeDir must be absolute and clean, and display a
// bare socket name inside it.
func Env(runtimeDir, display string) ([]string, error) {
	if !dirOK(runtimeDir) || !displayOK(display) {
		return nil, ErrEnv
	}
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"XDG_RUNTIME_DIR=" + runtimeDir,
		"WAYLAND_DISPLAY=" + display,
		"LC_ALL=C.UTF-8",
	}, nil
}

func dirOK(dir string) bool {
	return dir != "" && filepath.IsAbs(dir) && filepath.Clean(dir) == dir && !strings.ContainsAny(dir, "\x00\n")
}

func displayOK(display string) bool {
	if display == "" || display == "." || display == ".." || len(display) > 64 {
		return false
	}
	for _, r := range display {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return true
}

// envOK reports whether env is exactly what Env builds: each of its four
// names once, nothing else.
func envOK(env []string) bool {
	seen := map[string]bool{}
	for _, kv := range env {
		name, value, ok := strings.Cut(kv, "=")
		if !ok || seen[name] {
			return false
		}
		seen[name] = true
		switch name {
		case "PATH":
		case "XDG_RUNTIME_DIR":
			if !dirOK(value) {
				return false
			}
		case "WAYLAND_DISPLAY":
			if !displayOK(value) {
				return false
			}
		case "LC_ALL":
			if value != "C.UTF-8" {
				return false
			}
		default:
			return false
		}
	}
	return len(seen) == 4
}

// Allowed reports whether name and args are one of the two shapes this
// package runs, exactly:
//
//	wtype -s Settle -- CHUNK
//	wtype -s Settle [-M MOD]... -k KEY [-m MOD]...
//
// CHUNK is one non-empty argument of at most MaxChunk characters that passes
// Check. MODs are distinct modifiers, released in the reverse order they
// were pressed, and KEY is a keysym name.
func Allowed(name string, args ...string) bool {
	if name != Tool || len(args) < 4 || args[0] != "-s" || args[1] != Settle {
		return false
	}
	rest := args[2:]
	if rest[0] == "--" {
		if len(rest) != 2 {
			return false
		}
		c := rest[1]
		return c != "" && utf8.RuneCountInString(c) <= MaxChunk && Check(c) == nil
	}
	var down []string
	for len(rest) >= 2 && rest[0] == "-M" {
		if !modifierOK(rest[1]) || contains(down, rest[1]) {
			return false
		}
		down = append(down, rest[1])
		rest = rest[2:]
	}
	if len(rest) < 2 || rest[0] != "-k" || !keysymOK(rest[1]) {
		return false
	}
	rest = rest[2:]
	if len(rest) != 2*len(down) {
		return false
	}
	for i := range down {
		if rest[2*i] != "-m" || rest[2*i+1] != down[len(down)-1-i] {
			return false
		}
	}
	return true
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func modifierOK(m string) bool {
	switch m {
	case "ctrl", "shift", "alt", "logo", "altgr":
		return true
	}
	return false
}

// keysymOK accepts an XKB keysym name: a letter or digit, then letters,
// digits and underscores, never a dash, so a key can never read as an
// option.
func keysymOK(k string) bool {
	if k == "" || len(k) > 32 {
		return false
	}
	for i, r := range k {
		alnum := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
		if !alnum && (i == 0 || r != '_') {
			return false
		}
	}
	return true
}

// Exec is the real Runner for the display env names. It refuses an env Env
// would not build and any argv Allowed refuses, before starting anything;
// it runs the tool directly with no shell, no stdin and only env.
func Exec(env []string) Runner {
	env = append([]string(nil), env...)
	return func(ctx context.Context, name string, args ...string) error {
		if !envOK(env) {
			return ErrEnv
		}
		if !Allowed(name, args...) {
			return ErrNotAllowed
		}
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Env = env
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

// KeyArgs turns a chord such as ctrl+shift+t, alt+F4 or Return into the
// tool's arguments after the settle: each modifier pressed, the key typed,
// the modifiers let go in reverse. Names are XKB keysyms, with the
// spellings people use mapped onto them. A chord whose argv Allowed would
// refuse is ErrKey.
func KeyArgs(chord string) ([]string, error) {
	parts := strings.Split(strings.TrimSpace(chord), "+")
	if strings.TrimSpace(parts[len(parts)-1]) == "" {
		return nil, fmt.Errorf("%w: no key in %q", ErrKey, chord)
	}
	var modifiers []string
	for _, part := range parts[:len(parts)-1] {
		modifier, ok := keyModifiers[strings.ToLower(strings.TrimSpace(part))]
		if !ok {
			return nil, fmt.Errorf("%w: %q is not a modifier", ErrKey, part)
		}
		if contains(modifiers, modifier) {
			return nil, fmt.Errorf("%w: %q is held twice", ErrKey, part)
		}
		modifiers = append(modifiers, modifier)
	}
	key := strings.TrimSpace(parts[len(parts)-1])
	if name, ok := keyNames[strings.ToLower(key)]; ok {
		key = name
	}
	// Ctrl+L is how people write ctrl+l; the capital keysym would add a shift
	if len(modifiers) > 0 && len(key) == 1 {
		key = strings.ToLower(key)
	}
	if !keysymOK(key) {
		return nil, fmt.Errorf("%w: %q is not a key name", ErrKey, key)
	}
	var args []string
	for _, modifier := range modifiers {
		args = append(args, "-M", modifier)
	}
	args = append(args, "-k", key)
	for i := len(modifiers) - 1; i >= 0; i-- {
		args = append(args, "-m", modifiers[i])
	}
	return args, nil
}

var keyModifiers = map[string]string{
	"ctrl": "ctrl", "control": "ctrl", "shift": "shift", "alt": "alt", "option": "alt",
	"super": "logo", "meta": "logo", "win": "logo", "cmd": "logo", "logo": "logo", "altgr": "altgr",
}

var keyNames = map[string]string{
	"enter": "Return", "return": "Return", "tab": "Tab", "esc": "Escape", "escape": "Escape",
	"backspace": "BackSpace", "delete": "Delete", "del": "Delete", "space": "space",
	"up": "Up", "down": "Down", "left": "Left", "right": "Right",
	"arrowup": "Up", "arrowdown": "Down", "arrowleft": "Left", "arrowright": "Right",
	"home": "Home", "end": "End", "pageup": "Page_Up", "pagedown": "Page_Down",
	"page_up": "Page_Up", "page_down": "Page_Down", "insert": "Insert", "menu": "Menu",
	"f1": "F1", "f2": "F2", "f3": "F3", "f4": "F4", "f5": "F5", "f6": "F6",
	"f7": "F7", "f8": "F8", "f9": "F9", "f10": "F10", "f11": "F11", "f12": "F12",
}
