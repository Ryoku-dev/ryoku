package doctor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// ---- reconciler: rashin (the in-system AI) is on by default -----------------
//
// The service supervisor enables Rashin unless the user opted out. On systemd,
// ryoku-rashin ensure also maintains login lingering; on runit the Turnstile
// session roster owns startup. Both paths restart an enabled service that has
// wedged off and keep the agent skill wiring current.

const rashinUserUnit = "ryoku-rashin.service"

const prowlUserUnit = "ryoku-prowl.service"

var (
	rashinInit = func() (host.InitSystem, error) {
		return host.Default().Init()
	}
	rashinService = func(args []string) int {
		return host.Default().Service(args)
	}
)

// rashinUnitState is the subset of session supervisor state the reconciler
// decides on, split out so the decision is unit-testable.
type rashinUnitState struct {
	enabled bool
	active  bool
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
	return rashinService([]string{"--user", "is-enabled", rashinUserUnit}) == host.ExitOK
}

func rashinUnitFailed() bool {
	initSystem, err := rashinInit()
	if err != nil || initSystem != host.Systemd {
		return false
	}
	out, _ := exec.Command("systemctl", "--user", "is-failed", rashinUserUnit).Output()
	return strings.TrimSpace(string(out)) == "failed"
}

func rashinUnitActive() bool {
	return rashinService([]string{"--user", "is-active", rashinUserUnit}) == host.ExitOK
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

const (
	aiUsageTimer   = "ryoku-ai-usage.timer"
	aiUsageService = "ryoku-ai-usage.service"
)

func aiUsageUnit(initSystem host.InitSystem) string {
	if initSystem == host.Runit {
		return aiUsageService
	}
	return aiUsageTimer
}

func aiUsageUnitState(unit string) int {
	return rashinService([]string{"--user", "is-enabled", unit})
}

func reconcileRashinDaemon(checkOnly bool) recResult {
	if !sys.Has("ryoku-rashin") {
		return okRes(i18n.T("ryoku-rashin not installed"))
	}
	initSystem, err := rashinInit()
	if err != nil {
		return noteRes(i18n.T("could not identify the session supervisor; the rashin daemon was not changed"))
	}
	if !rashinUnitEnabled() {
		if rashinOptedOut() {
			return okRes(i18n.T("rashin left off by choice (`ryoku-rashin disable`)"))
		}
		if checkOnly {
			result := wouldRes(i18n.T("rashin (the Super+S needle and AI dashboard) is off; Ryoku turns it on by default"))
			if initSystem == host.Runit {
				return result.withFix(i18n.T("ryoku doctor enables it in the runit session roster; `ryoku-rashin disable` opts out"))
			}
			return result.withFix(i18n.T("ryoku doctor enables it at boot; `ryoku-rashin disable` opts out"))
		}
		if initSystem == host.Systemd {
			if err := exec.Command("ryoku-rashin", "ensure").Run(); err != nil {
				return failRes(i18n.T("could not enable the rashin daemon: %v"), err).
					withFix("ryoku-rashin enable --at-boot")
			}
			return fixedRes(i18n.T("enabled rashin at boot (the Super+S needle and AI dashboard); `ryoku-rashin disable` turns it off"))
		}
		if code := rashinService([]string{"--user", "enable", "--now", rashinUserUnit}); code != host.ExitOK {
			return failRes(i18n.T("could not enable the rashin daemon (service exit %d)"), code).
				withFix("ryoku-host svc --user enable --now " + rashinUserUnit)
		}
		return fixedRes(i18n.T("enabled rashin for this and future sessions; `ryoku-rashin disable` turns it off"))
	}
	user := doctorUser()
	state := rashinUnitState{
		enabled: true,
		active:  rashinUnitActive(),
		linger:  initSystem != host.Systemd || rashinLingerOn(user),
		failed:  rashinUnitFailed(),
	}
	enableLinger, clearFailed := rashinDaemonActions(state)
	start := initSystem == host.Runit && !state.active
	wireSkill := rashinSkillLinksMissing()
	if !enableLinger && !clearFailed && !start && !wireSkill {
		if initSystem == host.Runit {
			return okRes(i18n.T("rashin daemon enabled and supervised in the runit session roster; the ryoku skill is wired"))
		}
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
		case start:
			return wouldRes(i18n.T("the rashin service is enabled but down in this runit session")).
				withFix(i18n.T("ryoku doctor starts ryoku-rashin under the session supervisor"))
		default:
			return wouldRes(i18n.T("rashin is enabled but the ryoku agent skill is not wired into every agent")).
				withFix(i18n.T("ryoku doctor runs `ryoku-rashin wire`"))
		}
	}
	var did []string
	if enableLinger || clearFailed || start {
		_ = rashinService([]string{"--user", "daemon-reload"})
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
		_ = rashinService([]string{"--user", "reset-failed", rashinUserUnit})
		did = append(did, i18n.T("cleared the wedged failed state"))
	}
	if enableLinger || clearFailed || start {
		if code := rashinService([]string{"--user", "start", rashinUserUnit}); code != host.ExitOK {
			return failRes(i18n.T("could not start the rashin daemon (service exit %d)"), code).
				withFix("ryoku-host svc --user start " + rashinUserUnit)
		}
		did = append(did, i18n.T("reloaded the hardened unit"))
	}
	if wireSkill {
		_ = exec.Command("ryoku-rashin", "wire").Run()
		did = append(did, i18n.T("wired the ryoku agent skill"))
	}
	return fixedRes(i18n.T("converged the rashin daemon: ") + strings.Join(did, " and "))
}

