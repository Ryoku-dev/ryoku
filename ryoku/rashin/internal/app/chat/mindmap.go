package chat

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/gpu/headless"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/paths"
)

type mindmapNode struct {
	text          string
	parent, depth int
	children      []int
}

type mindmapVisual struct {
	source               string
	nodes                []mindmapNode
	err                  error
	folded               map[int]bool
	buttons              []mindmapButton
	vertical, horizontal layout.List
	points               []image.Point
	visible              []bool
	positioned           bool
	bounds               image.Rectangle
	zoom                 float32
	lastWidth            int
	viewport             image.Point
	metric               unit.Metric
}

type mindmapButton struct {
	click            kit.Click
	hover, press     anim.Tween
	hovered, pressed float32
}

func parseMindmap(source string) ([]mindmapNode, error) {
	if len(source) > 64<<10 {
		return nil, errors.New("Mind map is too large")
	}
	var nodes []mindmapNode
	var stack []int
	for line, raw := range strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		depth := indent / 2
		text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "- "))
		if strings.ContainsRune(raw[:indent], '\t') || strings.HasPrefix(raw[indent:], "\t") || indent%2 != 0 || depth > len(stack) || depth > 16 || len(nodes) >= 256 || len(text) > 512 {
			return nil, fmt.Errorf("Mind map line %d needs two spaces per level", line+1)
		}
		if len(nodes) > 0 && depth == 0 {
			return nil, errors.New("Mind maps need one root")
		}
		parent := -1
		if depth > 0 {
			parent = stack[depth-1]
		}
		index := len(nodes)
		nodes = append(nodes, mindmapNode{text: text, parent: parent, depth: depth})
		if parent >= 0 {
			nodes[parent].children = append(nodes[parent].children, index)
		}
		stack = append(stack[:depth], index)
	}
	if len(nodes) == 0 {
		return nil, errors.New("Mind map is empty")
	}
	return nodes, nil
}

func mindmapNodeBounds(node mindmapNode, center image.Point) image.Rectangle {
	width := min(188, max(100, utf8.RuneCountInString(node.text)*7+24))
	return image.Rect(center.X-width/2, center.Y-30, center.X-width/2+width, center.Y+30)
}

func (m *mindmapVisual) positions() ([]image.Point, []bool, int) {
	visible := make([]bool, len(m.nodes))
	weights := make([]int, len(m.nodes))
	levels := make([][]int, 17)
	halfWidths := make([]float64, len(levels))
	var weigh func(int) int
	weigh = func(index int) int {
		node := m.nodes[index]
		visible[index] = true
		levels[node.depth] = append(levels[node.depth], index)
		halfWidths[node.depth] = max(halfWidths[node.depth], float64(mindmapNodeBounds(node, image.Point{}).Dx())/2)
		if !m.folded[index] {
			for _, child := range node.children {
				weights[index] += weigh(child)
			}
		}
		weights[index] = max(1, weights[index])
		return weights[index]
	}
	weigh(0)
	angles := make([]float64, len(m.nodes))
	var spread func(int, float64, float64)
	spread = func(index int, start, span float64) {
		if m.folded[index] {
			return
		}
		total := 0
		for _, child := range m.nodes[index].children {
			total += weights[child]
		}
		for _, child := range m.nodes[index].children {
			sector := span * float64(weights[child]) / float64(max(1, total))
			angles[child] = start + sector/2
			spread(child, start, sector)
			start += sector
		}
	}
	spread(0, -math.Pi, 2*math.Pi)
	positions := make([]image.Point, len(m.nodes))
	placed := []int{0}
	radius := float64(0)
	const gap = 20
	for depth := 1; depth < len(levels) && len(levels[depth]) > 0; depth++ {
		radius += halfWidths[depth-1] + halfWidths[depth] + gap
		for {
			for _, index := range levels[depth] {
				positions[index] = image.Pt(int(math.Round(math.Cos(angles[index])*radius)), int(math.Round(math.Sin(angles[index])*radius)))
			}
			collides := false
			for at, index := range levels[depth] {
				box := mindmapNodeBounds(m.nodes[index], positions[index]).Inset(-gap / 2)
				for _, other := range placed {
					if box.Overlaps(mindmapNodeBounds(m.nodes[other], positions[other]).Inset(-gap / 2)) {
						collides = true
						break
					}
				}
				for _, other := range levels[depth][:at] {
					if box.Overlaps(mindmapNodeBounds(m.nodes[other], positions[other]).Inset(-gap / 2)) {
						collides = true
						break
					}
				}
				if collides {
					break
				}
			}
			if !collides {
				break
			}
			radius += gap
		}
		placed = append(placed, levels[depth]...)
	}
	bounds := image.Rectangle{}
	for _, index := range placed {
		bounds = bounds.Union(mindmapNodeBounds(m.nodes[index], positions[index]))
	}
	return positions, visible, max(bounds.Dx(), bounds.Dy())
}

