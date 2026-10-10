package doctor

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/ryokumanifest"
	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// shippedApp is one deliver-once package: installed once, then left alone if
// the user removes it. Membership rule: a standalone application whose absence
// costs only itself. Tools the shell calls by name (grim, playerctl, matugen,
// cava, mpv for the launcher's radio, the pill's OCR/capture backends) stay hard
// depends, because losing them breaks a Ryoku surface the user never touched.
// Ryotunes has its own official-release install/reconciliation path.
type shippedApp struct {
	pkg  string
	what string
}

// shippedApps is the deliver-once table. It lives in the manifest package so the
// release's control manifest and this reconciler read one list, never two.
func shippedApps() []shippedApp {
	apps := ryokumanifest.Apps()
	out := make([]shippedApp, 0, len(apps))
	for _, a := range apps {
		out = append(out, shippedApp{pkg: a.Pkg, what: a.What})
	}
	return out
}

type appPlan struct {
	install  []string // never seen here and absent: deliver once
	removed  []string // ledgered and gone: the user's call, honoured
	adopt    []string // present but unrecorded: ledger it
	explicit []string // present and installed-as-dependency: re-mark explicit
}

// planShippedApps is the three-way rule, pure so it is tested without pacman.
func planShippedApps(apps []shippedApp, installed, asDep, seen map[string]bool) appPlan {
	var p appPlan
	for _, a := range apps {
		switch {
		case installed[a.pkg]:
			if !seen[a.pkg] {
				p.adopt = append(p.adopt, a.pkg)
			}
			if asDep[a.pkg] {
				p.explicit = append(p.explicit, a.pkg)
			}
		case seen[a.pkg]:
			p.removed = append(p.removed, a.pkg)
		default:
			p.install = append(p.install, a.pkg)
		}
	}
	return p
}

func supportedShippedApps(manager host.PackageManager, apps []shippedApp) (supported []shippedApp, unavailable []string) {
	if manager == host.Pacman {
		return apps, nil
	}
	for _, app := range apps {
		if doctorPackageUnavailable(manager, app.pkg) {
			unavailable = append(unavailable, app.pkg)
			continue
		}
		supported = append(supported, app)
	}
	return supported, unavailable
}

func withUnavailablePackages(result recResult, unavailable []string) recResult {
	if len(unavailable) > 0 {
		result.detail += i18n.Tf("; not packaged for this host, so skipped: %s", strings.Join(unavailable, ", "))
	}
	return result
}

// Seams: the live box's answers, replaced in tests.
var (
	appInstalled      = func(pkg string) bool { return doctorPackageInstalled(pkg) }
	appInstalledAsDep = func(pkg string) bool {
		manager, err := doctorPackageManager()
		if err != nil {
			return false
		}
		if manager == host.XBPS {
			value, err := sys.RunOut("xbps-query", "-p", "automatic-install", pkg)
			return err == nil && strings.TrimSpace(value) == "true"
		}
		if manager == host.DNF {
			value, err := sys.RunOut(host.DNFCommand(), "repoquery", "-y", "--userinstalled", "--qf", "%{name}\n", pkg)
			return err == nil && strings.TrimSpace(value) == ""
		}
		return exec.Command("pacman", "-Qdq", pkg).Run() == nil
	}
	installShippedApps = func(pkgs []string) {
		_ = sys.Sudo(append([]string{"ryoku-host", "pkg", "install"}, pkgs...)...)
	}
	markAppsExplicit = func(pkgs []string) {
		_ = sys.Sudo(append([]string{"ryoku-host", "pkg", "explicit"}, pkgs...)...)
	}
	hasPacman = func() bool {
		manager, err := host.Default().PackageManager()
		return err == nil && manager == host.Pacman
	}
)

