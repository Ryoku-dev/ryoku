package anim

import (
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/op"
)

const (
	navigate = 200 * time.Millisecond
	stagger  = 40 * time.Millisecond
	riseBy   = 8
)

// TestRevealPlaysOnce: group 2 waits three staggers, rises over the move on the chisel curve,
// then stands for good and asks for nothing more.
func TestRevealPlaysOnce(t *testing.T) {
	var router input.Router
	var ops op.Ops
	start := time.Unix(1000, 0)
	reveal := Reveal{Order: 2}
	opacity, offset := reveal.Step(frame(&router, &ops, start), navigate, stagger, riseBy)
	if opacity != 0 || offset != riseBy {
		t.Fatalf("before its turn: %v, %v; want 0, %v", opacity, offset, riseBy)
	}
	at, wake := woke(&router)
	if want := start.Add(3 * stagger); !wake || !at.Equal(want) {
		t.Fatalf("asked for a frame at %v (%v), want at its turn %v", at, wake, want)
	}
	middle := start.Add(3*stagger + navigate/2)
	opacity, offset = reveal.Step(frame(&router, &ops, middle), navigate, stagger, riseBy)
	if want := EaseChisel(0.5); opacity != want || offset != riseBy*(1-want) {
		t.Errorf("halfway: %v, %v; want %v, %v", opacity, offset, want, riseBy*(1-want))
	}
	if _, wake := woke(&router); !wake {
		t.Error("a moving reveal asked for no frame")
	}
	end := start.Add(3*stagger + navigate)
	if opacity, offset = reveal.Step(frame(&router, &ops, end), navigate, stagger, riseBy); opacity != 1 || offset != 0 || !reveal.Done() {
		t.Errorf("at the end: %v, %v", opacity, offset)
	}
	if _, wake := woke(&router); wake {
		t.Error("a risen group asked for a frame")
	}
	// never re-arms: a later step, however late, stands
	gtx := frame(&router, &ops, end.Add(time.Hour))
	if opacity, offset = reveal.Step(gtx, navigate, stagger, riseBy); opacity != 1 || offset != 0 {
		t.Errorf("re-armed: %v, %v", opacity, offset)
	}
	if allocs := testing.AllocsPerRun(100, func() { reveal.Step(gtx, navigate, stagger, riseBy) }); allocs != 0 {
		t.Errorf("a risen group allocates %v per frame", allocs)
	}
}

// TestRevealFirstFive: only the first five groups rise (SettingsPage.qml:773-777).
func TestRevealFirstFive(t *testing.T) {
	var router input.Router
	var ops op.Ops
	gtx := frame(&router, &ops, time.Unix(1000, 0))
	for order := range 8 {
		reveal := Reveal{Order: order}
		opacity, _ := reveal.Step(gtx, navigate, stagger, riseBy)
		if rises := opacity == 0; rises != (order < RevealGroups) {
			t.Errorf("group %d rises = %v", order, rises)
		}
	}
}

// TestRevealReduced: reduced motion never arms it.
func TestRevealReduced(t *testing.T) {
	SetReduce(true)
	var router input.Router
	var ops op.Ops
	reveal := Reveal{}
	opacity, offset := reveal.Step(frame(&router, &ops, time.Unix(1000, 0)), navigate, stagger, riseBy)
	SetReduce(false)
	if opacity != 1 || offset != 0 || !reveal.Done() {
		t.Errorf("reduced: %v, %v", opacity, offset)
	}
	if _, wake := woke(&router); wake {
		t.Error("reduced reveal asked for a frame")
	}
	// and a later frame with motion back on does not arm it either
	if opacity, _ = reveal.Step(frame(&router, &ops, time.Unix(1001, 0)), navigate, stagger, riseBy); opacity != 1 {
		t.Error("re-armed after reduced motion")
	}
}

// TestRise: the page change fades over 240 ms and slides 14 dp over 300 ms on OutCubic, then
// rests (Hub.qml:1004-1022).
func TestRise(t *testing.T) {
	var router input.Router
	var ops op.Ops
	start := time.Unix(1000, 0)
	var rise Rise
	if opacity, offset := rise.Value(frame(&router, &ops, start)); opacity != 1 || offset != 0 {
		t.Fatalf("an idle rise: %v, %v", opacity, offset)
	}
	rise.Start(frame(&router, &ops, start))
	opacity, offset := rise.Value(frame(&router, &ops, start))
	if opacity != 0 || offset != 14 {
		t.Errorf("at the start: %v, %v; want 0, 14", opacity, offset)
	}
	opacity, offset = rise.Value(frame(&router, &ops, start.Add(120*time.Millisecond)))
	if opacity != EaseOutCubic(0.5) || offset != 14*(1-EaseOutCubic(0.4)) {
		t.Errorf("at 120 ms: %v, %v", opacity, offset)
	}
	opacity, offset = rise.Value(frame(&router, &ops, start.Add(260*time.Millisecond)))
	if opacity != 1 || offset <= 0 {
		t.Errorf("at 260 ms the fade is over but the slide is not: %v, %v", opacity, offset)
	}
	woke(&router)
	if opacity, offset = rise.Value(frame(&router, &ops, start.Add(300*time.Millisecond))); opacity != 1 || offset != 0 || rise.Active() {
		t.Errorf("at 300 ms: %v, %v", opacity, offset)
	}
	if _, wake := woke(&router); wake {
		t.Error("a finished rise asked for a frame")
	}
	SetReduce(true)
	rise.Start(frame(&router, &ops, start))
	SetReduce(false)
	if rise.Active() {
		t.Error("a rise started under reduced motion")
	}
}
