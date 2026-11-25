package look

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
)

type Placards struct {
	kit.Placard
}

func (placards *Placards) Layout(gtx layout.Context, th *Theme) layout.Dimensions {
	record := op.Record(gtx.Ops)
	dimensions := placards.Placard.Layout(gtx, th.Theme)
	call := record.Stop()
	x := max(0, (gtx.Constraints.Max.X-dimensions.Size.X)/2)
	y := max(0, gtx.Constraints.Max.Y-dimensions.Size.Y-gtx.Dp(13))
	offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func SkeletonRows(gtx layout.Context, th *Theme, rows int, twoLine bool) layout.Dimensions {
	if rows <= 0 {
		return layout.Dimensions{}
	}
	height := unit.Dp(36)
	if twoLine {
		height = 48
	}
	children := make([]layout.FlexChild, 0, rows*2)
	for index := range rows {
		if index > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: 2}.Layout))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = gtx.Dp(height), gtx.Dp(height)
			var skeleton kit.Skeleton
			skeleton.Active, skeleton.Rows = true, 1
			return skeleton.Layout(gtx, th.Theme)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}
