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

// Facts lays out a flat table of labelled values.
func Facts(th *theme.Theme, facts []Fact) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if len(facts) == 0 {
			return layout.Dimensions{}
		}
		rowHeight := max(1, gtx.Dp(tokens.SizeFactsRowHeight))
		size := gtx.Constraints.Constrain(image.Pt(gtx.Constraints.Max.X, len(facts)*rowHeight))
		viewport := clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops)
		for index, fact := range facts {
			y := index * rowHeight
			offset := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
			rowContext := gtx
			rowContext.Constraints = layout.Exact(image.Pt(size.X, rowHeight))
			value := fact.Value
			if value == "" {
				value = "-"
			}
			valueStyle := th.T.Body
			if fact.Mono {
				valueStyle = th.T.Mono
			}
			layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(rowContext,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					width := min(gtx.Constraints.Max.X, gtx.Dp(tokens.SizeFactsLabelWidth))
					gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
					return Text(th, th.T.Body, th.R.Fg3, fact.Label)(gtx)
				}),
				layout.Flexed(1, Text(th, valueStyle, th.R.Fg, value)),
			)
			offset.Pop()
		}
		viewport.Pop()
		return layout.Dimensions{Size: size}
	}
}
