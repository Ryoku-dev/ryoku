package main

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
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

func fakeQs(t *testing.T, reply func(ctx context.Context, attempt int) ([]byte, []byte, error)) func() [][]string {
	t.Helper()
	prevRun, prevTimeout, prevWindow, prevEvery := qsRun, ipcAttemptTimeout, ipcRetryWindow, ipcRetryEvery
	prevShellDir := shellDir
	shellDir = ""
	runtimeIPCReadyFile.Store("")
	ipcAttemptTimeout, ipcRetryWindow, ipcRetryEvery = 200*time.Millisecond, 300*time.Millisecond, 10*time.Millisecond
	var mu sync.Mutex
	var seen [][]string
	qsRun = func(ctx context.Context, argv []string) ([]byte, []byte, error) {
		mu.Lock()
		seen = append(seen, append([]string(nil), argv...))
		n := len(seen)
		mu.Unlock()
		return reply(ctx, n)
	}
	t.Cleanup(func() {
		qsRun, ipcAttemptTimeout, ipcRetryWindow, ipcRetryEvery = prevRun, prevTimeout, prevWindow, prevEvery
		shellDir = prevShellDir
	})
	return func() [][]string {
		mu.Lock()
		defer mu.Unlock()
		return seen
	}
}

func TestQsIpcMissingTargetIsFinal(t *testing.T) {
	for _, msg := range []string{"Target not found.", "function not found.", "PROPERTY NOT FOUND."} {
		calls := fakeQs(t, func(context.Context, int) ([]byte, []byte, error) {
			return nil, []byte(msg + "\n"), nil
		})
		_, err := qsIpc(ipcArgv("shell", "theme", "reload"))
		var ierr *ipcError
		if !errors.As(err, &ierr) || ierr.kind != ipcMissing {
			t.Fatalf("%q: err = %v, want a missing-target error", msg, err)
		}
		if n := len(calls()); n != 1 {
			t.Fatalf("%q: qs ran %d times, want 1 (no retry)", msg, n)
		}
		if got := ipcCallN("shell", "theme", "reload"); !strings.HasPrefix(got, "err ") {
			t.Fatalf("%q: ipcCallN = %q, want an err reply", msg, got)
		}
	}
}

func TestQsIpcRetriesNotRunningWithinWindow(t *testing.T) {
	calls := fakeQs(t, func(context.Context, int) ([]byte, []byte, error) {
		return nil, []byte("No running instances for shell\n"), errors.New("exit status 255")
	})
	start := time.Now()
	_, err := qsIpc(ipcArgv("shell", "shell", "openSurface", "", "launcher"))
	elapsed := time.Since(start)
	var ierr *ipcError
	if !errors.As(err, &ierr) || ierr.kind != ipcNotRunning {
		t.Fatalf("err = %v, want not-running", err)
	}
	if n := len(calls()); n < 2 {
		t.Fatalf("qs ran %d times, want retries while the shell is down", n)
	}
	if elapsed > ipcRetryWindow+ipcAttemptTimeout {
		t.Fatalf("gave up after %v, want within the %v retry window", elapsed, ipcRetryWindow)
	}

	calls = fakeQs(t, func(_ context.Context, attempt int) ([]byte, []byte, error) {
		if attempt < 3 {
			return nil, []byte("Not ready to accept queries yet.\n"), nil
		}
		return []byte("42\n"), nil, nil
	})
	value, err := qsIpc(ipcPropArgv("shell", "keypresses", "revision"))
	if err != nil || value != "42" {
		t.Fatalf("value, err = %q, %v; want 42 after the shell became ready", value, err)
	}
	if n := len(calls()); n != 3 {
		t.Fatalf("qs ran %d times, want 3", n)
	}
}

func TestQsIpcAttemptTimesOut(t *testing.T) {
	calls := fakeQs(t, func(ctx context.Context, _ int) ([]byte, []byte, error) {
		<-ctx.Done()
		return nil, nil, errors.New("signal: killed")
	})
	start := time.Now()
	_, err := qsIpc(ipcArgv("shell", "shell", "closeAllMenus", ""))
	var ierr *ipcError
	if !errors.As(err, &ierr) || ierr.kind != ipcTimedOut {
		t.Fatalf("err = %v, want a timeout", err)
	}
	if n := len(calls()); n != 1 {
		t.Fatalf("qs ran %d times, want 1", n)
	}
	if elapsed := time.Since(start); elapsed > 2*ipcAttemptTimeout {
		t.Fatalf("timed out after %v, want about %v", elapsed, ipcAttemptTimeout)
	}
}

func TestQsIpcOtherFailureIsFinal(t *testing.T) {
	calls := fakeQs(t, func(context.Context, int) ([]byte, []byte, error) {
		return nil, []byte("Could not parse argument\n"), errors.New("exit status 1")
	})
	_, err := qsIpc(ipcArgv("shell", "visualizer", "setFps", "x"))
	if err == nil || !strings.Contains(err.Error(), "Could not parse argument") {
		t.Fatalf("err = %v, want the qs message", err)
	}
	if n := len(calls()); n != 1 {
		t.Fatalf("qs ran %d times, want 1", n)
	}
}

func roundTripHandle(t *testing.T, d *daemon, line string) string {
	t.Helper()
	client, server := net.Pipe()
	go d.handle(server)
	defer client.Close()
	if _, err := io.WriteString(client, line+"\n"); err != nil {
		t.Fatal(err)
	}
	reply, err := io.ReadAll(client)
	if err != nil {
		t.Fatal(err)
	}
	return string(reply)
}

func TestIPCRequestOverSocket(t *testing.T) {
	calls := fakeQs(t, func(context.Context, int) ([]byte, []byte, error) {
		return []byte("ok\n"), nil, nil
	})
	d := &daemon{}
	line := encodeIPCRequest([]string{"call", "qsbar", "move", "clock", "right side", ""})
	if got := roundTripHandle(t, d, line); got != "ok\nok\n" {
		t.Fatalf("reply to %s = %q, want %q", line, got, "ok\nok\n")
	}
	want := []string{"-c", "shell", "ipc", "call", "qsbar", "move", "clock", "right side", ""}
	if got := calls()[0]; !reflect.DeepEqual(got, want) {
		t.Fatalf("qs argv = %q, want %q", got, want)
	}

	if got := roundTripHandle(t, d, encodeIPCRequest([]string{"get", "camera", "active"})); got != "ok\nok\n" {
		t.Fatalf("get reply = %q", got)
	}
	wantGet := []string{"-c", "shell", "ipc", "prop", "get", "camera", "active"}
	if got := calls()[1]; !reflect.DeepEqual(got, wantGet) {
		t.Fatalf("qs argv = %q, want %q", got, wantGet)
	}

	fakeQs(t, func(context.Context, int) ([]byte, []byte, error) {
		return nil, []byte("Target not found.\n"), nil
	})
	if got := roundTripHandle(t, d, encodeIPCRequest([]string{"call", "theme", "reload"})); got != "err ipc theme reload: Target not found.\n" {
		t.Fatalf("missing target reply = %q", got)
	}
	for _, bad := range []string{"ipc not-json", `ipc ["call","shell"]`, `ipc ["get","a","b","c"]`, `ipc ["set","a","b"]`} {
		if got := roundTripHandle(t, d, bad); !strings.HasPrefix(got, "err ipc:") {
			t.Fatalf("reply to %s = %q, want a usage err", bad, got)
		}
	}
}
