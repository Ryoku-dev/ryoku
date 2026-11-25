package kit

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op/clip"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Skeleton is a static loading placeholder. Inactive skeletons occupy no space.
type Skeleton struct {
	Active bool
	Rows   int
}

// Layout draws static tablet placeholders without requesting frames.
func (skeleton Skeleton) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	if !skeleton.Active || skeleton.Rows <= 0 {
		return layout.Dimensions{}
	}
	rowHeight := max(1, gtx.Dp(th.RowHeight()))
	gap := gtx.Dp(tokens.SizeRowListGap)
	total := skeleton.Rows*rowHeight + max(0, skeleton.Rows-1)*gap
	size := gtx.Constraints.Constrain(image.Pt(gtx.Constraints.Max.X, total))
	viewport := clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops)
	barHeight := max(1, gtx.Dp(tokens.SizeSkeletonBarHeight))
	barGap := gtx.Dp(tokens.SizeSkeletonBarGap)
	inset := gtx.Dp(tokens.SizeSkeletonInset)
	available := max(0, size.X-2*inset)
	for index := range skeleton.Rows {
		y := index * (rowHeight + gap)
		FillCut(gtx.Ops, image.Rect(0, y, size.X, y+rowHeight), 0, th.R.Stone2)
		barsHeight := 2*barHeight + barGap
		barY := y + max(0, (rowHeight-barsHeight)/2)
		long := int(float32(available) * float32(tokens.SizeSkeletonBarLongFraction))
		short := int(float32(available) * float32(tokens.SizeSkeletonBarShortFraction))
		FillCut(gtx.Ops, image.Rect(inset, barY, inset+long, barY+barHeight), 0, th.R.Stone3)
		FillCut(gtx.Ops, image.Rect(inset, barY+barHeight+barGap, inset+short, barY+2*barHeight+barGap), 0, th.R.Stone3)
	}
	viewport.Pop()
	return layout.Dimensions{Size: size}
}
