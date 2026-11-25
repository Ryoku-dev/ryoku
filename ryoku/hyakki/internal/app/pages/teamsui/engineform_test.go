package teamsui

import (
	"testing"

	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

func TestTeamModelFormFollowsAndOverridesTheDefault(t *testing.T) {
	snapshot := state.Snapshot{}
	snapshot.Settings.Agent.Model = protocol.RouteAuto
	snapshot.Settings.Teams.Defaults.Model = "rashin-models/lfm3-omni-2b"
	snapshot.Settings.Teams.Defaults.Options = map[string]string{protocol.OptionEffort: "low", "context": "large"}

	var form EngineForm
	form.Set("", nil)
	model, options := form.Effective(snapshot)
	if model != snapshot.Settings.Teams.Defaults.Model || options[protocol.OptionEffort] != "low" || options["context"] != "large" {
		t.Fatalf("inherited model = %q, options = %v", model, options)
	}
	if stored, picks := form.Value(); stored != "" || len(picks) != 0 {
		t.Fatalf("following a default stored model %q, options %v", stored, picks)
	}

	form.Set("openrouter/qwen3-coder:free", map[string]string{protocol.OptionEffort: "high"})
	model, options = form.Effective(snapshot)
	if model != "openrouter/qwen3-coder:free" || options[protocol.OptionEffort] != "high" || options["context"] != "large" {
		t.Fatalf("picked model = %q, options = %v", model, options)
	}
}

func TestTaskModelFormFollowsItsMember(t *testing.T) {
	snapshot := state.Snapshot{}
	snapshot.Settings.Agent.Model = protocol.RouteAuto
	member := protocol.Member{Name: "Researcher", Model: "ollama-local/qwen3", Options: map[string]string{protocol.OptionEffort: "medium"}}
	form := EngineForm{Keep: &member}
	form.Set("", nil)
	model, options := form.Effective(snapshot)
	if model != member.Model || options[protocol.OptionEffort] != "medium" {
		t.Fatalf("member's model = %q, options = %v", model, options)
	}
}
