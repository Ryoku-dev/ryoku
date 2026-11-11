package kit

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// PlaceBlade marks a selected sel surface with the onSel place blade, clipped
// to the surface's cut. Vertical runs it down the leading edge at full height,
// as rows carry it; Horizontal lays it along the bottom, as chips and segments
// carry it.
func PlaceBlade(gtx layout.Context, th *theme.Theme, bounds image.Rectangle, cut int, axis layout.Axis) {
	if bounds.Empty() {
		return
	}
	thickness := max(1, gtx.Dp(unit.Dp(tokens.SizeSelBlade)))
	bar := image.Rect(bounds.Min.X, bounds.Max.Y-thickness, bounds.Max.X, bounds.Max.Y)
	if axis == layout.Vertical {
		bar = image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Min.X+thickness, bounds.Max.Y)
	}
	stack := clip.Outline{Path: CutPath(gtx.Ops, bounds, cut, cut, cut, cut)}.Op().Push(gtx.Ops)
	FillCut(gtx.Ops, bar, 0, disabledInk(gtx, th.R.OnSel))
	stack.Pop()
}
