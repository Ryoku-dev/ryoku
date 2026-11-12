package chat

import (
	"fmt"
	"image"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/diffview"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// where a line's text starts, what unfolds hangs from the words and not from the icon
const workIndent = 24

type workVisual struct {
	click  kit.Click
	hover  look.Hover
	unfold look.Unfold
	morph  look.Morph
	open   bool
	seen   bool
	// it was running, so it folds once when it finishes
	ran   bool
	whole bool
	// when this view first saw it running, for the time it has taken
	since time.Time
	// a subagent's time between the reports that carry it
	clock *agentClock
	// a line that opens a popover was pressed, for its owner to open or close it
	pressed bool
}

type workLine struct {
	icon  *widget.Icon
	label string
	// literal is a path or a command, prose is quiet running text
	literal, prose string
	// how long it has run, shown once that is long enough to wonder about
	waited     string
	running    bool
	failed     bool
	expandable bool
	// it opens a popover on a press instead of unfolding
	pops bool
}

func (state *viewState) workVisual(key string) *workVisual {
	visual := state.work[key]
	if visual == nil {
		visual = new(workVisual)
		state.work[key] = visual
	}
	return visual
}

// open while it runs, folded once when it finishes, in between the reader's own toggle stands
func (visual *workVisual) follow(running bool) {
	switch {
	case !visual.seen:
		visual.open, visual.seen = running, true
	case running:
		visual.open = true
	case visual.ran:
		visual.open = false
	}
	visual.ran = running
}

func toolRunning(tool *protocol.ToolData) bool {
	return tool.Status == protocol.ToolRunning || tool.Status == protocol.ToolPending
}

func (state *viewState) layoutLine(gtx layout.Context, th *look.Theme, visual *workVisual, line workLine) layout.Dimensions {
	height := gtx.Dp(32)
	limit := gtx.Constraints.Max.X
	pressable := line.expandable || line.pops
	if visual.click.Clicked(gtx) {
		if line.expandable {
			visual.open = !visual.open
		}
		visual.pressed = line.pops
	}
	reserve := 0
	if line.running {
		reserve += gtx.Dp(24)
	}
	if line.waited != "" {
		reserve += gtx.Dp(72)
	}
	if line.failed {
		reserve += gtx.Dp(80)
	}
	if line.expandable {
		reserve += gtx.Dp(20)
	}
	record := op.Record(gtx.Ops)
	x := 0
	put := func(gap int, widget layout.Widget) {
		x += gap
		dimensions := look.MiddleLeft(gtx, x, height, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = max(0, limit-x-reserve)
			return widget(gtx)
		})
		x += dimensions.Size.X
	}
	hovered := visual.hover.Hovered() && pressable
	ink := th.R.Fg2
	if hovered || visual.open {
		ink = th.R.Fg
	}
	put(0, look.IconWidget(line.icon, 16, th.R.Fg3))
	put(gtx.Dp(8), kit.RawText(th.Theme, th.T.Label, ink, line.label))
	// the parts of a line are set apart by space, never by a separator glyph
	if line.literal != "" {
		put(gtx.Dp(th.Sp.S3), kit.RawText(th.Theme, th.T.MonoSm, th.R.Fg3, line.literal))
	}
	if line.prose != "" {
		put(gtx.Dp(th.Sp.S3), kit.RawText(th.Theme, th.T.Caption, th.R.Fg3, line.prose))
	}
	reserve = 0
	if line.running {
		put(gtx.Dp(8), func(gtx layout.Context) layout.Dimensions {
			return kit.Working{Active: true, Size: 16}.Layout(gtx, th.Theme)
		})
	}
	if line.waited != "" {
		put(gtx.Dp(8), kit.RawText(th.Theme, th.T.Caption, th.R.Fg3, line.waited))
	}
	if line.failed {
		put(gtx.Dp(13), kit.Claim(th.Theme, palette.Failed, i18n.T("Failed")))
	}
	if line.expandable {
		glyph := look.Icon.ExpandMore
		if visual.open {
			glyph = look.Icon.ExpandLess
		}
		put(gtx.Dp(4), func(gtx layout.Context) layout.Dimensions {
			return visual.morph.Layout(gtx, th, glyph, th.R.Fg3, 16)
		})
	}
	content := record.Stop()

	size := image.Pt(x, height)
	dimensions, hover := visual.hover.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
		if !pressable {
			return layout.Dimensions{Size: size}
		}
		return visual.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: size} })
	})
	if pressable && hover > 0 {
		pad := gtx.Dp(8)
		plane := image.Rect(-pad, 0, size.X+pad, size.Y)
		kit.FillCut(gtx.Ops, plane, gtx.Dp(th.Cut.Small), palette.WithAlpha(th.R.Stone3, hover))
	}
	content.Add(gtx.Ops)
	return dimensions
}

