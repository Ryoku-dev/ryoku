package render

import "math"

type segOp uint8

const (
	opMove segOp = iota
	opLine
	opQuad
	opCubic
	opClose
)

type seg struct {
	op  segOp
	pts [3]point
}

type path []seg

// contour is a flattened subpath.
type contour struct {
	pts    []point
	closed bool
}

func (p path) transform(m affine) path {
	out := make(path, len(p))
	for i, s := range p {
		out[i].op = s.op
		for j := range s.pts {
			out[i].pts[j] = m.apply(s.pts[j])
		}
	}
	return out
}

// flatten turns p into polylines no further than tol from the curves.
func (p path) flatten(tol float64) []contour {
	var out []contour
	var cur contour
	var pen, start point
	flush := func() {
		if len(cur.pts) > 0 {
			out = append(out, cur)
		}
		cur = contour{}
	}
	for _, s := range p {
		switch s.op {
		case opMove:
			flush()
			pen, start = s.pts[0], s.pts[0]
			cur.pts = append(cur.pts, pen)
		case opLine:
			if len(cur.pts) == 0 {
				cur.pts = append(cur.pts, pen)
			}
			pen = s.pts[0]
			cur.pts = append(cur.pts, pen)
		case opQuad:
			if len(cur.pts) == 0 {
				cur.pts = append(cur.pts, pen)
			}
			c, end := s.pts[0], s.pts[1]
			d := math.Hypot(pen.X-2*c.X+end.X, pen.Y-2*c.Y+end.Y)
			n := segments(math.Sqrt(d / (4 * tol)))
			for i := 1; i <= n; i++ {
				t := float64(i) / float64(n)
				u := 1 - t
				cur.pts = append(cur.pts, point{u*u*pen.X + 2*u*t*c.X + t*t*end.X, u*u*pen.Y + 2*u*t*c.Y + t*t*end.Y})
			}
			pen = end
		case opCubic:
			if len(cur.pts) == 0 {
				cur.pts = append(cur.pts, pen)
			}
			c1, c2, end := s.pts[0], s.pts[1], s.pts[2]
			d := max(math.Hypot(pen.X-2*c1.X+c2.X, pen.Y-2*c1.Y+c2.Y), math.Hypot(c1.X-2*c2.X+end.X, c1.Y-2*c2.Y+end.Y))
			n := segments(math.Sqrt(0.75 * d / tol))
			for i := 1; i <= n; i++ {
				t := float64(i) / float64(n)
				u := 1 - t
				a, b, cc, dd := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
				cur.pts = append(cur.pts, point{a*pen.X + b*c1.X + cc*c2.X + dd*end.X, a*pen.Y + b*c1.Y + cc*c2.Y + dd*end.Y})
			}
			pen = end
		case opClose:
			if len(cur.pts) > 0 {
				cur.closed = true
				flush()
			}
			pen = start
		}
	}
	flush()
	return out
}

func segments(n float64) int {
	if math.IsNaN(n) || n < 1 {
		return 1
	}
	return int(min(math.Ceil(n), 512))
}

type strokeStyle struct {
	width      float64
	join       string
	cap        string
	miterLimit float64
	dashes     []float64
	dashOffset float64
}

// strokePieces outlines contours as polygons whose nonzero union is the
// stroke: one quad per segment plus join and cap pieces, all wound alike.
func strokePieces(cs []contour, st strokeStyle, tol float64) [][]point {
	if st.width <= 0 {
		return nil
	}
	if len(st.dashes) > 0 {
		cs = dash(cs, st.dashes, st.dashOffset)
	}
	hw := st.width / 2
	var out [][]point
	add := func(poly []point) {
		if len(poly) >= 3 {
			out = append(out, oriented(poly))
		}
	}
	for _, c := range cs {
		pts := dedupe(c.pts, c.closed)
		if len(pts) == 1 {
			switch st.cap {
			case "round":
				add(circle(pts[0], hw, tol))
			case "square":
				p := pts[0]
				add([]point{{p.X - hw, p.Y - hw}, {p.X + hw, p.Y - hw}, {p.X + hw, p.Y + hw}, {p.X - hw, p.Y + hw}})
			}
			continue
		}
		n := len(pts)
		segs := n - 1
		if c.closed {
			segs = n
		}
		for i := range segs {
			a, b := pts[i], pts[(i+1)%n]
			nx, ny := normal(a, b, hw)
			add([]point{{a.X + nx, a.Y + ny}, {b.X + nx, b.Y + ny}, {b.X - nx, b.Y - ny}, {a.X - nx, a.Y - ny}})
		}
		for i := range n {
			if !c.closed && (i == 0 || i == n-1) {
				continue
			}
			prev, v, next := pts[(i-1+n)%n], pts[i], pts[(i+1)%n]
			for _, poly := range join(prev, v, next, hw, st, tol) {
				add(poly)
			}
		}
		if !c.closed {
			add(capPiece(pts[1], pts[0], hw, st.cap, tol))
			add(capPiece(pts[n-2], pts[n-1], hw, st.cap, tol))
		}
	}
	return out
}

func dedupe(pts []point, closed bool) []point {
	out := make([]point, 0, len(pts))
	for _, p := range pts {
		if len(out) == 0 || out[len(out)-1] != p {
			out = append(out, p)
		}
	}
	if closed && len(out) > 1 && out[0] == out[len(out)-1] {
		out = out[:len(out)-1]
	}
	return out
}

