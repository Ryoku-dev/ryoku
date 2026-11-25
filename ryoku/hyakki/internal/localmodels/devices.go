package localmodels

import (
	"bufio"
	"regexp"
	"strconv"
	"strings"

	"ryoku-rashin/internal/protocol"
)

// deviceLine is one line of llama-server --list-devices:
// "  Vulkan0: NVIDIA GeForce RTX 4060 Laptop GPU (8188 MiB, 5944 MiB free)".
var deviceLine = regexp.MustCompile(`^\s*([A-Za-z][A-Za-z0-9_]*):\s+(.+?)\s+\((\d+) MiB, (\d+) MiB free\)\s*$`)

// parseDevices reads --list-devices output; lines it does not know are
// skipped.
func parseDevices(out string) []protocol.LocalDevice {
	var devices []protocol.LocalDevice
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		m := deviceLine.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		total, _ := strconv.Atoi(m[3])
		free, _ := strconv.Atoi(m[4])
		devices = append(devices, protocol.LocalDevice{ID: m[1], Name: m[2], TotalMiB: total, FreeMiB: free})
	}
	return devices
}

// integrated is only a fallback when PCI identity is unavailable.
func integrated(name string) bool {
	n := strings.ToLower(name)
	if strings.Contains(n, "nvidia") || strings.Contains(n, "geforce") ||
		strings.Contains(n, "intel") && strings.Contains(n, "arc") ||
		strings.Contains(n, "radeon rx") || strings.Contains(n, "radeon pro") {
		return false
	}
	return strings.Contains(n, "intel") || strings.Contains(n, "radeon") || strings.Contains(n, "radv")
}

// deviceNone selects the processor only.
const deviceNone = "none"

func deviceFor(setting string, devices []protocol.LocalDevice, e Entry) string {
	return planModel(e, ggufMetadata{Params: e.Params, Context: e.Context, Tools: e.Tools}, hostHardware(devices), protocol.LocalModelOverride{Device: setting}).Device
}

func modelsMax(entries []Entry, devices []protocol.LocalDevice) int {
	if len(entries) < 2 {
		return 1
	}
	h := hostHardware(devices)
	plans := make([]protocol.LocalPlan, len(entries))
	for i, e := range entries {
		plans[i] = planModel(e, ggufMetadata{Params: e.Params, Context: e.Context}, h, protocol.LocalModelOverride{})
	}
	for i := range plans {
		for j := i + 1; j < len(plans); j++ {
			a, b := plans[i], plans[j]
			if a.Verdict == protocol.LocalVerdictTooBig || b.Verdict == protocol.LocalVerdictTooBig {
				return 1
			}
			cpuRoom := h.AvailableMiB - max(4096, h.RAMMiB/4)
			if a.Kind != protocol.LocalDeviceDiscrete && b.Kind != protocol.LocalDeviceDiscrete &&
				a.EstimatedMiB+b.EstimatedMiB > cpuRoom {
				return 1
			}
			if a.Device != b.Device {
				continue
			}
			room := cpuRoom
			for _, device := range h.Devices {
				if device.ID == a.Device {
					room = device.FreeMiB - max(768, device.FreeMiB/10)
					if device.Shared {
						room = min(room, cpuRoom)
					}
					break
				}
			}
			if a.EstimatedMiB+b.EstimatedMiB > room {
				return 1
			}
		}
	}
	return 2
}
