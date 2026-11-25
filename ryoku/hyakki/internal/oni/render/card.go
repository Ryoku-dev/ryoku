package render

import (
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/design"

	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/strand"
)

// Checklist marks on the guide card: a step done, the step now, a step
// ahead.
const (
	CardDone = "done"
	CardNow  = "now"
	CardNext = "next"
)

// cardMaxLines is the most lines the card's step text wraps to; the last
// one is elided.
const cardMaxLines = 6

// CardItem is one checklist row: its mark and its words.
type CardItem struct {
	Mark, Text string
}

// CardPill is one action the card offers: the action it names, the words on
// it, and whether it is the primary one (drawn filled).
type CardPill struct {
	Action, Text string
	Primary      bool
}

// CardContent is what the guide card says, in reading order: the kicker,
// the step, a hint under it, the checklist and the actions. The surface
// chooses it; render only lays it out and paints it.
type CardContent struct {
	Kicker, Text, Hint string
	Items              []CardItem
	Pills              []CardPill
}

// CardRow is one laid-out checklist row: Y is its top, card-local.
type CardRow struct {
	Mark  string
	Label Label
	Y     float64
}

// Pill is one laid-out action: Rect is card-local logical px, the pill's hit
// rect.
type Pill struct {
	Action  string
	Primary bool
	Rect    strand.Rect
	Label   Label
}

// Card is the guide card laid out in logical px, card-local. The baselines
// are the left baselines of their lines at x = Pad.
type Card struct {
	Size    spec.Point
	Pad     float64
	Kicker  Label
	KickerY float64
	Lines   []Label
	LinesY  []float64
	Hint    Label
	HintY   float64
	Rows    []CardRow
	RowH    float64
	Pills   []Pill
}

// Empty reports whether the card has nothing to show.
func (k Card) Empty() bool { return !(k.Size.X > 0) || !(k.Size.Y > 0) }

// PillAt is the action of the pill under p (card-local logical px), or "".
func (k Card) PillAt(p spec.Point) string {
	for _, pl := range k.Pills {
		r := pl.Rect
		if p.X >= r.X && p.X < r.X+r.W && p.Y >= r.Y && p.Y < r.Y+r.H {
			return pl.Action
		}
	}
	return ""
}

// LayoutCard sets c in the card: the bubble's kicker, text and hint sizes
// (strand.bubble), one em of the text size as padding, at most
// strand.bubble.maxWidthPx wide with the text wrapped to fit, then the
// checklist at the hint size and the pills in the perch chip's type and
// padding, right-aligned on the last row.
func LayoutCard(c CardContent, s *spec.OniSpec) Card {
	b, ch := s.Strand.Bubble, s.Perch.Chip
	pad := b.TextPx
	inner := b.MaxWidthPx - 2*pad
	k := Card{Pad: pad}
	gap := math.Ceil(b.KickerPx * 0.75)
	y, w := pad, 0.0
	block := false
	open := func() {
		if block {
			y += gap
		}
		block = true
	}
	if strings.TrimSpace(c.Kicker) != "" {
		open()
		k.Kicker = ShapeLabel(c.Kicker, b.KickerPx, inner)
		k.KickerY = y + k.Kicker.Ascent
		y += k.Kicker.H()
		w = max(w, k.Kicker.W)
	}
	if strings.TrimSpace(c.Text) != "" {
		open()
		lh := math.Ceil(b.TextPx * 1.35)
		for _, l := range WrapLabel(c.Text, b.TextPx, inner, cardMaxLines) {
			k.Lines = append(k.Lines, l)
			k.LinesY = append(k.LinesY, y+(lh-l.H())/2+l.Ascent)
			y += lh
			w = max(w, l.W)
		}
	}
	if strings.TrimSpace(c.Hint) != "" {
		open()
		k.Hint = ShapeLabel(c.Hint, b.HintPx, inner)
		k.HintY = y + k.Hint.Ascent
		y += k.Hint.H()
		w = max(w, k.Hint.W)
	}
	if len(c.Items) > 0 {
		open()
		k.RowH = math.Ceil(b.HintPx * 1.7)
		for _, it := range c.Items {
			l := ShapeLabel(it.Text, b.HintPx, inner-k.RowH)
			k.Rows = append(k.Rows, CardRow{Mark: it.Mark, Label: l, Y: y})
			y += k.RowH
			w = max(w, k.RowH+l.W)
		}
	}
	var pillW []float64
	if n := len(c.Pills); n > 0 {
		if block {
			y += math.Ceil(pad * 0.75)
		}
		block = true
		ph := math.Ceil(ch.TextPx*1.3) + 2*ch.PadYPx
		each := (inner-float64(n-1)*ch.PadXPx)/float64(n) - 2*ch.PadXPx
		total := float64(n-1) * ch.PadXPx
		for _, p := range c.Pills {
			l := ShapeLabel(p.Text, ch.TextPx, max(ch.TextPx, each))
			pw := math.Ceil(l.W + 2*ch.PadXPx)
			k.Pills = append(k.Pills, Pill{Action: p.Action, Primary: p.Primary, Label: l, Rect: strand.Rect{Y: y, W: pw, H: ph}})
			pillW = append(pillW, pw)
			total += pw
		}
		y += ph
		w = max(w, total)
	}
	if !block {
		return Card{}
	}
	k.Size = spec.Point{X: math.Ceil(min(w, inner) + 2*pad), Y: math.Ceil(y + pad)}
	x := k.Size.X - pad
	for i := len(k.Pills) - 1; i >= 0; i-- {
		x -= pillW[i]
		k.Pills[i].Rect.X = x
		x -= ch.PadXPx
	}
	return k
}

