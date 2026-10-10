package main

// hwcaps.go = the GPU-passthrough capability engine. before the Hub GPU page
// offers anything dangerous it asks two things: "can this machine hand its
// discrete GPU to a VM at all?" and "what would that cost right now (nothing,
// a relogin, or a reboot)?". this file answers both.
//
// the verdict is a pure function of gathered inputs (buildCapability), so it's
// unit-tested across hardware shapes without touching real sysfs.
// detectCapability() does the messy probing and feeds buildCapability.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Check = one row of the capability dossier the Hub shows.
type Check struct {
	ID    string `json:"id"`
	Level string `json:"level"` // ok | warn | fail
	Label string `json:"label"`
	Value string `json:"value"`
	Hint  string `json:"hint,omitempty"`
}

// GPU = a graphics device as the passthrough feature sees it.
type GPU struct {
	Slot          string   `json:"slot"` // PCI slot, e.g. 0000:01:00.0
	Model         string   `json:"model"`
	Driver        string   `json:"driver"`
	Class         string   `json:"class"` // integrated | discrete | egpu
	VramMB        int      `json:"vramMb"`
	IommuGroup    int      `json:"iommuGroup"`
	DrivesDisplay bool     `json:"drivesDisplay"`
	GroupIsolated bool     `json:"groupIsolated"`
	Functions     []string `json:"functions"` // sibling PCI funcs to pass together
}

// Capability = the whole verdict, JSON for the Hub.
type Capability struct {
	Chassis     string  `json:"chassis"` // laptop | desktop
	Mux         string  `json:"mux"`     // none | present-igpu | present-dgpu | unknown
	Cpu         string  `json:"cpu"`
	Host        *GPU    `json:"host"`        // the GPU Ryoku keeps
	Passthrough *GPU    `json:"passthrough"` // the candidate dGPU for the VM
	Enabled     bool    `json:"enabled"`     // the passthrough stack is installed
	Strategy    string  `json:"strategy"`    // live-bind | relogin-then-bind | mux-reboot | setup | none
	Verdict     string  `json:"verdict"`     // ready | needs-relogin | needs-reboot | needs-setup | incapable
	Checks      []Check `json:"checks"`
	RamTotalMB  int     `json:"ramTotalMb"`
	RamFreeMB   int     `json:"ramFreeMb"`
	VmReady     bool    `json:"vmReady"` // qemu + kvm present: a plain (non-passthrough) VM can run
	Qemu        bool    `json:"qemu"`    // qemu-desktop installed (a plain VM's only hard need)
	Kvm         bool    `json:"kvm"`     // /dev/kvm present
}

// gpuRecord = one entry from `ryoku-gpu detect --json`.
type gpuRecord struct {
	Slot      string `json:"slot"`
	Class     string `json:"class"`
	Card      string `json:"card"`
	Driver    string `json:"driver"`
	VRAM      int64  `json:"vram"`
	Connected int    `json:"connected"`
	Model     string `json:"model"`
}

// tooling: which pieces of the passthrough stack are present.
type tooling struct {
	qemu, libvirt, ovmf, swtpm, lookingGlass, kvmfr, hook bool
}

// capInputs = everything buildCapability needs, already gathered. tests build
// this directly; detectCapability() fills it from the live system.
type capInputs struct {
	records        []gpuRecord
	cpuVendor      string
	cpuVirt        bool
	kvm            bool
	iommuOn        bool
	groupOf        map[string]int   // slot -> IOMMU group number
	groupMembers   map[int][]string // IOMMU group -> member PCI slots
	chassis        string           // laptop | desktop
	ramTotalMB     int
	ramFreeMB      int
	tooling        tooling
	inLibvirtGroup bool
}

