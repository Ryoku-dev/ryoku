package kit

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// SlabContent lays out a standard slab: the slab fill lifted off its plane, a
// vein hairline inside the mid cut, and the canonical inset.
func SlabContent(gtx layout.Context, th *theme.Theme, body layout.Widget) layout.Dimensions {
	record := op.Record(gtx.Ops)
	dimensions := layout.UniformInset(tokens.SizeSlabInset).Layout(gtx, body)
	call := record.Stop()
	bounds := image.Rectangle{Max: dimensions.Size}
	cut := gtx.Dp(th.Cut.Mid)
	FillCut(gtx.Ops, bounds, cut, th.R.Slab)
	StrokeCut(gtx.Ops, bounds, cut, max(1, gtx.Dp(unit.Dp(tokens.SizeHairline))), th.R.Vein)
	call.Add(gtx.Ops)
	return dimensions
}

// SlabRows lays out rows inside a standard slab at full width, separated by
// vein lines that keep the slab's inset.
func SlabRows(gtx layout.Context, th *theme.Theme, rows ...layout.Widget) layout.Dimensions {
	return SlabContent(gtx, th, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		children := make([]layout.FlexChild, 0, max(0, len(rows)*2-1))
		for index, row := range rows {
			if index > 0 {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: th.Sp.S3, Bottom: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						size := image.Pt(gtx.Constraints.Max.X, max(1, gtx.Dp(unit.Dp(tokens.SizeHairline))))
						FillCut(gtx.Ops, image.Rectangle{Max: size}, 0, th.R.Vein)
						return layout.Dimensions{Size: size}
					})
				}))
			}
			children = append(children, layout.Rigid(row))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}
