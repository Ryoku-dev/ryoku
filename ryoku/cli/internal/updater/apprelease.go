package updater

import (
	"context"
	"time"

	"ryoku-cli/internal/apprelease"
	"ryoku-cli/internal/host"

	i18n "ryoku-i18n"
)

var (
	checkRyotunesRelease   = apprelease.Ryotunes.Check
	upgradeRyotunesRelease = apprelease.Ryotunes.Upgrade
	checkRyoportRelease    = apprelease.Ryoport.Check
	upgradeRyoportRelease  = apprelease.Ryoport.Upgrade
)

type independentReleaseApp struct {
	name, display string
	check         func(context.Context) (apprelease.Status, error)
	upgrade       func(context.Context) (apprelease.Status, error)
}

func independentReleaseApps() [2]independentReleaseApp {
	return [2]independentReleaseApp{
		{"ryotunes", "Ryotunes", checkRyotunesRelease, upgradeRyotunesRelease},
		{"ryoport", "Ryoport", checkRyoportRelease, upgradeRyoportRelease},
	}
}

// upgradeIndependentApps follows independently published Arch packages between
// repository imports. Native package managers use their normal Ryoku repository.
func upgradeIndependentApps() {
	if updatePackageManager() != host.Pacman {
		return
	}
	for _, app := range independentReleaseApps() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		st, err := app.upgrade(ctx)
		cancel()
		switch {
		case err != nil:
			progress.logf(i18n.T("Could not update %s: %v"), app.display, err)
		case st.Installed == "":
		case st.Updated:
			progress.logf(i18n.T("%s updated to %s"), app.display, st.Latest)
		default:
			progress.detailf(i18n.T("%s is current (%s)"), app.display, st.Installed)
		}
	}
}

func addIndependentAppUpdates(r *statusReport) {
	if updatePackageManager() != host.Pacman {
		return
	}
	for _, app := range independentReleaseApps() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		st, err := app.check(ctx)
		cancel()
		if err != nil || !st.Available {
			continue
		}
		r.Updates = append(r.Updates, updateItem{Name: app.name, Old: st.Installed, New: st.Latest})
		r.Behind++
		r.Available = true
	}
}
