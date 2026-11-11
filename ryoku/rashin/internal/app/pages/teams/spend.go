package teams

import (
	"math"
	"strconv"
	"time"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/protocol"
)

// the month's spend by day, member and routine, and the limit that caps it
type spendTab struct {
	scroll     look.Scroller
	fetch      load
	month      protocol.TeamSpend
	monthTeam  string
	limit      teamsui.TokenStepper
	warn       kit.Stepper
	turns      kit.Stepper
	save       kit.Plaque
	filledFrom string
}

func (s *spendTab) request(e *env.Env, team protocol.Team) {
	s.fetch.want(e, teamKey(e.Snapshot().Epoch, team), func(done func()) {
		var result protocol.TeamSpend
		e.Call(protocol.MTeamsSpend, protocol.TeamSpendParams{Team: team.ID}, &result, func(err error) {
			done()
			if err != nil {
				e.Notify(err.Error())
				return
			}
			s.month, s.monthTeam = result, team.ID
		})
	})
}

// the editor starts from the team's budget and keeps what was changed until it is saved or the team changes
func (s *spendTab) fill(team protocol.Team) {
	key := team.ID + "\x00" + team.Updated.String()
	if s.filledFrom == key {
		return
	}
	s.filledFrom = key
	s.limit.Value = team.Budget.MonthTokens
	s.warn.Value = team.Budget.WarnPercent
	s.turns.Value = team.Budget.TaskTurns
}

func (s *spendTab) layout(gtx layout.Context, e *env.Env, team protocol.Team) layout.Dimensions {
	th := e.Theme
	s.request(e, team)
	s.fill(team)
	spend := s.month
	if s.monthTeam != team.ID {
		spend = protocol.TeamSpend{Tokens: team.Stats.MonthTokens, Cap: team.Budget.MonthTokens}
	}
	left := func(gtx layout.Context) layout.Dimensions {
		return sections(gtx,
			section(th, i18n.T("This month"), func(gtx layout.Context) layout.Dimensions {
				children := []layout.FlexChild{
					layout.Rigid(teamsui.MonthMeter(th, spend.Tokens, spend.Cap, team.Budget.WarnPercent)),
					layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return dailyChart(gtx, th, spend) }),
				}
				if spend.Estimated {
					children = append(children,
						layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
						layout.Rigid(quiet(th, i18n.T("A harness did not report all of its tokens, so part of this is an estimate."))),
					)
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			}),
			section(th, i18n.T("Spending limit"), func(gtx layout.Context) layout.Dimensions { return s.layoutLimit(gtx, e, team) }),
		)
	}
	right := func(gtx layout.Context) layout.Dimensions {
		return sections(gtx,
			section(th, i18n.T("By member"), spendList(th, spend.Members, i18n.T("No member has spent anything this month."))),
			section(th, i18n.T("By routine"), spendList(th, spend.Routines, i18n.T("No routine has run this month."))),
		)
	}
	return s.scroll.Layout(gtx, th, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return pageInset(gtx, e, func(gtx layout.Context) layout.Dimensions {
			return look.Split(gtx, th, left, right)
		})
	})
}

// one point a day from the first of the month to today, or to its end for a month gone by
func dailyChart(gtx layout.Context, th *look.Theme, spend protocol.TeamSpend) layout.Dimensions {
	now := gtx.Now.Local()
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	if parsed, err := time.ParseInLocation("2006-01", spend.Month, time.Local); err == nil {
		month = parsed
	}
	days := month.AddDate(0, 1, -1).Day()
	if month.Year() == now.Year() && month.Month() == now.Month() {
		days = now.Day()
	}
	byDay := make(map[string]int64, len(spend.Days))
	for _, day := range spend.Days {
		byDay[day.Day] = day.Input + day.Output
	}
	values := make([]float64, days)
	ticks := make([]string, days)
	for index := range days {
		values[index] = float64(byDay[month.AddDate(0, 0, index).Format("2006-01-02")])
		if index == 0 || index == days-1 || (index+1)%7 == 0 {
			ticks[index] = strconv.Itoa(index + 1)
		}
	}
	chart := kit.EngravedChart{Values: values, Min: 0, Max: math.NaN(), Height: 120, Ticks: ticks, Empty: i18n.T("A chart of daily tokens shows from the second day of the month.")}
	return chart.Layout(gtx, th.Theme)
}

func spendList(th *look.Theme, spends []protocol.NamedSpend, none string) layout.Widget {
	if len(spends) == 0 {
		return quiet(th, none)
	}
	rows := make([]look.Setting, len(spends))
	for index, spend := range spends {
		rows[index] = look.Setting{
			Title: spend.Name, Detail: tasksWord(spend.Tasks),
			Control: kit.Text(th.Theme, th.T.MonoSm, th.R.Fg2, i18n.Tf("%s tokens", teamsui.Tokens(spend.Tokens))),
		}
	}
	return func(gtx layout.Context) layout.Dimensions { return look.SettingsSlab(gtx, th, rows) }
}

func (s *spendTab) layoutLimit(gtx layout.Context, e *env.Env, team protocol.Team) layout.Dimensions {
	th := e.Theme
	s.warn.Min, s.warn.Max, s.warn.Step, s.warn.Unit = 50, 95, 5, i18n.T("percent")
	s.turns.Min, s.turns.Max, s.turns.Step, s.turns.Unit = 1, 200, 1, i18n.T("turns")
	s.save.Kind, s.save.Small, s.save.Label = kit.PlaqueLine, true, i18n.T("Save the limit")
	budget := protocol.TeamBudget{MonthTokens: s.limit.Value, WarnPercent: s.warn.Value, TaskTurns: s.turns.Value}
	changed := budget != team.Budget
	if s.save.Clicked(gtx) && changed {
		s.save.Busy = true
		updateTeam(e, team.ID, map[string]any{"budget": budget}, func(error) { s.save.Busy = false })
	}
	rows := []look.Setting{
		{Title: i18n.T("Tokens a month"), Detail: i18n.T("At the limit the team pauses and asks you to raise it."),
			Control: func(gtx layout.Context) layout.Dimensions { return s.limit.Layout(gtx, th) }},
		{Title: i18n.T("Warn at"), Detail: i18n.T("You are told once a month when the team gets this close."),
			Control: func(gtx layout.Context) layout.Dimensions { return s.warn.Layout(gtx, th.Theme) }},
		{Title: i18n.T("Turns per task"), Detail: i18n.T("A task that uses them all stops and says so."),
			Control: func(gtx layout.Context) layout.Dimensions { return s.turns.Layout(gtx, th.Theme) }},
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.SettingsSlab(gtx, th, rows) }),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !changed {
				gtx = gtx.Disabled()
			}
			return s.save.Layout(gtx, th.Theme)
		}),
	)
}
