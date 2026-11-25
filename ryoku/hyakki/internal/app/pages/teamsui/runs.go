package teamsui

import (
	"time"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"

	"ryoku-rashin/internal/app/look"
)

// one coming run of a routine as a row that opens it: when, the
// routine, then its team when the row stands among every team's
func RunRow(gtx layout.Context, th *look.Theme, click *kit.Click, at time.Time, routine, team string) layout.Dimensions {
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		ink := th.R.Fg2
		if click.Hovered() {
			ink = th.R.Fg
		}
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		children := []layout.FlexChild{
			look.Cell(gtx.Dp(144), kit.Text(th.Theme, th.T.Caption, th.R.Fg3, look.When(gtx.Now, at))),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return kit.Label{Style: th.T.Body, Color: ink, MaxLines: 1}.Layout(gtx, th.Theme, routine)
			}),
		}
		if team != "" {
			children = append(children,
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
				layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, team)),
			)
		}
		return layout.Inset{Top: th.Sp.S1, Bottom: th.Sp.S1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx, children...)
		})
	})
}
