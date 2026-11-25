package surface

import (
	"image"
	"slices"
	"testing"
	"time"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/design"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
)

// inMasks reports whether logical point (x, y) lies in one of rects.
func inMasks(rects []protocol.OniRect, x, y float64) bool {
	for _, r := range rects {
		if x >= r.X && x <= r.X+r.W && y >= r.Y && y <= r.Y+r.H {
			return true
		}
	}
	return false
}

// strandLeaks counts the strand pixels of p that no mask covers: every
// corner of a drawn device pixel must lie in some mask, or a capture that
// paints the masks out still shows part of the thread.
func strandLeaks(p strandPresentation, masks []protocol.OniRect) int {
	b := p.img.Bounds()
	dpr := float64(b.Dx()) / float64(p.rect.Dx())
	leaks := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if p.img.NRGBAAt(x, y).A == 0 {
				continue
			}
			x0, y0 := float64(p.rect.Min.X)+float64(x-b.Min.X)/dpr, float64(p.rect.Min.Y)+float64(y-b.Min.Y)/dpr
			x1, y1 := x0+1/dpr, y0+1/dpr
			if !inMasks(masks, x0, y0) || !inMasks(masks, x1, y0) || !inMasks(masks, x0, y1) || !inMasks(masks, x1, y1) {
				leaks++
			}
		}
	}
	return leaks
}

// The masks hide every strand pixel of every committed frame, the landing
// burst included, in every mode the thread points with, yet leave most of
// the target it points at in the picture: a settle watch compares only what
// neither picture masks, and the change a step asks for happens there.
func TestStrandMasksHugTheThreadAndLeaveTheTargetInThePicture(t *testing.T) {
	// fractions of the perched window, perchBox, Oni on its top right
	region := &feature.Box{X: 0.02, Y: 0.02, W: 0.5, H: 0.4}
	// open is the share of the target a capture must still show: the
	// pointing thread stops short of its target, but the lasso's ring and
	// the region's pen cross its edges (by up to 4.3 and 5.4 px in the
	// strand package's own goldens), so their masks take the edges
	for _, tc := range []struct {
		name   string
		target *feature.StrandTargetFrame
		open   float64
	}{
		{"point at a menu", &feature.StrandTargetFrame{Seq: 2, X: 0.02, Y: 0.02, W: 0.06, H: 0.05}, 0.9},
		{"point mid window", &feature.StrandTargetFrame{Seq: 2, X: 0.4, Y: 0.45, W: 0.1, H: 0.06}, 0.9},
		{"point near Oni", &feature.StrandTargetFrame{Seq: 2, X: 0.85, Y: 0.1, W: 0.1, H: 0.06}, 0.9},
		{"point below", &feature.StrandTargetFrame{Seq: 2, X: 0.6, Y: 0.9, W: 0.1, H: 0.06}, 0.9},
		{"lasso", &feature.StrandTargetFrame{Seq: 2, X: 0.02, Y: 0.02, W: 0.06, H: 0.05, Mode: feature.StrandModeLasso}, 0.45},
		{"region", &feature.StrandTargetFrame{Seq: 2, X: 0.1, Y: 0.1, W: 0.1, H: 0.06, Mode: feature.StrandModeRegion, Region: region}, 0.45},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, d, _, now := perchedHost(t)
			h.look = markLook(design.Lithic)
			send(t, h, tc.target)
			// step drives one frame and checks that the masks the host
			// reports cover every strand pixel it committed
			checked := 0
			step := func(i int) {
				n := len(d.strands)
				now = now.Add(16 * time.Millisecond)
				h.step(now)
				masks := h.overlayRects()
				if len(masks) > feature.MaxOverlayRects {
					t.Fatalf("frame %d reports %d rects, over the wire's %d", i, len(masks), feature.MaxOverlayRects)
				}
				if len(d.strands) > n && h.strandShown {
					if leaks := strandLeaks(d.strands[len(d.strands)-1], masks); leaks > 0 {
						t.Fatalf("frame %d (%s): %d strand pixels outside the masks %+v", i, h.strandF.Phase, leaks, masks)
					}
					checked++
				}
			}
			held := 0
			for i := 0; held < 30; i++ {
				if i > 900 {
					t.Fatalf("the thread never held: landed=%d", h.landed)
				}
				step(i)
				if h.target != nil && h.target.Seq == 2 && h.landed == 2 {
					held++
				}
			}
			if checked < 10 {
				t.Fatalf("only %d strand frames were checked", checked)
			}
			// held through a breath, the masks grow round it, then hold
			// still: an idle thread sends no reports
			for i := range 300 {
				step(i)
			}
			masks := slices.Clone(h.overlayRects())
			for i := range 300 {
				step(i)
				if got := h.overlayRects(); !slices.Equal(got, masks) {
					t.Fatalf("a held thread's masks moved at frame %d: %+v, were %+v", i, got, masks)
				}
			}
			// the share of the target's pixels a capture still shows
			b := h.target.Box
			tr := image.Rect(int(b.X), int(b.Y), int(b.X+b.W), int(b.Y+b.H))
			open := 0
			for y := tr.Min.Y; y < tr.Max.Y; y++ {
				for x := tr.Min.X; x < tr.Max.X; x++ {
					if !inMasks(masks, float64(x)+0.5, float64(y)+0.5) {
						open++
					}
				}
			}
			share := float64(open) / float64(tr.Dx()*tr.Dy())
			t.Logf("%.0f%% of the target %v stays in the picture", 100*share, tr)
			if share < tc.open {
				t.Fatalf("the masks %+v leave %.0f%% of the target %v in the picture, want at least %.0f%%", masks, 100*share, tr, 100*tc.open)
			}
			// drawn back, the thread sheds the masks of its reach
			send(t, h, &feature.StrandRetractFrame{Seq: 2})
			for i := range 240 {
				step(i)
			}
			for _, m := range h.overlayRects() {
				if image.Rect(int(m.X), int(m.Y), int(m.X+m.W), int(m.Y+m.H)).Overlaps(tr) {
					t.Fatalf("drawn back, the mask %+v still covers the target %v", m, tr)
				}
			}
		})
	}
}
