package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Machine is what the store knows about this computer's headroom: its memory,
// rounded up to whole GiB because MemTotal reads a little under what is
// installed (the kernel and any integrated-GPU carveout take their share), and
// the strongest kind of GPU it has. Modest marks a machine with under 8 GB or no
// discrete GPU, where the store notes that heavy subshells and bar styles are not
// recommended and lists the lighter ones first. It only advises: nothing is ever
// refused because of it.
type Machine struct {
	MemoryGB int `json:"memoryGB"`
	// Graphics is discrete (an internal discrete card or an eGPU), integrated,
	// none (no GPU that can render), or unknown (the DRM tree could not be read).
	Graphics string `json:"graphics"`
	Modest   bool   `json:"modest"`
}

const (
	graphicsDiscrete   = "discrete"
	graphicsIntegrated = "integrated"
	graphicsNone       = "none"
	graphicsUnknown    = "unknown"

	modestMemoryGB = 8
)

// The probe reads two kernel interfaces and never spawns a process: asking
// nvidia-smi would wake a runtime-suspended discrete GPU just to list products.
// Tests point these at fixture trees; the DRM root honours the same override as
// system/hardware/gpu/ryoku-gpu-detect.
var (
	machineMeminfoPath = "/proc/meminfo"
	machineDRMRoot     = envOr("RYOKU_GPU_DRM_ROOT", "/sys/class/drm")
)

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func probeMachine() Machine {
	m := Machine{
		MemoryGB: readMemoryGB(machineMeminfoPath),
		Graphics: detectGraphics(machineDRMRoot),
	}
	m.Modest = (m.MemoryGB > 0 && m.MemoryGB < modestMemoryGB) ||
		m.Graphics == graphicsIntegrated || m.Graphics == graphicsNone
	return m
}

// readMemoryGB returns MemTotal rounded up to whole GiB, or 0 when meminfo
// cannot be read, which never marks the machine modest on its own.
func readMemoryGB(path string) int {
	file, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 || fields[0] != "MemTotal:" {
			continue
		}
		kib, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil || kib == 0 {
			return 0
		}
		const kibPerGiB = 1024 * 1024
		return int((kib + kibPerGiB - 1) / kibPerGiB)
	}
	return 0
}

// detectGraphics walks the DRM cards the way ryoku-gpu-detect's gpu_records
// does (real cards with a render node and a PCI slot, connector subdevices
// skipped) and classifies each with the same rule as its classify. The machine
// has discrete graphics when any card is discrete or an eGPU.
func detectGraphics(root string) string {
	cards, err := filepath.Glob(filepath.Join(root, "card[0-9]*"))
	if err != nil {
		return graphicsUnknown
	}
	if _, err := os.Stat(root); err != nil {
		return graphicsUnknown
	}
	found := graphicsNone
	for _, card := range cards {
		if strings.Contains(filepath.Base(card), "-") {
			continue
		}
		device := filepath.Join(card, "device")
		if _, err := os.Stat(device); err != nil {
			continue
		}
		if nodes, _ := filepath.Glob(filepath.Join(device, "drm", "renderD[0-9]*")); len(nodes) == 0 {
			continue
		}
		uevent := readUevent(filepath.Join(device, "uevent"))
		if uevent["PCI_SLOT_NAME"] == "" {
			continue
		}
		class := classifyGPU(uevent["DRIVER"],
			readSysUint(filepath.Join(device, "mem_info_vram_total")),
			readSysUint(filepath.Join(device, "mem_info_vis_vram_total")),
			readSysText(filepath.Join(device, "removable")))
		if class == "discrete" || class == "egpu" {
			return graphicsDiscrete
		}
		found = graphicsIntegrated
	}
	return found
}

// classifyGPU is ryoku-gpu-detect's classify: a removable card is an eGPU;
// NVIDIA is always discrete; an AMD or Intel APU's whole VRAM is a CPU-visible
// carveout of at most 8 GiB; otherwise 2 GiB or more of VRAM is discrete.
// gpu_classify_test.go runs the shell classifier on the same fixtures.
func classifyGPU(driver string, vram, visibleVRAM uint64, removable string) string {
	const (
		discreteVRAMMin = 2 << 30
		apuUMACeil      = 8 << 30
	)
	if removable == "removable" {
		return "egpu"
	}
	switch driver {
	case "nvidia", "nouveau":
		return "discrete"
	}
	if visibleVRAM > 0 && visibleVRAM == vram && vram <= apuUMACeil {
		return "integrated"
	}
	if vram >= discreteVRAMMin {
		return "discrete"
	}
	return "integrated"
}

func readUevent(path string) map[string]string {
	out := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok {
			if _, seen := out[key]; !seen {
				out[key] = value
			}
		}
	}
	return out
}

func readSysText(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func readSysUint(path string) uint64 {
	value, err := strconv.ParseUint(readSysText(path), 10, 64)
	if err != nil {
		return 0
	}
	return value
}
