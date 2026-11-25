package strand

import "ryoku-rashin/internal/oni/spec"

// startSwing retargets a thread that is out: the tip swings from where it
// is to the new landing without going home first. The new path's extension
// restarts at max(retarget.reachFrom, current extension) and reaches 1 over
// retargetBlendMs while the old main thread blends into it; strokes the old
// target had besides its main thread (ring, pen, curl) fade over the same
// time.
func (e *Engine) startSwing(a, land spec.Point, uOld float64, env Env) {
	n := e.s.Strand.BowedPath.Points
	old := e.last.Strokes
	e.from = resample(old[0].Pts, n)
	e.fades = e.fades[:0:0]
	for _, st := range old[1:] {
		e.fades = append(e.fades, Stroke{Pts: st.Pts, Width: st.Width, Alpha: st.Alpha})
	}
	e.fadeT = 0
	e.a0 = a
	e.path = bowed(e.s, a, land, env.Centre, e.s.Strand.BowedPath.Segments)
	e.uFrom = max(e.s.Strand.Retarget.ReachFrom, uOld)
	e.u = e.uFrom
	e.tapV, e.curl, e.lasso, e.kRev, e.k0, e.cinch, e.age, e.pen = 0, 0, false, 0, 0, 0, 0, 0
	e.phase, e.el = PhaseSwing, 0
}

// extent is how far out the main thread is, 0 home to 1 fully out.
func (e *Engine) extent() float64 {
	switch {
	case e.phase == PhaseHome:
		return 0
	case e.phase == PhaseSwing || e.k == kPoint || e.k == kPerch:
		return e.u
	case e.k == kLasso || e.lasso:
		return clamp01(e.kRev / float64(e.ls.mainSegs))
	}
	return 1
}

// swingPoints blends the old main thread into the new path at the swing's
// eased weight, then rides the root on the live anchor.
func (e *Engine) swingPoints(env Env) []spec.Point {
	n := e.s.Strand.BowedPath.Points
	w := ease(e.s.Strand.Retarget.Ease, e.el, e.s.Strand.RetargetBlendMs/1000)
	nv := resample(visible(e.path, e.u), n)
	out := make([]spec.Point, n)
	for i := range out {
		out[i] = lerp(e.from[i], nv[i], w)
	}
	seam(e.s, out, e.a0, env.Anchor)
	return out
}

// endSwing lands the swing on the new target's own phase.
func (e *Engine) endSwing() {
	e.u, e.from = 1, nil
	switch e.k {
	case kPoint:
		e.phase, e.tapV = PhaseTap, 0
	case kPerch:
		e.phase, e.curl = PhaseCurl, 0
	case kLasso:
		e.startLasso(true)
	case kRegion:
		e.path, e.pen = nil, 0
		e.phase = PhaseRegion
	}
}
