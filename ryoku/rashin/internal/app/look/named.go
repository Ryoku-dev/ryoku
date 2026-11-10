package look

import (
	"image"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
)

// the name a screen reader says for a control whose label is drawn somewhere else
func Named(gtx layout.Context, name string, control layout.Widget) layout.Dimensions {
	record := op.Record(gtx.Ops)
	dimensions := control(gtx)
	call := record.Stop()
	// wider than the control so its focus ring is not cut
	margin := gtx.Dp(8)
	bounds := image.Rectangle{Max: dimensions.Size}.Inset(-margin)
	area := clip.Rect(bounds).Push(gtx.Ops)
	semantic.LabelOp(name).Add(gtx.Ops)
	call.Add(gtx.Ops)
	area.Pop()
	return dimensions
}
