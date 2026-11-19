package look

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

const (
	sealTile  = "M24 128 C24 48 48 24 128 24 C208 24 232 48 232 128 C232 208 208 232 128 232 C48 232 24 208 24 128 Z"
	sealCut   = "M44 120 L44 136 L106.56 136 L128 216 L149.44 136 L212 136 L212 120 Z"
	sealNorth = "M128 40 L151.58 128 L104.42 128 Z"
	sealPivot = "M128 119 C133 119 137 123 137 128 C137 133 133 137 128 137 C123 137 119 133 119 128 C119 123 123 119 128 119 Z"
	markNorth = "M128 40 L149 119 H107 Z"
	markSouth = "M107 137 H149 L128 216 Z"
	markWest  = "M44 119 H119 V137 H44 Z"
	markEast  = "M137 119 H212 V137 H137 Z"
)

var sealBox = f32.Pt(256, 256)

func RashinSeal(th *Theme, size unit.Dp) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		pixels := max(0, gtx.Dp(size))
		box := image.Pt(pixels, pixels)
		drawSealPath(gtx, sealTile+" "+sealCut, box, th.R.Fg)
		drawSealPath(gtx, sealNorth, box, th.R.Sel)
		drawSealPath(gtx, sealPivot, box, th.R.Fg)
		return layout.Dimensions{Size: box}
	}
}

func RashinMark(size unit.Dp, ink color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		pixels := max(0, gtx.Dp(size))
		box := image.Pt(pixels, pixels)
		drawSealPath(gtx, markNorth, box, ink)
		drawSealPath(gtx, markSouth, box, ink)
		drawSealPath(gtx, markWest, box, ink)
		drawSealPath(gtx, markEast, box, ink)
		return layout.Dimensions{Size: box}
	}
}

func drawSealPath(gtx layout.Context, svg string, box image.Point, ink color.NRGBA) {
	if box.X <= 0 {
		return
	}
	if path, err := buildSVGPath(gtx.Ops, svg, sealBox, box); err == nil {
		paint.FillShape(gtx.Ops, ink, clip.Outline{Path: path}.Op())
	}
}
