package teams

import (
	"maps"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/protocol"
)

// a team has at most this many members, its lead included
const membersMax = 6

// edits one member of a team, or adds one, or sets every member's engine at once; saving sends the whole crew
type memberDialog struct {
	dialog kit.Dialog
	scroll look.Scroller
	team   protocol.Team
	adding bool
	every  bool
	title  string
	form   memberForm
	save   kit.Plaque
	cancel kit.Plaque
}

func (d *memberDialog) open(team protocol.Team, member protocol.Member, adding bool) {
	d.team, d.adding, d.every = team, adding, false
	d.dialog.Open, d.dialog.Width = true, 600
	d.form.set(member)
	d.title = i18n.Tf("Edit %s", member.Name)
	if adding {
		d.title = i18n.Tf("Add a member to %s", team.Name)
	}
}

// a crew that shares one engine starts from it, a mixed one from the teams default
func (d *memberDialog) openEvery(team protocol.Team) {
	d.team, d.adding, d.every = team, false, true
	d.dialog.Open, d.dialog.Width = true, 600
	d.title = i18n.Tf("Set every member's engine in %s", team.Name)
	shared := team.Members[0]
	for _, member := range team.Members[1:] {
		if member.Harness != shared.Harness || member.Model != shared.Model || !maps.Equal(member.Options, shared.Options) {
			shared = protocol.Member{}
			break
		}
	}
	d.form.engine.Set(shared.Harness, shared.Model, shared.Options)
}

// another member already has the name, and names tell members apart
func (d *memberDialog) taken(name string) bool {
	for _, member := range d.team.Members {
		if member.Name == name && (d.adding || member.ID != d.form.id) {
			return true
		}
	}
	return false
}

func (d *memberDialog) layout(gtx layout.Context, e *env.Env) {
	if d.team.ID == "" {
		return
	}
	th := e.Theme
	member := d.form.member()
	d.form.name.Error = ""
	if !d.every && d.taken(member.Name) {
		d.form.name.Error = i18n.T("Another member of the team has this name.")
	}
	ready := d.every || member.Name != "" && d.form.name.Error == ""
	ready = ready && protocol.ValidEngineModel(member.Model)
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	d.save.Kind, d.save.Label = kit.PlaqueGold, i18n.T("Save")
	switch {
	case d.every:
		d.save.Label = i18n.T("Set for every member")
	case d.adding:
		d.save.Label = i18n.T("Add member")
	}
	if d.cancel.Clicked(gtx) {
		d.dialog.Dismiss(gtx)
	}
	if d.save.Clicked(gtx) && ready {
		if d.every {
			d.submitEvery(e)
		} else {
			d.submit(e, member)
		}
		d.dialog.Dismiss(gtx)
	}
	rows := d.rows(e)
	d.dialog.Layout(gtx, th.Theme, d.title,
		func(gtx layout.Context) layout.Dimensions {
			// the body scrolls so the dialog fits a short window
			gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(520))
			gtx.Constraints.Min.Y = 0
			return d.scroll.Layout(gtx, th, len(rows), func(gtx layout.Context, index int) layout.Dimensions {
				return layout.Inset{Bottom: th.Sp.S3, Right: th.Sp.S3}.Layout(gtx, rows[index])
			})
		},
		func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions {
			if !ready {
				gtx = gtx.Disabled()
			}
			return d.save.Layout(gtx, th.Theme)
		},
	)
	if d.dialog.Closed() {
		d.team = protocol.Team{}
	}
}

// who the member is first, then the engine it runs on as a group of its own
func (d *memberDialog) rows(e *env.Env) []layout.Widget {
	th := e.Theme
	engine := []layout.Widget{
		kit.Text(th.Theme, th.T.Section, th.R.Fg, i18n.T("Engine")),
		d.form.engineField(e),
	}
	if d.every {
		note := quiet(th, i18n.Tf("All %d members run on this. Their names, titles and instructions stay.", len(d.team.Members)))
		return append([]layout.Widget{note}, engine[1:]...)
	}
	d.form.lead.Label = i18n.T("Leads the team")
	rows := []layout.Widget{
		d.form.nameField(th),
		d.form.titleField(th),
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.form.lead.Layout(gtx, th.Theme) }),
				layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
				layout.Rigid(quiet(th, i18n.T("The lead takes the team's tasks and hands pieces to the others. A team has one lead."))),
			)
		},
		d.form.instructionsField(th),
	}
	return append(rows, engine...)
}

// the crew is read again when saving, so a change made meanwhile elsewhere is kept
func (d *memberDialog) submit(e *env.Env, member protocol.Member) {
	team, ok := teamsui.TeamByID(e.Snapshot().Teams, d.team.ID)
	if !ok {
		return
	}
	members := append([]protocol.Member(nil), team.Members...)
	at := -1
	for index, candidate := range members {
		if !d.adding && candidate.ID == member.ID {
			at = index
		}
	}
	if at < 0 {
		members = append(members, member)
		at = len(members) - 1
	} else {
		members[at] = member
	}
	if member.Lead {
		for index := range members {
			members[index].Lead = index == at
		}
	}
	updateTeam(e, team.ID, map[string]any{"members": members}, func(error) {})
}

// one update carries the whole crew, so every member changes together or none does
func (d *memberDialog) submitEvery(e *env.Env) {
	team, ok := teamsui.TeamByID(e.Snapshot().Teams, d.team.ID)
	if !ok {
		return
	}
	harness, model, options := d.form.engine.Value()
	members := append([]protocol.Member(nil), team.Members...)
	for index := range members {
		members[index].Harness, members[index].Model, members[index].Options = harness, model, maps.Clone(options)
	}
	updateTeam(e, team.ID, map[string]any{"members": members}, func(error) {})
}

// removing a member asks first: its tasks stay, the member does not come back
type dropMember struct {
	confirm kit.Confirm
	team    protocol.Team
	member  protocol.Member
}

func (d *dropMember) open(team protocol.Team, member protocol.Member) {
	d.team, d.member = team, member
	d.confirm.Danger, d.confirm.Dialog.Open = true, true
}

func (d *dropMember) layout(gtx layout.Context, e *env.Env) {
	if d.team.ID == "" {
		return
	}
	title := i18n.Tf("Remove %s?", d.member.Name)
	body := i18n.Tf("%s leaves %s. The tasks it worked on stay in the record.", d.member.Name, d.team.Name)
	if d.member.Lead {
		body = i18n.Tf("%s leads %s. The next member becomes the lead. The tasks it worked on stay in the record.", d.member.Name, d.team.Name)
	}
	if d.confirm.Layout(gtx, e.Theme.Theme, title, body, i18n.T("Remove"), "") {
		if team, ok := teamsui.TeamByID(e.Snapshot().Teams, d.team.ID); ok {
			members := make([]protocol.Member, 0, len(team.Members))
			for _, member := range team.Members {
				if member.ID != d.member.ID {
					members = append(members, member)
				}
			}
			updateTeam(e, team.ID, map[string]any{"members": members}, func(error) {})
		}
	}
	if d.confirm.Closed() {
		d.team = protocol.Team{}
	}
}

// the Settings rows of the crew open these with the member they stand for
func (p *Page) editMember(team protocol.Team, member protocol.Member) {
	p.member.open(team, member, false)
}

func (p *Page) addMember(team protocol.Team) {
	p.member.open(team, protocol.Member{}, true)
}

func (p *Page) removeMember(team protocol.Team, member protocol.Member) {
	p.drop.open(team, member)
}

func (p *Page) setEveryEngine(team protocol.Team) {
	p.member.openEvery(team)
}
