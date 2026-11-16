package kit

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"reflect"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/design"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

func shapeContext(ops *op.Ops, scale float32) layout.Context {
	return layout.Context{Ops: ops, Metric: unit.Metric{PxPerDp: scale, PxPerSp: scale}}
}

// opBytes returns the encoded operations, so two painters can be compared
// byte for byte.
func opBytes(ops *op.Ops) []byte {
	return bytes.Clone(reflect.ValueOf(&ops.Internal).Elem().FieldByName("data").Bytes())
}

// In Lithic, the zero design, every style-aware painter must encode exactly
// what the cut primitive it replaces encodes.
func TestLithicPaintersEncodeTheCutPrimitives(t *testing.T) {
	th := theme.New(palette.Signature(true))
	if th.Design != design.Lithic {
		t.Fatalf("zero design = %v, want lithic", th.Design)
	}
	ink := color.NRGBA{R: 0xd0, G: 0x60, B: 0x30, A: 0xff}
	cases := []image.Rectangle{
		image.Rect(0, 0, 40, 22), image.Rect(3, 5, 120, 45), image.Rect(0, 0, 8, 20), image.Rect(0, 0, 0, 10),
	}
	for _, scale := range []float32{1, 1.75} {
		for _, bounds := range cases {
			for _, cut := range []int{0, 2, 7, 16} {
				var want, got op.Ops
				FillCut(&want, bounds, cut, ink)
				Fill(shapeContext(&got, scale), th, bounds, cut, ink)
				if !bytes.Equal(opBytes(&want), opBytes(&got)) {
					t.Errorf("Fill %v cut %d at %v differs from FillCut", bounds, cut, scale)
				}
				want.Reset()
				got.Reset()
				StrokeCut(&want, bounds, cut, 2, ink)
				Stroke(shapeContext(&got, scale), th, bounds, cut, 2, ink)
				if !bytes.Equal(opBytes(&want), opBytes(&got)) {
					t.Errorf("Stroke %v cut %d at %v differs from StrokeCut", bounds, cut, scale)
				}
				want.Reset()
				got.Reset()
				FillCutCorners(&want, bounds, cut, 0, cut, 0, ink)
				StrokeCutCorners(&want, bounds, 0, cut, 0, cut, 1, ink)
				gtx := shapeContext(&got, scale)
				FillCorners(gtx, th, bounds, cut, 0, cut, 0, ink)
				StrokeCorners(gtx, th, bounds, 0, cut, 0, cut, 1, ink)
				if !bytes.Equal(opBytes(&want), opBytes(&got)) {
					t.Errorf("FillCorners/StrokeCorners %v cut %d at %v differ from the cut primitives", bounds, cut, scale)
				}
				want.Reset()
				got.Reset()
				paint.FillShape(&want, ink, clip.Outline{Path: CutPath(&want, bounds, cut, cut, 0, 0)}.Op())
				paint.FillShape(&got, ink, clip.Outline{Path: Shape(shapeContext(&got, scale), th, bounds, cut, cut, 0, 0)}.Op())
				if !bytes.Equal(opBytes(&want), opBytes(&got)) {
					t.Errorf("Shape %v cut %d at %v differs from CutPath", bounds, cut, scale)
				}
			}
		}
	}
}

// The radii the round designs give the library's controls at scale 1, from
// Lithic.cornerRadius (Lithic.qml:61-69) and the profiles (Lithic.qml:48-55):
// a shape whose shorter side is within the design's pill size is a pill,
// otherwise the cut times the scale up to the design's maximum.
func TestRoundDesignRadii(t *testing.T) {
	cases := []struct {
		name                      string
		w, h, cut                 int
		material, adwaita, fluent float32
	}{
		{"toggle track", 40, 22, 4, 11, 11, 11},
		{"toggle knob", 14, 14, 2, 7, 7, 7},
		{"plaque", 120, 40, 7, 20, 11.2, 5.25},
		{"small plaque", 90, 32, 6, 16, 9.6, 4.5},
		{"segmented track", 300, 36, 7, 18, 11.2, 5.25},
		{"segment", 90, 28, 5, 14, 8, 3.75},
		{"text field", 320, 44, 7, 22, 11.2, 5.25},
		{"slider handle", 8, 20, 4, 4, 4, 4},
		{"scroll thumb", 3, 80, 1, 1.5, 1.5, 1.5},
		{"slab", 320, 120, 7, 18.2, 11.2, 5.25},
		{"menu", 240, 160, 8, 20.8, 12.8, 6},
		{"dialog", 360, 240, 16, 28, 14, 8},
		{"toggle focus ring", 48, 30, 6, 15, 9.6, 4.5},
	}
	for _, c := range cases {
		for style, want := range map[design.Style]float32{design.Material: c.material, design.Adwaita: c.adwaita, design.Fluent: c.fluent} {
			th := theme.New(palette.Signature(true))
			th.Design = style
			var ops op.Ops
			radii, round := cornerRadii(shapeContext(&ops, 1), th, image.Rect(0, 0, c.w, c.h), c.cut, c.cut, 0, c.cut)
			if !round {
				t.Fatalf("%v: not round", style)
			}
			for index, got := range radii {
				expected := want
				if index == 2 {
					expected = 0 // a cut of 0 stays square in every design
				}
				if math.Abs(float64(got-expected)) > 1e-4 {
					t.Errorf("%s in %v corner %d: radius %v, want %v", c.name, style, index, got, expected)
				}
			}
		}
	}
}

