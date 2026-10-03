package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
	wm "ryoku-wm"
)

// ---- diagnostic report -------------------------------------------------------

func reportPath(override string) string {
	if override != "" {
		return override
	}
	return filepath.Join(sys.Xdg("XDG_STATE_HOME", ".local/state"), "ryoku", "doctor-report.txt")
}

func writeReport(override string, findings []finding) (string, error) {
	path := reportPath(override)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(gatherReport(findings)), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// diagnosticPackages: the desktop stack whose versions decide whether the shell
// renders and how the session behaves. `ryoku status` reports only the
// ryoku-desktop channel commit, so these per-component and third-party versions
// are otherwise absent from a report a maintainer reads.
var diagnosticPackages = []string{
	"ryoku-desktop", "ryoku", "ryoku-shell", "ryoku-hub", "ryoku-blobs", "ryoku-sidebarfx", "ryoku-rashin",
	"quickshell",
	"qt6-base", "qt6-declarative", "qt6-wayland",
	"pipewire", "wireplumber", "nvidia-utils", "mesa", "limine", "snapper",
}

// compositorDiagnosticPackages: the live compositor's own stack, so a report
// carries the versions that decide the session instead of naming another
// compositor's. With nothing live (the case a report is usually read for) both
// providers' stacks are listed, since which one was meant may be the question.
func compositorDiagnosticPackages() []string {
	byName := map[string][]string{
		"hyprland": {"hyprland", "xdg-desktop-portal-hyprland"},
		"niri":     {"niri", "xwayland-satellite", "xdg-desktop-portal-gnome"},
		"mango":    {"mangowm", "scenefx", "xdg-desktop-portal-wlr", "wlsunset"},
	}
	if name := wm.Detect().Name; byName[name] != nil {
		return byName[name]
	}
	return []string{"hyprland", "xdg-desktop-portal-hyprland", "niri", "xwayland-satellite", "xdg-desktop-portal-gnome", "mangowm", "xdg-desktop-portal-wlr", "wlsunset"}
}

// gatherReport: one self-contained text report. doctor findings, then the
// system state a maintainer needs to diagnose the unknown. safe to share --
// system state + recent error logs, no secrets.
func gatherReport(findings []finding) string {
	var b strings.Builder
	line := func(f string, a ...any) { fmt.Fprintf(&b, f+"\n", a...) }
	section := func(title string) { line("\n## %s", title) }
	cmd := func(name string, args ...string) {
		line("$ %s %s", name, strings.Join(args, " "))
		line("%s", captureOut(name, args...))
	}

	line(i18n.T("Ryoku diagnostic report"))
	line("generated: %s", time.Now().Format(time.RFC3339))
	line(i18n.T("Safe to share with the Ryoku maintainers: system state and recent error"))
	line(i18n.T("logs only, no passwords or keys. Open an issue: %s"), ryokuIssuesURL)
	line(strings.Repeat("=", 70))

	section("doctor findings")
	for _, f := range findings {
		line("  %-5s %s: %s", f.res.status.label(), f.name, f.res.detail)
		if f.res.remedy != "" {
			line("        fix: %s", f.res.remedy)
		}
	}

	section("system")
	cmd("uname", "-srvmo")
	line("os-release:\n%s", readFileSafe("/etc/os-release"))

	section("ryoku")
	cmd("ryoku", "status")
	line("state dir:\n%s", captureOut("ls", "-la", filepath.Join(sys.Xdg("XDG_STATE_HOME", ".local/state"), "ryoku")))
	line("[ryoku] repo configured: %v", strings.Contains(readFileSafe("/etc/pacman.conf"), "[ryoku]"))

	section("storage (btrfs)")
	cmd("sudo", "-n", "btrfs", "filesystem", "usage", "/")
	cmd("sudo", "-n", "btrfs", "device", "stats", "/")
	line("/proc/swaps:\n%s", readFileSafe("/proc/swaps"))
	line("/etc/conf.d/snapper:\n%s", readFileSafe("/etc/conf.d/snapper"))

	section("packages")
	cmd("pacman", append(append([]string{"-Q"}, diagnosticPackages...), compositorDiagnosticPackages()...)...)
	cmd("pacman", "-Qtdq")
	cmd("pacman", "-Dk")
	line(".pacnew files:\n%s", captureOut("find", "/etc", "-name", "*.pacnew"))
	line("pacman.log (tail):\n%s", tailLines(readFileSafe("/var/log/pacman.log"), 25))

	section("services")
	cmd("systemctl", "--failed", "--no-legend", "--plain")
	cmd("systemctl", "--user", "--failed", "--no-legend", "--plain")
	line("journal errors this boot (tail):\n%s", tailLines(captureOut("journalctl", "-b", "-p", "err", "--no-pager"), 40))

	section("desktop")
	cmd("ryoku-shell", "status")
	cmd("pgrep", "-af", "quickshell")
	for _, v := range []string{"WAYLAND_DISPLAY", "XDG_CURRENT_DESKTOP", "XDG_SESSION_TYPE"} {
		line("%s=%s", v, os.Getenv(v))
	}
	d := wm.Detect()
	line("window manager: name=%s live=%t source=%s", d.Name, d.Live, d.Source)

	section("hardware")
	bl := backlightDevices()
	if len(bl) == 0 {
		line("backlight: (none found)")
	}
	for _, d := range bl {
		base := "/sys/class/backlight/" + d
		line("backlight %s: type=%s max=%s cur=%s actual=%s", d,
			readFileSafe(base+"/type"), readFileSafe(base+"/max_brightness"),
			readFileSafe(base+"/brightness"), readFileSafe(base+"/actual_brightness"))
	}
	line("gpu drivers loaded: %s", strings.Join(gpuDriversLoaded(), ", "))
	cmd("sh", "-c", "lspci -k 2>/dev/null | grep -iA3 'vga\\|3d controller' || true")
	line("kernel cmdline: %s", readFileSafe("/proc/cmdline"))
	line("kernel display log (tail):\n%s", captureOut("sh", "-c", "journalctl -k -b --no-pager 2>/dev/null | grep -iE 'backlight|amdgpu|nvidia|i915|drm' | tail -30 || true"))

	section("gpu / compositor stability")
	// vendor-agnostic GPU-hang signatures: amdgpu (reset/wedged/VRAM lost/ring),
	// nvidia (NVRM Xid), i915 (GPU HANG). skip a bare "Xid" so an r8169 NIC's
	// "XID" line doesn't get mistaken for a GPU fault. search across boots: a
	// crash needs a reboot, so the evidence is in the previous boot, not the
	// current one.
	const gpuHang = `GPU reset begin|device wedged|VRAM is lost|ring .* reset failed|NVRM: Xid|GPU HANG`
	line("GPU resets/hangs (last 14 days): %s", captureOut("sh", "-c",
		"journalctl --no-pager --since '-14 days' -g '"+gpuHang+"' 2>/dev/null | grep -cE '"+gpuHang+"'"))
	line("recent GPU reset/hang lines:\n%s", tailLines(captureOut("sh", "-c",
		"journalctl --no-pager --since '-14 days' -g '"+gpuHang+"' 2>/dev/null || true"), 12))
	line("compositor/session coredumps:\n%s", captureOut("sh", "-c",
		"coredumpctl list --no-pager 2>/dev/null | grep -iE 'Hyprland|Xwayland|quickshell|aquamarine' | tail -10 || true"))

	return b.String()
}

// Debug prints the shareable diagnostic bundle to stdout so a bug reporter can
// pipe or paste it straight into an issue. Same read-only content as
// `doctor --report` (system state and recent error logs, no secrets); it just
// goes to stdout instead of a file. Referenced by the bug issue template.
func Debug(args []string) error {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Println(i18n.T("Usage: ryoku debug   # print a shareable diagnostic bundle for bug reports"))
			return nil
		}
	}
	fmt.Print(gatherReport(runReconcilers(true)))
	return nil
}
