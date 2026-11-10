package graph

import (
	"image"
	"image/color"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/gitgraph"
	"ryoku-rashin/internal/protocol"
)

const (
	lanePitch         = 14
	graphRowHeight    = 30
	graphHeaderHeight = 26
	maxRefChips       = 3
)

type rowInput struct {
	tag       struct{}
	primary   bool
	secondary bool
	hovered   bool
	pointerID pointer.ID
}

func (input *rowInput) poll(gtx layout.Context, primary, secondary func()) {
	for {
		raw, ok := gtx.Event(pointer.Filter{
			Target: &input.tag,
			Kinds:  pointer.Enter | pointer.Leave | pointer.Press | pointer.Release | pointer.Cancel,
		})
		if !ok {
			return
		}
		e := raw.(pointer.Event)
		switch e.Kind {
		case pointer.Enter:
			input.hovered = true
		case pointer.Leave:
			input.hovered = false
		case pointer.Press:
			input.primary = e.Buttons.Contain(pointer.ButtonPrimary)
			input.secondary = e.Buttons.Contain(pointer.ButtonSecondary)
			input.pointerID = e.PointerID
		case pointer.Release:
			if e.PointerID != input.pointerID {
				continue
			}
			if input.secondary {
				secondary()
			} else if input.primary {
				primary()
			}
			input.primary, input.secondary = false, false
		case pointer.Cancel:
			if e.PointerID == input.pointerID {
				input.primary, input.secondary = false, false
			}
		}
	}
}

func (input *rowInput) register(gtx layout.Context, size image.Point) {
	area := clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops)
	event.Op(gtx.Ops, &input.tag)
	area.Pop()
}

type laneStroke struct {
	color  int
	points [5]f32.Point
	count  int
}

type graphPathCache struct {
	expanded bool
	width    int
	height   int
	scale    float32
	wip      bool
	strokes  []laneStroke
}

type refChipKind uint8

const (
	chipBranch refChipKind = iota
	chipRemote
	chipTag
	chipHead
	chipWorktree
)

type refChip struct {
	label string
	kind  refChipKind
	agent string
}

func (v *viewState) layoutEntry(gtx layout.Context, e *env.Env, index int) layout.Dimensions {
	entry := v.entries[index]
	row := v.row(entry.id())
	height := gtx.Dp(graphRowHeight)
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = height, height
	selected := entry.kind == v.selected.kind &&
		((entry.kind == rowCommit && entry.commit.Hash == v.selected.hash) ||
			(entry.kind == rowWIP && entry.wip.Worktree == v.selected.worktree))
	row.input.poll(gtx, func() {
		v.selectEntry(e, entry)
		if !v.sideDetails {
			v.detailOpen = true
		}
		gtx.Execute(key.FocusCmd{Tag: &v.keyTag})
	}, func() {
		v.selectEntry(e, entry)
		v.openRowMenu(gtx, e, row, entry)
		gtx.Execute(key.FocusCmd{Tag: &v.keyTag})
	})

	bounds := image.Rect(0, 0, gtx.Constraints.Max.X, height)
	if selected {
		paint.FillShape(gtx.Ops, e.Theme.R.Stone3, clip.Rect(bounds).Op())
		bar := max(1, gtx.Dp(3))
		paint.FillShape(gtx.Ops, e.Theme.R.Vein, clip.Rect(image.Rect(0, 0, bar, height)).Op())
	} else if row.input.hovered {
		hover := palette.Mix(e.Theme.R.Stone2, e.Theme.R.Fg, .06)
		paint.FillShape(gtx.Ops, hover, clip.Rect(bounds).Op())
	}
	row.input.register(gtx, bounds.Max)

	ink, quiet := e.Theme.R.Fg, e.Theme.R.Fg2
	if selected {
		ink, quiet = e.Theme.R.Fg, e.Theme.R.Fg2
	}
	laneWidth := v.laneColumnWidth(gtx)
	authorWidth, dateWidth, hashWidth := commitColumnWidths(gtx)
	if entry.kind == rowWIP {
		authorWidth, dateWidth, hashWidth = 0, 0, 0
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = laneWidth, laneWidth
			if entry.kind == rowWIP {
				v.drawWIPGraph(gtx, e, entry, row)
			} else {
				v.drawCommitGraph(gtx, e, entry.layout, row)
			}
			if !v.expandedLanes && entry.layout.Width > collapsedLanes {
				v.laneMarkerVisible = true
				if row.expand.Clicked(gtx) {
					v.expandedLanes = true
					gtx.Execute(op.InvalidateCmd{})
				}
				hidden := entry.layout.Width - collapsedLanes
				return layout.Stack{}.Layout(gtx,
					layout.Expanded(func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{Size: image.Pt(laneWidth, height)}
					}),
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						markerWidth := gtx.Dp(30)
						offset := op.Offset(image.Pt(max(0, laneWidth-markerWidth-3), gtx.Dp(6))).Push(gtx.Ops)
						child := gtx
						child.Constraints.Min = image.Pt(markerWidth, gtx.Dp(18))
						child.Constraints.Max = child.Constraints.Min
						dimensions := row.expand.Layout(child, func(gtx layout.Context) layout.Dimensions {
							return drawChip(gtx, e, i18n.Tf("+%d", hidden), e.Theme.R.Stone4, quiet, e.Theme.R.Vein)
						})
						offset.Pop()
						return dimensions
					}),
				)
			}
			return layout.Dimensions{Size: image.Pt(laneWidth, height)}
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if entry.kind == rowWIP {
				return v.layoutWIPText(gtx, e, entry.wip, ink, quiet)
			}
			return v.layoutCommitDescription(gtx, e, row, entry, ink)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if entry.kind == rowWIP {
				return layout.Dimensions{Size: image.Pt(authorWidth, height)}
			}
			return columnText(gtx, e, authorWidth, e.Theme.T.Caption, ink, entry.commit.Author)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if entry.kind == rowWIP {
				return layout.Dimensions{Size: image.Pt(dateWidth, height)}
			}
			return columnText(gtx, e, dateWidth, e.Theme.T.MonoSm, quiet, relativeTime(entry.commit.Time, time.Now()))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if entry.kind == rowWIP {
				return layout.Dimensions{Size: image.Pt(hashWidth, height)}
			}
			return columnText(gtx, e, hashWidth, e.Theme.T.MonoSm, quiet, shortHash(entry.commit.Hash))
		}),
	)
}

