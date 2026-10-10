package doctor

import (
	"errors"
	"testing"

	"ryoku-cli/internal/host"
)

// stubGuard drives the three seams the reconciler reads: which units are
// active, whether the cutover lock is free, and whether the stop succeeds.
func stubGuard(t *testing.T, active map[string]bool, lockFree bool, stopErr error) *[]string {
	t.Helper()
	units := active
	if units == nil {
		units = map[string]bool{}
	}
	origActive := userUnitActive
	userUnitActive = func(unit string) bool { return units[unit] }
	t.Cleanup(func() { userUnitActive = origActive })
	origLock := cutoverLockFree
	cutoverLockFree = func() bool { return lockFree }
	t.Cleanup(func() { cutoverLockFree = origLock })
	var stopped []string
	origStop := stopLeakedSleepGuard
	stopLeakedSleepGuard = func() error {
		stopped = append(stopped, "stop")
		return stopErr
	}
	t.Cleanup(func() { stopLeakedSleepGuard = origStop })
	return &stopped
}

func TestLeakedSleepGuardReclaim(t *testing.T) {
	guard := map[string]bool{updateSleepGuardUnit: true}
	waiter := map[string]bool{
		updateSleepGuardUnit: true,
		cutoverWaiterUnit:    true,
	}

	stubGuard(t, nil, false, nil)
	if r := reconcileLeakedSleepGuard(false); r.status != recOK {
		t.Fatalf("quiet box: %s %q, want ok", r.status.label(), r.detail)
	}

	stubbed := stubGuard(t, guard, false, nil)
	if r := reconcileLeakedSleepGuard(false); r.status != recOK {
		t.Fatalf("live cutover: %s %q, want ok", r.status.label(), r.detail)
	}
	if len(*stubbed) != 0 {
		t.Fatal("a live cutover's guard must never be stopped")
	}

	stubbed = stubGuard(t, waiter, true, nil)
	if r := reconcileLeakedSleepGuard(false); r.status != recOK {
		t.Fatalf("deferred cutover: %s %q, want ok", r.status.label(), r.detail)
	}
	if len(*stubbed) != 0 {
		t.Fatal("a deferred cutover's guard must never be stopped")
	}

	stubbed = stubGuard(t, guard, true, nil)
	if r := reconcileLeakedSleepGuard(true); r.status != recWouldFix {
		t.Fatalf("leak check: %s %q, want a todo", r.status.label(), r.detail)
	}
	if len(*stubbed) != 0 {
		t.Fatal("check-only must not stop the guard")
	}
	if r := reconcileLeakedSleepGuard(false); r.status != recFixed {
		t.Fatalf("leak repair: %s %q, want fixed", r.status.label(), r.detail)
	}
	if len(*stubbed) != 1 {
		t.Fatalf("leak repair: %d stops, want exactly 1", len(*stubbed))
	}

	stubGuard(t, guard, true, errors.New("boom"))
	if r := reconcileLeakedSleepGuard(false); r.status != recFailed {
		t.Fatalf("failed stop: %s %q, want fail", r.status.label(), r.detail)
	}
}

func TestSleepGuardUsesHostUserServices(t *testing.T) {
	oldService := sleepGuardService
	t.Cleanup(func() { sleepGuardService = oldService })
	var calls [][]string
	sleepGuardService = func(args []string) int {
		calls = append(calls, append([]string(nil), args...))
		return host.ExitOK
	}

	if !userUnitActive(updateSleepGuardUnit) {
		t.Fatal("active service should be reported active")
	}
	if err := stopLeakedSleepGuard(); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[0][1] != "is-active" || calls[1][1] != "stop" {
		t.Fatalf("service calls = %v, want user is-active then stop", calls)
	}
}
