// Package apply types a checked rewrite back over the selection it came
// from, and undoes it, with no clipboard anywhere.
//
// Typing goes through dictation's insert, so the window that had the
// selection is checked before every chunk and the untyped rest comes back
// to be copied. The selection is read again just before the first key: if
// it is no longer exactly the text that was rewritten, nothing is typed.
// Only a single paragraph is typed, because a line break presses Return and
// can send a message.
//
// Undo presses Ctrl+Z through hands/input, one press at a time, and reads
// the window's words back after each. It stops as soon as the original is
// back, presses again only when the read shows the typing coming undone,
// and never presses more than once per typed chunk plus one. Any read it
// cannot judge ends it unverified, and the caller keeps the original for
// the person to copy.
package apply

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"ryoku-rashin/internal/dictate/insert"
	"ryoku-rashin/internal/dictate/target"
	"ryoku-rashin/internal/hands/input"
	"ryoku-rashin/internal/oni/hands/apprules"
	"ryoku-rashin/internal/oni/window"
	"ryoku-rashin/internal/sensitive"
)

// UndoChord is the one key undo presses.
const UndoChord = "ctrl+z"

// DefaultWait is how long undo lets the window redraw before reading it.
const DefaultWait = 250 * time.Millisecond

// Why a window is never typed into.
const (
	BarredSelf     = "self"
	BarredPrivate  = "private"
	BarredTerminal = "terminal"
)

var (
	// ErrMultiline: the rewrite holds a line break, so it is not typed.
	ErrMultiline = errors.New("apply: the rewrite holds a line break")
	// ErrSelectionChanged: the selection is gone or no longer the text that
	// was rewritten, so nothing was typed.
	ErrSelectionChanged = errors.New("apply: the selection changed")
	// ErrUnverified: undo could not read back that the original returned.
	ErrUnverified = errors.New("apply: undo could not be verified")
	// ErrBarred: the window is one nothing is typed into.
	ErrBarred = errors.New("apply: the window is not one to type into")
	// ErrIncomplete: an Applier lacks a runner or a reader.
	ErrIncomplete = errors.New("apply: missing a runner or a reader")
)

// terminals are app ids of terminal emulators. A terminal has no undo for
// typed text and may run what is typed, so a rewrite is never typed there.
var terminals = []string{
	"kitty", "foot", "footclient", "alacritty", "wezterm", "org.wezfurlong.wezterm",
	"ghostty", "com.mitchellh.ghostty", "konsole", "org.kde.konsole",
	"gnome-console", "org.gnome.console", "kgx", "gnome-terminal", "gnome-terminal-server",
	"org.gnome.terminal", "ptyxis", "org.gnome.ptyxis", "xterm", "uxterm",
	"rxvt", "urxvt", "st", "sakura", "tilix", "com.gexperts.tilix",
	"terminator", "terminology", "xfce4-terminal", "org.xfce.terminal",
	"qterminal", "org.qterminal.qterminal", "lxterminal", "rio", "com.rioterm.rio",
	"blackbox", "com.raggesilver.blackbox", "contour", "warp", "dev.warp.warp",
}

// Terminal reports whether app is a terminal emulator.
func Terminal(app string) bool {
	app = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(app)), ".desktop")
	for _, t := range terminals {
		if app == t {
			return true
		}
	}
	return false
}

// Barred names why w is never typed into, or "": Hyakki's own surfaces, a
// private window (a password manager, a prompt for a secret, a lock screen,
// or a title naming a private field) and terminals.
func Barred(w window.Window) string {
	switch {
	case apprules.Self(w.App):
		return BarredSelf
	case apprules.PrivateApp(w.App) || sensitive.Place(w.Title):
		return BarredPrivate
	case Terminal(w.App):
		return BarredTerminal
	}
	return ""
}

// Look reads the words shown in the target window now, on this computer.
type Look func(ctx context.Context) ([]string, error)

// Applier types and undoes in one target window.
type Applier struct {
	// Windows reads the desktop's windows.
	Windows target.Source
	// Type runs wtype for typing (dictation's insert allowlist).
	Type insert.Runner
	// Keys runs wtype for the undo chord (hands/input's allowlist).
	Keys input.Runner
	// Select reads the selection again just before typing.
	Select func(ctx context.Context) (string, error)
	// Look reads the target's words back after each undo press.
	Look Look
	// Wait is the pause before each read back; 0 is DefaultWait, a
	// negative value none.
	Wait time.Duration
}

// guarded is a window source that fails while the target window is one
// Barred names, so every check before a chunk or a press also checks that
// the window did not turn private (a retitle to a password field).
type guarded struct {
	src target.Source
	id  window.ID
}

var errBarredNow = errors.New("the window is now one Hyakki does not type into")

func (g guarded) Snapshot(ctx context.Context) (window.Snapshot, error) {
	s, err := g.src.Snapshot(ctx)
	if err != nil {
		return s, err
	}
	for _, w := range s.Windows {
		if w.ID == g.id && Barred(w) != "" {
			return window.Snapshot{}, errBarredNow
		}
	}
	return s, nil
}

// Applied is what typing got done. Chunks counts the wtype calls that ran,
// the failed one included, which bounds the undo presses.
type Applied struct {
	Landed int
	Rest   string
	Unsure int
	Chunks int
}

