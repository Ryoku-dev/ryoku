package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// fakeBacklight lays a minimal /sys/class/backlight/<dev> tree: max, the live
// brightness, and actual mirroring it, returning the device dir.
func fakeBacklight(t *testing.T, max, live int) string {
	t.Helper()
	dev := filepath.Join(t.TempDir(), "amdgpu_bl0")
	if err := os.MkdirAll(dev, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, a := range []struct{ name, val string }{
		{"max_brightness", strconv.Itoa(max)},
		{"brightness", strconv.Itoa(live)},
		{"actual_brightness", strconv.Itoa(live)},
	} {
		if err := os.WriteFile(filepath.Join(dev, a.name), []byte(a.val+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dev
}

// The OSD must show the requested (linear) brightness, not the driver's
// actual_brightness: amdgpu's custom curve made 100% read as 88% (#176).
func TestReadBrightnessFractionIsLinear(t *testing.T) {
	dev := fakeBacklight(t, 65535, 65535)
	// A driver reporting a non-linear actual_brightness must not move the
	// published fraction: the writable attribute is the contract.
	if err := os.WriteFile(filepath.Join(dev, "actual_brightness"), []byte("41936\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	frac, raw, ok := readBrightnessFraction(dev, 65535)
	if !ok || raw != 65535 || frac != 1.0 {
		t.Fatalf("readBrightnessFraction = (%v,%d,%v), want (1,65535,true)", frac, raw, ok)
	}
	if _, _, ok := readBrightnessFraction(dev, 0); ok {
		t.Fatal("a zero max must not report a fraction")
	}
}

func TestBacklightLevelRoundTrip(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	saveBacklight(4200)
	b, err := os.ReadFile(backlightLevelPath())
	if err != nil {
		t.Fatalf("saved level unreadable: %v", err)
	}
	if got := strings.TrimSpace(string(b)); got != "4200" {
		t.Fatalf("saved level = %q, want 4200", got)
	}

	// Boot state: kernel reset the panel to max. Restore must push the level
	// back before the watcher's first read.
	dev := fakeBacklight(t, 65535, 65535)
	restoreBacklight(dev, 65535)
	if got, _ := readSysInt(filepath.Join(dev, "brightness")); got != 4200 {
		t.Fatalf("after restore brightness = %d, want 4200", got)
	}
}

func TestBacklightRestoreIgnoresUnusableState(t *testing.T) {
	cases := []struct {
		name  string
		file  string
		level int
	}{
		{"no file at all", "", 0},
		{"garbage", "not-a-number", 0},
		{"zero floor", "0", 0},
		{"above the panel max", "70000", 65535},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", t.TempDir())
			t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
			if c.file != "" {
				if err := os.MkdirAll(filepath.Dir(backlightLevelPath()), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(backlightLevelPath(), []byte(c.file), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			max := 65535
			if c.level > 0 {
				max = c.level
			}
			dev := fakeBacklight(t, max, max)
			restoreBacklight(dev, max)
			if got, _ := readSysInt(filepath.Join(dev, "brightness")); got != max {
				t.Fatalf("restore touched the device (%d) for %q state, want %d", got, c.file, max)
			}
		})
	}
}

func TestBacklightRestoreSkipsWhenAlreadyApplied(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	saveBacklight(3000)
	// The kernel/boot already landed on the saved level: no write, so a device
	// without a writable brightness node is not fought over.
	dev := fakeBacklight(t, 65535, 3000)
	if err := os.Chmod(filepath.Join(dev, "brightness"), 0o444); err != nil {
		t.Fatal(err)
	}
	restoreBacklight(dev, 65535)
	if got, _ := readSysInt(filepath.Join(dev, "brightness")); got != 3000 {
		t.Fatalf("brightness = %d, want untouched 3000", got)
	}
}

func TestIdleDimDoesNotReplaceOrRestoreSavedBrightness(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	saveBacklight(8000)

	idleDir := filepath.Join(runtimeDir, "ryoku-idle-brightness")
	if err := os.MkdirAll(idleDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(idleDir, "state"), []byte("intel_backlight\t8000\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	dev := fakeBacklight(t, 10000, 2000)
	restoreBacklight(dev, 10000)
	if got, _ := readSysInt(filepath.Join(dev, "brightness")); got != 2000 {
		t.Fatalf("shell restore raised active idle dim to %d, want 2000", got)
	}

	saveObservedBacklight(2000)
	b, err := os.ReadFile(backlightLevelPath())
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(b)); got != "8000" {
		t.Fatalf("idle dim replaced saved user level with %s, want 8000", got)
	}
}

// A dead backlight node must read as an error so the watcher reopens it; a
// readable but unparsable value must not, or a glitch would churn the fd.
// Returning ok-and-continue on a dead node busy-looped poll(2) at 100% CPU
// after nvidia re-registered its backlight on resume.
func TestReadActualClassifiesDeadNode(t *testing.T) {
	dev := fakeBacklight(t, 100, 42)
	buf := make([]byte, 32)
	open := func(name string) int {
		t.Helper()
		fd, err := unix.Open(filepath.Join(dev, name), unix.O_RDONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		return fd
	}

	fd := open("actual_brightness")
	if v, err := readActual(fd, buf); err != nil || v != 42 {
		t.Fatalf("readActual = (%d,%v), want (42,nil)", v, err)
	}
	unix.Close(fd)
	if _, err := readActual(fd, buf); err == nil || err == errBacklightValue {
		t.Fatalf("closed fd read = %v, want a dead-node error", err)
	}

	if err := os.WriteFile(filepath.Join(dev, "actual_brightness"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	fd = open("actual_brightness")
	defer unix.Close(fd)
	if _, err := readActual(fd, buf); err == nil || err == errBacklightValue {
		t.Fatalf("empty read = %v, want a dead-node error", err)
	}

	if err := os.WriteFile(filepath.Join(dev, "actual_brightness"), []byte("junk\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readActual(fd, buf); err != errBacklightValue {
		t.Fatalf("garbage read = %v, want errBacklightValue", err)
	}
}

func TestWatchBacklightNodeReturnsOnMissingNode(t *testing.T) {
	o := &osdState{}
	if o.watchBacklightNode(filepath.Join(t.TempDir(), "gone"), 100) {
		t.Fatal("a missing node must report it was never read")
	}
	if o.watchBacklightNode("", 100) {
		t.Fatal("no device must report it was never read")
	}
}
