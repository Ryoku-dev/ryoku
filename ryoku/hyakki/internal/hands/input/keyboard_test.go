package input

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

// recorder is a fake Runner that logs every call and the checks between
// them; it fails the call numbered failAt (from 1).
type recorder struct {
	log    []string
	calls  int
	failAt int
}

func (r *recorder) run(_ context.Context, name string, args ...string) error {
	r.calls++
	if !Allowed(name, args...) {
		r.log = append(r.log, "REFUSED")
		return ErrNotAllowed
	}
	if r.calls == r.failAt {
		r.log = append(r.log, "fail")
		return errors.New("tool exited 1")
	}
	if args[2] == "--" {
		r.log = append(r.log, "type:"+args[3])
	} else {
		r.log = append(r.log, "key:"+strings.Join(args[2:], " "))
	}
	return nil
}

func TestTypeSendsAllowedChunksOfAtMostForty(t *testing.T) {
	r := &recorder{}
	text := strings.Repeat("abcdefghij", 10)
	got, err := Keyboard{Run: r.run}.Type(context.Background(), text)
	if err != nil || got.Landed != 100 || got.Rest != "" {
		t.Fatalf("Type = %+v, %v", got, err)
	}
	if len(r.log) != 3 {
		t.Fatalf("log = %q", r.log)
	}
	for _, entry := range r.log {
		chunk, ok := strings.CutPrefix(entry, "type:")
		if !ok || utf8.RuneCountInString(chunk) > MaxChunk {
			t.Errorf("call %q", entry)
		}
	}
}

func TestTypeRefusesKeysInTextBeforeTypingAnything(t *testing.T) {
	for _, text := range []string{"line\nnext", "a\tb", "a\rb", "esc\x1b", "nul\x00", "a\u2029b", "bad\xff"} {
		r := &recorder{}
		got, err := Keyboard{Run: r.run}.Type(context.Background(), strings.Repeat("x", 50)+text)
		if !errors.Is(err, ErrText) || len(r.log) != 0 || got.Landed != 0 {
			t.Errorf("%q: %+v, %v, log %q", text, got, err, r.log)
		}
		calls := 0
		_, err = Keyboard{Run: r.run}.TypeChecked(context.Background(), text, func(context.Context) error { calls++; return nil })
		if !errors.Is(err, ErrText) || calls != 0 || len(r.log) != 0 {
			t.Errorf("checked %q: %v, %d checks, log %q", text, err, calls, r.log)
		}
	}
}

func TestTypeFailureReportsWhatLanded(t *testing.T) {
	r := &recorder{failAt: 2}
	text := strings.Repeat("a", 40) + strings.Repeat("b", 40) + "c"
	got, err := Keyboard{Run: r.run}.Type(context.Background(), text)
	if !errors.Is(err, ErrTool) || got.Landed != 40 || got.Unsure != 40 || got.Rest != text[40:] {
		t.Fatalf("Type = %+v, %v", got, err)
	}
	if len(r.log) != 2 {
		t.Fatalf("typed after a failure: %q", r.log)
	}
}

func TestTypeCheckedVerifiesBeforeAndAfterEveryChunk(t *testing.T) {
	r := &recorder{}
	check := func(context.Context) error { r.log = append(r.log, "check"); return nil }
	text := strings.Repeat("a", 40) + "b"
	got, err := Keyboard{Run: r.run}.TypeChecked(context.Background(), text, check)
	if err != nil || got.Landed != 41 {
		t.Fatalf("TypeChecked = %+v, %v", got, err)
	}
	want := "check,type:" + strings.Repeat("a", 40) + ",check,check,type:b,check"
	if strings.Join(r.log, ",") != want {
		t.Fatalf("log = %q", r.log)
	}
}

func TestTypeCheckedStopsAtTheFirstRefusal(t *testing.T) {
	moved := errors.New("the window moved")
	text := strings.Repeat("a", 40) + strings.Repeat("b", 40) + "c"
	cases := []struct {
		name     string
		failOn   int // the check numbered failOn (from 1) refuses
		landed   int
		typed    int
		restFrom int
	}{
		{"before the first chunk", 1, 0, 0, 0},
		{"after the first chunk", 2, 40, 1, 40},
		{"before the second chunk", 3, 40, 1, 40},
		{"after the second chunk", 4, 80, 2, 80},
	}
	for _, c := range cases {
		r := &recorder{}
		checks := 0
		check := func(context.Context) error {
			checks++
			if checks == c.failOn {
				return moved
			}
			return nil
		}
		got, err := Keyboard{Run: r.run}.TypeChecked(context.Background(), text, check)
		if err != moved {
			t.Errorf("%s: err = %v; want the check's own", c.name, err)
		}
		if got.Landed != c.landed || got.Rest != text[c.restFrom:] || got.Unsure != 0 || r.calls != c.typed || checks != c.failOn {
			t.Errorf("%s: %+v after %d calls and %d checks", c.name, got, r.calls, checks)
		}
	}
}

func TestTypeCheckedNeedsACheck(t *testing.T) {
	r := &recorder{}
	got, err := Keyboard{Run: r.run}.TypeChecked(context.Background(), "hi", nil)
	if !errors.Is(err, ErrNoCheck) || r.calls != 0 || got.Rest != "hi" {
		t.Fatalf("TypeChecked = %+v, %v after %d calls", got, err, r.calls)
	}
}

func TestTypeStopsWhenTheContextEnds(t *testing.T) {
	r := &recorder{}
	ctx, cancel := context.WithCancel(context.Background())
	check := func(context.Context) error { cancel(); return nil }
	text := strings.Repeat("a", 40) + "b"
	got, err := Keyboard{Run: r.run}.TypeChecked(ctx, text, check)
	if !errors.Is(err, context.Canceled) || r.calls != 1 || got.Landed != 40 {
		t.Fatalf("TypeChecked = %+v, %v after %d calls", got, err, r.calls)
	}
}

func TestChordPressesOneAllowedChord(t *testing.T) {
	r := &recorder{}
	k := Keyboard{Run: r.run}
	if err := k.Chord(context.Background(), "ctrl+z"); err != nil {
		t.Fatal(err)
	}
	if err := k.Chord(context.Background(), "ctrl+,"); !errors.Is(err, ErrKey) {
		t.Fatalf("ctrl+, = %v", err)
	}
	if strings.Join(r.log, ",") != "key:-M ctrl -k z -m ctrl" {
		t.Fatalf("log = %q", r.log)
	}
	if err := (Keyboard{}).Chord(context.Background(), "Tab"); !errors.Is(err, ErrNoRunner) {
		t.Fatalf("no runner = %v", err)
	}
}