func (state *viewState) layoutUnfolding(
	gtx layout.Context,
	th *look.Theme,
	visual *workVisual,
	line workLine,
	detail layout.Widget,
) layout.Dimensions {
	head := state.layoutLine(gtx, th, visual, line)
	if !line.expandable {
		return head
	}
	indent := gtx.Dp(workIndent)
	offset := op.Offset(image.Pt(indent, head.Size.Y)).Push(gtx.Ops)
	body := gtx
	body.Constraints.Max.X = max(0, gtx.Constraints.Max.X-indent)
	body.Constraints.Min = image.Point{}
	unfolded := visual.unfold.Layout(body, th, visual.open, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: th.Sp.S1, Bottom: th.Sp.S2}.Layout(gtx, detail)
	})
	offset.Pop()
	return layout.Dimensions{Size: image.Pt(max(head.Size.X, indent+unfolded.Size.X), head.Size.Y+unfolded.Size.Y)}
}

func (state *viewState) layoutThought(gtx layout.Context, th *look.Theme, key, value string, live bool) layout.Dimensions {
	value = strings.TrimSpace(value)
	visual := state.workVisual(key)
	line := workLine{icon: look.Icon.Thought, label: i18n.T("Thought"), prose: firstLine(value), expandable: strings.Contains(value, "\n") || len(value) > 80}
	if live {
		line.label = i18n.T("Thinking")
	}
	return state.layoutUnfolding(gtx, th, visual, line, func(gtx layout.Context) layout.Dimensions {
		return state.selectable(gtx, th, key, th.T.Body, th.R.Fg2, value)
	})
}

// while any runs each is its own line, once all have finished they fold into one line that counts them
func (state *viewState) layoutTools(gtx layout.Context, th *look.Theme, model *modelState, tools []*protocol.ToolData, key string) layout.Dimensions {
	live := make([]*protocol.ToolData, 0, len(tools))
	running := false
	boxed := false
	for _, tool := range tools {
		if tool != nil {
			live = append(live, tool)
			running = running || toolRunning(tool)
			boxed = boxed || len(tool.Agents) > 0
		}
	}
	if boxed {
		return state.layoutToolRuns(gtx, th, model, live, key)
	}
	switch {
	case len(live) == 0:
		return layout.Dimensions{}
	case len(live) == 1:
		return state.layoutTool(gtx, th, model, live[0], key)
	case running:
		return state.layoutToolLines(gtx, th, model, live, key)
	}
	visual := state.workVisual(key + ":group")
	label, summary := groupSummary(live)
	line := workLine{icon: look.Icon.Steps, label: label, prose: summary, failed: anyFailed(live), expandable: true}
	return state.layoutUnfolding(gtx, th, visual, line, func(gtx layout.Context) layout.Dimensions {
		return state.layoutToolLines(gtx, th, model, live, key)
	})
}

// a call that runs subagents stands as its own box, the plain calls around it
// still fold the way they do without it
func (state *viewState) layoutToolRuns(gtx layout.Context, th *look.Theme, model *modelState, tools []*protocol.ToolData, key string) layout.Dimensions {
	var children []layout.FlexChild
	add := func(widget layout.Widget) {
		if len(children) > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: 2}.Layout))
		}
		children = append(children, layout.Rigid(widget))
	}
	start := 0
	flush := func(end int) {
		if end > start {
			run := tools[start:end]
			add(func(gtx layout.Context) layout.Dimensions {
				return state.layoutTools(gtx, th, model, run, key+":"+run[0].Call)
			})
		}
	}
	for index, tool := range tools {
		if len(tool.Agents) == 0 {
			continue
		}
		flush(index)
		add(func(gtx layout.Context) layout.Dimensions {
			return state.layoutAgents(gtx, th, model, tool, key+":"+tool.Call)
		})
		start = index + 1
	}
	flush(len(tools))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (state *viewState) layoutToolLines(gtx layout.Context, th *look.Theme, model *modelState, tools []*protocol.ToolData, key string) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(tools)*2)
	for index, tool := range tools {
		if index > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: 2}.Layout))
		}
		toolKey := key + ":" + tool.Call
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return state.layoutTool(gtx, th, model, tool, toolKey)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (state *viewState) layoutTool(gtx layout.Context, th *look.Theme, model *modelState, tool *protocol.ToolData, key string) layout.Dimensions {
	if len(tool.Agents) > 0 {
		return state.layoutAgents(gtx, th, model, tool, key)
	}
	visual := state.workVisual(key)
	running := toolRunning(tool)
	// a diff or live output is worth watching, a read isn't, so only those open by themselves while running
	if tool.Diff != nil || tool.Terminal != "" {
		visual.follow(running)
	}
	line := workLine{
		icon: workIcon(tool.Kind), label: workLabel(tool), literal: workLiteral(tool),
		running: running, failed: tool.Status == protocol.ToolFailed, expandable: hasDetail(model, tool),
	}
	// a command that says nothing for a minute looks hung, so the line counts while the turn is live
	if running && state.busy && tool.Kind == protocol.ToolExecute {
		if visual.since.IsZero() {
			visual.since = gtx.Now
		}
		if waited := gtx.Now.Sub(visual.since); waited >= waitedFrom {
			line.waited = waitedFor(waited)
		}
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(time.Second)})
	} else {
		visual.since = time.Time{}
	}
	return state.layoutUnfolding(gtx, th, visual, line, func(gtx layout.Context) layout.Dimensions {
		return state.layoutToolDetail(gtx, th, model, tool, visual, key)
	})
}