// At a fractional scale the thresholds stay in dp, and a radius never passes
// half the shorter side, so an odd-pixel pill still closes. A control laid out
// at exactly the pill size stays a pill although gtx.Dp rounded it up.
func TestRoundRadiiAtScale(t *testing.T) {
	th := theme.New(palette.Signature(true))
	var ops op.Ops
	gtx := shapeContext(&ops, 1.75)
	cases := []struct {
		style  design.Style
		bounds image.Rectangle
		cut    int
		want   float32
	}{
		// the dialog's cut 16 dp is 28 px; Material's maximum 28 dp is 49 px
		{design.Material, image.Rect(0, 0, 630, 420), gtx.Dp(16), 49},
		// a 23 px tall pill rounds by 11.5, not by a rounded-up 12
		{design.Material, image.Rect(0, 0, 70, 23), gtx.Dp(4), 11.5},
		// a plaque 40 dp tall is 70 px; Adwaita: 12 px cut is 6.857 dp, times 1.6
		{design.Adwaita, image.Rect(0, 0, 210, 70), gtx.Dp(7), 12 * 1.6},
		// Fluent's 22 dp toggle track is gtx.Dp(22) = 39 px (22.3 dp): still a pill
		{design.Fluent, image.Rect(0, 0, gtx.Dp(40), gtx.Dp(22)), gtx.Dp(4), 19.5},
		// one pixel taller is past the pill: the 7 px cut (4 dp) times 0.75
		{design.Fluent, image.Rect(0, 0, 70, gtx.Dp(22)+1), gtx.Dp(4), 7 * 0.75},
	}
	for _, c := range cases {
		th.Design = c.style
		radii, _ := cornerRadii(gtx, th, c.bounds, c.cut, c.cut, c.cut, c.cut)
		if math.Abs(float64(radii[0]-c.want)) > 1e-3 {
			t.Errorf("%v %v cut %d: radius %v, want %v", c.style, c.bounds, c.cut, radii[0], c.want)
		}
		if got := th.Corner(gtx, c.cut, c.bounds); math.Abs(float64(got)-float64(c.want)) > 0.5 {
			t.Errorf("%v %v: theme.Corner %d, kit %v", c.style, c.bounds, got, radii[0])
		}
	}
	th.Design = design.Lithic
	if _, round := cornerRadii(gtx, th, image.Rect(0, 0, 40, 22), 4, 4, 4, 4); round {
		t.Error("lithic rounds")
	}
}

// A round painter encodes a curved outline, not the cut polygon, and draws
// nothing for an empty box or a clear ink.
func TestRoundPaintersDrawCurves(t *testing.T) {
	th := theme.New(palette.Signature(true))
	th.Design = design.Adwaita
	ink := color.NRGBA{A: 0xff}
	bounds := image.Rect(0, 0, 120, 40)
	var cut, round op.Ops
	FillCut(&cut, bounds, 7, ink)
	Fill(shapeContext(&round, 1), th, bounds, 7, ink)
	if bytes.Equal(opBytes(&cut), opBytes(&round)) {
		t.Fatal("Adwaita Fill encodes the cut polygon")
	}
	for _, paint := range []func(layout.Context){
		func(gtx layout.Context) { Fill(gtx, th, image.Rectangle{}, 7, ink) },
		func(gtx layout.Context) { Fill(gtx, th, bounds, 7, color.NRGBA{}) },
		func(gtx layout.Context) { Stroke(gtx, th, bounds, 7, 0, ink) },
		func(gtx layout.Context) { Stroke(gtx, th, image.Rectangle{}, 7, 1, ink) },
	} {
		var ops op.Ops
		paint(shapeContext(&ops, 1))
		if len(opBytes(&ops)) != 0 {
			t.Error("an empty paint encoded operations")
		}
	}
}

// The focus ring stands a gap outside the control (FocusRing.qml: outset is
// gap + ring width) and grows the cut by focusCutGrow.
func TestFocusRingBox(t *testing.T) {
	for _, c := range []struct {
		scale         float32
		outset, width int
	}{{1, 4, 2}, {1.75, 8, 4}} {
		var ops op.Ops
		ring, width := focusRingBox(shapeContext(&ops, c.scale), image.Rect(10, 10, 50, 32))
		if want := image.Rect(10-c.outset, 10-c.outset, 50+c.outset, 32+c.outset); ring != want || width != c.width {
			t.Errorf("scale %v: ring %v width %d, want %v width %d", c.scale, ring, width, want, c.width)
		}
	}
	th := theme.New(palette.Signature(true))
	ink := palette.WithAlpha(th.R.Fg2, 1)
	var want, got op.Ops
	StrokeCut(&want, image.Rect(6, 6, 54, 36), 6, 2, ink)
	FocusRing(shapeContext(&got, 1), th, image.Rect(10, 10, 50, 32), 4)
	if !bytes.Equal(opBytes(&want), opBytes(&got)) {
		t.Error("lithic focus ring is not the cut ring 4 px out with the cut grown by 2")
	}
}
