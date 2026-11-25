package presence

import (
	"errors"
	"slices"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
)

func handsAt(phase string, step int) feature.HandsFrame {
	f := feature.HandsFrame{Stage: protocol.OniHandsBrowser, Active: true, Reason: feature.ReasonAllowed, Scope: protocol.OniHandsScopeBrowser, Phase: phase, Step: step, Max: 15}
	if phase == protocol.OniHandsStatePaused {
		f.Pause = protocol.OniHandsPauseFocus
	}
	return f
}

var handsEnded = feature.HandsFrame{Stage: protocol.OniHandsBrowser, Reason: feature.ReasonAllowed}

func hands(t *testing.T, s *Service, f feature.HandsFrame) {
	t.Helper()
	if err := s.Hands(f); err != nil {
		t.Fatalf("hands %+v: %v", f, err)
	}
}

// The snapshot carries the hands frame only while a task runs, after the
// guide, with the handoff card it waits on; the inactive frame goes out once
// and is never kept.
func TestTheSnapshotCarriesHandsOnlyWhileATaskRuns(t *testing.T) {
	s, _, rec := started(t)
	// no task: an inactive frame sends nothing and the snapshot is as before
	hands(t, s, handsEnded)
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("an inactive frame with no task sent %q", got)
	}
	if got := kinds(s.Snapshot()); !slices.Equal(got, oldOrder) {
		t.Fatalf("snapshot with no task = %q", got)
	}
	working := handsAt(protocol.OniHandsStateWorking, 1)
	hands(t, s, working)
	if got, want := rec.take(), encode(t, &working); !slices.Equal(got, want) {
		t.Fatalf("starting sent %q, want %q", got, want)
	}
	// the same frame again sends nothing
	hands(t, s, working)
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("a repeated frame sent %q", got)
	}
	reading := guideAt(1, feature.GuideReading)
	guide(t, s, reading)
	rec.take()
	snap := s.Snapshot()
	if got, want := kinds(snap), append(slices.Clone(oldOrder), feature.KindGuide, feature.KindHands); !slices.Equal(got, want) {
		t.Fatalf("snapshot = %q, want %q", got, want)
	}
	guide(t, s, guideAt(2, feature.GuideIdle))
	rec.take()

	// a waiting task holds its handoff card; the app's cards are not held
	waiting := handsAt(protocol.OniHandsStateWaiting, 2)
	hands(t, s, waiting)
	card := feature.HandoffCard(feature.HandsBrowser, "h-2", "Your turn. Press Send when it looks right.", time.Minute)
	if err := s.Consent(*card); err != nil {
		t.Fatal(err)
	}
	if err := s.Consent(*feature.KeepOffer("keep-1", "Keep these steps?")); err != nil {
		t.Fatal(err)
	}
	rec.take()
	if got, want := kinds(s.Snapshot()), append(slices.Clone(oldOrder), feature.KindHands, feature.KindConsent); !slices.Equal(got, want) {
		t.Fatalf("waiting snapshot = %q, want %q", got, want)
	}
	if c := s.Snapshot()[len(oldOrder)+1].(*feature.ConsentFrame); c.ID != "h-2" {
		t.Fatalf("the snapshot holds card %q, want the handoff card", c.ID)
	}
	// once the person pressed it the card is gone from the snapshot
	hands(t, s, handsAt(protocol.OniHandsStateWorking, 3))
	if got, want := kinds(s.Snapshot()), append(slices.Clone(oldOrder), feature.KindHands); !slices.Equal(got, want) {
		t.Fatalf("snapshot after the card = %q, want %q", got, want)
	}
	// a card for a task that is not waiting is not held
	if err := s.Consent(*card); err != nil {
		t.Fatal(err)
	}
	if got := kinds(s.Snapshot()); slices.Contains(got, feature.KindConsent) {
		t.Fatalf("a card held while working: %q", got)
	}
	rec.take()

	// the inactive frame goes out once and leaves nothing behind
	hands(t, s, handsEnded)
	if got, want := rec.take(), encode(t, &handsEnded); !slices.Equal(got, want) {
		t.Fatalf("ending sent %q, want %q", got, want)
	}
	if got := kinds(s.Snapshot()); !slices.Equal(got, oldOrder) {
		t.Fatalf("snapshot after the task = %q", got)
	}
	hands(t, s, handsEnded)
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("a second inactive frame sent %q", got)
	}
}

// A task wakes a sleeping Oni and holds off sleep while it runs; once it
// ends only the quiet spell is armed.
func TestAHandsTaskKeepsOniAwake(t *testing.T) {
	s, clock, rec := started(t)
	clock.Advance(SleepAfter)
	rec.take()
	working := handsAt(protocol.OniHandsStateWorking, 1)
	hands(t, s, working)
	want := encode(t, &working, &feature.GestureFrame{Seq: 2, ID: "wake"}, &feature.StateFrame{State: "idle"})
	if got := rec.take(); !slices.Equal(got, want) {
		t.Fatalf("a task on a sleeping Oni sent %q, want %q", got, want)
	}
	if clock.Armed() != 0 {
		t.Fatalf("%d timers armed while Hands works", clock.Armed())
	}
	hands(t, s, handsAt(protocol.OniHandsStatePaused, 1))
	clock.Advance(10 * SleepAfter)
	for _, f := range rec.take() {
		if f == encode(t, &feature.GestureFrame{Seq: 3, ID: "sleep"})[0] {
			t.Fatal("Oni fell asleep while a task was paused")
		}
	}
	hands(t, s, handsEnded)
	rec.take()
	if clock.Armed() != 1 {
		t.Fatalf("%d timers armed after the task; only the quiet spell should be", clock.Armed())
	}
}

// An invalid frame changes nothing; an active one is refused while Oni is
// off, and turning Oni off forgets the task.
func TestHandsFramesAreCheckedAndOffForgetsTheTask(t *testing.T) {
	s, _, rec := started(t)
	bad := handsAt(protocol.OniHandsStateWorking, 1)
	bad.Pause = protocol.OniHandsPauseFocus
	if err := s.Hands(bad); err == nil {
		t.Fatal("a working frame with a pause was accepted")
	}
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("an invalid frame sent %q", got)
	}
	hands(t, s, handsAt(protocol.OniHandsStateWorking, 1))
	off := on()
	off.Master = false
	s.Settings(off)
	rec.take()
	if err := s.Hands(handsAt(protocol.OniHandsStateWorking, 2)); !errors.Is(err, ErrOff) {
		t.Fatalf("an active frame while off = %v, want ErrOff", err)
	}
	// the task was forgotten with Oni: no inactive frame is owed
	hands(t, s, handsEnded)
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("ending a forgotten task sent %q", got)
	}
	s.Settings(on())
	if got := kinds(s.Snapshot()); slices.Contains(got, feature.KindHands) {
		t.Fatalf("Oni back on still shows the task: %q", got)
	}
	s.Close()
	if err := s.Hands(handsAt(protocol.OniHandsStateWorking, 1)); !errors.Is(err, ErrClosed) {
		t.Fatalf("after close = %v", err)
	}
}
