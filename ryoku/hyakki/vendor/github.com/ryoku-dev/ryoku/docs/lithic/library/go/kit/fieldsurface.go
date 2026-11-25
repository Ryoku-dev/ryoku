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

// FieldSurface paints the surface every kit field shares, for a host's own
// field (a multi-line editor, a composer, a select): fill, the field role or
// that role under a hover wash, with an edge hairline at rest that turns fg2
// as focus goes from 0 to 1 and jasper while invalid. The corners follow the
// design from cut, and a disabled context fades both. Draw it behind the
// field's content; the field keeps its own focus ring.
func FieldSurface(gtx layout.Context, th *theme.Theme, bounds image.Rectangle, cut int, fill color.NRGBA, focus float32, invalid bool) {
	Fill(gtx, th, bounds, cut, disabledInk(gtx, th, fill))
	line := palette.Mix(th.R.Edge, th.R.Fg2, min(max(focus, 0), 1))
	if invalid {
		line = th.R.Jasper
	}
	Stroke(gtx, th, bounds, cut, max(1, gtx.Dp(unit.Dp(tokens.SizeHairline))), disabledInk(gtx, th, line))
}
