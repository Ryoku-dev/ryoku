package settings

import (
	"gioui.org/layout"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// what every team shares, each team's own settings live on its page
func (p *Page) layoutTeams(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	atOnce := e.Snapshot().Settings.Teams.MaxRunning
	if atOnce == 0 {
		atOnce = protocol.DefaultTeamsMaxRunning
	}
	p.atOnce.Value, p.atOnce.Min, p.atOnce.Max, p.atOnce.Step = atOnce, 1, 6, 1
	return look.SettingsSection(gtx, th, i18n.T("Teams"), []look.Setting{
		{
			Title:  i18n.T("Tasks at once"),
			Detail: i18n.T("How many team tasks work at the same time across all teams."),
			Control: func(gtx layout.Context) layout.Dimensions {
				dimensions := p.atOnce.Layout(gtx, th.Theme)
				if p.atOnce.Changed() {
					set(e, "teams", map[string]any{"maxRunning": p.atOnce.Value})
				}
				return dimensions
			},
		},
	})
}
