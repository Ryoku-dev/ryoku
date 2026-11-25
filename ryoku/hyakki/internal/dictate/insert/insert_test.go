package insert

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"ryoku-rashin/internal/dictate/target"
	"ryoku-rashin/internal/oni/window"
)

// windows serves one snapshot per read: read i gets at(i), counting from 0.
type windows struct {
	at    func(i int) window.Snapshot
	reads int
}

func (w *windows) Snapshot(context.Context) (window.Snapshot, error) {
	s := w.at(w.reads)
	w.reads++
	return s, nil
}

func editorWin() window.Window {
	return window.Window{ID: "31", App: "org.gnome.TextEditor", Title: "notes.txt", Output: "eDP-1", Focused: true, Visible: true, PID: 4120}
}

func termWin() window.Window {
	return window.Window{ID: "32", App: "foot", Output: "eDP-1", Visible: true, PID: 5003, FocusOrder: 1}
}

func steady() window.Snapshot {
	return window.Snapshot{Outputs: []window.Output{{Name: "eDP-1"}, {Name: "DP-1"}}, Windows: []window.Window{termWin(), editorWin()}}
}

func changed(f func(e, o *window.Window)) window.Snapshot {
	e, o := editorWin(), termWin()
	f(&e, &o)
	s := steady()
	s.Windows = []window.Window{o, e}
	return s
}

// recorder is a fake Runner: it records every argv and can fail one call.
type recorder struct {
	calls  [][]string
	failAt int // 1-based call that fails; 0 never
}

func (r *recorder) run(_ context.Context, name string, args ...string) error {
	r.calls = append(r.calls, append([]string{name}, args...))
	if r.failAt == len(r.calls) {
		return errors.New("exit status 1")
	}
	return nil
}

func (r *recorder) typed() string {
	var b strings.Builder
	for _, c := range r.calls {
		b.WriteString(c[len(c)-1])
	}
	return b.String()
}

var clipboardTools = []string{"wl-copy", "wl-paste", "xclip", "xsel", "cliphist", "copyq"}

// noClipboard fails if any recorded call is anything but `wtype -- CHUNK`.
func (r *recorder) noClipboard(t *testing.T) {
	t.Helper()
	for _, c := range r.calls {
		if slices.Contains(clipboardTools, c[0]) || !Allowed(c[0], c[1:]...) {
			t.Fatalf("runner saw %q", c)
		}
		for _, a := range c[1 : len(c)-1] {
			if strings.Contains(a, "clip") {
				t.Fatalf("runner saw a clipboard argument %q", c)
			}
		}
	}
}

func capture(t *testing.T, src target.Source) target.Target {
	t.Helper()
	tg, err := target.Capture(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	return tg
}

const long = "Meet me at the north gate at seven, and bring the blue folder with the signed forms please."

func TestInsertTypesEveryChunkAfterVerifying(t *testing.T) {
	src := &windows{at: func(int) window.Snapshot { return steady() }}
	tg := capture(t, src)
	src.reads = 0
	rec := &recorder{}
	res, err := Inserter{Run: rec.run, Windows: src}.Insert(context.Background(), tg, long)
	if err != nil {
		t.Fatal(err)
	}
	if res.Landed != utf8.RuneCountInString(long) || res.Rest != "" || res.Unsure != 0 {
		t.Fatalf("result %+v", res)
	}
	if rec.typed() != long {
		t.Fatalf("typed %q", rec.typed())
	}
	if len(rec.calls) != 3 || src.reads != len(rec.calls) {
		t.Fatalf("%d wtype calls, %d verifies; want 3 and one verify per chunk", len(rec.calls), src.reads)
	}
	// every call waits Settle ms after sending its keymap: Hyprland drops a
	// key sent at once, which lost the first character of nearly every chunk
	for _, c := range rec.calls {
		if len(c) != 5 || c[0] != "wtype" || c[1] != "-s" || c[2] != Settle || c[3] != "--" || utf8.RuneCountInString(c[4]) > MaxChunk {
			t.Fatalf("argv %q", c)
		}
	}
	rec.noClipboard(t)
}

func TestTargetLostBeforeInsertTypesNothing(t *testing.T) {
	cases := []struct {
		name string
		now  window.Snapshot
		want target.Reason
	}{
		{"closed", changed(func(e, _ *window.Window) { e.ID = "99" }), target.ReasonClosed},
		{"focus lost", changed(func(e, o *window.Window) { e.Focused, o.Focused = false, true }), target.ReasonUnfocused},
		{"output moved", changed(func(e, _ *window.Window) { e.Output = "DP-1" }), target.ReasonMovedOutput},
		{"minimized", changed(func(e, _ *window.Window) { e.Minimized = true }), target.ReasonHidden},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := &windows{at: func(i int) window.Snapshot {
				if i == 0 {
					return steady()
				}
				return c.now
			}}
			tg := capture(t, src)
			rec := &recorder{}
			res, err := Inserter{Run: rec.run, Windows: src}.Insert(context.Background(), tg, long)
			if target.ReasonOf(err) != c.want {
				t.Fatalf("reason %q (%v), want %q", target.ReasonOf(err), err, c.want)
			}
			if len(rec.calls) != 0 {
				t.Fatalf("runner called after the target was lost: %q", rec.calls)
			}
			if res.Landed != 0 || res.Rest != long || res.Unsure != 0 {
				t.Fatalf("result %+v", res)
			}
			if msg := Message(err); msg == "" || strings.Contains(msg, "north gate") || strings.Contains(err.Error(), "north gate") {
				t.Fatalf("refusal copy %q / %v", msg, err)
			}
		})
	}
}

