package chat

import (
	"image"
	"unicode"

	"gioui.org/font"
	"gioui.org/io/system"
	"gioui.org/text"
	"gioui.org/unit"
	"github.com/go-text/typesetting/segmenter"
	"golang.org/x/image/math/fixed"

	"ryoku-rashin/internal/app/look"
)

// wider than any line so the shaper never wraps a piece, and a width no other text asks for so they share no cached layout
const unwrapped = 1<<22 + 1

type inlineSet struct {
	spans []inlineSpan
	// bounds[i] is the first rune of span i, with one more for the end
	bounds []int
	text   []rune
	// kept as a string because a screen reader is given it every frame
	label           string
	lines           []inlineLine
	size            image.Point
	baseline, pitch int
	// the face has no italic, so emphasis is leaned by hand
	lean bool
}

type inlineLine struct {
	start, end int
	// edges[i] is where rune start+i begins; the last is where the line's text ends
	edges []fixed.Int26_6
	runs  []inlineRun
}

type inlineRun struct {
	span     int
	from, to fixed.Int26_6
	glyphs   []text.Glyph
}

type inlineFaces struct{ base, mono look.TextStyle }

func (faces inlineFaces) of(style inlineStyle) look.TextStyle {
	face := faces.base
	if style&inlineCode != 0 {
		face = faces.mono
	}
	if style&inlineStrong != 0 {
		face.Face.Weight = font.Bold
	}
	if style&inlineEmphasis != 0 {
		face.Face.Style = font.Italic
	}
	return face
}

type shapedText struct {
	glyphs []text.Glyph
	// edges[i] is where rune i begins, the last one the whole advance
	edges []fixed.Int26_6
	// cuts[i] is how many glyphs come before rune i, or -1 where the text cannot be parted
	cuts []int
}

func (shaped shapedText) runes() int { return len(shaped.edges) - 1 }

func (shaped shapedText) fit(from int, room fixed.Int26_6) int {
	to := from
	for rune := from + 1; rune < len(shaped.edges) && shaped.edges[rune]-shaped.edges[from] <= room; rune++ {
		if shaped.cuts[rune] >= 0 {
			to = rune
		}
	}
	return to
}

func (shaped shapedText) next(from int) int {
	for rune := from + 1; rune < len(shaped.cuts); rune++ {
		if shaped.cuts[rune] >= 0 {
			return rune
		}
	}
	return shaped.runes()
}

// the part of an unbreakable stretch that lies in one span, and the blanks after it
type inlinePiece struct {
	span   int
	start  int
	shaped shapedText
	blanks int
	blank  fixed.Int26_6
	// the text asked for a line break after it
	breaks bool
}

type inlineSetter struct {
	shaper *text.Shaper
	metric unit.Metric
	locale system.Locale
	faces  inlineFaces
	limit  fixed.Int26_6
	blanks map[look.TextStyle]fixed.Int26_6

	set  *inlineSet
	line inlineLine
	span int
	x    fixed.Int26_6
	// where the last glyph on the line ends, blanks after it hang past the edge
	inked fixed.Int26_6
}

// lines break where Unicode allows, and inside a word only when the word alone is too long
func setInline(shaper *text.Shaper, metric unit.Metric, locale system.Locale, faces inlineFaces, spans []inlineSpan, width int) *inlineSet {
	set := &inlineSet{spans: spans, bounds: make([]int, 0, len(spans)+1)}
	for _, span := range spans {
		set.bounds = append(set.bounds, len(set.text))
		set.text = append(set.text, []rune(span.text)...)
	}
	set.bounds = append(set.bounds, len(set.text))
	set.label = string(set.text)
	setter := inlineSetter{
		shaper: shaper, metric: metric, locale: locale, faces: faces,
		limit:  fixed.I(max(1, width)),
		blanks: make(map[look.TextStyle]fixed.Int26_6, 2),
		set:    set, line: inlineLine{edges: []fixed.Int26_6{0}},
	}
	rest := setter.shape(faces.base, []rune{'n'})
	var ascent, descent fixed.Int26_6
	if len(rest.glyphs) > 0 {
		ascent, descent = rest.glyphs[0].Ascent, rest.glyphs[0].Descent
	}
	set.baseline, set.pitch = ascent.Ceil(), max(1, metric.Sp(faces.base.Line))
	// asked for an italic it does not have, the shaper answers with the upright glyph
	if soft := setter.shape(faces.of(inlineEmphasis), []rune{'n'}); len(soft.glyphs) > 0 && len(rest.glyphs) > 0 {
		set.lean = soft.glyphs[0].ID == rest.glyphs[0].ID
	}

	from := 0
	for _, to := range setter.stretches() {
		setter.place(setter.pieces(from, to))
		from = to
	}
	if setter.line.end > setter.line.start || len(set.lines) == 0 {
		setter.endLine()
	}
	for _, line := range set.lines {
		for _, run := range line.runs {
			set.size.X = max(set.size.X, run.to.Ceil())
		}
	}
	set.size.Y = set.baseline + (len(set.lines)-1)*set.pitch + descent.Ceil()
	return set
}

