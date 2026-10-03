package main

import (
	"strings"
	"testing"

	"ryoku-cli/internal/updater"
)

// The compositor switch installs its variant package through a plain pacman
// transaction. On a box seeded by an older ISO or a dev deploy, unowned copies
// of ryoku-desktop-owned system paths abort that transaction with "exists in
// filesystem" and the switch fails; the fix every other ryoku lane already
// carries is --overwrite for exactly those paths. Pin that the switch lane
// uses the shared glob, so a new seed path joins it in one place.
func TestWmSwitchInstallCarriesTheOverwriteGlob(t *testing.T) {
	args := wmSwitchInstallArgs("ryoku-desktop-mango")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--overwrite "+updater.RyokuOverwriteGlob+" ryoku-desktop-mango") {
		t.Fatalf("switch install = %q, want --overwrite %q before the target", joined, updater.RyokuOverwriteGlob)
	}
	// The move stays in the Ryoku lane: never a sysupgrade, never -Sy.
	for _, banned := range []string{"-Su", "-Syu", "-Syyu", "-Sy", "-u"} {
		for _, a := range args {
			if a == banned {
				t.Errorf("switch install runs %q: that is not the plain -S the switch wants", banned)
			}
		}
	}
}
