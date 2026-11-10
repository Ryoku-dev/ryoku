package env

import (
	"sort"

	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

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
