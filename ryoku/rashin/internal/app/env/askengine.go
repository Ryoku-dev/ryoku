package env

import (
	"ryoku-i18n"

	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// one choice of what answers Ask: Rashin's providers (no ID) or a harness, perhaps no longer installed
type AskEngine struct {
	ID, Name  string
	Installed bool
}

// what can answer a new ask: Rashin's providers, every installed harness and the one the setting names even when it is gone
func AskEngines(snapshot state.Snapshot) (engines []AskEngine, selected int) {
	want := snapshot.Settings.Ask.Harness
	engines = []AskEngine{{Name: i18n.T("Rashin's providers"), Installed: true}}
	for _, harness := range snapshot.Harnesses {
		if !harness.Installed && harness.ID != want {
			continue
		}
		if harness.ID == want {
			selected = len(engines)
		}
		engines = append(engines, AskEngine{ID: harness.ID, Name: harness.Name, Installed: harness.Installed})
	}
	if want != "" && selected == 0 {
		selected = len(engines)
		engines = append(engines, AskEngine{ID: want, Name: want})
	}
	return engines, selected
}

// how the composer names what answers a conversation
func AskEngineName(snapshot state.Snapshot, info protocol.SessionInfo) string {
	if info.Harness == "" {
		return i18n.T("Rashin's providers")
	}
	for _, harness := range snapshot.Harnesses {
		if harness.ID == info.Harness && harness.Name != "" {
			return harness.Name
		}
	}
	return info.Harness
}
