package render

import (
	"image"
	"math"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/design"

	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/strand"
)

// perchTol is the flattening tolerance of the perch shapes, device px.
const perchTol = 0.1

// hairlinePx is Lithic's hairline border, logical px.
const hairlinePx = 1.0

// plate is the closed outline of r (device px) in the design's shape: the
// corner size (logical px) passes through the design's corner rule, then
// Lithic cuts its corners at 45 degrees and the round designs round them.
func plate(r strand.Rect, d design.Style, corner, dpr float64) []point {
	c := float64(d.Corner(float32(corner), float32(r.W/dpr), float32(r.H/dpr))) * dpr
	if d.Profile().Round {
		return roundRect(r, c)
	}
	return cutRect(r, c)
}

// cutRect is r with each corner cut at 45 degrees, cut long on each side.
func cutRect(r strand.Rect, cut float64) []point {
	cut = max(0, min(cut, r.W/2, r.H/2))
	if cut == 0 {
		return []point{{r.X, r.Y}, {r.X + r.W, r.Y}, {r.X + r.W, r.Y + r.H}, {r.X, r.Y + r.H}}
	}
	x0, y0, x1, y1 := r.X, r.Y, r.X+r.W, r.Y+r.H
	return []point{
		{x0 + cut, y0}, {x1 - cut, y0}, {x1, y0 + cut}, {x1, y1 - cut},
		{x1 - cut, y1}, {x0 + cut, y1}, {x0, y1 - cut}, {x0, y0 + cut},
	}
}

// roundRect is the closed outline of r (device px) with corner radius rad.
func roundRect(r strand.Rect, rad float64) []point {
	rad = max(0, min(rad, r.W/2, r.H/2))
	if rad == 0 {
		return []point{{r.X, r.Y}, {r.X + r.W, r.Y}, {r.X + r.W, r.Y + r.H}, {r.X, r.Y + r.H}}
	}
	n := 4
	if rad > perchTol {
		n = max(n, int(math.Ceil(math.Pi/2/math.Acos(1-perchTol/rad))))
	}
	n = min(n, 64)
	centres := [4]point{{r.X + r.W - rad, r.Y + rad}, {r.X + r.W - rad, r.Y + r.H - rad}, {r.X + rad, r.Y + r.H - rad}, {r.X + rad, r.Y + rad}}
	out := make([]point, 0, 4*(n+1))
	for q, c := range centres {
		for i := 0; i <= n; i++ {
			th := (float64(q) - 1 + float64(i)/float64(n)) * math.Pi / 2
			s, co := math.Sincos(th)
			out = append(out, point{c.X + rad*co, c.Y + rad*s})
		}
	}
	return out
}

// HighlightMargin is how far the drag highlight's pixels reach outside the
// window box, logical px: the pad plus the rimmed line.
func HighlightMargin(s *spec.OniSpec) float64 {
	h := s.Perch.Highlight
	return h.PadPx + h.WidthPx/2 + h.RimPx + 1
}

// PaintHighlight clears dst and draws the drag highlight round box: an
// outline perch.highlight.padPx outside it in the look's design shape,
// inked like the strand (an Ink rim each side of a White line), at alpha.
// origin is the output-local logical point at dst's top-left, box is
// output-local logical px. It returns the device-px bounds it painted.
func PaintHighlight(dst *image.NRGBA, origin image.Point, box strand.Rect, s *spec.OniSpec, look Look, dpr, alpha float64) image.Rectangle {
	clearImage(dst)
	if dst.Bounds().Empty() || !(dpr > 0) || !(alpha > 0) {
		return image.Rectangle{}
	}
	h := s.Perch.Highlight
	b := dst.Bounds()
	r := strand.Rect{
		X: (box.X-h.PadPx-float64(origin.X))*dpr + float64(b.Min.X),
		Y: (box.Y-h.PadPx-float64(origin.Y))*dpr + float64(b.Min.Y),
		W: (box.W + 2*h.PadPx) * dpr,
		H: (box.H + 2*h.PadPx) * dpr,
	}
	ring := []contour{{pts: plate(r, look.Design, h.RadiusPx+h.PadPx, dpr), closed: true}}
	join := "round"
	if !look.Design.Profile().Round {
		join = "miter"
	}
	sc := strandPool.Get().(*strandScratch)
	defer strandPool.Put(sc)
	sc.inkPolys = strokePieces(ring, strokeStyle{width: (h.WidthPx + 2*h.RimPx) * dpr, join: join, cap: "butt", miterLimit: 4}, perchTol)
	sc.whitePolys = strokePieces(ring, strokeStyle{width: h.WidthPx * dpr, join: join, cap: "butt", miterLimit: 4}, perchTol)
	return sc.paint(dst, look, alpha)
}

