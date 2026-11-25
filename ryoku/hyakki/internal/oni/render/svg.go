package render

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
)

// The SVG subset drawn here is the one profile/validate.go admits: shapes,
// paths with every command, groups, use and symbol, linear and radial
// gradients, clipPath and mask. What a file asks for beyond what this
// renderer draws is reported through compiler.report, never dropped quietly.

type elem struct {
	name string
	attr map[string]string
	kids []*elem
}

func (e *elem) get(name string) (string, bool) {
	v, ok := e.attr[name]
	return v, ok
}

func parseDocument(data []byte) (*elem, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var stack []*elem
	var root *elem
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			e := &elem{name: t.Name.Local, attr: map[string]string{}}
			for _, a := range t.Attr {
				if a.Name.Space == "xmlns" || a.Name.Local == "xmlns" {
					continue
				}
				e.attr[a.Name.Local] = strings.TrimSpace(a.Value)
			}
			if style, ok := e.attr["style"]; ok {
				for decl := range strings.SplitSeq(style, ";") {
					k, v, ok := strings.Cut(decl, ":")
					if !ok {
						continue
					}
					v = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(v), "!important"))
					e.attr[strings.ToLower(strings.TrimSpace(k))] = v
				}
			}
			if len(stack) > 0 {
				p := stack[len(stack)-1]
				p.kids = append(p.kids, e)
			} else if root == nil {
				root = e
			}
			stack = append(stack, e)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if root == nil || root.name != "svg" {
		return nil, errors.New("no svg element")
	}
	return root, nil
}

type colourRole uint8

const (
	roleAuthored colourRole = iota
	roleInk
	roleWhite
	roleBody
)

type colour struct {
	c    rgba
	role colourRole
}

type gstop struct {
	off float64
	c   colour
}

const (
	spreadPad = iota
	spreadReflect
	spreadRepeat
)

type gradient struct {
	linear                bool
	x1, y1, x2, y2        float64
	cx, cy, r, fx, fy, fr float64
	spread                int
	stops                 []gstop
}

// paint is a solid colour, or a gradient whose space m maps into art space.
type paint struct {
	solid   colour
	grad    *gradient
	m       affine
	opacity float64
}

type clipPart struct {
	polys   [][]point
	evenOdd bool
}

type clipShape struct {
	parts []clipPart
	clip  *clipShape
}

type maskRef struct {
	content *node
	region  []point
	alpha   bool
}

type bounds struct {
	minX, minY, maxX, maxY float64
	ok                     bool
}

func (b *bounds) add(p point) {
	if !b.ok {
		*b = bounds{p.X, p.Y, p.X, p.Y, true}
		return
	}
	b.minX, b.minY = min(b.minX, p.X), min(b.minY, p.Y)
	b.maxX, b.maxY = max(b.maxX, p.X), max(b.maxY, p.Y)
}

func (b *bounds) union(o bounds) {
	if o.ok {
		b.add(point{o.minX, o.minY})
		b.add(point{o.maxX, o.maxY})
	}
}

// node is one drawable element in art space (the root viewBox's units).
type node struct {
	fill     [][]point
	evenOdd  bool
	fillP    *paint
	stroke   [][]point
	strokeP  *paint
	children []*node
	opacity  float64
	clip     *clipShape
	mask     *maskRef
	geom     bounds
}

func (n *node) empty() bool {
	return n == nil || (len(n.fill) == 0 && len(n.stroke) == 0 && len(n.children) == 0)
}

// style is the inherited presentation state.
type style struct {
	fill, stroke               string
	fillOpacity, strokeOpacity float64
	fillRule, clipRule         string
	strokeWidth                string
	linecap, linejoin          string
	miterLimit                 float64
	dashArray, dashOffset      string
	colour                     string
	visible                    bool
}

func defaultStyle() style {
	return style{
		fill: "black", stroke: "none", fillOpacity: 1, strokeOpacity: 1,
		fillRule: "nonzero", clipRule: "nonzero", strokeWidth: "1",
		linecap: "butt", linejoin: "miter", miterLimit: 4, dashArray: "none",
		dashOffset: "0", colour: "black", visible: true,
	}
}

// art is one parsed SVG file.
type art struct {
	file string
	vb   [4]float64
	root *node
}

