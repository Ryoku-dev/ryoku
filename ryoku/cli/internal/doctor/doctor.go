package doctor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"ryoku-cli/internal/sys"
	"strconv"
	"strings"
	"syscall"
	"time"

	i18n "ryoku-i18n"
	wm "ryoku-wm"
)

// doctor = the convergent reconcilers. idempotent checks (plus a fix where
// it's safe) for the stateful drift that `ryoku update` / `ryoku materialize`
// can't say declaratively: disk layout, package channel, session bits. each
// one returns "ok" if the box already matches, else converges, prints the
// exact fix, or punts to a human.
//
// whatever can't be fixed gets written down: `ryoku doctor --report` dumps the
// findings + system state to one shareable text file for the maintainers.
//
// reconcilers are safe on every update. retire one once every supported install
// has run it, so the set stays small instead of piling up like a migration
// ledger.

const ryokuIssuesURL = "https://github.com/ryoku-dev/ryoku-arch/issues"

type recStatus int

const (
	recOK recStatus = iota
	recNote
	recFixed
	recWouldFix
	recWarn
	recFailed
)

func (s recStatus) label() string {
	switch s {
	case recOK:
		return "ok"
	case recNote:
		return "note"
	case recFixed:
		return "fixed"
	case recWouldFix:
		return "todo"
	case recWarn:
		return "warn"
	case recFailed:
		return "fail"
	}
	return "?"
}

type recResult struct {
	status recStatus
	detail string
	remedy string // exact command or hint, shown for todo/warn/fail
}

func okRes(f string, a ...any) recResult {
	return recResult{status: recOK, detail: fmt.Sprintf(f, a...)}
}
func fixedRes(f string, a ...any) recResult {
	return recResult{status: recFixed, detail: fmt.Sprintf(f, a...)}
}
func wouldRes(f string, a ...any) recResult {
	return recResult{status: recWouldFix, detail: fmt.Sprintf(f, a...)}
}
func warnRes(f string, a ...any) recResult {
	return recResult{status: recWarn, detail: fmt.Sprintf(f, a...)}
}
func failRes(f string, a ...any) recResult {
	return recResult{status: recFailed, detail: fmt.Sprintf(f, a...)}
}
func noteRes(f string, a ...any) recResult {
	return recResult{status: recNote, detail: fmt.Sprintf(f, a...)}
}

func (r recResult) withFix(f string, a ...any) recResult {
	r.remedy = fmt.Sprintf(f, a...)
	return r
}

type reconciler struct {
	name string
	run  func(checkOnly bool) recResult
}

func reconcilers() []reconciler {
	return []reconciler{
		{i18n.T("interface language"), reconcileShellLanguage},
		{i18n.T("swap kept out of snapshots"), reconcileSwapSubvolume},
		{i18n.T("snapper configuration"), reconcileSnapper},
		{i18n.T("snapshot read access"), reconcileSnapperAccess},
		{i18n.T("snapshot cleanup"), reconcileSnapperCleanup},
		{i18n.T("limine boot menu layout"), reconcileLimineLayout},
		{i18n.T("limine boot entry"), reconcileLimineBootEntry},
		{i18n.T("alongside boot entry"), reconcileAlongsideBootEntry},
		{i18n.T("limine UKI boot tree"), reconcileLimineUKITree},
		{i18n.T("limine kernel boot images"), reconcileLimineKernelImages},
		{i18n.T("boot menu dead entries"), reconcileLimineDeadEntries},
		{i18n.T("boot partition headroom"), reconcileBootSpace},
		{i18n.T("boot volume writability"), reconcileBootRW},
		{i18n.T("initramfs GPU trim"), reconcileInitramfsGPUTrim},
		{i18n.T("limine autoboot"), reconcileLimineAutoboot},
		{i18n.T("limine snapshot sync"), reconcileLimineOSName},
		{i18n.T("updatedb snapshot prune"), reconcileUpdatedbPrune},
		{i18n.T("pacman database lock"), reconcilePacmanLock},
		{i18n.T("pacman progress bar"), reconcilePacmanCandy},
		{i18n.T("multilib repository"), reconcileMultilibRepo},
		{i18n.T("conflicting Ryoku files"), reconcileConflictingRyokuFiles},
		{i18n.T("stale update run-state"), reconcileStaleUpdateRun},
		{i18n.T("stale install crypt mapper"), reconcileStaleCryptMapper},
		{i18n.T("ryoku package channel"), reconcileRyokuChannel},
		{i18n.T("ryoku package database"), reconcileRyokuSyncDB},
		{i18n.T("boot guard"), reconcileBootGuard},
		{i18n.T("update channel checkout"), reconcileUpdateChannel},
		{i18n.T("update checkout pointer"), reconcileRepoPointer},
		{i18n.T("stale dev residue"), reconcileDevResidue},
		{i18n.T("ryostore cache location"), reconcileRyostoreCache},
		{i18n.T("desktop settings store"), reconcileDesktopStore},
		{i18n.T("retired cursor keys"), reconcileRetiredCursorLeaf},
		{i18n.T("session target units"), reconcileSessionTarget},
		{i18n.T("desktop session components"), reconcileSessionComponents},
		{i18n.T("desktop portal routing"), reconcilePortalRouting},
		{i18n.T("desktop portal session"), reconcilePortalSession},
		{i18n.T("audio service health"), reconcileAudioService},
		{i18n.T("audio playback routing"), reconcileAudioRouting},
		{i18n.T("keyboard layout code"), reconcileKbLayoutCode},
		{i18n.T("keyboard layout"), reconcileKeymap},
		{i18n.T("keyboard layout detection"), reconcileKeyboardSeed},
		{i18n.T("in-session lockscreen"), reconcileLockscreen},
		{i18n.T("cursor theme"), reconcileCursorTheme},
		{i18n.T("GTK session theme"), reconcileGtkSession},
		{i18n.T("Material Symbols icon font"), reconcileIconFont},
		{i18n.T("frame bar style name"), reconcileFrameBarsStyle},
		{i18n.T("shell config schema"), reconcileShellConfig},
		{i18n.T("login shell source"), reconcileLoginShell},
		{i18n.T("shell style knobs"), reconcileLegacyStyleKnobs},
		{i18n.T("sumi bar simplification"), reconcileSumiBar},
		{i18n.T("dock config store"), reconcileDockStore},
		{i18n.T("retired shell menus"), reconcileRetiredMenus},
		{i18n.T("retired wallpaper keys"), reconcileRetiredWallpaperKeys},
		{i18n.T("window width cycle"), reconcileWidthCycle},
		{i18n.T("ryogami wallpaper daemon"), reconcileRyogamiWallpaper},
		{i18n.T("ryowalls app leftovers"), reconcileRyowallsRemoval},
		{i18n.T("quick-settings capture tab"), reconcileCaptureModule},
		{i18n.T("quick-settings stage tab"), reconcileStageModule},
		{i18n.T("ryostage cache"), reconcileRyostageCache},
		{i18n.T("stage migration leftovers"), reconcileStageLeftovers},
		{i18n.T("retired system sidebar"), reconcileLegacySystemSidebar},
		{i18n.T("stash features sidebar anchor"), reconcileStashSidebar},
		{i18n.T("shipped app packages"), reconcileShippedApps},
		{i18n.T("release control manifest"), reconcileManifest},
		{i18n.T("ghostty theme include"), reconcileGhostty},
		{i18n.T("obsidian palette snippet"), reconcileObsidianSnippet},
		{i18n.T("flatpak app channel"), reconcileFlatpakRemote},
		{i18n.T("browser theme host"), reconcileBrowserTheme},
		{i18n.T("zen browser policy"), reconcileZen},
		{i18n.T("zen browser animations"), reconcileZenUserChrome},
		{i18n.T("launcher local-frost default"), reconcileLauncherLocalFrostDefault},
		{i18n.T("user edits overlay"), reconcileUserEdits},
		{i18n.T("default app map"), reconcileMimeDefaults},
		{i18n.T("keyring unlock policy"), reconcileKeyring},
		{i18n.T("SDDM greeter theme"), reconcileGreeterTheme},
		{i18n.T("SDDM greeter display server"), reconcileGreeterDisplayServer},
		{i18n.T("login screen cursor"), reconcileGreeterCursor},
		{i18n.T("fastfetch readout emblem"), reconcileFastfetchEmblem},
		{i18n.T("rice fastfetch emblem"), reconcileRiceEmblem},
		{i18n.T("ryotunes"), reconcileRyotunes},
		{i18n.T("fastfetch OS line"), reconcileFastfetchOSLine},
		{i18n.T("brand mark image"), reconcileBrandLogo},
		{i18n.T("decor art"), reconcileRyodecors},
		{i18n.T("Hyprland config integrity"), reconcileHyprlandConfig},
		{i18n.T("niri config integrity"), reconcileNiriConfig},
		{i18n.T("window manager plugin builds"), reconcileWmPlugins},
		{i18n.T("stale window-border pin"), reconcileBorderPin},
		{i18n.T("orphaned theme.lua"), reconcileThemeLua},
		{i18n.T("follow-mouse default"), reconcileFollowMouseDefault},
		{i18n.T("follow-wallpaper default"), reconcileThemeFollowDefault},
		{i18n.T("quickshell runtime"), reconcileQuickshell},
		{i18n.T("compositor config tree"), reconcileConfigTree},
		{i18n.T("desktop loads"), reconcileShellLoad},
		{i18n.T("ryoku shell daemon"), reconcileShellDaemon},
		{i18n.T("duplicate desktop instances"), reconcileShellInstances},
		{i18n.T("rashin agent daemon"), reconcileRashinDaemon},
		{i18n.T("AI usage collector timer"), reconcileAiUsageTimer},
		{i18n.T("prowl for rashin"), reconcileProwlAgent},
		{i18n.T("recordings directory"), reconcileRecordingsDir},
		{i18n.T("failed services"), reconcileFailedUnits},
		{i18n.T("btrfs device health"), reconcileBtrfsHealth},
		{i18n.T("wireless regulatory domain"), reconcileWifiRegdom},
		{i18n.T("ASUS Aura lighting provider"), reconcileAsusAura},
		{i18n.T("QMK/VIA keyboard lighting provider"), reconcileQMK},
		{i18n.T("display backlight"), reconcileBacklight},
		{i18n.T("discrete GPU idle drain"), reconcileDgpuPanel},
		{i18n.T("stale GPU render pin"), reconcileGpuPin},
		{i18n.T("power profiles vs AMD GPU"), reconcilePpdAmdgpu},
		{i18n.T("display resolution"), reconcileDisplayModes},
		{i18n.T("phantom Wayland output"), reconcilePhantomOutput},
		{i18n.T("fingerprint unlock module"), reconcileFingerprintModule},
		{i18n.T("Kepler NVIDIA recovery"), reconcileKeplerNvidia},
		{i18n.T("NVIDIA boot reliability"), reconcileNvidiaModeset},
		{i18n.T("NVIDIA update guard hook"), reconcileNvidiaGuardHook},
		{i18n.T("NVIDIA sleep units"), reconcileNvidiaSleepUnits},
		{i18n.T("pending config (.pacnew)"), reconcilePacnew},
		{i18n.T("orphaned packages"), reconcileOrphans},
	}
}

type finding struct {
	name string
	res  recResult
}

func runReconcilers(checkOnly bool) []finding {
	out := make([]finding, 0, len(reconcilers()))
	for _, r := range reconcilers() {
		out = append(out, finding{r.name, r.run(checkOnly)})
	}
	return out
}

// printFindings: one line per finding (+ its remedy). without verbose, only
// non-ok lines surface -- a healthy box is quiet. returns warn + fail counts.
func printFindings(fs []finding, verbose bool) (warns, fails int) {
	width := sys.TermWidth()
	printed := 0
	for _, f := range fs {
		switch f.res.status {
		case recWarn:
			warns++
		case recFailed:
			fails++
		}
		if !verbose && (f.res.status == recOK || f.res.status == recNote) {
			continue
		}
		printed++
		w := os.Stdout
		if f.res.status == recWarn || f.res.status == recFailed {
			w = os.Stderr
		}
		fmt.Fprintf(w, "  %s %s\n", statusGlyph(f.res.status), statusName(f))
		if f.res.detail != "" {
			fmt.Fprintln(w, detailStyle(f.res.status, sys.Wrap(f.res.detail, width, "      ")))
		}
		if f.res.remedy != "" && (f.res.status >= recWouldFix || f.res.status == recNote) {
			fmt.Fprintln(w, sys.Brand(sys.Wrap("↳ "+f.res.remedy, width, "      ")))
		}
	}
	if printed == 0 {
		fmt.Println("  " + sys.Green("✓") + " " + i18n.T("all checks passed"))
	}
	return warns, fails
}

func statusGlyph(s recStatus) string {
	switch s {
	case recOK, recFixed:
		return sys.Green("✓")
	case recNote:
		return sys.Dim("·")
	case recWouldFix:
		return sys.Amber("›")
	case recWarn:
		return sys.Amber("!")
	case recFailed:
		return sys.Red("✗")
	}
	return " "
}

func statusName(f finding) string {
	if f.res.status == recOK || f.res.status == recNote {
		return f.name
	}
	name := sys.Bold(f.name)
	if f.res.status == recFixed {
		name += sys.Dim(" " + i18n.T("(fixed)"))
	}
	return name
}

func detailStyle(s recStatus, text string) string {
	if s == recOK || s == recFixed || s == recNote {
		return sys.Dim(text)
	}
	return text
}

func doctorUsage() {
	fmt.Print(i18n.Tf("Usage: ryoku doctor [--check] [--verbose] [--report [file]] [--explain]\n\n  (no args)        check, and apply the safe automatic fixes\n  --check, -n      report what is wrong without changing anything\n  --verbose, -v    also list the checks that passed and advisory notes\n  --report [file]  write a shareable diagnostic report for the maintainers\n                   (default: %s)\n  --explain        ask your cloud model (Groq/OpenRouter) to reason over the report\n  --json           emit findings as JSON (read-only; powers the Hub System Check)\n", reportPath("")))
}

// Run: check, apply the safe fixes; on anything it can't fix, write a
// maintainer report so the user always has something to share.
func Run(args []string) error {
	checkOnly, wantReport, wantExplain, wantJSON, verboseFlag := false, false, false, false, false
	reportTo := ""
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--check", "-n", "--dry-run":
			checkOnly = true
		case "--report":
			wantReport = true
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				reportTo = args[i+1]
				i++
			}
		case "--explain":
			wantExplain = true
		case "--verbose", "-v":
			verboseFlag = true
		case "--json":
			wantJSON = true
		case "-h", "--help":
			doctorUsage()
			return nil
		default:
			return fmt.Errorf(i18n.T("unknown argument: %s (try --help)"), a)
		}
	}

	// read-only modes never mutate; showAll also lists ok + advisory notes.
	readOnly := checkOnly || wantReport || wantExplain || wantJSON
	showAll := readOnly || verboseFlag
	findings := runReconcilers(readOnly)
	if wantJSON {
		return emitFindingsJSON(findings)
	}
	warns, fails := printFindings(findings, showAll)

	if wantExplain {
		return explainFindings(findings)
	}

	if wantReport {
		path, err := writeReport(reportTo, findings)
		if err != nil {
			return fmt.Errorf(i18n.T("writing report: %w"), err)
		}
		fmt.Printf(i18n.T("\n  %s diagnostic report written to %s\n"), sys.Brand("➜"), path)
		fmt.Println("    " + sys.Dim(i18n.Tf("share it with the maintainers: %s", ryokuIssuesURL)))
		return nil
	}

	// couldn't fix everything. surface the AI option + a saved report so the
	// user always has a next step and a file to share.
	if warns+fails > 0 {
		path, _ := writeReport("", findings)
		noun := i18n.T("issue")
		if warns+fails > 1 {
			noun = i18n.T("issues")
		}
		fmt.Fprintf(os.Stderr, "\n  %s %s\n", sys.Brand("➜"), sys.Bold(fmt.Sprintf(i18n.T("found %d %s"), warns+fails, noun)))
		fmt.Fprintf(os.Stderr, "    %s  %s\n", sys.Brand("ryoku doctor --explain"), sys.Dim(i18n.T("AI diagnosis and a suggested fix")))
		if path != "" {
			fmt.Fprintf(os.Stderr, "    %s\n", sys.Dim(i18n.Tf("report saved: %s", path)))
		}
	}
	if fails > 0 {
		return fmt.Errorf(i18n.T("%d check(s) failed"), fails)
	}
	return nil
}

// findingJSON is the machine-readable shape of a reconciler result, emitted by
// `ryoku doctor --json` so the Hub can render a System Check without parsing
// the human output.
type findingJSON struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Remedy string `json:"remedy,omitempty"`
}

