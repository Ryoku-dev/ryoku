package apply

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"ryoku-rashin/internal/dictate/insert"
	"ryoku-rashin/internal/dictate/target"
	"ryoku-rashin/internal/hands/input"
	"ryoku-rashin/internal/oni/window"
)

// editor is a fake text field with a selection and an undo history. A
// grouped editor merges all typing into one undo step with the deletion of
// the selection; otherwise the deletion and each typed chunk are steps.
type editor struct {
	mu                     sync.Mutex
	before, middle, after  string
	selected               bool
	grouped                bool
	history                []string
	typed                  int
	calls                  [][]string
	deaf                   bool // ignores Ctrl+Z
	onChunk                func(n int)
	window                 window.Window
	snapshots              int
	focusLostAfterSnapshot int
}

func newEditor(before, sel, after string, grouped bool) *editor {
	return &editor{before: before, middle: sel, after: after, selected: true, grouped: grouped,
		window: window.Window{ID: "0x1", App: "org.gnome.TextEditor", Title: "notes.txt", Output: "eDP-1", Visible: true, Focused: true, PID: 42}}
}

func (e *editor) Snapshot(context.Context) (window.Snapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.snapshots++
	w := e.window
	if e.focusLostAfterSnapshot > 0 && e.snapshots > e.focusLostAfterSnapshot {
		w.Focused = false
	}
	return window.Snapshot{Windows: []window.Window{w, {ID: "0x2", App: "firefox", Output: "eDP-1", Visible: true}}}, nil
}

func (e *editor) text() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.before + e.middle + e.after
}

func (e *editor) typeRun(_ context.Context, name string, args ...string) error {
	e.mu.Lock()
	e.calls = append(e.calls, append([]string{name}, args...))
	if !insert.Allowed(name, args...) {
		e.mu.Unlock()
		return errors.New("not allowed")
	}
	chunk := args[len(args)-1]
	if e.selected {
		e.history = append(e.history, e.middle)
		e.middle, e.selected = "", false
	}
	if !e.grouped {
		e.history = append(e.history, e.middle)
	}
	e.middle += chunk
	e.typed++
	n, hook := e.typed, e.onChunk
	e.mu.Unlock()
	if hook != nil {
		hook(n)
	}
	return nil
}

func (e *editor) keyRun(_ context.Context, name string, args ...string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls = append(e.calls, append([]string{name}, args...))
	if !input.Allowed(name, args...) || !slices.Equal(args, []string{"-s", input.Settle, "-M", "ctrl", "-k", "z", "-m", "ctrl"}) {
		return errors.New("not the undo chord")
	}
	if e.deaf || len(e.history) == 0 {
		return nil
	}
	e.middle = e.history[len(e.history)-1]
	e.history = e.history[:len(e.history)-1]
	return nil
}

func (e *editor) look(context.Context) ([]string, error) { return strings.Fields(e.text()), nil }

func (e *editor) selection(context.Context) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.selected {
		return "", errors.New("no selection")
	}
	return e.middle, nil
}

func (e *editor) applier() Applier {
	return Applier{Windows: e, Type: e.typeRun, Keys: e.keyRun, Select: e.selection, Look: e.look, Wait: -1}
}

func (e *editor) noClipboard(t *testing.T) {
	t.Helper()
	for _, c := range e.calls {
		if c[0] != "wtype" {
			t.Fatalf("ran %q", c)
		}
		for _, a := range c {
			if strings.Contains(a, "wl-copy") || strings.Contains(a, "wl-paste") || a == "v" || a == "c" {
				t.Fatalf("a clipboard key or tool ran: %q", c)
			}
		}
	}
}

func (e *editor) presses() int {
	n := 0
	for _, c := range e.calls {
		if slices.Contains(c, "-k") {
			n++
		}
	}
	return n
}

func capture(t *testing.T, e *editor) target.Target {
	t.Helper()
	tg, err := target.Capture(context.Background(), e)
	if err != nil {
		t.Fatal(err)
	}
	return tg
}

const (
	original  = "can u send me the report by tomorow"
	rewritten = "Could you please send me the report by tomorrow? Thank you so much for your help with this."
)

