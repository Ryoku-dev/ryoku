package look

import (
	"image"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
)

type motionFrame struct {
	operations op.Ops
	router     input.Router
	now        time.Time
	theme      *Theme
}

func newMotionFrame(t *testing.T) *motionFrame {
	t.Helper()
	previous := anim.Reduced()
	anim.SetReduce(false)
	t.Cleanup(func() { anim.SetReduce(previous) })
	return &motionFrame{now: time.Unix(100, 0), theme: New(0)}
}

func (frame *motionFrame) context() layout.Context {
	frame.operations.Reset()
	return layout.Context{
		Ops: &frame.operations, Now: frame.now, Source: frame.router.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(400, 300)),
	}
}

func (frame *motionFrame) finish(after time.Duration) bool {
	frame.router.Frame(&frame.operations)
	_, wake := frame.router.WakeupTime()
	frame.now = frame.now.Add(after)
	return wake
}

func TestSelectionTravelsToANewKeyAndFollowsItsOwnAtOnce(t *testing.T) {
	frame := newMotionFrame(t)
	var slab Selection
	first, second := image.Rect(0, 0, 100, 40), image.Rect(0, 200, 100, 240)
	travel := 200 * time.Millisecond

	if got := slab.Rect(frame.context(), "a", first, travel); got != first {
		t.Fatalf("the first place = %v, want it at once", got)
	}
	frame.finish(time.Millisecond)

	// same key in a new place, like a row when its list scrolls
	moved := first.Add(image.Pt(0, 30))
	if got := slab.Rect(frame.context(), "a", moved, travel); got != moved {
		t.Fatalf("a place that moved was trailed: %v, want %v", got, moved)
	}
	frame.finish(time.Millisecond)

	slab.Rect(frame.context(), "b", second, travel)
	if !frame.finish(travel / 2) {
		t.Fatal("a selection that changed asked for no frame to travel in")
	}
	half := slab.Rect(frame.context(), "b", second, travel)
	if half.Min.Y <= moved.Min.Y || half.Min.Y >= second.Min.Y {
		t.Fatalf("half way the slab stands at %v, want between %v and %v", half, moved, second)
	}
	frame.finish(travel)

	if got := slab.Rect(frame.context(), "b", second, travel); got != second || slab.Moving() {
		t.Fatalf("the slab came to rest at %v moving %v, want %v at rest", got, slab.Moving(), second)
	}
	frame.finish(time.Second)
	slab.Rect(frame.context(), "b", second, travel)
	if frame.finish(time.Millisecond) {
		t.Fatal("a slab at rest asked for another frame")
	}
}

func TestRiseAnimatesOnlyWhatArrivesAfterTheFirstFrame(t *testing.T) {
	frame := newMotionFrame(t)
	var rise Rise
	draw := func(key string) bool {
		rise.Layout(frame.context(), frame.theme, key, 8, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(100, 40)}
		})
		return frame.finish(time.Millisecond)
	}
	if draw("home") {
		t.Fatal("what was there when the window opened animated in")
	}
	if !draw("session") {
		t.Fatal("new content arrived without moving")
	}
	frame.now = frame.now.Add(time.Second)
	draw("session")
	if draw("session") {
		t.Fatal("content at rest asked for another frame")
	}
}

func TestUnfoldTakesNoRoomAndNoInputWhileClosed(t *testing.T) {
	frame := newMotionFrame(t)
	var unfold Unfold
	laid := 0
	draw := func(open bool) layout.Dimensions {
		dimensions := unfold.Layout(frame.context(), frame.theme, open, func(gtx layout.Context) layout.Dimensions {
			laid++
			return layout.Dimensions{Size: image.Pt(200, 120)}
		})
		frame.finish(time.Second)
		return dimensions
	}
	if size := draw(false).Size; size.Y != 0 || laid != 0 {
		t.Fatalf("closed content took %v and was laid out %d times", size, laid)
	}
	draw(true)
	if size := draw(true).Size; size.Y != 120 {
		t.Fatalf("open content stands %d tall, want 120", size.Y)
	}
	draw(false)
	if size := draw(false).Size; size.Y != 0 {
		t.Fatalf("content folded away still takes %d", size.Y)
	}
}
