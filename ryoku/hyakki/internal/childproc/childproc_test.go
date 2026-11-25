package childproc

import (
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestTerminateReapedProcessGroup(t *testing.T) {
	const helper = "HYAKKI_CHILDPROC_TERMINATE_HELPER"
	ready := filepath.Join(os.Getenv(helper+"_DIR"), "ready")
	signaled := filepath.Join(os.Getenv(helper+"_DIR"), "signaled")
	switch os.Getenv(helper) {
	case "leader":
		follower := exec.Command(os.Args[0], "-test.run=^TestTerminateReapedProcessGroup$")
		follower.Env = append(os.Environ(), helper+"=follower")
		if err := follower.Start(); err != nil {
			t.Fatal(err)
		}
		for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
			if _, err := os.Stat(ready); err == nil {
				return
			}
		}
		t.Fatal("follower did not join the process group")
	case "follower":
		term := make(chan os.Signal, 1)
		signal.Notify(term, syscall.SIGTERM)
		defer signal.Stop(term)
		if err := os.WriteFile(ready, nil, 0600); err != nil {
			t.Fatal(err)
		}
		select {
		case <-term:
			if err := os.WriteFile(signaled, nil, 0600); err != nil {
				t.Fatal(err)
			}
		case <-time.After(30 * time.Second):
			return
		}
		time.Sleep(30 * time.Second)
		return
	}

	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestTerminateReapedProcessGroup$")
	cmd.Env = append(os.Environ(), helper+"=leader", helper+"_DIR="+dir)
	cmd.SysProcAttr = Attr()
	if err := cmd.Run(); err != nil {
		t.Fatalf("process group leader: %v", err)
	}
	group := -cmd.Process.Pid
	// The live follower holds the reaped leader's group ID, so it cannot be reused.
	if err := syscall.Kill(group, 0); err != nil {
		t.Fatalf("reaped process group has no follower: %v", err)
	}
	t.Cleanup(func() { _ = syscall.Kill(group, syscall.SIGKILL) })

	exited := make(chan struct{})
	close(exited)
	Terminate(cmd, exited, time.Second)
	time.Sleep(150 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(dir, "signaled")); err == nil {
		t.Fatal("Terminate sent SIGTERM to a reaped child's process group")
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := syscall.Kill(group, 0); err != nil {
		t.Fatalf("Terminate killed the live follower: %v", err)
	}
}
