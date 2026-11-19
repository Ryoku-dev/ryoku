package localmodels

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestHardwareProfilesAndModelPlans(t *testing.T) {
	const gib = int64(1 << 30)
	card, shared, cpu, tooBig := protocol.LocalVerdictCard, protocol.LocalVerdictShared, protocol.LocalVerdictCPU, protocol.LocalVerdictTooBig
	profiles := []struct {
		name     string
		hardware hardwareProfile
		selected [6]string
		context  [6]int
		offload  [6]int
		verdict  [6]string
	}{
		{"hybrid", hardwareProfile{RAMMiB: 32768, AvailableMiB: 27000, Cores: 8, Devices: []hardwareDevice{
			{LocalDevice: protocol.LocalDevice{ID: "Vulkan0", Name: "NVIDIA RTX 4060", TotalMiB: 8192, FreeMiB: 7000}, Kind: protocol.LocalDeviceDiscrete},
			{LocalDevice: protocol.LocalDevice{ID: "Vulkan1", Name: "AMD 780M", TotalMiB: 26000, FreeMiB: 25000}, Kind: protocol.LocalDeviceIntegrated, Shared: true},
		}}, [6]string{"Vulkan0", "Vulkan0", "Vulkan0", "Vulkan0", "Vulkan0", "Vulkan0"},
			[6]int{32768, 32768, 32768, 32768, 512, 32768}, [6]int{24, 14, 16, 5, 12, 24},
			[6]string{card, protocol.LocalVerdictSplit, protocol.LocalVerdictSplit, protocol.LocalVerdictSplit, protocol.LocalVerdictUnsuitable, card}},
		{"shared16", hardwareProfile{RAMMiB: 16384, AvailableMiB: 14000, Cores: 8, Devices: []hardwareDevice{
			{LocalDevice: protocol.LocalDevice{ID: "Vulkan0", Name: "AMD 780M", TotalMiB: 12000, FreeMiB: 11500}, Kind: protocol.LocalDeviceIntegrated, Shared: true},
		}}, [6]string{"Vulkan0", "Vulkan0", "none", "none", "Vulkan0", "Vulkan0"},
			[6]int{32768, 32768, 32768, 8192, 512, 32768}, [6]int{24, 32, 0, 0, 12, 24},
			[6]string{shared, shared, cpu, tooBig, protocol.LocalVerdictUnsuitable, shared}},
		{"cpu8", hardwareProfile{RAMMiB: 8192, AvailableMiB: 6900, Cores: 4}, [6]string{"none", "none", "none", "none", "none", "none"},
			[6]int{32768, 16384, 8192, 8192, 512, 32768}, [6]int{0, 0, 0, 0, 0, 0},
			[6]string{cpu, cpu, tooBig, tooBig, protocol.LocalVerdictUnsuitable, cpu}},
		{"discrete24", hardwareProfile{RAMMiB: 65536, AvailableMiB: 53000, Cores: 16, Devices: []hardwareDevice{
			{LocalDevice: protocol.LocalDevice{ID: "Vulkan0", Name: "NVIDIA RTX 4090", TotalMiB: 24576, FreeMiB: 24000}, Kind: protocol.LocalDeviceDiscrete},
		}}, [6]string{"Vulkan0", "Vulkan0", "Vulkan0", "Vulkan0", "Vulkan0", "Vulkan0"},
			[6]int{32768, 32768, 32768, 32768, 512, 32768}, [6]int{24, 32, 40, 44, 12, 24},
			[6]string{card, card, card, protocol.LocalVerdictSplit, protocol.LocalVerdictUnsuitable, card}},
		{"arc16", hardwareProfile{RAMMiB: 32768, AvailableMiB: 27000, Cores: 12, Devices: []hardwareDevice{
			{LocalDevice: protocol.LocalDevice{ID: "Vulkan0", Name: "Intel Arc A770", TotalMiB: 16384, FreeMiB: 15000}, Kind: protocol.LocalDeviceDiscrete},
		}}, [6]string{"Vulkan0", "Vulkan0", "Vulkan0", "Vulkan0", "Vulkan0", "Vulkan0"},
			[6]int{32768, 32768, 32768, 32768, 512, 32768}, [6]int{24, 32, 40, 17, 12, 24},
			[6]string{card, card, card, protocol.LocalVerdictSplit, protocol.LocalVerdictUnsuitable, card}},
	}
	models := []struct {
		name          string
		params, bytes int64
		trained       int
		blocks        int
	}{
		{"2B", 2e9, 1200 * gib / 1024, 32768, 24},
		{"7B", 7e9, 4200 * gib / 1024, 32768, 32},
		{"14B", 14e9, 8000 * gib / 1024, 65536, 40},
		{"32B", 32e9, 19000 * gib / 1024, 65536, 64},
		{"tiny512", 200e6, 120 * gib / 1024, 512, 12},
		{"long128K", 2e9, 1200 * gib / 1024, 131072, 24},
	}
	for _, hw := range profiles {
		for i, model := range models {
			t.Run(hw.name+"/"+model.name, func(t *testing.T) {
				e := Entry{Context: model.trained, Params: model.params, Tools: true, Files: []File{{Role: protocol.LocalFileModel, Size: model.bytes}}}
				meta := ggufMetadata{Params: model.params, Context: model.trained, Tools: true, Blocks: model.blocks, Embedding: 4096, Heads: 32, KVHeads: 8}
				p := planModel(e, meta, hw.hardware, protocol.LocalModelOverride{})
				if p.Device != hw.selected[i] {
					t.Fatalf("device=%s, want %s (plan=%+v)", p.Device, hw.selected[i], p)
				}
				if p.Context != hw.context[i] || p.Context > model.trained || p.Context > 65536 {
					t.Fatalf("context=%d, want %d, trained=%d (plan=%+v)", p.Context, hw.context[i], model.trained, p)
				}
				if p.GPULayers != hw.offload[i] || p.Layers != model.blocks {
					t.Fatalf("offload=%d/%d, want %d/%d (plan=%+v)", p.GPULayers, p.Layers, hw.offload[i], model.blocks, p)
				}
				if p.Verdict != hw.verdict[i] {
					t.Fatalf("verdict=%s, want %s (plan=%+v)", p.Verdict, hw.verdict[i], p)
				}
			})
		}
	}
}

