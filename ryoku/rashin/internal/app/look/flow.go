package look

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

type Flow struct {
	Gap, LineGap unit.Dp
	Centre       bool
}

func (flow Flow) Layout(gtx layout.Context, count int, child func(layout.Context, int) layout.Dimensions) layout.Dimensions {
	gap, lineGap := gtx.Dp(flow.Gap), gtx.Dp(flow.LineGap)
	limit := gtx.Constraints.Max.X
	type placed struct {
		call op.CallOp
		size image.Point
	}
	var line []placed
	lineWidth, lineHeight, y, widest := 0, 0, 0, 0
	flush := func() {
		x := 0
		if flow.Centre {
			x = (limit - lineWidth) / 2
		}
		for _, item := range line {
			offset := op.Offset(image.Pt(x, y+(lineHeight-item.size.Y)/2)).Push(gtx.Ops)
			item.call.Add(gtx.Ops)
			offset.Pop()
			x += item.size.X + gap
		}
		widest = max(widest, lineWidth)
		y += lineHeight
		line, lineWidth, lineHeight = line[:0], 0, 0
	}
	for index := range count {
		record := op.Record(gtx.Ops)
		measure := gtx
		measure.Constraints.Min = image.Point{}
		dimensions := child(measure, index)
		call := record.Stop()
		if dimensions.Size.X == 0 && dimensions.Size.Y == 0 {
			continue
		}
		needed := dimensions.Size.X
		if len(line) > 0 {
			needed += gap
		}
		if len(line) > 0 && lineWidth+needed > limit {
			flush()
			y += lineGap
			needed = dimensions.Size.X
		}
		line = append(line, placed{call: call, size: dimensions.Size})
		lineWidth += needed
		lineHeight = max(lineHeight, dimensions.Size.Y)
	}
	if len(line) > 0 {
		flush()
	}
	width := widest
	if flow.Centre {
		width = limit
	}
	return layout.Dimensions{Size: image.Pt(width, y)}
}