// buildCapability = the pure verdict. gathered inputs in, Capability out.
func buildCapability(in capInputs) Capability {
	c := Capability{
		Chassis:    in.chassis,
		Cpu:        in.cpuVendor,
		RamTotalMB: in.ramTotalMB,
		RamFreeMB:  in.ramFreeMB,
		Mux:        "none",
	}

	var igpu, dgpu []*GPU
	for _, r := range in.records {
		g := gpuFromRecord(r, in)
		if g.Class == "discrete" || g.Class == "egpu" {
			dgpu = append(dgpu, g)
		} else {
			igpu = append(igpu, g)
		}
	}
	if len(dgpu) > 0 {
		c.Passthrough = dgpu[0] // records are strongest-first
	}
	if len(igpu) > 0 {
		c.Host = igpu[0]
	} else if len(dgpu) > 1 {
		c.Host = dgpu[1]
	}

	if in.chassis == "laptop" && c.Passthrough != nil {
		switch {
		case c.Passthrough.DrivesDisplay:
			c.Mux = "present-dgpu"
		case c.Host != nil && c.Host.DrivesDisplay:
			c.Mux = "present-igpu"
		default:
			c.Mux = "unknown"
		}
	}

	checks, hardFail := buildChecks(in, c.Host, c.Passthrough)
	c.Checks = checks
	c.Enabled = len(toolingMissing(in.tooling)) == 0 && in.iommuOn
	c.VmReady = in.tooling.qemu && in.kvm
	c.Qemu = in.tooling.qemu
	c.Kvm = in.kvm
	c.Strategy, c.Verdict = decide(in, c.Host, c.Passthrough, hardFail)
	return c
}

func gpuFromRecord(r gpuRecord, in capInputs) *GPU {
	g := &GPU{
		Slot:          r.Slot,
		Model:         prettyModel(r.Model),
		Driver:        r.Driver,
		Class:         r.Class,
		VramMB:        int(r.VRAM / 1024 / 1024),
		DrivesDisplay: r.Connected == 1,
		IommuGroup:    -1,
	}
	if grp, ok := in.groupOf[r.Slot]; ok {
		g.IommuGroup = grp
		prefix := pciPrefix(r.Slot)
		others := false
		for _, m := range in.groupMembers[grp] {
			if pciPrefix(m) == prefix {
				g.Functions = append(g.Functions, m)
			} else {
				others = true
			}
		}
		sort.Strings(g.Functions)
		g.GroupIsolated = !others
	}
	return g
}

// prettyModel turns a raw lspci device string into a clean product name for the
// UI: prefer the bracketed marketing name (NVIDIA/Intel list it), else drop the
// vendor company and the [AMD/ATI] tag, then trim Mobile/Max-Q and rev suffixes.
// "NVIDIA Corporation AD107M [GeForce RTX 4060 Max-Q / Mobile]" -> "GeForce RTX 4060".
var (
	reModelBracket = regexp.MustCompile(`\[([^\]]*(?:GeForce|Radeon|RX|RTX|GTX|Arc|Iris|UHD|HD Graphics)[^\]]*)\]`)
	reModelVendor  = regexp.MustCompile(`(?i)(NVIDIA Corporation|Advanced Micro Devices, Inc\.|Intel Corporation|\[AMD/ATI\])\s*`)
	reModelRev     = regexp.MustCompile(`\s*\(rev [^)]*\)`)
	reModelSuffix  = regexp.MustCompile(`(?i)\s*(Max-Q|Mobile|/ ?Mobile|/ ?Max-Q).*$`)
)

func prettyModel(s string) string {
	orig := strings.TrimSpace(s)
	out := orig
	if m := reModelBracket.FindStringSubmatch(out); m != nil {
		out = m[1]
	} else {
		out = reModelVendor.ReplaceAllString(out, "")
	}
	out = reModelRev.ReplaceAllString(out, "")
	out = reModelSuffix.ReplaceAllString(out, "")
	if out = strings.TrimSpace(out); out == "" {
		return orig
	}
	return out
}

// pciPrefix drops the function digit. 0000:01:00.0 -> 0000:01:00.
func pciPrefix(slot string) string {
	if i := strings.LastIndex(slot, "."); i >= 0 {
		return slot[:i]
	}
	return slot
}

