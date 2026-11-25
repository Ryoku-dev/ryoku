package strand

import (
	"math"
	"reflect"
	"testing"

	"ryoku-rashin/internal/oni/spec"
)

// curlOf is the curl part of a landed perch's main thread.
func curlOf(s *spec.OniSpec, f Frame) []spec.Point {
	main := f.Strokes[0].Pts
	return main[len(main)-s.Strand.Perch.Segments:]
}

// onRadius fails unless every curl point lies radiusPx from corner.
func onRadius(t *testing.T, s *spec.OniSpec, what string, curl []spec.Point, corner spec.Point) {
	t.Helper()
	for _, q := range curl {
		if math.Abs(dist(q, corner)-s.Strand.Perch.RadiusPx) > 1e-9 {
			t.Fatalf("%s: curl point %v is %v px from %v, want %v", what, q, dist(q, corner), corner, s.Strand.Perch.RadiusPx)
		}
	}
}

func TestPerchModeCurlsOnHalfAndMaximizedFrames(t *testing.T) {
	s := mustSpec(t)
	for _, c := range []struct {
		name   string
		frame  Rect
		corner spec.Point
		big    bool // a point target on the same frame covers the screen
	}{
		// The outer corner of a tile against the screen edge has no room
		// for the curl, so each half takes its inner corner.
		{"left half", Rect{X: 0, Y: 0, W: 960, H: 1080}, spec.Point{X: 960, Y: 0}, true},
		{"right half", Rect{X: 960, Y: 0, W: 960, H: 1080}, spec.Point{X: 960, Y: 0}, true},
		{"maximized", Rect{X: 0, Y: 32, W: 1920, H: 1048}, spec.Point{X: 1920, Y: 32}, true},
		{"under the perch minimum", Rect{X: 700, Y: 400, W: 200, H: 120}, spec.Point{X: 900, Y: 400}, false},
	} {
		env := baseEnv()
		if c.big {
			f := c.frame
			e := started(t, s, env, Target{Seq: 1, Box: f, Frame: &f})
			if tk := e.Step(step, env); tk.Frame.Skipped != SkipCoversScreen {
				t.Fatalf("%s: a point target on the frame was not skipped as covering the screen: %+v", c.name, tk.Frame)
			}
		}
		// Box alone carries the frame in perch mode.
		e := started(t, s, env, Target{Seq: 2, Box: c.frame, Mode: ModePerch})
		tk, n := runTo(t, e, env)
		if tk.Frame.Skipped != "" || tk.Frame.Seq != 2 || tk.Frame.Phase != PhaseHold {
			t.Fatalf("%s: perch-mode target ended %+v", c.name, tk.Frame)
		}
		if n != 120 {
			t.Fatalf("%s: landed after %d steps, want perch reach 0.8 s + curl 0.4 s", c.name, n)
		}
		onRadius(t, s, c.name, curlOf(s, tk.Frame), c.corner)
	}
}

func TestPerchModeStillSkipsOffOutput(t *testing.T) {
	s := mustSpec(t)
	env := baseEnv()
	for _, f := range []Rect{
		{X: 2000, Y: 0, W: 960, H: 1080},
		{X: -1000, Y: 100, W: 800, H: 600},
		{X: 200, Y: 1200, W: 800, H: 600},
	} {
		frame := f
		e := started(t, s, env, Target{Seq: 3, Box: Rect{X: 900, Y: 500, W: 10, H: 10}, Frame: &frame, Mode: ModePerch})
		tk := e.Step(step, env)
		if !tk.Frame.Arrived || tk.Frame.Skipped != SkipOffOutput || len(tk.Frame.Strokes) != 0 || !tk.Stop {
			t.Fatalf("%v: %+v, want skipped off-output by its frame", f, tk.Frame)
		}
	}
}

