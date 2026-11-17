package doctor

import (
	"os"
	"path/filepath"
	"strings"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// A legacy or forced dGPU-first pin can black-screen an iGPU-owned panel.

// drmRoot is the sysfs DRM tree the panel and driver reads use; a var so the
// detection is unit-tested against a fixture.
var drmRoot = "/sys/class/drm"

// pinFirstDriver resolves the leftmost entry of the effective AQ_DRM_DEVICES
// pin to its kernel driver, "" when there is no pin or it cannot be resolved.
// The verdict comes from ryoku-gpu itself (the same tool the stale-pin
// reconciler trusts), so this never re-implements the pin policy. --effective
// reads the pin the compositor would actually read, straight from the pin
// file: the plain order is the policy's recommendation and refuses to speak
// exactly when a stored hybrid/passthrough choice coexists with a pin: the
// drifted and forced-pin states the #270 guard most needs to see.
var pinFirstDriver = func() string {
	out, err := sys.RunOut("ryoku-gpu", "order", "--effective")
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
	if !gpuPinSupported() {
		return okRes(i18n.T("the active window manager has no render-pin capability"))
	}
	pin := pinFirstDriver()
	panel := panelDriver()
	if !renderPinPanelRisk(isLaptop(), panel, pin) {
		return okRes(i18n.T("no NVIDIA-first render pin over a non-NVIDIA panel"))
	}
	_ = checkOnly // the GPU-pin reconciler owns repair; this check adds diagnosis
	return noteRes(i18n.T("render pin puts NVIDIA first while the panel is driven by %s (reverse PRIME); this can leave the panel black at login or after DPMS (#270)"), panel).
		withFix(i18n.T("if the panel stays black, `ryoku-gpu mode hybrid` renders on the panel's GPU; a hardware MUX set to discrete removes the cross-GPU path"))
}
