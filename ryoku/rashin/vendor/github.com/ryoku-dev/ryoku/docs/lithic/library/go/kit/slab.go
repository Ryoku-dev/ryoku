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

// SlabContent lays out a standard slab: the slab fill lifted off its plane,
// set off by a vein hairline in light mode, inside the mid cut and the
// canonical inset.
func SlabContent(gtx layout.Context, th *theme.Theme, body layout.Widget) layout.Dimensions {
	return TonedSlabContent(gtx, th, palette.Idle, body)
}

// TonedSlabContent lays out a slab dyed by tone: the one hero plane of a view,
// such as the thing that needs you. Idle is the plain slab.
func TonedSlabContent(gtx layout.Context, th *theme.Theme, tone palette.Tone, body layout.Widget) layout.Dimensions {
	record := op.Record(gtx.Ops)
	dimensions := layout.UniformInset(tokens.SizeSlabInset).Layout(gtx, body)
	call := record.Stop()
	bounds := image.Rectangle{Max: dimensions.Size}
	cut := gtx.Dp(th.Cut.Mid)
	fill := th.R.Slab
	if tone != palette.Idle {
		fill = th.R.MineralField(tone)
	}
	SlabSurface(gtx, th, bounds, cut, fill)
	call.Add(gtx.Ops)
	return dimensions
}

// SlabSurface paints a slab's plane in bounds: the fill, plus a vein hairline
// in light mode, where a lighter slab alone would not separate.
func SlabSurface(gtx layout.Context, th *theme.Theme, bounds image.Rectangle, cut int, fill color.NRGBA) {
	FillCut(gtx.Ops, bounds, cut, fill)
	if !th.R.Dark {
		StrokeCut(gtx.Ops, bounds, cut, max(1, gtx.Dp(unit.Dp(tokens.SizeHairline))), th.R.Vein)
	}
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
