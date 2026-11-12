package main

// gpuvm.go: `ryoku-hub gpu vm ...`, the passthrough-VM manager behind Ryoport's
// Looking Glass lane and the compatible `ryovm lg` CLI. It generates a
// performance-tuned libvirt domain named `ryoku-<name>` (so the existing vfio
// hook binds the dGPU on start), defines/starts/stops/removes it via virsh, and
// launches looking-glass-client on start. It is strictly for GPU passthrough;
// ordinary VMs use Ryoport's Quickemu backend.
//
// Design mirrors hwcaps.go: buildDomainXML and buildPinPlan are pure functions
// of gathered inputs (unit-tested across hardware shapes); the messy probing
// (topology, virtio ISO, virsh) is isolated. The manager runs as the invoking
// user against qemu:///system -- the enable step put the user in the libvirt
// group and dropped a polkit rule, so no pkexec is needed here.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// vmMeta is the small record kept beside each VM's disk. The libvirt domain is
// the runtime source of truth; this records what the app needs to list a
// machine without parsing domain XML.
type vmMeta struct {
	Name   string `json:"name"`   // user-facing, without the ryoku- prefix
	OS     string `json:"os"`     // windows | linux
	RAMMB  int    `json:"ramMb"`  //
	Vcpus  int    `json:"vcpus"`  //
	DiskGB int    `json:"diskGb"` //
	ISO    string `json:"iso"`    // install media, for reference
}

// vmMachine is one row the app renders: metadata plus live libvirt state.
type vmMachine struct {
	vmMeta
	Domain string `json:"domain"` // ryoku-<name>
	State  string `json:"state"`  // running | shutoff | paused | absent
	Disk   string `json:"disk"`
}

// vmList is the `list` payload: readiness (so the lane can gate itself) plus the
// machines. Blocker is a human sentence when passthrough is not ready to launch.
type vmList struct {
	Ready    bool        `json:"ready"`
	Blocker  string      `json:"blocker,omitempty"`
	Machines []vmMachine `json:"machines"`
}

const vmDomainPrefix = "ryoku-"

func runGpuVM(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("gpu vm needs a subcommand: list|create|start|stop|remove|status")
	}
	flags := parseVMFlags(args[1:])
	switch args[0] {
	case "list":
		return vmDoList()
	case "status":
		return vmDoStatus(flags["name"])
	case "create":
		return vmDoCreate(flags)
	case "start":
		return vmDoStart(flags["name"])
	case "stop":
		return vmDoStop(flags["name"], flags["force"] != "")
	case "remove":
		return vmDoRemove(flags["name"], flags["delete-disk"] != "")
	default:
		return fmt.Errorf("unknown gpu vm subcommand: %s", args[0])
	}
}

// parseVMFlags reads --key value (and bare --flag) pairs. Deliberately tiny: the
// verbs take a fixed, documented set and the app builds the argv itself.
func parseVMFlags(args []string) map[string]string {
	out := map[string]string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "--") {
			continue
		}
		key := strings.TrimPrefix(a, "--")
		if eq := strings.IndexByte(key, '='); eq >= 0 {
			out[key[:eq]] = key[eq+1:]
			continue
		}
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
			out[key] = args[i+1]
			i++
		} else {
			out[key] = "true"
		}
	}
	return out
}

// ── readiness ────────────────────────────────────────────────────────────────

// vmReadiness reports whether a passthrough VM can actually launch right now,
// and a human blocker when not. It reuses the caps engine so the lane gates on
// exactly the dossier the GPU page shows.
func vmReadiness() (bool, string, *Capability) {
	cap, err := detectCapability()
	if err != nil {
		return false, "could not read the GPU passthrough state: " + err.Error(), nil
	}
	switch {
	case cap.Passthrough == nil:
		return false, "no discrete GPU is free for passthrough on this machine", &cap
	case cap.Verdict == "incapable":
		return false, "this machine cannot pass a GPU through (see Ryoku Settings > System > Graphics)", &cap
	case cap.Verdict == "needs-setup":
		return false, "turn on GPU passthrough first in Ryoku Settings > System > Graphics", &cap
	case cap.Verdict == "needs-reboot":
		return false, "reboot to finish enabling passthrough, then launch the VM", &cap
	case cap.Verdict == "needs-relogin":
		return false, "log out and back in (or reboot) to activate the libvirt group, then launch", &cap
	case cap.Verdict != "ready":
		return false, "passthrough is not ready yet (see Ryoku Settings > System > Graphics)", &cap
	}
	return true, "", &cap
}

// ── list / status ────────────────────────────────────────────────────────────

func vmDoList() error {
	ready, blocker, _ := vmReadiness()
	out := vmList{Ready: ready, Blocker: blocker, Machines: []vmMachine{}}
	root := vmRoot()
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if m, ok := loadMachine(e.Name()); ok {
			out.Machines = append(out.Machines, m)
		}
	}
	sort.Slice(out.Machines, func(i, j int) bool { return out.Machines[i].Name < out.Machines[j].Name })
	return printJSON(out)
}

