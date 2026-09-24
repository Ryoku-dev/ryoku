package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	i18n "ryoku-i18n"
)

// ---- reconciler: discrete GPU idle drain -------------------------------------
//
// On an ASUS MUX laptop set to Discrete mode the internal panel is wired
// straight to the discrete GPU, so the dGPU's PCIe function can never enter
// runtime D3cold: it stays `active` for the whole session and burns ~10 W and
// ~60 C of parasitic heat while the desktop sits idle -- the single largest
// battery and idle-temperature cost on this class of machine. Routing the panel
// back through the integrated GPU (MUX -> Hybrid) lets the dGPU runtime-suspend
// when nothing uses it. That switch is a hardware display-routing change that
// needs a reboot, so this check only REPORTS it: it never writes the MUX knob,
// never flips a sysfs value, and never reboots.
//
// Two distinct failures produce the same ~10 W bill, so both are checked:
//
//  1. MUX in Discrete: the panel is wired to the dGPU, which therefore can never
//     suspend. Remedy is the MUX switch plus a reboot.
//  2. MUX already Hybrid, yet the dGPU has still never suspended because some
//     process holds its device nodes open. A Wayland compositor that enumerates
//     every DRM device does this even with no display wired to the card. Without
//     this second case a machine that switched the MUX and gained nothing would
//     report clean, which is the worst outcome: the user believes it worked and
//     keeps paying the power.
//
// Either way the check REPORTS only: it never writes the MUX knob, never flips a
// sysfs value, never kills a process, and never reboots.
//
// Silent on: a desktop, a single-GPU laptop, and -- the healthy case -- any box
// whose dGPU is suspended now or has spent time suspended since boot
// (runtime_status == suspended OR runtime_suspended_time != 0). That pair is the
// tell that the card is merely idle rather than pinned awake, so a routine
// doctor pass never nags a machine that is already saving the power.

// dgpuState is the slice of sysfs this check looks at, lifted to a value so the
// decision (planDgpuPanel) is a pure function of it, testable through the one
// gatherDgpuState seam without a real /sys, DRM, or PCI tree.
type dgpuState struct {
	laptop      bool     // machine has a battery, i.e. an internal panel
	present     bool     // a discrete GPU exists on this box
	slot        string   // the dGPU's PCI slot, "" when unknown
	panelOnDgpu bool     // every connected DRM connector belongs to the dGPU
	status      string   // the dGPU's power/runtime_status
	suspended   int64    // power/runtime_suspended_time, -1 when unreadable
	watts       float64  // measured idle draw in W, -1 when unknown
	rtd3        string   // NVIDIA's own "Runtime D3 status" line, "" when absent
	holders     []string // processes with the dGPU's device nodes open (context only)
}

// cardInfo is one DRM card's identity from its device uevent.
type cardInfo struct {
	driver string
	slot   string
}

// gatherDgpuState reads the DRM/PCI tree into a dgpuState. A var so a test can
// drive every branch through the one seam without a real /sys. It resolves the
// discrete GPU, whether the connected panel hangs off it, and (only when the
// card is pinned awake, so a suspended GPU is never woken) its live draw.
var gatherDgpuState = func() dgpuState {
	s := dgpuState{laptop: isLaptop(), suspended: -1, watts: -1}
	if !s.laptop {
		return s
	}
	cards := drmCards()
	dcard, info, ok := discreteCard(cards)
	if !ok {
		return s
	}
	s.present = true
	s.slot = info.slot
	s.panelOnDgpu = allConnectorsOnDgpu(dcard, connectedConnectorCards())
	s.status = strings.TrimSpace(readFileSafe("/sys/class/drm/" + dcard + "/device/power/runtime_status"))
	if n, err := strconv.ParseInt(strings.TrimSpace(readFileSafe("/sys/class/drm/"+dcard+"/device/power/runtime_suspended_time")), 10, 64); err == nil {
		s.suspended = n
	}
	// Probe only a card confirmed pinned awake: a suspended card must not be
	// woken just to read numbers the report would not print.
	if !dgpuCanSuspend(s.status, s.suspended) {
		s.watts = dgpuWatts(dcard, info.driver)
		s.rtd3 = nvidiaRuntimeD3(info.slot)
		s.holders = dgpuHolders(dgpuDeviceNodes(info.slot, info.driver))
	}
	return s
}

