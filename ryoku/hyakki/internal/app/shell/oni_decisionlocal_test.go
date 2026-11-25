package shell

import (
	"context"
	"encoding/json"
	"testing"

	"ryoku-rashin/internal/app/demo"
	"ryoku-rashin/internal/protocol"
)

// setDecisions patches providers.decisions on the demo daemon
func (s *stage) setDecisions(patch map[string]any) {
	s.t.Helper()
	raw, _ := json.Marshal(map[string]any{"providers": map[string]any{"decisions": patch}})
	if err := s.runtime.client.Call(context.Background(), protocol.MSettingsSet, protocol.SettingsSetParams{Patch: raw}, new(json.RawMessage)); err != nil {
		s.t.Fatal(err)
	}
	s.settle()
}

// The decision row says what a look for the local decision server found,
// and looks only once the route can reach it: detected at the address the
// demo computer's server answers on, not found at another, and too small
// on a computer under the hardware gate.
func TestOniReflexDecisionRowSaysWhatTheLocalLookFound(t *testing.T) {
	stage := newReflexStage(t, demo.FeatureOni)
	if stage.mentions("Local decision server") || stage.mentions("Looking for the local decision server") {
		t.Fatalf("looked with the route on built-in rules; on screen: %v", stage.names())
	}
	stage.setDecisions(map[string]any{"route": protocol.DecisionLocal})
	stage.await("the server is detected", func() bool {
		return stage.mentions("Local decision server: detected at " + protocol.DefaultDecisionAddress + ".")
	})
	stage.setDecisions(map[string]any{"localAddress": "http://127.0.0.1:11436/v1"})
	stage.await("the server is not found at another address", func() bool {
		return stage.mentions("Local decision server: not found. No local decision server answers at http://127.0.0.1:11436/v1.")
	})
	stage.setDecisions(map[string]any{"route": protocol.DecisionRules})
	stage.await("the line goes with the route", func() bool { return !stage.mentions("Local decision server") })

	small := newReflexStage(t, demo.FeatureOniSmallComputer)
	small.setDecisions(map[string]any{"route": protocol.DecisionAuto, "localModel": "fixture-local"})
	small.await("the computer is too small", func() bool {
		return small.mentions("This computer is too small for a local decision model")
	})
	if small.mentions("Local decision server: detected") {
		t.Fatalf("a small computer says the server is detected; on screen: %v", small.names())
	}
}