type compiler struct {
	file     string
	ids      map[string]*elem
	report   func(what string)
	recolour func(rgba) colour
	tol      float64
	vw, vh   float64
	using    map[*elem]bool
}

// compileArt parses one SVG file into art-space geometry. maxPx is the most
// pixels the art's longer side ever spans; curves are flattened to a tenth
// of a pixel at that size.
func compileArt(file string, data []byte, recolour func(rgba) colour, maxPx float64, report func(string)) (*art, error) {
	root, err := parseDocument(data)
	if err != nil {
		return nil, err
	}
	c := &compiler{file: file, ids: map[string]*elem{}, report: report, recolour: recolour, using: map[*elem]bool{}}
	var index func(*elem)
	index = func(e *elem) {
		if id, ok := e.get("id"); ok && id != "" {
			if _, dup := c.ids[id]; !dup {
				c.ids[id] = e
			}
		}
		for _, k := range e.kids {
			index(k)
		}
	}
	index(root)
	vb, ok := c.viewBox(root)
	if !ok {
		w, _, okW := parseLength(root.attr["width"], 0)
		h, _, okH := parseLength(root.attr["height"], 0)
		if !okW || !okH || w <= 0 || h <= 0 || strings.HasSuffix(root.attr["width"], "%") || strings.HasSuffix(root.attr["height"], "%") {
			report("svg without a viewBox or an absolute size")
			w, h = 100, 100
		}
		vb = [4]float64{0, 0, w, h}
	}
	c.vw, c.vh = vb[2], vb[3]
	c.tol = 0.1 * max(vb[2], vb[3]) / max(maxPx, 1)
	a := &art{file: file, vb: vb}
	st := c.cascade(root, defaultStyle())
	g := &node{opacity: 1}
	for _, k := range root.kids {
		if n := c.element(k, identity, st); !n.empty() {
			g.children = append(g.children, n)
			g.geom.union(n.geom)
		}
	}
	a.root = c.decorate(root, g, identity)
	return a, nil
}

func (c *compiler) viewBox(e *elem) ([4]float64, bool) {
	v, ok := e.get("viewBox")
	if !ok {
		return [4]float64{}, false
	}
	n, ok := numbers(v)
	if !ok || len(n) != 4 || n[2] <= 0 || n[3] <= 0 {
		c.report("viewBox " + strconv.Quote(v))
		return [4]float64{}, false
	}
	return [4]float64{n[0], n[1], n[2], n[3]}, true
}

var inheritable = []string{
	"fill", "stroke", "fill-opacity", "stroke-opacity", "fill-rule", "clip-rule",
	"stroke-width", "stroke-linecap", "stroke-linejoin", "stroke-miterlimit",
	"stroke-dasharray", "stroke-dashoffset", "color", "visibility",
}

func (c *compiler) cascade(e *elem, st style) style {
	for _, name := range inheritable {
		v, ok := e.get(name)
		if !ok || v == "inherit" || v == "" {
			continue
		}
		switch name {
		case "fill":
			st.fill = v
		case "stroke":
			st.stroke = v
		case "fill-opacity":
			st.fillOpacity = c.opacity(v)
		case "stroke-opacity":
			st.strokeOpacity = c.opacity(v)
		case "fill-rule":
			st.fillRule = v
		case "clip-rule":
			st.clipRule = v
		case "stroke-width":
			st.strokeWidth = v
		case "stroke-linecap":
			st.linecap = v
		case "stroke-linejoin":
			st.linejoin = v
		case "stroke-miterlimit":
			if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 1 {
				st.miterLimit = f
			}
		case "stroke-dasharray":
			st.dashArray = v
		case "stroke-dashoffset":
			st.dashOffset = v
		case "color":
			st.colour = v
		case "visibility":
			st.visible = v == "visible"
		}
	}
	return st
}

func (c *compiler) opacity(v string) float64 {
	v = strings.TrimSpace(v)
	pct := strings.HasSuffix(v, "%")
	f, err := strconv.ParseFloat(strings.TrimSuffix(v, "%"), 64)
	if err != nil {
		c.report("opacity " + strconv.Quote(v))
		return 1
	}
	if pct {
		f /= 100
	}
	return min(max(f, 0), 1)
}