func TestMidInsertLossStopsOnAChunkBoundary(t *testing.T) {
	text := strings.Repeat("déjà vu ", 20) // 160 runes, 4 chunks
	for lostAt := 1; lostAt < 4; lostAt++ {
		src := &windows{at: func(i int) window.Snapshot {
			// Read 0 is the capture; read k verifies chunk k.
			if i <= lostAt {
				return steady()
			}
			return changed(func(e, o *window.Window) { e.Focused, o.Focused = false, true })
		}}
		tg := capture(t, src)
		rec := &recorder{}
		res, err := Inserter{Run: rec.run, Windows: src}.Insert(context.Background(), tg, text)
		if target.ReasonOf(err) != target.ReasonUnfocused {
			t.Fatalf("lost after %d chunks: %v", lostAt, err)
		}
		if len(rec.calls) != lostAt {
			t.Fatalf("lost after %d chunks but wtype ran %d times", lostAt, len(rec.calls))
		}
		typed := rec.typed()
		if res.Landed != utf8.RuneCountInString(typed) || res.Landed != lostAt*MaxChunk {
			t.Fatalf("landed %d, typed %d runes", res.Landed, utf8.RuneCountInString(typed))
		}
		if typed+res.Rest != text {
			t.Fatalf("typed %q + rest %q is not the text", typed, res.Rest)
		}
		rec.noClipboard(t)
	}
}

func TestWtypeFailureStopsAndMarksTheChunkUnsure(t *testing.T) {
	src := &windows{at: func(int) window.Snapshot { return steady() }}
	tg := capture(t, src)
	rec := &recorder{failAt: 2}
	res, err := Inserter{Run: rec.run, Windows: src}.Insert(context.Background(), tg, long)
	if !errors.Is(err, ErrTool) || target.ReasonOf(err) != target.ReasonUnavailable {
		t.Fatalf("err %v", err)
	}
	if len(rec.calls) != 2 {
		t.Fatalf("typing continued after a failure: %d calls", len(rec.calls))
	}
	first := rec.calls[0][len(rec.calls[0])-1]
	if res.Landed != utf8.RuneCountInString(first) || res.Rest != strings.TrimPrefix(long, first) || res.Unsure != utf8.RuneCountInString(rec.calls[1][len(rec.calls[1])-1]) {
		t.Fatalf("result %+v", res)
	}
}

func TestCancelStopsBetweenChunks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	src := &windows{at: func(int) window.Snapshot { return steady() }}
	tg := capture(t, src)
	rec := &recorder{}
	run := func(c context.Context, name string, args ...string) error {
		cancel()
		return rec.run(c, name, args...)
	}
	res, err := Inserter{Run: run, Windows: src}.Insert(ctx, tg, long)
	if !errors.Is(err, context.Canceled) || len(rec.calls) != 1 {
		t.Fatalf("err %v after %d calls", err, len(rec.calls))
	}
	if rec.typed()+res.Rest != long {
		t.Fatalf("result %+v", res)
	}
}

