package look

import (
	"gioui.org/layout"
	"gioui.org/unit"
)

// SplitFrom is the width from which a board's two columns stand side by side.
const SplitFrom unit.Dp = 860

// Split stands left and right 60 to 40 from SplitFrom wide; narrower, right
// follows left.
func Split(gtx layout.Context, th *Theme, left, right layout.Widget) layout.Dimensions {
	full := gtx.Constraints.Max.X
	fixed := func(width int, widget layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
			return widget(gtx)
		}
	}
	if full < gtx.Dp(SplitFrom) {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(fixed(full, left)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S5}.Layout),
			layout.Rigid(fixed(full, right)),
		)
	}
	gap := gtx.Dp(th.Sp.S5)
	leftWidth := (full - gap) * 3 / 5
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
		layout.Rigid(fixed(leftWidth, left)),
		layout.Rigid(layout.Spacer{Width: th.Sp.S5}.Layout),
		layout.Rigid(fixed(full-gap-leftWidth, right)),
	)
}
