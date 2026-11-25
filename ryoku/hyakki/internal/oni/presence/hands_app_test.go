package presence

import (
	"slices"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
)

// A task in a live window or on Hyakki's separate desktop rides in the
// snapshot with its scope, and the window's app id and output, while it
// runs; a scope change is a new frame; the handoff card of either scope is
// held while it waits; and the inactive frame leaves nothing behind.
func TestTheSnapshotCarriesAppAndDesktopTasksWithTheirScope(t *testing.T) {
	for _, tc := range []struct {
		name    string
		frame   feature.HandsFrame
		feature string
	}{
		{"window", feature.HandsFrame{Stage: protocol.OniHandsApp, Active: true, Reason: feature.ReasonAllowed, Scope: protocol.OniHandsScopeWindow,
			App: "org.example.Editor", Output: "DP-1", Phase: protocol.OniHandsStateWorking, Step: 1, Max: 25}, feature.HandsApp},
		{"nested", feature.HandsFrame{Stage: protocol.OniHandsApp, Active: true, Reason: feature.ReasonAllowed, Scope: protocol.OniHandsScopeNested,
			Phase: protocol.OniHandsStateWorking, Step: 1, Max: 25}, feature.HandsApp},
		{"desktop", feature.HandsFrame{Stage: protocol.OniHandsDesktop, Active: true, Reason: feature.ReasonAllowed, Scope: protocol.OniHandsScopeDesktop,
			Phase: protocol.OniHandsStateWorking, Step: 1, Max: 25}, feature.HandsDesktop},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, rec := started(t)
			working := tc.frame
			hands(t, s, working)
			if got, want := rec.take(), encode(t, &working); !slices.Equal(got, want) {
				t.Fatalf("starting sent %q, want %q", got, want)
			}
			snap := s.Snapshot()
			held, ok := snap[len(snap)-1].(*feature.HandsFrame)
			if !ok || *held != working {
				t.Fatalf("the snapshot holds %+v, want %+v", snap[len(snap)-1], working)
			}

			paused := working
			paused.Phase, paused.Pause = protocol.OniHandsStatePaused, protocol.OniHandsPauseKey
			hands(t, s, paused)
			if got, want := rec.take(), encode(t, &paused); !slices.Equal(got, want) {
				t.Fatalf("pausing sent %q, want %q", got, want)
			}

			waiting := working
			waiting.Phase = protocol.OniHandsStateWaiting
			hands(t, s, waiting)
			card := feature.HandoffCard(tc.feature, "h-7", "Your turn. Press Save when it looks right.", time.Minute)
			if err := s.Consent(*card); err != nil {
				t.Fatal(err)
			}
			rec.take()
			if got, want := kinds(s.Snapshot()), append(slices.Clone(oldOrder), feature.KindHands, feature.KindConsent); !slices.Equal(got, want) {
				t.Fatalf("waiting snapshot = %q, want %q", got, want)
			}
			if c := s.Snapshot()[len(oldOrder)+1].(*feature.ConsentFrame); c.ID != "h-7" || c.Feature != tc.feature {
				t.Fatalf("the snapshot holds card %q of %s", c.ID, c.Feature)
			}

			ended := feature.HandsFrame{Stage: working.Stage, Reason: feature.ReasonAllowed, End: protocol.OniHandsEndStopped}
			hands(t, s, ended)
			if got, want := rec.take(), encode(t, &ended); !slices.Equal(got, want) {
				t.Fatalf("ending sent %q, want %q", got, want)
			}
			if got := kinds(s.Snapshot()); !slices.Equal(got, oldOrder) {
				t.Fatalf("snapshot after the task = %q", got)
			}
		})
	}
}
