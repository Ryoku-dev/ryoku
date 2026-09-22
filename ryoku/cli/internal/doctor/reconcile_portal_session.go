package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	i18n "ryoku-i18n"
	wm "ryoku-wm"
)

// ---- reconciler: a portal frontend left over from a previous session ----------
//
// xdg-desktop-portal is only PartOf=graphical-session.target and nothing ever
// stops that target, so logging out and back in leaves the old session's
// frontend running. A ScreenCast request then times out inside it instead of
// reaching the compositor's portal backend: no source picker, and the app is
// handed nothing with no error anywhere the user looks. A frontend older than
// the session it serves cannot belong to this session, and restarting it is
// enough; the backend re-registers on demand.

// parseStartTicks reads field 22 of /proc/<pid>/stat, in clock ticks since boot.
// The comm field may itself hold spaces and parentheses, so the count starts at
// the LAST ')' instead of splitting the whole line.
func parseStartTicks(stat string) (uint64, bool) {
	paren := strings.LastIndexByte(stat, ')')
	if paren < 0 {
		return 0, false
	}
	// state (field 3) is the first field after comm, so starttime (22) is the
	// 20th of what remains.
	f := strings.Fields(stat[paren+1:])
	if len(f) < 20 {
		return 0, false
	}
	ticks, err := strconv.ParseUint(f[19], 10, 64)
	if err != nil {
		return 0, false
	}
	return ticks, true
}

func procStartTicks(pid int) (uint64, bool) {
	b, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return 0, false
	}
	return parseStartTicks(string(b))
}

// sessionStartTicks is when this login session began, read from its leader
// process so the check stays compositor-neutral: a portal frontend left from a
// previous login predates this leader whatever window manager runs.
func sessionStartTicks() (uint64, bool) {
	sid := strings.TrimSpace(os.Getenv("XDG_SESSION_ID"))
	if sid == "" {
		return 0, false
	}
	out, err := exec.Command("loginctl", "show-session", sid, "-p", "Leader", "--value").Output()
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	return procStartTicks(pid)
}

// userUnitMainPID is the MainPID of a --user unit, or 0 when it is not running.
func userUnitMainPID(unit string) int {
	out, err := exec.Command("systemctl", "--user", "show", "-p", "MainPID", "--value", unit).Output()
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0
	}
	return pid
}

// portalScreenCastInterface is the contract every app that asks "which screen
// should I share?" depends on. It is published by the portal frontend only when
// one of the configured backends actually implements it.
const portalScreenCastInterface = "org.freedesktop.portal.ScreenCast"

// portalInterfaceExposed asks the running frontend which portal interfaces it
// publishes. probeable is false when the bus cannot be reached at all (no
// session, or no busctl), which is not a finding: a headless or dev shell must
// not be told its screen sharing is broken when there is no desktop to share.
func portalInterfaceExposed(iface string) (exposed, probeable bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "busctl", "--user", "introspect",
		"org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop").Output()
	if err != nil {
		return false, false
	}
	// Introspection lists one interface per line; match the whole token so
	// ScreenCast is never satisfied by Screenshot.
	for _, line := range strings.Split(string(out), "\n") {
		for _, field := range strings.Fields(line) {
			if field == iface {
				return true, true
			}
		}
	}
	return false, true
}

// portalBackendFault names what to do about a missing ScreenCast, given the
// backend the running compositor declares. The package being absent is the
// common case and is installable; a package that is installed but never
// publishes its interface is a different, deeper fault, and telling the user to
// install it again would send them in circles.
func portalBackendFault() recResult {
	backend := ""
	if caps, err := wm.Open().Caps(); err == nil {
		backend = caps.PortalBackend
	}
	frontend, ok := portalFrontends[backend]
	if !ok {
		return warnRes(i18n.T("screen share offers no source picker: this session's portal publishes no ScreenCast interface")).
			withFix(i18n.T("ryoku doctor --check, then reinstall the desktop portal backend"))
	}
	if !anyPkgInstalled(frontend.pkgs...) {
		return warnRes(i18n.T("screen share offers no source picker: the portal backend this desktop needs is not installed")).
			withFix(frontend.fix)
	}
	return warnRes(i18n.T("screen share offers no source picker: the portal backend is installed but never published its interface")).
		withFix(fmt.Sprintf("systemctl --user status xdg-desktop-portal-%s.service", backend))
}

func reconcilePortalSession(checkOnly bool) recResult {
	fePID := userUnitMainPID("xdg-desktop-portal.service")
	exposed, probeable := portalInterfaceExposed(portalScreenCastInterface)
	if !probeable {
		// No bus to interrogate: fall back to the lifetime check, which is the
		// only signal available and still catches a frontend from a dead login.
		return reconcilePortalLifetime(checkOnly, fePID)
	}

	stale := func() bool {
		sessionStart, ok := sessionStartTicks()
		if !ok || fePID == 0 {
			return false
		}
		feStart, ok := procStartTicks(fePID)
		return ok && feStart < sessionStart
	}

	if exposed && !stale() {
		return okRes(i18n.T("screen share can offer a source picker"))
	}

	// Either the frontend is left over from a previous login (its backends are
	// bound to a dead Wayland session, so a ScreenCast request times out inside
	// it) or ScreenCast was never published at all. Restarting is the one action
	// that can fix the first and is a cheap re-test for the second.
	if checkOnly {
		if exposed {
			return wouldRes(i18n.T("the portal frontend predates this login session; screen share can silently share nothing")).
				withFix(i18n.T("ryoku doctor restarts xdg-desktop-portal"))
		}
		return portalBackendFault()
	}

	if fePID != 0 {
		if err := exec.Command("systemctl", "--user", "restart", "xdg-desktop-portal.service").Run(); err != nil {
			return failRes(i18n.T("could not restart the portal frontend: %v"), err).
				withFix("systemctl --user restart xdg-desktop-portal.service")
		}
		for range 6 {
			time.Sleep(500 * time.Millisecond)
			if exposed, _ := portalInterfaceExposed(portalScreenCastInterface); exposed {
				return fixedRes(i18n.T("restarted the portal frontend; screen share picks a source again"))
			}
		}
	}
	return portalBackendFault()
}

// reconcilePortalLifetime is the pre-probe check: a frontend older than the
// login session it serves cannot belong to it. Kept for shells where the bus
// cannot be interrogated.
func reconcilePortalLifetime(checkOnly bool, fePID int) recResult {
	sessionStart, ok := sessionStartTicks()
	if !ok {
		return okRes(i18n.T("no running session to compare against"))
	}
	if fePID == 0 {
		return okRes(i18n.T("portal frontend not running; it activates fresh on first use"))
	}
	feStart, ok := procStartTicks(fePID)
	if !ok {
		return okRes(i18n.T("could not read the portal frontend's start time"))
	}
	if feStart >= sessionStart {
		return okRes(i18n.T("portal frontend belongs to this session"))
	}
	if checkOnly {
		return wouldRes(i18n.T("the portal frontend predates this login session; screen share opens no source picker and silently shares nothing")).
			withFix(i18n.T("ryoku doctor restarts xdg-desktop-portal"))
	}
	if err := exec.Command("systemctl", "--user", "restart", "xdg-desktop-portal.service").Run(); err != nil {
		return failRes(i18n.T("could not restart the portal frontend: %v"), err).
			withFix("systemctl --user restart xdg-desktop-portal.service")
	}
	return fixedRes(i18n.T("restarted the portal frontend against this session; screen share picks a source again"))
}
