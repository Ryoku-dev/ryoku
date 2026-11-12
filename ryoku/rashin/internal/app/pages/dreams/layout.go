package dreams

import (
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"

	"ryoku-rashin/internal/app/look"
)

// the widgets of a column one under the other, a nil one left out
func stack(gtx layout.Context, widgets ...layout.Widget) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(widgets))
	for _, widget := range widgets {
		if widget != nil {
			children = append(children, layout.Rigid(widget))
		}
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func gap(height unit.Dp) layout.Widget {
	return layout.Spacer{Height: height}.Layout
}

func section(th *look.Theme, title string, body layout.Widget) layout.Widget {
	return kit.Section(th.Theme, title, body)
}
