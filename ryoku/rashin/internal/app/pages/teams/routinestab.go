package teams

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/protocol"
)

// the team's routines on the tablets the Routines page shows, with New routine
type routinesTab struct {
	scroll    look.Scroller
	add       kit.Plaque
	tablets   map[string]*teamsui.RoutineTablet
	schedules teamsui.Schedules
}

func (r *routinesTab) tablet(id string) *teamsui.RoutineTablet {
	if r.tablets == nil {
		r.tablets = make(map[string]*teamsui.RoutineTablet)
	}
	tablet := r.tablets[id]
	if tablet == nil {
		tablet = new(teamsui.RoutineTablet)
		r.tablets[id] = tablet
	}
	return tablet
}

func (r *routinesTab) layout(gtx layout.Context, e *env.Env, team protocol.Team) layout.Dimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	r.add.Kind, r.add.Small, r.add.Icon, r.add.Label = kit.PlaqueLine, true, look.Icon.Add, i18n.T("New routine")
	if r.add.Clicked(gtx) {
		e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabRoutines, Draft: env.DraftRoutine, Team: team.ID})
	}
	var rows []layout.Widget
	for _, routine := range snapshot.Routines {
		if routine.Team != team.ID {
			continue
		}
		tablet := r.tablet(routine.ID)
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			return tablet.Layout(gtx, e, &r.schedules, routine, false)
		})
	}
	return r.scroll.Layout(gtx, th, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return pageInset(gtx, e, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			body := stack(th, rows)
			if len(rows) == 0 {
				body = quiet(th, i18n.Tf("%s has no routine yet. A routine gives the team a task by itself, on a schedule.", team.Name))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return r.add.Layout(gtx, th.Theme) }),
				layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
				layout.Rigid(body),
			)
		})
	})
}
