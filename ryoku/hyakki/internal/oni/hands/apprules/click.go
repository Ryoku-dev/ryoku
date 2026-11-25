package apprules

import (
	"errors"

	"ryoku-rashin/internal/oni/window"
)

// Region is a part of the target window as fractions of its box: X and Y
// its top-left corner, W and H its size, each within 0 and 1.
type Region struct{ X, Y, W, H float64 }

// Why a click is refused. A click never lands on a window Hands cannot
// place, outside the target, or where another window may sit on top.
var (
	ErrUnplaced = errors.New("the window's place is not known")
	ErrRegion   = errors.New("the point is outside the window")
	ErrHidden   = errors.New("the window is not shown")
	ErrCovered  = errors.New("another window sits over the point")
)

// Click hit-tests one click at the centre of region in target and returns
// the point in target's output-local logical pixels. The target must be
// shown and placed (a tiled window whose box the compositor does not report
// takes keys only), the region must lie inside it, and no other shown window
// on the same output may hold the point: stacking order is not known, so a
// window that overlaps the point may be on top. There is no drag: a click is
// one press and release at one point.
func Click(target window.Window, region Region, others []window.Window) (x, y float64, err error) {
	if !target.Visible || target.Minimized {
		return 0, 0, ErrHidden
	}
	b := target.Box
	if b == nil || b.W <= 0 || b.H <= 0 {
		return 0, 0, ErrUnplaced
	}
	if !(region.X >= 0 && region.Y >= 0 && region.W >= 0 && region.H >= 0 &&
		region.X+region.W <= 1 && region.Y+region.H <= 1) {
		return 0, 0, ErrRegion
	}
	x = b.X + (region.X+region.W/2)*b.W
	y = b.Y + (region.Y+region.H/2)*b.H
	for _, o := range others {
		if o.ID == target.ID || o.Output != target.Output || !o.Visible || o.Minimized {
			continue
		}
		if o.Box == nil {
			if o.Fullscreen || o.Floating {
				// it is on top somewhere on this output, and its place is not known
				return 0, 0, ErrCovered
			}
			continue
		}
		ob := o.Box
		if x >= ob.X && x < ob.X+ob.W && y >= ob.Y && y < ob.Y+ob.H {
			return 0, 0, ErrCovered
		}
	}
	return x, y, nil
}
