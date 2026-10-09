package doctor

import (
	"errors"
	"testing"

	"ryoku-cli/internal/host"
)

func withTurnstileDoctorFakes(t *testing.T) {
	t.Helper()
	oldKind, oldCheck := turnstileHostKind, turnstileCheck
	oldUser, oldSystem, oldWrappers := turnstileEnsureUser, turnstileEnsureSystem, turnstileFixWrappers
	t.Cleanup(func() {
		turnstileHostKind, turnstileCheck = oldKind, oldCheck
		turnstileEnsureUser, turnstileEnsureSystem, turnstileFixWrappers = oldUser, oldSystem, oldWrappers
	})
}

func TestReconcileTurnstileIsNoopOnSystemd(t *testing.T) {
	withTurnstileDoctorFakes(t)
	turnstileHostKind = func() (host.InitSystem, error) { return host.Systemd, nil }
	turnstileCheck = func() (int, string) { t.Fatal("systemd ran Turnstile check"); return 0, "" }
	if result := reconcileTurnstile(false); result.status != recOK {
		t.Fatalf("result = %#v", result)
	}
}

func TestReconcileTurnstileCheckOnlyReportsFix(t *testing.T) {
	withTurnstileDoctorFakes(t)
	turnstileHostKind = func() (host.InitSystem, error) { return host.Runit, nil }
	turnstileCheck = func() (int, string) { return host.ExitFalse, "turnstile-pam: missing -> repair" }
	turnstileEnsureUser = func() int { t.Fatal("check-only repaired user state"); return 0 }
	result := reconcileTurnstile(true)
	if result.status != recWouldFix || result.remedy == "" {
		t.Fatalf("result = %#v", result)
	}
}

func TestReconcileTurnstileRepairsAndRechecks(t *testing.T) {
	withTurnstileDoctorFakes(t)
	turnstileHostKind = func() (host.InitSystem, error) { return host.Runit, nil }
	checks := 0
	turnstileCheck = func() (int, string) {
		checks++
		if checks == 1 {
			return host.ExitFalse, "broken"
		}
		return host.ExitOK, ""
	}
	var order []string
	turnstileEnsureUser = func() int { order = append(order, "user"); return host.ExitOK }
	turnstileEnsureSystem = func() error { order = append(order, "system"); return nil }
	turnstileFixWrappers = func() int { order = append(order, "wrappers"); return host.ExitOK }
	result := reconcileTurnstile(false)
	if result.status != recFixed {
		t.Fatalf("result = %#v", result)
	}
	if checks != 2 || len(order) != 3 || order[0] != "user" || order[1] != "system" || order[2] != "wrappers" {
		t.Fatalf("checks=%d order=%v", checks, order)
	}
}

func TestReconcileTurnstileReportsFailedRootRepair(t *testing.T) {
	withTurnstileDoctorFakes(t)
	turnstileHostKind = func() (host.InitSystem, error) { return host.Runit, nil }
	turnstileCheck = func() (int, string) { return host.ExitFalse, "broken" }
	turnstileEnsureUser = func() int { return host.ExitOK }
	turnstileEnsureSystem = func() error { return errors.New("no authorization") }
	result := reconcileTurnstile(false)
	if result.status != recFailed || result.remedy == "" {
		t.Fatalf("result = %#v", result)
	}
}
