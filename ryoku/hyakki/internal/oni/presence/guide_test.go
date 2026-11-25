package presence

import (
	"errors"
	"slices"
	"testing"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/spec"
)

// guideAt is a valid guide frame in phase.
func guideAt(seq uint64, phase string) feature.GuideFrame {
	f := feature.GuideFrame{Seq: seq, Phase: phase}
	switch phase {
	case feature.GuideReading:
		f.Step, f.Kicker, f.Actions = 1, "Step 1", []string{feature.GuideActionStop}
	case feature.GuidePointing, feature.GuideWaiting:
		f.Step, f.Kicker, f.Text, f.Target = 1, "Step 1", "Click Save to keep the notes.", 3
		f.Plan = &feature.GuidePlan{Current: "Save the notes", Next: []string{"Name the file"}}
		f.Actions = []string{feature.GuideActionNext, feature.GuideActionStop, feature.GuideActionCircle}
	case feature.GuideExplaining:
		f.Step, f.Text = 1, "Open the File menu first."
		f.Actions = []string{feature.GuideActionNext, feature.GuideActionStop}
	case feature.GuideKeeping:
		f.Step, f.Actions = 2, []string{feature.GuideActionKeep, feature.GuideActionSkip}
	case feature.GuideDone:
		f.Step, f.Text = 2, "Done, the notes are saved."
	case feature.GuideNudging:
		f.Text, f.Actions = "Save the notes as last time?", []string{feature.GuideActionNext, feature.GuideActionSkip}
	}
	return f
}

func guide(t *testing.T, s *Service, f feature.GuideFrame) {
	t.Helper()
	if err := s.Guide(f); err != nil {
		t.Fatalf("guide %s: %v", f.Phase, err)
	}
}

var oldOrder = []string{feature.KindState, feature.KindCharacter, feature.KindMotion, feature.KindPlacement, feature.KindVoice}

// Each guide phase derives the state the spec names, and the guide frame
// goes out ahead of the state it sets.
func TestGuidePhasesDeriveTheSpecsStates(t *testing.T) {
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		feature.GuideReading:    "reading",
		feature.GuidePointing:   "ready",
		feature.GuideWaiting:    "ready",
		feature.GuideExplaining: "ready",
		feature.GuideDone:       "ready",
		feature.GuideNudging:    "ready",
		feature.GuideKeeping:    "thinking",
		feature.GuideWinding:    "idle",
		feature.GuideIdle:       "idle",
	}
	s, _, rec := started(t)
	state := "idle"
	for i, phase := range []string{
		feature.GuideReading, feature.GuidePointing, feature.GuideWaiting, feature.GuideExplaining, feature.GuideReading,
		feature.GuideKeeping, feature.GuideDone, feature.GuideWinding, feature.GuideNudging, feature.GuideIdle,
	} {
		got, ok := sp.GuideState(phase)
		if !ok || (got != want[phase] && !(got == spec.NoState && want[phase] == "idle")) {
			t.Fatalf("the spec maps %s to %q, this test expects %q", phase, got, want[phase])
		}
		f := guideAt(uint64(i+1), phase)
		guide(t, s, f)
		frames := []feature.Payload{&f}
		if want[phase] != state {
			state = want[phase]
			frames = append(frames, &feature.StateFrame{State: state})
		}
		if sent := rec.take(); !slices.Equal(sent, encode(t, frames...)) {
			t.Fatalf("%s sent %q, want %q", phase, sent, encode(t, frames...))
		}
		if st := s.Snapshot()[0].(*feature.StateFrame); st.State != want[phase] {
			t.Fatalf("after %s the state is %+v, want %s", phase, st, want[phase])
		}
	}
}

