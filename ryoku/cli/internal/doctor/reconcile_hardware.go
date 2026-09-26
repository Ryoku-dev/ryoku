package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// ---- reconciler: display backlight -------------------------------------------

// reconcileBacklight flags the common brightness failures:
//   - no backlight interface at all.
//   - backlight present but no brightnessctl to drive it.
//   - hybrid-GPU laptop with only a firmware backlight.
//
// for the last we trust the kernel's own verdict (dGPU reports no native
// backlight) over a sysfs value the panel may ignore. detect-and-warn only;
// the fixes (GPU mux switch, kernel parameters) are too machine-specific to
// apply blindly.
func reconcileBacklight(_ bool) recResult {
	devs := backlightDevices()
	if len(devs) == 0 {
		if !isLaptop() {
			return okRes(i18n.T("no internal backlight (desktop or external display)"))
		}
		return warnRes(i18n.T("no backlight interface found; display brightness cannot be set")).
			withFix(i18n.T("try a kernel parameter such as acpi_backlight=native or acpi_backlight=vendor"))
	}
	if !sys.Has("brightnessctl") {
		return warnRes(i18n.T("backlight present but brightnessctl is missing; brightness keys and idle-dim will not work")).
			withFix("sudo pacman -S brightnessctl")
	}
	if gpus := gpuDriversLoaded(); len(gpus) >= 2 && onlyFirmwareBacklight(devs) {
		detail := fmt.Sprintf(i18n.T("hybrid GPU (%s) with only a firmware backlight (%s); the panel may not dim"),
			strings.Join(gpus, "+"), strings.Join(devs, ","))
		if nvidiaBacklightDead() {
			detail = fmt.Sprintf(i18n.T("hybrid GPU (%s): the kernel reports the dGPU has no working backlight, and the firmware fallback (%s) does not dim the panel"),
				strings.Join(gpus, "+"), strings.Join(devs, ","))
		}
		fix := i18n.T("route the panel to the iGPU: set the BIOS GPU/MUX mode to Hybrid and reboot")
		if name := igpuBacklightName(gpus); name != "" {
			fix += i18n.Tf(", then %s appears", name)
		} else {
			fix += i18n.T(" so the panel's native backlight appears")
		}
		if sys.Has("supergfxctl") {
			fix += i18n.T("; on a supported ASUS laptop `supergfxctl -m Hybrid` switches it without a BIOS trip")
		}
		return noteRes("%s", detail).withFix(fix)
	}
	if len(devs) > 1 {
		// More than one backlight means one of them is a phantom on this
		// hardware, and a brightness control that "only has off and full" is
		// exactly what writing percentages to the wrong one looks like. Name
		// the device every Ryoku writer targets and each candidate's current
		// level, so the next report from such a box pinpoints it in one line.
		pick := backlightPick()
		if pick == "" {
			pick = "-"
		}
		return noteRes(i18n.T("backlight: %s; Ryoku writes to %s (%s)"),
			strings.Join(devs, ", "), pick, backlightLevels(devs))
	}
	return okRes(i18n.T("backlight: %s"), strings.Join(devs, ", "))
}

// backlightSysRoot is the sysfs root the backlight readings use; a var so the
// level formatter is unit-tested against a fixture tree.
var backlightSysRoot = "/sys/class/backlight"

