package routines

import (
	"gioui.org/layout"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
)

// the routine runs that ended each day this month, open on the board under its monument
func (b *board) layoutRuns(gtx layout.Context, e *env.Env) layout.Dimensions {
	trend := teamsui.RunsTrend(gtx.Now, b.month.Spend(), i18n.T("Runs"))
	return look.HoldUntil(gtx, b.month.Loaded(), func(gtx layout.Context) layout.Dimensions { return trend.Layout(gtx, e.Theme) })
}