func TestMultiByteTextIsNeverSplitInsideACharacter(t *testing.T) {
	family := "\U0001F469\u200d\U0001F469\u200d\U0001F467" // one ZWJ sequence
	flag := "\U0001F1EF\U0001F1F5"
	wave := "\U0001F44B\U0001F3FD"
	accent := "e\u0301"
	text := strings.Repeat("日本語のテキスト "+family+" café "+accent+flag+wave+" ö ", 6)
	// The longest cluster here is the five-rune family; every size from
	// there up must keep each cluster whole.
	for size := 5; size <= MaxChunk; size++ {
		chunks := Chunks(text, size)
		if strings.Join(chunks, "") != text {
			t.Fatalf("size %d: chunks do not rejoin to the text", size)
		}
		for _, c := range chunks {
			if c == "" || !utf8.ValidString(c) || utf8.RuneCountInString(c) > size {
				t.Fatalf("size %d: bad chunk %q", size, c)
			}
			first, _ := utf8.DecodeRuneInString(c)
			if unicode.In(first, unicode.Mn) || first == '\u200d' || (first >= 0x1f3fb && first <= 0x1f3ff) {
				t.Fatalf("size %d: chunk %q starts inside a character", size, c)
			}
			if strings.HasPrefix(c, "\U0001F1F5") {
				t.Fatalf("size %d: flag pair split at %q", size, c)
			}
		}
	}
	// Each wtype argument is valid UTF-8 in the real insert path too.
	src := &windows{at: func(int) window.Snapshot { return steady() }}
	tg := capture(t, src)
	rec := &recorder{}
	if _, err := (Inserter{Run: rec.run, Windows: src}).Insert(context.Background(), tg, text); err != nil {
		t.Fatal(err)
	}
	if rec.typed() != text {
		t.Fatal("typed text differs")
	}
	for _, c := range rec.calls {
		if !utf8.ValidString(c[2]) {
			t.Fatalf("invalid UTF-8 argument %q", c[2])
		}
	}
}

func TestOversizedClusterFallsBackToRunes(t *testing.T) {
	text := "a" + strings.Repeat("\u0301", 50) + "b"
	chunks := Chunks(text, 8)
	if strings.Join(chunks, "") != text {
		t.Fatal("chunks do not rejoin")
	}
	for _, c := range chunks {
		if !utf8.ValidString(c) || utf8.RuneCountInString(c) > 8 {
			t.Fatalf("bad chunk %q", c)
		}
	}
}

func TestLeadingDashesAndNewlines(t *testing.T) {
	text := "-k Return --help -\nsecond line\n\n- item"
	src := &windows{at: func(int) window.Snapshot { return steady() }}
	tg := capture(t, src)
	src.reads = 0
	rec := &recorder{}
	if _, err := (Inserter{Run: rec.run, Windows: src}).Insert(context.Background(), tg, text); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"wtype", "-s", Settle, "--", "-k Return --help -"},
		{"wtype", "-s", Settle, "--", "\nsecond line"},
		{"wtype", "-s", Settle, "--", "\n\n- item"},
	}
	if !slices.EqualFunc(rec.calls, want, slices.Equal) {
		t.Fatalf("calls %q, want %q", rec.calls, want)
	}
	// A newline is typed as Return, so the target is checked right before
	// each one.
	if src.reads != 3 {
		t.Fatalf("%d verifies for 3 chunks", src.reads)
	}
}

func TestKeyPressingTextIsRefusedWhole(t *testing.T) {
	for _, text := range []string{"ok\x1b:q!", "line\r\n", "undo\b", "del\x7f", "next\u0085", "bad\xffutf8", "nul\x00"} {
		src := &windows{at: func(int) window.Snapshot { return steady() }}
		tg := capture(t, src)
		rec := &recorder{}
		res, err := Inserter{Run: rec.run, Windows: src}.Insert(context.Background(), tg, text)
		if !errors.Is(err, ErrText) || len(rec.calls) != 0 || res.Rest != text || res.Landed != 0 {
			t.Fatalf("%q: err %v, %d calls, %+v", text, err, len(rec.calls), res)
		}
	}
	if err := Check("tab\tand\nnewline"); err != nil {
		t.Fatalf("tab and newline refused: %v", err)
	}
}

