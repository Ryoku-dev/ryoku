package strand

import (
	"math"

	"ryoku-rashin/internal/oni/spec"
)

// bowed samples the strand's cubic from a to b with segs segments, both
// controls pushed from the chord toward centre by min(bowMax, dist·perDist)
// ·bowScale. With the spec's segment count it is spec.BowedPath exactly; the
// lasso and pen leads use the same curve with fewer points.
func bowed(s *spec.OniSpec, a, b, centre spec.Point, segs int) []spec.Point {
	bp := s.Strand.BowedPath
	dx, dy := b.X-a.X, b.Y-a.Y
	bow := math.Min(bp.BowMaxPx, math.Hypot(dx, dy)*bp.BowPerDistance) * bp.BowScale
	ox, oy := centre.X-(a.X+dx/2), centre.Y-(a.Y+dy/2)
	if n := math.Hypot(ox, oy); n > 0 {
		ox, oy = ox/n*bow, oy/n*bow
	} else {
		ox, oy = 0, 0
	}
	c1 := spec.Point{X: a.X + dx*bp.Controls[0] + ox, Y: a.Y + dy*bp.Controls[0] + oy}
	c2 := spec.Point{X: a.X + dx*bp.Controls[1] + ox, Y: a.Y + dy*bp.Controls[1] + oy}
	pts := make([]spec.Point, segs+1)
	for i := range pts {
		t := float64(i) / float64(segs)
		u := 1 - t
		w0, w1, w2, w3 := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
		pts[i] = spec.Point{
			X: w0*a.X + w1*c1.X + w2*c2.X + w3*b.X,
			Y: w0*a.Y + w1*c1.Y + w2*c2.Y + w3*b.Y,
		}
	}
	pts[0], pts[segs] = a, b
	return pts
}

// at is the polyline point at fractional index k (clamped).
func at(pts []spec.Point, k float64) spec.Point {
	n := len(pts)
	if n == 0 {
		return spec.Point{}
	}
	if !(k > 0) {
		return pts[0]
	}
	if k >= float64(n-1) {
		return pts[n-1]
	}
	i := int(k)
	return lerp(pts[i], pts[i+1], k-float64(i))
}

// visible is the drawn part of a polyline uniform in t at extension u: every
// sample up to u·(n−1) and the interpolated point at u. It is nil at u=0.
func visible(pts []spec.Point, u float64) []spec.Point {
	if !(u > 0) || len(pts) < 2 {
		return nil
	}
	k := math.Min(u, 1) * float64(len(pts)-1)
	i := int(k)
	out := make([]spec.Point, 0, i+2)
	out = append(out, pts[:i+1]...)
	if f := k - float64(i); f > 1e-12 {
		out = append(out, at(pts, k))
	}
	if len(out) < 2 {
		out = append(out, at(pts, k))
	}
	return out
}

// resample returns n points evenly spaced in index along pts.
func resample(pts []spec.Point, n int) []spec.Point {
	out := make([]spec.Point, n)
	if len(pts) == 0 {
		return out
	}
	for j := range out {
		out[j] = at(pts, float64(j)/float64(n-1)*float64(len(pts)-1))
	}
	return out
}

// seamWeight is how much of the anchor's drift the i-th point takes: 1 at
// the root, falling on a half cosine to 0 at the seam blend length.
func seamWeight(i, n int) float64 {
	if i >= n || n <= 0 {
		return 0
	}
	return 0.5 * (1 + math.Cos(math.Pi*float64(i)/float64(n)))
}

// seam blends the first seamBlendPoints of a path captured from a0 onto the
// live anchor, so the line keeps leaving the character as it moves. It
// writes into pts.
func seam(s *spec.OniSpec, pts []spec.Point, a0, anchor spec.Point) {
	d := sub(anchor, a0)
	if d.X == 0 && d.Y == 0 {
		return
	}
	n := s.Strand.BowedPath.SeamBlendPoints
	for i := 0; i < n && i < len(pts); i++ {
		pts[i] = add(pts[i], scale(d, seamWeight(i, n)))
	}
}

// tipAnchor is where a point reach lands: the nearest point of the box
// grown by tipGap, stopped tipStop short toward the anchor (or out along the
// nearest edge's normal when the anchor is inside).
func tipAnchor(s *spec.OniSpec, box Rect, a spec.Point) spec.Point {
	bp := s.Strand.BowedPath
	g := box.grow(bp.TipGapPx)
	p := spec.Point{X: math.Min(g.X+g.W, math.Max(g.X, a.X)), Y: math.Min(g.Y+g.H, math.Max(g.Y, a.Y))}
	if d := sub(a, p); math.Hypot(d.X, d.Y) > 1e-9 {
		return add(p, scale(unit(d), math.Min(bp.TipStopPx, math.Hypot(d.X, d.Y))))
	}
	// Inside: leave by the nearest edge.
	l, r, t, b := a.X-g.X, g.X+g.W-a.X, a.Y-g.Y, g.Y+g.H-a.Y
	switch m := math.Min(math.Min(l, r), math.Min(t, b)); m {
	case l:
		return spec.Point{X: g.X - bp.TipStopPx, Y: a.Y}
	case r:
		return spec.Point{X: g.X + g.W + bp.TipStopPx, Y: a.Y}
	case t:
		return spec.Point{X: a.X, Y: g.Y - bp.TipStopPx}
	}
	return spec.Point{X: a.X, Y: g.Y + g.H + bp.TipStopPx}
}

// ease reads a spec ease at progress t/d.
func ease(e spec.Ease, t, d float64) float64 {
	if !(d > 0) {
		return 1
	}
	return e.At(t / d)
}
