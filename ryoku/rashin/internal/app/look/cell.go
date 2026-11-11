package look

import (
	"image"

	"gioui.org/layout"
)

// Cell is a fixed-width cell of a record row, width in pixels, so the
// columns of rows under each other line up.
func Cell(width int, widget layout.Widget) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		width := min(width, gtx.Constraints.Max.X)
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
		dimensions := widget(gtx)
		return layout.Dimensions{Size: image.Pt(width, dimensions.Size.Y), Baseline: dimensions.Baseline}
	})
}
