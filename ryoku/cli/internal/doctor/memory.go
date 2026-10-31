package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"ryoku-cli/internal/host"

	i18n "ryoku-i18n"
)

const memoryProbeWindow = 2 * time.Second

type memoryPaths struct {
	meminfo    string
	swaps      string
	swappiness string
	cgroupRoot string
	procRoot   string
}

var doctorMemoryPaths = memoryPaths{
	meminfo:    "/proc/meminfo",
	swaps:      "/proc/swaps",
	swappiness: "/proc/sys/vm/swappiness",
	cgroupRoot: "/sys/fs/cgroup",
	procRoot:   "/proc",
}

type swapDevice struct {
	name             string
	sizeKiB, usedKiB uint64
}

type memoryProcess struct {
	pid, ppid int
	name      string
	rssKiB    uint64
}

type memoryDiagnostics struct {
	systemd                      bool
	ramTotalKiB, ramAvailableKiB uint64
	swapTotalKiB, swapFreeKiB    uint64
	zramTotalKiB, zramUsedKiB    uint64
	zramDevices                  int
	swappiness                   string
	oomd                         string
	cgroupFound                  bool
	cgroupCurrent, cgroupPeak    uint64
	cgroupOOMKills               uint64
	processes                    []memoryProcess
	kernelOOMKills               int
	kernelOOMKnown               bool
}

func parseMeminfo(raw string) map[string]uint64 {
	values := make(map[string]uint64)
	for _, line := range strings.Split(raw, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields := strings.Fields(value)
		if len(fields) == 0 {
			continue
		}
		n, err := strconv.ParseUint(fields[0], 10, 64)
		if err == nil {
			values[strings.TrimSpace(key)] = n
		}
	}
	return values
}

func parseSwapDevices(raw string) []swapDevice {
	var devices []swapDevice
	for index, line := range strings.Split(raw, "\n") {
		if index == 0 {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		size, sizeErr := strconv.ParseUint(fields[2], 10, 64)
		used, usedErr := strconv.ParseUint(fields[3], 10, 64)
		if sizeErr == nil && usedErr == nil {
			devices = append(devices, swapDevice{name: fields[0], sizeKiB: size, usedKiB: used})
		}
	}
	return devices
}

func parseMemoryEvents(raw string) map[string]uint64 {
	values := make(map[string]uint64)
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		n, err := strconv.ParseUint(fields[1], 10, 64)
		if err == nil {
			values[fields[0]] = n
		}
	}
	return values
}

func parseProcessStatus(raw string) (memoryProcess, bool) {
	var process memoryProcess
	for _, line := range strings.Split(raw, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch key {
		case "Name":
			process.name = strings.TrimSpace(value)
		case "Pid":
			process.pid, _ = strconv.Atoi(strings.TrimSpace(value))
		case "PPid":
			process.ppid, _ = strconv.Atoi(strings.TrimSpace(value))
		case "VmRSS":
			fields := strings.Fields(value)
			if len(fields) > 0 {
				process.rssKiB, _ = strconv.ParseUint(fields[0], 10, 64)
			}
		}
	}
	return process, process.name != ""
}

func cgroupProcesses(cgroupDir, procRoot string) []memoryProcess {
	raw, err := os.ReadFile(filepath.Join(cgroupDir, "cgroup.procs"))
	if err != nil {
		return nil
	}
	var processes []memoryProcess
	for _, pid := range strings.Fields(string(raw)) {
		pidValue, err := strconv.Atoi(pid)
		if err != nil {
			continue
		}
		status, err := os.ReadFile(filepath.Join(procRoot, pid, "status"))
		if err != nil {
			continue
		}
		if process, ok := parseProcessStatus(string(status)); ok {
			process.pid = pidValue
			processes = append(processes, process)
		}
	}
	sort.Slice(processes, func(i, j int) bool {
		if processes[i].rssKiB != processes[j].rssKiB {
			return processes[i].rssKiB > processes[j].rssKiB
		}
		return processes[i].name < processes[j].name
	})
	return processes
}

func boundedMemoryCommand(name string, args ...string) (string, bool) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", false
	}
	out := runBoundedProbe(exec.Command(path, args...), memoryProbeWindow, func(string) bool { return false })
	return strings.TrimSpace(out), true
}

