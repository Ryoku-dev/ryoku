package kit

import (
	"testing"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

// newPlacardHarness lays out a placard stack in the harness's 800x600 window,
// where a lone 560 wide placard spans x 120 to 680 and y 528 to 576. Each
// harness frame moves the clock one second.
func newPlacardHarness(t *testing.T) (*Placard, *keyHarness) {
	th := theme.New(palette.Signature(true))
	placard := &Placard{}
	harness := newKeyHarness(t, func(gtx layout.Context) {
		placard.Layout(gtx, th)
	})
	return placard, harness
}

func (h *keyHarness) pointerTo(x, y float32) {
	h.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(x, y)})
}

func TestPlacardLifeStandsStillWhileHovered(t *testing.T) {
	placard, h := newPlacardHarness(t)
	placard.Push(h.now, "Wallpaper changed", "Undo")
	h.frame(nil) // one second gone; the placard's area is now on screen
	h.pointerTo(400, 552)
	for range 8 {
		h.frame(nil)
	}
	if len(placard.entries) != 1 {
		t.Fatal("a hovered placard ran out while the pointer rested on it")
	}
	h.pointerTo(10, 10)
	h.frame(nil)
	if len(placard.entries) != 1 {
		t.Fatal("the placard closed the moment the pointer left; its life should resume where it stood")
	}
	for range 5 {
		h.frame(nil)
	}
	if len(placard.entries) != 0 {
		t.Fatal("the placard outlived its resumed life after the pointer left")
	}
}

func TestPlacardLifeStandsStillWhileItsActionHoldsFocus(t *testing.T) {
	placard, h := newPlacardHarness(t)
	placard.Push(h.now, "Wallpaper changed", "Undo")
	h.frame(nil)
	h.frame(func(gtx layout.Context) {
		gtx.Execute(key.FocusCmd{Tag: &placard.entries[0].button.Clickable})
	})
	for range 8 {
		h.frame(nil)
	}
	if len(placard.entries) != 1 {
		t.Fatal("a placard whose Undo held focus ran out")
	}
	h.frame(func(gtx layout.Context) { gtx.Execute(key.FocusCmd{}) })
	for range 6 {
		h.frame(nil)
	}
	if len(placard.entries) != 0 {
		t.Fatal("the placard outlived its resumed life after focus left")
	}
}