func (setter *inlineSetter) shape(style look.TextStyle, value []rune) shapedText {
	shaped := shapedText{edges: make([]fixed.Int26_6, 1, len(value)+1), cuts: make([]int, 1, len(value)+1)}
	if len(value) == 0 {
		return shaped
	}
	setter.shaper.LayoutString(text.Parameters{
		Font: style.Face, PxPerEm: fixed.I(setter.metric.Sp(style.Size)), MaxWidth: unwrapped,
		Locale: setter.locale, LineHeight: fixed.I(setter.metric.Sp(style.Line)), LineHeightScale: 1,
	}, string(value))
	var left, cluster fixed.Int26_6
	for glyph, ok := setter.shaper.NextGlyph(); ok; glyph, ok = setter.shaper.NextGlyph() {
		shaped.glyphs = append(shaped.glyphs, glyph)
		cluster += glyph.Advance
		if glyph.Flags&text.FlagClusterBreak == 0 || glyph.Runes == 0 {
			continue
		}
		for rune := fixed.Int26_6(1); rune <= fixed.Int26_6(glyph.Runes); rune++ {
			shaped.edges = append(shaped.edges, left+cluster*rune/fixed.Int26_6(glyph.Runes))
			shaped.cuts = append(shaped.cuts, -1)
		}
		shaped.cuts[len(shaped.cuts)-1] = len(shaped.glyphs)
		left += cluster
		cluster = 0
	}
	// a shaper that counted the runes another way leaves only the two ends to stand on
	if shaped.runes() != len(value) {
		shaped.edges, shaped.cuts = shaped.edges[:1], shaped.cuts[:1]
		for rune := 1; rune <= len(value); rune++ {
			shaped.edges = append(shaped.edges, (left+cluster)*fixed.Int26_6(rune)/fixed.Int26_6(len(value)))
			shaped.cuts = append(shaped.cuts, -1)
		}
		shaped.cuts[len(value)] = len(shaped.glyphs)
	}
	return shaped
}

func (setter *inlineSetter) blank(style look.TextStyle) fixed.Int26_6 {
	if advance, ok := setter.blanks[style]; ok {
		return advance
	}
	apart := setter.shape(style, []rune("n n"))
	advance := apart.edges[2] - apart.edges[1]
	setter.blanks[style] = advance
	return advance
}

// where each stretch no line may break ends: Unicode's rules, with two changes
// a line may also break after any blank, the rules as written part every path at its first slash instead
// and code short enough for one line stays whole, half a command reads as a different command
func (setter *inlineSetter) stretches() []int {
	set := setter.set
	if len(set.spans) == 0 {
		return nil
	}
	breaks := make([]bool, len(set.text)+1)
	var segments segmenter.Segmenter
	segments.Init(set.text)
	for stretch := segments.LineIterator(); stretch.Next(); {
		line := stretch.Line()
		breaks[line.Offset+len(line.Text)] = true
	}
	for rune := 1; rune < len(set.text); rune++ {
		if blank(set.text[rune-1]) && !unicode.IsSpace(set.text[rune]) {
			breaks[rune] = true
		}
	}
	breaks[len(set.text)] = true
	ends := make([]int, 0, len(set.text)/4+1)
	span, whole := 0, setter.code(0) && setter.fitsWhole(0)
	for end := 1; end <= len(set.text); end++ {
		for end >= set.bounds[span+1] && span < len(set.spans)-1 {
			span++
			whole = setter.code(span) && setter.fitsWhole(span)
		}
		if breaks[end] && (!whole || end <= set.bounds[span] || end >= set.bounds[span+1]) {
			ends = append(ends, end)
		}
	}
	return ends
}

func blank(character rune) bool { return character == ' ' || character == '\t' }

