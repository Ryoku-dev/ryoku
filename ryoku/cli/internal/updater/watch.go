package updater

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	i18n "ryoku-i18n"
)

// The run watches itself. Output alone cannot tell a quiet compile from a hung
// step: a C++ plugin build prints nothing for a minute while it works, and a
// `systemctl` call blocked on a dead unit prints nothing forever. So the
// heartbeat also walks the run's own process tree and reads its CPU time. Work
// that is printing or burning CPU is progress; neither, for long enough, is a
// stall -- and the newest command in the tree is the one to name.

const (
	quietAfter = 20 * time.Second
	stallAfter = 3 * time.Minute
)

// watchState is the watchdog's verdict, published with every heartbeat.
type watchState struct {
	State   string `json:"state"`             // working | quiet | stalled
	Current string `json:"current,omitempty"` // the newest command in the run's tree
	Quiet   int64  `json:"quiet"`             // seconds without output or CPU progress
	Pkg     bool   `json:"pkg,omitempty"`     // a package transaction is in the tree
}

type procInfo struct {
	pid, ppid int
	comm      string
	start     uint64 // clock ticks after boot
	cpu       uint64 // utime + stime, clock ticks
}

// readProc parses /proc/<pid>/stat. comm sits in parentheses and may itself
// hold spaces or parentheses, so the fields are read after the last ')'.
func readProc(pid int) (procInfo, bool) {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return procInfo{}, false
	}
	open, end := bytes.IndexByte(b, '('), bytes.LastIndexByte(b, ')')
	if open < 0 || end < open {
		return procInfo{}, false
	}
	f := strings.Fields(string(b[end+1:]))
	// after comm: state(0) ppid(1) ... utime(11) stime(12) ... starttime(19)
	if len(f) < 20 {
		return procInfo{}, false
	}
	pi := procInfo{pid: pid, comm: string(b[open+1 : end])}
	pi.ppid, _ = strconv.Atoi(f[1])
	ut, _ := strconv.ParseUint(f[11], 10, 64)
	st, _ := strconv.ParseUint(f[12], 10, 64)
	pi.cpu = ut + st
	pi.start, _ = strconv.ParseUint(f[19], 10, 64)
	return pi, true
}

// descendants lists every live process under root (root excluded).
func descendants(root int) []procInfo {
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	children := map[int][]procInfo{}
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		if pi, ok := readProc(pid); ok {
			children[pi.ppid] = append(children[pi.ppid], pi)
		}
	}
	var out []procInfo
	queue := []int{root}
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		for _, c := range children[next] {
			out = append(out, c)
			queue = append(queue, c.pid)
		}
	}
	return out
}

// packageManagers are the commands whose interruption can leave packages
// half-installed.
var packageManagers = map[string]bool{
	"pacman": true, "xbps-install": true, "xbps-remove": true,
	"dnf": true, "dnf5": true, "rpm": true,
	"yay": true, "paru": true, "makepkg": true, "flatpak": true,
}

// wrappers carry the real work rather than doing it; the command named to the
// user is the work they wrap.
var wrappers = map[string]bool{"sudo": true, "systemd-inhibit": true, "elogind-inhibit": true, "ryoku-host": true, "env": true, "sh": true, "bash": true, "script": true}

type watchdog struct {
	root       int
	lastCPU    uint64
	progressAt time.Time
}

func newWatchdog(root int) *watchdog { return &watchdog{root: root, progressAt: time.Now()} }

// sample walks the tree once. CPU counts as progress past a small floor, so the
// watchdog's own /proc walk and an idle wait loop do not read as work.
func (w *watchdog) sample(activityAtMs int64) watchState {
	tree := descendants(w.root)
	var cpu uint64
	if me, ok := readProc(w.root); ok {
		cpu = me.cpu
	}
	var newest procInfo
	pkg := false
	for _, pi := range tree {
		cpu += pi.cpu
		if packageManagers[pi.comm] {
			pkg = true
		}
		if !wrappers[pi.comm] && pi.start >= newest.start {
			newest = pi
		}
	}
	if w.lastCPU != 0 && cpu > w.lastCPU+5 {
		w.progressAt = time.Now()
	}
	w.lastCPU = cpu
	last := w.progressAt
	if a := time.UnixMilli(activityAtMs); activityAtMs > 0 && a.After(last) {
		last = a
	}
	return verdict(time.Since(last), commandLine(newest.pid), pkg)
}