func (v *viewState) layoutCommitDescription(gtx layout.Context, e *env.Env, row *rowState, entry graphEntry, ink color.NRGBA) layout.Dimensions {
	refs := v.refsFor(entry.commit.Hash)
	maxChips := min(gtx.Constraints.Max.X/2, gtx.Dp(360))
	return layout.Inset{Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Max.X = max(0, maxChips)
				return v.layoutRefChips(gtx, e, refs, entry.layout.Color, &row.refTip)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if len(refs) == 0 {
					return layout.Dimensions{}
				}
				return layout.Spacer{Width: 6}.Layout(gtx)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return kit.Text(e.Theme.Theme, e.Theme.T.Row, ink, entry.commit.Subject)(gtx)
			}),
		)
	})
}

func (v *viewState) layoutWIPText(gtx layout.Context, e *env.Env, wip protocol.WipRow, ink, quiet color.NRGBA) layout.Dimensions {
	name := v.worktreeLabel(wip.Worktree)
	counts := strings.Join(nonZeroCounts(wip.Staged, wip.Modified, wip.Untracked, wip.Conflicted), " · ")
	agent := strings.TrimSpace(wip.Agent)
	return layout.Inset{Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, kit.Text(e.Theme.Theme, e.Theme.T.Row, ink, i18n.T("Uncommitted changes")+" · "+name)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if counts == "" {
					return layout.Dimensions{}
				}
				return layout.Inset{Left: 8}.Layout(gtx, kit.Text(e.Theme.Theme, e.Theme.T.MonoSm, quiet, counts))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if agent == "" {
					return layout.Dimensions{}
				}
				return layout.Inset{Left: 8}.Layout(gtx, kit.Text(e.Theme.Theme, e.Theme.T.Label, agentColor(e, agent), agentLabel(agent)))
			}),
		)
	})
}

func nonZeroCounts(staged, modified, untracked, conflicted int) []string {
	counts := make([]string, 0, 4)
	if staged != 0 {
		counts = append(counts, i18n.Tf("%d staged", staged))
	}
	if modified != 0 {
		counts = append(counts, i18n.Tf("%d modified", modified))
	}
	if untracked != 0 {
		counts = append(counts, i18n.Tf("%d untracked", untracked))
	}
	if conflicted != 0 {
		counts = append(counts, i18n.Tf("%d conflicted", conflicted))
	}
	return counts
}