func (setter *inlineSetter) fitsWhole(span int) bool {
	set := setter.set
	style := setter.faces.of(set.spans[span].style)
	shaped := setter.shape(style, set.text[set.bounds[span]:set.bounds[span+1]])
	return shaped.edges[shaped.runes()] <= setter.limit
}

func (setter *inlineSetter) pieces(from, to int) []inlinePiece {
	set := setter.set
	pieces := make([]inlinePiece, 0, 1)
	for from < to {
		for from >= set.bounds[setter.span+1] {
			setter.span++
		}
		end := min(to, set.bounds[setter.span+1])
		body := end
		piece := inlinePiece{span: setter.span, start: from}
		for body > from && unicode.IsSpace(set.text[body-1]) {
			body--
			piece.breaks = piece.breaks || set.text[body] == '\n'
		}
		style := setter.faces.of(set.spans[setter.span].style)
		piece.shaped = setter.shape(style, set.text[from:body])
		if piece.blanks = end - body; piece.blanks > 0 {
			piece.blank = setter.blank(style)
		}
		pieces = append(pieces, piece)
		from = end
	}
	return pieces
}

func (setter *inlineSetter) code(span int) bool { return setter.set.spans[span].style&inlineCode != 0 }

func (setter *inlineSetter) empty() bool { return setter.line.end == setter.line.start }

// the blanks after the last piece may hang past the edge, so they are not counted
func (setter *inlineSetter) need(pieces []inlinePiece) fixed.Int26_6 {
	need := fixed.Int26_6(0)
	for index, piece := range pieces {
		need += piece.shaped.edges[piece.shaped.runes()]
		if index < len(pieces)-1 {
			need += piece.blank * fixed.Int26_6(piece.blanks)
		}
	}
	return need
}

func (setter *inlineSetter) place(pieces []inlinePiece) {
	if !setter.empty() && setter.x+setter.need(pieces) > setter.limit {
		setter.breakLine()
	}
	for _, piece := range pieces {
		setter.put(piece)
	}
}

// a piece longer than a line is set over as many as it takes
func (setter *inlineSetter) put(piece inlinePiece) {
	for from, runes := 0, piece.shaped.runes(); from < runes; {
		room := setter.limit - setter.x
		to := runes
		if piece.shaped.edges[runes]-piece.shaped.edges[from] > room {
			if to = piece.shaped.fit(from, room); to == from {
				if !setter.empty() {
					setter.breakLine()
					continue
				}
				to = piece.shaped.next(from)
			}
		}
		setter.add(piece, from, to)
		if from = to; from < runes {
			setter.breakLine()
		}
	}
	setter.addBlanks(piece)
	if piece.breaks {
		setter.breakLine()
	}
}

func (setter *inlineSetter) add(piece inlinePiece, from, to int) {
	shaped, line := piece.shaped, &setter.line
	shift := setter.x - shaped.edges[from]
	if last := len(line.runs) - 1; last < 0 || line.runs[last].span != piece.span {
		line.runs = append(line.runs, inlineRun{span: piece.span, from: setter.x})
	}
	run := &line.runs[len(line.runs)-1]
	for _, glyph := range shaped.glyphs[shaped.cuts[from]:shaped.cuts[to]] {
		glyph.X += shift
		run.glyphs = append(run.glyphs, glyph)
	}
	for rune := from + 1; rune <= to; rune++ {
		line.edges = append(line.edges, shaped.edges[rune]+shift)
	}
	setter.x = shaped.edges[to] + shift
	setter.inked, run.to, line.end = setter.x, setter.x, piece.start+to
}

func (setter *inlineSetter) addBlanks(piece inlinePiece) {
	if piece.blanks == 0 {
		return
	}
	line, first := &setter.line, piece.start+piece.shaped.runes()
	for index := range piece.blanks {
		if setter.set.text[first+index] != '\n' {
			setter.x += piece.blank
		}
		line.edges = append(line.edges, setter.x)
	}
	line.end = first + piece.blanks
	if last := len(line.runs) - 1; last >= 0 && line.runs[last].span == piece.span {
		line.runs[last].to = setter.x
	}
}

func (setter *inlineSetter) endLine() {
	if last := len(setter.line.runs) - 1; last >= 0 {
		setter.line.runs[last].to = min(setter.line.runs[last].to, setter.inked)
	}
	setter.set.lines = append(setter.set.lines, setter.line)
}

func (setter *inlineSetter) breakLine() {
	setter.endLine()
	next := setter.line.end
	setter.line = inlineLine{start: next, end: next, edges: []fixed.Int26_6{0}}
	setter.x, setter.inked = 0, 0
}