// unsupportedAttrs are presentation attributes the validator lets through
// that this renderer does not draw.
var unsupportedAttrs = map[string]string{
	"filter": "none", "marker": "none", "marker-start": "none", "marker-mid": "none",
	"marker-end": "none", "paint-order": "normal", "vector-effect": "none",
	"mix-blend-mode": "normal",
}

func (c *compiler) checkAttrs(e *elem) {
	for name, neutral := range unsupportedAttrs {
		if v, ok := e.get(name); ok && v != neutral && v != "" {
			c.report(e.name + " " + name)
		}
	}
}

func (c *compiler) length(e *elem, name string, ref, def float64) float64 {
	v, ok := e.get(name)
	if !ok || v == "" {
		return def
	}
	f, unsupported, ok := parseLength(v, ref)
	if unsupported != "" {
		c.report(e.name + " " + name + " " + unsupported)
		return def
	}
	if !ok {
		c.report(e.name + " " + name + " " + strconv.Quote(v))
		return def
	}
	return f
}

func (c *compiler) diag() float64 { return math.Sqrt((c.vw*c.vw + c.vh*c.vh) / 2) }

func (c *compiler) local(e *elem, ctm affine) affine {
	v, ok := e.get("transform")
	if !ok || v == "" {
		return ctm
	}
	t, err := parseTransform(v)
	if err != nil {
		c.report(e.name + " transform " + strconv.Quote(v))
		return ctm
	}
	return ctm.mul(t)
}

// element compiles one rendered element; nil when it draws nothing.
func (c *compiler) element(e *elem, ctm affine, parent style) *node {
	if e.attr["display"] == "none" {
		return nil
	}
	switch e.name {
	case "defs", "linearGradient", "radialGradient", "stop", "title", "desc", "clipPath", "mask", "symbol":
		return nil
	}
	c.checkAttrs(e)
	st := c.cascade(e, parent)
	local := c.local(e, ctm)
	var n *node
	switch e.name {
	case "g":
		n = c.group(e.kids, local, st)
	case "svg":
		n = c.nested(e, e, local, st, c.length(e, "x", c.vw, 0), c.length(e, "y", c.vh, 0))
	case "use":
		n = c.use(e, local, st)
	default:
		p, ok := c.geometry(e)
		if !ok {
			return nil
		}
		n = c.shape(e, p, local, st)
	}
	if n.empty() {
		return nil
	}
	return c.decorate(e, n, local)
}

func (c *compiler) group(kids []*elem, ctm affine, st style) *node {
	g := &node{opacity: 1}
	for _, k := range kids {
		if n := c.element(k, ctm, st); !n.empty() {
			g.children = append(g.children, n)
			g.geom.union(n.geom)
		}
	}
	return g
}

// nested draws an svg or symbol's children in its own viewport at (x, y),
// clipped to it.
func (c *compiler) nested(sizeFrom, content *elem, ctm affine, st style, x, y float64) *node {
	w := c.length(sizeFrom, "width", c.vw, c.vw)
	h := c.length(sizeFrom, "height", c.vh, c.vh)
	if w <= 0 || h <= 0 {
		return nil
	}
	inner := ctm.mul(translate(x, y))
	if vb, ok := c.viewBox(content); ok {
		inner = ctm.mul(c.fit(vb, x, y, w, h, content.attr["preserveAspectRatio"]))
	}
	st = c.cascade(content, st)
	g := c.group(content.kids, inner, st)
	if g.empty() {
		return nil
	}
	if ov := content.attr["overflow"]; ov != "visible" && ov != "auto" {
		rect := []point{ctm.apply(point{x, y}), ctm.apply(point{x + w, y}), ctm.apply(point{x + w, y + h}), ctm.apply(point{x, y + h})}
		g = &node{opacity: 1, children: []*node{g}, geom: g.geom, clip: &clipShape{parts: []clipPart{{polys: [][]point{rect}}}}}
	}
	return g
}