func (v *viewState) layoutRefChips(gtx layout.Context, e *env.Env, refs []protocol.GitRef, lane int, tip *look.Tip) layout.Dimensions {
	chips := v.refChips(refs)
	if len(chips) == 0 {
		return layout.Dimensions{}
	}
	shown := min(maxRefChips, len(chips))
	children := make([]layout.FlexChild, 0, shown*2)
	for index := range shown {
		chip := chips[index]
		if index != 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: 3}.Layout))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.layoutRefChip(gtx, e, chip, lane)
		}))
	}
	if len(chips) > shown {
		rest := chips[shown:]
		labels := make([]string, len(rest))
		for index := range rest {
			labels[index] = rest[index].label
		}
		children = append(children,
			layout.Rigid(layout.Spacer{Width: 3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				body := func(gtx layout.Context) layout.Dimensions {
					return drawChip(gtx, e, i18n.Tf("+%d", len(rest)), e.Theme.R.Stone4, e.Theme.R.Fg2, e.Theme.R.Vein)
				}
				return tip.Layout(gtx, e.Theme, e.Overlays, strings.Join(labels, " · "), body)
			}),
		)
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

func (v *viewState) refChips(refs []protocol.GitRef) []refChip {
	chips := make([]refChip, 0, len(refs)*2)
	for _, ref := range refs {
		if ref.Kind == protocol.RefHead || ref.Current {
			chips = append(chips, refChip{label: i18n.T("HEAD"), kind: chipHead})
			break
		}
	}
	for _, ref := range refs {
		if ref.Kind != protocol.RefBranch {
			continue
		}
		chips = append(chips, refChip{label: ref.Name, kind: chipBranch})
		if ref.Worktree != "" {
			chips = append(chips, refChip{
				label: v.worktreeLabel(ref.Worktree),
				kind:  chipWorktree,
				agent: v.worktreeAgent(ref.Worktree),
			})
		}
	}
	for _, ref := range refs {
		if ref.Kind == protocol.RefTag {
			chips = append(chips, refChip{label: ref.Name, kind: chipTag})
		}
	}
	for _, ref := range refs {
		if ref.Kind == protocol.RefRemote {
			chips = append(chips, refChip{label: ref.Name, kind: chipRemote})
		}
	}
	return chips
}

func (v *viewState) layoutRefChip(gtx layout.Context, e *env.Env, chip refChip, lane int) layout.Dimensions {
	fill, ink, outline := color.NRGBA{}, e.Theme.R.Fg2, color.NRGBA{}
	switch chip.kind {
	case chipBranch:
		ink = laneColor(e, lane)
		fill = palette.WithAlpha(ink, .16)
	case chipRemote:
		ink, outline = e.Theme.R.Fg2, e.Theme.R.Vein
	case chipTag:
		fill = e.Theme.R.Field(palette.Attention)
		ink = e.Theme.R.Amber
	case chipHead:
		fill, ink = e.Theme.R.Fg, e.Theme.R.Stone1
	case chipWorktree:
		ink = e.Theme.R.Fg2
		fill = e.Theme.R.Stone4
		if chip.agent != "" {
			ink = agentColor(e, chip.agent)
			fill = palette.WithAlpha(ink, .14)
			chip.label += " · " + agentLabel(chip.agent)
		}
	}
	return drawChip(gtx, e, chip.label, fill, ink, outline)
}

func drawChip(gtx layout.Context, e *env.Env, label string, fill, ink, outline color.NRGBA) layout.Dimensions {
	height := gtx.Dp(18)
	child := gtx
	child.Constraints.Min.Y, child.Constraints.Max.Y = height, height
	child.Constraints.Max.X = min(child.Constraints.Max.X, gtx.Dp(132))
	record := op.Record(gtx.Ops)
	dimensions := layout.Inset{Right: 5, Left: 5}.Layout(child, func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, kit.Text(e.Theme.Theme, e.Theme.T.MonoSm, ink, label))
	})
	call := record.Stop()
	bounds := image.Rectangle{Max: dimensions.Size}
	cut := min(gtx.Dp(3), dimensions.Size.Y/2)
	if fill.A != 0 {
		kit.FillCut(gtx.Ops, bounds, cut, fill)
	}
	if outline.A != 0 {
		kit.StrokeCut(gtx.Ops, bounds, cut, max(1, gtx.Dp(1)), outline)
	}
	call.Add(gtx.Ops)
	return dimensions
}

