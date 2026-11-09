package kit

import (
	"image"
	"image/color"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// BladeMeter is a quiet determinate host meter.
type BladeMeter struct {
	Fraction float64
	Ink      color.NRGBA
}

// Layout draws the meter, using amber automatically from 90 percent.
func (meter BladeMeter) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	height := max(1, gtx.Dp(tokens.SizeBladeTrack))
	size := gtx.Constraints.Constrain(image.Pt(gtx.Constraints.Max.X, height))
	FillCut(gtx.Ops, image.Rectangle{Max: size}, 0, th.R.Stone3)
	fraction := min(max(meter.Fraction, 0), 1)
	ink := meter.Ink
	if ink.A == 0 {
		ink = th.R.Fg2
	}
	if fraction >= tokens.SizeBladeMeterAlertAt {
		ink = th.R.Amber
	}
	filled := int(float64(size.X) * fraction)
	if filled > 0 {
		Blade(gtx.Ops, image.Rect(0, 0, filled, size.Y), ink)
	}
	return layout.Dimensions{Size: size}
}