func shellCgroupPath(root string) string {
	out, ok := boundedMemoryCommand("systemctl", "--user", "show", "ryoku-shell.service", "--property=ControlGroup", "--value", "--no-pager")
	if !ok {
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "/") {
			return filepath.Join(root, strings.TrimPrefix(line, "/"))
		}
	}
	return ""
}

func readUintFile(path string) (uint64, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	value, err := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
	return value, err == nil
}

func kernelOOMKillCount() (int, bool) {
	out, ok := boundedMemoryCommand("journalctl", "-k", "-b", "--no-pager", "--output=cat", "--grep=Killed process")
	if !ok {
		return 0, false
	}
	count := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Killed process") {
			count++
		}
	}
	return count, true
}

func systemdOOMDState() string {
	out, ok := boundedMemoryCommand("systemctl", "is-active", "systemd-oomd.service")
	if !ok {
		return "unknown"
	}
	state := strings.TrimSpace(strings.Split(out, "\n")[0])
	if state == "" {
		return "unknown"
	}
	return state
}

func gatherMemoryDiagnostics(initSystem host.InitSystem, paths memoryPaths) memoryDiagnostics {
	diagnostic := memoryDiagnostics{systemd: initSystem == host.Systemd}
	if raw, err := os.ReadFile(paths.meminfo); err == nil {
		values := parseMeminfo(string(raw))
		diagnostic.ramTotalKiB = values["MemTotal"]
		diagnostic.ramAvailableKiB = values["MemAvailable"]
		diagnostic.swapTotalKiB = values["SwapTotal"]
		diagnostic.swapFreeKiB = values["SwapFree"]
	}
	if raw, err := os.ReadFile(paths.swaps); err == nil {
		for _, device := range parseSwapDevices(string(raw)) {
			if strings.HasPrefix(filepath.Base(device.name), "zram") {
				diagnostic.zramDevices++
				diagnostic.zramTotalKiB += device.sizeKiB
				diagnostic.zramUsedKiB += device.usedKiB
			}
		}
	}
	if raw, err := os.ReadFile(paths.swappiness); err == nil {
		diagnostic.swappiness = strings.TrimSpace(string(raw))
	}
	if !diagnostic.systemd {
		return diagnostic
	}

	diagnostic.oomd = systemdOOMDState()
	diagnostic.kernelOOMKills, diagnostic.kernelOOMKnown = kernelOOMKillCount()
	cgroupDir := shellCgroupPath(paths.cgroupRoot)
	if cgroupDir == "" {
		return diagnostic
	}
	if _, err := os.Stat(cgroupDir); err != nil {
		return diagnostic
	}
	diagnostic.cgroupFound = true
	diagnostic.cgroupCurrent, _ = readUintFile(filepath.Join(cgroupDir, "memory.current"))
	diagnostic.cgroupPeak, _ = readUintFile(filepath.Join(cgroupDir, "memory.peak"))
	if raw, err := os.ReadFile(filepath.Join(cgroupDir, "memory.events")); err == nil {
		diagnostic.cgroupOOMKills = parseMemoryEvents(string(raw))["oom_kill"]
	}
	diagnostic.processes = cgroupProcesses(cgroupDir, paths.procRoot)
	return diagnostic
}

func memoryMiB(kib uint64) uint64 {
	return (kib + 512) / 1024
}

func bytesMiB(value uint64) uint64 {
	return (value + 512*1024) / (1024 * 1024)
}

func memoryProcessList(processes []memoryProcess, limit int) string {
	if len(processes) > limit {
		processes = processes[:limit]
	}
	parts := make([]string, 0, len(processes))
	for _, process := range processes {
		parts = append(parts, fmt.Sprintf("%s %d MiB", process.name, memoryMiB(process.rssKiB)))
	}
	return strings.Join(parts, ", ")
}