func TestAllowlist(t *testing.T) {
	cases := []struct {
		argv []string
		ok   bool
	}{
		{[]string{"wtype", "-s", Settle, "--", "hello"}, true},
		{[]string{"wtype", "-s", Settle, "--", "-M ctrl"}, true},
		{[]string{"wtype", "-s", Settle, "--", strings.Repeat("é", MaxChunk)}, true},
		{[]string{"wtype", "-s", Settle, "--", strings.Repeat("é", MaxChunk+1)}, false},
		{[]string{"wtype", "--", "hello"}, false},
		{[]string{"wtype", "-s", "0", "--", "hello"}, false},
		{[]string{"wtype", "-s", Settle, "hello"}, false},
		{[]string{"wtype", "-s", Settle, "--", "-s", "hello"}, false},
		{[]string{"wtype", "hello"}, false},
		{[]string{"wtype", "-s", Settle, "--", ""}, false},
		{[]string{"wtype", "-s", Settle, "--", "a", "b"}, false},
		{[]string{"wtype", "-"}, false},
		{[]string{"wtype", "-k", "Return"}, false},
		{[]string{"wtype", "-M", "ctrl", "--", "v"}, false},
		{[]string{"wtype", "-d", "10", "--", "x"}, false},
		{[]string{"wtype", "-s", Settle, "--", "esc\x1b"}, false},
		{[]string{"/usr/bin/wtype", "-s", Settle, "--", "hello"}, false},
		{[]string{"sh", "-c", "wtype -s " + Settle + " -- hello"}, false},
		{[]string{"wl-copy", "hello"}, false},
		{[]string{"wl-copy", "--", "hello"}, false},
		{[]string{"ydotool", "type", "hello"}, false},
	}
	for _, c := range cases {
		if got := Allowed(c.argv[0], c.argv[1:]...); got != c.ok {
			t.Errorf("Allowed(%q) = %v, want %v", c.argv, got, c.ok)
		}
	}
}

func TestExecRefusesBeforeStartingAProcess(t *testing.T) {
	run := Exec([]string{"PATH=/nonexistent"})
	for _, argv := range [][]string{{"wl-copy", "secret"}, {"wtype", "-k", "Return"}, {"wtype", "secret"}} {
		err := run(context.Background(), argv[0], argv[1:]...)
		if !errors.Is(err, ErrNotAllowed) {
			t.Fatalf("%q: %v", argv, err)
		}
		if strings.Contains(err.Error(), "secret") {
			t.Fatalf("refusal names the text: %v", err)
		}
	}
}

func TestExecForcesAUTF8Locale(t *testing.T) {
	got := utf8Env([]string{"PATH=/usr/bin", "LC_ALL=C", "LC_CTYPE=POSIX", "LANG=de_DE.UTF-8", "WAYLAND_DISPLAY=wayland-1"})
	want := []string{"PATH=/usr/bin", "LANG=de_DE.UTF-8", "WAYLAND_DISPLAY=wayland-1", "LC_ALL=C.UTF-8"}
	if !slices.Equal(got, want) {
		t.Fatalf("env %q, want %q", got, want)
	}
}

func TestMissingRunnerOrWindowsTypesNothing(t *testing.T) {
	src := &windows{at: func(int) window.Snapshot { return steady() }}
	tg := capture(t, src)
	if res, err := (Inserter{Windows: src}).Insert(context.Background(), tg, "hi"); !errors.Is(err, ErrNoRunner) || res.Rest != "hi" {
		t.Fatalf("no runner: %v %+v", err, res)
	}
	rec := &recorder{}
	res, err := Inserter{Run: rec.run}.Insert(context.Background(), tg, "hi")
	if target.ReasonOf(err) != target.ReasonUnavailable || len(rec.calls) != 0 || res.Rest != "hi" {
		t.Fatalf("no windows: %v %+v %q", err, res, rec.calls)
	}
	res, err = Inserter{Run: rec.run, Windows: src}.Insert(context.Background(), target.Target{}, "hi")
	if target.ReasonOf(err) != target.ReasonUnavailable || len(rec.calls) != 0 || res.Rest != "hi" {
		t.Fatalf("zero target: %v %+v", err, res)
	}
}

func TestMessageNeverEmpty(t *testing.T) {
	for _, err := range []error{ErrText, ErrTool, ErrNoRunner, ErrNotAllowed, context.Canceled, &target.Refusal{Reason: target.ReasonClosed}} {
		if Message(err) == "" {
			t.Errorf("no copy for %v", err)
		}
	}
	if Message(nil) != "" {
		t.Error("copy for nil")
	}
}
