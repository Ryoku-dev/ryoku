package main

import (
	"bufio"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// startLeader runs script as a process-group leader, the way the daemon starts
// a shell, reads the pids it prints (one per line, then "up"), and waits for
// the leader to exit, leaving its children behind as Quickshell does on SIGTERM.
func startLeader(t *testing.T, script string) (int, []int) {
	t.Helper()
	cmd := exec.Command("sh", "-c", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var pids []int
	sc := bufio.NewScanner(out)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "up" {
			break
		}
		pid, err := strconv.Atoi(line)
		if err != nil {
			t.Fatalf("leader printed %q", line)
		}
		pids = append(pids, pid)
	}
	_ = cmd.Wait()
	t.Cleanup(func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		for _, pid := range pids {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})
	return cmd.Process.Pid, pids
}

func TestReapProcessGroupEndsHelpersTheShellLeft(t *testing.T) {
	old := shellGroupGrace
	shellGroupGrace = 300 * time.Millisecond
	t.Cleanup(func() { shellGroupGrace = old })

	// A plain watcher, and one that ignores SIGTERM and needs the SIGKILL.
	leader, helpers := startLeader(t, `
		sleep 300 >/dev/null 2>&1 & echo $!
		sh -c 'trap "" TERM; while :; do sleep 1; done' >/dev/null 2>&1 & echo $!
		sleep 0.2; echo up`)
	for _, pid := range helpers {
		if !pidAlive(pid) {
			t.Fatalf("helper %d should outlive its leader before the reap", pid)
		}
	}

	reapProcessGroup(leader)

	if left := waitGone(helpers, 2*time.Second); len(left) > 0 {
		t.Fatalf("helpers %v survived the shell's group reap", left)
	}
}

// Apps opened from the shell are detached into their own session and must
// survive a reload.
func TestReapProcessGroupLeavesDetachedSessions(t *testing.T) {
	old := shellGroupGrace
	shellGroupGrace = 300 * time.Millisecond
	t.Cleanup(func() { shellGroupGrace = old })

	leader, pids := startLeader(t, `
		setsid sleep 300 >/dev/null 2>&1 & echo $!
		sleep 300 >/dev/null 2>&1 & echo $!
		sleep 0.2; echo up`)
	app, helper := pids[0], pids[1]

	reapProcessGroup(leader)

	if left := waitGone([]int{helper}, 2*time.Second); len(left) > 0 {
		t.Fatal("the shell's own helper survived the reap")
	}
	if !pidAlive(app) {
		t.Fatal("a detached app was killed with the shell")
	}
}