func buildChecks(in capInputs, host, pass *GPU) (checks []Check, hardFail bool) {
	add := func(c Check) { checks = append(checks, c) }

	if in.cpuVirt {
		add(Check{ID: "cpu-virt", Level: "ok", Label: "CPU virtualization", Value: in.cpuVendor + " enabled"})
	} else {
		hardFail = true
		add(Check{ID: "cpu-virt", Level: "fail", Label: "CPU virtualization", Value: "unavailable", Hint: "Enable SVM (AMD) or VT-x (Intel) in firmware."})
	}

	if in.kvm {
		add(Check{ID: "kvm", Level: "ok", Label: "KVM", Value: "/dev/kvm present"})
	} else {
		hardFail = true
		add(Check{ID: "kvm", Level: "fail", Label: "KVM", Value: "/dev/kvm missing", Hint: "Enable virtualization in firmware; load the kvm modules."})
	}

	if in.iommuOn {
		add(Check{ID: "iommu", Level: "ok", Label: "IOMMU", Value: "enabled"})
	} else {
		// IOMMU groups absent = VT-d/AMD-Vi is off in firmware. Modern kernels
		// already default intel_iommu=on when VT-d is present, so a kernel
		// cmdline token cannot substitute for the firmware switch; the only
		// real fix is the BIOS/UEFI setting.
		hardFail = true
		add(Check{ID: "iommu", Level: "fail", Label: "IOMMU", Value: "off", Hint: "Enable IOMMU / VT-d / AMD-Vi in firmware."})
	}

	if host != nil && pass != nil {
		add(Check{ID: "two-gpus", Level: "ok", Label: "Two GPUs", Value: host.Class + " + " + pass.Class})
	} else {
		hardFail = true
		add(Check{ID: "two-gpus", Level: "fail", Label: "Two GPUs", Value: "single GPU", Hint: "Passthrough needs a second GPU to keep the host alive; single-GPU is not supported."})
	}

	if pass != nil {
		if pass.GroupIsolated {
			add(Check{ID: "iommu-isolation", Level: "ok", Label: "dGPU isolation", Value: "group " + itoa(pass.IommuGroup) + " isolated"})
		} else {
			add(Check{ID: "iommu-isolation", Level: "warn", Label: "dGPU isolation", Value: "group " + itoa(pass.IommuGroup) + " shared", Hint: "Other devices share the dGPU's IOMMU group; they would be pulled into the VM."})
		}
	}

	if pass != nil {
		switch {
		case !pass.DrivesDisplay:
			add(Check{ID: "display-owner", Level: "ok", Label: "Display", Value: "host runs on " + hostLabel(host)})
		case in.chassis == "laptop":
			add(Check{ID: "display-owner", Level: "warn", Label: "Display", Value: "dGPU drives the panel", Hint: "Your screen is wired to the discrete GPU. Switch to Hybrid GPU mode in the BIOS/firmware (look for GPU Mode, MUX, or Hybrid/Optimus) and reboot, so the built-in GPU drives it."})
		case host != nil && host.DrivesDisplay:
			add(Check{ID: "display-owner", Level: "warn", Label: "Display", Value: "dGPU drives a monitor", Hint: "Ryoku moves to the other GPU on the next login."})
		default:
			hardFail = true
			add(Check{ID: "display-owner", Level: "fail", Label: "Display", Value: "only display is on the dGPU", Hint: "Connect a monitor to the other GPU, or the host would go headless."})
		}
	}

	if in.ramTotalMB >= 8192 && in.ramFreeMB >= 4096 {
		add(Check{ID: "ram", Level: "ok", Label: "Memory", Value: itoa(in.ramFreeMB) + " MB free"})
	} else {
		add(Check{ID: "ram", Level: "warn", Label: "Memory", Value: itoa(in.ramFreeMB) + " MB free", Hint: "A VM wants 8 GB+; close apps or lower the VM's RAM."})
	}

	switch miss := toolingMissing(in.tooling); {
	case len(miss) == 0:
		add(Check{ID: "tooling", Level: "ok", Label: "Virtualization stack", Value: "installed"})
	case !in.tooling.qemu:
		add(Check{ID: "tooling", Level: "warn", Label: "Virtualization stack", Value: "QEMU not installed", Hint: "A plain VM needs QEMU. Install it with `sudo ryoku-host pkg install qemu-desktop`."})
	default:
		add(Check{ID: "tooling", Level: "warn", Label: "Passthrough stack", Value: "missing: " + strings.Join(miss, ", ") + " (passthrough only)", Hint: "Only for the GPU-passthrough VM; plain VMs need none of it. Run `ryoku-host pkg advice <package>` for host-specific install help."})
	}
	if in.tooling.libvirt {
		if in.inLibvirtGroup {
			add(Check{ID: "session", Level: "ok", Label: "libvirt access", Value: "active"})
		} else {
			add(Check{ID: "session", Level: "warn", Label: "libvirt access", Value: "log in again", Hint: "You were added to the libvirt group; log out and back in before launching the VM."})
		}
	}
	return checks, hardFail
}

