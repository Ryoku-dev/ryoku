package placement

import (
	"image"
	"sync"

	"ryoku-rashin/internal/oni/spec"
)

var defaultThreshold = sync.OnceValue(func() int {
	s, err := spec.Load()
	if err != nil {
		panic(err)
	}
	return s.Placement.DragThresholdPx
})

// DragState tells a click from a drag. Origin is the frame's top-left when
// the press lands; Threshold overrides the spec's drag threshold when set.
// The zero value is ready to use.
type DragState struct {
	Origin    image.Point
	Threshold int

	press    image.Point
	pressed  bool
	dragging bool
}

func (d *DragState) Press(p image.Point) {
	d.press, d.pressed, d.dragging = p, true, false
}

// Move reports whether the press has become a drag (once the pointer has
// travelled more than the threshold from the press) and where the frame's
// top-left now is; before that the frame stays at Origin.
func (d *DragState) Move(p image.Point) (dragging bool, pos image.Point) {
	if !d.pressed {
		return false, d.Origin
	}
	delta := p.Sub(d.press)
	if !d.dragging {
		t := d.Threshold
		if t <= 0 {
			t = defaultThreshold()
		}
		if delta.X*delta.X+delta.Y*delta.Y <= t*t {
			return false, d.Origin
		}
		d.dragging = true
	}
	return true, d.Origin.Add(delta)
}

// Release ends the press and reports whether it was a drag (false is a
// click).
func (d *DragState) Release() (dragged bool) {
	dragged = d.pressed && d.dragging
	d.pressed, d.dragging = false, false
	return dragged
}
