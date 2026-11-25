package kit

import (
	"image"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// BladeProgress is determinate at a nonnegative fraction and indeterminate
// below zero. Lithic draws a square track, a blade with a 45 degree leading
// edge and a gold tick at the head of active work; a round design draws the
// track and the bar with round ends and the head as a gold dot.
type BladeProgress struct {
	Fraction float64
	Active   bool
}

// Layout draws job progress and requests sweep frames only while active.
func (progress BladeProgress) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	height := max(1, gtx.Dp(tokens.SizeBladeTrack))
	size := gtx.Constraints.Constrain(image.Pt(gtx.Constraints.Max.X, height))
	track := image.Rectangle{Max: size}
	round := barCut(th, track) > 0
	area, clipped := pushBladeTrack(gtx, th, track)
	start, end := 0, 0
	if progress.Fraction < 0 {
		if progress.Active && size.X > 0 {
			phase := float32(0)
			if !anim.Reduced() && th.Motion.Sweep > 0 {
				// align every sweep to one low-rate clock instead of waking each display frame
				const step = 80 * time.Millisecond
				tick := gtx.Now.Truncate(step)
				gtx.Execute(op.InvalidateCmd{At: tick.Add(step)})
				phase = float32(tick.UnixNano()%int64(th.Motion.Sweep)) / float32(th.Motion.Sweep)
			}
			phase += float32(tokens.SizeBladeIndeterminateFraction)
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
		if !clipped {
			// the sweep enters and leaves through the track's ends
			area, clipped = clip.Rect(track).Push(gtx.Ops), true
		}
		FillBlade(gtx, th, image.Rect(start, 0, end, size.Y), th.R.Lapis)
	}
	if clipped {
		area.Pop()
	}
	if progress.Active && end > 0 && size.X > 0 {
		if round {
			// the head is the bar's own round end, so it never leaves the track
			x := max(0, min(size.X, end)-size.Y)
			head := image.Rect(x, 0, x+size.Y, size.Y)
			Fill(gtx, th, head, barCut(th, head), th.R.Gold)
		} else {
			markerWidth := min(size.X, max(1, gtx.Dp(tokens.SizeBladeMarker)))
			x := max(0, min(size.X-markerWidth, end-markerWidth/2))
			FillCut(gtx.Ops, image.Rect(x, 0, x+markerWidth, size.Y), 0, th.R.Gold)
		}
	}
	return layout.Dimensions{Size: size}
}
