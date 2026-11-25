package strand

import (
	"math"
	"sort"

	"ryoku-rashin/internal/oni/spec"
)

// BubbleRect places a card of size (width capped at bubble.maxWidthPx) by
// the tip so it never covers the target: it sits outside the target grown
// by bowedPath.tipGapPx + tipStopPx (where a point reach stops, so the tip
// touches the card's edge), on the side the thread approached from (approach
// is the direction the tip travelled into the target, Frame.Approach),
// centred on the tip along that side and kept on screen. Sides are tried
// from the one facing the thread round to the far side; when none fits the
// screen it takes the side with the most room and may run off screen, still
// clear of the target. For a lasso pass the ring's bounds as target so the
// card sits beside the ring.
func BubbleRect(target Rect, tip, approach spec.Point, size spec.Point, screen Rect, s *spec.OniSpec) Rect {
	w := math.Min(size.X, s.Strand.Bubble.MaxWidthPx)
	h := size.Y
	bp := s.Strand.BowedPath
	g := target.grow(bp.TipGapPx + bp.TipStopPx)
	back := scale(unit(approach), -1)
	if back == (spec.Point{}) {
		back = unit(sub(tip, target.centre()))
	}
	if back == (spec.Point{}) {
		back = spec.Point{Y: -1}
	}
	type side struct {
		n    spec.Point
		room float64
	}
	sides := []side{
		{spec.Point{X: -1}, g.X - screen.X},
		{spec.Point{X: 1}, screen.X + screen.W - (g.X + g.W)},
		{spec.Point{Y: -1}, g.Y - screen.Y},
		{spec.Point{Y: 1}, screen.Y + screen.H - (g.Y + g.H)},
	}
	sort.SliceStable(sides, func(i, j int) bool {
		return back.X*sides[i].n.X+back.Y*sides[i].n.Y > back.X*sides[j].n.X+back.Y*sides[j].n.Y
	})
	along := func(c, size, lo, hi float64) float64 {
		x := c - size/2
		if size <= hi-lo {
			x = math.Min(hi-size, math.Max(lo, x))
		} else {
			x = lo
		}
		return x
	}
	place := func(sd side) Rect {
		switch {
		case sd.n.X < 0:
			return Rect{g.X - w, along(tip.Y, h, screen.Y, screen.Y+screen.H), w, h}
		case sd.n.X > 0:
			return Rect{g.X + g.W, along(tip.Y, h, screen.Y, screen.Y+screen.H), w, h}
		case sd.n.Y < 0:
			return Rect{along(tip.X, w, screen.X, screen.X+screen.W), g.Y - h, w, h}
		}
		return Rect{along(tip.X, w, screen.X, screen.X+screen.W), g.Y + g.H, w, h}
	}
	need := func(sd side) float64 {
		if sd.n.X != 0 {
			return w
		}
		return h
	}
	for _, sd := range sides {
		if sd.room >= need(sd) {
			return place(sd)
		}
	}
	best := sides[0]
	for _, sd := range sides[1:] {
		if sd.room-need(sd) > best.room-need(best) {
			best = sd
		}
	}
	return place(best)
}
