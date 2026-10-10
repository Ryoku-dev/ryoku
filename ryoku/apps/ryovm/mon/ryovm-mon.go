// ryovm-mon: live control of a running quickemu VM through the sockets quickemu
// already creates -- the HMP text monitor and, when the guest runs
// qemu-guest-agent, the QGA JSON socket. No QEMU flag injection, no engine
// changes. Each verb prints one JSON object to stdout; errors go to stderr with
// a nonzero exit.
//
// Usage:
//
//	ryovm-mon stats   <monitorSock> <agentSock|-> <pidFile|->
//	ryovm-mon power   <monitorSock> <pause|resume|reset|shutdown>
//	ryovm-mon balloon <monitorSock> <MB>
//	ryovm-mon pin     <monitorSock> <pidFile> <auto|off>
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	dialTimeout = 800 * time.Millisecond  // socket connect budget
	cmdDeadline = 1500 * time.Millisecond // per-HMP-command read budget
	qgaDeadline = 600 * time.Millisecond  // guest agent is best-effort, keep it snappy
	cpuSample   = 300 * time.Millisecond  // gap between the two /proc CPU samples
)

func emit(v map[string]any) {
	b, _ := json.Marshal(v)
	fmt.Println(string(b))
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "ryovm-mon: "+msg)
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		fail("usage: ryovm-mon <stats|power|balloon|pin> ...")
	}
	switch os.Args[1] {
	case "stats":
		if len(os.Args) != 5 {
			fail("usage: ryovm-mon stats <monitorSock> <agentSock|-> <pidFile|->")
		}
		doStats(os.Args[2], os.Args[3], os.Args[4])
	case "power":
		if len(os.Args) != 4 {
			fail("usage: ryovm-mon power <monitorSock> <pause|resume|reset|shutdown>")
		}
		doPower(os.Args[2], os.Args[3])
	case "balloon":
		if len(os.Args) != 4 {
			fail("usage: ryovm-mon balloon <monitorSock> <MB>")
		}
		doBalloon(os.Args[2], os.Args[3])
	case "pin":
		if len(os.Args) != 5 {
			fail("usage: ryovm-mon pin <monitorSock> <pidFile> <auto|off>")
		}
		doPin(os.Args[2], os.Args[3], os.Args[4])
	default:
		fail("unknown verb " + os.Args[1])
	}
}

// --- HMP monitor client -----------------------------------------------------

// ansi strips the CSI escapes QEMU's readline emits while echoing typed input
// (cursor moves and line erases); the prompt "(qemu)" itself is plain bytes.
var ansi = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

type monitor struct{ conn net.Conn }

func dialMonitor(sock string) (*monitor, error) {
	conn, err := net.DialTimeout("unix", sock, dialTimeout)
	if err != nil {
		return nil, err
	}
	m := &monitor{conn}
	m.readReply() // consume the banner and first prompt
	return m, nil
}

func (m *monitor) close() { m.conn.Close() }

// readReply reads until the next "(qemu)" prompt or the read deadline, then
// returns the ANSI-stripped text. QEMU echoes each typed character with cursor
// escapes but never reprints the prompt mid-reply, so the marker reliably ends
// one command's output.
func (m *monitor) readReply() string {
	m.conn.SetReadDeadline(time.Now().Add(cmdDeadline))
	buf := make([]byte, 4096)
	var acc []byte
	for {
		n, err := m.conn.Read(buf)
		if n > 0 {
			acc = append(acc, buf[:n]...)
			if strings.Contains(string(acc), "(qemu)") {
				break
			}
		}
		if err != nil {
			break
		}
	}
	return ansi.ReplaceAllString(string(acc), "")
}

func (m *monitor) cmd(command string) string {
	m.conn.Write([]byte(command + "\n"))
	return m.readReply()
}

var (
	reStatus  = regexp.MustCompile(`VM status:\s*(\S+)`)
	reBalloon = regexp.MustCompile(`actual=(\d+)`)
	reThread  = regexp.MustCompile(`thread_id=(\d+)`)
)

