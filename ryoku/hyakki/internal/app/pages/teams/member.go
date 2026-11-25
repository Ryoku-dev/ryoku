package teams

import (
	"fmt"
	"maps"
	"time"

	"gioui.org/layout"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/protocol"
)

const membersMax = protocol.TeamsMembersMax

// The dialog holds the exact reviewed member-list revision. Refresh never
// replaces it under the pointer, and only a successful atomic save closes it.
type memberDialog struct {
	dialog                             kit.Dialog
	scroll                             look.Scroller
	base, team                         protocol.Team
	adding, every, busy                bool
	title, section, problem            string
	form                               memberForm
	details                            memberDetails
	config                             memberConfigEditor
	sectionPick, managerPick, rootPick look.Select
	review                             kit.Toggle
	reviewedValue                      string
	save, cancel, reload               kit.Plaque
	generation                         uint64
}

func (d *memberDialog) open(team protocol.Team, member protocol.Member, adding bool) {
	d.generation++
	d.base, d.team = editableTeam(team), editableTeam(team)
	d.adding, d.every, d.busy, d.problem, d.section = adding, false, false, "", "identity"
	d.review.Value, d.reviewedValue = false, ""
	d.dialog.Open, d.dialog.Width = true, 680
	d.scroll.ScrollTo(0)
	if adding {
		member.ID = fmt.Sprintf("member-%d", time.Now().UnixNano())
		d.team.Members = append(d.team.Members, member)
	}
	d.form.set(member)
	d.details.set(member)
	d.config.reset(team.ID, member.ID)
	d.title = i18n.Tf("Edit %s", member.Name)
	if adding {
		d.title = i18n.Tf("Add a spirit to %s", team.Name)
	}
}

func (d *memberDialog) openEvery(team protocol.Team) {
	if len(team.Members) == 0 {
		return
	}
	d.open(team, team.Members[0], false)
	d.every = true
	d.title = i18n.Tf("Set every member's model in %s", team.Name)
	shared := team.Members[0]
	for _, member := range team.Members[1:] {
		if member.Model != shared.Model || !maps.Equal(member.Options, shared.Options) {
			shared = protocol.Member{}
			break
		}
	}
	d.form.engine.Set(shared.Model, shared.Options)
}

func (d *memberDialog) proposed() (protocol.Team, error) {
	t := editableTeam(d.team)
	if d.every {
		model, options := d.form.engine.Value()
		if !protocol.ValidEngineModel(model) || !protocol.ValidEngineOptions(options) {
			return t, fmt.Errorf("%s", i18n.T("Choose a valid model and options."))
		}
		for i := range t.Members {
			t.Members[i].Model, t.Members[i].Options = model, maps.Clone(options)
		}
		return t, nil
	}
	member := d.form.member()
	if member.Name == "" {
		return t, fmt.Errorf("%s", i18n.T("A spirit needs a name."))
	}
	if !protocol.ValidEngineModel(member.Model) || !protocol.ValidEngineOptions(member.Options) {
		return t, fmt.Errorf("%s", i18n.T("Choose a valid model and options."))
	}
	if err := d.details.apply(&member); err != nil {
		return t, err
	}
	found := false
	for i, current := range t.Members {
		if current.ID != member.ID && current.Name == member.Name {
			return t, fmt.Errorf("%s", i18n.T("Another spirit already has this name."))
		}
		if current.ID == member.ID {
			member.Lead, member.ReportsTo = current.Lead, current.ReportsTo
			t.Members[i] = member
			found = true
		}
	}
	if !found {
		return t, fmt.Errorf("%s", i18n.T("This spirit no longer exists in the draft. Reload the team deliberately."))
	}
	return t, protocol.ValidateReporting(t)
}

