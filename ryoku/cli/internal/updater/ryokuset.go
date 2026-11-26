package updater

import (
	"bufio"
	"regexp"
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
// Explicit targets also move a package DOWN, which a channel move and rollback
// need; the set excludes older serves on an ordinary update.
// `--needed` leaves a package already at the repo's version alone, so a run
// with nothing to do is a no-op instead of a reinstall.
//
// SNAP_PAC_SKIP=y because `ryoku update` already brackets the run with one
// snapper pre/post pair; --overwrite adopts the paths the installer and
// deploy.sh seed unowned (see RyokuOverwriteGlob).
func ryokuInstallArgs(set []string) []string {
	switch updatePackageManager() {
	case host.XBPS:
		args := []string{"sudo", "env", "RYOKU_MANAGED_UPDATE=1", "xbps-install", "-Syu"}
		return append(args, set...)
	case host.DNF:
		args := []string{"sudo", "env", "RYOKU_MANAGED_UPDATE=1", host.DNFCommand(), "distro-sync", "-y"}
		return append(args, set...)
	default:
		args := []string{"sudo", "env", "SNAP_PAC_SKIP=y", "RYOKU_MANAGED_UPDATE=1",
			"pacman", "-S", "--needed", "--noconfirm", "--overwrite", RyokuOverwriteGlob}
		return append(args, set...)
	}
}

func targetPackageNames(targets []string) map[string]bool {
	names := make(map[string]bool, len(targets))
	switch updatePackageManager() {
	case host.Pacman:
		for _, target := range targets {
			names[strings.TrimPrefix(target, ryokuRepo+"/")] = true
		}
	case host.DNF:
		packages, _ := repositoryHost().RepoPackages()
		for _, target := range targets {
			for _, pkg := range packages {
				if target == pkg.Name || target == pkg.Name+"-"+pkg.Version {
					names[pkg.Name] = true
					break
				}
			}
		}
	default:
		for _, target := range targets {
			names[target] = true
		}
	}
	return names
}

// systemLanePending: what the user's lane would take, after our own -Sy has
// already refreshed the databases, minus the Ryoku set we just moved. `pacman
// -Qu` needs no root and no second sync, unlike checkupdates, which is why
// status uses that and the update run uses this.
func systemLanePending(ryokuTargets []string) []updateItem {
	ours := targetPackageNames(ryokuTargets)
	switch updatePackageManager() {
	case host.XBPS:
		return xbpsPendingUpdates(ours)
	case host.DNF:
		return dnfPendingUpdates(ours)
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
