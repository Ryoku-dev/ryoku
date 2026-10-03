package main

import (
	"bufio"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// The duplicate-desktop bug: a leftover quickshell rendering the shell was only
// recognised when its argv matched the running daemon's mode character for
// character, so a packaged daemon ("-c shell") never reaped a checkout's
// leftover ("-p .../quickshell/shell") and both drew a desktop.
func TestSelectsComponentMatchesEitherSelectorForm(t *testing.T) {
	byName := []string{"-c", "shell"}
	byPath := []string{"-p", "/home/u/.config/quickshell/shell"}

	for _, tc := range []struct {
		name string
		argv []string
		sel  []string
		want bool
	}{
		{"same mode, by name", []string{"qs", "-c", "shell"}, byName, true},
		{"packaged daemon finds a checkout leftover", []string{"qs", "-p", "/repo/ryoku/shell/quickshell/shell"}, byName, true},
		{"checkout daemon finds a packaged leftover", []string{"qs", "-c", "shell"}, byPath, true},
		{"long flags", []string{"qs", "--config", "shell"}, byName, true},
		{"the other binary name", []string{"/usr/bin/quickshell", "-c", "shell"}, byName, true},
		{"absolute qs path", []string{"/usr/bin/qs", "-c", "shell"}, byName, true},
		{"another config is not ours", []string{"qs", "-c", "ryovm"}, byName, false},
		{"an app path is not ours", []string{"qs", "-p", "/home/u/.config/quickshell/ryovm"}, byName, false},
		{"an ipc client is not an instance", []string{"qs", "-c", "shell", "ipc", "call", "bar", "hide"}, byName, false},
		{"a kill client is not an instance", []string{"qs", "-c", "shell", "kill"}, byName, false},
		{"instance flags after the selector still draw a desktop", []string{"qs", "-c", "shell", "--no-duplicate", "--log-times"}, byName, true},
		{"a daemonized instance is still an instance", []string{"qs", "-c", "shell", "-d"}, byName, true},
		{"an option value is not a subcommand", []string{"qs", "-c", "shell", "--log-rules", "qt.*=false"}, byName, true},
		{"a flagged checkout leftover is still a stray", []string{"qs", "-p", "/repo/ryoku/shell/quickshell/shell", "--no-duplicate"}, byName, true},
		{"some other program", []string{"kitty", "-c", "shell"}, byName, false},
		{"no selector", []string{"qs", "ipc", "call"}, byName, false},
	} {
		if got := selectsComponent(tc.argv, tc.sel); got != tc.want {
			t.Errorf("%s: selectsComponent(%v, %v) = %v, want %v", tc.name, tc.argv, tc.sel, got, tc.want)
		}
	}
}

// A watcher spawned as `bash -c "... inotifywait"` is a grandchild of the
// surface: killing only the surface reparents it to init, where it leaks for
// the rest of the session. The reap must take the whole subtree.
func TestWithDescendantsIncludesChildProcesses(t *testing.T) {
	nested := exec.Command("bash", "-c", "sleep 60 & sleep 60")
	nested.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := nested.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = syscall.Kill(-nested.Process.Pid, syscall.SIGKILL)
		_ = nested.Wait()
	}()
	time.Sleep(200 * time.Millisecond)

	all := withDescendants([]int{nested.Process.Pid})
	if len(all) < 2 {
		t.Fatalf("expected the backgrounded child too, got %v", all)
	}
	root := false
	for _, pid := range all {
		if pid == nested.Process.Pid {
			root = true
		}
	}
	if !root {
		t.Fatal("the root pid must stay in the set")
	}
}