// A guide wakes a sleeping Oni and holds off sleep while it runs; a settled
// guide, winding or ended, lets Oni rest again.
func TestAGuideWakesASleepingOniAndASettledOneLetsItRest(t *testing.T) {
	s, clock, rec := started(t)
	clock.Advance(SleepAfter)
	rec.take()
	if st := s.Snapshot()[0].(*feature.StateFrame); st.State != "sleeping" {
		t.Fatalf("Oni did not fall asleep: %+v", st)
	}
	reading := guideAt(1, feature.GuideReading)
	guide(t, s, reading)
	want := encode(t, &reading, &feature.GestureFrame{Seq: 2, ID: "wake"}, &feature.StateFrame{State: "reading"})
	if got := rec.take(); !slices.Equal(got, want) {
		t.Fatalf("a guide on a sleeping Oni sent %q, want %q", got, want)
	}
	if clock.Armed() != 0 {
		t.Fatalf("%d timers armed while a guide reads", clock.Armed())
	}
	guide(t, s, guideAt(2, feature.GuideWaiting))
	rec.take()
	clock.Advance(10 * SleepAfter)
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("Oni fell asleep while a guide waits on the user: %q", got)
	}
	guide(t, s, guideAt(3, feature.GuideWinding))
	if clock.Armed() != 1 {
		t.Fatalf("%d timers armed once the guide winds; only the quiet spell should be", clock.Armed())
	}
	guide(t, s, guideAt(4, feature.GuideIdle))
	rec.take()
	if clock.Armed() != 1 {
		t.Fatalf("%d timers armed once the guide ended; only the quiet spell should be", clock.Armed())
	}
	clock.Advance(SleepAfter)
	if st := s.Snapshot()[0].(*feature.StateFrame); st.State != "sleeping" {
		t.Fatalf("a settled guide kept Oni awake: %+v", st)
	}
}

// A guide held in one phase arms no timer, and the same frame again sends
// nothing; changing the caller's copy afterwards changes nothing either.
func TestAHeldGuideArmsNoTimerAndCommitsNothing(t *testing.T) {
	s, clock, rec := started(t)
	waiting := guideAt(1, feature.GuideWaiting)
	guide(t, s, waiting)
	rec.take()
	if clock.Armed() != 0 {
		t.Fatalf("%d timers armed while a guide waits", clock.Armed())
	}
	guide(t, s, guideAt(1, feature.GuideWaiting))
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("the same guide frame again sent %q", got)
	}
	if clock.Armed() != 0 {
		t.Fatalf("%d timers armed by a repeated guide frame", clock.Armed())
	}
	waiting.Plan.Next[0] = "Something else"
	waiting.Actions[0] = feature.GuideActionSkip
	snap := s.Snapshot()
	if got := snap[len(snap)-1].(*feature.GuideFrame); got.Plan.Next[0] != "Name the file" || got.Actions[0] != feature.GuideActionNext {
		t.Fatalf("the caller's copy reached the held frame: %+v", got)
	}
	got := snap[len(snap)-1].(*feature.GuideFrame)
	got.Plan.Current = "changed"
	if again := s.Snapshot(); again[len(again)-1].(*feature.GuideFrame).Plan.Current != "Save the notes" {
		t.Fatal("a snapshot's frame aliases the held one")
	}
	if got := rec.take(); len(got) != 0 || clock.Armed() != 0 {
		t.Fatalf("reading the held guide sent %q and armed %d timers", got, clock.Armed())
	}

	// idle with no guide running is nothing to end
	guide(t, s, guideAt(2, feature.GuideIdle))
	rec.take()
	guide(t, s, guideAt(3, feature.GuideIdle))
	if got := rec.take(); len(got) != 0 || clock.Armed() != 1 {
		t.Fatalf("ending no guide sent %q and armed %d timers", got, clock.Armed())
	}
}

