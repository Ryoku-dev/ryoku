package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

func TestIpcArgvRoutesShellCallsToReadyRuntime(t *testing.T) {
	previousShellDir := shellDir
	shellDir = ""
	t.Cleanup(func() {
		shellDir = previousShellDir
		runtimeIPCReadyFile.Store("")
	})
	resident := []string{"-c", "shell", "ipc", "call", "shell", "openSurface", "DP-1"}
	ready := filepath.Join(t.TempDir(), "ready")

	runtimeIPCReadyFile.Store("")
	if got := ipcArgv("shell", "shell", "openSurface", "DP-1"); !reflect.DeepEqual(got, resident) {
		t.Fatalf("no runtime argv = %v, want %v", got, resident)
	}

	runtimeIPCReadyFile.Store(ready)
	if got := ipcArgv("shell", "shell", "openSurface", "DP-1"); !reflect.DeepEqual(got, resident) {
		t.Fatalf("absent ready file argv = %v, want %v", got, resident)
	}

	if err := os.WriteFile(ready, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := ipcArgv("shell", "shell", "openSurface", "DP-1"); !reflect.DeepEqual(got, resident) {
		t.Fatalf("empty ready file argv = %v, want %v", got, resident)
	}

	pid := strconv.Itoa(os.Getpid())
	if err := os.WriteFile(ready, []byte(pid+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := []string{"ipc", "--pid", pid, "call", "shell", "openSurface", "DP-1"}
	if got := ipcArgv("shell", "shell", "openSurface", "DP-1"); !reflect.DeepEqual(got, want) {
		t.Fatalf("ready runtime argv = %v, want %v", got, want)
	}
	if got := ipcArgv("hub", "nav", "open"); !reflect.DeepEqual(got, []string{"-c", "hub", "ipc", "call", "nav", "open"}) {
		t.Fatalf("non-shell component was routed to the runtime: %v", got)
	}

	if err := os.WriteFile(ready, []byte("2147483600\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := ipcArgv("shell", "shell", "openSurface", "DP-1"); !reflect.DeepEqual(got, resident) {
		t.Fatalf("stale pid argv = %v, want %v", got, resident)
	}

	if err := os.WriteFile(ready, []byte(pid), 0o600); err != nil {
		t.Fatal(err)
	}
	d := &daemon{}
	run := &desktopRun{runtime: true, ready: ready}
	d.setRuntimeRun(run)
	d.clearRuntimeRun(run)
	if got := ipcArgv("shell", "shell", "openSurface", "DP-1"); !reflect.DeepEqual(got, resident) {
		t.Fatalf("stopped runtime argv = %v, want %v", got, resident)
	}
}