// emitFindingsJSON prints the findings as a JSON array on stdout.
func emitFindingsJSON(findings []finding) error {
	out := make([]findingJSON, 0, len(findings))
	for _, f := range findings {
		out = append(out, findingJSON{
			Name:   f.name,
			Status: f.res.status.label(),
			Detail: f.res.detail,
			Remedy: f.res.remedy,
		})
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

// ---- reconciler: swapfile out of snapshotted subvolumes ----------------------

// reconcileSwapSubvolume: a swapfile inside @ (the snapshotted root) gets
// moved into its own btrfs subvolume. btrfs can't snapshot a subvolume that
// holds an active swapfile, so the old installer layout made every snapper
// snapshot fail. auto-fix only on the exact old layout (one swapfile in a
// plain dir on btrfs); anything else is flagged for a human. no-op once it
// already sits in its own subvolume. skipped on machines that don't snapshot
// root.
func reconcileSwapSubvolume(checkOnly bool) recResult {
	if !sys.Exists("/etc/snapper/configs/root") {
		return okRes(i18n.T("root snapshots not configured, nothing to keep out of them"))
	}
	for _, sw := range activeSwapFiles() {
		dir := filepath.Dir(sw.path)
		if !sys.IsBtrfs(dir) || sys.IsBtrfsSubvolumeRoot(dir) {
			continue
		}
		if !dirOnlyContains(dir, filepath.Base(sw.path)) {
			return warnRes(i18n.T("swapfile %s blocks snapshots; %s holds other files"), sw.path, dir).
				withFix(i18n.T("move the swapfile into its own subvolume by hand"))
		}
		if checkOnly {
			return wouldRes(i18n.T("swapfile %s sits in snapshotted %s"), sw.path, dir).
				withFix(i18n.T("ryoku doctor (moves it into its own btrfs subvolume)"))
		}
		if err := relocateSwapToSubvolume(sw, dir); err != nil {
			return failRes(i18n.T("relocating %s: %v"), sw.path, err)
		}
		return fixedRes(i18n.T("moved %s into its own btrfs subvolume so snapshots work"), sw.path)
	}
	return okRes(i18n.T("swap is out of snapshots"))
}

// ---- reconciler: snapper configuration ---------------------------------------

// the snapper "root" config = the safety net behind every ryoku update: the
// pre/post snapshot pair plus the Limine boot-menu entries that make rollback
// work. the installer (installation/backend/lib/snapshots.sh) writes it, but a
// deploy box, an upgrade from an older release, or hand-edited drift can leave
// it missing -- and snapper proceeds silently when it is, so the user believes
// they have rollback when they don't. doctor restores the canonical layout on
// a btrfs root, warns honestly on a non-btrfs root (no snapshots there), stays
// idempotent on a healthy box.
//
// snapperRootConfig mirrors installation/backend/lib/snapshots.sh verbatim:
// keep the two in sync so a doctored box matches a fresh install.
const snapperRootConfig = `# Ryoku snapper config for the root filesystem. Written by ryoku doctor when
# the installer's config is missing (a deploy box, an upgrade from an older
# release, or drift). Keys not listed here fall back to snapper's built-in
# defaults.
SUBVOLUME="/"
FSTYPE="btrfs"
QGROUP=""
SPACE_LIMIT="0.5"
FREE_LIMIT="0.2"
ALLOW_USERS=""
ALLOW_GROUPS=""
SYNC_ACL="no"
BACKGROUND_COMPARISON="yes"
NUMBER_CLEANUP="yes"
NUMBER_MIN_AGE="1800"
NUMBER_LIMIT="10"
NUMBER_LIMIT_IMPORTANT="10"
TIMELINE_CREATE="no"
TIMELINE_CLEANUP="yes"
TIMELINE_MIN_AGE="1800"
TIMELINE_LIMIT_HOURLY="10"
TIMELINE_LIMIT_DAILY="7"
TIMELINE_LIMIT_WEEKLY="0"
TIMELINE_LIMIT_MONTHLY="0"
TIMELINE_LIMIT_YEARLY="0"
EMPTY_PRE_POST_CLEANUP="yes"
EMPTY_PRE_POST_MIN_AGE="1800"
`

const snapperConfdRoot = `## Path: System/Snapper
## Type: string
## Default: ""
# Snapper configs the systemd units and pacman hooks operate on.
SNAPPER_CONFIGS="root"
`

// snapperOutcome: what planSnapper hands to reconcileSnapper. ok = leave the
// box alone; the two warn variants surface as-is; create writes the canonical
// layout.
type snapperOutcome int

const (
	snapperOK snapperOutcome = iota
	snapperWarnNotBtrfs
	snapperWarnInconsistent
	snapperCreate
	snapperWarnMissingPkgs
	snapperOptedOut
)

// snapperState: the slice of the filesystem reconcileSnapper looks at, lifted
// to a value so planSnapper is unit-testable without real /etc or running
// snapper/btrfs.
type snapperState struct {
	rootIsBtrfs         bool
	configExists        bool
	optedOut            bool
	snapshotsExists     bool
	snapshotsIsSubvol   bool
	snapshotsMode       os.FileMode
	confdExists         bool
	confdContents       string
	snapperInstalled    bool
	snapPacInstalled    bool
	limineInstalled     bool
	limineSyncInstalled bool
	limineSyncEnabled   bool
}

// planSnapper picks the branch from observed state. pure, no IO. the
// "configured" branch runs the same consistency checks the old reconciler
// did, so a healthy box still reads ok.
func planSnapper(s snapperState) (snapperOutcome, []string) {
	if !s.configExists {
		if !s.rootIsBtrfs {
			return snapperWarnNotBtrfs, nil
		}
		// the installer records an explicit "no snapshots" choice
		// (RYOKU_SUBVOL_SNAPSHOTS=0) as /etc/ryoku/snapshots-disabled; creating
		// the layout anyway would silently revert it on the first update. an
		// existing config always wins over a stale marker.
		if s.optedOut {
			return snapperOptedOut, nil
		}
		if !s.snapperInstalled {
			return snapperWarnMissingPkgs, nil
		}
		return snapperCreate, nil
	}
	var problems []string
	if s.snapshotsExists && !s.snapshotsIsSubvol {
		problems = append(problems, i18n.T("/.snapshots is a plain directory, not a btrfs subvolume"))
	}
	if s.snapshotsExists && s.snapshotsMode != 0o750 {
		problems = append(problems, fmt.Sprintf(i18n.T("/.snapshots is mode %04o, expected 0750"), s.snapshotsMode))
	}
	if s.confdExists && !strings.Contains(s.confdContents, "root") {
		problems = append(problems, i18n.T("/etc/conf.d/snapper does not list the root config (timers and hooks will skip it)"))
	}
	if !s.snapperInstalled {
		problems = append(problems, i18n.T("snapper is not installed; the root config exists but cannot be used (sudo pacman -S snapper)"))
	}
	if !s.snapPacInstalled {
		problems = append(problems, i18n.T("snap-pac is not installed, so pacman transactions are not auto-snapshotted (sudo pacman -S snap-pac)"))
	}
	// only meaningful under Limine; a GRUB box (converted CachyOS and the
	// like) is healthy without it and must not warn forever.
	if s.limineInstalled {
		if !s.limineSyncInstalled {
			problems = append(problems, i18n.T("limine-snapper-sync is not installed, so snapshots are not in the Limine boot menu (ryoku-pkg-aur-add limine-snapper-sync)"))
		} else if !s.limineSyncEnabled {
			problems = append(problems, i18n.T("limine-snapper-sync.service is disabled, so new snapshots never reach the Limine boot menu (sudo systemctl enable --now limine-snapper-sync.service)"))
		}
	}
	if len(problems) == 0 {
		return snapperOK, nil
	}
	return snapperWarnInconsistent, problems
}

// gatherSnapperState reads /etc + /.snapshots into a snapperState. all
// non-privileged stats and world-readable files; privileged writes happen
// later in the create branch under sudo, like every other reconciler.
func gatherSnapperState() snapperState {
	s := snapperState{
		rootIsBtrfs:         sys.IsBtrfs("/"),
		configExists:        sys.Exists("/etc/snapper/configs/root"),
		optedOut:            sys.Exists("/etc/ryoku/snapshots-disabled"),
		snapperInstalled:    sys.Has("snapper"),
		snapPacInstalled:    sys.PkgInstalled("snap-pac"),
		limineInstalled:     sys.PkgInstalled("limine"),
		limineSyncInstalled: sys.PkgInstalled("limine-snapper-sync"),
		limineSyncEnabled:   sys.UnitEnabled("limine-snapper-sync.service"),
	}
	if fi, err := os.Stat("/.snapshots"); err == nil {
		s.snapshotsExists = true
		s.snapshotsMode = fi.Mode().Perm()
		s.snapshotsIsSubvol = sys.IsBtrfsSubvolumeRoot("/.snapshots")
	}
	if b, err := os.ReadFile("/etc/conf.d/snapper"); err == nil {
		s.confdExists = true
		s.confdContents = string(b)
	}
	return s
}

// reconcileSnapper converges the snapper "root" config. btrfs root + no config
// -> write the canonical installer layout. non-btrfs root -> warn honestly
// instead of silently ok. healthy box -> consistency checks gate "ok".
func reconcileSnapper(checkOnly bool) recResult {
	st := gatherSnapperState()
	outcome, problems := planSnapper(st)
	switch outcome {
	case snapperWarnNotBtrfs:
		return warnRes(i18n.T("root filesystem is not btrfs; snapshot and rollback are unavailable on this machine"))
	case snapperWarnMissingPkgs:
		return warnRes(i18n.T("root is btrfs but snapper is not installed; snapshots and rollback are off")).
			withFix(i18n.T("sudo pacman -S snapper snap-pac, then ryoku doctor"))
	case snapperOptedOut:
		return okRes(i18n.T("snapshots were declined at install (/etc/ryoku/snapshots-disabled); delete the marker and run `ryoku doctor` to enable them"))
	case snapperCreate:
		if checkOnly {
			return wouldRes(i18n.T("snapper root config is missing; snapshots and rollback are off")).
				withFix(i18n.T("ryoku doctor (creates the snapper root config)"))
		}
		return createSnapperRootConfig(st)
	case snapperWarnInconsistent:
		return warnRes("%s", strings.Join(problems, "; ")).
			withFix(i18n.T("see https://wiki.archlinux.org/title/Snapper"))
	}
	return okRes(i18n.T("snapper root config is consistent"))
}

// reconcileSnapperAccess grants the primary user read access to the root
// snapshots so `ryoku status` (and the Hub panel, the update island) can count
// them without sudo -- snapper's own ALLOW_USERS + SYNC_ACL mechanism. Without
// it the count came from a `sudo -n` that fails whenever no credential is cached,
// and a failed read rendered as a bare "0", which reads as "no safety net" when
// the safety net is fine. Idempotent and gated on whether an unprivileged read
// already works, so it never has to read the 0640 config to know.
func reconcileSnapperAccess(checkOnly bool) recResult {
	if !sys.Has("snapper") || !sys.Exists("/etc/snapper/configs/root") {
		return okRes(i18n.T("root snapshots not configured, no access to grant"))
	}
	user := doctorUser()
	if user == "" || user == "root" {
		return okRes(i18n.T("no primary user to grant snapshot access to"))
	}
	if snapperReadableUnprivileged() {
		return okRes(i18n.T("snapshots are readable without sudo"))
	}
	if checkOnly {
		return wouldRes(i18n.T("`ryoku status` cannot count snapshots without a cached sudo credential, so it can read as \"0\" on a machine that has them")).
			withFix(i18n.T("grant %s read access (snapper ALLOW_USERS + SYNC_ACL)"), user)
	}
	if err := sys.Run("sudo", "snapper", "-c", "root", "set-config",
		"ALLOW_USERS="+user, "SYNC_ACL=yes"); err != nil {
		return failRes(i18n.T("granting %s snapshot read access: %v"), user, err).
			withFix(i18n.T("sudo snapper -c root set-config ALLOW_USERS=%s SYNC_ACL=yes"), user)
	}
	if !snapperReadableUnprivileged() {
		return warnRes(i18n.T("granted %s snapshot access, but an unprivileged read still fails; the ACL sync may land on the next snapshot"), user)
	}
	return fixedRes(i18n.T("granted %s read access to snapshots (ALLOW_USERS, SYNC_ACL); `ryoku status` counts them without sudo now"), user)
}

// snapperReadableUnprivileged reports whether the current user can list the root
// snapshots without escalating -- i.e. the ALLOW_USERS + SYNC_ACL grant is in
// effect. It runs snapper directly (no sudo), so it never prompts. A denied read
// prints "No permissions." to stderr but still exits 0, so success is judged by
// a real CSV listing on stdout (its header), not the exit code.
func snapperReadableUnprivileged() bool {
	out, err := sys.RunOut("snapper", "-c", "root", "--csvout", "list", "--columns", "number")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(out, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			return strings.HasPrefix(s, "number")
		}
	}
	return false
}

// createSnapperRootConfig lays the installer's layout down on a live box.
// mirrors installation/backend/lib/snapshots.sh:
//   - /.snapshots = btrfs subvolume, owned root:root, mode 0750.
//   - write /etc/snapper/configs/root.
//   - register "root" in /etc/conf.d/snapper without dropping siblings.
//   - best-effort enable snapper-cleanup.timer (+ limine-snapper-sync.service
//     when its unit is present).
//
// a pre-existing plain-directory /.snapshots is left to a human: it might
// hold user data, and the risk of rmdir clobbering it isn't worth saving the
// extra command.
func createSnapperRootConfig(st snapperState) recResult {
	var actions []string

	switch {
	case !st.snapshotsExists:
		if err := sys.Run("sudo", "btrfs", "subvolume", "create", "/.snapshots"); err != nil {
			return failRes(i18n.T("creating /.snapshots subvolume: %v"), err).
				withFix(i18n.T("sudo btrfs subvolume create /.snapshots, then re-run ryoku doctor"))
		}
		actions = append(actions, "/.snapshots subvolume")
	case !st.snapshotsIsSubvol:
		return warnRes(i18n.T("/.snapshots exists as a plain directory; remove or convert it before the snapper config can be created")).
			withFix(i18n.T("inspect /.snapshots, then `sudo rmdir /.snapshots && sudo btrfs subvolume create /.snapshots` and re-run ryoku doctor"))
	}

	if err := sys.Run("sudo", "chmod", "0750", "/.snapshots"); err != nil {
		return failRes(i18n.T("chmod /.snapshots: %v"), err)
	}
	if err := sys.Run("sudo", "chown", "root:root", "/.snapshots"); err != nil {
		return failRes(i18n.T("chown /.snapshots: %v"), err)
	}

	if err := writeRootFile("/etc/snapper/configs/root", snapperRootConfig, "0640"); err != nil {
		return failRes(i18n.T("writing /etc/snapper/configs/root: %v"), err)
	}
	actions = append(actions, "/etc/snapper/configs/root")

	newConfd, changed := mergedConfdRoot(st.confdExists, st.confdContents)
	if changed {
		if err := writeRootFile("/etc/conf.d/snapper", newConfd, "0644"); err != nil {
			return failRes(i18n.T("writing /etc/conf.d/snapper: %v"), err)
		}
		actions = append(actions, "/etc/conf.d/snapper")
	}

	// services: best-effort. a healthy install has both; an offline AUR install
	// can be missing limine-snapper-sync, and a failure here doesn't undo the
	// config we just wrote.
	_ = sys.Run("sudo", "systemctl", "enable", "--now", "snapper-cleanup.timer")
	if sys.Exists("/usr/lib/systemd/system/limine-snapper-sync.service") {
		_ = sys.Run("sudo", "systemctl", "enable", "--now", "limine-snapper-sync.service")
	}

	return fixedRes(i18n.T("created snapper root config: %s"), strings.Join(actions, ", "))
}

// mergedConfdRoot returns the desired /etc/conf.d/snapper contents + whether
// they differ from the current file. missing file -> canonical snippet.
// SNAPPER_CONFIGS already lists root -> unchanged. SNAPPER_CONFIGS lists other
// configs -> append "root", keep them. file present, no SNAPPER_CONFIGS line
// -> add one. split-on-whitespace tolerates both "a b" and single-name styles
// snapper accepts in the wild.
func mergedConfdRoot(present bool, current string) (string, bool) {
	if !present {
		return snapperConfdRoot, true
	}
	lines := strings.Split(current, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "SNAPPER_CONFIGS=") {
			continue
		}
		value := strings.Trim(strings.TrimPrefix(trimmed, "SNAPPER_CONFIGS="), `"`)
		configs := strings.Fields(value)
		for _, c := range configs {
			if c == "root" {
				return current, false
			}
		}
		configs = append(configs, "root")
		lines[i] = fmt.Sprintf(`SNAPPER_CONFIGS="%s"`, strings.Join(configs, " "))
		return strings.Join(lines, "\n"), true
	}
	if current != "" && !strings.HasSuffix(current, "\n") {
		current += "\n"
	}
	return current + `SNAPPER_CONFIGS="root"` + "\n", true
}

// writeRootFile: stage contents in a temp file, then `sudo install -D` into
// place at the given mode, owned root:root, so a regular-user `ryoku doctor`
// still converges /etc. install -D makes the parent dir in one shot, same
// pattern as the other privileged reconcilers that go through sudo.
func writeRootFile(path, contents, mode string) error {
	tmp, err := os.CreateTemp("", "ryoku-snapper-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(contents); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return sys.Run("sudo", "install", "-D", "-m", mode, "-o", "root", "-g", "root", tmp.Name(), path)
}

// markMigration records a one-shot seed under the state dir, atomically, so a
// reconciler that plants a default plants it exactly once and a later removal
// by the user stands.
func markMigration(marker string) error {
	dir := filepath.Dir(marker)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(marker)+".ryoku-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write([]byte("done\n")); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, marker)
}

// ---- reconciler: stale pacman lock -------------------------------------------

func reconcilePacmanLock(checkOnly bool) recResult {
	const lock = "/var/lib/pacman/db.lck"
	if !sys.Exists(lock) {
		return okRes(i18n.T("no stale pacman lock"))
	}
	if processRunning("pacman") {
		return okRes(i18n.T("pacman is running; lock is in use"))
	}
	if checkOnly {
		return wouldRes(i18n.T("stale pacman lock present (no pacman running)")).withFix("sudo rm %s", lock)
	}
	if err := sys.Run("sudo", "rm", "-f", lock); err != nil {
		return failRes(i18n.T("could not remove stale lock: %v"), err).withFix("sudo rm %s", lock)
	}
	return fixedRes(i18n.T("removed stale pacman lock"))
}

// ---- reconciler: stale update run-state --------------------------------------

// reconcileStaleUpdateRun clears the run-state file a crashed `ryoku update`
// left in "running" (or an unanswered "prompt"): the shell's update island and
// the Hub keep rendering that phantom run for the rest of the session. A live
// `ryoku update` (stage 1 or --stage2) owns the file and is left alone; so is
// the update this doctor may itself be running inside (the process match).
// updateProcessLive: is a `ryoku update` (stage 1 or --stage2) running right
// now? A package var so tests can stub it: a real pgrep scan is neither
// hermetic (a dev's live update flips the result) nor guaranteed cheap.
var updateProcessLive = func() bool {
	return exec.Command("pgrep", "-f", "ryoku update").Run() == nil
}

func reconcileStaleUpdateRun(checkOnly bool) recResult {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = "/tmp"
	}
	path := filepath.Join(dir, "ryoku-update.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return okRes(i18n.T("no update run-state"))
	}
	var st struct {
		Phase string `json:"phase"`
	}
	if json.Unmarshal(b, &st) != nil || (st.Phase != "running" && st.Phase != "prompt") {
		return okRes(i18n.T("update run-state is settled"))
	}
	if updateProcessLive() {
		return okRes(i18n.T("an update is running; run-state is live"))
	}
	if checkOnly {
		return wouldRes(i18n.T("update island stuck on a crashed run (phase %s)"), st.Phase).withFix("rm %s", path)
	}
	idle := []byte(`{"phase":"idle"}`)
	if os.WriteFile(path+".tmp", idle, 0o644) == nil && os.Rename(path+".tmp", path) == nil {
		return fixedRes(i18n.T("cleared a crashed update's run-state"))
	}
	if err := os.Remove(path); err != nil {
		return failRes(i18n.T("could not clear the stale run-state: %v"), err).withFix("rm %s", path)
	}
	return fixedRes(i18n.T("cleared a crashed update's run-state"))
}

// ---- reconciler: stale install crypt mapper ----------------------------------

// reconcileStaleCryptMapper clears a /dev/mapper/root the installer left open.
// ryoku-install opens the encrypted root under that name; a failed run (or a
// retry) leaves it held, so the next `cryptsetup open ... root` aborts with
// "Device root already exists". Closing the orphan frees the name.
// Safe: a "root" node backing a live mount (the running root) is never touched,
// only a true orphan with no mount; closing a LUKS mapper only re-locks it.
func reconcileStaleCryptMapper(checkOnly bool) recResult {
	nodes := cryptMapperNodes()
	if len(nodes) == 0 {
		return okRes(i18n.T("no crypt mappers present"))
	}
	stale := staleInstallMapper(nodes, mountSourceOf("/"), mountedSources())
	if stale == "" {
		return okRes(i18n.T("no orphaned install crypt mapper"))
	}
	node := "/dev/mapper/" + stale
	if checkOnly {
		return wouldRes(i18n.T("orphaned crypt mapper %s from a failed install blocks `cryptsetup open ... %s`"), node, stale).
			withFix("sudo cryptsetup close %s", stale)
	}
	if err := sys.Run("sudo", "cryptsetup", "close", stale); err != nil {
		return failRes(i18n.T("could not close orphaned crypt mapper %s: %v"), node, err).
			withFix("sudo cryptsetup close %s", stale)
	}
	return fixedRes(i18n.T("closed orphaned crypt mapper %s left by a failed install"), node)
}

// staleInstallMapper returns "root" only when /dev/mapper/root is a true orphan:
// present, not backing "/", and holding no mount. pure, so the safety gate is
// testable without device-mapper.
func staleInstallMapper(cryptNodes []string, rootSource string, mountedSources map[string]bool) string {
	const node = "/dev/mapper/root"
	present := false
	for _, m := range cryptNodes {
		if m == node {
			present = true
			break
		}
	}
	if !present {
		return ""
	}
	if rootSource == node || mountedSources[node] {
		return "" // backs a live mount: the running root or a mounted target
	}
	return "root"
}

// cryptMapperNodes lists the device-mapper nodes of type crypt as
// /dev/mapper/<name>. empty when dmsetup is absent, unprivileged, or finds none.
func cryptMapperNodes() []string {
	out, err := sys.RunOut("dmsetup", "ls", "--target", "crypt")
	if err != nil {
		return nil
	}
	return parseCryptMapperNodes(out)
}

// parseCryptMapperNodes maps `dmsetup ls --target crypt` lines to /dev/mapper
// paths; "No devices found" yields none.
func parseCryptMapperNodes(out string) []string {
	var nodes []string
	for _, ln := range nonEmptyLines(out) {
		f := strings.Fields(ln)
		if len(f) == 0 || f[0] == "No" {
			continue
		}
		nodes = append(nodes, "/dev/mapper/"+f[0])
	}
	return nodes
}

// mountSourceOf returns the bare backing device of a mountpoint, btrfs
// subvolume suffix stripped (/dev/mapper/root[/@] -> /dev/mapper/root).
func mountSourceOf(path string) string {
	out, _ := sys.RunOut("findmnt", "-n", "-o", "SOURCE", path)
	return baseSource(out)
}

// mountedSources is the set of block devices with a current mount, subvolume
// suffixes stripped, so a crypt mapper backing any live mount is recognizable.
func mountedSources() map[string]bool {
	m := map[string]bool{}
	out, _ := sys.RunOut("findmnt", "-rn", "-o", "SOURCE")
	for _, ln := range nonEmptyLines(out) {
		if s := baseSource(ln); s != "" {
			m[s] = true
		}
	}
	return m
}

// baseSource trims findmnt's btrfs subvolume suffix:
// "/dev/mapper/root[/@home]" -> "/dev/mapper/root".
func baseSource(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '['); i >= 0 {
		s = s[:i]
	}
	return s
}

// ---- reconciler: ryoku package channel + keyring -----------------------------

