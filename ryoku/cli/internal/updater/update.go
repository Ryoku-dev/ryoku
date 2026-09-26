package updater

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/godbus/dbus/v5"
	"os"
	"os/exec"
	"path/filepath"
	"ryoku-cli/internal/sys"
	i18n "ryoku-i18n"
	wm "ryoku-wm"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"
)

const snapperConfig = "root"

// gitSteps / pkgSteps are the ordered stages the GUI renders as a determinate
// multi-segment bar. The git-channel path (a dev/mirror checkout) and the
// packaged path (pacman) run different stages; stage2 re-begins pkgSteps and
// marks the pre-handoff steps done so the exec handoff keeps one continuous bar.
var (
	gitSteps = []runStep{
		{Key: "snapshot", Label: i18n.T("Taking a snapshot")},
		{Key: "channel", Label: i18n.T("Pulling the latest commits")},
		{Key: "deploy", Label: i18n.T("Deploying the desktop")},
		{Key: "doctor", Label: i18n.T("Healing the system")},
		{Key: "finalize", Label: i18n.T("Finishing up")},
	}
	// The Ryoku lane, the default: only the packages Ryoku publishes move (see
	// ryokuset.go). The base system, its kernel, the AUR and Flatpak are the
	// user's lane, reported at the end and taken with `sudo pacman -Syu`.
	pkgSteps = []runStep{
		{Key: "snapshot", Label: i18n.T("Taking a snapshot")},
		{Key: "packages", Label: i18n.T("Updating the Ryoku packages")},
		{Key: "apply", Label: i18n.T("Applying the new configuration")},
		{Key: "reload", Label: i18n.T("Reloading the desktop")},
		{Key: "doctor", Label: i18n.T("Healing the system")},
		{Key: "finalize", Label: i18n.T("Finishing up")},
	}
	// `ryoku update --system`: the same run plus the user's lane, for a box
	// that wants one command. The extra steps sit where they used to.
	systemSteps = []runStep{
		{Key: "snapshot", Label: i18n.T("Taking a snapshot")},
		{Key: "packages", Label: i18n.T("Updating the Ryoku packages")},
		{Key: "system", Label: i18n.T("Updating the system packages")},
		{Key: "aur", Label: i18n.T("Updating AUR packages")},
		{Key: "flatpak", Label: i18n.T("Updating Flatpak apps")},
		{Key: "apply", Label: i18n.T("Applying the new configuration")},
		{Key: "reload", Label: i18n.T("Reloading the desktop")},
		{Key: "doctor", Label: i18n.T("Healing the system")},
		{Key: "finalize", Label: i18n.T("Finishing up")},
	}
)

// Update = the whole safe update, wrapped in a snapper pre/post pair.
// checkout box -> git channel (fast-forward + redeploy). packaged box ->
// the Ryoku packages, then hand off to the binary pacman just installed
// (--stage2) so the deploy and doctor semantics of the new release apply
// during this same update, not one release late. stage2 quiesces the shell,
// materializes, brings the desktop back, and runs `ryoku doctor` (same one
// users run by hand) to heal stateful drift, then the post snapshot.
// snapshots are best-effort: an unconfigured snapper never blocks an update,
// but a failed step still aborts first. Each stage is published to the
// run-state file so the update island and Hub show real, determinate progress.
//
// It updates the Ryoku set only (ryokuset.go). The base system and its kernel
// come from Arch or CachyOS, whichever the box installed, and stay the user's
// to take with `sudo pacman -Syu`; the run reports what is waiting there.
// `--system` runs that lane too, in one command.
func Update(args []string) error {
	stage2 := len(args) >= 2 && args[0] == "--stage2"
	channelSwitch := false
	withSystem := false
	for _, a := range args {
		if a == "-v" || a == "--verbose" {
			verboseLog = true
		}
		if a == "--channel-switch" {
			channelSwitch = true
		}
		if a == "--system" || a == "--full" {
			withSystem = true
		}
	}

	// One update at a time: a second run mid-transaction (a double-click, a timer
	// racing a manual update) can corrupt pacman or the config swap. Best-effort
	// -- a lock we cannot even create never blocks an update, only a held one does.
	lock, busy := acquireUpdateLock()
	if busy != nil {
		return busy
	}
	if lock != nil {
		defer lock.Close()
	}

	if stage2 {
		return updateStage2(args[1], withSystem)
	}

	// Stop before starting if the disk is too full: a pacman transaction or a
	// copy-on-write snapshot that runs out of space leaves the system half-upgraded.
	if free, ok := enoughFreeSpace(); !ok {
		return fmt.Errorf(i18n.T("only %s free on /; free up space before updating "+
			"(an update that runs out of disk can leave the system half-upgraded)"), free)
	}

	// Cache the sudo credential once, on the terminal, before any step needs it.
	// The pre-snapshot, pacman, yay and the post snapshot all escalate, several
	// through pipes or RunOut where a prompt cannot be seen; one prompt up front is
	// what users were doing by hand with `sudo -v && ryoku update`.
	primeSudo()
	stopKeepalive := sudoKeepalive()
	defer stopKeepalive()

	checkout := sys.ResolveRepo() != ""
	switch {
	case checkout:
		progress.begin(gitSteps)
	case withSystem:
		progress.begin(systemSteps)
	default:
		progress.begin(pkgSteps)
	}

	progress.at("snapshot")
	pre := snapperPre(snapshotDesc())
	progress.setSnapshot(pre)

	// checkout: update through the git channel. packaged: pacman + a hand-off
	// to the freshly installed binary (stage2).
	if checkout {
		logPath := startUpdateLog()
		defer stopUpdateLog()
		if err := channelUpdate(); err != nil {
			progress.fail(err)
			return err
		}
		rashinReindex()
		prowlRefresh()
		upgradeRyotunes()
		progress.at("doctor")
		offerSnapperHelpers()
		runFreshDoctor()
		progress.at("finalize")
		snapperPost(pre, "ryoku-update")
		progress.logf(i18n.T("Update complete"))
		if logPath != "" {
			fmt.Println("  " + sys.Dim(i18n.T("full log: ")+logPath))
		}
		return finishRun()
	}

	progress.at("packages")
	// the release this box runs before pacman moves it; stage2 (the new
	// binary) reads it from the environment to arm the boot guard.
	if from := sys.ReadRelease().Release; from != "" {
		os.Setenv("RYOKU_UPDATE_FROM", from)
	}
	clearStalePacmanLock()
	// Refresh first, then read the set: a rollback onto a frozen release must
	// only ask for packages that release actually served. An ordinary update
	// drops names the box already carries at a newer version than [ryoku]
	// serves (a distro repo ahead of our vendored copy, a split package whose
	// pinned dep a forced downgrade would break); a channel move keeps them,
	// since moving the set down is the point there.
	if err := sys.Sudo(refreshDBArgs(channelSwitch)[1:]...); err != nil {
		progress.logf(i18n.T("Could not refresh the package databases: %v"), err)
	}
	set, held, err := installedRyokuSet(channelSwitch)
	if err != nil {
		e := fmt.Errorf(i18n.T("cannot read the [ryoku] repository, so there is nothing safe to update: %w"), err)
		progress.fail(e)
		return e
	}
	if channelSwitch {
		// A rollback onto a release that predates the compositor split carries
		// no ryoku-desktop-hyprland/niri; their exact pins would fail the whole
		// downgrade transaction (#271). Drop them before building the target.
		if dropped := dropSplitMetasNotServed(repoServedSet()); len(dropped) > 0 {
			progress.logf(i18n.T("Removed %s: the target release predates the compositor split"), strings.Join(dropped, ", "))
		}
	}
	if err != nil {
		e := fmt.Errorf(i18n.T("cannot read the [ryoku] repository, so there is nothing safe to update: %w"), err)
		progress.fail(e)
		return e
	}
	switch {
	case len(set) == 0 && held == 0:
		e := fmt.Errorf(i18n.T("no packages from the [ryoku] repository are installed; `ryoku doctor` checks the repo setup"))
		progress.fail(e)
		return e
	case len(set) == 0:
		// Every served package is already newer here than [ryoku] publishes
		// (a distro repo ahead of our vendored copies). Forcing them down
		// would flip-flop versions and churn .pacnew files every run, so the
		// Ryoku lane has nothing to do; the rest of the update still runs.
		progress.logf(i18n.T("%d Ryoku package(s) are already newer than the [ryoku] repository serves; leaving them as they are"), held)
	default:
		if held > 0 {
			progress.logf(i18n.T("Updating %d Ryoku package(s); %d already newer than the repository serves stay as they are"), len(set), held)
		} else {
			progress.logf(i18n.T("Updating %d Ryoku package(s); the base system stays as it is"), len(set))
		}
		if conflicts, err := runRyokuUpgrade(set); err != nil {
			healPackageUpgrade(conflicts, err)
			// One in-place recovery, then a single retry: clear the unowned files a
			// new package now claims (an installer/deploy stray), or, with nothing to
			// clear, drop a stale [ryoku] db whose signature no longer matches and
			// refresh it clean.
			healPackageUpgrade(conflicts, err)
			if _, err = runRyokuUpgrade(set); err != nil {
				// only advertise `ryoku rollback` when the pre snapshot it needs exists;
				// snapperPre is best-effort and returns "" when it was skipped.
				hint := i18n.T("no pre-update snapshot exists (snapper was unavailable), so `ryoku rollback` cannot revert this; recover with pacman directly")
				if pre != "" {
					hint = i18n.T("see `ryoku rollback` (pre-update snapshot ") + pre + ")"
				}
				e := fmt.Errorf(i18n.T("the Ryoku package upgrade failed; %s: %w"), hint, err)
				progress.fail(e)
				return e
			}
		}
	}

	// The user's lane. Off by default: the kernel and the base come from Arch
	// or CachyOS on the user's schedule, and this run must not decide that for
	// them. Reported either way, so nobody has to guess whether something is
	// waiting; `--system` takes it here instead.
	pendingSystem := systemLanePending(set)
	if withSystem {
		runSystemLane(pendingSystem)
	} else {
		reportSystemLane(pendingSystem)
	}

	// exec replaces this process with the freshly installed binary; on any
	// failure fall through and finish in-process, exactly as before.
	if sys.Exists("/usr/bin/ryoku") {
		hand := []string{"ryoku", "update", "--stage2", pre}
		if withSystem {
			hand = append(hand, "--system")
		}
		if err := syscall.Exec("/usr/bin/ryoku", hand, os.Environ()); err != nil {
			fmt.Fprintf(os.Stderr, i18n.T("warning: could not hand off to the updated binary: %v\n"), err)
		}
	}
	return updateStage2(pre, withSystem)
}

