package doctor

// A shell config that fails to load is a black screen: every Ryoku surface is one
// Quickshell instance, so a single broken QML file takes the whole desktop, at
// login and after an update alike. Nothing looked for it, so the user saw an
// empty screen with no way in.
//
// The loader is not always specific about which file broke (a failed singleton
// often surfaces only as `module "shell.services" is not installed`), so the
// repair does not depend on parsing one out: anything under the failing module
// that does not match the shipped copy is put back, and a user override that
// breaks the desktop is moved aside. What is left is our own bug, and that is
// said plainly, with the two commands that do help.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

var (
	// ERROR: caused by file:///home/u/.config/quickshell/shell/services/Media.qml[76:1]: Syntax error
	qmlFileErrRe = regexp.MustCompile(`caused by file://([^\[\s]+)(?:\[(-?\d+):-?\d+\])?: (.+)$`)
	// ERROR: caused by @shell.qml[13:1]: module "shell.services" is not installed
	qmlModuleErrRe = regexp.MustCompile(`module "([\w.]+)" is not installed`)
	// The plugin '/usr/lib/qt6/qml/Ryoku/Blobs/libryoku-blobsplugin.so' uses incompatible Qt library. (6.12.0) [release]
	qtPluginMismatchRe = regexp.MustCompile(`The plugin '([^']+)' uses incompatible Qt library\. \((\d+\.\d+)[\d.]*\)`)
)

// swapped in tests, which cannot fake /proc
var liveShells = liveShellInstances

func reconcileShellLoad(checkOnly bool) recResult {
	if len(liveShells()) > 0 {
		return okRes(i18n.T("the desktop is loaded"))
	}
	// The updater runs this right after restarting the shell, so give the surface
	// time to come up before calling it dead: a slow start is not a failure.
	report := ""
	deadline := time.Now().Add(8 * time.Second)
	for {
		if len(liveShells()) > 0 {
			return okRes(i18n.T("the desktop is loaded"))
		}
		if r := loggedFailure(); r != "" {
			report = r
			break
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(400 * time.Millisecond)
	}
	if report == "" {
		report = probeConfig()
	}
	if report == "" {
		return okRes(i18n.T("no desktop is running and no load error was reported"))
	}
	// A compiled module built against a newer Qt than the one running is refused
	// by Qt itself: the QML tree is fine, the box is behind its distribution.
	// Only the system update fixes that, and the updater leaves the base system
	// alone on purpose, so say exactly which command does.
	if plugin, built := qtPluginMismatch(report); plugin != "" {
		return warnRes(i18n.T("the desktop cannot load: %s was built against Qt %s but this system runs Qt %s"), plugin, built, runningQt()).
			withFix(i18n.T("sudo pacman -Syu brings Qt up to date, then the shell loads again"))
	}

	// "quickshell/shell/services" when the loader named the module, the whole
	// config tree when it named nothing: either way the base holds the truth.
	scope := failingScope(report)
	blamed := blamedFile(report)

	overrides := brokenOverrides(scope)
	stale := staleManaged(scope)
	if len(overrides) == 0 && len(stale) == 0 {
		return warnRes(i18n.T("the desktop cannot load%s, and every file under %s matches the shipped one"), blamed, scope).
			withFix(i18n.T("ryoku update takes a release with the fix; ryoku rollback returns to the last working snapshot"))
	}

	if checkOnly {
		return wouldRes(i18n.T("the desktop cannot load%s: %s"), blamed, changeSummary(overrides, stale)).
			withFix(i18n.T("ryoku doctor puts the shipped files back and restarts the shell"))
	}
	for _, o := range overrides {
		if err := os.Rename(o, o+".broken"); err != nil {
			return failRes(i18n.T("could not move the override %s aside: %v"), o, err)
		}
	}
	restored, err := restoreShipped(stale)
	if err != nil {
		return failRes(i18n.T("could not restore %s: %v"), scope, err).
			withFix(i18n.T("ryoku materialize, then restart the shell"))
	}
	restartShell()
	return fixedRes(i18n.T("the desktop could not load; %s and restarted the shell"),
		repairSummary(overrides, restored))
}

// loggedFailure is the load failure the shell daemon captured. The log is
// truncated on every start, so a stale failure cannot outlive its attempt.
func loggedFailure() string {
	log := filepath.Join(sys.Xdg("XDG_STATE_HOME", ".local/state"), "ryoku", "surfaces", "shell.log")
	if b, err := os.ReadFile(log); err == nil && isLoadFailure(string(b)) {
		return string(b)
	}
	return ""
}

// probeConfig loads the config once, for a box whose daemon kept no log.
func probeConfig() string {
	// no renderer to ask: the quickshell check owns that failure
	if _, err := exec.LookPath("qs"); err != nil {
		return ""
	}
	out, _ := exec.Command("qs", "-c", "shell").CombinedOutput()
	if isLoadFailure(string(out)) {
		return string(out)
	}
	return ""
}

func isLoadFailure(s string) bool {
	return strings.Contains(s, "Failed to load configuration")
}

// qtPluginMismatch is the plugin Qt refused for being built against a newer Qt
// minor, and that minor, or "" when the report holds no such refusal.
func qtPluginMismatch(report string) (plugin, built string) {
	m := qtPluginMismatchRe.FindStringSubmatch(report)
	if m == nil {
		return "", ""
	}
	return m[1], m[2]
}

// runningQt is the installed qt6-base version, or "an older one" when pacman
// cannot say.
func runningQt() string {
	out, err := sys.RunOut("pacman", "-Q", "qt6-base")
	if f := strings.Fields(out); err == nil && len(f) == 2 {
		return strings.SplitN(f[1], "-", 2)[0]
	}
	return i18n.T("an older one")
}

// failingScope is the config-relative directory to repair.
func failingScope(report string) string {
	const tree = "quickshell"
	if m := qmlModuleErrRe.FindStringSubmatch(report); m != nil {
		return filepath.Join(tree, filepath.Join(strings.Split(m[1], ".")...))
	}
	if file, _, _ := parseQmlError(report); file != "" {
		if rel := shellConfigRel(filepath.Dir(file)); rel != "" {
			return rel
		}
	}
	return tree
}

// blamedFile is the file and line the loader named, when it named one; it makes
// the message specific but nothing depends on it.
func blamedFile(report string) string {
	file, line, reason := parseQmlError(report)
	if file == "" {
		if m := qmlModuleErrRe.FindStringSubmatch(report); m != nil {
			return fmt.Sprintf(i18n.T(" (%s did not load)"), m[1])
		}
		return ""
	}
	if line != "" {
		return fmt.Sprintf(i18n.T(" (%s line %s: %s)"), file, line, reason)
	}
	return fmt.Sprintf(" (%s: %s)", file, reason)
}

func parseQmlError(report string) (file, line, reason string) {
	// the last cause is the file that broke; the ones above it are the imports
	// that could not resolve because of it.
	for _, l := range strings.Split(report, "\n") {
		if m := qmlFileErrRe.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
			file, line, reason = m[1], m[2], strings.TrimSpace(m[3])
			if line == "-1" {
				line = ""
			}
		}
	}
	return file, line, reason
}

