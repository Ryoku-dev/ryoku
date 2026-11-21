package main

import (
	"testing"

	wm "ryoku-wm"
)

func TestCapsManifestHonoursClaims(t *testing.T) {
	want := []wm.Capability{
		wm.CapWorkspaces, wm.CapMonitorConfig, wm.CapNativeOverview, wm.CapOverviewState,
		wm.CapOutputPower, wm.CapSessionExit, wm.CapTouchpadToggle,
	}
	for _, c := range want {
		found := false
		for _, have := range capsManifest {
			if have == c {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("capsManifest missing %s", c)
		}
	}
}
