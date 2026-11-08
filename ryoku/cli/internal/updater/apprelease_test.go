package updater

import (
	"context"
	"testing"

	"ryoku-cli/internal/apprelease"
)

func TestNativePackageManagersSkipStandaloneAppLanes(t *testing.T) {
	for _, manager := range []string{"dnf", "xbps"} {
		t.Run(manager, func(t *testing.T) {
			oldTunesCheck, oldTunesUpgrade := checkRyotunesRelease, upgradeRyotunesRelease
			oldPortCheck, oldPortUpgrade := checkRyoportRelease, upgradeRyoportRelease
			t.Cleanup(func() {
				checkRyotunesRelease, upgradeRyotunesRelease = oldTunesCheck, oldTunesUpgrade
				checkRyoportRelease, upgradeRyoportRelease = oldPortCheck, oldPortUpgrade
			})
			t.Setenv("RYOKU_HOST_PKGMGR", manager)
			unexpected := func(context.Context) (apprelease.Status, error) {
				t.Fatal("native repository lane queried or installed a standalone Arch release")
				return apprelease.Status{}, nil
			}
			checkRyotunesRelease, upgradeRyotunesRelease = unexpected, unexpected
			checkRyoportRelease, upgradeRyoportRelease = unexpected, unexpected

			upgradeIndependentApps()
			report := statusReport{}
			addIndependentAppUpdates(&report)
			if len(report.Updates) != 0 || report.Behind != 0 || report.Available {
				t.Fatalf("standalone lane changed native report: %#v", report)
			}
		})
	}
}