// verdict maps the time since the last sign of progress to a state. pure.
func verdict(quiet time.Duration, current string, pkg bool) watchState {
	ws := watchState{State: "working", Current: current, Quiet: int64(quiet / time.Second), Pkg: pkg}
	switch {
	case quiet >= stallAfter:
		ws.State = "stalled"
	case quiet >= quietAfter:
		ws.State = "quiet"
	}
	return ws
}

// commandLine renders a pid's argv as a short, readable command: the binary's
// base name and its first few arguments, never more than 64 runes.
func commandLine(pid int) string {
	if pid <= 0 {
		return ""
	}
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline")
	if err != nil || len(b) == 0 {
		return ""
	}
	args := strings.Split(strings.TrimRight(string(b), "\x00"), "\x00")
	args[0] = filepath.Base(args[0])
	if len(args) > 4 {
		args = append(args[:4], "…")
	}
	return clip(strings.Join(args, " "), 64)
}

// isUpdateProcess reports whether pid is a live `ryoku update`, so a recycled
// pid is never mistaken for the run that published it.
func isUpdateProcess(pid int) bool {
	if pid <= 1 {
		return false
	}
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline")
	if err != nil {
		return false
	}
	args := strings.Split(strings.TrimRight(string(b), "\x00"), "\x00")
	return len(args) >= 2 && strings.HasPrefix(filepath.Base(args[0]), "ryoku") && args[1] == "update"
}

// RunOwnerLive reports whether the run-state phase is held by a live update.
// A document with no pid (a crashed run, or one an old build wrote outside an
// update) has no owner. The doctor's stale-run reconciler asks this.
var RunOwnerLive = func(pid int) bool { return isUpdateProcess(pid) }

// cancelRun is `ryoku update --cancel`: stop the run the run-state names. The
// update traps the signal, hands back what it quiesced (the shell, the sleep
// guard) and records the stop; a package transaction already committing is
// left to finish on its own rather than torn mid-write.
func cancelRun() error {
	st, ok := readState()
	if !ok || (st.Phase != "running" && st.Phase != "prompt" && st.Phase != "auth") {
		return fmt.Errorf(i18n.T("no update is running"))
	}
	if !isUpdateProcess(st.PID) {
		clearOrphanedRun(st, i18n.T("The update was no longer running."))
		return nil
	}
	if err := syscall.Kill(st.PID, syscall.SIGTERM); err != nil {
		return fmt.Errorf(i18n.T("could not stop the update (pid %d): %w"), st.PID, err)
	}
	for range 50 {
		if !isUpdateProcess(st.PID) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf(i18n.T("the update (pid %d) did not stop; it is still finishing a step"), st.PID)
}

// clearOrphanedRun settles a run-state whose owner is gone into an error the
// Hub can show and dismiss, keeping the steps it reached.
func clearOrphanedRun(st runState, why string) {
	st.Phase = "error"
	st.Error = why
	st.Prompt, st.Watch = nil, nil
	for i := range st.Steps {
		if st.Steps[i].State == stepRunning {
			st.Steps[i].State = stepFailed
		}
	}
	st.Beat = nowMs()
	writeState(st)
}

// terminateTree signals every descendant except package managers, which are
// left to complete or roll back their own transaction.
func terminateTree(root int) {
	for _, pi := range descendants(root) {
		if !packageManagers[pi.comm] {
			_ = syscall.Kill(pi.pid, syscall.SIGTERM)
		}
	}
}
