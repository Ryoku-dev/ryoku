package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// fakeMux stands in for ryoku-gpu-mux: status answers the given JSON, set
// succeeds
func fakeMux(t *testing.T, status string) {
	t.Helper()
	fakeTools(t, map[string]string{
		"ryoku-gpu-mux": `[ "$1" = status ] && { echo '` + status + `'; exit 0; }; [ "$1" = set ] && exit 0; exit 1`,
	})
}

func muxSet(t *testing.T, mode string) muxState {
	t.Helper()
	var st muxState
	out := captureStdout(t, func() {
		if err := runGpuMux([]string{"set", mode}); err != nil {
			t.Errorf("gpu mux set %s: %v", mode, err)
		}
	})
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &st); err != nil {
		t.Fatalf("set printed %q: %v", out, err)
	}
	return st
}

// switching back to the wiring the machine booted with undoes the change, so
// no restart is pending; switching away from it needs one
func TestMuxSetPendingFollowsTheBootedWiring(t *testing.T) {
	fakeMux(t, `{"capable":true,"mode":"discrete","reboot_pending":true,"booted_mode":"hybrid"}`)
	if st := muxSet(t, "hybrid"); st.Mode != "hybrid" || st.RebootPend {
		t.Errorf("back to the booted hybrid wiring = %+v, want no restart pending", st)
	}
	fakeMux(t, `{"capable":true,"mode":"hybrid","reboot_pending":false,"booted_mode":"hybrid"}`)
	if st := muxSet(t, "discrete"); st.Mode != "discrete" || !st.RebootPend {
		t.Errorf("away from the booted wiring = %+v, want a restart pending", st)
	}
}

// with the booted wiring unknown, a write leaves the firmware's restart pending
func TestMuxSetPendingWhenTheBootedWiringIsUnknown(t *testing.T) {
	fakeMux(t, `{"capable":true,"mode":"discrete","reboot_pending":true,"booted_mode":"unknown"}`)
	if st := muxSet(t, "hybrid"); !st.RebootPend || st.BootedMode != "unknown" {
		t.Errorf("unknown booted wiring = %+v, want a restart pending and unknown kept", st)
	}
}

func TestMuxCancelHelper(t *testing.T) {
	if os.Getenv("RYOKU_MUX_CANCEL_HELPER") != "1" {
		return
	}
	os.Setenv("PATH", os.Getenv("RYOKU_MUX_TEST_PATH"))
	if runGpuMux([]string{"set", "hybrid"}) == nil {
		os.Exit(3)
	}
	os.Exit(0)
}

// Settings' Cancel stops `gpu mux set` with SIGTERM while ryoku-gpu-mux has
// exec'd pkexec. The stop must reach pkexec, never leave its prompt open to make
// the change after the user gave up.
func TestMuxSetCancelStopsThePasswordPrompt(t *testing.T) {
	dir := t.TempDir()
	started, stopped := filepath.Join(dir, "started"), filepath.Join(dir, "stopped")
	utilities := os.Getenv("PATH")
	fakeTools(t, map[string]string{
		"ryoku-gpu-mux": `[ "$1" = status ] && { echo '{"capable":true,"mode":"discrete","booted_mode":"discrete"}'; exit 0; }; exec pkexec /usr/bin/ryoku-gpu-mux "$@"`,
		"pkexec":        `trap 'echo stopped > "$STOPPED"; exit 126' TERM; echo $$ > "$STARTED"; while :; do sleep 0.05; done`,
	})
	cmd := exec.Command(os.Args[0], "-test.run=^TestMuxCancelHelper$")
	cmd.Env = append(os.Environ(), "RYOKU_MUX_CANCEL_HELPER=1", "RYOKU_MUX_TEST_PATH="+os.Getenv("PATH")+":"+utilities,
		"STARTED="+started, "STOPPED="+stopped)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if pid, err := strconv.Atoi(strings.TrimSpace(readFile(started))); err == nil {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	for !fileExists(started) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !fileExists(started) {
		cmd.Process.Kill()
		t.Fatal("the stand-in pkexec never started")
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	deadline = time.Now().Add(3 * time.Second)
	for !fileExists(stopped) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !fileExists(stopped) {
		t.Fatal("Cancel left pkexec running with its prompt open")
	}
}
