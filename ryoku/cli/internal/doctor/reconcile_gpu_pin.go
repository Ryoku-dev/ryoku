package doctor

import (
	"os/exec"
	"strings"

	i18n "ryoku-i18n"
)

// ---- reconciler: GPU render pin drift ---------------------------------------
//
// ryoku-gpu pins the strongest GPU as Hyprland's primary renderer on every
// multi-GPU machine, laptops included: GPU work on a laptop's iGPU heats the
// same die the CPU does, so video playback and compositing push the package
// past its thermal limits while a parked discrete GPU cools on the side. A
// machine installed under the old laptop-unpinned policy (or one whose pin was
// cleared by hand) keeps the missing hl.env line forever, since nothing else
// owns gpu.lua; conversely, a machine whose user explicitly chose Hybrid
// (battery) or Passthrough must never have that pin re-added under it.
//
// The verdict lives in `ryoku-gpu check-pin`, right beside the policy it
// audits, so this reconciler never re-implements laptop or GPU detection:
//   ok               pin matches the policy and the stored mode choice
//   forced           the pin carries the RYOKU_GPU_FORCE marker; user intent
//   stale-pin SLOT   policy says unpinned; `ryoku-gpu disable` clears it
//   missing-pin      policy says pinned; `ryoku-gpu persist` writes it
//
// The repair rewrites only the managed gpu.lua (through the owning tool) and
// takes effect at the next Hyprland login; doctor never restarts the session.

var (
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
			return wouldRes(i18n.T("gpu.lua carries no render pin, but ryoku-gpu policy pins the strongest GPU: desktop GPU work runs on the integrated GPU, which shares the CPU die and heats the whole package. `ryoku-gpu persist` writes the pin (takes effect on the next Hyprland login; keep the battery-first routing instead with `ryoku-gpu mode hybrid`)"))
		}
		if err := persist(); err != nil {
			return failRes(i18n.T("could not write the GPU render pin: %v (write it by hand with `ryoku-gpu persist`)"), err)
		}
		return fixedRes(i18n.T("pinned the strongest GPU as the primary renderer; the desktop leaves the integrated GPU's heat budget after the next Hyprland login (opt out with `ryoku-gpu mode hybrid`)"))
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
	verdict, err := gpuPinVerdict()
	return planGpuPin(verdict, err, checkOnly, gpuPinDisable, gpuPinPersist)
}
