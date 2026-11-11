package work

import (
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// a task's title is its brief's first line, cut to what a row can show
const titleRunes = 80

// the dialog that gives a team a task
type askDialog struct {
	shown  bool
	dialog kit.Dialog
	brief  look.MultiLine
	team   look.Select
	member look.Select
	// the picked team and member by id, "" for the lead
	teamID, memberID string
	start, cancel    kit.Plaque
}

func (d *askDialog) open(team string) {
	d.shown, d.dialog.Open, d.dialog.Width = true, true, 600
	d.teamID, d.memberID = team, ""
	d.brief.SetText("")
	d.brief.Focus()
}

func (d *askDialog) layout(gtx layout.Context, e *env.Env) {
	if !d.shown {
		return
	}
	th := e.Theme
	teams := e.Snapshot().Teams
	team, picked := teamsui.TeamByID(teams, d.teamID)
	if !picked && len(teams) > 0 {
		team, picked = teams[0], true
		d.teamID, d.memberID = team.ID, ""
	}
	brief := strings.TrimSpace(d.brief.Text())
	ready := picked && brief != ""
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	d.start.Kind, d.start.Label = kit.PlaqueGold, i18n.T("Start")
	if d.cancel.Clicked(gtx) {
		d.dialog.Dismiss(gtx)
	}
	if (d.start.Clicked(gtx) || d.brief.Submitted(gtx)) && ready {
		d.create(e, protocol.TeamTaskCreateParams{Team: team.ID, Member: d.memberID, Title: briefTitle(brief), Brief: brief})
		d.dialog.Dismiss(gtx)
	}
	d.dialog.Layout(gtx, th.Theme, i18n.T("Give a team a task"),
		func(gtx layout.Context) layout.Dimensions { return d.body(gtx, e, teams, team, picked) },
		func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions {
			if !ready {
				gtx = gtx.Disabled()
			}
			return d.start.Layout(gtx, th.Theme)
		},
	)
	if d.dialog.Closed() {
		d.shown = false
	}
}

func (d *askDialog) body(gtx layout.Context, e *env.Env, teams []protocol.Team, team protocol.Team, picked bool) layout.Dimensions {
	th := e.Theme
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			d.brief.Lines, d.brief.Placeholder = 6, i18n.T("What should the team do?")
			return d.brief.Layout(gtx, th)
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
	}
	if !picked {
		children = append(children, layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg3, i18n.T("A task goes to a team. Start one first, then give it something to do."))))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	names := make([]string, len(teams))
	teamIndex := 0
	for index, each := range teams {
		names[index] = each.Name
		if each.ID == team.ID {
			teamIndex = index
		}
	}
	// the lead hands pieces to the members, so naming the lead itself would change nothing
	members := []look.SelectOption{{Label: i18n.T("The lead decides"), Detail: i18n.T("The lead takes it and hands pieces to the others.")}}
	ids := []string{""}
	memberIndex := 0
	for _, member := range team.Members {
		if member.Lead {
			continue
		}
		if member.ID == d.memberID {
			memberIndex = len(ids)
		}
		members = append(members, look.SelectOption{Label: member.Name, Detail: member.Title})
		ids = append(ids, member.ID)
	}
	d.team.Form, d.team.Label, d.team.Icon = look.SelectQuiet, i18n.T("Team"), look.Icon.Teams
	d.member.Form, d.member.Label, d.member.Icon = look.SelectQuiet, i18n.T("Member"), look.Icon.Member
	children = append(children,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dimensions := d.team.Layout(gtx, th, e.Overlays, names, teamIndex)
					if index, ok := d.team.Changed(); ok && index < len(teams) {
						d.teamID, d.memberID = teams[index].ID, ""
					}
					return dimensions
				}),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dimensions := d.member.LayoutOptions(gtx, th, e.Overlays, members, memberIndex)
					if index, ok := d.member.Changed(); ok && index < len(ids) {
						d.memberID = ids[index]
					}
					return dimensions
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg3, workNote(team))),
	)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (d *askDialog) create(e *env.Env, params protocol.TeamTaskCreateParams) {
	var created protocol.TeamTask
	e.Call(protocol.MTeamsTaskCreate, params, &created, func(err error) {
		if err != nil {
			// the brief is still in the field, so the dialog comes back with it
			d.shown, d.dialog.Open = true, true
			e.Notify(err.Error())
			return
		}
		teamsui.OpenTask(e, created.ID)
	})
}

func briefTitle(brief string) string {
	line, _, _ := strings.Cut(brief, "\n")
	runes := []rune(strings.TrimSpace(line))
	if len(runes) > titleRunes {
		runes = runes[:titleRunes]
	}
	return strings.TrimSpace(string(runes))
}

// where the team works and how far it goes before asking, so the task is given knowing it
func workNote(team protocol.Team) string {
	folder := paths.Short(team.Folder)
	var note string
	switch team.Approvals {
	case protocol.ApprovalsReadOnly:
		note = i18n.Tf("%s works in %s and only reads: it changes nothing.", team.Name, folder)
	case protocol.ApprovalsAuto:
		note = i18n.Tf("%s works in %s and acts without asking.", team.Name, folder)
	default:
		note = i18n.Tf("%s works in %s and asks before it changes files.", team.Name, folder)
	}
	if team.Paused {
		note += " " + i18n.T("It is paused, so the task waits until it works again.")
	}
	return note
}
