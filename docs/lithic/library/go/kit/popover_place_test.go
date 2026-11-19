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
		{"room below", image.Pt(200, 100), image.Pt(8, 48)},
		// below would end exactly at the bottom edge, so it flips above
		{"flush below flips above", image.Pt(200, 252), image.Pt(8, -260)},
		// neither side fits: shrink on the larger side, never over the anchor
		{"shrunk above the anchor", image.Pt(200, 560), image.Pt(8, -292)},
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

func TestPopoverRoomChoosesASideWithoutCoveringTheAnchor(t *testing.T) {
	viewport := image.Rect(0, 0, 400, 300)
	for _, test := range []struct {
		name       string
		anchor     image.Rectangle
		desired    int
		wantY      int
		wantHeight int
	}{
		{"below fits", image.Rect(20, 20, 140, 60), 100, 68, 100},
		{"above fits", image.Rect(20, 210, 140, 250), 100, 102, 100},
		{"larger below shrinks", image.Rect(20, 120, 140, 160), 432, 168, 124},
		{"larger above shrinks", image.Rect(20, 150, 140, 190), 432, 8, 134},
	} {
		t.Run(test.name, func(t *testing.T) {
			below, above := popoverRoom(viewport, test.anchor, 8, 8)
			height := min(test.desired, max(below, above))
			position := placePopover(viewport, test.anchor, image.Pt(220, height), 8, 8)
			bounds := image.Rectangle{Min: position, Max: position.Add(image.Pt(220, height))}
			if position.Y != test.wantY || height != test.wantHeight {
				t.Fatalf("placed at %v with height %d, want Y %d height %d", position, height, test.wantY, test.wantHeight)
			}
			if !bounds.In(viewport.Inset(8)) || bounds.Overlaps(test.anchor) {
				t.Fatalf("content %v is outside the inset window or covers anchor %v", bounds, test.anchor)
			}
		})
	}
}
