package motion

import (
	"math"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/spec"
)

const frame = time.Second / 60

func mustSpec(t testing.TB) *spec.OniSpec {
	t.Helper()
	s, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func started(t testing.TB) (*Engine, *spec.OniSpec) {
	t.Helper()
	s := mustSpec(t)
	e := New(s)
	if tk := e.Step(0, Input{}); !tk.Redraw {
		t.Fatal("the first frame was not committed")
	}
	return e, s
}

func steps(e *Engine, n int, in Input) Tick {
	var tk Tick
	for range n {
		tk = e.Step(frame, in)
	}
	return tk
}

func TestSleepCommitsOnceThenStops(t *testing.T) {
	e, s := started(t)
	e.Play("sleep")
	e.SetState("sleeping", "")
	var commit Tick
	var at time.Duration
	for at < 3*time.Second {
		commit = e.Step(frame, Input{})
		at += frame
		if commit.Stop {
			break
		}
		if commit.FPS != 20 {
			t.Fatalf("commit window ran at %d fps, want the asleep tier", commit.FPS)
		}
	}
	want := time.Duration(s.Sleep.CommitS * float64(time.Second))
	if !commit.Stop || at < want || at > want+frame {
		t.Fatalf("stopped=%v at %v, want a stop within one frame of %v", commit.Stop, at, want)
	}
	if !commit.Redraw || commit.Next != 0 || commit.FPS != 0 {
		t.Fatalf("the sleep commit must paint once and arm nothing: %+v", commit)
	}
	p := commit.Pose
	if p.State != "sleeping" || p.Gesture != "" || p.Flat != 1 || p.Lap.Progress != 1 || p.Lap.Glow != 0 {
		t.Fatalf("committed pose is not the settled sleeping pose: %+v", p)
	}
	if e.Activity() != "asleep" {
		t.Fatalf("activity %q while asleep", e.Activity())
	}
	for i := range 30 {
		tk := e.Step(time.Second, Input{Pointer: &spec.Point{X: float64(i), Y: 3}, Mic: 0.4})
		if !tk.Stop || tk.Redraw || tk.Pose != p {
			t.Fatalf("asleep engine woke for pointer/mic input at step %d: %+v", i, tk)
		}
	}

	e.Play("wake")
	e.SetState("idle", "")
	tk := e.Step(frame, Input{})
	if tk.Stop || !tk.Redraw || tk.FPS != 60 || tk.Pose.Gesture != "wake" {
		t.Fatalf("wake did not resume at the motion tier: %+v", tk)
	}
}

func TestSleepWithoutCommitStopKeepsAsleepTier(t *testing.T) {
	s := mustSpec(t)
	s.Frames.AsleepStopsAfterCommit = false
	e := New(s)
	e.Step(0, Input{})
	e.SetState("sleeping", "")
	tk := steps(e, 150, Input{})
	if tk.Stop || tk.FPS != 20 {
		t.Fatalf("with the stop disabled the asleep loop must keep the asleep tier: %+v", tk)
	}
}

func TestReducedMotionPaintsOneFramePerEvent(t *testing.T) {
	e, s := started(t)
	commits, events := 0, 0
	tick := func(in Input) Tick {
		t.Helper()
		in.Reduced = true
		tk := e.Step(frame, in)
		if !tk.Stop || tk.Next != 0 {
			t.Fatalf("reduced motion armed a timer: %+v", tk)
		}
		if tk.Redraw {
			commits++
		}
		return tk
	}
	idle := func(n int) {
		for i := range n {
			tick(Input{Pointer: &spec.Point{X: float64(i), Y: 1}, Mic: float64(i) / 100, Speech: 0.5})
		}
	}

	events++
	if tk := tick(Input{}); !tk.Redraw || tk.Pose.HoverY != 0 {
		t.Fatalf("switching to reduced motion must land one static frame: %+v", tk)
	}
	idle(20)

	e.SetState("listening", "")
	events++
	tk := tick(Input{})
	want, _ := s.State("listening")
	if !tk.Redraw || tk.Pose.Scale != want.Scale || tk.Pose.Loose != 1 || tk.Pose.HoverY != 0 {
		t.Fatalf("listening did not land on its static pose: %+v", tk.Pose)
	}
	if l := tk.Pose.Lap; l.To != "listening" || l.Progress != 1 || l.Glow != s.Colour.Glow.Strength*s.Colour.Glow.High {
		t.Fatalf("listening accent is not static: %+v", l)
	}
	idle(20)

	for _, g := range []string{"poke", "done"} {
		e.Play(g)
		events++
		tk = tick(Input{})
		if tk.Pose.Gesture != "" || tk.Pose.LeanX != 0 || tk.Pose.Lap.To != "listening" {
			t.Fatalf("%s moved the reduced pose: %+v", g, tk.Pose)
		}
		idle(10)
	}

	e.SetState("sleeping", "")
	events++
	if tk = tick(Input{}); tk.Pose.Flat != 1 {
		t.Fatalf("sleeping static pose: %+v", tk.Pose)
	}
	idle(10)
	if commits != events {
		t.Fatalf("%d commits for %d events", commits, events)
	}
}

func TestStepCapsDt(t *testing.T) {
	s := mustSpec(t)
	run := func(dt time.Duration) Tick {
		e := New(s)
		e.Step(0, Input{})
		e.SetState("listening", "")
		e.Play("acknowledge")
		return e.Step(dt, Input{})
	}
	capped := run(time.Duration(s.Frames.DtCapMs) * time.Millisecond)
	for _, dt := range []time.Duration{51 * time.Millisecond, time.Second, 10 * time.Minute} {
		if got := run(dt); got != capped {
			t.Errorf("dt %v not capped:\n got %+v\nwant %+v", dt, got.Pose, capped.Pose)
		}
	}
	if run(40*time.Millisecond).Pose == capped.Pose {
		t.Error("a 40 ms frame advanced like the cap")
	}
	if a, b := run(0), run(-time.Second); a != b {
		t.Errorf("a negative dt moved the pose differently from zero: %+v vs %+v", a.Pose, b.Pose)
	}
}

func TestSettledStopsAfterThirtySeconds(t *testing.T) {
	e, s := started(t)
	step := time.Second / 30
	var at time.Duration
	var tk Tick
	for at <= 31*time.Second {
		tk = e.Step(step, Input{})
		at += step
		if tk.Stop {
			break
		}
		if tk.FPS != 30 {
			t.Fatalf("settled idle ran at %d fps, want the rest tier", tk.FPS)
		}
	}
	settle := time.Duration(s.Frames.SettledStopMs) * time.Millisecond
	if !tk.Stop || at < settle || at > settle+step {
		t.Fatalf("stopped=%v at %v, want within one frame of %v", tk.Stop, at, settle)
	}
	frozen := tk.Pose
	for range 5 {
		if tk = e.Step(time.Hour, Input{}); !tk.Stop || tk.Redraw || tk.Pose != frozen {
			t.Fatalf("a settled engine kept working: %+v", tk)
		}
	}
	if tk = e.Step(step, Input{Pointer: &spec.Point{X: 5, Y: 5}}); tk.Stop || tk.FPS == 0 {
		t.Fatalf("pointer input did not resume a settled engine: %+v", tk)
	}
}

func TestFrameTiers(t *testing.T) {
	e, _ := started(t)
	if tk := e.Step(frame, Input{}); tk.FPS != 30 || e.Activity() != "idle" {
		t.Fatalf("idle: %d fps, activity %q", tk.FPS, e.Activity())
	}
	e.SetState("listening", "")
	if tk := e.Step(frame, Input{}); tk.FPS != 60 || e.Activity() != "motion" {
		t.Fatalf("state change: %d fps, activity %q", tk.FPS, e.Activity())
	}
	if tk := steps(e, 180, Input{}); tk.FPS != 60 || e.Activity() != "breathing" {
		t.Fatalf("settled listening: %d fps, activity %q", tk.FPS, e.Activity())
	}
	saver := Input{BatterySaver: true}
	if tk := e.Step(frame, saver); tk.FPS != 30 {
		t.Fatalf("battery saver breathing: %d fps", tk.FPS)
	}
	e.Play("nudge")
	if tk := e.Step(frame, saver); tk.FPS != 60 {
		t.Fatalf("battery saver gesture: %d fps", tk.FPS)
	}
	if tk := e.Step(frame, Input{BatterySaver: true, Drag: &Drag{}}); tk.FPS != 60 {
		t.Fatalf("drag: %d fps", tk.FPS)
	}
	e.SetState("sleeping", "")
	if tk := e.Step(frame, saver); tk.FPS != 20 {
		t.Fatalf("sleeping: %d fps", tk.FPS)
	}
}

func TestGestureHandoffBlendsResidual(t *testing.T) {
	s := mustSpec(t)
	a, b := New(s), New(s)
	a.Step(0, Input{})
	b.Step(0, Input{})
	a.Play("poke")
	before := steps(a, 4, Input{})
	steps(b, 4, Input{})

	a.Play("done")
	b.Play("done")
	ta, tb := a.Step(frame, Input{}), b.Step(frame, Input{})
	if ta.Pose.Gesture != "done" {
		t.Fatalf("replacement gesture not running: %q", ta.Pose.Gesture)
	}
	jump := math.Abs(tb.Pose.LeanX - before.Pose.LeanX)
	if jump < 0.05 {
		t.Fatalf("poke lean %v too small for the handoff to matter", before.Pose.LeanX)
	}
	if got := math.Abs(ta.Pose.LeanX - before.Pose.LeanX); got > jump/2 {
		t.Fatalf("handoff jumped %v of a %v cut", got, jump)
	}
	handoff := time.Duration(s.Gestures.HandoffMs * float64(time.Millisecond))
	for at := frame; at+frame < handoff; at += frame {
		ta, tb = a.Step(frame, Input{}), b.Step(frame, Input{})
	}
	if ta.Pose == tb.Pose {
		t.Fatal("residual vanished before the handoff ended")
	}
	ta, tb = a.Step(frame, Input{}), b.Step(frame, Input{})
	if ta.Pose != tb.Pose {
		t.Fatalf("residual outlived the handoff:\n%+v\n%+v", ta.Pose, tb.Pose)
	}
}

func TestEveryGestureReturnsToBase(t *testing.T) {
	s := mustSpec(t)
	for _, id := range spec.GestureIDs {
		e := New(s)
		base := e.Step(0, Input{}).Pose
		if !e.Play(id) {
			t.Fatalf("%s rejected", id)
		}
		g, _ := s.Gesture(id)
		peak := 0.0
		n := int(math.Ceil(g.DurationS * 60))
		var tk Tick
		for range n {
			tk = e.Step(frame, Input{})
			peak = max(peak, math.Abs(tk.Pose.Squash-base.Squash)+math.Abs(tk.Pose.LeanX)+math.Abs(tk.Pose.LeanY)+math.Abs(tk.Pose.Bright-1)+math.Abs(tk.Pose.Unravel)+math.Abs(tk.Pose.Straighten))
		}
		if peak < 0.02 {
			t.Errorf("%s barely moved (peak %v)", id, peak)
		}
		tk = steps(e, 60, Input{})
		if tk.Pose.Gesture != "" || tk.Pose.Squash != 0 || tk.Pose.LeanX != 0 || tk.Pose.LeanY != 0 || tk.Pose.Unravel != 0 || tk.Pose.Straighten != 0 {
			t.Errorf("%s did not return to base: %+v", id, tk.Pose)
		}
	}
	e := New(s)
	if e.Play("reach") || e.Play("") {
		t.Error("a non-gesture id played")
	}
}

func TestPokeMirrorsTowardPointerSide(t *testing.T) {
	s := mustSpec(t)
	lean := func(x float64) float64 {
		e := New(s)
		in := Input{Pointer: &spec.Point{X: x, Y: 0}}
		e.Step(0, in)
		e.Play("poke")
		return steps(e, 4, in).Pose.LeanX
	}
	left, right := lean(-20), lean(20)
	if !(left > 0.05 && right < -0.05) {
		t.Fatalf("poke from the left leans %v, from the right %v", left, right)
	}
}

func TestAccentLapAndBeats(t *testing.T) {
	e, s := started(t)
	e.SetState("listening", "")
	tk := e.Step(frame, Input{})
	if l := tk.Pose.Lap; l.From != "idle" || l.To != "listening" || l.Progress <= 0 || l.Progress >= 1 {
		t.Fatalf("lap did not start: %+v", l)
	}
	tk = steps(e, int(s.Colour.Lap.DurationS*60)+1, Input{})
	if l := tk.Pose.Lap; l.Progress != 1 || l.Glow <= 0 {
		t.Fatalf("lap did not finish lit: %+v", l)
	}
	e.Play("done")
	tk = e.Step(frame, Input{})
	if tk.Pose.Lap.To != "done" {
		t.Fatalf("done beat colour missing: %+v", tk.Pose.Lap)
	}
	tk = steps(e, int(s.Colour.Beats.Done.HoldS*60)+2, Input{})
	if tk.Pose.Lap.To != "listening" || tk.Pose.Lap.From != "done" {
		t.Fatalf("beat did not lap back to the state colour: %+v", tk.Pose.Lap)
	}
	e.SetState("idle", "")
	tk = steps(e, int(max(s.Colour.Lap.LeaveS, s.Colour.Lap.DurationS)*60)+2, Input{})
	if l := tk.Pose.Lap; l.To != "idle" || l.Glow != 0 {
		t.Fatalf("glow did not leave on idle: %+v", l)
	}
}

func TestHoverFloatIsQuantised(t *testing.T) {
	e, s := started(t)
	q := s.Motion.HoverFloat.QuantizePx
	seen := map[float64]bool{}
	for range 300 {
		h := e.Step(frame, Input{}).Pose.HoverY
		if r := math.Mod(math.Abs(h), q); r != 0 {
			t.Fatalf("hover %v is not a multiple of %v", h, q)
		}
		seen[h] = true
	}
	if len(seen) < 5 {
		t.Fatalf("hover barely floated: %v", seen)
	}
}

func TestResetReturnsToFreshEngine(t *testing.T) {
	s := mustSpec(t)
	e := New(s)
	fresh := e.Step(0, Input{})
	e.SetState("thinking", "")
	e.Play("gulp")
	steps(e, 40, Input{Speech: 0.4})
	e.Reset()
	if got := e.Step(0, Input{}); got != fresh {
		t.Fatalf("reset pose %+v, fresh %+v", got.Pose, fresh.Pose)
	}
}
