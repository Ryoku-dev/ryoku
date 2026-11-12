package look

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
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
