package updater

import (
	"bufio"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"
)

// The Ryoku package lane. Arch/CachyOS move only the installed packages served
// by [ryoku], leaving the distribution upgrade to pacman. Void uses XBPS's
// native safe-upgrade transaction with the installed set served by the signed
// Ryoku repository. Package-manager details stay behind the host seam.

// ryokuRepo is the repository name in /etc/pacman.conf. Targets are qualified
// with it ("ryoku/<name>"), so pacman resolves them from our repo even for a
// name that also exists in core/extra, whatever the section order is.
const ryokuRepo = "ryoku"

// externalReleasePkgs update from their own release channel on Arch after
// initial installation. Void serves and updates them through XBPS, so this
// exclusion is applied only to pacman.
var externalReleasePkgs = map[string]bool{"ryotunes": true}

// ryokuSet: the installed packages the [ryoku] repo serves, repo-qualified and
// sorted. Pure over its two inputs, so the selection is unit-testable without
// pacman: repoNames is `pacman -Slq ryoku`, installed is `pacman -Qq`.
func ryokuSet(repoNames, installed []string) []string {
	have := make(map[string]bool, len(installed))
	for _, p := range installed {
		if p = strings.TrimSpace(p); p != "" {
			have[p] = true
		}
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(repoNames))
	for _, p := range repoNames {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] || !have[p] || externalReleasePkgs[p] {
			continue
		}
		seen[p] = true
		out = append(out, ryokuRepo+"/"+p)
	}
	sort.Strings(out)
	return out
}

// installedRyokuSet reads the box: the installed packages the [ryoku] repo
// serves, repo-qualified, and how many of them were held back. An error means
// the question could not be answered (no [ryoku] section, an unsynced db, no
// pacman): the caller must stop rather than fall back to a system upgrade,
// which is the other lane.
//
// allowDowngrade is false for an ordinary `ryoku update`: a package the box
// already carries at a NEWER version than [ryoku] serves is held back and
// counted in skipped. Two shapes make that real: a distro repo (CachyOS,
// extra) ahead of our vendored copy -- re-issuing ryoku/<name> there
// flip-flops the package up and back down inside one run and writes a .pacnew
// every time -- and a split official package (asusctl and rog-control-center)
// whose pinned dep an explicit downgrade would break, failing the whole
// transaction. A channel move and a rollback onto a frozen release pass true:
// there, moving the set DOWN is the point, and the frozen release is the only
// thing the box should keep.
func installedRyokuSet(allowDowngrade bool) (set []string, skipped int, err error) {
	return repoInstalledSet(allowDowngrade)
}

// repoServedSet is the names the currently pointed [ryoku] repo serves, as a
// set. installedRyokuSet already fails the run when the repo cannot be read, so
// an empty set here only ever means the repo genuinely lacks the name.
func repoServedSet() map[string]bool {
	out := map[string]bool{}
	packages, err := host.Default().RepoPackages()
	if err != nil {
		return out
	}
	for _, pkg := range packages {
		out[pkg.Name] = true
	}
	return out
}

// dropOlderServes removes every target whose [ryoku] serve is older than what
// the box has installed. Each name gets two read-only exact-name queries:
// `pacman -Qi` for the installed version and `pacman -Si ryoku/<name>` for the
// repo version, parsed off the Version field. They are per-name on purpose: a
// name the repo shares with a distro repo (asusctl exists in both [ryoku] and
// extra, limine-snapper-sync in both [ryoku] and cachyos) makes an unqualified
// or bulk query ambiguous, and one unresolvable name must not poison the
// answer for every other package. vercmp is pacman's own version ordering, so
// the decision is exactly what pacman would have done. A name either side
// cannot answer for is kept: an update must not silently skip a package
// because a query failed.
func dropOlderServes(set []string) []string {
	var keep []string
	for _, target := range set {
		name := strings.TrimPrefix(target, ryokuRepo+"/")
		inst, err := runPacman("pacman", "-Qi", name)
		if err != nil {
			keep = append(keep, target)
			continue
		}
		repo, err := runPacman("pacman", "-Si", target)
		if err != nil {
			keep = append(keep, target)
			continue
		}
		installedVer := versionField(inst)
		repoVer := versionField(repo)
		if installedVer == "" || repoVer == "" {
			keep = append(keep, target)
			continue
		}
		if vercmp(installedVer, repoVer) > 0 {
			continue // the box is ahead of [ryoku]; an explicit -S would move it back
		}
		keep = append(keep, target)
	}
	sort.Strings(keep)
	return keep
}

