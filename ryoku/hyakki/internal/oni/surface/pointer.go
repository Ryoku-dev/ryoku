package surface

import (
	"image"
	"math"
	"time"

	"ryoku-rashin/internal/oni/placement"
	"ryoku-rashin/internal/oni/spec"
)

// grip follows one press on the character: a click, or once the pointer has
// travelled past the drag threshold, a drag. Points are global logical px.
type grip struct {
	drag     placement.DragState
	pressed  bool
	dragging bool
	at       time.Time
	last     spec.Point
	vel      spec.Point
}

func (g *grip) press(origin image.Point, p spec.Point, now time.Time) {
	g.drag.Origin = origin
	g.drag.Press(roundPt(p))
	g.pressed, g.dragging = true, false
	g.at, g.last, g.vel = now, p, spec.Point{}
}

// move follows the pointer. started is true on the move that turns the press
// into a drag; pos is the frame's top-left while dragging.
func (g *grip) move(p spec.Point, now time.Time) (started, dragging bool, pos image.Point) {
	if !g.pressed {
		return false, false, image.Point{}
	}
	if dt := now.Sub(g.at).Seconds(); dt > 0 {
		v := spec.Point{X: (p.X - g.last.X) / dt, Y: (p.Y - g.last.Y) / dt}
		g.vel = spec.Point{X: (g.vel.X + v.X) / 2, Y: (g.vel.Y + v.Y) / 2}
	}
	g.at, g.last = now, p
	dragging, pos = g.drag.Move(roundPt(p))
	started = dragging && !g.dragging
	g.dragging = dragging
	return started, dragging, pos
}

// release ends the press; dragged is false for a click.
func (g *grip) release() (dragged bool) {
	dragged = g.drag.Release()
	g.pressed, g.dragging, g.vel = false, false, spec.Point{}
	return dragged
}

func roundPt(p spec.Point) image.Point {
	return image.Pt(int(math.Round(p.X)), int(math.Round(p.Y)))
}
