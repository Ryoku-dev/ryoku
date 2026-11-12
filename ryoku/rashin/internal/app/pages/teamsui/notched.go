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
// in the corner notch; the view's one hero card takes the mineral field instead
func NotchedSlab(gtx layout.Context, th *look.Theme, tone palette.Tone, inset unit.Dp, body layout.Widget) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	record := op.Record(gtx.Ops)
	dimensions := layout.UniformInset(inset).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return body(gtx)
	})
	call := record.Stop()
	bounds, cut := image.Rectangle{Max: dimensions.Size}, gtx.Dp(th.Cut.Mid)
	kit.SlabSurface(gtx, th.Theme, bounds, cut, th.R.Slab)
	kit.Notch(gtx.Ops, image.Point{}, gtx.Dp(th.Cut.Notch), th.R.Stone(tone))
	call.Add(gtx.Ops)
	return dimensions
}

// the view's one hero plane, the thing that needs the user first: the slab dyed by the attention
// mineral at low chroma, its own tone still in the notch, holding the view's one gold
func HeroSlab(gtx layout.Context, th *look.Theme, tone palette.Tone, body layout.Widget) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	dimensions := kit.TonedSlabContent(gtx, th.Theme, palette.Attention, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return body(gtx)
	})
	kit.Notch(gtx.Ops, image.Point{}, gtx.Dp(th.Cut.Notch), th.R.Stone(tone))
	return dimensions
}
