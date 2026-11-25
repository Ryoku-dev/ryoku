package render

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/design"

	"ryoku-rashin/internal/oni/spec"
)

func handsPill() (string, []CardPill) {
	return "Hands is working", []CardPill{{Action: "stop", Text: "Stop"}, {Action: "resume", Text: "Resume", Primary: true}}
}

// The pill is one row: the line first, then its actions, none overlapping,
// inside the padding, and each action hit where it is drawn.
func TestThePillIsOneRowWithItsActionsAfterItsLine(t *testing.T) {
	s, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	text, pills := handsPill()
	k := LayoutPill("", text, pills, s)
	if k.Empty() || len(k.Lines) != 1 || len(k.Pills) != 2 {
		t.Fatalf("pill = %+v", k)
	}
	if k.Size.X > s.Strand.Bubble.MaxWidthPx {
		t.Fatalf("the pill is %v wide, more than %v", k.Size.X, s.Strand.Bubble.MaxWidthPx)
	}
	line := k.Lines[0]
	prev := k.Pad + line.W
	for _, p := range k.Pills {
		r := p.Rect
		if r.X < prev {
			t.Fatalf("pill %s at %v overlaps what is before it (ends %v)", p.Action, r, prev)
		}
		if r.Y < k.Pad-0.5 || r.Y+r.H > k.Size.Y-k.Pad+0.5 || r.X+r.W > k.Size.X-k.Pad+0.5 {
			t.Fatalf("pill %s at %v leaves the %v pill's padding", p.Action, r, k.Size)
		}
		if got := k.PillAt(spec.Point{X: r.X + r.W/2, Y: r.Y + r.H/2}); got != p.Action {
			t.Fatalf("the middle of %s hits %q", p.Action, got)
		}
		prev = r.X + r.W
	}
	if got := k.PillAt(spec.Point{X: k.Pad + 1, Y: k.LinesY[0]}); got != "" {
		t.Fatalf("the line hits %q", got)
	}
	if k := LayoutPill("", "", nil, s); !k.Empty() {
		t.Fatalf("an empty pill is %v", k.Size)
	}
	if k := LayoutPill("In org.example.Editor", "", nil, s); !k.Empty() {
		t.Fatalf("a scope alone lays out a %v pill", k.Size)
	}
}

// A scope line sits above the row in the kicker's type: the row and its
// actions move down by it, nothing overlaps, the pill holds the longer of
// the two, and the actions still hit where they are drawn.
func TestThePillsScopeLineSitsAboveItsRow(t *testing.T) {
	s, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	text, pills := handsPill()
	plain := LayoutPill("", text, pills, s)
	scoped := LayoutPill("On Hyakki's separate desktop, away from yours", text, pills, s)
	if plain.Kicker.W != 0 || scoped.Kicker.W == 0 {
		t.Fatalf("kicker widths %v and %v", plain.Kicker.W, scoped.Kicker.W)
	}
	drop := scoped.LinesY[0] - plain.LinesY[0]
	if !(drop >= scoped.Kicker.H()) {
		t.Fatalf("the row moved %v down under a %v-high scope line", drop, scoped.Kicker.H())
	}
	if scoped.Size.Y-plain.Size.Y != drop {
		t.Fatalf("the pill grew %v for a %v drop", scoped.Size.Y-plain.Size.Y, drop)
	}
	if scoped.Size.X < scoped.Pad*2+scoped.Kicker.W || scoped.Size.X < plain.Size.X || scoped.Size.X > s.Strand.Bubble.MaxWidthPx {
		t.Fatalf("scoped pill %v wide for a %v kicker, plain %v", scoped.Size.X, scoped.Kicker.W, plain.Size.X)
	}
	for i, p := range scoped.Pills {
		r := p.Rect
		if r.Y != plain.Pills[i].Rect.Y+drop || r.Y < scoped.KickerY {
			t.Fatalf("pill %s at %v, kicker baseline %v", p.Action, r, scoped.KickerY)
		}
		if got := scoped.PillAt(spec.Point{X: r.X + r.W/2, Y: r.Y + r.H/2}); got != p.Action {
			t.Fatalf("the middle of %s hits %q", p.Action, got)
		}
	}
	if got := scoped.PillAt(spec.Point{X: scoped.Pad + 1, Y: scoped.KickerY}); got != "" {
		t.Fatalf("the scope line hits %q", got)
	}
}

