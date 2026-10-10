package doctor

import (
	"os/exec"
	"strings"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"
)

// systemd 262 fails four TPM units on every boot of a box whose UKI is not
// PCR-signed (every Ryoku box): the NvPCRs they set up and extend need
// tpm2-pcr-public-key.pem. The shipped udev rule (60-ryoku-tpm-nvpcr.rules)
// marks such a TPM TPM2_BROKEN_NVPCR so the next boot skips them cleanly; this
// clears the failures the current boot already recorded, once the TPM carries
// the mark, so the failed-services check stops reporting a fixed problem.

// nvpcrUnit reports whether unit is one of the TPM units that fail only for
// want of NvPCRs. pure.
func nvpcrUnit(unit string) bool {
	switch {
	case unit == "systemd-tpm2-setup-early.service", unit == "systemd-pcrproduct.service":
		return true
	case strings.HasPrefix(unit, "systemd-pcrlogin@") && strings.HasSuffix(unit, ".service"):
		return true
	}
	return false
}

// Seams over the live box, replaced in tests.
var (
	tpmNvpcrInit = func() (host.InitSystem, error) {
		return host.Default().Init()
	}
	tpmNvpcrMarked = func() bool {
		out, err := exec.Command("udevadm", "info", "-q", "property", "-n", "/dev/tpmrm0").Output()
		return err == nil && strings.Contains(string(out), "TPM2_BROKEN_NVPCR=1")
	}
	tpmNvpcrRuleInstalled = func() bool {
		return sys.Exists("/usr/lib/udev/rules.d/60-ryoku-tpm-nvpcr.rules")
	}
	tpmNvpcrRetrigger = func() {
		_ = exec.Command("sudo", "-n", "udevadm", "trigger", "--action=change", "--subsystem-match=tpmrm", "--settle").Run()
	}
	tpmNvpcrRerun = func(unit string) error {
		if err := exec.Command("sudo", "-n", "systemctl", "reset-failed", unit).Run(); err != nil {
			return err
		}
		// the early setup is condition-skipped once the SRK is published;
		// the measurement units now exit cleanly on a marked TPM
		if unit == "systemd-tpm2-setup-early.service" {
			return nil
		}
		return exec.Command("sudo", "-n", "systemctl", "restart", unit).Run()
	}
)

// clearNvpcrFailures re-runs the failed NvPCR units once the TPM is marked
// (retriggering udev when the rule is installed but the device predates it)
// and returns the units it cleared. Unmarked, it leaves them for the report.
func clearNvpcrFailures(failed []string) []string {
	initSystem, err := tpmNvpcrInit()
	if err != nil || initSystem != host.Systemd {
		return nil
	}
	var units []string
	for _, u := range failed {
		if nvpcrUnit(u) {
			units = append(units, u)
		}
	}
	if len(units) == 0 {
		return nil
	}
	if !tpmNvpcrMarked() {
		if !tpmNvpcrRuleInstalled() {
			return nil
		}
		tpmNvpcrRetrigger()
		if !tpmNvpcrMarked() {
			return nil
		}
	}
	var cleared []string
	for _, u := range units {
		if tpmNvpcrRerun(u) == nil {
			cleared = append(cleared, u)
		}
	}
	return cleared
}
