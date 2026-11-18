package look

import (
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
)

// The sidebar draws a destination's glyph at 20 and the same glyph at 18 in
// Settings' own list, in more than one ink. A widget.Icon keeps one picture,
// so each frame drew every such glyph again and sent it to the GPU anew.
func TestAnIconShownAtTwoSizesIsDrawnOnce(t *testing.T) {
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Constraints: layout.Exact(image.Pt(400, 400))}
	quiet, bright := color.NRGBA{R: 120, G: 120, B: 120, A: 255}, color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	wide := IconWidget(Icon.Teams, 20, bright)
	frame := func() {
		ops.Reset()
		PlaceIcon(gtx, Icon.Teams, image.Pt(0, 0), 20, quiet)
		PlaceIcon(gtx, Icon.Teams, image.Pt(0, 40), 18, quiet)
		PlaceIcon(gtx, Icon.Teams, image.Pt(0, 80), 18, bright)
		wide(gtx)
	}
	frame()
	if allocations := testing.AllocsPerRun(10, frame); allocations > 0 {
		t.Fatalf("a frame of icons already drawn allocated %.0f times", allocations)
	}
}

func TestLongOverlayMeasuresOnlyTheAvailableSide(t *testing.T) {
	anim.SetReduce(true)
	t.Cleanup(func() { anim.SetReduce(false) })
	th := New(palette.Dark, palette.Look{})
	var ops op.Ops
	gtx := layout.Context{
		Ops: &ops, Now: time.Unix(1000, 0),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(600, 600)),
	}
	var overlays Overlays
	var menu kit.Menu
	var tag int
	anchor := image.Rect(300, 270, 500, 310)
	items := make([]kit.MenuItem, 400)
	for index := range items {
		items[index].Label = "Long command"
	}
	measuredHeight := 0
	overlays.Open(&tag, anchor, Placement{}, func(gtx layout.Context) layout.Dimensions {
		measuredHeight = gtx.Constraints.Max.Y
		return menu.Layout(gtx, th.Theme, items)
	})
	overlays.Layout(gtx, th)
	entry := overlays.entry(&tag)
	if measuredHeight != 274 || entry == nil || entry.rect.Dy() != 274 {
		t.Fatalf("long overlay measured at %d, entry=%+v; want the 274-pixel lower side", measuredHeight, entry)
	}
	if !entry.rect.In(image.Rect(8, 8, 592, 592)) || entry.rect.Min.Y != anchor.Max.Y+8 {
		t.Fatalf("shrinking overlay %v moved over its opener %v or outside the window", entry.rect, anchor)
	}
}

func TestOverlayPlacementShrinksOnTheLargerSide(t *testing.T) {
	window := image.Pt(400, 300)
	for _, test := range []struct {
		name    string
		anchor  image.Rectangle
		desired int
		prefer  Side
		want    Side
	}{
		{"below fits", image.Rect(20, 20, 140, 60), 100, Below, Below},
		{"above fits", image.Rect(20, 210, 140, 250), 100, Below, Above},
		{"larger below", image.Rect(20, 120, 140, 160), 432, Below, Below},
		{"larger above", image.Rect(20, 150, 140, 190), 432, Below, Above},
		{"explicit above", image.Rect(20, 140, 140, 180), 80, Above, Above},
	} {
		t.Run(test.name, func(t *testing.T) {
			below, above := overlayRoom(window, test.anchor, 8, 8)
			height := min(test.desired, max(below, above))
			position, side := placeOverlay(window, test.anchor, image.Pt(220, height), 8, 8, test.prefer, Start)
			bounds := image.Rectangle{Min: position, Max: position.Add(image.Pt(220, height))}
			if side != test.want || !bounds.In(image.Rectangle{Max: window}.Inset(8)) || bounds.Overlaps(test.anchor) {
				t.Fatalf("placed %v on side %v, want side %v inside the window without covering %v", bounds, side, test.want, test.anchor)
			}
		})
	}
}
