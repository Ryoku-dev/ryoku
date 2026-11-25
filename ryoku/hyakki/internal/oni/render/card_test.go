package render

import (
	"image"
	"image/color"
	"math"
	"strings"
	"testing"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/design"

	"ryoku-rashin/internal/oni/spec"
)

var designs = []design.Style{design.Lithic, design.Material, design.Adwaita, design.Fluent}

func guideCard() CardContent {
	return CardContent{
		Kicker: "Step 2",
		Text:   "Press Save in the File menu.",
		Hint:   "Click it yourself, or press Next",
		Items:  []CardItem{{CardDone, "Open the File menu"}, {CardNow, "Press Save"}, {CardNext, "Name the file"}},
		Pills:  []CardPill{{Action: "stop", Text: "Stop"}, {Action: "next", Text: "Next", Primary: true}},
	}
}

func paintedCard(t *testing.T, c CardContent, look Look, dpr float64) (*image.NRGBA, Card) {
	t.Helper()
	s, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	k := LayoutCard(c, s)
	img := image.NewNRGBA(image.Rect(0, 0, int(math.Ceil(k.Size.X*dpr)), int(math.Ceil(k.Size.Y*dpr))))
	PaintCard(img, k, s, look, dpr, 1, "")
	return img, k
}

func TestTheCardWearsTheDesktopsCorners(t *testing.T) {
	s, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	const dpr = 4.0
	for _, d := range designs {
		img, k := paintedCard(t, guideCard(), perchLook(d), dpr)
		ch := s.Perch.Chip
		corner := float64(d.Corner(float32(ch.RadiusPx+ch.PadYPx), float32(k.Size.X), float32(k.Size.Y))) * dpr
		if corner < 8 {
			t.Fatalf("%s: corner %v device px is too small to tell its shape", d, corner)
		}
		// on the diagonal at 0.36 of the corner: outside a 45-degree cut,
		// inside a quarter circle
		at := int(0.36 * corner)
		inside := img.NRGBAAt(at, at).A > 0
		if d == design.Lithic && inside {
			t.Errorf("lithic: the card's corner is rounded, not cut")
		}
		if d != design.Lithic && !inside {
			t.Errorf("%s: the card's corner is cut, not rounded", d)
		}
		if a := img.NRGBAAt(int(0.1*corner), int(0.1*corner)).A; a != 0 {
			t.Errorf("%s: the very corner is painted (alpha %d)", d, a)
		}
		// the pills wear the same shape
		p := k.Pills[len(k.Pills)-1].Rect
		pc := float64(d.Corner(float32(ch.RadiusPx), float32(p.W), float32(p.H))) * dpr
		px, py := int(p.X*dpr+0.36*pc), int(p.Y*dpr+0.36*pc)
		filled := img.NRGBAAt(px, py) == perchLook(d).Ink
		if d == design.Lithic && filled {
			t.Errorf("lithic: the primary pill's corner is rounded, not cut")
		}
		if d != design.Lithic && !filled {
			t.Errorf("%s: the primary pill's corner is cut, not rounded", d)
		}
	}
}

// With every role one colour, every pixel the card paints is that colour:
// nothing on the card comes from anywhere but the look.
func TestTheCardDrawsOnlyTheLooksColours(t *testing.T) {
	c := color.NRGBA{R: 37, G: 151, B: 90, A: 255}
	for _, d := range designs {
		look := Look{Ink: c, White: c, Body: c, Design: d,
			Accent: map[string]color.NRGBA{"idle": c, "listening": c, "thinking": c, "speaking": c, "done": c, "error": c}}
		img, _ := paintedCard(t, guideCard(), look, 2)
		painted := 0
		for i := 0; i < len(img.Pix); i += 4 {
			px := img.Pix[i : i+4]
			if px[3] == 0 {
				continue
			}
			painted++
			for ch, want := range []uint8{c.R, c.G, c.B} {
				if diff := int(px[ch]) - int(want); diff < -1 || diff > 1 {
					t.Fatalf("%s: pixel %d is %v, a colour the look does not have", d, i/4, px)
				}
			}
		}
		if painted == 0 {
			t.Fatalf("%s: the card painted nothing", d)
		}
	}
}