// WrapLabel shapes text into at most maxLines lines no wider than maxW,
// breaking between words; what does not fit the last line is elided there,
// as is a single word wider than maxW.
func WrapLabel(text string, px, maxW float64, maxLines int) []Label {
	words := strings.Fields(text)
	var out []Label
	for len(words) > 0 && len(out) < maxLines {
		if len(out) == maxLines-1 {
			out = append(out, ShapeLabel(strings.Join(words, " "), px, maxW))
			break
		}
		line, n := ShapeLabel(words[0], px, maxW), 1
		for n < len(words) {
			l := ShapeLabel(strings.Join(words[:n+1], " "), px, 0)
			if l.W > maxW {
				break
			}
			line, n = l, n+1
		}
		out = append(out, line)
		words = words[n:]
	}
	return out
}

// PaintCard clears dst (sized to the card at dpr) and draws k in the look's
// design shape and roles only: a White card with an Ink border (Lithic's
// hairline, the bubble's border in the round designs), the kicker, hint and
// checklist in Body, the step and the row now in Ink, the marks for a done
// step and the step now in the done and listening accents (Ink where the
// look has none), the primary pill filled Ink with White words and the
// others White with an Ink border. The pill named pressed is filled Body.
// Everything is drawn at alpha.
func PaintCard(dst *image.NRGBA, k Card, s *spec.OniSpec, look Look, dpr, alpha float64, pressed string) {
	clearImage(dst)
	if dst.Bounds().Empty() || k.Empty() || !(dpr > 0) || !(alpha > 0) {
		return
	}
	b, ch := s.Strand.Bubble, s.Perch.Chip
	o := point{float64(dst.Bounds().Min.X), float64(dst.Bounds().Min.Y)}
	at := func(x, y float64) point { return point{o.X + x*dpr, o.Y + y*dpr} }
	dev := func(r strand.Rect) strand.Rect {
		return strand.Rect{X: o.X + r.X*dpr, Y: o.Y + r.Y*dpr, W: r.W * dpr, H: r.H * dpr}
	}
	paintPlate(dst, dev(strand.Rect{W: k.Size.X, H: k.Size.Y}), look.Design, ch.RadiusPx+ch.PadYPx, b.BorderPx, look.White, look.Ink, dpr, alpha)
	paintLabel(dst, k.Kicker, at(k.Pad, k.KickerY), look.Body, dpr, alpha)
	for i, l := range k.Lines {
		paintLabel(dst, l, at(k.Pad, k.LinesY[i]), look.Ink, dpr, alpha)
	}
	paintLabel(dst, k.Hint, at(k.Pad, k.HintY), look.Body, dpr, alpha)
	for _, r := range k.Rows {
		ink := look.Body
		if r.Mark == CardNow {
			ink = look.Ink
		}
		paintMark(dst, r.Mark, dev(strand.Rect{X: k.Pad, Y: r.Y, W: k.RowH, H: k.RowH}), b.HintPx*0.62*dpr, look, dpr, alpha)
		paintLabel(dst, r.Label, at(k.Pad+k.RowH, r.Y+(k.RowH-r.Label.H())/2+r.Label.Ascent), ink, dpr, alpha)
	}
	for _, p := range k.Pills {
		fill, edge, words := look.White, look.Ink, look.Ink
		if p.Primary {
			fill, words = look.Ink, look.White
		}
		if p.Action == pressed {
			fill = look.Body
		}
		paintPlate(dst, dev(p.Rect), look.Design, ch.RadiusPx, ch.BorderPx, fill, edge, dpr, alpha)
		paintLabel(dst, p.Label, at(p.Rect.X+ch.PadXPx, p.Rect.Y+(p.Rect.H-p.Label.H())/2+p.Label.Ascent), words, dpr, alpha)
	}
}

