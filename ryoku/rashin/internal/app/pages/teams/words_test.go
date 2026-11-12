package teams

import (
	"testing"

	appstate "ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

func TestTheHeaderNamesTheModelOfATeamOnOneEngine(t *testing.T) {
	snapshot := appstate.Snapshot{Harnesses: []protocol.HarnessInfo{{
		ID: "claude", Name: "Claude Code", Installed: true,
		Capabilities: &protocol.HarnessCapabilities{Models: true, ModelList: []protocol.Choice{
			{ID: "haiku", Name: "Haiku 4.5"}, {ID: "opus", Name: "Opus 5.5"},
		}},
	}}}
	alone := []protocol.Member{{ID: "lead", Lead: true, Harness: "claude", Model: "haiku"}}
	if got, want := runsFact(snapshot, alone), "Runs Haiku 4.5 on Claude Code"; got != want {
		t.Errorf("one engine reads %q, want %q", got, want)
	}
	// two models on one harness: the header names the harness and leaves the models to the crew
	mixed := append(alone, protocol.Member{ID: "second", Harness: "claude", Model: "opus"})
	if got, want := runsFact(snapshot, mixed), "Runs Claude Code"; got != want {
		t.Errorf("two engines read %q, want %q", got, want)
	}
}
