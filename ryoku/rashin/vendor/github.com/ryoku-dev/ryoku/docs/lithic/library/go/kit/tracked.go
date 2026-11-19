package kit

import (
	"image"
	"image/color"
	"strings"
	"sync"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"golang.org/x/image/math/fixed"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

// SpacedCaps returns one line of spaced capitals, as the Hub sets its running heads, folder
// names and spec labels (docs/lithic/design.md "Spaced capitals"): the text is written in
// sentence case and capitalised here, so a translation stays whole, and tracking (Qt's
// font.letterSpacing, in sp: the Hub uses 0.6 to 2) is added after every cluster.
//
// Gio's text package has no letter spacing, so the line is shaped once with th.Shaper,
// keeping the face's kerning, and each cluster then moves right by the tracking. The shaped
// outline is recorded once per (text, face, size, tracking) and replayed, so a line at rest
// costs a cache lookup and allocates nothing. ink is painted as given; a disabled source
// fades it as Text does.
func SpacedCaps(th *theme.Theme, style theme.TextStyle, ink color.NRGBA, tracking float32, value string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return LayoutSpacedCaps(gtx, th, style, ink, tracking, value)
	}
}

// LayoutSpacedCaps lays out SpacedCaps without building a widget, for callers that draw
// every frame and keep no closure.
func LayoutSpacedCaps(gtx layout.Context, th *theme.Theme, style theme.TextStyle, ink color.NRGBA, tracking float32, value string) layout.Dimensions {
	return layoutTracked(gtx, th, style, disabledInk(gtx, th, ink), tracking, value, true)
}

// LayoutTracked draws one case-preserving tracked line without a semantic node.
// A containing control can label its measured bounds without inheriting a
// widget.Label viewport larger than the text.
func LayoutTracked(gtx layout.Context, th *theme.Theme, style theme.TextStyle, ink color.NRGBA, tracking float32, value string) layout.Dimensions {
	return layoutTracked(gtx, th, style, disabledInk(gtx, th, ink), tracking, value, false)
}

// layoutTracked draws one tracked line and returns its size and baseline. The line box is
// placed as a Qt label with a fixed line height places it: the box is the style's line height
// or the glyphs' own height, whichever is taller, and the glyphs stand at its top, the extra
// height falling below them (measured with Qt 6.12: Martian Mono 9 in a 15 line has its
// baseline 9 from the top; at 168 in a 16 line the label is 202 tall).
func layoutTracked(gtx layout.Context, th *theme.Theme, style theme.TextStyle, ink color.NRGBA, tracking float32, value string, caps bool) layout.Dimensions {
	run := trackedLine(gtx, th, style, tracking, value, caps)
	run.paint(gtx, image.Point{}, ink)
	size := gtx.Constraints.Constrain(image.Pt(run.width, run.lineHeight(gtx, style)))
	return layout.Dimensions{Size: size, Baseline: size.Y - run.ascent}
}

// lineHeight is the height of the run's line box in style: the style's line or the glyphs.
func (run *trackedRun) lineHeight(gtx layout.Context, style theme.TextStyle) int {
	return max(run.ascent+run.descent, gtx.Sp(style.Line))
}

// trackedRun is one shaped, tracked line: its outline recorded once into its own ops, with
// the baseline at ascent below the origin.
type trackedRun struct {
	ops     op.Ops
	call    op.CallOp
	width   int
	ascent  int
	descent int
	// advances is the sum of the glyphs' own advances and clusters the clusters tracked; the
	// width is their total.
	advances fixed.Int26_6
	clusters int
}

// paint replays the run at at, in ink.
func (run *trackedRun) paint(gtx layout.Context, at image.Point, ink color.NRGBA) {
	if run.width == 0 && run.ascent == 0 {
		return
	}
	offset := op.Offset(at).Push(gtx.Ops)
	paint.ColorOp{Color: ink}.Add(gtx.Ops)
	run.call.Add(gtx.Ops)
	offset.Pop()
}

type trackedKey struct {
	shaper   *text.Shaper
	face     font.Font
	size     fixed.Int26_6
	tracking fixed.Int26_6
	caps     bool
	value    string
}

// trackedCacheLimit bounds the shared cache; a ticking clock or a scrolling list would
// otherwise grow it without end. When it fills it starts again.
const trackedCacheLimit = 512

var trackedCache struct {
	sync.Mutex
	runs   map[trackedKey]*trackedRun
	glyphs []text.Glyph
}

// trackedLine returns the run for value, shaping and recording it on first use.
func trackedLine(gtx layout.Context, th *theme.Theme, style theme.TextStyle, tracking float32, value string, caps bool) *trackedRun {
	key := trackedKey{
		shaper: th.Shaper, face: style.Face, size: fixed.I(gtx.Sp(style.Size)),
		tracking: fixed.Int26_6(tracking*gtx.Metric.PxPerSp*64 + sign(tracking)*0.5), caps: caps, value: value,
	}
	trackedCache.Lock()
	defer trackedCache.Unlock()
	if run := trackedCache.runs[key]; run != nil {
		return run
	}
	if trackedCache.runs == nil || len(trackedCache.runs) >= trackedCacheLimit {
		trackedCache.runs = make(map[trackedKey]*trackedRun)
	}
	run := shapeTracked(gtx, key)
	trackedCache.runs[key] = run
	return run
}

func sign(v float32) float32 {
	if v < 0 {
		return -1
	}
	return 1
}

// shapeTracked shapes key.value on one line and moves each cluster right by the tracking
// times the clusters before it. A run flowing toward the origin (right to left) is left
// untracked: spaced capitals are a Latin and kana convention.
func shapeTracked(gtx layout.Context, key trackedKey) *trackedRun {
	value := key.value
	if key.caps {
		value = strings.ToUpper(value)
	}
	run := &trackedRun{}
	if value == "" {
		return run
	}
	shaper := key.shaper
	shaper.LayoutString(text.Parameters{
		Font: key.face, PxPerEm: key.size, MaxLines: 1, MaxWidth: 1 << 24, Locale: gtx.Locale,
	}, value)
	glyphs := trackedCache.glyphs[:0]
	var shift, ascent, descent fixed.Int26_6
	for {
		glyph, ok := shaper.NextGlyph()
		if !ok {
			break
		}
		ascent, descent = max(ascent, glyph.Ascent), max(descent, glyph.Descent)
		run.advances += glyph.Advance
		if glyph.Flags&text.FlagTowardOrigin == 0 {
			glyph.X += shift
			if glyph.Flags&text.FlagClusterBreak != 0 {
				shift += key.tracking
				run.clusters++
			}
		}
		glyphs = append(glyphs, glyph)
	}
	trackedCache.glyphs = glyphs[:0]
	if len(glyphs) == 0 {
		return run
	}
	run.ascent, run.descent = ascent.Ceil(), descent.Ceil()
	run.width = max(0, (run.advances + fixed.Int26_6(run.clusters)*key.tracking).Ceil())

	first := glyphs[0]
	macro := op.Record(&run.ops)
	origin := op.Affine(f32.AffineId().Offset(f32.Pt(float32(first.X)/64, float32(run.ascent)))).Push(&run.ops)
	outline := clip.Outline{Path: shaper.Shape(glyphs)}.Op().Push(&run.ops)
	paint.PaintOp{}.Add(&run.ops)
	outline.Pop()
	if bitmaps := shaper.Bitmaps(glyphs); bitmaps != (op.CallOp{}) {
		bitmaps.Add(&run.ops)
	}
	origin.Pop()
	run.call = macro.Stop()
	return run
}
