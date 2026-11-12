package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// A lock held by this process proves nothing: POSIX record locks belong to the
// process, so a same-process probe always succeeds and would pass whatever the
// prune does. These tests hold the lock from a child, the way quickshell does.
const lockHolderEnv = "RYOKU_TEST_HOLD_QS_LOCK"

func holdLockForever(path string) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		os.Exit(1)
	}
	rec := syscall.Flock_t{Type: syscall.F_WRLCK, Whence: io.SeekStart}
	if err := syscall.FcntlFlock(f.Fd(), syscall.F_SETLK, &rec); err != nil {
		os.Exit(1)
	}
	// Announce the lock is held, then park until the test kills us.
	_, _ = os.Stdout.WriteString("locked\n")
	time.Sleep(time.Minute)
}

// startLockHolder returns once a child process holds a record lock on path.
func startLockHolder(t *testing.T, path string) {
	t.Helper()
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), lockHolderEnv+"="+path)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
	buf := make([]byte, len("locked\n"))
	if _, err := io.ReadFull(out, buf); err != nil {
		t.Fatalf("lock holder never took the lock: %v", err)
	}
}

// The bug this pins: quickshell takes a POSIX record lock, the janitor probed
// with flock, and the two families do not see each other, so a live instance
// read as dead and its ipc.sock was deleted under it.
func TestPruneKeepsAnInstanceHoldingARecordLock(t *testing.T) {
	rt := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", rt)
	root := filepath.Join(rt, "quickshell", "by-id")

	live := filepath.Join(root, "live")
	if err := os.MkdirAll(live, 0o755); err != nil {
		t.Fatal(err)
	}
	startLockHolder(t, filepath.Join(live, qsInstanceLock))
	sock := filepath.Join(live, "ipc.sock")
	if err := os.WriteFile(sock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	big := filepath.Join(live, "log.log")
	os.WriteFile(big, make([]byte, qsLogCap+1), 0o644)
	small := filepath.Join(live, "log.qslog")
	os.WriteFile(small, []byte("keep"), 0o644)

	pruneQuickshellRuntime()

	if _, err := os.Stat(sock); err != nil {
		t.Fatal("a live instance keeps its socket: every shell verb dies without it")
	}
	if st, err := os.Stat(big); err != nil || st.Size() != 0 {
		t.Fatalf("an oversized live log must be truncated, got %v %v", st, err)
	}
	if b, _ := os.ReadFile(small); string(b) != "keep" {
		t.Fatal("a small live log must be left alone")
	}
}

func TestPruneRemovesAnInstanceNobodyHolds(t *testing.T) {
	rt := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", rt)
	dead := filepath.Join(rt, "quickshell", "by-id", "dead")
	if err := os.MkdirAll(dead, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dead, qsInstanceLock), nil, 0o644)
	os.WriteFile(filepath.Join(dead, "log.log"), []byte("x"), 0o644)

	pruneQuickshellRuntime()

	if _, err := os.Stat(dead); !os.IsNotExist(err) {
		t.Fatal("a dead instance dir must be removed")
	}
}

// An instance that has not written its lock yet is still live, and the by-pid
// link naming it is the proof.
func TestPruneKeepsAStartingInstanceALivePidClaims(t *testing.T) {
	rt := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", rt)
	starting := filepath.Join(rt, "quickshell", "by-id", "starting")
	byPid := filepath.Join(rt, "quickshell", "by-pid")
	if err := os.MkdirAll(starting, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(byPid, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(starting, filepath.Join(byPid, strconv.Itoa(os.Getpid()))); err != nil {
		t.Fatal(err)
	}

	pruneQuickshellRuntime()

	if _, err := os.Stat(starting); err != nil {
		t.Fatal("an instance a live process claims must survive a missing lock file")
	}
}

func TestPruneClearsDanglingIndexLinks(t *testing.T) {
	rt := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", rt)
	root := filepath.Join(rt, "quickshell", "by-id")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	live := filepath.Join(root, "live")
	if err := os.MkdirAll(live, 0o755); err != nil {
		t.Fatal(err)
	}
	startLockHolder(t, filepath.Join(live, qsInstanceLock))

	byPid := filepath.Join(rt, "quickshell", "by-pid")
	if err := os.MkdirAll(byPid, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(byPid, "4242")
	if err := os.Symlink(filepath.Join(root, "gone"), stale); err != nil {
		t.Fatal(err)
	}
	kept := filepath.Join(byPid, "4243")
	if err := os.Symlink(live, kept); err != nil {
		t.Fatal(err)
	}

	pruneQuickshellRuntime()

	if _, err := os.Lstat(stale); !os.IsNotExist(err) {
		t.Fatal("a link to a gone instance must be removed")
	}
	if _, err := os.Lstat(kept); err != nil {
		t.Fatal("a link to a live instance must be left alone")
	}
}
