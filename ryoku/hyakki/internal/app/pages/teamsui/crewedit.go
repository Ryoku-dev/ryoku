package teamsui

import (
	"image"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"

	"ryoku-rashin/internal/app/look"
)

// CrewEditor changes only the containing form's draft. The reporting picker is
// its keyboard equivalent; the containing form validates before its atomic save.
type CrewEditor struct {
	drag          gesture.Drag
	from          int
	active, moved bool
	start         image.Point
	// Reparent receives source and target member indices. -1 is the Summoner.
	Reparent func(from, to int)
}

func (e *CrewEditor) layout(gtx layout.Context, th *look.Theme, size image.Point, positions []image.Point, tiles []placedTile, head image.Rectangle) {
	hit := func(p image.Point) int {
		if p.In(head) {
			return -1
		}
		for i, at := range positions {
			if p.In(image.Rectangle{Min: at, Max: at.Add(tiles[i].size)}) {
				return i
			}
		}
		return -2
	}
	for {
		ev, ok := e.drag.Update(gtx.Metric, gtx.Source, gesture.Both)
		if !ok {
			break
		}
		p := ev.Position.Round()
		switch ev.Kind {
		case pointer.Press:
			e.from, e.start, e.moved = hit(p), p, false
			e.active = e.from >= 0
		case pointer.Drag:
			d := p.Sub(e.start)
			e.moved = e.moved || d.X*d.X+d.Y*d.Y >= gtx.Dp(6)*gtx.Dp(6)
		case pointer.Release:
			to := hit(p)
			if e.active && e.moved && to >= -1 && to != e.from && e.Reparent != nil {
				e.Reparent(e.from, to)
				gtx.Execute(op.InvalidateCmd{})
			}
			e.active = false
		case pointer.Cancel:
			e.active = false
		}
	}
	if e.active && e.from < len(positions) {
		at := positions[e.from]
		kit.SlabOutline(gtx, th.Theme, image.Rectangle{Min: at, Max: at.Add(tiles[e.from].size)}, gtx.Dp(th.Cut.Small))
	}
	area := clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops)
	e.drag.Add(gtx.Ops)
	pointer.CursorGrab.Add(gtx.Ops)
	area.Pop()
}
