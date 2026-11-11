package teamsui

import (
	"image"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"

	"ryoku-rashin/internal/app/look"
)

// a team's tokens this month against its limit: amber from the share it warns
// at, a mark where the warning falls, the numbers under it
func MonthMeter(th *look.Theme, used, limit int64, warn int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		share := Share(used, limit)
		meter := kit.BladeMeter{Fraction: float64(share)}
		if warn > 0 && limit > 0 && share*100 >= float32(warn) {
			meter.Ink = th.R.Amber
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				dimensions := meter.Layout(gtx, th.Theme)
				if warn > 0 && limit > 0 {
					x := dimensions.Size.X * min(warn, 100) / 100
					rise := gtx.Dp(3)
					look.FillRect(gtx, image.Rect(x, -rise, x+max(1, gtx.Dp(1)), dimensions.Size.Y+rise), th.R.Fg3)
				}
				return dimensions
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(kit.Text(th.Theme, th.T.MonoSm, th.R.Fg2, TokensOf(used, limit))),
		)
	}
}
