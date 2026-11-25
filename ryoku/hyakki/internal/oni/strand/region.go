package strand

import (
	"math"
	"sort"

	"ryoku-rashin/internal/oni/spec"
)

// regionGeom is the pen's path round a named region: a rounded rect padPx
// outside it with radiusPx corners, run clockwise from the point nearest
// the character and closed overrun·stepPx past its start so the line
// crosses itself, displaced along its outward normal by the seeded wobble.
// The pen's speed is arc-length eased: full speed on the straights, down to
// cornerSpeed at each corner over cornerSlowPx and at both ends over
// slowEnds px, its whole run taking clamp(length/speedPxS, clampS).
type regionGeom struct {
	x, y, w, h, r float64
	per           float64 // perimeter
	s0            float64 // start along the perimeter
	total         float64 // pen length, perimeter plus overrun
	dur           float64
	corners       []float64 // corner centres along the pen
	grid          float64
	tau           []float64 // cumulative 1/speed over the pen at grid steps
	samples       []float64 // σ of the drawn samples
	phi1, phi2    float64
	wobble, rate  float64
}

func newRegion(s *spec.OniSpec, region Rect, a spec.Point) regionGeom {
	rs := s.Strand.Region
	p := region.grow(rs.PadPx)
	g := regionGeom{x: p.X, y: p.Y, w: p.W, h: p.H}
	g.r = math.Min(rs.RadiusPx, math.Min(p.W, p.H)/2)
	l1, l2, arc := g.w-2*g.r, g.h-2*g.r, g.r*math.Pi/2
	g.per = 2*l1 + 2*l2 + 4*arc
	// Start at the perimeter point nearest the character.
	best := math.Inf(1)
	for sp := 0.0; sp < g.per; sp += 0.5 {
		if d := dist(g.perim(sp).p, a); d < best {
			best, g.s0 = d, sp
		}
	}
	g.total = g.per + rs.Overrun*rs.StepPx
	g.dur = math.Min(rs.ClampS[1], math.Max(rs.ClampS[0], g.total/rs.SpeedPxS))
	centres := []float64{l1 + arc/2, l1 + arc + l2 + arc/2, 2*l1 + 2*arc + l2 + arc/2, 2*l1 + 3*arc + 2*l2 + arc/2}
	for _, c := range centres {
		for c := c - g.s0 - g.per; c <= g.total+g.per; c += g.per {
			g.corners = append(g.corners, c)
		}
	}
	sort.Float64s(g.corners)
	seed := float64(uint32(rs.WobbleSeed))
	frac := func(x float64) float64 { return x - math.Floor(x) }
	g.phi1 = 2 * math.Pi * frac(seed*0.6180339887)
	g.phi2 = 2 * math.Pi * frac(seed*0.4142135624)
	g.wobble, g.rate = rs.Wobble, rs.WobbleRate
	// Time weights: τ(σ) = ∫ dσ / v(σ), trapezoid at 1 px steps.
	n := int(math.Ceil(g.total))
	g.grid = g.total / float64(n)
	g.tau = make([]float64, n+1)
	for i := 1; i <= n; i++ {
		a, b := g.speed(s, float64(i-1)*g.grid), g.speed(s, float64(i)*g.grid)
		g.tau[i] = g.tau[i-1] + g.grid*(1/a+1/b)/2
	}
	for sg := 0.0; sg < g.total-1e-9; sg += rs.StepPx {
		g.samples = append(g.samples, sg)
	}
	g.samples = append(g.samples, g.total)
	return g
}

type perimPoint struct{ p, n spec.Point }