// The pill wears the desktop's corners and only the look's colours, as the
// card does.
func TestThePillWearsTheDesktopsShapeAndTheLooksColours(t *testing.T) {
	s, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	text, pills := handsPill()
	k := LayoutPill("In org.example.Editor", text, pills, s)
	const dpr = 4.0
	ch := s.Perch.Chip
	one := color.NRGBA{R: 37, G: 151, B: 90, A: 255}
	for _, d := range designs {
		img := image.NewNRGBA(image.Rect(0, 0, int(math.Ceil(k.Size.X*dpr)), int(math.Ceil(k.Size.Y*dpr))))
		PaintCard(img, k, s, perchLook(d), dpr, 1, "")
		corner := float64(d.Corner(float32(ch.RadiusPx+ch.PadYPx), float32(k.Size.X), float32(k.Size.Y))) * dpr
		at := int(0.36 * corner)
		inside := img.NRGBAAt(at, at).A > 0
		if d == design.Lithic && inside {
			t.Errorf("lithic: the pill's corner is rounded, not cut")
		}
		if d != design.Lithic && !inside {
			t.Errorf("%s: the pill's corner is cut, not rounded", d)
		}
		look := Look{Ink: one, White: one, Body: one, Design: d}
		PaintCard(img, k, s, look, 2, 1, "stop")
		for i := 0; i < len(img.Pix); i += 4 {
			px := img.Pix[i : i+4]
			if px[3] == 0 {
				continue
			}
			for c, want := range []uint8{one.R, one.G, one.B} {
				if diff := int(px[c]) - int(want); diff < -1 || diff > 1 {
					t.Fatalf("%s: pixel %d is %v, a colour the look does not have", d, i/4, px)
				}
			}
		}
	}
}

// The glow covers the four edges StripPx deep, strongest at the edge and
// gone at its inner side, in the look's accent; its inner corner is cut in
// Lithic and round in the others.
func TestTheGlowHugsTheEdgesInTheDesktopsShape(t *testing.T) {
	s, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	S := s.Hands.Glow.StripPx
	out := image.Pt(1280, 720)
	accent := color.NRGBA{R: 214, G: 160, B: 40, A: 255}
	look := Look{Ink: color.NRGBA{A: 255}, Accent: map[string]color.NRGBA{"thinking": accent}}
	for _, d := range designs {
		look.Design = d
		g := NewEdgeGlow(out, S, d, 1)
		if !g.Fits(out, S, d, 1) || g.Fits(out, S, d, 2) {
			t.Fatalf("%s: Fits is wrong", d)
		}
		want := [GlowStrips]image.Rectangle{
			image.Rect(0, 0, 1280, S), image.Rect(0, 720-S, 1280, 720),
			image.Rect(0, S, S, 720-S), image.Rect(1280-S, S, 1280, 720-S),
		}
		if g.Rects != want {
			t.Fatalf("%s: strips %v, want %v", d, g.Rects, want)
		}
		var strips [GlowStrips]*image.NRGBA
		for i := range GlowStrips {
			sz := g.DeviceSize(i)
			strips[i] = image.NewNRGBA(image.Rectangle{Max: sz})
			g.Paint(i, strips[i], look, 1)
		}
		top := strips[GlowTop]
		alpha := func(x, y int) uint8 { return top.NRGBAAt(x, y).A }
		if a := alpha(640, 0); a < 250 {
			t.Errorf("%s: the edge itself is %d strong", d, a)
		}
		if a := alpha(640, S-1); a > 2 {
			t.Errorf("%s: the inner side is %d strong", d, a)
		}
		if px := top.NRGBAAt(640, 10); px.R != accent.R || px.G != accent.G || px.B != accent.B {
			t.Errorf("%s: the glow is %v, not the accent", d, px)
		}
		// at 0.7 of the depth from both edges of the corner
		in := int(0.7 * float64(S))
		corner, edge := alpha(in, in), alpha(640, in)
		if d == design.Lithic && corner != edge {
			t.Errorf("lithic: the corner at depth %d is %d, the edge %d; the corner is not cut", in, corner, edge)
		}
		if d != design.Lithic && corner <= edge+8 {
			t.Errorf("%s: the corner at depth %d is %d, the edge %d; the corner is not round", d, in, corner, edge)
		}
		// the strips meet without a seam: the left strip's top row goes on
		// from the top strip's last row
		if a, b := strips[GlowLeft].NRGBAAt(0, 0).A, alpha(0, S-1); int(a)-int(b) > 6 {
			t.Errorf("%s: the left strip starts at %d under a top strip ending at %d", d, a, b)
		}
		// half strength is half
		g.Paint(GlowTop, top, look, 0.5)
		if a := alpha(640, 0); a < 125 || a > 130 {
			t.Errorf("%s: half strength paints %d at the edge", d, a)
		}
	}
	// a tiny output never asks for more than half of it
	g := NewEdgeGlow(image.Pt(100, 60), S, design.Lithic, 0.5)
	if r := g.Rects[GlowTop]; r.Dy() != 30 || !g.Rects[GlowLeft].Empty() {
		t.Fatalf("a 100x60 output got strips %v", g.Rects)
	}
	if sz := g.DeviceSize(GlowTop); sz != image.Pt(50, 15) {
		t.Fatalf("at scale 0.5 the top strip's buffer is %v", sz)
	}
}
