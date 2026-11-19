package kit

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/design"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

var roundDesigns = []design.Style{design.Material, design.Adwaita, design.Fluent}

// designTheme shares partsTh's shaper; building one loads every face.
func designTheme(style design.Style) *theme.Theme {
	th := theme.New(palette.Signature(true), theme.WithShaper(partsTh().Shaper))
	th.Design = style
	return th
}

func encoded(paint func(layout.Context)) []byte {
	var ops op.Ops
	paint(shapeContext(&ops, 1))
	return opBytes(&ops)
}

// A plain slab is the design's card: Lithic's is the paper drawn by its vein
// hairline alone, exactly as before; a round design lays its card tone, with
// an outline only where the design draws one.
func TestSlabSurfaceIsTheDesignsCard(t *testing.T) {
	bounds := image.Rect(0, 0, 320, 120)
	lithic := designTheme(design.Lithic)
	want := encoded(func(gtx layout.Context) { StrokeCut(gtx.Ops, bounds, 7, 1, lithic.R.Vein) })
	if got := encoded(func(gtx layout.Context) { SlabSurface(gtx, lithic, bounds, 7, lithic.R.Slab) }); !bytes.Equal(got, want) {
		t.Error("a plain Lithic slab is not its vein hairline alone")
	}
	for _, style := range roundDesigns {
		th := designTheme(style)
		planes := th.Planes()
		want := encoded(func(gtx layout.Context) {
			Fill(gtx, th, bounds, 7, planes.Card)
			Stroke(gtx, th, bounds, 7, 1, planes.CardStroke)
		})
		got := encoded(func(gtx layout.Context) { SlabSurface(gtx, th, bounds, 7, th.R.Slab) })
		if !bytes.Equal(got, want) {
			t.Errorf("%v: a plain slab is not the card plane and its stroke", style)
		}
		if planes.Card == th.R.Slab {
			t.Errorf("%v: the card is the paper, so a slab would not show", style)
		}
		outline := encoded(func(gtx layout.Context) { SlabOutline(gtx, th, bounds, 7) })
		if want := encoded(func(gtx layout.Context) { Stroke(gtx, th, bounds, 7, 1, th.R.Vein) }); !bytes.Equal(outline, want) {
			t.Errorf("%v: an outlined slab is not its vein hairline alone", style)
		}
		field := th.R.MineralField(palette.Attention)
		toned := encoded(func(gtx layout.Context) { SlabSurface(gtx, th, bounds, 7, field) })
		if want := encoded(func(gtx layout.Context) { Fill(gtx, th, bounds, 7, field) }); !bytes.Equal(toned, want) {
			t.Errorf("%v: a toned slab is not its field alone", style)
		}
	}
}

// The work pane keeps Lithic's recipe; the page pane cuts its foot by the mid
// cut; every kind takes the design's page plane and stroke when it is round.
func TestPaneTakesTheDesignsPlanes(t *testing.T) {
	size := image.Pt(400, 300)
	bounds := image.Rectangle{Max: size}
	body := func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: size} }
	lay := func(th *theme.Theme, kind PaneKind) []byte {
		return encoded(func(gtx layout.Context) { Pane{Kind: kind}.Layout(gtx, th, body) })
	}
	recipe := func(th *theme.Theme, foot int, fill, line color.NRGBA) []byte {
		return encoded(func(gtx layout.Context) {
			// the pane records its body, paints, then replays the body
			call := op.Record(gtx.Ops).Stop()
			FillCorners(gtx, th, bounds, 16, 16, foot, foot, fill)
			StrokeCorners(gtx, th, bounds, 16, 16, foot, foot, 1, line)
			call.Add(gtx.Ops)
		})
	}
	lithic := designTheme(design.Lithic)
	for _, c := range []struct {
		name string
		kind PaneKind
		foot int
		fill color.NRGBA
	}{
		{"list", PaneList, 0, lithic.R.Stone1},
		{"work", PaneWork, 0, lithic.R.Stone2},
		{"page", PanePage, 7, lithic.R.Stone2},
	} {
		if !bytes.Equal(lay(lithic, c.kind), recipe(lithic, c.foot, c.fill, lithic.R.Vein)) {
			t.Errorf("lithic %s pane: not cut 16 at the top and %d at the foot with a vein hairline", c.name, c.foot)
		}
	}
	for _, style := range roundDesigns {
		th := designTheme(style)
		planes := th.Planes()
		for _, kind := range []PaneKind{PaneList, PaneWork} {
			if !bytes.Equal(lay(th, kind), recipe(th, 0, planes.Pane, planes.PaneStroke)) {
				t.Errorf("%v pane %d: not the design's page plane and stroke", style, kind)
			}
		}
		if !bytes.Equal(lay(th, PanePage), recipe(th, 7, planes.Pane, planes.PaneStroke)) {
			t.Errorf("%v page pane: not the design's page plane with a mid foot", style)
		}
	}
}