func (v *viewState) drawCommitGraph(gtx layout.Context, e *env.Env, graph gitgraph.Row, row *rowState) {
	area := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	for _, stroke := range v.cachedGraphPaths(gtx, row, graph, false) {
		drawCachedStroke(gtx, laneColor(e, stroke.color), stroke)
	}
	lane, visible := v.visibleLane(graph.Lane, graph.Width)
	if !visible {
		area.Pop()
		return
	}
	center := f32.Pt(float32(gtx.Dp(unit.Dp(9+lane*lanePitch))), float32(gtx.Constraints.Max.Y)/2)
	ink := laneColor(e, graph.Color)
	head := v.isHead(graph.Commit)
	if head {
		strokeSquare(gtx, center, float32(gtx.Dp(14)), float32(max(1, gtx.Dp(2))), ink)
	}
	if graph.IsMerge {
		strokeSquare(gtx, center, float32(gtx.Dp(9)), float32(max(1, gtx.Dp(2))), ink)
	} else {
		size := gtx.Dp(7)
		if head {
			size = gtx.Dp(9)
		}
		paint.FillShape(gtx.Ops, ink, clip.Rect(image.Rect(
			int(center.X)-size/2,
			int(center.Y)-size/2,
			int(center.X)+(size+1)/2,
			int(center.Y)+(size+1)/2,
		)).Op())
	}
	area.Pop()
}

func (v *viewState) drawWIPGraph(gtx layout.Context, e *env.Env, entry graphEntry, row *rowState) {
	area := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	for _, stroke := range v.cachedGraphPaths(gtx, row, entry.layout, true) {
		drawCachedStroke(gtx, laneColor(e, stroke.color), stroke)
	}
	lane, visible := v.visibleLane(entry.layout.Lane, entry.layout.Width)
	if !visible {
		area.Pop()
		return
	}
	center := f32.Pt(float32(gtx.Dp(unit.Dp(9+lane*lanePitch))), float32(gtx.Constraints.Max.Y)/2)
	strokeSquare(gtx, center, float32(gtx.Dp(10)), float32(max(1, gtx.Dp(2))), laneColor(e, entry.layout.Color))
	area.Pop()
}

func (v *viewState) cachedGraphPaths(gtx layout.Context, row *rowState, graph gitgraph.Row, wip bool) []laneStroke {
	cache := &row.paths
	if cache.strokes != nil &&
		cache.expanded == v.expandedLanes &&
		cache.width == graph.Width &&
		cache.height == gtx.Constraints.Max.Y &&
		cache.scale == gtx.Metric.PxPerDp &&
		cache.wip == wip {
		return cache.strokes
	}
	cache.expanded = v.expandedLanes
	cache.width = graph.Width
	cache.height = gtx.Constraints.Max.Y
	cache.scale = gtx.Metric.PxPerDp
	cache.wip = wip
	cache.strokes = cache.strokes[:0]
	height := float32(gtx.Constraints.Max.Y)
	center := height / 2
	if wip {
		lane, visible := v.visibleLane(graph.Lane, graph.Width)
		if !visible {
			return cache.strokes
		}
		x := float32(gtx.Dp(unit.Dp(9 + lane*lanePitch)))
		dash := float32(max(2, gtx.Dp(4)))
		gap := float32(max(1, gtx.Dp(3)))
		for y := center + float32(gtx.Dp(6)); y < height; y += dash + gap {
			end := min(y+dash, height)
			cache.strokes = append(cache.strokes, laneStroke{
				color:  graph.Color,
				points: [5]f32.Point{f32.Pt(x, y), f32.Pt(x, end)},
				count:  2,
			})
		}
		return cache.strokes
	}
	for _, segment := range graph.Above {
		cache.strokes = append(cache.strokes, v.lanePath(gtx, segment, 0, center, graph.Width))
	}
	for _, segment := range graph.Segments {
		cache.strokes = append(cache.strokes, v.lanePath(gtx, segment, center, height, graph.Width))
	}
	return cache.strokes
}

func (v *viewState) lanePath(gtx layout.Context, segment gitgraph.Segment, top, bottom float32, width int) laneStroke {
	from, fromVisible := v.visibleLane(segment.From, width)
	to, toVisible := v.visibleLane(segment.To, width)
	if !fromVisible && !toVisible {
		return laneStroke{}
	}
	x1 := float32(gtx.Dp(unit.Dp(9 + from*lanePitch)))
	x2 := float32(gtx.Dp(unit.Dp(9 + to*lanePitch)))
	return laneStroke{
		color:  segment.Color,
		points: [5]f32.Point{f32.Pt(x1, top), f32.Pt(x2, bottom)},
		count:  2,
	}
}

func drawCachedStroke(gtx layout.Context, ink color.NRGBA, stroke laneStroke) {
	if stroke.count < 2 {
		return
	}
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(stroke.points[0])
	for index := 1; index < stroke.count; index++ {
		path.LineTo(stroke.points[index])
	}
	paint.FillShape(gtx.Ops, ink, clip.Stroke{
		Path:  path.End(),
		Width: float32(max(1, gtx.Dp(2))),
	}.Op())
}