// ryokuRepoStanza is the [ryoku] block the installer appends to pacman.conf
// (installation/backend/lib/deploy.sh ryoku_repo_pacman_conf). doctor re-adds
// this exact block when a pacnew merge or a hand-edit drops it, so a package box
// does not silently fall off the update channel.
const ryokuRepoStanza = "\n[ryoku]\nSigLevel = Required\nServer = " + sys.RepoBase + "/$arch\n"

func reconcileRyokuChannel(checkOnly bool) recResult {
	if !sys.PkgInstalled("ryoku-desktop") {
		return okRes(i18n.T("not a packaged install (desktop runs from a checkout)"))
	}
	conf, err := os.ReadFile("/etc/pacman.conf")
	if err != nil {
		return warnRes(i18n.T("could not read /etc/pacman.conf: %v"), err)
	}
	if !sys.PkgInstalled("ryoku-keyring") {
		// without the trusted key a Required repo fails every fetch, and the key
		// only comes from the package, so adding the stanza alone would not help.
		return warnRes(i18n.T("ryoku-keyring is missing, so the [ryoku] repo cannot be trusted; updates will fail signature checks")).
			withFix(i18n.T("sudo pacman -S ryoku-keyring, then run ryoku doctor"))
	}
	if strings.Contains(string(conf), "[ryoku]") {
		// the Server line is the channel. name it, and flag a server Ryoku
		// does not publish without touching it: a local build-repo.sh tree or
		// a private mirror is deliberate, but it means no release reaches here.
		switch ch := sys.ChannelOfServer(sys.RyokuServer()); {
		case ch == sys.ChannelStable:
			return okRes(i18n.T("ryoku channel: stable packages (named releases); `ryoku track unstable-dev` follows testing"))
		case ch == sys.ChannelTesting:
			return okRes(i18n.T("ryoku channel: testing packages (rebuilt on every unstable-dev push); `ryoku track main` returns to stable releases"))
		case sys.IsReleaseTag(ch):
			return okRes(i18n.T("ryoku channel: pinned to release %s (packages); `ryoku track main` follows releases again"), ch)
		default:
			return warnRes(i18n.T("the [ryoku] repo points at %s, which Ryoku does not publish; releases will not arrive from it"), sys.RyokuServer()).
				withFix("ryoku track main")
		}
	}
	// the keyring is here but the repo stanza is gone (a pacnew merge or a
	// hand-edit dropped it): re-add it so `ryoku update` reaches the package
	// channel again, instead of stranding the box with no way to get updates.
	if checkOnly {
		return wouldRes(i18n.T("ryoku-desktop is installed but the [ryoku] repo is not in pacman.conf; updates will not arrive")).
			withFix("ryoku doctor")
	}
	if err := writeRootFile("/etc/pacman.conf", string(conf)+ryokuRepoStanza, "0644"); err != nil {
		return warnRes(i18n.T("the [ryoku] repo is missing from pacman.conf and could not be re-added: %v"), err).
			withFix(i18n.T("add the [ryoku] repo by hand (see docs/development.md)"))
	}
	// a reset trustdb would still fail Required even with the keyring installed;
	// re-populating is idempotent and cheap.
	_ = sys.Sudo("pacman-key", "--populate", "ryoku")
	return fixedRes(i18n.T("re-added the [ryoku] repo to pacman.conf so updates arrive again"))
}

// ---- reconciler: ryoku sync database health ----------------------------------

// reconcileRyokuSyncDB heals a cached [ryoku] sync db wedged against its
// signature. The db is non-reproducible and its detached .sig is fetched fresh
// on every refresh even when the db itself is unchanged (a 304), so any box
// that syncs while the mirror briefly serves a db and .sig from different
// builds caches a mismatched pair. pacman then rejects it -- "invalid or
// corrupted database (PGP signature)" -- on every transaction, and -Sy will not
// replace a db it thinks is current, so `pacman -S <anything>` stays wedged
// until the cache is dropped. `ryoku update` self-heals on the spot; this
// catches a box a user only ever drives through plain pacman. Detection is
// read-only (`pacman -Sl` loads and verifies the cached db); the fix drops it
// and pulls a fresh, matched pair.
func reconcileRyokuSyncDB(checkOnly bool) recResult {
	if !sys.PkgInstalled("ryoku-desktop") {
		return okRes(i18n.T("not a packaged install (desktop runs from a checkout)"))
	}
	// a missing key is a keyring problem the channel reconciler owns; dropping
	// the db would only refetch one that fails the same way.
	if !sys.PkgInstalled("ryoku-keyring") || sys.RyokuServer() == "" {
		return okRes(i18n.T("[ryoku] repo or keyring absent; nothing to verify"))
	}
	if !sys.Exists("/var/lib/pacman/sync/ryoku.db") {
		return okRes(i18n.T("no cached [ryoku] sync db to check"))
	}
	out, err := sys.RunOut("sh", "-c", "LC_ALL=C pacman -Sl ryoku 2>&1")
	if err == nil {
		return okRes(i18n.T("the [ryoku] sync db loads and verifies"))
	}
	if !strings.Contains(out, "invalid or corrupted database") && !strings.Contains(out, "signature") {
		// another failure (offline, transient); not the stale-signature wedge.
		return okRes(i18n.T("the [ryoku] sync db is present; no signature wedge"))
	}
	if checkOnly {
		return wouldRes(i18n.T("the cached [ryoku] sync db no longer matches its signature; every pacman transaction fails until it is refreshed")).
			withFix("ryoku doctor")
	}
	if err := sys.DropRyokuSyncDB(); err != nil {
		return failRes(i18n.T("could not drop the stale [ryoku] sync db: %v"), err).
			withFix("sudo rm -f /var/lib/pacman/sync/ryoku.* && sudo pacman -Syy")
	}
	// the db is gone now, so a plain -Sy pulls a fresh, matched pair; best-effort
	// so an offline box simply refetches on its next update.
	_ = sys.Sudo("pacman", "-Sy", "--noconfirm")
	return fixedRes(i18n.T("dropped the stale [ryoku] sync db so pacman refetches a matched db and signature"))
}

// ---- reconciler: Material Symbols icon font ------------------------------------

// reconcileIconFont converges the icon font onto boxes that predate it being a
// ryoku-desktop dependency: every shell glyph is a Material Symbols ligature
// (MaterialIcon.qml), so without the font each icon renders as its name in
// plain text ("network_wifi"). the package depend heals packaged boxes on
// their next full update; this heals git-channel boxes and anyone already
// broken today.
func reconcileIconFont(checkOnly bool) recResult {
	if wm.Detect().Name == "" {
		return okRes(i18n.T("no window manager provider"))
	}
	if anyPkgInstalled("ttf-material-symbols-variable", "ttf-material-symbols-variable-git") {
		return okRes(i18n.T("Material Symbols icon font installed"))
	}
	if checkOnly {
		return wouldRes(i18n.T("Material Symbols font missing; every shell icon renders as its ligature name")).
			withFix(i18n.T("ryoku doctor installs ttf-material-symbols-variable"))
	}
	if err := sys.Sudo("pacman", "-S", "--needed", "--noconfirm", "ttf-material-symbols-variable"); err != nil {
		return failRes(i18n.T("could not install ttf-material-symbols-variable: %v"), err).
			withFix("sudo pacman -S ttf-material-symbols-variable")
	}
	return fixedRes(i18n.T("installed the Material Symbols icon font; `ryoku reload` picks it up"))
}

// ---- reconciler: stale dev/recovery residue ------------------------------------

// reconcileDevResidue clears home-installed Ryoku artifacts off a packaged box.
// deploy.sh (the dev loop, and `ryoku recovery`) installs binaries into
// ~/.local/bin and QML modules into ~/.local/lib/qt6/qml; both outrank the
// packaged copies on PATH and the QML import path, so once the box is back on
// the pacman channel the leftovers pin it to whatever vintage last deployed
// them and every later package update is silently shadowed. a checkout box
// (git channel) IS the dev loop: left alone.
func reconcileDevResidue(checkOnly bool) recResult {
	if sys.ResolveRepo() != "" {
		return okRes(i18n.T("checkout box; home-deployed artifacts are the live desktop"))
	}
	if !sys.PkgInstalled("ryoku-desktop") {
		return okRes(i18n.T("not a packaged install"))
	}
	var residue []string
	if qml := filepath.Join(sys.Home(), ".local", "lib", "qt6", "qml", "Ryoku"); sys.Exists(qml) {
		residue = append(residue, qml)
	}
	// every ~/.local/bin entry that shadows a Ryoku-packaged /usr/bin binary.
	// deploy.sh installs a wide, release-dependent set (shell, livewall, CLI,
	// hub, rashin, hardware helpers, app bins), so pacman is the manifest: a
	// home copy of anything a ryoku package ships is residue. A fixed name
	// list here once missed ryoku-livewall and the helpers, leaving a stale
	// player and tools shadowing every later update. Anything the packages
	// never shipped (a user's own script) has no ryoku-owned /usr/bin twin
	// and is left alone; a wrapper deliberately named after a packaged ryoku
	// tool is treated as residue too -- doctor owns converging the packaged
	// toolchain, and an intercepted tool is exactly the drift it heals.
	localBin := filepath.Join(sys.Home(), ".local", "bin")
	entries, _ := os.ReadDir(localBin)
	for _, e := range entries {
		usr := "/usr/bin/" + e.Name()
		if !sys.Exists(usr) {
			continue
		}
		owner, err := sys.RunOut("pacman", "-Qoq", usr)
		if err != nil || !strings.HasPrefix(strings.TrimSpace(owner), "ryoku") {
			continue
		}
		residue = append(residue, filepath.Join(localBin, e.Name()))
	}
	if len(residue) == 0 {
		return okRes(i18n.T("no home-deployed artifacts shadowing the packages"))
	}
	if checkOnly {
		return wouldRes(i18n.T("stale home-deployed artifacts shadow the packaged install: %s"), strings.Join(residue, ", ")).
			withFix(i18n.T("ryoku doctor removes them; the packaged copies take over on the next reload"))
	}
	// report what could not be removed: a survivor keeps shadowing the packaged
	// install (Hyprland's autostart relaunches it by PATH at next login), so
	// claiming "removed" while it lives would hide the very drift this heals.
	var kept []string
	for _, p := range residue {
		if err := os.RemoveAll(p); err != nil {
			kept = append(kept, p)
		}
	}
	if len(kept) > 0 {
		return failRes(i18n.T("could not remove home-deployed artifact(s) still shadowing the packaged install: %s"), strings.Join(kept, ", ")).
			withFix(i18n.T("remove them by hand (check ownership/permissions), then `ryoku reload`"))
	}
	return fixedRes(i18n.T("removed %d home-deployed artifact(s) shadowing the packaged install; `ryoku reload` switches to the packaged shell"), len(residue))
}

// ---- reconciler: ryostore cache directory ------------------------------------

// reconcileRyostoreCache migrates the fetched-catalogue cache from its former
// ryoku/extras location to ryoku/ryostore after the ryostore rename. It is a
// same-filesystem rename, so the archive survives with no re-download and the
// store still renders offline right after the upgrade. Idempotent: it acts only
// when the old directory exists and the new one does not.
func reconcileRyostoreCache(checkOnly bool) recResult {
	cacheHome := os.Getenv("XDG_CACHE_HOME")
	if cacheHome == "" {
		cacheHome = filepath.Join(sys.Home(), ".cache")
	}
	old := filepath.Join(cacheHome, "ryoku", "extras")
	cur := filepath.Join(cacheHome, "ryoku", "ryostore")
	if !sys.Exists(old) || sys.Exists(cur) {
		return okRes(i18n.T("ryostore cache is on the current path"))
	}
	if checkOnly {
		return wouldRes(i18n.T("the store cache still lives at the pre-rename %s"), old).
			withFix(i18n.T("ryoku doctor moves it to %s"), cur)
	}
	if err := os.MkdirAll(filepath.Dir(cur), 0o755); err != nil {
		return failRes(i18n.T("could not create %s: %v"), filepath.Dir(cur), err)
	}
	if err := os.Rename(old, cur); err != nil {
		return failRes(i18n.T("could not move %s to %s: %v"), old, cur, err)
	}
	return fixedRes(i18n.T("moved the store cache from %s to %s"), old, cur)
}

// ---- reconciler: shell config schema -------------------------------------------

