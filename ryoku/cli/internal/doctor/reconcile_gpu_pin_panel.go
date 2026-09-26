package doctor

import (
	"os"
	"path/filepath"
	"strings"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// ---- reconciler: NVIDIA-first render pin on an iGPU-panel laptop -------------
//
// ryoku-gpu pins the strongest GPU leftmost in AQ_DRM_DEVICES, which on a
// hybrid laptop means Hyprland renders on the dGPU and scans out through the
// iGPU's panel (reverse PRIME). On some kernel/driver combinations the very
// first cross-GPU commit of a session fails once ("amdgpu: Not enough memory
// for command submission"), leaving a live compositor with a black panel until
// reboot (#270). Clearing the pin makes it disappear, so the pin is part of the
// recipe; it is also deliberate Ryoku policy on every multi-GPU machine, so
// this reconciler never touches it. It names the combination and the two ways
// out, so a black-panel report carries its own suspect.

// drmRoot is the sysfs DRM tree the panel and driver reads use; a var so the
// detection is unit-tested against a fixture.
var drmRoot = "/sys/class/drm"

// pinFirstDriver resolves the leftmost entry of the effective AQ_DRM_DEVICES
// pin to its kernel driver, "" when there is no pin or it cannot be resolved.
// The verdict comes from ryoku-gpu itself (the same tool the stale-pin
// reconciler trusts), so this never re-implements the pin policy.
var pinFirstDriver = func() string {
	out, err := sys.RunOut("ryoku-gpu", "order")
	if err != nil {
		return ""
	}
	for _, entry := range strings.FieldsFunc(strings.TrimSpace(out), func(r rune) bool { return r == ':' || r == '\n' }) {
		if card := drmCardOf(entry); card != "" {
			return cardDriver(card)
		}
	}
	return ""
}

// drmCardOf resolves one AQ_DRM_DEVICES entry (/dev/dri/cardN, or a colon-safe
// by-path name) to its cardN basename.
func drmCardOf(entry string) string {
	base := filepath.Base(strings.TrimSpace(entry))
	if strings.HasPrefix(base, "card") {
		return base
	}
	if target, err := filepath.EvalSymlinks(entry); err == nil {
		if base = filepath.Base(target); strings.HasPrefix(base, "card") {
			return base
		}
	}
	return ""
}

// cardDriver is the kernel driver bound to a DRM card, "" when unreadable.
func cardDriver(card string) string {
	target, err := os.Readlink(filepath.Join(drmRoot, card, "device", "driver"))
	if err != nil {
		return ""
	}
	return filepath.Base(target)
}

// panelDriver is the driver of the card carrying a connected internal panel
// (eDP/LVDS/DSI), "" on a desktop or when no panel is connected.
func panelDriver() string {
	cards, _ := filepath.Glob(filepath.Join(drmRoot, "card[0-9]*"))
	for _, card := range cards {
		conns, _ := filepath.Glob(card + "-*")
		for _, conn := range conns {
			base := filepath.Base(conn)
			if !strings.Contains(base, "-eDP-") && !strings.Contains(base, "-LVDS-") && !strings.Contains(base, "-DSI-") {
				continue
			}
			if b, err := os.ReadFile(filepath.Join(conn, "status")); err != nil || strings.TrimSpace(string(b)) != "connected" {
				continue
			}
			return cardDriver(filepath.Base(card))
		}
	}
	return ""
}

// renderPinPanelRisk is the pure verdict: a laptop whose connected panel sits
// on a non-NVIDIA card while the render pin puts NVIDIA first is the #270
// combination. Everything else (desktops, NVIDIA-driven panels, no pin, an
// iGPU-first pin) carries no known first-commit hazard.
func renderPinPanelRisk(laptop bool, panelDriver, pinFirst string) bool {
	return laptop && panelDriver != "" && pinFirst == "nvidia" && panelDriver != "nvidia"
}

func reconcileRenderPinPanel(checkOnly bool) recResult {
	pin := pinFirstDriver()
	panel := panelDriver()
	if !renderPinPanelRisk(isLaptop(), panel, pin) {
		return okRes(i18n.T("no NVIDIA-first render pin over a non-NVIDIA panel"))
	}
	_ = checkOnly // report-only: the pin is deliberate policy, never auto-changed
	return noteRes(i18n.T("render pin puts NVIDIA first while the panel is driven by %s (reverse PRIME); on some kernels the session's first cross-GPU commit fails once and the panel stays black until reboot (#270)"), panel).
		withFix(i18n.T("if a boot or wake ever lands on a black panel: `ryoku-gpu disable` clears the pin (Hyprland then picks the iGPU itself); `ryoku-gpu persist` restores the pin"))
}
