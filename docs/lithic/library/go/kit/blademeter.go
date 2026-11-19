package kit

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// BladeMeter is a quiet determinate host meter. Lithic draws a square track
// and a blade with a 45 degree leading edge; a round design draws both with
// round ends.
type BladeMeter struct {
	Fraction float64
	Ink      color.NRGBA
}

// Layout draws the meter, using amber automatically from 90 percent.
func (meter BladeMeter) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	height := max(1, gtx.Dp(tokens.SizeBladeTrack))
	size := gtx.Constraints.Constrain(image.Pt(gtx.Constraints.Max.X, height))
	track := image.Rectangle{Max: size}
	fraction := min(max(meter.Fraction, 0), 1)
	ink := meter.Ink
	if ink.A == 0 {
		ink = th.R.Fg2
	}
	if fraction >= tokens.SizeBladeMeterAlertAt {
		ink = th.R.Amber
	}
	area, clipped := pushBladeTrack(gtx, th, track)
	if filled := int(float64(size.X) * fraction); filled > 0 {
		FillBlade(gtx, th, image.Rect(0, 0, filled, size.Y), ink)
	}
	if clipped {
		area.Pop()
	}
	return layout.Dimensions{Size: size}
}

// pushBladeTrack paints a meter's field track in th's design. A round track
// then clips what is drawn until the returned stack is popped, so a short bar
// keeps the track's round end; Lithic's square track needs no clip and
// reports false.
func pushBladeTrack(gtx layout.Context, th *theme.Theme, track image.Rectangle) (clip.Stack, bool) {
	cut := barCut(th, track)
	Fill(gtx, th, track, cut, th.R.Field)
	if cut == 0 {
		return clip.Stack{}, false
	}
	return clip.Outline{Path: Shape(gtx, th, track, cut, cut, cut, cut)}.Op().Push(gtx.Ops), true
}
