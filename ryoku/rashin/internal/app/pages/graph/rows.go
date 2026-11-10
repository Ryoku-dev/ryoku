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
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/gitgraph"
	"ryoku-rashin/internal/protocol"
)

const (
	lanePitch         = 14
	graphRowHeight    = 30
	graphHeaderHeight = 28
	maxRefChips       = 3
	// room between a row's slab and the list's right edge
	rowEdge = 8
)

type columns struct {
	lanes, text, author, date, hash, end int
}

func (v *viewState) columns(gtx layout.Context, width int) columns {
	author, date, hash := commitColumnWidths(gtx, width)
	lanes := v.laneColumnWidth(gtx)
	result := columns{lanes: lanes, text: lanes + gtx.Dp(8), end: width - gtx.Dp(rowEdge) - gtx.Dp(8)}
	result.hash = result.end - hash
	result.date = result.hash - date
	result.author = result.date - author
	return result
}

// end sets the widget against the right edge by laying it out twice, so it's for text only
func cell(gtx layout.Context, left, right, height int, end bool, widget layout.Widget) {
	if right <= left {
		return
	}
	child := gtx
	child.Constraints = layout.Constraints{Max: image.Pt(right-left, height)}
	x := left
	if end {
		record := op.Record(gtx.Ops)
		x = right - widget(child).Size.X
		record.Stop()
	}
	look.MiddleLeft(child, x, height, widget)
}

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
	// a worktree of the same name has this branch checked out
	checkedOut bool
}

func (v *viewState) layoutEntry(gtx layout.Context, e *env.Env, index int) layout.Dimensions {
	th := e.Theme
	entry := v.entries[index]
	row := v.row(entry.id())
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(graphRowHeight))
	gtx.Constraints = layout.Exact(size)
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

	cols := v.columns(gtx, size.X)
	hover := look.Transition(gtx, &row.hover, row.input.hovered && !selected, th.Motion.HoverIn, th.Motion.HoverOut)
	if hover > 0 {
		inset := max(1, gtx.Dp(1))
		slab := image.Rect(cols.lanes, inset, size.X-gtx.Dp(rowEdge), size.Y-inset)
		kit.FillCut(gtx.Ops, slab, gtx.Dp(th.Cut.Small), palette.WithAlpha(th.R.Stone3, hover))
	}
	row.input.register(gtx, size)

	look.Place(gtx, image.Rect(0, 0, cols.lanes, size.Y), func(gtx layout.Context) layout.Dimensions {
		return v.layoutLanes(gtx, e, row, entry)
	})
	ink, quiet, faint := th.R.Fg, th.R.Fg2, th.R.Fg3
	if selected {
		ink, quiet, faint = th.R.OnSel, th.R.OnSel2, th.R.OnSel2
	}
	if entry.kind == rowWIP {
		cell(gtx, cols.text, cols.end, size.Y, false, func(gtx layout.Context) layout.Dimensions {
			return v.layoutWIPText(gtx, e, entry.wip, selected)
		})
		return layout.Dimensions{Size: size}
	}
	gap := gtx.Dp(13)
	cell(gtx, cols.text, cols.author-gap, size.Y, false, func(gtx layout.Context) layout.Dimensions {
		return v.layoutCommitDescription(gtx, e, row, entry, ink)
	})
	cell(gtx, cols.author, cols.date-gap, size.Y, false, kit.RawText(th.Theme, th.T.Caption, quiet, entry.commit.Author))
	cell(gtx, cols.date, cols.hash-gap, size.Y, true, kit.RawText(th.Theme, th.T.Caption, faint, relativeTime(entry.commit.Time, gtx.Now)))
	cell(gtx, cols.hash, cols.end, size.Y, false, kit.RawText(th.Theme, th.T.MonoSm, faint, shortHash(entry.commit.Hash)))
	return layout.Dimensions{Size: size}
}

func (v *viewState) layoutLanes(gtx layout.Context, e *env.Env, row *rowState, entry graphEntry) layout.Dimensions {
	size := gtx.Constraints.Max
	if entry.kind == rowWIP {
		v.drawWIPGraph(gtx, e, entry, row)
	} else {
		v.drawCommitGraph(gtx, e, entry.layout, row)
	}
	if v.expandedLanes || entry.layout.Width <= collapsedLanes {
		return layout.Dimensions{Size: size}
	}
	v.laneMarkerVisible = true
	if row.expand.Clicked(gtx) {
		v.expandedLanes = true
		gtx.Execute(op.InvalidateCmd{})
	}
	hidden := entry.layout.Width - collapsedLanes
	marker := image.Pt(gtx.Dp(30), gtx.Dp(18))
	at := image.Pt(max(0, size.X-marker.X-gtx.Dp(3)), (size.Y-marker.Y)/2)
	look.Place(gtx, image.Rectangle{Min: at, Max: at.Add(marker)}, func(gtx layout.Context) layout.Dimensions {
		return row.expand.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return drawChip(gtx, e, nil, i18n.Tf("+%d", hidden), e.Theme.R.Stone4, e.Theme.R.Fg2, color.NRGBA{})
		})
	})
	return layout.Dimensions{Size: size}
}