func (state *viewState) layoutMindmap(gtx layout.Context, th *look.Theme, key, source string) layout.Dimensions {
	visual := state.codeVisual(key)
	if visual.mindmap == nil || visual.mindmap.source != source {
		nodes, err := parseMindmap(source)
		visual.mindmap = &mindmapVisual{source: source, nodes: nodes, err: err, folded: make(map[int]bool), buttons: make([]mindmapButton, len(nodes)), vertical: layout.List{Axis: layout.Vertical}, horizontal: layout.List{Axis: layout.Horizontal}, zoom: 1}
	}
	m := visual.mindmap
	if m.err != nil {
		return state.layoutCodeBlock(gtx, th, key, "mindmap", source)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			button := state.plaque(key + ":save-map")
			button.Kind, button.Label = kit.PlaqueTonal, i18n.T("Save as picture")
			if button.Clicked(gtx) {
				path, err := m.save(th)
				if err != nil {
					state.artifactNotice(err.Error())
				} else {
					state.artifactNotice(i18n.Tf("Saved to %s", path))
				}
			}
			return button.Layout(gtx, th.Theme)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			controls := [...]struct {
				id, label string
				zoom      float32
			}{
				{"fit", i18n.T("Fit map"), 1},
				{"out", i18n.T("Zoom out"), max(1, m.zoom/1.25)},
				{"in", i18n.T("Zoom in"), min(3, m.zoom*1.25)},
			}
			return look.Flow{Gap: th.Sp.S2, LineGap: th.Sp.S2}.Layout(gtx, len(controls), func(gtx layout.Context, index int) layout.Dimensions {
				control := controls[index]
				button := state.plaque(key + ":map-" + control.id)
				button.Kind, button.Label, button.Small = kit.PlaqueQuiet, control.label, true
				if button.Clicked(gtx) {
					m.zoom, m.positioned = control.zoom, false
				}
				return button.Layout(gtx, th.Theme)
			})
		}),
		layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg3, i18n.T("Select a branch to fold it. Zoom in and scroll to explore."))),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			m.prepare()
			width := max(1, gtx.Constraints.Max.X)
			_, height := mindmapFit(m.bounds, float32(width)/max(gtx.Metric.PxPerDp, .001))
			heightPx := min(gtx.Constraints.Max.Y, gtx.Dp(unit.Dp(height)))
			m.viewport, m.metric = image.Pt(width, heightPx), gtx.Metric
			gtx.Constraints = layout.Exact(m.viewport)
			return m.canvas(gtx, th, true)
		}),
	)
}

func (m *mindmapVisual) prepare() {
	if len(m.points) != 0 {
		return
	}
	m.points, m.visible, _ = m.positions()
	m.bounds = image.Rectangle{}
	for index, center := range m.points {
		if m.visible[index] {
			m.bounds = m.bounds.Union(mindmapNodeBounds(m.nodes[index], center))
		}
	}
}

func mindmapFit(bounds image.Rectangle, width float32) (float32, int) {
	scale := min(float32(1), max(1, width-32)/float32(max(1, bounds.Dx())), float32(448)/float32(max(1, bounds.Dy())))
	scale = max(.7, scale)
	height := min(480, max(240, int(math.Ceil(float64(float32(bounds.Dy())*scale)))+32))
	return scale, height
}

func (m *mindmapVisual) canvas(gtx layout.Context, th *look.Theme, interactive bool) layout.Dimensions {
	m.prepare()
	viewport := gtx.Constraints.Max
	scale, _ := mindmapFit(m.bounds, float32(viewport.X)/max(gtx.Metric.PxPerDp, .001))
	scale *= max(1, m.zoom)
	tree := image.Pt(int(math.Ceil(float64(float32(gtx.Dp(unit.Dp(m.bounds.Dx())))*scale))), int(math.Ceil(float64(float32(gtx.Dp(unit.Dp(m.bounds.Dy())))*scale))))
	content := image.Pt(max(viewport.X, tree.X+gtx.Dp(32)), max(viewport.Y, tree.Y+gtx.Dp(32)))
	if !m.positioned || m.lastWidth != viewport.X {
		m.horizontal.Position.Offset = max(0, (content.X-viewport.X)/2)
		m.vertical.Position.Offset = max(0, (content.Y-viewport.Y)/2)
		m.positioned, m.lastWidth = true, viewport.X
	}
	bounds := image.Rectangle{Max: viewport}
	cut := gtx.Dp(th.Cut.Mid)
	kit.Fill(gtx, th.Theme, bounds, cut, th.R.Field)
	area := clip.Rect(bounds.Inset(max(1, gtx.Dp(1)))).Push(gtx.Ops)
	m.vertical.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return m.horizontal.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
			center := op.Offset(image.Pt((content.X-tree.X)/2, (content.Y-tree.Y)/2)).Push(gtx.Ops)
			transform := op.Affine(f32.Affine2D{}.Scale(f32.Point{}, f32.Pt(scale, scale))).Push(gtx.Ops)
			origin := op.Offset(image.Pt(-gtx.Dp(unit.Dp(m.bounds.Min.X)), -gtx.Dp(unit.Dp(m.bounds.Min.Y)))).Push(gtx.Ops)
			m.draw(gtx, th, interactive)
			origin.Pop()
			transform.Pop()
			center.Pop()
			return layout.Dimensions{Size: content}
		})
	})
	area.Pop()
	kit.Stroke(gtx, th.Theme, bounds, cut, max(1, gtx.Dp(1)), th.R.Edge)
	return layout.Dimensions{Size: viewport}
}

