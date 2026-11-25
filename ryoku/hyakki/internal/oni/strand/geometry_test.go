package strand

import (
	"math"
	"testing"

	"ryoku-rashin/internal/oni/spec"
)

// runTo steps until the target lands, returning the landing tick and the
// steps taken.
func runTo(t *testing.T, e *Engine, env Env) (Tick, int) {
	t.Helper()
	for i := 1; i <= 1000; i++ {
		if tk := e.Step(step, env); tk.Frame.Arrived {
			return tk, i
		}
	}
	t.Fatal("never arrived")
	return Tick{}, 0
}

func TestPerchCurlsOnTheNearerTopCorner(t *testing.T) {
	s := mustSpec(t)
	frame := Rect{X: 400, Y: 200, W: 800, H: 500}
	p := s.Strand.Perch
	for _, c := range []struct {
		anchor spec.Point
		corner spec.Point
	}{
		{spec.Point{X: 100, Y: 900}, spec.Point{X: 400, Y: 200}},
		{spec.Point{X: 1700, Y: 900}, spec.Point{X: 1200, Y: 200}},
		// Equidistant: the corner nearer the screen centre (960, 540).
		{spec.Point{X: 800, Y: 1000}, spec.Point{X: 1200, Y: 200}},
	} {
		env := baseEnv()
		env.Anchor = c.anchor
		e := started(t, s, env, Target{Seq: 1, Box: frame, Frame: &frame, Quiet: true})
		tk, n := runTo(t, e, env)
		if n != 120 {
			t.Fatalf("perch landed after %d steps, want perch reach 0.8 s + curl 0.4 s", n)
		}
		main := tk.Frame.Strokes[0].Pts
		curl := main[len(main)-p.Segments:]
		for _, q := range curl {
			if math.Abs(dist(q, c.corner)-p.RadiusPx) > 1e-9 {
				t.Fatalf("anchor %v: curl point %v is %v px from %v, want radius %v", c.anchor, q, dist(q, c.corner), c.corner, p.RadiusPx)
			}
			if q.X > frame.X+1e-9 && q.X < frame.X+frame.W-1e-9 && q.Y > frame.Y+1e-9 {
				t.Fatalf("curl point %v inside the frame", q)
			}
		}
		landing := main[len(main)-p.Segments-1]
		if landing.Y != frame.Y || math.Abs(math.Abs(landing.X-c.corner.X)-p.OffsetPx) > 1e-9 {
			t.Fatalf("perch landed at %v, want the top edge %v px in from %v", landing, p.OffsetPx, c.corner)
		}
		want := spec.Point{X: c.corner.X, Y: c.corner.Y + p.RadiusPx}
		if !near(tk.Frame.Tip, want, 1e-9) {
			t.Fatalf("curl ends at %v, want down the outer side at %v", tk.Frame.Tip, want)
		}
	}
	// Under the minimum size the target is a plain reach and tap.
	env := baseEnv()
	small := Rect{X: 400, Y: 200, W: 319, H: 500}
	e := started(t, s, env, Target{Seq: 1, Box: small, Frame: &small, Quiet: true})
	if _, n := runTo(t, e, env); n != 92 {
		t.Fatalf("a %vx%v frame perched (landed after %d steps)", small.W, small.H, n)
	}
	short := Rect{X: 400, Y: 200, W: 800, H: 199}
	e = started(t, s, env, Target{Seq: 1, Box: short, Frame: &short, Quiet: true})
	if _, n := runTo(t, e, env); n != 92 {
		t.Fatalf("a %vx%v frame perched", short.W, short.H)
	}
}

// growth solves the superellipse growth e of a ring point about the
// lasso's grown box.
func growth(s *spec.OniSpec, g lassoGeom, q spec.Point) float64 {
	p := s.Strand.Lasso.Power
	level := func(e float64) float64 {
		return math.Pow(math.Abs(q.X-g.c.X)/(g.hw+e), p) + math.Pow(math.Abs(q.Y-g.c.Y)/(g.hh+e), p)
	}
	lo, hi := -8.0, 30.0
	for range 200 {
		m := (lo + hi) / 2
		if level(m) > 1 {
			lo = m
		} else {
			hi = m
		}
	}
	return (lo + hi) / 2
}

