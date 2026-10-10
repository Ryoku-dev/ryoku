package doctor

import (
	"fmt"
	"os"
	"strings"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"
	"ryoku-cli/internal/updater"

	i18n "ryoku-i18n"
)

var (
	bootGuardCheckout = func() bool { return sys.ResolveRepo() != "" }
	bootGuardShipped  = func(manager host.PackageManager) bool {
		if manager == host.XBPS {
			return sys.Exists("/etc/sv/ryoku-boot-guard")
		}
		return sys.Exists("/usr/lib/systemd/system/ryoku-boot-guard.service")
	}
	bootGuardWritable = bootOKWritable
	bootGuardNotice   = updater.BootNotice
)

// ---- reconciler: boot guard ----------------------------------------------------
//
// reconcileBootGuard keeps ryoku-boot-guard.service enabled on a packaged box
// (the ryoku package ships the unit; enabling it here is how boxes installed
// before it get it, since doctor runs after every update) and surfaces the
// guard's last notice: an update it reverted, or a boot menu it repointed at
// the pre-update snapshot. The notice is a one-time report and is cleared
// once shown, so it never nags.
func reconcileBootGuard(checkOnly bool) recResult {
	manager, err := doctorPackageManager()
	if err != nil || manager != host.Pacman && manager != host.XBPS {
		return okRes(i18n.T("boot guard package checks are not available on this host"))
	}
	if bootGuardCheckout() || !doctorPackageInstalled("ryoku-desktop") {
		return okRes(i18n.T("not a packaged install; the boot guard watches package updates only"))
	}
	if n := bootGuardNotice(); n != nil {
		msg := fmt.Sprintf(i18n.T("the boot guard acted on %s: %s"), n.At, n.Detail)
		if !checkOnly {
			_ = sys.Sudo("rm", "-f", "/var/lib/ryoku/boot/notice.json")
		}
		return warnRes("%s", msg).withFix(i18n.T("see `ryoku rollback` and `ryoku status`"))
	}
	if !bootGuardShipped(manager) {
		return okRes(i18n.T("boot guard not shipped by this release yet"))
	}
	var problems []string
	if !systemServiceEnabled("ryoku-boot-guard.service") {
		problems = append(problems, i18n.T("ryoku-boot-guard.service is not enabled"))
	}
	if !bootGuardWritable() {
		problems = append(problems, i18n.T("/var/lib/ryoku/boot does not take a session's boot record"))
	}
	if len(problems) == 0 {
		return okRes(i18n.T("boot guard enabled"))
	}

	fix := "sudo systemctl enable ryoku-boot-guard.service && sudo systemd-tmpfiles --create /usr/lib/tmpfiles.d/ryoku.conf"
	if manager == host.XBPS {
		fix = i18n.T("sudo ryoku-host svc --system enable ryoku-boot-guard.service && sudo install -d -m 0755 /var/lib/ryoku && sudo install -d -m 1777 /var/lib/ryoku/boot")
	}
	if checkOnly {
		return wouldRes(i18n.T("%s, so a failed update is not reverted automatically"), strings.Join(problems, "; ")).withFix(fix)
	}
	if manager == host.XBPS {
		if err := runSystemService("enable", "ryoku-boot-guard.service"); err != nil {
			return failRes(i18n.T("could not enable ryoku-boot-guard.service: %v"), err).withFix(fix)
		}
		if err := sys.Sudo("install", "-d", "-m", "0755", "/var/lib/ryoku"); err != nil {
			return failRes(i18n.T("could not prepare /var/lib/ryoku: %v"), err).withFix(fix)
		}
		if err := sys.Sudo("install", "-d", "-m", "1777", "/var/lib/ryoku/boot"); err != nil || !bootGuardWritable() {
			return failRes(i18n.T("could not prepare /var/lib/ryoku/boot: %v"), err).withFix(fix)
		}
		return fixedRes(i18n.T("enabled the boot guard; a packaged update that cannot boot twice is reverted"))
	}
	if err := sys.Sudo("systemctl", "enable", "ryoku-boot-guard.service"); err != nil {
		return failRes(i18n.T("could not enable ryoku-boot-guard.service: %v"), err).withFix(fix)
	}
	if err := sys.Sudo("systemd-tmpfiles", "--create", "/usr/lib/tmpfiles.d/ryoku.conf"); err != nil || !bootGuardWritable() {
		return failRes(i18n.T("could not prepare /var/lib/ryoku/boot: %v"), err).withFix(fix)
	}
	return fixedRes(i18n.T("enabled the boot guard; a packaged update that cannot boot twice is reverted"))
}

// bootOKWritable reports whether this user can drop a boot record where the
// guard reads it: the directory exists, the parent is traversable, and the
// sticky world-writable bit is on.
func bootOKWritable() bool {
	st, err := os.Stat("/var/lib/ryoku/boot")
	if err != nil || !st.IsDir() {
		return false
	}
	return st.Mode().Perm()&0o002 != 0 && st.Mode()&os.ModeSticky != 0
}