func (m *mindmapVisual) draw(gtx layout.Context, th *look.Theme, interactive bool) {
	positions, visible := m.points, m.visible
	for index, node := range m.nodes {
		if !visible[index] || node.parent < 0 {
			continue
		}
		a, b := positions[node.parent], positions[index]
		var line clip.Path
		line.Begin(gtx.Ops)
		line.MoveTo(f32.Pt(float32(gtx.Dp(unit.Dp(a.X))), float32(gtx.Dp(unit.Dp(a.Y)))))
		line.LineTo(f32.Pt(float32(gtx.Dp(unit.Dp(b.X))), float32(gtx.Dp(unit.Dp(b.Y)))))
		paint.FillShape(gtx.Ops, th.R.Vein, clip.Stroke{Path: line.End(), Width: float32(gtx.Dp(2))}.Op())
	}
	for index, node := range m.nodes {
		if !visible[index] {
			continue
		}
		label := node.text
		if m.folded[index] {
			label += fmt.Sprintf(" (%d)", len(node.children))
		}
		button := &m.buttons[index]
		if interactive && button.click.Clicked(gtx) && len(node.children) > 0 {
			m.folded[index] = !m.folded[index]
			m.points = nil
			m.positioned = false
			gtx.Execute(op.InvalidateCmd{})
		}
		box := mindmapNodeBounds(node, positions[index])
		at := image.Pt(gtx.Dp(unit.Dp(box.Min.X)), gtx.Dp(unit.Dp(box.Min.Y)))
		offset := op.Offset(at).Push(gtx.Ops)
		child := gtx
		child.Constraints = layout.Exact(image.Pt(gtx.Dp(unit.Dp(box.Dx())), gtx.Dp(unit.Dp(box.Dy()))))
		button.layout(child, th, node.depth, label, interactive)
		offset.Pop()
	}
}

func (button *mindmapButton) layout(gtx layout.Context, th *look.Theme, depth int, label string, interactive bool) layout.Dimensions {
	draw := func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		if interactive {
			button.hovered = look.Transition(gtx, &button.hover, button.click.Hovered(), th.Motion.HoverIn, th.Motion.HoverOut)
			button.pressed = look.Transition(gtx, &button.press, button.click.Pressed(), th.Motion.Press, th.Motion.Press)
			semantic.Button.Add(gtx.Ops)
			semantic.LabelOp(label).Add(gtx.Ops)
		}
		scale := 1 - .025*button.pressed
		transform := op.Affine(f32.Affine2D{}.Scale(f32.Pt(float32(size.X)/2, float32(size.Y)/2), f32.Pt(scale, scale))).Push(gtx.Ops)
		defer transform.Pop()
		fill, ink := th.R.Stone3, th.R.Fg
		switch {
		case depth == 0:
			fill, ink = th.R.Sel, th.R.OnSel
		case depth > 1:
			fill, ink = palette.Mix(th.R.Field, th.R.Stone3, .35), th.R.Fg2
		}
		fill = palette.Mix(fill, th.R.Sel, .12*button.hovered)
		bounds := image.Rectangle{Max: size}
		cut := gtx.Dp(th.Cut.Mid)
		kit.Fill(gtx, th.Theme, bounds, cut, fill)
		kit.Stroke(gtx, th.Theme, bounds, cut, max(1, gtx.Dp(1)), th.R.Vein)
		layout.Inset{Left: 10, Right: 10, Top: 6, Bottom: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return kit.Label{Style: th.T.Label, Color: ink, MaxLines: 2, Align: text.Middle}.Layout(gtx, th.Theme, label)
			})
		})
		return layout.Dimensions{Size: size}
	}
	if interactive {
		return button.click.Layout(gtx, draw)
	}
	return draw(gtx)
}

func (m *mindmapVisual) save(th *look.Theme) (string, error) {
	size := m.viewport
	if size.X <= 0 || size.Y <= 0 {
		return "", errors.New(i18n.T("Open the mind map before saving it."))
	}
	window, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		return "", err
	}
	defer window.Release()
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Now: time.Now(), Metric: m.metric, Constraints: layout.Exact(size)}
	m.canvas(gtx, th, false)
	if err := window.Frame(&ops); err != nil {
		return "", err
	}
	picture := image.NewRGBA(image.Rectangle{Max: size})
	if err := window.Screenshot(picture); err != nil {
		return "", err
	}
	folder := filepath.Join(paths.Home(), "Pictures", "Rashin")
	if err := os.MkdirAll(folder, 0o700); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(folder, "mindmap-*.png")
	if err != nil {
		return "", err
	}
	name := file.Name()
	if err := png.Encode(file, picture); err != nil {
		_ = file.Close()
		_ = os.Remove(name)
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}