// Chip is the perch chip laid out in logical px: Size is the card, Label the
// text it shows (elided to fit perch.chip.maxWidthPx), and Detach the square
// of its detach action, chip-local.
type Chip struct {
	Size   spec.Point
	Detach strand.Rect
	Label  Label
}

// LayoutChip sets text in the chip: the label, then the detach action, a
// square one line high, each with the chip's side padding; the whole chip
// is at most perch.chip.maxWidthPx wide, the label elided to fit.
func LayoutChip(text string, s *spec.OniSpec) Chip {
	c := s.Perch.Chip
	line := math.Ceil(c.TextPx * 1.3)
	l := ShapeLabel(text, c.TextPx, max(c.TextPx, c.MaxWidthPx-3*c.PadXPx-line))
	line = math.Ceil(max(line, l.H()))
	w := math.Ceil(c.PadXPx + l.W + c.PadXPx + line + c.PadXPx)
	return Chip{
		Size:   spec.Point{X: w, Y: line + 2*c.PadYPx},
		Detach: strand.Rect{X: w - c.PadXPx - line, Y: c.PadYPx, W: line, H: line},
		Label:  l,
	}
}

// PaintChip clears dst (sized to the chip at dpr) and draws ch in the look's
// design shape: a White card with an Ink border (Lithic's hairline, the
// spec's border in the round designs), the label in Ink and the detach
// action as an Ink cross, at alpha.
func PaintChip(dst *image.NRGBA, ch Chip, s *spec.OniSpec, look Look, dpr, alpha float64) {
	clearImage(dst)
	if dst.Bounds().Empty() || !(dpr > 0) || !(alpha > 0) {
		return
	}
	c := s.Perch.Chip
	b := dst.Bounds()
	o := point{float64(b.Min.X), float64(b.Min.Y)}
	card := strand.Rect{X: o.X, Y: o.Y, W: ch.Size.X * dpr, H: ch.Size.Y * dpr}
	fillPolys(dst, [][]point{plate(card, look.Design, c.RadiusPx, dpr)}, look.White, alpha)
	border, join := c.BorderPx, "round"
	if !look.Design.Profile().Round {
		border, join = min(border, hairlinePx), "miter"
	}
	bw := border * dpr
	inset := strand.Rect{X: card.X + bw/2, Y: card.Y + bw/2, W: card.W - bw, H: card.H - bw}
	ring := strokePieces([]contour{{pts: plate(inset, look.Design, c.RadiusPx-border/2, dpr), closed: true}}, strokeStyle{width: bw, join: join, cap: "butt", miterLimit: 4}, perchTol)
	fillPolys(dst, ring, look.Ink, alpha)
	bw = c.BorderPx * dpr
	baseline := point{o.X + c.PadXPx*dpr, o.Y + (c.PadYPx+(ch.Detach.H-ch.Label.H())/2+ch.Label.Ascent)*dpr}
	paintLabel(dst, ch.Label, baseline, look.Ink, dpr, alpha)
	d := ch.Detach
	in := d.W * 0.28
	x0, y0 := o.X+(d.X+in)*dpr, o.Y+(d.Y+in)*dpr
	x1, y1 := o.X+(d.X+d.W-in)*dpr, o.Y+(d.Y+d.H-in)*dpr
	cross := strokePieces([]contour{{pts: []point{{x0, y0}, {x1, y1}}}, {pts: []point{{x1, y0}, {x0, y1}}}}, strokeStyle{width: bw, join: "round", cap: "round"}, perchTol)
	fillPolys(dst, cross, look.Ink, alpha)
}

func clearImage(dst *image.NRGBA) {
	b := dst.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		o := dst.PixOffset(b.Min.X, y)
		clear(dst.Pix[o : o+b.Dx()*4])
	}
}
