package doctor

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// ---- reconciler: the in-session lockscreen -----------------------------------
// `ryoku-shell lock` enters the stable ryoku-qylock-lock launcher, which selects
// ~/.local/share/quickshell-lockscreen/lock.sh. Only the ISO installer used to
// lay that tree down. A box that predates the step, or where it failed, has a
// dead lock button and, worse, cannot complete the shell daemon's
// compositor-secure lock-before-suspend handshake. The bundle now
// ships in ryoku-desktop (and lives in a checkout), so this can heal it anywhere.
//
// It also converges an INSTALLED bundle onto the shipped one. The copy under
// ~/.local/share is outside materialize's tree, so before this a lock fix (a
// reveal, a cursor default, a shim change) reached only fresh installs; every
// existing box kept the lock it was installed with. Only the shipped default
// skin (clockwork/orbital) is ever refreshed; a skin the user picked from the
// Store is never touched.

const defaultLockSkin = "clockwork/orbital"

// lockBundle finds the shipped qylock bundle: the package payload first, the
// checkout on a dev box.
func lockBundle() string {
	if p := "/usr/share/ryoku/lockscreen/qylock"; sys.Exists(p) {
		return p
	}
	if repo := sys.ResolveRepo(); repo != "" {
		if p := filepath.Join(repo, "ryoku", "lockscreen", "qylock"); sys.Exists(p) {
			return p
		}
	}
	return ""
}

// treeDiffers reports whether any regular file under src is missing at, or
// differs from, the same relative path under dst. Extra files under dst (the
// themes_link symlink, a user's additions) do not count.
func treeDiffers(src, dst string) bool {
	differs := false
	_ = filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil || differs || !d.Type().IsRegular() {
			return nil
		}
		rel, _ := filepath.Rel(src, p)
		want, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		got, err := os.ReadFile(filepath.Join(dst, rel))
		if err != nil || !bytes.Equal(want, got) {
			differs = true
		}
		return nil
	})
	return differs
}

// lockscreenStale: the installed in-session lock (its runner and the default
// skin) no longer matches what the package ships.
func lockscreenStale(bundle string) bool {
	home := os.Getenv("HOME")
	return treeDiffers(filepath.Join(bundle, "quickshell-lockscreen"), filepath.Join(home, ".local", "share", "quickshell-lockscreen")) ||
		treeDiffers(filepath.Join(bundle, "themes", defaultLockSkin), filepath.Join(home, ".local", "share", "qylock", "themes", defaultLockSkin))
}

func greeterClock24h() bool {
	b, err := os.ReadFile(filepath.Join(sys.ConfigHome(), "ryoku", "shell.json"))
	if err != nil {
		return false
	}
	var settings struct {
		General struct {
			ClockFormat24H bool `json:"clock_format_24_h"`
		} `json:"general"`
	}
	return json.Unmarshal(b, &settings) == nil && settings.General.ClockFormat24H
}

func installedGreeterClock24h(path string) (bool, bool) {
	for _, line := range strings.Split(readFileSafe(path), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok && strings.TrimSpace(key) == "clock24h" {
			clock24h, err := strconv.ParseBool(strings.TrimSpace(value))
			return clock24h, err == nil
		}
	}
	return false, false
}

// greeterStale checks the shared SDDM host for every skin. A selected optional
// skin remains untouched beneath theme/; only the host is compared. The shipped
// default is also compared with the bundle so package fixes reach it.
func greeterStale(bundle string) bool {
	pref := strings.TrimSpace(readFileSafe(filepath.Join(sys.ConfigHome(), "qylock", "theme")))
	if !sys.Exists(filepath.Join(greeterThemeDir, "Main.qml")) {
		return false
	}
	host := filepath.Join(bundle, "sddm-host")
	installedClock24h, hasClockFormat := installedGreeterClock24h(filepath.Join(greeterThemeDir, "theme.conf"))
	if fileDiffers(filepath.Join(host, "Main.qml"), filepath.Join(greeterThemeDir, "Main.qml")) ||
		fileDiffers(filepath.Join(host, "metadata.desktop"), filepath.Join(greeterThemeDir, "metadata.desktop")) ||
		!hasClockFormat || installedClock24h != greeterClock24h() {
		return true
	}
	if pref != "" && pref != defaultLockSkin {
		return false
	}
	return treeDiffers(filepath.Join(bundle, "themes", defaultLockSkin), filepath.Join(greeterThemeDir, "theme"))
}

// refreshGreeter lays the selected skin beneath the shared SDDM host. An old
// optional skin installed directly at the greeter root is used as its own
// migration source; the default is refreshed from the shipped bundle.
func refreshGreeter(bundle string) error {
	pref := strings.TrimSpace(readFileSafe(filepath.Join(sys.ConfigHome(), "qylock", "theme")))
	src := filepath.Join(bundle, "themes", defaultLockSkin)
	if pref != "" && pref != defaultLockSkin {
		wrapped := filepath.Join(greeterThemeDir, "theme")
		if sys.Exists(filepath.Join(wrapped, "Main.qml")) {
			src = wrapped
		} else {
			src = greeterThemeDir
		}
	}
	installer := filepath.Join(bundle, "sddm-host", "install-greeter-theme")
	return sys.Run("sudo", installer, src, greeterThemeDir, strconv.FormatBool(greeterClock24h()))
}

