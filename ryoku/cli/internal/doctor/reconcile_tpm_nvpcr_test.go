package doctor

import (
	"slices"
	"testing"

	"ryoku-cli/internal/host"
)

// Only the TPM units that fail for want of NvPCRs are cleared, and only once
// the TPM carries the mark; a box without the rule keeps them reported.
func TestClearNvpcrFailures(t *testing.T) {
	failed := []string{
		"systemd-pcrlogin@1000.service", "systemd-pcrproduct.service",
		"systemd-tpm2-setup-early.service", "bluetooth.service", "systemd-tpm2-setup.service",
	}
	stub := func(t *testing.T, marked, ruleInstalled, markAfterTrigger bool) (*[]string, *int) {
		var rerun []string
		triggers := 0
		pi, pm, pr, pt, pu := tpmNvpcrInit, tpmNvpcrMarked, tpmNvpcrRuleInstalled, tpmNvpcrRetrigger, tpmNvpcrRerun
		t.Cleanup(func() {
			tpmNvpcrInit, tpmNvpcrMarked, tpmNvpcrRuleInstalled, tpmNvpcrRetrigger, tpmNvpcrRerun = pi, pm, pr, pt, pu
		})
		tpmNvpcrInit = func() (host.InitSystem, error) { return host.Systemd, nil }
		tpmNvpcrMarked = func() bool { return marked }
		tpmNvpcrRuleInstalled = func() bool { return ruleInstalled }
		tpmNvpcrRetrigger = func() { triggers++; marked = markAfterTrigger }
		tpmNvpcrRerun = func(u string) error { rerun = append(rerun, u); return nil }
		return &rerun, &triggers
	}

	t.Run("marked", func(t *testing.T) {
		rerun, _ := stub(t, true, true, true)
		got := clearNvpcrFailures(failed)
		want := []string{"systemd-pcrlogin@1000.service", "systemd-pcrproduct.service", "systemd-tpm2-setup-early.service"}
		if !slices.Equal(got, want) || !slices.Equal(*rerun, want) {
			t.Errorf("cleared %v (reran %v), want exactly the NvPCR units %v", got, *rerun, want)
		}
	})
	t.Run("rule installed, device predates it", func(t *testing.T) {
		_, triggers := stub(t, false, true, true)
		if got := clearNvpcrFailures(failed); len(got) != 3 || *triggers != 1 {
			t.Errorf("cleared %v after %d retrigger(s), want 3 after one", got, *triggers)
		}
	})
	t.Run("no rule", func(t *testing.T) {
		rerun, triggers := stub(t, false, false, true)
		if got := clearNvpcrFailures(failed); got != nil || len(*rerun) != 0 || *triggers != 0 {
			t.Errorf("unmarked TPM without the rule: cleared %v, reran %v, triggered %d; want nothing", got, *rerun, *triggers)
		}
	})
	t.Run("retrigger does not mark", func(t *testing.T) {
		rerun, _ := stub(t, false, true, false)
		if got := clearNvpcrFailures(failed); got != nil || len(*rerun) != 0 {
			t.Errorf("still unmarked: cleared %v, reran %v; want nothing", got, *rerun)
		}
	})
	t.Run("runit has no systemd NvPCR units", func(t *testing.T) {
		rerun, _ := stub(t, true, true, true)
		tpmNvpcrInit = func() (host.InitSystem, error) { return host.Runit, nil }
		if got := clearNvpcrFailures(failed); got != nil || len(*rerun) != 0 {
			t.Errorf("runit cleared %v and reran %v, want no systemd-unit work", got, *rerun)
		}
	})
}