func TestApplyAndUndoRestoreTheOriginal(t *testing.T) {
	for _, grouped := range []bool{true, false} {
		for _, text := range []string{"Could you send the report tomorrow?", rewritten} {
			e := newEditor("Hi Sam, ", original, " Thanks.", grouped)
			a := e.applier()
			tg := capture(t, e)
			got, err := a.Apply(context.Background(), tg, original, text)
			if err != nil || got.Rest != "" || got.Landed != len([]rune(text)) {
				t.Fatalf("apply: %+v %v", got, err)
			}
			if e.text() != "Hi Sam, "+text+" Thanks." {
				t.Fatalf("field %q", e.text())
			}
			chunks := len(insert.Chunks(text, insert.MaxChunk))
			if got.Chunks != chunks {
				t.Fatalf("chunks %d, want %d", got.Chunks, chunks)
			}
			undone, err := a.Undo(context.Background(), Record{Target: tg, Original: original, Rewritten: text, Chunks: got.Chunks})
			if err != nil || !undone.Verified {
				t.Fatalf("grouped=%v undo: %+v %v (field %q)", grouped, undone, err, e.text())
			}
			if e.text() != "Hi Sam, "+original+" Thanks." {
				t.Fatalf("field after undo %q", e.text())
			}
			if e.presses() > got.Chunks+1 || grouped && e.presses() != 1 {
				t.Fatalf("grouped=%v: %d presses for %d chunks", grouped, e.presses(), got.Chunks)
			}
			e.noClipboard(t)
		}
	}
}

func TestApplyTypesOnlyOneParagraph(t *testing.T) {
	e := newEditor("", original, "", true)
	got, err := e.applier().Apply(context.Background(), capture(t, e), original, "Line one.\nLine two.")
	if !errors.Is(err, ErrMultiline) || got.Landed != 0 || got.Rest != "Line one.\nLine two." || len(e.calls) != 0 {
		t.Fatalf("%+v %v, calls %q", got, err, e.calls)
	}
}

func TestApplyTypesNothingWhenTheSelectionMoved(t *testing.T) {
	e := newEditor("", "something else now", "", true)
	got, err := e.applier().Apply(context.Background(), capture(t, e), original, rewritten)
	if !errors.Is(err, ErrSelectionChanged) || got.Landed != 0 || len(e.calls) != 0 {
		t.Fatalf("%+v %v, calls %q", got, err, e.calls)
	}
	e.selected = false
	if _, err := e.applier().Apply(context.Background(), capture(t, e), original, rewritten); !errors.Is(err, ErrSelectionChanged) || len(e.calls) != 0 {
		t.Fatalf("no selection: %v, calls %q", err, e.calls)
	}
}

func TestApplyStopsWhenTheWindowChangesBetweenChunks(t *testing.T) {
	for name, change := range map[string]func(e *editor){
		"lost focus":           func(e *editor) { e.window.Focused = false },
		"retitled to a secret": func(e *editor) { e.window.Title = "Enter password" },
		"closed":               func(e *editor) { e.window.ID = "0x9" },
	} {
		e := newEditor("", original, "", false)
		tg := capture(t, e)
		e.onChunk = func(n int) {
			if n == 1 {
				e.mu.Lock()
				change(e)
				e.mu.Unlock()
			}
		}
		got, err := e.applier().Apply(context.Background(), tg, original, rewritten)
		var refusal *target.Refusal
		if !errors.As(err, &refusal) || e.typed != 1 || got.Chunks != 1 {
			t.Fatalf("%s: %+v %v, typed %d", name, got, err, e.typed)
		}
		first := insert.Chunks(rewritten, insert.MaxChunk)[0]
		if got.Rest != strings.TrimPrefix(rewritten, first) || got.Landed != len([]rune(first)) {
			t.Fatalf("%s: rest %q landed %d", name, got.Rest, got.Landed)
		}
	}
}

