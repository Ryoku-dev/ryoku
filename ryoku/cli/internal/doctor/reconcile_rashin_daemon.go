package doctor

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// ---- reconciler: rashin (the in-system AI) is on by default -----------------
//
// Not enabled and not opted out -> enable at boot (delegated to `ryoku-rashin
// ensure`, which re-checks the opt-out). Already enabled -> keep it healthy
// (daemon-reload, lingering, reset a wedged `failed`). `ryoku-rashin disable`
// is the one-line opt-out and is respected. Idempotent; safe on every update.

const rashinUserUnit = "ryoku-rashin.service"

// rashinUnitState is the subset of systemd state the reconciler decides on,
// split out so the decision is unit-testable without a live user manager.
type rashinUnitState struct {
	enabled bool
	linger  bool
	failed  bool
}

// rashinDaemonActions: what an enabled box needs -- boot-start when lingering is
// off, and clearing a `failed` wedge. Pure, so it is unit-testable.
func rashinDaemonActions(s rashinUnitState) (enableLinger, clearFailed bool) {
	if !s.enabled {
		return false, false
	}
	return !s.linger, s.failed
}

func rashinUnitEnabled() bool {
	out, _ := exec.Command("systemctl", "--user", "is-enabled", rashinUserUnit).Output()
	return strings.TrimSpace(string(out)) == "enabled"
}

func rashinUnitFailed() bool {
	out, _ := exec.Command("systemctl", "--user", "is-failed", rashinUserUnit).Output()
	return strings.TrimSpace(string(out)) == "failed"
}

// rashinLingerOn reads the marker systemd-logind maintains for a lingering user,
// which is readable the same whether doctor runs as the user or under sudo.
func rashinLingerOn(user string) bool {
	if user == "" {
		return false
	}
	_, err := os.Stat("/var/lib/systemd/linger/" + user)
	return err == nil
}

func doctorUser() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return os.Getenv("LOGNAME")
}

// rashinOptedOut reads the flag `ryoku-rashin disable` writes; a missing file
// means "never chose", so the default-on path runs.
func rashinOptedOut() bool {
	cfgHome := os.Getenv("XDG_CONFIG_HOME")
	if cfgHome == "" {
		cfgHome = filepath.Join(os.Getenv("HOME"), ".config")
	}
	b, err := os.ReadFile(filepath.Join(cfgHome, "ryoku", "rashin.json"))
	if err != nil {
		return false
	}
	var c struct {
		OptedOut bool `json:"optedOut"`
	}
	return json.Unmarshal(b, &c) == nil && c.OptedOut
}

const aiUsageTimer = "ryoku-ai-usage.timer"

// the usage-collector timer that feeds the bar AI pill.
func aiUsageTimerKnown() bool {
	out, _ := exec.Command("systemctl", "--user", "list-unit-files", aiUsageTimer, "--no-legend").Output()
	return strings.Contains(string(out), aiUsageTimer)
}
func aiUsageTimerEnabled() bool {
	out, _ := exec.Command("systemctl", "--user", "is-enabled", aiUsageTimer).Output()
	return strings.TrimSpace(string(out)) == "enabled"
}

func reconcileRashinDaemon(checkOnly bool) recResult {
	if !sys.Has("ryoku-rashin") {
		return okRes(i18n.T("ryoku-rashin not installed"))
	}
	if !rashinUnitEnabled() {
		if rashinOptedOut() {
			return okRes(i18n.T("rashin left off by choice (`ryoku-rashin disable`)"))
		}
		if checkOnly {
			return wouldRes(i18n.T("rashin (the Super+S needle and AI dashboard) is off; Ryoku turns it on by default")).
				withFix(i18n.T("ryoku doctor enables it at boot; `ryoku-rashin disable` opts out"))
		}
		if err := exec.Command("ryoku-rashin", "ensure").Run(); err != nil {
			return failRes(i18n.T("could not enable the rashin daemon: %v"), err).
				withFix("ryoku-rashin enable --at-boot")
		}
		return fixedRes(i18n.T("enabled rashin at boot (the Super+S needle and AI dashboard); `ryoku-rashin disable` turns it off"))
	}
	user := doctorUser()
	state := rashinUnitState{enabled: true, linger: rashinLingerOn(user), failed: rashinUnitFailed()}
	enableLinger, clearFailed := rashinDaemonActions(state)
	wireSkill := rashinSkillLinksMissing()
	if !enableLinger && !clearFailed && !wireSkill {
		return okRes(i18n.T("rashin daemon enabled with boot-start; the ryoku skill is wired"))
	}
	if checkOnly {
		switch {
		case clearFailed:
			return wouldRes(i18n.T("the rashin daemon is enabled but wedged off (failed); the dashboard is down")).
				withFix(i18n.T("ryoku doctor reloads the hardened unit and restarts it"))
		case enableLinger:
			return wouldRes(i18n.T("rashin is enabled but only starts at login; a headless boot leaves the dashboard down")).
				withFix(i18n.T("ryoku doctor enables lingering so it starts at boot"))
		default:
			return wouldRes(i18n.T("rashin is enabled but the ryoku agent skill is not wired into every agent")).
				withFix(i18n.T("ryoku doctor runs `ryoku-rashin wire`"))
		}
	}
	var did []string
	if enableLinger || clearFailed {
		// daemon-reload so the just-delivered hardened unit is the one systemd runs.
		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	}
	if enableLinger {
		if user == "" {
			return failRes(i18n.T("cannot enable rashin boot-start: no login user in the environment")).
				withFix("sudo loginctl enable-linger <you>")
		}
		if err := sys.Sudo("loginctl", "enable-linger", user); err != nil {
			return failRes(i18n.T("could not enable lingering for the rashin daemon: %v"), err).
				withFix("sudo loginctl enable-linger " + user)
		}
		did = append(did, i18n.T("enabled boot-start (lingering)"))
	}
	if clearFailed {
		_ = exec.Command("systemctl", "--user", "reset-failed", rashinUserUnit).Run()
		did = append(did, i18n.T("cleared the wedged failed state"))
	}
	if enableLinger || clearFailed {
		_ = exec.Command("systemctl", "--user", "start", rashinUserUnit).Run()
		did = append(did, i18n.T("reloaded the hardened unit"))
	}
	if wireSkill {
		// wire is idempotent and cheap: it drops the ryoku skill symlink into
		// every agent's skills dir and refreshes the vault pointers.
		_ = exec.Command("ryoku-rashin", "wire").Run()
		did = append(did, i18n.T("wired the ryoku agent skill"))
	}
	return fixedRes(i18n.T("converged the rashin daemon: ") + strings.Join(did, " and "))
}

