package teams

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
)

const (
	tileMin  unit.Dp = 320
	tileMax  unit.Dp = 420
	tileTall unit.Dp = 154
	tileGap  unit.Dp = 8
	// a list element measures against no height limit
	unbounded = 1 << 24
)

// the board is centred up to its width, inset from the pane like the header's title
func boardColumn(gtx layout.Context, e *env.Env, child layout.Widget) layout.Dimensions {
	th := e.Theme
	return layout.Inset{Top: th.Sp.S4, Bottom: th.Sp.S5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return look.Centred(gtx, e.Class, look.BoardWidth, child)
	})
}

// a tab's body stands inset from the pane's sides like the header's title
func pageInset(gtx layout.Context, e *env.Env, child layout.Widget) layout.Dimensions {
	th := e.Theme
	side := look.ContentInset(e.Class)
	return layout.Inset{Top: th.Sp.S4, Bottom: th.Sp.S5, Left: side, Right: side}.Layout(gtx, child)
}

// the sections of a column, a nil one is left out
func sections(gtx layout.Context, widgets ...layout.Widget) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(widgets))
	for _, widget := range widgets {
		if widget != nil {
			children = append(children, layout.Rigid(widget))
		}
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func section(th *look.Theme, title string, body layout.Widget) layout.Widget {
	return kit.Section(th.Theme, title, body)
}

// a quiet line said where a section has nothing to show
func quiet(th *look.Theme, value string) layout.Widget {
	return look.Paragraph(th, th.T.Caption, th.R.Fg3, value)
}

// tiles fill rows as wide as they may be, each row as high as its tallest tile so the slabs line up
func tileGrid(gtx layout.Context, count int, tile func(gtx layout.Context, index int) layout.Dimensions) layout.Dimensions {
	available := gtx.Constraints.Max.X
	if count == 0 || available <= 0 {
		return layout.Dimensions{}
	}
	gap, least, most, tall := gtx.Dp(tileGap), gtx.Dp(tileMin), gtx.Dp(tileMax), gtx.Dp(tileTall)
	columns := max(1, (available+gap)/(least+gap))
	width := min(most, (available-gap*(columns-1))/columns)
	y := 0
	for start := 0; start < count; start += columns {
		end := min(count, start+columns)
		height := tall
		for index := start; index < end; index++ {
			measure := gtx
			measure.Constraints = layout.Constraints{Min: image.Pt(width, tall), Max: image.Pt(width, unbounded)}
			record := op.Record(gtx.Ops)
			height = max(height, tile(measure, index).Size.Y)
			record.Stop()
		}
		for index := start; index < end; index++ {
			child := gtx
			child.Constraints = layout.Exact(image.Pt(width, height))
			stack := op.Offset(image.Pt((index-start)*(width+gap), y)).Push(gtx.Ops)
			tile(child, index)
			stack.Pop()
		}
		y += height + gap
	}
	return layout.Dimensions{Size: image.Pt(available, y-gap)}
}
