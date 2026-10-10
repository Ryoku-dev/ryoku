package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"ryoku-cli/internal/host"
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
			withFix(host.Default().InstallAdvice("brightnessctl"))
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
// backlight and fell back to the often-broken ACPI/EC interface. Void does not
// require a journal, so read the kernel ring buffer when journalctl is absent.
var kernelLogOutput = func() string {
	if sys.Has("journalctl") {
		return captureOut("journalctl", "-k", "-b", "--no-pager")
	}
	return captureOut("dmesg")
}

func nvidiaBacklightDead() bool {
	return nvidiaBacklightDeadFrom(kernelLogOutput())
}

func nvidiaBacklightDeadFrom(log string) bool {
	return strings.Contains(strings.ToLower(log), "no nvidia native backlight")
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

const nvidiaModprobeConf = `options nvidia_drm modeset=1 fbdev=1
options nvidia NVreg_PreserveVideoMemoryAllocations=1
blacklist nouveau
options nouveau modeset=0
`

const (
	nvidiaMkinitcpioConf       = "MODULES=(nvidia nvidia_modeset nvidia_uvm nvidia_drm)\n"
	nvidiaDracutConf           = "force_drivers+=\" nvidia nvidia_modeset nvidia_drm \"\nadd_drivers+=\" nvidia_uvm \"\n"
	nvidiaVoidSafeModprobeConf = `# Ryoku keeps this file so it shadows the NVIDIA package default.
# Nouveau remains available until DKMS produces a loadable nvidia module.
`
	nvidiaVoidSafeDracutConf = "# Ryoku leaves NVIDIA out of the initramfs until its kernel module exists.\n"
)

var (
	nvidiaPackageManager = func() host.PackageManager {
		manager, err := host.Default().PackageManager()
		if err != nil {
			return host.Pacman
		}
		return manager
	}
	nvidiaInitSystem = func() host.InitSystem {
		init, err := host.Default().Init()
		if err != nil {
			return host.Systemd
		}
		return init
	}
	nvidiaPackageInstalled = func(name string) bool {
		if nvidiaPackageManager() == host.XBPS {
			return host.Default().Package([]string{"installed", name}) == host.ExitOK
		}
		return sys.PkgInstalled(name)
	}
	nvidiaRemovePackage = func(name string) error {
		if nvidiaPackageManager() == host.XBPS {
			if code := host.Default().Package([]string{"remove", name}); code != host.ExitOK {
				return fmt.Errorf("package removal exited %d", code)
			}
			return nil
		}
		return sys.Sudo(kepler580RemovalArgs()...)
	}
	nvidiaPCIOutput = func() string {
		out, err := exec.Command("lspci").Output()
		if err != nil {
			return ""
		}
		return string(out)
	}
	nvidia580Installed   = func() bool { return nvidiaPackageInstalled(nvidia580Package()) }
	removeKepler580      = func() error { return nvidiaRemovePackage(nvidia580Package()) }
	restoreKeplerNouveau = restoreNouveauConfig
	rebuildKeplerNouveau = rebuildInitramfs
	xbpsPackageList      = func() string { return captureOut("xbps-query", "-l") }
)

func nvidia580Package() string {
	if nvidiaPackageManager() == host.XBPS {
		return "nvidia580"
	}
	return "nvidia-580xx-dkms"
}

func kepler580RemovalArgs() []string {
	return []string{"pacman", "-R", "--noconfirm", "nvidia-580xx-dkms"}
}

func keplerGpuPresent() bool {
	pci := strings.ToLower(nvidiaPCIOutput())
	return strings.Contains(pci, "nvidia") && strings.Contains(pci, "gk")
}

func reconcileKeplerNvidia(checkOnly bool) recResult {
	pkg := nvidia580Package()
	if !keplerGpuPresent() || !nvidia580Installed() {
		return okRes(i18n.T("no incompatible 580xx driver on Kepler hardware"))
	}
	if checkOnly {
		return wouldRes(i18n.T("Kepler hardware has %s, which cannot bind this GPU and leaves Nouveau blacklisted"), pkg).
			withFix(i18n.T("ryoku doctor  (removes 580xx and restores Nouveau)"))
	}
	if err := removeKepler580(); err != nil {
		remedy := "sudo pacman -R --noconfirm nvidia-580xx-dkms"
		if nvidiaPackageManager() == host.XBPS {
			remedy = "sudo xbps-remove -y nvidia580"
		}
		return failRes(i18n.T("could not remove incompatible %s: %v"), pkg, err).withFix(remedy)
	}
	if err := restoreKeplerNouveau(); err != nil {
		if nvidiaPackageManager() != host.XBPS {
			return failRes(i18n.T("removed 580xx but could not restore Nouveau: %v"), err).
				withFix("sudo rm /etc/modprobe.d/nvidia.conf /etc/mkinitcpio.conf.d/nvidia.conf")
		}
		modprobe, initramfs := nvidiaConfigPaths()
		return failRes(i18n.T("removed 580xx but could not restore Nouveau: %v"), err).
			withFix(i18n.T("restore Nouveau in %s and %s"), modprobe, initramfs)
	}
	if err := rebuildKeplerNouveau(); err != nil {
		return warnRes(i18n.T("restored Nouveau, but the initramfs rebuild failed: %v"), err).
			withFix(nvidiaInitramfsAdvice())
	}
	return fixedRes(i18n.T("removed unsupported 580xx from Kepler hardware and restored Nouveau for the next boot"))
}

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
	if nvidiaPackageManager() == host.XBPS {
		return anyNvidiaPackageInstalled("nvidia", "nvidia580", "nvidia470")
	}
	return anyPkgInstalled("nvidia-open-dkms", "nvidia-dkms", "nvidia-open", "nvidia", "nvidia-lts", "nvidia-open-lts", "nvidia-470xx-dkms", "nvidia-580xx-dkms")
}

