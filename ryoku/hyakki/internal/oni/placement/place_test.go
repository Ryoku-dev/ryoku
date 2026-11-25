package placement

import (
	"fmt"
	"image"
	"math"
	"testing"

	"ryoku-rashin/internal/oni/spec"
)

func mustSpec(t testing.TB) *spec.OniSpec {
	t.Helper()
	s, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// halfPx is the rounding a whole-pixel placement may add to a fraction.
const halfPx = 0.5 + 1e-9

// panel is a 2560x1440 panel at a fractional scale, placed right of a
// 1920-px-wide logical output, as a compositor reports it in logical px.
func panel(scale float64) Output {
	w, h := int(math.Round(2560/scale)), int(math.Round(1440/scale))
	return Output{ID: fmt.Sprintf("DP-%v", scale), Rect: image.Rect(1920, 0, 1920+w, h), Scale: scale}
}

func frameFor(s *spec.OniSpec, size string) image.Point {
	f, _ := s.Size(size)
	px := int(math.Round(float64(s.Character.ButtonPx) * f))
	return image.Pt(px, px)
}

func TestResolveAcrossScalesAndSizes(t *testing.T) {
	s := mustSpec(t)
	m := s.Placement.CornerMarginPx
	for _, scale := range []float64{1, 1.25, 1.5, 2} {
		o := panel(scale)
		for _, size := range spec.SizeIDs {
			fr := frameFor(s, size)
			name := fmt.Sprintf("x%v/%s", scale, size)
			corners := map[string]image.Point{
				"tl": {o.Rect.Min.X + m, o.Rect.Min.Y + m},
				"tr": {o.Rect.Max.X - m - fr.X, o.Rect.Min.Y + m},
				"bl": {o.Rect.Min.X + m, o.Rect.Max.Y - m - fr.Y},
				"br": {o.Rect.Max.X - m - fr.X, o.Rect.Max.Y - m - fr.Y},
			}
			for corner, want := range corners {
				if got := Resolve(o, fr, corner, &Saved{FX: 0.5, FY: 0.5}, m); got != want {
					t.Errorf("%s pinned %s at %v, want %v", name, corner, got, want)
				}
			}
			if got := Resolve(o, fr, "", nil, m); got != corners[DefaultCorner] {
				t.Errorf("%s with nothing saved sits at %v, want the %s pin", name, got, DefaultCorner)
			}
			if got := Resolve(o, fr, "middle", nil, m); got != corners[DefaultCorner] {
				t.Errorf("%s unknown corner sits at %v", name, got)
			}

			for _, frac := range []Saved{{0.1, 0.2}, {0.5, 0.5}, {0.77, 0.31}, {0.9, 0.85}} {
				p := Resolve(o, fr, "", &frac, m)
				back := Fraction(o, p, fr)
				if math.Abs(back.FX-frac.FX)*float64(o.Rect.Dx()-fr.X) > halfPx || math.Abs(back.FY-frac.FY)*float64(o.Rect.Dy()-fr.Y) > halfPx {
					t.Errorf("%s fraction %v resolved to %v and back to %v", name, frac, p, back)
				}
				if again := Resolve(o, fr, "", &back, m); again != p {
					t.Errorf("%s drop at %v resolves back to %v", name, p, again)
				}
			}
		}
	}
	// One saved fraction lands on the same relative spot at every scale.
	fr := frameFor(s, "normal")
	frac := Saved{FX: 0.6, FY: 0.4}
	for _, scale := range []float64{1, 1.25, 1.5, 2} {
		o := panel(scale)
		p := Resolve(o, fr, "", &frac, m)
		rx := float64(p.X-o.Rect.Min.X) / float64(o.Rect.Dx()-fr.X)
		ry := float64(p.Y-o.Rect.Min.Y) / float64(o.Rect.Dy()-fr.Y)
		if math.Abs(rx-0.6) > 1/float64(o.Rect.Dx()-fr.X) || math.Abs(ry-0.4) > 1/float64(o.Rect.Dy()-fr.Y) {
			t.Errorf("x%v: fraction landed at %.4f,%.4f", scale, rx, ry)
		}
	}
}

func TestClampKeepsMarginInside(t *testing.T) {
	s := mustSpec(t)
	m := s.Placement.CornerMarginPx
	if m != 40 {
		t.Fatalf("corner margin %d, want 40", m)
	}
	for _, scale := range []float64{1, 1.25, 1.5, 2} {
		o := panel(scale)
		fr := frameFor(s, "big")
		for _, c := range []struct {
			in, want image.Point
		}{
			{image.Pt(o.Rect.Min.X-500, o.Rect.Min.Y-3), image.Pt(o.Rect.Min.X+m, o.Rect.Min.Y+m)},
			{image.Pt(o.Rect.Min.X+10, o.Rect.Max.Y), image.Pt(o.Rect.Min.X+m, o.Rect.Max.Y-m-fr.Y)},
			{image.Pt(o.Rect.Max.X-fr.X, o.Rect.Min.Y+m+1), image.Pt(o.Rect.Max.X-m-fr.X, o.Rect.Min.Y+m+1)},
			{image.Pt(o.Rect.Min.X+m, o.Rect.Min.Y+m), image.Pt(o.Rect.Min.X+m, o.Rect.Min.Y+m)},
			{image.Pt(o.Rect.Min.X+300, o.Rect.Min.Y+200), image.Pt(o.Rect.Min.X+300, o.Rect.Min.Y+200)},
		} {
			if got := Clamp(o, c.in, fr, m); got != c.want {
				t.Errorf("x%v: clamp %v = %v, want %v", scale, c.in, got, c.want)
			}
		}
		saved := Fraction(o, image.Pt(o.Rect.Max.X, o.Rect.Max.Y), fr)
		if saved != (Saved{1, 1}) {
			t.Errorf("x%v: a point past the output saved %v", scale, saved)
		}
		if got := Resolve(o, fr, "", &saved, m); got != image.Pt(o.Rect.Max.X-m-fr.X, o.Rect.Max.Y-m-fr.Y) {
			t.Errorf("x%v: saved edge fraction resolved outside the margin: %v", scale, got)
		}
	}
	tiny := Output{ID: "tiny", Rect: image.Rect(0, 0, 150, 1000), Scale: 1}
	fr := image.Pt(100, 100)
	if got := Clamp(tiny, image.Pt(0, 0), fr, m); got != image.Pt(25, m) {
		t.Errorf("an output narrower than frame and margins must centre the frame: %v", got)
	}
	if f := Fraction(tiny, image.Pt(25, 500), image.Pt(200, 100)); f.FX != 0.5 {
		t.Errorf("a negative free span gave fraction %v", f.FX)
	}
}

func TestChooseFallsBackWhenTheOutputIsRemoved(t *testing.T) {
	left := Output{ID: "eDP-1", Rect: image.Rect(0, 0, 1920, 1200), Scale: 1.25}
	right := panel(1.5)
	place := Place{Corner: "", Active: right.ID, Outputs: map[string]Saved{right.ID: {0.25, 0.75}, left.ID: {0.9, 0.1}}}

	if o, ok := Choose([]Output{left, right}, place, left.ID); !ok || o.ID != right.ID {
		t.Fatalf("active output present: chose %q", o.ID)
	}
	o, ok := Choose([]Output{left}, place, left.ID)
	if !ok || o.ID != left.ID {
		t.Fatalf("active output removed: chose %q, %v", o.ID, ok)
	}
	saved := place.Outputs[o.ID]
	fr := image.Pt(72, 72)
	p := Resolve(o, fr, place.Corner, &saved, 40)
	back := Fraction(o, p, fr)
	if !p.In(o.Rect) || math.Abs(back.FX-saved.FX)*float64(o.Rect.Dx()-fr.X) > halfPx || math.Abs(back.FY-saved.FY)*float64(o.Rect.Dy()-fr.Y) > halfPx {
		t.Fatalf("fallback output placement %v does not honour its own fraction %v", p, saved)
	}
	third := Output{ID: "HDMI-A-1", Rect: image.Rect(-1920, 0, 0, 1080), Scale: 1}
	if o, ok := Choose([]Output{third, left}, place, "gone"); !ok || o.ID != third.ID {
		t.Fatalf("no active or focused output: chose %q, want the first", o.ID)
	}
	if o, ok := Choose([]Output{third, left}, Place{}, left.ID); !ok || o.ID != left.ID {
		t.Fatalf("nothing remembered: chose %q, want focused", o.ID)
	}
	if _, ok := Choose(nil, place, left.ID); ok {
		t.Fatal("chose an output from none")
	}
	if got := Resolve(third, fr, "", nil, 40); got != image.Pt(third.Rect.Max.X-40-72, third.Rect.Max.Y-40-72) {
		t.Fatalf("an output with no saved fraction did not get the default pin: %v", got)
	}
}