func reconcileShellConfig(checkOnly bool) recResult {
	path := filepath.Join(sys.ConfigHome(), "ryoku", "shell.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return okRes(i18n.T("no shell.json yet (seeded on first shell run)"))
	}
	migrated, changes, err := migrateShellConfig(raw)
	if err != nil {
		return warnRes(i18n.T("shell.json does not parse (%v); the shell falls back to defaults"), err).
			withFix(i18n.T("delete %s to re-seed it"), path)
	}
	if len(changes) == 0 {
		return okRes(i18n.T("shell.json is on the current schema"))
	}
	if checkOnly {
		return wouldRes(i18n.T("shell.json carries retired state: %s"), strings.Join(changes, "; ")).
			withFix(i18n.T("ryoku doctor migrates it in place"))
	}
	tmp := path + ".ryoku-tmp"
	if err := os.WriteFile(tmp, migrated, 0o644); err != nil {
		return failRes(i18n.T("could not write %s: %v"), tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return failRes(i18n.T("could not replace %s: %v"), path, err)
	}
	return fixedRes(i18n.T("migrated shell.json to the current schema: %s"), strings.Join(changes, "; "))
}

// ---- reconciler: dock config store -------------------------------------------

// reconcileDockStore moves the retired dock knobs out of the qsbar map and into
// the top-level `dock` object the shell reads now that the dock is its own
// shell surface for every bar style. Only the five keys a box could have
// persisted move (enabled, magnify, pinned, frost, shadow); the rest of the
// dock object (edge, autohide, labels, media) is left absent so Config.qml's
// defaults apply. Surgical and idempotent: a dock object the shell already
// wrote is never clobbered, the old keys are dropped from qsbar, and a store
// with none of them is left alone.
func reconcileDockStore(checkOnly bool) recResult {
	path := filepath.Join(sys.ConfigHome(), "ryoku", "shell.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return okRes(i18n.T("no shell.json yet (seeded on first shell run)"))
	}
	migrated, changed, err := migrateDockStore(raw)
	if err != nil {
		return warnRes(i18n.T("shell.json does not parse (%v); the shell falls back to defaults"), err).
			withFix(i18n.T("delete %s to re-seed it"), path)
	}
	if !changed {
		return okRes(i18n.T("dock config lives in the top-level dock object"))
	}
	if checkOnly {
		return wouldRes(i18n.T("shell.json still keeps dock knobs in the qsbar map")).
			withFix(i18n.T("ryoku doctor moves them into the top-level dock object in place"))
	}
	tmp := path + ".ryoku-tmp"
	if err := os.WriteFile(tmp, migrated, 0o644); err != nil {
		return failRes(i18n.T("could not write %s: %v"), tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return failRes(i18n.T("could not replace %s: %v"), path, err)
	}
	return fixedRes(i18n.T("moved the retired qsbar dock knobs into the top-level dock object"))
}

// oldDockKeys map each retired qsbar dock knob to its key in the top-level dock
// object. Only these five carried a persisted value; the dock's other keys are
// left to the shell's defaults.
var oldDockKeys = map[string]string{
	"dockEnabled": "enabled",
	"dockMagnify": "magnify",
	"dockPinned":  "pinned",
	"dockFrost":   "frost",
	"dockShadow":  "shadow",
}

// migrateDockStore lifts the retired qsbar.dock* knobs into a top-level dock
// object under their new names, deleting them from qsbar. Every other key
// (qsbar's own settings and each moved value) is preserved as raw bytes, a dock
// object the shell already wrote wins key-by-key, and a store with none of the
// old keys is a no-op, so a second run reads clean.
func migrateDockStore(raw []byte) ([]byte, bool, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, false, err
	}
	qsbarRaw, ok := top["qsbar"]
	if !ok {
		return nil, false, nil
	}
	var qsbar map[string]json.RawMessage
	if err := json.Unmarshal(qsbarRaw, &qsbar); err != nil {
		return nil, false, err
	}
	moved := map[string]json.RawMessage{}
	for old, want := range oldDockKeys {
		if v, ok := qsbar[old]; ok {
			moved[want] = v
			delete(qsbar, old)
		}
	}
	if len(moved) == 0 {
		return nil, false, nil
	}
	// A dock object the shell already wrote wins: only fill the keys it lacks,
	// so a re-run or a hand edit is never clobbered.
	dock := map[string]json.RawMessage{}
	if dockRaw, ok := top["dock"]; ok {
		if err := json.Unmarshal(dockRaw, &dock); err != nil {
			return nil, false, err
		}
	}
	for k, v := range moved {
		if _, ok := dock[k]; !ok {
			dock[k] = v
		}
	}
	dockBytes, err := json.Marshal(dock)
	if err != nil {
		return nil, false, err
	}
	top["dock"] = dockBytes
	qsbarBytes, err := json.Marshal(qsbar)
	if err != nil {
		return nil, false, err
	}
	top["qsbar"] = qsbarBytes
	out, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return append(out, '\n'), true, nil
}

// ---- reconciler: frame bar style name ----------------------------------------

// reconcileFrameBarsStyle converges a persisted frameBars.style that still
// carries the retired bar-style name. The style itself did not change, only its
// name, so an install that saved the old value would silently fall back to the
// default until the store is rewritten. Surgical and idempotent: only the one
// string moves, every other key survives untouched, and a store already on a
// current style (or with no style key) is left alone.
func reconcileFrameBarsStyle(checkOnly bool) recResult {
	path := filepath.Join(sys.ConfigHome(), "ryoku", "shell.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return okRes(i18n.T("no shell.json yet (seeded on first shell run)"))
	}
	migrated, changed, err := migrateFrameBarsStyle(raw)
	if err != nil {
		return warnRes(i18n.T("shell.json does not parse (%v); the shell falls back to defaults"), err).
			withFix(i18n.T("delete %s to re-seed it"), path)
	}
	if !changed {
		return okRes(i18n.T("frameBars.style names a current bar style"))
	}
	if checkOnly {
		return wouldRes(i18n.T("frameBars.style still names the retired bar style")).
			withFix(i18n.T("ryoku doctor renames it in place"))
	}
	tmp := path + ".ryoku-tmp"
	if err := os.WriteFile(tmp, migrated, 0o644); err != nil {
		return failRes(i18n.T("could not write %s: %v"), tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return failRes(i18n.T("could not replace %s: %v"), path, err)
	}
	return fixedRes(i18n.T("renamed the retired frameBars.style to the current bar style"))
}

// migrateFrameBarsStyle rewrites a shell store whose frameBars.style names a
// retired bar style so it converges on the current default. The two names the
// shell reads today are the only values left alone; anything else present (the
// old name included) moves to the default. Every other key is preserved as its
// own raw bytes, and an absent or non-string style is a no-op, so a store that
// is already current comes back unchanged.
func migrateFrameBarsStyle(raw []byte) ([]byte, bool, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, false, err
	}
	frameRaw, ok := top["frameBars"]
	if !ok {
		return nil, false, nil
	}
	var frame map[string]json.RawMessage
	if err := json.Unmarshal(frameRaw, &frame); err != nil {
		return nil, false, err
	}
	var style string
	if err := json.Unmarshal(frame["style"], &style); err != nil {
		return nil, false, nil
	}
	if style == "slate-frame" || style == "ryoku-frame" {
		return nil, false, nil
	}
	next, err := json.Marshal("slate-frame")
	if err != nil {
		return nil, false, err
	}
	frame["style"] = next
	frameBytes, err := json.Marshal(frame)
	if err != nil {
		return nil, false, err
	}
	top["frameBars"] = frameBytes
	out, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return append(out, '\n'), true, nil
}

// ---- reconciler: retired shell style knobs -----------------------------------

// reconcileLegacyStyleKnobs strips the style keys a persisted shell.json may
// still carry for looks the shell no longer has. surfaceColor / roundness each
// duplicated a Theme token that now owns the value (surface, radiusWidget);
// frameSmoothing / shadowStrength / shadowSize drove
// the retired soft-chrome look, whose blur hid the frame's crisp 2px border.
// Surgical and idempotent: only these keys move, every other key survives
// untouched, and a store already free of them is left alone.
func reconcileLegacyStyleKnobs(checkOnly bool) recResult {
	path := filepath.Join(sys.ConfigHome(), "ryoku", "shell.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return okRes(i18n.T("no shell.json yet (seeded on first shell run)"))
	}
	migrated, changed, err := stripLegacyStyleKnobs(raw)
	if err != nil {
		return warnRes(i18n.T("shell.json does not parse (%v); the shell falls back to defaults"), err).
			withFix(i18n.T("delete %s to re-seed it"), path)
	}
	if !changed {
		return okRes(i18n.T("shell.json carries no retired style knobs"))
	}
	if checkOnly {
		return wouldRes(i18n.T("shell.json still carries retired style knobs (%s)"), strings.Join(legacyStyleKnobs, " / ")).
			withFix(i18n.T("ryoku doctor strips them in place"))
	}
	tmp := path + ".ryoku-tmp"
	if err := os.WriteFile(tmp, migrated, 0o644); err != nil {
		return failRes(i18n.T("could not write %s: %v"), tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return failRes(i18n.T("could not replace %s: %v"), path, err)
	}
	return fixedRes(i18n.T("stripped retired style knobs from shell.json"))
}

// legacyStyleKnobs are shell.json keys the shell no longer reads: each was a look
// override for a style the shell no longer has.
var legacyStyleKnobs = []string{
	"surfaceColor", "roundness",
	"frameRadius", "frameBorder",
	"frameSmoothing", "shadowStrength", "shadowSize",
}

// stripLegacyStyleKnobs removes the retired style knobs from a shell store,
// preserving every other key as its own raw bytes. An absent knob is a no-op, so
// a store already free of them comes back unchanged.
func stripLegacyStyleKnobs(raw []byte) ([]byte, bool, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, false, err
	}
	changed := false
	for _, key := range legacyStyleKnobs {
		if _, ok := top[key]; ok {
			delete(top, key)
			changed = true
		}
	}
	if !changed {
		return nil, false, nil
	}
	out, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return append(out, '\n'), true, nil
}

// ---- reconciler: sumi bar simplification -------------------------------------

// reconcileSumiBar defaults a persisted shell.json's bar to the shipped QS Bar
// (barStyle "qsbar") when it is absent -- a store older than the pluggable
// bar-style system, which renders nothing until a default -- or still names a
// retired Atoll-era style, so an existing box lands on the current default look
// on update. Only the built-in Sumi rail keeps its left-only simplification (its
// top/bottom/right rails ship disabled). Surgical and idempotent: a current or
// installed store style, every zone widget array, and other keys survive
// untouched, and a store already on a valid style is left alone.
func reconcileSumiBar(checkOnly bool) recResult {
	path := filepath.Join(sys.ConfigHome(), "ryoku", "shell.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return okRes(i18n.T("no shell.json yet (seeded on first shell run)"))
	}
	migrated, changed, err := migrateSumiBar(raw)
	if err != nil {
		return warnRes(i18n.T("shell.json does not parse (%v); the shell falls back to defaults"), err).
			withFix(i18n.T("delete %s to re-seed it"), path)
	}
	if !changed {
		return okRes(i18n.T("barStyle names a current style; any Sumi frame is left-only"))
	}
	if checkOnly {
		return wouldRes(i18n.T("shell.json is missing barStyle or names a retired bar style (or a Sumi frame still has extra rails)")).
			withFix(i18n.T("ryoku doctor converges it to the qsbar default in place"))
	}
	tmp := path + ".ryoku-tmp"
	if err := os.WriteFile(tmp, migrated, 0o644); err != nil {
		return failRes(i18n.T("could not write %s: %v"), tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return failRes(i18n.T("could not replace %s: %v"), path, err)
	}
	return fixedRes(i18n.T("defaulted the bar to qsbar / reduced any Sumi frame to left-only"))
}

// retiredBarStyles are old top-level barStyle values from the pre-pluggable
// Atoll/Washi era. The shell can no longer render them (they fall back to the
// empty Sumi rail), so they migrate to the current default instead.
var retiredBarStyles = map[string]bool{"atoll": true, "washi": true, "dyad": true, "ilyamiro": true}

// migrateSumiBar defaults a shell store's bar to the shipped QS Bar when barStyle
// is absent (a store older than the pluggable bar-style system) or names a
// retired Atoll-era style, and -- only for the built-in Sumi rail -- reduces the
// frame to its left rail. A current or installed store style, its zone widget
// arrays, and every other key are preserved as raw bytes, so a store already on
// a valid style comes back unchanged.
func migrateSumiBar(raw []byte) ([]byte, bool, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, false, err
	}
	changed := false
	style := ""
	if b, ok := top["barStyle"]; ok {
		_ = json.Unmarshal(b, &style)
	}
	if _, ok := top["barStyle"]; !ok || retiredBarStyles[style] {
		next, err := json.Marshal("qsbar")
		if err != nil {
			return nil, false, err
		}
		top["barStyle"] = next
		style = "qsbar"
		changed = true
	}
	// Only the built-in Sumi rail keeps the left-only simplification; every other
	// style owns its own rails.
	if style == "sumi" {
		if frameRaw, ok := top["frameBars"]; ok {
			var frame map[string]json.RawMessage
			if err := json.Unmarshal(frameRaw, &frame); err != nil {
				return nil, false, err
			}
			if railsRaw, ok := frame["rails"]; ok {
				var rails map[string]json.RawMessage
				if err := json.Unmarshal(railsRaw, &rails); err != nil {
					return nil, false, err
				}
				railsChanged := false
				for _, side := range []string{"top", "bottom", "right"} {
					railRaw, ok := rails[side]
					if !ok {
						continue
					}
					var rail map[string]json.RawMessage
					if err := json.Unmarshal(railRaw, &rail); err != nil {
						return nil, false, err
					}
					var enabled bool
					if err := json.Unmarshal(rail["enabled"], &enabled); err != nil || !enabled {
						continue
					}
					off, err := json.Marshal(false)
					if err != nil {
						return nil, false, err
					}
					rail["enabled"] = off
					railBytes, err := json.Marshal(rail)
					if err != nil {
						return nil, false, err
					}
					rails[side] = railBytes
					railsChanged = true
				}
				if railsChanged {
					railsBytes, err := json.Marshal(rails)
					if err != nil {
						return nil, false, err
					}
					frame["rails"] = railsBytes
					frameBytes, err := json.Marshal(frame)
					if err != nil {
						return nil, false, err
					}
					top["frameBars"] = frameBytes
					changed = true
				}
			}
		}
	}
	if !changed {
		return nil, false, nil
	}
	out, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return append(out, '\n'), true, nil
}

func frameRail(enabled bool, size float64, zones map[string][]any) map[string]any {
	rail := map[string]any{"enabled": enabled, "size": size, "reveal": true}
	for zone, ids := range zones {
		rail[zone] = ids
	}
	return rail
}

func defaultFrameBarsFromLegacy(_ map[string]any) map[string]any {
	return map[string]any{
		"version": float64(1),
		"style":   "slate-frame",
		"rails": map[string]any{
			"top":    frameRail(false, 32, map[string][]any{"start": {}, "center": {}, "end": {}}),
			"left":   frameRail(true, 48, map[string][]any{"top": {"quick-settings", "workspaces"}, "center": {"dock"}, "bottom": {"recording", "tray", "audio-input", "audio-output", "bluetooth", "network", "clock", "battery"}}),
			"bottom": frameRail(false, 32, map[string][]any{"start": {}, "center": {}, "end": {}}),
			"right":  frameRail(false, 48, map[string][]any{"top": {}, "center": {}, "bottom": {}}),
		},
		"menus": map[string]any{
			"quick-settings": map[string]any{"anchor": "left", "minWidth": float64(410), "expansion": "always", "widgets": []any{"quick-settings"}, "modules": []any{"home", "notifications", "weather", "capture", "stage"}},
			"wallpaper":      map[string]any{"anchor": "bottom", "minWidth": float64(1400), "expansion": "always", "widgets": []any{"theme", "wallpaper"}},
			"theme":          map[string]any{"anchor": "right", "minWidth": float64(320), "expansion": "never", "widgets": []any{"theme"}},
			"weather":        map[string]any{"anchor": "right", "minWidth": float64(320), "expansion": "never", "widgets": []any{"weather"}},
		},
		"surfaces": map[string]any{
			"stash": map[string]any{"anchor": "right", "minWidth": float64(340), "panes": []any{"stash"}},
		},
		"dock": map[string]any{"pinned": []any{}},
	}
}

// frameBarAxes mirrors ryoku/shell/framebars/BarCatalog.js: the bar widgets the
// frame renders, with the rail axes each one fits. An id absent here is stripped
// from any rail on normalize, so this list stays in step with the QML catalogue.
var frameBarAxes = map[string][]string{
	"audio-input": {"horizontal", "vertical"}, "audio-output": {"horizontal", "vertical"},
	"battery": {"horizontal", "vertical"}, "bluetooth": {"horizontal", "vertical"},
	"clock": {"horizontal", "vertical"}, "dock": {"vertical"},
	"workspaces": {"horizontal", "vertical"}, "lock": {"horizontal", "vertical"},
	"logout": {"horizontal", "vertical"}, "music": {"horizontal", "vertical"},
	"network": {"horizontal", "vertical"}, "notifications": {"horizontal", "vertical"},
	"quick-settings": {"horizontal", "vertical"}, "recording": {"horizontal", "vertical"},
	"shutdown": {"horizontal", "vertical"}, "tray": {"horizontal", "vertical"},
	"vpn": {"horizontal", "vertical"},
}

func frameMap(value any) map[string]any {
	m, _ := value.(map[string]any)
	return m
}

func frameStringList(value any, allowed map[string]bool) []any {
	values, _ := value.([]any)
	out := make([]any, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		id, ok := value.(string)
		if ok && allowed[id] && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
func frameUniqueStrings(value any) []any {
	values, _ := value.([]any)
	out := make([]any, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		id, ok := value.(string)
		if ok && id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func frameRailList(value any, axis string) []any {
	values, _ := value.([]any)
	out := make([]any, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		id, ok := value.(string)
		if !ok || seen[id] {
			continue
		}
		for _, supported := range frameBarAxes[id] {
			if supported == axis {
				seen[id] = true
				out = append(out, id)
				break
			}
		}
	}
	return out
}

func frameNumber(value any, fallback, low, high float64) float64 {
	n, ok := value.(float64)
	if !ok {
		return fallback
	}
	return min(max(n, low), high)
}

func frameAnchor(value any, fallback string) string {
	anchor, ok := value.(string)
	if !ok {
		return fallback
	}
	for _, valid := range []string{"bottom", "bottom-left", "bottom-right", "left", "right", "top", "top-left", "top-right"} {
		if anchor == valid {
			return anchor
		}
	}
	return fallback
}

func normalizeFrameBars(v any) (map[string]any, []string) {
	base := defaultFrameBarsFromLegacy(nil)
	source := frameMap(v)
	out := defaultFrameBarsFromLegacy(nil)
	if style, ok := source["style"].(string); ok && (style == "slate-frame" || style == "ryoku-frame") {
		out["style"] = style
	}
	baseRails := frameMap(base["rails"])
	sourceRails := frameMap(source["rails"])
	outRails := frameMap(out["rails"])
	for _, edge := range []string{"top", "left", "bottom", "right"} {
		fallback := frameMap(baseRails[edge])
		raw := frameMap(sourceRails[edge])
		rail := frameMap(outRails[edge])
		if enabled, ok := raw["enabled"].(bool); ok {
			rail["enabled"] = enabled
		}
		if reveal, ok := raw["reveal"].(bool); ok {
			rail["reveal"] = reveal
		}
		horizontal := edge == "top" || edge == "bottom"
		if horizontal {
			rail["size"] = frameNumber(raw["size"], fallback["size"].(float64), 16, 96)
		} else {
			rail["size"] = frameNumber(raw["size"], fallback["size"].(float64), 24, 112)
		}
		axis := "vertical"
		zones := []string{"top", "center", "bottom"}
		if horizontal {
			axis, zones = "horizontal", []string{"start", "center", "end"}
		}
		for _, zone := range zones {
			if _, present := raw[zone]; present {
				rail[zone] = frameRailList(raw[zone], axis)
			}
		}
	}
	baseMenus, sourceMenus, outMenus := frameMap(base["menus"]), frameMap(source["menus"]), frameMap(out["menus"])
	for _, id := range []string{"quick-settings", "wallpaper", "theme", "weather"} {
		fallback, raw, menu := frameMap(baseMenus[id]), frameMap(sourceMenus[id]), frameMap(outMenus[id])
		menu["anchor"] = frameAnchor(raw["anchor"], fallback["anchor"].(string))
		menu["minWidth"] = frameNumber(raw["minWidth"], fallback["minWidth"].(float64), 1, 10000)
		if expansion, ok := raw["expansion"].(string); ok {
			switch expansion {
			case "always", "never", "both", "up", "down":
				menu["expansion"] = expansion
			}
		}
	}
	// Modules are catalogued and filtered by the shell. Doctor only guarantees a
	// unique string list so a future module can be enabled without teaching this
	// migration layer about every QML component.
	menuRaw, menuOut := frameMap(sourceMenus["quick-settings"]), frameMap(outMenus["quick-settings"])
	if modules := frameUniqueStrings(menuRaw["modules"]); len(modules) > 0 {
		menuOut["modules"] = modules
	}
	baseSurfaces, sourceSurfaces, outSurfaces := frameMap(base["surfaces"]), frameMap(source["surfaces"]), frameMap(out["surfaces"])
	fallback, raw, surface := frameMap(baseSurfaces["stash"]), frameMap(sourceSurfaces["stash"]), frameMap(outSurfaces["stash"])
	surface["anchor"] = frameAnchor(raw["anchor"], fallback["anchor"].(string))
	surface["minWidth"] = frameNumber(raw["minWidth"], fallback["minWidth"].(float64), 1, 10000)
	if _, present := raw["panes"]; present {
		allowed := map[string]bool{"stash": true}
		surface["panes"] = frameStringList(raw["panes"], allowed)
	}
	dockRaw, dockOut := frameMap(source["dock"]), frameMap(out["dock"])
	if pinned, ok := dockRaw["pinned"].([]any); ok {
		outPinned := make([]any, 0, len(pinned))
		for _, value := range pinned {
			if id, ok := value.(string); ok {
				outPinned = append(outPinned, id)
			}
		}
		dockOut["pinned"] = outPinned
	}
	before, _ := json.Marshal(source)
	after, _ := json.Marshal(out)
	if bytes.Equal(before, after) {
		return out, nil
	}
	return out, []string{i18n.T("normalized frameBars")}
}

// retiredShellKeys are shell.json keys no shipped surface reads any more: the
// Atoll bar geometry and skins, its module/toggle lists, the island knobs, and
// the sidebar openers that frame surfaces replaced. Frame bars carry all of it
// now, so a machine upgrading from any older release sheds them here rather
// than keeping dead state around forever.
var retiredShellKeys = []string{
	"atollVariant", "barEnabled", "barHeight", "barLayoutCentre", "barLayoutLeft",
	"barLayoutRight", "barOccupiedWorkspaces", "barPosition", "barShowMedia",
	"barShowSpecialWs", "barShowStatus", "barShowTitle", "barShowWeather",
	"barToggles", "dyadVariant", "islandAlong", "islandEdge",
	"islandHidden", "islandModules", "islandRadius", "ryolayerEnabled", "sidebarClickless",
	"sidebarCornerSize", "sidebarLeftEnabled", "sidebarRightEnabled",
	"washiVariant",
}

func migrateShellConfig(raw []byte) ([]byte, []string, error) {
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, nil, err
	}
	var changes []string
	if _, present := cfg["frameBars"]; !present {
		cfg["frameBars"] = defaultFrameBarsFromLegacy(cfg)
		changes = append(changes, i18n.T("migrated Atoll settings to frame bars"))
	}
	if _, left := cfg["sidebarLeftPanes"]; left || cfg["sidebarRightPanes"] != nil || cfg["sidebarWidth"] != nil {
		frameBars := frameMap(cfg["frameBars"])
		surfaces := frameMap(frameBars["surfaces"])
		stash := frameMap(surfaces["stash"])
		if panes, ok := cfg["sidebarLeftPanes"]; ok {
			stash["panes"] = panes
		}
		if width, ok := cfg["sidebarWidth"]; ok {
			stash["minWidth"] = width
		}
		surfaces["stash"] = stash
		frameBars["surfaces"] = surfaces
		cfg["frameBars"] = frameBars
		delete(cfg, "sidebarLeftPanes")
		delete(cfg, "sidebarRightPanes")
		delete(cfg, "sidebarWidth")
		changes = append(changes, i18n.T("migrated stash sidebar and retired system sidebar settings"))
	}
	normalized, frameChanges := normalizeFrameBars(cfg["frameBars"])
	cfg["frameBars"] = normalized
	changes = append(changes, frameChanges...)
	removed := false
	for _, key := range retiredShellKeys {
		if _, exists := cfg[key]; exists {
			delete(cfg, key)
			removed = true
		}
	}
	if removed {
		changes = append(changes, i18n.T("removed retired settings"))
	}
	if len(changes) == 0 {
		return nil, nil, nil
	}
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return append(out, '\n'), changes, nil
}

// ---- reconciler: desktop session components ----------------------------------

// portalFrontends maps a declared backend to the frontend package that serves
// it, keyed by the seam's own provider constant so no bare compositor name is
// branched on here.
var portalFrontends = map[string]struct {
	fix  string
	pkgs []string
}{
	wm.ProviderHyprland: {"sudo pacman -S xdg-desktop-portal-hyprland", []string{"xdg-desktop-portal-hyprland"}},
	"gnome":             {"sudo pacman -S xdg-desktop-portal-gnome", []string{"xdg-desktop-portal-gnome"}},
	"kde":               {"sudo pacman -S xdg-desktop-portal-kde", []string{"xdg-desktop-portal-kde"}},
	"wlr":               {"sudo pacman -S xdg-desktop-portal-wlr", []string{"xdg-desktop-portal-wlr"}},
}

// portalFrontendCheck resolves the xdg-desktop-portal frontend this session
// needs from the live provider's declared backend, with the command to install
// it. With no provider answering (a broken or headless box) every frontend is
// accepted rather than pointing at one compositor's, which is what the check
// did before the backend was read.
func portalFrontendCheck() (fix string, pkgs []string) {
	backend := ""
	if caps, err := wm.Open().Caps(); err == nil {
		backend = caps.PortalBackend
	}
	if f, ok := portalFrontends[backend]; ok {
		return f.fix, f.pkgs
	}
	return "", []string{
		"xdg-desktop-portal-hyprland", "xdg-desktop-portal-gnome",
		"xdg-desktop-portal-kde", "xdg-desktop-portal-wlr",
	}
}

func reconcileSessionComponents(_ bool) recResult {
	if wm.Detect().Name == "" {
		return okRes(i18n.T("no window manager provider"))
	}
	portalFix, portalPkgs := portalFrontendCheck()
	checks := []struct {
		role, fix string
		any       []string
	}{
		{i18n.T("authentication agent"), "sudo pacman -S hyprpolkitagent", []string{"hyprpolkitagent", "polkit-gnome", "polkit-kde-agent", "lxsession"}},
		// The frontend the session needs is the one its compositor declares
		// (Caps.PortalBackend): checking Hyprland's on a niri box reported the
		// portal missing and told the user to install the wrong backend.
		{i18n.T("desktop portal"), portalFix, portalPkgs},
		{i18n.T("audio server"), "sudo pacman -S pipewire wireplumber", []string{"pipewire"}},
		{i18n.T("network manager"), "sudo pacman -S networkmanager", []string{"networkmanager"}},
	}
	var missing []string
	for _, c := range checks {
		if !anyPkgInstalled(c.any...) {
			missing = append(missing, fmt.Sprintf("%s [%s]", c.role, c.fix))
		}
	}
	if len(missing) == 0 {
		return okRes(i18n.T("desktop session components present"))
	}
	return warnRes(i18n.T("missing: %s"), strings.Join(missing, "; "))
}

// ---- reconciler: desktop portal routing ----------------------------------------

// portalDesktopToken is the name xdg-desktop-portal prefixes its desktop
// config with: the first XDG_CURRENT_DESKTOP entry, lowercased (portals.conf(5)
// reads <desktop>-portals.conf). Outside a session that variable is empty, so
// fall back to the detected provider name, which matches what the next login
// will carry. An empty token means no desktop-specific file to look for.
func portalDesktopToken(desktopEnv, provider string) string {
	if v := strings.TrimSpace(desktopEnv); v != "" {
		if i := strings.IndexByte(v, ':'); i >= 0 {
			v = v[:i]
		}
		return strings.ToLower(strings.TrimSpace(v))
	}
	return strings.ToLower(strings.TrimSpace(provider))
}

// portalConfigCandidates lists every file xdg-desktop-portal consults on a
// <desktop> session, highest precedence first (portals.conf(5)): user config,
// XDG_CONFIG_DIRS, /etc, user data, XDG_DATA_DIRS. in each location the
// desktop-specific name is read before the generic one, and the first file
// that exists wins outright, nothing merges. that order is the trap: a
// user-level generic portals.conf beats the packaged <desktop>-portals.conf.
func portalConfigCandidates(home, desktop string) []string {
	var dirs []string
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		dirs = append(dirs, v)
	} else {
		dirs = append(dirs, filepath.Join(home, ".config"))
	}
	confDirs := os.Getenv("XDG_CONFIG_DIRS")
	if confDirs == "" {
		confDirs = "/etc/xdg"
	}
	dirs = append(dirs, strings.Split(confDirs, ":")...)
	dirs = append(dirs, "/etc")
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		dirs = append(dirs, v)
	} else {
		dirs = append(dirs, filepath.Join(home, ".local/share"))
	}
	dataDirs := os.Getenv("XDG_DATA_DIRS")
	if dataDirs == "" {
		dataDirs = "/usr/local/share:/usr/share"
	}
	dirs = append(dirs, strings.Split(dataDirs, ":")...)
	dirs = append(dirs, "/usr/share")
	var out []string
	seen := map[string]bool{}
	for _, d := range dirs {
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		if desktop != "" {
			out = append(out, filepath.Join(d, "xdg-desktop-portal", desktop+"-portals.conf"))
		}
		out = append(out, filepath.Join(d, "xdg-desktop-portal", "portals.conf"))
	}
	return out
}

// portalRoutesBackend: does this config hand the default portal role to backend?
// per-interface overrides next to a sane default are a deliberate user tweak and
// stay untouched.
func portalRoutesBackend(content, backend string) bool {
	section := ""
	for _, ln := range strings.Split(content, "\n") {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			section = t
			continue
		}
		if section != "[preferred]" {
			continue
		}
		k, v, ok := strings.Cut(t, "=")
		if !ok || strings.TrimSpace(k) != "default" {
			continue
		}
		for _, b := range strings.Split(v, ";") {
			if strings.TrimSpace(b) == backend {
				return true
			}
		}
	}
	return false
}

// reconcilePortalRouting keeps xdg-desktop-portal pointed at the provider's
// backend (Caps.PortalBackend). a migrated box can carry a leftover user or /etc
// portals.conf that outranks the packaged one and names the gnome backend, which
// hangs under a non-GNOME session: every app that reads the settings portal at
// startup waits out a ~25s D-Bus timeout ("apps are slow to open"). heals boxes
// converted before the installer started moving the user file aside, and /etc.
func reconcilePortalRouting(checkOnly bool) recResult {
	caps, _ := wm.Open().Caps()
	backend := caps.PortalBackend
	if backend == "" {
		// no provider, or a compositor that declares no preferred portal backend.
		return okRes(i18n.T("no preferred portal backend to enforce"))
	}
	// the portal reads <desktop>-portals.conf for the running desktop, so that
	// is the file to look for; the provider name is the fallback when the
	// session has not exported XDG_CURRENT_DESKTOP yet. the first existing
	// candidate is the one the portal loads, so every misrouted file ahead of a
	// healthy one has to move aside.
	desktop := portalDesktopToken(os.Getenv("XDG_CURRENT_DESKTOP"), caps.Name)
	var offenders []string
	healthy := ""
	for _, p := range portalConfigCandidates(sys.Home(), desktop) {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if portalRoutesBackend(string(b), backend) {
			healthy = p
			break
		}
		offenders = append(offenders, p)
	}
	if len(offenders) == 0 {
		if healthy == "" {
			return okRes(i18n.T("no portal routing config found"))
		}
		return okRes(i18n.T("portal routing follows %s"), healthy)
	}
	if healthy == "" {
		return warnRes(i18n.T("no config routes portals to the %s backend; screenshare and portal dialogs cannot work"), backend).
			withFix(i18n.T("sudo pacman -S xdg-desktop-portal-%s"), backend)
	}
	list := strings.Join(offenders, ", ")
	if checkOnly {
		return wouldRes(i18n.T("%s routes portals away from the %s backend; apps stall ~25s at launch and screenshare breaks"), list, backend).
			withFix(i18n.T("ryoku doctor moves the file(s) aside and restarts the portal"))
	}
	for _, p := range offenders {
		bak := p + ".ryoku-bak"
		var err error
		if strings.HasPrefix(p, sys.Home()+string(os.PathSeparator)) {
			err = os.Rename(p, bak)
		} else {
			err = sys.Sudo("mv", p, bak)
		}
		if err != nil {
			return failRes(i18n.T("could not move %s aside: %v"), p, err).withFix("mv %s %s", p, bak)
		}
	}
	// a hung foreign backend keeps its stall alive until it dies. best-effort
	// and quiet: outside a session the next login picks the routing up anyway.
	for _, u := range []string{"xdg-desktop-portal-gnome.service", "xdg-desktop-portal-kde.service",
		"xdg-desktop-portal-wlr.service", "xdg-desktop-portal-lxqt.service"} {
		_ = exec.Command("systemctl", "--user", "stop", u).Run()
	}
	_ = exec.Command("systemctl", "--user", "try-restart", "xdg-desktop-portal.service").Run()
	return fixedRes(i18n.T("moved %s aside; the portal now follows %s"), list, healthy)
}

// ---- reconciler: cursor theme ------------------------------------------------

const defaultCursorTheme = "Bibata-Modern-Ice"

// cursorSearchDirs: the XCursor theme search path, in load order. the system dir
// (where ryoku-cursors installs the Bibata family) first, then the two per-user
// dirs a Hub-installed third-party theme can land in.
func cursorSearchDirs() []string {
	return []string{
		"/usr/share/icons",
		filepath.Join(sys.Home(), ".local", "share", "icons"),
		filepath.Join(sys.Home(), ".icons"),
	}
}

// cursorThemeInstalled: is <theme>/cursors present under any search dir? a bare
// theme dir with no cursors/ is an index-only stub the loader cannot use.
func cursorThemeInstalled(theme string, dirs []string) bool {
	if theme == "" {
		return false
	}
	for _, d := range dirs {
		if sys.Exists(filepath.Join(d, theme, "cursors")) {
			return true
		}
	}
	return false
}

// configuredCursor: the theme + size the desktop actually loads = the Hub
// override in desktop.json when set, else the shipped env.lua default. pure, so
// the converge decision is unit-testable without a live desktop.
func configuredCursor(raw []byte) (string, int) {
	theme, size := defaultCursorTheme, 24
	var cfg struct {
		Desktop struct {
			Cursor struct {
				Theme string `json:"theme"`
				Size  int    `json:"size"`
			} `json:"cursor"`
		} `json:"desktop"`
	}
	if json.Unmarshal(raw, &cfg) == nil {
		if cfg.Desktop.Cursor.Theme != "" {
			theme = wm.ResolveCursorTheme(cfg.Desktop.Cursor.Theme)
		}
		if cfg.Desktop.Cursor.Size > 0 {
			size = cfg.Desktop.Cursor.Size
		}
	}
	return theme, size
}

// resetCursorTheme rewrites desktop.cursor.theme to def, leaving every other key
// (size, the rest of the store) intact. pure and idempotent.
func resetCursorTheme(raw []byte, def string) ([]byte, error) {
	cfg := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return nil, err
		}
	}
	desktop, _ := cfg["desktop"].(map[string]any)
	if desktop == nil {
		desktop = map[string]any{}
	}
	cur, _ := desktop["cursor"].(map[string]any)
	if cur == nil {
		cur = map[string]any{}
	}
	cur["theme"] = def
	desktop["cursor"] = cur
	cfg["desktop"] = desktop
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// reconcileCursorTheme keeps the desktop pointing at a cursor theme that is
// actually on disk. ryoku-cursors now ships the Bibata family as a hard
// ryoku-desktop dependency, so the configured default is package-guaranteed and
// the failure worth healing is a Hub-picked third-party theme that was never
// installed (or was removed): the pointer then falls back to a bare bitmap.
// the fix is config-side -- reset hypr.json to the guaranteed default -- never a
// pacman call. when even the default is absent (a dev checkout with no package)
// there is nothing config can heal, so it only warns.
func reconcileCursorTheme(checkOnly bool) recResult {
	if wm.Detect().Name == "" {
		return okRes(i18n.T("no window manager provider"))
	}
	dirs := cursorSearchDirs()
	store := filepath.Join(sys.ConfigHome(), "ryoku", "desktop.json")
	raw, _ := os.ReadFile(store)
	theme, size := configuredCursor(raw)
	if cursorThemeInstalled(theme, dirs) {
		return okRes(i18n.T("configured cursor theme %q is installed"), theme)
	}
	if !cursorThemeInstalled(defaultCursorTheme, dirs) {
		// the package default is missing too: no config edit can conjure the
		// theme files, and doctor never drives pacman.
		return warnRes(i18n.T("cursor theme %q is missing on disk and so is the default %q; the pointer falls back to a bitmap"), theme, defaultCursorTheme).
			withFix(i18n.T("install ryoku-cursors (it ships the Bibata family and is a ryoku-desktop dependency)"))
	}
	if checkOnly {
		return wouldRes(i18n.T("cursor theme %q missing on disk; would reset to %s"), theme, defaultCursorTheme).
			withFix(i18n.T("ryoku doctor resets it to %s; re-pick your theme in the Hub after installing it"), defaultCursorTheme)
	}
	out, err := resetCursorTheme(raw, defaultCursorTheme)
	if err != nil {
		return warnRes(i18n.T("cursor theme %q missing on disk and desktop.json does not parse (%v)"), theme, err).
			withFix(i18n.T("set desktop.cursor.theme to %s in %s"), defaultCursorTheme, store)
	}
	tmp := store + ".ryoku-tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return failRes(i18n.T("could not write %s: %v"), tmp, err)
	}
	if err := os.Rename(tmp, store); err != nil {
		os.Remove(tmp)
		return failRes(i18n.T("could not replace %s: %v"), store, err)
	}
	// best-effort live apply; no-ops off a running session.
	_ = wm.Open().Act(wm.ActionCursorSet, defaultCursorTheme, fmt.Sprintf("%d", size))
	return fixedRes(i18n.T("cursor theme %q missing on disk; reset to %s (re-pick your theme in the Hub after installing it)"), theme, defaultCursorTheme)
}

