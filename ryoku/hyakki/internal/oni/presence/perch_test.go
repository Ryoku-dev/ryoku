package presence

import (
	"errors"
	"slices"
	"sync"
	"testing"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
)

var editor = feature.PerchFrame{Window: "0x55d1c2a0", App: "editor", Title: "notes.txt", Corner: "top-right", Output: "DP-1", Box: &feature.Box{X: 100, Y: 80, W: 900, H: 600}}

// detaches records what OnDetach reported.
type detaches struct {
	mu      sync.Mutex
	reasons []string
}

func watchDetach(s *Service) *detaches {
	d := &detaches{}
	s.OnDetach(func(reason string) {
		d.mu.Lock()
		defer d.mu.Unlock()
		d.reasons = append(d.reasons, reason)
	})
	return d
}

func (d *detaches) take() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := d.reasons
	d.reasons = nil
	return out
}

func off() protocol.OniSettings {
	o := on()
	o.Master = false
	return o
}

func perched(t *testing.T) (*Service, *fakeClock, *recorder, *detaches) {
	t.Helper()
	s, clock, rec, _ := presenting(t)
	d := watchDetach(s)
	seq, err := s.Perch(editor)
	if err != nil || seq != 1 {
		t.Fatalf("perch = %d, %v", seq, err)
	}
	return s, clock, rec, d
}

func TestPerchSendsTheFrameThenCurlsOnTheWindow(t *testing.T) {
	s, _, rec, _ := perched(t)
	want := editor
	want.Seq, want.Attached = 1, true
	if got := rec.take(); !slices.Equal(got, encode(t, &want, &feature.StrandTargetFrame{Seq: 1, Mode: feature.StrandModePerch, W: 1, H: 1})) {
		t.Fatalf("perch sent %q", got)
	}
	if p, ok := s.Perched(); !ok || p.Seq != 1 || *p.Box != *editor.Box || p.Window != editor.Window {
		t.Fatalf("perched = %+v, %v", p, ok)
	}
	snap := s.Snapshot()
	if got := kinds(snap); !slices.Equal(got, []string{"state", "character", "motion", "placement", "perch", "voice"}) {
		t.Fatalf("a perched snapshot is %q", got)
	}
}

func TestPerchNeedsAPresenterAndOniOnAndShown(t *testing.T) {
	s, clock, rec := started(t)
	if _, err := s.Perch(editor); !errors.Is(err, ErrNoPresenter) {
		t.Fatalf("perch with nothing drawing: %v", err)
	}
	s.Presenter(true)
	hidden := on()
	hidden.Hidden = true
	s.Settings(hidden)
	rec.take()
	if _, err := s.Perch(editor); !errors.Is(err, ErrHidden) {
		t.Fatalf("perch while hidden: %v", err)
	}
	s.Settings(off())
	rec.take()
	if _, err := s.Perch(editor); !errors.Is(err, ErrOff) {
		t.Fatalf("perch while off: %v", err)
	}
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("refused perches sent %q", got)
	}
	if _, err := s.Strand(feature.StrandTargetFrame{Mode: feature.StrandModePerch, W: 1, H: 1}); !errors.Is(err, ErrNotPerched) {
		t.Fatalf("a perch target with no perch: %v", err)
	}
	if clock.Armed() != 0 {
		t.Fatalf("%d timers armed with Oni off", clock.Armed())
	}
}

func TestAMoveSendsANewSeqAndANewOutputCurlsAgain(t *testing.T) {
	s, _, rec, _ := perched(t)
	rec.take()
	if seq := s.PerchUpdate(*editor.Box, editor.Output); seq != 1 {
		t.Fatalf("an unchanged window took seq %d", seq)
	}
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("an unchanged window sent %q", got)
	}
	moved := feature.Box{X: 140, Y: 90, W: 900, H: 600}
	if seq := s.PerchUpdate(moved, editor.Output); seq != 2 {
		t.Fatalf("a move took seq %d", seq)
	}
	want := editor
	want.Seq, want.Attached, want.Box = 2, true, &moved
	if got := rec.take(); !slices.Equal(got, encode(t, &want)) {
		t.Fatalf("a move sent %q", got)
	}
	if seq := s.PerchUpdate(moved, "HDMI-A-1"); seq != 3 {
		t.Fatalf("an output change took seq %d", seq)
	}
	want.Seq, want.Output = 3, "HDMI-A-1"
	if got := rec.take(); !slices.Equal(got, encode(t, &want, &feature.StrandTargetFrame{Seq: 2, Mode: feature.StrandModePerch, W: 1, H: 1})) {
		t.Fatalf("an output change sent %q", got)
	}
}

