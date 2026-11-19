package teamsui

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"

	"ryoku-rashin/internal/app/look"
)

// a team's tokens this month against its limit: the library's meter, a blade in Lithic and a
// round bar elsewhere, in the attention mineral from the share it warns at, a tick where the
// warning falls, the numbers under it
func MonthMeter(th *look.Theme, used, limit int64, warn int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		share := Share(used, limit)
		warned := warn > 0 && limit > 0
		ink := th.R.Fg2
		if warned && share*100 >= float32(warn) {
			ink = th.R.Stone(palette.Attention)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				// the tick stands a little over and under the track; a meter without one keeps the same room, so meters side by side line up
				rise := gtx.Dp(th.Sp.S1) / 2
				stack := op.Offset(image.Pt(0, rise)).Push(gtx.Ops)
				dimensions := budgetMeter(gtx, th, share, ink)
				if warned {
					x := dimensions.Size.X * min(warn, 100) / 100
					line := max(1, gtx.Dp(unit.Dp(tokens.SizeHairline)))
					look.FillRect(gtx, image.Rect(x, -rise, x+line, dimensions.Size.Y+rise), th.R.Fg3)
				}
				stack.Pop()
				dimensions.Size.Y += 2 * rise
				return dimensions
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg2, TokensOf(used, limit))),
		)
	}
}

// the team's own warning share can differ from the library's general 90% alert
func budgetMeter(gtx layout.Context, th *look.Theme, share float32, ink color.NRGBA) layout.Dimensions {
	height := max(1, gtx.Dp(unit.Dp(tokens.SizeBladeTrack)))
	size := gtx.Constraints.Constrain(image.Pt(gtx.Constraints.Max.X, height))
	bounds := image.Rectangle{Max: size}
	cut := 0
	if th.Design.Profile().Round {
		cut = gtx.Dp(th.Cut.Token)
	}
	kit.Fill(gtx, th.Theme, bounds, cut, th.R.Field)
	if size.X > 0 && share > 0 {
		if cut > 0 {
			area := clip.Outline{Path: kit.Shape(gtx, th.Theme, bounds, cut, cut, cut, cut)}.Op().Push(gtx.Ops)
			kit.FillBlade(gtx, th.Theme, image.Rect(0, 0, int(float32(size.X)*min(share, 1)), size.Y), ink)
			area.Pop()
		} else {
			kit.FillBlade(gtx, th.Theme, image.Rect(0, 0, int(float32(size.X)*min(share, 1)), size.Y), ink)
		}
	}
	return layout.Dimensions{Size: size}
}