// ---- reconciler: GTK session theme -------------------------------------------

// resolveGtkThemeName maps the theme.json GTK choice and the session's light or
// dark mode to the gsettings gtk-theme name, the exact C3 mapping the daemon
// uses so doctor never converges to a variant the next repaint flips: adw uses
// adw-gtk3 (whose widget rules derive from the named colours the palette emits),
// adwaita the stock GNOME look, and system means Ryoku never owns gtk-theme.
func resolveGtkThemeName(pref string, dark bool) string {
	switch pref {
	case "adwaita":
		if dark {
			return "Adwaita-dark"
		}
		return "Adwaita"
	case "system":
		return ""
	default: // "adw", and an absent choice
		if dark {
			return "adw-gtk3-dark"
		}
		return "adw-gtk3"
	}
}

// gtkThemePref reads the GTK base-theme choice out of theme.json. Absent (an
// older file, or none) reads as "adw", matching the hub's default.
func gtkThemePref() string {
	var s struct {
		GtkTheme string `json:"gtkTheme"`
	}
	b, err := os.ReadFile(filepath.Join(sys.ConfigHome(), "ryoku", "theme.json"))
	if err != nil || json.Unmarshal(b, &s) != nil || s.GtkTheme == "" {
		return "adw"
	}
	return s.GtkTheme
}

// gsettingsInterface reads one org.gnome.desktop.interface key, unquoted. ok is
// false when gsettings is absent or no settings bus answers, so a TTY or a
// non-desktop box reads as "can't tell" and the GTK reconciler stands down
// instead of inventing drift.
func gsettingsInterface(key string) (string, bool) {
	out, err := sys.RunOut("gsettings", "get", "org.gnome.desktop.interface", key)
	if err != nil {
		return "", false
	}
	return strings.Trim(strings.TrimSpace(out), "'"), true
}

// restoreGtkSettingsIni copies the missing settings.ini baselines from the
// packaged base config tree materialize lays, the same files the shell deploys.
// The daemon rewrites their contents on every palette or mode change, so
// restoring the baseline only needs to make the files exist. false when the
// packaged copy is itself absent (a box still on the pre-fix package), where the
// cure is to pull the update first.
func restoreGtkSettingsIni(missingRel []string) (bool, error) {
	for _, rel := range missingRel {
		src := filepath.Join(sys.BaseConfigDir(), rel)
		if !sys.Exists(src) {
			return false, nil
		}
		if err := sys.CopyFile(src, filepath.Join(sys.ConfigHome(), rel)); err != nil {
			return false, err
		}
	}
	return true, nil
}

// reconcileGtkSession keeps the GTK session in step with the theme.json GTK
// choice: gsettings gtk-theme should be the C3 name resolved for the current
// light/dark mode, and the two settings.ini baselines (the on-disk theme names a
// bare Wayland session's xsettings-less apps read) should exist. The daemon
// writes gtk-theme on every palette or mode change (C4); this converges the
// drift a crash, a hand-edit, or a box that updated before the theme shipped
// leaves behind. system mode leaves gtk-theme untouched. Idempotent, and quiet
// off a desktop session or when no settings bus answers.
func reconcileGtkSession(checkOnly bool) recResult {
	pref := gtkThemePref()
	if pref == "system" {
		return okRes(i18n.T("GTK theme left to the system (gtkTheme=system); gtk-theme not managed"))
	}
	if !sys.Has("gsettings") {
		return okRes(i18n.T("gsettings unavailable; GTK theme not managed here"))
	}
	scheme, ok := gsettingsInterface("color-scheme")
	if !ok {
		return okRes(i18n.T("no reachable settings bus; GTK theme reconciled at next login"))
	}
	want := resolveGtkThemeName(pref, !strings.Contains(scheme, "light"))
	cur, ok := gsettingsInterface("gtk-theme")
	if !ok {
		return okRes(i18n.T("no reachable settings bus; GTK theme reconciled at next login"))
	}

	var missing []string
	for _, rel := range []string{"gtk-3.0/settings.ini", "gtk-4.0/settings.ini"} {
		if !sys.Exists(filepath.Join(sys.ConfigHome(), rel)) {
			missing = append(missing, rel)
		}
	}
	themeDrift := cur != want
	if !themeDrift && len(missing) == 0 {
		return okRes(i18n.T("GTK session theme is %s and the settings.ini baselines are present"), want)
	}

	if checkOnly {
		switch {
		case themeDrift && len(missing) > 0:
			return wouldRes(i18n.T("GTK theme is %q, want %q, and %s missing"), cur, want, strings.Join(missing, ", ")).withFix("ryoku doctor")
		case themeDrift:
			return wouldRes(i18n.T("GTK theme is %q, want %q for the current mode"), cur, want).withFix("ryoku doctor")
		default:
			return wouldRes(i18n.T("GTK settings.ini baseline(s) missing: %s"), strings.Join(missing, ", ")).withFix("ryoku doctor")
		}
	}

	var did []string
	if themeDrift {
		if err := sys.Run("gsettings", "set", "org.gnome.desktop.interface", "gtk-theme", want); err != nil {
			return failRes(i18n.T("could not set gtk-theme to %s: %v"), want, err).withFix("ryoku doctor")
		}
		did = append(did, fmt.Sprintf(i18n.T("set gtk-theme to %s"), want))
	}
	if len(missing) > 0 {
		restored, err := restoreGtkSettingsIni(missing)
		if err != nil {
			return failRes(i18n.T("could not restore GTK settings.ini: %v"), err).withFix("ryoku materialize")
		}
		if !restored {
			return warnRes(i18n.T("GTK settings.ini missing (%s) and no packaged baseline to restore"), strings.Join(missing, ", ")).withFix("ryoku update")
		}
		did = append(did, i18n.Tf("restored %s", strings.Join(missing, ", ")))
	}
	return fixedRes(i18n.T("GTK session reconciled: %s"), strings.Join(did, "; "))
}

// ---- reconciler: SDDM greeter theme readable ---------------------------------

const greeterThemeDir = "/usr/share/sddm/themes/ryoku"

// greeterThemeHealthy: can the unprivileged `sddm` greeter read the theme? sddm
// is neither the owner nor a group member, so readability rides on the world
// bits -- the theme dir needs o+rx and Main.qml o+r. root ownership is required
// too, so a regular user can't swap the QML the login screen loads.
func greeterThemeHealthy(ownerUID uint32, dirPerm, mainPerm os.FileMode) bool {
	return ownerUID == 0 && dirPerm&0o005 == 0o005 && mainPerm&0o004 != 0
}

// reconcileGreeterTheme keeps the SDDM greeter theme readable by the sddm user.
// `ryoku-hub lock set` copies the picked skin into the fixed greeter dir; a skin
// pulled from the catalogue downloads into an os.MkdirTemp dir (always 0700,
// user-owned), so an older `cp -a` left the greeter unreadable to sddm and SDDM
// silently fell back to its embedded theme on every boot. installGreeter now
// normalizes on write; this backports the fix to boxes that already picked a
// skin. only ever touches the one fixed Ryoku greeter dir.
func reconcileGreeterTheme(checkOnly bool) recResult {
	di, err := os.Stat(greeterThemeDir)
	if err != nil {
		return okRes(i18n.T("no Ryoku greeter theme installed"))
	}
	mi, err := os.Stat(filepath.Join(greeterThemeDir, "Main.qml"))
	if err != nil {
		return okRes(i18n.T("no Ryoku greeter theme installed"))
	}
	st, ok := di.Sys().(*syscall.Stat_t)
	if !ok {
		return okRes(i18n.T("greeter theme ownership not checkable"))
	}
	if greeterThemeHealthy(st.Uid, di.Mode().Perm(), mi.Mode().Perm()) {
		return okRes(i18n.T("greeter theme readable by the sddm greeter"))
	}
	fix := fmt.Sprintf("sudo chown -R root:root %s && sudo chmod -R a+rX %s", greeterThemeDir, greeterThemeDir)
	if checkOnly {
		return wouldRes(i18n.T("greeter theme unreadable by the sddm greeter; SDDM falls back to its default")).withFix(fix)
	}
	if err := sys.Run("sudo", "chown", "-R", "root:root", greeterThemeDir); err != nil {
		return failRes(i18n.T("could not fix greeter theme ownership: %v"), err).withFix(fix)
	}
	if err := sys.Run("sudo", "chmod", "-R", "a+rX", greeterThemeDir); err != nil {
		return failRes(i18n.T("could not fix greeter theme permissions: %v"), err).withFix(fix)
	}
	return fixedRes(i18n.T("normalized greeter theme so the sddm greeter can read it"))
}

// ---- reconciler: SDDM greeter on Wayland ------------------------------------

const sddmWaylandConf = "/etc/sddm.conf.d/10-ryoku-wayland.conf"

// The greeter compositor. When the ryoku-greeter wrapper is present (shipped by
// ryoku-desktop) it runs weston --shell=kiosk at each output's top mode; else
// plain weston at its preferred mode. Never set DisplayServer=wayland without
// weston present -- the greeter could not start.
var greeterCompositorBin = "/usr/share/ryoku/lockscreen/ryoku-greeter"

// nvidiaVendorGlob is where sysfs exposes each DRM card's PCI vendor id.
var nvidiaVendorGlob = "/sys/class/drm/card*/device/vendor"

// greeterCompositor picks the SDDM greeter compositor command. On NVIDIA the
// weston cursor-plane path silently drops the greeter pointer (#184), so the
// fallback runs the pixman renderer: a software cursor, always visible. The
// wrapper script makes the same call at run time.
func greeterCompositor() string {
	if sys.Exists(greeterCompositorBin) {
		return greeterCompositorBin
	}
	if nvidiaDRMPresent() {
		return "weston --shell=kiosk --renderer=pixman"
	}
	return "weston --shell=kiosk"
}

// nvidiaDRMPresent reports whether a DRM card carries NVIDIA's vendor id.
func nvidiaDRMPresent() bool {
	vendors, _ := filepath.Glob(nvidiaVendorGlob)
	for _, v := range vendors {
		if b, err := os.ReadFile(v); err == nil && strings.TrimSpace(string(b)) == "0x10de" {
			return true
		}
	}
	return false
}

// sessionWrapperBin waits for the greeter to release the GPU before the session
// compositor probes KMS (see sddmWaylandBody). sddmDefaultWaylandSession is
// SDDM's own default [Wayland] SessionCommand, used until the wrapper is shipped.
const sessionWrapperBin = "/usr/share/ryoku/lockscreen/ryoku-wayland-session"
const sddmDefaultWaylandSession = "/usr/share/sddm/scripts/wayland-session"