func TestDetachRewindsTheStrandThenSendsADetachedFrame(t *testing.T) {
	s, clock, rec, d := perched(t)
	before := clock.Armed()
	s.Arrived(1, "")
	rec.take()
	s.Detach(feature.PerchUser)
	if got := rec.take(); !slices.Equal(got, encode(t, &feature.StrandRetractFrame{Seq: 1}, &feature.PerchFrame{Seq: 2})) {
		t.Fatalf("detach sent %q", got)
	}
	if got := d.take(); !slices.Equal(got, []string{feature.PerchUser}) {
		t.Fatalf("detach reported %q", got)
	}
	if _, ok := s.Perched(); ok {
		t.Fatal("still perched")
	}
	if got := kinds(s.Snapshot()); !slices.Equal(got, []string{"state", "character", "motion", "placement", "voice"}) {
		t.Fatalf("a detached snapshot is %q", got)
	}
	s.Detach(feature.PerchUser)
	if got := rec.take(); len(got) != 0 || len(d.take()) != 0 {
		t.Fatalf("a second detach sent %q", got)
	}
	// the backstop was armed only while the curl waited; perching leaves no
	// timer of its own behind
	if clock.Armed() > before {
		t.Fatalf("%d timers armed after detach, %d before", clock.Armed(), before)
	}
}

func TestEveryWayOniGoesAwayDetachesWithItsReason(t *testing.T) {
	cases := []struct {
		reason string
		end    func(s *Service)
		frames bool
	}{
		{feature.PerchPresenterLost, func(s *Service) { s.Presenter(false) }, true},
		{feature.PerchOniOff, func(s *Service) { s.Settings(off()) }, true},
		{feature.PerchHidden, func(s *Service) {
			o := on()
			o.Hidden = true
			s.Settings(o)
		}, true},
		{feature.PerchPresenterLost, func(s *Service) { s.Close() }, false},
	}
	for _, c := range cases {
		s, _, rec, d := perched(t)
		rec.take()
		c.end(s)
		got := rec.take()
		if c.frames && (len(got) < 2 || !slices.Contains(got, encode(t, &feature.PerchFrame{Seq: 2})[0])) {
			t.Errorf("%s sent %q", c.reason, got)
		}
		if !c.frames && len(got) != 0 {
			t.Errorf("%s sent %q", c.reason, got)
		}
		if r := d.take(); !slices.Equal(r, []string{c.reason}) {
			t.Errorf("reported %q, want %s", r, c.reason)
		}
	}
}

func TestBubbleHangsOnTheStrandOut(t *testing.T) {
	s, _, rec, _ := presenting(t)
	if err := s.Bubble(0, "answer", "Here."); !errors.Is(err, ErrNoStrand) {
		t.Fatalf("a bubble with no strand: %v", err)
	}
	seq := target(t, s)
	rec.take()
	if err := s.Bubble(0, "answer", "Here."); err != nil {
		t.Fatal(err)
	}
	if got := rec.take(); !slices.Equal(got, encode(t, &feature.BubbleFrame{Seq: seq, Tone: "answer", Text: "Here."})) {
		t.Fatalf("bubble sent %q", got)
	}
	if err := s.Bubble(seq, "shout", "Here."); err == nil {
		t.Fatal("an unknown tone was sent")
	}
}

func TestPerchingReplacesThePerchWithoutADetach(t *testing.T) {
	s, _, rec, d := perched(t)
	rec.take()
	other := feature.PerchFrame{Window: "14", Corner: "top-left", Output: "DP-1", Box: &feature.Box{X: 0, Y: 0, W: 400, H: 300}}
	if seq, err := s.Perch(other); err != nil || seq != 2 {
		t.Fatalf("perch = %d, %v", seq, err)
	}
	want := other
	want.Seq, want.Attached = 2, true
	if got := rec.take(); !slices.Equal(got, encode(t, &want, &feature.StrandTargetFrame{Seq: 2, Mode: feature.StrandModePerch, W: 1, H: 1})) {
		t.Fatalf("a second perch sent %q", got)
	}
	if r := d.take(); len(r) != 0 {
		t.Fatalf("a second perch reported %q", r)
	}
}

func TestAPerchedOniReportsItsSleepAndWake(t *testing.T) {
	s, clock, _, _ := perched(t)
	var mu sync.Mutex
	var rests []bool
	s.OnRest(func(resting bool) {
		mu.Lock()
		defer mu.Unlock()
		rests = append(rests, resting)
	})
	s.Arrived(1, "")
	clock.Advance(SleepAfter)
	s.Interact("poke")
	mu.Lock()
	got := slices.Clone(rests)
	mu.Unlock()
	if !slices.Equal(got, []bool{true, false}) {
		t.Fatalf("rests = %v", got)
	}
	s.Detach(feature.PerchUser)
	clock.Advance(SleepAfter)
	s.Interact("poke")
	mu.Lock()
	defer mu.Unlock()
	if len(rests) != 2 {
		t.Fatalf("a detached Oni reported rests %v", rests)
	}
}
