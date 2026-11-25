package kit

import (
	"image"
	"math"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

// Flake draws the state parallelogram for tone and a becoming fill fraction.
func Flake(gtx layout.Context, th *theme.Theme, tone palette.Tone, fraction float64, size unit.Dp) layout.Dimensions {
	height := max(1, int(float32(gtx.Dp(size))*0.7+0.5))
	width := max(1, int(float32(gtx.Dp(size))*1.4+0.5))
	dimensions := layout.Dimensions{Size: gtx.Constraints.Constrain(image.Pt(width, height))}
	cut := min(dimensions.Size.Y, dimensions.Size.X/2)
	points := [...]f32.Point{
		f32.Pt(float32(cut), 0), f32.Pt(float32(dimensions.Size.X), 0),
		f32.Pt(float32(dimensions.Size.X-cut), float32(dimensions.Size.Y)), f32.Pt(0, float32(dimensions.Size.Y)),
	}
	path := pathFromPoints(gtx.Ops, points[:])
	if tone == palette.Idle {
		paint.FillShape(gtx.Ops, th.R.Vein, clip.Stroke{Path: path, Width: float32(max(1, gtx.Dp(unit.Dp(1.5))))}.Op())
		return dimensions
	}
	shape := clip.Outline{Path: path}.Op()
	stone := th.R.Stone(tone)
	if tone == palette.Becoming && fraction >= 0 && fraction < 1 {
		paint.FillShape(gtx.Ops, th.R.Vein, shape)
		filled := int(math.Round(float64(dimensions.Size.X) * min(max(fraction, 0), 1)))
		if filled > 0 {
			stack := clip.Rect(image.Rect(0, 0, filled, dimensions.Size.Y)).Push(gtx.Ops)
			paint.FillShape(gtx.Ops, stone, shape)
			stack.Pop()
		}
		return dimensions
	}
	paint.FillShape(gtx.Ops, stone, shape)
	return dimensions
}
