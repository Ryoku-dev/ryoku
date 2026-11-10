package look

import (
	"image"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
)

// one tone above the pane is no edge on a wallpaper palette, so the field takes a vein hairline until its own focus stroke shows
func PaneField(gtx layout.Context, th *Theme, focused bool, field layout.Widget) layout.Dimensions {
	dimensions := field(gtx)
	if !focused {
		kit.StrokeCut(gtx.Ops, image.Rectangle{Max: dimensions.Size}, gtx.Dp(th.Cut.Mid), max(1, gtx.Dp(1)), th.R.Vein)
	}
	return dimensions
}
