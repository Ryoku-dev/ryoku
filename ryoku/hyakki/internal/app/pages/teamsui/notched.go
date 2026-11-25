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

// a card that asks for attention is the design's card with its tone at the corner, a notch in
// Lithic and a dot where corners are round; the view's one hero card takes the mineral field instead
func NotchedSlab(gtx layout.Context, th *look.Theme, tone palette.Tone, inset unit.Dp, body layout.Widget) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	record := op.Record(gtx.Ops)
	dimensions := layout.UniformInset(markRoom(th, tone, inset)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return body(gtx)
	})
	call := record.Stop()
	bounds := image.Rectangle{Max: dimensions.Size}
	kit.SlabSurface(gtx, th.Theme, bounds, gtx.Dp(th.Cut.Mid), th.R.Slab)
	kit.ToneMark(gtx, th.Theme, bounds, tone)
	call.Add(gtx.Ops)
	return dimensions
}

// the view's one hero plane, the thing that needs the user first: the card dyed by the attention
// mineral at low chroma, its own tone still at the corner, holding the view's one gold
func HeroSlab(gtx layout.Context, th *look.Theme, tone palette.Tone, body layout.Widget) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	dimensions := kit.TonedSlabContent(gtx, th.Theme, palette.Attention, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return body(gtx)
	})
	// the slab's own inset keeps the words clear of the mark in every design
	kit.ToneMark(gtx, th.Theme, image.Rectangle{Max: dimensions.Size}, tone)
	return dimensions
}

// a round corner holds its dot further in than a notch reaches, so a tight card gives it room
func markRoom(th *look.Theme, tone palette.Tone, inset unit.Dp) unit.Dp {
	if tone != palette.Idle && th.Design.Profile().Round {
		return max(inset, th.Sp.S3)
	}
	return inset
}