func decide(in capInputs, host, pass *GPU, hardFail bool) (strategy, verdict string) {
	if hardFail || host == nil || pass == nil {
		return "none", "incapable"
	}
	switch {
	case !pass.DrivesDisplay:
		strategy = "live-bind"
	case in.chassis == "laptop":
		strategy = "mux-reboot"
	case host.DrivesDisplay:
		strategy = "relogin-then-bind"
	default:
		return "none", "incapable"
	}
	if len(toolingMissing(in.tooling)) != 0 || !in.iommuOn {
		return strategy, "needs-setup"
	}
	switch strategy {
	case "live-bind":
		if !in.inLibvirtGroup {
			return strategy, "needs-relogin" // stack ready, group needs a fresh login
		}
		return strategy, "ready"
	case "relogin-then-bind":
		return strategy, "needs-relogin"
	case "mux-reboot":
		return strategy, "needs-reboot"
	}
	return strategy, "needs-setup"
}

func toolingMissing(t tooling) []string {
	var m []string
	if !t.qemu {
		m = append(m, "qemu")
	}
	if !t.libvirt {
		m = append(m, "libvirt")
	}
	if !t.ovmf {
		m = append(m, "ovmf")
	}
	if !t.swtpm {
		m = append(m, "swtpm")
	}
	if !t.lookingGlass {
		m = append(m, "looking-glass")
	}
	if !t.kvmfr {
		m = append(m, "kvmfr")
	}
	if !t.hook {
		m = append(m, "libvirt-hook")
	}
	return m
}

func hostLabel(host *GPU) string {
	if host == nil {
		return "the other GPU"
	}
	if host.Class == "integrated" {
		return "the iGPU"
	}
	return host.Model
}

func itoa(n int) string { return strconv.Itoa(n) }

// ── system probing (detectCapability) ──────────────────────────────────────────

func sysfsRoot() string {
	if r := os.Getenv("RYOKU_SYSFS_ROOT"); r != "" {
		return r
	}
	return "/"
}

// detectCapability: probe the live system, return the verdict.
func detectCapability() (Capability, error) {
	root := sysfsRoot()
	in := capInputs{
		groupOf:      map[string]int{},
		groupMembers: map[int][]string{},
	}
	recs, err := gpuRecordsFromTool()
	if err != nil {
		return Capability{}, err
	}
	in.records = recs
	in.cpuVendor, in.cpuVirt = readCPUVirt(root)
	in.kvm = fileExists(filepath.Join(root, "dev/kvm"))
	in.iommuOn = dirHasEntries(filepath.Join(root, "sys/kernel/iommu_groups"))
	in.chassis = readChassis(root)
	in.ramTotalMB, in.ramFreeMB = readMeminfo(root)
	for _, r := range recs {
		if grp, ok := readIommuGroup(root, r.Slot); ok {
			in.groupOf[r.Slot] = grp
			if _, seen := in.groupMembers[grp]; !seen {
				in.groupMembers[grp] = readGroupMembers(root, grp)
			}
		}
	}
	in.tooling = detectTooling(root)
	in.inLibvirtGroup = userInGroup("libvirt")
	return buildCapability(in), nil
}

// userInGroup: is this process actually a member of the named group? lets us
// tell a freshly-enabled passthrough (group added but the session predates it)
// apart from one that's actually ready to launch.
func userInGroup(name string) bool {
	g, err := user.LookupGroup(name)
	if err != nil {
		return false
	}
	gids, err := os.Getgroups()
	if err != nil {
		return false
	}
	for _, gid := range gids {
		if strconv.Itoa(gid) == g.Gid {
			return true
		}
	}
	return false
}

// gpuDetectTimeout caps the wait on `ryoku-gpu detect`. The detector already
// bounds its own host probes, so this is a backstop for the detector itself
// hanging or never returning; without it a stuck call would freeze the Hub GPU
// page on "Detecting..." forever. A test overrides it.
var gpuDetectTimeout = 12 * time.Second

func gpuRecordsFromTool() ([]gpuRecord, error) {
	bin := os.Getenv("RYOKU_GPU_BIN")
	if bin == "" {
		bin = "ryoku-gpu"
	}
	ctx, cancel := context.WithTimeout(context.Background(), gpuDetectTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "detect", "--json")
	// A wedged detector can spawn a host probe (nvidia-smi) that outlives a plain
	// kill and holds the stdout pipe open, so a context kill alone would not
	// unblock Output(). Run it in its own process group, kill the whole group on
	// timeout, and cap the pipe wait so Output() always returns.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 2 * time.Second
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("%s detect timed out after %s", bin, gpuDetectTimeout)
		}
		return nil, err
	}
	// An out-of-date ryoku-gpu predates `detect --json` and prints its table
	// instead, which is not JSON. Say so plainly rather than leaking a cryptic
	// "invalid character 'C'" from the parser.
	if s := strings.TrimSpace(string(out)); s == "" || s[0] != '[' {
		head := s
		if len(head) > 80 {
			head = head[:80]
		}
		return nil, fmt.Errorf("%s detect --json did not return JSON (out-of-date %s?): %q", bin, bin, head)
	}
	var recs []gpuRecord
	if err := json.Unmarshal(out, &recs); err != nil {
		return nil, err
	}
	return recs, nil
}

