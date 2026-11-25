package presence

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/spec"
)

// outcomes records what OnArrive and OnCancel reported.
type outcomes struct {
	mu        sync.Mutex
	arrived   []string
	cancelled []string
}

func (o *outcomes) watch(s *Service) {
	s.OnArrive(func(seq uint64, skipped string) {
		o.mu.Lock()
		defer o.mu.Unlock()
		o.arrived = append(o.arrived, note(seq, skipped))
	})
	s.OnCancel(func(seq uint64, why string) {
		o.mu.Lock()
		defer o.mu.Unlock()
		o.cancelled = append(o.cancelled, note(seq, why))
	})
}

func (o *outcomes) take() (arrived, cancelled []string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	arrived, cancelled = o.arrived, o.cancelled
	o.arrived, o.cancelled = nil, nil
	return arrived, cancelled
}

func note(seq uint64, s string) string { return fmt.Sprintf("%d %s", seq, s) }

var button = feature.StrandTargetFrame{X: 0.25, Y: 0.5, W: 0.125, H: 0.0625, Label: "Save button"}

func presenting(t *testing.T) (*Service, *fakeClock, *recorder, *outcomes) {
	t.Helper()
	s, clock, rec := started(t)
	o := &outcomes{}
	o.watch(s)
	s.Presenter(true)
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("a presenter arriving sent %q", got)
	}
	return s, clock, rec, o
}

func target(t *testing.T, s *Service) uint64 {
	t.Helper()
	seq, err := s.Strand(button)
	if err != nil {
		t.Fatal(err)
	}
	return seq
}

func TestArrivalTimeoutIsTheSpecsNumber(t *testing.T) {
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Duration(sp.Session.ArrivalTimeoutMs * float64(time.Millisecond)); ArrivalTimeout != want || want != 10*time.Second {
		t.Fatalf("ArrivalTimeout = %v, the spec says %v", ArrivalTimeout, want)
	}
}

func TestStrandNeedsAPresenterAndOniOn(t *testing.T) {
	s, clock, rec := started(t)
	if seq, err := s.Strand(button); !errors.Is(err, ErrNoPresenter) || seq != 0 {
		t.Fatalf("a strand with no presenter: %d, %v", seq, err)
	}
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("a strand with no presenter sent %q", got)
	}
	if clock.Armed() != 1 {
		t.Fatalf("%d timers armed; only the quiet spell should be", clock.Armed())
	}
	s.Presenter(true)
	off := on()
	off.Master = false
	s.Settings(off)
	rec.take()
	if _, err := s.Strand(button); !errors.Is(err, ErrOff) {
		t.Fatalf("a strand while Oni is off: %v", err)
	}
	if got := rec.take(); len(got) != 0 || clock.Armed() != 0 {
		t.Fatalf("a strand while Oni is off sent %q and armed %d timers", got, clock.Armed())
	}
	s.Settings(on())
	rec.take()
	bad := button
	bad.Mode = feature.StrandModeRegion
	if _, err := s.Strand(bad); err == nil {
		t.Fatal("a region target without its region was sent")
	}
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("an invalid target sent %q", got)
	}
	if seq := target(t, s); seq != 1 {
		t.Fatalf("the first target after a refused one is seq %d, want 1", seq)
	}
	s.Close()
	if _, err := s.Strand(button); !errors.Is(err, ErrClosed) {
		t.Fatalf("a strand after Close: %v", err)
	}
}