func TestUndoNeverPressesBlind(t *testing.T) {
	t.Run("a window that ignores undo gets one press", func(t *testing.T) {
		e := newEditor("", original, "", false)
		a := e.applier()
		tg := capture(t, e)
		got, _ := a.Apply(context.Background(), tg, original, rewritten)
		e.deaf = true
		undone, err := a.Undo(context.Background(), Record{Target: tg, Original: original, Rewritten: rewritten, Chunks: got.Chunks})
		if !errors.Is(err, ErrUnverified) || undone.Presses != 1 || undone.Verified {
			t.Fatalf("%+v %v", undone, err)
		}
	})
	t.Run("an unreadable window gets one press", func(t *testing.T) {
		e := newEditor("", original, "", false)
		a := e.applier()
		tg := capture(t, e)
		got, _ := a.Apply(context.Background(), tg, original, rewritten)
		a.Look = func(context.Context) ([]string, error) { return nil, errors.New("no text reader") }
		undone, err := a.Undo(context.Background(), Record{Target: tg, Original: original, Rewritten: rewritten, Chunks: got.Chunks})
		if !errors.Is(err, ErrUnverified) || undone.Presses != 1 {
			t.Fatalf("%+v %v", undone, err)
		}
	})
	t.Run("the cap holds even when every read shows progress", func(t *testing.T) {
		e := newEditor("", original, "", false)
		a := e.applier()
		tg := capture(t, e)
		got, _ := a.Apply(context.Background(), tg, original, rewritten)
		// the record claims fewer chunks than were typed
		undone, err := a.Undo(context.Background(), Record{Target: tg, Original: original, Rewritten: rewritten, Chunks: 1})
		if !errors.Is(err, ErrUnverified) || undone.Presses != 2 || got.Chunks < 3 {
			t.Fatalf("%+v %v (chunks %d)", undone, err, got.Chunks)
		}
	})
	t.Run("a window that lost focus gets no press", func(t *testing.T) {
		e := newEditor("", original, "", true)
		a := e.applier()
		tg := capture(t, e)
		got, _ := a.Apply(context.Background(), tg, original, rewritten)
		e.window.Focused = false
		undone, err := a.Undo(context.Background(), Record{Target: tg, Original: original, Rewritten: rewritten, Chunks: got.Chunks})
		var refusal *target.Refusal
		if !errors.As(err, &refusal) || undone.Presses != 0 || e.presses() != 0 {
			t.Fatalf("%+v %v", undone, err)
		}
	})
	t.Run("the original already back stops further presses", func(t *testing.T) {
		e := newEditor("", original, "", true)
		a := e.applier()
		tg := capture(t, e)
		got, _ := a.Apply(context.Background(), tg, original, rewritten)
		if _, err := a.Undo(context.Background(), Record{Target: tg, Original: original, Rewritten: rewritten, Chunks: got.Chunks}); err != nil {
			t.Fatal(err)
		}
		if e.presses() != 1 {
			t.Fatalf("%d presses", e.presses())
		}
	})
}

func TestBarredWindows(t *testing.T) {
	for _, tc := range []struct {
		w    window.Window
		want string
	}{
		{window.Window{App: "dev.ryoku.Hyakki"}, BarredSelf},
		{window.Window{App: "dev.ryoku.Hyakki.Oni"}, BarredSelf},
		{window.Window{App: "org.keepassxc.KeePassXC"}, BarredPrivate},
		{window.Window{App: "firefox", Title: "Sign in - Enter your password"}, BarredPrivate},
		{window.Window{App: "foot"}, BarredTerminal},
		{window.Window{App: "com.mitchellh.ghostty"}, BarredTerminal},
		{window.Window{App: "kitty.desktop"}, BarredTerminal},
		{window.Window{App: "org.gnome.TextEditor", Title: "notes.txt"}, ""},
		{window.Window{App: "firefox", Title: "Inbox"}, ""},
	} {
		if got := Barred(tc.w); got != tc.want {
			t.Errorf("%+v: %q, want %q", tc.w, got, tc.want)
		}
	}
}

func TestUndoJudgesTheWordsAroundTheSelection(t *testing.T) {
	t.Run("words of the rewrite already in the window", func(t *testing.T) {
		for _, grouped := range []bool{true, false} {
			e := newEditor("Please help: ", original, " Thank you.", grouped)
			a := e.applier()
			tg := capture(t, e)
			got, _ := a.Apply(context.Background(), tg, original, rewritten)
			undone, err := a.Undo(context.Background(), Record{Target: tg, Original: original, Rewritten: rewritten, Chunks: got.Chunks})
			if err != nil || !undone.Verified || e.text() != "Please help: "+original+" Thank you." {
				t.Fatalf("grouped=%v: %+v %v, field %q", grouped, undone, err, e.text())
			}
		}
	})
	t.Run("the original's words already elsewhere", func(t *testing.T) {
		// the window shows the original's words before any press, so their
		// return proves nothing; a press that leaves the rewrite's words
		// up is not a verified undo
		e := newEditor("Earlier: "+original+". Now: ", original, "", false)
		a := e.applier()
		tg := capture(t, e)
		got, _ := a.Apply(context.Background(), tg, original, rewritten)
		undone, err := a.Undo(context.Background(), Record{Target: tg, Original: original, Rewritten: rewritten, Chunks: got.Chunks})
		if !errors.Is(err, ErrUnverified) || undone.Verified || undone.Presses != 1 {
			t.Fatalf("%+v %v", undone, err)
		}
	})
}
