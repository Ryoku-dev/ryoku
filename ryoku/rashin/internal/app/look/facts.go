package look

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
)

type FactRow struct {
	Label  string
	Value  string
	Mono   bool
	Widget layout.Widget
}

func Facts(gtx layout.Context, th *Theme, rows []FactRow) layout.Dimensions {
	label, height, gap := gtx.Dp(112), gtx.Dp(32), gtx.Dp(13)
	width := gtx.Constraints.Max.X
	for index, row := range rows {
		y := index * height
		labels := gtx
		labels.Constraints = layout.Constraints{Max: image.Pt(label, height)}
		offset := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
		MiddleLeft(labels, 0, height, kit.RawText(th.Theme, th.T.Caption, th.R.Fg3, row.Label))
		offset.Pop()

		value := row.Widget
		if value == nil {
			style := th.T.Body
			if row.Mono {
				style = th.T.Mono
			}
			value = kit.RawText(th.Theme, style, th.R.Fg, row.Value)
		}
		values := gtx
		values.Constraints = layout.Constraints{Max: image.Pt(max(0, width-label-gap), height)}
		offset = op.Offset(image.Pt(label+gap, y)).Push(gtx.Ops)
		MiddleLeft(values, 0, height, value)
		offset.Pop()
	}
	return layout.Dimensions{Size: image.Pt(width, len(rows)*height)}
}

func Rule(gtx layout.Context, th *Theme) layout.Dimensions {
	size := image.Pt(gtx.Constraints.Max.X, max(1, gtx.Dp(1)))
	FillRect(gtx, image.Rectangle{Max: size}, th.R.Vein)
	return layout.Dimensions{Size: size}
}
