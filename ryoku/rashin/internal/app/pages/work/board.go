package work

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

// every team's tasks on one board, filtered by team
type board struct {
	header  look.PaneHeader
	scroll  look.Scroller
	filters kit.Filters
	// the board's Team is the team the filter shows, "" for every team
	tasks teamsui.TaskBoard
	empty kit.Plaque
}

func (b *board) layout(gtx layout.Context, e *env.Env, page *Page) layout.Dimensions {
	snapshot := e.Snapshot()
	b.keepTeam(snapshot)
	rows, total := b.tasks.Rows(gtx, snapshot)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			content := look.PaneHeaderContent{
				Title:    i18n.T("Work"),
				Actions:  []look.HeaderAction{{Icon: look.Icon.Add, Label: i18n.T("New task")}},
				Scrolled: b.scroll.Position.First > 0 || b.scroll.Position.Offset > 0,
			}
			dimensions := b.header.Layout(gtx, e.Theme, e.Overlays, content)
			if _, ok := b.header.ActionClicked(); ok {
				page.Ask(e, b.tasks.Team)
			}
			return dimensions
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(snapshot.Teams) == 0 || total == 0 && b.tasks.Team == "" {
				return b.layoutEmpty(gtx, e, page, len(snapshot.Teams) > 0)
			}
			return b.scroll.Layout(gtx, e.Theme, 1+rows, func(gtx layout.Context, index int) layout.Dimensions {
				return look.Centred(gtx, e.Class, look.BoardWidth, func(gtx layout.Context) layout.Dimensions {
					if index == 0 {
						return layout.Inset{Top: e.Theme.Sp.S4, Bottom: e.Theme.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return b.layoutFilters(gtx, e, snapshot)
						})
					}
					return b.tasks.LayoutRow(gtx, e, index-1)
				})
			})
		}),
	)
}

// a team that was removed takes its filter with it
func (b *board) keepTeam(snapshot state.Snapshot) {
	if _, ok := teamsui.TeamByID(snapshot.Teams, b.tasks.Team); !ok {
		b.tasks.Team = ""
	}
}

func (b *board) layoutFilters(gtx layout.Context, e *env.Env, snapshot state.Snapshot) layout.Dimensions {
	counts := make(map[string]int)
	all := 0
	for _, task := range snapshot.TeamTasks {
		if task.Parent == "" {
			counts[task.Team]++
			all++
		}
	}
	b.filters.Labels = append(b.filters.Labels[:0], i18n.T("All"))
	b.filters.Counts = append(b.filters.Counts[:0], all)
	b.filters.Selected = 0
	for index, team := range snapshot.Teams {
		b.filters.Labels = append(b.filters.Labels, team.Name)
		b.filters.Counts = append(b.filters.Counts, counts[team.ID])
		if team.ID == b.tasks.Team {
			b.filters.Selected = index + 1
		}
	}
	dimensions := b.filters.Layout(gtx, e.Theme.Theme)
	if index, ok := b.filters.Changed(); ok {
		b.tasks.Team = ""
		if index > 0 && index <= len(snapshot.Teams) {
			b.tasks.Team = snapshot.Teams[index-1].ID
		}
	}
	return dimensions
}

func (b *board) layoutEmpty(gtx layout.Context, e *env.Env, page *Page, teams bool) layout.Dimensions {
	th := e.Theme
	empty := kit.Empty{Icon: look.Icon.Tasks, Actions: []*kit.Plaque{&b.empty}}
	b.empty.Kind = kit.PlaqueLine
	if teams {
		empty.Title, empty.Body = i18n.T("No task yet"), i18n.T("Give a team a task and it shows here while it works, then with its report.")
		b.empty.Label = i18n.T("New task")
		if b.empty.Clicked(gtx) {
			page.Ask(e, "")
		}
	} else {
		empty.Title, empty.Body = i18n.T("No team yet"), i18n.T("Tasks go to a team. Start one and give it something to do.")
		b.empty.Label = i18n.T("Start a team")
		if b.empty.Clicked(gtx) {
			e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabTeams, Draft: env.DraftTeam})
		}
	}
	return empty.Layout(gtx, th.Theme)
}
