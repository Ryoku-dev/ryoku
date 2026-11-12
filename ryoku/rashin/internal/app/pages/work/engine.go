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

// the engine a task runs on as its details list it: the harness with its version, the
// model by its own id so it can be copied, and each option it sets by the harness's names
func engineFacts(th *look.Theme, snapshot state.Snapshot, task protocol.TeamTask) []layout.Widget {
	engine := teamsui.TaskEngine(snapshot, task)
	id, model, options := teamsui.TaskEngineIDs(snapshot, task)
	harness := engine.Harness
	var offered []protocol.SessionOption
	for _, info := range snapshot.Harnesses {
		if info.ID != id || !info.Installed {
			continue
		}
		harness = info.Name
		if info.Version != "" {
			harness = i18n.Tf("%s %s", info.Name, info.Version)
		}
		// the task ran in its team's own home, whose harness offers its own options
		if capabilities := info.TeamCapabilities(task.Team); capabilities != nil {
			offered = capabilities.Options
		}
	}
	facts := []layout.Widget{wordFact(th, i18n.T("Harness"), harness)}
	// no model of its own is the harness's default, which has no id to show and is said in words
	if model == "" {
		facts = append(facts, wordFact(th, i18n.T("Model"), engine.Model))
	} else {
		facts = append(facts, kit.AsideFact(th.Theme, i18n.T("Model"), model))
	}
	for _, option := range offered {
		if value, ok := options[option.ID]; ok {
			facts = append(facts, wordFact(th, option.Name, teamsui.ChoiceName(option.Choices, value)))
		}
	}
	// an option the harness no longer offers keeps its id
	for _, key := range slices.Sorted(maps.Keys(options)) {
		if !slices.ContainsFunc(offered, func(option protocol.SessionOption) bool { return option.ID == key }) {
			facts = append(facts, kit.AsideFact(th.Theme, key, options[key]))
		}
	}
	return facts
}
