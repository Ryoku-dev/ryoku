package doctor

import (
	"errors"
	"strings"
	"testing"
)

func TestPlanWmPlugins(t *testing.T) {
	noRepair := func() (map[string]string, error) { t.Fatal("repair must not run"); return nil, nil }
	stale := wmPluginState{capable: true, listed: true, enabled: 2, stale: []string{"dynamic-cursors"}, toolchain: true}

	t.Run("no plugin capability is not applicable", func(t *testing.T) {
		r := planWmPlugins(wmPluginState{}, false, noRepair)
		if r.status != recOK {
			t.Fatalf("status = %v, want ok", r.status)
		}
	})

	t.Run("unanswered provider is a note", func(t *testing.T) {
		r := planWmPlugins(wmPluginState{capable: true}, false, noRepair)
		if r.status != recNote {
			t.Fatalf("status = %v, want note", r.status)
		}
	})

	t.Run("nothing enabled passes", func(t *testing.T) {
		r := planWmPlugins(wmPluginState{capable: true, listed: true, toolchain: true}, true, noRepair)
		if r.status != recOK {
			t.Fatalf("status = %v, want ok", r.status)
		}
	})

	t.Run("enabled and current passes", func(t *testing.T) {
		r := planWmPlugins(wmPluginState{capable: true, listed: true, enabled: 3, toolchain: true}, true, noRepair)
		if r.status != recOK {
			t.Fatalf("status = %v, want ok", r.status)
		}
	})

	t.Run("stale reports in check mode", func(t *testing.T) {
		r := planWmPlugins(stale, true, noRepair)
		if r.status != recWouldFix || !strings.Contains(r.detail, "dynamic-cursors") {
			t.Fatalf("result = %v %q", r.status, r.detail)
		}
	})

	t.Run("stale rebuilds in apply mode", func(t *testing.T) {
		ran := false
		r := planWmPlugins(stale, false, func() (map[string]string, error) { ran = true; return nil, nil })
		if !ran || r.status != recFixed {
			t.Fatalf("ran=%v status=%v", ran, r.status)
		}
	})

	t.Run("a plugin the builder could not rebuild is named", func(t *testing.T) {
		r := planWmPlugins(stale, false, func() (map[string]string, error) {
			return map[string]string{"dynamic-cursors": "build step failed: make all"}, nil
		})
		if r.status != recFailed || !strings.Contains(r.detail, "make all") {
			t.Fatalf("result = %v %q", r.status, r.detail)
		}
	})

	t.Run("a builder that cannot run fails", func(t *testing.T) {
		r := planWmPlugins(stale, false, func() (map[string]string, error) { return nil, errors.New("no git") })
		if r.status != recFailed {
			t.Fatalf("status = %v, want failed", r.status)
		}
	})

	t.Run("no toolchain warns with what to install", func(t *testing.T) {
		oldHasPacman := hasPacman
		hasPacman = func() bool { return true }
		t.Cleanup(func() { hasPacman = oldHasPacman })
		s := stale
		s.toolchain, s.missing = false, []string{"cmake", "compositor headers"}
		r := planWmPlugins(s, false, noRepair)
		if r.status != recWarn || !strings.Contains(r.detail, "cmake") || !strings.Contains(r.remedy, "base-devel") {
			t.Fatalf("result = %v %q fix=%q", r.status, r.detail, r.remedy)
		}
	})
}
