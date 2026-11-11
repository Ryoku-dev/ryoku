package kit

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// drawFieldSurface paints the shared field recipe: fill with an edge hairline
// at rest that turns fg2 with focus and jasper on error.
func drawFieldSurface(gtx layout.Context, th *theme.Theme, bounds image.Rectangle, cut int, fill color.NRGBA, focus float32, invalid bool) {
	FillCut(gtx.Ops, bounds, cut, disabledInk(gtx, fill))
	line := palette.Mix(th.R.Edge, th.R.Fg2, min(max(focus, 0), 1))
	if invalid {
		line = th.R.Jasper
	}
	StrokeCut(gtx.Ops, bounds, cut, max(1, gtx.Dp(unit.Dp(tokens.SizeHairline))), disabledInk(gtx, line))
}
