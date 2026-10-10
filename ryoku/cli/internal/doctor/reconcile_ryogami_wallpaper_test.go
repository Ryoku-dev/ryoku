package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"ryoku-cli/internal/host"
)

func TestRyogamiWallpaperActions(t *testing.T) {
	cases := []struct {
		name                                        string
		state                                       ryogamiWallpaperState
		wantEnable, wantFailed, wantStart, wantAwww bool
	}{
		{"fresh cutover: not enabled, awww still up",
			ryogamiWallpaperState{enabled: false, active: false, awwwRunning: true, inSession: true},
			true, false, true, true},
		{"enabled and running does nothing",
			ryogamiWallpaperState{enabled: true, active: true, inSession: true},
			false, false, false, false},
		{"enabled but wedged clears failed and starts",
			ryogamiWallpaperState{enabled: true, active: false, failed: true, inSession: true},
			false, true, true, false},
		{"enabled with stray awww stops it",
			ryogamiWallpaperState{enabled: true, active: true, awwwRunning: true, inSession: true},
			false, false, false, true},
		// The case an enabled-only check cannot see: systemd skipped the unit on
		// its ConditionEnvironment, so it is enabled, inactive and not failed,
		// and the desktop has no wallpaper while nothing looks wrong.
		{"enabled but skipped by its condition is started",
			ryogamiWallpaperState{enabled: true, active: false, failed: false, inSession: true},
			false, false, true, false},
		// Outside the session the condition would refuse a start and inactive is
		// the correct state, so it must not be reported as a fault.
		{"inactive outside the graphical session is fine",
			ryogamiWallpaperState{enabled: true, active: false, failed: false, inSession: false},
			false, false, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotEnable, gotFailed, gotStart, gotAwww := ryogamiWallpaperActions(c.state)
			if gotEnable != c.wantEnable || gotFailed != c.wantFailed || gotStart != c.wantStart || gotAwww != c.wantAwww {
				t.Fatalf("ryogamiWallpaperActions(%+v) = (enable=%v, failed=%v, start=%v, stopAwww=%v), want (%v, %v, %v, %v)",
					c.state, gotEnable, gotFailed, gotStart, gotAwww,
					c.wantEnable, c.wantFailed, c.wantStart, c.wantAwww)
			}
		})
	}
}

func TestRyogamiWallpaperRunitRepairSettles(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "ryogami"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("WAYLAND_DISPLAY", "wayland-1")

	oldInit, oldService, oldAwww := ryogamiInit, ryogamiService, awwwDaemonRunning
	t.Cleanup(func() {
		ryogamiInit, ryogamiService, awwwDaemonRunning = oldInit, oldService, oldAwww
	})
	ryogamiInit = func() (host.InitSystem, error) { return host.Runit, nil }
	awwwDaemonRunning = func() bool { return false }

	enabled, active := false, false
	var calls [][]string
	ryogamiService = func(args []string) int {
		calls = append(calls, append([]string(nil), args...))
		switch args[1] {
		case "is-enabled":
			if enabled {
				return host.ExitOK
			}
			return host.ExitFalse
		case "is-active":
			if active {
				return host.ExitOK
			}
			return host.ExitFalse
		case "enable":
			enabled = true
		case "start":
			active = true
		}
		return host.ExitOK
	}

	if r := reconcileRyogamiWallpaper(false); r.status != recFixed {
		t.Fatalf("repair = %s %q, want fixed", r.status.label(), r.detail)
	}
	if !enabled || !active {
		t.Fatalf("runit service state = enabled %v active %v, want both true; calls=%v", enabled, active, calls)
	}
	if r := reconcileRyogamiWallpaper(false); r.status != recOK {
		t.Fatalf("second run = %s %q, want settled ok", r.status.label(), r.detail)
	}
}
