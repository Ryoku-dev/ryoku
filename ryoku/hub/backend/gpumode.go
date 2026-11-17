package main

// gpumode.go: read + set which GPU Ryoku renders on. three modes:
// Hybrid (no pin), Performance (pin the dGPU), Passthrough (pin the iGPU
// alone so the dGPU is free for a VM). the actual gpu.lua write goes to
// `ryoku-gpu mode`, the single owner of AQ_DRM_DEVICES; this layer reports
// the current mode + the cost of a change (a relogin, or a reboot when a
// laptop MUX has to flip).

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var aqDevicesRe = regexp.MustCompile(`AQ_DRM_DEVICES",\s*"([^"]*)"`)

type gpuModeReport struct {
	Mode                 string `json:"mode"`
	PerformanceAvailable bool   `json:"performance_available"`
	PerformanceReason    string `json:"performance_reason"`
	PerformanceRoute     string `json:"performance_route"`
}

type gpuPolicyReport struct {
	PerformanceAvailable *bool  `json:"performance_available"`
	PerformanceReason    string `json:"performance_reason"`
	PerformanceRoute     string `json:"performance_route"`
}

var gpuPolicyStatusOutput = func() ([]byte, error) {
	return exec.Command(ryokuGpuBin(), "status", "--json").Output()
}

func gpuPerformancePolicy() gpuModeReport {
	open := gpuModeReport{PerformanceAvailable: true}
	out, err := gpuPolicyStatusOutput()
	if err != nil {
		return open
	}
	var report gpuPolicyReport
	if json.Unmarshal(out, &report) != nil || report.PerformanceAvailable == nil {
		return open
	}
	return gpuModeReport{
		PerformanceAvailable: *report.PerformanceAvailable,
		PerformanceReason:    report.PerformanceReason,
		PerformanceRoute:     report.PerformanceRoute,
	}
}

func runGpuMode(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("gpu mode needs get|set")
	}
	switch args[0] {
	case "get":
		report := gpuPerformancePolicy()
		report.Mode = currentGpuMode()
		return printJSON(report)
	case "set":
		if len(args) < 2 {
			return fmt.Errorf("gpu mode set needs hybrid|performance|passthrough")
		}
		return setGpuMode(args[1])
	default:
		return fmt.Errorf("gpu mode needs get|set")
	}
}

func setGpuMode(mode string) error {
	switch mode {
	case "hybrid", "performance", "passthrough":
	default:
		return fmt.Errorf("mode must be hybrid|performance|passthrough")
	}
	report, _ := detectCapability()
	if mode == "passthrough" && wouldStrandDisplay(report) {
		return fmt.Errorf("passthrough needs the discrete GPU free, but it is driving your screen right now. Switch the laptop to Hybrid GPU mode in the BIOS/firmware setup (often labelled GPU Mode, MUX, or Hybrid/Optimus) and reboot first, so the built-in GPU drives the display")
	}
	if out, err := exec.Command(ryokuGpuBin(), "mode", mode).CombinedOutput(); err != nil {
		return fmt.Errorf("ryoku-gpu mode %s: %v: %s", mode, err, strings.TrimSpace(string(out)))
	}
	return printJSON(map[string]string{"mode": mode, "cost": gpuModeCost(mode, report)})
}

// wouldStrandDisplay: would pinning the iGPU alone (passthrough mode) leave
// the desktop without a screen? true when the dGPU drives the display and the
// iGPU does not. on such a machine the MUX has to flip to hybrid (reboot) first.
func wouldStrandDisplay(c Capability) bool {
	if c.Passthrough == nil || !c.Passthrough.DrivesDisplay {
		return false
	}
	return c.Host == nil || !c.Host.DrivesDisplay
}

func currentGpuMode() string {
	value := readAQValue(gpuLuaPath())
	if value == "" {
		return "hybrid"
	}
	report, err := detectCapability()
	if err != nil {
		return "hybrid"
	}
	dgpu, igpu := "", ""
	if report.Passthrough != nil {
		dgpu = report.Passthrough.Slot
	}
	if report.Host != nil {
		igpu = report.Host.Slot
	}
	return classifyMode(value, dgpu, igpu)
}

// classifyMode: a written AQ_DRM_DEVICES value -> a mode, given the dGPU and
// iGPU PCI slots. the value's segments are colon-separated, colon-free device
// names (the udev symlinks or cardN nodes), primary first.
func classifyMode(value, dgpuSlot, igpuSlot string) string {
	if strings.TrimSpace(value) == "" {
		return "hybrid"
	}
	segs := strings.Split(value, ":")
	primary := segs[0]
	switch {
	case dgpuSlot != "" && strings.Contains(primary, slotToken(dgpuSlot)):
		return "performance"
	case igpuSlot != "" && len(segs) == 1 && strings.Contains(primary, slotToken(igpuSlot)):
		return "passthrough"
	default:
		return "hybrid"
	}
}

// gpuModeCost: what entering a mode costs the user right now. any pin change
// takes effect on the next Hyprland login (relogin); passthrough on a laptop
// whose dGPU drives the panel additionally needs a MUX flip (reboot).
func gpuModeCost(mode string, report Capability) string {
	if mode == "passthrough" && report.Strategy == "mux-reboot" {
		return "reboot"
	}
	return "relogin"
}

// slotToken renders a PCI slot the way the udev symlink does: 0000:01:00.0 ->
// 0000-01-00-0 (colons + the dot become hyphens).
func slotToken(slot string) string {
	return strings.NewReplacer(":", "-", ".", "-").Replace(slot)
}

func readAQValue(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if m := aqDevicesRe.FindStringSubmatch(string(b)); len(m) == 2 {
		return m[1]
	}
	return ""
}

func gpuLuaPath() string {
	if p := os.Getenv("RYOKU_GPU_CONF"); p != "" {
		return p
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "hypr", "gpu.lua")
}

func ryokuGpuBin() string {
	if b := os.Getenv("RYOKU_GPU_BIN"); b != "" {
		return b
	}
	return "ryoku-gpu"
}
