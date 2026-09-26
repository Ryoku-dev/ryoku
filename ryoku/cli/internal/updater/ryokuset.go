package updater

import (
	"bufio"
	"sort"
	"strconv"
	"strings"

	"ryoku-cli/internal/sys"
)

// The Ryoku lane.
//
// `ryoku update` moves the packages Ryoku publishes, and nothing else. The base
// system and its kernel belong to the distribution the box was installed from
// (Arch or CachyOS), and the user takes those with `sudo pacman -Syu`, on their
// own schedule. Two lanes, for reasons that are the whole design:
//
//   - The kernel is not ours to move. Ryoku ships two variants and neither
//     kernel is published by us; a Ryoku release must never decide when a box
//     changes kernel, rebuilds its DKMS modules, or rewrites its boot image.
//   - A release must be reversible. `ryoku rollback` puts the Ryoku set back;
//     it cannot put Arch back, so an update that moved both was never fully
//     reversible in the first place.
//   - The lanes fail independently. A box that cannot take an Arch upgrade
//     today (a mirror out of sync, a held package, a full ESP) must still be
//     able to take a Ryoku fix, and the other way round.
//
// So this file is the only place that decides what `ryoku update` may touch:
// the installed packages the [ryoku] repository serves. Everything else is
// reported, never moved. `ryoku update --system` opts back into one command
// that also runs the user's lane, for people who want it.

// ryokuRepo is the repository name in /etc/pacman.conf. Targets are qualified
// with it ("ryoku/<name>"), so pacman resolves them from our repo even for a
// name that also exists in core/extra, whatever the section order is.
const ryokuRepo = "ryoku"

// externalReleasePkgs are packages the [ryoku] repo builds for a first install
// but that update on their OWN published-release channel afterwards, not through
// the Ryoku package lane. Ryotunes ships prebuilt Arch packages on its GitHub
// releases; `ryoku update` installs those directly (internal/ryotunesrelease,
// upgrade-only, sha256/arch/version-verified). Moving it from the [ryoku] repo
// set would DOWNGRADE a newer external build onto the repo's base version (an
// explicit `-S` moves a package down as well as up), so it is dropped from the
// update set and from the distribution lane's pending list, and tracked through
// its own channel instead. The initial install still comes from the repo (ISO
// pacstrap, ryoku-desktop optdepend) -- only the update path skips it.
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
	repo, err := sys.RunOut("pacman", "-Slq", ryokuRepo)
	if err != nil {
		return nil, 0, err
	}
	installed, err := sys.RunOut("pacman", "-Qq")
	if err != nil {
		return nil, 0, err
	}
	set = ryokuSet(lines(repo), lines(installed))
	if allowDowngrade {
		return set, 0, nil
	}
	kept := dropOlderServes(set)
	return kept, len(set) - len(kept), nil
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

// refreshDBArgs syncs the package databases, and nothing else. It runs BEFORE
// the set is read, so the set is what the repo serves NOW: a rollback onto a
// frozen release must not ask pacman for a package that release never had
// ("target not found" would fail the whole transaction).
//
// force (-Syy) is for a channel move: pacman skips a db that is not newer than
// its cached copy, and a frozen release directory is older than the channel the
// box just left, so a plain -Sy kept the old db against the new signature and
// failed with "invalid or corrupted database (PGP signature)".
func refreshDBArgs(force bool) []string {
	op := "-Sy"
	if force {
		op = "-Syy"
	}
	return []string{"sudo", "pacman", op, "--noconfirm"}
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
// deploy.sh seed unowned (see ryokuOverwriteGlob).
func ryokuInstallArgs(set []string) []string {
	args := []string{"sudo", "env", "SNAP_PAC_SKIP=y", "RYOKU_MANAGED_UPDATE=1",
		"pacman", "-S", "--needed", "--noconfirm", "--overwrite", ryokuOverwriteGlob}
	return append(args, set...)
}

// systemLanePending: what the user's lane would take, after our own -Sy has
// already refreshed the databases, minus the Ryoku set we just moved. `pacman
// -Qu` needs no root and no second sync, unlike checkupdates, which is why
// status uses that and the update run uses this.
func systemLanePending(ryokuTargets []string) []updateItem {
	ours := make(map[string]bool, len(ryokuTargets))
	for _, t := range ryokuTargets {
		ours[strings.TrimPrefix(t, ryokuRepo+"/")] = true
	}
	out, err := sys.RunOut("pacman", "-Qu")
	if err != nil {
		return nil // "no upgrades" is also a non-zero exit; either way, nothing to report
	}
	var ups []updateItem
	for _, l := range lines(out) {
		f := strings.Fields(l)
		if len(f) < 4 || f[2] != "->" || ours[f[0]] || externalReleasePkgs[f[0]] {
			continue
		}
		ups = append(ups, updateItem{Name: f[0], Old: f[1], New: f[3]})
	}
	return ups
}
