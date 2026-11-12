package kit

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Fact is one labelled value in a Facts table.
type Fact struct {
	Label string
	Value string
	Mono  bool
}

// Facts lays out a flat table of labelled values. Labels keep the 140 column
// and values start s4 after it; either side wraps, and a row grows past 44 to
// fit its taller side, with both sides centred in it.
func Facts(th *theme.Theme, facts []Fact) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if len(facts) == 0 {
			return layout.Dimensions{}
		}
		width := gtx.Constraints.Max.X
		gap := gtx.Dp(th.Sp.S4)
		labelWidth := min(gtx.Dp(tokens.SizeFactsLabelWidth), max(0, width-gap))
		valueWidth := max(0, width-labelWidth-gap)
		minRow := gtx.Dp(tokens.SizeFactsRowHeight)

		macro := op.Record(gtx.Ops)
		y := 0
		for _, fact := range facts {
			value := fact.Value
			if value == "" {
				value = "-"
			}
			valueStyle := th.T.Body
			if fact.Mono {
				valueStyle = th.T.Mono
			}
			label, labelSize := factsCell(gtx, labelWidth, func(gtx layout.Context) layout.Dimensions {
				return Label{Style: th.T.Body, Color: th.R.Fg3}.Layout(gtx, th, fact.Label)
			})
			cell, valueSize := factsCell(gtx, valueWidth, func(gtx layout.Context) layout.Dimensions {
				return Label{Style: valueStyle, Color: th.R.Fg}.Layout(gtx, th, value)
			})
			row := max(minRow, labelSize.Y, valueSize.Y)
			offset := op.Offset(image.Pt(0, y+(row-labelSize.Y)/2)).Push(gtx.Ops)
			label.Add(gtx.Ops)
			offset.Pop()
			offset = op.Offset(image.Pt(labelWidth+gap, y+(row-valueSize.Y)/2)).Push(gtx.Ops)
			cell.Add(gtx.Ops)
			offset.Pop()
			y += row
		}
		call := macro.Stop()

		size := gtx.Constraints.Constrain(image.Pt(width, y))
		viewport := clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops)
		call.Add(gtx.Ops)
		viewport.Pop()
		return layout.Dimensions{Size: size}
	}
}

// factsCell records one side of a row at a fixed width with unbounded height,
// so wrapped lines are measured rather than clipped.
func factsCell(gtx layout.Context, width int, content layout.Widget) (op.CallOp, image.Point) {
	gtx.Constraints = layout.Constraints{Max: image.Pt(width, 1<<24)}
	macro := op.Record(gtx.Ops)
	dims := content(gtx)
	return macro.Stop(), dims.Size
}