func TestLassoThrowsWideThenCinches(t *testing.T) {
	s := mustSpec(t)
	l := s.Strand.Lasso
	env := baseEnv()
	box := Rect{X: 600, Y: 400, W: 160, H: 48}
	e := started(t, s, env, Target{Seq: 1, Box: box, Mode: ModeLasso, Quiet: true})
	var tk Tick
	for range 62 {
		tk = e.Step(step, env)
	}
	if tk.Frame.Phase != PhaseLasso || len(tk.Frame.Strokes) != 2 {
		t.Fatalf("after the draw: %s with %d strokes", tk.Frame.Phase, len(tk.Frame.Strokes))
	}
	ring := tk.Frame.Strokes[1].Pts
	if len(ring) != l.Points {
		t.Fatalf("thrown ring has %d points, want %d", len(ring), l.Points)
	}
	for _, q := range ring {
		if g := growth(s, e.ls, q); math.Abs(g-l.ThrownPx) > 0.1 {
			t.Fatalf("thrown ring point %v grown %v px, want %v", q, g, l.ThrownPx)
		}
	}
	lead := tk.Frame.Strokes[0].Pts
	if len(lead) != l.LeadPoints || lead[0] != env.Anchor || lead[len(lead)-1] != ring[0] {
		t.Fatalf("lead line %d points from %v to %v", len(lead), lead[0], lead[len(lead)-1])
	}
	tk, n := runTo(t, e, env)
	if n != 50 {
		t.Fatalf("cinch landed %d steps after the throw, want %v s", n, l.CinchS)
	}
	ring = tk.Frame.Strokes[1].Pts
	for _, q := range ring {
		if g := growth(s, e.ls, q); math.Abs(g) > 1e-6 {
			t.Fatalf("cinched ring point grown %v px", g)
		}
	}
	// The ring starts at the point of the ring nearest the character.
	d0 := dist(ring[0], env.Anchor)
	for i := range 20000 {
		th := 2 * math.Pi * float64(i) / 20000
		if d := dist(e.ls.ringAt(s, th, 0), env.Anchor); d < d0-1e-6 {
			t.Fatalf("ring starts %v px from the character, a point at %v is %v", d0, th, d)
		}
	}
	// The sweep runs lasso.turns.
	if q := e.ls.at(s, 1, 0); !near(q, e.ls.ringAt(s, e.ls.th0+2*math.Pi*l.Turns, 0), 1e-9) || !near(ring[len(ring)-1], q, 1e-9) {
		t.Fatalf("ring end %v, want %v turns round", ring[len(ring)-1], l.Turns)
	}
	if tk := e.Step(step, env); tk.Frame.Phase != PhaseHold {
		t.Fatalf("lasso holds as %s", tk.Frame.Phase)
	}
	e.Retract(1)
	steps := 0
	for tk := e.Step(step, env); !tk.Stop; tk = e.Step(step, env) {
		steps++
	}
	if want := int(math.Ceil(s.Strand.Rewind.LassoS*s.Strand.Rewind.LassoScale*100)) - 1; steps != want {
		t.Fatalf("lasso rewound in %d steps, want %d", steps+1, want+1)
	}
}

