package teams

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// deleting a team cannot be undone, so its name is typed to confirm
type removeTeam struct {
	confirm kit.Confirm
	team    protocol.Team
}

func (d *removeTeam) open(team protocol.Team) {
	d.team = team
	d.confirm.Danger, d.confirm.Dialog.Open = true, true
}

func (d *removeTeam) layout(gtx layout.Context, e *env.Env) {
	if d.team.ID == "" {
		return
	}
	team := d.team
	title := i18n.Tf("Delete %s?", team.Name)
	body := i18n.Tf("Its tasks, its routines and its use of credentials go. The folder %s stays, with what the team made.", paths.Short(team.Folder))
	if d.confirm.Layout(gtx, e.Theme.Theme, title, body, i18n.T("Delete team"), team.Name) {
		e.Call(protocol.MTeamsDelete, protocol.TeamRef{ID: team.ID}, new(protocol.Empty), func(err error) {
			if err != nil {
				e.Notify(err.Error())
				return
			}
			e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabTeams})
			e.Notify(i18n.Tf("Deleted %s.", team.Name))
		})
	}
	if d.confirm.Closed() {
		d.team = protocol.Team{}
	}
}