func strokeSquare(gtx layout.Context, center f32.Point, size, width float32, ink color.NRGBA) {
	half := size / 2
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(f32.Pt(center.X-half, center.Y-half))
	path.LineTo(f32.Pt(center.X+half, center.Y-half))
	path.LineTo(f32.Pt(center.X+half, center.Y+half))
	path.LineTo(f32.Pt(center.X-half, center.Y+half))
	path.Close()
	paint.FillShape(gtx.Ops, ink, clip.Stroke{Path: path.End(), Width: width}.Op())
}

func (v *viewState) visibleLane(lane, width int) (int, bool) {
	if v.expandedLanes || width <= collapsedLanes || lane < collapsedLanes {
		return lane, true
	}
	return collapsedLanes, false
}

func (v *viewState) graphLaneCount() int {
	width := 1
	for _, entry := range v.entries {
		width = max(width, entry.layout.Width)
	}
	if !v.expandedLanes {
		width = min(width, collapsedLanes)
	}
	return width
}

func (v *viewState) laneColumnWidth(gtx layout.Context) int {
	width := gtx.Dp(unit.Dp(18 + v.graphLaneCount()*lanePitch))
	if !v.expandedLanes {
		for _, entry := range v.entries {
			if entry.layout.Width > collapsedLanes {
				return width + gtx.Dp(32)
			}
		}
	}
	return max(gtx.Dp(48), width)
}

func (v *viewState) isHead(hash string) bool {
	for _, ref := range v.refs {
		if ref.Target == hash && (ref.Kind == protocol.RefHead || ref.Current) {
			return true
		}
	}
	return false
}

func (v *viewState) worktreeLabel(path string) string {
	if worktree, ok := v.worktree(path); ok && worktree.Name != "" {
		return worktree.Name
	}
	name := filepath.Base(path)
	if name == "." || name == string(filepath.Separator) || name == "" {
		return path
	}
	return name
}

func (v *viewState) worktreeAgent(path string) string {
	for _, row := range v.wip {
		if row.Worktree == path {
			return row.Agent
		}
	}
	return ""
}

func agentColor(e *env.Env, state string) color.NRGBA {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "working", "starting", "creating", "running":
		return e.Theme.R.Lapis
	case "waiting", "paused", "needs you":
		return e.Theme.R.Amber
	case "failed", "error", "stopped unexpectedly":
		return e.Theme.R.Jasper
	case "ready", "connected", "online":
		return e.Theme.R.Jade
	default:
		return e.Theme.R.Fg2
	}
}

func agentLabel(state string) string {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "working", "starting", "creating":
		return i18n.T("Working")
	case "running", "ready", "connected", "online":
		return i18n.T("Ready")
	case "waiting", "paused", "needs you":
		return i18n.T("Waiting")
	case "failed", "error", "stopped unexpectedly":
		return i18n.T("Failed")
	case "stopped":
		return i18n.T("Stopped")
	default:
		return i18n.T(state)
	}
}

func laneColor(e *env.Env, index int) color.NRGBA {
	colors := [...]color.NRGBA{e.Theme.R.Jade, e.Theme.R.Lapis, e.Theme.R.Amber, e.Theme.R.Jasper}
	if index < 0 {
		index = -index
	}
	return colors[index%len(colors)]
}

func commitColumnWidths(gtx layout.Context) (author, date, hash int) {
	width := gtx.Constraints.Max.X
	switch {
	case width >= gtx.Dp(700):
		return gtx.Dp(140), gtx.Dp(96), gtx.Dp(72)
	case width >= gtx.Dp(560):
		return gtx.Dp(112), gtx.Dp(88), gtx.Dp(68)
	default:
		return gtx.Dp(96), gtx.Dp(78), gtx.Dp(64)
	}
}

func columnText(gtx layout.Context, e *env.Env, width int, style theme.TextStyle, ink color.NRGBA, value string) layout.Dimensions {
	gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
	return layout.Inset{Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, kit.Text(e.Theme.Theme, style, ink, value))
	})
}

func relativeTime(value, now time.Time) string {
	if value.IsZero() {
		return i18n.T("Unknown")
	}
	delta := now.Sub(value)
	if delta < time.Minute {
		return i18n.T("Now")
	}
	if delta < time.Hour {
		return i18n.Tf("%d min ago", int(delta/time.Minute))
	}
	if delta < 24*time.Hour {
		return i18n.Tf("%d hr ago", int(delta/time.Hour))
	}
	if delta < 7*24*time.Hour {
		return i18n.Tf("%d d ago", int(delta/(24*time.Hour)))
	}
	return value.Local().Format("2006-01-02")
}
