package teams

import (
	"strings"

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
	scroll look.Scroller
	month  teamsui.MonthSpend
	change kit.Plaque
}

func (s *spendTab) layout(gtx layout.Context, e *env.Env, p *Page, team protocol.Team) layout.Dimensions {
	th := e.Theme
	s.month.Team = team.ID
	s.month.Want(e, gtx.Now)
	loaded := s.month.Loaded()
	spend := s.month.Spend()
	if !loaded {
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
				}
				// the line waits for the month the daemon counted, never drawing an empty one first
				if loaded {
					children = append(children,
						layout.Rigid(layout.Spacer{Height: th.Sp.S5}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return teamsui.TokensTrend(gtx.Now, spend, "").Layout(gtx, th)
						}),
					)
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

// a section lies open on its pane: each row its name and task count on the left, its tokens on the right, rows set apart by space
func spendList(th *look.Theme, spends []protocol.NamedSpend, none string) layout.Widget {
	if len(spends) == 0 {
		return quiet(th, none)
	}
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		children := make([]layout.FlexChild, 0, 2*len(spends))
		for index, spend := range spends {
			if index > 0 {
				children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout))
			}
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return kit.Label{Style: th.T.Label, Color: th.R.Fg, MaxLines: 2}.Layout(gtx, th.Theme, spend.Name)
							}),
							layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, tasksWord(spend.Tasks))),
						)
					}),
					layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
					layout.Rigid(kit.Text(th.Theme, th.T.Detail, th.R.Fg2, i18n.Tf("%s tokens", teamsui.Tokens(spend.Tokens)))),
				)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}
