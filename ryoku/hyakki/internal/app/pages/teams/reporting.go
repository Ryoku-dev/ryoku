package teams

import (
	"reflect"
	"strings"

	"gioui.org/layout"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/protocol"
)

type reportingDraft struct {
	base, team             protocol.Team
	structure, culture     kit.TextField
	modePick, rootPick     look.Select
	managers               map[string]*look.Select
	chart                  teamsui.CrewEditor
	review                 kit.Toggle
	save, reload           kit.Plaque
	busy                   bool
	generation             uint64
	problem, reviewedValue string
}

func editableTeam(team protocol.Team) protocol.Team {
	team.Members = append([]protocol.Member(nil), team.Members...)
	if team.Dream != nil {
		d := *team.Dream
		team.Dream = &d
	}
	// Old teams already had an explicit lead. Resolve their legacy star once,
	// without assigning a peer a manager or selecting a new lead.
	if team.Reporting == "" {
		team.Reporting = protocol.ReportingTree
		lead := ""
		for _, m := range team.Members {
			if m.Lead {
				lead = m.ID
			}
		}
		for i := range team.Members {
			if !team.Members[i].Lead {
				team.Members[i].ReportsTo = lead
			}
		}
	}
	return team
}

func (d *reportingDraft) set(t protocol.Team) {
	d.generation++
	d.busy = false
	d.base, d.team = editableTeam(t), editableTeam(t)
	d.structure.SetText(t.Structure)
	d.culture.SetText(t.Culture)
	d.problem, d.reviewedValue, d.review.Value = "", "", false
}
func (d *reportingDraft) value() protocol.Team {
	t := d.team
	t.Structure, t.Culture = strings.TrimSpace(d.structure.Text()), strings.TrimSpace(d.culture.Text())
	return t
}
func (d *reportingDraft) dirty() bool {
	t := d.value()
	return t.Structure != d.base.Structure || t.Culture != d.base.Culture || t.Reporting != d.base.Reporting || !reflect.DeepEqual(t.Members, d.base.Members) || !reflect.DeepEqual(t.Dream, d.base.Dream)
}
func graphRoot(t protocol.Team) string {
	for _, m := range t.Members {
		if m.Lead {
			return m.ID
		}
	}
	return ""
}

func replaceRoot(t *protocol.Team, root string) {
	old := graphRoot(*t)
	for i := range t.Members {
		m := &t.Members[i]
		m.Lead = m.ID == root
		if m.Lead {
			m.ReportsTo = ""
		} else if m.ID == old && t.Reporting == protocol.ReportingTree {
			m.ReportsTo = root
		}
	}
	if t.Dream != nil {
		t.Dream.Manager = root
	}
}

func graphChanges(before, after protocol.Team) string {
	var changes []string
	if before.Reporting != after.Reporting {
		changes = append(changes, i18n.Tf("Reporting: %s → %s", before.Reporting, after.Reporting))
	}
	for _, old := range before.Members {
		now, ok := protocol.MemberByID(after.Members, old.ID)
		if !ok {
			changes = append(changes, i18n.Tf("Remove %s; direct reports: %s", old.Name, reportNames(before, old.ID)))
			continue
		}
		if old.Lead != now.Lead {
			changes = append(changes, i18n.Tf("%s: root / synthesizer %t → %t", old.Name, old.Lead, now.Lead))
		}
		if old.ReportsTo != now.ReportsTo {
			changes = append(changes, i18n.Tf("%s reports to %s → %s", old.Name, managerName(before, old.ReportsTo), managerName(after, now.ReportsTo)))
		}
	}
	for _, added := range after.Members {
		if _, exists := protocol.MemberByID(before.Members, added.ID); !exists {
			changes = append(changes, i18n.Tf("Add %s; reports to %s; root / synthesizer: %t", added.Name, managerName(after, added.ReportsTo), added.Lead))
		}
	}
	return strings.Join(changes, "\n")
}
func managerName(t protocol.Team, id string) string {
	if id == "" {
		return i18n.T("Summoner")
	}
	if m, ok := protocol.MemberByID(t.Members, id); ok {
		return m.Name
	}
	return i18n.Tf("Missing manager (%s)", id)
}
func reportNames(t protocol.Team, id string) string {
	names := []string{}
	for _, m := range protocol.DirectReports(t, id) {
		names = append(names, m.Name)
	}
	if len(names) == 0 {
		return i18n.T("none")
	}
	return strings.Join(names, ", ")
}

