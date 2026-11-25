package work

import (
	"maps"
	"slices"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// the task's model and its chosen options are the facts worth keeping beside its report
func engineFacts(th *look.Theme, snapshot state.Snapshot, task protocol.TeamTask) []layout.Widget {
	model, options := teamsui.TaskEngineIDs(snapshot, task)
	if model == "" {
		model = teamsui.TaskEngine(snapshot, task).ID
	}
	facts := []layout.Widget{wordFact(th, i18n.T("Model"), teamsui.ModelEngine(snapshot, model, nil).Place())}
	if !protocol.IsRoute(model) {
		facts = append(facts, kit.AsideFact(th.Theme, i18n.T("Model ID"), model))
	}
	for _, id := range slices.Sorted(maps.Keys(options)) {
		name, value := teamsui.OptionFact(id, options[id])
		facts = append(facts, wordFact(th, name, value))
	}
	return facts
}