const waitedFrom = 3 * time.Second

func waitedFor(waited time.Duration) string {
	seconds := int(waited.Seconds())
	if seconds < 60 {
		return i18n.Tf("%d s", seconds)
	}
	return i18n.Tf("%d min %d s", seconds/60, seconds%60)
}

func (state *viewState) layoutToolDetail(
	gtx layout.Context,
	th *look.Theme,
	model *modelState,
	tool *protocol.ToolData,
	visual *workVisual,
	key string,
) layout.Dimensions {
	if tool.Diff != nil {
		return state.layoutToolDiff(gtx, th, tool, key)
	}
	output := tool.Output
	if terminal := model.terminals[tool.Terminal]; terminal != nil && terminal.text != "" {
		output = terminal.text
	}
	sections := make([]string, 0, 3)
	if input := strings.TrimSpace(tool.Input); input != "" {
		sections = append(sections, input)
	}
	// while it runs the news is at the end of what it printed
	live := -1
	if output = strings.TrimSpace(output); output != "" {
		if toolRunning(tool) {
			live = len(sections)
		}
		sections = append(sections, output)
	}
	if len(sections) == 0 && len(tool.Locations) > 0 {
		sections = append(sections, locationList(tool.Locations))
	}
	return slab(gtx, th, 13, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		children := make([]layout.FlexChild, 0, len(sections)*2+1)
		cut := false
		for index, section := range sections {
			shown, more := section, false
			switch {
			case visual.whole:
			case index == live:
				shown, more = lastLines(section, 12)
			default:
				shown, more = firstLines(section, 12)
			}
			cut = cut || more
			ink := th.R.Fg2
			if index > 0 {
				ink = th.R.Fg3
				children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout))
			}
			sectionKey := fmt.Sprintf("%s:%d", key, index)
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return state.selectable(gtx, th, sectionKey, th.T.Mono, ink, shown)
			}))
		}
		if cut {
			whole := state.plaque(key + ":whole")
			whole.Kind, whole.Small, whole.Label = kit.PlaqueQuiet, true, i18n.T("Show all")
			if whole.Clicked(gtx) {
				visual.whole = true
			}
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return whole.Layout(gtx, th.Theme) }),
			)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (state *viewState) layoutToolDiff(gtx layout.Context, th *look.Theme, tool *protocol.ToolData, key string) layout.Dimensions {
	view := state.diffs[key]
	if view == nil {
		view = new(diffview.View)
		state.diffs[key] = view
	}
	gtx.Constraints.Min = image.Point{}
	shown := func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.Y = gtx.Dp(320)
		result := protocol.DiffResult{Files: []protocol.FileDiff{diffview.FromTexts(tool.Diff.Path, tool.Diff.Old, tool.Diff.New)}}
		record := op.Record(gtx.Ops)
		dimensions := view.Layout(gtx, th, result, diffview.Options{Overlays: state.overlays, FoldOver: 80, Headless: true})
		call := record.Stop()
		bounds, cut := image.Rectangle{Max: dimensions.Size}, gtx.Dp(th.Cut.Mid)
		kit.SlabSurface(gtx, th.Theme, bounds, cut, th.R.Slab)
		call.Add(gtx.Ops)
		return dimensions
	}
	output := strings.TrimSpace(tool.Output)
	if output == "" {
		return shown(gtx)
	}
	// what the tool said about the change, such as where the earlier version was kept
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(shown),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return state.selectable(gtx, th, key+":said", th.T.Caption, th.R.Fg3, output)
		}),
	)
}