func TestOnlyTheNewestTargetAdvancesAndOnlyOnce(t *testing.T) {
	s, _, rec, o := presenting(t)
	var seqs []uint64
	for range 6 {
		seqs = append(seqs, target(t, s))
	}
	if !slices.Equal(seqs, []uint64{1, 2, 3, 4, 5, 6}) {
		t.Fatalf("seqs = %v, want 1 to 6", seqs)
	}
	var want []string
	for _, seq := range seqs {
		f := button
		f.Seq = seq
		want = append(want, encode(t, &f)...)
	}
	if got := rec.take(); !slices.Equal(got, want) {
		t.Fatalf("targets sent %q, want %q", got, want)
	}
	if _, cancelled := o.take(); len(cancelled) != 5 || cancelled[4] != note(5, CancelSuperseded) {
		t.Fatalf("superseded targets reported %q", cancelled)
	}
	if s.Arrived(5, "") {
		t.Fatal("a report for the superseded target 5 advanced")
	}
	if arrived, _ := o.take(); len(arrived) != 0 {
		t.Fatalf("a stale report advanced %q", arrived)
	}
	if !s.Arrived(6, "") {
		t.Fatal("the report for the pending target 6 did not advance")
	}
	if s.Arrived(6, "") || s.Arrived(7, "") || s.Arrived(0, "") {
		t.Fatal("a repeated or unknown report advanced")
	}
	if arrived, cancelled := o.take(); !slices.Equal(arrived, []string{note(6, "")}) || len(cancelled) != 0 {
		t.Fatalf("arrived %q, cancelled %q", arrived, cancelled)
	}
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("reports sent %q", got)
	}
}

func TestTheBackstopIsArmedOnlyWhileATargetWaits(t *testing.T) {
	s, clock, rec, o := presenting(t)
	if clock.Armed() != 1 {
		t.Fatalf("%d timers armed with nothing pending; only the quiet spell should be", clock.Armed())
	}
	seq := target(t, s)
	if clock.Armed() != 1 {
		t.Fatalf("%d timers armed while a target waits; only the backstop should be", clock.Armed())
	}
	clock.Advance(ArrivalTimeout - time.Millisecond)
	if _, cancelled := o.take(); len(cancelled) != 0 {
		t.Fatalf("cancelled %q before the backstop", cancelled)
	}
	clock.Advance(time.Millisecond)
	if arrived, cancelled := o.take(); len(arrived) != 0 || !slices.Equal(cancelled, []string{note(seq, CancelTimeout)}) {
		t.Fatalf("the backstop reported arrived %q, cancelled %q", arrived, cancelled)
	}
	if s.Arrived(seq, "") {
		t.Fatal("a report after the backstop advanced")
	}
	rec.take()
	clock.Advance(SleepAfter)
	if got := kinds(decodeAll(t, rec.take())); !slices.Equal(got, []string{feature.KindGesture, feature.KindState}) {
		t.Fatalf("Oni did not fall asleep after the backstop: %q", got)
	}
	if clock.Armed() != 0 {
		t.Fatalf("%d timers armed while asleep with nothing pending", clock.Armed())
	}
}

func TestATargetHoldsOffSleepAndLandingStartsTheQuietSpell(t *testing.T) {
	s, clock, rec, _ := presenting(t)
	clock.Advance(SleepAfter / 2)
	seq := target(t, s)
	rec.take()
	clock.Advance(ArrivalTimeout - time.Second)
	if !s.Arrived(seq, "") {
		t.Fatal("the pending target did not land")
	}
	if clock.Armed() != 1 {
		t.Fatalf("%d timers armed after landing; only the quiet spell should be", clock.Armed())
	}
	clock.Advance(SleepAfter - time.Millisecond)
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("Oni slept %q before a full quiet spell after landing", got)
	}
}

func TestAStrandWakesASleepingOniFirst(t *testing.T) {
	s, clock, rec, _ := presenting(t)
	clock.Advance(SleepAfter)
	rec.take()
	if clock.Armed() != 0 {
		t.Fatal("a sleeping Oni has a timer armed")
	}
	seq := target(t, s)
	f := button
	f.Seq = seq
	want := encode(t, &feature.GestureFrame{Seq: 2, ID: "wake"}, &feature.StateFrame{State: "idle"}, &f)
	if got := rec.take(); !slices.Equal(got, want) {
		t.Fatalf("a strand to a sleeping Oni sent %q, want %q", got, want)
	}
	if clock.Armed() != 1 {
		t.Fatalf("%d timers armed; only the backstop should be", clock.Armed())
	}
}

