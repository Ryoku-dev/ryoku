package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// the clock knob shows the lock that was set, never the clock running now
func TestClockLockKnobShowsTheLockNotTheClock(t *testing.T) {
	probe := func(lock int) []Tunable {
		return buildTunables(tuneInputs{gpus: []gpuProbe{{
			slot: "0000:01:00.0", driver: "nvidia",
			nv: &nvInfo{clockMaxGr: 3105, clockCurGr: 525, clockLock: lock},
		}}})
	}
	if cl := has(probe(0), "0000:01:00.0", "clock_lock"); cl == nil || !cl.Auto || cl.Current != 3105 {
		t.Fatalf("no lock must read as no limit at the top of the range: %+v", cl)
	}
	if cl := has(probe(1700), "0000:01:00.0", "clock_lock"); cl == nil || cl.Auto || cl.Current != 1700 {
		t.Fatalf("a recorded lock must be what the knob shows: %+v", cl)
	}
}

// a fan on automatic has no duty anyone chose; a fixed one reports its duty
func TestFanKnobAutomaticOrFixed(t *testing.T) {
	probe := func(manual bool, pct int) *Tunable {
		return has(buildTunables(tuneInputs{gpus: []gpuProbe{{
			slot: "0000:03:00.0", driver: "amdgpu", amdFan: true, amdFanManual: manual, amdFanPct: pct,
		}}}), "0000:03:00.0", "fan_pct")
	}
	if f := probe(false, 30); f == nil || !f.Auto {
		t.Fatalf("an automatic fan must say so: %+v", f)
	}
	if f := probe(true, 60); f == nil || f.Auto || f.Current != 60 {
		t.Fatalf("a fixed fan must report its duty: %+v", f)
	}
}

func TestNextClockLocks(t *testing.T) {
	locks := nextClockLocks(map[string]int{}, "a", 1500)
	locks = nextClockLocks(locks, "b", 900)
	if locks["a"] != 1500 || locks["b"] != 900 {
		t.Fatalf("locks not kept per card: %v", locks)
	}
	if locks = nextClockLocks(locks, "a", 0); len(locks) != 1 || locks["b"] != 900 {
		t.Fatalf("0 must clear only that card: %v", locks)
	}
	if locks = nextClockLocks(locks, "", 0); len(locks) != 0 {
		t.Fatalf("an empty slot must clear every card: %v", locks)
	}
}

// a record from an earlier boot is no lock: the reboot already dropped it
func TestClockLocksForgetEarlierBoots(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ryoku", "gpu-clock-locks.json")
	if err := writeClockLocks(path, "boot-1", map[string]int{"a": 1200}); err != nil {
		t.Fatal(err)
	}
	if got := readClockLocks(path, "boot-1")["a"]; got != 1200 {
		t.Fatalf("same boot must read the lock back, got %d", got)
	}
	if got := readClockLocks(path, "boot-2"); len(got) != 0 {
		t.Fatalf("another boot must read no locks, got %v", got)
	}
	if err := writeClockLocks(path, "boot-1", map[string]int{}); err != nil {
		t.Fatal(err)
	}
	if got := readClockLocks(path, "boot-1"); len(got) != 0 {
		t.Fatalf("clearing every lock must remove the record, got %v", got)
	}
}

// fakeNvidiaTuneHost: one NVIDIA card whose clock the tuner can lock, and a
// pkexec that logs each escalated command and exits pkexecExit
func fakeNvidiaTuneHost(t *testing.T, pkexecExit int) (slot, calls string) {
	t.Helper()
	if currentBootID() == "" {
		t.Skip("no boot id: the clock-lock record needs one")
	}
	slot = "0000:01:00.0"
	calls = filepath.Join(t.TempDir(), "calls")
	t.Setenv("CALLS", calls)
	t.Setenv("RYOKU_GPU_BIN", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	fakeTools(t, map[string]string{
		"ryoku-gpu":  `echo '[{"slot":"0000:01:00.0","card":"card1","driver":"nvidia","class":"discrete"}]'`,
		"nvidia-smi": `echo 'Supported, 100, 50, 150, 120, Disabled, 525, 3105'`,
		"pkexec":     fmt.Sprintf(`echo "$*" >> "$CALLS"; exit %d`, pkexecExit),
	})
	return slot, calls
}

func readOnlyRecordDir(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := filepath.Dir(gpuClockLockPath())
	mustMkdir(t, dir)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
}

// the knob shows the lock from the record, so a record that cannot be written
// must leave the GPU as it was, not locked with nothing saying so
func TestClockLockRecordFailureLeavesGPUAsItWas(t *testing.T) {
	slot, calls := fakeNvidiaTuneHost(t, 0)
	readOnlyRecordDir(t)
	if err := tuneSet(slot, "clock_lock", "1500"); err == nil {
		t.Fatal("a lock whose record cannot be written must fail")
	}
	escalated := strings.Split(strings.TrimSpace(readFile(calls)), "\n")
	if last := escalated[len(escalated)-1]; strings.HasSuffix(last, "clock_lock 1500") {
		t.Fatalf("the GPU was left locked to 1500 MHz with no record of it; escalations: %q", escalated)
	}
}

// a reset whose record cannot be cleared must not lift the lock the record shows
func TestClockLockResetRecordFailureLeavesGPUAsItWas(t *testing.T) {
	slot, calls := fakeNvidiaTuneHost(t, 0)
	if err := writeClockLocks(gpuClockLockPath(), currentBootID(), map[string]int{slot: 1200}); err != nil {
		t.Fatal(err)
	}
	readOnlyRecordDir(t)
	if err := tuneReset(slot); err == nil {
		t.Fatal("a reset whose record cannot be cleared must fail")
	}
	if strings.Contains(readFile(calls), "gpu tune reset") && clockLockFor(slot) == 1200 {
		t.Fatal("the GPU's lock was lifted but the record still shows 1200 MHz")
	}
}

// a lock the GPU never took (cancelled or failed escalation) never reaches the record
func TestClockLockFailedEscalationKeepsRecord(t *testing.T) {
	slot, _ := fakeNvidiaTuneHost(t, 126)
	if err := writeClockLocks(gpuClockLockPath(), currentBootID(), map[string]int{slot: 1200}); err != nil {
		t.Fatal(err)
	}
	if err := tuneSet(slot, "clock_lock", "1500"); err == nil {
		t.Fatal("a failed escalation must fail the set")
	}
	if got := clockLockFor(slot); got != 1200 {
		t.Fatalf("record = %d MHz after a failed lock, want the 1200 the GPU still has", got)
	}
	if err := tuneReset(slot); err == nil {
		t.Fatal("a failed escalation must fail the reset")
	}
	if got := clockLockFor(slot); got != 1200 {
		t.Fatalf("record = %d MHz after a failed reset, want the 1200 the GPU still has", got)
	}
}

// a reset that ran but failed on another knob still lifted the lock, so the
// record stays cleared while the error says what failed
func TestClockLockResetThatRanClearsRecord(t *testing.T) {
	slot, _ := fakeNvidiaTuneHost(t, 1)
	if err := writeClockLocks(gpuClockLockPath(), currentBootID(), map[string]int{slot: 1200}); err != nil {
		t.Fatal(err)
	}
	if err := tuneReset(slot); err == nil {
		t.Fatal("a failed reset must say so")
	}
	if got := clockLockFor(slot); got != 0 {
		t.Fatalf("record = %d MHz after a reset that lifted the lock, want none", got)
	}
}
