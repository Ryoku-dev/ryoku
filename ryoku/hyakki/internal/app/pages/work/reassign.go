package work

import (
	"time"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type reassignDialog struct {
	id, team, session, originalMember string
	teamUpdated                       time.Time
	dialog                            kit.Dialog
	member                            look.Select
	options                           []look.SelectOption
	members                           []string
	selected                          int
	do, cancel                        kit.Plaque
}

func (d *reassignDialog) open(task protocol.TeamTask, team protocol.Team) {
	d.id, d.team, d.session, d.originalMember = task.ID, team.ID, task.Session, task.Member
	d.teamUpdated, d.selected = team.Updated, -1
	d.dialog.Open, d.dialog.Width = true, 480
	d.options = make([]look.SelectOption, len(team.Members))
	d.members = make([]string, len(team.Members))
	for index, member := range team.Members {
		d.options[index] = look.SelectOption{Label: member.Name, Detail: member.Title, Disabled: member.ID == task.Member}
		d.members[index] = member.ID
	}
}

func (d *reassignDialog) layout(gtx layout.Context, e *env.Env) {
	if d.id == "" {
		return
	}
	if index, changed := d.member.Changed(); changed && index >= 0 && index < len(d.members) {
		d.selected = index
	}
	snapshot := e.Snapshot()
	task, known := snapshot.TeamTasks[d.id]
	current := known && task.Team == d.team && task.Session == d.session && task.Member == d.originalMember
	teamCurrent := false
	for _, team := range snapshot.Teams {
		if team.ID == d.team {
			teamCurrent = team.Updated.Equal(d.teamUpdated)
			break
		}
	}
	current = current && teamCurrent
	valid := current && d.selected >= 0 && d.selected < len(d.members) && d.members[d.selected] != d.originalMember
	d.member.Label = i18n.T("New assignee")
	d.do.Kind, d.do.Label = kit.PlaqueGold, i18n.T("Stop and reassign")
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	if d.cancel.Clicked(gtx) {
		d.dialog.Dismiss(gtx)
	} else if d.do.Clicked(gtx) && valid {
		e.Call(protocol.MTeamsTaskReassign, protocol.TeamTaskReassignParams{ID: d.id, Member: d.members[d.selected]}, new(protocol.TeamTask), e.Report)
		d.dialog.Dismiss(gtx)
	}
	th := e.Theme
	body := i18n.T("The current execution and its delegated work stop first. Its conversations and previous result stay in its history. The new assignee must follow the current team policy and obtain fresh strategy and action approvals.")
	if !current {
		body = i18n.T("The task assignment or team changed. Cancel and review the current task before reassigning it.")
	}
	d.dialog.Layout(gtx, th.Theme, i18n.T("Reassign this task"),
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg2, body)),
				layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !current {
						gtx = gtx.Disabled()
					}
					return d.member.LayoutOptions(gtx, th, e.Overlays, d.options, d.selected)
				}),
			)
		},
		func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions {
			if !valid {
				gtx = gtx.Disabled()
			}
			return d.do.Layout(gtx, th.Theme)
		},
	)
	if d.dialog.Closed() {
		d.member.Close(e.Overlays)
		d.id, d.options, d.members = "", nil, nil
	}
}
