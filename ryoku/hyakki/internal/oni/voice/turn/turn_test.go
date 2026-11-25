package turn

import (
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/spec"
)

func load(t *testing.T) *spec.OniSpec {
	t.Helper()
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	return sp
}

var t0 = time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC)

func ms(n int) time.Duration { return time.Duration(n) * time.Millisecond }

func TestEchoWindow(t *testing.T) {
	tr := New(load(t), nil)
	if tr.Echoing(t0) || !tr.EchoUntil().IsZero() {
		t.Fatal("echo window open before Oni spoke")
	}
	for _, n := range []int{0, 1, 10, 42, 600} {
		tr := New(load(t), nil)
		tr.Spoke(n, t0)
		want := t0.Add(ms(900 + 70*n))
		if got := tr.EchoUntil(); !got.Equal(want) {
			t.Fatalf("%d chars: window ends %v after, want %v", n, got.Sub(t0), want.Sub(t0))
		}
		if !tr.Echoing(want.Add(-time.Nanosecond)) || tr.Echoing(want) {
			t.Fatalf("%d chars: window edges wrong", n)
		}
	}
	// Characters, not bytes: five runes, ten bytes.
	tr.Said("ÉéÉéÉ", t0)
	if got := tr.EchoUntil().Sub(t0); got != ms(900+70*5) {
		t.Fatalf("said 5 runes: window %v", got)
	}
	// A shorter, later line never shortens a longer window.
	tr = New(load(t), nil)
	tr.Spoke(100, t0)
	tr.Spoke(1, t0.Add(ms(10)))
	if got := tr.EchoUntil().Sub(t0); got != ms(900+7000) {
		t.Fatalf("window shortened to %v", got)
	}
	tr.Spoke(-5, t0)
	if got := tr.EchoUntil().Sub(t0); got != ms(900+7000) {
		t.Fatalf("negative length moved the window to %v", got)
	}
}

func TestBargeStartsANewTurn(t *testing.T) {
	cuts := 0
	var tr *Turn
	tr = New(load(t), func() {
		cuts++
		_ = tr.ID() // the hook may read the turn back
	})
	old := tr.ID()
	tr.Said("Here is the long answer you asked for", t0)
	id := tr.Barge()
	if id != old+1 || tr.ID() != id {
		t.Fatalf("barge id %d after %d", id, old)
	}
	if cuts != 1 {
		t.Fatalf("cancel hook ran %d times", cuts)
	}
	if !tr.EchoUntil().IsZero() || tr.Echoing(t0.Add(ms(1))) {
		t.Fatal("echo window survived the barge")
	}
	if tr.Heard("here is the long answer", t0.Add(ms(1))) {
		t.Fatal("an old line still counted as heard back after the barge")
	}
	if New(load(t), nil).Barge() != 1 {
		t.Fatal("nil cancel hook")
	}
}

func TestStaleTurnLinesDropped(t *testing.T) {
	tr := New(load(t), nil)
	first := tr.Barge()
	if !tr.Accept(first) {
		t.Fatal("current turn refused")
	}
	// An answer for the first press arrives after the second press.
	second := tr.Barge()
	var spoken []uint64
	for _, id := range []uint64{first, second, first, 0, second + 1} {
		if tr.Accept(id) {
			spoken = append(spoken, id)
		}
	}
	if len(spoken) != 1 || spoken[0] != second {
		t.Fatalf("accepted %v, want only %d", spoken, second)
	}
}

