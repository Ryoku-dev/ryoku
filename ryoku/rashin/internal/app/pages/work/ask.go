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
	// an engine for this task alone waits under More, set again for each member it is for
	more      kit.Disclosure
	moreClick kit.Click
	engine    teamsui.EngineForm
	engineFor string
}

func (d *askDialog) open(team string) {
	d.shown, d.dialog.Open, d.dialog.Width = true, true, 600
	d.teamID, d.memberID = team, ""
	d.brief.SetText("")
	d.brief.Focus()
	d.more.Open, d.engineFor = false, ""
}

func (d *askDialog) layout(gtx layout.Context, e *env.Env) {
	if !d.shown {
		return
	}
	th := e.Theme
	snapshot := e.Snapshot()
	teams := snapshot.Teams
	team, picked := teamsui.TeamByID(teams, d.teamID)
	if !picked && len(teams) > 0 {
		team, picked = teams[0], true
		d.teamID, d.memberID = team.ID, ""
	}
	d.keep(team)
	_, model, options := d.engine.Value()
	brief := strings.TrimSpace(d.brief.Text())
	ready := picked && brief != "" && protocol.ValidEngineModel(model)
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	d.start.Kind, d.start.Label = kit.PlaqueGold, i18n.T("Start")
	if d.cancel.Clicked(gtx) {
		d.dialog.Dismiss(gtx)
	}
	if (d.start.Clicked(gtx) || d.brief.Submitted(gtx)) && ready {
		params := protocol.TeamTaskCreateParams{Team: team.ID, Member: d.memberID, Title: protocol.BriefTitle(brief), Brief: brief}
		if model != "" || len(options) > 0 {
			_, params.Model, params.Options = d.engine.Effective(snapshot)
		}
		d.create(e, params)
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

// the engine form changes the engine of the member the task goes to: the lead unless one is picked
func (d *askDialog) keep(team protocol.Team) {
	var chosen *protocol.Member
	for index := range team.Members {
		member := &team.Members[index]
		if member.ID == d.memberID || d.memberID == "" && member.Lead {
			chosen = member
		}
	}
	d.engine.Keep, d.engine.Team = chosen, team.ID
	key := team.ID + "\x00" + d.memberID
	if key != d.engineFor {
		d.engineFor = key
		d.engine.Set("", "", nil)
	}
}

func (d *askDialog) body(gtx layout.Context, e *env.Env, teams []protocol.Team, team protocol.Team, picked bool) layout.Dimensions {
	th := e.Theme
	snapshot := e.Snapshot()
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
	leads := i18n.T("The lead takes it and hands pieces to the others.")
	if lead, at := teamsui.Lead(team.Members); at >= 0 {
		leads = i18n.Tf("The lead takes it on %s and hands pieces to the others.", teamsui.MemberEngine(snapshot, lead).Short())
	}
	members := []look.SelectOption{{Label: i18n.T("The lead decides"), Detail: leads}}
	ids := []string{""}
	memberIndex := 0
	for _, member := range team.Members {
		if member.Lead {
			continue
		}
		if member.ID == d.memberID {
			memberIndex = len(ids)
		}
		members = append(members, look.SelectOption{Label: member.Name, Detail: i18n.Tf("%s on %s", member.Title, teamsui.MemberEngine(snapshot, member).Short())})
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
	if d.engine.Keep != nil {
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout), layout.Rigid(d.layoutMore(gtx, e)))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// most tasks run on their member's engine, so another one for this task waits under More
func (d *askDialog) layoutMore(gtx layout.Context, e *env.Env) layout.Widget {
	th := e.Theme
	if d.moreClick.Clicked(gtx) {
		d.more.Open = !d.more.Open
	}
	arrow := look.Icon.ExpandMore
	if d.more.Open {
		arrow = look.Icon.ExpandLess
	}
	content := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(kit.Text(th.Theme, th.T.Row, th.R.Fg, i18n.T("Engine for this task"))),
			layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg3, i18n.Tf("Only this task runs on it; %s keeps its own.", d.engine.Keep.Name))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.engine.Layout(gtx, e, "") }),
		)
	}
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return d.moreClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(kit.Text(th.Theme, th.T.Section, th.R.Fg, i18n.T("More"))),
						layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
						layout.Rigid(look.IconWidget(arrow, 20, th.R.Fg2)),
					)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.more.Layout(gtx, th.Theme, content) }),
		)
	}
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
