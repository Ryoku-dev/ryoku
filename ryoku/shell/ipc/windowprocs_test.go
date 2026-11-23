package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	wm "ryoku-wm"
)

type fakeProc struct {
	pid, ppid int
	comm      string
	uid       int
	rssKB     int
	swapKB    int
	pssKB     int // 0 leaves smaps_rollup out, as when it is unreadable
	swapPssKB int
}

func writeFakeProcs(t *testing.T, procs ...fakeProc) string {
	t.Helper()
	root := t.TempDir()
	for _, p := range procs {
		dir := filepath.Join(root, strconv.Itoa(p.pid))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		stat := fmt.Sprintf("%d (%s) S %d %d 0 0 -1 4194560 100 0 0 0\n", p.pid, p.comm, p.ppid, p.pid)
		status := fmt.Sprintf("Name:\t%s\nUid:\t%d\t%d\t%d\t%d\nVmRSS:\t%d kB\nVmSwap:\t%d kB\n",
			p.comm, p.uid, p.uid, p.uid, p.uid, p.rssKB, p.swapKB)
		files := map[string]string{"stat": stat, "status": status}
		if p.pssKB > 0 {
			files["smaps_rollup"] = fmt.Sprintf("00400000-7fff0000 ---p 00000000 00:00 0 [rollup]\nRss:\t%d kB\nPss:\t%d kB\nPss_Anon:\t1 kB\nSwap:\t%d kB\nSwapPss:\t%d kB\n",
				p.rssKB, p.pssKB, p.swapKB, p.swapPssKB)
		}
		for name, body := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

func entriesByID(entries []windowMemoryEntry) map[string]windowMemoryEntry {
	m := make(map[string]windowMemoryEntry, len(entries))
	for _, e := range entries {
		m[e.ID] = e
	}
	return m
}

func TestWindowMemorySumsHelpersButNotOtherWindows(t *testing.T) {
	uid := os.Getuid()
	root := writeFakeProcs(t,
		// browser: smaps_rollup wins over its status numbers
		fakeProc{pid: 100, ppid: 50, comm: "browser", uid: uid, rssKB: 999999, swapKB: 999999, pssKB: 1000, swapPssKB: 100},
		// a helper with no readable rollup falls back to status
		fakeProc{pid: 101, ppid: 100, comm: "Web Content", uid: uid, rssKB: 200, swapKB: 20},
		// a comm that would fake ppid 1 if read up to the first ')'
		fakeProc{pid: 102, ppid: 101, comm: "gpu) S 1 (x", uid: uid, rssKB: 30, swapKB: 3},
		// a child of the browser that owns its own window
		fakeProc{pid: 200, ppid: 100, comm: "editor", uid: uid, pssKB: 500, swapPssKB: 50},
		fakeProc{pid: 201, ppid: 200, comm: "lsp", uid: uid, pssKB: 40},
		fakeProc{pid: 300, ppid: 50, comm: "other", uid: uid, pssKB: 7},
	)
	d := &daemon{wmWindows: []wm.Window{
		{ID: "a", Pid: 100},
		{ID: "b", Pid: 200},
		{ID: "c", Pid: 200},
		{ID: "x", Pid: 0},
	}}

	all := entriesByID(d.windowMemory(nil, root, 9999))
	if len(all) != 4 {
		t.Fatalf("every window should be answered, got %+v", all)
	}
	if got, want := all["a"].Bytes, uint64(1000+100+200+20+30+3)<<10; got != want {
		t.Fatalf("browser memory = %d, want %d (itself and helpers, not the editor)", got, want)
	}
	if got, want := all["b"].Bytes, uint64(500+50+40)<<10; got != want {
		t.Fatalf("editor memory = %d, want %d", got, want)
	}
	if all["a"].Shared != 1 || all["b"].Shared != 2 || all["c"].Shared != 2 {
		t.Fatalf("shared counts wrong: %+v", all)
	}
	if !all["a"].Killable || all["a"].Reason != "" {
		t.Fatalf("an ordinary program should be killable: %+v", all["a"])
	}
	if x := all["x"]; x.Bytes != 0 || x.Killable || x.Reason == "" {
		t.Fatalf("a window without a pid must not be killable: %+v", x)
	}

	some := d.windowMemory([]string{"b", "missing"}, root, 9999)
	if len(some) != 1 || some[0].ID != "b" {
		t.Fatalf("only known requested windows should be answered, got %+v", some)
	}
}

func TestWindowStatParsesTrickyComm(t *testing.T) {
	comm, ppid, ok := parseProcStat("4242 (a (b) c) S) R 77 4242 0 0")
	if !ok || ppid != 77 || comm != "a (b) c) S" {
		t.Fatalf("got comm %q ppid %d ok %v", comm, ppid, ok)
	}
}

func TestWindowKillGuard(t *testing.T) {
	uid := os.Getuid()
	self := 500
	root := writeFakeProcs(t,
		fakeProc{pid: 500, ppid: 1, comm: "ryoku-shell", uid: uid},
		fakeProc{pid: 501, ppid: 500, comm: "qs", uid: uid},
		fakeProc{pid: 600, ppid: 1, comm: "Xwayland", uid: uid},
		fakeProc{pid: 601, ppid: 1, comm: "xwayland-satell", uid: uid},
		fakeProc{pid: 602, ppid: 1, comm: "firefox", uid: uid + 1},
		fakeProc{pid: 603, ppid: 1, comm: "firefox", uid: uid},
	)
	cases := []struct {
		name    string
		pid     int
		allowed bool
	}{
		{"unknown owner", 0, false},
		{"init", 1, false},
		{"the daemon", 500, false},
		{"a surface the daemon runs", 501, false},
		{"xwayland", 600, false},
		{"xwayland-satellite", 601, false},
		{"another user", 602, false},
		{"already closed", 777, false},
		{"an ordinary program", 603, true},
	}
	for _, c := range cases {
		reason := windowKillGuard(root, self, c.pid)
		if (reason == "") != c.allowed {
			t.Errorf("%s: reason %q, allowed want %v", c.name, reason, c.allowed)
		}
	}
}

func startWindowChild(t *testing.T, name string, args ...string) (*exec.Cmd, <-chan error) {
	t.Helper()
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	})
	return cmd, done
}

