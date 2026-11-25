package strand

import (
	"math"

	"ryoku-rashin/internal/oni/spec"
)

// mainPoints is the captured main thread at extension u, with its root
// blended onto the live anchor, the tap bob on its tip and, for a perch,
// the curl round the frame corner.
func (e *Engine) mainPoints(env Env) []spec.Point {
	pts := visible(e.path, e.u)
	if len(pts) < 2 {
		return nil
	}
	seam(e.s, pts, e.a0, env.Anchor)
	if e.phase == PhaseTap && e.tapV > 0 {
		// The tip bobs sin(v·π)·bob px on into the target and back.
		n := len(e.path)
		dir := unit(sub(e.path[n-1], e.path[n-2]))
		bob := math.Sin(e.tapV*math.Pi) * e.s.Strand.Tap.BobPx
		pts[len(pts)-1] = add(pts[len(pts)-1], scale(dir, bob))
	}
	if e.k == kPerch && e.curl > 0 && e.u >= 1 {
		pts = append(pts, e.pr.arc(e.s, e.curl)...)
	}
	return pts
}

// burstLines are the short lines that spring from the tip when a card
// opens: lines·(anglePi + i·step)·π, from fromPx out, growing growPx over
// the burst, fading as 1−v.
func burstLines(s *spec.OniSpec, tip spec.Point, t float64) []Stroke {
	b := s.Strand.Burst
	v := clamp01(t / b.DurationS)
	out := make([]Stroke, 0, b.Lines)
	inner := b.FromPx + b.GrowPx*v
	for i := range b.Lines {
		a := (b.AnglePi + float64(i)*b.Step) * math.Pi
		d := spec.Point{X: math.Cos(a), Y: math.Sin(a)}
		out = append(out, Stroke{
			Pts:   []spec.Point{add(tip, scale(d, inner)), add(tip, scale(d, inner+b.GrowPx))},
			Width: b.WidthPx,
			Alpha: 1 - v,
		})
	}
	return out
}
