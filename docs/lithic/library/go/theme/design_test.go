package theme

import (
	"image"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/design"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
)

func testTheme(style design.Style) *Theme {
	t := New(palette.Signature(true), WithShaper(text.NewShaper()))
	t.Design = style
	return t
}

func TestNewDrawsLithic(t *testing.T) {
	if testTheme(design.Lithic).Design != design.Lithic || New(palette.Signature(true), WithShaper(text.NewShaper())).Design != design.Lithic {
		t.Fatal("a theme from New does not draw in Lithic")
	}
}

func TestCornerConvertsThroughTheMetric(t *testing.T) {
	for _, test := range []struct {
		style  design.Style
		scale  float32
		cut    int
		bounds image.Rectangle
		want   int
	}{
		// Lithic returns the cut at any scale
		{design.Lithic, 1, 7, image.Rect(0, 0, 120, 40), 7},
		{design.Lithic, 1.75, 12, image.Rect(0, 0, 210, 70), 12},
		{design.Material, 1, 0, image.Rect(0, 0, 120, 40), 0},
		// plaque, 40 dp tall, cut 7 dp: pill 20 / 11.2 / 5.25 dp
		{design.Material, 1, 7, image.Rect(0, 0, 120, 40), 20},
		{design.Adwaita, 1, 7, image.Rect(0, 0, 120, 40), 11},
		{design.Fluent, 1, 7, image.Rect(0, 0, 120, 40), 5},
		{design.Material, 1.75, 12, image.Rect(0, 0, 210, 70), 35},
		{design.Adwaita, 1.75, 12, image.Rect(0, 0, 210, 70), 19},
		{design.Fluent, 1.75, 12, image.Rect(0, 0, 210, 70), 9},
		// a shape sized at the pill threshold through gtx.Dp stays a pill at a fractional
		// scale: Fluent's 22 dp toggle track is Dp(22) = 39 px at 1.75 (22.29 dp)
		{design.Fluent, 1.75, 7, image.Rect(0, 0, 70, 39), 20},
		{design.Adwaita, 1.75, 7, image.Rect(0, 0, 175, 46), 23},
		// one pixel past Dp(pill) rounds by the cut again
		{design.Fluent, 1.75, 7, image.Rect(0, 0, 70, 40), 5},
		{design.Adwaita, 1.75, 7, image.Rect(0, 0, 175, 47), 11},
		// dialog, cut 16 dp at 2x: the caps are dp too (28 / 14 / 8 dp)
		{design.Material, 2, 32, image.Rect(0, 0, 800, 600), 56},
		{design.Adwaita, 2, 32, image.Rect(0, 0, 800, 600), 28},
		{design.Fluent, 2, 32, image.Rect(0, 0, 800, 600), 16},
	} {
		gtx := layout.Context{Metric: unit.Metric{PxPerDp: test.scale, PxPerSp: test.scale}}
		if got := testTheme(test.style).Corner(gtx, test.cut, test.bounds); got != test.want {
			t.Errorf("%s at %.2f, cut %d on %v: corner = %d, want %d", test.style, test.scale, test.cut, test.bounds.Size(), got, test.want)
		}
	}
}

func TestPlanesFollowTheDesign(t *testing.T) {
	for _, style := range []design.Style{design.Lithic, design.Material, design.Adwaita, design.Fluent} {
		th := testTheme(style)
		if th.Planes() != th.R.Planes(style) {
			t.Errorf("%s planes differ from the roles'", style)
		}
	}
}

func TestNewForCarriesTheDesktop(t *testing.T) {
	defer anim.SetReduce(anim.Reduced())
	roles := palette.Signature(false)
	d := palette.Desktop{Design: design.Fluent, ReduceMotion: true, Roles: roles}
	th := NewFor(d, WithShaper(text.NewShaper()))
	if th.Design != design.Fluent || th.R != roles || !anim.Reduced() {
		t.Fatalf("NewFor = design %v, reduced %v", th.Design, anim.Reduced())
	}
	d.ReduceMotion = false
	NewFor(d, WithShaper(text.NewShaper()))
	if anim.Reduced() {
		t.Fatal("NewFor left motion reduced")
	}
}

func TestMotionCarriesTheHubDurations(t *testing.T) {
	m := testTheme(design.Lithic).Motion
	for name, pair := range map[string][2]time.Duration{
		"Quick":         {m.Quick, 120 * time.Millisecond},
		"Move":          {m.Move, 240 * time.Millisecond},
		"HubUnfold":     {m.HubUnfold, 260 * time.Millisecond},
		"Stagger":       {m.Stagger, 18 * time.Millisecond},
		"RevealStagger": {m.RevealStagger, 40 * time.Millisecond},
		"Highlight":     {m.Highlight, 1600 * time.Millisecond},
		"CountUp":       {m.CountUp, 640 * time.Millisecond},
		// existing names keep Lithic's values
		"Unfold":   {m.Unfold, 200 * time.Millisecond},
		"Navigate": {m.Navigate, 200 * time.Millisecond},
	} {
		if pair[0] != pair[1] {
			t.Errorf("%s = %v, want %v", name, pair[0], pair[1])
		}
	}
}
