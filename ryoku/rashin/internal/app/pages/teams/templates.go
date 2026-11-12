package teams

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/protocol"
)

// the ready-made teams as tablets in the draft, grouped by level, the one it started from selected; picking one fills the form
type templateRows struct {
	rows map[string]*kit.Tablet
}

func (t *templateRows) row(id string) *kit.Tablet {
	if t.rows == nil {
		t.rows = make(map[string]*kit.Tablet)
	}
	row := t.rows[id]
	if row == nil {
		row = new(kit.Tablet)
		t.rows[id] = row
	}
	return row
}

func (t *templateRows) layoutPick(gtx layout.Context, e *env.Env, templates []protocol.TeamTemplate, picked string, pick func(protocol.TeamTemplate)) layout.Dimensions {
	th := e.Theme
	if len(templates) == 0 {
		return look.SkeletonRows(gtx, th, 5, true)
	}
	return teamsui.LayoutGroups(gtx, th, templates, func(gtx layout.Context, template protocol.TeamTemplate) layout.Dimensions {
		tablet := t.row(template.ID)
		tablet.Pane = th.R.Stone2
		if tablet.Clicked(gtx) && template.ID != picked {
			pick(template)
		}
		content := kit.TabletContent{Mark: teamsui.Mark(th, template.Name, 28, false), Title: template.Name, Facts: teamsui.TemplateLine(template)}
		return tablet.Layout(gtx, th.Theme, content, template.ID == picked, false)
	})
}
