package render

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// scanner reads SVG's number lists: separators are optional whitespace and
// one comma, and "1.5.5" or "1-2" hold two numbers.
type scanner struct {
	s  string
	at int
}

func (sc *scanner) skip() {
	for sc.at < len(sc.s) {
		switch sc.s[sc.at] {
		case ' ', '\t', '\n', '\r', '\f':
			sc.at++
		default:
			return
		}
	}
}

func (sc *scanner) sep() {
	sc.skip()
	if sc.at < len(sc.s) && sc.s[sc.at] == ',' {
		sc.at++
		sc.skip()
	}
}

func (sc *scanner) done() bool { sc.skip(); return sc.at >= len(sc.s) }

func (sc *scanner) number() (float64, bool) {
	sc.skip()
	start := sc.at
	i := sc.at
	if i < len(sc.s) && (sc.s[i] == '+' || sc.s[i] == '-') {
		i++
	}
	digits, dot := false, false
	for i < len(sc.s) {
		c := sc.s[i]
		switch {
		case c >= '0' && c <= '9':
			digits = true
		case c == '.' && !dot:
			dot = true
		default:
			goto exp
		}
		i++
	}
exp:
	if !digits {
		return 0, false
	}
	if i < len(sc.s) && (sc.s[i] == 'e' || sc.s[i] == 'E') {
		j := i + 1
		if j < len(sc.s) && (sc.s[j] == '+' || sc.s[j] == '-') {
			j++
		}
		k := j
		for k < len(sc.s) && sc.s[k] >= '0' && sc.s[k] <= '9' {
			k++
		}
		if k > j {
			i = k
		}
	}
	v, err := strconv.ParseFloat(sc.s[start:i], 64)
	if err != nil || math.IsInf(v, 0) {
		return 0, false
	}
	sc.at = i
	sc.sep()
	return v, true
}

// flag reads an arc flag, which may touch the next number ("011").
func (sc *scanner) flag() (bool, bool) {
	sc.skip()
	if sc.at >= len(sc.s) {
		return false, false
	}
	c := sc.s[sc.at]
	if c != '0' && c != '1' {
		return false, false
	}
	sc.at++
	sc.sep()
	return c == '1', true
}

func numbers(s string) ([]float64, bool) {
	sc := scanner{s: s}
	var out []float64
	for !sc.done() {
		v, ok := sc.number()
		if !ok {
			return out, false
		}
		out = append(out, v)
	}
	return out, true
}

var errPathData = errors.New("bad path data")