// fit is the viewBox transform for preserveAspectRatio par.
func (c *compiler) fit(vb [4]float64, x, y, w, h float64, par string) affine {
	sx, sy := w/vb[2], h/vb[3]
	fields := strings.Fields(par)
	align, slice := "xMidYMid", false
	if len(fields) > 0 && fields[0] == "defer" {
		fields = fields[1:]
	}
	if len(fields) > 0 {
		align = fields[0]
	}
	if len(fields) > 1 {
		slice = fields[1] == "slice"
	}
	if align == "none" {
		return translate(x, y).mul(scaling(sx, sy)).mul(translate(-vb[0], -vb[1]))
	}
	s := min(sx, sy)
	if slice {
		s = max(sx, sy)
	}
	tx, ty := x, y
	switch {
	case strings.Contains(align, "xMid"):
		tx += (w - vb[2]*s) / 2
	case strings.Contains(align, "xMax"):
		tx += w - vb[2]*s
	}
	switch {
	case strings.Contains(align, "YMid"):
		ty += (h - vb[3]*s) / 2
	case strings.Contains(align, "YMax"):
		ty += h - vb[3]*s
	}
	return translate(tx, ty).mul(scaling(s, s)).mul(translate(-vb[0], -vb[1]))
}

func (c *compiler) use(e *elem, ctm affine, st style) *node {
	href := e.attr["href"]
	id := strings.TrimPrefix(href, "#")
	target, ok := c.ids[id]
	if !strings.HasPrefix(href, "#") || !ok {
		c.report("use of missing " + strconv.Quote(href))
		return nil
	}
	if c.using[target] || len(c.using) >= 8 {
		c.report("use loop through " + strconv.Quote(href))
		return nil
	}
	c.using[target] = true
	defer delete(c.using, target)
	x, y := c.length(e, "x", c.vw, 0), c.length(e, "y", c.vh, 0)
	switch target.name {
	case "symbol", "svg":
		if target.attr["display"] == "none" {
			return nil
		}
		sized := target
		if _, ok := e.get("width"); ok {
			sized = e
		}
		n := c.nested(sized, target, ctm, st, x, y)
		if n.empty() {
			return nil
		}
		return c.decorate(target, n, ctm)
	}
	inner := c.element(target, ctm.mul(translate(x, y)), st)
	if inner.empty() {
		return nil
	}
	return &node{opacity: 1, children: []*node{inner}, geom: inner.geom}
}

// geometry is a shape element's outline in its user space.
func (c *compiler) geometry(e *elem) (path, bool) {
	switch e.name {
	case "path":
		p, err := parsePath(e.attr["d"])
		if err != nil {
			c.report("path data")
		}
		return p, len(p) > 0
	case "rect":
		x, y := c.length(e, "x", c.vw, 0), c.length(e, "y", c.vh, 0)
		w, h := c.length(e, "width", c.vw, 0), c.length(e, "height", c.vh, 0)
		if w <= 0 || h <= 0 {
			return nil, false
		}
		rx, hasRX := e.get("rx")
		ry, hasRY := e.get("ry")
		var rxv, ryv float64
		if hasRX && rx != "auto" {
			rxv = c.length(e, "rx", c.vw, 0)
		}
		if hasRY && ry != "auto" {
			ryv = c.length(e, "ry", c.vh, 0)
		}
		if !hasRX || rx == "auto" {
			rxv = ryv
		}
		if !hasRY || ry == "auto" {
			ryv = rxv
		}
		rxv, ryv = min(max(rxv, 0), w/2), min(max(ryv, 0), h/2)
		if rxv == 0 || ryv == 0 {
			return path{
				{op: opMove, pts: [3]point{{x, y}}}, {op: opLine, pts: [3]point{{x + w, y}}},
				{op: opLine, pts: [3]point{{x + w, y + h}}}, {op: opLine, pts: [3]point{{x, y + h}}}, {op: opClose},
			}, true
		}
		p := path{{op: opMove, pts: [3]point{{x + rxv, y}}}, {op: opLine, pts: [3]point{{x + w - rxv, y}}}}
		p = arcTo(p, point{x + w - rxv, y}, point{x + w, y + ryv}, rxv, ryv, 0, false, true)
		p = append(p, seg{op: opLine, pts: [3]point{{x + w, y + h - ryv}}})
		p = arcTo(p, point{x + w, y + h - ryv}, point{x + w - rxv, y + h}, rxv, ryv, 0, false, true)
		p = append(p, seg{op: opLine, pts: [3]point{{x + rxv, y + h}}})
		p = arcTo(p, point{x + rxv, y + h}, point{x, y + h - ryv}, rxv, ryv, 0, false, true)
		p = append(p, seg{op: opLine, pts: [3]point{{x, y + ryv}}})
		p = arcTo(p, point{x, y + ryv}, point{x + rxv, y}, rxv, ryv, 0, false, true)
		return append(p, seg{op: opClose}), true
	case "circle":
		r := c.length(e, "r", c.diag(), 0)
		return ellipse(c.length(e, "cx", c.vw, 0), c.length(e, "cy", c.vh, 0), r, r)
	case "ellipse":
		rx, ry := c.length(e, "rx", c.vw, -1), c.length(e, "ry", c.vh, -1)
		if rx < 0 {
			rx = ry
		}
		if ry < 0 {
			ry = rx
		}
		return ellipse(c.length(e, "cx", c.vw, 0), c.length(e, "cy", c.vh, 0), rx, ry)
	case "line":
		return path{
			{op: opMove, pts: [3]point{{c.length(e, "x1", c.vw, 0), c.length(e, "y1", c.vh, 0)}}},
			{op: opLine, pts: [3]point{{c.length(e, "x2", c.vw, 0), c.length(e, "y2", c.vh, 0)}}},
		}, true
	case "polyline", "polygon":
		v, ok := numbers(e.attr["points"])
		if !ok {
			c.report(e.name + " points")
		}
		if len(v)%2 == 1 {
			c.report(e.name + " points")
			v = v[:len(v)-1]
		}
		if len(v) < 2 {
			return nil, false
		}
		p := path{{op: opMove, pts: [3]point{{v[0], v[1]}}}}
		for i := 2; i+1 < len(v); i += 2 {
			p = append(p, seg{op: opLine, pts: [3]point{{v[i], v[i+1]}}})
		}
		if e.name == "polygon" {
			p = append(p, seg{op: opClose})
		}
		return p, true
	}
	c.report("element " + e.name)
	return nil, false
}

