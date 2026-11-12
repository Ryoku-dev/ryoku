package main

// gpulock.go: the core-clock locks this Hub set, kept for the rest of the boot.
// nvidia-smi reports the clock running now, not a lock, so this record is how
// the page shows what was set. A reboot drops the locks, and the boot id in the
// record drops it with them.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

type gpuClockLocks struct {
	Boot  string         `json:"boot"`
	Locks map[string]int `json:"locks"`
}

func gpuClockLockPath() string {
	return filepath.Join(xdgHome("XDG_STATE_HOME", ".local/state"), "ryoku", "gpu-clock-locks.json")
}

func currentBootID() string {
	return readTrim("/proc/sys/kernel/random/boot_id")
}

// readClockLocks: the locks recorded in this boot; a record from an earlier
// boot, or none, is no locks
func readClockLocks(path, boot string) map[string]int {
	var f gpuClockLocks
	if boot == "" || json.Unmarshal([]byte(readFile(path)), &f) != nil || f.Boot != boot || f.Locks == nil {
		return map[string]int{}
	}
	return f.Locks
}

// nextClockLocks is the pure step: mhz above 0 locks slot, 0 clears it, and an
// empty slot clears every card
func nextClockLocks(locks map[string]int, slot string, mhz int) map[string]int {
	out := map[string]int{}
	if slot == "" {
		return out
	}
	for k, v := range locks {
		out[k] = v
	}
	if mhz > 0 {
		out[slot] = mhz
	} else {
		delete(out, slot)
	}
	return out
}

func writeClockLocks(path, boot string, locks map[string]int) error {
	if len(locks) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(gpuClockLocks{Boot: boot, Locks: locks})
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// withClockLocks records next(locks) before apply makes the escalated write,
// so a record that cannot be saved leaves the GPU untouched (undoing a lock
// after the fact would take a second password prompt). If apply fails the
// record goes back as it was, unless tookAnyway says the GPU changed regardless.
func withClockLocks(next func(map[string]int) map[string]int, apply func() error, tookAnyway func(error) bool) error {
	path, boot := gpuClockLockPath(), currentBootID()
	before, err := snapshotFiles(path)
	if err != nil {
		return fmt.Errorf("read the clock-lock record: %w", err)
	}
	if err := writeClockLocks(path, boot, next(readClockLocks(path, boot))); err != nil {
		return fmt.Errorf("record the clock lock, so the GPU was left as it was: %w", err)
	}
	err = apply()
	if err == nil || (tookAnyway != nil && tookAnyway(err)) {
		return err
	}
	if rerr := before.restore(); rerr != nil {
		return errors.Join(err, fmt.Errorf("the GPU's clock lock is unchanged, but %s could not be put back and records the change it did not take: %w", path, rerr))
	}
	return err
}

// escalationRan: the escalated command ran and failed, as opposed to pkexec's
// own 126 (dismissed or not authorised), 127 (could not run it) or a signal
func escalationRan(err error) bool {
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() > 0 && exit.ExitCode() != 126 && exit.ExitCode() != 127
}

func clockLockFor(slot string) int {
	return readClockLocks(gpuClockLockPath(), currentBootID())[slot]
}