// The tone mark is Lithic's notch, scaled down on a short plane, and a dot in
// a round design that stays inside the plane's corner; idle marks nothing.
func TestToneMark(t *testing.T) {
	lithic := designTheme(design.Lithic)
	for _, c := range []struct {
		bounds image.Rectangle
		notch  int
	}{
		{image.Rect(0, 0, 320, 56), 12},
		{image.Rect(10, 20, 330, 60), 10},
		{image.Rect(0, 0, 200, 36), 9},
		{image.Rect(0, 0, 24, 24), 6},
	} {
		want := encoded(func(gtx layout.Context) { Notch(gtx.Ops, c.bounds.Min, c.notch, lithic.R.Amber) })
		got := encoded(func(gtx layout.Context) { ToneMark(gtx, lithic, c.bounds, palette.Attention) })
		if !bytes.Equal(got, want) {
			t.Errorf("lithic mark on %v is not a %d notch at its corner", c.bounds, c.notch)
		}
	}
	if got := encoded(func(gtx layout.Context) { ToneMark(gtx, lithic, image.Rect(0, 0, 320, 56), palette.Idle) }); len(got) != 0 {
		t.Error("an idle plane wears a mark")
	}
	bounds := image.Rect(0, 0, 320, 56)
	notch := encoded(func(gtx layout.Context) { Notch(gtx.Ops, bounds.Min, 12, lithic.R.Amber) })
	for _, style := range roundDesigns {
		th := designTheme(style)
		got := encoded(func(gtx layout.Context) { ToneMark(gtx, th, bounds, palette.Attention) })
		if len(got) == 0 || bytes.Equal(got, notch) {
			t.Errorf("%v: the mark is the angular notch, or nothing", style)
		}
		if got := encoded(func(gtx layout.Context) { ToneMark(gtx, th, bounds, palette.Idle) }); len(got) != 0 {
			t.Errorf("%v: an idle plane wears a mark", style)
		}
		if got := encoded(func(gtx layout.Context) { toneMark(gtx, th, bounds, 12, th.R.Amber, 0) }); len(got) != 0 {
			t.Errorf("%v: a mark not yet grown is drawn", style)
		}
	}
}