func managerField(e *env.Env, pick *look.Select, t protocol.Team, member *protocol.Member) layout.Widget {
	ids, names := []string{""}, []string{i18n.T("Summoner (no manager)")}
	if t.Reporting != protocol.ReportingPeers {
		for _, other := range t.Members {
			if other.ID != member.ID {
				ids = append(ids, other.ID)
				names = append(names, other.Name)
			}
		}
	}
	found := false
	for _, id := range ids {
		found = found || id == member.ReportsTo
	}
	if !found {
		ids = append(ids, member.ReportsTo)
		names = append(names, i18n.T("Invalid manager — choose another"))
	}
	return choose(e, pick, i18n.Tf("%s reports to", member.Name), ids, names, &member.ReportsTo)
}
func rootField(e *env.Env, pick *look.Select, t *protocol.Team) layout.Widget {
	ids, names := []string{""}, []string{i18n.T("No synthesizer (peers only)")}
	for _, m := range t.Members {
		ids = append(ids, m.ID)
		names = append(names, m.Name)
	}
	return func(gtx layout.Context) layout.Dimensions {
		old := graphRoot(*t)
		root := old
		dims := choose(e, pick, i18n.T("Root / optional peer synthesizer"), ids, names, &root)(gtx)
		if root != old {
			replaceRoot(t, root)
		}
		return dims
	}
}

func (d *reportingDraft) reparent(from, to int) {
	candidate := editableTeam(d.team)
	if from < 0 || from >= len(candidate.Members) || to < -1 || to >= len(candidate.Members) {
		return
	}
	candidate.Members[from].ReportsTo = ""
	if to >= 0 {
		candidate.Members[from].ReportsTo = candidate.Members[to].ID
	}
	if err := protocol.ValidateReporting(candidate); err != nil {
		d.problem = err.Error()
		return
	}
	d.team = candidate
	d.problem = ""
	d.review.Value = false
}

func (d *reportingDraft) layout(e *env.Env, current protocol.Team) layout.Widget {
	if d.base.ID != current.ID || !d.busy && !d.dirty() && current.Updated.After(d.base.Updated) {
		d.set(current)
	}
	th := e.Theme
	return func(gtx layout.Context) layout.Dimensions {
		if d.busy {
			gtx = gtx.Disabled()
		}
		rows := []layout.Widget{field(e, &d.structure, i18n.T("Current structure / template ID")), field(e, &d.culture, i18n.T("Current culture")), quiet(th, i18n.T("Changing these labels never respawns spirits or applies a recipe. Edit the existing reporting graph below.")), choose(e, &d.modePick, i18n.T("Reporting mode"), []string{protocol.ReportingTree, protocol.ReportingPeers}, []string{i18n.T("Tree: one reachable root"), i18n.T("Peers: every spirit reports to Summoner")}, &d.team.Reporting), rootField(e, &d.rootPick, &d.team)}
		d.chart.Reparent = d.reparent
		rows = append(rows, quiet(th, i18n.T("Drag a spirit's chart tile onto its manager (or onto Summoner). Root replacement uses the picker and reviewed changes. Reporting pickers below provide the same keyboard-reachable operation.")), func(gtx layout.Context) layout.Dimensions {
			style := teamsui.FullCrew(th)
			style.Editor = &d.chart
			return teamsui.CrewChart(gtx, th, e.Snapshot(), d.team, nil, style)
		})
		if d.managers == nil {
			d.managers = map[string]*look.Select{}
		}
		for i := range d.team.Members {
			m := &d.team.Members[i]
			if d.managers[m.ID] == nil {
				d.managers[m.ID] = new(look.Select)
			}
			rows = append(rows, managerField(e, d.managers[m.ID], d.team, m))
		}
		t := d.value()
		changes := graphChanges(d.base, t)
		signature := changes + t.Reporting + t.Structure + t.Culture
		if signature != d.reviewedValue {
			d.reviewedValue = signature
			d.review.Value = false
		}
		valid := protocol.ValidateReporting(t)
		if valid != nil {
			rows = append(rows, look.Paragraph(th, th.T.Detail, th.R.Jasper, valid.Error()))
		}
		if changes != "" {
			rows = append(rows, quiet(th, changes))
		}
		if d.dirty() {
			rows = append(rows, toggleField(e, &d.review, i18n.T("I reviewed the replacement root and affected reporting lines")), action(e, &d.save, i18n.T("Save reviewed reporting graph"), valid == nil && d.review.Value && !d.busy, func() {
				d.busy, d.problem = true, ""
				generation, epoch := d.generation, e.Snapshot().Epoch
				patch := map[string]any{"members": t.Members, "reporting": t.Reporting, "structure": t.Structure, "culture": t.Culture}
				var saved protocol.Team
				e.Call(protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: t.ID, ExpectedUpdated: d.base.Updated, Patch: rawJSON(patch)}, &saved, func(err error) {
					if generation != d.generation {
						return
					}
					d.busy = false
					if err != nil {
						d.problem = i18n.Tf("Not saved: %s. Your graph edits are kept. Reload deliberately to recover from a conflict.", err.Error())
						return
					}
					if epoch != e.Snapshot().Epoch {
						d.problem = i18n.T("The connection changed. Your draft is kept; reload current graph deliberately.")
						return
					}
					d.set(saved)
				})
			}))
		}
		if d.problem != "" {
			rows = append(rows, look.Paragraph(th, th.T.Detail, th.R.Jasper, d.problem))
		}
		if d.dirty() || d.problem != "" {
			rows = append(rows, action(e, &d.reload, i18n.T("Reload current graph (discard draft)"), !d.busy, func() { d.set(current) }))
		}
		return uiStack(e, rows...)(gtx)
	}
}