// parsePath reads path data into absolute segments. On an error it returns
// what came before it, which SVG still draws.
func parsePath(d string) (path, error) {
	sc := scanner{s: d}
	var p path
	var pen, start, ctrl point
	var last byte
	for !sc.done() {
		c := sc.s[sc.at]
		cmd := last
		if strings.IndexByte("MmLlHhVvCcSsQqTtAaZz", c) >= 0 {
			cmd = c
			sc.at++
		} else if last == 0 || last == 'Z' || last == 'z' {
			return p, errPathData
		}
		if len(p) == 0 && cmd != 'M' && cmd != 'm' {
			return p, errPathData
		}
		rel := cmd >= 'a'
		off := func(q point) point {
			if rel {
				return point{q.X + pen.X, q.Y + pen.Y}
			}
			return q
		}
		pt := func() (point, bool) {
			x, ok := sc.number()
			if !ok {
				return point{}, false
			}
			y, ok := sc.number()
			return off(point{x, y}), ok
		}
		prevCtrl := ctrl
		ctrl = point{math.NaN(), 0}
		switch cmd {
		case 'M', 'm':
			q, ok := pt()
			if !ok {
				return p, errPathData
			}
			p = append(p, seg{op: opMove, pts: [3]point{q}})
			pen, start = q, q
			// later pairs are implicit lines
			if rel {
				last = 'l'
			} else {
				last = 'L'
			}
			continue
		case 'L', 'l':
			q, ok := pt()
			if !ok {
				return p, errPathData
			}
			p = append(p, seg{op: opLine, pts: [3]point{q}})
			pen = q
		case 'H', 'h':
			x, ok := sc.number()
			if !ok {
				return p, errPathData
			}
			if rel {
				x += pen.X
			}
			pen = point{x, pen.Y}
			p = append(p, seg{op: opLine, pts: [3]point{pen}})
		case 'V', 'v':
			y, ok := sc.number()
			if !ok {
				return p, errPathData
			}
			if rel {
				y += pen.Y
			}
			pen = point{pen.X, y}
			p = append(p, seg{op: opLine, pts: [3]point{pen}})
		case 'C', 'c':
			c1, ok1 := pt()
			c2, ok2 := pt()
			e, ok3 := pt()
			if !ok1 || !ok2 || !ok3 {
				return p, errPathData
			}
			p = append(p, seg{op: opCubic, pts: [3]point{c1, c2, e}})
			pen, ctrl = e, c2
		case 'S', 's':
			c2, ok1 := pt()
			e, ok2 := pt()
			if !ok1 || !ok2 {
				return p, errPathData
			}
			c1 := pen
			if !math.IsNaN(prevCtrl.X) && strings.IndexByte("CcSs", last) >= 0 {
				c1 = point{2*pen.X - prevCtrl.X, 2*pen.Y - prevCtrl.Y}
			}
			p = append(p, seg{op: opCubic, pts: [3]point{c1, c2, e}})
			pen, ctrl = e, c2
		case 'Q', 'q':
			c1, ok1 := pt()
			e, ok2 := pt()
			if !ok1 || !ok2 {
				return p, errPathData
			}
			p = append(p, seg{op: opQuad, pts: [3]point{c1, e}})
			pen, ctrl = e, c1
		case 'T', 't':
			e, ok := pt()
			if !ok {
				return p, errPathData
			}
			c1 := pen
			if !math.IsNaN(prevCtrl.X) && strings.IndexByte("QqTt", last) >= 0 {
				c1 = point{2*pen.X - prevCtrl.X, 2*pen.Y - prevCtrl.Y}
			}
			p = append(p, seg{op: opQuad, pts: [3]point{c1, e}})
			pen, ctrl = e, c1
		case 'A', 'a':
			rx, ok1 := sc.number()
			ry, ok2 := sc.number()
			rot, ok3 := sc.number()
			large, ok4 := sc.flag()
			sweep, ok5 := sc.flag()
			e, ok6 := pt()
			if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 {
				return p, errPathData
			}
			p = arcTo(p, pen, e, rx, ry, rot, large, sweep)
			pen = e
		case 'Z', 'z':
			p = append(p, seg{op: opClose})
			pen = start
		}
		last = cmd
	}
	return p, nil
}

// arcTo appends the SVG elliptical arc from a to b as cubics (endpoint to
// centre parameterisation, SVG 1.1 appendix F.6).
func arcTo(p path, a, b point, rx, ry, rotDeg float64, large, sweep bool) path {
	if a == b {
		return p
	}
	rx, ry = math.Abs(rx), math.Abs(ry)
	if rx == 0 || ry == 0 {
		return append(p, seg{op: opLine, pts: [3]point{b}})
	}
	sinPhi, cosPhi := math.Sincos(rotDeg * math.Pi / 180)
	dx, dy := (a.X-b.X)/2, (a.Y-b.Y)/2
	x1 := cosPhi*dx + sinPhi*dy
	y1 := -sinPhi*dx + cosPhi*dy
	if l := x1*x1/(rx*rx) + y1*y1/(ry*ry); l > 1 {
		s := math.Sqrt(l)
		rx, ry = rx*s, ry*s
	}
	num := rx*rx*ry*ry - rx*rx*y1*y1 - ry*ry*x1*x1
	den := rx*rx*y1*y1 + ry*ry*x1*x1
	k := 0.0
	if den > 0 && num > 0 {
		k = math.Sqrt(num / den)
	}
	if large == sweep {
		k = -k
	}
	cx1, cy1 := k*rx*y1/ry, -k*ry*x1/rx
	cx := cosPhi*cx1 - sinPhi*cy1 + (a.X+b.X)/2
	cy := sinPhi*cx1 + cosPhi*cy1 + (a.Y+b.Y)/2
	angle := func(ux, uy, vx, vy float64) float64 {
		return math.Atan2(ux*vy-uy*vx, ux*vx+uy*vy)
	}
	t1 := angle(1, 0, (x1-cx1)/rx, (y1-cy1)/ry)
	dt := angle((x1-cx1)/rx, (y1-cy1)/ry, (-x1-cx1)/rx, (-y1-cy1)/ry)
	if !sweep && dt > 0 {
		dt -= 2 * math.Pi
	} else if sweep && dt < 0 {
		dt += 2 * math.Pi
	}
	n := int(math.Ceil(math.Abs(dt) / (math.Pi / 2)))
	step := dt / float64(n)
	alpha := 4.0 / 3 * math.Tan(step/4)
	at := func(t float64) (point, point) {
		s, c := math.Sincos(t)
		pos := point{cx + rx*c*cosPhi - ry*s*sinPhi, cy + rx*c*sinPhi + ry*s*cosPhi}
		der := point{-rx*s*cosPhi - ry*c*sinPhi, -rx*s*sinPhi + ry*c*cosPhi}
		return pos, der
	}
	for i := range n {
		ta, tb := t1+float64(i)*step, t1+float64(i+1)*step
		pa, da := at(ta)
		pb, db := at(tb)
		if i == n-1 {
			pb = b
		}
		p = append(p, seg{op: opCubic, pts: [3]point{
			{pa.X + alpha*da.X, pa.Y + alpha*da.Y},
			{pb.X - alpha*db.X, pb.Y - alpha*db.Y},
			pb,
		}})
	}
	return p
}