// perim is the unwobbled point and outward normal at perimeter length sp,
// clockwise from the top edge's left end.
func (g regionGeom) perim(sp float64) perimPoint {
	sp = math.Mod(sp, g.per)
	if sp < 0 {
		sp += g.per
	}
	l1, l2, arc := g.w-2*g.r, g.h-2*g.r, g.r*math.Pi/2
	corner := func(cx, cy, a0, t float64) perimPoint {
		a := a0 + t/g.r
		n := spec.Point{X: math.Cos(a), Y: math.Sin(a)}
		return perimPoint{spec.Point{X: cx + g.r*n.X, Y: cy + g.r*n.Y}, n}
	}
	x0, y0, x1, y1 := g.x, g.y, g.x+g.w, g.y+g.h
	switch {
	case sp < l1:
		return perimPoint{spec.Point{X: x0 + g.r + sp, Y: y0}, spec.Point{Y: -1}}
	case sp < l1+arc:
		return corner(x1-g.r, y0+g.r, -math.Pi/2, sp-l1)
	case sp < l1+arc+l2:
		return perimPoint{spec.Point{X: x1, Y: y0 + g.r + sp - l1 - arc}, spec.Point{X: 1}}
	case sp < l1+2*arc+l2:
		return corner(x1-g.r, y1-g.r, 0, sp-l1-arc-l2)
	case sp < 2*l1+2*arc+l2:
		return perimPoint{spec.Point{X: x1 - g.r - (sp - l1 - 2*arc - l2), Y: y1}, spec.Point{Y: 1}}
	case sp < 2*l1+3*arc+l2:
		return corner(x0+g.r, y1-g.r, math.Pi/2, sp-2*l1-2*arc-l2)
	case sp < 2*l1+3*arc+2*l2:
		return perimPoint{spec.Point{X: x0, Y: y1 - g.r - (sp - 2*l1 - 3*arc - l2)}, spec.Point{X: -1}}
	}
	return corner(x0+g.r, y0+g.r, math.Pi, sp-2*l1-3*arc-2*l2)
}

// wob is the hand wobble at pen length σ (docs/oni-animation-spec.md
// strand.region), positive outward.
func (g regionGeom) wob(sg float64) float64 {
	return g.wobble * (0.6*math.Sin(2*math.Pi*g.rate*sg+g.phi1) + 0.4*math.Sin(2*math.Pi*g.rate*1.7*sg+g.phi2))
}

// at is the pen point at length σ.
func (g regionGeom) at(sg float64) spec.Point {
	pp := g.perim(g.s0 + sg)
	return add(pp.p, scale(pp.n, g.wob(sg)))
}

func smooth(x float64) float64 { x = clamp01(x); return x * x * (3 - 2*x) }

// speed is the pen's relative speed at σ.
func (g regionGeom) speed(s *spec.OniSpec, sg float64) float64 {
	rs := s.Strand.Region
	cs := rs.CornerSpeed
	v := 1.0
	for _, c := range g.corners {
		if d := math.Abs(sg - c); d < rs.CornerSlowPx {
			v = math.Min(v, cs+(1-cs)*smooth(d/rs.CornerSlowPx))
		}
	}
	if d := math.Min(sg, g.total-sg); d < rs.SlowEnds {
		v = math.Min(v, cs+(1-cs)*smooth(d/rs.SlowEnds))
	}
	return v
}

// sigmaAt is how far the pen has drawn t seconds into its run.
func (g regionGeom) sigmaAt(t float64) float64 {
	if !(t > 0) {
		return 0
	}
	if t >= g.dur {
		return g.total
	}
	want := t / g.dur * g.tau[len(g.tau)-1]
	i := sort.SearchFloat64s(g.tau, want)
	if i <= 0 {
		return 0
	}
	a, b := g.tau[i-1], g.tau[i]
	return (float64(i-1) + (want-a)/(b-a)) * g.grid
}

// visible is the drawn pen line up to σ.
func (g regionGeom) visible(sg float64) []spec.Point {
	if !(sg > 0) {
		return nil
	}
	out := make([]spec.Point, 0, len(g.samples)+1)
	for _, x := range g.samples {
		if x >= sg-1e-9 {
			break
		}
		out = append(out, g.at(x))
	}
	return append(out, g.at(math.Min(sg, g.total)))
}

// char is the pen-sized character riding aheadPx along the line and
// outsidePx out from it.
func (g regionGeom) char(s *spec.OniSpec, sg float64, tip spec.Point) *CharPose {
	rs := s.Strand.Region
	pp := g.perim(g.s0 + sg)
	t := spec.Point{X: -pp.n.Y, Y: pp.n.X}
	return &CharPose{Pos: add(tip, add(scale(t, rs.AheadPx), scale(pp.n, rs.OutsidePx))), Scale: rs.PenScale}
}

// endRegion hands the drawn mark over to the lasso round the control: the
// pen's lead fades while the throw leaves the anchor.
func (e *Engine) endRegion(env Env) {
	pen := e.rg.visible(e.pen)
	e.fades, e.fadeT = nil, 0
	if len(pen) >= 2 {
		lead := bowed(e.s, env.Anchor, pen[len(pen)-1], env.Centre, e.s.Strand.Region.LeadPoints-1)
		e.fades = []Stroke{{Pts: lead, Width: e.s.Strand.Stroke.WidthPx, Alpha: 1}}
	}
	e.ls = newLasso(e.s, e.tgt.Box, env.Anchor)
	e.startLasso(false)
}