// reconcileAiUsageTimer keeps the bar AI pill fed. systemd schedules the
// collector with a timer; runit supervises the equivalent loop service.
func reconcileAiUsageTimer(checkOnly bool) recResult {
	initSystem, err := rashinInit()
	if err != nil {
		return noteRes(i18n.T("could not identify the session supervisor; the AI usage collector was not checked"))
	}
	unit := aiUsageUnit(initSystem)
	state := aiUsageUnitState(unit)
	if state == host.ExitAbsent || rashinOptedOut() {
		if initSystem == host.Runit {
			return okRes(i18n.T("AI usage collector service not applicable"))
		}
		return okRes(i18n.T("AI usage collector timer not applicable"))
	}
	if state == host.ExitOK {
		if initSystem == host.Systemd {
			return okRes(i18n.T("AI usage collector timer enabled"))
		}
		return okRes(i18n.T("AI usage collector service enabled"))
	}
	if checkOnly {
		if initSystem == host.Systemd {
			return wouldRes(i18n.T("the AI usage collector timer is off, so the bar AI pill goes stale")).
				withFix(i18n.T("ryoku doctor enables ryoku-ai-usage.timer"))
		}
		return wouldRes(i18n.T("the AI usage collector service is off, so the bar AI pill goes stale")).
			withFix(i18n.T("ryoku doctor enables ryoku-ai-usage in the runit session roster"))
	}
	if code := rashinService([]string{"--user", "enable", "--now", unit}); code != host.ExitOK {
		return failRes(i18n.T("could not enable the AI usage collector (service exit %d)"), code).
			withFix("ryoku-host svc --user enable --now " + unit)
	}
	if initSystem == host.Systemd {
		return fixedRes(i18n.T("enabled the AI usage collector timer"))
	}
	return fixedRes(i18n.T("enabled the AI usage collector service"))
}

// reconcileProwl surfaces a rashin box that lost the prowl binary.
// ryoku-rashin depends on prowl for its vault code map and agent skills, so a
// box that enabled rashin before that dependency shipped can run without it.
// Package installation remains the user's call.
func reconcileProwl(_ bool) recResult {
	enabled := rashinUnitEnabled()
	present := sys.Has("prowl")
	if !prowlNeeded(enabled, present) {
		if !enabled {
			return okRes(i18n.T("rashin daemon is opt-in and not enabled"))
		}
		return okRes(i18n.T("prowl is present for the rashin agent index"))
	}
	return warnRes(i18n.T("rashin is enabled but prowl is missing; the vault code index and agent skills will not refresh")).
		withFix(doctorInstallAdvice("prowl"))
}

