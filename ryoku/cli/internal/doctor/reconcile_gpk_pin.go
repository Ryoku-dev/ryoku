package doctor

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

var (
	gpkPinnedVersion = func(repo string) string {
		b, err := os.ReadFile(filepath.Join(repo, "release", "packages", "gpk", "PKGBUILD"))
		if err != nil {
			return ""
		}
		for _, line := range strings.Split(string(b), "\n") {
			if v, ok := strings.CutPrefix(strings.TrimSpace(line), "pkgver="); ok {
				return strings.Trim(v, `"'`)
			}
		}
		return ""
	}
	gpkInstalledVersion = func() (host.PackageManager, string) {
		app := host.Default()
		manager, _ := app.PackageManager()
		version, _ := app.PackageVersion("gpk")
		return manager, version
	}
)

// reconcileGpkPin reports, never fixes, a checkout box whose gpk is older than
// the checkout pins. The checkout's Settings and Ryostore drive gpk's event
// stream, which an older gpk lacks, and a checkout box takes its packages from
// a channel that can lag the checkout. A packaged box's gpk is a dependency of
// ryoku-desktop at the version it needs, so it is never checked here.
func reconcileGpkPin(bool) recResult {
	repo := sys.ResolveRepo()
	if repo == "" {
		return okRes(i18n.T("packaged box: gpk comes with ryoku-desktop"))
	}
	pinned := gpkPinnedVersion(repo)
	manager, installed := gpkInstalledVersion()
	have := upstreamVersion(installed)
	switch {
	case pinned == "":
		return okRes(i18n.T("the checkout pins no gpk version"))
	case have == "":
		return warnRes(i18n.Tf("gpk is not installed; this checkout needs %s", pinned)).withFix("%s", gpkPinFix(manager))
	case dottedOlder(have, pinned):
		return warnRes(i18n.Tf("gpk %s is older than the %s this checkout pins; installing and removing apps needs it", have, pinned)).withFix("%s", gpkPinFix(manager))
	}
	return okRes(i18n.Tf("gpk %s", have))
}

func gpkPinFix(manager host.PackageManager) string {
	if manager == host.Pacman {
		return i18n.T("ryoku deploy (builds the pinned gpk package and installs it)")
	}
	return i18n.T("ryoku update once the Ryoku repository serves the pinned gpk")
}

// upstreamVersion drops the epoch and the package release: "1:0.6.10-1",
// "0.6.10-1.fc44" and "0.6.10_1" are all 0.6.10.
func upstreamVersion(version string) string {
	if _, rest, ok := strings.Cut(version, ":"); ok {
		version = rest
	}
	if cut := strings.LastIndexAny(version, "-_"); cut > 0 {
		version = version[:cut]
	}
	return strings.TrimSpace(version)
}

// dottedOlder compares two dotted numeric versions; a part that is not a
// number compares as zero.
func dottedOlder(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := range max(len(pa), len(pb)) {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			return x < y
		}
	}
	return false
}
