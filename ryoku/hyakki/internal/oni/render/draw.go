package render

import (
	"image"
	"image/color"
	"math"
)

// canvas is premultiplied RGBA in floats over rect.
type canvas struct {
	rect image.Rectangle
	pix  []float32
}

func (cv *canvas) setRect(r image.Rectangle) {
	cv.rect = r
	n := r.Dx() * r.Dy() * 4
	if cap(cv.pix) < n {
		cv.pix = make([]float32, n)
	}
	cv.pix = cv.pix[:n]
	clear(cv.pix)
}

func (cv *canvas) offset(x, y int) int {
	return ((y-cv.rect.Min.Y)*cv.rect.Dx() + x - cv.rect.Min.X) * 4
}

type premul [4]float32

// drawer holds the scratch one Draw reuses.
type drawer struct {
	look   Look
	bright float64
	ras    rasterizer
	cov    mask
	layers []*canvas
	masks  []*mask
	depth  int
	mdepth int
	lut    [256]premul
}

func (d *drawer) layer(r image.Rectangle) *canvas {
	if d.depth == len(d.layers) {
		d.layers = append(d.layers, &canvas{})
	}
	l := d.layers[d.depth]
	d.depth++
	l.setRect(r)
	return l
}

func (d *drawer) plane(r image.Rectangle) *mask {
	if d.mdepth == len(d.masks) {
		d.masks = append(d.masks, &mask{})
	}
	m := d.masks[d.mdepth]
	d.mdepth++
	m.setRect(r)
	clear(m.pix)
	return m
}

func (d *drawer) resolve(c colour) rgba {
	var role color.NRGBA
	switch c.role {
	case roleInk:
		role = d.look.Ink
	case roleWhite:
		role = d.look.White
	case roleBody:
		role = d.look.Body
	default:
		return c.c
	}
	return rgba{float64(role.R) / 255, float64(role.G) / 255, float64(role.B) / 255, c.c.A * float64(role.A) / 255}
}

func toPremul(c rgba, opacity float64) premul {
	a := c.A * opacity
	return premul{float32(c.R * a), float32(c.G * a), float32(c.B * a), float32(a)}
}

func (d *drawer) node(n *node, dst *canvas, m affine) {
	layered := n.opacity < 1 || n.clip != nil || n.mask != nil
	target := dst
	if layered {
		target = d.layer(dst.rect)
	}
	if n.fillP != nil && len(n.fill) > 0 {
		d.fill(target, n.fill, n.evenOdd, n.fillP, m)
	}
	if n.strokeP != nil && len(n.stroke) > 0 {
		d.fill(target, n.stroke, false, n.strokeP, m)
	}
	for _, k := range n.children {
		d.node(k, target, m)
	}
	if !layered {
		return
	}
	factor := d.plane(dst.rect)
	for i := range factor.pix {
		factor.pix[i] = float32(n.opacity)
	}
	if n.clip != nil {
		d.applyClip(factor, n.clip, m)
	}
	if n.mask != nil {
		d.applyMask(factor, n.mask, m)
	}
	composite(dst, target, factor)
	d.mdepth--
	d.depth--
}

func (d *drawer) fill(dst *canvas, polys [][]point, evenOdd bool, p *paint, m affine) {
	d.ras.reset()
	for _, poly := range polys {
		d.ras.add(poly, m)
	}
	d.cov.setRect(dst.rect)
	r := d.ras.cover(&d.cov, evenOdd)
	if r.Empty() {
		return
	}
	if p.grad == nil {
		src := toPremul(d.resolve(p.solid), p.opacity)
		if src[3] == 0 {
			return
		}
		for y := r.Min.Y; y < r.Max.Y; y++ {
			row := d.cov.row(y, r.Min.X, r.Max.X)
			o := dst.offset(r.Min.X, y)
			for _, c := range row {
				if c > 0 {
					blend(dst.pix[o:o+4], src, c)
				}
				o += 4
			}
		}
		return
	}
	inv, ok := m.mul(p.m).invert()
	if !ok {
		return
	}
	d.buildLUT(p)
	g := p.grad
	for y := r.Min.Y; y < r.Max.Y; y++ {
		row := d.cov.row(y, r.Min.X, r.Max.X)
		o := dst.offset(r.Min.X, y)
		for i, c := range row {
			if c > 0 {
				q := inv.apply(point{float64(r.Min.X+i) + 0.5, float64(y) + 0.5})
				blend(dst.pix[o:o+4], d.lut[lutIndex(g.offset(q), g.spread)], c)
			}
			o += 4
		}
	}
}

