package teams

import (
	"math"
	"strconv"
	"strings"
	"time"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	appstate "ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// the month's spend by day, member, routine and engine; the limit itself is set in the Settings tab
type spendTab struct {
	scroll    look.Scroller
	fetch     load
	month     protocol.TeamSpend
	monthTeam string
	change    kit.Plaque
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

func (s *spendTab) layout(gtx layout.Context, e *env.Env, p *Page, team protocol.Team) layout.Dimensions {
	th := e.Theme
	s.request(e, team)
	spend := s.month
	if s.monthTeam != team.ID {
		spend = protocol.TeamSpend{Tokens: team.Stats.MonthTokens, Cap: team.Budget.MonthTokens}
	}
	s.change.Kind, s.change.Small, s.change.Icon, s.change.Label = kit.PlaqueTonal, true, look.Icon.Settings, i18n.T("Change the limit")
	if s.change.Clicked(gtx) {
		openTeam(e, team.ID, env.TeamSettings)
		p.settings.jump(sectionBudget)
	}
	left := func(gtx layout.Context) layout.Dimensions {
		return sections(gtx,
			section(th, i18n.T("This month"), func(gtx layout.Context) layout.Dimensions {
				children := []layout.FlexChild{
					layout.Rigid(teamsui.MonthMeter(th, spend.Tokens, spend.Cap, team.Budget.WarnPercent)),
					layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return s.change.Layout(gtx, th.Theme) }),
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
			section(th, i18n.T("By engine"), spendList(th, engineSpends(e.Snapshot(), spend.Engines), i18n.T("No task has run on an engine this month."))),
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

// the daemon leaves engines unworded, so each reads as the task header words the engine a task ran on
func engineSpends(snapshot appstate.Snapshot, engines []protocol.NamedSpend) []protocol.NamedSpend {
	named := make([]protocol.NamedSpend, len(engines))
	for index, engine := range engines {
		harness, model, _ := strings.Cut(engine.ID, protocol.EngineSpendSeparator)
		named[index] = engine
		if harness != "" {
			named[index].Name = teamsui.TaskEngine(snapshot, protocol.TeamTask{Harness: harness, Model: model}).Line()
		}
	}
	return named
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
