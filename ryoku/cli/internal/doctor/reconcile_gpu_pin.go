package doctor

import (
	"os/exec"
	"strings"

	i18n "ryoku-i18n"
	wm "ryoku-wm"
)

// Aquamarine can lose an iGPU panel when a dGPU-first pin crosses GPUs.

var (
	gpuPinProviderInstalled = func(name string) bool {
		return wm.SessionGap(name) != "compositor"
	}
	gpuPinSupported = func() bool {
		for _, name := range wm.Providers() {
			if wm.GpuPinFile(name) != "" && gpuPinProviderInstalled(name) {
				return true
			}
		}
		return false
	}
	gpuPinVerdict = func() (string, error) {
		out, err := exec.Command("ryoku-gpu", "check-pin").Output()
		return strings.TrimSpace(string(out)), err
	}
	gpuPinDisable = func() error {
		return exec.Command("ryoku-gpu", "disable").Run()
	}
	gpuPinPersist = func() error {
		return exec.Command("ryoku-gpu", "persist").Run()
	}
)

// planGpuPin turns the tool's verdict into a result. pure over its inputs, so
// every branch is unit-testable without a real config or GPU.
func planGpuPin(verdict string, verdictErr error, checkOnly bool, disable, persist func() error) recResult {
	if verdictErr != nil {
		// No ryoku-gpu on PATH (partial install) or the probe failed: there is
		// nothing this check can safely audit, and inventing a fault helps no
		// one. The GPU tooling has its own delivery checks.
		return okRes(i18n.T("ryoku-gpu is not available to audit the render pin (%v)"), verdictErr)
	}
	switch {
	case verdict == "ok":
		return okRes(i18n.T("the Hyprland GPU render pin matches ryoku-gpu policy"))
	case verdict == "forced":
		return okRes(i18n.T("the GPU render pin is a deliberate RYOKU_GPU_FORCE override; kept"))
	case verdict == "missing-pin":
		if checkOnly {
			return wouldRes(i18n.T("gpu.lua carries no render pin, but ryoku-gpu can safely pin the strongest GPU. `ryoku-gpu persist` writes the pin (takes effect on the next Hyprland login; keep compositor selection instead with `ryoku-gpu mode hybrid`)"))
		}
		if err := persist(); err != nil {
			return failRes(i18n.T("could not write the GPU render pin: %v (write it by hand with `ryoku-gpu persist`)"), err)
		}
		return fixedRes(i18n.T("pinned the strongest panel-safe GPU as the primary renderer for the next Hyprland login (opt out with `ryoku-gpu mode hybrid`)"))
	case strings.HasPrefix(verdict, "unsafe-pin"):
		slot := strings.TrimSpace(strings.TrimPrefix(verdict, "unsafe-pin"))
		if checkOnly {
			return wouldRes(i18n.T("gpu.lua puts %s first while the connected internal panel is wired to another GPU. Aquamarine's cross-GPU scanout can leave that panel black at login or after DPMS; `ryoku-gpu disable` returns selection to the compositor on the next Hyprland login"), slot)
		}
		if err := disable(); err != nil {
			return failRes(i18n.T("could not clear the unsafe GPU render pin: %v (clear it by hand with `ryoku-gpu disable`)"), err)
		}
		return fixedRes(i18n.T("cleared the unsafe GPU render pin on %s; Hyprland will choose the panel-safe renderer on the next login"), slot)
	case strings.HasPrefix(verdict, "stale-pin"):
		slot := strings.TrimSpace(strings.TrimPrefix(verdict, "stale-pin"))
		if checkOnly {
			return wouldRes(i18n.T("gpu.lua still pins %s as the primary renderer, but this machine's stored mode says unpinned: the pin keeps the discrete GPU awake at idle. `ryoku-gpu disable` clears it (takes effect on the next Hyprland login; switch back with `ryoku-gpu mode performance`)"), slot)
		}
		if err := disable(); err != nil {
			return failRes(i18n.T("could not clear the stale GPU render pin: %v (clear it by hand with `ryoku-gpu disable`)"), err)
		}
		return fixedRes(i18n.T("cleared the GPU render pin on %s; the discrete GPU can runtime-suspend after the next Hyprland login (switch back with `ryoku-gpu mode performance`)"), slot)
	default:
		return warnRes(i18n.T("ryoku-gpu check-pin answered %q, which this ryoku version does not understand; update ryoku or run `ryoku-gpu status`"), verdict)
	}
}

func reconcileGpuPin(checkOnly bool) recResult {
	if !gpuPinSupported() {
		return okRes(i18n.T("the active window manager has no render-pin capability"))
	}
	verdict, err := gpuPinVerdict()
	return planGpuPin(verdict, err, checkOnly, gpuPinDisable, gpuPinPersist)
}