func TestTheCardFollowsALightAndADarkLook(t *testing.T) {
	dark := Look{Ink: color.NRGBA{R: 14, G: 14, B: 18, A: 255}, White: color.NRGBA{R: 236, G: 232, B: 222, A: 255}, Body: color.NRGBA{R: 120, G: 118, B: 112, A: 255}, Dark: true, Design: design.Material,
		Accent: map[string]color.NRGBA{"listening": {R: 214, G: 160, B: 40, A: 255}, "done": {R: 60, G: 170, B: 120, A: 255}}}
	light := Look{Ink: dark.White, White: dark.Ink, Body: dark.Body, Design: design.Material, Accent: dark.Accent}
	for name, look := range map[string]Look{"dark": dark, "light": light} {
		img, k := paintedCard(t, guideCard(), look, 1)
		if got := img.NRGBAAt(int(k.Pad/2), int(k.Size.Y/2)); got != look.White {
			t.Errorf("%s: the card is %v, want its White %v", name, got, look.White)
		}
		next := k.Pills[len(k.Pills)-1]
		if !next.Primary || next.Action != "next" {
			t.Fatalf("%s: the last pill is %+v", name, next)
		}
		// the left padding of the primary pill, clear of its words
		if got := img.NRGBAAt(int(next.Rect.X+3), int(next.Rect.Y+next.Rect.H/2)); got != look.Ink {
			t.Errorf("%s: the primary pill is %v, want Ink %v", name, got, look.Ink)
		}
		stop := k.Pills[0]
		if got := img.NRGBAAt(int(stop.Rect.X+3), int(stop.Rect.Y+stop.Rect.H/2)); got != look.White {
			t.Errorf("%s: the second pill is %v, want White %v", name, got, look.White)
		}
		now := k.Rows[1]
		mark := img.NRGBAAt(int(k.Pad+k.RowH/2-k.RowH*0.15), int(now.Y+k.RowH/2))
		if mark != look.Accent["listening"] {
			t.Errorf("%s: the mark of the step now is %v, want the listening accent", name, mark)
		}
	}
}

func TestTheCardWrapsItsStepAndRightAlignsItsPills(t *testing.T) {
	s, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	c := guideCard()
	c.Text = strings.Repeat("Open the menu with three dots and choose the second entry. ", 6)
	k := LayoutCard(c, s)
	if k.Size.X > s.Strand.Bubble.MaxWidthPx {
		t.Fatalf("the card is %v wide, more than %v", k.Size.X, s.Strand.Bubble.MaxWidthPx)
	}
	if len(k.Lines) != cardMaxLines || !strings.HasSuffix(k.Lines[len(k.Lines)-1].Text, "…") {
		t.Fatalf("a long step wrapped to %d lines ending %q", len(k.Lines), k.Lines[len(k.Lines)-1].Text)
	}
	for i, l := range k.Lines {
		if l.W > k.Size.X-2*k.Pad+0.5 {
			t.Errorf("line %d is %v wide in a %v card", i, l.W, k.Size.X)
		}
		if i > 0 && k.LinesY[i] <= k.LinesY[i-1] {
			t.Errorf("line %d is not below line %d", i, i-1)
		}
	}
	last := k.Pills[len(k.Pills)-1].Rect
	if right := last.X + last.W; math.Abs(right-(k.Size.X-k.Pad)) > 0.5 {
		t.Errorf("the pills end at %v, want the right padding %v", right, k.Size.X-k.Pad)
	}
	if bottom := last.Y + last.H; bottom > k.Size.Y-k.Pad+0.5 || last.Y < k.Rows[len(k.Rows)-1].Y+k.RowH {
		t.Errorf("the pills at %v are not on the last row of a %v card", last, k.Size)
	}
	a, b := k.Pills[0].Rect, k.Pills[1].Rect
	if a.X+a.W > b.X {
		t.Errorf("the pills overlap: %v and %v", a, b)
	}
	if got := k.PillAt(spec.Point{X: b.X + b.W/2, Y: b.Y + b.H/2}); got != "next" {
		t.Errorf("the middle of Next hits %q", got)
	}
	if got := k.PillAt(spec.Point{X: a.X + a.W + 1, Y: a.Y + a.H/2}); got != "" {
		t.Errorf("the gap between the pills hits %q", got)
	}
	if got := k.PillAt(spec.Point{X: k.Pad, Y: k.LinesY[0]}); got != "" {
		t.Errorf("the step text hits %q", got)
	}
}

func TestAnEmptyCardLaysOutNothing(t *testing.T) {
	s, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	if k := LayoutCard(CardContent{Text: "   "}, s); !k.Empty() {
		t.Fatalf("an empty card is %v", k.Size)
	}
}