func ellipse(cx, cy, rx, ry float64) (path, bool) {
	if rx <= 0 || ry <= 0 {
		return nil, false
	}
	p := path{{op: opMove, pts: [3]point{{cx + rx, cy}}}}
	p = arcTo(p, point{cx + rx, cy}, point{cx, cy + ry}, rx, ry, 0, false, true)
	p = arcTo(p, point{cx, cy + ry}, point{cx - rx, cy}, rx, ry, 0, false, true)
	p = arcTo(p, point{cx - rx, cy}, point{cx, cy - ry}, rx, ry, 0, false, true)
	p = arcTo(p, point{cx, cy - ry}, point{cx + rx, cy}, rx, ry, 0, false, true)
	return append(p, seg{op: opClose}), true
}

func (c *compiler) shape(e *elem, p path, local affine, st style) *node {
	n := &node{opacity: 1, evenOdd: st.fillRule == "evenodd"}
	userTol := c.tol / max(local.scale(), 1e-9)
	cs := p.flatten(userTol)
	var user bounds
	for _, ct := range cs {
		for _, q := range ct.pts {
			user.add(q)
			n.geom.add(local.apply(q))
		}
	}
	if !st.visible {
		return n
	}
	if fp := c.paint(st.fill, st.fillOpacity, st, user, local); fp != nil && e.name != "line" {
		n.fillP = fp
		for _, ct := range cs {
			if len(ct.pts) < 3 {
				continue
			}
			poly := make([]point, len(ct.pts))
			for i, q := range ct.pts {
				poly[i] = local.apply(q)
			}
			n.fill = append(n.fill, poly)
		}
	}
	if sp := c.paint(st.stroke, st.strokeOpacity, st, user, local); sp != nil {
		ss := strokeStyle{
			width:      c.strokeLength(st.strokeWidth, "stroke-width"),
			join:       st.linejoin,
			cap:        st.linecap,
			miterLimit: st.miterLimit,
			dashOffset: c.strokeLength(st.dashOffset, "stroke-dashoffset"),
		}
		if st.dashArray != "none" && st.dashArray != "" {
			for f := range strings.FieldsFuncSeq(st.dashArray, func(r rune) bool { return r == ',' || r == ' ' }) {
				d := c.strokeLength(f, "stroke-dasharray")
				if d < 0 {
					ss.dashes = nil
					break
				}
				ss.dashes = append(ss.dashes, d)
			}
			if len(ss.dashes)%2 == 1 {
				ss.dashes = append(ss.dashes, ss.dashes...)
			}
		}
		for _, piece := range strokePieces(cs, ss, userTol) {
			for i, q := range piece {
				piece[i] = local.apply(q)
			}
			n.stroke = append(n.stroke, piece)
		}
		if len(n.stroke) > 0 {
			n.strokeP = sp
		}
	}
	return n
}