func anyNvidiaPackageInstalled(names ...string) bool {
	for _, name := range names {
		if nvidiaPackageInstalled(name) {
			return true
		}
	}
	return false
}

func nvidiaConfigPaths() (string, string) {
	if nvidiaPackageManager() == host.XBPS {
		return "/etc/modprobe.d/nvidia.conf", "/etc/dracut.conf.d/nvidia.conf"
	}
	return "/etc/modprobe.d/nvidia.conf", "/etc/mkinitcpio.conf.d/nvidia.conf"
}

func nvidiaVoidModprobeConf(preserve bool) string {
	out := "options nvidia_drm modeset=1 fbdev=1\n"
	if preserve {
		out += "options nvidia NVreg_PreserveVideoMemoryAllocations=1\n"
	}
	return out + "blacklist nouveau\noptions nouveau modeset=0\nblacklist nova_core\nblacklist nova_drm\n"
}

func nvidiaHostModprobeConf() string {
	if nvidiaPackageManager() != host.XBPS {
		return nvidiaModprobeConf
	}
	return nvidiaVoidModprobeConf(anyNvidiaPackageInstalled("nvidia", "nvidia580"))
}

func nvidiaConfigOK(modprobe, mkinit string) bool {
	return nvidiaConfigOKFor(host.Pacman, modprobe, mkinit)
}

func nvidiaConfigOKFor(manager host.PackageManager, modprobe, initramfs string) bool {
	if !strings.Contains(modprobe, "blacklist nouveau") ||
		!strings.Contains(modprobe, "nvidia_drm modeset=1 fbdev=1") {
		return false
	}
	if manager == host.XBPS {
		return strings.Contains(modprobe, "blacklist nova_core") &&
			strings.Contains(initramfs, "force_drivers+=") &&
			strings.Contains(initramfs, "nvidia_drm") &&
			strings.Contains(initramfs, "add_drivers+=") &&
			strings.Contains(initramfs, "nvidia_uvm")
	}
	return strings.Contains(initramfs, "nvidia_drm")
}

func nvidiaModuleOnDisk() bool {
	dirs, _ := filepath.Glob("/usr/lib/modules/*")
	requireEveryKernel := nvidiaPackageManager() == host.XBPS
	found := false
	for _, d := range dirs {
		kv := filepath.Base(d)
		if exec.Command("modinfo", "-k", kv, "nvidia").Run() == nil {
			found = true
			if !requireEveryKernel {
				return true
			}
			continue
		}
		if requireEveryKernel {
			return false
		}
	}
	return found
}

func removeRootFiles(paths ...string) error {
	args := append([]string{"rm", "-f"}, paths...)
	return sys.Sudo(args...)
}