func vmDoStatus(name string) error {
	if name == "" {
		return fmt.Errorf("gpu vm status needs --name")
	}
	m, ok := loadMachine(name)
	if !ok {
		return fmt.Errorf("no Ryoku passthrough VM named %q", name)
	}
	return printJSON(m)
}

func loadMachine(name string) (vmMachine, bool) {
	meta, err := readMeta(name)
	if err != nil {
		return vmMachine{}, false
	}
	m := vmMachine{vmMeta: meta, Domain: vmDomainPrefix + name, Disk: diskPath(name)}
	m.State = domainState(m.Domain)
	return m, true
}

// domainState maps virsh's state to our vocabulary; a domain that was never
// defined (or whose define failed) reads as absent.
func domainState(domain string) string {
	out, err := exec.Command("virsh", "--connect", "qemu:///system", "domstate", domain).Output()
	if err != nil {
		return "absent"
	}
	s := strings.TrimSpace(string(out))
	switch s {
	case "running":
		return "running"
	case "shut off":
		return "shutoff"
	case "paused":
		return "paused"
	default:
		return s
	}
}

// ── create ───────────────────────────────────────────────────────────────────

func vmDoCreate(f map[string]string) error {
	name := f["name"]
	if err := validVMName(name); err != nil {
		return err
	}
	if _, err := readMeta(name); err == nil {
		return fmt.Errorf("a Ryoku passthrough VM named %q already exists", name)
	}
	ready, blocker, cap := vmReadiness()
	if !ready {
		return fmt.Errorf("%s", blocker)
	}
	guest := strings.ToLower(f["os"])
	if guest != "windows" && guest != "linux" {
		return fmt.Errorf("gpu vm create needs --os windows|linux")
	}
	iso := f["iso"]
	if iso == "" || !fileExists(iso) {
		return fmt.Errorf("gpu vm create needs --iso <path to an install ISO> (got %q)", iso)
	}
	diskGB := atoiOr(f["disk-gb"], 64)
	ramMB := atoiOr(f["ram-mb"], defaultRAMMB(cap))
	wantCores := atoiOr(f["cores"], 0) // 0 = auto

	topo := detectTopology()
	pin := buildPinPlan(topo, wantCores)

	dir := vmDir(name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	disk := diskPath(name)
	if !fileExists(disk) {
		if err := runErr("qemu-img", "create", "-f", "qcow2", disk, fmt.Sprintf("%dG", diskGB)); err != nil {
			return fmt.Errorf("creating the disk image failed: %w", err)
		}
	}

	spec := vmSpec{
		Domain:     vmDomainPrefix + name,
		MemMB:      ramMB,
		Pin:        pin,
		Disk:       disk,
		InstallISO: iso,
		VirtioISO:  findVirtioISO(),
		GPU:        *cap.Passthrough,
		KvmfrMB:    kvmfrStaticMB,
		Windows:    guest == "windows",
	}
	xml := buildDomainXML(spec)
	xmlPath := filepath.Join(dir, "domain.xml")
	if err := os.WriteFile(xmlPath, []byte(xml), 0o644); err != nil {
		return err
	}
	if err := runErr("virsh", "--connect", "qemu:///system", "define", xmlPath); err != nil {
		return fmt.Errorf("defining the VM with libvirt failed: %w (is the libvirt group active? a reboot may be needed)", err)
	}

	meta := vmMeta{Name: name, OS: guest, RAMMB: ramMB, Vcpus: len(pin.Vcpus), DiskGB: diskGB, ISO: iso}
	if err := writeMeta(name, meta); err != nil {
		return err
	}
	return printJSON(map[string]any{
		"name": name, "domain": spec.Domain, "defined": true,
		"vcpus": len(pin.Vcpus), "ramMb": ramMB, "virtio": spec.VirtioISO != "",
	})
}

// ── start / stop / remove ────────────────────────────────────────────────────

func vmDoStart(name string) error {
	if name == "" {
		return fmt.Errorf("gpu vm start needs --name")
	}
	if _, err := readMeta(name); err != nil {
		return fmt.Errorf("no Ryoku passthrough VM named %q", name)
	}
	ready, blocker, _ := vmReadiness()
	if !ready {
		return fmt.Errorf("%s", blocker)
	}
	domain := vmDomainPrefix + name
	if domainState(domain) != "running" {
		// virsh start triggers the libvirt hook, which binds the dGPU to vfio-pci
		// before qemu comes up and hands it back on shutdown.
		if err := runErr("virsh", "--connect", "qemu:///system", "start", domain); err != nil {
			return fmt.Errorf("starting the VM failed: %w", err)
		}
	}
	launched := launchLookingGlass(domain)
	return printJSON(map[string]any{"name": name, "domain": domain, "started": true, "lookingGlass": launched})
}

func vmDoStop(name string, force bool) error {
	if name == "" {
		return fmt.Errorf("gpu vm stop needs --name")
	}
	domain := vmDomainPrefix + name
	verb := "shutdown"
	if force {
		verb = "destroy"
	}
	if err := runErr("virsh", "--connect", "qemu:///system", verb, domain); err != nil {
		return fmt.Errorf("stopping the VM failed: %w", err)
	}
	return printJSON(map[string]any{"name": name, "domain": domain, "stopped": true})
}

func vmDoRemove(name string, deleteDisk bool) error {
	if name == "" {
		return fmt.Errorf("gpu vm remove needs --name")
	}
	domain := vmDomainPrefix + name
	if domainState(domain) == "running" {
		return fmt.Errorf("stop %q before removing it", name)
	}
	// undefine --nvram drops the per-domain UEFI vars libvirt created; ignore a
	// failure so a half-defined VM can still be cleaned up.
	_ = runErr("virsh", "--connect", "qemu:///system", "undefine", "--nvram", domain)
	if deleteDisk {
		_ = os.RemoveAll(vmDir(name))
	} else {
		// keep the disk; drop only our metadata so the machine leaves the list.
		_ = os.Remove(filepath.Join(vmDir(name), "meta.json"))
		_ = os.Remove(filepath.Join(vmDir(name), "domain.xml"))
	}
	return printJSON(map[string]any{"name": name, "removed": true, "diskDeleted": deleteDisk})
}

// launchLookingGlass starts looking-glass-client detached, pointed at the
// domain's SPICE endpoint for input and the kvmfr device for frames. Best
// effort: the VM is already up, so a missing client is reported, not fatal.
func launchLookingGlass(domain string) bool {
	client, err := exec.LookPath("looking-glass-client")
	if err != nil {
		return false
	}
	args := []string{"-f", "/dev/kvmfr0"}
	if host, port := spiceEndpoint(domain); port != "" {
		args = append(args, "spice:host", host, "spice:port", port)
	}
	cmd := exec.Command(client, args...)
	cmd.Stdout, cmd.Stderr = nil, nil
	if err := cmd.Start(); err != nil {
		return false
	}
	// don't reap it: the client outlives this short-lived helper process.
	_ = cmd.Process.Release()
	return true
}

// spiceEndpoint reads the domain's live SPICE host:port from virsh domdisplay
// (e.g. "spice://127.0.0.1:5900"). Empty port when SPICE is not up yet.
func spiceEndpoint(domain string) (string, string) {
	out, err := exec.Command("virsh", "--connect", "qemu:///system", "domdisplay", "--type", "spice", domain).Output()
	if err != nil {
		return "", ""
	}
	uri := strings.TrimSpace(string(out))
	uri = strings.TrimPrefix(uri, "spice://")
	host, port, ok := strings.Cut(uri, ":")
	if !ok {
		return "", ""
	}
	if host == "" {
		host = "127.0.0.1"
	}
	return host, port
}

// ── paths / metadata ─────────────────────────────────────────────────────────

func vmRoot() string {
	if d := os.Getenv("RYOKU_VM_ROOT"); d != "" {
		return d
	}
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "ryoku", "vm")
}

