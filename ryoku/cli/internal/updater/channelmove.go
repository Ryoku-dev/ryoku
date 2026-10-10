package updater

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"
	i18n "ryoku-i18n"
	wm "ryoku-wm"
)

// The one place that moves the Ryoku set onto a package channel.
//
// Two things always travel together and used to be reimplemented per caller
// (`ryoku track` and the boot guard's revert), which is how #291 happened: the
// boot guard rewrote the pin to the previous release, then ran a narrower
// pacman transaction (`-S ryoku-desktop`) that could not move the compositor
// split metas, so the downgrade failed with the pin already changed and the
// box left with its pin, sync db, and installed set all disagreeing.
//
//   - moveRyokuSetToChannel does the pacman side once: force-refresh the dbs,
//     read the set the target channel serves (downgrades allowed), drop the
//     split metas it does not serve (#271), and move the whole set in one
//     transaction. It runs the pacman commands root-aware and with no progress
//     UI, so it holds from the boot-guard service (no session) exactly as it
//     does from an interactive `ryoku track`.
//   - retargetChannel makes the pin transactional: record the current Server,
//     write the new one, run the move, and on ANY failure put the previous
//     Server back byte-for-byte and re-sync, so the three never drift apart.

// errChannelUnreachable marks a move that failed at the database refresh (no
// network / mirror down) rather than at the transaction. The boot guard retries
// such a boot instead of writing a revert-failed notice.
var errChannelUnreachable = errors.New("the [ryoku] package channel is unreachable")

// privileged runs argv as root: directly when the process already is root (the
// boot-guard service), else through sudo. So the same move works from the guard
// and from a user command, and a test can record the exact argv. Root's sudo
// never prompts, but the guard has no reason to shell through it at all.
var privileged = func(argv ...string) error {
	if os.Geteuid() == 0 {
		return sys.Run(argv[0], argv[1:]...)
	}
	return sys.Sudo(argv...)
}

// ryokuMoveArgs is the one channel-move transaction: move exactly the set from
// our repo, in either direction. `-S <targets>` (never `-Su`) moves a package
// DOWN as well as up, which a channel move and a rollback onto a frozen release
// need; --needed leaves a package already at the channel's version alone.
// SNAP_PAC_SKIP because the interactive path brackets the run with a snapper
// pair and the guard wants no snapshot noise; --overwrite adopts the paths the
// installer and deploy.sh seed unowned (see RyokuOverwriteGlob).
func ryokuMoveArgs(set []string) []string {
	if updatePackageManager() == host.XBPS {
		args := []string{"env", "RYOKU_MANAGED_UPDATE=1", "xbps-install", "-Sfy"}
		return append(args, set...)
	}
	args := []string{"env", "SNAP_PAC_SKIP=y", "RYOKU_MANAGED_UPDATE=1",
		"pacman", "-S", "--needed", "--noconfirm", "--overwrite", RyokuOverwriteGlob}
	return append(args, set...)
}

func prependMissingMoveTargets(set []string, targets ...string) []string {
	present := make(map[string]bool, len(set))
	for _, name := range set {
		present[name] = true
	}
	prefix := make([]string, 0, len(targets))
	for _, name := range targets {
		if !present[name] {
			prefix = append(prefix, name)
			present[name] = true
		}
	}
	return append(prefix, set...)
}
func ryokuTarget(name string) string {
	if updatePackageManager() == host.XBPS {
		return name
	}
	return ryokuRepo + "/" + name
}

// moveRyokuSetToChannel performs the pacman side of a channel move against the
// currently pointed [ryoku] repo. The pin is the caller's job (retargetChannel);
// this only moves packages. Returns the set it moved so the caller can exclude
// it from the system lane.
func moveRyokuSetToChannel() ([]string, error) {
	clearStalePacmanLock()
	if err := syncRepoForMove(true); err != nil {
		return nil, fmt.Errorf("%w: %v", errChannelUnreachable, err)
	}
	set, err := ryokuSetForMove()
	if err != nil {
		return nil, fmt.Errorf(i18n.T("cannot read the [ryoku] repository, so there is nothing safe to move: %w"), err)
	}
	retired := wm.RetiredCompositor()
	if splitMetaInstalled(retired.VariantPackage) {
		name := detectCompositorForMove()
		supported := false
		for _, candidate := range wm.Providers() {
			if candidate == name {
				supported = true
				break
			}
		}
		if !supported {
			name = ""
			for _, candidate := range wm.Providers() {
				if pkgInstalledForMove(ryokuDesktopPkg + "-" + candidate) {
					name = candidate
					break
				}
			}
		}
		if name == "" {
			name = wm.Providers()[0]
		}
		set = prependMissingMoveTargets(set, ryokuTarget(ryokuDesktopPkg+"-"+name))
	}
	// A box migrating onto packages (a checkout retired by `ryoku track`) has
	// no ryoku-desktop installed, and the set above is installed-only: without
	// this the move would run, succeed, and install nothing -- the track
	// message promises the switch installs the base, and a box left with its
	// source lane retired and no packaged base cannot update at all. The
	// umbrella pulls the compositor virtual; naming the active variant makes
	// the choice deterministic on a box that has no provider installed yet
	// (the same pair the ISO installer lays, lib/deploy.sh).
	if !pkgInstalledForMove(ryokuDesktopPkg) {
		name := detectCompositorForMove()
		if name == "" {
			name = wm.Providers()[0] // the shipped default variant
		}
		set = prependMissingMoveTargets(set,
			ryokuTarget(ryokuDesktopPkg),
			ryokuTarget(ryokuDesktopPkg+"-"+name),
		)
	}
	// A channel that predates the compositor split carries no
	// ryoku-desktop-hyprland/niri; their exact-version pins would fail the
	// whole transaction, so drop the installed metas the target does not serve
	// before building it (#271, #291). Record them: -Rdd is a separate,
	// committed step, so a transaction that then fails must be able to put them
	// back from the restored channel, which does serve them.
	lastDroppedMetas = dropSplitMetasNotServed(servedSetForMove())
	if len(lastDroppedMetas) > 0 {
		progress.logf(i18n.T("Removed %s: the target channel does not serve it"), strings.Join(lastDroppedMetas, ", "))
	}
	if len(set) == 0 {
		return set, nil
	}
	if err := runRyokuMove(set); err != nil {
		return set, err
	}
	return set, nil
}

