package render

import (
	"image"
	"math"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/design"

	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/strand"
)

// LayoutPill sets Hands' pill: an optional scope line, where Hands works,
// in the kicker's type above one row with its status line on the left and
// its actions right after it, in the perch chip's type and padding, at most
// strand.bubble.maxWidthPx wide. It is a Card, so PaintCard draws it in the
// look's shape and roles like the guide card; an empty line and no actions
// lay out nothing, whatever the scope says.
func LayoutPill(scope, text string, pills []CardPill, s *spec.OniSpec) Card {
	b, ch := s.Strand.Bubble, s.Perch.Chip
	if text == "" && len(pills) == 0 {
		return Card{}
	}
	pad := math.Ceil(ch.PadXPx)
	ph := math.Ceil(ch.TextPx*1.3) + 2*ch.PadYPx
	inner := b.MaxWidthPx - 2*pad
	k := Card{Pad: pad}
	top := pad
	if scope != "" {
		k.Kicker = ShapeLabel(scope, b.KickerPx, inner)
		k.KickerY = pad + k.Kicker.Ascent
		top += math.Ceil(k.Kicker.H() + b.KickerPx*0.5)
	}
	var widths []float64
	used := 0.0
	for _, p := range pills {
		l := ShapeLabel(p.Text, ch.TextPx, max(ch.TextPx, inner/float64(len(pills)+1)))
		pw := math.Ceil(l.W + 2*ch.PadXPx)
		k.Pills = append(k.Pills, Pill{Action: p.Action, Primary: p.Primary, Label: l, Rect: strand.Rect{Y: top, W: pw, H: ph}})
		widths = append(widths, pw)
		used += pw + ch.PadXPx
	}
	x := pad
	if text != "" {
		l := ShapeLabel(text, ch.TextPx, max(ch.TextPx, inner-used))
		k.Lines = []Label{l}
		k.LinesY = []float64{top + (ph-l.H())/2 + l.Ascent}
		x += math.Ceil(l.W) + ch.PadXPx
	}
	for i := range k.Pills {
		k.Pills[i].Rect.X = x
		x += widths[i] + ch.PadXPx
	}
	// every part left one gap after it
	x -= ch.PadXPx
	w := max(x, pad+math.Ceil(k.Kicker.W))
	k.Size = spec.Point{X: math.Ceil(w + pad), Y: math.Ceil(top + ph + pad)}
	return k
}

// The Hands glow's strips round an output.
const (
	GlowTop = iota
	GlowBottom
	GlowLeft
	GlowRight
	GlowStrips
)

// EdgeGlow is how strong the Hands glow is over each of its four strips,
// worked out once for an output's size, the strip depth, the desktop's
// design and the device scale, so each breath only scales it. The glow is
// strongest at the output's edge and fades to nothing StripPx in; where two
// edges meet its inner edge is a cut corner in Lithic and a quarter circle
// in the round designs.
type EdgeGlow struct {
	out   image.Point
	strip int
	round bool
	scale float64
	// Rects are the strips, output-local logical px: top and bottom span
	// the width, left and right the height between them.
	Rects [GlowStrips]image.Rectangle
	size  [GlowStrips]image.Point
	base  [GlowStrips][]uint8
}

// NewEdgeGlow lays the glow out on an output of size out (logical px),
// stripPx deep, at scale device px per logical px.
func NewEdgeGlow(out image.Point, stripPx int, d design.Style, scale float64) *EdgeGlow {
	g := &EdgeGlow{out: out, strip: stripPx, round: d.Profile().Round, scale: scale}
	s := max(1, min(stripPx, out.X/2, out.Y/2))
	if out.X <= 0 || out.Y <= 0 || !(scale > 0) {
		return g
	}
	w, h := out.X, out.Y
	g.Rects = [GlowStrips]image.Rectangle{
		GlowTop:    image.Rect(0, 0, w, s),
		GlowBottom: image.Rect(0, h-s, w, h),
		GlowLeft:   image.Rect(0, s, s, h-s),
		GlowRight:  image.Rect(w-s, s, w, h-s),
	}
	S := float64(s)
	for i, r := range g.Rects {
		if r.Empty() {
			g.Rects[i] = image.Rectangle{}
			continue
		}
		size := image.Pt(int(math.Ceil(float64(r.Dx())*scale)), int(math.Ceil(float64(r.Dy())*scale)))
		base := make([]uint8, size.X*size.Y)
		for y := range size.Y {
			py := float64(r.Min.Y) + (float64(y)+0.5)/scale
			dy := min(py, float64(h)-py)
			for x := range size.X {
				px := float64(r.Min.X) + (float64(x)+0.5)/scale
				dx := min(px, float64(w)-px)
				d := min(dx, dy)
				if g.round && dx < S && dy < S {
					d = max(0, S-math.Hypot(S-dx, S-dy))
				}
				if d >= S {
					continue
				}
				t := 1 - max(d, 0)/S
				base[y*size.X+x] = uint8(math.Round(255 * t * t))
			}
		}
		g.size[i], g.base[i] = size, base
	}
	return g
}

// Fits reports whether g was laid out for these numbers.
func (g *EdgeGlow) Fits(out image.Point, stripPx int, d design.Style, scale float64) bool {
	return g != nil && g.out == out && g.strip == stripPx && g.round == d.Profile().Round && g.scale == scale
}

// DeviceSize is the size in device px of strip i's buffer.
func (g *EdgeGlow) DeviceSize(i int) image.Point { return g.size[i] }

// Paint draws strip i into dst, sized DeviceSize(i), at level of its full
// strength, in the look's thinking accent (the attention role Hands wears),
// or Ink where the look has none.
func (g *EdgeGlow) Paint(i int, dst *image.NRGBA, look Look, level float64) {
	c := look.accentOr("thinking", look.Ink)
	level = min(max(level, 0), 1)
	base := g.base[i]
	n := min(len(base), len(dst.Pix)/4)
	for k := range n {
		p := dst.Pix[k*4 : k*4+4]
		p[0], p[1], p[2] = c.R, c.G, c.B
		p[3] = uint8(math.Round(float64(base[k]) * level * float64(c.A) / 255))
	}
}