// greeterScriptSource is the shipped greeter compositor script. On a package box
// the ryoku-desktop package owns and updates /usr/share/ryoku/lockscreen/ryoku-greeter
// itself, so there is nothing to reconcile; a dev checkout, whose deploy flow
// never lays the script down, is the only place doctor must keep it current.
// Return the checkout copy when a repo resolves, else "" to skip.
func greeterScriptSource() string {
	if repo := sys.ResolveRepo(); repo != "" {
		if p := filepath.Join(repo, "ryoku", "lockscreen", "sddm", "ryoku-greeter"); sys.Exists(p) {
			return p
		}
	}
	return ""
}

// greeterScriptStale reports whether the installed greeter compositor script is
// missing or drifted from the checkout copy. A stale script strands greeter
// fixes -- the NVIDIA software-cursor renderer among them -- on a box that only
// ever ran an older deploy.
func greeterScriptStale(src string) bool {
	return src != "" && fileDiffers(src, greeterCompositorBin)
}

// fileDiffers reports whether dst is missing or differs byte-for-byte from src.
func fileDiffers(src, dst string) bool {
	a, err := os.ReadFile(src)
	if err != nil {
		return false
	}
	b, err := os.ReadFile(dst)
	if err != nil {
		return true
	}
	return !bytes.Equal(a, b)
}

// refreshGreeterScript installs the shipped greeter compositor script over the
// stale one, executable and root-owned like the package lays it down.
func refreshGreeterScript(src string) error {
	return sys.Run("sudo", "install", "-Dm755", src, greeterCompositorBin)
}

func lockerPath() string {
	return filepath.Join(os.Getenv("HOME"), ".local", "share", "quickshell-lockscreen", "lock.sh")
}

func stagedLockerPath() string {
	return filepath.Join(os.Getenv("HOME"), ".local", "share", "ryoku", "qylock-next", "lockscreen", "lock.sh")
}

func stageLockscreen(installer string) ([]byte, error) {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = os.TempDir()
	}
	cutoverLock, err := os.OpenFile(
		filepath.Join(runtimeDir, "ryoku-power-cutover.lock"),
		os.O_CREATE|os.O_RDWR, 0o600,
	)
	if err != nil {
		return nil, err
	}
	defer cutoverLock.Close()
	if err := syscall.Flock(int(cutoverLock.Fd()), syscall.LOCK_EX); err != nil {
		return nil, err
	}

	guarded := false
	helper, helperErr := exec.LookPath("ryoku-power-cutover")
	if helperErr == nil {
		if out, err := exec.Command(helper, "generation-guard-start").CombinedOutput(); err != nil {
			return out, err
		}
		guarded = true
		defer exec.Command(helper, "generation-guard-stop").Run()
	}

	cmd := exec.Command(installer)
	cmd.Env = append(os.Environ(),
		"RYOKU_QYLOCK_USER_ONLY=1",
		"RYOKU_QYLOCK_MODE=stage",
	)
	if guarded {
		cmd.Env = append(cmd.Env, "RYOKU_QYLOCK_GENERATION_GUARDED=1")
	}
	return cmd.CombinedOutput()
}

var legacyTapeHashes = map[string]string{
	"Main.qml":              "106fee628bb634e2b7ee87a4851532a42cbe635ae745d385fc5296e9098dc015",
	"font/Outfit-Black.ttf": "f240e6128c31a75aa3f456ea1ff3b0fda382176681788ae3d244d08e3fa7d6cd",
	"metadata.desktop":      "37615671bab45ab45979cc9938c0bb8892f58760bd865a5286f58c342dacdf97",
	"theme.conf":            "002c24b024b3e0788052f178acd7c6cec25f08e2486b4bfb123c7c8b1b8a4475",
	"preview.gif":           "93237dfb00b51b9fcd0bc153e076a03a7315b490052f16756d9fd020057b1a02",
}

func legacyTapeNeedsMigration() bool {
	themeRoot := filepath.Join(os.Getenv("HOME"), ".local", "share", "qylock", "themes")
	root := filepath.Join(themeRoot, "clockwork", "tape")
	if sys.Exists(filepath.Join(themeRoot, "clockwork-tape")) {
		return false
	}
	count := 0
	valid := true
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			valid = false
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			valid = false
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		want, known := legacyTapeHashes[rel]
		body, readErr := os.ReadFile(path)
		if relErr != nil || !known || readErr != nil {
			valid = false
			return nil
		}
		wantBytes, decodeErr := hex.DecodeString(want)
		sum := sha256.Sum256(body)
		if decodeErr != nil || !bytes.Equal(sum[:], wantBytes) {
			valid = false
		}
		count++
		return nil
	})
	return err == nil && valid && count == len(legacyTapeHashes)
}

func needsLockscreenInstaller(lockerPresent, legacyTape bool) bool {
	return !lockerPresent || legacyTape
}

