package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// #244: the dictation surface stayed open after Voxtype returned to idle,
// because the only close path was a second tap. The watcher must close on the
// idle transition, but not on the transient idle that immediately follows
// `record start` before Voxtype has actually begun.

func TestVoiceStopOnIdleWaitsForActive(t *testing.T) {
	cases := []struct {
		name      string
		sawActive bool
		class     string
		wantClose bool
		wantAct   bool
	}{
		{"initial idle ignored", false, "idle", false, false},
		{"recording arms", false, "recording", false, true},
		{"transcribing keeps armed", true, "transcribing", false, true},
		{"idle after active closes", true, "idle", true, true},
		{"stopped after active closes", true, "stopped", true, true},
		{"case insensitive", true, "IDLE", true, true},
	}
	for _, tc := range cases {
		closeNow, next := voiceStopOnIdle(tc.sawActive, tc.class)
		if closeNow != tc.wantClose || next != tc.wantAct {
			t.Errorf("%s: voiceStopOnIdle(%v,%q)=(%v,%v), want (%v,%v)",
				tc.name, tc.sawActive, tc.class, closeNow, next, tc.wantClose, tc.wantAct)
		}
	}
}

func TestParseVoxtypeClass(t *testing.T) {
	if c, ok := parseVoxtypeClass(`{"text":"🎙️","class":"recording","alt":"recording"}`); !ok || c != "recording" {
		t.Errorf("class not parsed: %q %v", c, ok)
	}
	// Some builds carry the state only in `alt`.
	if c, ok := parseVoxtypeClass(`{"class":"","alt":"idle"}`); !ok || c != "idle" {
		t.Errorf("alt fallback not parsed: %q %v", c, ok)
	}
	for _, junk := range []string{"", "not json", "[1,2]", `{"text":"x"}`} {
		if _, ok := parseVoxtypeClass(junk); ok {
			t.Errorf("junk accepted: %q", junk)
		}
	}
}

const fakeVoxtype = `#!/bin/sh
set -eu
printf '%s\n' "$$" >> "$VOXTYPE_PID_FILE"
printf '{"class":"recording"}\n'
if [ "${VOXTYPE_MODE:-forever}" = idle ]; then
	printf '{"class":"idle"}\n'
fi
exec sleep 300
`

func installFakeVoxtype(t *testing.T, mode string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "voxtype")
	if err := os.WriteFile(path, []byte(fakeVoxtype), 0755); err != nil {
		t.Fatal(err)
	}
	pidFile := filepath.Join(dir, "pids")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("VOXTYPE_PID_FILE", pidFile)
	t.Setenv("VOXTYPE_MODE", mode)
	return pidFile
}

func startVoiceWatcher(d *daemon, stop <-chan struct{}) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		d.watchVoice(stop)
		close(done)
	}()
	return done
}

func waitVoxtypePID(t *testing.T, pidFile string, count int) int {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(pidFile)
		if err == nil {
			fields := strings.Fields(string(data))
			if len(fields) >= count {
				pid, err := strconv.Atoi(fields[count-1])
				if err != nil {
					t.Fatalf("bad fake voxtype pid %q: %v", fields[count-1], err)
				}
				return pid
			}
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("fake voxtype did not start process %d", count)
	return 0
}

func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

func waitWatcherDone(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	case <-time.After(2 * time.Second):
		return false
	}
}

func killTestProcess(pid int, done <-chan struct{}) {
	_ = syscall.Kill(pid, syscall.SIGKILL)
	select {
	case <-done:
	case <-time.After(time.Second):
	}
}

func TestWatchVoiceReapsFollowProcess(t *testing.T) {
	t.Run("manual stop", func(t *testing.T) {
		pidFile := installFakeVoxtype(t, "forever")
		d := &daemon{quit: make(chan struct{})}
		stop := make(chan struct{})
		done := startVoiceWatcher(d, stop)
		pid := waitVoxtypePID(t, pidFile, 1)

		close(stop)
		if !waitWatcherDone(done) {
			killTestProcess(pid, done)
			t.Fatal("watchVoice did not kill and reap voxtype after stop")
		}
		if processAlive(pid) {
			killTestProcess(pid, done)
			t.Fatalf("voxtype process %d remains after stop", pid)
		}
	})

	t.Run("idle close", func(t *testing.T) {
		pidFile := installFakeVoxtype(t, "idle")
		d := &daemon{quit: make(chan struct{})}
		stop := make(chan struct{})
		done := startVoiceWatcher(d, stop)
		pid := waitVoxtypePID(t, pidFile, 1)

		if !waitWatcherDone(done) {
			killTestProcess(pid, done)
			t.Fatal("watchVoice did not kill and reap voxtype after idle")
		}
		if processAlive(pid) {
			killTestProcess(pid, done)
			t.Fatalf("voxtype process %d remains after idle", pid)
		}
	})
}

func TestWatchVoiceConsecutiveSessionsDoNotOverlap(t *testing.T) {
	pidFile := installFakeVoxtype(t, "forever")
	d := &daemon{quit: make(chan struct{})}

	stopFirst := make(chan struct{})
	firstDone := startVoiceWatcher(d, stopFirst)
	firstPID := waitVoxtypePID(t, pidFile, 1)
	close(stopFirst)

	stopSecond := make(chan struct{})
	secondDone := startVoiceWatcher(d, stopSecond)
	secondPID := waitVoxtypePID(t, pidFile, 2)
	if processAlive(firstPID) {
		killTestProcess(firstPID, firstDone)
		close(stopSecond)
		killTestProcess(secondPID, secondDone)
		t.Fatalf("first voxtype process %d still alive when session two started", firstPID)
	}

	close(stopSecond)
	if !waitWatcherDone(firstDone) || !waitWatcherDone(secondDone) {
		killTestProcess(firstPID, firstDone)
		killTestProcess(secondPID, secondDone)
		t.Fatal("consecutive voice watchers did not exit")
	}
	if processAlive(secondPID) {
		killTestProcess(secondPID, secondDone)
		t.Fatalf("second voxtype process %d remains after stop", secondPID)
	}
}