func memoryReportLines(diagnostic memoryDiagnostics) []string {
	var lines []string
	if diagnostic.ramTotalKiB > 0 {
		lines = append(lines, fmt.Sprintf("RAM: %d MiB total, %d MiB available", memoryMiB(diagnostic.ramTotalKiB), memoryMiB(diagnostic.ramAvailableKiB)))
	} else {
		lines = append(lines, "RAM: unavailable")
	}
	if diagnostic.swapTotalKiB > 0 {
		used := diagnostic.swapTotalKiB - min(diagnostic.swapFreeKiB, diagnostic.swapTotalKiB)
		lines = append(lines, fmt.Sprintf("swap: %d MiB total, %d MiB used", memoryMiB(diagnostic.swapTotalKiB), memoryMiB(used)))
	} else {
		lines = append(lines, "swap: none")
	}
	if diagnostic.zramDevices > 0 {
		lines = append(lines, fmt.Sprintf("zram: %d MiB total, %d MiB used (%d device(s))", memoryMiB(diagnostic.zramTotalKiB), memoryMiB(diagnostic.zramUsedKiB), diagnostic.zramDevices))
	} else {
		lines = append(lines, "zram: none")
	}
	if diagnostic.swappiness != "" {
		lines = append(lines, "vm.swappiness: "+diagnostic.swappiness)
	}
	if diagnostic.systemd {
		lines = append(lines, "systemd-oomd: "+diagnostic.oomd)
		if diagnostic.cgroupFound {
			lines = append(lines, fmt.Sprintf("ryoku-shell.service: %d MiB current, %d MiB peak, %d OOM kill(s)", bytesMiB(diagnostic.cgroupCurrent), bytesMiB(diagnostic.cgroupPeak), diagnostic.cgroupOOMKills))
			if len(diagnostic.processes) > 0 {
				lines = append(lines, "top RSS in ryoku-shell.service: "+memoryProcessList(diagnostic.processes, 5))
			}
		} else {
			lines = append(lines, "ryoku-shell.service memory: unavailable")
		}
		if diagnostic.kernelOOMKnown {
			lines = append(lines, fmt.Sprintf("kernel OOM kills this boot: %d", diagnostic.kernelOOMKills))
		}
	}
	return lines
}

func looksLikeUserApp(name string) bool {
	name = strings.ToLower(name)
	switch name {
	case "qs", "adb", "cliphist", "flock", "bash", "sh", "dbus-monitor",
		"ryoku", "lspci", "grep":
		return false
	}
	return !strings.HasPrefix(name, "ryoku-") && !strings.HasPrefix(name, "ryogami") &&
		!strings.HasPrefix(name, "wl-") && !strings.HasPrefix(name, "python3")
}

func inProcessTree(process memoryProcess, rootPID int, processes map[int]memoryProcess) bool {
	if rootPID <= 0 || process.pid <= 0 {
		return false
	}
	for range len(processes) + 1 {
		if process.pid == rootPID || process.ppid == rootPID {
			return true
		}
		parent, ok := processes[process.ppid]
		if !ok {
			return false
		}
		process = parent
	}
	return false
}

func userAppsInShell(processes []memoryProcess, doctorPID int) []memoryProcess {
	byPID := make(map[int]memoryProcess, len(processes))
	for _, process := range processes {
		byPID[process.pid] = process
	}
	apps := make([]memoryProcess, 0, len(processes))
	for _, process := range processes {
		if looksLikeUserApp(process.name) && !inProcessTree(process, doctorPID, byPID) {
			apps = append(apps, process)
		}
	}
	return apps
}

func reconcileMemoryHealth(_ bool) recResult {
	initSystem, _ := doctorInitSystem()
	diagnostic := gatherMemoryDiagnostics(initSystem, doctorMemoryPaths)
	var problems []string
	if diagnostic.kernelOOMKnown && diagnostic.kernelOOMKills > 0 {
		problems = append(problems, fmt.Sprintf(i18n.T("the kernel killed %d process(es) for memory this boot"), diagnostic.kernelOOMKills))
	} else if diagnostic.cgroupOOMKills > 0 {
		problems = append(problems, fmt.Sprintf(i18n.T("the shell service recorded %d memory kill(s)"), diagnostic.cgroupOOMKills))
	}
	if apps := userAppsInShell(diagnostic.processes, os.Getpid()); len(apps) > 0 {
		problems = append(problems, fmt.Sprintf(i18n.T("user apps are still inside the shell service: %s"), memoryProcessList(apps, 5)))
	}
	if len(problems) > 0 {
		return warnRes(strings.Join(problems, "; "))
	}
	return noteRes(strings.Join(memoryReportLines(diagnostic), "; "))
}
