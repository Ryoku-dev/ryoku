package teamsui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"

	"ryoku-rashin/internal/app/look"
)

// a card that asks for attention lifts off its pane as a slab does and carries its tone
// in the corner notch; a mineral field is left to the header of the thing that is selected
func NotchedSlab(gtx layout.Context, th *look.Theme, tone palette.Tone, inset unit.Dp, body layout.Widget) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	record := op.Record(gtx.Ops)
	dimensions := layout.UniformInset(inset).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return body(gtx)
	})
	call := record.Stop()
	bounds, cut := image.Rectangle{Max: dimensions.Size}, gtx.Dp(th.Cut.Mid)
	kit.FillCut(gtx.Ops, bounds, cut, th.R.Slab)
	kit.StrokeCut(gtx.Ops, bounds, cut, max(1, gtx.Dp(1)), th.R.Vein)
	kit.Notch(gtx.Ops, image.Point{}, gtx.Dp(th.Cut.Notch), th.R.Stone(tone))
	call.Add(gtx.Ops)
	return dimensions
}
