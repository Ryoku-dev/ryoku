package strand

import (
	"math"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/spec"
)

const step = 10 * time.Millisecond

func mustSpec(t testing.TB) *spec.OniSpec {
	t.Helper()
	s, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

var screen = Rect{X: 0, Y: 0, W: 1920, H: 1080}

func baseEnv() Env {
	return Env{Anchor: spec.Point{X: 1700, Y: 900}, Centre: spec.Point{X: 960, Y: 540}, Screen: screen}
}

var control = Rect{X: 300, Y: 200, W: 120, H: 40}

func near(a, b spec.Point, tol float64) bool { return dist(a, b) <= tol }

// started returns an engine that has settled home and been sent t.
func started(t *testing.T, s *spec.OniSpec, env Env, tg Target) *Engine {
	t.Helper()
	e := New(s)
	if tk := e.Step(0, env); !tk.Stop {
		t.Fatal("a fresh engine does not stop at home")
	}
	e.Target(tg)
	return e
}

func TestReachTipFollowsBowedPathAndArrivesOnce(t *testing.T) {
	s := mustSpec(t)
	env := baseEnv()
	e := started(t, s, env, Target{Seq: 1, Box: control, Quiet: true})
	land := tipAnchor(s, control, env.Anchor)
	want := spec.BowedPath(env.Anchor, land, env.Centre)
	own := bowed(s, env.Anchor, land, env.Centre, s.Strand.BowedPath.Segments)
	for i := range want {
		if want[i] != own[i] {
			t.Fatalf("bowed path point %d differs from spec.BowedPath: %v vs %v", i, own[i], want[i])
		}
	}
	tk := e.Step(0, env)
	if tk.Frame.Phase != PhaseReach || tk.Frame.U != 0 || !near(tk.Frame.Tip, want[0], 1e-9) {
		t.Fatalf("t=0: phase %s u %v tip %v", tk.Frame.Phase, tk.Frame.U, tk.Frame.Tip)
	}
	arrived := 0
	for i := 1; i <= 300; i++ {
		tk = e.Step(step, env)
		if tk.Frame.Arrived {
			arrived++
			if i != 92 {
				t.Fatalf("arrived at step %d, want 92 (0.74 s reach + 0.18 s tap)", i)
			}
		}
		at := float64(i) / 100
		switch i {
		case 30:
			u := spec.Reach(at)
			if math.Abs(tk.Frame.U-u) > 1e-9 {
				t.Fatalf("t=0.3: u %v, want %v", tk.Frame.U, u)
			}
			if p := atIndex(want, u); !near(tk.Frame.Tip, p, 1e-9) {
				t.Fatalf("t=0.3: tip %v, want %v on the bowed path", tk.Frame.Tip, p)
			}
		case 74:
			if tk.Frame.Phase != PhaseTap || !near(tk.Frame.Tip, want[len(want)-1], 1e-9) {
				t.Fatalf("t=0.74: phase %s tip %v, want tap at %v", tk.Frame.Phase, tk.Frame.Tip, want[len(want)-1])
			}
		case 92:
			if tk.Frame.Phase != PhaseHold || !tk.Frame.Arrived || !near(tk.Frame.Tip, want[len(want)-1], 1e-9) {
				t.Fatalf("t=0.92: phase %s arrived %v tip %v", tk.Frame.Phase, tk.Frame.Arrived, tk.Frame.Tip)
			}
		}
		if i < 92 && tk.FPS != 60 {
			t.Fatalf("step %d: reach runs at %d fps, want the motion tier", i, tk.FPS)
		}
	}
	if arrived != 1 {
		t.Fatalf("arrived %d times, want exactly once", arrived)
	}
}

func atIndex(pts []spec.Point, u float64) spec.Point { return at(pts, u*float64(len(pts)-1)) }

func TestTapBobPeaksAtMidTap(t *testing.T) {
	s := mustSpec(t)
	env := baseEnv()
	e := started(t, s, env, Target{Seq: 1, Box: control, Quiet: true})
	for range 74 {
		e.Step(step, env)
	}
	end := tipAnchor(s, control, env.Anchor)
	peak := 0.0
	for i := 1; i <= 18; i++ {
		tk := e.Step(step, env)
		d := dist(tk.Frame.Tip, end)
		peak = math.Max(peak, d)
		if i == 9 {
			if math.Abs(d-s.Strand.Tap.BobPx) > 1e-9 {
				t.Fatalf("bob at 0.09 s is %v px, want %v", d, s.Strand.Tap.BobPx)
			}
			if !(dist(tk.Frame.Tip, control.centre()) < dist(end, control.centre())) {
				t.Fatal("the tap bobs away from the target")
			}
		}
	}
	if peak > s.Strand.Tap.BobPx+1e-9 {
		t.Fatalf("bob peaked at %v px, above %v", peak, s.Strand.Tap.BobPx)
	}
}

func TestSwingNeverDropsBelowRetargetFloor(t *testing.T) {
	s := mustSpec(t)
	env := baseEnv()
	other := Rect{X: 1300, Y: 120, W: 90, H: 30}
	for _, after := range []int{12, 50, 120} {
		e := started(t, s, env, Target{Seq: 1, Box: control, Quiet: true})
		var last Tick
		for range after {
			last = e.Step(step, env)
		}
		uOld := last.Frame.U
		if last.Frame.Phase == PhaseHold || last.Frame.Phase == PhaseTap {
			uOld = 1
		}
		e.Target(Target{Seq: 2, Box: other, Quiet: true})
		floor := math.Max(s.Strand.Retarget.ReachFrom, uOld)
		arrived := 0
		for i := 1; i <= 60; i++ {
			tk := e.Step(step, env)
			if tk.Frame.Arrived {
				arrived++
				if tk.Frame.Seq != 2 {
					t.Fatalf("arrival for seq %d after the retarget", tk.Frame.Seq)
				}
			}
			if i < 30 {
				if tk.Frame.Phase != PhaseSwing {
					t.Fatalf("after %d: step %d is %s, want swing for 300 ms", after, i, tk.Frame.Phase)
				}
				if tk.Frame.U < floor-1e-12 || len(tk.Frame.Strokes) == 0 {
					t.Fatalf("after %d: swing u %v below the floor %v (or went home)", after, tk.Frame.U, floor)
				}
			}
			if i == 30 && tk.Frame.Phase != PhaseTap {
				t.Fatalf("after %d: swing did not complete in 300 ms: %s", after, tk.Frame.Phase)
			}
		}
		if arrived != 1 {
			t.Fatalf("after %d: seq 2 arrived %d times", after, arrived)
		}
		if tk := e.Step(step, env); !near(tk.Frame.Tip, tipAnchor(s, other, env.Anchor), 1e-9) {
			t.Fatalf("after %d: swing landed at %v", after, tk.Frame.Tip)
		}
	}
}

func TestRewindEndsHomeAndStops(t *testing.T) {
	s := mustSpec(t)
	env := baseEnv()
	e := started(t, s, env, Target{Seq: 7, Box: control})
	for range 120 {
		e.Step(step, env)
	}
	e.Retract(6) // stale
	if tk := e.Step(step, env); tk.Frame.Phase != PhaseHold {
		t.Fatalf("a stale retract moved the thread: %s", tk.Frame.Phase)
	}
	e.Retract(7)
	prev := 2.0
	var tk Tick
	n := 0
	for !tk.Stop {
		tk = e.Step(step, env)
		n++
		if tk.Frame.U > prev+1e-12 {
			t.Fatalf("rewind u rose from %v to %v", prev, tk.Frame.U)
		}
		prev = tk.Frame.U
		if n > 100 {
			t.Fatal("rewind never stopped")
		}
	}
	if n != 52 || tk.Frame.Phase != PhaseHome || tk.Frame.U != 0 || len(tk.Frame.Strokes) != 0 || tk.Frame.Bbox != (Rect{}) {
		t.Fatalf("rewind ended after %d steps at %+v", n, tk.Frame)
	}
	if tk.FPS != 0 || tk.Next != 0 || tk.Frame.Arrived {
		t.Fatalf("stopped tick armed a timer or arrived: %+v", tk)
	}
	if e.Out() {
		t.Fatal("Out after the rewind")
	}
	if tk := e.Step(step, env); !tk.Stop || tk.Redraw {
		t.Fatalf("a settled strand redrew: %+v", tk)
	}
}

func TestHoldStopsAfterHoldStopMsFrozen(t *testing.T) {
	s := mustSpec(t)
	env := baseEnv()
	e := started(t, s, env, Target{Seq: 1, Box: control, Quiet: true})
	frame := time.Second / 60
	var tk Tick
	for !tk.Frame.Arrived {
		tk = e.Step(frame, env)
	}
	hold := 0.0
	moved := env
	for i := 0; ; i++ {
		if i == 10 {
			moved.Anchor = spec.Point{X: 1690, Y: 905}
			if tk = e.Step(frame, moved); !tk.Redraw || !near(tk.Frame.Strokes[0].Pts[0], moved.Anchor, 1e-9) {
				t.Fatal("the held thread does not follow the anchor")
			}
		} else {
			tk = e.Step(frame, moved)
		}
		hold += frame.Seconds()
		if tk.Stop {
			break
		}
		if tk.FPS != 60 {
			t.Fatalf("hold runs at %d fps, want the breathing tier", tk.FPS)
		}
		if hold > 31 {
			t.Fatal("hold never stopped")
		}
	}
	if math.Abs(hold-s.Strand.HoldStopMs/1000) > frame.Seconds()+1e-6 {
		t.Fatalf("hold stopped after %v s, want %v", hold, s.Strand.HoldStopMs/1000)
	}
	frozen := tk.Frame
	moved.Anchor = spec.Point{X: 1500, Y: 700}
	after := e.Step(frame, moved)
	if !after.Stop || after.Redraw || after.FPS != 0 || after.Next != 0 || !samePixels(after.Frame, frozen) {
		t.Fatalf("a stopped hold repainted or armed a timer: %+v", after)
	}
	if !e.Out() {
		t.Fatal("a held thread is not Out")
	}
	saver := baseEnv()
	saver.BatterySaver = true
	e2 := started(t, s, saver, Target{Seq: 1, Box: control, Quiet: true})
	if tk := e2.Step(frame, saver); tk.FPS != 60 {
		t.Fatalf("battery saver reach at %d fps, want the motion tier", tk.FPS)
	}
	for tk := e2.Step(frame, saver); !tk.Frame.Arrived; tk = e2.Step(frame, saver) {
	}
	if tk := e2.Step(frame, saver); tk.FPS != 30 {
		t.Fatalf("battery saver hold at %d fps, want the rest tier", tk.FPS)
	}
}

func TestReducedMotionJumpsToHoldInOneFrame(t *testing.T) {
	s := mustSpec(t)
	env := baseEnv()
	env.Reduced = true
	targets := []Target{
		{Seq: 1, Box: control},
		{Seq: 1, Box: control, Mode: ModeLasso},
		{Seq: 1, Box: control, Mode: ModeRegion, Region: &Rect{X: 250, Y: 150, W: 400, H: 200}},
		{Seq: 1, Box: Rect{X: 200, Y: 150, W: 800, H: 600}, Frame: &Rect{X: 200, Y: 150, W: 800, H: 600}},
	}
	for _, tg := range targets {
		e := started(t, s, env, tg)
		tk := e.Step(step, env)
		if !tk.Frame.Arrived || !tk.Stop || !tk.Redraw || tk.Frame.Phase != PhaseHold || tk.FPS != 0 {
			t.Fatalf("mode %d: reduced target did not land in one stopped frame: %+v", tg.Mode, tk)
		}
		if tk.Frame.Char != nil || len(tk.Frame.Strokes) == 0 {
			t.Fatalf("mode %d: reduced hold frame %+v", tg.Mode, tk.Frame)
		}
		for _, st := range tk.Frame.Strokes {
			if st.Alpha != 1 {
				t.Fatalf("mode %d: reduced hold kept a fading stroke", tg.Mode)
			}
		}
		if tg.Mode == ModePoint && tg.Frame == nil && !near(tk.Frame.Tip, tipAnchor(s, control, env.Anchor), 1e-9) {
			t.Fatalf("reduced tip %v", tk.Frame.Tip)
		}
		if tk := e.Step(step, env); !tk.Stop || tk.Redraw || tk.Frame.Arrived {
			t.Fatalf("mode %d: reduced motion painted a second frame", tg.Mode)
		}
		e.Retract(1)
		tk = e.Step(step, env)
		if !tk.Stop || !tk.Redraw || tk.Frame.Phase != PhaseHome || len(tk.Frame.Strokes) != 0 {
			t.Fatalf("mode %d: reduced retract is not one home frame: %+v", tg.Mode, tk)
		}
		if tk := e.Step(step, env); !tk.Stop || tk.Redraw {
			t.Fatalf("mode %d: reduced retract painted twice", tg.Mode)
		}
	}
}

func TestReducedMotionMidReachLands(t *testing.T) {
	s := mustSpec(t)
	env := baseEnv()
	e := started(t, s, env, Target{Seq: 3, Box: control, Quiet: true})
	for range 20 {
		e.Step(step, env)
	}
	env.Reduced = true
	tk := e.Step(step, env)
	if !tk.Frame.Arrived || !tk.Stop || tk.Frame.Phase != PhaseHold {
		t.Fatalf("switching to reduced motion mid-reach: %+v", tk)
	}
}

func TestSkipLargeAndOffOutputTargets(t *testing.T) {
	s := mustSpec(t)
	env := baseEnv()
	cases := []struct {
		box  Rect
		want string
	}{
		{Rect{X: 100, Y: 100, W: 960, H: 540}, SkipCoversScreen},
		{Rect{X: 0, Y: 0, W: 1920, H: 1080}, SkipCoversScreen},
		{Rect{X: 2400, Y: 300, W: 100, H: 40}, SkipOffOutput},
		{Rect{X: 500, Y: -400, W: 100, H: 40}, SkipOffOutput},
	}
	for _, c := range cases {
		e := started(t, s, env, Target{Seq: 4, Box: c.box})
		tk := e.Step(step, env)
		if !tk.Frame.Arrived || tk.Frame.Skipped != c.want || tk.Frame.Seq != 4 || !tk.Stop || len(tk.Frame.Strokes) != 0 {
			t.Fatalf("%v: %+v, want skipped %q", c.box, tk, c.want)
		}
		if tk := e.Step(step, env); tk.Frame.Arrived || tk.Frame.Skipped != "" || !tk.Stop {
			t.Fatalf("%v: skip reported twice", c.box)
		}
	}
	// Not quite half the screen in both directions reaches.
	e := started(t, s, env, Target{Seq: 1, Box: Rect{X: 100, Y: 100, W: 959, H: 700}})
	if tk := e.Step(step, env); tk.Frame.Skipped != "" || tk.Frame.Phase != PhaseReach {
		t.Fatalf("a target under the cover rule skipped: %+v", tk.Frame)
	}
	// A skipped target while out winds the old thread home.
	e = started(t, s, env, Target{Seq: 1, Box: control})
	for range 120 {
		e.Step(step, env)
	}
	e.Target(Target{Seq: 2, Box: screen})
	tk := e.Step(step, env)
	if !tk.Frame.Arrived || tk.Frame.Skipped != SkipCoversScreen || tk.Frame.Phase != PhaseRewind {
		t.Fatalf("skip while out: %+v", tk.Frame)
	}
	for !tk.Stop {
		tk = e.Step(step, env)
		if tk.Frame.Arrived {
			t.Fatal("the skip arrived twice")
		}
	}
	if tk.Frame.Phase != PhaseHome {
		t.Fatalf("skip while out ended %s", tk.Frame.Phase)
	}
}

func TestStaleTargetsAreIgnored(t *testing.T) {
	s := mustSpec(t)
	env := baseEnv()
	e := started(t, s, env, Target{Seq: 5, Box: control, Quiet: true})
	for range 120 {
		e.Step(step, env)
	}
	e.Target(Target{Seq: 4, Box: Rect{X: 1300, Y: 120, W: 90, H: 30}})
	e.Target(Target{Seq: 5, Box: Rect{X: 1300, Y: 120, W: 90, H: 30}})
	tk := e.Step(step, env)
	if tk.Frame.Seq != 5 || tk.Frame.Phase != PhaseHold || tk.Frame.Arrived {
		t.Fatalf("a stale or repeated target moved the thread: %+v", tk.Frame)
	}
}

func TestStoppedClockArmsNothing(t *testing.T) {
	s := mustSpec(t)
	e := New(s)
	for i := range 3 {
		tk := e.Step(step, baseEnv())
		if !tk.Stop || tk.FPS != 0 || tk.Next != 0 || (i > 0 && tk.Redraw) || e.Out() {
			t.Fatalf("home step %d: %+v", i, tk)
		}
	}
}

func TestBboxGrowsMonotonicallyUntilHome(t *testing.T) {
	s := mustSpec(t)
	env := baseEnv()
	e := started(t, s, env, Target{Seq: 1, Box: control})
	var prev Rect
	for i := range 400 {
		if i == 150 {
			e.Retract(1)
		}
		tk := e.Step(step, env)
		b := tk.Frame.Bbox
		if tk.Frame.Phase == PhaseHome {
			if b != (Rect{}) {
				t.Fatalf("home keeps a bbox %v", b)
			}
			return
		}
		if len(tk.Frame.Strokes) > 0 {
			if !prev.empty() && prev.union(b) != b {
				t.Fatalf("step %d: bbox shrank from %v to %v", i, prev, b)
			}
			pad := s.Strand.Stroke.BboxPadPx
			for _, st := range tk.Frame.Strokes {
				for _, p := range st.Pts {
					if !b.grow(-pad + 1e-9).contains(p) {
						t.Fatalf("step %d: point %v inside the bbox padding of %v", i, p, b)
					}
				}
			}
			prev = b
		}
	}
	t.Fatal("never went home")
}

func TestSeamBlendRidesTheAnchor(t *testing.T) {
	s := mustSpec(t)
	a0 := spec.Point{X: 100, Y: 100}
	pts := bowed(s, a0, spec.Point{X: 600, Y: 300}, spec.Point{X: 960, Y: 540}, 96)
	orig := append([]spec.Point(nil), pts...)
	live := spec.Point{X: 110, Y: 92}
	seam(s, pts, a0, live)
	n := s.Strand.BowedPath.SeamBlendPoints
	if pts[0] != live {
		t.Fatalf("root %v, want the live anchor %v", pts[0], live)
	}
	prev := math.Inf(1)
	for i := range pts {
		moved := dist(pts[i], orig[i])
		if i >= n && moved != 0 {
			t.Fatalf("point %d past the seam moved %v", i, moved)
		}
		if moved > prev+1e-12 {
			t.Fatalf("seam weight rises at %d", i)
		}
		prev = moved
	}
}
