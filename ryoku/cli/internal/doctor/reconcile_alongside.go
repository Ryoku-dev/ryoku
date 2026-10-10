package doctor

import (
	"os/exec"
	"strings"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// ---- reconciler: alongside UEFI boot entry -----------------------------------

// An alongside install shares the existing OS's ESP: the installer writes our
// static stage-1 hop to /efi/EFI/ryoku/limine.conf (the shared ESP is mounted at
// /efi, an fstab line the installer added) and registers an NVRAM entry loading
// \EFI\ryoku\BOOTX64.EFI so the firmware boots us. Some firmware silently drops
// NVRAM entries across updates (the QEMU/OVMF install demonstrated the class);
// when that happens the box falls off the boot menu and boots only via the
// shared ESP's removable EFI/BOOT fallback, if at all.

const (
	alongsideHopPath    = "/efi/EFI/ryoku/limine.conf"
	alongsideLoaderPath = `\EFI\ryoku\BOOTX64.EFI`
)

// isAlongsideSystem: an alongside box has the shared ESP mounted at /efi (an
// fstab entry) AND our stage-1 hop present on it. both must hold, so a
// whole-disk box (no /efi mount) or a foreign /efi mount without our hop is
// never mistaken for one.
func isAlongsideSystem(fstab string, hopPresent bool) bool {
	return hopPresent && fstabHasMount(fstab, "/efi")
}

// fstabHasMount: is there an uncommented fstab line whose mount point (field 2)
// is exactly mp?
func fstabHasMount(fstab, mp string) bool {
	for _, l := range strings.Split(fstab, "\n") {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if f := strings.Fields(t); len(f) >= 2 && f[1] == mp {
			return true
		}
	}
	return false
}

// hasAlongsideBootEntry: does any active NVRAM entry load our stage-1 hop
// (\EFI\ryoku\BOOTX64.EFI)? efibootmgr -v prints the loader path in the device
// path; match case-insensitively (firmware may upcase it). matching the LOADER
// PATH, not the "Ryoku" label, disambiguates from a whole-disk Ryoku entry. pure,
// so the watch is testable with fixture efibootmgr output.
func hasAlongsideBootEntry(efibootmgr string) bool {
	needle := strings.ToLower(alongsideLoaderPath)
	for _, line := range strings.Split(efibootmgr, "\n") {
		if len(line) < 9 || !strings.HasPrefix(line, "Boot") || !isHex4(line[4:8]) || line[8] != '*' {
			continue // only active boot entries
		}
		if strings.Contains(strings.ToLower(line), needle) {
			return true
		}
	}
	return false
}

// efibootmgrVerbose returns `efibootmgr -v` output (needed to see loader paths,
// which plain efibootmgr omits), or "" when the tool is absent or errors.
func efibootmgrVerbose() string {
	out, err := exec.Command("efibootmgr", "-v").Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// reconcileAlongsideBootEntry warns when an alongside box has lost the NVRAM
// entry that boots our stage-1 hop. warn-only: re-registering NVRAM reorders the
// user's firmware boot menu, so a reconciler must not do it silently -- it prints
// the exact command instead. not an alongside box, or no efibootmgr, = nothing
// to watch. never writes NVRAM.
func reconcileAlongsideBootEntry(_ bool) recResult {
	if !limineManagedBoot() {
		return okRes(i18n.T("Ryoku's Limine tooling does not manage this boot"))
	}
	if !isAlongsideSystem(readFileSafe("/etc/fstab"), sys.Exists(alongsideHopPath)) {
		return okRes(i18n.T("not an alongside install (no shared ESP at /efi with our stage-1 hop)"))
	}
	if !sys.Has("efibootmgr") {
		return okRes(i18n.T("no efibootmgr to inspect the UEFI boot menu"))
	}
	out := efibootmgrVerbose()
	if out == "" {
		return okRes(i18n.T("no UEFI boot entries to check"))
	}
	if hasAlongsideBootEntry(out) {
		return okRes(i18n.T("alongside UEFI boot entry present (loads \\EFI\\ryoku\\BOOTX64.EFI)"))
	}
	return warnRes(i18n.T("the alongside 'Ryoku' UEFI boot entry (\\EFI\\ryoku\\BOOTX64.EFI on the shared ESP) is missing; some firmware drops NVRAM entries across updates, so the machine now boots only via the removable EFI/BOOT fallback, if at all")).
		withFix(`sudo efibootmgr --create --disk <shared-ESP disk> --part <shared-ESP part> --label Ryoku --loader '\EFI\ryoku\BOOTX64.EFI' --unicode`)
}