func TestHeardBack(t *testing.T) {
	tr := New(load(t), nil)
	line := "Okay — opening Firefox now."
	tr.Said(line, t0)
	end := t0.Add(ms(900 + 70*len([]rune(line))))
	in := t0.Add(ms(400))
	own := []string{
		"okay opening firefox now",
		"OKAY, OPENING FIREFOX NOW!",
		"opening Firefox",  // the mic caught a run of it
		"  firefox  now. ", // the tail of it
	}
	for _, s := range own {
		if !tr.Heard(s, in) {
			t.Errorf("own line %q taken as the user's", s)
		}
	}
	user := []string{
		"open firefox",                          // different words
		"okay opening firefox now and close it", // longer than the line
		"now firefox",                           // same words, other order
		"close the browser",                     // unrelated
		"",                                      // nothing heard
		"—!",                                    // no words
	}
	for _, s := range user {
		if tr.Heard(s, in) {
			t.Errorf("user utterance %q dropped as Oni's own", s)
		}
	}
	if !tr.Heard(line, end.Add(-time.Millisecond)) {
		t.Fatal("own line missed at the window's last moment")
	}
	if tr.Heard(line, end) {
		t.Fatal("own line still guarded after its echo window")
	}

	// Two lines: each is guarded for its own window only.
	tr = New(load(t), nil)
	tr.Said("A long first line that keeps talking for quite some time", t0)
	tr.Said("Done", t0.Add(ms(100)))
	late := t0.Add(ms(100 + 900 + 70*4))
	if tr.Heard("done", late) {
		t.Fatal("short line guarded past its window")
	}
	if !tr.Heard("keeps talking", late) {
		t.Fatal("long line dropped before its window ended")
	}
}

// The mic hears Oni's lines back to back: the acknowledgement and the
// reply after it arrive as one utterance, which is still Oni's own. A run
// across two lines counts only while both are in their windows, in the
// order they were said.
func TestHeardBackAcrossConsecutiveLines(t *testing.T) {
	tr := New(load(t), nil)
	tr.Said("One moment.", t0)
	tr.Said("It keeps short ledger notes.", t0.Add(ms(900)))
	in := t0.Add(ms(1500))
	for _, s := range []string{
		"One moment, it keeps short ledger notes.",
		"moment it keeps short",
	} {
		if !tr.Heard(s, in) {
			t.Errorf("Oni's two lines heard back as %q were taken as the user's", s)
		}
	}
	for _, s := range []string{
		"it keeps short ledger notes one moment", // the lines in the other order
		"one moment please it keeps short ledger notes",
	} {
		if tr.Heard(s, in) {
			t.Errorf("user utterance %q dropped as Oni's own", s)
		}
	}
	// once the first line's window has closed, a run across both is the user's
	late := t0.Add(ms(900 + 70*len([]rune("One moment."))))
	if tr.Heard("One moment, it keeps short ledger notes.", late) {
		t.Error("a run across a line whose window had closed was still guarded")
	}
	if !tr.Heard("it keeps short ledger notes", late) {
		t.Error("the second line was dropped before its window ended")
	}
}

func TestOneAckPerPressWithGap(t *testing.T) {
	sp := load(t)
	gap := ms(int(sp.Voice.Turn.AckGapMs))
	if gap != ms(300) {
		t.Fatalf("spec ack gap %v", gap)
	}
	tr := New(sp, nil)
	tr.Barge()
	if !tr.Ack(t0) {
		t.Fatal("first ack refused")
	}
	if tr.Ack(t0.Add(5 * time.Second)) {
		t.Fatal("second ack in one press")
	}
	tr.Barge()
	if tr.Ack(t0.Add(gap - time.Millisecond)) {
		t.Fatal("ack inside the gap")
	}
	if !tr.Ack(t0.Add(gap)) {
		t.Fatal("ack of a new press after the gap refused")
	}
	if tr.Ack(t0.Add(gap + time.Second)) {
		t.Fatal("second ack in the second press")
	}
}

func TestConcurrentUse(t *testing.T) {
	tr := New(load(t), func() {})
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 200 {
				now := t0.Add(ms(i*1000 + j))
				switch j % 6 {
				case 0:
					tr.Barge()
				case 1:
					tr.Said("one two three", now)
				case 2:
					tr.Heard("two three", now)
				case 3:
					tr.Accept(tr.ID())
				case 4:
					tr.Ack(now)
				case 5:
					tr.Spoke(j, now)
					tr.Echoing(now)
				}
			}
		}()
	}
	wg.Wait()
	if tr.ID() != 8*34 {
		t.Fatalf("turn id %d after %d barges", tr.ID(), 8*34)
	}
}
