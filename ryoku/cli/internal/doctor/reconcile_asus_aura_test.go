package doctor

import "testing"

func TestReconcileAsusAuraIgnoresOtherHardware(t *testing.T) {
	withAsusAuraTestState(t, asusAuraStatus{})
	got := reconcileAsusAura(false)
	if got.status != recOK {
		t.Fatalf("status = %v, detail = %s", got.status, got.detail)
	}
}

func TestReconcileAsusAuraInstallsAndStartsProvider(t *testing.T) {
	withAsusAuraTestState(t, asusAuraStatus{supported: true})
	var installed, started bool
	oldInstall, oldStart := installAsusAura, startAsusAura
	installAsusAura = func() error { installed = true; return nil }
	startAsusAura = func() error { started = true; return nil }
	defer func() { installAsusAura, startAsusAura = oldInstall, oldStart }()

	got := reconcileAsusAura(false)
	if got.status != recFixed || !installed || !started {
		t.Fatalf("result = %+v, installed=%v started=%v", got, installed, started)
	}
}

func TestReconcileAsusAuraCheckOnlyAndTLPConflict(t *testing.T) {
	withAsusAuraTestState(t, asusAuraStatus{supported: true})
	if got := reconcileAsusAura(true); got.status != recWouldFix {
		t.Fatalf("check status = %v, detail = %s", got.status, got.detail)
	}

	withAsusAuraTestState(t, asusAuraStatus{supported: true, tlp: true})
	if got := reconcileAsusAura(false); got.status != recWarn {
		t.Fatalf("TLP status = %v, detail = %s", got.status, got.detail)
	}
}

func TestReconcileAsusAuraNotesUnavailableProvider(t *testing.T) {
	withAsusAuraTestState(t, asusAuraStatus{supported: true})
	asusProviderAvailable = func() bool { return false }
	asusInstallAdvice = func() string { return "asusctl is not packaged for this system" }
	oldInstall := installAsusAura
	installAsusAura = func() error { t.Fatal("unavailable provider was installed"); return nil }
	t.Cleanup(func() { installAsusAura = oldInstall })

	got := reconcileAsusAura(false)
	if got.status != recNote || got.remedy != "asusctl is not packaged for this system" {
		t.Fatalf("result = %#v, want unavailable note with host advice", got)
	}
}

func TestAsusAuraServiceUsesSystemHostSeam(t *testing.T) {
	old := asusService
	var args []string
	asusService = func(got []string) int {
		args = append([]string(nil), got...)
		return 0
	}
	t.Cleanup(func() { asusService = old })

	if err := startAsusAura(); err != nil {
		t.Fatal(err)
	}
	if len(args) != 3 || args[0] != "--system" || args[1] != "start" || args[2] != "asusd" {
		t.Fatalf("service args = %v, want host system service start", args)
	}
}

func TestAsusAuraRunitRemedyUsesHostService(t *testing.T) {
	t.Setenv("RYOKU_HOST_INIT", "runit")
	if got := asusServiceAdvice(); got != "sudo ryoku-host svc --system start asusd" {
		t.Fatalf("remedy = %q, want runit-safe host service command", got)
	}
}

func withAsusAuraTestState(t *testing.T, state asusAuraStatus) {
	t.Helper()
	oldStatus := readAsusAuraStatus
	oldAvailable, oldAdvice := asusProviderAvailable, asusInstallAdvice
	readAsusAuraStatus = func() asusAuraStatus { return state }
	asusProviderAvailable = func() bool { return true }
	asusInstallAdvice = func() string { return "sudo pacman -S asusctl" }
	t.Cleanup(func() {
		readAsusAuraStatus = oldStatus
		asusProviderAvailable, asusInstallAdvice = oldAvailable, oldAdvice
	})
	isolateProvisioned(t)
}