func parseStatus(reply string) string {
	if m := reStatus.FindStringSubmatch(reply); m != nil {
		return m[1]
	}
	return "unknown"
}

// parseBalloon reads "balloon: actual=<MiB>"; absent when the VM has no balloon
// device, in which case the field is omitted.
func parseBalloon(reply string) (int, bool) {
	if m := reBalloon.FindStringSubmatch(reply); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n, true
	}
	return 0, false
}

// parseCpus returns the vCPU thread ids in cpu-index order.
func parseCpus(reply string) []int {
	ms := reThread.FindAllStringSubmatch(reply, -1)
	tids := make([]int, 0, len(ms))
	for _, m := range ms {
		n, _ := strconv.Atoi(m[1])
		tids = append(tids, n)
	}
	return tids
}

// --- verbs ------------------------------------------------------------------

func doStats(monSock, agentSock, pidFile string) {
	m, err := dialMonitor(monSock)
	if err != nil {
		// a stopped VM has no live monitor; report it plainly, not as an error.
		emit(map[string]any{"running": false})
		return
	}
	defer m.close()

	out := map[string]any{"running": true}
	out["status"] = parseStatus(m.cmd("info status"))
	if mb, ok := parseBalloon(m.cmd("info balloon")); ok {
		out["balloonMB"] = mb
	}
	tids := parseCpus(m.cmd("info cpus"))
	out["vcpus"] = len(tids)
	out["vcpuTids"] = tids

	if pidFile != "-" {
		if pid, err := readPid(pidFile); err == nil {
			if pct, ok := hostCpuPct(pid); ok {
				out["hostCpuPct"] = pct
			}
			if rss, ok := hostRssMB(pid); ok {
				out["hostRssMB"] = rss
			}
			out["pinned"] = allPinned(pid, tids)
			if io, ok := processIO(pid); ok {
				for key, value := range io {
					out[key] = value
				}
			}
			if uptime, ok := processUptime(pid); ok {
				out["uptimeS"] = uptime
			}
		}
	}
	if agentSock != "-" {
		if ip, osName := qgaInfo(agentSock); ip != "" || osName != "" {
			if ip != "" {
				out["guestIp"] = ip
			}
			if osName != "" {
				out["guestOs"] = osName
			}
		}
	}
	emit(out)
}

func doPower(monSock, action string) {
	var hmp string
	switch action {
	case "pause":
		hmp = "stop"
	case "resume":
		hmp = "cont"
	case "reset":
		hmp = "system_reset"
	case "shutdown":
		hmp = "system_powerdown"
	default:
		fail("power action must be pause|resume|reset|shutdown")
	}
	m, err := dialMonitor(monSock)
	if err != nil {
		fail("monitor: " + err.Error())
	}
	defer m.close()
	m.cmd(hmp)
	emit(map[string]any{"ok": true})
}

func doBalloon(monSock, mbArg string) {
	mb, err := strconv.Atoi(mbArg)
	if err != nil || mb <= 0 {
		fail("balloon MB must be a positive integer")
	}
	m, err := dialMonitor(monSock)
	if err != nil {
		fail("monitor: " + err.Error())
	}
	defer m.close()
	m.cmd("balloon " + strconv.Itoa(mb))
	emit(map[string]any{"ok": true, "balloonMB": mb})
}

func doPin(monSock, pidFile, mode string) {
	if mode != "auto" && mode != "off" {
		fail("pin mode must be auto|off")
	}
	if _, err := readPid(pidFile); err != nil {
		fail("pid: " + err.Error())
	}
	m, err := dialMonitor(monSock)
	if err != nil {
		fail("monitor: " + err.Error())
	}
	tids := parseCpus(m.cmd("info cpus"))
	m.close()
	if len(tids) == 0 {
		fail("no vcpus reported by monitor")
	}

	nproc := runtime.NumCPU()
	// off spreads each vcpu across every host cpu; auto pins vcpu i to core i%nproc.
	all := make([]int, nproc)
	for c := range all {
		all[c] = c
	}
	pinned := make([]map[string]any, 0, len(tids))
	for i, tid := range tids {
		mask, core := all, -1
		if mode == "auto" {
			core = i % nproc
			mask = []int{core}
		}
		if err := setAffinity(tid, mask); err != nil {
			fail(fmt.Sprintf("setaffinity tid %d: %v", tid, err))
		}
		pinned = append(pinned, map[string]any{"tid": tid, "core": core})
	}
	emit(map[string]any{"ok": true, "pinned": pinned})
}

