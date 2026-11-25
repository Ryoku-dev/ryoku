package render

import (
	"image"
	"math"
	"slices"
)

// subrows is the vertical supersampling of the scanline rasteriser; the
// horizontal coverage of each subrow is exact.
const subrows = 16

type point struct{ X, Y float64 }

// affine maps (x, y) to (A*x + C*y + E, B*x + D*y + F), SVG's matrix order.
type affine struct{ A, B, C, D, E, F float64 }

var identity = affine{A: 1, D: 1}

func (m affine) apply(p point) point {
	return point{m.A*p.X + m.C*p.Y + m.E, m.B*p.X + m.D*p.Y + m.F}
}

// mul is m after n: (m.mul(n)).apply(p) == m.apply(n.apply(p)).
func (m affine) mul(n affine) affine {
	return affine{
		A: m.A*n.A + m.C*n.B,
		B: m.B*n.A + m.D*n.B,
		C: m.A*n.C + m.C*n.D,
		D: m.B*n.C + m.D*n.D,
		E: m.A*n.E + m.C*n.F + m.E,
		F: m.B*n.E + m.D*n.F + m.F,
	}
}

func (m affine) det() float64 { return m.A*m.D - m.B*m.C }

func (m affine) invert() (affine, bool) {
	d := m.det()
	if d == 0 || math.IsNaN(d) || math.IsInf(d, 0) {
		return affine{}, false
	}
	return affine{
		A: m.D / d, B: -m.B / d, C: -m.C / d, D: m.A / d,
		E: (m.C*m.F - m.D*m.E) / d,
		F: (m.B*m.E - m.A*m.F) / d,
	}, true
}

// scale is the largest stretch m applies to any length.
func (m affine) scale() float64 {
	a := math.Hypot(m.A, m.B)
	b := math.Hypot(m.C, m.D)
	return max(a, b)
}

func translate(x, y float64) affine { return affine{A: 1, D: 1, E: x, F: y} }
func scaling(x, y float64) affine   { return affine{A: x, D: y} }
func rotation(rad float64) affine {
	s, c := math.Sincos(rad)
	return affine{A: c, B: s, C: -s, D: c}
}

type edge struct {
	x0, y0, y1, dxdy float64
	dir              int
}

// rasterizer turns closed polygons into a coverage mask under the nonzero or
// even-odd rule. Its buffers are reused across draws.
type rasterizer struct {
	edges   []edge
	active  []int
	cross   []crossing
	frac    []float32
	step    []float32
	minY    float64
	maxY    float64
	minX    float64
	maxX    float64
	touched bool
}

type crossing struct {
	x   float64
	dir int
}

func (r *rasterizer) reset() {
	r.edges = r.edges[:0]
	r.touched = false
}

// add appends polygon pts, mapped by m, as one closed contour.
func (r *rasterizer) add(pts []point, m affine) {
	if len(pts) < 2 {
		return
	}
	prev := m.apply(pts[len(pts)-1])
	for _, p := range pts {
		q := m.apply(p)
		r.addEdge(prev, q)
		prev = q
	}
}

func (r *rasterizer) addEdge(a, b point) {
	if a.Y == b.Y || math.IsNaN(a.X+a.Y+b.X+b.Y) {
		return
	}
	dir := 1
	if a.Y > b.Y {
		a, b = b, a
		dir = -1
	}
	if !r.touched {
		r.minY, r.maxY, r.minX, r.maxX = a.Y, b.Y, min(a.X, b.X), max(a.X, b.X)
		r.touched = true
	} else {
		r.minY, r.maxY = min(r.minY, a.Y), max(r.maxY, b.Y)
		r.minX, r.maxX = min(r.minX, a.X, b.X), max(r.maxX, a.X, b.X)
	}
	r.edges = append(r.edges, edge{x0: a.X, y0: a.Y, y1: b.Y, dxdy: (b.X - a.X) / (b.Y - a.Y), dir: dir})
}