// Apply types rewritten over the selection in t when the selection is still
// exactly original. It returns ErrMultiline for a rewrite with a line break,
// ErrSelectionChanged when the selection moved on, and otherwise insert's
// errors: a *target.Refusal, insert.ErrText, insert.ErrTool.
func (a Applier) Apply(ctx context.Context, t target.Target, original, rewritten string) (Applied, error) {
	res := Applied{Rest: rewritten}
	if strings.ContainsAny(rewritten, "\n\r\u2028\u2029") {
		return res, ErrMultiline
	}
	if a.Type == nil || a.Select == nil || a.Windows == nil {
		return res, ErrIncomplete
	}
	src := guarded{src: a.Windows, id: t.Window}
	if err := t.Verify(ctx, src); err != nil {
		return res, err
	}
	now, err := a.Select(ctx)
	if err != nil || now != original {
		return res, ErrSelectionChanged
	}
	got, err := insert.Inserter{Run: a.Type, Windows: src}.Insert(ctx, t, rewritten)
	res.Landed, res.Rest, res.Unsure = got.Landed, got.Rest, got.Unsure
	typed := rewritten[:len(rewritten)-len(got.Rest)]
	if typed != "" {
		res.Chunks = len(insert.Chunks(typed, insert.MaxChunk))
	}
	if got.Unsure > 0 {
		res.Chunks++
	}
	return res, err
}

// Record is what undo needs: the window, the two texts and how many chunks
// were typed.
type Record struct {
	Target    target.Target
	Original  string
	Rewritten string
	Chunks    int
}

// Undone is what undo did: Presses of Ctrl+Z, and Verified when the read
// back showed the original back and the rewrite gone.
type Undone struct {
	Presses  int
	Verified bool
}

// Undo presses Ctrl+Z in r's window until a read back shows the original.
// It checks the window before each press, presses a second or later time
// only while the read shows fewer of the rewrite's own words than the read
// before, and never more than r.Chunks+1 times. It returns ErrUnverified
// when it stopped without seeing the original back, or the window's
// refusal.
//
// The selection was deleted before the first chunk was typed, so in a
// linear undo history the original comes back only once all the typing is
// undone. When the read before the first press did not show the original,
// its return is the proof, and words of the rewrite still shown are the
// window's own text around it (no more of them than before). When it did
// (its words are elsewhere in the window too), none of the rewrite's own
// words may be left.
func (a Applier) Undo(ctx context.Context, r Record) (Undone, error) {
	var out Undone
	if a.Keys == nil || a.Look == nil || a.Windows == nil {
		return out, ErrIncomplete
	}
	if r.Chunks < 1 {
		return out, ErrUnverified
	}
	mark := marks(r.Original, r.Rewritten)
	src := guarded{src: a.Windows, id: r.Target.Window}
	keys := input.Keyboard{Run: a.Keys}
	prev, first, seenBefore := -1, -1, true
	if words, err := a.Look(ctx); err == nil {
		seenBefore, prev = mark.read(words)
		first = prev
	}
	for out.Presses < r.Chunks+1 {
		if err := r.Target.Verify(ctx, src); err != nil {
			return out, err
		}
		if err := keys.Chord(ctx, UndoChord); err != nil {
			return out, err
		}
		out.Presses++
		if err := a.pause(ctx); err != nil {
			return out, err
		}
		words, err := a.Look(ctx)
		if err != nil {
			return out, ErrUnverified
		}
		back, left := mark.read(words)
		if back {
			if left == 0 || !seenBefore && left <= first {
				out.Verified = true
				return out, nil
			}
			return out, ErrUnverified
		}
		// press again only on visible progress, never on a guess
		if prev < 0 || left >= prev {
			return out, ErrUnverified
		}
		prev = left
	}
	return out, ErrUnverified
}

func (a Applier) pause(ctx context.Context) error {
	d := a.Wait
	if d == 0 {
		d = DefaultWait
	}
	if d < 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// marker holds the words that tell the two texts apart: gone are the
// original's words the rewrite dropped, added the rewrite's own.
type marker struct {
	gone, added []string
	all         []string
}

func marks(original, rewritten string) marker {
	o, w := tokens(original), tokens(rewritten)
	in := func(list []string, s string) bool {
		for _, x := range list {
			if x == s {
				return true
			}
		}
		return false
	}
	var m marker
	m.all = o
	for _, t := range o {
		if !in(w, t) {
			m.gone = append(m.gone, t)
		}
	}
	for _, t := range w {
		if !in(o, t) {
			m.added = append(m.added, t)
		}
	}
	return m
}

// read judges the words read: back when at least four in five of the
// original's own words (all its words, when the rewrite dropped none) are
// shown, and left, how many of the rewrite's own words still are.
func (m marker) read(words []string) (back bool, left int) {
	seen := map[string]bool{}
	for _, w := range words {
		for _, t := range tokens(w) {
			seen[t] = true
		}
	}
	want := m.gone
	if len(want) == 0 {
		want = m.all
	}
	found := 0
	for _, t := range want {
		if seen[t] {
			found++
		}
	}
	for _, t := range m.added {
		if seen[t] {
			left++
		}
	}
	return len(want) > 0 && found*5 >= len(want)*4, left
}

// tokens are the distinct words of s, folded to lower case letters and
// digits, two characters or longer.
func tokens(s string) []string {
	var out []string
	have := map[string]bool{}
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		f = strings.ToLower(f)
		if utf8.RuneCountInString(f) < 2 || have[f] {
			continue
		}
		have[f] = true
		out = append(out, f)
	}
	return out
}