func reconcileShippedApps(checkOnly bool) recResult {
	manager, err := doctorPackageManager()
	if hasPacman() {
		manager, err = host.Pacman, nil
	}
	if err != nil || manager != host.Pacman && manager != host.XBPS && manager != host.DNF {
		return okRes(i18n.T("shipped apps are not managed on this host"))
	}
	apps, unavailable := supportedShippedApps(manager, shippedApps())
	installed, asDep := map[string]bool{}, map[string]bool{}
	for _, a := range apps {
		if appInstalled(a.pkg) {
			installed[a.pkg] = true
			asDep[a.pkg] = appInstalledAsDep(a.pkg)
		}
	}
	plan := planShippedApps(apps, installed, asDep, provisioned())

	if len(plan.install) == 0 && len(plan.adopt) == 0 && len(plan.explicit) == 0 {
		if len(plan.removed) > 0 {
			return withUnavailablePackages(noteRes(i18n.T("%s stay removed (you deleted them; Ryoku does not put them back)"),
				strings.Join(plan.removed, ", ")), unavailable)
		}
		if len(unavailable) > 0 {
			return noteRes(i18n.T("not packaged for this host, so skipped: %s"), strings.Join(unavailable, ", "))
		}
		return okRes(i18n.T("every shipped app is present and owned by you"))
	}
	if checkOnly {
		var parts []string
		if len(plan.install) > 0 {
			parts = append(parts, i18n.Tf("would install %s", strings.Join(plan.install, ", ")))
		}
		if len(plan.explicit) > 0 || len(plan.adopt) > 0 {
			parts = append(parts, fmt.Sprintf(i18n.T("would take ownership of %d present app(s)"),
				len(union(plan.adopt, plan.explicit))))
		}
		if len(unavailable) > 0 {
			parts = append(parts, i18n.Tf("skip %s (not packaged for this host)", strings.Join(unavailable, ", ")))
		}
		return wouldRes("%s", strings.Join(parts, "; ")).
			withFix(i18n.T("run `ryoku doctor` (or `ryoku update`) to apply"))
	}

	// Ownership first: it cannot fail the run, and it protects what is already
	// here even if the install half finds no mirror.
	if len(plan.explicit) > 0 {
		markAppsExplicit(plan.explicit)
	}
	for _, pkg := range plan.adopt {
		recordProvisioned(pkg)
	}

	var landed, missed []string
	if len(plan.install) > 0 {
		installShippedApps(plan.install)
		for _, pkg := range plan.install {
			if appInstalled(pkg) {
				recordProvisioned(pkg)
				landed = append(landed, pkg)
			} else {
				missed = append(missed, pkg)
			}
		}
		if len(landed) > 0 {
			markAppsExplicit(landed)
		}
	}

	switch {
	case len(missed) > 0 && len(landed) > 0:
		fix := doctorInstallAdvice(missed...)
		if manager == host.Pacman {
			fix = fmt.Sprintf("sudo pacman -S %s", strings.Join(missed, " "))
		}
		return withUnavailablePackages(warnRes(i18n.T("installed %s; %s did not land"), strings.Join(landed, ", "), strings.Join(missed, ", ")).
			withFix(fix), unavailable)
	case len(missed) > 0:
		fix := doctorInstallAdvice(missed...)
		if manager == host.Pacman {
			fix = fmt.Sprintf("sudo pacman -Sy && sudo pacman -S %s", strings.Join(missed, " "))
		}
		return withUnavailablePackages(warnRes(i18n.T("%s could not be installed"), strings.Join(missed, ", ")).
			withFix(fix), unavailable)
	case len(landed) > 0:
		return withUnavailablePackages(fixedRes(i18n.T("installed %s (delete any of them and Ryoku will not reinstall it)"),
			strings.Join(landed, ", ")), unavailable)
	}
	return withUnavailablePackages(fixedRes(i18n.T("took ownership of %d shipped app(s) so an orphan sweep cannot remove them"),
		len(union(plan.adopt, plan.explicit))), unavailable)
}

func union(a, b []string) []string {
	seen := map[string]bool{}
	for _, s := range append(append([]string{}, a...), b...) {
		seen[s] = true
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