// backlightPick is the device every Ryoku brightness writer targets (the media
// keys, the OSD daemon and the bar slider all call the same helper). Empty when
// the helper is absent or cannot decide.
var backlightPick = func() string {
	out, err := sys.RunOut("ryoku-hw-backlight")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// backlightLevels renders "name cur/max" per device from sysfs, so a phantom
// (stuck at max while the panel dims, or stuck at 0) is visible in the report.
func backlightLevels(devs []string) string {
	parts := make([]string, 0, len(devs))
	for _, d := range devs {
		read := func(attr string) string {
			b, err := os.ReadFile(filepath.Join(backlightSysRoot, d, attr))
			if err != nil {
				return "?"
			}
			return strings.TrimSpace(string(b))
		}
		parts = append(parts, fmt.Sprintf("%s %s/%s", d, read("brightness"), read("max_brightness")))
	}
	return strings.Join(parts, ", ")
}

// nvidiaBacklightDead: the kernel's own tell that the dGPU has no usable
// backlight and fell back to the often-broken ACPI/EC interface.
func nvidiaBacklightDead() bool {
	n := strings.TrimSpace(captureOut("sh", "-c",
		"journalctl -k -b --no-pager 2>/dev/null | grep -ic 'no NVIDIA native backlight'"))
	return n != "" && n != "0"
}

func backlightDevices() []string {
	entries, err := os.ReadDir("/sys/class/backlight")
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

func onlyFirmwareBacklight(devs []string) bool {
	for _, d := range devs {
		if strings.TrimSpace(readFileSafe("/sys/class/backlight/"+d+"/type")) != "firmware" {
			return false
		}
	}
	return len(devs) > 0
}

// gpuDriversLoaded: loaded GPU kernel drivers, so a hybrid-GPU box is
// recognizable.
func gpuDriversLoaded() []string {
	var out []string
	for _, m := range []string{"amdgpu", "nvidia", "i915", "nouveau", "xe"} {
		if sys.Exists("/sys/module/" + m) {
			out = append(out, m)
		}
	}
	return out
}

// igpuBacklightName is the native panel backlight the integrated GPU exposes
// once the panel is routed to it: intel_backlight for an Intel iGPU (i915/xe),
// amdgpu_bl0 for an AMD one. Intel wins when both are present, since an Intel
// iGPU drives the eDP even beside an AMD discrete card. "" when neither driver
// is loaded, so the hint drops the device name rather than naming the wrong one.
func igpuBacklightName(gpus []string) string {
	has := func(m string) bool {
		for _, g := range gpus {
			if g == m {
				return true
			}
		}
		return false
	}
	switch {
	case has("i915") || has("xe"):
		return "intel_backlight"
	case has("amdgpu"):
		return "amdgpu_bl0"
	default:
		return ""
	}
}

// isLaptop: machine has a battery, i.e. an internal panel whose backlight
// we'd expect to control.
func isLaptop() bool {
	entries, err := os.ReadDir("/sys/class/power_supply")
	if err != nil {
		return false
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "BAT") {
			return true
		}
	}
	return false
}

// ---- reconciler: NVIDIA boot reliability -------------------------------------

// reconcileNvidiaModeset backports the installer's NVIDIA reliability config
// to a box running the proprietary/open nvidia modules but installed (or
// last doctored) before the fix. without it, nouveau and nvidia race for the
// card at boot, so the GPU "shows up only on some boots" -- the intermittent
// detection failure users hit. mirrors system/hardware/drivers/nvidia.sh:
// blacklist nouveau, force DRM modeset, load the modules early, then rebuild
// the initramfs so it takes effect. acts ONLY when an nvidia kernel-module
// package is installed (or the module is loaded); a box on nouveau by choice
// has no such package and stays untouched -- blacklisting nouveau there
// would break its display.

// nvidiaModprobeConf mirrors system/hardware/drivers/nvidia.sh verbatim, so
// a doctored box matches a fresh install.
const nvidiaModprobeConf = `options nvidia_drm modeset=1 fbdev=1
options nvidia NVreg_PreserveVideoMemoryAllocations=1
blacklist nouveau
options nouveau modeset=0
`

const nvidiaMkinitcpioConf = "MODULES=(nvidia nvidia_modeset nvidia_uvm nvidia_drm)\n"

var (
	nvidiaPCIOutput = func() string {
		out, err := exec.Command("lspci").Output()
		if err != nil {
			return ""
		}
		return string(out)
	}
	nvidia580Installed   = func() bool { return sys.PkgInstalled("nvidia-580xx-dkms") }
	removeKepler580      = func() error { return sys.Sudo(kepler580RemovalArgs()...) }
	restoreKeplerNouveau = func() error {
		return removeRootFiles("/etc/modprobe.d/nvidia.conf", "/etc/mkinitcpio.conf.d/nvidia.conf")
	}
	rebuildKeplerNouveau = rebuildInitramfs
)

func kepler580RemovalArgs() []string {
	return []string{"pacman", "-R", "--noconfirm", "nvidia-580xx-dkms"}
}

func keplerGpuPresent() bool {
	pci := strings.ToLower(nvidiaPCIOutput())
	return strings.Contains(pci, "nvidia") && strings.Contains(pci, "gk")
}

func reconcileKeplerNvidia(checkOnly bool) recResult {
	if !keplerGpuPresent() || !nvidia580Installed() {
		return okRes(i18n.T("no incompatible 580xx driver on Kepler hardware"))
	}
	if checkOnly {
		return wouldRes(i18n.T("Kepler hardware has nvidia-580xx-dkms, which cannot bind this GPU and leaves Nouveau blacklisted")).
			withFix(i18n.T("ryoku doctor  (removes 580xx and restores Nouveau)"))
	}
	if err := removeKepler580(); err != nil {
		return failRes(i18n.T("could not remove incompatible nvidia-580xx-dkms: %v"), err).
			withFix("sudo pacman -R --noconfirm nvidia-580xx-dkms")
	}
	if err := restoreKeplerNouveau(); err != nil {
		return failRes(i18n.T("removed 580xx but could not restore Nouveau: %v"), err).
			withFix("sudo rm /etc/modprobe.d/nvidia.conf /etc/mkinitcpio.conf.d/nvidia.conf")
	}
	if err := rebuildKeplerNouveau(); err != nil {
		return warnRes(i18n.T("restored Nouveau, but the initramfs rebuild failed: %v"), err).
			withFix(i18n.T("sudo limine-mkinitcpio  (or: sudo mkinitcpio -P)"))
	}
	return fixedRes(i18n.T("removed unsupported 580xx from Kepler hardware and restored Nouveau for the next boot"))
}

// nvidiaDriverActive: does this box use the proprietary/open nvidia driver?
// a loaded module is the clearest tell, but the bug we repair is exactly
// that nouveau won the boot race, so the module may NOT be loaded -- fall
// back to "an nvidia kernel-module package is installed". nvidia-utils
// (userspace) alone is excluded: with no module to load, writing
// MODULES=(nvidia ...) would only break the initramfs.
func nvidiaDriverActive() bool {
	return nvidiaDriverActiveFor(keplerGpuPresent(), nvidiaDriverPackagePresent(), gpuDriversLoaded())
}

func nvidiaDriverActiveFor(kepler, packagePresent bool, loaded []string) bool {
	if kepler && !packagePresent {
		return false
	}
	for _, m := range loaded {
		if m == "nvidia" {
			return true
		}
	}
	return packagePresent
}

func nvidiaDriverPackagePresent() bool {
	return anyPkgInstalled("nvidia-open-dkms", "nvidia-dkms", "nvidia-open", "nvidia", "nvidia-lts", "nvidia-open-lts", "nvidia-470xx-dkms", "nvidia-580xx-dkms")
}

// nvidiaConfigOK: do the modprobe + mkinitcpio drop-ins already carry the
// reliability essentials (nouveau blacklisted, DRM modeset on, nvidia
// modules in the initramfs)? pure, so the idempotency that keeps doctor
// quiet on a healthy box -- and stops it rebuilding the initramfs every
// run -- is unit-testable.
func nvidiaConfigOK(modprobe, mkinit string) bool {
	return strings.Contains(modprobe, "blacklist nouveau") &&
		strings.Contains(modprobe, "nvidia_drm modeset=1 fbdev=1") &&
		strings.Contains(mkinit, "nvidia_drm")
}

// nvidiaModuleOnDisk: modinfo finds the nvidia module for any installed
// kernel tree, the same probe nvidia.sh gates on at install time.
func nvidiaModuleOnDisk() bool {
	dirs, _ := filepath.Glob("/usr/lib/modules/*")
	for _, d := range dirs {
		kv := filepath.Base(d)
		if exec.Command("modinfo", "-k", kv, "nvidia").Run() == nil {
			return true
		}
	}
	return false
}

// removeRootFiles deletes root-owned files through sudo; absent files are fine.
func removeRootFiles(paths ...string) error {
	args := append([]string{"rm", "-f"}, paths...)
	return sys.Sudo(args...)
}

func reconcileNvidiaModeset(checkOnly bool) recResult {
	if !nvidiaDriverActive() {
		return okRes(i18n.T("no proprietary NVIDIA driver in use"))
	}
	// nouveau blacklisted with no loadable nvidia module = no driver can bind
	// the card (the SDDM login loop). Restore nouveau so the next boot has a
	// display; installing the matching driver is then an ordinary fix.
	blacklist := strings.Contains(readFileSafe("/etc/modprobe.d/nvidia.conf"), "blacklist nouveau")
	if blacklist && !nvidiaModuleOnDisk() {
		if checkOnly {
			return wouldRes(i18n.T("nouveau is blacklisted but no nvidia module exists for any installed kernel; the session cannot start (the SDDM login loop)")).
				withFix(i18n.T("ryoku doctor  (restores nouveau, rebuilds the initramfs)"))
		}
		if err := removeRootFiles("/etc/modprobe.d/nvidia.conf", "/etc/mkinitcpio.conf.d/nvidia.conf"); err != nil {
			return failRes(i18n.T("could not remove the stale NVIDIA config: %v"), err).
				withFix("sudo rm /etc/modprobe.d/nvidia.conf /etc/mkinitcpio.conf.d/nvidia.conf && sudo mkinitcpio -P")
		}
		if err := rebuildInitramfs(); err != nil {
			return warnRes(i18n.T("restored nouveau, but the initramfs rebuild failed: %v"), err).
				withFix(i18n.T("sudo limine-mkinitcpio  (or: sudo mkinitcpio -P)"))
		}
		return fixedRes(i18n.T("no nvidia module exists for the installed kernel(s); restored nouveau and rebuilt the initramfs so the next boot has a display. Install a matching driver (pacman -Syu nvidia-open) and run ryoku doctor again to switch back"))
	}
	modprobe := readFileSafe("/etc/modprobe.d/nvidia.conf")
	mkinit := readFileSafe("/etc/mkinitcpio.conf.d/nvidia.conf")
	ok := nvidiaConfigOK(modprobe, mkinit)
	if ok {
		return okRes(i18n.T("NVIDIA modeset + fbdev + nouveau blacklist in place"))
	}
	if checkOnly {
		return wouldRes(i18n.T("NVIDIA driver in use but nouveau is not blacklisted / DRM modeset + fbdev not set; the GPU or an external display can fail to come up on some boots")).
			withFix(i18n.T("ryoku doctor  (writes /etc/modprobe.d/nvidia.conf and rebuilds the initramfs)"))
	}
	if err := writeRootFile("/etc/modprobe.d/nvidia.conf", nvidiaModprobeConf, "0644"); err != nil {
		return failRes(i18n.T("could not write /etc/modprobe.d/nvidia.conf: %v"), err).
			withFix(i18n.T("re-run with sudo access"))
	}
	if err := writeRootFile("/etc/mkinitcpio.conf.d/nvidia.conf", nvidiaMkinitcpioConf, "0644"); err != nil {
		return failRes(i18n.T("could not write /etc/mkinitcpio.conf.d/nvidia.conf: %v"), err).
			withFix(i18n.T("re-run with sudo access"))
	}
	if err := rebuildInitramfs(); err != nil {
		return warnRes(i18n.T("wrote the NVIDIA reliability config, but the initramfs rebuild failed: %v"), err).
			withFix(i18n.T("sudo limine-mkinitcpio  (or: sudo mkinitcpio -P)"))
	}
	return fixedRes(i18n.T("blacklisted nouveau, enabled NVIDIA DRM modeset, and rebuilt the initramfs"))
}

// rebuildInitramfs regenerates the boot image after a module/blacklist
// change. limine-mkinitcpio when present (the UKI path Ryoku uses), else
// plain mkinitcpio -P.
func rebuildInitramfs() error {
	if _, err := exec.LookPath("limine-mkinitcpio"); err == nil {
		return sys.Run("sudo", "limine-mkinitcpio")
	}
	return sys.Run("sudo", "mkinitcpio", "-P")
}

// ---- reconciler: NVIDIA update guard hook ------------------------------------

// nvidiaGuardHookPath / nvidiaGuardHook: the pacman hook that runs
// ryoku-nvidia-guard after every kernel or NVIDIA-driver transaction. Mirrors
// system/hardware/drivers/nvidia.sh verbatim so a doctored box matches a fresh
// install. Its job is the SDDM login loop: a -dkms module that fails to rebuild
// on a kernel update leaves nouveau blacklisted with no nvidia module, so no
// driver binds the card and the Wayland session cannot start. `ryoku update`
// heals that after the fact, but a plain `pacman -Syu` never runs doctor -- this
// hook makes the guard run in that transaction instead.
const nvidiaGuardHookPath = "/etc/pacman.d/hooks/ryoku-nvidia.hook"

const nvidiaGuardHook = `[Trigger]
Operation=Install
Operation=Upgrade
Operation=Remove
Type=Path
Target=usr/lib/modules/*/vmlinuz

[Trigger]
Operation=Install
Operation=Upgrade
Operation=Remove
Type=Package
Target=nvidia*
Target=lib32-nvidia*
Target=libva-nvidia-driver
Target=linux-*-nvidia*

[Action]
Description=Ryoku: reconciling NVIDIA modules and the initramfs (guards the login loop)...
When=PostTransaction
NeedsTargets
Exec=/usr/bin/ryoku-nvidia-guard
`

// nvidiaGuardHookOK: is the installed hook already the canonical content? pure,
// so the idempotency (doctor stays quiet on a healthy box) is unit-testable.
// Trailing-whitespace tolerant: readFileSafe returns an error string when the
// file is absent, which never matches.
func nvidiaGuardHookOK(got string) bool {
	return strings.TrimSpace(got) == strings.TrimSpace(nvidiaGuardHook)
}

func reconcileNvidiaGuardHook(checkOnly bool) recResult {
	if !nvidiaDriverActive() {
		return okRes(i18n.T("no proprietary NVIDIA driver in use"))
	}
	if nvidiaGuardHookOK(readFileSafe(nvidiaGuardHookPath)) {
		return okRes(i18n.T("NVIDIA update guard hook in place"))
	}
	if checkOnly {
		return wouldRes(i18n.T("the NVIDIA update guard pacman hook is missing or stale; a failed DKMS rebuild on a kernel update could strand the box at the SDDM login (the login loop)")).
			withFix(i18n.T("ryoku doctor  (installs /etc/pacman.d/hooks/ryoku-nvidia.hook)"))
	}
	if err := writeRootFile(nvidiaGuardHookPath, nvidiaGuardHook, "0644"); err != nil {
		return failRes(i18n.T("could not write %s: %v"), nvidiaGuardHookPath, err).
			withFix(i18n.T("re-run with sudo access"))
	}
	return fixedRes(i18n.T("installed the NVIDIA update guard hook so a failed DKMS rebuild can't strand the login"))
}

// ---- reconciler: NVIDIA sleep units ------------------------------------------

// nvidiaSleepUnits are the systemd sleep hooks nvidia-utils ships and the
// installer enables (system/hardware/drivers/nvidia.sh). They save and restore
// the GPU's VRAM across suspend/hibernate; with the driver's default
// NVreg_UseKernelSuspendNotifiers=1 they exit early (the kernel PM chain does
// the work), but boxes that turn the notifier off depend on them. A box
// installed before the installer's enable step, or converted from another
// distro, carries them disabled: drift from the shipped contract, repaired here.
var nvidiaSleepUnits = []string{
	"nvidia-suspend.service",
	"nvidia-hibernate.service",
	"nvidia-resume.service",
}

const nvidiaUnitDir = "/usr/lib/systemd/system"

// planNvidiaSleepUnits decides from observed state: which of the units exist
// and which are enabled. Pure, so the healthy-box silence and the
// partial-enable repair are testable without a live systemd.
func planNvidiaSleepUnits(nvidiaActive bool, exists map[string]bool, enabled map[string]bool) (missing []string, verdict string) {
	if !nvidiaActive {
		return nil, "no proprietary NVIDIA driver in use"
	}
	for _, u := range nvidiaSleepUnits {
		if !exists[u] {
			// nvidia-utils not installed (or a mesa-only box): nothing to enable.
			return nil, "the NVIDIA sleep units are not installed on this machine"
		}
	}
	for _, u := range nvidiaSleepUnits {
		if !enabled[u] {
			missing = append(missing, u)
		}
	}
	if len(missing) == 0 {
		return nil, "the NVIDIA sleep units are enabled"
	}
	return missing, ""
}

func reconcileNvidiaSleepUnits(checkOnly bool) recResult {
	exists := map[string]bool{}
	enabled := map[string]bool{}
	for _, u := range nvidiaSleepUnits {
		exists[u] = sys.Exists(nvidiaUnitDir + "/" + u)
		enabled[u] = sys.UnitEnabled(u)
	}
	missing, verdict := planNvidiaSleepUnits(nvidiaDriverActive(), exists, enabled)
	if verdict != "" {
		return okRes(i18n.T("%s"), verdict)
	}
	if checkOnly {
		return wouldRes(i18n.T("the NVIDIA sleep units (%s) are disabled, so VRAM is not saved across suspend the way the installer sets up; a box with the kernel suspend notifier off wakes to a corrupted session"), strings.Join(missing, ", ")).
			withFix(i18n.T("ryoku doctor  (enables them with sudo systemctl)"))
	}
	args := append([]string{"systemctl", "enable"}, missing...)
	if err := sys.Sudo(args...); err != nil {
		return failRes(i18n.T("could not enable %s: %v"), strings.Join(missing, ", "), err).
			withFix(i18n.T("sudo systemctl enable %s"), strings.Join(missing, " "))
	}
	return fixedRes(i18n.T("enabled the NVIDIA sleep units so VRAM is preserved across suspend, matching the installer's contract"))
}