// A meter's bar is Lithic's blade as it was, and a round-ended bar on a
// round-ended track in the round designs.
func TestBladesFollowTheDesign(t *testing.T) {
	bar := image.Rect(0, 0, 120, 6)
	ink := color.NRGBA{R: 0x40, G: 0x80, B: 0xc0, A: 0xff}
	lithic := designTheme(design.Lithic)
	blade := encoded(func(gtx layout.Context) { Blade(gtx.Ops, bar, ink) })
	if got := encoded(func(gtx layout.Context) { FillBlade(gtx, lithic, bar, ink) }); !bytes.Equal(got, blade) {
		t.Error("lithic FillBlade is not Blade")
	}
	if got := barCut(lithic, bar); got != 0 {
		t.Errorf("lithic bar cut %d, want a square track", got)
	}
	gtx := func(ops *op.Ops) layout.Context {
		gtx := shapeContext(ops, 1)
		gtx.Constraints = layout.Exact(image.Pt(200, 6))
		return gtx
	}
	var was op.Ops
	FillCut(&was, image.Rect(0, 0, 200, 6), 0, lithic.R.Field)
	Blade(&was, image.Rect(0, 0, 124, 6), lithic.R.Fg2)
	var now op.Ops
	BladeMeter{Fraction: .62}.Layout(gtx(&now), lithic)
	if !bytes.Equal(opBytes(&was), opBytes(&now)) {
		t.Error("the Lithic meter is not a square field track under a blade")
	}
	for _, style := range roundDesigns {
		th := designTheme(style)
		if got := encoded(func(gtx layout.Context) { FillBlade(gtx, th, bar, ink) }); bytes.Equal(got, blade) || len(got) == 0 {
			t.Errorf("%v: the bar keeps the 45 degree edge", style)
		}
		if got := barCut(th, bar); got != 3 {
			t.Errorf("%v: bar cut %d, want half the track", style, got)
		}
		radii, _ := cornerRadii(shapeContext(new(op.Ops), 1), th, bar, 3, 3, 3, 3)
		if radii != [4]float32{3, 3, 3, 3} {
			t.Errorf("%v: a 6 tall track rounds by %v, want round ends", style, radii)
		}
		var ops op.Ops
		BladeMeter{Fraction: .62}.Layout(gtx(&ops), th)
		if bytes.Equal(opBytes(&ops), opBytes(&was)) {
			t.Errorf("%v: the meter is drawn as Lithic's", style)
		}
	}
}

// A chosen row wears the Hub's selection mark: the wash and the ink on it,
// and an opaque container from the desktop's palette with inks of its own.
func TestSelectionInks(t *testing.T) {
	for _, dark := range []bool{true, false} {
		th := theme.New(palette.Signature(dark), theme.WithShaper(partsTh().Shaper))
		wash, ink, quiet := selectionInks(th)
		if wash != th.R.Select() || wash == th.R.Sel || wash.A == 255 {
			t.Errorf("dark=%v: the wash %v is not the faint selection wash", dark, wash)
		}
		if ink != th.R.Fg || quiet != th.R.Fg3 {
			t.Errorf("dark=%v: inks %v %v, want fg and fg3", dark, ink, quiet)
		}
	}
	th := theme.New(palette.Signature(true), theme.WithShaper(partsTh().Shaper))
	th.R.Container = color.NRGBA{R: 0x4a, G: 0x45, B: 0x58, A: 0xff}
	th.R.OnContainer = color.NRGBA{R: 0xe8, G: 0xde, B: 0xf8, A: 0xff}
	wash, ink, quiet := selectionInks(th)
	if wash != th.R.Container || ink != th.R.OnContainer {
		t.Errorf("an opaque container: wash %v ink %v", wash, ink)
	}
	if quiet == th.R.Fg3 || quiet == ink {
		t.Errorf("the quiet ink %v is not mixed from the container's own", quiet)
	}
}

// Keycaps set faces 4 apart on 24 tall plates and leave out what does not fit.
func TestKeycaps(t *testing.T) {
	th := partsTh()
	var ops op.Ops
	wide := shapeContext(&ops, 1)
	wide.Constraints = layout.Constraints{Max: image.Pt(1000, 100)}
	super := Keycap(th, "Super")(wide)
	shift := Keycap(th, "Shift")(wide)
	enter := Keycap(th, "Enter")(wide)
	if super.Size.Y != 24 {
		t.Errorf("a keycap is %d tall, want 24", super.Size.Y)
	}
	all := Keycaps(th, "Super", "Shift", "Enter")(wide)
	if want := super.Size.X + shift.Size.X + enter.Size.X + 2*4; all.Size.X != want || all.Size.Y != 24 {
		t.Errorf("three keycaps measure %v, want %d wide and 24 tall", all.Size, want)
	}
	narrow := wide
	narrow.Constraints.Max.X = super.Size.X + 4 + shift.Size.X + 4 + enter.Size.X - 1
	if got := Keycaps(th, "Super", "Shift", "Enter")(narrow); got.Size.X != super.Size.X+4+shift.Size.X {
		t.Errorf("a line one pixel short is %d wide, want the first two caps (%d)", got.Size.X, super.Size.X+4+shift.Size.X)
	}
	if got := Keycaps(th)(wide); got.Size != (image.Point{}) {
		t.Errorf("no faces measure %v", got.Size)
	}
}