func restoreNouveauConfig() error {
	modprobe, initramfs := nvidiaConfigPaths()
	if nvidiaPackageManager() != host.XBPS {
		return removeRootFiles(modprobe, initramfs)
	}
	if err := writeRootFile(modprobe, nvidiaVoidSafeModprobeConf, "0644"); err != nil {
		return err
	}
	return writeRootFile(initramfs, nvidiaVoidSafeDracutConf, "0644")
}

func reconcileNvidiaModeset(checkOnly bool) recResult {
	if !nvidiaDriverActive() {
		return okRes(i18n.T("no proprietary NVIDIA driver in use"))
	}
	manager := nvidiaPackageManager()
	modprobePath, initramfsPath := nvidiaConfigPaths()
	modprobe := readFileSafe(modprobePath)
	blacklist := strings.Contains(modprobe, "blacklist nouveau")
	moduleMissing := !nvidiaModuleOnDisk()
	if moduleMissing && (blacklist || manager == host.XBPS) {
		if checkOnly {
			return wouldRes(i18n.T("nouveau is blacklisted but no nvidia module exists for any installed kernel; the session cannot start (the SDDM login loop)")).
				withFix(i18n.T("ryoku doctor  (restores nouveau, rebuilds the initramfs)"))
		}
		if err := restoreNouveauConfig(); err != nil {
			if manager != host.XBPS {
				return failRes(i18n.T("could not remove the stale NVIDIA config: %v"), err).
					withFix("sudo rm /etc/modprobe.d/nvidia.conf /etc/mkinitcpio.conf.d/nvidia.conf && sudo mkinitcpio -P")
			}
			return failRes(i18n.T("could not restore the safe NVIDIA config: %v"), err).
				withFix(i18n.T("restore Nouveau in %s and %s"), modprobePath, initramfsPath)
		}
		if err := rebuildInitramfs(); err != nil {
			return warnRes(i18n.T("restored Nouveau, but the initramfs rebuild failed: %v"), err).
				withFix(nvidiaInitramfsAdvice())
		}
		detail := i18n.T("no nvidia module exists for the installed kernel(s); restored nouveau and rebuilt the initramfs so the next boot has a display. Install a matching driver (pacman -Syu nvidia-open) and run ryoku doctor again to switch back")
		if manager == host.XBPS {
			detail = fmt.Sprintf(i18n.T("no nvidia module exists for the installed kernel(s); restored nouveau and rebuilt the initramfs so the next boot has a display. %s, then run ryoku doctor again"), host.Default().InstallAdvice(nvidiaRecommendedVoidPackage()))
		}
		return fixedRes("%s", detail)
	}
	initramfs := readFileSafe(initramfsPath)
	if nvidiaConfigOKFor(manager, modprobe, initramfs) {
		return okRes(i18n.T("NVIDIA modeset + fbdev + nouveau blacklist in place"))
	}
	if checkOnly {
		return wouldRes(i18n.T("NVIDIA driver in use but nouveau is not blacklisted / DRM modeset + fbdev not set; the GPU or an external display can fail to come up on some boots")).
			withFix(i18n.T("ryoku doctor  (writes /etc/modprobe.d/nvidia.conf and rebuilds the initramfs)"))
	}
	if err := writeRootFile(modprobePath, nvidiaHostModprobeConf(), "0644"); err != nil {
		return failRes(i18n.T("could not write %s: %v"), modprobePath, err).
			withFix(i18n.T("re-run with sudo access"))
	}
	initramfsConf := nvidiaMkinitcpioConf
	if manager == host.XBPS {
		initramfsConf = nvidiaDracutConf
	}
	if err := writeRootFile(initramfsPath, initramfsConf, "0644"); err != nil {
		return failRes(i18n.T("could not write %s: %v"), initramfsPath, err).
			withFix(i18n.T("re-run with sudo access"))
	}
	if err := rebuildInitramfs(); err != nil {
		return warnRes(i18n.T("wrote the NVIDIA reliability config, but the initramfs rebuild failed: %v"), err).
			withFix(nvidiaInitramfsAdvice())
	}
	return fixedRes(i18n.T("blacklisted nouveau, enabled NVIDIA DRM modeset, and rebuilt the initramfs"))
}

var voidKernelPackage = regexp.MustCompile(`^linux[0-9]+\.[0-9]+$`)

