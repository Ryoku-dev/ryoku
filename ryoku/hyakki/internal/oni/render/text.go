package render

import (
	"image"
	"image/color"
	"math"
	"strings"
	"sync"

	giofont "gioui.org/font"
	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/fonts"
	"golang.org/x/image/math/fixed"
)

// ellipsis ends a label cut to its width.
const ellipsis = "…"

// typesetter shapes and outlines with Lithic's text face (Geologica Medium).
// A go-text face and the shaper are not safe for concurrent use, so every
// use holds mu.
type typesetter struct {
	mu     sync.Mutex
	face   *font.Face
	shaper shaping.HarfbuzzShaper
}

var setter = sync.OnceValue(func() *typesetter {
	faces := fonts.Collection()
	pick := -1
	for i, ff := range faces {
		if !strings.Contains(string(ff.Font.Typeface), "Geologica") || ff.Font.Style != giofont.Regular {
			continue
		}
		if pick < 0 || ff.Font.Weight == giofont.Medium {
			pick = i
		}
	}
	if pick < 0 {
		pick = 0
	}
	return &typesetter{face: faces[pick].Face.Face()}
})

// glyphAt is one shaped glyph: its id and pen position from the label's
// left baseline, logical px with y down.
type glyphAt struct {
	id   font.GID
	x, y float64
}

// Label is one line of text shaped at a size: Text is what it draws (the
// input, or a prefix of it ending in an ellipsis when it was wider than the
// width it was given), W its advance and Ascent/Descent its line extents,
// all logical px.
type Label struct {
	Text            string
	W               float64
	Ascent, Descent float64
	size            float64
	glyphs          []glyphAt
}

// H is the label's line height.
func (l Label) H() float64 { return l.Ascent + l.Descent }

// ShapeLabel shapes text on one line at px, cut with an ellipsis to fit
// maxW when maxW > 0. Text runs left to right in its first strong script.
func ShapeLabel(text string, px, maxW float64) Label {
	ts := setter()
	ts.mu.Lock()
	defer ts.mu.Unlock()
	runes := []rune(strings.Join(strings.Fields(text), " "))
	l := ts.shape(runes, px)
	if !(maxW > 0) || l.W <= maxW {
		return l
	}
	// the longest prefix that fits with the ellipsis after it
	lo, hi := 0, len(runes)
	best := ts.shape([]rune(ellipsis), px)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		cut := append([]rune(strings.TrimRight(string(runes[:mid]), " ")), []rune(ellipsis)...)
		if c := ts.shape(cut, px); c.W <= maxW {
			best, lo = c, mid
		} else {
			hi = mid - 1
		}
	}
	return best
}

func (ts *typesetter) shape(runes []rune, px float64) Label {
	l := Label{Text: string(runes), size: px}
	if len(runes) == 0 || !(px > 0) {
		return l
	}
	script := language.Latin
	for _, r := range runes {
		if s := language.LookupScript(r); s != language.Common && s != language.Inherited && s != language.Unknown {
			script = s
			break
		}
	}
	out := ts.shaper.Shape(shaping.Input{
		Text: runes, RunStart: 0, RunEnd: len(runes),
		Direction: di.DirectionLTR, Face: ts.face,
		Size: fixed.Int26_6(math.Round(px * 64)), Script: script, Language: language.DefaultLanguage(),
	})
	pen := 0.0
	for _, g := range out.Glyphs {
		l.glyphs = append(l.glyphs, glyphAt{id: g.GlyphID, x: pen + fromFixed(g.XOffset), y: -fromFixed(g.YOffset)})
		pen += fromFixed(g.XAdvance)
	}
	l.W = pen
	l.Ascent = fromFixed(out.LineBounds.Ascent)
	l.Descent = -fromFixed(out.LineBounds.Descent)
	return l
}

func fromFixed(v fixed.Int26_6) float64 { return float64(v) / 64 }

// outline is the label's glyph outlines as one path in device px, its left
// baseline at origin.
func (ts *typesetter) outline(l Label, origin point, dpr float64) path {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	k := l.size / float64(ts.face.Upem()) * dpr
	var p path
	for _, g := range l.glyphs {
		data, ok := ts.face.GlyphData(g.id).(font.GlyphOutline)
		if !ok {
			continue
		}
		ox, oy := origin.X+g.x*dpr, origin.Y+g.y*dpr
		at := func(sp ot.SegmentPoint) point {
			return point{ox + float64(sp.X)*k, oy - float64(sp.Y)*k}
		}
		open := false
		for _, s := range data.Segments {
			var sg seg
			switch s.Op {
			case ot.SegmentOpMoveTo:
				if open {
					p = append(p, seg{op: opClose})
				}
				sg = seg{op: opMove, pts: [3]point{at(s.Args[0])}}
				open = true
			case ot.SegmentOpLineTo:
				sg = seg{op: opLine, pts: [3]point{at(s.Args[0])}}
			case ot.SegmentOpQuadTo:
				sg = seg{op: opQuad, pts: [3]point{at(s.Args[0]), at(s.Args[1])}}
			case ot.SegmentOpCubeTo:
				sg = seg{op: opCubic, pts: [3]point{at(s.Args[0]), at(s.Args[1]), at(s.Args[2])}}
			}
			p = append(p, sg)
		}
		if open {
			p = append(p, seg{op: opClose})
		}
	}
	return p
}

// fillScratch is the coverage plane a flat fill rasterises into.
type fillScratch struct {
	ras rasterizer
	cov mask
}

var fillPool = sync.Pool{New: func() any { return new(fillScratch) }}

// fillPolys lays c over dst wherever the nonzero union of polys covers it,
// at alpha, and returns the device rectangle it touched.
func fillPolys(dst *image.NRGBA, polys [][]point, c color.NRGBA, alpha float64) image.Rectangle {
	if len(polys) == 0 || !(alpha > 0) {
		return image.Rectangle{}
	}
	sc := fillPool.Get().(*fillScratch)
	defer fillPool.Put(sc)
	sc.ras.reset()
	for _, p := range polys {
		sc.ras.add(p, identity)
	}
	sc.cov.setRect(dst.Bounds())
	r := sc.ras.cover(&sc.cov, false)
	if r.Empty() {
		return image.Rectangle{}
	}
	col := unitColour(c)
	a := float32(alpha) * col[3]
	for y := r.Min.Y; y < r.Max.Y; y++ {
		row := sc.cov.row(y, r.Min.X, r.Max.X)
		o := dst.PixOffset(r.Min.X, y)
		for _, cv := range row {
			if cv > 0 {
				over(dst.Pix[o:o+4], [3]float32{col[0], col[1], col[2]}, cv*a)
			}
			o += 4
		}
	}
	return r
}

// paintLabel draws l in c at alpha with its left baseline at the device
// point origin of dst.
func paintLabel(dst *image.NRGBA, l Label, origin point, c color.NRGBA, dpr, alpha float64) {
	if len(l.glyphs) == 0 {
		return
	}
	var polys [][]point
	for _, ct := range setter().outline(l, origin, dpr).flatten(perchTol) {
		polys = append(polys, ct.pts)
	}
	fillPolys(dst, polys, c, alpha)
}