func (d *memberDialog) submit(e *env.Env, team protocol.Team) {
	d.busy, d.problem = true, ""
	generation, epoch := d.generation, e.Snapshot().Epoch
	patch := map[string]any{"members": team.Members, "reporting": team.Reporting}
	e.Call(protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: team.ID, ExpectedUpdated: d.base.Updated, Patch: rawJSON(patch)}, new(protocol.Team), func(err error) {
		if generation != d.generation {
			return
		}
		d.busy = false
		if err != nil {
			d.problem = i18n.Tf("Not saved: %s. This form and all edits remain open. Reload deliberately to resolve a stale revision.", err.Error())
			return
		}
		if epoch != e.Snapshot().Epoch {
			d.problem = i18n.T("The connection changed. Reload the saved configuration deliberately before continuing.")
			return
		}
		d.dialog.Open = false
	})
}

func (d *memberDialog) layout(gtx layout.Context, e *env.Env) {
	if d.team.ID == "" || !d.dialog.Open {
		return
	}
	th := e.Theme
	if d.busy || d.config.busy {
		gtx = gtx.Disabled()
	}
	proposed, invalid := d.proposed()
	changes := graphChanges(d.base, proposed)
	signature := changes
	if d.reviewedValue != signature {
		d.reviewedValue = signature
		d.review.Value = false
	}
	yamlDirty := d.config.current.Revision != 0 && d.config.editor.Text() != d.config.current.YAML
	ready := invalid == nil && (changes == "" || d.review.Value) && !yamlDirty
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	d.save.Kind, d.save.Label = kit.PlaqueGold, i18n.T("Save spirit")
	if d.every {
		d.save.Label = i18n.T("Set for every member")
	}
	if d.adding {
		d.save.Label = i18n.T("Add spirit")
	}
	if d.cancel.Clicked(gtx) && !d.busy && !d.config.busy {
		d.dialog.Dismiss(gtx)
	}
	if d.save.Clicked(gtx) && ready && !d.busy && !d.config.busy && d.section != "advanced" {
		d.submit(e, proposed)
	}
	rows := d.rows(e)
	if yamlDirty && d.section != "advanced" {
		rows = append(rows, quiet(th, i18n.T("Unsaved YAML is kept in Advanced. Save it or deliberately reload current YAML before saving and closing this form.")))
	}
	if current, ok := teamsui.TeamByID(e.Snapshot().Teams, d.team.ID); !ok || !current.Updated.Equal(d.base.Updated) {
		rows = append(rows, quiet(th, i18n.T("The saved team changed while this form was open. Your edits are kept; reload deliberately before saving a stale draft.")))
	}
	if changes != "" {
		rows = append(rows, quiet(th, changes), toggleField(e, &d.review, i18n.T("I reviewed the replacement root and affected reports")))
	}
	if invalid != nil && d.section != "advanced" {
		rows = append(rows, look.Paragraph(th, th.T.Detail, th.R.Jasper, invalid.Error()))
	}
	if d.problem != "" {
		rows = append(rows, look.Paragraph(th, th.T.Detail, th.R.Jasper, d.problem))
	}
	rows = append(rows, action(e, &d.reload, i18n.T("Reload current spirit (discard form and YAML edits)"), !d.busy && !d.config.busy, func() {
		current, ok := teamsui.TeamByID(e.Snapshot().Teams, d.team.ID)
		if !ok {
			d.problem = i18n.T("The team was removed. Your edits remain available here until you cancel.")
			return
		}
		if d.every {
			d.openEvery(current)
			return
		}
		if d.adding {
			d.open(current, protocol.Member{}, true)
			return
		}
		member, ok := protocol.MemberByID(current.Members, d.form.base.ID)
		if !ok {
			d.problem = i18n.T("This spirit was removed. It will not be recreated by saving. Cancel or add a new spirit explicitly.")
			return
		}
		d.open(current, member, false)
	}))
	d.dialog.Layout(gtx, th.Theme, d.title, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(560))
		gtx.Constraints.Min.Y = 0
		return d.scroll.Layout(gtx, th, len(rows), func(gtx layout.Context, index int) layout.Dimensions {
			return layout.Inset{Bottom: th.Sp.S3, Right: th.Sp.S3}.Layout(gtx, rows[index])
		})
	}, func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, th.Theme) }, func(gtx layout.Context) layout.Dimensions {
		if !ready || d.section == "advanced" {
			gtx = gtx.Disabled()
		}
		return d.save.Layout(gtx, th.Theme)
	})
	if d.dialog.Closed() {
		d.team = protocol.Team{}
		d.generation++
		d.config.generation++
	}
}

