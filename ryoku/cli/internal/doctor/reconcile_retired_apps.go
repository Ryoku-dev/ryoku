package doctor

import (
	"strings"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// ---- reconciler: retired first-party apps ------------------------------------

// retiredApp is an application Ryoku used to ship and no longer does. It shipped
// as a hard depend once, so a box installed before it was dropped still carries
// it and pacman never removes it on its own. doctor removes it, the same way it
// converges any other drift from the shipped package set.
type retiredApp struct {
	pkg   string
	label string
}

// retiredApps is the removal table. Ryoku Motion (ryomotion) was the screen-demo
// recorder and editor; the recorder is GPU Screen Recorder now, driven by
// `ryoku-shell record`, so ryomotion is no longer built or shipped.
var retiredApps = []retiredApp{
	{"ryomotion", "Ryoku Motion"},
}

// Seams: the live box's answers, replaced in tests.
var (
	retiredAppInstalled = func(pkg string) bool { return doctorPackageInstalled(pkg) }
	removeRetiredApp    = func(pkg string) error { return sys.Sudo("ryoku-host", "pkg", "remove", pkg) }
)

func retiredAppRemovalArgs(pkg string) []string {
	if manager, err := doctorPackageManager(); err == nil && manager == host.XBPS {
		return []string{"xbps-remove", "-y", pkg}
	}
	return []string{"pacman", "-Rns", "--noconfirm", pkg}
}

// reconcileRetiredApps removes any app in the table that is still installed. A
// box that never carried it, or where the user already removed it, is left
// untouched, so the check stays quiet once the package is gone.
func reconcileRetiredApps(checkOnly bool) recResult {
	manager, err := doctorPackageManager()
	if !hasPacman() && (err != nil || manager != host.XBPS) {
		return okRes(i18n.T("retired app package checks are not available on this host"))
	}
	var removed []string
	for _, app := range retiredApps {
		if !retiredAppInstalled(app.pkg) {
			continue
		}
		fix := "sudo " + strings.Join(retiredAppRemovalArgs(app.pkg), " ")
		if checkOnly {
			return wouldRes(i18n.T("%s is retired; remove it"), app.label).withFix(fix)
		}
		if err := removeRetiredApp(app.pkg); err != nil {
			return failRes(i18n.T("could not remove retired %s: %v"), app.label, err).withFix(fix)
		}
		removed = append(removed, app.label)
	}
	if len(removed) > 0 {
		return fixedRes(i18n.T("removed retired %s"), strings.Join(removed, ", "))
	}
	return okRes(i18n.T("no retired app packages installed"))
}
