package routines

import (
	"image"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
)

func (p *Page) List(gtx layout.Context, e *env.Env) look.ListDimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	groups := make([]look.SideGroup, 0, len(snapshot.Teams))
	for _, team := range snapshot.Teams {
		var rows []look.SideRow
		for _, routine := range snapshot.Routines {
			if routine.Team != team.ID {
				continue
			}
			facts := p.schedules.Sentence(routine)
			if live(snapshot, routine) && routine.NextRun != nil {
				facts = look.Clauses(facts, look.Until(gtx.Now, *routine.NextRun))
			}
			row := look.SideRow{ID: routine.ID, Title: routine.Title, Facts: facts, Menu: true}
			if word, tone := teamsui.RoutineStanding(routine); word != "" {
				row.Trail = kit.Claim(th.Theme, tone, word)
				// only a failing routine wants attention, one that is off is just off
				if tone != palette.Paused {
					row.Tone = tone
				}
			} else if quiet := teamsui.QuietStreak(routine); quiet != "" {
				row.Trail = kit.Claim(th.Theme, palette.Idle, quiet)
			}
			rows = append(rows, row)
		}
		if len(rows) > 0 {
			groups = append(groups, look.SideGroup{Title: team.Name, Count: len(rows), Rows: rows})
		}
	}
	p.list.Empty = i18n.T("Routines give a team a task on a clock. They are kept here by team.")
	dimensions := p.list.Layout(gtx, th, e.Overlays, groups, p.selected)
	for _, event := range p.list.Events() {
		switch event.Kind {
		case look.SideOpen:
			teamsui.OpenRoutine(e, event.ID)
		case look.SideMenu:
			p.openRowMenu(e, event.ID, event.Anchor)
		case look.SideRename:
			if event.Title != "" {
				teamsui.UpdateRoutine(e, event.ID, map[string]any{"title": event.Title}, e.Report)
			}
		}
	}
	return dimensions
}

func (p *Page) openRowMenu(e *env.Env, id string, anchor image.Rectangle) {
	routine, ok := routineByID(e.Snapshot(), id)
	if !ok {
		return
	}
	turn := kit.MenuItem{Icon: look.Icon.Pause, Label: i18n.T("Turn off")}
	if !routine.Enabled {
		turn = kit.MenuItem{Icon: look.Icon.Play, Label: i18n.T("Turn on")}
	}
	items := []kit.MenuItem{
		{Icon: look.Icon.Play, Label: i18n.T("Run now")},
		turn,
		{Icon: look.Icon.Rename, Label: i18n.T("Rename"), Shortcut: "F2"},
		{Divider: true},
		{Icon: look.Icon.Delete, Label: i18n.T("Delete"), Danger: true},
	}
	tag := &p.rowMenu
	e.Overlays.Open(tag, anchor, look.Placement{}, func(gtx layout.Context) layout.Dimensions {
		dimensions := p.rowMenu.Layout(gtx, e.Theme.Theme, items)
		item, ok := p.rowMenu.Clicked(gtx)
		if !ok {
			return dimensions
		}
		e.Overlays.Close(tag)
		switch item {
		case 0:
			teamsui.RunRoutine(e, routine)
		case 1:
			teamsui.SetRoutineEnabled(e, e.Snapshot(), routine, !routine.Enabled, e.Report)
		case 2:
			p.list.Rename(id)
		case 4:
			p.remove.open(routine)
		}
		return dimensions
	})
}