func (d *memberDialog) rows(e *env.Env) []layout.Widget {
	th := e.Theme
	if d.every {
		return []layout.Widget{quiet(th, i18n.Tf("All %d spirits change together; identities and instructions remain unchanged.", len(d.team.Members))), d.form.engineField(e)}
	}
	sections, names := []string{"identity", "reporting", "behavior", "model", "policy", "advanced"}, []string{i18n.T("Identity and lore"), i18n.T("Reporting and delegation"), i18n.T("Behavior and runtime"), i18n.T("Model and options"), i18n.T("Tools, budget and persona"), i18n.T("Advanced YAML and history")}
	rows := []layout.Widget{choose(e, &d.sectionPick, i18n.T("Spirit settings section"), sections, names, &d.section)}
	switch d.section {
	case "identity":
		rows = append(rows, d.form.nameField(th), d.form.titleField(th), d.form.identityFields(th), field(e, &d.details.seat, i18n.T("Seat identity")), field(e, &d.details.capabilities, i18n.T("Capabilities (description, not grants)")), quiet(th, i18n.T("Spirit identity is the editable lore reference. Rank and office describe a role, never tool, credential, filesystem or paid-model authority.")))
	case "reporting":
		rows = append(rows, rootField(e, &d.rootPick, &d.team))
		for i := range d.team.Members {
			if d.team.Members[i].ID == d.form.base.ID {
				rows = append(rows, managerField(e, &d.managerPick, d.team, &d.team.Members[i]))
			}
		}
		d.details.depth.Min, d.details.depth.Max, d.details.depth.Step, d.details.depth.Unit = 0, protocol.TeamsMembersMax, 1, i18n.T("levels (0 means direct reports)")
		rows = append(rows, quiet(th, i18n.Tf("Direct reports: %s", reportNames(d.team, d.form.base.ID))), labelled(th, i18n.T("Delegation depth"), func(gtx layout.Context) layout.Dimensions { return d.details.depth.Layout(gtx, th.Theme) }), quiet(th, i18n.T("Changing a root reassigns the former root to the replacement; review every affected reporting line before saving. Peer synthesizers are not managers.")))
	case "behavior":
		rows = append(rows, d.form.instructionsField(th))
		rows = append(rows, d.details.runtimeRows(e)...)
	case "model":
		rows = append(rows, d.form.engineField(e))
	case "policy":
		rows = append(rows, d.details.policyRows(e, d.team)...)
	case "advanced":
		if d.adding {
			rows = append(rows, quiet(th, i18n.T("Add this spirit first, then reopen Advanced for its canonical YAML and retained revisions.")))
		} else {
			rows = append(rows, d.config.rows(e)...)
		}
	}
	return rows
}

// Removal is a reviewed atomic graph edit, with an explicit surviving
// replacement for roots/managers. Nothing is respawned or promoted by order.
type dropMember struct {
	confirm                kit.Confirm
	team                   protocol.Team
	member                 protocol.Member
	replacement            string
	pick                   look.Select
	scroll                 look.Scroller
	review                 kit.Toggle
	save, cancel           kit.Plaque
	busy                   bool
	problem, reviewedValue string
}

