package kit

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// DraftColumns lays out a form and its live aside at the canonical breakpoint.
func DraftColumns(gtx layout.Context, th *theme.Theme, form, aside layout.Widget) layout.Dimensions {
	available := gtx.Constraints.Max.X
	column := func(width int, widget layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
			if widget == nil {
				return layout.Dimensions{Size: gtx.Constraints.Min}
			}
			return widget(gtx)
		}
	}
	formMax := gtx.Dp(tokens.SizeDraftFormMax)
	if aside == nil {
		return column(min(available, formMax), form)(gtx)
	}
	if available < gtx.Dp(tokens.SizeDraftWideAt) {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(column(min(available, formMax), form)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S5}.Layout),
			layout.Rigid(column(min(available, gtx.Dp(tokens.SizeDraftAsideBelow)), aside)),
		)
	}
	gap := gtx.Dp(th.Sp.S5)
	asideWidth := int(float32(available) * float32(tokens.SizeDraftAsideFraction))
	asideWidth = max(gtx.Dp(tokens.SizeDraftAsideMin), min(asideWidth, gtx.Dp(tokens.SizeDraftAsideMax)))
	formWidth := min(formMax, available-gap-asideWidth)
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
		layout.Rigid(column(formWidth, form)),
		layout.Rigid(layout.Spacer{Width: th.Sp.S5}.Layout),
		layout.Rigid(column(asideWidth, aside)),
	)
}