func (c *compiler) strokeLength(v, name string) float64 {
	f, unsupported, ok := parseLength(v, c.diag())
	if unsupported != "" || !ok {
		c.report(name + " " + strconv.Quote(v))
		if name == "stroke-width" {
			return 1
		}
		return 0
	}
	return f
}

// decorate applies an element's own opacity, clip-path and mask to n.
func (c *compiler) decorate(e *elem, n *node, local affine) *node {
	op := 1.0
	if v, ok := e.get("opacity"); ok {
		op = c.opacity(v)
	}
	user := c.userBounds(n.geom, local)
	var clip *clipShape
	if v, ok := e.get("clip-path"); ok && v != "none" {
		clip = c.clipRef(v, local, user)
	}
	var mask *maskRef
	if v, ok := e.get("mask"); ok && v != "none" {
		mask = c.maskRefFor(v, local, user)
	}
	if op == 1 && clip == nil && mask == nil {
		return n
	}
	if n.opacity == 1 && n.clip == nil && n.mask == nil {
		n.opacity, n.clip, n.mask = op, clip, mask
		return n
	}
	return &node{opacity: op, clip: clip, mask: mask, children: []*node{n}, geom: n.geom}
}

// userBounds maps art-space bounds back into the space local draws from.
func (c *compiler) userBounds(b bounds, local affine) bounds {
	inv, ok := local.invert()
	if !b.ok || !ok {
		return bounds{}
	}
	var out bounds
	for _, p := range []point{{b.minX, b.minY}, {b.maxX, b.minY}, {b.maxX, b.maxY}, {b.minX, b.maxY}} {
		out.add(inv.apply(p))
	}
	return out
}

func bboxMatrix(b bounds) (affine, bool) {
	w, h := b.maxX-b.minX, b.maxY-b.minY
	if !b.ok || w <= 0 || h <= 0 {
		return affine{}, false
	}
	return affine{A: w, D: h, E: b.minX, F: b.minY}, true
}

func (c *compiler) paint(spec string, opacity float64, st style, user bounds, local affine) *paint {
	spec = strings.TrimSpace(spec)
	if spec == "none" || spec == "" {
		return nil
	}
	if id, rest, ok := urlRef(spec); ok {
		g, found := c.ids[id]
		if found && (g.name == "linearGradient" || g.name == "radialGradient") {
			return c.gradient(g, opacity, user, local)
		}
		if rest != "" {
			return c.paint(rest, opacity, st, user, local)
		}
		c.report("paint " + strconv.Quote(spec))
		return nil
	}
	if spec == "currentColor" {
		spec = st.colour
	}
	col, ok := parseColour(spec)
	if !ok {
		c.report("colour " + strconv.Quote(spec))
		return nil
	}
	return &paint{solid: c.recolour(col), opacity: opacity}
}

// gradientAttr follows href from g to the first gradient that sets name.
func (c *compiler) gradientAttr(g *elem, name string) (string, *elem) {
	seen := map[*elem]bool{}
	for e := g; e != nil && !seen[e]; {
		seen[e] = true
		if v, ok := e.get(name); ok {
			return v, e
		}
		next, ok := c.ids[strings.TrimPrefix(e.attr["href"], "#")]
		if !ok || (next.name != "linearGradient" && next.name != "radialGradient") {
			break
		}
		e = next
	}
	return "", nil
}