// greeterEnvironment is SDDM's comma-separated GreeterEnvironment. The greeter
// is a Qt client of the weston kiosk with no session behind it, so it inherits
// no XCURSOR_*: Qt then asks libwayland-cursor for the "default" theme, and on a
// box where that chain resolves to nothing the greeter sets a null cursor and
// the login screen has no visible pointer. Pin the shipped Bibata set
// (ryoku-cursors, a hard depend) at the size env.lua uses. This only helps
// clients that honor XCURSOR_THEME; reconcileGreeterCursor establishes the
// "default" theme itself for the ones (SDDM's Wayland greeter, weston) that fall
// back to it regardless. QML_XHR_ALLOW_FILE_READ lets the greeter theme's
// bundled I18n read the shipped catalog with a file:// request (Qt6 blocks local
// XHR reads without it), so the login screen localises without pulling in the
// shell's Quickshell-backed singletons (#162).
const greeterEnvironment = "QT_QPA_PLATFORM=wayland,XCURSOR_THEME=Bibata-Modern-Ice,XCURSOR_SIZE=24,QML_XHR_ALLOW_FILE_READ=1"

func sddmWaylandBody() string {
	compositor := greeterCompositor()
	// SessionCommand wraps the session start with a wait for the greeter (weston)
	// to exit before the compositor probes KMS: on a hybrid-GPU laptop weston can
	// still hold a DRM device when SDDM starts the session on the next VT, so the
	// probe misses that GPU and lands on a headless dGPU -- a black screen (#174).
	// Falls back to SDDM's own default session script until the wrapper ships,
	// which is behaviourally a no-op.
	session := sddmDefaultWaylandSession
	if sys.Exists(sessionWrapperBin) {
		session = sessionWrapperBin
	}
	return "[General]\nDisplayServer=wayland\nGreeterEnvironment=" + greeterEnvironment +
		"\n\n[Wayland]\nCompositorCommand=" + compositor +
		"\nSessionCommand=" + session + "\n"
}

// reconcileGreeterDisplayServer moves the SDDM greeter to Wayland. SDDM's
// default X11 greeter is orphaned when a Wayland session (Hyprland) starts:
// sddm-helper dies mid-teardown without reaping sddm-greeter-qt6, which lingers
// on a leftover Xorg and keeps drawing power (a video skin decodes forever).
// Running the greeter on Wayland like the session lets SDDM own the VT and stop
// it cleanly at login. Installer boxes get this from sddm/setup; this backports
// it. Only ever writes our own conf, and only once weston is installed -- a hard
// depend `pacman -Syu` lands before doctor runs, so this is unmet only on a box
// that has not pulled the package yet.
func reconcileGreeterDisplayServer(checkOnly bool) recResult {
	if !sys.Exists(greeterThemeDir) {
		return okRes(i18n.T("no Ryoku greeter installed"))
	}
	if !sys.Exists("/usr/bin/weston") {
		return warnRes(i18n.T("the Wayland greeter needs weston, which is not installed yet")).
			withFix("ryoku update")
	}
	want := sddmWaylandBody()
	have := readFileSafe(sddmWaylandConf)
	if have == strings.TrimRight(want, "\n") {
		return okRes(i18n.T("SDDM greeter runs on Wayland (weston kiosk)"))
	}
	if have != "" {
		if checkOnly {
			return wouldRes(i18n.T("SDDM greeter config is out of date (the pinned cursor theme, the compositor wrapper)")).
				withFix("ryoku doctor")
		}
		if err := writeRootFile(sddmWaylandConf, want, "0644"); err != nil {
			return failRes(i18n.T("could not write %s: %v"), sddmWaylandConf, err).
				withFix(i18n.T("check sudo access, then re-run ryoku doctor"))
		}
		return fixedRes(i18n.T("refreshed the SDDM greeter config; the login screen pointer uses the shipped cursor theme"))
	}
	if checkOnly {
		return wouldRes(i18n.T("SDDM greeter still runs on X11; it is orphaned when a Wayland session starts and keeps drawing power")).
			withFix("ryoku doctor")
	}
	if err := writeRootFile(sddmWaylandConf, want, "0644"); err != nil {
		return failRes(i18n.T("could not write %s: %v"), sddmWaylandConf, err).
			withFix(i18n.T("check sudo access, then re-run ryoku doctor"))
	}
	return fixedRes(i18n.T("moved the SDDM greeter to Wayland (weston kiosk); it is torn down cleanly at login now"))
}

// ---- reconciler: login screen cursor -----------------------------------------

const defaultCursorDir = "/usr/share/icons/default"
const defaultCursorIndex = defaultCursorDir + "/index.theme"

// defaultCursorIndexBody points the freedesktop "default" cursor theme at the
// shipped Bibata set. libwayland-cursor (weston's own pointer, and the SDDM
// greeter's Qt client) and libXcursor fall back to the theme literally named
// "default" whenever the requested theme is missing -- or, as SDDM's Wayland
// greeter does, silently ignored. Ryoku ships no /usr/share/icons/default, so
// that fallback resolves to nothing and the login screen (at system start and
// after logout) draws no pointer at all. An Inherits= stub gives "default" a
// real target, covering every client that does not honor GreeterEnvironment's
// XCURSOR_THEME. Kept as a stub, not a copy, so it tracks whatever Bibata ships.
func defaultCursorIndexBody() string {
	return "[Icon Theme]\nName=Default\nComment=Ryoku default cursor\nInherits=" + defaultCursorTheme + "\n"
}

// defaultCursorEstablished: the system already has a "default" cursor theme --
// our index.theme, a foreign one, or a real cursors/ dir. Any of these means the
// fallback resolves, so we must not clobber it (a user or another package may
// own it).
func defaultCursorEstablished() bool {
	return sys.Exists(defaultCursorIndex) || sys.Exists(defaultCursorDir+"/cursors")
}

// reconcileGreeterCursor keeps the login screen pointer visible. The SDDM
// greeter runs on a weston kiosk with no session env; where the greeter or
// weston falls back to the "default" cursor theme (SDDM's Wayland greeter
// ignores XCURSOR_THEME), a box with no /usr/share/icons/default draws no
// pointer at all -- the "no cursor on login/logout" break. This establishes the
// fallback, pointing "default" at the shipped Bibata set. Only ever creates the
// stub when absent, so a user's own default cursor is left untouched. Scoped to
// login boxes (a Ryoku greeter is installed) and only once ryoku-cursors, which
// ships Bibata, has landed.
func reconcileGreeterCursor(checkOnly bool) recResult {
	if !sys.Exists(greeterThemeDir) {
		return okRes(i18n.T("no Ryoku greeter installed"))
	}
	if defaultCursorEstablished() {
		return okRes(i18n.T("the \"default\" cursor theme resolves; the login screen has a pointer"))
	}
	if !cursorThemeInstalled(defaultCursorTheme, []string{"/usr/share/icons"}) {
		return warnRes(i18n.T("cursor theme %q is not on disk yet; the login screen pointer cannot be pinned"), defaultCursorTheme).
			withFix("ryoku update")
	}
	if checkOnly {
		return wouldRes(i18n.T("no \"default\" cursor theme; the SDDM greeter draws no pointer at system start or after logout")).
			withFix("ryoku doctor")
	}
	if err := writeRootFile(defaultCursorIndex, defaultCursorIndexBody(), "0644"); err != nil {
		return failRes(i18n.T("could not write %s: %v"), defaultCursorIndex, err).
			withFix(i18n.T("check sudo access, then re-run ryoku doctor"))
	}
	return fixedRes(i18n.T("pointed the \"default\" cursor theme at %s; the login screen has a pointer now"), defaultCursorTheme)
}

// ---- reconciler: fastfetch readout emblem ------------------------------------

const fastfetchEmblem = "fastfetch-emblem.png"

// fastfetchLogoSource pulls the logo image path out of a fastfetch config.jsonc
// (JSONC, so it won't json.Unmarshal). the readout declares exactly one
// "source", the logo image; comment lines are skipped. false when there is no
// source line, i.e. no Ryoku fastfetch logo to keep alive.
func fastfetchLogoSource(cfg string) (string, bool) {
	for _, ln := range strings.Split(cfg, "\n") {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, "//") || !strings.HasPrefix(ln, `"source"`) {
			continue
		}
		colon := strings.IndexByte(ln, ':')
		if colon < 0 {
			continue
		}
		rest := ln[colon+1:]
		a := strings.IndexByte(rest, '"')
		if a < 0 {
			continue
		}
		b := strings.IndexByte(rest[a+1:], '"')
		if b < 0 {
			continue
		}
		return rest[a+1 : a+1+b], true
	}
	return "", false
}

// expandTilde resolves a leading ~ to the home dir, matching how fastfetch
// expands the logo source at runtime.
func expandTilde(p string) string {
	switch {
	case p == "~":
		return sys.Home()
	case strings.HasPrefix(p, "~/"):
		return filepath.Join(sys.Home(), p[2:])
	}
	return p
}

// reconcileFastfetchEmblem keeps the branded fastfetch readout off the stock
// Arch logo. config.jsonc draws a kitty-direct logo from an image file; when
// that file is missing fastfetch SILENTLY drops to its built-in distro logo
// (empty stderr), so the terminal greets with Arch instead of the Ryoku emblem.
// the emblem now materializes into the config dir beside config.jsonc, but a box
// that updated before that shipped points config.jsonc at an emblem it never
// received. restore it from the packaged base config tree, the same file
// `ryoku materialize` lays. no-op when the readout resolves, when the logo is
// user-customized, or on a box with no Ryoku fastfetch config.
func reconcileFastfetchEmblem(checkOnly bool) recResult {
	src, ok := fastfetchLogoSource(readFileSafe(filepath.Join(sys.ConfigHome(), "fastfetch", "config.jsonc")))
	if !ok {
		return okRes(i18n.T("no Ryoku fastfetch logo configured"))
	}
	if filepath.Base(src) != fastfetchEmblem {
		return okRes(i18n.T("fastfetch logo is user-customized"))
	}
	dst := expandTilde(src)
	if sys.Exists(dst) {
		return okRes(i18n.T("fastfetch emblem present"))
	}
	// the canonical copy materialize lays; absent only on a box still on the
	// pre-fix package, where the cure is to pull it first.
	base := filepath.Join(sys.BaseConfigDir(), "fastfetch", fastfetchEmblem)
	if !sys.Exists(base) {
		return warnRes(i18n.T("fastfetch emblem missing (%s); the readout shows the Arch logo"), dst).
			withFix("ryoku update")
	}
	if checkOnly {
		return wouldRes(i18n.T("fastfetch emblem missing (%s); the readout shows the Arch logo"), dst).
			withFix("ryoku materialize")
	}
	if err := sys.CopyFile(base, dst); err != nil {
		return failRes(i18n.T("could not restore fastfetch emblem: %v"), err).withFix("ryoku materialize")
	}
	return fixedRes(i18n.T("restored the fastfetch emblem; the readout no longer falls back to the Arch logo"))
}

// ---- reconciler: fastfetch OS line -------------------------------------------

// fastfetch's OS line used to run `ryoku version`; with named releases it runs
// `ryoku version --pretty` ("Ryoku Onogoro v0.56.3-beta.19"). config.jsonc is
// seeded once and then owned by the box (the Hub, the store and the user edit
// it in place), so the shipped change never reaches an existing box on its
// own; this rewrites that one command and nothing else.
// the shipped seed writes `2>`; a config the Hub saved carries Go's JSON
// escape `2\u003e` for the same byte, so both spellings are the old line.
var fastfetchOSLineOld = []string{`ryoku version 2>/dev/null`, `ryoku version 2\u003e/dev/null`}

func upgradeFastfetchOSLine(raw string) (string, bool) {
	for _, old := range fastfetchOSLineOld {
		if strings.Contains(raw, old) {
			return strings.Replace(raw, old, strings.Replace(old, "ryoku version ", "ryoku version --pretty ", 1), 1), true
		}
	}
	return raw, false
}

