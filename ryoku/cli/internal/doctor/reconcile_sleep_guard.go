package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"ryoku-cli/internal/host"

	i18n "ryoku-i18n"
)

// ---- reconciler: leaked update sleep guard ---------------------------------
//
// The update cutover takes a durable sleep inhibitor
// (ryoku-power-cutover-guard.service) before it stops the lid and idle
// owners, and it holds ryoku-power-cutover.lock for the whole run. Builds
// before the guard-release fix could exit on a mid-cutover failure with the
// inhibitor still live, and logind then denied every later suspend with
// "Operation denied due to active block inhibitor" until logout or a reboot
// (#282, #285). The lock is the ownership proof: every starter takes it
// before the guard and keeps it until the run ends, so a guard that is
// active while the lock is free belongs to a process that is already gone.
// The one honest gap is the deferred generation cutover, which keeps the
// guard between the deferring run and its waiter; a live
// ryoku-qylock-cutover-wait.service owns it and is left alone.

const (
	updateSleepGuardUnit = "ryoku-power-cutover-guard.service"
	cutoverWaiterUnit    = "ryoku-qylock-cutover-wait.service"
)

// cutoverLockFree reports whether this user's power-cutover lock can be
// taken right now. A package var so tests drive both verdicts without a
// live cutover.
var cutoverLockFree = func() bool {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	f, err := os.OpenFile(filepath.Join(dir, "ryoku-power-cutover.lock"),
		os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return false
	}
	defer f.Close()
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) == nil
}

// The host seam maps these names to systemd user units or the services
// supervised under ~/.config/service.
var sleepGuardService = func(args []string) int {
	return host.Default().Service(args)
}

var userUnitActive = func(unit string) bool {
	return sleepGuardService([]string{"--user", "is-active", unit}) == host.ExitOK
}

// stopLeakedSleepGuard stops the orphaned guard. A var so the repair test
// never touches the real session supervisor.
var stopLeakedSleepGuard = func() error {
	if code := sleepGuardService([]string{"--user", "stop", updateSleepGuardUnit}); code != host.ExitOK {
		return fmt.Errorf("service stop exited %d", code)
	}
	return nil
}

func reconcileLeakedSleepGuard(checkOnly bool) recResult {
	if !userUnitActive(updateSleepGuardUnit) {
		return okRes(i18n.T("no update sleep guard is holding suspend"))
	}
	if !cutoverLockFree() {
		return okRes(i18n.T("an update cutover is running; its sleep guard is live"))
	}
	if userUnitActive(cutoverWaiterUnit) {
		return okRes(i18n.T("a deferred generation cutover owns the sleep guard"))
	}
	fix := "ryoku-host svc --user stop " + updateSleepGuardUnit
	if checkOnly {
		return wouldRes(i18n.T("a crashed update left its sleep guard active; suspend is denied")).
			withFix("%s", fix)
	}
	if err := stopLeakedSleepGuard(); err != nil {
		return failRes(i18n.T("could not release the leaked sleep guard: %v"), err).
			withFix("%s", fix)
	}
	return fixedRes(i18n.T("released a leaked update sleep guard; suspend works again"))
}