// drmCards maps each real DRM card (cardN, not its cardN-CONNECTOR subdevices)
// to its driver and PCI slot from the device uevent.
func drmCards() map[string]cardInfo {
	out := map[string]cardInfo{}
	entries, err := os.ReadDir("/sys/class/drm")
	if err != nil {
		return out
	}
	for _, e := range entries {
		name := e.Name()
		if !isCardName(name) {
			continue
		}
		driver, slot := parseUevent(readFileSafe("/sys/class/drm/" + name + "/device/uevent"))
		out[name] = cardInfo{driver: driver, slot: slot}
	}
	return out
}

// connectedConnectorCards lists, for every DRM connector currently reporting
// `connected`, the card it belongs to (cardN-eDP-1 -> cardN).
func connectedConnectorCards() []string {
	entries, err := os.ReadDir("/sys/class/drm")
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		card := connectorCard(name)
		if card == "" {
			continue
		}
		if strings.TrimSpace(readFileSafe("/sys/class/drm/"+name+"/status")) == "connected" {
			out = append(out, card)
		}
	}
	return out
}

// isCardName reports whether a DRM entry is a card node (card + digits), not a
// cardN-CONNECTOR subdevice or a renderD/controlD node. pure.
func isCardName(name string) bool {
	if !strings.HasPrefix(name, "card") {
		return false
	}
	digits := name[len("card"):]
	if digits == "" {
		return false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// connectorCard returns the card a DRM connector belongs to (cardN-eDP-1 ->
// cardN), or "" when name is a card node itself or not a connector at all. pure.
func connectorCard(name string) string {
	i := strings.IndexByte(name, '-')
	if i < 0 {
		return ""
	}
	prefix := name[:i]
	if !isCardName(prefix) {
		return ""
	}
	return prefix
}

// parseUevent pulls DRIVER and PCI_SLOT_NAME out of a DRM card's device uevent
// body. pure, so the parse is unit-testable without a real /sys.
func parseUevent(body string) (driver, slot string) {
	for _, line := range strings.Split(body, "\n") {
		switch {
		case strings.HasPrefix(line, "DRIVER="):
			driver = strings.TrimPrefix(line, "DRIVER=")
		case strings.HasPrefix(line, "PCI_SLOT_NAME="):
			slot = strings.TrimPrefix(line, "PCI_SLOT_NAME=")
		}
	}
	return driver, slot
}

// discreteCard picks the discrete GPU out of the DRM cards. NVIDIA (nvidia /
// nouveau) is the verified discrete class this check defends -- the MUX-pinned
// panel drain measured on this hardware -- so it is the only driver treated as
// discrete; an amdgpu/i915 iGPU is never mistaken for one. pure over its input.
func discreteCard(cards map[string]cardInfo) (card string, info cardInfo, ok bool) {
	for name, c := range cards {
		if c.driver == "nvidia" || c.driver == "nouveau" {
			return name, c, true
		}
	}
	return "", cardInfo{}, false
}

// allConnectorsOnDgpu reports whether every connected connector belongs to the
// discrete GPU's card, i.e. the internal panel is wired to the dGPU. False with
// no connected connector: with nothing lit we cannot conclude the panel routes
// through the dGPU. pure, so the wiring verdict is unit-testable.
func allConnectorsOnDgpu(dgpuCard string, connected []string) bool {
	if len(connected) == 0 {
		return false
	}
	for _, c := range connected {
		if c != dgpuCard {
			return false
		}
	}
	return true
}

// dgpuCanSuspend reports whether the discrete GPU is able to runtime-suspend: it
// is asleep right now, or it has spent some time asleep since boot (or that time
// could not be read). Any of these means the card is not pinned awake, so the
// drain this check warns about does not apply. pure, so the gate is testable.
func dgpuCanSuspend(status string, suspended int64) bool {
	return status == "suspended" || suspended != 0
}

// dgpuWatts reads the discrete GPU's current draw in watts, -1 when no source
// answers. Prefers the standard hwmon power1_average node (microwatts, what an
// amdgpu dGPU exposes); for NVIDIA, which has no such node, falls back to
// nvidia-smi power.draw under a hard timeout so a wedged card cannot stall
// doctor. Called only for a card already confirmed awake, so this never wakes a
// suspended GPU.
func dgpuWatts(card, driver string) float64 {
	if m, _ := filepath.Glob("/sys/class/drm/" + card + "/device/hwmon/hwmon*/power1_average"); len(m) > 0 {
		if uw, err := strconv.ParseInt(strings.TrimSpace(readFileSafe(m[0])), 10, 64); err == nil && uw > 0 {
			return float64(uw) / 1e6
		}
	}
	if driver == "nvidia" {
		if _, err := exec.LookPath("nvidia-smi"); err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			out, err := exec.CommandContext(ctx, "nvidia-smi",
				"--query-gpu=power.draw", "--format=csv,noheader,nounits").Output()
			if err == nil {
				if w, err := strconv.ParseFloat(firstLine(string(out)), 64); err == nil {
					return w
				}
			}
		}
	}
	return -1
}

// dgpuDeviceNodes lists the device files whose being held open keeps the discrete
// GPU powered up: its DRM card and render nodes, resolved through the PCI-slot
// by-path symlinks so cardN renumbering across boots cannot mislead, plus
// NVIDIA's own character devices, which are not DRM nodes but pin the card just
// the same. Matching every /dev/nvidia* is coarse on a hypothetical multi-NVIDIA
// box; on the laptops this check defends there is exactly one.
func dgpuDeviceNodes(slot, driver string) []string {
	var out []string
	if slot != "" {
		for _, suffix := range []string{"card", "render"} {
			if p, err := filepath.EvalSymlinks("/dev/dri/by-path/pci-" + slot + "-" + suffix); err == nil {
				out = append(out, p)
			}
		}
	}
	if driver == "nvidia" || driver == "nouveau" {
		nodes, _ := filepath.Glob("/dev/nvidia*")
		for _, n := range nodes {
			if st, err := os.Stat(n); err == nil && !st.IsDir() {
				out = append(out, n)
			}
		}
	}
	return out
}

// dgpuHolders names the processes holding one of those nodes open, deduplicated
// and sorted. An open fd is exactly what stops runtime PM from suspending the
// card, so naming the holder turns "the dGPU never sleeps" from a mystery into
// something the user can act on. /proc entries that vanish mid-scan, or belong to
// another user, are skipped rather than failing the whole scan.
func dgpuHolders(nodes []string) []string {
	if len(nodes) == 0 {
		return nil
	}
	want := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		want[n] = true
	}
	procs, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range procs {
		if _, err := strconv.Atoi(p.Name()); err != nil {
			continue
		}
		fds, err := os.ReadDir("/proc/" + p.Name() + "/fd")
		if err != nil {
			continue
		}
		for _, fd := range fds {
			target, err := os.Readlink("/proc/" + p.Name() + "/fd/" + fd.Name())
			if err != nil || !want[target] {
				continue
			}
			if comm := strings.TrimSpace(readFileSafe("/proc/" + p.Name() + "/comm")); comm != "" && !seen[comm] {
				seen[comm] = true
				out = append(out, comm)
			}
			break
		}
	}
	sort.Strings(out)
	return out
}

