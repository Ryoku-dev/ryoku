package teams

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func (d *draft) layoutForm(gtx layout.Context, e *env.Env, p *Page) layout.Dimensions {
	th := e.Theme
	if !d.filled {
		return look.SkeletonRows(gtx, th, 6, true)
	}
	d.handleMembers(gtx)
	parts := []layout.Widget{
		section(th, i18n.T("Start from"), func(gtx layout.Context) layout.Dimensions {
			return d.starts.layoutPick(gtx, e, p.templates.List(e), d.template, func(template protocol.TeamTemplate) { d.apply(e, template) })
		}),
		section(th, i18n.T("The team"), d.layoutIdentity(e)),
		section(th, i18n.T("Lead"), d.layoutMember(e, d.members[0], false)),
		section(th, i18n.T("Members"), d.layoutMembers(e)),
		section(th, i18n.T("Tools"), d.layoutTools(th)),
		section(th, i18n.T("Permissions and spending"), d.layoutLimits(e)),
		d.layoutMore(gtx, e),
	}
	if d.problem != "" {
		parts = append(parts, look.Paragraph(th, th.T.Body, th.R.Jasper, d.problem))
	}
	return sections(gtx, parts...)
}

// clicks are taken before the rows are drawn, so a removed row is never drawn this frame
func (d *draft) handleMembers(gtx layout.Context) {
	for index := 1; index < len(d.members); index++ {
		if d.members[index].remove.Clicked(gtx) {
			d.members = append(d.members[:index], d.members[index+1:]...)
			break
		}
	}
	d.add.Kind, d.add.Small, d.add.Icon, d.add.Label = kit.PlaqueLine, true, look.Icon.Add, i18n.T("Add a member")
	if d.add.Clicked(gtx) && len(d.members) < membersMax {
		d.members = append(d.members, new(draftMember))
	}
	d.markLead()
}

func (d *draft) layoutIdentity(e *env.Env) layout.Widget {
	th := e.Theme
	d.name.Label, d.name.Placeholder = i18n.T("Name"), i18n.T("Research desk")
	d.purpose.Label, d.purpose.Lines = i18n.T("Purpose"), 3
	d.purpose.Placeholder = i18n.T("What the team is for, in a sentence or two. Every member reads it with every task.")
	d.folder.Label, d.folder.Title = i18n.T("Folder"), i18n.T("The team's folder")
	d.folder.Placeholder = paths.Short(d.folderPath())
	d.folder.Help = i18n.T("Where the team works and keeps what it makes. Empty makes one under ~/Teams.")
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.name.Layout(gtx, th.Theme) }),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.purpose.Layout(gtx, th) }),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.folder.Layout(gtx, e) }),
		)
	}
}

// two fields side by side, 8 apart
func pair(th *look.Theme, left, right layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
			layout.Flexed(1, left),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Flexed(1, right),
		)
	}
}

func (d *draft) layoutMember(e *env.Env, row *draftMember, removable bool) layout.Widget {
	th := e.Theme
	form := &row.form
	fields := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(pair(th, form.nameField(th), form.titleField(th))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(form.instructionsField(th)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(d.layoutEngine(e, row)),
		)
	}
	if !removable {
		return fields
	}
	row.remove.Kind, row.remove.Small, row.remove.Icon = kit.PlaqueQuiet, true, look.Icon.Close
	row.remove.Description = i18n.T("Leave this member out")
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
			layout.Flexed(1, fields),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return row.tip.Layout(gtx, th, e.Overlays, row.remove.Description, func(gtx layout.Context) layout.Dimensions {
						return row.remove.Layout(gtx, th.Theme)
					})
				})
			}),
		)
	}
}

// the engine reads as one line until it is opened, as most members keep the teams default
func (d *draft) layoutEngine(e *env.Env, row *draftMember) layout.Widget {
	th := e.Theme
	return func(gtx layout.Context) layout.Dimensions {
		if row.engineClick.Clicked(gtx) {
			row.engine.Open = !row.engine.Open
		}
		line := teamsui.MemberEngine(e.Snapshot(), row.form.member()).Line()
		arrow := look.Icon.ExpandMore
		if row.engine.Open {
			arrow = look.Icon.ExpandLess
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return row.engineClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(look.IconWidget(arrow, 20, th.R.Fg2)),
						layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
						layout.Flexed(1, look.Paragraph(th, th.T.Caption, th.R.Fg2, i18n.Tf("Engine: %s", line))),
					)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return row.engine.Layout(gtx, th.Theme, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: th.Sp.S2}.Layout(gtx, row.form.engineField(e))
				})
			}),
		)
	}
}

func (d *draft) layoutMembers(e *env.Env) layout.Widget {
	th := e.Theme
	rows := make([]layout.Widget, 0, len(d.members))
	for _, row := range d.members[1:] {
		rows = append(rows, d.layoutMember(e, row, true))
	}
	full := len(d.members) >= membersMax
	return func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, 2*len(rows)+2)
		if len(rows) == 0 {
			children = append(children, layout.Rigid(quiet(th, i18n.T("The lead works alone. Add members it can hand pieces of a task to."))))
		}
		for _, row := range rows {
			children = append(children, layout.Rigid(row), layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if full {
				return quiet(th, i18n.Tf("A team has at most %d members, its lead included.", membersMax))(gtx)
			}
			return layout.Inset{Top: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return d.add.Layout(gtx, th.Theme) })
		}))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}