func voidKernelSeriesFrom(packages string) []string {
	seen := map[string]bool{}
	var out []string
	for _, line := range strings.Split(packages, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "ii" {
			continue
		}
		name := fields[1]
		if dash := strings.IndexByte(name, '-'); dash >= 0 {
			name = name[:dash]
		}
		if voidKernelPackage.MatchString(name) && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

func rebuildInitramfs() error {
	if nvidiaPackageManager() == host.XBPS {
		series := voidKernelSeriesFrom(xbpsPackageList())
		if len(series) == 0 {
			return fmt.Errorf("no installed Void kernel series found")
		}
		for _, kernel := range series {
			if err := sys.Run("sudo", "xbps-reconfigure", "-f", kernel); err != nil {
				return err
			}
		}
		return nil
	}
	if _, err := exec.LookPath("limine-mkinitcpio"); err == nil {
		return sys.Run("sudo", "limine-mkinitcpio")
	}
	return sys.Run("sudo", "mkinitcpio", "-P")
}

func nvidiaInitramfsAdvice() string {
	if nvidiaPackageManager() == host.XBPS {
		return i18n.T("sudo xbps-reconfigure -fa")
	}
	return i18n.T("sudo limine-mkinitcpio  (or: sudo mkinitcpio -P)")
}

func nvidiaRecommendedVoidPackage() string {
	if keplerGpuPresent() {
		return "nvidia470"
	}
	pci := strings.ToLower(nvidiaPCIOutput())
	if strings.Contains(pci, "gm") || strings.Contains(pci, "gp") || strings.Contains(pci, "gv") {
		return "nvidia580"
	}
	return "nvidia"
}

// ---- reconciler: NVIDIA update guard hook ------------------------------------

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

const (
	voidNvidiaGuardHookPath = "/etc/kernel.d/post-install/15-ryoku-nvidia"
	voidNvidiaGuardHook     = `#!/bin/sh
# Runs after 10-dkms and before 20-initramfs. The latter builds the image from
# the policy this hook writes, so a failed DKMS build cannot hide nouveau.

set -eu

kernel=${2:-}
[ -n "$kernel" ] || exit 0

modprobe_conf=${RYOKU_MODPROBE_CONF:-/etc/modprobe.d/nvidia.conf}
dracut_conf=${RYOKU_DRACUT_CONF:-/etc/dracut.conf.d/nvidia.conf}
mkdir -p "$(dirname "$modprobe_conf")" "$(dirname "$dracut_conf")"
modules_dir=${RYOKU_MODULES_DIR:-/usr/lib/modules}

modules_ready() {
  modinfo -k "$kernel" nvidia >/dev/null 2>&1 || return 1
  for tree in "$modules_dir"/*; do
    [ -d "$tree" ] || continue
    version=${tree##*/}
    modinfo -k "$version" nvidia >/dev/null 2>&1 || return 1
  done
}

preserve=0
if xbps-query nvidia >/dev/null 2>&1 || xbps-query nvidia580 >/dev/null 2>&1; then
  preserve=1
fi

if modules_ready; then
  {
    printf '%s\n' 'options nvidia_drm modeset=1 fbdev=1'
    [ "$preserve" -eq 1 ] && printf '%s\n' 'options nvidia NVreg_PreserveVideoMemoryAllocations=1'
    printf '%s\n' 'blacklist nouveau' 'options nouveau modeset=0' 'blacklist nova_core' 'blacklist nova_drm'
  } >"$modprobe_conf"
  cat >"$dracut_conf" <<'EOF'
force_drivers+=" nvidia nvidia_modeset nvidia_drm "
add_drivers+=" nvidia_uvm "
EOF
else
  cat >"$modprobe_conf" <<'EOF'
# Ryoku keeps this file so it shadows the NVIDIA package default.
# Nouveau remains available until DKMS produces a loadable nvidia module.
EOF
  cat >"$dracut_conf" <<'EOF'
# Ryoku leaves NVIDIA out of the initramfs until its kernel module exists.
EOF
  logger -t ryoku-nvidia "nvidia module missing for $kernel after DKMS; leaving nouveau available" || true
fi
`
)

func nvidiaGuardHookOK(got string) bool {
	return strings.TrimSpace(got) == strings.TrimSpace(nvidiaGuardHook)
}

func nvidiaGuardHookOKFor(manager host.PackageManager, got string, executable bool) bool {
	if manager == host.XBPS {
		return executable && strings.TrimSpace(got) == strings.TrimSpace(voidNvidiaGuardHook)
	}
	return nvidiaGuardHookOK(got)
}

func nvidiaGuardSpec() (path, content, mode string, executable bool) {
	if nvidiaPackageManager() == host.XBPS {
		return voidNvidiaGuardHookPath, voidNvidiaGuardHook, "0755", true
	}
	return nvidiaGuardHookPath, nvidiaGuardHook, "0644", false
}

func reconcileNvidiaGuardHook(checkOnly bool) recResult {
	if !nvidiaDriverActive() {
		return okRes(i18n.T("no proprietary NVIDIA driver in use"))
	}
	manager := nvidiaPackageManager()
	path, content, mode, needsExec := nvidiaGuardSpec()
	executable := true
	if needsExec {
		info, err := os.Stat(path)
		executable = err == nil && info.Mode()&0o111 != 0
	}
	if nvidiaGuardHookOKFor(manager, readFileSafe(path), executable) {
		return okRes(i18n.T("NVIDIA update guard hook in place"))
	}
	if checkOnly {
		if manager == host.XBPS {
			return wouldRes(i18n.T("the NVIDIA post-install kernel hook is missing, stale, or not executable; a failed DKMS rebuild could leave nouveau blacklisted for the new kernel")).
				withFix(i18n.T("ryoku doctor  (installs /etc/kernel.d/post-install/15-ryoku-nvidia)"))
		}
		return wouldRes(i18n.T("the NVIDIA update guard pacman hook is missing or stale; a failed DKMS rebuild on a kernel update could strand the box at the SDDM login (the login loop)")).
			withFix(i18n.T("ryoku doctor  (installs /etc/pacman.d/hooks/ryoku-nvidia.hook)"))
	}
	if err := writeRootFile(path, content, mode); err != nil {
		return failRes(i18n.T("could not write %s: %v"), path, err).
			withFix(i18n.T("re-run with sudo access"))
	}
	return fixedRes(i18n.T("installed the NVIDIA update guard hook so a failed DKMS rebuild can't strand the login"))
}

// ---- reconciler: NVIDIA sleep integration ------------------------------------

var nvidiaSleepUnits = []string{
	"nvidia-suspend.service",
	"nvidia-hibernate.service",
	"nvidia-resume.service",
}

const (
	nvidiaUnitDir          = "/usr/lib/systemd/system"
	nvidiaElogindSleepHook = "/usr/libexec/elogind/system-sleep/nvidia.sh"
)

func planNvidiaSleepUnits(nvidiaActive bool, exists map[string]bool, enabled map[string]bool) (missing []string, verdict string) {
	if !nvidiaActive {
		return nil, "no proprietary NVIDIA driver in use"
	}
	for _, u := range nvidiaSleepUnits {
		if !exists[u] {
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

func planNvidiaElogindHook(nvidiaActive, branchShipsHook, hookExists bool) (missing bool, verdict string) {
	if !nvidiaActive {
		return false, "no proprietary NVIDIA driver in use"
	}
	if !branchShipsHook {
		return false, "the installed NVIDIA branch uses the kernel suspend path and does not ship an elogind sleep hook"
	}
	if hookExists {
		return false, "the NVIDIA elogind sleep hook is installed"
	}
	return true, ""
}

func reconcileNvidiaSleepUnits(checkOnly bool) recResult {
	if nvidiaInitSystem() == host.Runit {
		active := nvidiaDriverActive()
		shipsHook := anyNvidiaPackageInstalled("nvidia", "nvidia580")
		missing, verdict := planNvidiaElogindHook(active, shipsHook, sys.Exists(nvidiaElogindSleepHook))
		if verdict != "" {
			return okRes(i18n.T("%s"), verdict)
		}
		if missing {
			pkg := "nvidia"
			if nvidiaPackageInstalled("nvidia580") {
				pkg = "nvidia580"
			}
			return warnRes(i18n.T("the NVIDIA elogind sleep hook is missing, so VRAM may not survive suspend")).
				withFix(host.Default().InstallAdvice(pkg))
		}
	}

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