// nvidiaRuntimeD3 returns the driver's own verdict on runtime D3 for this card,
// e.g. "Enabled (fine-grained)" or "Disabled". This is authoritative where sysfs
// is only circumstantial: NVIDIA documents that in fine-grained mode the driver
// tracks actual GPU *usage*, so a merely-open fd does not keep the card up, and
// the documented hard blockers are driving a display or a running CUDA app.
// "" when the procfs entry is absent (no NVIDIA driver, or a non-NVIDIA card).
func nvidiaRuntimeD3(slot string) string {
	if slot == "" {
		return ""
	}
	return parseRuntimeD3(readFileSafe("/proc/driver/nvidia/gpus/" + slot + "/power"))
}

// parseRuntimeD3 pulls the "Runtime D3 status" value out of that procfs body.
// pure, so the parse is unit-testable without an NVIDIA driver.
func parseRuntimeD3(body string) string {
	const key = "Runtime D3 status:"
	for _, line := range strings.Split(body, "\n") {
		if _, v, ok := strings.Cut(line, key); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// rtd3Off reports whether the driver says runtime D3 is off for this card. An
// unreadable status is not treated as off: we do not invent a fault we cannot see.
func rtd3Off(v string) bool {
	return v != "" && !strings.HasPrefix(v, "Enabled")
}

// dgpuMuxFix is the one remedy: route the panel through the iGPU by switching the
// hardware MUX to hybrid, then reboot. Report-only, because it changes display
// routing and needs a reboot -- doctor must never apply it under the user's feet.
// A Performance-mode owner pays this draw on purpose: the wording says so.
func dgpuMuxFix() string {
	return i18n.T("switch the hardware MUX to hybrid so the panel routes through the iGPU and the dGPU can runtime-suspend: `ryoku-gpu-mux set hybrid`, then reboot. This is a display-routing change that needs a reboot, so doctor never applies it automatically. If you chose Performance graphics mode, this draw is the intended cost of running everything on the discrete GPU.")
}

// dgpuRtd3OffFix applies when the driver itself reports runtime D3 off. Then no
// amount of closing applications will help: the feature is switched off.
func dgpuRtd3OffFix() string {
	return i18n.T("the NVIDIA driver reports runtime D3 power management off for this card, so it can never power down. Enable fine-grained control with `NVreg_DynamicPowerManagement=0x02` in a /etc/modprobe.d/nvidia.conf `options nvidia` line, rebuild the initramfs (`sudo mkinitcpio -P`), and reboot.")
}

// dgpuHeldFix applies when the driver says runtime D3 is on yet the card has still
// never slept. NVIDIA documents the blockers as driving a display or a running
// CUDA application -- NOT merely having the device open, which fine-grained mode
// tolerates by tracking real usage. So the listed holders are leads to check, not
// a verdict, and this must not tell the user to go kill their compositor.
func dgpuHeldFix() string {
	return i18n.T("runtime D3 is enabled yet the card has never slept, so something is genuinely keeping it busy. NVIDIA documents the blockers as driving a display or a running CUDA application: check for an attached external display on the dGPU and for a CUDA/compute process. The processes listed are the ones with the card open, which is a lead rather than proof, since fine-grained mode tolerates an idle open device.")
}

// planDgpuPanel turns observed state into a result. pure, so every branch is
// unit-testable without hardware. The draw is quoted only when it was actually
// measured (watts >= 0); a machine whose draw could not be read is never handed
// a fabricated wattage.
func planDgpuPanel(s dgpuState) recResult {
	if !s.laptop {
		return okRes(i18n.T("desktop: no internal panel a discrete GPU could pin awake"))
	}
	if !s.present {
		return okRes(i18n.T("single-GPU laptop: no discrete GPU to idle"))
	}
	if !s.panelOnDgpu {
		if dgpuCanSuspend(s.status, s.suspended) {
			return okRes(i18n.T("the internal panel is driven by the integrated GPU and the discrete GPU can runtime-suspend"))
		}
		// The MUX is already right, so the panel is not what pins the card. Left
		// unreported, a box that switched the MUX and gained nothing would look
		// clean while still paying the full idle draw.
		draw := ""
		if s.watts >= 0 {
			draw = fmt.Sprintf(i18n.T(" and is drawing %.1f W"), s.watts)
		}
		if rtd3Off(s.rtd3) {
			return noteRes(i18n.T("the discrete GPU does not drive the panel yet has never runtime-suspended%s: the driver reports runtime D3 %q"), draw, s.rtd3).
				withFix(dgpuRtd3OffFix())
		}
		held := ""
		if len(s.holders) > 0 {
			held = i18n.Tf(", with the card open in %s", strings.Join(s.holders, ", "))
		}
		return noteRes(i18n.T("the discrete GPU does not drive the panel yet has never runtime-suspended%s%s"), draw, held).
			withFix(dgpuHeldFix())
	}
	if dgpuCanSuspend(s.status, s.suspended) {
		return okRes(i18n.T("the discrete GPU drives the panel but can still runtime-suspend, so it is not pinned awake"))
	}
	slot := ""
	if s.slot != "" {
		slot = " (" + s.slot + ")"
	}
	if s.watts >= 0 {
		return noteRes(i18n.T("the internal panel is wired to the discrete GPU%s, so it can never runtime-suspend and is drawing %.1f W, running hot at idle instead of powering down"), slot, s.watts).
			withFix(dgpuMuxFix())
	}
	return noteRes(i18n.T("the internal panel is wired to the discrete GPU%s, so it can never runtime-suspend and stays awake burning power and running hot at idle instead of powering down"), slot).
		withFix(dgpuMuxFix())
}

func reconcileDgpuPanel(_ bool) recResult {
	return planDgpuPanel(gatherDgpuState())
}