func (d *draft) layoutTools(th *look.Theme) layout.Widget {
	d.screen.Label, d.mail.Label = i18n.T("Its own screen"), i18n.T("Mail")
	tool := func(toggle *kit.Toggle, detail string) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return toggle.Layout(gtx, th.Theme) }),
				layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
				layout.Rigid(quiet(th, detail)),
			)
		}
	}
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(tool(&d.screen, i18n.T("Opens apps on a hidden screen you can watch. It does not ask each time."))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(tool(&d.mail, i18n.T("Writes mail; every mail asks you first unless you trust its recipient."))),
		)
	}
}

// a labelled control in the form: the caption over it, as a field's label stands
func labelled(th *look.Theme, label string, control layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, label)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(control),
		)
	}
}

func (d *draft) layoutLimits(e *env.Env) layout.Widget {
	th := e.Theme
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(labelled(th, i18n.T("Permissions"), func(gtx layout.Context) layout.Dimensions {
				dimensions := teamsui.ApprovalsSelect(gtx, e, &d.approvals, d.approvalsValue)
				if index, changed := d.approvals.Changed(); changed && index >= 0 && index < len(teamsui.ApprovalValues) {
					d.approvalsValue = teamsui.ApprovalValues[index]
				}
				return dimensions
			})),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(labelled(th, i18n.T("Spending limit"), func(gtx layout.Context) layout.Dimensions {
				return d.limit.Layout(gtx, th)
			})),
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(quiet(th, i18n.Tf("At the limit the team pauses and asks you to raise it. You are told at %d percent.", d.warn))),
		)
	}
}

// what most teams never change waits under More
func (d *draft) layoutMore(gtx layout.Context, e *env.Env) layout.Widget {
	th := e.Theme
	if d.moreClick.Clicked(gtx) {
		d.more.Open = !d.more.Open
	}
	d.turns.Min, d.turns.Max, d.turns.Step, d.turns.Unit = 1, protocol.TeamTaskTurnsMax, 1, i18n.T("turns per task")
	d.minutes.Min, d.minutes.Max, d.minutes.Step, d.minutes.Unit = 0, protocol.TeamTaskMinutesMax, 15, i18n.T("minutes per task")
	toggleRoutines := make([]layout.Widget, len(d.routines))
	for index, routine := range d.routines {
		toggle := &d.routineOn[index]
		toggle.Label = i18n.T("Turn this routine on")
		when := d.sentences[routine.Schedule]
		if when == "" {
			when = routine.Schedule
		}
		toggleRoutines[index] = func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(kit.Text(th.Theme, th.T.Row, th.R.Fg, routine.Title)),
				layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg2, when)),
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return toggle.Layout(gtx, th.Theme) }),
			)
		}
	}
	content := func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(labelled(th, i18n.T("Turns per task"), func(gtx layout.Context) layout.Dimensions { return d.turns.Layout(gtx, th.Theme) })),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(labelled(th, i18n.T("Minutes per task"), func(gtx layout.Context) layout.Dimensions { return d.minutes.Layout(gtx, th.Theme) })),
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(quiet(th, i18n.T("A task that runs past it stops and says so. 0 lets it work as long as it needs."))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(labelled(th, i18n.T("Notifications"), func(gtx layout.Context) layout.Dimensions {
				dimensions := teamsui.NotifySelect(gtx, e, &d.notify, d.notifyValue)
				if index, changed := d.notify.Changed(); changed && index >= 0 && index < len(teamsui.NotifyValues) {
					d.notifyValue = teamsui.NotifyValues[index]
				}
				return dimensions
			})),
		}
		for _, routine := range toggleRoutines {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout), layout.Rigid(routine))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	arrow := look.Icon.ExpandMore
	if d.more.Open {
		arrow = look.Icon.ExpandLess
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

func (d *draft) layoutAside(gtx layout.Context, e *env.Env, members []protocol.Member) layout.Dimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	rows := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions {
			return crewChart(gtx, th, snapshot, members, nil, miniCrew(th))
		},
		kit.AsideNote(th.Theme, i18n.Tf("Works in %s.", paths.Short(d.folderPath()))),
		kit.AsideNote(th.Theme, i18n.T("Runs in a sandbox: it reads your files but writes only in its folder, and never reaches your keyring, the vault or other teams.")),
	}
	if d.screen.Value {
		rows = append(rows, kit.AsideNote(th.Theme, i18n.T("May use its own screen.")))
	}
	rows = append(rows, kit.AsideNote(th.Theme, approvalsSentence(d.approvalsValue)))
	if d.mail.Value {
		rows = append(rows, kit.AsideNote(th.Theme, i18n.T("Asks you before every mail.")))
	}
	rows = append(rows, kit.AsideNote(th.Theme, i18n.Tf("Spends up to %s tokens a month, then stops and asks.", teamsui.Tokens(d.limit.Value))))
	for index, routine := range d.routines {
		when := d.sentences[routine.Schedule]
		if when == "" {
			when = routine.Schedule
		}
		note := i18n.Tf("%s: %s (off).", routine.Title, when)
		if index < len(d.routineOn) && d.routineOn[index].Value {
			note = i18n.Tf("%s: %s.", routine.Title, when)
		}
		rows = append(rows, kit.AsideNote(th.Theme, note))
	}
	return kit.Aside(gtx, th.Theme, i18n.T("What this makes"), rows...)
}
