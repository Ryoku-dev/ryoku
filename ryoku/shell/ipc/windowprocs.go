package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	wm "ryoku-wm"
)

// windowKillGrace is how long a program gets to close after SIGTERM before
// SIGKILL. A variable so tests can shorten it.
var windowKillGrace = 3 * time.Second

const (
	windowReasonNoPid     = "The compositor does not say which program owns this window."
	windowReasonSystem    = "That window belongs to the system itself."
	windowReasonDesktop   = "That window belongs to the desktop itself."
	windowReasonXwayland  = "It runs the windows of every X11 app. Close the window instead."
	windowReasonOtherUser = "It belongs to another user."
	windowReasonClosed    = "That program has already closed."
	windowReasonGone      = "That window is already gone."
)

type windowMemoryEntry struct {
	ID       string `json:"id"`
	Bytes    uint64 `json:"bytes"`
	Shared   int    `json:"shared"`
	Killable bool   `json:"killable"`
	Reason   string `json:"reason"`
}

func (d *daemon) registerWindowProcs() {
	d.registerCall("windows.memory", func(raw json.RawMessage) (any, error) {
		var a struct {
			IDs []string `json:"ids"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		return map[string]any{"windows": d.windowMemory(a.IDs, "/proc", os.Getpid())}, nil
	})
	// A scan walks all of /proc; a kill must never wait behind one.
	d.setCallLane("windows.memory", "scan")
	d.registerCall("windows.kill", func(raw json.RawMessage) (any, error) {
		var a struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		if err := d.killWindow(a.ID, "/proc", os.Getpid()); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})
}

func (d *daemon) windowsSnapshot() []wm.Window {
	d.wmMu.Lock()
	defer d.wmMu.Unlock()
	return append([]wm.Window(nil), d.wmWindows...)
}

// windowMemory answers for the requested windows, or every window when ids is
// empty. Unknown ids are left out.
func (d *daemon) windowMemory(ids []string, procRoot string, self int) []windowMemoryEntry {
	windows := d.windowsSnapshot()
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	owners := make(map[int]int, len(windows))
	for _, w := range windows {
		if w.Pid > 0 {
			owners[w.Pid]++
		}
	}
	var children map[int][]int
	guards := make(map[int]string)
	out := make([]windowMemoryEntry, 0, len(windows))
	for _, w := range windows {
		if len(want) > 0 && !want[w.ID] {
			continue
		}
		if w.Pid <= 0 {
			out = append(out, windowMemoryEntry{ID: w.ID, Shared: 1, Reason: windowReasonNoPid})
			continue
		}
		if children == nil {
			children = procChildren(procRoot)
		}
		reason, ok := guards[w.Pid]
		if !ok {
			reason = windowKillGuard(procRoot, self, w.Pid)
			guards[w.Pid] = reason
		}
		out = append(out, windowMemoryEntry{
			ID:       w.ID,
			Bytes:    processTreeMemory(procRoot, w.Pid, children, owners),
			Shared:   owners[w.Pid],
			Killable: reason == "",
			Reason:   reason,
		})
	}
	return out
}

// processTreeMemory sums root and its descendants. A descendant that owns a
// window of its own is skipped with its subtree: that memory is shown on its
// window instead.
func processTreeMemory(procRoot string, root int, children map[int][]int, owners map[int]int) uint64 {
	var total uint64
	seen := map[int]bool{root: true}
	stack := []int{root}
	for len(stack) > 0 {
		pid := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		total += procMemory(procRoot, pid)
		for _, c := range children[pid] {
			if seen[c] || owners[c] > 0 {
				continue
			}
			seen[c] = true
			stack = append(stack, c)
		}
	}
	return total
}

// procMemory is what a process holds in RAM, by proportional set size so pages
// shared between helpers are not counted once per helper. Swapped-out pages
// are left out, as a system monitor leaves them out. smaps_rollup is
// unreadable for some processes, so resident size from status is the fallback.
func procMemory(procRoot string, pid int) uint64 {
	dir := filepath.Join(procRoot, strconv.Itoa(pid))
	if b, err := os.ReadFile(filepath.Join(dir, "smaps_rollup")); err == nil {
		if pss, ok := readKBField(strings.NewReader(string(b)), "Pss"); ok {
			return pss
		}
	}
	f, err := os.Open(filepath.Join(dir, "status"))
	if err != nil {
		return 0
	}
	defer f.Close()
	rss, _ := readKBField(f, "VmRSS")
	return rss
}

// procChildren maps every pid under procRoot to its children.
func procChildren(procRoot string) map[int][]int {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil
	}
	children := make(map[int][]int)
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue
		}
		_, ppid, ok := readProcStat(procRoot, pid)
		if !ok {
			continue
		}
		children[ppid] = append(children[ppid], pid)
	}
	return children
}

func readProcStat(procRoot string, pid int) (comm string, ppid int, ok bool) {
	b, err := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(pid), "stat"))
	if err != nil {
		return "", 0, false
	}
	return parseProcStat(string(b))
}

// parseProcStat reads comm and ppid from /proc/<pid>/stat. comm may hold
// spaces and parentheses, so the fields resume after the last ')'.
func parseProcStat(stat string) (comm string, ppid int, ok bool) {
	open := strings.IndexByte(stat, '(')
	end := strings.LastIndexByte(stat, ')')
	if open < 0 || end < open {
		return "", 0, false
	}
	fields := strings.Fields(stat[end+1:])
	if len(fields) < 2 {
		return "", 0, false
	}
	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return "", 0, false
	}
	return stat[open+1 : end], ppid, true
}

func readProcRealUID(procRoot string, pid int) (int, bool) {
	b, err := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(pid), "status"))
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(line, "Uid:") {
			continue
		}
		fields := strings.Fields(line[len("Uid:"):])
		if len(fields) == 0 {
			return 0, false
		}
		uid, err := strconv.Atoi(fields[0])
		return uid, err == nil
	}
	return 0, false
}

// windowKillGuard returns why the program owning pid must not be killed, or
// "" when it may be.
func windowKillGuard(procRoot string, self, pid int) string {
	switch {
	case pid <= 0:
		return windowReasonNoPid
	case pid == 1:
		return windowReasonSystem
	case pid == self:
		return windowReasonDesktop
	}
	comm, ppid, ok := readProcStat(procRoot, pid)
	if !ok {
		return windowReasonClosed
	}
	if ppid == self {
		return windowReasonDesktop
	}
	// comm is cut to 15 bytes, so xwayland-satellite reads xwayland-satell.
	if strings.HasPrefix(strings.ToLower(comm), "xwayland") {
		return windowReasonXwayland
	}
	uid, ok := readProcRealUID(procRoot, pid)
	if !ok {
		return windowReasonClosed
	}
	if uid != os.Getuid() {
		return windowReasonOtherUser
	}
	return ""
}

// killWindow resolves the window's pid from the compositor's list now, so a
// caller can never name an arbitrary process.
func (d *daemon) killWindow(id, procRoot string, self int) error {
	pid, found := 0, false
	for _, w := range d.windowsSnapshot() {
		if w.ID == id {
			pid, found = w.Pid, true
			break
		}
	}
	if !found {
		return errors.New(windowReasonGone)
	}
	if reason := windowKillGuard(procRoot, self, pid); reason != "" {
		return errors.New(reason)
	}
	return killWindowProcess(pid, windowKillGrace)
}

// killWindowProcess sends SIGTERM and, if the program is still running after
// grace, SIGKILL. Both go through one pidfd, so a pid reused by another
// program in between is never signalled.
func killWindowProcess(pid int, grace time.Duration) error {
	fd, err := unix.PidfdOpen(pid, 0)
	if err != nil {
		if errors.Is(err, unix.ESRCH) {
			return errors.New(windowReasonClosed)
		}
		return errors.New("Ryoku could not reach that program.")
	}
	if err := unix.PidfdSendSignal(fd, unix.SIGTERM, nil, 0); err != nil {
		unix.Close(fd)
		if errors.Is(err, unix.ESRCH) {
			return errors.New(windowReasonClosed)
		}
		return errors.New("Ryoku could not stop that program.")
	}
	go func() {
		defer unix.Close(fd)
		if !waitPidfdExit(fd, grace) {
			_ = unix.PidfdSendSignal(fd, unix.SIGKILL, nil, 0)
		}
	}()
	return nil
}

// waitPidfdExit reports whether the process exits within grace. A pidfd turns
// readable when its process exits.
func waitPidfdExit(fd int, grace time.Duration) bool {
	deadline := time.Now().Add(grace)
	for {
		left := time.Until(deadline)
		if left < 0 {
			left = 0
		}
		pfds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		n, err := unix.Poll(pfds, int((left+time.Millisecond-1)/time.Millisecond))
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return false
		}
		return n > 0
	}
}