func TestPerchCornerRules(t *testing.T) {
	s := mustSpec(t)
	centre := spec.Point{X: 960, Y: 540}
	mid := Rect{X: 400, Y: 200, W: 800, H: 500}
	for _, c := range []struct {
		name   string
		frame  Rect
		from   spec.Point
		screen Rect
		want   string
	}{
		{"nearer left", mid, spec.Point{X: 100, Y: 900}, screen, CornerTopLeft},
		{"nearer right", mid, spec.Point{X: 1700, Y: 900}, screen, CornerTopRight},
		{"tie to centre", mid, spec.Point{X: 800, Y: 1000}, screen, CornerTopRight},
		{"left tile from the left", Rect{X: 0, Y: 0, W: 960, H: 1080}, spec.Point{X: 10, Y: 10}, screen, CornerTopRight},
		{"right tile from the right", Rect{X: 960, Y: 0, W: 960, H: 1080}, spec.Point{X: 1900, Y: 10}, screen, CornerTopLeft},
		{"curl just fits on the left", Rect{X: 18, Y: 0, W: 942, H: 1080}, spec.Point{X: 10, Y: 10}, screen, CornerTopLeft},
		{"neither fits: nearer", Rect{X: 0, Y: 32, W: 1920, H: 1048}, spec.Point{X: 10, Y: 900}, screen, CornerTopLeft},
		{"unknown screen: nearer", Rect{X: 0, Y: 0, W: 960, H: 1080}, spec.Point{X: 10, Y: 10}, Rect{}, CornerTopLeft},
	} {
		if got := PerchCorner(s, c.frame, c.from, centre, c.screen); got != c.want {
			t.Errorf("%s: corner %q, want %q", c.name, got, c.want)
		}
	}
	// A named corner wins over the nearer one.
	env := baseEnv()
	e := started(t, s, env, Target{Seq: 1, Box: mid, Mode: ModePerch, Corner: CornerTopLeft})
	tk, _ := runTo(t, e, env)
	onRadius(t, s, "named corner", curlOf(s, tk.Frame), spec.Point{X: mid.X, Y: mid.Y})
}

func TestPerchSeatClearsTheCurlAndStaysOnTheOutput(t *testing.T) {
	s := mustSpec(t)
	st := s.Perch.Seat
	side := s.Character.ButtonPx
	// Away from every edge the seat sits outside the corner.
	mid := Rect{X: 600, Y: 400, W: 600, H: 400}
	if got, want := PerchSeat(s, mid, CornerTopRight, side, screen), (spec.Point{X: 1200 + st.GapPx, Y: 400 - st.LiftPx - side}); got != want {
		t.Fatalf("top-right seat %v, want %v", got, want)
	}
	if got, want := PerchSeat(s, mid, CornerTopLeft, side, screen), (spec.Point{X: 600 - st.GapPx - side, Y: 400 - st.LiftPx - side}); got != want {
		t.Fatalf("top-left seat %v, want %v", got, want)
	}
	// An unknown screen flips and clamps nothing.
	tile := Rect{X: 0, Y: 0, W: 960, H: 1080}
	if got, want := PerchSeat(s, tile, CornerTopLeft, side, Rect{}), (spec.Point{X: -st.GapPx - side, Y: -st.LiftPx - side}); got != want {
		t.Fatalf("unclamped seat %v, want %v", got, want)
	}
	// A screen too narrow for the frame and both margins centres it.
	narrow := Rect{X: 100, Y: 0, W: side + st.MarginPx, H: 1080}
	if got := PerchSeat(s, Rect{X: 100, Y: 300, W: 40, H: 40}, CornerTopLeft, side, narrow); got.X != 100+st.MarginPx/2 {
		t.Fatalf("seat on a narrow screen at x %v, want centred %v", got.X, 100+st.MarginPx/2)
	}
	// Sweep frames against every edge: the seat stays marginPx inside the
	// screen and never touches the curl's ink.
	ink := s.Strand.Stroke.WidthPx/2 + s.Strand.Stroke.OutlinePx
	checked := 0
	for _, f := range []Rect{
		mid, tile,
		{X: 960, Y: 0, W: 960, H: 1080},
		{X: 0, Y: 32, W: 1920, H: 1048},
		{X: 0, Y: 0, W: 1920, H: 1080},
		{X: 30, Y: 20, W: 500, H: 300},
		{X: 1400, Y: 10, W: 500, H: 900},
		{X: 700, Y: 900, W: 400, H: 170},
		{X: 10, Y: 700, W: 400, H: 370},
	} {
		for _, corner := range []string{CornerTopLeft, CornerTopRight} {
			p := PerchSeat(s, f, corner, side, screen)
			seat := Rect{X: p.X, Y: p.Y, W: side, H: side}
			if seat.X < screen.X+st.MarginPx || seat.Y < screen.Y+st.MarginPx ||
				seat.X+side > screen.X+screen.W-st.MarginPx || seat.Y+side > screen.Y+screen.H-st.MarginPx {
				t.Fatalf("%v %s: seat %v crosses the screen margin", f, corner, seat)
			}
			g := newPerch(s, f, corner)
			for _, q := range append(g.arc(s, 1), g.land) {
				if seat.grow(ink).contains(q) {
					t.Fatalf("%v %s: seat %v sits on the curl at %v", f, corner, seat, q)
				}
				checked++
			}
		}
	}
	if checked < 18*s.Strand.Perch.Segments {
		t.Fatalf("checked only %d curl points", checked)
	}
}

