package demo

import (
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// The demo answers a look for the local decision server as the daemon does:
// detected at the default address, not found anywhere else, too small on
// the small computer, and an address off this computer refused.
func TestDemoLocalDecisionLookStates(t *testing.T) {
	look := func(s *server) protocol.DecisionLocalStatus {
		t.Helper()
		return call(t, s, protocol.MFastDecideLocal, protocol.Empty{}).(protocol.DecisionLocalStatus)
	}
	s := &server{data: makeFixtures(FeatureOni)}
	if status := look(s); status.State != protocol.DecisionLocalDetected || status.Address != protocol.DefaultDecisionAddress || status.Reason != "" {
		t.Fatalf("default address: %+v", status)
	}
	s.data.snapshot.Settings.Providers.Decisions.LocalAddress = "http://localhost:11436/v1"
	if status := look(s); status.State != protocol.DecisionLocalAbsent || !strings.Contains(status.Reason, "No local decision server answers at http://127.0.0.1:11436/v1") {
		t.Fatalf("another address: %+v", status)
	}
	s.data.snapshot.Settings.Providers.Decisions.LocalAddress = "http://192.168.1.4:11435/v1"
	if status := look(s); status.State != protocol.DecisionLocalRefused {
		t.Fatalf("an address off this computer: %+v", status)
	}

	small := &server{data: makeFixtures(FeatureOni, FeatureOniSmallComputer)}
	if small.data.snapshot.Settings.Features[FeatureOniSmallComputer] {
		t.Fatal("the demo switch leaked into the feature flags")
	}
	if status := look(small); status.State != protocol.DecisionLocalSmall || !strings.Contains(status.Reason, "too small") || status.Cores != 2 {
		t.Fatalf("small computer: %+v", status)
	}
}