func TestPartialOffloadPreservesEnoughRoomForTheCache(t *testing.T) {
	h := hardwareProfile{RAMMiB: 32768, AvailableMiB: 27000, Cores: 8,
		Devices: []hardwareDevice{{LocalDevice: protocol.LocalDevice{ID: "Vulkan0", Name: "NVIDIA", TotalMiB: 8192, FreeMiB: 7000}, Kind: protocol.LocalDeviceDiscrete}}}
	const gib = int64(1 << 30)
	e := Entry{Context: 65536, Params: 14e9, Files: []File{{Role: protocol.LocalFileModel, Size: 8000 * gib / 1024}}}
	m := ggufMetadata{Context: 65536, Params: 14e9, Blocks: 40, Embedding: 4096, Heads: 32, KVHeads: 8}
	p := planModel(e, m, h, protocol.LocalModelOverride{})
	if p.Device != "Vulkan0" || p.GPULayers < 1 || p.GPULayers >= p.Layers || p.Context != 32768 || p.Verdict != protocol.LocalVerdictSplit {
		t.Fatalf("partial GPU plan with a usable cache = %+v", p)
	}
}

func TestHardwareClassifiesPCIAndSharedFallback(t *testing.T) {
	root := t.TempDir()
	put := func(path, body string) {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	put("proc/meminfo", "MemTotal:       32768000 kB\nMemAvailable:   27000000 kB\n")
	put("proc/cpuinfo", strings.Repeat("physical id : 0\ncore id : 0\n\n", 2)+"physical id : 0\ncore id : 1\n\n")
	put("sys/bus/pci/devices/0000:01:00.0/class", "0x030000\n")
	put("sys/bus/pci/devices/0000:01:00.0/vendor", "0x10de\n")
	put("sys/bus/pci/devices/0000:01:00.0/boot_vga", "0\n")
	put("sys/bus/pci/devices/0000:05:00.0/class", "0x030000\n")
	put("sys/bus/pci/devices/0000:05:00.0/vendor", "0x1002\n")
	put("sys/bus/pci/devices/0000:05:00.0/boot_vga", "1\n")
	put("sys/bus/pci/devices/0000:05:00.0/mem_info_vram_total", "536870912\n")
	put("sys/bus/pci/devices/0000:05:00.0/mem_info_gtt_total", "17179869184\n")
	put("sys/bus/pci/devices/0000:06:00.0/class", "0x030200\n")
	put("sys/bus/pci/devices/0000:06:00.0/vendor", "0x8086\n")
	put("sys/bus/pci/devices/0000:06:00.0/product_name", "Arc A770\n")
	put("sys/bus/pci/devices/0000:07:00.0/class", "0x030000\n")
	put("sys/bus/pci/devices/0000:07:00.0/vendor", "0x1002\n")
	put("sys/bus/pci/devices/0000:07:00.0/boot_vga", "0\n")
	put("sys/bus/pci/devices/0000:07:00.0/mem_info_vram_total", "17179869184\n")
	h := scanHardware(os.DirFS(root), []protocol.LocalDevice{
		{ID: "Vulkan0", Name: "NVIDIA RTX 4060", TotalMiB: 8192, FreeMiB: 7000},
		{ID: "Vulkan1", Name: "AMD Radeon 780M", TotalMiB: 26000, FreeMiB: 25000},
		{ID: "Vulkan2", Name: "Intel Arc A770", TotalMiB: 16384, FreeMiB: 14000},
		{ID: "Vulkan3", Name: "NVIDIA RTX 5090", TotalMiB: 26000, FreeMiB: 22000},
		{ID: "Vulkan4", Name: "AMD Radeon RX 7800", TotalMiB: 16384, FreeMiB: 14000},
		{ID: "Vulkan5", Name: "Intel(R) Arc(TM) Graphics", TotalMiB: 26000, FreeMiB: 25000},
	})
	if h.Cores != 2 || h.RAMMiB != 32000 || h.Devices[0].Kind != protocol.LocalDeviceDiscrete || h.Devices[1].Kind != protocol.LocalDeviceIntegrated || h.Devices[2].Kind != protocol.LocalDeviceDiscrete || h.Devices[3].Kind != protocol.LocalDeviceDiscrete || h.Devices[4].Kind != protocol.LocalDeviceDiscrete || h.Devices[5].Kind != protocol.LocalDeviceDiscrete {
		t.Fatalf("hardware profile = %+v", h)
	}
}

func TestLongContextSelectsQ8CacheOnlyWhenItBuysNextStep(t *testing.T) {
	h := hardwareProfile{RAMMiB: 32768, AvailableMiB: 27000, Cores: 8, Devices: []hardwareDevice{{LocalDevice: protocol.LocalDevice{ID: "Vulkan0", Name: "NVIDIA", TotalMiB: 8192, FreeMiB: 5400}, Kind: protocol.LocalDeviceDiscrete}}}
	e := Entry{Params: 2e9, Context: 131072, Tools: true, Files: []File{{Role: protocol.LocalFileModel, Size: 1200 << 20}}}
	m := ggufMetadata{Params: e.Params, Context: e.Context, Tools: true, Blocks: 32, Embedding: 4096, Heads: 32, KVHeads: 8}
	p := planModel(e, m, h, protocol.LocalModelOverride{})
	if p.Context != 32768 || p.KVType != protocol.LocalKVQ8 {
		t.Fatalf("expected 32K q8 cache, got %+v", p)
	}
}

func TestKnownSlidingWindowReducesOnlyLocalAttentionCache(t *testing.T) {
	m := ggufMetadata{Architecture: "gemma3", Blocks: 36, Embedding: 4096, Heads: 32, KVHeads: 8, SlidingWindow: 1024}
	full := kvBytesPerToken(m) * 65536
	got := kvBytesForContext(m, 65536)
	if got >= full || got <= full/6 {
		t.Fatalf("mixed sliding/full cache=%d, full=%d", got, full)
	}
	m.Architecture = "unknown"
	if got := kvBytesForContext(m, 65536); got != full {
		t.Fatalf("unknown attention pattern underestimates cache: %d != %d", got, full)
	}
}