func normal(a, b point, hw float64) (float64, float64) {
	dx, dy := b.X-a.X, b.Y-a.Y
	l := math.Hypot(dx, dy)
	if l == 0 {
		return 0, 0
	}
	return -dy / l * hw, dx / l * hw
}

func join(prev, v, next point, hw float64, st strokeStyle, tol float64) [][]point {
	if st.join == "round" {
		return [][]point{circle(v, hw, tol)}
	}
	n1x, n1y := normal(prev, v, hw)
	n2x, n2y := normal(v, next, hw)
	cross := (v.X-prev.X)*(next.Y-v.Y) - (v.Y-prev.Y)*(next.X-v.X)
	if cross == 0 {
		return nil
	}
	// the outer side of the turn is opposite the turn direction
	if cross > 0 {
		n1x, n1y, n2x, n2y = -n1x, -n1y, -n2x, -n2y
	}
	o1 := point{v.X + n1x, v.Y + n1y}
	o2 := point{v.X + n2x, v.Y + n2y}
	if st.join == "miter" || st.join == "miter-clip" || st.join == "arcs" {
		// miter length / stroke width = 1/sin(theta/2)
		d1x, d1y := v.X-prev.X, v.Y-prev.Y
		d2x, d2y := next.X-v.X, next.Y-v.Y
		cosTurn := (d1x*d2x + d1y*d2y) / (math.Hypot(d1x, d1y) * math.Hypot(d2x, d2y))
		theta := math.Pi - math.Acos(min(max(cosTurn, -1), 1))
		if s := math.Sin(theta / 2); s > 0 && 1/s <= st.miterLimit {
			mx, my := (n1x+n2x)/2, (n1y+n2y)/2
			ml := math.Hypot(mx, my)
			if ml > 0 {
				k := hw / s / ml
				tip := point{v.X + mx*k, v.Y + my*k}
				return [][]point{{v, o1, tip, o2}}
			}
		}
	}
	return [][]point{{v, o1, o2}}
}

func capPiece(from, end point, hw float64, kind string, tol float64) []point {
	switch kind {
	case "round":
		return circle(end, hw, tol)
	case "square":
		dx, dy := end.X-from.X, end.Y-from.Y
		l := math.Hypot(dx, dy)
		if l == 0 {
			return nil
		}
		ux, uy := dx/l*hw, dy/l*hw
		nx, ny := -uy, ux
		return []point{{end.X + nx, end.Y + ny}, {end.X + nx + ux, end.Y + ny + uy}, {end.X - nx + ux, end.Y - ny + uy}, {end.X - nx, end.Y - ny}}
	}
	return nil
}

func circle(c point, r, tol float64) []point {
	n := 8
	if r > tol {
		n = max(n, int(math.Ceil(math.Pi/math.Acos(1-tol/r))))
	}
	n = min(n, 256)
	out := make([]point, n)
	for i := range n {
		s, co := math.Sincos(2 * math.Pi * float64(i) / float64(n))
		out[i] = point{c.X + r*co, c.Y + r*s}
	}
	return out
}

func area(poly []point) float64 {
	var a float64
	for i, p := range poly {
		q := poly[(i+1)%len(poly)]
		a += p.X*q.Y - q.X*p.Y
	}
	return a / 2
}

func oriented(poly []point) []point {
	if area(poly) < 0 {
		for i, j := 0, len(poly)-1; i < j; i, j = i+1, j-1 {
			poly[i], poly[j] = poly[j], poly[i]
		}
	}
	return poly
}

// dash cuts contours into the visible dashes of pattern, starting offset in.
func dash(cs []contour, pattern []float64, offset float64) []contour {
	total := 0.0
	for _, d := range pattern {
		total += d
	}
	if total <= 0 {
		return cs
	}
	var out []contour
	for _, c := range cs {
		pts := c.pts
		if c.closed && len(pts) > 0 {
			pts = append(append([]point(nil), pts...), pts[0])
		}
		idx := 0
		pos := math.Mod(offset, total)
		if pos < 0 {
			pos += total
		}
		for pos >= pattern[idx] {
			pos -= pattern[idx]
			idx = (idx + 1) % len(pattern)
		}
		left := pattern[idx] - pos
		on := idx%2 == 0
		var cur []point
		if on && len(pts) > 0 {
			cur = []point{pts[0]}
		}
		for i := 1; i < len(pts); i++ {
			a, b := pts[i-1], pts[i]
			l := math.Hypot(b.X-a.X, b.Y-a.Y)
			t := 0.0
			for l-t > left {
				t += left
				p := point{a.X + (b.X-a.X)*t/l, a.Y + (b.Y-a.Y)*t/l}
				if on {
					cur = append(cur, p)
					out = append(out, contour{pts: cur})
					cur = nil
				} else {
					cur = []point{p}
				}
				on = !on
				idx = (idx + 1) % len(pattern)
				left = pattern[idx]
			}
			left -= l - t
			if on {
				cur = append(cur, b)
			}
		}
		if on && len(cur) > 1 {
			out = append(out, contour{pts: cur})
		}
	}
	return out
}