func TestLosingThePresenterCancelsWithoutAdvancing(t *testing.T) {
	s, clock, rec, o := presenting(t)
	seq := target(t, s)
	rec.take()
	s.Presenter(false)
	if arrived, cancelled := o.take(); len(arrived) != 0 || !slices.Equal(cancelled, []string{note(seq, CancelPresenterLost)}) {
		t.Fatalf("losing the presenter reported arrived %q, cancelled %q", arrived, cancelled)
	}
	if s.Arrived(seq, "") {
		t.Fatal("a report after the presenter left advanced")
	}
	if clock.Armed() != 1 {
		t.Fatalf("%d timers armed after the cancel; only the quiet spell should be", clock.Armed())
	}
	if _, err := s.Strand(button); !errors.Is(err, ErrNoPresenter) {
		t.Fatalf("a strand after the presenter left: %v", err)
	}
	s.Retract()
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("retracting a strand that left with its presenter sent %q", got)
	}

	s.Presenter(true)
	next := target(t, s)
	if next != seq+1 {
		t.Fatalf("a new presenter's first target is seq %d, want %d", next, seq+1)
	}
	s.Presenter(true)
	if _, cancelled := o.take(); !slices.Equal(cancelled, []string{note(next, CancelPresenterLost)}) {
		t.Fatalf("a presenter starting over reported %q", cancelled)
	}
}

func TestRetractRewindsTheStrandOnScreen(t *testing.T) {
	s, _, rec, o := presenting(t)
	seq := target(t, s)
	s.Arrived(seq, "")
	rec.take()
	o.take()
	s.Retract()
	if got := rec.take(); !slices.Equal(got, encode(t, &feature.StrandRetractFrame{Seq: seq})) {
		t.Fatalf("retract sent %q", got)
	}
	s.Retract()
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("a second retract sent %q", got)
	}
	if arrived, cancelled := o.take(); len(arrived)+len(cancelled) != 0 {
		t.Fatalf("retracting a landed strand reported %q %q", arrived, cancelled)
	}

	mid := target(t, s)
	rec.take()
	s.Retract()
	if got := rec.take(); !slices.Equal(got, encode(t, &feature.StrandRetractFrame{Seq: mid})) {
		t.Fatalf("retract mid-reach sent %q", got)
	}
	if _, cancelled := o.take(); !slices.Equal(cancelled, []string{note(mid, CancelRetracted)}) {
		t.Fatalf("retract mid-reach reported %q", cancelled)
	}
	if s.Arrived(mid, "") {
		t.Fatal("a report for a retracted target advanced")
	}
}

func TestASkippedTargetAdvancesAndLeavesNothingToRetract(t *testing.T) {
	s, _, rec, o := presenting(t)
	seq := target(t, s)
	rec.take()
	if !s.Arrived(seq, feature.SkipCovered) {
		t.Fatal("a skipped report did not resolve the target")
	}
	if arrived, _ := o.take(); !slices.Equal(arrived, []string{note(seq, feature.SkipCovered)}) {
		t.Fatalf("a skipped report advanced as %q", arrived)
	}
	s.Retract()
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("retracting a skipped strand sent %q", got)
	}
}

func TestTurningOniOffOrStoppingCancels(t *testing.T) {
	s, clock, rec, o := presenting(t)
	seq := target(t, s)
	off := on()
	off.Master = false
	s.Settings(off)
	if _, cancelled := o.take(); !slices.Equal(cancelled, []string{note(seq, CancelOff)}) {
		t.Fatalf("turning Oni off reported %q", cancelled)
	}
	if clock.Armed() != 0 {
		t.Fatalf("%d timers armed while Oni is off", clock.Armed())
	}
	s.Settings(on())
	rec.take()
	s.Retract()
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("Oni off took its strand, yet retract sent %q", got)
	}
	again := target(t, s)
	s.Close()
	if _, cancelled := o.take(); !slices.Equal(cancelled, []string{note(again, CancelClosed)}) {
		t.Fatalf("stopping reported %q", cancelled)
	}
	if clock.Armed() != 0 || s.Arrived(again, "") {
		t.Fatal("a stopped presence still waits")
	}
}

