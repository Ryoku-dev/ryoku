package look

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
)

// the Hub's sheet geometry, read by no style
const (
	MastheadHeight unit.Dp = 34
	ColophonHeight unit.Dp = 26

	mastheadInset    unit.Dp = 21
	colophonMottoGap unit.Dp = 16
	barHeight        unit.Dp = 9
	barUnit          unit.Dp = 1
	barWordsGap      unit.Dp = 3
	barWordsTall     unit.Dp = 14
	mottoSize                = 11
	mottoTracking            = 2
	codeTracking             = 1.6
)

// what a page says of itself on the sheet; PageFor and PartPage fill it from the numbering table
type SheetPage struct {
	// "03.2"
	Number string
	// "Projects  /  Graph", translated sentence case
	Where string
	// "案・譜", dropped by the sheet while the desktop's glosses are off
	Gloss string
	// masthead facts after the page's place and before the clock, set in capitals
	Facts []string
	// the colophon's code in bars, the number when empty
	Code string
}

// Sheet is a page's share of the Hub's sheet: the frame draws Masthead above
// every page, a board or a settings page lays itself over Layout and ends its
// scroll with Colophon. The zero value is ready and a sheet at rest allocates
// nothing.
type Sheet struct {
	head  kit.Masthead
	folio kit.Folio

	ready bool
	gloss bool
	page  SheetPage
	kept  []string
	facts []string

	bars      op.Ops
	barsCall  op.CallOp
	barsKey   barsKey
	barsUpper string
	barsUnits int
}

type barsKey struct {
	code  string
	scale float32
	ink   color.NRGBA
}

func samePage(a, b SheetPage, kept []string) bool {
	if a.Number != b.Number || a.Where != b.Where || a.Gloss != b.Gloss || len(kept) != len(b.Facts) {
		return false
	}
	for index, fact := range b.Facts {
		if kept[index] != fact {
			return false
		}
	}
	return true
}

// Masthead draws the running head across the width it is given, 34 tall, on
// the pane header's inset. progress is how far the page is read, 0 to 1, or
// negative for a page that does not scroll.
func (s *Sheet) Masthead(gtx layout.Context, th *Theme, page SheetPage, progress float32) layout.Dimensions {
	gloss := th.Desktop.Gloss
	if !s.ready || gloss != s.gloss || !samePage(s.page, page, s.kept) {
		s.ready, s.gloss = true, gloss
		s.kept = append(s.kept[:0], page.Facts...)
		s.page = page
		s.page.Facts = nil
		s.head.Number, s.head.Where, s.head.Gloss = page.Number, page.Where, ""
		if gloss {
			s.head.Gloss = page.Gloss
		}
		s.facts = s.facts[:0]
		if place, ok := placeOfNumber(page.Number); ok {
			s.facts = append(s.facts, place.PagePlace())
		}
		for _, fact := range page.Facts {
			if fact != "" {
				s.facts = append(s.facts, strings.ToUpper(fact))
			}
		}
		s.head.Facts, s.head.Clock = s.facts, true
		s.head.EndRoom = mastheadInset
	}
	size := image.Pt(gtx.Constraints.Max.X, min(gtx.Dp(MastheadHeight), gtx.Constraints.Max.Y))
	inset := gtx.Dp(mastheadInset)
	head := gtx
	head.Constraints = layout.Exact(image.Pt(max(0, size.X-inset), size.Y))
	offset := op.Offset(image.Pt(inset, 0)).Push(gtx.Ops)
	s.head.Layout(head, th.Theme, progress)
	offset.Pop()
	return layout.Dimensions{Size: size}
}

// Layout lays body over the registration sheet and the page's folio, both
// filling gtx.Constraints.Max. It is for boards, Settings, Memory, Learning
// and project views; a conversation and its composer stand on the bare pane.
func (s *Sheet) Layout(gtx layout.Context, th *Theme, page SheetPage, body layout.Widget) layout.Dimensions {
	size := gtx.Constraints.Max
	under := gtx
	under.Constraints = layout.Exact(size)
	kit.RegistrationSheet(under, th.Theme)
	gloss := ""
	if th.Desktop.Gloss {
		gloss = lastGloss(page.Gloss)
	}
	s.folio.Layout(under, th.Theme, page.Number, gloss)
	body(gtx)
	return layout.Dimensions{Size: size}
}

// the folio sets one word huge, so a part's page shows the part's own
func lastGloss(gloss string) string {
	if at := strings.LastIndex(gloss, "・"); at >= 0 {
		return gloss[at+len("・"):]
	}
	return gloss
}