// brokenOverrides are the user's own files under the failing scope. They win over
// the shipped tree, so one of them can hold the desktop down on its own.
func brokenOverrides(scope string) []string {
	var out []string
	root := filepath.Join(sys.UserEditsDir(), scope)
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		out = append(out, p)
		return nil
	})
	return out
}

// staleManaged are shipped files the live tree no longer matches: an old release
// left behind, or a hand edit made in place.
func staleManaged(scope string) []string {
	base, cfg := sys.BaseConfigDir(), sys.ConfigHome()
	var out []string
	_ = filepath.WalkDir(filepath.Join(base, scope), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(base, p)
		if relErr != nil {
			return nil
		}
		want, readErr := os.ReadFile(p)
		if readErr != nil {
			return nil
		}
		if have, err := os.ReadFile(filepath.Join(cfg, rel)); err != nil || string(have) != string(want) {
			out = append(out, rel)
		}
		return nil
	})
	return out
}

func restoreShipped(rels []string) ([]string, error) {
	base, cfg := sys.BaseConfigDir(), sys.ConfigHome()
	var done []string
	for _, rel := range rels {
		dst := filepath.Join(cfg, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return done, err
		}
		if err := sys.CopyFile(filepath.Join(base, rel), dst); err != nil {
			return done, err
		}
		done = append(done, rel)
	}
	return done, nil
}

// shellConfigRel turns an absolute path into its path under ~/.config, or "" when
// it lives elsewhere (a checkout, a system module).
func shellConfigRel(path string) string {
	rel, err := filepath.Rel(sys.ConfigHome(), path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return ""
	}
	return rel
}

func changeSummary(overrides, stale []string) string {
	var parts []string
	if n := len(overrides); n > 0 {
		parts = append(parts, fmt.Sprintf(i18n.T("%s of yours %s the shipped desktop"), plural(n, i18n.T("file")), pick(n, i18n.T("overrides"), i18n.T("override"))))
	}
	if n := len(stale); n > 0 {
		parts = append(parts, fmt.Sprintf(i18n.T("%s %s the shipped desktop"), plural(n, i18n.T("file")), pick(n, i18n.T("does not match"), i18n.T("do not match"))))
	}
	return strings.Join(parts, " and ")
}

func repairSummary(overrides, restored []string) string {
	var parts []string
	if n := len(overrides); n > 0 {
		parts = append(parts, fmt.Sprintf(i18n.T("moved %s of yours aside (kept as .broken)"), plural(n, i18n.T("override"))))
	}
	if n := len(restored); n > 0 {
		parts = append(parts, fmt.Sprintf(i18n.T("put %s back"), plural(n, i18n.T("shipped file"))))
	}
	if len(parts) == 0 {
		return i18n.T("found nothing to put back")
	}
	return strings.Join(parts, ", ")
}

func pick(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// restartShell saves the user knowing the command; a failure here is not fatal,
// the repair still stands. A var so a test never restarts the desktop it runs on.
var restartShell = func() {
	_ = exec.Command("systemctl", "--user", "restart", "ryoku-shell").Run()
}
