package updater

import (
	"context"
	"errors"
	"time"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/ryotunesrelease"

	i18n "ryoku-i18n"
)

var (
	checkRyotunesRelease   = ryotunesrelease.Check
	upgradeRyotunesRelease = ryotunesrelease.Upgrade
)

// upgradeRyotunes follows the independently published Arch package between
// repository imports. DNF and XBPS receive Ryotunes through their normal Ryoku
// repositories, so their ordinary package transaction owns every update and
// this standalone pacman package lane stays inactive.
//
// Best-effort, like the AUR and Flatpak lanes: a failure is reported and never
// fails the whole update. A removed Ryotunes remains removed.
func upgradeRyotunes() {
	if updatePackageManager() != host.Pacman {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	st, err := upgradeRyotunesRelease(ctx)
	switch {
	case errors.Is(err, ryotunesrelease.ErrNoRelease):
		progress.logf(i18n.T("No Ryotunes 1.x release is published; keeping Ryotunes %s"), st.Installed)
	case err != nil:
		progress.logf(i18n.T("Could not update Ryotunes: %v"), err)
	case st.Installed == "":
		// Ryotunes is not installed; nothing to update (a removal stays removed).
	case st.Updated:
		progress.logf(i18n.T("Ryotunes updated to %s"), st.Latest)
	default:
		progress.detailf(i18n.T("Ryotunes is current (%s)"), st.Installed)
	}
}

// addRyotunesUpdate folds a pending standalone Arch release into the Ryoku-lane
// report. DNF and XBPS already report their repository package through the
// normal edition lane and never query the GitHub package channel here.
func addRyotunesUpdate(r *statusReport) {
	if updatePackageManager() != host.Pacman {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	st, err := checkRyotunesRelease(ctx)
	if err != nil || !st.Available {
		return
	}
	r.Updates = append(r.Updates, updateItem{Name: "ryotunes", Old: st.Installed, New: st.Latest})
	r.Behind++
	r.Available = true
}
