package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// advice only, nothing is ever refused on a modest machine
type Machine struct {
	MemoryGB int `json:"memoryGB"`
	// an eGPU counts as discrete, unknown means the DRM tree couldn't be read
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

// no nvidia-smi here, it would wake a runtime-suspended dGPU just to list products
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

// rounded up since MemTotal reads under what's installed, the kernel and iGPU carveout take a share
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

// walks the cards like ryoku-gpu-detect's gpu_records so both agree on the machine
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

// ryoku-gpu-detect's classify, gpu_classify_test.go runs both on the same fixtures
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