func (v *viewState) layoutCommitDescription(gtx layout.Context, e *env.Env, row *rowState, entry graphEntry, ink color.NRGBA) layout.Dimensions {
	th := e.Theme
	refs := v.refsFor(entry.commit.Hash)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	maxChips := min(gtx.Constraints.Max.X*3/5, gtx.Dp(420))
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = max(0, maxChips)
			return v.layoutRefChips(gtx, e, refs, entry.layout.Color, &row.refTip)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if len(refs) == 0 {
				return layout.Dimensions{}
			}
			return layout.Spacer{Width: th.Sp.S2}.Layout(gtx)
		}),
		layout.Flexed(1, kit.RawText(th.Theme, th.T.Body, ink, entry.commit.Subject)),
	)
}

func (v *viewState) layoutWIPText(gtx layout.Context, e *env.Env, wip protocol.WipRow, selected bool) layout.Dimensions {
	th := e.Theme
	ink, faint := th.R.Fg2, th.R.Fg3
	if selected {
		ink, faint = th.R.OnSel, th.R.OnSel2
	}
	counts := strings.Join(nonZeroCounts(wip.Staged, wip.Modified, wip.Untracked, wip.Conflicted), ", ")
	agent := strings.TrimSpace(wip.Agent)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(kit.RawText(th.Theme, th.T.Body, ink, i18n.Tf("Uncommitted in %s", v.worktreeLabel(wip.Worktree)))),
		layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
		layout.Flexed(1, kit.RawText(th.Theme, th.T.Caption, faint, counts)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			switch {
			case agent == "":
				return layout.Dimensions{}
			case selected:
				return kit.RawText(th.Theme, th.T.Label, th.R.OnSel2, agentLabel(agent))(gtx)
			default:
				return kit.Claim(th.Theme, agentTone(agent), agentLabel(agent))(gtx)
			}
		}),
	)
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
					return drawChip(gtx, e, nil, i18n.Tf("+%d", len(rest)), e.Theme.R.Stone4, e.Theme.R.Fg2, color.NRGBA{})
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
		chip := refChip{label: ref.Name, kind: chipBranch}
		if ref.Worktree == "" {
			chips = append(chips, chip)
			continue
		}
		// a worktree named after its branch is a mark on the branch chip, a second chip would just repeat it
		name := v.worktreeLabel(ref.Worktree)
		chip.checkedOut = name == ref.Name || strings.HasSuffix(ref.Name, "/"+name)
		chips = append(chips, chip)
		if !chip.checkedOut {
			chips = append(chips, refChip{label: name, kind: chipWorktree})
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

// opaque fill so a chip reads the same on the pane and on the selected row's slab
func (v *viewState) layoutRefChip(gtx layout.Context, e *env.Env, chip refChip, lane int) layout.Dimensions {
	th := e.Theme
	fill, ink, outline := th.R.Stone1, th.R.Fg2, color.NRGBA{}
	var icon *widget.Icon
	switch chip.kind {
	case chipBranch:
		ink = laneColor(e, lane)
		fill = palette.Mix(th.R.Stone1, ink, .18)
		if chip.checkedOut {
			icon = look.Icon.Folder
		}
	case chipRemote:
		outline = th.R.Vein
	case chipTag:
		ink = th.R.Amber
		fill = palette.Mix(th.R.Stone1, ink, .18)
	case chipHead:
		fill, ink = th.R.Fg, th.R.Stone1
	case chipWorktree:
		fill, icon = th.R.Stone4, look.Icon.Folder
	}
	return drawChip(gtx, e, icon, chip.label, fill, ink, outline)
}

func drawChip(gtx layout.Context, e *env.Env, icon *widget.Icon, label string, fill, ink, outline color.NRGBA) layout.Dimensions {
	th := e.Theme
	height, inset := gtx.Dp(18), gtx.Dp(6)
	lead := 0
	if icon != nil {
		lead = gtx.Dp(12 + 4)
	}
	child := gtx
	child.Constraints = layout.Constraints{Max: image.Pt(max(0, min(gtx.Constraints.Max.X, gtx.Dp(168))-2*inset-lead), height)}
	record := op.Record(gtx.Ops)
	text := look.MiddleLeft(child, inset+lead, height, kit.RawText(th.Theme, th.T.MonoSm, ink, label))
	if icon != nil {
		look.PlaceIcon(gtx, icon, image.Pt(inset, (height-gtx.Dp(12))/2), 12, ink)
	}
	call := record.Stop()
	bounds := image.Rect(0, 0, text.Size.X+2*inset+lead, height)
	cut := gtx.Dp(th.Cut.Token)
	if fill.A != 0 {
		kit.FillCut(gtx.Ops, bounds, cut, fill)
	}
	if outline.A != 0 {
		kit.StrokeCut(gtx.Ops, bounds, cut, max(1, gtx.Dp(1)), outline)
	}
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: bounds.Max}
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

func agentTone(state string) palette.Tone {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "working", "starting", "creating", "running":
		return palette.Becoming
	case "waiting", "paused", "needs you":
		return palette.Attention
	case "failed", "error", "stopped unexpectedly":
		return palette.Failed
	case "ready", "connected", "online":
		return palette.Alive
	default:
		return palette.Idle
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

// a narrow graph drops the author before it squeezes the description
func commitColumnWidths(gtx layout.Context, width int) (author, date, hash int) {
	switch {
	case width >= gtx.Dp(700):
		return gtx.Dp(132), gtx.Dp(100), gtx.Dp(76)
	case width >= gtx.Dp(520):
		return gtx.Dp(104), gtx.Dp(96), gtx.Dp(72)
	default:
		return 0, gtx.Dp(92), gtx.Dp(72)
	}
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