// prowlNeeded reports whether a box should be told to install prowl.
func prowlNeeded(rashinEnabled, prowlPresent bool) bool {
	return rashinEnabled && !prowlPresent
}

const defaultProwlGatewayPort = 8788

var prowlGatewayClient = &http.Client{Timeout: 2 * time.Second}

func prowlGatewayPort() int {
	port, err := strconv.Atoi(strings.TrimSpace(os.Getenv("RYOKU_PROWL_PORT")))
	if err != nil || port < 1 || port > 65535 {
		return defaultProwlGatewayPort
	}
	return port
}

func prowlGatewayAnswers() bool {
	url := fmt.Sprintf("http://127.0.0.1:%d/api/ping", prowlGatewayPort())
	resp, err := prowlGatewayClient.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var answer struct {
		Status string `json:"status"`
	}
	return json.NewDecoder(resp.Body).Decode(&answer) == nil && answer.Status == "ok"
}

// prowlGatewayNeeded is the pure gateway health decision.
func prowlGatewayNeeded(state rashinUnitState, prowlPresent, gatewayAnswering bool) bool {
	return state.enabled && state.active && prowlPresent && !gatewayAnswering
}

func reconcileProwlGateway(checkOnly bool) recResult {
	state := rashinUnitState{
		enabled: rashinUnitEnabled(),
		active:  rashinUnitActive(),
	}
	present := sys.Has("prowl")
	answering := false
	if state.enabled && state.active && present {
		answering = prowlGatewayAnswers()
	}
	if !prowlGatewayNeeded(state, present, answering) {
		switch {
		case !state.enabled:
			return okRes(i18n.T("rashin daemon is not enabled; Prowl's gateway is not expected"))
		case !state.active:
			return okRes(i18n.T("rashin daemon is not active; Prowl's gateway is not expected"))
		case !present:
			return okRes(i18n.T("prowl is not installed; gateway health is not applicable"))
		default:
			return okRes(i18n.T("Prowl's gateway answers for rashin"))
		}
	}
	initSystem, initErr := rashinInit()
	if checkOnly {
		if initErr == nil && initSystem == host.Runit {
			return wouldRes(i18n.T("rashin is running but Prowl's gateway does not answer")).
				withFix(i18n.T("ryoku doctor starts the ryoku-prowl runit service"))
		}
		return wouldRes(i18n.T("rashin is running but Prowl's gateway does not answer")).
			withFix(i18n.T("ryoku doctor runs `ryoku-rashin ensure`"))
	}
	if initErr == nil && initSystem == host.Runit {
		if code := rashinService([]string{"--user", "start", prowlUserUnit}); code != host.ExitOK {
			return failRes(i18n.T("rashin is running but Prowl's gateway does not answer; the ryoku-prowl service could not start (exit %d)"), code).
				withFix("ryoku-host svc --user start " + prowlUserUnit)
		}
	} else if err := exec.Command("ryoku-rashin", "ensure").Run(); err != nil {
		return failRes(i18n.T("rashin is running but Prowl's gateway does not answer; `ryoku-rashin ensure` failed: %v"), err).
			withFix("ryoku-rashin ensure")
	}
	if !prowlGatewayAnswers() {
		if initErr == nil && initSystem == host.Runit {
			return failRes(i18n.T("rashin is running but Prowl's gateway still does not answer after starting ryoku-prowl")).
				withFix("ryoku-host svc --user restart " + prowlUserUnit)
		}
		return failRes(i18n.T("rashin is running but Prowl's gateway still does not answer after `ryoku-rashin ensure`")).
			withFix("ryoku-rashin ensure")
	}
	return fixedRes(i18n.T("restored Prowl's gateway for rashin"))
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