// runPacman is the read-only pacman seam, a var so tests pin the hold-back
// decision without a live database.
var runPacman = sys.RunOut

// versionField extracts the Version value from `pacman -Qi`/`-Si` output.
func versionField(out string) string {
	for _, ln := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(ln, ":")
		if ok && strings.TrimSpace(k) == "Version" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// vercmp is pacman's version comparison, a var so tests pin the ordering
// without shelling out.
var vercmp = func(a, b string) int {
	out, err := sys.RunOut("vercmp", a, b)
	if err != nil {
		return 0 // unreadable comparison: keep the target, never skip on a doubt
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0
	}
	return n
}

// lines splits command output into non-empty trimmed lines.
func lines(out string) []string {
	var xs []string
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); l != "" {
			xs = append(xs, l)
		}
	}
	return xs
}

// refreshDBArgs routes package database refresh through the host repository
// seam. The host preserves pacman's -Sy/-Syy behavior and maps Void to XBPS.
func refreshDBArgs(force bool) []string {
	args := []string{"sudo", "ryoku-host", "repo", "sync"}
	if force {
		args = append(args, "--force")
	}
	return args
}

// ryokuInstallArgs installs exactly the set, from our repo.
//
// `-S <targets>`, never `-Su`: a sysupgrade is the user's lane. Explicit
// targets also move a package DOWN, which is what a channel move and a
// rollback onto a frozen release need (`-Su` only ever moves up); the set
// itself excludes older serves on an ordinary update (installedRyokuSet).
// `--needed` leaves a package already at the repo's version alone, so a run
// with nothing to do is a no-op instead of a reinstall.
//
// SNAP_PAC_SKIP=y because `ryoku update` already brackets the run with one
// snapper pre/post pair; --overwrite adopts the paths the installer and
// deploy.sh seed unowned (see RyokuOverwriteGlob).
func ryokuInstallArgs(set []string) []string {
	if updatePackageManager() == host.XBPS {
		args := []string{"sudo", "env", "RYOKU_MANAGED_UPDATE=1", "xbps-install", "-Syu"}
		return append(args, set...)
	}
	args := []string{"sudo", "env", "SNAP_PAC_SKIP=y", "RYOKU_MANAGED_UPDATE=1",
		"pacman", "-S", "--needed", "--noconfirm", "--overwrite", RyokuOverwriteGlob}
	return append(args, set...)
}

// systemLanePending: what the user's lane would take, after our own -Sy has
// already refreshed the databases, minus the Ryoku set we just moved. `pacman
// -Qu` needs no root and no second sync, unlike checkupdates, which is why
// status uses that and the update run uses this.
func systemLanePending(ryokuTargets []string) []updateItem {
	ours := make(map[string]bool, len(ryokuTargets))
	for _, target := range ryokuTargets {
		ours[strings.TrimPrefix(target, ryokuRepo+"/")] = true
	}
	if updatePackageManager() == host.XBPS {
		return xbpsPendingUpdates(ours)
	}
	out, err := sys.RunOut("pacman", "-Qu")
	if err != nil {
		return nil
	}
	var updates []updateItem
	for _, line := range lines(out) {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[2] != "->" || ours[fields[0]] || externalReleasePkgs[fields[0]] {
			continue
		}
		updates = append(updates, updateItem{Name: fields[0], Old: fields[1], New: fields[3]})
	}
	return updates
}

var xbpsPkgverPattern = regexp.MustCompile(`^(.+)-([^-]+_[0-9]+)$`)

func xbpsPendingUpdates(exclude map[string]bool) []updateItem {
	out, err := sys.RunOut("xbps-install", "-Mun")
	if err != nil {
		return nil
	}
	var updates []updateItem
	for _, line := range lines(out) {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[1] != "update" {
			continue
		}
		match := xbpsPkgverPattern.FindStringSubmatch(fields[0])
		if len(match) != 3 || exclude[match[1]] {
			continue
		}
		old, _ := sys.RunOut("xbps-query", "-p", "pkgver", match[1])
		old = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(old), match[1]+"-"))
		updates = append(updates, updateItem{Name: match[1], Old: old, New: match[2]})
	}
	return updates
}
