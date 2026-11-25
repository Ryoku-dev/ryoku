package strand

import (
	"math"

	"ryoku-rashin/internal/oni/spec"
)

// Perch corners, as the perch frame names them.
const (
	CornerTopLeft  = "top-left"
	CornerTopRight = "top-right"
)

// perches reports whether a point target is a whole window or big panel:
// the target and its frame both at least perch.minWidthPx × minHeightPx. A
// perch-mode target always perches.
func perches(s *spec.OniSpec, t Target) bool {
	if t.Mode == ModePerch {
		return true
	}
	p := s.Strand.Perch
	big := func(r Rect) bool { return r.W >= p.MinWidthPx && r.H >= p.MinHeightPx }
	return t.Frame != nil && big(*t.Frame) && big(t.Box)
}

// perchFrame is the frame a perch curls on: Frame when set, else (perch
// mode) the box itself.
func perchFrame(t Target) Rect {
	if t.Frame != nil {
		return *t.Frame
	}
	return t.Box
}

// PerchCorner picks the top corner of frame nearer from (a tie takes the
// corner nearer centre): the engine's rule for a target that names no
// corner, and the one the daemon or host uses to name it before the
// character flies. On a known screen a corner whose curl would run off the
// output on the outer side (strand.perch.radiusPx past the corner) is passed
// over while the other corner's curl fits.
func PerchCorner(s *spec.OniSpec, frame Rect, from, centre spec.Point, screen Rect) string {
	tl := spec.Point{X: frame.X, Y: frame.Y}
	tr := spec.Point{X: frame.X + frame.W, Y: frame.Y}
	if !screen.empty() {
		r := s.Strand.Perch.RadiusPx
		leftFits := tl.X-r >= screen.X
		rightFits := tr.X+r <= screen.X+screen.W
		if leftFits != rightFits {
			if rightFits {
				return CornerTopRight
			}
			return CornerTopLeft
		}
	}
	dl, dr := dist(from, tl), dist(from, tr)
	right := dr < dl
	if dl == dr {
		right = dist(centre, tr) < dist(centre, tl)
	}
	if right {
		return CornerTopRight
	}
	return CornerTopLeft
}

// PerchSeat is where the character stands while perched: the output-local
// top-left of its side × side frame. It sits outside the corner, its inner
// edge perch.seat.gapPx beyond the window's side and its bottom
// perch.seat.liftPx above the top edge. An axis that would cross
// perch.seat.marginPx of the screen flips to the inside of the window; the
// frame is then clamped marginPx inside the screen (an axis too short for
// it centres). An empty screen clamps nothing.
func PerchSeat(s *spec.OniSpec, frame Rect, corner string, side float64, screen Rect) spec.Point {
	st := s.Perch.Seat
	known := !screen.empty()
	lo := func(v, edge float64) bool { return known && v < edge+st.MarginPx }
	hi := func(v, edge float64) bool { return known && v > edge-st.MarginPx }
	var x float64
	if corner == CornerTopRight {
		x = frame.X + frame.W + st.GapPx
		if hi(x+side, screen.X+screen.W) {
			x = frame.X + frame.W - st.GapPx - side
		}
	} else {
		x = frame.X - st.GapPx - side
		if lo(x, screen.X) {
			x = frame.X + st.GapPx
		}
	}
	y := frame.Y - st.LiftPx - side
	if lo(y, screen.Y) {
		y = frame.Y + st.LiftPx
	}
	if known {
		x = clampSeat(x, screen.X, screen.W, side, st.MarginPx)
		y = clampSeat(y, screen.Y, screen.H, side, st.MarginPx)
	}
	return spec.Point{X: x, Y: y}
}

func clampSeat(v, lo, span, size, margin float64) float64 {
	minV, maxV := lo+margin, lo+span-margin-size
	if minV > maxV {
		return lo + (span-size)/2
	}
	return math.Min(math.Max(v, minV), maxV)
}

// PerchJumps reports whether a seat moving from one point to another jumps
// with its window (at most perch.reattach.snapPx) rather than flying.
func PerchJumps(s *spec.OniSpec, from, to spec.Point) bool {
	return dist(from, to) <= s.Perch.Reattach.SnapPx
}

// perchGeom is the curl round one top corner of a window frame. The thread
// lands on the top edge offsetPx in from the corner, then curls on the
// outside of the frame: an arc about the corner sweeping sweep·π (over the
// top, round the outer side and down it), its radius easing from offsetPx
// to radiusPx, sampled in perch.segments.
type perchGeom struct {
	c    spec.Point
	sx   float64 // +1 top-right corner, −1 top-left
	land spec.Point
}

// newPerch builds the curl on the named corner of f.
func newPerch(s *spec.OniSpec, f Rect, corner string) perchGeom {
	g := perchGeom{c: spec.Point{X: f.X, Y: f.Y}, sx: -1}
	if corner == CornerTopRight {
		g = perchGeom{c: spec.Point{X: f.X + f.W, Y: f.Y}, sx: 1}
	}
	g.land = spec.Point{X: g.c.X - g.sx*s.Strand.Perch.OffsetPx, Y: g.c.Y}
	return g
}

// corner names the corner the curl wraps.
func (g perchGeom) corner() string {
	if g.sx > 0 {
		return CornerTopRight
	}
	return CornerTopLeft
}

// at is the curl point at fraction f of its sweep.
func (g perchGeom) at(s *spec.OniSpec, f float64) spec.Point {
	p := s.Strand.Perch
	start := 0.0
	if g.sx > 0 {
		start = math.Pi
	}
	th := start + g.sx*p.Sweep*math.Pi*f
	r := p.OffsetPx + (p.RadiusPx-p.OffsetPx)*f
	return spec.Point{X: g.c.X + r*math.Cos(th), Y: g.c.Y + r*math.Sin(th)}
}

// arc is the drawn curl after the landing point at curl fraction k.
func (g perchGeom) arc(s *spec.OniSpec, k float64) []spec.Point {
	n := s.Strand.Perch.Segments
	kf := clamp01(k) * float64(n)
	out := make([]spec.Point, 0, int(kf)+1)
	for j := 1; float64(j) <= kf; j++ {
		out = append(out, g.at(s, float64(j)/float64(n)))
	}
	if kf-math.Floor(kf) > 1e-12 {
		out = append(out, g.at(s, kf/float64(n)))
	}
	return out
}