// reconcileAiUsageTimer keeps the bar AI pill fed: the usage-collector timer
// should run whenever the user has not opted out of the AI. Enabling a user
// timer is per-user, so the package cannot do it; doctor (in the session) can.
func reconcileAiUsageTimer(checkOnly bool) recResult {
	if !aiUsageTimerKnown() || rashinOptedOut() {
		return okRes(i18n.T("AI usage collector timer not applicable"))
	}
	if aiUsageTimerEnabled() {
		return okRes(i18n.T("AI usage collector timer enabled"))
	}
	if checkOnly {
		return wouldRes(i18n.T("the AI usage collector timer is off, so the bar AI pill goes stale")).
			withFix(i18n.T("ryoku doctor enables ryoku-ai-usage.timer"))
	}
	if err := exec.Command("systemctl", "--user", "enable", "--now", aiUsageTimer).Run(); err != nil {
		return failRes(i18n.T("could not enable the AI usage collector timer: %v"), err).
			withFix("systemctl --user enable --now " + aiUsageTimer)
	}
	return fixedRes(i18n.T("enabled the AI usage collector timer"))
}

// reconcileProwlAgent surfaces a rashin box that lost the prowl binary.
// ryoku-rashin now depends on prowl (its `index` builds the vault code map
// and its `wire` installs prowl's agent skills), so a box that enabled rashin
// before that dependency shipped can run without it. `pacman -Syu` delivers it
// going forward; this reports the gap for a box still stuck without it. Reported,
// never auto-run: installing a package is the user's call.
func reconcileProwlAgent(checkOnly bool) recResult {
	enabled := rashinUnitEnabled()
	// The CLI was renamed prowl-agent -> prowl; upstream still ships the old
	// binary name during the transition, so accept either one on PATH.
	present := sys.Has("prowl") || sys.Has("prowl-agent")
	if !prowlAgentNeeded(enabled, present) {
		if !enabled {
			return okRes(i18n.T("rashin daemon is opt-in and not enabled"))
		}
		return okRes(i18n.T("prowl is present for the rashin agent index"))
	}
	return warnRes(i18n.T("rashin is enabled but prowl is missing; the vault code index and agent skills will not refresh")).
		withFix("sudo pacman -S prowl-agent")
}

// prowlAgentNeeded reports whether a box should be told to install prowl:
// rashin is enabled but the binary is absent. Split out so the decision is
// unit-testable without a live systemd or PATH.
func prowlAgentNeeded(rashinEnabled, prowlPresent bool) bool {
	return rashinEnabled && !prowlPresent
}

// packagedSkillRoot is where ryoku-desktop ships the skill tree. A var so a
// test can point it at an empty dir: a dev box that is also a packaged install
// has the real tree there, and the probe below would resolve it no matter what
// the test's override says.
var packagedSkillRoot = "/usr/share/ryoku/skills"

// rashinSkillSource resolves the shipped `ryoku` skill dir the same way
// ryoku-rashin wire does: an override, the packaged tree, then a dev checkout.
// Returns "" when the skill is not installed, so a box without it stays quiet.
func rashinSkillSource() string {
	var roots []string
	if v := strings.TrimSpace(os.Getenv("RYOKU_RASHIN_SKILLS")); v != "" {
		roots = append(roots, v)
	}
	roots = append(roots, packagedSkillRoot)
	if repo := sys.ResolveRepo(); repo != "" {
		roots = append(roots, filepath.Join(repo, "ryoku", "rashin", "skills"))
	}
	for _, r := range roots {
		if sys.Exists(filepath.Join(r, "ryoku", "SKILL.md")) {
			return filepath.Join(r, "ryoku")
		}
	}
	return ""
}

// rashinSkillLinksMissing reports whether the skill is installed but an
// always-created link (~/.agents, ~/.hermes) is absent or points elsewhere.
// Cheap: a couple of Lstat calls.
func rashinSkillLinksMissing() bool {
	src := rashinSkillSource()
	if src == "" {
		return false // skill not installed; nothing to wire
	}
	for _, link := range []string{
		filepath.Join(sys.Home(), ".agents", "skills", "ryoku"),
		filepath.Join(sys.Home(), ".hermes", "skills", "ryoku"),
	} {
		if !symlinkPointsAt(link, src) {
			return true
		}
	}
	return false
}