// --- update safeguards (concurrency, disk, sleep, snapshot noise) -----------

// acquireUpdateLock takes an exclusive, non-blocking flock so only one
// `ryoku update` runs at a time. Returns (nil, err) when another update already
// holds it (the caller aborts); (nil, nil) when the lock file cannot even be
// created (proceed best-effort, like the snapshot); (f, nil) when acquired -- the
// caller keeps f open for the update's lifetime and closes it to release. The fd
// is close-on-exec, so the stage1->stage2 handoff re-acquires cleanly.
func acquireUpdateLock() (*os.File, error) {
	f, err := os.OpenFile(filepath.Join(sys.Xdg("XDG_RUNTIME_DIR", ".cache"), "ryoku-update.lock"),
		os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, nil // cannot create a lock file -> never block the update
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf(i18n.T("another ryoku update is already running; wait for it to finish"))
	}
	return f, nil
}

// enoughFreeSpace reports whether / has room for an update (the download, the new
// packages, and a copy-on-write snapshot). A statfs it cannot read never blocks.
// The returned string is the human-readable free size, for the error message.
func enoughFreeSpace() (string, bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs("/", &st); err != nil {
		return "", true
	}
	free := st.Bavail * uint64(st.Bsize)
	return humanBytes(free), free >= (1 << 30) // 1 GiB floor
}

func humanBytes(n uint64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GiB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%d MiB", n>>20)
	default:
		return fmt.Sprintf("%d bytes", n)
	}
}

// primeSudo caches the sudo credential once, up front, on the real terminal, so
// every escalation the rest of the update makes finds it instead of prompting.
// Several of those prompts cannot be seen or answered: the pre-snapshot runs
// through RunOut (no tty), pacman's runs through the curated output pipe, and yay
// and flatpak escalate on their own -- an unseen prompt there is exactly why users
// learned to run `sudo -v` by hand first. No tty -> skip (a GUI or timer run has
// no terminal to prompt on); a NOPASSWD box sees nothing. Best-effort.
func primeSudo() {
	if !sys.StdinIsTTY() {
		return
	}
	_ = sys.Run("sudo", "-v")
}

// sudoKeepalive refreshes the cached credential every minute so a long
// transaction (a large AUR compile) cannot let it lapse mid-run and re-prompt
// where the prompt is invisible. `-n` never prompts, so once the credential is
// gone this is a silent no-op, and RunOut keeps its "a password is required" off
// the terminal. The returned stop func ends the refresher; the stage1->stage2
// exec replaces the process, so a stop it never reaches leaks nothing.
func sudoKeepalive() func() {
	if !sys.StdinIsTTY() {
		return func() {}
	}
	stop := make(chan struct{})
	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				_, _ = sys.RunOut("sudo", "-n", "-v")
			}
		}
	}()
	return func() { close(stop) }
}

// snapshotDesc labels the pre-update snapshot (and its Limine boot-menu entry)
// with the version being updated from, so a user restoring after a bad update can
// tell the snapshots apart instead of a row of identical "ryoku-update".
func snapshotDesc() string {
	v := ""
	if repo := sys.ResolveRepo(); repo != "" {
		v = gitShort(repo, "HEAD")
	} else {
		v = shortCommit(sys.InstalledVersion())
	}
	if v == "" {
		return "ryoku-update"
	}
	return "ryoku-update (from " + v + ")"
}

// runRyokuUpgrade moves the Ryoku set, sleep-inhibited (a lid-close or idle
// suspend mid-transaction cannot corrupt it). It returns any "exists in
// filesystem" conflict paths so a failed run can clear unowned strays and
// retry. The argv, and why it is not a sysupgrade, live in ryokuset.go.
func runRyokuUpgrade(set []string) ([]string, error) {
	return runUpgradeCollecting("Ryoku", "Ryoku package upgrade", ryokuInstallArgs(set))
}

// runSystemLane is `ryoku update --system`: the user's lane, opted into. It is
// the full sysupgrade, so it can move the kernel; that is the point of asking
// for it. Failures warn and continue, like the AUR and Flatpak steps: the
// Ryoku set is already in, and stage2 still has to bring the desktop back.
func runSystemLane(pending []updateItem) {
	progress.at("system")
	progress.logf(i18n.T("Updating %d system package(s) (pacman -Syu, kernel included)"), len(pending))
	if err := runInhibited(i18n.T("System"), "System package upgrade", systemUpgradeArgs()); err != nil {
		fmt.Fprintf(os.Stderr, i18n.T("warning: the system upgrade reported errors: %v\n"), err)
	}
	if sys.Has("yay") {
		progress.at("aur")
		progress.logf(i18n.T("Updating AUR packages (yay)"))
		if err := runAURUpgrade(); err != nil {
			fmt.Fprintf(os.Stderr, i18n.T("warning: yay update reported errors: %v\n"), err)
		}
	} else {
		progress.skip("aur")
	}
	// Flatpak apps are a separate channel from pacman and the AUR. Skipped when
	// there is nothing to update: an offline box, or one with the client but no
	// remote, must not turn a whole update red.
	if flatpakUpdatable() {
		progress.at("flatpak")
		progress.logf(i18n.T("Updating Flatpak apps"))
		if err := runFlatpakUpgrade(); err != nil {
			fmt.Fprintf(os.Stderr, i18n.T("warning: flatpak update reported errors: %v\n"), err)
		}
	} else {
		progress.skip("flatpak")
	}
}

// reportSystemLane names what the other lane is holding and the one command
// that takes it. Silence here would read as "everything is up to date", which
// is the whole failure this split has to avoid.
func reportSystemLane(pending []updateItem) {
	if len(pending) == 0 {
		progress.logf(i18n.T("The base system is current; nothing waiting outside the Ryoku set"))
		return
	}
	progress.logf(i18n.T("%d system package(s) waiting from your distribution (kernel included): take them with `sudo pacman -Syu`"), len(pending))
}

// healPackageUpgrade recovers from a failed Ryoku upgrade in place, once. Files
// that block the transaction and that no package owns ("exists in filesystem"
// for an installer/deploy stray a new package now claims) are removed so the
// package adopts them; a file another package owns is a real conflict and is
// left untouched for the retry to surface. With nothing to clear, a stale
// [ryoku] db whose signature no longer matches is dropped for a clean refresh,
// but ONLY when the failure actually names the database: dropping it after a
// dependency or conflict failure destroys the evidence and turns the next run
// into a "target not found" storm (#271).
func healPackageUpgrade(conflicts []string, err error) {
	if strays := unownedFiles(conflicts); len(strays) > 0 {
		progress.logf(i18n.T("Clearing %d unowned file(s) blocking the upgrade, then retrying"), len(strays))
		_ = sys.Sudo(append([]string{"rm", "-f"}, strays...)...)
		return
	}
	if !dbRejection(err) {
		progress.logf(i18n.T("The Ryoku transaction failed for a reason that is not the package database; keeping the [ryoku] db so the error stays readable"))
		return
	}
	progress.logf(i18n.T("Package database rejected; dropping the stale [ryoku] db and retrying"))
	_ = sys.DropRyokuSyncDB()
}

