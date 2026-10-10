package doctor

import (
	"os"
	"os/exec"
	"strings"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// ---- reconciler: cut existing boxes over from the old awww daemon to Ryogami -
// The wallpaper backend moved from awww (a swww fork the shell drove by name) to
// Ryogami, the in-repo daemon the shell now drives over ryogami.sock. Updating
// the package alone can leave an existing box in a broken middle: the Ryogami
// service is delivered but disabled, while a stale awww-daemon keeps a surface
// mapped over it. The host service seam reloads and enables the systemd unit on
// Arch or the supervised runit service on Void, clears a systemd failed state
// when one exists, and stops the leftover daemon. Idempotent; retired once every
// box has cut over.

const ryogamiUserUnit = "ryogami.service"

var (
	ryogamiInit = func() (host.InitSystem, error) {
		return host.Default().Init()
	}
	ryogamiService = func(args []string) int {
		return host.Default().Service(args)
	}
)

// ryogamiWallpaperState is the subset of session supervisor state the
// reconciler decides on, split out so the decision is unit-testable.
type ryogamiWallpaperState struct {
	enabled     bool
	active      bool
	failed      bool
	awwwRunning bool
	inSession   bool // doctor is running inside the graphical session
}

// ryogamiWallpaperActions decides what a box needs: enable the delivered unit
// when it is not, clear a wedged failed state, start a daemon that is down, and
// stop a leftover awww-daemon so it stops stacking over Ryogami's surface.
//
// `start` is the case an enabled-only check cannot see, and the one that leaves
// a user staring at a black desktop while doctor reports everything fine: the
// unit carries ConditionEnvironment=WAYLAND_DISPLAY, so a login that lost the
// race against the environment import has systemd skip it. A skipped unit is
// inactive, exit 0, and NOT failed, so is-enabled and is-failed both look clean.
// Only judged inside the graphical session: from a TTY, or during the update's
// quiesced stage, inactive is correct and the condition would refuse a start
// anyway.
func ryogamiWallpaperActions(s ryogamiWallpaperState) (enable, clearFailed, start, stopAwww bool) {
	return !s.enabled, s.failed, s.inSession && !s.active, s.awwwRunning
}

func ryogamiUnitEnabled() bool {
	return ryogamiService([]string{"--user", "is-enabled", ryogamiUserUnit}) == host.ExitOK
}

func ryogamiUnitActive() bool {
	return ryogamiService([]string{"--user", "is-active", ryogamiUserUnit}) == host.ExitOK
}

func ryogamiUnitFailed() bool {
	initSystem, err := ryogamiInit()
	if err != nil || initSystem != host.Systemd {
		return false
	}
	out, _ := exec.Command("systemctl", "--user", "is-failed", ryogamiUserUnit).Output()
	return strings.TrimSpace(string(out)) == "failed"
}

// inGraphicalSession: the same signal the unit's own condition tests, so doctor
// judges the daemon exactly when systemd would agree to run it.
func inGraphicalSession() bool {
	return os.Getenv("WAYLAND_DISPLAY") != ""
}

var awwwDaemonRunning = func() bool {
	return exec.Command("pgrep", "-x", "awww-daemon").Run() == nil
}

func reconcileRyogamiWallpaper(checkOnly bool) recResult {
	if !sys.Has("ryogami") {
		return okRes(i18n.T("ryogami not installed yet (arrives with the ryoku-desktop update)"))
	}
	state := ryogamiWallpaperState{
		enabled:     ryogamiUnitEnabled(),
		active:      ryogamiUnitActive(),
		failed:      ryogamiUnitFailed(),
		awwwRunning: awwwDaemonRunning(),
		inSession:   inGraphicalSession(),
	}
	enable, clearFailed, start, stopAwww := ryogamiWallpaperActions(state)
	if !enable && !clearFailed && !start && !stopAwww {
		return okRes(i18n.T("ryogami wallpaper daemon enabled and running"))
	}
	if checkOnly {
		switch {
		case stopAwww:
			return wouldRes(i18n.T("the retired awww wallpaper daemon is still running and stacks over Ryogami")).
				withFix(i18n.T("ryoku doctor stops awww-daemon and enables the ryogami unit"))
		case clearFailed:
			return wouldRes(i18n.T("the ryogami wallpaper daemon is wedged off (failed); the wallpaper is down")).
				withFix(i18n.T("ryoku doctor reloads and restarts the ryogami unit"))
		case enable:
			return wouldRes(i18n.T("the ryogami wallpaper daemon is delivered but not enabled")).
				withFix(i18n.T("ryoku doctor enables the ryogami unit so the session starts it"))
		default:
			return wouldRes(i18n.T("the ryogami wallpaper daemon is down in this session; the desktop has no wallpaper")).
				withFix(i18n.T("ryoku doctor starts the ryogami unit"))
		}
	}
	_ = ryogamiService([]string{"--user", "daemon-reload"})
	var did []string
	if enable {
		if code := ryogamiService([]string{"--user", "enable", ryogamiUserUnit}); code != host.ExitOK {
			return failRes(i18n.T("could not enable the ryogami wallpaper daemon (service exit %d)"), code).
				withFix("ryoku-host svc --user enable " + ryogamiUserUnit)
		}
		did = append(did, i18n.T("enabled the ryogami unit"))
	}
	if clearFailed {
		_ = ryogamiService([]string{"--user", "reset-failed", ryogamiUserUnit})
		did = append(did, i18n.T("cleared the wedged failed state"))
	}
	if stopAwww {
		_ = exec.Command("pkill", "-x", "awww-daemon").Run()
		did = append(did, i18n.T("stopped the retired awww-daemon"))
	}
	if start {
		did = append(did, i18n.T("started the wallpaper daemon"))
	}
	// A unit systemd skipped on its ConditionEnvironment stays inactive until
	// something asks again, and `start` alone would be refused a second time on
	// the stale condition result, so reset-failed clears the recorded outcome
	// first. try-restart then refreshes a daemon still running a pre-update
	// binary so the delivered one paints (it restores the recorded wallpaper, or
	// a shipped default when none is recorded, instead of leaving an empty
	// frame), and start brings up one that is down. Best-effort: outside a
	// graphical session the condition refuses them all and autostart starts the
	// daemon at the next login.
	if start {
		_ = ryogamiService([]string{"--user", "reset-failed", ryogamiUserUnit})
	}
	_ = ryogamiService([]string{"--user", "try-restart", ryogamiUserUnit})
	startCode := ryogamiService([]string{"--user", "start", ryogamiUserUnit})
	if state.inSession && (startCode != host.ExitOK || !ryogamiUnitActive()) {
		return failRes(i18n.T("could not start the ryogami wallpaper daemon (service exit %d)"), startCode).
			withFix("ryoku-host svc --user start " + ryogamiUserUnit)
	}
	return fixedRes(i18n.T("cut the wallpaper over to Ryogami: ") + strings.Join(did, ", "))
}