func TestOutcomesMayCallBackIn(t *testing.T) {
	s, _, rec, _ := presenting(t)
	next := make(chan uint64, 1)
	s.OnArrive(func(uint64, string) {
		seq, err := s.Strand(button)
		if err != nil {
			t.Error(err)
		}
		next <- seq
	})
	first := target(t, s)
	done := make(chan bool)
	go func() { done <- s.Arrived(first, "") }()
	select {
	case ok := <-done:
		if !ok {
			t.Fatal("the pending target did not land")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("an outcome that called back in deadlocked")
	}
	if got := <-next; got != first+1 {
		t.Fatalf("the target sent from OnArrive is seq %d", got)
	}
	if got := kinds(decodeAll(t, rec.take())); !slices.Equal(got, []string{feature.KindStrandTarget, feature.KindStrandTarget}) {
		t.Fatalf("sent %q", got)
	}
}

func TestSoundsReachTheMotionFrame(t *testing.T) {
	s, _, rec := started(t)
	if m := s.Snapshot()[2].(*feature.MotionFrame); m.Sounds {
		t.Fatal("sounds are on by default")
	}
	loud := on()
	loud.Sounds = true
	s.Settings(loud)
	want := *s.motionFrame()
	if !want.Sounds {
		t.Fatal("the setting did not reach the motion frame")
	}
	if got := rec.take(); !slices.Equal(got, encode(t, &want)) {
		t.Fatalf("turning sounds on sent %q", got)
	}
}

func TestVoicePhasesReachTheRendererOnlyWhenTheyChange(t *testing.T) {
	s, clock, rec := started(t)
	armed := clock.Armed()
	if err := s.Voice(feature.VoiceFrame{Phase: VoiceIdle}); err != nil {
		t.Fatal(err)
	}
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("an unchanged idle phase sent %q", got)
	}
	listening := feature.VoiceFrame{Phase: "listening"}
	if err := s.Voice(listening); err != nil {
		t.Fatal(err)
	}
	if got := rec.take(); !slices.Equal(got, encode(t, &listening, &feature.StateFrame{State: "listening"})) {
		t.Fatalf("listening sent %q", got)
	}
	if armed = clock.Armed(); armed != 0 {
		t.Fatal("the quiet spell runs while listening")
	}
	if v := s.Snapshot()[4].(*feature.VoiceFrame); *v != listening {
		t.Fatalf("the snapshot says %+v while listening", v)
	}
	if err := s.Voice(feature.VoiceFrame{Phase: "listening", Text: "stray"}); err == nil {
		t.Fatal("an invalid voice frame was taken")
	}
	if err := s.Voice(feature.VoiceFrame{Phase: "humming"}); err == nil {
		t.Fatal("an unknown phase was taken")
	}
	if got := rec.take(); len(got) != 0 || clock.Armed() != armed {
		t.Fatalf("refused voice frames sent %q or armed a timer", got)
	}
	off := on()
	off.Master = false
	s.Settings(off)
	rec.take()
	if err := s.Voice(feature.VoiceFrame{Phase: VoiceIdle}); err != nil {
		t.Fatal(err)
	}
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("Oni off still sent %q", got)
	}
	s.Settings(on())
	frames := decodeAll(t, rec.take())
	if v, ok := frames[len(frames)-1].(*feature.VoiceFrame); !ok || v.Phase != VoiceIdle {
		t.Fatalf("turning Oni on ended with %+v", frames[len(frames)-1])
	}
}
