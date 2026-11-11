package look

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
)

func TestATipStandsUnderItsControlWhereverThePointerCameFrom(t *testing.T) {
	frame := newMotionFrame(t)
	var overlays Overlays
	var tip Tip
	control := image.Rect(120, 40, 220, 70)
	draw := func() {
		gtx := frame.context()
		offset := op.Offset(control.Min).Push(gtx.Ops)
		tip.Layout(gtx, frame.theme, &overlays, "Teams", func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: control.Size()}
		})
		offset.Pop()
		overlays.Layout(gtx, frame.theme)
		frame.finish(16 * time.Millisecond)
	}
	draw()
	// a hand that comes from far away: the last step onto the control is a long one
	for _, at := range []f32.Point{{X: 380, Y: 280}, {X: 300, Y: 200}, {X: 130, Y: 50}} {
		frame.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: at, Time: frame.now.Sub(time.Unix(100, 0))})
		draw()
	}
	for range 60 {
		draw()
	}
	entry := overlays.entry(&tip.tag)
	if entry == nil || !entry.desired {
		t.Fatal("the tip never opened under a resting pointer")
	}
	if entry.anchor != control {
		t.Fatalf("the tip is anchored at %v, want its control at %v", entry.anchor, control)
	}
}
