package kit

import (
	"image"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Working is the three-flake active work mark.
type Working struct {
	Active bool
	Size   unit.Dp
}

// Layout requests frames only while Active.
func (working Working) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	size := working.Size
	if size <= 0 {
		size = tokens.SizeIconPlaqueIcon
	}
	pixels := max(1, gtx.Dp(size))
	flakeWidth := max(2, int(float32(pixels)*float32(tokens.SizeWorkingFlakeWidthRatio)))
	flakeHeight := max(2, int(float32(pixels)*float32(tokens.SizeWorkingFlakeHeightRatio)))
	gap := max(1, int(float32(pixels)*float32(tokens.SizeWorkingFlakeGapRatio)))
	totalWidth := flakeWidth*tokens.SizeWorkingFlakeCount + gap*(tokens.SizeWorkingFlakeCount-1)
	x := (pixels - totalWidth) / 2
	y := (pixels - flakeHeight) / 2
	phase := float32(0)
	activeIndex := -1
	activeFraction := float32(0)
	reduced := working.Active && anim.Reduced()
	if working.Active && !reduced {
		phase = anim.Phase(gtx, th.Motion.Working) + 1/(2*float32(tokens.SizeWorkingFlakeCount))
		if phase >= 1 {
			phase--
		}
		scaled := phase * float32(tokens.SizeWorkingFlakeCount)
		activeIndex = int(scaled)
		activeFraction = scaled - float32(activeIndex)
	}
	for index := range tokens.SizeWorkingFlakeCount {
		rectangle := image.Rect(x+index*(flakeWidth+gap), y, x+index*(flakeWidth+gap)+flakeWidth, y+flakeHeight)
		cut := min(rectangle.Dx()/2, rectangle.Dy()/2)
		points := [...]f32.Point{
			f32.Pt(float32(rectangle.Min.X+cut), float32(rectangle.Min.Y)),
			f32.Pt(float32(rectangle.Max.X), float32(rectangle.Min.Y)),
			f32.Pt(float32(rectangle.Max.X-cut), float32(rectangle.Max.Y)),
			f32.Pt(float32(rectangle.Min.X), float32(rectangle.Max.Y)),
		}
		path := pathFromPoints(gtx.Ops, points[:])
		shape := clip.Outline{Path: path}.Op()
		paint.FillShape(gtx.Ops, palette.WithAlpha(th.R.Vein, tokens.AlphaWorkingBase), shape)
		fraction := float32(0)
		if reduced {
			fraction = 1
		} else if index == activeIndex {
			fraction = activeFraction
		}
		if fraction > 0 {
			fill := clip.Rect(image.Rect(rectangle.Min.X, rectangle.Min.Y, rectangle.Min.X+int(float32(rectangle.Dx())*fraction+.5), rectangle.Max.Y)).Push(gtx.Ops)
			paint.FillShape(gtx.Ops, th.R.Lapis, shape)
			fill.Pop()
		}
	}
	return layout.Dimensions{Size: gtx.Constraints.Constrain(image.Pt(pixels, pixels))}
}
