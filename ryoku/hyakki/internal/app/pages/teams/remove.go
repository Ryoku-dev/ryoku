package teams

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// the danger zone's confirms: deleting a team, whose name is typed since it cannot be undone, or clearing its finished tasks
type removeTeam struct {
	confirm kit.Confirm
	team    protocol.Team
	// finished is how many finished tasks a clear takes, zero when the team itself goes
	finished int
}

func (d *removeTeam) open(team protocol.Team) {
	d.team, d.finished = team, 0
	d.confirm.Danger, d.confirm.Dialog.Open = true, true
}

func (d *removeTeam) openClear(team protocol.Team, finished int) {
	d.team, d.finished = team, finished
	d.confirm.Danger, d.confirm.Dialog.Open = true, true
}

func (d *removeTeam) layout(gtx layout.Context, e *env.Env) {
	if d.team.ID == "" {
		return
	}
	if d.finished > 0 {
		d.layoutClear(gtx, e)
	} else {
		d.layoutDelete(gtx, e)
	}
	if d.confirm.Closed() {
		d.team = protocol.Team{}
	}
}

func (d *removeTeam) layoutDelete(gtx layout.Context, e *env.Env) {
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
}

func (d *removeTeam) layoutClear(gtx layout.Context, e *env.Env) {
	team, count := d.team, d.finished
	body := i18n.Tf("Delete %d finished tasks and their conversations? What they made stays in the folder.", count)
	if count == 1 {
		body = i18n.T("Delete 1 finished task and its conversation? What it made stays in the folder.")
	}
	if d.confirm.Layout(gtx, e.Theme.Theme, i18n.Tf("Clear the finished tasks of %s?", team.Name), body, i18n.T("Clear"), "") {
		e.Call(protocol.MTeamsPrune, protocol.TeamPruneParams{Team: team.ID, All: true}, new(protocol.Empty), func(err error) {
			if err != nil {
				e.Notify(err.Error())
				return
			}
			e.Notify(i18n.Tf("Cleared the finished tasks of %s.", team.Name))
		})
	}
}
