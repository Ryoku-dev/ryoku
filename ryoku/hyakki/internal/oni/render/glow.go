package render

import (
	"image"
	"image/color"
	"math"

	"ryoku-rashin/internal/oni/motion"
)

// accentAt is the accent a lap shows: the From colour hands over to the To
// colour as the lap goes round, so each side weighs in by its share.
type accentAt struct {
	from, to       color.NRGBA
	fromOn, toOn   bool
	progress, glow float64
}

func lapAccent(l motion.Lap, look Look) (accentAt, bool) {
	a := accentAt{progress: min(max(l.Progress, 0), 1), glow: min(max(l.Glow, 0), 1)}
	a.from, a.fromOn = look.accent(l.From)
	a.to, a.toOn = look.accent(l.To)
	if a.glow <= 0 || (!a.fromOn && !a.toOn) {
		return a, false
	}
	if a.weight() <= 1e-3 {
		return a, false
	}
	return a, true
}

func (a accentAt) weight() float64 {
	w := 0.0
	if a.fromOn {
		w += 1 - a.progress
	}
	if a.toOn {
		w += a.progress
	}
	return w
}

// mixed is the glow colour, the two sides blended by their share.
func (a accentAt) mixed() (r, g, b float32) {
	wf, wt := 0.0, 0.0
	if a.fromOn {
		wf = 1 - a.progress
	}
	if a.toOn {
		wt = a.progress
	}
	t := float32(wt / (wf + wt))
	mix := func(x, y uint8) float32 { return (float32(x) + (float32(y)-float32(x))*t) / 255 }
	return mix(a.from.R, a.to.R), mix(a.from.G, a.to.G), mix(a.from.B, a.to.B)
}

// drawAccent paints the accent under the character in main into out: a soft
// glow spread from the silhouette and a rim around it whose colour laps
// round from From to To, brightest at the lap's head.
func (d *drawer) drawAccent(out, main *canvas, a accentAt, spin float64, centre point, k float64, glowSpread, outlinePx, core, headWidth float64) {
	radius := max(1, int(math.Round(glowSpread*k/2)))
	rimRadius := max(1, int(math.Round(outlinePx*k)))
	// three box passes reach 3r; nothing of the accent lies further out
	rect := alphaBounds(main).Inset(-(3*radius + rimRadius + 1)).Intersect(main.rect)
	if rect.Empty() {
		return
	}
	alpha := d.plane(rect)
	w := rect.Dx()
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		row := alpha.row(y, rect.Min.X, rect.Max.X)
		o := main.offset(rect.Min.X, y) + 3
		for x := range row {
			row[x] = main.pix[o+x*4]
		}
	}
	halo := d.plane(rect)
	copy(halo.pix, alpha.pix)
	for range 3 {
		boxBlur(halo, radius, d)
	}
	rim := d.plane(rect)
	copy(rim.pix, alpha.pix)
	boxBlur(rim, rimRadius, d)

	gr, gg, gb := a.mixed()
	glowA := float32(a.glow * a.weight())
	rimGain := float32(min(1, a.glow*2))
	head := spin + a.progress*2*math.Pi
	if a.progress >= 1 {
		head = spin
	}
	halfWidth := max(headWidth*math.Pi, 1e-3)
	for i, base := range alpha.pix {
		outside := 1 - base
		hv := min(halo.pix[i]*1.6, 1) * glowA * outside
		var px premul
		if hv > 0 {
			px = premul{gr * hv, gg * hv, gb * hv, hv}
		}
		x, y := rect.Min.X+i%w, rect.Min.Y+i/w
		if rv := min(rim.pix[i]*4, 1) * outside; rv > 0.002 {
			theta := math.Atan2(float64(y)+0.5-centre.Y, float64(x)+0.5-centre.X)
			u := math.Mod(theta-spin, 2*math.Pi)
			if u < 0 {
				u += 2 * math.Pi
			}
			side, on := a.from, a.fromOn
			if u/(2*math.Pi) < a.progress || a.progress >= 1 {
				side, on = a.to, a.toOn
			}
			if on {
				dh := math.Abs(math.Remainder(theta-head, 2*math.Pi))
				bump := 0.0
				if dh < halfWidth {
					bump = 0.5 + 0.5*math.Cos(math.Pi*dh/halfWidth)
				}
				v := rv * rimGain * float32(core+(1-core)*bump)
				blend(px[:], premul{float32(side.R) / 255 * v, float32(side.G) / 255 * v, float32(side.B) / 255 * v, v}, 1)
			}
		}
		o := out.offset(x, y)
		out.pix[o], out.pix[o+1], out.pix[o+2], out.pix[o+3] = px[0], px[1], px[2], px[3]
	}
	d.mdepth -= 3
}

// alphaBounds is the smallest rectangle holding every painted pixel of cv.
func alphaBounds(cv *canvas) image.Rectangle {
	r := image.Rectangle{}
	w := cv.rect.Dx()
	for y := cv.rect.Min.Y; y < cv.rect.Max.Y; y++ {
		row := cv.pix[(y-cv.rect.Min.Y)*w*4 : (y-cv.rect.Min.Y+1)*w*4]
		first, last := -1, -1
		for x := range w {
			if row[x*4+3] > 0 {
				if first < 0 {
					first = x
				}
				last = x
			}
		}
		if first >= 0 {
			r = r.Union(image.Rect(cv.rect.Min.X+first, y, cv.rect.Min.X+last+1, y+1))
		}
	}
	return r
}

// boxBlur blurs m in place with a (2r+1)-wide box, horizontally then
// vertically; outside m counts as zero.
func boxBlur(m *mask, r int, d *drawer) {
	w, h := m.rect.Dx(), m.rect.Dy()
	if w == 0 || h == 0 {
		return
	}
	tmp := d.plane(m.rect)
	inv := 1 / float32(2*r+1)
	line := func(src, dst []float32, n, stride int) {
		var sum float32
		for i := 0; i <= r && i < n; i++ {
			sum += src[i*stride]
		}
		for i := range n {
			dst[i*stride] = sum * inv
			if j := i + r + 1; j < n {
				sum += src[j*stride]
			}
			if j := i - r; j >= 0 {
				sum -= src[j*stride]
			}
		}
	}
	for y := range h {
		line(m.pix[y*w:], tmp.pix[y*w:], w, 1)
	}
	for x := range w {
		line(tmp.pix[x:], m.pix[x:], h, w)
	}
	d.mdepth--
}