func TestPerchJumpsWithinSnap(t *testing.T) {
	s := mustSpec(t)
	snap := s.Perch.Reattach.SnapPx
	from := spec.Point{X: 500, Y: 500}
	if !PerchJumps(s, from, spec.Point{X: 500 + snap, Y: 500}) || !PerchJumps(s, from, from) {
		t.Fatal("a seat moved at most snapPx does not jump")
	}
	if PerchJumps(s, from, spec.Point{X: 500 + snap*0.8, Y: 500 + snap*0.8}) {
		t.Fatal("a seat moved past snapPx jumps instead of flying")
	}
}

// perched lands a quiet perch-mode target on frame, the character seated on
// its corner, and returns the engine and env.
func perched(t *testing.T, s *spec.OniSpec, frame Rect, reduced bool) (*Engine, Env, string) {
	t.Helper()
	env := baseEnv()
	env.Reduced = reduced
	corner := PerchCorner(s, frame, env.Anchor, env.Centre, env.Screen)
	env.Anchor = seatAnchor(s, frame, corner, env.Screen)
	e := started(t, s, env, Target{Seq: 5, Box: frame, Mode: ModePerch, Corner: corner, Quiet: true})
	runTo(t, e, env)
	return e, env, corner
}

func cornerOf(f Rect, corner string) spec.Point {
	if corner == CornerTopRight {
		return spec.Point{X: f.X + f.W, Y: f.Y}
	}
	return spec.Point{X: f.X, Y: f.Y}
}

func TestReframeMovesTheCurlWithoutArriving(t *testing.T) {
	s := mustSpec(t)
	a := Rect{X: 600, Y: 400, W: 600, H: 400}
	b := Rect{X: 650, Y: 430, W: 620, H: 380}
	e, env, corner := perched(t, s, a, false)
	for range 10 {
		e.Step(step, env)
	}
	env.Anchor = seatAnchor(s, b, corner, env.Screen)
	if !e.Reframe(b) {
		t.Fatal("a perch-mode target refused the reframe")
	}
	tk := e.Step(step, env)
	if !tk.Redraw || tk.Frame.Arrived || tk.Frame.Phase != PhaseHold || tk.Frame.Seq != 5 {
		t.Fatalf("reframe step: %+v", tk)
	}
	onRadius(t, s, "reframed", curlOf(s, tk.Frame), cornerOf(b, corner))
	main := tk.Frame.Strokes[0].Pts
	if main[0] != env.Anchor || len(tk.Frame.Strokes) != 1 {
		t.Fatalf("reframed thread leaves %v, want the seat's anchor %v, and draws only itself", main[0], env.Anchor)
	}
	// The drawn bounds restart at the new geometry: no trace of the old.
	pad := s.Strand.Stroke.BboxPadPx
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, q := range main {
		x0, y0, x1, y1 = math.Min(x0, q.X), math.Min(y0, q.Y), math.Max(x1, q.X), math.Max(y1, q.Y)
	}
	if want := (Rect{X: x0 - pad, Y: y0 - pad, W: x1 - x0 + 2*pad, H: y1 - y0 + 2*pad}); tk.Frame.Bbox != want {
		t.Fatalf("reframed bbox %v, want %v", tk.Frame.Bbox, want)
	}
	for range 200 {
		if tk := e.Step(step, env); tk.Frame.Arrived {
			t.Fatal("a reframe arrived")
		}
	}
	// Under reduced motion a stopped perch wakes for exactly one frame.
	e, env, corner = perched(t, s, a, true)
	if tk := e.Step(step, env); !tk.Stop || tk.Redraw {
		t.Fatalf("a settled reduced perch is not stopped: %+v", tk)
	}
	env.Anchor = seatAnchor(s, b, corner, env.Screen)
	e.Reframe(b)
	tk = e.Step(step, env)
	if !tk.Stop || !tk.Redraw || tk.Frame.Arrived {
		t.Fatalf("reduced reframe: %+v", tk)
	}
	onRadius(t, s, "reduced reframe", curlOf(s, tk.Frame), cornerOf(b, corner))
	if tk := e.Step(step, env); !tk.Stop || tk.Redraw {
		t.Fatalf("a reduced reframe kept running: %+v", tk)
	}
}