func (state *viewState) layoutTerminal(gtx layout.Context, th *look.Theme, terminal *terminalState, key string) layout.Dimensions {
	if terminal == nil {
		return layout.Dimensions{}
	}
	visual := state.workVisual(key)
	visual.follow(!terminal.data.Done)
	line := workLine{
		icon: look.Icon.Terminal, label: i18n.T("Ran"), literal: firstLine(terminal.data.Command),
		running: !terminal.data.Done, failed: terminal.data.Exit != nil && *terminal.data.Exit != 0,
		expandable: strings.TrimSpace(terminal.text) != "",
	}
	return state.layoutUnfolding(gtx, th, visual, line, func(gtx layout.Context) layout.Dimensions {
		return slab(gtx, th, 13, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			output, _ := firstLines(terminal.text, 24)
			return state.selectable(gtx, th, key, th.T.Mono, th.R.Fg2, output)
		})
	})
}

func (state *viewState) layoutRecord(gtx layout.Context, th *look.Theme, key string, icon *widget.Icon, label, literal string) layout.Dimensions {
	return state.layoutLine(gtx, th, state.workVisual(key), workLine{icon: icon, label: label, literal: literal})
}

func anyFailed(tools []*protocol.ToolData) bool {
	for _, tool := range tools {
		if tool.Status == protocol.ToolFailed {
			return true
		}
	}
	return false
}

func groupSummary(tools []*protocol.ToolData) (label, summary string) {
	var reads, commands, edits, others int
	for _, tool := range tools {
		switch tool.Kind {
		case protocol.ToolRead, protocol.ToolSearch, protocol.ToolFetch:
			reads++
		case protocol.ToolExecute:
			commands++
		case protocol.ToolEdit, protocol.ToolDelete, protocol.ToolMove:
			edits++
		default:
			others++
		}
	}
	parts := make([]string, 0, 4)
	add := func(count int, one, many string) {
		switch {
		case count == 1:
			parts = append(parts, one)
		case count > 1:
			parts = append(parts, i18n.Tf(many, count))
		}
	}
	add(reads, i18n.T("1 read"), "%d reads")
	add(commands, i18n.T("1 command"), "%d commands")
	add(edits, i18n.T("1 edit"), "%d edits")
	add(others, i18n.T("1 other"), "%d others")
	return i18n.Tf("%d steps", len(tools)), strings.Join(parts, ", ")
}

func workIcon(kind string) *widget.Icon {
	switch kind {
	case protocol.ToolRead:
		return look.Icon.File
	case protocol.ToolSearch:
		return look.Icon.Search
	case protocol.ToolFetch:
		return look.Icon.Link
	case protocol.ToolEdit, protocol.ToolMove:
		return look.Icon.Rename
	case protocol.ToolDelete:
		return look.Icon.Delete
	case protocol.ToolExecute:
		return look.Icon.Terminal
	case protocol.ToolThink:
		return look.Icon.Thought
	default:
		return look.Icon.Steps
	}
}

func workLabel(tool *protocol.ToolData) string {
	if title := strings.TrimSpace(tool.Title); title != "" {
		return title
	}
	switch tool.Kind {
	case protocol.ToolRead:
		return i18n.T("Read")
	case protocol.ToolSearch:
		return i18n.T("Searched")
	case protocol.ToolFetch:
		return i18n.T("Fetched")
	case protocol.ToolEdit:
		return i18n.T("Edited")
	case protocol.ToolDelete:
		return i18n.T("Deleted")
	case protocol.ToolMove:
		return i18n.T("Moved")
	case protocol.ToolExecute:
		return i18n.T("Ran")
	case protocol.ToolThink:
		return i18n.T("Thought")
	default:
		return i18n.T("Worked")
	}
}

func workLiteral(tool *protocol.ToolData) string {
	switch {
	case len(tool.Locations) > 0:
		return locationText(tool.Locations[0])
	case tool.Diff != nil:
		return tool.Diff.Path
	case tool.Kind == protocol.ToolExecute:
		return firstLine(tool.Input)
	default:
		return ""
	}
}

func locationText(location protocol.Location) string {
	if location.Line > 0 {
		return fmt.Sprintf("%s:%d", location.Path, location.Line)
	}
	return location.Path
}

func locationList(locations []protocol.Location) string {
	lines := make([]string, len(locations))
	for index, location := range locations {
		lines[index] = locationText(location)
	}
	return strings.Join(lines, "\n")
}

// a terminal counts once it has printed something, an empty one would unfold to nothing
func hasDetail(model *modelState, tool *protocol.ToolData) bool {
	if terminal := model.terminals[tool.Terminal]; terminal != nil && strings.TrimSpace(terminal.text) != "" {
		return true
	}
	return tool.Diff != nil || strings.TrimSpace(tool.Output) != "" ||
		strings.Contains(strings.TrimSpace(tool.Input), "\n") || len(tool.Locations) > 1
}