func TestInvalidGuideFramesChangeNothing(t *testing.T) {
	s, clock, rec := started(t)
	bad := map[string]feature.GuideFrame{
		"no seq":            {Phase: feature.GuideReading, Step: 1},
		"unknown phase":     {Seq: 1, Phase: "dancing", Step: 1},
		"idle with a step":  {Seq: 1, Phase: feature.GuideIdle, Step: 1},
		"reading with text": {Seq: 1, Phase: feature.GuideReading, Step: 1, Text: "Click Save."},
		"pointing at none":  {Seq: 1, Phase: feature.GuidePointing, Step: 1, Text: "Click Save."},
		"too many steps":    {Seq: 1, Phase: feature.GuideReading, Step: feature.MaxGuideSteps() + 1},
		"action not offered": {Seq: 1, Phase: feature.GuideReading, Step: 1,
			Actions: []string{feature.GuideActionKeep}},
	}
	for name, f := range bad {
		if err := s.Guide(f); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("invalid guide frames sent %q", got)
	}
	if got := kinds(s.Snapshot()); !slices.Equal(got, oldOrder) || clock.Armed() != 1 {
		t.Fatalf("invalid guide frames left snapshot %q and %d timers", got, clock.Armed())
	}

	off := on()
	off.Master = false
	s.Settings(off)
	rec.take()
	if err := s.Guide(guideAt(1, feature.GuideReading)); !errors.Is(err, ErrOff) {
		t.Fatalf("a guide while Oni is off: %v", err)
	}
	if err := s.Guide(guideAt(2, feature.GuideIdle)); err != nil {
		t.Fatalf("ending a guide while Oni is off: %v", err)
	}
	if got := rec.take(); len(got) != 0 || clock.Armed() != 0 {
		t.Fatalf("a guide while Oni is off sent %q and armed %d timers", got, clock.Armed())
	}
	s.Close()
	if err := s.Guide(guideAt(3, feature.GuideReading)); !errors.Is(err, ErrClosed) {
		t.Fatalf("a guide after Close: %v", err)
	}
}

// The snapshot carries the guide frame, last, only while a guide runs, so a
// presenter that joins mid-guide gets the current frame; with no guide the
// snapshot is what it was before guides existed.
func TestTheSnapshotCarriesTheGuideOnlyWhileOneRuns(t *testing.T) {
	s, _, rec := started(t)
	if got := kinds(s.Snapshot()); !slices.Equal(got, oldOrder) {
		t.Fatalf("snapshot with no guide = %q, want %q", got, oldOrder)
	}
	guide(t, s, guideAt(1, feature.GuidePointing))
	waiting := guideAt(2, feature.GuideWaiting)
	guide(t, s, waiting)
	rec.take()
	var joined []feature.Payload
	s.Open(false, func(snap []feature.Payload) { joined = snap })
	if got := kinds(joined); !slices.Equal(got, append(slices.Clone(oldOrder), feature.KindGuide)) {
		t.Fatalf("snapshot mid-guide = %q", got)
	}
	if got, want := encode(t, joined[len(joined)-1]), encode(t, &waiting); !slices.Equal(got, want) {
		t.Fatalf("a presenter joining mid-guide got %q, want %q", got, want)
	}
	if st := joined[0].(*feature.StateFrame); st.State != "ready" {
		t.Fatalf("snapshot state mid-guide = %+v", st)
	}

	winding := guideAt(3, feature.GuideWinding)
	guide(t, s, winding)
	if snap := s.Snapshot(); !slices.Equal(encode(t, snap[len(snap)-1]), encode(t, &winding)) {
		t.Fatal("a winding guide is still running but left the snapshot")
	}
	rec.take()
	idle := guideAt(4, feature.GuideIdle)
	guide(t, s, idle)
	if got := rec.take(); !slices.Equal(got, encode(t, &idle)) {
		t.Fatalf("ending the guide sent %q; the renderer must hear it ended", got)
	}
	if got := kinds(s.Snapshot()); !slices.Equal(got, oldOrder) {
		t.Fatalf("snapshot after the guide = %q, want %q", got, oldOrder)
	}

	// turning Oni off drops a running guide; turning it on shows none
	guide(t, s, guideAt(5, feature.GuideReading))
	off := on()
	off.Master = false
	s.Settings(off)
	s.Settings(on())
	if got := kinds(s.Snapshot()); !slices.Equal(got, oldOrder) {
		t.Fatalf("a guide outlived Oni going off: %q", got)
	}
}
