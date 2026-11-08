package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const gib = uint64(1) << 30

type fixtureCard struct {
	name, driver, slot string
	vram, visibleVRAM  uint64
	removable          bool
	render             bool
}

func writeDRMTree(t *testing.T, cards ...fixtureCard) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "drm")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, card := range cards {
		device := filepath.Join(root, card.name, "device")
		write := func(name, body string) {
			path := filepath.Join(device, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		write("uevent", "DRIVER="+card.driver+"\nPCI_CLASS=30000\nPCI_SLOT_NAME="+card.slot+"\n")
		if card.vram > 0 {
			write("mem_info_vram_total", strconv.FormatUint(card.vram, 10)+"\n")
		}
		if card.visibleVRAM > 0 {
			write("mem_info_vis_vram_total", strconv.FormatUint(card.visibleVRAM, 10)+"\n")
		}
		if card.removable {
			write("removable", "removable\n")
		} else {
			write("removable", "fixed\n")
		}
		if card.render {
			if err := os.MkdirAll(filepath.Join(device, "drm", "renderD128"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		connector := filepath.Join(root, card.name+"-eDP-1")
		if err := os.MkdirAll(connector, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(connector, "status"), []byte("connected\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func writeMeminfo(t *testing.T, memTotalKiB string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "meminfo")
	body := "MemTotal:       " + memTotalKiB + " kB\nMemFree:         1048576 kB\nMemAvailable:    2097152 kB\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func useMachine(t *testing.T, meminfo, drm string) {
	t.Helper()
	oldMeminfo, oldDRM := machineMeminfoPath, machineDRMRoot
	machineMeminfoPath, machineDRMRoot = meminfo, drm
	t.Cleanup(func() { machineMeminfoPath, machineDRMRoot = oldMeminfo, oldDRM })
}

var (
	apu       = fixtureCard{name: "card1", driver: "amdgpu", slot: "0000:c1:00.0", vram: 512 << 20, visibleVRAM: 512 << 20, render: true}
	bigAPU    = fixtureCard{name: "card1", driver: "amdgpu", slot: "0000:c1:00.0", vram: 4 * gib, visibleVRAM: 4 * gib, render: true}
	intel     = fixtureCard{name: "card0", driver: "i915", slot: "0000:00:02.0", render: true}
	nvidia    = fixtureCard{name: "card0", driver: "nvidia", slot: "0000:01:00.0", render: true}
	radeonDGP = fixtureCard{name: "card0", driver: "amdgpu", slot: "0000:03:00.0", vram: 8 * gib, visibleVRAM: 256 << 20, render: true}
	eGPU      = fixtureCard{name: "card2", driver: "amdgpu", slot: "0000:3a:00.0", vram: 512 << 20, visibleVRAM: 512 << 20, removable: true, render: true}
	kmsOnly   = fixtureCard{name: "card0", driver: "simpledrm", slot: "0000:00:01.0"}
)

func TestDetectGraphicsClassifiesDRMTrees(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cards []fixtureCard
		want  string
	}{
		{"AMD APU only", []fixtureCard{apu}, graphicsIntegrated},
		{"4 GiB UMA APU stays integrated", []fixtureCard{bigAPU}, graphicsIntegrated},
		{"Intel iGPU without a VRAM node", []fixtureCard{intel}, graphicsIntegrated},
		{"hybrid APU plus NVIDIA", []fixtureCard{nvidia, apu}, graphicsDiscrete},
		{"desktop Radeon with dedicated VRAM", []fixtureCard{radeonDGP}, graphicsDiscrete},
		{"APU laptop with an eGPU", []fixtureCard{apu, eGPU}, graphicsDiscrete},
		{"KMS-only framebuffer cannot render", []fixtureCard{kmsOnly}, graphicsNone},
		{"no cards at all", nil, graphicsNone},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := detectGraphics(writeDRMTree(t, tc.cards...)); got != tc.want {
				t.Fatalf("detectGraphics = %q, want %q", got, tc.want)
			}
		})
	}
	if got := detectGraphics(filepath.Join(t.TempDir(), "absent")); got != graphicsUnknown {
		t.Fatalf("unreadable DRM root = %q, want %q", got, graphicsUnknown)
	}
}

func TestReadMemoryGBRoundsUpToInstalled(t *testing.T) {
	for _, tc := range []struct {
		memTotal string
		want     int
	}{
		{"16303420", 16}, // 16 GB with the kernel's share taken
		{"7990000", 8},   // 8 GB reads a little under 8 GiB
		{"5950000", 6},   // 8 GB with a 2 GiB iGPU carveout leaves 6 to the system
		{"3900000", 4},
		{"garbage", 0},
	} {
		if got := readMemoryGB(writeMeminfo(t, tc.memTotal)); got != tc.want {
			t.Errorf("MemTotal %s kB = %d GB, want %d", tc.memTotal, got, tc.want)
		}
	}
	if got := readMemoryGB(filepath.Join(t.TempDir(), "absent")); got != 0 {
		t.Fatalf("unreadable meminfo = %d, want 0", got)
	}
}

func TestProbeMachineModest(t *testing.T) {
	for _, tc := range []struct {
		name     string
		memTotal string
		cards    []fixtureCard
		want     Machine
	}{
		{"16 GB with a discrete GPU has headroom", "16303420", []fixtureCard{nvidia, apu},
			Machine{MemoryGB: 16, Graphics: graphicsDiscrete}},
		{"8 GB with a discrete GPU has headroom", "7990000", []fixtureCard{radeonDGP},
			Machine{MemoryGB: 8, Graphics: graphicsDiscrete}},
		{"16 GB on integrated graphics only is modest", "16303420", []fixtureCard{apu},
			Machine{MemoryGB: 16, Graphics: graphicsIntegrated, Modest: true}},
		{"6 GB with a discrete GPU is modest", "5950000", []fixtureCard{nvidia},
			Machine{MemoryGB: 6, Graphics: graphicsDiscrete, Modest: true}},
		{"no render-capable GPU is modest", "16303420", []fixtureCard{kmsOnly},
			Machine{MemoryGB: 16, Graphics: graphicsNone, Modest: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			useMachine(t, writeMeminfo(t, tc.memTotal), writeDRMTree(t, tc.cards...))
			if got := probeMachine(); got != tc.want {
				t.Fatalf("probeMachine = %+v, want %+v", got, tc.want)
			}
		})
	}
	useMachine(t, filepath.Join(t.TempDir(), "absent"), filepath.Join(t.TempDir(), "absent"))
	if got := probeMachine(); got != (Machine{Graphics: graphicsUnknown}) {
		t.Fatalf("unreadable machine = %+v", got)
	}
}

func TestClassifyGPUMatchesRyokuGPUDetect(t *testing.T) {
	detector, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "system", "hardware", "gpu", "ryoku-gpu-detect"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(detector); err != nil {
		t.Skipf("ryoku-gpu-detect not in this checkout: %v", err)
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available")
	}
	cards := []fixtureCard{apu, intel, nvidia, radeonDGP, eGPU}
	for i := range cards {
		cards[i].name = "card" + strconv.Itoa(i)
		cards[i].slot = "0000:0" + strconv.Itoa(i) + ":00.0"
	}
	root := writeDRMTree(t, cards...)
	cmd := exec.Command(bash, "-c", `source "$1"; gpu_records | cut -f2,4`, "detect", detector)
	cmd.Env = append(os.Environ(), "RYOKU_GPU_DRM_ROOT="+root, "RYOKU_GPU_DRI_DIR="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ryoku-gpu-detect: %v\n%s", err, out)
	}
	shell := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		class, card, ok := strings.Cut(line, "\t")
		if ok {
			shell[card] = class
		}
	}
	if len(shell) != len(cards) {
		t.Fatalf("shell detector saw %d cards, want %d: %q", len(shell), len(cards), out)
	}
	for _, card := range cards {
		got := classifyGPU(card.driver, card.vram, card.visibleVRAM, map[bool]string{true: "removable", false: "fixed"}[card.removable])
		if got != shell[card.name] {
			t.Errorf("%s (%s): Go %q, ryoku-gpu-detect %q", card.name, card.driver, got, shell[card.name])
		}
	}
}
