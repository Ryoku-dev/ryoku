package chat

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"

	"ryoku-rashin/internal/app/look"
)

// a slab of the transcript: the design's card on the pane, with tone a state mark on its corner
func (state *viewState) slab(gtx layout.Context, th *look.Theme, inset unit.Dp, body layout.Widget) layout.Dimensions {
	return state.tonedSlab(gtx, th, palette.Idle, inset, body)
}

func (state *viewState) tonedSlab(gtx layout.Context, th *look.Theme, tone palette.Tone, inset unit.Dp, body layout.Widget) layout.Dimensions {
	record := op.Record(gtx.Ops)
	state.nest++
	dimensions := layout.UniformInset(inset).Layout(gtx, body)
	state.nest--
	call := record.Stop()
	bounds := image.Rectangle{Max: dimensions.Size}
	state.plane(gtx, th, bounds)
	kit.ToneMark(gtx, th.Theme, bounds, tone)
	call.Add(gtx.Ops)
	return dimensions
}

// a slab inside another card is drawn by its hairline alone, a card on a card would lie tone on tone in the round designs
func (state *viewState) plane(gtx layout.Context, th *look.Theme, bounds image.Rectangle) {
	cut := gtx.Dp(th.Cut.Mid)
	if state.nest > 0 {
		kit.SlabOutline(gtx, th.Theme, bounds, cut)
		return
	}
	kit.SlabSurface(gtx, th.Theme, bounds, cut, th.R.Slab)
}

// icon-only controls say their name to a screen reader alone, so only glyphs everyone reads go bare
func (state *viewState) icon(gtx layout.Context, th *look.Theme, key string, icon *widget.Icon, description string) (layout.Dimensions, bool) {
	control := state.icons[key]
	if control == nil {
		control = &kit.IconPlaque{Kind: kit.PlaqueQuiet, Small: true}
		state.icons[key] = control
	}
	control.Icon, control.Description = icon, description
	clicked := control.Clicked(gtx)
	return control.Layout(gtx, th.Theme), clicked
}