// dbRejection reports whether a pacman failure is a database/signature
// rejection rather than a dependency, conflict or download error.
func dbRejection(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, needle := range []string{"invalid or corrupted", "signature", "database", "could not read db"} {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}

// splitMetapackages are the compositor-split metas the 0.73.x packaging
// introduced. A release that predates the split does not serve them, and their
// exact-version pins on six packages make any downgrade transaction
// unsatisfiable, so moving across the split has to drop them first; the
// downgrade of ryoku-desktop itself re-ties the graph afterwards (#271).
var splitMetapackages = []string{"ryoku-desktop-hyprland", "ryoku-desktop-niri"}

// Seams over the live box, replaced in tests.
var (
	splitMetaInstalled = func(name string) bool { return sys.PkgInstalled(name) }
	splitMetaRemove    = func(name string) error {
		return sys.Sudo("pacman", "-Rdd", "--noconfirm", name)
	}
)

// dropSplitMetasNotServed removes installed split metapackages the currently
// pointed [ryoku] repo does not serve. -Rdd: the meta's own dependencies are
// exactly what the transaction is about to move, and pacman's walk cannot see
// past the meta's exact pins. Returns what it removed, for the log.
func dropSplitMetasNotServed(served map[string]bool) []string {
	var dropped []string
	for _, meta := range splitMetapackages {
		if !splitMetaInstalled(meta) || served[meta] {
			continue
		}
		if err := splitMetaRemove(meta); err != nil {
			progress.logf(i18n.T("could not remove %s, which the target release does not serve; the downgrade may fail on its exact pins (%v)"), meta, err)
			continue
		}
		dropped = append(dropped, meta)
	}
	return dropped
}

// unownedFiles keeps only the paths no installed package owns: pacman -Qo fails
// on a stray, and removing a file a package ships would break that package.
func unownedFiles(paths []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range paths {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		if _, err := sys.RunOut("pacman", "-Qo", p); err != nil {
			out = append(out, p)
		}
	}
	return out
}

// ryokuOverwriteGlob names the ryoku-desktop-owned paths that the ISO installer
// and ryoku/shell/deploy.sh seed unowned before the package began owning them:
// the privileged helpers (ryoku-dns, ryoku-network-kill, ryoku-boot-apply,
// ryoku-wifi-powersave), their polkit rules, the Plymouth splash theme, the
// ryoku-owned systemd units, the mkinitcpio install hooks the HOOKS drop-in
// names, the shipped boot configs under /usr/share/ryoku/boot, and the logind
// lid-switch drop-in. Every ryoku-desktop (re)install --overwrites these, or the
// first upgrade that starts owning a seeded path aborts the whole transaction
// ("exists in filesystem") and blocks every update until the files are removed
// by hand. Keep in sync with the doctor's ryokuSystemGlobs, which clears the
// same paths on an already-wedged box.
const ryokuOverwriteGlob = "/usr/bin/ryoku-*," +
	"/usr/lib/systemd/system/ryoku-*," +
	"/usr/lib/initcpio/install/ryoku-*," +
	"/usr/share/polkit-1/rules.d/*ryoku*.rules," +
	"/usr/share/plymouth/themes/ryoku/*," +
	"/usr/share/ryoku/boot/*," +
	"/etc/systemd/logind.conf.d/10-ryoku-lid.conf"

// systemUpgradeArgs is the user's lane, run only by `ryoku update --system`:
// the full sysupgrade, kernel included, exactly what `sudo pacman -Syu` does
// by hand. It keeps the --overwrite glob so a seeded path a Ryoku package now
// owns cannot abort the transaction here either.
func systemUpgradeArgs() []string {
	return []string{"sudo", "env", "SNAP_PAC_SKIP=y", "RYOKU_MANAGED_UPDATE=1",
		"pacman", "-Syu", "--noconfirm", "--overwrite", ryokuOverwriteGlob}
}

// channelSwitchArgs installs the [ryoku] channel's ryoku-desktop explicitly,
// which pacman honours in either direction (a downgrade warns and proceeds),
// pulling the umbrella's exact-version depends with it.
func channelSwitchArgs() []string {
	return []string{"sudo", "env", "SNAP_PAC_SKIP=y", "RYOKU_MANAGED_UPDATE=1",
		"pacman", "-S", "--noconfirm", "--overwrite", ryokuOverwriteGlob, "ryoku-desktop"}
}

// runAURUpgrade runs `yay -Sua` under the same sleep inhibitor.
func runAURUpgrade() error {
	return runInhibited("AUR", "AUR package upgrade", []string{"yay", "-Sua", "--noconfirm"})
}

// flatpakUpdatable reports whether a flatpak update is worth attempting at all:
// the client is present and at least one remote is configured. A fresh offline
// install has the client and no remote (doctor adds it once there is a network),
// and `flatpak update` there would only print a confusing error into the middle
// of an otherwise clean run.
func flatpakUpdatable() bool {
	if !sys.Has("flatpak") {
		return false
	}
	out, err := sys.RunOut("flatpak", "remotes", "--columns=name")
	return err == nil && strings.TrimSpace(out) != ""
}

// runFlatpakUpgrade updates every installed flatpak app and runtime under the
// same sleep inhibitor as the package steps, since a suspend mid-deploy leaves a
// half-written app tree.
func runFlatpakUpgrade() error {
	return runInhibited("Flatpak", "Flatpak app upgrade",
		[]string{"flatpak", "update", "--noninteractive", "--assumeyes"})
}

// runInhibited runs argv while holding a logind sleep+idle block, so a suspend
// mid-upgrade cannot interrupt a package transaction. Degrades to running argv
// directly when systemd-inhibit is unavailable. On a real terminal it renders a
// curated view of the output (phase is the header label); for pipes, logs, and
// --verbose it streams raw so nothing that scrapes the output breaks.
func runInhibited(phase, why string, argv []string) error {
	full := argv
	if sys.Has("systemd-inhibit") {
		head := []string{"systemd-inhibit", "--what=sleep:idle", "--who=ryoku update", "--why=" + why, "--mode=block"}
		full = append(head, argv...)
	}
	if verboseLog || !sys.StdoutIsTTY() {
		return sys.Run(full[0], full[1:]...)
	}
	return renderUpgrade(phase, full)
}

// finishRun publishes the terminal "done" state, holds it briefly so a watching
// GUI catches the completion, then clears the run so the island folds away.
func finishRun() error {
	progress.finish()
	time.Sleep(1200 * time.Millisecond)
	progress.idle()
	return nil
}

// updateStage2 finishes an update after the package transactions: deploy the
// new configs with the shell quiesced, bring the desktop back, heal drift. It
// runs in the freshly installed binary (a new process after the exec handoff),
// so it re-begins the packaged step list and marks the pre-handoff steps done
// to keep one continuous progress bar.
func updateStage2(pre string, withSystem bool) error {
	// A fresh process after the exec handoff: re-cache the credential (a no-op
	// inside the timeout) so materialize, the post snapshot and doctor never
	// prompt where it cannot be seen.
	primeSudo()
	stopKeepalive := sudoKeepalive()
	defer stopKeepalive()
	// Re-begin the SAME step list stage1 published, or the island would lose
	// the steps that already ran when a --system run hands over.
	steps := pkgSteps
	if withSystem {
		steps = systemSteps
	}
	progress.begin(steps)
	progress.setSnapshot(pre)
	progress.markDone("snapshot", "packages")
	if withSystem {
		progress.markDone("system")
		if sys.Has("yay") {
			progress.markDone("aur")
		} else {
			progress.skip("aur")
		}
		if flatpakUpdatable() {
			progress.markDone("flatpak")
		} else {
			progress.skip("flatpak")
		}
	}

	// The packages are in. From here the boot guard watches: if the next two
	// boots never bring the desktop up, it puts the Ryoku set back on the
	// release this box ran before. Armed only for a real move (the release
	// changed) so a no-op update never leaves a marker behind.
	armBootGuard(pre)

	progress.at("apply")
	progress.logf(i18n.T("Applying the new configuration"))
	if err := ensurePackagePowerCutover(); err != nil {
		progress.fail(err)
		return err
	}
	graphicalPresent, shellExpected, err := graphicalSessionState()
	if err != nil {
		progress.fail(err)
		return err
	}
	wallpaperWasActive := shellExpected &&
		exec.Command("systemctl", "--user", "is-active", "--quiet", "ryogami.service").Run() == nil
	powerCutoverLock, err := acquirePowerCutoverLock()
	if err != nil {
		progress.fail(err)
		return err
	}
	defer func() {
		if powerCutoverLock != nil {
			_ = powerCutoverLock.Close()
		}
	}()
	cutoverGuard, err := acquireUpdateSleepGuard()
	if err != nil {
		progress.fail(err)
		return err
	}
	defer cutoverGuard.Disconnect()
	// Activate logind's sessionless fallback while every old owner and the
	// durable block are still live. Only after that succeeds may the old
	// direct-suspend hooks and clamshell process be quiesced.
	if err := reloadLogindLidPolicy(); err != nil {
		progress.fail(err)
		return err
	}
	if err := preparePowerPolicyUpdate(); err != nil {
		progress.fail(err)
		return err
	}
	if graphicalPresent {
		// Stop admission before quiescing the old daemon. New launchers queue at
		// this boundary; already-loaded wrappers can still use the old daemon to
		// complete their ordered unlock.
		if err := runPowerHelper("ryoku-power-cutover", "generation-guard-start"); err != nil {
			progress.fail(err)
			return err
		}
	}
	// stop the shell first: a live quickshell would hot-reload the half-copied
	// tree mid-swap, re-instantiating the new QML against whatever plugin .so
	// the old process still has mapped. pause Hyprland's Lua auto-reload for
	// the same reason (= emergency overlay popping up with no keybinds).
	if err := stopShell(); err != nil {
		progress.fail(err)
		return err
	}
	if graphicalPresent {
		// The first rollout can encounter a pre-guard wrapper. With its launcher
		// now quiesced, drain that final legacy client before replacing files.
		if err := runPowerHelper("ryoku-power-cutover", "wait-lock-clients"); err != nil {
			progress.fail(err)
			return err
		}
	}
	pauseConfigAutoreload()
	if err := Materialize(); err != nil {
		if shellExpected {
			_ = reloadConfig()
			_ = startShell()
			if ready, readyErr := waitForShellReady(true); readyErr == nil && ready {
				if activateErr := activatePowerPolicy(); activateErr != nil {
					fmt.Fprintf(os.Stderr, i18n.Tf("warning: could not restore the lid policy: %v\n", activateErr))
				}
			} else if readyErr != nil {
				fmt.Fprintf(os.Stderr, i18n.Tf("warning: could not restore the shell sleep guard: %v\n", readyErr))
			}
			restartWallpaper(false)
		}
		progress.fail(err)
		return err
	}
	if err := regenerateConfig(); err != nil {
		progress.logf(i18n.Tf("could not re-author the compositor settings: %v", err))
	}
	if graphicalPresent {
		if err := runPowerHelper("ryoku-power-cutover", "session-bind"); err != nil {
			if shellExpected {
				_ = reloadConfig()
				_ = startShell()
				restartWallpaper(wallpaperWasActive)
			}
			progress.fail(err)
			return err
		}
	}

	if shellExpected {
		progress.at("reload")
		progress.logf(i18n.T("Reloading the desktop"))
		// One clean reload picks up the new config and restores auto-reload. The
		// lid owner comes back as soon as the new shell answers sleep-ready;
		// long-running reindex, doctor, and snapshot work starts only afterward.
		if err := reloadConfig(); err != nil {
			restartWallpaper(wallpaperWasActive)
			progress.fail(err)
			return fmt.Errorf("reload compositor power bindings: %w", err)
		}
		if err := startShell(); err != nil {
			restartWallpaper(wallpaperWasActive)
			progress.fail(err)
			return err
		}
		ready, err := waitForShellReady(true)
		if err != nil {
			restartWallpaper(wallpaperWasActive)
			progress.fail(err)
			return err
		}
		if ready {
			if err := activatePowerPolicy(); err != nil {
				restartWallpaper(wallpaperWasActive)
				progress.fail(err)
				return err
			}
		}
		restartWallpaper(wallpaperWasActive)
	}
	// Either the foreground shell owns the normal sleep block, or every
	// inactive graphical session is qylock-secured and observed while logind
	// owns the sessionless lid fallback. Release only after proving that state.
	if shellExpected {
		if err := runPowerHelper("ryoku-power-cutover", "session-check"); err != nil {
			progress.fail(err)
			return err
		}
	}
	if graphicalPresent {
		if err := runPowerHelper("ryoku-power-cutover", "generation-guard-stop"); err != nil {
			progress.fail(err)
			return err
		}
	}
	if err := cutoverGuard.Release(); err != nil {
		progress.fail(err)
		return err
	}
	_ = powerCutoverLock.Close()
	powerCutoverLock = nil
	rashinReindex()
	prowlRefresh()
	upgradeRyotunes()

	progress.at("doctor")
	offerSnapperHelpers()
	runFreshDoctor()

	progress.at("finalize")
	snapperPost(pre, "ryoku-update")
	progress.logf(i18n.T("Update complete"))
	return finishRun()
}

func acquirePowerCutoverLock() (*os.File, error) {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	lock, err := os.OpenFile(filepath.Join(dir, "ryoku-power-cutover.lock"),
		os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open power cutover lock: %w", err)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		_ = lock.Close()
		return nil, fmt.Errorf("acquire power cutover lock: %w", err)
	}
	return lock, nil
}

const updateSleepGuardUnit = "ryoku-power-cutover-guard.service"
const packagePowerCutoverMarker = "/var/lib/ryoku/power-cutover-hook-active"

func needsPackagePowerCutover(markerPresent, rootGuardActive bool) bool {
	return !markerPresent || rootGuardActive
}

func ensurePackagePowerCutover() error {
	markerPresent := false
	if _, err := os.Stat(packagePowerCutoverMarker); err == nil {
		markerPresent = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect package power cutover state: %w", err)
	}
	rootGuardActive := exec.Command(
		"sudo", "-n", "systemctl", "is-active", "--quiet", updateSleepGuardUnit,
	).Run() == nil
	if !needsPackagePowerCutover(markerPresent, rootGuardActive) {
		return nil
	}
	const helper = "/usr/bin/ryoku-power-cutover"
	if _, err := os.Stat(helper); err != nil {
		if os.IsNotExist(err) && !rootGuardActive {
			return nil
		}
		return fmt.Errorf("inspect package power cutover helper: %w", err)
	}
	if err := sys.Sudo(helper, "package"); err != nil {
		return fmt.Errorf("adopt packaged power policy in live sessions: %w", err)
	}
	return nil
}

type login1Inhibitor struct {
	What string
	Who  string
	Why  string
	Mode string
	UID  uint32
	PID  uint32
}

type updateSleepGuard struct {
	conn *dbus.Conn
}

func hasUpdateSleepGuard(inhibitors []login1Inhibitor, uid uint32) bool {
	for _, inhibitor := range inhibitors {
		if inhibitor.UID != uid || inhibitor.Who != "ryoku-session-cutover" ||
			inhibitor.Mode != "block" {
			continue
		}
		for _, what := range strings.Split(inhibitor.What, ":") {
			if what == "sleep" {
				return true
			}
		}
	}
	return false
}

func acquireUpdateSleepGuard() (*updateSleepGuard, error) {
	for _, command := range []string{"systemctl", "systemd-run"} {
		if _, err := exec.LookPath(command); err != nil {
			return nil, fmt.Errorf("%s is required for the update sleep guard", command)
		}
	}
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, fmt.Errorf("connect login1 for update sleep guard: %w", err)
	}
	guard := &updateSleepGuard{conn: conn}
	active := exec.Command("systemctl", "--user", "is-active", "--quiet", updateSleepGuardUnit).Run() == nil
	if !active {
		_ = exec.Command("systemctl", "--user", "reset-failed", updateSleepGuardUnit).Run()
		output, runErr := exec.Command(
			"systemd-run", "--user", "--quiet", "--collect", "--unit="+updateSleepGuardUnit,
			"--property=Type=exec", "--property=TimeoutStopSec=5s",
			"/usr/bin/systemd-inhibit", "--what=sleep", "--mode=block",
			"--who=ryoku-session-cutover",
			"--why=keep the desktop awake while suspend owners are replaced",
			"/usr/bin/sleep", "infinity",
		).CombinedOutput()
		if runErr != nil &&
			exec.Command("systemctl", "--user", "is-active", "--quiet", updateSleepGuardUnit).Run() != nil {
			guard.Disconnect()
			return nil, fmt.Errorf("start durable update sleep guard: %w: %s", runErr, strings.TrimSpace(string(output)))
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var inhibitors []login1Inhibitor
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err = conn.Object(
			"org.freedesktop.login1",
			dbus.ObjectPath("/org/freedesktop/login1"),
		).CallWithContext(
			ctx,
			"org.freedesktop.login1.Manager.ListInhibitors",
			0,
		).Store(&inhibitors)
		cancel()
		if err == nil && hasUpdateSleepGuard(inhibitors, uint32(os.Getuid())) {
			return guard, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = exec.Command("systemctl", "--user", "stop", updateSleepGuardUnit).Run()
	guard.Disconnect()
	if err != nil {
		return nil, fmt.Errorf("verify durable update sleep guard: %w", err)
	}
	return nil, fmt.Errorf("durable update sleep guard did not acquire a login1 block")
}

func (g *updateSleepGuard) Release() error {
	if g == nil {
		return nil
	}
	if exec.Command("systemctl", "--user", "is-active", "--quiet", updateSleepGuardUnit).Run() == nil {
		if output, err := exec.Command("systemctl", "--user", "stop", updateSleepGuardUnit).CombinedOutput(); err != nil {
			return fmt.Errorf("release durable update sleep guard: %w: %s", err, strings.TrimSpace(string(output)))
		}
	}
	return nil
}

func (g *updateSleepGuard) Disconnect() {
	if g != nil && g.conn != nil {
		_ = g.conn.Close()
		g.conn = nil
	}
}

// preparePowerPolicyUpdate removes pre-upgrade suspend owners before the shell
// is quiesced. hypridle.conf is generated state outside materialize, while a
// running shell-script daemon keeps executing the old file after replacement.
func preparePowerPolicyUpdate() error {
	_ = exec.Command("systemctl", "--user", "stop",
		"ryoku-clamshell.service", "ryoku-idle.service").Run()
	if err := runPowerHelper("ryoku-clamshell", "stop"); err != nil {
		return fmt.Errorf("stop previous lid policy: %w", err)
	}
	if err := runPowerHelper("ryoku-idle", "stop"); err != nil {
		return fmt.Errorf("stop previous idle policy: %w", err)
	}
	return nil
}

func hasStatusLine(status, want string) bool {
	for _, line := range strings.Split(status, "\n") {
		if line == want {
			return true
		}
	}
	return false
}

// activatePowerPolicy starts both policy owners as session services only after
// the new shell can serve its secure suspend transaction.
func activatePowerPolicy() error {
	if output, err := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("reload user services: %w: %s", err, strings.TrimSpace(string(output)))
	}
	_ = exec.Command("systemctl", "--user", "reset-failed",
		"ryoku-idle.service", "ryoku-clamshell.service").Run()
	if output, err := exec.Command("systemctl", "--user", "restart", "ryoku-idle.service").CombinedOutput(); err != nil {
		return fmt.Errorf("start idle policy service: %w: %s", err, strings.TrimSpace(string(output)))
	}
	idleReady := false
	for range 40 {
		output, _ := exec.Command(pkgBin("ryoku-idle"), "status").Output()
		status := string(output)
		if hasStatusLine(status, "idle=inactive") ||
			(hasStatusLine(status, "idle=active") &&
				hasStatusLine(status, "running=yes") &&
				exec.Command("systemctl", "--user", "is-active", "--quiet", "ryoku-idle.service").Run() == nil) {
			idleReady = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !idleReady {
		return fmt.Errorf("idle policy service did not acquire its hypridle owner")
	}

	if output, err := exec.Command("systemctl", "--user", "restart", "ryoku-clamshell.service").CombinedOutput(); err != nil {
		return fmt.Errorf("start lid policy service: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if exec.Command(pkgBin("ryoku-clamshell"), "is-laptop").Run() != nil {
		return nil
	}
	for range 40 {
		output, _ := exec.Command(pkgBin("ryoku-clamshell"), "status").Output()
		status := string(output)
		if hasStatusLine(status, "owner-monitor=ready") &&
			hasStatusLine(status, "inhibitor=held") &&
			exec.Command("systemctl", "--user", "is-active", "--quiet", "ryoku-clamshell.service").Run() == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("lid policy service did not acquire its login1 inhibitor")
}

func runPowerHelper(name, arg string) error {
	if !sys.Has(name) {
		return fmt.Errorf("%s is not installed", name)
	}
	if err := exec.Command(pkgBin(name), arg).Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, arg, err)
	}
	return nil
}

func reloadLogindLidPolicy() error {
	if err := exec.Command("sudo", "systemctl", "reload", "systemd-logind").Run(); err != nil {
		return fmt.Errorf("reload logind lid policy: %w", err)
	}
	return nil
}

// rashinReindex refreshes the agent-OS vault after an update so agents see
// the new system immediately. Best effort: rashin is optional and a failed
// index never blocks an update.
func rashinReindex() {
	if !sys.Has("ryoku-rashin") {
		return
	}
	fmt.Println(i18n.T("==> Reindexing the Rashin vault"))
	if err := sys.Run(pkgBin("ryoku-rashin"), "index"); err != nil {
		fmt.Fprintf(os.Stderr, i18n.T("warning: rashin reindex failed: %v\n"), err)
	}
	// Re-wire every agent after the index: the shipped ryoku skill and Prowl's
	// skills may have moved or grown with this update, and wire is idempotent.
	if err := sys.Run(pkgBin("ryoku-rashin"), "wire"); err != nil {
		fmt.Fprintf(os.Stderr, i18n.T("warning: rashin wire failed: %v\n"), err)
	}
}

// prowlRefresh keeps a dev box's prowl current after an update. A packaged box
// already got it through `pacman -Syu`, so this runs `<bin> update` only when
// the binary is on PATH but not owned by a pacman package (a dev or manual
// install). The CLI was renamed prowl-agent -> prowl; prefer the new name and
// fall back to the old one upstream still ships. Best effort, one line either way.
func prowlRefresh() {
	path, err := exec.LookPath("prowl")
	if err != nil {
		if path, err = exec.LookPath("prowl-agent"); err != nil {
			return
		}
	}
	switch prowlDecide(true, prowlPacmanOwned(path)) {
	case prowlManaged:
		fmt.Println(i18n.T("==> prowl is managed by pacman; refreshed with the system packages"))
	case prowlSelfUpdate:
		fmt.Println(i18n.T("==> Updating prowl"))
		if err := sys.Run(path, "update"); err != nil {
			fmt.Fprintf(os.Stderr, i18n.T("warning: prowl update failed: %v\n"), err)
		}
	}
}

// prowlPacmanOwned reports whether path belongs to an installed pacman package;
// `pacman -Qo <path>` exits non-zero for a file no package owns (a dev install).
func prowlPacmanOwned(path string) bool {
	return exec.Command("pacman", "-Qo", path).Run() == nil
}

// prowlAction is what an update should do about prowl.
type prowlAction int

const (
	prowlNoop       prowlAction = iota // not installed; nothing to do
	prowlManaged                       // pacman-owned; the system upgrade covered it
	prowlSelfUpdate                    // dev install; run `<bin> update`
)

// prowlDecide is the pure update decision, split out so it is unit-testable
// without a live PATH or pacman.
func prowlDecide(onPath, pacmanOwned bool) prowlAction {
	if !onPath {
		return prowlNoop
	}
	if pacmanOwned {
		return prowlManaged
	}
	return prowlSelfUpdate
}

// clearStalePacmanLock mirrors doctor's reconcilePacmanLock right before the
// system upgrade: a db.lck left by a crashed pacman would fail the very update
// the user is running to heal the box. A lock owned by a live pacman is left
// alone. Composed from sys primitives, same reason as snapHelpers below.
func clearStalePacmanLock() {
	const lock = "/var/lib/pacman/db.lck"
	if !sys.Exists(lock) {
		return
	}
	if exec.Command("pgrep", "-x", "pacman").Run() == nil {
		return
	}
	progress.logf(i18n.T("Removing a stale pacman lock (no pacman running)"))
	_ = sys.Sudo("rm", "-f", lock)
}

// snapHelpers: the snapshot facts the offer gates on, composed from sys
// primitives so the updater runs the same checks the doctor does without
// importing the doctor package.
type snapHelpers struct {
	rootBtrfs  bool
	snapper    bool
	snapPac    bool
	limineSync bool
	limine     bool
}

func gatherSnapHelpers() snapHelpers {
	return snapHelpers{
		rootBtrfs:  sys.IsBtrfs("/"),
		snapper:    sys.Has("snapper"),
		snapPac:    sys.PkgInstalled("snap-pac"),
		limineSync: sys.PkgInstalled("limine-snapper-sync"),
		limine:     sys.PkgInstalled("limine"),
	}
}

// wantedSnapperHelpers: which snapshot helpers to offer, given the facts. pure,
// so the gating (no snapshots without btrfs + snapper; limine-snapper-sync only
// under Limine) is unit-testable without touching /etc or pacman.
func wantedSnapperHelpers(h snapHelpers) []string {
	if !h.rootBtrfs || !h.snapper {
		return nil
	}
	var want []string
	if !h.snapPac {
		want = append(want, "snap-pac")
	}
	if !h.limineSync && h.limine {
		want = append(want, "limine-snapper-sync")
	}
	return want
}

// offerSnapperHelpers: ask before installing the missing helpers, then
// install whoever was picked. snap-pac = a snapshot on every pacman txn;
// limine-snapper-sync, on a Limine box, puts those snapshots in the boot
// menu. together = the rollback safety net behind every `ryoku update`.
// opt-in + best-effort: Skip (or no answer) leaves them for `ryoku doctor`
// to keep recommending, and a failed install never aborts the update.
func offerSnapperHelpers() {
	want := wantedSnapperHelpers(gatherSnapHelpers())
	if len(want) == 0 {
		return
	}
	var blurbs []string
	for _, p := range want {
		switch p {
		case "snap-pac":
			blurbs = append(blurbs, i18n.T("auto-snapshot every update"))
		case "limine-snapper-sync":
			blurbs = append(blurbs, i18n.T("snapshots in the boot menu"))
		}
	}
	detail := strings.Join(want, " + ") + i18n.T(" back the rollback safety net (") + strings.Join(blurbs, ", ") + ")."
	if !askInstall(i18n.T("Enable snapshot helpers?"), detail, want) {
		fmt.Printf(i18n.T("==> Snapshot helpers skipped (%s); ryoku doctor keeps recommending them\n"), strings.Join(want, ", "))
		return
	}
	fmt.Printf(i18n.T("==> Installing snapshot helpers: %s\n"), strings.Join(want, ", "))
	for _, p := range want {
		tool := "ryoku-pkg-add"
		if p == "limine-snapper-sync" {
			tool = "ryoku-pkg-aur-add"
		}
		if err := sys.Run(tool, p); err != nil {
			fmt.Fprintf(os.Stderr, i18n.T("warning: installing %s failed: %v\n"), p, err)
		}
	}
}

// askInstall: consent for installing pkgs. hub-launched update
// (RYOKU_UPDATE_UI=hub) -> ask through the run-state prompt and wait.
// plain terminal -> y/N. non-interactive -> decline.
func askInstall(title, detail string, pkgs []string) bool {
	if os.Getenv("RYOKU_UPDATE_UI") == "hub" {
		publishPrompt("snapper-helpers", title, detail, []string{"Install", "Skip"})
		choice, ok := awaitAnswer(120 * time.Second)
		progress.publish("running") // clear the prompt; resume the step view
		return ok && choice == "Install"
	}
	if sys.StdinIsTTY() {
		fmt.Printf(i18n.T("%s install %s? [y/N] "), title, strings.Join(pkgs, ", "))
		var resp string
		_, _ = fmt.Scanln(&resp)
		resp = strings.ToLower(strings.TrimSpace(resp))
		return resp == "y" || resp == "yes"
	}
	return false
}

// runFreshDoctor runs `ryoku doctor` after the new binary lands, so the
// reconcilers shipped in this release run inside the same update. same
// command users run by hand; calling it here keeps doctor one thing instead
// of a copy baked into update. best-effort: a finding never fails update.
func runFreshDoctor() {
	fmt.Println(i18n.T("==> Running doctor"))
	// pkgBin, not PATH: on a box with ~/.local/bin residue the bare name is the
	// STALE CLI, whose doctor predates the reconcilers this release ships --
	// including the residue scan that would clear that very shadow.
	_ = sys.Run(pkgBin("ryoku"), "doctor")
}

// Rollback is the way back, on two levels. `--to <tag>` moves the Ryoku set
// (its packages and config) to a published release without a reboot: the
// [ryoku] repo is pinned at that frozen release directory and the update runs,
// so the set moves in one pacman transaction while Arch stays current;
// `ryoku track stable` follows releases again afterwards. A snapshot id guides
// a whole-system restore from the boot menu. With no argument it shows both:
// the releases the ledger knows and the snapshots on disk.
//
// The snapshot path is a boot-menu restore, not a live one: Ryoku pins the
// root subvolume on the kernel cmdline and in fstab (rootflags=subvol=@), and
// `snapper rollback` cannot serve that layout, since it works by flipping the
// btrfs default subvolume, which a pinned subvol= simply ignores;
// limine-snapper-sync's own tooling states the layout is "not compatible with
// 'snapper rollback'". So the command teaches that flow instead of running a
// snapper command that cannot restore the system.
func Rollback(args []string) error {
	to := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--to" && i+1 < len(args) {
			to = args[i+1]
			i++
		}
	}
	if to != "" {
		if !sys.IsReleaseTag(to) {
			return fmt.Errorf(i18n.T("--to takes a release tag (see `ryoku rollback` for the list), got %q"), to)
		}
		fmt.Printf(i18n.T("==> Moving the Ryoku set to release %s\n"), to)
		return Track(to)
	}
	if len(args) > 0 {
		return restoreGuide(args[0])
	}

	fmt.Println(i18n.T("Two ways back:"))
	fmt.Println(i18n.T("  the Ryoku set (its packages and config) to a published release, live;"))
	fmt.Println(i18n.T("  the whole system (Arch included) to a snapshot, from the boot menu."))
	fmt.Println()
	printReleases()
	fmt.Println()
	fmt.Println(i18n.T("SNAPSHOTS  the whole system, on this disk"))
	if err := Snapshots(); err != nil {
		return err
	}
	return nil
}

// printReleases is the RELEASES block of `ryoku rollback`: what a packaged box
// runs, what it can move to, and how; a checkout box is told releases do not
// apply to it instead of being shown nothing.
func printReleases() {
	if sys.ResolveRepo() != "" {
		fmt.Println(i18n.T("RELEASES  not on this box"))
		fmt.Printf(i18n.T("  this box builds from a source checkout of %s; releases apply to packaged installs.\n"), ryokuChannel())
		fmt.Println(i18n.T("  ryoku track main|unstable-dev   moves it onto stable|testing packages (with releases)"))
		return
	}
	ch := sys.PackagedChannel()
	rel := sys.ReadRelease()
	fmt.Printf(i18n.T("RELEASES  repo.ryoku.dev, channel: %s\n"), orDash(ch))
	if rel.Release != "" {
		fmt.Printf(i18n.T("  running    %s%s\n"), withSpace(rel.Name), rel.Release)
	}
	l := ledger()
	if len(l.Releases) == 0 {
		fmt.Println(i18n.T("  no published releases yet"))
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 3, ' ', 0)
	for _, r := range l.Releases {
		mark := " "
		if r.Tag == rel.Release {
			mark = "*"
		}
		fmt.Fprintf(w, "  %s %s\t%s\t%s\n", mark, r.Tag, r.Date[:min(10, len(r.Date))], r.Name)
	}
	w.Flush()
	fmt.Println(i18n.T("  ryoku rollback --to <tag>   moves the Ryoku set to that release, no reboot"))
	fmt.Println(i18n.T("  ryoku track stable          follows new releases again afterwards"))
}

// restoreGuide is `ryoku rollback <id>`: the boot-menu restore, step by step,
// naming the snapshot when snapper can describe it.
func restoreGuide(id string) error {
	label := id
	if rows, err := snapshotRows(); err == nil {
		for _, r := range rows {
			if r.number == id {
				label = fmt.Sprintf("%s  (%s, %s %s)", id, shortSnapDate(r.date), r.kind, orDash(r.description))
				break
			}
		}
	}
	fmt.Printf(i18n.T("Restoring snapshot %s\n\n"), label)
	fmt.Println(i18n.T("Ryoku boots the @ subvolume directly, so a live `snapper rollback` cannot"))
	fmt.Println(i18n.T("restore the system; the restore runs from the boot menu:"))
	fmt.Printf(i18n.T("  1. Reboot, and in the Limine menu open Snapshots -> %s.\n"), id)
	fmt.Println(i18n.T("  2. In that session run:  sudo limine-snapper-restore"))
	fmt.Println(i18n.T("     (it restores the snapshot you booted, matching kernels included)"))
	fmt.Println(i18n.T("  3. Reboot into the restored system."))
	if !sys.PkgInstalled("limine-snapper-sync") {
		fmt.Println()
		fmt.Println(i18n.T("limine-snapper-sync is not installed, so snapshots are missing from the boot"))
		fmt.Println(i18n.T("menu. Install it first:"))
		fmt.Println("  ryoku-pkg-aur-add limine-snapper-sync && sudo systemctl enable --now limine-snapper-sync.service")
	}
	return nil
}

// Snapshots prints the snapshot table (the SNAPSHOTS block of `ryoku rollback`).
func Snapshots() error {
	if !sys.Has("snapper") {
		return fmt.Errorf(i18n.T("snapper is not installed"))
	}
	if !sys.Exists("/etc/snapper/configs/root") {
		fmt.Println(i18n.T("  not configured on this machine; `ryoku doctor` enables them"))
		return nil
	}
	rows, err := snapshotRows()
	if err != nil {
		// fall back to snapper's own table rather than showing nothing.
		return sys.Sudo("snapper", "-c", snapperConfig, "list")
	}
	printSnapshotTable(rows)
	return nil
}

// snapshotRows lists the root snapshots, parsed, without ever prompting. It runs
// snapper unprivileged first -- which succeeds once the config grants the user
// read access (ALLOW_USERS + SYNC_ACL, snapper's own mechanism, which `ryoku
// doctor` sets up) -- then a cached-credential `sudo -n`, which never prompts,
// so it needs no tty and cannot trip pam_faillock. Only a real CSV listing
// counts as success: snapper prints "No permissions." to stderr and still exits
// 0 on a denied read, so the exit code alone would read an empty stdout as an
// empty store. The error is returned when neither attempt yields a listing, so a
// caller can tell a failed read from a genuinely empty one instead of both
// looking like zero.
func snapshotRows() ([]snapshotRow, error) {
	args := []string{"-c", snapperConfig, "--csvout", "list",
		"--columns", "number,type,date,description,cleanup"}
	if out, err := sys.RunOut("snapper", args...); err == nil && isSnapshotCSV(out) {
		return parseSnapshotRows(out), nil
	}
	if out, err := sys.RunOut("sudo", append([]string{"-n", "snapper"}, args...)...); err == nil && isSnapshotCSV(out) {
		return parseSnapshotRows(out), nil
	}
	return nil, fmt.Errorf(i18n.T("snapper root snapshots are not readable (grant access with `ryoku doctor` or prime sudo)"))
}

// isSnapshotCSV reports whether out is a real `snapper --csvout list` listing:
// its first non-empty line is the column header. A denied read ("No
// permissions." on stderr, empty stdout, exit 0) fails this, so it is never
// mistaken for an empty store.
func isSnapshotCSV(out string) bool {
	for _, line := range strings.Split(out, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			return strings.HasPrefix(s, "number,")
		}
	}
	return false
}

// snapshotRow is one parsed line of `snapper --csvout list`.
type snapshotRow struct {
	number      string
	kind        string // pre | post | single
	date        string
	description string
	cleanup     string
}

// parseSnapshotRows parses the CSV list, dropping the header and base snapshot 0.
func parseSnapshotRows(out string) []snapshotRow {
	var rows []snapshotRow
	r := csv.NewReader(strings.NewReader(out))
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return rows
	}
	for _, rec := range records {
		if len(rec) < 5 {
			continue
		}
		num := strings.TrimSpace(rec[0])
		if num == "" || num == "number" || num == "0" {
			continue
		}
		rows = append(rows, snapshotRow{
			number:      num,
			kind:        strings.TrimSpace(rec[1]),
			date:        strings.TrimSpace(rec[2]),
			description: strings.TrimSpace(rec[3]),
			cleanup:     strings.TrimSpace(rec[4]),
		})
	}
	return rows
}

