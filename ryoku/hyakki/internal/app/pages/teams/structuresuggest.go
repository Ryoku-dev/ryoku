package teams

import (
	"gioui.org/layout"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

type structureAdvice struct {
	open, busy                           bool
	request                              uint64
	problem                              string
	job                                  kit.TextField
	answers                              protocol.StructureAnswers
	requested                            protocol.StructureAnswers
	resultsEpoch                         int
	resultsRevision                      uint64
	work, flow, size, oversight, culture look.Select
	count                                kit.Stepper
	start, suggest, reject               kit.Plaque
	results                              []protocol.StructureSuggestion
	accept                               []*kit.Plaque
	inspect                              []*kit.Plaque
	expanded                             []bool
}

func (a *structureAdvice) layout(gtx layout.Context, e *env.Env, d *draft) layout.Dimensions {
	th := e.Theme
	rows := []layout.Widget{action(e, &a.start, i18n.T("Auto-suggest"), !d.busy, func() { a.open = !a.open; a.request++; a.busy = false })}
	if !a.open {
		return uiStack(e, rows...)(gtx)
	}
	current := a.answers
	current.Job = a.job.Text()
	if current.Job == "" {
		current.Job = d.purpose.Text()
	}
	current.Count = a.count.Value
	if a.results != nil && (current != a.requested || a.resultsEpoch != e.Snapshot().Epoch || a.resultsRevision != e.Snapshot().StructuresRevision) {
		a.results = nil
	}
	if a.busy {
		gtx = gtx.Disabled()
	}
	rows = append(rows, field(e, &a.job, i18n.T("One-line job for local advice")))
	if a.answers.Work == "" {
		a.answers.Work = "build"
		a.answers.Flow = "deep"
		a.answers.Size = "choose"
		a.answers.Oversight = "risky-steps"
	}
	rows = append(rows, quiet(th, i18n.T("Five quick questions. Advice is computed locally; no team, external call or credit spend is created.")),
		choose(e, &a.work, i18n.T("Kind of work"), []string{"build", "research", "watch", "decide", "triage", "create", "verify", "harden"}, []string{i18n.T("Build"), i18n.T("Research"), i18n.T("Watch"), i18n.T("Decide"), i18n.T("Triage"), i18n.T("Create"), i18n.T("Verify"), i18n.T("Harden")}, &a.answers.Work),
		choose(e, &a.flow, i18n.T("Work flow"), []string{"deep", "wide", "ring", "horde"}, []string{i18n.T("One direction"), i18n.T("Many angles"), i18n.T("Parallel domains"), i18n.T("Many hands")}, &a.answers.Flow),
		choose(e, &a.size, i18n.T("Team size"), []string{"few", "court", "horde", "choose"}, []string{i18n.T("A few"), i18n.T("A court"), i18n.T("A horde"), i18n.T("You choose")}, &a.answers.Size),
		choose(e, &a.oversight, i18n.T("Suggested oversight"), []string{"every-step", "risky-steps", "within-budget"}, []string{i18n.T("Approve every step"), i18n.T("Approve risky steps"), i18n.T("Within my finite budget")}, &a.answers.Oversight),
		choose(e, &a.culture, i18n.T("Culture preference (optional)"), []string{"", "goetia", "japanese", "elemental", "mixed"}, []string{i18n.T("No preference"), i18n.T("Goetia"), i18n.T("Japanese"), i18n.T("Elemental"), i18n.T("Mixed")}, &a.answers.Culture))
	a.count.Min, a.count.Max, a.count.Step, a.count.Unit = 0, protocol.TeamsMembersMax, 1, i18n.T("seats (0 lets the recipe choose)")
	rows = append(rows, labelled(th, i18n.T("Exact count (optional, includes synthesizer)"), func(gtx layout.Context) layout.Dimensions { return a.count.Layout(gtx, th.Theme) }), action(e, &a.suggest, i18n.T("Show local suggestions"), !a.busy && e.Snapshot().Connection == state.Ready, func() {
		a.request++
		request, epoch, revision := a.request, e.Snapshot().Epoch, e.Snapshot().StructuresRevision
		a.busy, a.problem = true, ""
		answers := a.answers
		answers.Job = a.job.Text()
		if answers.Job == "" {
			answers.Job = d.purpose.Text()
		}
		answers.Count = a.count.Value
		a.requested = answers
		var result protocol.StructureSuggestionsResult
		e.Call(protocol.MTeamsStructureSuggest, answers, &result, func(err error) {
			if request != a.request {
				return
			}
			a.busy = false
			if epoch != e.Snapshot().Epoch {
				a.problem = i18n.T("Connection changed. Request local suggestions again.")
				return
			}
			if revision != e.Snapshot().StructuresRevision {
				a.problem = i18n.T("Recipes changed while suggesting. Request fresh local suggestions.")
				return
			}
			if err != nil {
				a.problem = err.Error()
				return
			}
			a.results = result.Suggestions
			a.accept = make([]*kit.Plaque, len(a.results))
			a.inspect = make([]*kit.Plaque, len(a.results))
			a.expanded = make([]bool, len(a.results))
			a.resultsEpoch, a.resultsRevision = epoch, revision
			for i := range a.accept {
				a.accept[i] = new(kit.Plaque)
				a.inspect[i] = new(kit.Plaque)
				a.expanded[i] = a.results[i].SeatCount <= 3
			}
		})
	}))
	if a.busy {
		rows = append(rows, quiet(th, i18n.T("Comparing local structure recipes…")))
	}
	if a.problem != "" {
		rows = append(rows, look.Paragraph(th, th.T.Body, th.R.Jasper, a.problem))
	}
	if a.results != nil && len(a.results) == 0 {
		rows = append(rows, quiet(th, i18n.T("No visible recipe matches these answers and exact count. Change the count or select a structure directly.")))
	}
	for i, s := range a.results {
		rows = append(rows, kit.Text(th.Theme, th.T.Row, th.R.Fg, i18n.Tf("%s — %s, %d seats", s.Template.Name, s.Template.Shape, s.SeatCount)), quiet(th, s.Why), quiet(th, i18n.Tf("Proposed approvals: %s. Plan approval: %t. Review before summoning.", s.Approvals, s.PlanApproval)))
		rows = append(rows, action(e, a.inspect[i], i18n.Tf("Inspect proposed roster: %s", s.Template.Name), true, func() { a.expanded[i] = !a.expanded[i] }))
		if a.expanded[i] {
			for _, m := range s.Members {
				rows = append(rows, quiet(th, i18n.Tf("%s / %s — %s; reports to %s", m.Name, m.Rank, m.Instructions, rosterManagerName(s.Members, m.ReportsTo))))
			}
		}
		rows = append(rows, action(e, a.accept[i], i18n.Tf("Use suggestion: %s", s.Template.Name), !d.busy && !d.rosterBusy, func() {
			current, ok := d.catalog.byID(s.Template.ID)
			if !ok || current.Hidden || current.Revision != s.Template.Revision {
				a.problem = i18n.T("This recipe changed or was removed. Reload structures and request suggestions again.")
				return
			}
			d.applyStructure(e, s.Template, s.Members)
			d.approvalsValue, d.planApproval.Value = s.Approvals, s.PlanApproval
			if job := a.job.Text(); job != "" {
				d.purpose.SetText(job)
			}
			a.open = false
			a.request++
		}))
	}
	rows = append(rows, action(e, &a.reject, i18n.T("Reject suggestions and keep my roster"), true, func() { a.results = nil; a.open = false; a.request++; a.busy = false }))
	return uiStack(e, rows...)(gtx)
}