// parseTransform reads an SVG transform list.
func parseTransform(s string) (affine, error) {
	m := identity
	rest := strings.TrimSpace(s)
	for rest != "" {
		open := strings.IndexByte(rest, '(')
		closing := strings.IndexByte(rest, ')')
		if open < 0 || closing < open {
			return identity, fmt.Errorf("transform %q", s)
		}
		name := strings.TrimSpace(rest[:open])
		args, ok := numbers(rest[open+1 : closing])
		rest = strings.TrimLeft(rest[closing+1:], " \t\n\r,")
		if !ok {
			return identity, fmt.Errorf("transform %q", s)
		}
		var t affine
		switch {
		case name == "matrix" && len(args) == 6:
			t = affine{args[0], args[1], args[2], args[3], args[4], args[5]}
		case name == "translate" && (len(args) == 1 || len(args) == 2):
			args = append(args, 0)
			t = translate(args[0], args[1])
		case name == "scale" && (len(args) == 1 || len(args) == 2):
			if len(args) == 1 {
				args = append(args, args[0])
			}
			t = scaling(args[0], args[1])
		case name == "rotate" && len(args) == 1:
			t = rotation(args[0] * math.Pi / 180)
		case name == "rotate" && len(args) == 3:
			t = translate(args[1], args[2]).mul(rotation(args[0] * math.Pi / 180)).mul(translate(-args[1], -args[2]))
		case name == "skewX" && len(args) == 1:
			t = affine{A: 1, C: math.Tan(args[0] * math.Pi / 180), D: 1}
		case name == "skewY" && len(args) == 1:
			t = affine{A: 1, B: math.Tan(args[0] * math.Pi / 180), D: 1}
		default:
			return identity, fmt.Errorf("transform %q", s)
		}
		m = m.mul(t)
	}
	return m, nil
}

// unitPx is CSS pixels per absolute unit.
var unitPx = map[string]float64{"": 1, "px": 1, "pt": 4.0 / 3, "pc": 16, "in": 96, "cm": 96 / 2.54, "mm": 96 / 25.4, "q": 96 / 101.6}

// parseLength reads a length; percentages are of ref. Font-relative units
// have no font to measure here and report unsupported.
func parseLength(s string, ref float64) (v float64, unsupported string, ok bool) {
	s = strings.TrimSpace(s)
	i := len(s)
	for i > 0 && (s[i-1] >= 'a' && s[i-1] <= 'z' || s[i-1] >= 'A' && s[i-1] <= 'Z' || s[i-1] == '%') {
		i--
	}
	num, err := strconv.ParseFloat(s[:i], 64)
	if err != nil || math.IsNaN(num) || math.IsInf(num, 0) {
		return 0, "", false
	}
	unit := strings.ToLower(s[i:])
	if unit == "%" {
		return num * ref / 100, "", true
	}
	if k, known := unitPx[unit]; known {
		return num * k, "", true
	}
	return 0, "length unit " + unit, false
}

// rgba is a straight (not premultiplied) colour, 0..1.
type rgba struct{ R, G, B, A float64 }

