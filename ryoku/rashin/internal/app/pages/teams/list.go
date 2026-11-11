package teams

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/protocol"
)

// List draws every team by name with how its crew stands.
func (p *Page) List(gtx layout.Context, e *env.Env) look.ListDimensions {
	snapshot := e.Snapshot()
	rows := make([]look.SideRow, 0, len(snapshot.Teams))
	for _, team := range snapshot.Teams {
		rows = append(rows, teamRow(e.Theme, team))
	}
	p.list.Empty = i18n.T("Teams you make are listed here.")
	selected := ""
	if p.place.Draft == "" {
		selected = p.place.Team
	}
	dimensions := p.list.Layout(gtx, e.Theme, e.Overlays, []look.SideGroup{{Rows: rows}}, selected)
	for _, event := range p.list.Events() {
		if event.Kind != look.SideOpen {
			continue
		}
		view := env.TeamOverview
		if event.ID == p.place.Team {
			view = p.place.View
		}
		openTeam(e, event.ID, view)
	}
	return dimensions
}

func teamRow(th *look.Theme, team protocol.Team) look.SideRow {
	_, tone, working := teamsui.TeamStanding(team)
	row := look.SideRow{
		ID: team.ID, Title: team.Name, Working: working,
		Facts: i18n.Tf("%s · %d working", membersWord(len(team.Members)), team.Stats.Working),
	}
	if tone == palette.Attention {
		row.Tone = tone
	}
	if team.Paused {
		row.Trail = kit.Text(th.Theme, th.T.Label, th.R.Stone(tone), i18n.T("Paused"))
	}
	return row
}
