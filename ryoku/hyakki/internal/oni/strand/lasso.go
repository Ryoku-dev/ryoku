package strand

import (
	"math"

	"ryoku-rashin/internal/oni/spec"
)

// lassoGeom is the ring round a control: a superellipse of power
// lasso.power about the box grown by gapPx, swept lasso.turns from the
// parameter nearest the character.
type lassoGeom struct {
	c        spec.Point
	hw, hh   float64
	th0      float64
	mainSegs int // segments of the main thread the throw reveals first
}

func spow(x, p float64) float64 { return math.Copysign(math.Pow(math.Abs(x), 2/p), x) }

func newLasso(s *spec.OniSpec, box Rect, a spec.Point) lassoGeom {
	gap := s.Strand.Lasso.GapPx
	g := lassoGeom{c: box.centre(), hw: box.W/2 + gap, hh: box.H/2 + gap}
	g.th0 = g.nearest(s, a)
	return g
}

// ringAt is the superellipse point at parameter th grown by extra px.
func (g lassoGeom) ringAt(s *spec.OniSpec, th, extra float64) spec.Point {
	p := s.Strand.Lasso.Power
	return spec.Point{
		X: g.c.X + (g.hw+extra)*spow(math.Cos(th), p),
		Y: g.c.Y + (g.hh+extra)*spow(math.Sin(th), p),
	}
}

// nearest is the ring parameter closest to a: a dense scan refined by a
// ternary search between its neighbours.
func (g lassoGeom) nearest(s *spec.OniSpec, a spec.Point) float64 {
	const n = 1440
	best, bd := 0.0, math.Inf(1)
	for i := range n {
		th := 2 * math.Pi * float64(i) / n
		if d := dist(g.ringAt(s, th, 0), a); d < bd {
			best, bd = th, d
		}
	}
	lo, hi := best-2*math.Pi/n, best+2*math.Pi/n
	for range 60 {
		m1, m2 := lo+(hi-lo)/3, hi-(hi-lo)/3
		if dist(g.ringAt(s, m1, 0), a) <= dist(g.ringAt(s, m2, 0), a) {
			hi = m2
		} else {
			lo = m1
		}
	}
	return (lo + hi) / 2
}

// extra is the ring's growth at sweep fraction f: thrown wide by thrownPx,
// cinched to 0 as c runs 0→1, with a decaying ripple of ripple.waves over
// the ring that the cinch also takes out.
func extra(s *spec.OniSpec, f, cinch, age float64) float64 {
	l := s.Strand.Lasso
	open := 1 - cinch
	ripple := l.Ripple.Amp * math.Exp(-age*l.Ripple.Decay) * math.Sin(2*math.Pi*l.Ripple.Waves*f)
	return (l.ThrownPx + ripple) * open
}

// at is the ring point at sweep fraction f with growth ext.
func (g lassoGeom) at(s *spec.OniSpec, f, ext float64) spec.Point {
	return g.ringAt(s, g.th0+2*math.Pi*s.Strand.Lasso.Turns*f, ext)
}

func (g lassoGeom) ringSegs(s *spec.OniSpec) int { return s.Strand.Lasso.Points - 1 }

func (g lassoGeom) total(s *spec.OniSpec) float64 { return float64(g.mainSegs + g.ringSegs(s)) }

// startLasso throws the ring. A fresh throw reveals the lead (from the live
// anchor) and the ring as one line; after a swing the main thread is
// already out, so only the ring draws.
func (e *Engine) startLasso(swung bool) {
	if !swung {
		e.path = nil
		e.ls.mainSegs = e.s.Strand.Lasso.LeadPoints - 1
		e.k0 = 0
	} else {
		e.ls.mainSegs = e.s.Strand.BowedPath.Segments
		e.k0 = float64(e.ls.mainSegs)
	}
	e.lasso = true
	e.kRev, e.cinch, e.age = e.k0, 0, 0
	e.phase, e.el = PhaseLasso, 0
}

// lassoProgress: the throw reveals over drawS, then the ring cinches over
// cinchS; the ripple ages from the throw.
func (e *Engine) lassoProgress() {
	l := e.s.Strand.Lasso
	e.age = e.el
	if e.el < l.DrawS {
		e.kRev = e.k0 + (e.ls.total(e.s)-e.k0)*ease(l.Ease, e.el, l.DrawS)
		e.cinch = 0
	} else {
		e.kRev = e.ls.total(e.s)
		e.cinch = ease(l.Ease, e.el-l.DrawS, l.CinchS)
	}
	if e.path != nil {
		e.u = clamp01(e.kRev / float64(e.ls.mainSegs))
	}
}

// lassoPoints is the main thread and the ring at the current reveal.
func (e *Engine) lassoPoints(env Env) (main, ring []spec.Point) {
	g := e.ls
	ms := float64(g.mainSegs)
	mainFrac := clamp01(e.kRev / ms)
	if e.path != nil {
		main = visible(e.path, mainFrac)
		if len(main) >= 2 {
			seam(e.s, main, e.a0, env.Anchor)
		}
	} else {
		start := g.at(e.s, 0, extra(e.s, 0, e.cinch, e.age))
		lead := bowed(e.s, env.Anchor, start, env.Centre, g.mainSegs)
		main = visible(lead, mainFrac)
	}
	rs := g.ringSegs(e.s)
	kf := math.Min(math.Max(0, e.kRev-ms), float64(rs))
	if !(kf > 0) {
		return main, nil
	}
	pt := func(k float64) spec.Point {
		f := k / float64(rs)
		return g.at(e.s, f, extra(e.s, f, e.cinch, e.age))
	}
	ring = make([]spec.Point, 0, int(kf)+2)
	for j := 0; float64(j) <= kf; j++ {
		ring = append(ring, pt(float64(j)))
	}
	if kf-math.Floor(kf) > 1e-12 {
		ring = append(ring, pt(kf))
	}
	return main, ring
}
