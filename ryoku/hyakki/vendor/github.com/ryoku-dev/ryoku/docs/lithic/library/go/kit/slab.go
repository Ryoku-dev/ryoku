package kit

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// SlabContent lays out a standard slab: the design's card plane (SlabSurface)
// inside the mid cut and the canonical inset.
func SlabContent(gtx layout.Context, th *theme.Theme, body layout.Widget) layout.Dimensions {
	return TonedSlabContent(gtx, th, palette.Idle, body)
}

// TonedSlabContent lays out a slab dyed by tone: the one hero plane of a view,
// such as the thing that needs you. Idle is the plain slab.
func TonedSlabContent(gtx layout.Context, th *theme.Theme, tone palette.Tone, body layout.Widget) layout.Dimensions {
	fill := th.R.Slab
	if tone != palette.Idle {
		fill = th.R.MineralField(tone)
	}
	return slabContent(gtx, th, body, func(gtx layout.Context, bounds image.Rectangle, cut int) {
		SlabSurface(gtx, th, bounds, cut, fill)
	})
}

// OutlinedSlabContent lays out a slab that stands on another card, in a dialog
// or in a popover: SlabOutline inside the mid cut and the canonical inset.
func OutlinedSlabContent(gtx layout.Context, th *theme.Theme, body layout.Widget) layout.Dimensions {
	return slabContent(gtx, th, body, func(gtx layout.Context, bounds image.Rectangle, cut int) {
		SlabOutline(gtx, th, bounds, cut)
	})
}

func slabContent(gtx layout.Context, th *theme.Theme, body layout.Widget, surface func(layout.Context, image.Rectangle, int)) layout.Dimensions {
	record := op.Record(gtx.Ops)
	dimensions := layout.UniformInset(tokens.SizeSlabInset).Layout(gtx, body)
	call := record.Stop()
	surface(gtx, image.Rectangle{Max: dimensions.Size}, gtx.Dp(th.Cut.Mid))
	call.Add(gtx.Ops)
	return dimensions
}

// SlabSurface paints a slab's plane in bounds. A plain slab (fill is the slab
// role) is the design's card, th.Planes().Card with its CardStroke: in Lithic
// the paper it stands on, drawn by its vein hairline alone; in Material and
// Adwaita a tonal card with no line; in Fluent a tonal card with a thin
// outline. Any other fill, a mineral field or a caller's tone, is filled as it
// is and carries no line. The corners follow the design from cut.
//
// A plain slab is a card on the page pane. One that stands on another card, in
// a dialog or in a popover would lie tone on tone there: use SlabOutline.
func SlabSurface(gtx layout.Context, th *theme.Theme, bounds image.Rectangle, cut int, fill color.NRGBA) {
	if fill != th.R.Slab {
		Fill(gtx, th, bounds, cut, fill)
		return
	}
	planes := th.Planes()
	if planes.Card != th.R.Slab {
		Fill(gtx, th, bounds, cut, planes.Card)
	}
	Stroke(gtx, th, bounds, cut, max(1, gtx.Dp(unit.Dp(tokens.SizeHairline))), planes.CardStroke)
}

// SlabOutline paints a slab by its vein hairline alone, in every design, as
// the Hub draws a slab it gives no card plane: for a slab nested in a card, a
// dialog or a popover. The corners follow the design from cut.
func SlabOutline(gtx layout.Context, th *theme.Theme, bounds image.Rectangle, cut int) {
	Stroke(gtx, th, bounds, cut, max(1, gtx.Dp(unit.Dp(tokens.SizeHairline))), th.R.Vein)
}

// SlabRows lays out rows inside a standard slab at full width. Space, not a
// rule, separates them.
func SlabRows(gtx layout.Context, th *theme.Theme, rows ...layout.Widget) layout.Dimensions {
	return SlabContent(gtx, th, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		children := make([]layout.FlexChild, 0, max(0, len(rows)*2-1))
		for index, row := range rows {
			if index > 0 {
				children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout))
			}
			children = append(children, layout.Rigid(row))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}
