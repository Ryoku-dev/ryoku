package anim

import (
	"math"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// frame is a context at now whose invalidations land in router.
func frame(router *input.Router, ops *op.Ops, now time.Time) layout.Context {
	ops.Reset()
	return layout.Context{Ops: ops, Now: now, Source: router.Source(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
}

// woke reports whether the frame asked for another and when.
func woke(router *input.Router) (time.Time, bool) { return router.WakeupTime() }

func TestEaseOutCubic(t *testing.T) {
	for _, test := range []struct{ in, want float32 }{{-1, 0}, {0, 0}, {0.5, 0.875}, {0.25, 0.578125}, {1, 1}, {2, 1}} {
		if got := EaseOutCubic(test.in); math.Abs(float64(got-test.want)) > 1e-6 {
			t.Errorf("EaseOutCubic(%v) = %v, want %v", test.in, got, test.want)
		}
	}
	previous := float32(0)
	for i := 1; i <= 100; i++ {
		got := EaseOutCubic(float32(i) / 100)
		if got < previous {
			t.Fatalf("EaseOutCubic falls at %d", i)
		}
		previous = got
	}
}

// chaseRun follows a 0 -> 1 move at hz frames a second for length and returns the value at
// each frame's time, checking it never overshoots and never steps back.
func chaseRun(t *testing.T, hz float64, length time.Duration) map[time.Duration]float32 {
	t.Helper()
	var router input.Router
	var ops op.Ops
	start := time.Unix(1000, 0)
	var chase Chase
	chase.Set(1)
	out := map[time.Duration]float32{}
	previous := float32(-1)
	for i := 0; ; i++ {
		at := time.Duration(float64(i) * float64(time.Second) / hz)
		if at > length {
			break
		}
		value := chase.Value(frame(&router, &ops, start.Add(at)))
		if value > 1 {
			t.Fatalf("%v Hz: overshoot %v at %v", hz, value, at)
		}
		if value < previous {
			t.Fatalf("%v Hz: stepped back to %v at %v", hz, value, at)
		}
		previous = value
		out[at] = value
	}
	return out
}

// TestChaseFrameRateIndependent: at 60 and 165 Hz the value at the same moment is the same,
// and both match the closed form 1 - exp(-rate * t).
func TestChaseFrameRateIndependent(t *testing.T) {
	sixty := chaseRun(t, 60, 400*time.Millisecond)
	fast := chaseRun(t, 165, 400*time.Millisecond)
	// 60 and 165 Hz share a frame every 200 ms (12 and 33 frames).
	for _, at := range []time.Duration{0, 200 * time.Millisecond} {
		a, b := sixty[at], fast[at]
		want := float32(1 - math.Exp(-chaseRate*at.Seconds()))
		if math.Abs(float64(a-b)) > 1e-4 || math.Abs(float64(a-want)) > 1e-4 {
			t.Errorf("at %v: 60 Hz %v, 165 Hz %v, closed form %v", at, a, b, want)
		}
	}
	// 90 % of the move in about 80 ms (Chase.qml:12).
	at := time.Duration(80 * float64(time.Millisecond))
	var near float32
	for when, value := range fast {
		if when <= at && value > near {
			near = value
		}
	}
	if near < 0.85 || near > 0.92 {
		t.Errorf("after about 80 ms at 165 Hz the chase stands at %v, want about 0.9", near)
	}
	if got := sixty[400*time.Millisecond]; got != 1 {
		t.Errorf("after 400 ms the chase stands at %v, want the target", got)
	}
}

// TestChaseRests: no frame is asked for once settled, and a settled chase allocates nothing.
func TestChaseRests(t *testing.T) {
	var router input.Router
	var ops op.Ops
	now := time.Unix(1000, 0)
	var chase Chase
	chase.Snap(3)
	chase.Value(frame(&router, &ops, now))
	if _, wake := woke(&router); wake {
		t.Fatal("a snapped chase asked for a frame")
	}
	chase.Set(4)
	for i := range 200 {
		now = now.Add(time.Second / 165)
		chase.Value(frame(&router, &ops, now))
		if _, wake := woke(&router); !wake {
			if chase.Value(frame(&router, &ops, now)) != 4 {
				t.Fatalf("stopped asking for frames short of the target at frame %d", i)
			}
			break
		}
	}
	chase.Value(frame(&router, &ops, now.Add(time.Second)))
	if _, wake := woke(&router); wake {
		t.Fatal("a settled chase asked for a frame")
	}
	gtx := frame(&router, &ops, now)
	if allocs := testing.AllocsPerRun(100, func() { chase.Value(gtx) }); allocs != 0 {
		t.Errorf("a settled chase allocates %v per frame", allocs)
	}
}

// TestChaseLongFrame: a stalled frame steps at most 100 ms' worth (Chase.qml:31).
func TestChaseLongFrame(t *testing.T) {
	var router input.Router
	var ops op.Ops
	now := time.Unix(1000, 0)
	var chase Chase
	chase.Set(1)
	chase.Value(frame(&router, &ops, now))
	got := chase.Value(frame(&router, &ops, now.Add(5*time.Second)))
	want := float32(1 - math.Exp(-chaseRate*chaseLongest))
	if math.Abs(float64(got-want)) > 1e-5 {
		t.Errorf("after a 5 s stall the chase stands at %v, want %v", got, want)
	}
}

// TestChaseReduced: under reduced motion the chase jumps and asks for nothing.
func TestChaseReduced(t *testing.T) {
	SetReduce(true)
	defer SetReduce(false)
	var router input.Router
	var ops op.Ops
	var chase Chase
	chase.Set(7)
	if got := chase.Value(frame(&router, &ops, time.Unix(1000, 0))); got != 7 {
		t.Errorf("reduced chase stands at %v, want 7", got)
	}
	if _, wake := woke(&router); wake {
		t.Error("reduced chase asked for a frame")
	}
}