// printSnapshotTable shows the snapshots (newest last, the way snapper counts)
// plus a count, free space, and whether the boot menu lists them.
func printSnapshotTable(rows []snapshotRow) {
	if len(rows) == 0 {
		fmt.Println(i18n.T("  none yet; `ryoku update` takes one before and after each update"))
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	for _, s := range rows {
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", s.number, shortSnapDate(s.date), s.kind, orDash(s.description))
	}
	w.Flush()
	fmt.Println()
	summary := fmt.Sprintf(i18n.T("  %d snapshots"), len(rows))
	if free := rootFree(); free != "" {
		summary += ", " + free + i18n.T(" free on /")
	}
	if snapshotsInBootMenu() {
		summary += i18n.T(", listed in the Limine boot menu")
	} else {
		summary += i18n.T(", NOT in the boot menu (ryoku doctor fixes that)")
	}
	fmt.Println(summary)
	fmt.Println(i18n.T("  ryoku rollback <#>          shows how to boot into a snapshot and restore it"))
}

// shortSnapDate trims snapper's "2026-09-03 22:10:06" to the minute.
func shortSnapDate(d string) string {
	if len(d) >= 16 {
		return d[:16]
	}
	return d
}

// snapshotsInBootMenu reports whether limine-snapper-sync is in place to list
// snapshots in the Limine boot menu (Ryoku's only supported restore path).
func snapshotsInBootMenu() bool {
	return sys.PkgInstalled("limine") &&
		sys.PkgInstalled("limine-snapper-sync") &&
		sys.UnitEnabled("limine-snapper-sync.service")
}

// rootFree is the human-readable free space on /, or "" when statfs fails.
func rootFree() string {
	var st syscall.Statfs_t
	if err := syscall.Statfs("/", &st); err != nil {
		return ""
	}
	return humanBytes(st.Bavail * uint64(st.Bsize))
}

func Status(args []string) error {
	jsonOut := false
	for _, a := range args {
		if a == "--json" {
			jsonOut = true
		}
	}
	r := buildStatus()

	if jsonOut {
		b, _ := json.Marshal(r)
		fmt.Println(string(b))
		return nil
	}

	fmt.Printf(i18n.T("config base:   %s\n"), sys.BaseConfigDir())
	fmt.Printf(i18n.T("channel:       %s\n"), orDash(r.Channel))
	if r.Release != "" {
		if r.ChannelRelease != "" && r.ChannelRelease != r.Release {
			fmt.Printf(i18n.T("release:       %s%s -> %s%s\n"), withSpace(r.ReleaseName), r.Release, withSpace(r.ChannelReleaseName), r.ChannelRelease)
		} else {
			fmt.Printf(i18n.T("release:       %s%s\n"), withSpace(r.ReleaseName), r.Release)
		}
	}
	fmt.Printf(i18n.T("installed:     %s\n"), orDash(r.Installed))
	if r.Available {
		fmt.Printf(i18n.T("available:     %s\n"), orDash(r.Latest))
		fmt.Printf(i18n.T("behind:        %d commit(s)\n"), r.Behind)
	} else {
		fmt.Println(i18n.T("behind:        up to date"))
	}
	// the other lane, named as such: `ryoku update` never moves these, so a
	// box that reads "up to date" above can still owe its distribution a
	// kernel.
	if r.SystemPending > 0 {
		fmt.Printf(i18n.T("system:        %d package(s) waiting (sudo pacman -Syu)\n"), r.SystemPending)
	} else {
		fmt.Println(i18n.T("system:        up to date"))
	}
	// Three distinct states, never conflated: no root config at all (doctor
	// restores it), a config we could not read (a bare "0" here used to look
	// like a real empty store -- the opposite meaning), and the real count.
	switch {
	case !sys.Exists("/etc/snapper/configs/root"):
		fmt.Println(i18n.T("snapshots:     not configured (run ryoku doctor)"))
	case !r.SnapshotsKnown:
		fmt.Println(i18n.T("snapshots:     unavailable (grant read access: run ryoku doctor)"))
	default:
		fmt.Printf(i18n.T("snapshots:     %d\n"), r.Snapshots)
	}
	return nil
}

// statusReport = what the Hub and the update island read from
// `ryoku status --json`. installed + available versions, how far behind,
// per-item list. from the git update channel on a Ryoku checkout (the live
// mirror), else from the [ryoku] pacman repo.
type statusReport struct {
	Installed string       `json:"installedVersion"`
	Latest    string       `json:"latestVersion"`
	Available bool         `json:"available"`
	Behind    int          `json:"pendingUpdates"`
	Updates   []updateItem `json:"updates"`
	Recent    []updateItem `json:"recent"`
	Channel   string       `json:"channel"`
	Snapshots int          `json:"snapshots"`
	// SnapshotsKnown is false when the count could not be read (no snapper access,
	// cold sudo) rather than genuinely zero, so a consumer never reads a failed
	// query as "no safety net". A bare `snapshots: 0` used to conflate the two.
	SnapshotsKnown bool `json:"snapshotsKnown"`
	// Packages is the OTHER lane: what the distribution (Arch or CachyOS) has
	// waiting, kernel included. `sudo pacman -Syu` takes those; `ryoku update`
	// deliberately does not, so Available stays about the Ryoku lane alone and
	// the update button never offers a run that would move none of them.
	Packages      []updateItem `json:"packages"`
	SystemPending int          `json:"systemUpdates"`
	// packaged boxes: the release this box runs (/etc/ryoku-release) and the
	// one its channel serves now (release.json beside the channel's db), so
	// the island and the Hub can say "v0.55.7 -> v0.55.9" instead of a sha.
	Release            string `json:"release,omitempty"`
	ReleaseName        string `json:"releaseName,omitempty"`
	ChannelRelease     string `json:"channelRelease,omitempty"`
	ChannelReleaseName string `json:"channelReleaseName,omitempty"`
}

// withSpace is a release name as a prefix: "Onogoro " or "" when unnamed.
func withSpace(name string) string {
	if name == "" {
		return ""
	}
	return name + " "
}

// buildStatus is the full Updates report: the Ryoku channel (baseStatus) plus,
// listed separately, what the distribution has waiting. baseStatus prefers the
// git update channel (a checkout tracking main); a packaged install reads the
// running and available commits from the [ryoku] repo's package versions and
// lists what is incoming via the public GitHub compare API, so the Hub's list
// is the same commit subjects a dev box shows, not bare package names.
func buildStatus() statusReport {
	r := baseStatus()
	// The user's lane, check-only. It never sets Available: `ryoku update`
	// would not move any of it, and a button that promises otherwise is how a
	// box ends up looking updated while its kernel never moves.
	r.Packages = systemPackageUpdates()
	r.SystemPending = len(r.Packages)
	addRyotunesUpdate(&r)
	return r
}

// baseStatus builds the Ryoku-channel report: the git update channel on a
// checkout, else the [ryoku] repo package versions on a packaged install.
func baseStatus() statusReport {
	if r, ok := channelStatus(); ok {
		// a checkout has no release, but it runs a named line (CODENAME)
		r.ReleaseName = ReleaseName()
		return r
	}
	installed := sys.InstalledVersion()
	latest := latestAvailable("ryoku-desktop")
	for _, u := range pendingUpdates() {
		if u.Name == "ryoku-desktop" {
			latest = u.New
		}
	}
	return packagedStatus(installed, latest)
}

// packagedStatus builds the report for a packaged install from the running and
// available package versions. The GitHub lookups are best-effort and stubbable
// (RYOKU_GITHUB_API), so the sha/compare/recent branching is unit-testable
// without pacman, the same reason wantedSnapperHelpers is split out.
func packagedStatus(installed, latest string) statusReport {
	installedSha := shortCommit(installed)
	latestSha := shortCommit(latest)

	snaps, snapsKnown := snapshotCount()
	r := statusReport{
		Installed:      installedSha,
		Latest:         latestSha,
		Updates:        []updateItem{}, // non-nil, so a current box marshals [] like the git path
		Recent:         []updateItem{}, // non-nil, so the JSON stays stable when nothing is fetched
		Channel:        ryokuChannel(),
		Snapshots:      snaps,
		SnapshotsKnown: snapsKnown,
		Release:        sys.ReadRelease().Release,
		ReleaseName:    ReleaseName(),
	}
	if ch := sys.PackagedChannel(); ch != "" {
		serves := channelServes(ch)
		r.ChannelRelease, r.ChannelReleaseName = serves.Release, serves.Name
	}
	// up to date: nothing incoming, but list the recent history the installed
	// version contains (best-effort, newest-first) so the Hub's Updates page
	// still shows meaningful content instead of a blank section.
	if installedSha != "" && installedSha == latestSha {
		if rec := recentCommits(installedSha); len(rec) > 0 {
			r.Recent = rec
		}
		return r
	}
	// the [ryoku] repo isn't synced yet: nothing to compare.
	if installedSha == "" || latestSha == "" {
		return r
	}
	r.Available = true
	if ups, behind := incomingCommits(installedSha, latestSha); len(ups) > 0 {
		r.Updates = ups
		r.Behind = behind
	} else {
		// compare unreachable (offline / rate-limited): still surface the
		// pending Ryoku bump so the section isn't empty and available holds.
		r.Updates = []updateItem{{Name: "ryoku-desktop", Old: installed, New: latest}}
		r.Behind = 1
	}
	return r
}

// shortCommit pulls the abbreviated commit hash out of a packaged version
// shaped <core>.r<count>.g<sha>(-pkgrel) (what the repo build embeds), so
// Hub and CLI can show the exact commit a packaged box runs. no gNNNN token
// (a hand-pinned 0.1.0-3, say) -> input comes back unchanged.
func shortCommit(ver string) string {
	for _, tok := range strings.FieldsFunc(ver, func(r rune) bool { return r == '.' || r == '-' }) {
		if len(tok) >= 8 && tok[0] == 'g' && isHex(tok[1:]) {
			return tok[1:]
		}
	}
	return ver
}

func isHex(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return s != ""
}

// latestAvailable: version of pkg in the [ryoku] repo, or "" when the repo
// isn't synced/configured. `pacman -Sl ryoku` = "<repo> <pkg> <ver>".
func latestAvailable(pkg string) string {
	out, err := sys.RunOut("pacman", "-Sl", "ryoku")
	if err != nil {
		return ""
	}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) >= 3 && f[1] == pkg {
			return f[2]
		}
	}
	return ""
}

