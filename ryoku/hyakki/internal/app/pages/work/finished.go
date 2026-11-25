package work

import (
	"gioui.org/layout"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
)

// the tasks every team finished each day this month, open on the board between its monument and its filters
func (b *board) layoutFinished(gtx layout.Context, e *env.Env) layout.Dimensions {
	trend := teamsui.FinishedTrend(gtx.Now, b.month.Spend(), i18n.T("Finished"))
	return look.HoldUntil(gtx, b.month.Loaded(), func(gtx layout.Context) layout.Dimensions { return trend.Layout(gtx, e.Theme) })
}
