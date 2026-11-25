package render

import (
	"image"
	"image/color"
	"sync"

	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/strand"
)

// strandOutline is the ink each side of every strand stroke's white core.
var strandOutline = sync.OnceValue(func() float64 {
	s, err := spec.Load()
	if err != nil {
		panic(err)
	}
	return s.Strand.Stroke.OutlinePx
})

// strandTol is the flattening tolerance of the round joins and caps, device px.
const strandTol = 0.1

type strandScratch struct {
	ras        rasterizer
	ink, white mask
	inkPolys   [][]point
	whitePolys [][]point
}

var strandPool = sync.Pool{New: func() any { return new(strandScratch) }}

// PaintStrand clears dst and draws frame f's strokes into it: each stroke an
// ink line Width + 2·strand.stroke.outlinePx wide under a white core Width
// wide, both with round joins and caps, in the Look's Ink and White roles.
// The opaque strokes are one line (all their ink, then all their white), so
// where the thread meets its ring or pen it reads as one; translucent ones
// (a swing's fading line, a fading burst) lie beneath, each at its alpha.
// origin is the output-local logical point at dst's top-left, and dpr is
// device px per logical px. It returns the device-px bounds it painted.
func PaintStrand(dst *image.NRGBA, origin image.Point, f strand.Frame, look Look, dpr float64) image.Rectangle {
	b := dst.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		o := dst.PixOffset(b.Min.X, y)
		clear(dst.Pix[o : o+b.Dx()*4])
	}
	if b.Empty() || !(dpr > 0) {
		return image.Rectangle{}
	}
	sc := strandPool.Get().(*strandScratch)
	defer strandPool.Put(sc)

	outline := strandOutline()
	toDevice := func(pts []spec.Point) []point {
		out := make([]point, len(pts))
		for i, p := range pts {
			out[i] = point{(p.X-float64(origin.X))*dpr + float64(b.Min.X), (p.Y-float64(origin.Y))*dpr + float64(b.Min.Y)}
		}
		return out
	}
	pieces := func(into [][]point, pts []point, width float64) [][]point {
		st := strokeStyle{width: width, join: "round", cap: "round"}
		return append(into, strokePieces([]contour{{pts: pts}}, st, strandTol)...)
	}

	var painted image.Rectangle
	pass := func(alpha float64) {
		r := sc.paint(dst, look, alpha)
		painted = painted.Union(r)
	}
	for _, st := range f.Strokes {
		if !(st.Alpha > 0) || st.Alpha >= 1 || len(st.Pts) == 0 || !(st.Width > 0) {
			continue
		}
		pts := toDevice(st.Pts)
		sc.inkPolys = pieces(sc.inkPolys[:0], pts, (st.Width+2*outline)*dpr)
		sc.whitePolys = pieces(sc.whitePolys[:0], pts, st.Width*dpr)
		pass(st.Alpha)
	}
	sc.inkPolys, sc.whitePolys = sc.inkPolys[:0], sc.whitePolys[:0]
	for _, st := range f.Strokes {
		if st.Alpha < 1 || len(st.Pts) == 0 || !(st.Width > 0) {
			continue
		}
		pts := toDevice(st.Pts)
		sc.inkPolys = pieces(sc.inkPolys, pts, (st.Width+2*outline)*dpr)
		sc.whitePolys = pieces(sc.whitePolys, pts, st.Width*dpr)
	}
	pass(1)
	return painted
}

// paint covers the current ink and white polygons and lays white over ink,
// together at alpha, over dst.
func (sc *strandScratch) paint(dst *image.NRGBA, look Look, alpha float64) image.Rectangle {
	if len(sc.inkPolys) == 0 {
		return image.Rectangle{}
	}
	id := affine{A: 1, D: 1}
	sc.ras.reset()
	for _, p := range sc.inkPolys {
		sc.ras.add(p, id)
	}
	sc.ink.setRect(dst.Bounds())
	r := sc.ras.cover(&sc.ink, false)
	if r.Empty() {
		return image.Rectangle{}
	}
	sc.white.setRect(dst.Bounds())
	sc.white.clear(r)
	sc.ras.reset()
	for _, p := range sc.whitePolys {
		sc.ras.add(p, id)
	}
	wr := sc.ras.cover(&sc.white, false)

	ink, white := unitColour(look.Ink), unitColour(look.White)
	ka := float32(alpha)
	minX, minY, maxX, maxY := r.Max.X, r.Max.Y, r.Min.X, r.Min.Y
	for y := r.Min.Y; y < r.Max.Y; y++ {
		irow := sc.ink.row(y, r.Min.X, r.Max.X)
		var wrow []float32
		if y >= wr.Min.Y && y < wr.Max.Y {
			wrow = sc.white.row(y, r.Min.X, r.Max.X)
		}
		o := dst.PixOffset(r.Min.X, y)
		for i, ci := range irow {
			var cw float32
			if wrow != nil {
				cw = wrow[i]
			}
			ci *= ink[3]
			cw *= white[3]
			a := cw + ci*(1-cw)
			if a <= 0 {
				o += 4
				continue
			}
			ki := ci * (1 - cw) / a
			kw := cw / a
			over(dst.Pix[o:o+4], [3]float32{
				white[0]*kw + ink[0]*ki,
				white[1]*kw + ink[1]*ki,
				white[2]*kw + ink[2]*ki,
			}, a*ka)
			x := r.Min.X + i
			minX, minY = min(minX, x), min(minY, y)
			maxX, maxY = max(maxX, x+1), max(maxY, y+1)
			o += 4
		}
	}
	if maxX <= minX {
		return image.Rectangle{}
	}
	return image.Rect(minX, minY, maxX, maxY)
}

func unitColour(c color.NRGBA) [4]float32 {
	return [4]float32{float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255, float32(c.A) / 255}
}

// over lays colour c at alpha a over the non-premultiplied pixel px.
func over(px []uint8, c [3]float32, a float32) {
	da := float32(px[3]) / 255
	oa := a + da*(1-a)
	if oa <= 0 {
		return
	}
	keep := da * (1 - a) / oa
	put := a / oa
	px[0] = channel(c[0]*put + float32(px[0])/255*keep)
	px[1] = channel(c[1]*put + float32(px[1])/255*keep)
	px[2] = channel(c[2]*put + float32(px[2])/255*keep)
	px[3] = channel(oa)
}
