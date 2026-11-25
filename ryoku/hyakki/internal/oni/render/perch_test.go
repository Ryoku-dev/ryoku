package render

import (
	"image"
	"image/color"
	"math"
	"strings"
	"testing"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/design"

	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/strand"
)

func perchLook(d design.Style) Look {
	return Look{
		Ink:    color.NRGBA{R: 20, G: 20, B: 24, A: 255},
		White:  color.NRGBA{R: 240, G: 236, B: 228, A: 255},
		Design: d,
	}
}

func TestTheChipWearsTheDesktopsCorners(t *testing.T) {
	s, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	const dpr = 4.0
	ch := LayoutChip("Fixture · Notes", s)
	for _, d := range []design.Style{design.Lithic, design.Material, design.Adwaita, design.Fluent} {
		img := image.NewNRGBA(image.Rect(0, 0, int(math.Ceil(ch.Size.X*dpr)), int(math.Ceil(ch.Size.Y*dpr))))
		PaintChip(img, ch, s, perchLook(d), dpr, 1)
		corner := float64(d.Corner(float32(s.Perch.Chip.RadiusPx), float32(ch.Size.X), float32(ch.Size.Y))) * dpr
		if corner < 8 {
			t.Fatalf("%s: corner %v device px is too small to tell its shape", d, corner)
		}
		// on the diagonal at 0.36 of the corner: outside a 45-degree cut
		// (which reaches half way), inside a quarter circle (0.29)
		at := int(0.36 * corner)
		inside := img.NRGBAAt(at, at).A > 0
		if d == design.Lithic && inside {
			t.Errorf("lithic: the chip's corner is rounded, not cut")
		}
		if d != design.Lithic && !inside {
			t.Errorf("%s: the chip's corner is cut, not rounded", d)
		}
		if a := img.NRGBAAt(int(0.1*corner), int(0.1*corner)).A; a != 0 {
			t.Errorf("%s: the very corner is painted (alpha %d)", d, a)
		}
		if mid := img.NRGBAAt(img.Bounds().Dx()/2, img.Bounds().Dy()/2); mid.A == 0 {
			t.Errorf("%s: the chip's middle is empty", d)
		}
	}
}

func TestTheHighlightWearsTheDesktopsCorners(t *testing.T) {
	s, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	box := strand.Rect{X: 100, Y: 80, W: 400, H: 300}
	m := HighlightMargin(s)
	origin := image.Pt(int(box.X-m), int(box.Y-m))
	size := image.Pt(int(math.Ceil(box.W+2*m)), int(math.Ceil(box.H+2*m)))
	h := s.Perch.Highlight
	for _, d := range []design.Style{design.Lithic, design.Material} {
		img := image.NewNRGBA(image.Rectangle{Max: size})
		if r := PaintHighlight(img, origin, box, s, perchLook(d), 1, 1); r.Empty() {
			t.Fatalf("%s: the highlight painted nothing", d)
		}
		// the outline's corner point, where a cut and a round corner part
		ring := strand.Rect{X: box.X - h.PadPx - float64(origin.X), Y: box.Y - h.PadPx - float64(origin.Y), W: box.W + 2*h.PadPx, H: box.H + 2*h.PadPx}
		c := float64(d.Corner(float32(h.RadiusPx+h.PadPx), float32(ring.W), float32(ring.H)))
		x, y := int(ring.X+0.3*c), int(ring.Y+0.3*c)
		// a round line passes the diagonal at 0.29c, a cut one at 0.5c
		ink := img.NRGBAAt(x, y).A > 0
		if d == design.Lithic && ink {
			t.Errorf("lithic: the outline rounds its corner")
		}
		if d != design.Lithic && !ink {
			t.Errorf("%s: the outline cuts its corner", d)
		}
	}
}

func TestALongLabelIsElidedToItsWidth(t *testing.T) {
	l := ShapeLabel(strings.Repeat("window title ", 40), 11, 160)
	if l.W > 160 || !strings.HasSuffix(l.Text, "…") || len(l.glyphs) == 0 {
		t.Fatalf("label %q is %v wide with %d glyphs", l.Text, l.W, len(l.glyphs))
	}
	short := ShapeLabel("Notes", 11, 160)
	if short.Text != "Notes" || !(short.W > 0) || !(short.Ascent > 0) {
		t.Fatalf("short label = %+v", short)
	}
}
