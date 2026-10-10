package doctor

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ryoku-cli/internal/host"
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
	"ryoku-desktop", "ryoku", "ryoku-shell", "ryoku-hub", "ryoku-blobs", "ryoku-rashin",
	"quickshell",
	"qt6-base", "qt6-declarative", "qt6-wayland",
	"pipewire", "wireplumber", "nvidia-utils", "mesa", "limine", "snapper",
}

// compositorDiagnosticPackages returns only the package stack declared by the
// provider that owns the running session.
func compositorDiagnosticPackages() []string {
	detected := wm.Detect()
	if detected.Name == "" || !detected.Live {
		return nil
	}
	packages := []string{detected.Name}
	if caps, err := wm.Open().Caps(); err == nil {
		packages = append(packages, portalFrontends[caps.PortalBackend]...)
	}
	return packages
}

var reportPackageVersion = func(name string) (string, bool) {
	var output bytes.Buffer
	app := host.New(host.Config{Stdout: &output, Stderr: io.Discard})
	if app.Package([]string{"version", name}) != host.ExitOK {
		return "", false
	}
	return strings.TrimSpace(output.String()), true
}

type diagnosticService struct {
	scope string
	name  string
}

var diagnosticServices = []diagnosticService{
	{"--system", "NetworkManager"},
	{"--system", "sddm"},
	{"--system", "bluetooth"},
	{"--user", "pipewire"},
	{"--user", "wireplumber"},
	{"--user", "xdg-desktop-portal"},
	{"--user", "ryotunesd.socket"},
	{"--user", "ryoku-shell.service"},
	{"--user", "ryogami.service"},
	{"--user", "ryoku-rashin.service"},
}

func hostStateLabel(code int) string {
	switch code {
	case host.ExitOK:
		return "yes"
	case host.ExitFalse:
		return "no"
	case host.ExitAbsent:
		return "absent"
	case host.ExitNotProvided:
		return "not provided"
	default:
		return "unknown"
	}
}