// --- host stats (/proc) -----------------------------------------------------

func readPid(pidFile string) (int, error) {
	b, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(b)))
}

// procJiffies sums the process utime+stime. The comm field can hold spaces and
// parens, so index fields relative to the last ')'.
func procJiffies(pid int) (int64, bool) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, false
	}
	s := string(b)
	i := strings.LastIndexByte(s, ')')
	if i < 0 {
		return 0, false
	}
	f := strings.Fields(s[i+1:]) // f[0] is state (field 3); utime=14 -> f[11], stime=15 -> f[12]
	if len(f) < 13 {
		return 0, false
	}
	ut, _ := strconv.ParseInt(f[11], 10, 64)
	st, _ := strconv.ParseInt(f[12], 10, 64)
	return ut + st, true
}

// totalJiffies sums the aggregate cpu line of /proc/stat across all cores.
func totalJiffies() (int64, bool) {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, false
	}
	line := string(b)
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	f := strings.Fields(line)
	if len(f) < 2 || f[0] != "cpu" {
		return 0, false
	}
	var total int64
	for _, v := range f[1:] {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			continue
		}
		total += n
	}
	return total, true
}

// hostCpuPct is a top-style reading (100% == one full core) taken from two
// samples ~cpuSample apart: the process' jiffy delta over the system-wide delta,
// scaled by core count so a busy single core reads ~100.
func hostCpuPct(pid int) (float64, bool) {
	p1, ok1 := procJiffies(pid)
	t1, ok2 := totalJiffies()
	if !ok1 || !ok2 {
		return 0, false
	}
	time.Sleep(cpuSample)
	p2, _ := procJiffies(pid)
	t2, _ := totalJiffies()
	dp, dt := p2-p1, t2-t1
	if dt <= 0 {
		return 0, true
	}
	pct := 100 * float64(dp) / float64(dt) * float64(runtime.NumCPU())
	return math.Round(pct*10) / 10, true
}

func hostRssMB(pid int) (int, bool) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			f := strings.Fields(line) // VmRSS: <kB> kB
			if len(f) >= 2 {
				kb, _ := strconv.Atoi(f[1])
				return kb / 1024, true
			}
		}
	}
	return 0, false
}

// QEMU's character I/O includes socket traffic while read_bytes/write_bytes
// account for storage. Their difference is the best per-process network stream
// available without privileged packet tracing.
func processIO(pid int) (map[string]uint64, bool) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/io", pid))
	if err != nil {
		return nil, false
	}
	values := map[string]uint64{}
	for _, line := range strings.Split(string(b), "\n") {
		key, raw, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		n, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
		if err == nil {
			values[key] = n
		}
	}
	readBytes, readOK := values["read_bytes"]
	writeBytes, writeOK := values["write_bytes"]
	rchar, rcharOK := values["rchar"]
	wchar, wcharOK := values["wchar"]
	if !readOK || !writeOK {
		return nil, false
	}
	netRX, netTX := uint64(0), uint64(0)
	if rcharOK && rchar > readBytes {
		netRX = rchar - readBytes
	}
	if wcharOK && wchar > writeBytes {
		netTX = wchar - writeBytes
	}
	return map[string]uint64{
		"diskReadBytes":  readBytes,
		"diskWriteBytes": writeBytes,
		"netRxBytes":     netRX,
		"netTxBytes":     netTX,
	}, true
}

