package env

import (
	"sort"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// what a harness will run with: its own settings where Rashin can read them, else what it came up with last time
func HarnessFacts(harness protocol.HarnessInfo) []protocol.HarnessFact {
	if len(harness.Setup) > 0 {
		return harness.Setup
	}
	if harness.Capabilities != nil {
		return harness.Capabilities.Started
	}
	return nil
}

// the harness's facts with the model a new session starts on: the pick where there is one, the harness's own model otherwise
func StartFacts(harness protocol.HarnessInfo, model string) []protocol.HarnessFact {
	facts := HarnessFacts(harness)
	if model == "" {
		return facts
	}
	pick := protocol.HarnessFact{Label: i18n.T("Model"), Value: ModelName(harness.Capabilities, model)}
	// a copy, the facts belong to the snapshot
	result := make([]protocol.HarnessFact, 0, len(facts)+1)
	replaced := false
	for _, fact := range facts {
		if fact.Label == pick.Label && !replaced {
			fact, replaced = pick, true
		}
		result = append(result, fact)
	}
	if !replaced {
		result = append([]protocol.HarnessFact{pick}, result...)
	}
	return result
}

func FactsSentence(facts []protocol.HarnessFact) string {
	parts := make([]string, len(facts))
	for index, fact := range facts {
		parts[index] = fact.Label + " " + fact.Value
	}
	return strings.Join(parts, ", ")
}

// two providers offer a model under one name and the id tells them apart, an id that only repeats the name says nothing
func ChoiceOption(choice protocol.Choice) look.SelectOption {
	name, detail := choice.Name, choice.Description
	if name == "" {
		name = choice.ID
	}
	if detail == "" && !strings.EqualFold(choice.ID, name) {
		detail = choice.ID
	}
	return look.SelectOption{Label: name, Detail: detail}
}

// each fact as a quiet label and its value in mono, wrapping when the row is full
func LayoutFacts(gtx layout.Context, th *look.Theme, facts []protocol.HarnessFact) layout.Dimensions {
	return look.Flow{Gap: 13, LineGap: 4}.Layout(gtx, len(facts), func(gtx layout.Context, index int) layout.Dimensions {
		fact := facts[index]
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
			layout.Rigid(kit.RawText(th.Theme, th.T.Caption, th.R.Fg3, fact.Label)),
			layout.Rigid(layout.Spacer{Width: 5}.Layout),
			layout.Rigid(kit.RawText(th.Theme, th.T.MonoSm, th.R.Fg2, fact.Value)),
		)
	})
}

// the one new sessions start with comes first
func InstalledHarnesses(snapshot state.Snapshot) []protocol.HarnessInfo {
	result := make([]protocol.HarnessInfo, 0, len(snapshot.Harnesses))
	for _, harness := range snapshot.Harnesses {
		if harness.Installed {
			result = append(result, harness)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Default != result[j].Default {
			return result[i].Default
		}
		return result[i].Name < result[j].Name
	})
	return result
}

// picking one makes it the setting, so every place a session starts from and Settings show the same harness
type HarnessPick struct {
	// picked here and not yet confirmed by the daemon, with the setting it replaces
	picked, from string
}

// harnesses is what InstalledHarnesses returned
func (pick *HarnessPick) Index(snapshot state.Snapshot, harnesses []protocol.HarnessInfo) int {
	want := snapshot.Settings.Agent.Harness
	if pick.picked != "" {
		if want == pick.from {
			want = pick.picked
		} else {
			// the setting moved, to this pick or to one made elsewhere, and it rules either way
			pick.picked = ""
		}
	}
	for index, harness := range harnesses {
		if harness.ID == want {
			return index
		}
	}
	return 0
}

func (pick *HarnessPick) Use(e *Env, harnesses []protocol.HarnessInfo, index int) {
	if index < 0 || index >= len(harnesses) {
		return
	}
	id, current := harnesses[index].ID, e.Snapshot().Settings.Agent.Harness
	if id == current {
		pick.picked = ""
		return
	}
	pick.picked, pick.from = id, current
	e.SetSettings(map[string]any{"agent": map[string]any{"harness": id}}, func(err error) {
		if err != nil {
			pick.picked = ""
			e.Notify(err.Error())
		}
	})
}
