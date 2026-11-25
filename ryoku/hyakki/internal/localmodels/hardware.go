package localmodels

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"ryoku-rashin/internal/protocol"
)

// hardwareProfile is a snapshot: a router start rechecks free memory, never reuses a previous plan.
type hardwareProfile struct {
	Devices      []hardwareDevice
	RAMMiB       int
	AvailableMiB int
	Cores        int
}

type hardwareDevice struct {
	protocol.LocalDevice
	Kind   string
	Shared bool
}

func hostHardware(devices []protocol.LocalDevice) hardwareProfile {
	return readHardware(os.DirFS("/"), devices)
}

// readHardware takes a filesystem rooted at /; tests supply a simulated proc and sys.
func readHardware(root fs.FS, devices []protocol.LocalDevice) hardwareProfile {
	h := hardwareProfile{Cores: 1}
	if data, err := fs.ReadFile(root, "proc/meminfo"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			n, _ := strconv.Atoi(fields[1])
			switch fields[0] {
			case "MemTotal:":
				h.RAMMiB = n / 1024
			case "MemAvailable:":
				h.AvailableMiB = n / 1024
			}
		}
	}
	data, _ := fs.ReadFile(root, "proc/cpuinfo")
	physical := map[string]bool{}
	var packageID, coreID string
	for _, line := range append(strings.Split(string(data), "\n"), "") {
		if strings.TrimSpace(line) == "" {
			if coreID != "" {
				physical[packageID+"/"+coreID] = true
			}
			packageID, coreID = "", ""
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "physical id":
			packageID = strings.TrimSpace(value)
		case "core id":
			coreID = strings.TrimSpace(value)
		}
	}
	if len(physical) > 0 {
		h.Cores = len(physical)
	}
	if len(physical) == 0 {
		cpus, _ := fs.Glob(root, "sys/devices/system/cpu/cpu[0-9]*")
		for _, cpu := range cpus {
			pkg := strings.TrimSpace(readHardwareFile(root, cpu+"/topology/physical_package_id"))
			core := strings.TrimSpace(readHardwareFile(root, cpu+"/topology/core_id"))
			if core != "" {
				physical[pkg+"/"+core] = true
			}
		}
		if len(physical) > 0 {
			h.Cores = len(physical)
		}
	}
	if h.Cores < 1 {
		h.Cores = 1
	}
	paths, _ := fs.Glob(root, "sys/bus/pci/devices/*")
	type pci struct {
		name   string
		kind   string
		total  int64
		shared int64
		vendor string
	}
	var cards []pci
	for _, base := range paths {
		class := strings.TrimSpace(readHardwareFile(root, base+"/class"))
		if !strings.HasPrefix(class, "0x0300") && !strings.HasPrefix(class, "0x0302") {
			continue
		}
		vendor := strings.TrimSpace(readHardwareFile(root, base+"/vendor"))
		driver := ""
		if link, err := fs.ReadLink(root, base+"/driver"); err == nil {
			driver = filepath.Base(link)
		}
		if driver == "" {
			driver = strings.TrimSpace(readHardwareFile(root, base+"/driver/module/name"))
		}
		name := strings.ToLower(strings.TrimSpace(readHardwareFile(root, base+"/product_name")))
		total, _ := strconv.ParseInt(strings.TrimSpace(readHardwareFile(root, base+"/mem_info_vram_total")), 10, 64)
		shared, _ := strconv.ParseInt(strings.TrimSpace(readHardwareFile(root, base+"/mem_info_gtt_total")), 10, 64)
		boot := strings.TrimSpace(readHardwareFile(root, base+"/boot_vga")) == "1"
		kind := "discrete"
		switch {
		case vendor == "0x10de" || driver == "nvidia":
			kind = "discrete"
		case vendor == "0x1002" || driver == "amdgpu":
			if total == 0 || shared > total*2 || boot && total <= 2<<30 {
				kind = "integrated"
			}
		case vendor == "0x8086":
			if !strings.Contains(name, "arc") && !strings.Contains(name, "dg2") && (boot || total == 0) {
				kind = "integrated"
			}
		}
		cards = append(cards, pci{name, kind, total, shared, vendor})
	}
	for _, d := range devices {
		kind := "discrete"
		matched := false
		lower := strings.ToLower(d.Name)
		for _, card := range cards {
			if card.vendor == "0x10de" && (strings.Contains(lower, "nvidia") || strings.Contains(lower, "geforce")) || card.vendor == "0x1002" && (strings.Contains(lower, "amd") || strings.Contains(lower, "radeon") || strings.Contains(lower, "radv")) || card.vendor == "0x8086" && strings.Contains(lower, "intel") {
				// When several cards share a vendor, match product strings first, then by VRAM size.
				if card.total > 0 && abs64(card.total/(1<<20)-int64(d.TotalMiB)) > 2048 {
					continue
				}
				if card.total == 0 && card.vendor == "0x1002" && (card.kind == "integrated") != integrated(d.Name) {
					continue
				}
				if card.name != "" && !strings.Contains(lower, card.name) && card.total == 0 {
					continue
				}
				kind, matched = card.kind, true
				break
			}
		}
		if strings.Contains(lower, "intel") && strings.Contains(lower, "arc") {
			kind = protocol.LocalDeviceDiscrete
		}
		if !matched {
			if integrated(d.Name) {
				kind = "integrated"
			}
		}
		if !matched && h.RAMMiB > 0 && d.TotalMiB >= h.RAMMiB/2 && !strings.Contains(lower, "nvidia") && !strings.Contains(lower, "geforce") && !(strings.Contains(lower, "intel") && strings.Contains(lower, "arc")) {
			kind = "integrated"
		}
		h.Devices = append(h.Devices, hardwareDevice{LocalDevice: d, Kind: kind, Shared: kind == "integrated"})
	}
	return h
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func readHardwareFile(root fs.FS, name string) string {
	data, _ := fs.ReadFile(root, name)
	return string(data)
}

// scanHardware is used for devices independently of any runtime process.
func scanHardware(root fs.FS, devices []protocol.LocalDevice) hardwareProfile {
	return readHardware(root, devices)
}
