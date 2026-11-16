package kit

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

// Masthead is the sheet's running head over a page, in the smallest mono, as the Hub draws it
// (components/sheet/Masthead.qml), read by no style. At the left: "§ " and the page's number in
// the accent, the page's place in spaced capitals, and its gloss; at the right, kept EndRoom
// short of the edge for the window's close button: the facts with 3 by 3 squares between, the
// last in fg2, the others in fg3, then the clock when Clock is set. Under it a tick ruler
// whose marker stands where the page is read to.
//
// A Ryotunes strip reads "§ 03  NOW PLAYING  /  LYRICS  ...  P 02/05 · 12:40". The masthead
// is 34 tall and fills its width; at rest it allocates nothing and asks for no frame, save
// the clock's tick while Ambient is on.
type Masthead struct {
	// Number is the page's number, "01.3"; Home is "00".
	Number string
	// Where is the page's place in sentence case, "Appearance  /  Window style" (two spaces
	// each side of the slash); it is set in capitals.
	Where string
	// Gloss is the page's kanji, "外観・窓飾"; leave it empty while glosses are off
	// (palette.Desktop.Gloss).
	Gloss string
	// Facts stand at the right before the clock, "P 02/39", "NIRI".
	Facts []string
	// Clock ends the facts on the time, with seconds while Ambient is on.
	Clock bool
	// EndRoom is kept free at the right; zero means 48.
	EndRoom unit.Dp

	ruler TickRuler
	clock Clock
	// number is "§ " + Number, kept so a frame at rest builds no string
	number, numberOf string
}

// Masthead geometry, in dp (Masthead.qml:49-130).
const (
	mastheadHeight    = 34
	mastheadWordsY    = 8
	mastheadWordsH    = 14
	mastheadLeadGap   = 10
	mastheadFactGap   = 12
	mastheadSquare    = 3
	mastheadEndRoom   = 48
	mastheadRulerFoot = 2
	mastheadGlossSize = 11
)

// Masthead tracking, in sp (Masthead.qml:61,68,81,117).
const (
	trackNumber = 0.8
	trackWhere  = 1.4
	trackGloss  = 1
	trackFacts  = 1.2
)

// Layout draws the masthead across the width it is given. progress is how far the page is
// read, 0..1, or negative for a page that does not scroll.
func (masthead *Masthead) Layout(gtx layout.Context, th *theme.Theme, progress float32) layout.Dimensions {
	size := gtx.Constraints.Constrain(image.Pt(gtx.Constraints.Max.X, gtx.Dp(mastheadHeight)))
	centre := gtx.Dp(mastheadWordsY) + gtx.Dp(mastheadWordsH)/2
	endRoom := masthead.EndRoom
	if endRoom == 0 {
		endRoom = mastheadEndRoom
	}
	end := size.X - gtx.Dp(endRoom)
	mono := th.T.MonoSm

	// the lead row, 10 apart
	if masthead.numberOf != masthead.Number {
		masthead.number, masthead.numberOf = "", masthead.Number
		if masthead.Number != "" {
			masthead.number = "§ " + masthead.Number
		}
	}
	lead := gtx.Dp(mastheadLeadGap)
	x := 0
	if width := mastheadText(gtx, th, mono, th.R.Sel, trackNumber, masthead.number, x, centre, false); width > 0 {
		x += width + lead
	}
	if width := mastheadText(gtx, th, mono, th.R.Fg2, trackWhere, masthead.Where, x, centre, true); width > 0 {
		x += width + lead
	}
	if masthead.Gloss != "" {
		gloss := th.T.Caption
		gloss.Size = mastheadGlossSize
		mastheadText(gtx, th, gloss, th.R.Fg3, trackGloss, masthead.Gloss, x, centre, false)
	}

	// the facts, laid right to left from the end room, 12 apart with a square between
	gap, square := gtx.Dp(mastheadFactGap), gtx.Dp(mastheadSquare)
	right := end
	count := len(masthead.Facts)
	if masthead.Clock {
		count++
	}
	for index := count - 1; index >= 0; index-- {
		var fact string
		if index < len(masthead.Facts) {
			fact = masthead.Facts[index]
		} else {
			fact = masthead.clock.Text(gtx)
		}
		ink := th.R.Fg3
		if index == count-1 {
			ink = th.R.Fg2
		}
		run := trackedLine(gtx, th, mono, trackFacts, fact, false)
		right -= run.width
		mastheadPaint(gtx, run, mono, ink, right, centre)
		if index > 0 {
			right -= gap + square
			top := centre - square/2
			paint.FillShape(gtx.Ops, th.R.Fg3, clip.Rect(image.Rect(right, top, right+square, top+square)).Op())
			right -= gap
		}
	}

	// the ruler, 2 above the foot, ending at the end room
	rulerGtx := gtx
	rulerGtx.Constraints = layout.Exact(image.Pt(max(0, end), gtx.Dp(rulerTall)))
	offset := op.Offset(image.Pt(0, size.Y-gtx.Dp(mastheadRulerFoot)-gtx.Dp(rulerTall))).Push(gtx.Ops)
	masthead.ruler.Style = RulerMasthead
	masthead.ruler.Layout(rulerGtx, th, progress)
	offset.Pop()
	return layout.Dimensions{Size: size}
}

// mastheadText draws value with its line centred on centre at x, and returns its width.
func mastheadText(gtx layout.Context, th *theme.Theme, style theme.TextStyle, ink color.NRGBA, tracking float32, value string, x, centre int, caps bool) int {
	if value == "" {
		return 0
	}
	run := trackedLine(gtx, th, style, tracking, value, caps)
	mastheadPaint(gtx, run, style, ink, x, centre)
	return run.width
}

// mastheadPaint centres a run's line box on centre, as Qt anchors a fixed-height label's
// verticalCenter on the words row (a half pixel rounds down the page); the glyphs stand at the
// box's top.
func mastheadPaint(gtx layout.Context, run *trackedRun, style theme.TextStyle, ink color.NRGBA, x, centre int) {
	top := (2*centre - run.lineHeight(gtx, style) + 1) / 2
	run.paint(gtx, image.Pt(x, top), ink)
}