func (d *dropMember) open(team protocol.Team, member protocol.Member) {
	d.team, d.member = editableTeam(team), member
	d.confirm.Dialog.Open, d.confirm.Dialog.Width = true, 620
	d.replacement, d.problem, d.reviewedValue, d.busy, d.review.Value = "", "", "", false, false
}
func (d *dropMember) removal() protocol.Team {
	t := editableTeam(d.team)
	t.Members = nil
	for _, m := range d.team.Members {
		if m.ID == d.member.ID {
			continue
		}
		if m.ReportsTo == d.member.ID {
			m.ReportsTo = d.replacement
			if m.ID == d.replacement {
				m.ReportsTo = d.member.ReportsTo
			}
		}
		t.Members = append(t.Members, m)
	}
	if d.member.Lead {
		replaceRoot(&t, d.replacement)
	}
	return t
}
func (d *dropMember) layout(gtx layout.Context, e *env.Env) {
	if d.team.ID == "" || !d.confirm.Dialog.Open {
		return
	}
	if d.busy {
		gtx = gtx.Disabled()
	}
	th := e.Theme
	t := d.removal()
	invalid := protocol.ValidateReporting(t)
	needsReplacement := d.member.Lead && t.Reporting == protocol.ReportingTree || len(protocol.DirectReports(d.team, d.member.ID)) > 0
	ids, names := []string{""}, []string{i18n.T("No replacement (peer synthesizer only)")}
	for _, m := range t.Members {
		ids = append(ids, m.ID)
		names = append(names, m.Name)
	}
	changes := graphChanges(d.team, t)
	if d.reviewedValue != changes {
		d.reviewedValue = changes
		d.review.Value = false
	}
	ready := invalid == nil && (!needsReplacement || d.replacement != "") && d.review.Value
	d.save.Label, d.save.Kind = i18n.T("Remove spirit and save reviewed graph"), kit.PlaqueGold
	d.cancel.Label, d.cancel.Kind = i18n.T("Cancel"), kit.PlaqueQuiet
	if d.cancel.Clicked(gtx) && !d.busy {
		d.confirm.Dialog.Dismiss(gtx)
	}
	if d.save.Clicked(gtx) && ready && !d.busy {
		d.busy, d.problem = true, ""
		patch := map[string]any{"members": t.Members, "reporting": t.Reporting}
		e.Call(protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: t.ID, ExpectedUpdated: d.team.Updated, Patch: rawJSON(patch)}, new(protocol.Team), func(err error) {
			d.busy = false
			if err != nil {
				d.problem = i18n.Tf("Not removed: %s. Cancel and reopen to review current revisions; this draft remains unchanged.", err.Error())
				return
			}
			d.confirm.Dialog.Open = false
		})
	}
	rows := []layout.Widget{quiet(th, i18n.Tf("%s leaves %s permanently. Its tasks stay in the record. Direct reports: %s", d.member.Name, d.team.Name, reportNames(d.team, d.member.ID)))}
	if needsReplacement || d.member.Lead {
		rows = append(rows, choose(e, &d.pick, i18n.T("Replacement root / manager for affected reports"), ids, names, &d.replacement))
	}
	rows = append(rows, quiet(th, changes), toggleField(e, &d.review, i18n.T("I reviewed the replacement and every affected report")))
	if invalid != nil {
		rows = append(rows, look.Paragraph(th, th.T.Detail, th.R.Jasper, invalid.Error()))
	}
	if d.problem != "" {
		rows = append(rows, look.Paragraph(th, th.T.Detail, th.R.Jasper, d.problem))
	}
	d.confirm.Dialog.Layout(gtx, th.Theme, i18n.Tf("Remove %s?", d.member.Name), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(480))
		gtx.Constraints.Min.Y = 0
		return d.scroll.Layout(gtx, th, len(rows), func(gtx layout.Context, index int) layout.Dimensions {
			return layout.Inset{Bottom: th.Sp.S3}.Layout(gtx, rows[index])
		})
	}, func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, th.Theme) }, func(gtx layout.Context) layout.Dimensions {
		if !ready {
			gtx = gtx.Disabled()
		}
		return d.save.Layout(gtx, th.Theme)
	})
	if d.confirm.Dialog.Closed() {
		d.team = protocol.Team{}
	}
}

func (p *Page) editMember(team protocol.Team, member protocol.Member) {
	p.member.open(team, member, false)
}
func (p *Page) addMember(team protocol.Team)                            { p.member.open(team, protocol.Member{}, true) }
func (p *Page) removeMember(team protocol.Team, member protocol.Member) { p.drop.open(team, member) }
func (p *Page) setEveryEngine(team protocol.Team)                       { p.member.openEvery(team) }
