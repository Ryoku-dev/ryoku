package kit

import (
	"image"
	"testing"
)

func TestPlacePopoverKeepsAMarginFromTheEdges(t *testing.T) {
	// an anchor 40 high at the top-left of a 400 by 300 area, as Popover
	// lays it out, with room above as far as the area is tall
	viewport := image.Rect(0, -300, 400, 300)
	anchor := image.Rect(0, 0, 120, 40)
	cases := []struct {
		name    string
		content image.Point
		want    image.Point
	}{
		{"room below", image.Pt(200, 100), image.Pt(0, 48)},
		// below would end exactly at the bottom edge, so it flips above
		{"flush below flips above", image.Pt(200, 252), image.Pt(0, -260)},
		// neither side fits: clamped 8 inside the bottom edge
		{"clamped inside the edge", image.Pt(200, 560), image.Pt(0, -268)},
	}
	for _, test := range cases {
		if got := placePopover(viewport, anchor, test.content, 8, 8); got != test.want {
			t.Errorf("%s: placePopover = %v, want %v", test.name, got, test.want)
		}
	}
	// in a preview the area is the window, so the far edges keep the margin
	preview := image.Rect(0, 0, 400, 300)
	if got := placePopover(preview, image.Rect(300, 0, 340, 40), image.Pt(200, 100), 8, 8); got.X != 192 {
		t.Errorf("a popover near the right edge sits at %d, want 192 (8 inside the edge)", got.X)
	}
}