func blend(dst []float32, src premul, c float32) {
	inv := 1 - src[3]*c
	dst[0] = src[0]*c + dst[0]*inv
	dst[1] = src[1]*c + dst[1]*inv
	dst[2] = src[2]*c + dst[2]*inv
	dst[3] = src[3]*c + dst[3]*inv
}

// composite draws src over dst scaled per pixel by factor.
func composite(dst, src *canvas, factor *mask) {
	for i, f := range factor.pix {
		if f <= 0 {
			continue
		}
		o := i * 4
		s := src.pix[o : o+4]
		if s[3] == 0 {
			continue
		}
		blend(dst.pix[o:o+4], premul{s[0], s[1], s[2], s[3]}, f)
	}
}

func (d *drawer) applyClip(factor *mask, cs *clipShape, m affine) {
	cov := d.plane(factor.rect)
	part := d.plane(factor.rect)
	for _, p := range cs.parts {
		d.ras.reset()
		for _, poly := range p.polys {
			d.ras.add(poly, m)
		}
		r := d.ras.cover(part, p.evenOdd)
		for y := r.Min.Y; y < r.Max.Y; y++ {
			src := part.row(y, r.Min.X, r.Max.X)
			dst := cov.row(y, r.Min.X, r.Max.X)
			for i, v := range src {
				dst[i] = max(dst[i], v)
			}
		}
	}
	for i := range factor.pix {
		factor.pix[i] *= cov.pix[i]
	}
	d.mdepth -= 2
	if cs.clip != nil {
		d.applyClip(factor, cs.clip, m)
	}
}

func (d *drawer) applyMask(factor *mask, mr *maskRef, m affine) {
	if mr.content == nil || len(mr.region) == 0 {
		clear(factor.pix)
		return
	}
	content := d.layer(factor.rect)
	d.node(mr.content, content, m)
	region := d.plane(factor.rect)
	d.ras.reset()
	d.ras.add(mr.region, m)
	d.ras.cover(region, false)
	for i := range factor.pix {
		o := i * 4
		v := content.pix[o+3]
		if !mr.alpha {
			v = 0.2125*content.pix[o] + 0.7154*content.pix[o+1] + 0.0721*content.pix[o+2]
		}
		factor.pix[i] *= v * region.pix[i]
	}
	d.mdepth--
	d.depth--
}

func (d *drawer) buildLUT(p *paint) {
	stops := p.grad.stops
	cols := make([]premul, len(stops))
	for i, s := range stops {
		cols[i] = toPremul(d.resolve(s.c), p.opacity)
	}
	for i := range d.lut {
		t := float64(i) / 255
		switch {
		case t <= stops[0].off:
			d.lut[i] = cols[0]
		case t >= stops[len(stops)-1].off:
			d.lut[i] = cols[len(cols)-1]
		default:
			j := 1
			for stops[j].off < t {
				j++
			}
			a, b := stops[j-1], stops[j]
			f := float32(0)
			if b.off > a.off {
				f = float32((t - a.off) / (b.off - a.off))
			}
			for ch := range 4 {
				d.lut[i][ch] = cols[j-1][ch] + (cols[j][ch]-cols[j-1][ch])*f
			}
		}
	}
}

func lutIndex(t float64, spread int) int {
	switch spread {
	case spreadRepeat:
		t -= math.Floor(t)
	case spreadReflect:
		t = math.Mod(math.Abs(t), 2)
		if t > 1 {
			t = 2 - t
		}
	}
	return int(min(max(t, 0), 1)*255 + 0.5)
}

// offset is where q falls along the gradient, 0 at its start.
func (g *gradient) offset(q point) float64 {
	if g.linear {
		dx, dy := g.x2-g.x1, g.y2-g.y1
		return ((q.X-g.x1)*dx + (q.Y-g.y1)*dy) / (dx*dx + dy*dy)
	}
	// two-point conical: q lies on the circle centred f + t(c-f) with
	// radius fr + t(r-fr); take the largest t with a non-negative radius
	cdx, cdy := g.cx-g.fx, g.cy-g.fy
	dr := g.r - g.fr
	px, py := q.X-g.fx, q.Y-g.fy
	a := cdx*cdx + cdy*cdy - dr*dr
	b := px*cdx + py*cdy + g.fr*dr
	c := px*px + py*py - g.fr*g.fr
	if math.Abs(a) < 1e-12 {
		if b == 0 {
			return 0
		}
		return c / (2 * b)
	}
	disc := b*b - a*c
	if disc < 0 {
		return 0
	}
	s := math.Sqrt(disc)
	t1, t2 := (b+s)/a, (b-s)/a
	t := max(t1, t2)
	if g.fr+t*dr < 0 {
		t = min(t1, t2)
	}
	return t
}