// lockscreenInstaller finds the shipped installer: the package payload first,
// the checkout on a dev box.
func lockscreenInstaller() string {
	if p := "/usr/share/ryoku/lockscreen/install-qylock"; sys.Exists(p) {
		return p
	}
	if repo := sys.ResolveRepo(); repo != "" {
		if p := filepath.Join(repo, "ryoku", "lockscreen", "install-qylock"); sys.Exists(p) {
			return p
		}
	}
	return ""
}

func reconcileLockscreen(checkOnly bool) recResult {
	lockerPresent := sys.Exists(lockerPath())
	legacyTape := legacyTapeNeedsMigration()
	if !needsLockscreenInstaller(lockerPresent, legacyTape) {
		return reconcileLockscreenDrift(checkOnly)
	}
	installer := lockscreenInstaller()
	if installer == "" {
		if legacyTape {
			return warnRes(i18n.T("the legacy Tape lockscreen needs migration and no lockscreen bundle is available")).
				withFix(i18n.T("ryoku update (ships the lockscreen bundle), then ryoku doctor"))
		}
		return warnRes(i18n.T("the in-session lockscreen is missing and no bundle is available to install it; the lock button and lock-on-sleep do nothing")).
			withFix(i18n.T("ryoku update (ships the lockscreen bundle), then ryoku doctor"))
	}
	if checkOnly {
		if legacyTape {
			return wouldRes(i18n.T("the legacy Tape lockscreen needs one-time Store migration")).
				withFix("ryoku doctor")
		}
		return wouldRes(i18n.T("the in-session lockscreen is missing; the lock button and lock-on-sleep do nothing")).
			withFix("ryoku doctor")
	}
	// A doctor run must not replace the lock client underneath a live daemon:
	// the two share the proof/unlock protocol. Stage the whole generation for
	// ryoku-shell.service to promote before its next matching daemon start.
	out, err := stageLockscreen(installer)
	if err != nil {
		return failRes(i18n.T("lockscreen install failed: %v (%s)"), err, firstLine(string(out))).
			withFix(i18n.T("run %s by hand to see why"), installer)
	}
	if !sys.Exists(stagedLockerPath()) {
		return failRes(i18n.T("lockscreen installer ran but %s did not appear"), stagedLockerPath())
	}
	if legacyTape {
		return fixedRes(i18n.T("staged the legacy Tape migration for the next shell daemon start"))
	}
	if lockerPresent {
		return okRes(i18n.T("in-session lockscreen staged; custom legacy Tape retained"))
	}
	return fixedRes(i18n.T("staged the in-session lockscreen for the next shell daemon start"))
}

// reconcileLockscreenDrift refreshes an installed lock onto the shipped bundle
// (see the header). The user half re-runs the installer without the greeter
// step; the greeter half copies the skin as root, only when it is the shipped
// default and only when a sudo is available without a prompt (an update has
// one cached; a plain doctor run may not, and reports instead).
func reconcileLockscreenDrift(checkOnly bool) recResult {
	bundle := lockBundle()
	if bundle == "" {
		return okRes(i18n.T("in-session lockscreen installed"))
	}
	lockStale := lockscreenStale(bundle)
	greeter := greeterStale(bundle)
	gscriptSrc := greeterScriptSource()
	gscriptStale := greeterScriptStale(gscriptSrc)
	if !lockStale && !greeter && !gscriptStale {
		return okRes(i18n.T("in-session lockscreen installed and current"))
	}
	if checkOnly {
		return wouldRes(i18n.T("the installed lockscreen predates the shipped one; lock fixes have not reached this box")).
			withFix("ryoku doctor")
	}
	var did []string
	if lockStale {
		installer := lockscreenInstaller()
		if installer == "" {
			return warnRes(i18n.T("the installed lockscreen predates the shipped one and no installer is available")).
				withFix("ryoku update")
		}
		out, err := stageLockscreen(installer)
		if err != nil {
			return failRes(i18n.T("lockscreen refresh failed: %v (%s)"), err, firstLine(string(out))).
				withFix(i18n.T("run %s by hand to see why"), installer)
		}
		did = append(did, i18n.T("in-session lock (next daemon start)"))
	}
	if greeter || gscriptStale {
		if exec.Command("sudo", "-n", "true").Run() != nil {
			return noteRes(i18n.T("the SDDM greeter predates the shipped bundle; refreshing it needs sudo")).
				withFix(i18n.T("ryoku doctor (or the next ryoku update)"))
		}
		if greeter {
			if err := refreshGreeter(bundle); err != nil {
				return failRes(i18n.T("could not refresh the SDDM greeter skin: %v"), err).
					withFix("sudo " + lockscreenInstaller())
			}
			did = append(did, i18n.T("SDDM greeter"))
		}
		if gscriptStale {
			if err := refreshGreeterScript(gscriptSrc); err != nil {
				return failRes(i18n.T("could not refresh the SDDM greeter compositor: %v"), err).
					withFix(i18n.T("ryoku doctor"))
			}
			did = append(did, i18n.T("greeter compositor"))
		}
	}
	return fixedRes(i18n.T("refreshed the %s to the shipped lockscreen bundle"), strings.Join(did, " and "))
}