func waitWindowChildSignal(t *testing.T, done <-chan error, want syscall.Signal) {
	t.Helper()
	select {
	case err := <-done:
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("child ended with %v, want signal %v", err, want)
		}
		ws, ok := ee.Sys().(syscall.WaitStatus)
		if !ok || !ws.Signaled() || ws.Signal() != want {
			t.Fatalf("child ended with %v, want signal %v", err, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the program is still running")
	}
}

func TestKillWindowEndsProgram(t *testing.T) {
	cmd, done := startWindowChild(t, "sleep", "30")
	d := &daemon{wmWindows: []wm.Window{{ID: "w", Pid: cmd.Process.Pid}}}
	// The test process is the child's parent, so stand in a different daemon pid.
	if err := d.killWindow("w", "/proc", -1); err != nil {
		t.Fatal(err)
	}
	waitWindowChildSignal(t, done, syscall.SIGTERM)

	if err := d.killWindow("gone", "/proc", -1); err == nil {
		t.Fatal("an unknown window was accepted")
	}
}

func TestKillWindowRefusesTheDesktop(t *testing.T) {
	cmd, done := startWindowChild(t, "sleep", "30")
	d := &daemon{wmWindows: []wm.Window{{ID: "w", Pid: cmd.Process.Pid}}}
	if err := d.killWindow("w", "/proc", os.Getpid()); err == nil {
		t.Fatal("a surface the daemon runs was killed")
	}
	select {
	case err := <-done:
		t.Fatalf("the refused program ended: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestKillWindowEscalatesWhenTermIsIgnored(t *testing.T) {
	old := windowKillGrace
	windowKillGrace = 200 * time.Millisecond
	t.Cleanup(func() { windowKillGrace = old })

	// exec keeps one process with TERM ignored, so SIGKILL leaves nothing behind.
	cmd, done := startWindowChild(t, "sh", "-c", `trap "" TERM; exec sleep 30`)
	commPath := filepath.Join("/proc", strconv.Itoa(cmd.Process.Pid), "comm")
	deadline := time.Now().Add(5 * time.Second)
	for {
		b, _ := os.ReadFile(commPath)
		if strings.TrimSpace(string(b)) == "sleep" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the child never reached sleep")
		}
		time.Sleep(10 * time.Millisecond)
	}

	d := &daemon{wmWindows: []wm.Window{{ID: "w", Pid: cmd.Process.Pid}}}
	start := time.Now()
	if err := d.killWindow("w", "/proc", -1); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Fatal("the kill waited for the program instead of answering at once")
	}
	waitWindowChildSignal(t, done, syscall.SIGKILL)
}