// updateItem = one row in the update list. pacman -> a package (name,
// old -> new). git channel -> a commit (subject in Name, short hash in New).
type updateItem struct {
	Name string `json:"name"`
	Old  string `json:"old"`
	New  string `json:"new"`
}

// pendingUpdates: packages with a newer version available, via checkupdates
// (pacman-contrib). syncs to a private db, so no root needed. empty when
// the system is current or checkupdates is absent.
func pendingUpdates() []updateItem {
	ups := []updateItem{}
	if !sys.Has("checkupdates") {
		return ups
	}
	// cap the check: checkupdates syncs package dbs over the network and the
	// update island polls this, so it MUST never hang status. generous so a
	// slow sync still finishes.
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, "checkupdates").Output()
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) >= 4 && f[2] == "->" && !externalReleasePkgs[f[0]] {
			ups = append(ups, updateItem{Name: f[0], Old: f[1], New: f[3]})
		}
	}
	return ups
}

// systemPackageUpdates lists what a system upgrade would pull outside the Ryoku
// channel: repo packages (checkupdates) and AUR packages (yay -Qua). Check-only.
func systemPackageUpdates() []updateItem {
	return append(pendingUpdates(), aurUpdates()...)
}

// aurUpdates lists AUR packages with a newer version via `yay -Qua` (no install).
func aurUpdates() []updateItem {
	ups := []updateItem{}
	if !sys.Has("yay") {
		return ups
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, "yay", "-Qua").Output()
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) >= 4 && f[2] == "->" {
			ups = append(ups, updateItem{Name: f[0], Old: f[1], New: f[3]})
		}
	}
	return ups
}