func TestReframeAndReperchNeedAPerchModeTarget(t *testing.T) {
	s := mustSpec(t)
	env := baseEnv()
	win := Rect{X: 200, Y: 150, W: 800, H: 600}
	for _, tg := range []Target{
		{Seq: 1, Box: control},
		{Seq: 1, Box: win, Frame: &win}, // a point target that perches by size
		{Seq: 1, Box: control, Mode: ModeLasso},
	} {
		// A twin left alone is the reference for what the refusal changed.
		e, twin := started(t, s, env, tg), started(t, s, env, tg)
		runTo(t, e, env)
		runTo(t, twin, env)
		if e.Reframe(win) || e.Reperch(win) {
			t.Fatalf("mode %d target took a reframe or reperch", tg.Mode)
		}
		for i := range 60 {
			if got, want := e.Step(step, env), twin.Step(step, env); !reflect.DeepEqual(got, want) {
				t.Fatalf("mode %d target, step %d after a refused reframe: %+v, want %+v", tg.Mode, i, got, want)
			}
		}
	}
	// While home a reframe only stores the frame.
	a := Rect{X: 600, Y: 400, W: 600, H: 400}
	e, env, _ := perched(t, s, a, false)
	e.Retract(5)
	for tk := e.Step(step, env); !tk.Stop; tk = e.Step(step, env) {
	}
	if !e.Reframe(Rect{X: 100, Y: 100, W: 500, H: 400}) {
		t.Fatal("a retracted perch-mode target refused the reframe")
	}
	if tk := e.Step(step, env); !tk.Stop || tk.Redraw || len(tk.Frame.Strokes) != 0 || e.Out() {
		t.Fatalf("a reframe while home drew or woke: %+v", tk)
	}
}

func TestReperchCurlsAgainQuietly(t *testing.T) {
	s := mustSpec(t)
	a := Rect{X: 600, Y: 400, W: 600, H: 400}
	b := Rect{X: 100, Y: 200, W: 700, H: 500}
	env := baseEnv()
	corner := PerchCorner(s, a, env.Anchor, env.Centre, env.Screen)
	env.Anchor = seatAnchor(s, a, corner, env.Screen)
	// Not quiet: the first landing bursts, the reperch must not.
	e := started(t, s, env, Target{Seq: 9, Box: a, Mode: ModePerch, Corner: corner})
	if tk, _ := runTo(t, e, env); len(tk.Frame.Strokes) < 2 {
		t.Fatal("the first perch landing opened no burst")
	}
	e.Retract(9)
	for tk := e.Step(step, env); !tk.Stop; tk = e.Step(step, env) {
	}
	env.Anchor = seatAnchor(s, b, corner, env.Screen)
	if !e.Reperch(b) {
		t.Fatal("the perch-mode target refused the reperch")
	}
	var last Tick
	for i := 1; i <= 400; i++ {
		last = e.Step(step, env)
		if last.Frame.Arrived || last.Frame.Skipped != "" {
			t.Fatalf("step %d: the reperch reported an arrival", i)
		}
		if len(last.Frame.Strokes) > 1 {
			t.Fatalf("step %d: the reperch drew %d strokes (a burst)", i, len(last.Frame.Strokes))
		}
		if i == 119 && last.Frame.Phase != PhaseCurl {
			t.Fatalf("step 119 is %s, want the curl", last.Frame.Phase)
		}
		if i == 120 && last.Frame.Phase != PhaseHold {
			t.Fatalf("step 120 is %s, want held: reach 0.8 s + curl 0.4 s", last.Frame.Phase)
		}
	}
	if last.Frame.Seq != 9 {
		t.Fatalf("reperch changed the seq to %d", last.Frame.Seq)
	}
	onRadius(t, s, "reperched", curlOf(s, last.Frame), cornerOf(b, corner))
	// A new target after a reperch arrives as usual.
	e.Target(Target{Seq: 10, Box: a, Mode: ModePerch, Corner: corner, Quiet: true})
	if tk, _ := runTo(t, e, env); tk.Frame.Seq != 10 {
		t.Fatalf("seq 10 landed as %d", tk.Frame.Seq)
	}
	// A reperch onto a frame off the output stays home and says nothing.
	e.Retract(10)
	for tk := e.Step(step, env); !tk.Stop; tk = e.Step(step, env) {
	}
	e.Reperch(Rect{X: 2400, Y: 100, W: 600, H: 400})
	for range 200 {
		if tk := e.Step(step, env); tk.Frame.Arrived || tk.Frame.Skipped != "" || len(tk.Frame.Strokes) != 0 {
			t.Fatalf("an off-output reperch drew or reported: %+v", tk.Frame)
		}
	}
	// Under reduced motion a reperch lands in one frame, quietly.
	e, env, corner = perched(t, s, a, true)
	e.Retract(5)
	e.Step(step, env)
	env.Anchor = seatAnchor(s, b, corner, env.Screen)
	e.Reperch(b)
	tk := e.Step(step, env)
	if !tk.Stop || tk.Frame.Phase != PhaseHold || tk.Frame.Arrived {
		t.Fatalf("reduced reperch: %+v", tk)
	}
	onRadius(t, s, "reduced reperch", curlOf(s, tk.Frame), cornerOf(b, corner))
}
