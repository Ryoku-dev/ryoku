package teams

import (
	"fmt"
	"strings"

	"gioui.org/layout"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type recipeEditor struct {
	active, busy                      bool
	base                              protocol.StructureTemplate
	cloneSource                       string
	problem                           string
	id, name, culture, source, jobFit kit.TextField
	description                       look.MultiLine
	shape, reporting                  look.Select
	shapeValue, reportingValue        string
	minimum, maximum, count           kit.Stepper
	variable                          kit.Toggle
	seats                             []*recipeSeat
	next                              int
	save, cancel, add, reload, copy   kit.Plaque
}

type recipeSeat struct {
	base                              protocol.StructureSeat
	title, rank, spirit, capabilities kit.TextField
	duty                              look.MultiLine
	model, manager                    look.Select
	modelValue, managerValue          string
	lead                              kit.Toggle
	remove, details                   kit.Plaque
	expanded                          bool
}

func (r *recipeEditor) create() {
	r.open(protocol.StructureTemplate{Shape: "flat", Reporting: protocol.ReportingPeers, MinSeats: 1, MaxSeats: 1, DefaultSeats: 1, Seats: []protocol.StructureSeat{{ID: seatID(1), Title: i18n.T("Specialist"), Rank: i18n.T("Peer")}}})
}

func (r *recipeEditor) cloneFrom(t protocol.StructureTemplate) {
	r.open(t)
	r.cloneSource = t.ID
	r.id.SetText("")
	r.name.SetText(i18n.Tf("%s copy", t.Name))
}

func (r *recipeEditor) open(t protocol.StructureTemplate) {
	r.active, r.busy, r.problem, r.cloneSource = true, false, "", ""
	r.base = t
	r.id.SetText(strings.TrimPrefix(t.ID, "custom:"))
	r.name.SetText(t.Name)
	r.culture.SetText(t.Culture)
	r.description.SetText(t.Description)
	r.source.SetText(t.Source)
	r.jobFit.SetText(strings.Join(t.JobFit, ", "))
	r.shapeValue, r.reportingValue = t.Shape, t.Reporting
	r.minimum.Value, r.maximum.Value, r.count.Value = t.MinSeats, t.MaxSeats, t.DefaultSeats
	r.variable.Value = t.Variable
	r.seats = nil
	r.next = len(t.Seats)
	for _, s := range t.Seats {
		r.seats = append(r.seats, newRecipeSeat(s, len(t.Seats) <= 3))
	}
}

func newRecipeSeat(s protocol.StructureSeat, expanded bool) *recipeSeat {
	row := &recipeSeat{base: s, modelValue: s.SuggestedModelClass, managerValue: s.ReportsToSeat, expanded: expanded}
	row.title.SetText(s.Title)
	row.rank.SetText(s.Rank)
	row.spirit.SetText(s.DefaultSpirit)
	row.capabilities.SetText(s.CapabilitiesHint)
	row.duty.SetText(s.Duty)
	row.lead.Value = s.Lead
	return row
}

func (s *recipeSeat) value() protocol.StructureSeat {
	seat := s.base
	seat.Title, seat.Rank, seat.DefaultSpirit = strings.TrimSpace(s.title.Text()), strings.TrimSpace(s.rank.Text()), strings.TrimSpace(s.spirit.Text())
	seat.Duty, seat.CapabilitiesHint = strings.TrimSpace(s.duty.Text()), strings.TrimSpace(s.capabilities.Text())
	seat.SuggestedModelClass, seat.ReportsToSeat, seat.Lead = s.modelValue, s.managerValue, s.lead.Value
	return seat
}

func (r *recipeEditor) value() protocol.StructureTemplate {
	t := r.base
	if t.ID == "" {
		t.ID = "custom:" + strings.TrimSpace(r.id.Text())
	}
	t.Name, t.Culture, t.Description = strings.TrimSpace(r.name.Text()), strings.TrimSpace(r.culture.Text()), strings.TrimSpace(r.description.Text())
	t.Shape, t.Reporting, t.Source = r.shapeValue, r.reportingValue, strings.TrimSpace(r.source.Text())
	t.MinSeats, t.MaxSeats, t.DefaultSeats, t.Variable = r.minimum.Value, r.maximum.Value, r.count.Value, r.variable.Value
	t.Builtin = false
	t.JobFit = nil
	for _, fit := range strings.Split(r.jobFit.Text(), ",") {
		if fit = strings.TrimSpace(fit); fit != "" {
			t.JobFit = append(t.JobFit, fit)
		}
	}
	t.Seats = make([]protocol.StructureSeat, len(r.seats))
	for i, s := range r.seats {
		t.Seats[i] = s.value()
	}
	// A changed custom recipe must not claim that an inherited example is its current roster.
	t.Example = protocol.StructureExample{}
	return t
}

func field(e *env.Env, f *kit.TextField, label string) layout.Widget {
	f.Label = label
	return func(gtx layout.Context) layout.Dimensions { return f.Layout(gtx, e.Theme.Theme) }
}

func (r *recipeEditor) saveValue(e *env.Env, c *structureCatalogue) {
	r.busy, r.problem = true, ""
	var result protocol.StructureTemplate
	if r.cloneSource != "" {
		e.Call(protocol.MTeamsStructureClone, protocol.StructureCloneParams{ID: r.cloneSource, NewID: "custom:" + strings.TrimSpace(r.id.Text()), Name: strings.TrimSpace(r.name.Text())}, &result, func(err error) {
			r.busy = false
			if err != nil {
				r.problem = err.Error()
				return
			}
			r.open(result)
			c.refresh()
		})
		return
	}
	t := r.value()
	e.Call(protocol.MTeamsStructureSave, protocol.StructureSaveParams{Template: t, ExpectedRevision: r.base.Revision}, &result, func(err error) {
		r.busy = false
		if err != nil {
			r.problem = i18n.Tf("Recipe was not saved: %s. Your edits are kept. Reload structures, then reopen the latest recipe to resolve a stale edit, or save your edits as a new recipe.", err.Error())
			return
		}
		r.open(result)
		c.refresh()
	})
}

func (r *recipeEditor) layout(gtx layout.Context, e *env.Env, c *structureCatalogue) layout.Dimensions {
	th := e.Theme
	if r.busy {
		gtx = gtx.Disabled()
	}
	rows := []layout.Widget{kit.Text(th.Theme, th.T.Section, th.R.Fg, i18n.T("Custom structure recipe")), quiet(th, i18n.T("A reusable org chart, not a permission grant. Saving a recipe never changes a team or your reviewed draft."))}
	rows = append(rows, field(e, &r.name, i18n.T("Recipe name")))
	if r.base.ID == "" || r.cloneSource != "" {
		r.id.Help = i18n.T("Unique lowercase letters, digits and hyphens. Deleted IDs must be restored, not reused.")
		rows = append(rows, field(e, &r.id, i18n.T("Custom recipe ID")))
	}
	if r.cloneSource != "" {
		rows = append(rows, quiet(th, i18n.T("Clone saves a separate local recipe. Its seat forms open next; the original stays unchanged.")), action(e, &r.save, i18n.T("Clone recipe"), strings.TrimSpace(r.id.Text()) != "" && strings.TrimSpace(r.name.Text()) != "", func() { r.saveValue(e, c) }))
	} else {
		rows = append(rows, field(e, &r.culture, i18n.T("Recipe culture")), field(e, &r.source, i18n.T("Recipe source / provenance")), field(e, &r.jobFit, i18n.T("Suitable work (comma-separated keys)")))
		r.jobFit.Help = i18n.T("For example: build, research, watch, decide, triage, create, verify, harden.")
		r.description.Label, r.description.Lines = i18n.T("Recipe description"), 3
		rows = append(rows, func(gtx layout.Context) layout.Dimensions { return r.description.Layout(gtx, th) }, choose(e, &r.shape, i18n.T("Recipe shape"), []string{"deep", "wide", "flat", "court", "ring", "horde"}, []string{i18n.T("Deep"), i18n.T("Wide"), i18n.T("Flat"), i18n.T("Court"), i18n.T("Ring"), i18n.T("Horde")}, &r.shapeValue), choose(e, &r.reporting, i18n.T("Recipe reporting"), []string{protocol.ReportingTree, protocol.ReportingPeers}, []string{i18n.T("Tree: one reachable lead"), i18n.T("Peers: Summoner or optional synthesizer")}, &r.reportingValue))
		r.variable.Label = i18n.T("Repeat worker prototypes after the first marshal")
		rows = append(rows, func(gtx layout.Context) layout.Dimensions { return r.variable.Layout(gtx, th.Theme) }, quiet(th, i18n.T("Variable processions require a first marshal and worker prototypes reporting to it. Fixed recipes need one seat per maximum count; each selectable prefix must contain its managers.")))
		for _, s := range []*kit.Stepper{&r.minimum, &r.maximum, &r.count} {
			s.Min, s.Max, s.Step, s.Unit = 1, protocol.TeamsMembersMax, 1, i18n.T("seats")
		}
		rows = append(rows, labelled(th, i18n.T("Minimum recipe seats"), func(gtx layout.Context) layout.Dimensions { return r.minimum.Layout(gtx, th.Theme) }), labelled(th, i18n.T("Maximum recipe seats"), func(gtx layout.Context) layout.Dimensions { return r.maximum.Layout(gtx, th.Theme) }), labelled(th, i18n.T("Default recipe seats"), func(gtx layout.Context) layout.Dimensions { return r.count.Layout(gtx, th.Theme) }))
		for i, s := range r.seats {
			if s.remove.Clicked(gtx) {
				r.seats = append(r.seats[:i], r.seats[i+1:]...)
				break
			}
		}
		for _, s := range r.seats {
			rows = append(rows, r.seatLayout(e, s))
		}
		rows = append(rows, action(e, &r.add, i18n.T("Add recipe seat"), len(r.seats) < protocol.TeamsMembersMax, func() {
			for {
				r.next++
				id := seatID(r.next)
				found := false
				for _, s := range r.seats {
					found = found || s.base.ID == id
				}
				if !found {
					r.seats = append(r.seats, newRecipeSeat(protocol.StructureSeat{ID: id}, true))
					break
				}
			}
		}), quiet(th, i18n.Tf("%d seat definitions. Saving validates missing managers, cycles and every selectable seat count.", len(r.seats))), action(e, &r.save, i18n.T("Save recipe"), strings.TrimSpace(r.name.Text()) != "" && len(r.seats) > 0, func() { r.saveValue(e, c) }))
	}
	if r.problem != "" {
		rows = append(rows, look.Paragraph(th, th.T.Body, th.R.Jasper, r.problem), action(e, &r.reload, i18n.T("Reopen latest recipe (discard these edits)"), !c.busy, func() {
			c.refresh()
			r.busy = true
			var result protocol.TeamStructuresResult
			e.Call(protocol.MTeamsStructures, protocol.Empty{}, &result, func(err error) {
				r.busy = false
				if err != nil {
					r.problem = err.Error()
					return
				}
				for _, t := range result.Templates {
					if t.ID == r.base.ID {
						r.open(t)
						return
					}
				}
				r.problem = i18n.T("Recipe was deleted. Restore it from the catalogue, or save your edits as a new recipe.")
			})
		}), action(e, &r.copy, i18n.T("Keep edits as a new recipe"), r.cloneSource == "", func() { r.base.ID = ""; r.base.Revision = 0; r.base.Hidden = false; r.id.SetText(""); r.problem = "" }))
	}
	rows = append(rows, action(e, &r.cancel, i18n.T("Close recipe editor"), !r.busy, func() { r.active = false }))
	return uiStack(e, rows...)(gtx)
}

func (r *recipeEditor) seatLayout(e *env.Env, s *recipeSeat) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		th := e.Theme
		label := i18n.Tf("Edit recipe seat: %s (%s)", s.title.Text(), s.base.ID)
		if s.expanded {
			label = i18n.Tf("Close recipe seat: %s", s.title.Text())
		}
		rows := []layout.Widget{action(e, &s.details, label, true, func() { s.expanded = !s.expanded })}
		if !s.expanded {
			return uiStack(e, rows...)(gtx)
		}
		rows = append(rows, field(e, &s.title, i18n.T("Seat title")), field(e, &s.rank, i18n.T("Seat rank")), field(e, &s.spirit, i18n.T("Default spirit identity")), field(e, &s.capabilities, i18n.T("Capabilities hint (not grants)")))
		s.duty.Label, s.duty.Lines = i18n.T("Seat duty"), 3
		rows = append(rows, func(gtx layout.Context) layout.Dimensions { return s.duty.Layout(gtx, th) }, choose(e, &s.model, i18n.T("Suggested model class (advice only)"), []string{"", "smart", "fast", "fast-tool-capable", "specialist", "reasoning", "vision"}, []string{i18n.T("No hint"), i18n.T("Smart"), i18n.T("Fast"), i18n.T("Fast tool-capable"), i18n.T("Specialist"), i18n.T("Reasoning"), i18n.T("Vision")}, &s.modelValue))
		ids, labels := []string{""}, []string{i18n.T("Summoner (no manager)")}
		for _, other := range r.seats {
			if other != s {
				ids = append(ids, other.base.ID)
				labels = append(labels, fmt.Sprintf("%s (%s)", other.title.Text(), other.base.ID))
			}
		}
		found := s.managerValue == ""
		for _, id := range ids {
			found = found || id == s.managerValue
		}
		if !found {
			ids = append(ids, s.managerValue)
			labels = append(labels, i18n.T("Removed manager — choose another"))
		}
		rows = append(rows, choose(e, &s.manager, i18n.T("Recipe seat reports to"), ids, labels, &s.managerValue))
		s.lead.Label = i18n.T("Recipe lead / optional peer synthesizer")
		rows = append(rows, func(gtx layout.Context) layout.Dimensions { return s.lead.Layout(gtx, th.Theme) }, func(gtx layout.Context) layout.Dimensions {
			s.remove.Label, s.remove.Small, s.remove.Kind = i18n.T("Remove recipe seat"), true, kit.PlaqueQuiet
			return s.remove.Layout(gtx, th.Theme)
		})
		return uiStack(e, rows...)(gtx)
	}
}
