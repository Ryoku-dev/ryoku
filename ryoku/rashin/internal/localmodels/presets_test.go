package localmodels

import (
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestRenderPresetsPinsRouterFlags(t *testing.T) {
	ini := renderPresets([]presetModel{{ID: "lfm3-omni-2b", Model: "/data/model.gguf", Mmproj: "/data/mmproj.gguf", Context: 32768, Device: "Vulkan0", PlanArgs: planPresetArgs(protocol.LocalPlan{Context: 32768, GPULayers: 24, KVType: protocol.LocalKVF16}, 8), Args: map[string]string{"reasoning": "off", "temp": "0", "host": "0.0.0.0", "evil": "foo\ninjected = true", "parallel": "4"}}})
	for _, want := range []string{"version = 1", "[*]", "jinja = true", "fit = on", "flash-attn = auto", "parallel = 1", "[lfm3-omni-2b]", "model = /data/model.gguf", "mmproj = /data/mmproj.gguf", "c = 32768", "device = Vulkan0", "n-gpu-layers = all", "fit-ctx = 32768", "threads = 8", "reasoning = off", "temp = 0"} {
		if !strings.Contains(ini, want) {
			t.Fatalf("missing %q in presets:\n%s", want, ini)
		}
	}
	if strings.Contains(ini, "0.0.0.0") || strings.Contains(ini, "injected") || strings.Contains(ini, "parallel = 4") {
		t.Fatalf("untrusted preset value escaped: %s", ini)
	}
}

func TestDeviceForHybridKeepsDiscreteGPU(t *testing.T) {
	devices := parseDevices("Vulkan0: NVIDIA GeForce RTX 4060 (8188 MiB, 5717 MiB free)\nVulkan1: AMD Radeon 780M Graphics (RADV PHOENIX) (26069 MiB, 25970 MiB free)\n")
	if len(devices) != 2 {
		t.Fatalf("devices = %+v", devices)
	}
	small := Entry{Class: protocol.LocalClassLight, Files: []File{{Size: 1 << 30}}}
	if got := deviceFor("", devices, small); got != "Vulkan0" {
		t.Fatalf("light model device = %s", got)
	}
	if got := deviceFor("cpu", devices, small); got != "none" {
		t.Fatalf("CPU device = %s", got)
	}
	if got := deviceFor("Vulkan0", devices, small); got != "Vulkan0" {
		t.Fatalf("chosen device = %s", got)
	}
}

func TestVersionGate(t *testing.T) {
	for _, tc := range []struct {
		text  string
		works bool
	}{
		{"version: 1.0.0-dev (build 12521, commit abc)", true},
		{"version: 6153 (old)", false},
		{"version: 0.8.0", true},
		{"version: unknown", false},
	} {
		build, version, semver, _ := parseVersion(tc.text)
		if got := (binary{Build: build, Version: version, Semver: semver}).meets(DefaultMinBuild); got != tc.works {
			t.Errorf("%q passes=%v, want=%v", tc.text, got, tc.works)
		}
	}
}