// snapshotCount returns how many snapshots the root store holds and whether it
// could be read at all. A failed read -- no snapper access and no cached sudo --
// returns (0, false), distinct from a genuinely empty store (0, true), so
// `ryoku status` can say "unavailable" instead of a bare "0" that means the
// opposite. snapshotRows never prompts, so this is safe from the GUI's
// terminal-less poll (a prompt with no tty trips pam_faillock and can lock the
// account out of sudo -- found the loud way).
func snapshotCount() (int, bool) {
	if !sys.Has("snapper") {
		return 0, false
	}
	rows, err := snapshotRows()
	if err != nil {
		return 0, false
	}
	return len(rows), true
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// Deploy = the DEV loop: build the Go binaries + plugin and materialize
// from a repo checkout. production installs never see this; they pull
// everything from the [ryoku] pacman repo.
func Deploy(_ []string) error {
	repo := os.Getenv("RYOKU_REPO")
	if repo == "" {
		return fmt.Errorf(i18n.T("set RYOKU_REPO to a Ryoku checkout for `ryoku deploy`"))
	}
	script := filepath.Join(repo, "ryoku", "shell", "deploy.sh")
	if !sys.Exists(script) {
		return fmt.Errorf(i18n.T("not a Ryoku checkout (missing %s)"), script)
	}
	return sys.Run(script)
}

// --- snapper pre/post (best-effort) ----------------------------------------

func snapperPre(desc string) string {
	if !sys.Has("snapper") {
		fmt.Fprintln(os.Stderr, i18n.T("note: snapper not installed; skipping pre-update snapshot"))
		return ""
	}
	// no root config -> the create below fails with an opaque
	// "config 'root' does not exist". point the user at the fix.
	if !sys.Exists("/etc/snapper/configs/root") {
		fmt.Fprintln(os.Stderr, i18n.T("note: snapshot skipped, snapper root config missing; run 'ryoku doctor' to enable snapshots"))
		return ""
	}
	out, err := sys.RunOut("sudo", "snapper", "-c", snapperConfig, "create",
		"-t", "pre", "-c", "number", "-p", "-d", desc)
	if err != nil {
		fmt.Fprintf(os.Stderr, i18n.T("note: pre-update snapshot skipped: %v\n"), err)
		return ""
	}
	return strings.TrimSpace(out)
}

func snapperPost(pre, desc string) {
	if pre == "" {
		return
	}
	_ = sys.Sudo("snapper", "-c", snapperConfig, "create",
		"-t", "post", "--pre-number", pre, "-c", "number", "-d", desc)
	// Prune here, on the path every update takes: snapper-cleanup.timer is
	// unreliable (its service is coupled to limine-snapper-sync), so without
	// this the pile grows unbounded. best-effort.
	_ = sys.Sudo("snapper", "-c", snapperConfig, "cleanup", "number")
}

// pauseConfigAutoreload stops the compositor reloading its config mid-swap, so a
// half-written tree is never observed.
func pauseConfigAutoreload() {
	c := wm.Open()
	if c.Detection().Live {
		_ = c.Act(wm.ActionConfigAutoreload, "off")
	}
}

// reloadConfig applies materialized config in one clean pass where the provider
// exposes an imperative reload. Niri watches its KDL and deliberately has no
// reload action, so unsupported means the file watcher owns this step. Any
// other live provider failure is fatal while the update sleep guard is held:
// starting the lid inhibitor without its compositor event producer would turn
// every close into a no-op.
func configReloadResult(err error) error {
	if errors.Is(err, wm.ErrUnsupported) {
		return nil
	}
	return err
}

func reloadConfig() error {
	c := wm.Open()
	if !c.Detection().Live {
		return nil
	}
	return configReloadResult(c.Act(wm.ActionConfigReload))
}

// regenerateConfig re-authors the live provider's generated config from the
// store. Those files are a pure function of the store and the provider that
// wrote them, so a provider update that emits a block differently (niri's
// border only draws with an explicit on flag) has to rewrite them, or the old
// output stays in force until a Hub edit happens to apply again. A box with
// no store yet is left to the first Hub save.
func regenerateConfig() error {
	store := filepath.Join(sys.ConfigHome(), "ryoku", "desktop.json")
	if !sys.Exists(store) {
		return nil
	}
	_, err := wm.Open().Apply(store)
	return err
}

// pkgBin resolves a Ryoku binary an update drives. The packaged /usr/bin copy
// is preferred over a bare PATH lookup: a past `ryoku recovery` or dev deploy
// leaves builds in ~/.local/bin that outrank /usr/bin, and driving those runs
// stale code inside the very update meant to supersede it -- a stale daemon
// restarted over the new QML replays an old supervisor against a one-shot
// switcher (the beta-17 switcher-reopen loop), and a stale doctor predates the
// reconcilers this release ships. A box without the package (a pure checkout)
// falls back to PATH, where the just-deployed build is the right one.
func pkgBin(name string) string {
	if p := "/usr/bin/" + name; sys.Exists(p) {
		return p
	}
	return name
}

// stopShell quiesces the desktop for a config swap: ask the daemon to quit,
// wait for it to go, then drop orphaned surfaces still holding a config's
// single-instance lock (one survivor kills the fresh daemon's components).
// The component list mirrors shell/ipc/daemon.go; "plugins" and "wallpaper"
// are retired resident components, still reaped on boxes whose live daemon
// predates their removal.
func stopShell() error {
	if !sys.Has("ryoku-shell") {
		return nil
	}
	// Under systemd the unit would respawn the daemon two seconds after the
	// quit below and the update would race its own quiesce. Stopping the unit
	// is a no-op where it does not exist yet.
	_ = exec.Command("systemctl", "--user", "stop", "ryoku-shell").Run()
	shell := pkgBin("ryoku-shell")
	_ = exec.Command(shell, "quit").Run()
	stopped := false
	for range 50 {
		if exec.Command(shell, "ping").Run() != nil {
			stopped = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !stopped {
		return fmt.Errorf("pre-upgrade ryoku-shell did not stop")
	}
	// the pattern is anchored: quickshell is a general-purpose tool, and a bare
	// "qs -c wallpaper" would also match a user's own longer config name
	// ("qs -c wallpaperclock"). the daemon always spawns the config name as the
	// final argv element.
	for _, c := range []string{"pill", "launcher", "visualizer", "widgets", "overview", "plugins", "wallpaper"} {
		_ = exec.Command("pkill", "-f", "qs -c "+c+"($| )").Run()
	}
	// The video players outlive the daemon (spawned detached): kill the
	// current one so the restarted daemon relaunches it on the new binary, and
	// the legacy backends older releases shipped (mpvpaper, phonto) -- the new
	// daemon no longer knows their names, and an orphan left on the background
	// layer stacks above Ryogami's surface and swallows every static set after
	// the update.
	for _, p := range []string{"ryoku-livewall", "mpvpaper", "phonto"} {
		_ = exec.Command("pkill", "-x", p).Run()
	}
	// The Hub (Ryoku Settings) is a separate, session-resident quickshell
	// instance under its own /tmp/ryoku-hub.lock, not one of the daemon's
	// components, so the quit above never touches it. Left running it keeps the
	// old QML mapped, and a settings page this update just shipped only appears
	// after a relogin. Reap it so the next open loads the new pages; the lock is
	// advisory and frees with the process.
	_ = exec.Command("pkill", "-f", "hub/quickshell").Run()
	_ = exec.Command("pkill", "-f", "qs -c hub($| )").Run()
	time.Sleep(200 * time.Millisecond)
	return nil
}

// startShell brings the shell daemon back up, under systemd where the unit
// exists so it stays supervised, else detached on the current binary. The
// daemon-reload is what lets a unit materialize just laid down be found; a
// stale-cached user manager would otherwise report it unknown at start.
func startShell() error {
	if !sys.Has("ryoku-shell") {
		return nil
	}
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	if exec.Command("systemctl", "--user", "restart", "ryoku-shell").Run() == nil {
		return nil
	}
	if exec.Command("systemctl", "--user", "cat", "ryoku-shell.service").Run() == nil {
		return fmt.Errorf("ryoku-shell.service failed its guarded restart")
	}
	activator := pkgBin("ryoku-qylock-activate")
	if err := exec.Command(activator).Run(); err != nil {
		return fmt.Errorf("activate qylock before fallback shell start: %w", err)
	}
	cmd := exec.Command("setsid", pkgBin("ryoku-shell"), "daemon")
	logp := filepath.Join(sys.Xdg("XDG_STATE_HOME", ".local/state"), "ryoku-shell.log")
	_ = os.MkdirAll(filepath.Dir(logp), 0o755)
	if f, err := os.OpenFile(logp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644); err == nil {
		cmd.Stdout, cmd.Stderr = f, f
		defer f.Close()
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start fallback ryoku-shell: %w", err)
	}
	return nil
}

type updaterLogin1Session struct {
	ID   string
	UID  uint32
	User string
	Seat string
	Path dbus.ObjectPath
}

func login1GraphicalUserProperties(properties map[string]dbus.Variant) bool {
	sessionType, typeOK := properties["Type"].Value().(string)
	sessionClass, classOK := properties["Class"].Value().(string)
	desktop, desktopOK := properties["Desktop"].Value().(string)
	desktop = strings.ToLower(desktop)
	supported := false
	for _, provider := range wm.Providers() {
		if desktop == strings.ToLower(provider) {
			supported = true
			break
		}
	}
	return typeOK && classOK && desktopOK && supported &&
		sessionType == "wayland" &&
		(sessionClass == "user" || sessionClass == "user-early")
}

func graphicalSessionState() (present bool, active bool, err error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return false, false, fmt.Errorf("connect login1 to inspect graphical sessions: %w", err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	manager := conn.Object("org.freedesktop.login1", dbus.ObjectPath("/org/freedesktop/login1"))
	var sessions []updaterLogin1Session
	if err := manager.CallWithContext(
		ctx,
		"org.freedesktop.login1.Manager.ListSessions",
		0,
	).Store(&sessions); err != nil {
		return false, false, fmt.Errorf("list login1 sessions: %w", err)
	}
	var inspectErr error
	for _, session := range sessions {
		if session.UID != uint32(os.Getuid()) {
			continue
		}
		var properties map[string]dbus.Variant
		err := conn.Object("org.freedesktop.login1", session.Path).CallWithContext(
			ctx,
			"org.freedesktop.DBus.Properties.GetAll",
			0,
			"org.freedesktop.login1.Session",
		).Store(&properties)
		if err != nil {
			inspectErr = err
			continue
		}
		if !login1GraphicalUserProperties(properties) {
			continue
		}
		present = true
		isActive, ok := properties["Active"].Value().(bool)
		if !ok {
			inspectErr = fmt.Errorf("login1 session %s has no boolean Active property", session.ID)
			continue
		}
		if isActive {
			return true, true, nil
		}
	}
	if inspectErr != nil {
		return false, false, fmt.Errorf("inspect login1 sessions: %w", inspectErr)
	}
	return present, false, nil
}

func waitForShellReady(expected bool) (bool, error) {
	if !sys.Has("ryoku-shell") {
		if expected {
			return false, fmt.Errorf("ryoku-shell is not installed")
		}
		return false, nil
	}
	shell := pkgBin("ryoku-shell")
	if exec.Command(shell, "sleep-ready").Run() == nil {
		return true, nil
	}
	if !expected {
		return false, nil
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		if exec.Command(shell, "sleep-ready").Run() == nil {
			return true, nil
		}
	}
	return false, fmt.Errorf("new ryoku-shell sleep guard did not become ready after restart")
}

// restartWallpaper brings a previously running wallpaper daemon onto the
// binary the update just installed. Ryogami is supervised beside the shell and
// owns the socket used by the wallpaper, picker and palette. A lifecycle rebind
// stops the whole session target, so try-restart alone would leave it down; in
// that case forceStart preserves the active state recorded before the rebind.
// Outside a graphical session a down daemon is not forced up.
func restartWallpaper(forceStart bool) {
	if !sys.Has("ryogami") {
		return
	}
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	verb := "try-restart"
	if forceStart {
		_ = exec.Command("systemctl", "--user", "reset-failed", "ryogami.service").Run()
		verb = "restart"
	}
	if exec.Command("systemctl", "--user", verb, "ryogami.service").Run() == nil {
		return
	}
	// No unit yet on a box mid-cutover: drop the old process so the shell's
	// respawn picks up the installed binary.
	_ = exec.Command("pkill", "-x", "ryogami").Run()
}

func materializeStatePath() string {
	return filepath.Join(sys.Xdg("XDG_STATE_HOME", ".local/state"), "ryoku", "materialized")
}