func (c *compiler) gradientStops(g *elem) []gstop {
	seen := map[*elem]bool{}
	for e := g; e != nil && !seen[e]; {
		seen[e] = true
		var stops []gstop
		last := 0.0
		for _, k := range e.kids {
			if k.name != "stop" {
				continue
			}
			off := 0.0
			if v, ok := k.get("offset"); ok {
				pct := strings.HasSuffix(v, "%")
				f, err := strconv.ParseFloat(strings.TrimSuffix(v, "%"), 64)
				if err != nil {
					c.report("stop offset " + strconv.Quote(v))
				}
				if pct {
					f /= 100
				}
				off = min(max(f, 0), 1)
			}
			off = max(off, last)
			last = off
			col := rgba{A: 1}
			if v, ok := k.get("stop-color"); ok && v != "" {
				if v == "currentColor" {
					v = k.attr["color"]
				}
				parsed, ok := parseColour(v)
				if !ok {
					c.report("colour " + strconv.Quote(v))
				} else {
					col = parsed
				}
			}
			if v, ok := k.get("stop-opacity"); ok {
				col.A *= c.opacity(v)
			}
			stops = append(stops, gstop{off: off, c: c.recolour(col)})
		}
		if len(stops) > 0 {
			return stops
		}
		next, ok := c.ids[strings.TrimPrefix(e.attr["href"], "#")]
		if !ok || (next.name != "linearGradient" && next.name != "radialGradient") {
			break
		}
		e = next
	}
	return nil
}

func (c *compiler) gradient(g *elem, opacity float64, user bounds, local affine) *paint {
	stops := c.gradientStops(g)
	if len(stops) == 0 {
		return nil
	}
	if len(stops) == 1 {
		return &paint{solid: stops[0].c, opacity: opacity}
	}
	units, _ := c.gradientAttr(g, "gradientUnits")
	bbox := units != "userSpaceOnUse"
	m := local
	if bbox {
		bm, ok := bboxMatrix(user)
		if !ok {
			return nil
		}
		m = m.mul(bm)
	}
	if v, _ := c.gradientAttr(g, "gradientTransform"); v != "" {
		t, err := parseTransform(v)
		if err != nil {
			c.report("gradientTransform " + strconv.Quote(v))
		} else {
			m = m.mul(t)
		}
	}
	coord := func(name string, ref, def float64) float64 {
		v, _ := c.gradientAttr(g, name)
		if v == "" {
			return def
		}
		if bbox {
			pct := strings.HasSuffix(v, "%")
			f, err := strconv.ParseFloat(strings.TrimSuffix(v, "%"), 64)
			if err != nil {
				c.report(g.name + " " + name + " " + strconv.Quote(v))
				return def
			}
			if pct {
				f /= 100
			}
			return f
		}
		f, unsupported, ok := parseLength(v, ref)
		if !ok {
			c.report(g.name + " " + name + " " + strconv.Quote(v) + " " + unsupported)
			return def
		}
		return f
	}
	full := func(ref float64) float64 {
		if bbox {
			return 1
		}
		return ref
	}
	gr := &gradient{linear: g.name == "linearGradient", stops: stops}
	switch v, _ := c.gradientAttr(g, "spreadMethod"); v {
	case "reflect":
		gr.spread = spreadReflect
	case "repeat":
		gr.spread = spreadRepeat
	}
	if gr.linear {
		gr.x1 = coord("x1", c.vw, 0)
		gr.y1 = coord("y1", c.vh, 0)
		gr.x2 = coord("x2", c.vw, full(c.vw))
		gr.y2 = coord("y2", c.vh, 0)
		if gr.x1 == gr.x2 && gr.y1 == gr.y2 {
			return &paint{solid: stops[len(stops)-1].c, opacity: opacity}
		}
	} else {
		gr.cx = coord("cx", c.vw, full(c.vw)/2)
		gr.cy = coord("cy", c.vh, full(c.vh)/2)
		gr.r = coord("r", c.diag(), full(c.diag())/2)
		gr.fx = coord("fx", c.vw, gr.cx)
		gr.fy = coord("fy", c.vh, gr.cy)
		gr.fr = coord("fr", c.diag(), 0)
		if gr.r <= 0 {
			return &paint{solid: stops[len(stops)-1].c, opacity: opacity}
		}
	}
	return &paint{grad: gr, m: m, opacity: opacity}
}

func (c *compiler) clipRef(v string, local affine, user bounds) *clipShape {
	id, _, ok := urlRef(v)
	e, found := c.ids[id]
	if !ok || !found || e.name != "clipPath" {
		c.report("clip-path " + strconv.Quote(v))
		return nil
	}
	if c.using[e] {
		c.report("clip-path loop through " + strconv.Quote(v))
		return nil
	}
	c.using[e] = true
	defer delete(c.using, e)
	m := c.local(e, local)
	if e.attr["clipPathUnits"] == "objectBoundingBox" {
		bm, ok := bboxMatrix(user)
		if !ok {
			return &clipShape{}
		}
		m = c.local(e, local.mul(bm))
	}
	cs := &clipShape{}
	st := c.cascade(e, defaultStyle())
	for _, k := range e.kids {
		c.clipChild(cs, k, m, st)
	}
	if v, ok := e.get("clip-path"); ok && v != "none" {
		cs.clip = c.clipRef(v, local, user)
	}
	return cs
}

