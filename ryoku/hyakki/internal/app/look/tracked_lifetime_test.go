package look

import (
	"image"
	"runtime"
	"testing"
	"time"
	"weak"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
)

func retiredTrackedOwner() (*op.Ops, weak.Pointer[text.Shaper]) {
	th := NewWithRoles(palette.Signature(true))
	ops := new(op.Ops)
	gtx := layout.Context{
		Ops: ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(400, 60)),
	}
	kit.LayoutSpacedCaps(gtx, th.Theme, th.T.Mono, th.R.Fg, 1, "retired owner")
	return ops, weak.Make(th.Shaper)
}

func TestTrackedTextDoesNotRetainRetiredShaper(t *testing.T) {
	ops, owner := retiredTrackedOwner()
	defer runtime.KeepAlive(ops)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		runtime.GC()
		if owner.Value() == nil {
			return
		}
		runtime.Gosched()
	}
	t.Fatal("cached tracked text retains its retired font shaper after collection")
}

func TestTrackedTextCacheHitDoesNotAllocate(t *testing.T) {
	th := NewWithRoles(palette.Signature(true))
	var ops op.Ops
	gtx := layout.Context{
		Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(400, 60)),
	}
	frame := func() {
		ops.Reset()
		kit.LayoutSpacedCaps(gtx, th.Theme, th.T.Mono, th.R.Fg, 1, "steady owner")
	}
	frame()
	if allocations := testing.AllocsPerRun(20, frame); allocations != 0 {
		t.Fatalf("cached tracked text allocates %.0f times per frame", allocations)
	}
}
