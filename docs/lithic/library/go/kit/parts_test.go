package kit

import (
	"image"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
	"weak"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"golang.org/x/image/math/fixed"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

var (
	partsThemeOnce sync.Once
	partsTheme     *theme.Theme
)

// partsTh shares one shaper across the parts' tests; building one loads every face.
func partsTh() *theme.Theme {
	partsThemeOnce.Do(func() { partsTheme = theme.New(palette.Signature(true)) })
	return partsTheme
}

func partsFrame(router *input.Router, ops *op.Ops, now time.Time, size image.Point) layout.Context {
	ops.Reset()
	return layout.Context{
		Ops: ops, Now: now, Source: router.Source(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: size},
	}
}

// shapedAdvances shapes value plainly and returns the sum of its glyphs' advances and its
// clusters, the measure SpacedCaps adds its tracking to.
func shapedAdvances(th *theme.Theme, style theme.TextStyle, value string) (fixed.Int26_6, int) {
	th.Shaper.LayoutString(text.Parameters{Font: style.Face, PxPerEm: fixed.I(int(style.Size)), MaxLines: 1, MaxWidth: 1 << 24}, value)
	var sum fixed.Int26_6
	clusters := 0
	for {
		glyph, ok := th.Shaper.NextGlyph()
		if !ok {
			return sum, clusters
		}
		sum += glyph.Advance
		if glyph.Flags&text.FlagClusterBreak != 0 {
			clusters++
		}
	}
}

// TestSpacedCapsWidth: a spaced line is as wide as its capitals' advances plus the tracking
// after every cluster, as Qt's letterSpacing adds it.
func TestSpacedCapsWidth(t *testing.T) {
	th := partsTh()
	var router input.Router
	var ops op.Ops
	gtx := partsFrame(&router, &ops, time.Unix(1000, 0), image.Pt(4000, 100))
	for _, test := range []struct {
		style    theme.TextStyle
		tracking float32
		value    string
	}{
		{th.T.MonoSm, 1.4, "Appearance  /  Window style"},
		{th.T.Group, 1.4, "Appearance"},
		{th.T.MonoSm, 1.2, "P 02/39"},
		{th.T.Caption, 1, "外観・窓飾"},
		{th.T.Group, 0, "Settings"},
	} {
		advances, clusters := shapedAdvances(th, test.style, strings.ToUpper(test.value))
		if clusters != len([]rune(test.value)) {
			t.Fatalf("%q: %d clusters for %d runes", test.value, clusters, len([]rune(test.value)))
		}
		tracking := fixed.Int26_6(test.tracking*64 + 0.5)
		want := (advances + fixed.Int26_6(clusters)*tracking).Ceil()
		got := LayoutSpacedCaps(gtx, th, test.style, th.R.Fg, test.tracking, test.value)
		if got.Size.X != want {
			t.Errorf("%q at %v: width %d, want %d (advances %v + %d x %v)", test.value, test.tracking, got.Size.X, want, advances, clusters, tracking)
		}
		// the line height, or the glyphs' own box when a fallback face (CJK) is taller
		if got.Size.Y < gtx.Sp(test.style.Line) {
			t.Errorf("%q: height %d, under the line %d", test.value, got.Size.Y, gtx.Sp(test.style.Line))
		}
	}
	// the capitals come from the text, so a lower-case line spaces as its upper case
	lower := LayoutSpacedCaps(gtx, th, th.T.Group, th.R.Fg, 1.4, "window style")
	upper := LayoutSpacedCaps(gtx, th, th.T.Group, th.R.Fg, 1.4, "WINDOW STYLE")
	if lower.Size != upper.Size {
		t.Errorf("lower case %v, upper case %v", lower.Size, upper.Size)
	}
}

// TestSpacedCapsAtRest: a cached line allocates nothing and asks for no frame.
func TestSpacedCapsAtRest(t *testing.T) {
	th := partsTh()
	var router input.Router
	var ops op.Ops
	gtx := partsFrame(&router, &ops, time.Unix(1000, 0), image.Pt(800, 100))
	LayoutSpacedCaps(gtx, th, th.T.MonoSm, th.R.Fg2, 1.4, "Now playing  /  Lyrics")
	if allocs := testing.AllocsPerRun(100, func() {
		ops.Reset()
		LayoutSpacedCaps(gtx, th, th.T.MonoSm, th.R.Fg2, 1.4, "Now playing  /  Lyrics")
	}); allocs != 0 {
		t.Errorf("a cached spaced line allocates %v per frame", allocs)
	}
	if _, wake := router.WakeupTime(); wake {
		t.Error("a spaced line asked for a frame")
	}
}

func TestTrackedLabelsReleaseRetiredShapers(t *testing.T) {
	retired := func() weak.Pointer[text.Shaper] {
		th := theme.New(palette.Signature(true))
		var router input.Router
		var ops op.Ops
		gtx := partsFrame(&router, &ops, time.Unix(1000, 0), image.Pt(800, 100))
		LayoutTracked(gtx, th, th.T.Group, th.R.Fg, 1.4, "Closed native window")
		return weak.Make(th.Shaper)
	}()
	runtime.GC()
	runtime.GC()
	if retired.Value() != nil {
		t.Fatal("cached labels retain a retired theme's font shaper")
	}
}

func TestPad2AndOrdinal(t *testing.T) {
	for _, test := range []struct {
		n    int
		want string
	}{{0, "00"}, {3, "03"}, {9, "09"}, {10, "10"}, {39, "39"}, {99, "99"}, {120, "120"}} {
		if got := Pad2(test.n); got != test.want {
			t.Errorf("Pad2(%d) = %q, want %q", test.n, got, test.want)
		}
	}
	if got := Ordinal(1, 8); got != "01 / 08" {
		t.Errorf("Ordinal(1, 8) = %q", got)
	}
	if got := Ordinal(12, 112); got != "12 / 112" {
		t.Errorf("Ordinal(12, 112) = %q", got)
	}
	if allocs := testing.AllocsPerRun(100, func() { _ = Pad2(7) }); allocs != 0 {
		t.Errorf("Pad2 allocates %v", allocs)
	}
}

// TestAmbientAndClock: the clock shows seconds and ticks only while the window is focused and
// motion is allowed (Ambient.qml, Masthead.qml:31-35).
func TestAmbientAndClock(t *testing.T) {
	var router input.Router
	var ops op.Ops
	now := time.Date(2026, 11, 16, 12, 40, 7, 300e6, time.Local)
	gtx := partsFrame(&router, &ops, now, image.Pt(100, 100))
	defer SetFocused(false)

	SetFocused(false)
	var clock Clock
	if Ambient(gtx) {
		t.Fatal("ambient while unfocused")
	}
	if got := clock.Text(gtx); got != "12:40" {
		t.Errorf("unfocused clock %q", got)
	}
	if _, wake := router.WakeupTime(); wake {
		t.Error("an unfocused clock asked for a frame")
	}
	if allocs := testing.AllocsPerRun(100, func() { clock.Text(gtx) }); allocs != 0 {
		t.Errorf("a resting clock allocates %v", allocs)
	}

	SetFocused(true)
	if !Ambient(gtx) {
		t.Fatal("not ambient while focused")
	}
	if got := clock.Text(gtx); got != "12:40:07" {
		t.Errorf("focused clock %q", got)
	}
	at, wake := router.WakeupTime()
	if want := time.Date(2026, 11, 16, 12, 40, 8, 0, time.Local); !wake || !at.Equal(want) {
		t.Errorf("ticked for %v (%v), want the next second %v", at, wake, want)
	}

	anim.SetReduce(true)
	reduced := Ambient(gtx)
	anim.SetReduce(false)
	if reduced {
		t.Error("ambient under reduced motion")
	}
}

// TestPartsAtRest: once drawn, the masthead, its ruler, the registration sheet and a ruled
// section head allocate nothing and ask for no frame.
func TestPartsAtRest(t *testing.T) {
	th := partsTh()
	var router input.Router
	var ops op.Ops
	gtx := partsFrame(&router, &ops, time.Unix(1000, 0), image.Pt(1000, 800))
	masthead := Masthead{Number: "03", Where: "Now playing  /  Lyrics", Gloss: "再生・歌詞", Facts: []string{"P 02/05", "NIRI"}}
	var motto TickRuler
	motto.Style = RulerMotto
	section := SectionStyle{Title: "Corners", Ruled: true, Mark: "01 / 08"}
	draw := func() {
		ops.Reset()
		RegistrationSheet(gtx, th)
		masthead.Layout(gtx, th, 0.4)
		motto.Layout(gtx, th, -1)
		section.Layout(gtx, th, nil)
	}
	draw()
	router.WakeupTime()
	start := gtx.Now
	gtx.Now = start.Add(time.Second)
	draw()
	if _, wake := router.WakeupTime(); wake {
		t.Error("the parts asked for a frame at rest")
	}
	if allocs := testing.AllocsPerRun(50, draw); allocs != 0 {
		t.Errorf("the parts allocate %v per frame at rest", allocs)
	}
}

// TestTickRulerMarker: the marker eases to a new place over 120 ms, asking for frames only
// while it moves.
func TestTickRulerMarker(t *testing.T) {
	th := partsTh()
	var router input.Router
	var ops op.Ops
	start := time.Unix(1000, 0)
	var ruler TickRuler
	ruler.Layout(partsFrame(&router, &ops, start, image.Pt(400, 10)), th, 0)
	ruler.Layout(partsFrame(&router, &ops, start, image.Pt(400, 10)), th, 1)
	if _, wake := router.WakeupTime(); !wake {
		t.Error("a moving marker asked for no frame")
	}
	ruler.Layout(partsFrame(&router, &ops, start.Add(th.Motion.Quick), image.Pt(400, 10)), th, 1)
	if _, wake := router.WakeupTime(); wake {
		t.Error("a settled marker asked for a frame")
	}
	if got := ruler.marker.Value(partsFrame(&router, &ops, start.Add(time.Hour), image.Pt(400, 10))); got != 1 {
		t.Errorf("marker settled at %v", got)
	}
}

// TestFolioRoll: a new number rolls in over 480 ms and then rests; the first number and any
// number under reduced motion stand at once.
func TestFolioRoll(t *testing.T) {
	th := partsTh()
	var router input.Router
	var ops op.Ops
	start := time.Unix(1000, 0)
	size := image.Pt(800, 300)
	var folio Folio
	folio.Layout(partsFrame(&router, &ops, start, size), th, "01.2", "")
	if _, wake := router.WakeupTime(); wake {
		t.Error("the first number rolled")
	}
	folio.Layout(partsFrame(&router, &ops, start, size), th, "01.3", "窓飾")
	if _, wake := router.WakeupTime(); !wake || folio.leaving != "01.2" {
		t.Error("a new number did not roll")
	}
	folio.Layout(partsFrame(&router, &ops, start.Add(479*time.Millisecond), size), th, "01.3", "窓飾")
	if _, wake := router.WakeupTime(); !wake {
		t.Error("the roll stopped early")
	}
	folio.Layout(partsFrame(&router, &ops, start.Add(480*time.Millisecond), size), th, "01.3", "窓飾")
	if _, wake := router.WakeupTime(); wake || folio.leaving != "" {
		t.Error("the roll did not end at 480 ms")
	}
	gtx := partsFrame(&router, &ops, start.Add(time.Second), size)
	if allocs := testing.AllocsPerRun(50, func() {
		ops.Reset()
		folio.Layout(gtx, th, "01.3", "窓飾")
	}); allocs != 0 {
		t.Errorf("a resting folio allocates %v per frame", allocs)
	}
	anim.SetReduce(true)
	folio.Layout(partsFrame(&router, &ops, start.Add(2*time.Second), size), th, "02", "")
	anim.SetReduce(false)
	if _, wake := router.WakeupTime(); wake || folio.leaving != "" {
		t.Error("a number rolled under reduced motion")
	}
}