func (c *compiler) clipChild(cs *clipShape, k *elem, m affine, st style) {
	if k.attr["display"] == "none" {
		return
	}
	st = c.cascade(k, st)
	if !st.visible {
		return
	}
	local := c.local(k, m)
	if k.name == "use" {
		target, ok := c.ids[strings.TrimPrefix(k.attr["href"], "#")]
		if !ok || c.using[target] {
			c.report("clipPath use " + strconv.Quote(k.attr["href"]))
			return
		}
		c.using[target] = true
		c.clipChild(cs, target, local.mul(translate(c.length(k, "x", c.vw, 0), c.length(k, "y", c.vh, 0))), st)
		delete(c.using, target)
		return
	}
	switch k.name {
	case "title", "desc":
		return
	case "path", "rect", "circle", "ellipse", "line", "polyline", "polygon":
	default:
		c.report("clipPath child " + k.name)
		return
	}
	if v, ok := k.get("clip-path"); ok && v != "none" {
		c.report("clipPath child clip-path")
	}
	p, ok := c.geometry(k)
	if !ok {
		return
	}
	part := clipPart{evenOdd: st.clipRule == "evenodd"}
	for _, ct := range p.flatten(c.tol / max(local.scale(), 1e-9)) {
		if len(ct.pts) < 3 {
			continue
		}
		poly := make([]point, len(ct.pts))
		for i, q := range ct.pts {
			poly[i] = local.apply(q)
		}
		part.polys = append(part.polys, poly)
	}
	if len(part.polys) > 0 {
		cs.parts = append(cs.parts, part)
	}
}

func (c *compiler) maskRefFor(v string, local affine, user bounds) *maskRef {
	id, _, ok := urlRef(v)
	e, found := c.ids[id]
	if !ok || !found || e.name != "mask" {
		c.report("mask " + strconv.Quote(v))
		return nil
	}
	if c.using[e] {
		c.report("mask loop through " + strconv.Quote(v))
		return nil
	}
	c.using[e] = true
	defer delete(c.using, e)
	bbox := e.attr["maskUnits"] != "userSpaceOnUse"
	var x, y, w, h float64
	if bbox {
		bm, ok := bboxMatrix(user)
		if !ok {
			return &maskRef{}
		}
		frac := func(name string, def float64) float64 {
			v, ok := e.get(name)
			if !ok {
				return def
			}
			pct := strings.HasSuffix(v, "%")
			f, err := strconv.ParseFloat(strings.TrimSuffix(v, "%"), 64)
			if err != nil {
				c.report("mask " + name + " " + strconv.Quote(v))
				return def
			}
			if pct {
				f /= 100
			}
			return f
		}
		p0 := bm.apply(point{frac("x", -0.1), frac("y", -0.1)})
		x, y = p0.X, p0.Y
		w, h = frac("width", 1.2)*bm.A, frac("height", 1.2)*bm.D
	} else {
		x, y = c.length(e, "x", c.vw, -0.1*c.vw), c.length(e, "y", c.vh, -0.1*c.vh)
		w, h = c.length(e, "width", c.vw, 1.2*c.vw), c.length(e, "height", c.vh, 1.2*c.vh)
	}
	ref := &maskRef{alpha: e.attr["mask-type"] == "alpha"}
	if w <= 0 || h <= 0 {
		return ref
	}
	ref.region = []point{local.apply(point{x, y}), local.apply(point{x + w, y}), local.apply(point{x + w, y + h}), local.apply(point{x, y + h})}
	content := local
	if e.attr["maskContentUnits"] == "objectBoundingBox" {
		bm, ok := bboxMatrix(user)
		if !ok {
			return ref
		}
		content = local.mul(bm)
	}
	ref.content = c.group(e.kids, content, c.cascade(e, defaultStyle()))
	return ref
}