// paintPlate fills r (device px) with fill in the design's shape and rims
// it with edge: Lithic's hairline, borderPx (logical) in the round designs.
func paintPlate(dst *image.NRGBA, r strand.Rect, d design.Style, corner, borderPx float64, fill, edge color.NRGBA, dpr, alpha float64) {
	fillPolys(dst, [][]point{plate(r, d, corner, dpr)}, fill, alpha)
	border, join := borderPx, "round"
	if !d.Profile().Round {
		border, join = min(border, hairlinePx), "miter"
	}
	if !(border > 0) {
		return
	}
	bw := border * dpr
	inset := strand.Rect{X: r.X + bw/2, Y: r.Y + bw/2, W: r.W - bw, H: r.H - bw}
	ring := strokePieces([]contour{{pts: plate(inset, d, corner-border/2, dpr), closed: true}}, strokeStyle{width: bw, join: join, cap: "butt", miterLimit: 4}, perchTol)
	fillPolys(dst, ring, edge, alpha)
}

// paintMark draws a checklist mark of size m (device px) centred in cell
// (device px): a tick for a step done in the done accent, a filled plate in
// the design's shape for the step now in the listening accent, a hollow
// plate for a step ahead in Body.
func paintMark(dst *image.NRGBA, mark string, cell strand.Rect, m float64, look Look, dpr, alpha float64) {
	cx, cy := cell.X+cell.W/2-cell.W*0.15, cell.Y+cell.H/2
	box := strand.Rect{X: cx - m/2, Y: cy - m/2, W: m, H: m}
	line := max(1, 0.15*m)
	switch mark {
	case CardDone:
		tick := []point{{box.X + 0.08*m, cy + 0.02*m}, {box.X + 0.4*m, box.Y + 0.85*m}, {box.X + 0.95*m, box.Y + 0.15*m}}
		fillPolys(dst, strokePieces([]contour{{pts: tick}}, strokeStyle{width: line, join: "round", cap: "round"}, perchTol), look.accentOr("done", look.Ink), alpha)
	case CardNow:
		fillPolys(dst, [][]point{plate(box, look.Design, m/2/dpr, dpr)}, look.accentOr("listening", look.Ink), alpha)
	default:
		inset := strand.Rect{X: box.X + line/2, Y: box.Y + line/2, W: box.W - line, H: box.H - line}
		ring := strokePieces([]contour{{pts: plate(inset, look.Design, m/2/dpr, dpr), closed: true}}, strokeStyle{width: line, join: "miter", cap: "butt", miterLimit: 4}, perchTol)
		fillPolys(dst, ring, look.Body, alpha)
	}
}

// accentOr is palette id's accent, or fb when the look has none.
func (l Look) accentOr(id string, fb color.NRGBA) color.NRGBA {
	if c, ok := l.Accent[id]; ok && c.A > 0 {
		return c
	}
	return fb
}