// Colophon is the sheet's foot, 26 tall: the page's code in Code 39 bars with
// its words under them at the right and, while glosses are on, Ryoku's motto
// to its left. Lay it out as the last row of the page's scroll.
func (s *Sheet) Colophon(gtx layout.Context, th *Theme, page SheetPage) layout.Dimensions {
	code := page.Code
	if code == "" {
		code = page.Number
	}
	size := gtx.Constraints.Constrain(image.Pt(gtx.Constraints.Max.X, gtx.Dp(ColophonHeight)))
	ink := th.R.Fg3
	key := barsKey{code: code, scale: gtx.Metric.PxPerDp, ink: ink}
	if key != s.barsKey || s.barsCall == (op.CallOp{}) {
		s.barsKey, s.barsUpper = key, strings.ToUpper(code)
		s.recordBars(gtx)
	}
	codeWidth := s.barsUnits * max(1, gtx.Dp(barUnit))
	codeHeight := gtx.Dp(barHeight) + gtx.Dp(barWordsTall)
	x, y := size.X-codeWidth, (size.Y-codeHeight)/2

	offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
	s.barsCall.Add(gtx.Ops)
	words := op.Offset(image.Pt(0, gtx.Dp(barHeight)+gtx.Dp(barWordsGap))).Push(gtx.Ops)
	free := gtx
	free.Constraints.Min = image.Point{}
	kit.LayoutSpacedCaps(free, th.Theme, th.T.MonoSm, ink, codeTracking, s.barsUpper)
	words.Pop()
	offset.Pop()

	if th.Desktop.Gloss {
		style := th.T.Caption
		style.Size = mottoSize
		record := op.Record(gtx.Ops)
		motto := kit.LayoutSpacedCaps(free, th.Theme, style, ink, mottoTracking, Motto)
		call := record.Stop()
		place := op.Offset(image.Pt(x-gtx.Dp(colophonMottoGap)-motto.Size.X, (size.Y-motto.Size.Y)/2)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		place.Pop()
	}
	return layout.Dimensions{Size: size}
}

// recorded once per code, scale and ink, then replayed
func (s *Sheet) recordBars(gtx layout.Context) {
	s.bars.Reset()
	macro := op.Record(&s.bars)
	bars, units := code39(s.barsKey.code)
	s.barsUnits = units
	unitPx, tall := max(1, gtx.Dp(barUnit)), gtx.Dp(barHeight)
	var path clip.Path
	path.Begin(&s.bars)
	for _, bar := range bars {
		x0, x1 := bar[0]*unitPx, (bar[0]+bar[1])*unitPx
		path.MoveTo(layout.FPt(image.Pt(x0, 0)))
		path.LineTo(layout.FPt(image.Pt(x1, 0)))
		path.LineTo(layout.FPt(image.Pt(x1, tall)))
		path.LineTo(layout.FPt(image.Pt(x0, tall)))
		path.Close()
	}
	paint.FillShape(&s.bars, s.barsKey.ink, clip.Outline{Path: path.End()}.Op())
	s.barsCall = macro.Stop()
}

// each character's nine elements: b and w a narrow bar and gap, B and W wide, as the Hub's card barcode draws them
var code39Elements = map[rune]string{
	'0': "bwbWBwBwb", '1': "BwbWbwbwB", '2': "bwBWbwbwB", '3': "BwBWbwbwb",
	'4': "bwbWBwbwB", '5': "BwbWBwbwb", '6': "bwBWBwbwb", '7': "bwbWbwBwB",
	'8': "BwbWbwBwb", '9': "bwBWbwBwb", 'A': "BwbwbWbwB", 'B': "bwBwbWbwB",
	'C': "BwBwbWbwb", 'D': "bwbwBWbwB", 'E': "BwbwBWbwb", 'F': "bwBwBWbwb",
	'G': "bwbwbWBwB", 'H': "BwbwbWBwb", 'I': "bwBwbWBwb", 'J': "bwbwBWBwb",
	'K': "BwbwbwbWB", 'L': "bwBwbwbWB", 'M': "BwBwbwbWb", 'N': "bwbwBwbWB",
	'O': "BwbwBwbWb", 'P': "bwBwBwbWb", 'Q': "bwbwbwBWB", 'R': "BwbwbwBWb",
	'S': "bwBwbwBWb", 'T': "bwbwBwBWb", 'U': "BWbwbwbwB", 'V': "bWBwbwbwB",
	'W': "BWBwbwbwb", 'X': "bWbwBwbwB", 'Y': "BWbwBwbwb", 'Z': "bWBwBwbwb",
	'-': "bWbwbwBwB", '.': "BWbwbwBwb", ' ': "bWBwbwBwb", '*': "bWbwBwBwb",
}

// each bar's start and width in units between the start and stop character, and the code's whole width; a character Code 39 has no bars for is left out
func code39(text string) (bars [][2]int, width int) {
	x := 0
	for _, character := range "*" + strings.ToUpper(text) + "*" {
		elements, ok := code39Elements[character]
		if !ok {
			continue
		}
		for _, element := range elements {
			wide := 1
			if element == 'B' || element == 'W' {
				wide = 3
			}
			if element == 'b' || element == 'B' {
				bars = append(bars, [2]int{x, wide})
			}
			x += wide
		}
		x++
	}
	return bars, max(0, x-1)
}