func reconcileFastfetchOSLine(checkOnly bool) recResult {
	path := filepath.Join(sys.ConfigHome(), "fastfetch", "config.jsonc")
	raw := readFileSafe(path)
	if raw == "" {
		return okRes(i18n.T("no Ryoku fastfetch config"))
	}
	migrated, changed := upgradeFastfetchOSLine(raw)
	if !changed {
		return okRes(i18n.T("fastfetch's OS line names the release"))
	}
	if checkOnly {
		return wouldRes(i18n.T("fastfetch's OS line predates named releases (ryoku version -> ryoku version --pretty)")).
			withFix(i18n.T("ryoku doctor rewrites that one line"))
	}
	tmp := path + ".ryoku-tmp"
	if err := os.WriteFile(tmp, []byte(migrated), 0o644); err != nil {
		return failRes(i18n.T("could not write %s: %v"), tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return failRes(i18n.T("could not replace %s: %v"), path, err)
	}
	return fixedRes(i18n.T("fastfetch's OS line now names the release (ryoku version --pretty)"))
}

// ---- reconciler: rice fastfetch emblem ---------------------------------------

// riceEmblemAsset reports the active rice's fastfetch emblem (rice.json
// assets.fastfetch), or false when no rice is active or it carries none.
func riceEmblemAsset() (string, bool) {
	rices := filepath.Join(sys.ConfigHome(), "ryoku", "rices")
	slug := strings.TrimSpace(readFileSafe(filepath.Join(rices, ".active")))
	if slug == "" {
		return "", false
	}
	var r struct {
		Assets struct {
			Fastfetch string `json:"fastfetch"`
		} `json:"assets"`
	}
	if json.Unmarshal([]byte(readFileSafe(filepath.Join(rices, slug, "rice.json"))), &r) != nil || r.Assets.Fastfetch == "" {
		return "", false
	}
	return filepath.Join(rices, slug, r.Assets.Fastfetch), true
}

// reconcileRiceEmblem puts an applied rice's fastfetch emblem back after an
// update. `rice apply` used to copy the emblem over the SHIPPED
// fastfetch-emblem.png, which `ryoku materialize` re-lays on every update, so
// the rice's logo reset to the brand mark each time. apply now lands it on the
// user-owned ryoku-logo path; this converges boxes riced before that, exactly
// once: it acts only while the readout still draws the shipped emblem and the
// active rice carries one, and never touches a logo the user imported.
func reconcileRiceEmblem(checkOnly bool) recResult {
	asset, ok := riceEmblemAsset()
	if !ok {
		return okRes(i18n.T("no rice emblem to keep"))
	}
	if !sys.Exists(asset) {
		return okRes(i18n.T("active rice's emblem file is gone; nothing to restore"))
	}
	src, ok := fastfetchLogoSource(readFileSafe(filepath.Join(sys.ConfigHome(), "fastfetch", "config.jsonc")))
	if ok && filepath.Base(src) != fastfetchEmblem {
		return okRes(i18n.T("rice emblem in place"))
	}
	if !sys.Has("ryoku-hub") {
		return warnRes(i18n.T("the rice's fastfetch emblem was reset by an update; ryoku-hub is not installed to restore it")).
			withFix("ryoku update")
	}
	if checkOnly {
		return wouldRes(i18n.T("the rice's fastfetch emblem was reset to the brand mark by an update")).
			withFix(i18n.T("ryoku doctor (runs `ryoku-hub rice emblem`)"))
	}
	if err := sys.Run("ryoku-hub", "rice", "emblem"); err != nil {
		return failRes(i18n.T("could not restore the rice's fastfetch emblem: %v"), err).
			withFix(i18n.T("re-apply the rice from Ryoku Settings"))
	}
	return fixedRes(i18n.T("restored the rice's fastfetch emblem on a path updates never overwrite"))
}

// ---- reconciler: brand mark image --------------------------------------------

// brandMarkImage lifts the markImage override out of a brand.json body. false
// when the file does not parse, so a garbled brand leaves the reconciler
// nothing to defend (the JsonAdapter defaults, i.e. the 力 text seal, render).
func brandMarkImage(raw []byte) (string, bool) {
	var b struct {
		MarkImage string `json:"markImage"`
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		return "", false
	}
	return b.MarkImage, true
}

// clearBrandImage blanks markImage in a brand.json body while preserving every
// other field (markText / markTint / name), so a dangling image override falls
// back to the text seal without dropping the user's name or tint pick.
func clearBrandImage(raw []byte) ([]byte, error) {
	var b map[string]any
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, err
	}
	b["markImage"] = ""
	out, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// expandBrandImage resolves a markImage the way the shell does before loading
// it: drop a file:// scheme, then expand a leading ~ to the home dir. an
// already-absolute path passes through, ready to stat.
func expandBrandImage(p string) string {
	return expandTilde(strings.TrimPrefix(p, "file://"))
}

// brandImageUsable: can the shell actually load this mark image? open (catches
// a missing or permission-locked file) and reject a directory, matching the
// only two ways QML's Image comes up empty on a set source.
func brandImageUsable(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	fi, err := f.Stat()
	return err == nil && !fi.IsDir()
}

// reconcileBrandLogo keeps the desktop brand off a broken image. brand.json's
// markImage override (Ryoku Settings -> Shell -> Global) wins over the 力 text
// seal everywhere in system chrome, but a moved, deleted, or unreadable image
// leaves every branded surface (pill, launcher, fastfetch, ...) with an empty
// mark: QML's Image renders nothing on a dangling source. clear the override so
// the mark falls back to the text seal, keeping the user's name and tint pick.
// no-op when brand.json is absent (text seal always renders), the file does not
// parse, markImage is empty, or the image resolves.
func reconcileBrandLogo(checkOnly bool) recResult {
	path := filepath.Join(sys.ConfigHome(), "ryoku", "brand.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return okRes(i18n.T("no brand override yet (seeded on first shell run)"))
	}
	img, ok := brandMarkImage(raw)
	if !ok || img == "" {
		return okRes(i18n.T("brand mark uses the text seal"))
	}
	resolved := expandBrandImage(img)
	if brandImageUsable(resolved) {
		return okRes(i18n.T("brand mark image resolves (%s)"), resolved)
	}
	if checkOnly {
		return wouldRes(i18n.T("brand mark image missing or unreadable (%s); the mark renders empty"), resolved).
			withFix(i18n.T("ryoku doctor clears it back to the text seal"))
	}
	cleared, err := clearBrandImage(raw)
	if err != nil {
		return failRes(i18n.T("could not rewrite brand.json: %v"), err).withFix(i18n.T("delete %s to re-seed it"), path)
	}
	tmp := path + ".ryoku-tmp"
	if err := os.WriteFile(tmp, cleared, 0o644); err != nil {
		return failRes(i18n.T("could not write %s: %v"), tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return failRes(i18n.T("could not replace %s: %v"), path, err)
	}
	return fixedRes(i18n.T("cleared the broken brand image (%s); the mark falls back to the text seal"), img)
}

// ---- reconciler: decor art (Pictures/ryodecors) ------------------------------

// ryodecorsSource is where the shipped decor art set lives: the repo checkout on
// a dev box (ryoku deploy records it), else /usr/share/ryoku/ryodecors from the
// ryoku-desktop package. "" when neither is present (a box on a pre-fix package,
// cured by pulling the update first).
func ryodecorsSource() string {
	if repo := sys.ResolveRepo(); repo != "" {
		if p := filepath.Join(repo, "ryoku", "assets", "ryodecors"); sys.Exists(p) {
			return p
		}
	}
	if p := "/usr/share/ryoku/ryodecors"; sys.Exists(p) {
		return p
	}
	return ""
}

// reconcileRyodecors keeps the shipped decor art present in ~/Pictures/ryodecors,
// the folder the Decor and Placard components render from (beside Wallpapers and
// livewalls). The installer seeds it on a fresh install; this delivers it -- and
// any art a later release adds -- to a box that updated before it shipped, or one
// where a file went missing. Missing-only: a file the user swapped or added is
// left alone, and nothing is pruned (it is a user-owned Pictures folder).
func reconcileRyodecors(checkOnly bool) recResult {
	src := ryodecorsSource()
	if src == "" {
		return okRes(i18n.T("no decor art source yet (ships with the desktop package)"))
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return okRes(i18n.T("decor art source unreadable (%v)"), err)
	}
	dst := filepath.Join(sys.Home(), "Pictures", "ryodecors")
	var missing []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !sys.Exists(filepath.Join(dst, e.Name())) {
			missing = append(missing, e.Name())
		}
	}
	if len(missing) == 0 {
		return okRes(i18n.T("decor art present in %s"), dst)
	}
	if checkOnly {
		return wouldRes(i18n.T("%d decor art file(s) missing from %s"), len(missing), dst).
			withFix("ryoku doctor")
	}
	for _, name := range missing {
		if err := sys.CopyFile(filepath.Join(src, name), filepath.Join(dst, name)); err != nil {
			return failRes(i18n.T("could not seed decor art %s: %v"), name, err).withFix("ryoku doctor")
		}
	}
	return fixedRes(i18n.T("seeded %d decor art file(s) into %s"), len(missing), dst)
}

// ---- reconciler: follow-mouse default ---------------------------------------

// followMouseMarker records that the one-time follow-mouse heal has run, so a
// later deliberate "detached" pick in Ryoku Settings is never quietly undone.
func followMouseMarker() string {
	return filepath.Join(sys.Xdg("XDG_STATE_HOME", ".local/state"), "ryoku", "migrations", "follow-mouse-cursor-default")
}

// hyprGetFollowMouse pulls desktop.input.followMouse out of the neutral store.
func hyprGetFollowMouse(raw string) (int, bool) {
	var o struct {
		Desktop struct {
			Input struct {
				FollowMouse *int `json:"followMouse"`
			} `json:"input"`
		} `json:"desktop"`
	}
	if json.Unmarshal([]byte(raw), &o) != nil || o.Desktop.Input.FollowMouse == nil {
		return 0, false
	}
	return *o.Desktop.Input.FollowMouse, true
}

// hyprSetFollowMouse rewrites desktop.input.followMouse in the neutral store,
// preserving every other field.
func hyprSetFollowMouse(raw string, v int) (string, error) {
	var doc map[string]any
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return "", err
	}
	desktop, _ := doc["desktop"].(map[string]any)
	if desktop == nil {
		desktop = map[string]any{}
		doc["desktop"] = desktop
	}
	input, _ := desktop["input"].(map[string]any)
	if input == nil {
		input = map[string]any{}
		desktop["input"] = input
	}
	input["followMouse"] = v
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// reconcileFollowMouseDefault: focus follows the cursor by default on every
// compositor. Stores written while the detached experiment shipped keep its 2
// baked in, so the pointer walks over a window without focusing it; restore 1
// once and drop a marker, so re-picking "detached" (2) in Settings afterwards
// sticks.
func reconcileFollowMouseDefault(checkOnly bool) recResult {
	marker := followMouseMarker()
	if sys.Exists(marker) {
		return okRes(i18n.T("follow-mouse default already reconciled"))
	}
	mark := func() {
		if checkOnly {
			return
		}
		_ = os.MkdirAll(filepath.Dir(marker), 0o755)
		_ = os.WriteFile(marker, []byte("done\n"), 0o644)
	}
	store := filepath.Join(sys.ConfigHome(), "ryoku", "desktop.json")
	if !sys.Exists(store) {
		mark() // nothing saved to migrate; the shipped base default (1) stands.
		return okRes(i18n.T("no saved desktop input; follow-mouse uses the base default"))
	}
	raw := readFileSafe(store)
	fm, ok := hyprGetFollowMouse(raw)
	if !ok || fm != 2 {
		mark()
		return okRes(i18n.T("follow-mouse is not on the retired detached default"))
	}
	if checkOnly {
		return wouldRes(i18n.T("follow-mouse is pinned to the retired detached default 2; focus should follow the cursor")).
			withFix("ryoku doctor")
	}
	fixed, err := hyprSetFollowMouse(raw, 1)
	if err != nil {
		return failRes(i18n.T("could not update desktop settings: %v"), err)
	}
	if err := writeStore(store, []byte(fixed)); err != nil {
		return failRes(i18n.T("could not save the follow-mouse fix: %v"), err).withFix("ryoku doctor")
	}
	_, _ = wm.Open().Apply(store)
	mark()
	return fixedRes(i18n.T("restored follow-mouse to 1; keyboard focus follows the cursor"))
}

// ---- reconciler: ryoku shell daemon ------------------------------------------

// reconcileShellDaemon: is the Ryoku shell control plane alive? the daemon
// (`ryoku-shell daemon`, autostarted by Hyprland) owns the Unix socket every
// keybind and quickshell component talks to -- if it dies, the whole shell
// is dead while the session target still looks up. Hyprland starts it once
// at login, so a crash leaves nothing to bring it back: that's what doctor
// is for. inside a live session it restarts the daemon; from a TTY or ssh
// there's no shell to manage, so it stays quiet.
func reconcileShellDaemon(checkOnly bool) recResult {
	live := liveInstance()
	if live == "" {
		return okRes(i18n.T("not in a live session"))
	}
	if !sys.Has("ryoku-shell") {
		return warnRes(i18n.T("ryoku-shell is not installed; the desktop shell cannot run")).
			withFix(i18n.T("redeploy the shell: `ryoku update` (or ryoku/shell/deploy.sh from a checkout)"))
	}
	if shellDaemonReachable() {
		// A reachable daemon can still be stale. One left over from a previous
		// Hyprland instance -- it survived a relogin or crash (deploy starts it
		// detached, and it need not die with the compositor) -- keeps answering
		// ping, but it and every quickshell child it supervises stay pinned to
		// the dead instance's IPC socket: the workspace indicator freezes and
		// power (any monitor-aware command) resolves no monitor. Compare the
		// daemon's instance to this live session and restart a mismatch.
		sig, ok := shellDaemonSignature()
		if !daemonIsStale(live, sig, ok) {
			// Same instance, but the binary under it may be gone: an update
			// that replaced /usr/bin/ryoku-shell without quiescing the shell
			// (updaters before beta-17 did) leaves the old daemon serving
			// surfaces that hot-reload QML newer than it can host -- the
			// "module Ryoku.FrameBars is not installed" class of breakage.
			if !shellDaemonOutdated() {
				return okRes(i18n.T("shell daemon reachable"))
			}
			if checkOnly {
				return wouldRes(i18n.T("shell daemon is running a replaced binary; its surfaces load config newer than it")).
					withFix(i18n.T("ryoku doctor restarts the daemon on the installed binary"))
			}
			if err := restartShellDaemon(); err != nil {
				return failRes(i18n.T("shell daemon runs a replaced binary and could not be restarted: %v"), err).
					withFix(i18n.T("`ryoku-shell quit`, then `ryoku-shell daemon` in a terminal"))
			}
			if waitDaemonReachable(5 * time.Second) {
				return fixedRes(i18n.T("shell daemon was running a replaced binary; restarted it on the installed one"))
			}
			return failRes(i18n.T("restarted the outdated shell daemon but it did not come back")).
				withFix(i18n.T("run `ryoku-shell daemon` in a terminal to see why it exits"))
		}
		if checkOnly {
			return wouldRes(i18n.T("shell daemon is bound to a previous Hyprland instance; workspaces and monitor-aware menus are dead")).
				withFix(i18n.T("ryoku doctor restarts the daemon against the live session"))
		}
		if err := restartShellDaemon(); err != nil {
			return failRes(i18n.T("shell daemon is stale and could not be restarted: %v"), err).
				withFix(i18n.T("`ryoku-shell quit`, then `ryoku-shell daemon` in a terminal"))
		}
		if waitDaemonReachable(5 * time.Second) {
			return fixedRes(i18n.T("shell daemon was bound to a dead Hyprland instance; restarted it against the live session"))
		}
		return failRes(i18n.T("restarted the stale shell daemon but it did not come back")).
			withFix(i18n.T("run `ryoku-shell daemon` in a terminal to see why it exits"))
	}
	if checkOnly {
		return wouldRes(i18n.T("shell daemon is down; the shell, keybinds and panels are dead")).
			withFix(i18n.T("start it with `ryoku-shell daemon` (Hyprland autostarts it at login)"))
	}
	if err := startShellDaemon(); err != nil {
		return failRes(i18n.T("shell daemon is down and could not be started: %v"), err).
			withFix(i18n.T("start it in a terminal to see why: `ryoku-shell daemon`"))
	}
	if waitDaemonReachable(5 * time.Second) {
		return fixedRes(i18n.T("shell daemon was down; restarted it"))
	}
	return failRes(i18n.T("started ryoku-shell daemon but it did not come up")).
		withFix(i18n.T("run `ryoku-shell daemon` in a terminal to see why it exits"))
}

// shellDaemonReachable dials the shell control socket and pings it: same
// round-trip the keybinds make. a stale socket from a crashed daemon refuses
// the connection; a hung daemon accepts but never replies, so the read is
// bounded.
func shellDaemonReachable() bool {
	conn, err := net.DialTimeout("unix", shellSockPath(), time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := fmt.Fprintln(conn, "ping"); err != nil {
		return false
	}
	buf := make([]byte, 64)
	n, _ := conn.Read(buf)
	return strings.TrimSpace(string(buf[:n])) == "ok"
}

// startShellDaemon launches `ryoku-shell daemon` detached from doctor: own
// session so it outlives this process, stdio to /dev/null. the daemon clears
// a stale socket and refuses to double-start, so this is only safe to call
// when the socket is already unreachable. On a packaged box the packaged
// binary is preferred over PATH, where dev/recovery residue in ~/.local/bin
// would resurrect a stale daemon; a checkout box's home deploy IS the
// desktop, so PATH is right there.
func startShellDaemon() error {
	// Prefer the unit so a recovered daemon stays supervised; the reload lets a
	// freshly delivered unit be found. Falls through to a bare start where the
	// unit does not exist, so recovery never depends on it. The env push first:
	// if login's import never reached the user manager, the unit's
	// ConditionEnvironment=WAYLAND_DISPLAY skips it and restart "succeeds"
	// while starting nothing.
	_ = exec.Command("dbus-update-activation-environment", "--systemd", "--all").Run()
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	if exec.Command("systemctl", "--user", "restart", "ryoku-shell").Run() == nil {
		return nil
	}
	shell := "ryoku-shell"
	if sys.ResolveRepo() == "" && sys.Exists("/usr/bin/ryoku-shell") {
		shell = "/usr/bin/ryoku-shell"
	}
	cmd := exec.Command(shell, "daemon")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if devnull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0); err == nil {
		cmd.Stdin, cmd.Stdout, cmd.Stderr = devnull, devnull, devnull
		defer devnull.Close()
	}
	return cmd.Start()
}

// waitDaemonReachable polls until the daemon answers or the deadline passes.
// it needs a moment to bind the socket and bootstrap.
func waitDaemonReachable(d time.Duration) bool {
	deadline := time.Now().Add(d)
	for {
		if shellDaemonReachable() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(150 * time.Millisecond)
	}
}

// shellSockPath is the shell daemon's control socket -- the one the keybinds and
// quickshell components dial.
func shellSockPath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = "/tmp"
	}
	return filepath.Join(dir, "ryoku-shell.sock")
}

// shellDaemonSignature asks the running daemon which compositor instance it was
// launched under (the `signature` command). ok is false when the query fails or
// the daemon predates the command, so a "can't tell" is never read as stale.
func shellDaemonSignature() (sig string, ok bool) {
	conn, err := net.DialTimeout("unix", shellSockPath(), time.Second)
	if err != nil {
		return "", false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := fmt.Fprintln(conn, "signature"); err != nil {
		return "", false
	}
	buf := make([]byte, 4096)
	n, _ := conn.Read(buf)
	resp := strings.TrimSpace(string(buf[:n]))
	if resp == "" || strings.HasPrefix(resp, "err ") {
		return "", false
	}
	return resp, true
}

// daemonIsStale reports whether a reachable daemon is bound to a different
// compositor instance than this session (live), from the signature it reported
// and whether that report was usable (ok). "can't tell" (ok=false) is never
// stale, so doctor never restarts a daemon it could not identify.
func daemonIsStale(live, sig string, ok bool) bool {
	return ok && live != "" && sig != live
}

// liveInstance is the running compositor's opaque per-session handle, or "" when
// nothing is live, compared against the daemon's reported handle to spot a
// daemon left bound to a compositor that has since restarted.
func liveInstance() string {
	caps, err := wm.Open().Caps()
	if err != nil {
		return ""
	}
	return caps.Instance
}

// shellDaemonOutdated: is the running daemon's binary gone from disk? pacman
// replacing /usr/bin/ryoku-shell (or a deploy replacing the home build) turns
// the daemon's /proc exe link into "... (deleted)". Matches by exact cmdline,
// never pgrep -f, so doctor's own shell can't shadow the answer.
func shellDaemonOutdated() bool {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false
	}
	for _, e := range entries {
		pid := e.Name()
		if pid[0] < '0' || pid[0] > '9' {
			continue
		}
		cmd, err := os.ReadFile("/proc/" + pid + "/cmdline")
		if err != nil {
			continue
		}
		exe, _ := os.Readlink("/proc/" + pid + "/exe")
		if daemonBinaryReplaced(string(cmd), exe) {
			return true
		}
	}
	return false
}

// daemonBinaryReplaced decides from a /proc cmdline and exe link whether this
// process is the shell daemon left running on a deleted binary.
func daemonBinaryReplaced(cmdline, exeLink string) bool {
	fields := strings.Split(cmdline, "\x00")
	if len(fields) < 2 || fields[1] != "daemon" || filepath.Base(fields[0]) != "ryoku-shell" {
		return false
	}
	return strings.HasSuffix(exeLink, " (deleted)")
}

// restartShellDaemon replaces a stale daemon with one bound to the live session:
// quit the incumbent (so it reaps its own quickshell children and frees the
// socket), then start a fresh daemon, which inherits doctor's live session
// environment and passes it to every component it supervises.
func restartShellDaemon() error {
	quitShellDaemon()
	return startShellDaemon()
}

// quitShellDaemon sends the daemon a quit and waits, bounded, for the control
// socket to go quiet, so the fresh daemon binds without racing the old one's
// teardown.
func quitShellDaemon() {
	if conn, err := net.DialTimeout("unix", shellSockPath(), time.Second); err == nil {
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
		fmt.Fprintln(conn, "quit")
		conn.Close()
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !shellDaemonReachable() {
			return
		}
		time.Sleep(150 * time.Millisecond)
	}
}

// ---- reconciler: Hyprland config integrity -----------------------------------

// hyprDropin: a runtime-generated Hyprland Lua drop-in. hyprland.lua loads
// monitors.lua (ryoku-monitor) and gpu.lua (ryoku-gpu); both get rewritten
// while the session is live (a display hotplug or a GPU reset re-runs the
// generators), so a crash or a torn write can leave one unparseable. on the
// next Hyprland reload the whole config gets rejected and the compositor
// drops into on-screen emergency mode -- the "reload/doctor/update do
// nothing, only a reboot fixes it" failure -- until the file is repaired.
type hyprDropin struct {
	name     string   // file under ~/.config/hypr
	regen    []string // generator that rewrites it from live state
	needLive bool     // generator needs a running compositor
	seed     string   // known-good fallback: always parseable, safe default
}

func hyprDropins() []hyprDropin {
	return []hyprDropin{
		{
			name:     "monitors.lua",
			regen:    []string{"ryoku-monitor", "persist"},
			needLive: true,
			seed: "-- Reset by ryoku doctor after a corrupt write. Brings every output up at a\n" +
				"-- safe 1x; `ryoku-monitor autoscale` (runs at the next login) restores scaling.\n" +
				"hl.monitor({ output = \"\", mode = \"highrr\", position = \"auto\", scale = 1 })\n",
		},
		{
			name:     "gpu.lua",
			regen:    []string{"ryoku-gpu", "persist"},
			needLive: false,
			seed: "-- Reset by ryoku doctor after a corrupt write. Hyprland picks its own GPU;\n" +
				"-- run `ryoku-gpu persist` to re-pin the primary on a multi-GPU machine.\n",
		},
		{
			// user-owned and seeded once, so materialize never repairs it. no
			// generator to re-run: reseed the default layout and let the user
			// re-pick in settings (input).
			name:     "keyboard.lua",
			needLive: false,
			seed: "-- Reset by ryoku doctor after a corrupt write. Re-pick your layout in\n" +
				"-- ryoku settings (input), or edit this file; see the wiki for variants.\n" +
				"hl.config({\n" +
				"    input = {\n" +
				"        kb_layout = \"us\",\n" +
				"        kb_variant = \"\",\n" +
				"        kb_options = \"\",\n" +
				"    },\n" +
				"})\n",
		},
	}
}

// reconcileHyprlandConfig keeps the runtime-generated drop-ins loadable: a torn
// one wedges the next reload into emergency mode. It validates each parses and
// regenerates a corrupt one, reloading a live session after. Whether the emitted
// config is honoured is the provider's ApplyReport, not a live buffer doctor probes.
func reconcileHyprlandConfig(checkOnly bool) recResult {
	// Hyprland's drop-ins are only in play while Hyprland is the live window
	// manager: a niri session neither reads them nor can reload them, so the
	// check would report on and repair files nothing in that session uses.
	if name := wm.Detect().Name; name != "" && name != wm.ProviderHyprland {
		return okRes(i18n.T("no Hyprland session"))
	}
	dir := filepath.Join(sys.ConfigHome(), wm.ConfigDir(wm.ProviderHyprland))
	if !sys.Exists(filepath.Join(dir, "hyprland.lua")) {
		return okRes(i18n.T("no Hyprland config present"))
	}
	live := wm.Detect().Live

	if checkOnly {
		var broken []string
		for _, d := range hyprDropins() {
			p := filepath.Join(dir, d.name)
			if sys.Exists(p) && !hyprLuaParseable(p) {
				broken = append(broken, d.name)
			}
		}
		if len(broken) > 0 {
			return wouldRes(i18n.T("corrupt Hyprland drop-in(s) would wedge the next reload into emergency mode: %s"), strings.Join(broken, ", ")).
				withFix(i18n.T("run `ryoku doctor` to regenerate them"))
		}
		return okRes(i18n.T("Hyprland config loads cleanly"))
	}

	var repaired, failed []string
	for _, d := range hyprDropins() {
		p := filepath.Join(dir, d.name)
		if !sys.Exists(p) || hyprLuaParseable(p) {
			continue
		}
		if repairHyprDropin(dir, d, live) {
			repaired = append(repaired, d.name)
		} else {
			failed = append(failed, d.name)
		}
	}

	// A clean reload pulls a live session out of emergency mode right away.
	if live && len(repaired) > 0 {
		_ = wm.Open().Act(wm.ActionConfigReload)
	}

	switch {
	case len(failed) > 0:
		return failRes(i18n.T("could not repair Hyprland drop-in(s): %s"), strings.Join(failed, ", ")).
			withFix(i18n.T("inspect ~/.config/hypr/%s by hand"), failed[0])
	case len(repaired) > 0:
		return fixedRes(i18n.T("regenerated corrupt Hyprland drop-in(s): %s; the config loads cleanly again"), strings.Join(repaired, ", "))
	}
	return okRes(i18n.T("Hyprland config loads cleanly"))
}

// ---- reconciler: niri config integrity ---------------------------------------

// missingInclude reports that niri could not read a file config.kdl includes.
// niri says "failed to read included config from \"<path>\": No such file or
// directory"; both halves are niri's own vocabulary, checked here so doctor can
// tell "not applied yet" from "written badly".
func missingInclude(out []byte) bool {
	s := string(out)
	return strings.Contains(s, "failed to read included config") &&
		strings.Contains(s, "No such file or directory")
}

// reconcileNiriConfig validates the config the session will read, the way niri
// itself reads it. config.kdl includes the generated files by name, and a
// missing or unparseable include is fatal to the session rather than a rejected
// reload, so a file an update authored badly costs the user the login. The
// provider validates nothing before writing, so niri's own parser is the only
// oracle: doctor runs it over the installed tree.
//
// Repair re-authors the provider's config from the neutral store, the same way a
// store fix reaches the session elsewhere in doctor, then validates again.
func reconcileNiriConfig(checkOnly bool) recResult {
	if wm.Detect().Name != wm.ProviderNiri {
		return okRes(i18n.T("no niri session"))
	}
	entry := filepath.Join(sys.ConfigHome(), wm.ConfigDir(wm.ProviderNiri), "config.kdl")
	if !sys.Exists(entry) {
		return okRes(i18n.T("no niri config present"))
	}
	if _, err := exec.LookPath("niri"); err != nil {
		return noteRes(i18n.T("niri config not checked (niri is not on PATH)"))
	}
	failure := func(out []byte) string {
		first := strings.TrimSpace(string(out))
		if i := strings.IndexByte(first, '\n'); i >= 0 {
			first = first[:i]
		}
		return first
	}
	out, err := exec.Command("niri", "validate", "-c", entry).CombinedOutput()
	if err == nil {
		return okRes(i18n.T("niri config loads cleanly"))
	}
	// The provider generates the files config.kdl includes (settings.kdl,
	// rebinds.kdl) when it applies, so a config that is missing an include has
	// simply not been applied yet on this box: the next apply writes it. That is
	// news for a report, not a fault, and nothing to repair by hand.
	if missingInclude(out) {
		if checkOnly {
			return noteRes(i18n.T("niri config is not applied yet (an include is still to be written)"))
		}
	}
	if checkOnly {
		return warnRes(i18n.T("niri config does not load: %s"), failure(out)).
			withFix(i18n.T("ryoku doctor"))
	}
	store := filepath.Join(sys.ConfigHome(), "ryoku", "desktop.json")
	if _, err := wm.Open().Apply(store); err != nil {
		return failRes(i18n.T("niri config does not load and re-applying the settings failed: %v"), err)
	}
	if out, err := exec.Command("niri", "validate", "-c", entry).CombinedOutput(); err != nil {
		return failRes(i18n.T("niri config still does not load: %s"), failure(out)).
			withFix(i18n.T("fix %s by hand"), entry)
	}
	return fixedRes(i18n.T("re-authored the niri config; it loads cleanly again"))
}