func processUptime(pid int) (int64, bool) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, false
	}
	// comm is parenthesized and may contain spaces. starttime is field 22,
	// index 19 after the closing parenthesis.
	end := strings.LastIndexByte(string(b), ')')
	if end < 0 {
		return 0, false
	}
	fields := strings.Fields(string(b)[end+1:])
	if len(fields) <= 19 {
		return 0, false
	}
	startTicks, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return 0, false
	}
	up, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, false
	}
	var upSeconds float64
	if _, err := fmt.Sscanf(string(up), "%f", &upSeconds); err != nil {
		return 0, false
	}
	const clockTicks = 100
	seconds := int64(upSeconds) - int64(startTicks/clockTicks)
	if seconds < 0 {
		seconds = 0
	}
	return seconds, true
}

func cpusAllowedList(pid, tid int) (string, bool) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/task/%d/status", pid, tid))
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "Cpus_allowed_list:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Cpus_allowed_list:")), true
		}
	}
	return "", false
}

// allPinned is true when every vcpu thread's affinity is a single cpu (the list
// has no range or comma).
func allPinned(pid int, tids []int) bool {
	if len(tids) == 0 {
		return false
	}
	for _, tid := range tids {
		list, ok := cpusAllowedList(pid, tid)
		if !ok || list == "" || strings.ContainsAny(list, ",-") {
			return false
		}
	}
	return true
}

// setAffinity pins one thread (Linux tasks are threads, so a tid works) to the
// given cpu set via sched_setaffinity; same-user, no root required. A 128-byte
// mask covers 1024 cpus and is a multiple of sizeof(long) as the kernel demands.
func setAffinity(tid int, cpus []int) error {
	var mask [128]byte
	for _, c := range cpus {
		if c >= 0 && c/8 < len(mask) {
			mask[c/8] |= 1 << (uint(c) % 8)
		}
	}
	_, _, errno := syscall.Syscall(syscall.SYS_SCHED_SETAFFINITY,
		uintptr(tid), uintptr(len(mask)), uintptr(unsafe.Pointer(&mask[0])))
	if errno != 0 {
		return errno
	}
	return nil
}

// --- QGA guest agent (optional) ---------------------------------------------

// qgaInfo returns the guest's first non-loopback IPv4 and OS pretty-name. It is
// best-effort: if the agent socket is missing or silent within qgaDeadline both
// values come back empty and stats omits them.
func qgaInfo(sock string) (ip, osName string) {
	conn, err := net.DialTimeout("unix", sock, dialTimeout)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(qgaDeadline))
	dec := json.NewDecoder(conn)

	// guest-sync flushes stale data: send a random id and read until it echoes.
	id := int(time.Now().UnixNano() & 0x7fffffff)
	fmt.Fprintf(conn, "{\"execute\":\"guest-sync\",\"arguments\":{\"id\":%d}}\n", id)
	for {
		var r struct {
			Return json.Number `json:"return"`
		}
		if dec.Decode(&r) != nil {
			return
		}
		if n, _ := r.Return.Int64(); int(n) == id {
			break
		}
	}

	fmt.Fprint(conn, `{"execute":"guest-network-get-interfaces"}`+"\n")
	var nif struct {
		Return []struct {
			Name string `json:"name"`
			IPs  []struct {
				Type string `json:"ip-address-type"`
				Addr string `json:"ip-address"`
			} `json:"ip-addresses"`
		} `json:"return"`
	}
	if dec.Decode(&nif) == nil {
		for _, iface := range nif.Return {
			if iface.Name == "lo" {
				continue
			}
			for _, a := range iface.IPs {
				if a.Type == "ipv4" && a.Addr != "127.0.0.1" {
					ip = a.Addr
					break
				}
			}
			if ip != "" {
				break
			}
		}
	}

	fmt.Fprint(conn, `{"execute":"guest-get-osinfo"}`+"\n")
	var osi struct {
		Return struct {
			PrettyName string `json:"pretty-name"`
		} `json:"return"`
	}
	if dec.Decode(&osi) == nil {
		osName = osi.Return.PrettyName
	}
	return
}