// cover writes the coverage of everything added so far into m, which it
// clears inside clip first, and returns the rectangle it may have touched.
func (r *rasterizer) cover(m *mask, evenOdd bool) image.Rectangle {
	if !r.touched {
		return image.Rectangle{}
	}
	bounds := image.Rect(int(math.Floor(r.minX)), int(math.Floor(r.minY)), int(math.Ceil(r.maxX))+1, int(math.Ceil(r.maxY))+1).Intersect(m.rect)
	if bounds.Empty() {
		return image.Rectangle{}
	}
	m.clear(bounds)
	slices.SortFunc(r.edges, func(a, b edge) int {
		switch {
		case a.y0 < b.y0:
			return -1
		case a.y0 > b.y0:
			return 1
		}
		return 0
	})
	width := bounds.Dx()
	if cap(r.frac) < width+1 {
		r.frac = make([]float32, width+1)
		r.step = make([]float32, width+1)
	}
	frac, step := r.frac[:width+1], r.step[:width+1]
	r.active = r.active[:0]
	next := 0
	const w = 1.0 / subrows
	left := float64(bounds.Min.X)
	for py := bounds.Min.Y; py < bounds.Max.Y; py++ {
		clear(frac)
		clear(step)
		any := false
		for s := range subrows {
			sy := float64(py) + (float64(s)+0.5)/subrows
			for next < len(r.edges) && r.edges[next].y0 <= sy {
				r.active = append(r.active, next)
				next++
			}
			r.cross = r.cross[:0]
			kept := r.active[:0]
			for _, i := range r.active {
				e := &r.edges[i]
				if e.y1 <= sy {
					continue
				}
				kept = append(kept, i)
				r.cross = append(r.cross, crossing{x: e.x0 + (sy-e.y0)*e.dxdy, dir: e.dir})
			}
			r.active = kept
			if len(r.cross) < 2 {
				continue
			}
			sortCrossings(r.cross)
			winding := 0
			for i, c := range r.cross[:len(r.cross)-1] {
				winding += c.dir
				inside := winding != 0
				if evenOdd {
					inside = winding&1 != 0
				}
				if !inside {
					continue
				}
				a := min(max(c.x-left, 0), float64(width))
				b := min(max(r.cross[i+1].x-left, 0), float64(width))
				if b <= a {
					continue
				}
				ia, ib := int(a), int(b)
				frac[ia] += float32(w * (float64(ia+1) - a))
				step[ia] += w
				frac[ib] -= float32(w * (float64(ib+1) - b))
				step[ib] -= w
				any = true
			}
		}
		if !any {
			continue
		}
		row := m.row(py, bounds.Min.X, bounds.Max.X)
		var run float32
		for x := range width {
			v := frac[x] + run
			run += step[x]
			row[x] = min(max(v, 0), 1)
		}
	}
	return bounds
}

func sortCrossings(c []crossing) {
	slices.SortFunc(c, func(a, b crossing) int {
		switch {
		case a.x < b.x:
			return -1
		case a.x > b.x:
			return 1
		}
		return 0
	})
}

// mask is a coverage plane over rect, one float per pixel.
type mask struct {
	rect image.Rectangle
	pix  []float32
}

func (m *mask) setRect(r image.Rectangle) {
	m.rect = r
	n := r.Dx() * r.Dy()
	if cap(m.pix) < n {
		m.pix = make([]float32, n)
	}
	m.pix = m.pix[:n]
}

func (m *mask) row(y, x0, x1 int) []float32 {
	w := m.rect.Dx()
	i := (y-m.rect.Min.Y)*w + x0 - m.rect.Min.X
	return m.pix[i : i+x1-x0]
}

func (m *mask) clear(r image.Rectangle) {
	r = r.Intersect(m.rect)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		clear(m.row(y, r.Min.X, r.Max.X))
	}
}

func (m *mask) at(x, y int) float32 {
	return m.pix[(y-m.rect.Min.Y)*m.rect.Dx()+x-m.rect.Min.X]
}
