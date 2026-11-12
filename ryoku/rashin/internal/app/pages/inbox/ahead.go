package inbox

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// the runs listed under Coming up
const comingRuns = 5

// what runs next and what the month has cost so far
func (b *board) layoutAhead(gtx layout.Context, e *env.Env, snapshot state.Snapshot) layout.Dimensions {
	th := e.Theme
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: th.Sp.S2}.Layout(gtx, look.SectionTitle(th, i18n.T("Coming up"), 0))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return b.layoutComing(gtx, e, snapshot) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: th.Sp.S4, Bottom: th.Sp.S2}.Layout(gtx, look.SectionTitle(th, i18n.T("This month"), 0))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return b.layoutMonth(gtx, e, snapshot) }),
	)
}

func (b *board) layoutComing(gtx layout.Context, e *env.Env, snapshot state.Snapshot) layout.Dimensions {
	th := e.Theme
	runs := teamsui.Upcoming(snapshot, "")
	if len(runs) == 0 {
		return look.Paragraph(th, th.T.Caption, th.R.Fg3, i18n.T("No routine is on."))(gtx)
	}
	runs = runs[:min(len(runs), comingRuns)]
	children := make([]layout.FlexChild, 0, len(runs))
	for _, routine := range runs {
		link := b.link("routine:" + routine.ID)
		if link.Clicked(gtx) {
			teamsui.OpenRoutine(e, routine.ID)
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return teamsui.RunRow(gtx, th, link, *routine.NextRun, routine.Title, teamsui.TeamName(snapshot.Teams, routine.Team))
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// every team's tokens a day with its proof in words, then each team's tokens against its limit, open on the pane
func (b *board) layoutMonth(gtx layout.Context, e *env.Env, snapshot state.Snapshot) layout.Dimensions {
	th := e.Theme
	b.month.Want(e, gtx.Now)
	trend := teamsui.TokensTrend(gtx.Now, b.month.Spend(), "")
	loaded := b.month.Loaded()
	children := make([]layout.FlexChild, 0, 1+len(snapshot.Teams)*2)
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return look.HoldUntil(gtx, loaded, func(gtx layout.Context) layout.Dimensions { return trend.Layout(gtx, th) })
	}))
	for index, team := range snapshot.Teams {
		gap := th.Sp.S3
		if index == 0 {
			gap = th.Sp.S5
		}
		children = append(children, layout.Rigid(layout.Spacer{Height: gap}.Layout))
		link := b.link("team:" + team.ID)
		if link.Clicked(gtx) {
			e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabTeams, Team: team.ID, View: env.TeamSpend})
		}
		ink := th.R.Fg2
		if link.Hovered() {
			ink = th.R.Fg
		}
		meter := teamsui.MonthMeter(th, team.Stats.MonthTokens, team.Budget.MonthTokens, team.Budget.WarnPercent)
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return link.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return kit.Label{Style: th.T.Label, Color: ink, MaxLines: 1}.Layout(gtx, th.Theme, team.Name)
					}),
					layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
					layout.Rigid(meter),
				)
			})
		}))
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}
