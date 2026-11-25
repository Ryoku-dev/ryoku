package teams

import (
	"fmt"
	"strings"

	"gioui.org/layout"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// Catalogue refreshes never apply a recipe to an edited roster.
type structureCatalogue struct {
	templates, deleted []protocol.StructureTemplate
	spirits            []protocol.StructureSpirit
	epoch              int
	revision           uint64
	asked, busy        bool
	request            uint64
	problem            string
	retry              kit.Plaque
	restore            map[string]*kit.Plaque
}

func (c *structureCatalogue) load(e *env.Env) {
	s := e.Snapshot()
	if s.Connection != state.Ready {
		return
	}
	if c.asked && c.epoch == s.Epoch && c.revision == s.StructuresRevision {
		return
	}
	c.asked, c.busy, c.epoch, c.revision = true, true, s.Epoch, s.StructuresRevision
	c.request++
	request, epoch, revision := c.request, c.epoch, c.revision
	var result protocol.TeamStructuresResult
	e.Call(protocol.MTeamsStructures, protocol.Empty{}, &result, func(err error) {
		now := e.Snapshot()
		if request != c.request || epoch != now.Epoch || revision != now.StructuresRevision {
			return
		}
		c.busy, c.problem = false, ""
		if err != nil {
			c.problem = err.Error()
			return
		}
		c.templates, c.deleted, c.spirits = result.Templates, result.Deleted, result.Spirits
	})
}

func (c *structureCatalogue) byID(id string) (protocol.StructureTemplate, bool) {
	for _, t := range c.templates {
		if t.ID == id {
			return t, true
		}
	}
	return protocol.StructureTemplate{}, false
}

func (c *structureCatalogue) refresh() { c.asked = false; c.request++; c.busy = false }

func uiStack(e *env.Env, rows ...layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, len(rows)*2)
		for i, row := range rows {
			if i > 0 {
				children = append(children, layout.Rigid(layout.Spacer{Height: e.Theme.Sp.S2}.Layout))
			}
			children = append(children, layout.Rigid(row))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}

func action(e *env.Env, button *kit.Plaque, label string, enabled bool, act func()) layout.Widget {
	button.Label, button.Small, button.Kind = label, true, kit.PlaqueTonal
	return func(gtx layout.Context) layout.Dimensions {
		if button.Clicked(gtx) && enabled {
			act()
		}
		if !enabled {
			gtx = gtx.Disabled()
		}
		return button.Layout(gtx, e.Theme.Theme)
	}
}

func choose(e *env.Env, control *look.Select, label string, values, labels []string, value *string) layout.Widget {
	control.Label = label
	return func(gtx layout.Context) layout.Dimensions {
		selected := -1
		for i, v := range values {
			if v == *value {
				selected = i
			}
		}
		dimensions := control.Layout(gtx, e.Theme, e.Overlays, labels, selected)
		if i, changed := control.Changed(); changed && i >= 0 && i < len(values) {
			*value = values[i]
		}
		return dimensions
	}
}

func (d *draft) chooseStructure(e *env.Env, t protocol.StructureTemplate, count int) {
	d.rosterRequest++
	request, epoch := d.rosterRequest, e.Snapshot().Epoch
	d.rosterBusy, d.problem = true, ""
	var result protocol.StructureDraftResult
	e.Call(protocol.MTeamsStructureDraft, protocol.StructureDraftParams{ID: t.ID, Count: count}, &result, func(err error) {
		if request != d.rosterRequest {
			return
		}
		d.rosterBusy = false
		if epoch != e.Snapshot().Epoch {
			d.problem = i18n.T("Connection changed while loading the roster. Select the recipe again.")
			d.filled = true
			return
		}
		if err != nil {
			d.problem = err.Error()
			d.filled = true
			return
		}
		d.applyStructure(e, result.Template, result.Members)
	})
}

func (d *draft) applyStructure(e *env.Env, t protocol.StructureTemplate, members []protocol.Member) {
	seedPolicy := !d.policyReady
	d.rosterRequest++
	d.structure, d.template, d.wanted, d.reporting = t, t.ID, t.ID, t.Reporting
	d.filled, d.problem, d.rosterBusy, d.legacy.Value = true, "", false, false
	d.count.Value = len(members)
	d.members = nil
	for _, member := range members {
		row := new(draftMember)
		row.form.set(member)
		row.expanded = len(members) <= 3
		d.members = append(d.members, row)
	}
	if strings.TrimSpace(d.name.Text()) == "" {
		d.name.SetText(t.Name)
	}
	if seedPolicy {
		defaults := e.Snapshot().Settings.Teams.Defaults
		d.screen.Value, d.mail.Value = false, false
		d.approvalsValue, d.notifyValue = defaults.Approvals, defaults.Notify
		if !protocol.KnownApprovals(d.approvalsValue) {
			d.approvalsValue = protocol.ApprovalsAsk
		}
		d.limit.Value, d.warn = defaults.Budget.MonthTokens, defaults.Budget.WarnPercent
		if d.limit.Value <= 0 {
			d.limit.Value = 100000
		}
		if d.warn <= 0 {
			d.warn = 80
		}
		d.turns.Value, d.minutes.Value = max(1, defaults.Budget.TaskTurns), defaults.Budget.TaskMinutes
		d.planApproval.Value = true
		d.policyReady = true
	}
	d.routines, d.routineOn = nil, nil
	d.reviewed.Value = false
}

func (d *draft) layoutCatalogue(gtx layout.Context, e *env.Env, p *Page) layout.Dimensions {
	th, c := e.Theme, &d.catalog
	rows := []layout.Widget{quiet(th, i18n.T("Choose a structure or get local advice. Changing a recipe replaces the roster only when you explicitly select it or press Replace roster."))}
	if c.problem != "" {
		rows = append(rows, look.Paragraph(th, th.T.Body, th.R.Jasper, c.problem))
	}
	if d.problem != "" && !d.filled {
		rows = append(rows, quiet(th, d.problem))
	}
	rows = append(rows, action(e, &c.retry, i18n.T("Reload structures"), !c.busy, func() { c.refresh(); c.load(e) }))
	if e.Snapshot().Connection != state.Ready {
		rows = append(rows, quiet(th, i18n.T("Connect to oni to load local structures. Your draft stays here.")))
	}
	if c.busy {
		rows = append(rows, func(gtx layout.Context) layout.Dimensions { return look.SkeletonRows(gtx, th, 2, true) })
	}
	labels, ids := []string{}, []string{}
	for _, t := range c.templates {
		if !t.Hidden {
			ids = append(ids, t.ID)
			labels = append(labels, t.Name)
		}
	}
	if len(ids) == 0 && !c.busy && c.problem == "" {
		rows = append(rows, quiet(th, i18n.T("No visible structures. Restore a hidden recipe below or create your own.")))
	}
	d.structurePick.Label, d.structurePick.Searched = i18n.T("Structure recipe"), true
	rows = append(rows, func(gtx layout.Context) layout.Dimensions {
		selected := -1
		for i, id := range ids {
			if id == d.structure.ID {
				selected = i
			}
		}
		if d.rosterBusy {
			gtx = gtx.Disabled()
		}
		dimensions := d.structurePick.Layout(gtx, th, e.Overlays, labels, selected)
		if i, changed := d.structurePick.Changed(); changed && i >= 0 && i < len(ids) {
			if t, ok := c.byID(ids[i]); ok {
				d.chooseStructure(e, t, t.DefaultSeats)
			}
		}
		return dimensions
	})
	if d.rosterBusy {
		rows = append(rows, quiet(th, i18n.T("Loading proposed roster…")))
	}
	if t := d.structure; t.ID != "" {
		rows = append(rows, quiet(th, t.Description), quiet(th, i18n.Tf("Culture: %s. Shape: %s. Seats: %d–%d, including any marshal or synthesizer.", t.Culture, t.Shape, t.MinSeats, t.MaxSeats)))
		d.count.Min, d.count.Max, d.count.Step, d.count.Unit = t.MinSeats, t.MaxSeats, 1, i18n.T("seats")
		rows = append(rows, labelled(th, i18n.T("Recipe seat count"), func(gtx layout.Context) layout.Dimensions { return d.count.Layout(gtx, th.Theme) }), action(e, &d.replace, i18n.T("Replace roster"), !d.rosterBusy && !d.busy, func() { d.chooseStructure(e, t, d.count.Value) }))
		label := i18n.T("Inspect worked example and sources")
		if d.exampleOpen {
			label = i18n.T("Close worked example and sources")
		}
		rows = append(rows, action(e, &d.example, label, true, func() { d.exampleOpen = !d.exampleOpen }))
		if d.exampleOpen {
			rows = append(rows, d.exampleContent(e, t))
		}
		rows = append(rows, action(e, &d.cloneRecipe, i18n.T("Clone and edit recipe"), !d.recipe.busy, func() { d.recipe.cloneFrom(t) }))
		if !t.Builtin {
			rows = append(rows, action(e, &d.editRecipe, i18n.T("Edit saved recipe"), !d.recipe.busy, func() {
				if current, ok := c.byID(t.ID); ok {
					d.recipe.open(current)
				}
			}))
		}
		rows = append(rows, action(e, &d.hideRecipe, i18n.T("Hide selected recipe"), !c.busy, func() {
			current, ok := c.byID(t.ID)
			if !ok {
				return
			}
			c.busy = true
			if current.Builtin {
				e.Call(protocol.MTeamsStructureDelete, protocol.StructureRef{ID: current.ID, ExpectedRevision: current.Revision}, new(protocol.Empty), func(err error) {
					c.busy = false
					if err != nil {
						c.problem = err.Error()
						return
					}
					c.refresh()
				})
			} else {
				current.Hidden = true
				e.Call(protocol.MTeamsStructureSave, protocol.StructureSaveParams{Template: current, ExpectedRevision: current.Revision}, new(protocol.StructureTemplate), func(err error) {
					c.busy = false
					if err != nil {
						c.problem = err.Error()
						return
					}
					c.refresh()
				})
			}
		}))
		if !t.Builtin {
			label := i18n.T("Delete selected recipe")
			if d.deleteArmed == t.ID {
				label = i18n.T("Confirm delete recipe (teams are kept)")
			}
			rows = append(rows, action(e, &d.deleteRecipe, label, !c.busy, func() {
				if d.deleteArmed != t.ID {
					d.deleteArmed = t.ID
					return
				}
				current, ok := c.byID(t.ID)
				if !ok {
					return
				}
				c.busy = true
				e.Call(protocol.MTeamsStructureDelete, protocol.StructureRef{ID: current.ID, ExpectedRevision: current.Revision}, new(protocol.Empty), func(err error) {
					c.busy = false
					if err != nil {
						c.problem = err.Error()
						return
					}
					d.deleteArmed = ""
					c.refresh()
				})
			}))
		}
	}
	rows = append(rows, action(e, &d.newRecipe, i18n.T("Create custom recipe"), !d.recipe.busy, func() { d.recipe.create() }), func(gtx layout.Context) layout.Dimensions { return d.advice.layout(gtx, e, d) })
	if d.recipe.active {
		rows = append(rows, func(gtx layout.Context) layout.Dimensions { return d.recipe.layout(gtx, e, c) })
	}
	rows = append(rows, func(gtx layout.Context) layout.Dimensions { return c.management(gtx, e) })
	d.legacy.Label = i18n.T("Show optional workflow presets")
	rows = append(rows, func(gtx layout.Context) layout.Dimensions { return d.legacy.Layout(gtx, th.Theme) })
	if d.legacy.Value {
		rows = append(rows, quiet(th, i18n.T("Workflow presets configure tools and routines, separately from structure recipes.")), func(gtx layout.Context) layout.Dimensions {
			return d.starts.layoutPick(gtx, e, p.templates.List(e), d.template, func(t protocol.TeamTemplate) { d.apply(e, t) })
		})
	}
	return uiStack(e, rows...)(gtx)
}

func (d *draft) exampleContent(e *env.Env, t protocol.StructureTemplate) layout.Widget {
	th, ex := e.Theme, t.Example
	if !t.Builtin {
		if ex.Title == "" {
			return uiStack(e, quiet(th, i18n.T("This custom recipe has no worked example. Review its editable seat definitions before summoning.")), quiet(th, i18n.Tf("Source: %s", t.Source)))
		}
		ex.Title = i18n.Tf("Original source example (not the edited custom roster): %s", ex.Title)
	}
	rows := []layout.Widget{kit.Text(th.Theme, th.T.Row, th.R.Fg, i18n.Tf("Worked example: %s", ex.Title)), quiet(th, i18n.T("Illustrative workflow, not a completed run.")), quiet(th, ex.Goal)}
	names := make(map[string]string)
	for _, m := range ex.Members {
		names[m.ID] = m.Name
		rows = append(rows, quiet(th, i18n.Tf("%s — %s; %s; reports to %s", m.Name, m.Rank, m.Instructions, rosterManagerName(ex.Members, m.ReportsTo))))
	}
	for _, step := range ex.Steps {
		who := make([]string, len(step.Members))
		for i, id := range step.Members {
			who[i] = names[id]
			if who[i] == "" {
				who[i] = id
			}
		}
		rows = append(rows, quiet(th, i18n.Tf("%s: %s", strings.Join(who, ", "), step.Action)))
	}
	rows = append(rows, quiet(th, i18n.Tf("Intended result: %s", ex.Result)), quiet(th, i18n.Tf("Guardrail: %s", ex.Guardrail)), quiet(th, i18n.Tf("Source: %s", t.Source)))
	for _, note := range t.Notes {
		rows = append(rows, quiet(th, note))
	}
	seen := make(map[string]bool)
	for _, seat := range t.Seats {
		rows = append(rows, quiet(th, i18n.Tf("Application seat mapping: %s / %s — %s. Suggested model class: %s (advice only).", seat.Title, seat.Rank, seat.Duty, seat.SuggestedModelClass)))
		for _, spirit := range d.catalog.spirits {
			if spirit.ID == seat.DefaultSpirit && !seen[spirit.ID] {
				seen[spirit.ID] = true
				office := spirit.Office
				if office == "" {
					office = i18n.T("Not supplied by the preserved source")
				}
				rows = append(rows, quiet(th, i18n.Tf("Source identity: %s. Canonical ranks: %s. Historical office: %s", spirit.Name, strings.Join(spirit.Ranks, ", "), office)), quiet(th, i18n.Tf("Application role mapping: %s", spirit.RoleMapping)), quiet(th, i18n.Tf("Source: %s", spirit.Source)))
				if spirit.Notes != "" {
					rows = append(rows, quiet(th, spirit.Notes))
				}
			}
		}
	}
	return uiStack(e, rows...)
}

func rosterManagerName(members []protocol.Member, id string) string {
	if id == "" {
		return i18n.T("Summoner")
	}
	for _, m := range members {
		if m.ID == id {
			return m.Name
		}
	}
	return i18n.Tf("Missing manager (%s)", id)
}

func (d *draft) graphProblem(members []protocol.Member) string {
	if d.structure.ID == "" || len(members) == 0 {
		return ""
	}
	if err := protocol.ValidateReporting(protocol.Team{Reporting: d.reporting, Members: members}); err != nil {
		return err.Error()
	}
	return ""
}

func (d *draft) reportingField(e *env.Env, row *draftMember) layout.Widget {
	if d.structure.ID == "" {
		return quiet(e.Theme, i18n.T("Workflow preset: the first seat leads."))
	}
	labels, ids := []string{i18n.T("Summoner (no manager)")}, []string{""}
	if d.reporting != protocol.ReportingPeers {
		for _, other := range d.members {
			m := other.form.member()
			if m.ID != row.form.base.ID {
				labels = append(labels, m.Name)
				ids = append(ids, m.ID)
			}
		}
	}
	if row.form.base.ReportsTo != "" {
		found := false
		for _, id := range ids {
			found = found || id == row.form.base.ReportsTo
		}
		if !found {
			ids = append(ids, row.form.base.ReportsTo)
			label := i18n.T("Removed manager — choose another")
			if d.reporting == protocol.ReportingPeers {
				label = i18n.T("Peers report to the Summoner — choose no manager")
			}
			labels = append(labels, label)
		}
	}
	row.form.lead.Label = i18n.T("Lead / optional peer synthesizer")
	return uiStack(e, choose(e, &row.manager, i18n.T("Reports to"), ids, labels, &row.form.base.ReportsTo), func(gtx layout.Context) layout.Dimensions { return row.form.lead.Layout(gtx, e.Theme.Theme) })
}

func (d *draft) layoutRosterRow(e *env.Env, row *draftMember) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		m := row.form.member()
		manager := i18n.T("Summoner")
		if m.ReportsTo != "" {
			manager = i18n.T("Removed manager — choose another")
			for _, other := range d.members {
				if other.form.base.ID == m.ReportsTo {
					manager = other.form.name.Text()
					break
				}
			}
		}
		label := i18n.Tf("Edit seat: %s · %s · reports to %s", m.Name, m.Rank, manager)
		if row.expanded {
			label = i18n.Tf("Close seat: %s", m.Name)
		}
		rows := []layout.Widget{action(e, &row.details, label, true, func() { row.expanded = !row.expanded })}
		if row.expanded {
			rows = append(rows, d.layoutMember(e, row, true))
		}
		return uiStack(e, rows...)(gtx)
	}
}

func (c *structureCatalogue) management(gtx layout.Context, e *env.Env) layout.Dimensions {
	if c.restore == nil {
		c.restore = make(map[string]*kit.Plaque)
	}
	rows := []layout.Widget{}
	entries := append([]protocol.StructureTemplate(nil), c.deleted...)
	for _, t := range c.templates {
		if t.Hidden {
			entries = append(entries, t)
		}
	}
	for _, t := range entries {
		button := c.restore[t.ID]
		if button == nil {
			button = new(kit.Plaque)
			c.restore[t.ID] = button
		}
		rows = append(rows, action(e, button, i18n.Tf("Restore recipe: %s", t.Name), !c.busy, func() {
			c.busy = true
			var result protocol.StructureTemplate
			e.Call(protocol.MTeamsStructureRestore, protocol.StructureRef{ID: t.ID, ExpectedRevision: t.Revision}, &result, func(err error) {
				c.busy = false
				if err != nil {
					c.problem = err.Error()
					return
				}
				c.refresh()
			})
		}))
	}
	return uiStack(e, rows...)(gtx)
}

// Stable IDs belong to seats, never to editable names.
func seatID(n int) string { return fmt.Sprintf("seat-%03d", n) }