func uncommentedConfig(value string) string {
	var lines []string
	for _, line := range strings.Split(value, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return "(none)"
	}
	return strings.Join(lines, "\n")
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
	line("os-release:\n%s", uncommentedConfig(readFileSafe("/etc/os-release")))

	packageManager, packageErr := doctorPackageManager()
	initSystem, initErr := doctorInitSystem()

	section("ryoku")
	cmd("ryoku", "status")
	line("state dir:\n%s", captureOut("ls", "-la", filepath.Join(sys.Xdg("XDG_STATE_HOME", ".local/state"), "ryoku")))
	if packageErr != nil {
		line("package channel: unknown (%v)", packageErr)
	} else if packageManager == host.Pacman {
		line("[ryoku] repo configured: %v", strings.Contains(readFileSafe("/etc/pacman.conf"), "[ryoku]"))
	} else if packageManager == host.DNF {
		repository, err := host.Default().RepoURL()
		line("Ryoku RPM repository: %s", repository)
		if err != nil {
			line("Ryoku RPM repository error: %v", err)
		}
	} else {
		line("package channel: managed by %s", packageManager)
	}

	if sys.IsBtrfs("/") {
		section("storage (btrfs)")
		cmd("sudo", "-n", "btrfs", "filesystem", "usage", "/")
		cmd("sudo", "-n", "btrfs", "device", "stats", "/")
		line("/proc/swaps:\n%s", readFileSafe("/proc/swaps"))
		if supported, _ := doctorSnapshots(); supported && sys.Exists("/etc/conf.d/snapper") {
			line("/etc/conf.d/snapper values:\n%s", uncommentedConfig(readFileSafe("/etc/conf.d/snapper")))
		}
	}

	section("packages")
	if packageErr != nil {
		line("package manager: unknown (%v)", packageErr)
	} else {
		line("package manager: %s", packageManager)
	}
	seenPackages := map[string]bool{}
	for _, name := range append(append([]string{}, diagnosticPackages...), compositorDiagnosticPackages()...) {
		if seenPackages[name] {
			continue
		}
		seenPackages[name] = true
		if version, ok := reportPackageVersion(name); ok {
			line("%s %s", name, version)
		}
	}
	if orphans, err := doctorOrphans(); err != nil {
		line("orphans: unavailable (%v)", err)
	} else if len(orphans) == 0 {
		line("orphans: (none)")
	} else {
		line("orphans:\n%s", strings.Join(orphans, "\n"))
	}
	var pending []string
	switch packageManager {
	case host.XBPS:
		pending = doctorFindPendingConfig("*.new-*")
	case host.DNF:
		pending = append(doctorFindPendingConfig("*.rpmnew"), doctorFindPendingConfig("*.rpmsave")...)
	default:
		pending = doctorFindPendingConfig("*.pacnew")
	}
	if len(pending) == 0 {
		line("pending config files: (none)")
	} else {
		line("pending config files:\n%s", strings.Join(pending, "\n"))
	}
	if packageManager == host.Pacman {
		line("pacman.log (tail):\n%s", tailLines(readFileSafe("/var/log/pacman.log"), 25))
	} else if packageManager == host.XBPS && sys.Exists("/var/log/xbps.log") {
		line("xbps.log (tail):\n%s", tailLines(readFileSafe("/var/log/xbps.log"), 25))
	}

	section("services")
	if initErr != nil {
		line("service manager: unknown (%v)", initErr)
	} else {
		line("service manager: %s", initSystem)
		for _, service := range diagnosticServices {
			active := hostStateLabel(doctorService(service.scope, "is-active", service.name))
			enabled := hostStateLabel(doctorService(service.scope, "is-enabled", service.name))
			line("%s %s: active=%s enabled=%s", strings.TrimPrefix(service.scope, "--"), service.name, active, enabled)
		}
	}
	if initSystem == host.Systemd {
		line("journal errors this boot (tail):\n%s", tailLines(captureOut("journalctl", "-b", "-p", "err", "--no-pager"), 40))
	}

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
	if initSystem == host.Systemd {
		line("kernel display log (tail):\n%s", captureOut("sh", "-c", "journalctl -k -b --no-pager 2>/dev/null | grep -iE 'backlight|amdgpu|nvidia|i915|drm' | tail -30 || true"))
	} else {
		line("kernel display log (tail):\n%s", captureOut("sh", "-c", "dmesg 2>/dev/null | grep -iE 'backlight|amdgpu|nvidia|i915|drm' | tail -30 || true"))
	}

	section("gpu / compositor stability")
	// The journal path searches across boots because a crash may force a reboot.
	// On runit, where there is no journal, the report records the current kernel
	// buffer instead.
	const gpuHang = `GPU reset begin|device wedged|VRAM is lost|ring .* reset failed|NVRM: Xid|GPU HANG`
	if initSystem == host.Systemd {
		line("GPU resets/hangs (last 14 days): %s", captureOut("sh", "-c",
			"journalctl --no-pager --since '-14 days' -g '"+gpuHang+"' 2>/dev/null | grep -cE '"+gpuHang+"'"))
		line("recent GPU reset/hang lines:\n%s", tailLines(captureOut("sh", "-c",
			"journalctl --no-pager --since '-14 days' -g '"+gpuHang+"' 2>/dev/null || true"), 12))
		line("compositor/session coredumps:\n%s", captureOut("sh", "-c",
			"coredumpctl list --no-pager 2>/dev/null | grep -iE 'Hyprland|Xwayland|quickshell|aquamarine' | tail -10 || true"))
	} else {
		line("recent GPU reset/hang lines:\n%s", tailLines(captureOut("sh", "-c",
			"dmesg 2>/dev/null | grep -E '"+gpuHang+"' || true"), 12))
	}

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