var namedColours = map[string]uint32{
	"black": 0x000000, "silver": 0xc0c0c0, "gray": 0x808080, "grey": 0x808080, "white": 0xffffff,
	"maroon": 0x800000, "red": 0xff0000, "purple": 0x800080, "fuchsia": 0xff00ff, "magenta": 0xff00ff,
	"green": 0x008000, "lime": 0x00ff00, "olive": 0x808000, "yellow": 0xffff00, "navy": 0x000080,
	"blue": 0x0000ff, "teal": 0x008080, "aqua": 0x00ffff, "cyan": 0x00ffff, "orange": 0xffa500,
	"darkgray": 0xa9a9a9, "darkgrey": 0xa9a9a9, "lightgray": 0xd3d3d3, "lightgrey": 0xd3d3d3,
	"dimgray": 0x696969, "dimgrey": 0x696969, "gold": 0xffd700, "pink": 0xffc0cb, "brown": 0xa52a2a,
	"crimson": 0xdc143c, "coral": 0xff7f50, "tomato": 0xff6347, "salmon": 0xfa8072, "khaki": 0xf0e68c,
	"beige": 0xf5f5dc, "ivory": 0xfffff0, "tan": 0xd2b48c, "chocolate": 0xd2691e, "sienna": 0xa0522d,
	"indigo": 0x4b0082, "violet": 0xee82ee, "orchid": 0xda70d6, "plum": 0xdda0dd, "lavender": 0xe6e6fa,
	"skyblue": 0x87ceeb, "steelblue": 0x4682b4, "royalblue": 0x4169e1, "darkblue": 0x00008b,
	"darkgreen": 0x006400, "darkred": 0x8b0000, "forestgreen": 0x228b22, "seagreen": 0x2e8b57,
	"turquoise": 0x40e0d0, "slategray": 0x708090, "slategrey": 0x708090, "whitesmoke": 0xf5f5f5,
	"gainsboro": 0xdcdcdc, "snow": 0xfffafa, "wheat": 0xf5deb3, "goldenrod": 0xdaa520,
	"orangered": 0xff4500, "firebrick": 0xb22222, "midnightblue": 0x191970, "darkorange": 0xff8c00,
}

// parseColour reads a CSS colour. ok is false for anything it does not know.
func parseColour(s string) (rgba, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "transparent" {
		return rgba{}, true
	}
	if v, ok := namedColours[s]; ok {
		return hexRGB(v, 1), true
	}
	if strings.HasPrefix(s, "#") {
		h := s[1:]
		if len(h) == 3 || len(h) == 4 {
			var b strings.Builder
			for _, c := range h {
				b.WriteRune(c)
				b.WriteRune(c)
			}
			h = b.String()
		}
		if len(h) != 6 && len(h) != 8 {
			return rgba{}, false
		}
		v, err := strconv.ParseUint(h, 16, 32)
		if err != nil {
			return rgba{}, false
		}
		if len(h) == 8 {
			return hexRGB(uint32(v>>8), float64(v&0xff)/255), true
		}
		return hexRGB(uint32(v), 1), true
	}
	for _, fn := range []string{"rgba(", "rgb("} {
		if !strings.HasPrefix(s, fn) || !strings.HasSuffix(s, ")") {
			continue
		}
		fields := strings.FieldsFunc(s[len(fn):len(s)-1], func(r rune) bool { return r == ',' || r == ' ' || r == '/' })
		if len(fields) != 3 && len(fields) != 4 {
			return rgba{}, false
		}
		var ch [4]float64
		ch[3] = 1
		for i, f := range fields {
			pct := strings.HasSuffix(f, "%")
			v, err := strconv.ParseFloat(strings.TrimSuffix(f, "%"), 64)
			if err != nil {
				return rgba{}, false
			}
			switch {
			case pct:
				v /= 100
			case i < 3:
				v /= 255
			}
			ch[i] = min(max(v, 0), 1)
		}
		return rgba{ch[0], ch[1], ch[2], ch[3]}, true
	}
	return rgba{}, false
}

func hexRGB(v uint32, a float64) rgba {
	return rgba{float64(v>>16&0xff) / 255, float64(v>>8&0xff) / 255, float64(v&0xff) / 255, a}
}

// urlRef splits "url(#id) fallback" into the id and the rest.
func urlRef(s string) (id, rest string, ok bool) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "url(") {
		return "", "", false
	}
	end := strings.IndexByte(s, ')')
	if end < 0 {
		return "", "", false
	}
	ref := strings.Trim(strings.TrimSpace(s[4:end]), `'"`)
	if !strings.HasPrefix(ref, "#") {
		return "", "", false
	}
	return ref[1:], strings.TrimSpace(s[end+1:]), true
}
