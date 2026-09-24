package doctor

import (
	"errors"
	"strings"
	"testing"
)

func TestPlanGpuPin(t *testing.T) {
	noDisable := func() error { t.Fatal("disable must not run"); return nil }
	noPersist := func() error { t.Fatal("persist must not run"); return nil }

	t.Run("probe failure is silent", func(t *testing.T) {
		r := planGpuPin("", errors.New("exec: not found"), true, noDisable, noPersist)
		if r.status != recOK {
			t.Fatalf("status = %v, want ok", r.status)
		}
	})

	t.Run("ok verdict passes", func(t *testing.T) {
		r := planGpuPin("ok", nil, true, noDisable, noPersist)
		if r.status != recOK {
			t.Fatalf("status = %v, want ok", r.status)
		}
	})

	t.Run("forced pin is kept", func(t *testing.T) {
		r := planGpuPin("forced", nil, false, noDisable, noPersist)
		if r.status != recOK || !strings.Contains(r.detail, "RYOKU_GPU_FORCE") {
			t.Fatalf("result = %v %q", r.status, r.detail)
		}
	})

	t.Run("stale pin reports in check mode", func(t *testing.T) {
		r := planGpuPin("stale-pin 0000:01:00.0", nil, true, noDisable, noPersist)
		if r.status != recWouldFix {
			t.Fatalf("status = %v, want would-fix", r.status)
		}
		for _, want := range []string{"0000:01:00.0", "ryoku-gpu disable", "next Hyprland login"} {
			if !strings.Contains(r.detail, want) {
				t.Fatalf("detail %q missing %q", r.detail, want)
			}
		}
	})

	t.Run("stale pin repairs in apply mode", func(t *testing.T) {
		ran := false
		r := planGpuPin("stale-pin 0000:01:00.0", nil, false, func() error { ran = true; return nil }, noPersist)
		if !ran || r.status != recFixed {
			t.Fatalf("ran=%v status=%v", ran, r.status)
		}
	})

	t.Run("failed repair says so", func(t *testing.T) {
		r := planGpuPin("stale-pin 0000:01:00.0", nil, false, func() error { return errors.New("denied") }, noPersist)
		if r.status != recFailed || !strings.Contains(r.detail, "ryoku-gpu disable") {
			t.Fatalf("result = %v %q", r.status, r.detail)
		}
	})

	t.Run("missing pin reports in check mode", func(t *testing.T) {
		r := planGpuPin("missing-pin", nil, true, noDisable, noPersist)
		if r.status != recWouldFix || !strings.Contains(r.detail, "ryoku-gpu persist") {
			t.Fatalf("result = %v %q", r.status, r.detail)
		}
	})

	t.Run("missing pin repairs in apply mode", func(t *testing.T) {
		ran := false
		r := planGpuPin("missing-pin", nil, false, noDisable, func() error { ran = true; return nil })
		if !ran || r.status != recFixed {
			t.Fatalf("ran=%v status=%v", ran, r.status)
		}
	})

	t.Run("failed persist says so", func(t *testing.T) {
		r := planGpuPin("missing-pin", nil, false, noDisable, func() error { return errors.New("denied") })
		if r.status != recFailed || !strings.Contains(r.detail, "ryoku-gpu persist") {
			t.Fatalf("result = %v %q", r.status, r.detail)
		}
	})

	t.Run("unknown verdict warns", func(t *testing.T) {
		r := planGpuPin("whatever", nil, true, noDisable, noPersist)
		if r.status != recWarn {
			t.Fatalf("status = %v, want warn", r.status)
		}
	})
}
