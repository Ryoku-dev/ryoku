package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	wm "ryoku-wm"
)

// fakePowerProvider drops a `ryoku-wm-testwm` on PATH that reports the
// outputPower capability and appends every act invocation to a log file, so
// the wake guard's calls are observable without a compositor.
func fakePowerProvider(t *testing.T, logPath string) {
	t.Helper()
	bin := t.TempDir()
	provider := filepath.Join(bin, "ryoku-wm-testwm")
	script := "#!/usr/bin/env bash\n" +
		"if [[ \"$1\" == caps ]]; then printf '%s' '{\"name\":\"testwm\",\"supports\":[\"outputPower\"]}'; exit 0; fi\n" +
		"printf '%s\\n' \"$*\" >>\"" + logPath + "\"\n" +
		"exit 0\n"
	if err := os.WriteFile(provider, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestHoldAwakeReAssertsOutputPower(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "acts.log")
	fakePowerProvider(t, logPath)

	oldHold, oldStep := wakeHold, wakeStep
	wakeHold, wakeStep = 250*time.Millisecond, 50*time.Millisecond
	t.Cleanup(func() { wakeHold, wakeStep = oldHold, oldStep })

	d := &daemon{wmc: wm.OpenNamed("testwm"), quit: make(chan struct{})}
	d.holdAwake()

	b, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("no act log (guard never fired): %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) < 2 {
		t.Fatalf("guard asserted %d time(s), want a re-assert loop over the window", len(lines))
	}
	for _, l := range lines {
		if l != "act output.power on" {
			t.Fatalf("act line = %q, want %q", l, "act output.power on")
		}
	}
}

func TestHoldAwakeStopsWithoutCapability(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "acts.log")
	bin := t.TempDir()
	provider := filepath.Join(bin, "ryoku-wm-testwm")
	script := "#!/usr/bin/env bash\n" +
		"if [[ \"$1\" == caps ]]; then printf '%s' '{\"name\":\"testwm\",\"supports\":[]}'; exit 0; fi\n" +
		"printf '%s\\n' \"$*\" >>\"" + logPath + "\"\n" +
		"exit 0\n"
	if err := os.WriteFile(provider, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	oldHold, oldStep := wakeHold, wakeStep
	wakeHold, wakeStep = 150*time.Millisecond, 50*time.Millisecond
	t.Cleanup(func() { wakeHold, wakeStep = oldHold, oldStep })

	d := &daemon{wmc: wm.OpenNamed("testwm"), quit: make(chan struct{})}
	d.holdAwake()

	if _, err := os.Stat(logPath); err == nil {
		t.Fatal("guard fired act calls on a provider without outputPower")
	}
}

func TestHoldAwakeHonoursQuit(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "acts.log")
	fakePowerProvider(t, logPath)

	oldHold, oldStep := wakeHold, wakeStep
	wakeHold, wakeStep = time.Hour, 20*time.Millisecond
	t.Cleanup(func() { wakeHold, wakeStep = oldHold, oldStep })

	d := &daemon{wmc: wm.OpenNamed("testwm"), quit: make(chan struct{})}
	go func() { time.Sleep(120 * time.Millisecond); close(d.quit) }()
	done := make(chan struct{})
	go func() { d.holdAwake(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("holdAwake ignored the quit signal")
	}
}