// reconcileThemeLua prunes an orphaned ~/.config/hypr/theme.lua. The Appearance
// Themes feature copied a rice's motion Lua there and hyprland.lua loaded it via
// optional("theme"); both are gone now, so the file no longer loads and lingers
// as dead state, only on a box that had a theme applied. Remove it so the config
// dir matches the shipped layout. Idempotent: ok when absent.
func reconcileThemeLua(checkOnly bool) recResult {
	p := filepath.Join(sys.ConfigHome(), "hypr", "theme.lua")
	if !sys.Exists(p) {
		return okRes(i18n.T("no orphaned theme.lua"))
	}
	if checkOnly {
		return wouldRes(i18n.T("orphaned theme.lua from the retired Themes feature: %s"), p).
			withFix(i18n.T("run `ryoku doctor` to remove it"))
	}
	if err := os.Remove(p); err != nil {
		return failRes(i18n.T("could not remove orphaned theme.lua: %v"), err).
			withFix(i18n.T("remove %s by hand"), p)
	}
	return fixedRes(i18n.T("removed the orphaned theme.lua left by the retired Themes feature"))
}

// reconcileDisplayModes recovers a monitor a degraded link left below its
// available resolution. after a cold boot or a post-upgrade config reload,
// a DP/HDMI link can briefly advertise only a VESA fallback (e.g. 800x600);
// Hyprland resolves monitors.lua's `highrr` against that list and never
// re-picks once the link trains, so the panel stays low-res until a relogin.
// `ryoku-monitor settle` re-asserts each output's intended mode (respecting
// an explicit Ryoku Settings pick and monitors_user.lua); `settle --check`
// is the read-only signal. live-only: no session = nothing to re-assert, the
// next login takes care of it.
func reconcileDisplayModes(checkOnly bool) recResult {
	if !wm.Detect().Live {
		return okRes(i18n.T("no live session; displays settle at the next login"))
	}
	if !sys.Has("ryoku-monitor") {
		return okRes(i18n.T("ryoku-monitor not installed"))
	}
	if exec.Command("ryoku-monitor", "settle", "--check").Run() == nil {
		return okRes(i18n.T("every display is at its best available resolution"))
	}
	if checkOnly {
		return wouldRes(i18n.T("a display is below its available resolution (the link came up degraded)")).
			withFix(i18n.T("ryoku doctor (re-asserts each display's intended mode)"))
	}
	if err := exec.Command("ryoku-monitor", "settle").Run(); err != nil {
		return warnRes(i18n.T("a display is below its available resolution and ryoku-monitor settle did not recover it")).
			withFix(i18n.T("open Ryoku Settings > Displays and pick the resolution, or replug the cable"))
	}
	return fixedRes(i18n.T("re-asserted a display that came up below its available resolution"))
}

// repairHyprDropin rewrites a corrupt drop-in: regenerate from the live box
// if the generator is available, else fall back to the safe seed. both
// outcomes get re-validated, so even a generator that wrote garbage ends at
// a parseable file.
func repairHyprDropin(dir string, d hyprDropin, live bool) bool {
	p := filepath.Join(dir, d.name)
	if len(d.regen) > 0 && sys.Has(d.regen[0]) && (!d.needLive || live) {
		if exec.Command(d.regen[0], d.regen[1:]...).Run() == nil && hyprLuaParseable(p) {
			return true
		}
	}
	if os.WriteFile(p, []byte(d.seed), 0o644) != nil {
		return false
	}
	return hyprLuaParseable(p)
}

// hyprLuaParseable: will this Lua drop-in load? luac -p is definitive (the
// lua toolchain ships with Hyprland's config stack). without it, fall back
// to catching the truncation failure mode -- empty file or unbalanced
// brackets, which a whole generated drop-in (no brackets inside its string
// literals) never has. unreadable file is left alone, not clobbered.
func hyprLuaParseable(path string) bool {
	if sys.Has("luac") {
		return exec.Command("luac", "-p", path).Run() == nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	return hyprLuaSane(string(b))
}

func hyprLuaSane(s string) bool {
	if strings.TrimSpace(s) == "" {
		return false
	}
	return balancedRunes(s, '(', ')') && balancedRunes(s, '{', '}')
}

func balancedRunes(s string, open, shut rune) bool {
	depth := 0
	for _, r := range s {
		switch r {
		case open:
			depth++
		case shut:
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

// ---- reconciler: failed systemd units ----------------------------------------

func reconcileFailedUnits(checkOnly bool) recResult {
	// Clear the lingering transient app scopes (safe: the app is already gone),
	// then report whatever real failures remain.
	var reset int
	if !checkOnly {
		for _, u := range failedUnits("--user") {
			if transientAppScope(u) && sys.Run("systemctl", "--user", "reset-failed", u) == nil {
				reset++
			}
		}
	}
	var failed []string
	failed = append(failed, failedUnits()...)
	for _, u := range failedUnits("--user") {
		if checkOnly && transientAppScope(u) {
			failed = append(failed, u+i18n.T(" (user, clearable)"))
		} else {
			failed = append(failed, u+i18n.T(" (user)"))
		}
	}
	if len(failed) == 0 {
		if reset > 0 {
			return fixedRes(i18n.T("reset %d stale app scope(s)"), reset)
		}
		return okRes(i18n.T("no failed services"))
	}
	return warnRes(i18n.T("failed: %s"), strings.Join(failed, ", ")).
		withFix(i18n.T("inspect with `systemctl status <unit>` and `journalctl -u <unit>`"))
}

// ---- reconciler: btrfs device health -----------------------------------------

func reconcileBtrfsHealth(_ bool) recResult {
	if !sys.IsBtrfs("/") {
		return okRes(i18n.T("root is not btrfs"))
	}
	opts, _ := sys.RunOut("findmnt", "-n", "-o", "OPTIONS", "/")
	if first := strings.SplitN(strings.TrimSpace(opts), ",", 2); len(first) > 0 && first[0] == "ro" {
		return warnRes(i18n.T("root filesystem is mounted read-only (btrfs may be protecting itself)")).
			withFix(i18n.T("check `btrfs filesystem usage /`; may need `btrfs balance` or more free space"))
	}
	stats, err := sys.RunOut("sudo", "-n", "btrfs", "device", "stats", "/")
	if err != nil || strings.TrimSpace(stats) == "" {
		return okRes(i18n.T("btrfs ok (device error counters need root for full detail)"))
	}
	for _, l := range nonEmptyLines(stats) {
		f := strings.Fields(l)
		if len(f) == 2 && f[1] != "0" {
			return warnRes(i18n.T("btrfs reports device errors: %s"), strings.TrimSpace(l)).
				withFix(i18n.T("back up, then `sudo btrfs scrub start /`; a non-zero counter can mean a failing disk"))
		}
	}
	return okRes(i18n.T("btrfs device error counters clean"))
}

// ---- reconciler: pending .pacnew config --------------------------------------

// pacnewOutcome classifies one .pacnew against its live file. pure, so the
// resolve decision is unit-testable without root or a real /etc.
type pacnewOutcome int

const (
	pacnewIdentical pacnewOutcome = iota // bytes match: pacman's new default is already in place
	pacnewRyokuOnly                      // differs only by Ryoku's deterministic [ryoku] repo stanza
	pacnewLocaleGen                      // locale.gen re-shipping the template that re-comments the user's locale
	pacnewConflict                       // a real merge only a human should make
)

// classifyPacnew decides whether a .pacnew is safe to drop. identical bytes mean
// the packaged default already equals the live file (the flagged edit was
// reverted, or the bump was metadata-only). the [ryoku] repo stanza the
// installer appends to pacman.conf is Ryoku's own deterministic addition, not a
// user edit: a live pacman.conf that equals the .pacnew once that stanza is
// stripped carries nothing to merge. anything else is a genuine conflict left
// for the user + pacdiff.
func classifyPacnew(livePath string, live, pacnew []byte) pacnewOutcome {
	if bytes.Equal(live, pacnew) {
		return pacnewIdentical
	}
	if filepath.Base(livePath) == "pacman.conf" &&
		bytes.Equal(trimTrailing(stripRyokuRepoStanza(live)), trimTrailing(pacnew)) {
		return pacnewRyokuOnly
	}
	// locale.gen perpetually .pacnews: the installer uncomments the user's
	// locale, so every glibc bump re-ships the all-commented template (and now and
	// then a new obscure commented locale). On a configured system it is never
	// user-actionable -- the active locales live in the real file and dropping the
	// .pacnew never touches them -- so clear it instead of nagging forever.
	if filepath.Base(livePath) == "locale.gen" && localeConfigured(live) {
		return pacnewLocaleGen
	}
	return pacnewConflict
}

var localeCharsetRe = regexp.MustCompile(`^[A-Z0-9][A-Z0-9._-]*$`)

// localeConfigured reports whether locale.gen has at least one active
// (uncommented) locale definition, i.e. a normal configured system whose
// locale.gen.pacnew is just the re-shipped template. A charset-shaped second
// field distinguishes a locale line from the prose header.
func localeConfigured(live []byte) bool {
	sc := bufio.NewScanner(bytes.NewReader(live))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.ContainsAny(fields[0], "_.") && localeCharsetRe.MatchString(fields[1]) {
			return true
		}
	}
	return false
}

// stripRyokuRepoStanza removes the `[ryoku]` section (and a single blank
// separator before it) the installer appends to pacman.conf, so what remains is
// the base config pacman ships. a later section header ends the stanza, so a
// user's edits after [ryoku] survive the comparison.
func stripRyokuRepoStanza(conf []byte) []byte {
	lines := strings.Split(string(conf), "\n")
	out := make([]string, 0, len(lines))
	inRyoku := false
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if t == "[ryoku]" {
			inRyoku = true
			for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
				out = out[:len(out)-1]
			}
			continue
		}
		if inRyoku {
			if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
				inRyoku = false // a new section begins; keep it
			} else {
				continue // still inside the [ryoku] stanza
			}
		}
		out = append(out, l)
	}
	return []byte(strings.Join(out, "\n"))
}

// trimTrailing drops trailing whitespace/newlines so a lone trailing-newline
// difference never reads as a conflict.
func trimTrailing(b []byte) []byte { return bytes.TrimRight(b, " \t\r\n") }

// reconcilePacnew resolves the .pacnew files pacman drops when it upgrades a
// package whose tracked /etc file the install (or a later edit) changed. it
// auto-clears only the provably safe ones -- a .pacnew identical to the live
// file, or a pacman.conf whose sole diff is the [ryoku] repo stanza the
// installer appends -- and never overwrites a user-modified base config with the
// packaged default. genuine merges are reported for `sudo pacdiff`. idempotent:
// once the safe ones are gone a re-run only sees (and reports) the conflicts.
func reconcilePacnew(checkOnly bool) recResult {
	out, _ := sys.RunOut("find", "/etc", "-name", "*.pacnew")
	files := nonEmptyLines(out)
	if len(files) == 0 {
		return okRes(i18n.T("no pending config updates"))
	}
	resolved, conflicts := 0, 0
	for _, pacnew := range files {
		live := strings.TrimSuffix(pacnew, ".pacnew")
		lb, lerr := os.ReadFile(live)
		pb, perr := os.ReadFile(pacnew)
		if lerr != nil || perr != nil || classifyPacnew(live, lb, pb) == pacnewConflict {
			conflicts++
			continue
		}
		if checkOnly {
			resolved++
			continue
		}
		if err := sys.Sudo("rm", "-f", pacnew); err != nil {
			conflicts++
			continue
		}
		resolved++
	}
	if conflicts == 0 {
		if checkOnly {
			return wouldRes(i18n.T("%d pending .pacnew are safe to drop (identical to the live config, only the [ryoku] repo addition, or a re-commented locale.gen)"), resolved)
		}
		return fixedRes(i18n.T("cleared %d safe .pacnew (identical to the live config, only the [ryoku] repo addition, or a re-commented locale.gen)"), resolved)
	}
	msg := warnRes(i18n.T("%d pending config update(s) (.pacnew) need review"), conflicts)
	if resolved > 0 {
		verb := i18n.T("cleared")
		if checkOnly {
			verb = i18n.T("safe to drop")
		}
		msg = warnRes(i18n.T("%d pending config update(s) (.pacnew) need review (%d %s)"), conflicts, resolved, verb)
	}
	return msg.withFix(i18n.T("review and merge with `sudo pacdiff` (from pacman-contrib)"))
}

// ---- reconciler: orphaned packages -------------------------------------------

func reconcileOrphans(_ bool) recResult {
	out, err := sys.RunOut("pacman", "-Qtdq")
	orphans := nonEmptyLines(out)
	if err != nil || len(orphans) == 0 {
		return okRes(i18n.T("no orphaned packages"))
	}
	return noteRes(i18n.T("%d orphaned package(s)"), len(orphans)).
		withFix(i18n.T("review `pacman -Qtd`, then `sudo pacman -Rns $(pacman -Qtdq)` if unneeded"))
}

// ---- swap helpers ------------------------------------------------------------

type swapFile struct {
	path   string
	sizeKB int64
}

func activeSwapFiles() []swapFile {
	b, err := os.ReadFile("/proc/swaps")
	if err != nil {
		return nil
	}
	return parseProcSwaps(string(b))
}

// parseProcSwaps: file-backed swaps from /proc/swaps content. first line is
// a header; the path field escapes spaces as \040.
func parseProcSwaps(s string) []swapFile {
	var out []swapFile
	sc := bufio.NewScanner(strings.NewReader(s))
	for i := 0; sc.Scan(); i++ {
		if i == 0 {
			continue
		}
		f := strings.Fields(sc.Text())
		if len(f) < 3 || f[1] != "file" {
			continue
		}
		size, err := strconv.ParseInt(f[2], 10, 64)
		if err != nil {
			continue
		}
		out = append(out, swapFile{path: strings.ReplaceAll(f[0], `\040`, " "), sizeKB: size})
	}
	return out
}

func dirOnlyContains(dir, name string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	return len(entries) == 1 && entries[0].Name() == name
}

// relocateSwapToSubvolume: swapoff the file, turn its dir into a btrfs
// subvolume, recreate the swapfile inside at the same path + size, swap it
// back on. path is unchanged, so the fstab swap entry still resolves and
// the nested subvolume comes up with its parent -- no fstab edit.
func relocateSwapToSubvolume(sw swapFile, dir string) error {
	steps := [][]string{
		{"swapoff", sw.path},
		{"rm", "-f", sw.path},
		{"rmdir", dir},
		{"btrfs", "subvolume", "create", dir},
		{"btrfs", "filesystem", "mkswapfile", "--size", fmt.Sprintf("%dk", sw.sizeKB), sw.path},
		{"swapon", sw.path},
	}
	for _, s := range steps {
		if err := sys.Run("sudo", s...); err != nil {
			return fmt.Errorf("%s: %w", strings.Join(s, " "), err)
		}
	}
	return nil
}

// ---- small shared helpers ----------------------------------------------------

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

func anyPkgInstalled(names ...string) bool {
	for _, n := range names {
		if sys.PkgInstalled(n) {
			return true
		}
	}
	return false
}

func processRunning(name string) bool {
	return exec.Command("pgrep", "-x", name).Run() == nil
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

// captureOut runs a command and returns combined output (or the error text):
// best-effort diagnostics where a non-zero exit is still informative.
func captureOut(name string, args ...string) string {
	out, err := exec.Command(name, args...).CombinedOutput()
	s := strings.TrimRight(string(out), "\n")
	if s == "" && err != nil {
		return "(" + err.Error() + ")"
	}
	if s == "" {
		return "(none)"
	}
	return s
}

func readFileSafe(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return "(" + err.Error() + ")"
	}
	return strings.TrimRight(string(b), "\n")
}

func tailLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// ---- reconciler: unowned Ryoku system files (deploy-seeded) -------------------

// ryokuSystemGlobs are the ryoku-desktop-owned paths that the ISO installer
// (bootloader.sh) and ryoku/shell deploy.sh also seed unowned: the privileged
// helpers + their polkit rules (so a dev checkout's pkexec has a rule to match),
// the ryoku-owned systemd units, the shipped boot configs under
// /usr/share/ryoku/boot, the Plymouth splash theme, and the logind lid-switch
// drop-in. On a packaged box an unowned copy from an earlier dev deploy, an
// older ISO, or `ryoku recovery` collides with the package on `pacman -Syu`
// ("exists in filesystem") and aborts the whole atomic transaction, so no update
// lands. `ryoku update` now passes --overwrite for these
// (updater.ryokuOverwriteGlob), but a box already wedged cannot reach that fixed
// binary; clearing the copies here lets the next update adopt them.
var ryokuSystemGlobs = []string{
	"/usr/bin/ryoku-*",
	"/usr/lib/systemd/system/ryoku-*",
	"/usr/share/polkit-1/rules.d/*ryoku*.rules",
	"/usr/share/plymouth/themes/ryoku/*",
	"/usr/share/ryoku/boot/*",
	"/etc/systemd/logind.conf.d/10-ryoku-lid.conf",
}

// pkgOwnsFile reports whether an installed package owns path. A var so tests stub
// the probe without a real pacman database.
var pkgOwnsFile = func(path string) bool {
	return sys.Run("pacman", "-Qo", path) == nil
}

// strayRyokuFiles returns the files matching globs that pacman does not own: the
// deploy-seeded leftovers that block -Syu. Pure over its injected probes, so the
// selection is unit-testable without a real filesystem or pacman.
func strayRyokuFiles(globs []string, glob func(string) ([]string, error), owned func(string) bool) []string {
	var out []string
	seen := map[string]bool{}
	for _, g := range globs {
		matches, _ := glob(g)
		for _, m := range matches {
			if seen[m] || owned(m) {
				continue
			}
			seen[m] = true
			out = append(out, m)
		}
	}
	return out
}

func reconcileConflictingRyokuFiles(checkOnly bool) recResult {
	// Only a packaged box hits the conflict. A dev checkout has no ryoku-desktop,
	// and deploy.sh's unowned helpers there are correct, so leave them be.
	if !sys.PkgInstalled("ryoku-desktop") {
		return okRes(i18n.T("dev checkout (no ryoku-desktop); deploy-seeded helpers are expected"))
	}
	stray := strayRyokuFiles(ryokuSystemGlobs, filepath.Glob, pkgOwnsFile)
	if len(stray) == 0 {
		return okRes(i18n.T("no unowned Ryoku files blocking pacman"))
	}
	if checkOnly {
		return wouldRes(i18n.T("%d unowned Ryoku file(s) block `pacman -Syu` (deploy-seeded): %s"),
			len(stray), strings.Join(stray, ", ")).
			withFix("sudo rm -f %s", strings.Join(stray, " "))
	}
	for _, f := range stray {
		if err := sys.Sudo("rm", "-f", f); err != nil {
			return failRes(i18n.T("could not remove %s: %v"), f, err).
				withFix("sudo rm -f %s", strings.Join(stray, " "))
		}
	}
	return fixedRes(i18n.T("removed %d unowned Ryoku file(s) so the next update adopts them: %s"),
		len(stray), strings.Join(stray, ", "))
}

// transientAppScope reports whether a --user unit is a transient GUI app-launch
// scope (app-<id>-<n>.scope). systemd creates one per app launch; it lingers
// "failed" after the app exits or is killed, and reset-failed clears the dead
// record. A real .service failure never matches, so it stays reported.
func transientAppScope(unit string) bool {
	return strings.HasPrefix(unit, "app-") && strings.HasSuffix(unit, ".scope")
}

// failedUnits lists failed unit names from `systemctl [extra] --failed`.
func failedUnits(extra ...string) []string {
	args := append(append([]string{}, extra...), "--failed", "--no-legend", "--plain")
	out, _ := sys.RunOut("systemctl", args...)
	var names []string
	for _, l := range nonEmptyLines(out) {
		if f := strings.Fields(l); len(f) > 0 {
			names = append(names, f[0])
		}
	}
	return names
}