func TestPidAliveTracksRealProcesses(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	if !pidAlive(pid) {
		t.Fatal("a running process must read as alive")
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait() // reaped, so not left a zombie
	if pidAlive(pid) {
		t.Fatal("a reaped process must read as gone")
	}
}

// waitGone is what turns "asked it to stop" into "it is stopped": the reap used
// to signal and return, so the replacement drew over a surface that was still up.
func TestWaitGoneReturnsSurvivors(t *testing.T) {
	quick := exec.Command("sleep", "0.1")
	stubborn := exec.Command("sleep", "30")
	for _, c := range []*exec.Cmd{quick, stubborn} {
		if err := c.Start(); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_ = stubborn.Process.Kill()
		_ = stubborn.Wait()
	})
	go func() { _ = quick.Wait() }()

	left := waitGone([]int{quick.Process.Pid, stubborn.Process.Pid}, 2*time.Second)
	if len(left) != 1 || left[0] != stubborn.Process.Pid {
		t.Fatalf("survivors = %v, want just the stubborn pid %d", left, stubborn.Process.Pid)
	}
}

// A process that ignores SIGTERM is exactly the case that left a second desktop
// on screen forever: the reap must escalate.
func TestSignalAllEscalatesToKill(t *testing.T) {
	// a loop, not a bare sleep: a shell running one final command execs it and
	// drops the trap with itself, so the process would honour SIGTERM after all.
	cmd := exec.Command("sh", "-c", "trap '' TERM; echo armed; while :; do sleep 0.2; done")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	// signal only once the trap is installed, or the subject dies to the very
	// SIGTERM it is meant to ignore.
	if _, err := bufio.NewReader(out).ReadString('\n'); err != nil {
		t.Fatalf("subject never armed: %v", err)
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()

	signalAll([]int{pid}, syscall.SIGTERM)
	if left := waitGone([]int{pid}, 700*time.Millisecond); len(left) != 1 {
		t.Fatal("a process ignoring SIGTERM must survive it; the test lost its subject")
	}
	signalAll([]int{pid}, syscall.SIGKILL)
	if left := waitGone([]int{pid}, 2*time.Second); len(left) != 0 {
		t.Fatalf("SIGKILL left %v alive", left)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the killed process was never reaped")
	}
}

func TestOwnedPidsExcludesOurOwnChildFromTheReap(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	d := &daemon{proc: map[string]*exec.Cmd{"shell": cmd}}
	mine := d.ownedPids()
	if !mine[cmd.Process.Pid] {
		t.Fatalf("the supervised child %d must be excluded from the reap", cmd.Process.Pid)
	}
	if mine[os.Getpid()] {
		t.Fatal("only supervised children belong in the owned set")
	}
}

// The lock is the guard that keeps a second daemon from supervising a second
// desktop when the socket file is missing or was cleaned away.
func TestHoldDaemonLockIsExclusive(t *testing.T) {
	path := t.TempDir() + "/ryoku-shell.sock.lock"
	first, err := holdDaemonLock(path)
	if err != nil {
		t.Fatalf("first daemon could not take the lock: %v", err)
	}
	start := time.Now()
	if _, err := holdDaemonLock(path); err == nil {
		t.Fatal("a second daemon took the lock while the first held it")
	}
	if waited := time.Since(start); waited < 2*time.Second {
		t.Fatalf("gave up after %v; it should retry for about 3s so a take-over can win", waited)
	}

	// releasing it (as process exit does) lets the next daemon in.
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := holdDaemonLock(path)
	if err != nil {
		t.Fatalf("lock not released with the file: %v", err)
	}
	second.Close()
}

func TestTailLineReadsTheLastRealLine(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/surface.log"
	body := "INFO: starting\n\nquickshell: symbol lookup error: undefined symbol: x, version Qt_6_PRIVATE_API\n\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := tailLine(path); got != "quickshell: symbol lookup error: undefined symbol: x, version Qt_6_PRIVATE_API" {
		t.Fatalf("tailLine = %q", got)
	}
	if got := tailLine(dir + "/missing.log"); got != "" {
		t.Fatalf("a missing log has no line, got %q", got)
	}
}