// The read-only queries the move makes, as seams: they hit the live database
// and the running session, so a test pins them without either.
const ryokuDesktopPkg = "ryoku-desktop"

var (
	syncRepoForMove = repoSync
	ryokuSetForMove = func() ([]string, error) {
		set, _, err := installedRyokuSet(true)
		return set, err
	}
	servedSetForMove    = repoServedSet
	pkgInstalledForMove = func(name string) bool {
		return host.Default().Package([]string{"installed", name}) == host.ExitOK
	}
	detectCompositorForMove = func() string { return wm.Detect().Name }
)

// lastDroppedMetas holds the split metas the most recent move removed with -Rdd.
// The move is single-threaded, so a package var is enough; retargetChannel reads
// it to put them back when the transaction that followed the drop failed.
var lastDroppedMetas []string

// runRyokuMove executes the one channel-move transaction, sleep-inhibited so a
// lid-close or idle suspend mid-transaction cannot corrupt it, and root-aware so
// it holds from the boot guard (which is already root) as well as an interactive
// `ryoku track` (which escalates). Inside an update the console curates its
// output; from `ryoku track` and the boot service it streams as it is. A var
// so a test records the set without a live pacman.
var runRyokuMove = func(set []string) error {
	argv := ryokuMoveArgs(set)
	if os.Geteuid() != 0 {
		argv = append([]string{"sudo"}, argv...)
	}
	return runInhibited(i18n.T("Ryoku channel move"), argv)
}

// retargetChannel makes a channel move transactional: record the current
// [ryoku] Server, pin the new channel, run move, and on any failure put the
// previous Server back exactly and re-sync so the pin, the sync db, and the
// installed set never disagree (the #291 invariant). Used by both `ryoku track`
// and the boot guard's revert.
func retargetChannel(channel string, move func() error) error {
	prev := packagedRepoURL()
	lastDroppedMetas = nil
	if err := repoSetChannel(channel); err != nil {
		return err
	}
	if err := move(); err != nil {
		// The pin change is undone, then the split metas the move dropped are put
		// back from the restored channel (which serves them) -- without this a
		// failed downgrade would strand a box with those metas gone for good,
		// since the ordinary update only ever moves installed AND served names.
		restorePreviousServer(prev)
		reinstallDroppedMetas()
		return err
	}
	return nil
}

// restorePreviousServer undoes a failed channel move: put the recorded Server
// line back byte-for-byte and re-sync so the cached db matches it again. Best
// effort -- offline it cannot re-sync, but the pin is restored and the dropped
// db simply refetches on the next online -Sy, so nothing is left disagreeing.
func restorePreviousServer(prev string) {
	if prev == "" {
		return
	}
	_ = repoSetURL(prev)
	_ = syncRepoForMove(true)
}

// reinstallDroppedMetas puts the split metas a failed move removed back from the
// (now restored) channel, which serves them. It runs after restorePreviousServer
// so the pin and its db already point at the channel that has them. Best effort
// and one-shot: it clears the record so a later successful move never re-adds a
// meta the target legitimately dropped.
func reinstallDroppedMetas() {
	if len(lastDroppedMetas) == 0 {
		return
	}
	targets := make([]string, 0, len(lastDroppedMetas))
	for _, name := range lastDroppedMetas {
		targets = append(targets, ryokuTarget(name))
	}
	if err := privileged(ryokuMoveArgs(targets)...); err != nil {
		progress.logf(i18n.T("could not reinstall %s after the failed move; run `ryoku update`: %v"), strings.Join(lastDroppedMetas, ", "), err)
	}
	lastDroppedMetas = nil
}

// RetargetChannel is the exported transactional move the doctor uses to heal a
// box whose pin drifted onto a stale release (reconcile_channel_pin.go).
func RetargetChannel(channel string) error {
	return retargetChannel(channel, func() error {
		_, err := moveRyokuSetToChannel()
		return err
	})
}