func readCPUVirt(root string) (vendor string, virt bool) {
	f, err := os.Open(filepath.Join(root, "proc/cpuinfo"))
	if err != nil {
		return "", false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if vendor == "" && strings.HasPrefix(line, "vendor_id") {
			if strings.Contains(line, "AuthenticAMD") {
				vendor = "AMD"
			} else if strings.Contains(line, "GenuineIntel") {
				vendor = "Intel"
			}
		}
		if !virt && strings.HasPrefix(line, "flags") {
			if strings.Contains(line, " svm") || strings.Contains(line, " vmx") {
				virt = true
			}
		}
	}
	return vendor, virt
}

func readChassis(root string) string {
	if b, err := os.ReadFile(filepath.Join(root, "sys/class/dmi/id/chassis_type")); err == nil {
		switch strings.TrimSpace(string(b)) {
		case "8", "9", "10", "11", "14", "30", "31", "32":
			return "laptop"
		case "3", "4", "5", "6", "7", "13", "23", "24", "25":
			return "desktop"
		}
	}
	if dirHasEntries(filepath.Join(root, "sys/class/power_supply")) {
		entries, _ := os.ReadDir(filepath.Join(root, "sys/class/power_supply"))
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "BAT") {
				return "laptop"
			}
		}
	}
	return "desktop"
}

func readMeminfo(root string) (totalMB, freeMB int) {
	f, err := os.Open(filepath.Join(root, "proc/meminfo"))
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		kb, _ := strconv.Atoi(fields[1]) // value is in kB
		switch fields[0] {
		case "MemTotal:":
			totalMB = kb / 1024
		case "MemAvailable:":
			freeMB = kb / 1024
		}
	}
	return totalMB, freeMB
}

func readIommuGroup(root, slot string) (int, bool) {
	link := filepath.Join(root, "sys/bus/pci/devices", slot, "iommu_group")
	target, err := os.Readlink(link)
	if err != nil {
		return 0, false
	}
	n, err := strconv.Atoi(filepath.Base(target))
	if err != nil {
		return 0, false
	}
	return n, true
}

func readGroupMembers(root string, group int) []string {
	dir := filepath.Join(root, "sys/kernel/iommu_groups", strconv.Itoa(group), "devices")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var members []string
	for _, e := range entries {
		members = append(members, e.Name())
	}
	sort.Strings(members)
	return members
}

func detectTooling(root string) tooling {
	return tooling{
		qemu:         lookPath("qemu-system-x86_64"),
		libvirt:      lookPath("virsh"),
		ovmf:         ovmfPresent(root),
		swtpm:        lookPath("swtpm"),
		lookingGlass: lookPath("looking-glass-client"),
		kvmfr:        kvmfrPresent(root),
		hook:         hookInstalled(root),
	}
}

func ovmfPresent(root string) bool {
	for _, p := range []string{
		"usr/share/edk2/x64/OVMF_CODE.4m.fd",
		"usr/share/edk2/x64/OVMF_CODE.fd",
		"usr/share/edk2-ovmf/x64/OVMF_CODE.fd",
		"usr/share/OVMF/OVMF_CODE.fd",
	} {
		if fileExists(filepath.Join(root, p)) {
			return true
		}
	}
	return false
}

func kvmfrPresent(root string) bool {
	if fileExists(filepath.Join(root, "dev/kvmfr0")) || dirHasEntries(filepath.Join(root, "sys/module/kvmfr")) {
		return true
	}
	return lookPath("looking-glass-client") && exec.Command("modinfo", "kvmfr").Run() == nil
}

func hookInstalled(root string) bool {
	b, err := os.ReadFile(filepath.Join(root, "etc/libvirt/hooks/qemu"))
	return err == nil && strings.Contains(string(b), "ryoku")
}

func lookPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func dirHasEntries(p string) bool {
	entries, err := os.ReadDir(p)
	return err == nil && len(entries) > 0
}