func vmDir(name string) string    { return filepath.Join(vmRoot(), name) }
func diskPath(name string) string { return filepath.Join(vmDir(name), "disk.qcow2") }

func readMeta(name string) (vmMeta, error) {
	var m vmMeta
	b, err := os.ReadFile(filepath.Join(vmDir(name), "meta.json"))
	if err != nil {
		return m, err
	}
	return m, json.Unmarshal(b, &m)
}

func writeMeta(name string, m vmMeta) error {
	b, _ := json.MarshalIndent(m, "", "  ")
	return os.WriteFile(filepath.Join(vmDir(name), "meta.json"), b, 0o644)
}

// ── small helpers ────────────────────────────────────────────────────────────

// validVMName keeps names to a safe, libvirt-friendly slug so ryoku-<name> is a
// valid domain name and a safe directory.
func validVMName(name string) error {
	if name == "" {
		return fmt.Errorf("gpu vm needs --name")
	}
	for _, r := range name {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_'
		if !ok {
			return fmt.Errorf("VM name %q must be letters, digits, - or _", name)
		}
	}
	return nil
}

func defaultRAMMB(cap *Capability) int {
	// half the host's RAM, clamped to a sane floor, rounded to 1 GiB.
	half := 8192
	if cap != nil && cap.RamTotalMB > 0 {
		half = cap.RamTotalMB / 2
	}
	if half < 4096 {
		half = 4096
	}
	return (half / 1024) * 1024
}

func atoiOr(s string, def int) int {
	if s == "" {
		return def
	}
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		return n
	}
	return def
}

// findVirtioISO locates a virtio-win ISO so a Windows install sees the virtio
// disk. Ryoport keeps one validated copy in its shared images directory; fall
// back to the usual system spots. Empty when none is present (Linux guests need none).
func findVirtioISO() string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "share")
	}
	for _, p := range []string{
		filepath.Join(base, "ryoku", "vms", ".images", "virtio-win.iso"),
		"/usr/share/virtio-win/virtio-win.iso",
		"/var/lib/libvirt/images/virtio-win.iso",
	} {
		if fileExists(p) {
			return p
		}
	}
	return ""
}