func TestRegionPenRidesAheadAndSlowsIntoCorners(t *testing.T) {
	s := mustSpec(t)
	rs := s.Strand.Region
	env := baseEnv()
	region := Rect{X: 400, Y: 300, W: 500, H: 300}
	box := Rect{X: 450, Y: 350, W: 100, H: 40}
	e := started(t, s, env, Target{Seq: 1, Box: box, Region: &region, Mode: ModeRegion, Quiet: true})
	tk := e.Step(step, env)
	g := e.rg
	if want := math.Min(rs.ClampS[1], math.Max(rs.ClampS[0], g.total/rs.SpeedPxS)); g.dur != want {
		t.Fatalf("pen run %v s, want %v", g.dur, want)
	}
	if math.Abs(g.total-g.per-rs.Overrun*rs.StepPx) > 1e-9 {
		t.Fatalf("pen overruns %v px, want %v", g.total-g.per, rs.Overrun*rs.StepPx)
	}
	rides := 0
	for tk.Frame.Phase == PhaseRegion {
		c := tk.Frame.Char
		if c == nil || c.Scale != rs.PenScale {
			t.Fatalf("pen phase without the pen-sized character: %+v", c)
		}
		pp := g.perim(g.s0 + e.pen)
		tangent := spec.Point{X: -pp.n.Y, Y: pp.n.X}
		d := sub(c.Pos, tk.Frame.Tip)
		if a, o := d.X*tangent.X+d.Y*tangent.Y, d.X*pp.n.X+d.Y*pp.n.Y; math.Abs(a-rs.AheadPx) > 1e-9 || math.Abs(o-rs.OutsidePx) > 1e-9 {
			t.Fatalf("pen rides %v ahead %v out, want %v and %v", a, o, rs.AheadPx, rs.OutsidePx)
		}
		pen := tk.Frame.Strokes[len(tk.Frame.Strokes)-1].Pts
		if pen[len(pen)-1] != tk.Frame.Tip {
			t.Fatal("the tip is not the pen's end")
		}
		rides++
		tk = e.Step(step, env)
	}
	if rides < int(g.dur*100)-1 || tk.Frame.Char != nil {
		t.Fatalf("pen rode %d steps over %v s, then char %+v", rides, g.dur, tk.Frame.Char)
	}
	if tk.Frame.Phase != PhaseLasso {
		t.Fatalf("after the pen: %s, want the lasso", tk.Frame.Phase)
	}
	// Corner slowdown: dσ/dt at a corner is cornerSpeed of a straight's.
	rate := func(sg float64) float64 {
		lo, hi := 0.0, g.dur
		for range 100 {
			m := (lo + hi) / 2
			if g.sigmaAt(m) < sg {
				lo = m
			} else {
				hi = m
			}
		}
		h := 1e-4
		return (g.sigmaAt(lo+h) - g.sigmaAt(lo-h)) / (2 * h)
	}
	var corner float64
	for _, c := range g.corners {
		if c > rs.CornerSlowPx && c < g.total-rs.CornerSlowPx {
			corner = c
			break
		}
	}
	straight := -1.0
	for sg := rs.SlowEnds; sg < g.total-rs.SlowEnds; sg++ {
		if g.speed(s, sg) == 1 {
			straight = sg
			break
		}
	}
	if straight < 0 {
		t.Fatal("no full-speed stretch on the pen path")
	}
	if r := rate(corner) / rate(straight); math.Abs(r-rs.CornerSpeed) > 0.01 {
		t.Fatalf("corner speed ratio %v, want %v", r, rs.CornerSpeed)
	}
	if r := rate(1) / rate(straight); !(r < 0.5) {
		t.Fatalf("pen does not slow at its start: %v", r)
	}
	arrived := 0
	for range 300 {
		if tk = e.Step(step, env); tk.Frame.Arrived {
			arrived++
		}
	}
	if arrived != 1 || tk.Frame.Phase != PhaseHold || len(tk.Frame.Strokes) != 3 {
		t.Fatalf("region arrived %d times, holding %s with %d strokes", arrived, tk.Frame.Phase, len(tk.Frame.Strokes))
	}
	// The pen's mark crosses itself: its end passes its start.
	pen := tk.Frame.Strokes[2].Pts
	if dist(pen[0], pen[len(pen)-1]) > rs.Overrun*rs.StepPx+2*rs.Wobble {
		t.Fatal("the pen mark does not close")
	}
}

