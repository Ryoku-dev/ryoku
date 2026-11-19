package teams

import (
	"testing"

	"ryoku-rashin/internal/app/pages/teamsui"
	appstate "ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

func TestTheHeaderNamesWhatATeamRunsOn(t *testing.T) {
	local := teamsui.ModelEngine(appstate.Snapshot{}, protocol.RouteAuto, nil)
	if got, want := runsFact([]teamsui.Engine{local}), "Runs Auto"; got != want {
		t.Errorf("one route reads %q, want %q", got, want)
	}
	// the same model thinking harder is one name in the header; the crew tells the two members apart
	harder := local
	harder.Extra = "effort high"
	if got, want := runsFact([]teamsui.Engine{local, harder}), "Runs Auto"; got != want {
		t.Errorf("one route with two sets of options reads %q, want %q", got, want)
	}
	remote := teamsui.Engine{ID: "anthropic/claude-sonnet", Model: "Claude Sonnet"}
	if got, want := runsFact([]teamsui.Engine{local, remote}), "Runs Auto and Claude Sonnet"; got != want {
		t.Errorf("two models read %q, want %q", got, want)
	}
	if got := runsFact(nil); got != "" {
		t.Errorf("a team without members reads %q, want nothing", got)
	}
}

func TestATileNamesOnlyAModelThatCannotRun(t *testing.T) {
	ready := teamsui.Engine{Model: "LFM3 2B"}
	gone := teamsui.Engine{Model: "Claude Sonnet", Missing: true}
	if got := stalledLine([]teamsui.Engine{ready}); got != "" {
		t.Errorf("a team whose model runs says %q on its tile, want nothing", got)
	}
	if got, want := stalledLine([]teamsui.Engine{ready, gone}), "Claude Sonnet cannot run now"; got != want {
		t.Errorf("one stalled model reads %q, want %q", got, want)
	}
}
