package dreams

import (
	"gioui.org/layout"
	"gioui.org/unit"

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

// one part of a dream's page: what its ruled head says, over its body
type part struct {
	title string
	body  layout.Widget
}

// the parts one under the other, each numbered by its place among the page's of parts,
// counted on from first; order is the group each rises as when the page opens
func layoutParts(gtx layout.Context, th *look.Theme, reveal *look.Reveal, parts []part, first, of, order int) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(parts))
	for index, each := range parts {
		head := look.Head{Title: each.title, Number: first + index, Of: of}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return reveal.Group(gtx, th, order+index, func(gtx layout.Context) layout.Dimensions {
				return look.Section(gtx, th, head, each.body)
			})
		}))
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}