func TestRegionWobbleIsTheSeededFormula(t *testing.T) {
	s := mustSpec(t)
	rs := s.Strand.Region
	g := newRegion(s, Rect{X: 100, Y: 100, W: 300, H: 200}, spec.Point{})
	seed := float64(rs.WobbleSeed)
	phi1 := 2 * math.Pi * (seed*0.6180339887 - math.Floor(seed*0.6180339887))
	phi2 := 2 * math.Pi * (seed*0.4142135624 - math.Floor(seed*0.4142135624))
	for _, sg := range []float64{0, 13, 100, 517.5} {
		want := rs.Wobble * (0.6*math.Sin(2*math.Pi*rs.WobbleRate*sg+phi1) + 0.4*math.Sin(2*math.Pi*rs.WobbleRate*1.7*sg+phi2))
		if got := g.wob(sg); math.Abs(got-want) > 1e-12 {
			t.Fatalf("wobble at %v: %v, want %v", sg, got, want)
		}
		pp := g.perim(g.s0 + sg)
		if d := sub(g.at(sg), pp.p); math.Abs(d.X*pp.n.X+d.Y*pp.n.Y-want) > 1e-9 {
			t.Fatal("wobble is not along the outward normal")
		}
	}
	h := newRegion(s, Rect{X: 100, Y: 100, W: 300, H: 200}, spec.Point{})
	for sg := 0.0; sg < g.total; sg += 7 {
		if g.at(sg) != h.at(sg) {
			t.Fatal("the pen path is not deterministic")
		}
	}
}

func TestBubbleNeverCoversTheTarget(t *testing.T) {
	s := mustSpec(t)
	sizes := []spec.Point{{X: 240, Y: 90}, {X: 400, Y: 200}, {X: 120, Y: 40}}
	boxes := []spec.Point{{X: 120, Y: 40}, {X: 40, Y: 40}, {X: 300, Y: 220}, {X: 900, Y: 30}}
	anchors := []spec.Point{{X: 1700, Y: 900}, {X: 80, Y: 80}, {X: 960, Y: 1040}, {X: 1880, Y: 60}}
	checked := 0
	for _, bs := range boxes {
		for y := -20.0; y < screen.H; y += 53 {
			for x := -20.0; x < screen.W; x += 97 {
				box := Rect{X: x, Y: y, W: bs.X, H: bs.Y}
				for _, a := range anchors {
					tip := tipAnchor(s, box, a)
					approach := unit(sub(tip, a))
					for _, size := range sizes {
						r := BubbleRect(box, tip, approach, size, screen, s)
						checked++
						if r.overlaps(box) {
							t.Fatalf("bubble %v covers target %v (tip %v)", r, box, tip)
						}
						if r.W != math.Min(size.X, s.Strand.Bubble.MaxWidthPx) || r.H != size.Y {
							t.Fatalf("bubble size %vx%v for %v", r.W, r.H, size)
						}
						if r.contains(tip) && !(r.X == tip.X || r.Y == tip.Y || r.X+r.W == tip.X || r.Y+r.H == tip.Y) {
							t.Fatalf("bubble %v covers the tip %v", r, tip)
						}
					}
				}
			}
		}
	}
	// With room on the approach side the card sits there, on screen.
	box := Rect{X: 900, Y: 500, W: 120, H: 40}
	r := BubbleRect(box, spec.Point{X: 1024, Y: 520}, spec.Point{X: -1, Y: 0}, spec.Point{X: 240, Y: 90}, screen, s)
	if r.X != box.X+box.W+s.Strand.BowedPath.TipGapPx+s.Strand.BowedPath.TipStopPx || !(screen.contains(spec.Point{X: r.X, Y: r.Y}) && screen.contains(spec.Point{X: r.X + r.W, Y: r.Y + r.H})) {
		t.Fatalf("bubble %v not on the approach side of %v", r, box)
	}
	if checked < 10000 {
		t.Fatalf("sweep checked only %d placements", checked)
	}
}

func TestApproachFollowsTheLeadingStroke(t *testing.T) {
	s := mustSpec(t)
	env := baseEnv()
	e := started(t, s, env, Target{Seq: 1, Box: control})
	tk, _ := runTo(t, e, env)
	a := tk.Frame.Approach()
	main := tk.Frame.Strokes[0].Pts
	want := unit(sub(main[len(main)-1], main[len(main)-2]))
	if !near(a, want, 1e-9) {
		t.Fatalf("approach %v, want %v", a, want)
	}
	if (Frame{}).Approach() != (spec.Point{}) {
		t.Fatal("an empty frame has an approach")
	}
}
