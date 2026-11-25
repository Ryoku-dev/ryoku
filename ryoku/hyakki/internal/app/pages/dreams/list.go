package dreams

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// List draws every dream by name with its phase and team.
func (p *Page) List(gtx layout.Context, e *env.Env) look.ListDimensions {
	snapshot := e.Snapshot()
	dreams := ordered(teamsui.Dreams(snapshot.Teams))
	rows := make([]look.SideRow, 0, len(dreams))
	for _, dream := range dreams {
		rows = append(rows, dreamRow(snapshot, dream))
	}
	p.list.Empty = i18n.T("Dreams you start are listed here.")
	dimensions := p.list.Layout(gtx, e.Theme, e.Overlays, []look.SideGroup{{Rows: rows}}, p.place.Team)
	for _, event := range p.list.Events() {
		if event.Kind == look.SideOpen {
			openDream(e, event.ID)
		}
	}
	return dimensions
}

func dreamRow(snapshot state.Snapshot, dream protocol.Team) look.SideRow {
	_, tone, working := standing(snapshot, dream)
	row := look.SideRow{ID: dream.ID, Title: dream.Name, Working: working, Facts: i18n.Tf("%s, team of %d", phaseWord(dream.Dream.Phase), len(dream.Members))}
	if tone == palette.Attention {
		row.Tone = tone
	}
	return row
}
