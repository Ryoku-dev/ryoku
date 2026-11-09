package kit

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op/clip"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// BladeProgress is determinate at a nonnegative fraction and indeterminate below zero.
type BladeProgress struct {
	Fraction float64
	Active   bool
}

// Layout draws job progress and requests sweep frames only while active.
func (progress BladeProgress) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	height := max(1, gtx.Dp(tokens.SizeBladeTrack))
	size := gtx.Constraints.Constrain(image.Pt(gtx.Constraints.Max.X, height))
	track := image.Rectangle{Max: size}
	FillCut(gtx.Ops, track, 0, th.R.Stone3)
	start, end := 0, 0
	if progress.Fraction < 0 {
		if progress.Active && size.X > 0 {
			phase := anim.Phase(gtx, th.Motion.Sweep) + float32(tokens.SizeBladeIndeterminateFraction)
			if phase > 1 {
				phase--
			}
			segment := max(1, int(float32(size.X)*float32(tokens.SizeBladeIndeterminateFraction)))
			start = int(float32(size.X+segment)*phase) - segment
			end = start + segment
		}
	} else {
		end = int(float64(size.X) * min(max(progress.Fraction, 0), 1))
	}
	if end > start {
		stack := clip.Rect(track).Push(gtx.Ops)
		Blade(gtx.Ops, image.Rect(start, 0, end, size.Y), th.R.Lapis)
		stack.Pop()
	}
	if progress.Active && end > 0 && size.X > 0 {
		markerWidth := min(size.X, max(1, gtx.Dp(tokens.SizeBladeMarker)))
		x := max(0, min(size.X-markerWidth, end-markerWidth/2))
		FillCut(gtx.Ops, image.Rect(x, 0, x+markerWidth, size.Y), 0, th.R.Gold)
	}
	return layout.Dimensions{Size: size}
}
