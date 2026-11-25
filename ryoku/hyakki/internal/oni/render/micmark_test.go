package render

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/design"
)

func micLook(d design.Style) Look {
	l := perchLook(d)
	l.Accent = map[string]color.NRGBA{"listening": {R: 230, G: 170, B: 40, A: 255}}
	return l
}

func TestTheMicMarkWearsEachDesignsCorners(t *testing.T) {
	const dpr = 4.0
	ext := image.Pt(120, 120)
	r := MicMarkRect(ext)
	if r.W != 36 || r.X+r.W != 120 || r.Y+r.H != 120 {
		t.Fatalf("mark rect %+v, want 36 px at the bottom-right", r)
	}
	if small := MicMarkRect(image.Pt(30, 30)); small.W != micMarkMinPx {
		t.Fatalf("a small sprite's mark is %v px", small.W)
	}
	for _, d := range []design.Style{design.Lithic, design.Material, design.Adwaita, design.Fluent} {
		look := micLook(d)
		img := image.NewNRGBA(image.Rect(0, 0, ext.X*dpr, ext.Y*dpr))
		painted := PaintMicMark(img, r, look, dpr)
		if painted.Empty() || painted.Min.X < int(r.X*dpr)-1 || painted.Min.Y < int(r.Y*dpr)-1 {
			t.Fatalf("%s: painted %v outside the mark %+v", d, painted, r)
		}
		// nothing outside the mark's plate
		if a := img.NRGBAAt(int(r.X*dpr)-4, int(r.Y*dpr)-4).A; a != 0 {
			t.Errorf("%s: painted outside the mark", d)
		}
		corner := float64(d.Corner(float32(r.W*micMarkCorner), float32(r.W), float32(r.H))) * dpr
		if corner < 8 {
			t.Fatalf("%s: corner %v device px is too small to tell its shape", d, corner)
		}
		x0, y0 := r.X*dpr, r.Y*dpr
		at := 0.36 * corner
		inside := img.NRGBAAt(int(x0+at), int(y0+at)).A > 0
		if d == design.Lithic && inside {
			t.Errorf("lithic: the mark's corner is rounded, not cut")
		}
		if d != design.Lithic && !inside {
			t.Errorf("%s: the mark's corner is cut, not rounded", d)
		}
		if a := img.NRGBAAt(int(x0+0.1*corner), int(y0+0.1*corner)).A; a != 0 {
			t.Errorf("%s: the very corner is painted (alpha %d)", d, a)
		}
		// the capsule wears the listening accent, the plate beside it White
		s := r.W * dpr
		cx := x0 + s/2
		if c := img.NRGBAAt(int(cx), int(y0+0.3*s)); c != look.Accent["listening"] {
			t.Errorf("%s: the microphone is %v, want the listening accent", d, c)
		}
		if c := img.NRGBAAt(int(x0+0.12*s), int(y0+0.5*s)); c != look.White {
			t.Errorf("%s: the plate is %v, want White", d, c)
		}
		// every opaque pixel is one of the look's colours
		allowed := map[color.NRGBA]bool{look.Ink: true, look.White: true, look.Accent["listening"]: true}
		for y := range img.Bounds().Dy() {
			for x := range img.Bounds().Dx() {
				if c := img.NRGBAAt(x, y); c.A == 255 && !allowed[c] && !between(c, look) {
					t.Fatalf("%s: pixel %d,%d is %v, not a look colour", d, x, y, c)
				}
			}
		}
	}
	// without a listening accent the microphone is Ink
	img := image.NewNRGBA(image.Rect(0, 0, ext.X*dpr, ext.Y*dpr))
	PaintMicMark(img, r, perchLook(design.Lithic), dpr)
	if c := img.NRGBAAt(int((r.X+r.W/2)*dpr), int((r.Y+0.3*r.W)*dpr)); c != perchLook(design.Lithic).Ink {
		t.Fatalf("no accent: the microphone is %v", c)
	}
}

// between reports whether c is an antialiased blend of two look colours.
func between(c color.NRGBA, l Look) bool {
	cs := []color.NRGBA{l.Ink, l.White}
	for _, a := range l.Accent {
		cs = append(cs, a)
	}
	for i, a := range cs {
		for _, b := range cs[i+1:] {
			if onSegment(c, a, b) {
				return true
			}
		}
	}
	return false
}

func onSegment(c, a, b color.NRGBA) bool {
	d := [3]float64{float64(b.R) - float64(a.R), float64(b.G) - float64(a.G), float64(b.B) - float64(a.B)}
	p := [3]float64{float64(c.R) - float64(a.R), float64(c.G) - float64(a.G), float64(c.B) - float64(a.B)}
	dd := d[0]*d[0] + d[1]*d[1] + d[2]*d[2]
	if dd == 0 {
		return false
	}
	k := (p[0]*d[0] + p[1]*d[1] + p[2]*d[2]) / dd
	if k < 0 || k > 1 {
		return false
	}
	for i := range 3 {
		if math.Abs(p[i]-k*d[i]) > 3 {
			return false
		}
	}
	return true
}
