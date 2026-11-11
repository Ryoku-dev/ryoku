package chat

import (
	"image"
	"image/color"
	"strconv"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/diffview"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type viewState struct {
	scroller transcriptScroller
	follow   bool
	ready    bool
	// a turn is running, and this conversation can run a command for the reader
	busy, runsHere bool
	// set after the first frame so only what arrives while you watch animates in
	drawn   bool
	actions []Action
	visible []int
	turnOf  []int
	// a finished turn's work folds into its first work row
	folds    map[int]*foldInfo
	overlays *look.Overlays

	jump     kit.IconPlaque
	jumpShow anim.Tween
	thumb    anim.Tween

	plaques     map[string]*kit.Plaque
	icons       map[string]*iconControl
	selectables map[string]*widget.Selectable
	links       map[string]*widget.Clickable
	markdown    map[string]*markdownState
	messages    map[string]*messageTarget
	diffs       map[string]*diffview.View
	work        map[string]*workVisual
	permissions map[string]*permissionVisual
	rises       map[string]*look.Rise
	turns       map[int]*turnVisual
	codes       map[string]*codeVisual
	inlines     map[string]*inlineState
	tables      map[string]*tableState
	menu        kit.Menu

	// inline paragraphs set this frame and the one before, and the frame's number
	shown, wasShown []*inlineState
	frame           int
	marked          marked
	// the tag presses in the transcript are watched under, and the frames left in which a press that just ended still counts
	presses  struct{}
	settling int
}

type iconControl struct {
	action look.IconAction
	tip    look.Tip
}

func (view *View) ensureState() *viewState {
	if view.state == nil {
		view.state = &viewState{}
	}
	view.state.ensure()
	return view.state
}

func (state *viewState) ensure() {
	if state.ready {
		return
	}
	state.ready, state.follow = true, true
	state.plaques = make(map[string]*kit.Plaque)
	state.icons = make(map[string]*iconControl)
	state.selectables = make(map[string]*widget.Selectable)
	state.links = make(map[string]*widget.Clickable)
	state.markdown = make(map[string]*markdownState)
	state.messages = make(map[string]*messageTarget)
	state.diffs = make(map[string]*diffview.View)
	state.work = make(map[string]*workVisual)
	state.permissions = make(map[string]*permissionVisual)
	state.rises = make(map[string]*look.Rise)
	state.turns = make(map[int]*turnVisual)
	state.codes = make(map[string]*codeVisual)
	state.inlines = make(map[string]*inlineState)
	state.tables = make(map[string]*tableState)
}

func (state *viewState) takeActions() []Action {
	actions := state.actions
	state.actions = nil
	return actions
}

func (state *viewState) act(action Action) { state.actions = append(state.actions, action) }

func (state *viewState) layout(
	gtx layout.Context,
	th *look.Theme,
	info protocol.SessionInfo,
	model *modelState,
) layout.Dimensions {
	if model == nil {
		model = &modelState{}
	}
	state.releaseMarked(gtx)
	state.watchPresses(gtx)
	state.buildVisible(model)
	state.updateTurnHover(gtx)
	busy := model.activeTurn != nil
	state.busy, state.runsHere = busy, info.Kind == protocol.KindAsk
	class := look.ClassFor(float32(gtx.Constraints.Max.X) / max(gtx.Metric.PxPerDp, .001))
	lastTurn := state.lastTurn(model)

	state.scroller.SetFollow(state.follow)
	dimensions := state.scroller.Layout(gtx, len(state.visible)+1, func(gtx layout.Context, position int) layout.Dimensions {
		gtx.Constraints.Min.Y = 0
		if position == len(state.visible) {
			return state.layoutTail(gtx, th, model, class, busy)
		}
		index := state.visible[position]
		row := &model.rows[index]
		block := blockContext{info: info, model: model, row: row, index: index, turn: state.turnOf[index], last: state.turnOf[index] == lastTurn}
		return layout.Inset{Top: state.gapBefore(model, position)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return look.ReadingColumn(gtx, class, func(gtx layout.Context) layout.Dimensions {
				return state.trackTurn(gtx, block.turn, func(gtx layout.Context) layout.Dimensions {
					return state.layoutRow(gtx, th, block)
				})
			})
		})
	})
	if state.scroller.UserScrolled() {
		state.follow = state.scroller.AtEnd()
		state.scroller.SetFollow(state.follow)
	}
	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect(image.Rectangle{Max: dimensions.Size}).Push(gtx.Ops)
	event.Op(gtx.Ops, &state.presses)
	area.Pop()
	pass.Pop()
	state.dropUnseenSets()
	state.layoutThumb(gtx, th, dimensions.Size)
	state.layoutJump(gtx, th, dimensions.Size)
	state.drawn = true
	return dimensions
}

type blockContext struct {
	info  protocol.SessionInfo
	model *modelState
	row   *transcriptRow
	index int
	turn  int
	last  bool
}

func (block blockContext) key() string { return strconv.Itoa(block.index) }

// a terminal owned by a tool call is drawn inside that call
func (state *viewState) buildVisible(model *modelState) {
	state.visible = state.visible[:0]
	state.turnOf = state.turnOf[:0]
	state.folds = state.foldTurns(model)
	turn := 0
	folded := false
	for index := range model.rows {
		row := &model.rows[index]
		if row.kind == rowUser {
			turn++
		}
		state.turnOf = append(state.turnOf, turn)
		if row.kind == rowTerminal && model.terminalOwners[row.key] != "" {
			continue
		}
		if fold := state.folds[index]; fold != nil {
			folded = !state.workVisual(foldKey(index)).open
		} else if row.kind == rowUser || row.kind == rowTurnEnd {
			folded = false
		} else if folded && isWorkRow(model, row) {
			continue
		}
		state.visible = append(state.visible, index)
	}
}

func (state *viewState) lastTurn(model *modelState) int {
	if len(state.turnOf) == 0 {
		return 0
	}
	return state.turnOf[len(model.rows)-1]
}

func (state *viewState) gapBefore(model *modelState, position int) unit.Dp {
	if position == 0 {
		return 21
	}
	current := model.rows[state.visible[position]].kind
	previous := model.rows[state.visible[position-1]].kind
	isWork := func(kind rowKind) bool { return kind == rowTool || kind == rowThought || kind == rowTerminal }
	switch {
	case current == rowUser:
		return 34
	case previous == rowUser:
		return 21
	case isWork(current) && isWork(previous):
		return 2
	case current == rowTurnEnd:
		return 8
	default:
		return 13
	}
}

func (state *viewState) layoutRow(gtx layout.Context, th *look.Theme, block blockContext) layout.Dimensions {
	row, key := block.row, block.key()
	switch row.kind {
	case rowUser:
		if row.user.Goal {
			return state.layoutGoalTurn(gtx, th, row.user.Text)
		}
		return state.layoutMessage(gtx, th, "user:"+row.key, row.seq, row.user.Text+row.user.Run, block.info.Kind == protocol.KindAgent,
			func(gtx layout.Context) layout.Dimensions { return state.layoutQuestion(gtx, th, row.user, row.key) })
	case rowText:
		return state.layoutMessage(gtx, th, "text:"+row.key, row.seq, row.text, block.info.Kind == protocol.KindAgent,
			func(gtx layout.Context) layout.Dimensions {
				return state.layoutMarkdown(gtx, th, "text:"+row.key, row.text, offered(row.turn))
			})
	case rowThought:
		// only live while it's the newest row
		live := block.model.activeTurn != nil && block.index == len(block.model.rows)-1
		if fold := state.folds[block.index]; fold != nil {
			return state.layoutFold(gtx, th, block.index, fold, func(gtx layout.Context) layout.Dimensions {
				return state.layoutThought(gtx, th, "thought:"+row.key, row.text, live)
			})
		}
		return state.layoutThought(gtx, th, "thought:"+row.key, row.text, live)
	case rowTool:
		if fold := state.folds[block.index]; fold != nil {
			return state.layoutFold(gtx, th, block.index, fold, func(gtx layout.Context) layout.Dimensions {
				return state.layoutTools(gtx, th, block.model, row.tools, "tool:"+row.key)
			})
		}
		return state.layoutTools(gtx, th, block.model, row.tools, "tool:"+row.key)
	case rowPermission:
		return state.layoutPermission(gtx, th, row.permission)
	case rowPlan:
		return state.layoutPlan(gtx, th, row.plan)
	case rowTerminal:
		if fold := state.folds[block.index]; fold != nil {
			return state.layoutFold(gtx, th, block.index, fold, func(gtx layout.Context) layout.Dimensions {
				return state.layoutTerminal(gtx, th, row.terminal, "terminal:"+row.key)
			})
		}
		return state.layoutTerminal(gtx, th, row.terminal, "terminal:"+row.key)
	case rowAnswer:
		return state.layoutAnswer(gtx, th, row.answer, "answer:"+key, row.turn != nil && row.turn.offeredInText)
	case rowError:
		return state.layoutError(gtx, th, block, "error:"+key)
	case rowTurnEnd:
		return state.layoutTurnFoot(gtx, th, block)
	case rowGoal:
		return state.layoutGoalEvent(gtx, th, row.goal, "goal:"+row.key)
	default:
		return layout.Dimensions{}
	}
}

func (state *viewState) layoutTail(gtx layout.Context, th *look.Theme, model *modelState, class look.Class, busy bool) layout.Dimensions {
	if !busy {
		return layout.Spacer{Height: th.Sp.S5}.Layout(gtx)
	}
	shown, label := tailMark(model)
	return layout.Inset{Top: th.Sp.S3, Bottom: th.Sp.S5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return look.ReadingColumn(gtx, class, func(gtx layout.Context) layout.Dimensions {
			// same height with or without the mark so the column doesn't jump as steps start and end
			gtx.Constraints.Min.Y = gtx.Sp(th.T.Label.Line)
			if !shown {
				return layout.Dimensions{Size: image.Pt(0, gtx.Constraints.Min.Y)}
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return kit.Working{Active: true, Size: 16}.Layout(gtx, th.Theme)
				}),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(kit.Text(th.Theme, th.T.Label, th.R.Fg2, label)),
			)
		})
	})
}

// a running step or an arriving thought already moves, arriving words need the mark but no word, anything else is thinking between steps
func tailMark(model *modelState) (shown bool, label string) {
	if len(model.rows) == 0 {
		return true, i18n.T("Thinking")
	}
	// what a running command prints belongs to its call, so the row before the output is the one that counts
	at := len(model.rows) - 1
	for at > 0 && model.rows[at].kind == rowTerminal && model.terminalOwners[model.rows[at].key] != "" {
		at--
	}
	switch last := &model.rows[at]; last.kind {
	case rowThought:
		return false, ""
	case rowText:
		return true, ""
	case rowTool:
		for _, tool := range last.tools {
			if tool != nil && toolRunning(tool) {
				return false, ""
			}
		}
	}
	return true, i18n.T("Thinking")
}

func (state *viewState) layoutJump(gtx layout.Context, th *look.Theme, size image.Point) {
	state.jump.Kind, state.jump.Icon, state.jump.Small = kit.PlaqueQuiet, look.Icon.ArrowDown, true
	state.jump.Description = i18n.T("Go to the end")
	if state.jump.Clicked(gtx) {
		state.follow = true
		state.scroller.JumpToEnd()
	}
	shown := look.Transition(gtx, &state.jumpShow, !state.follow, th.Motion.HoverIn, th.Motion.HoverOut)
	if shown <= 0 {
		return
	}
	plaque := gtx.Dp(32)
	bounds := image.Rect((size.X-plaque)/2, size.Y-plaque-gtx.Dp(8), (size.X+plaque)/2, size.Y-gtx.Dp(8))
	defer paint.PushOpacity(gtx.Ops, shown).Pop()
	look.Place(gtx, bounds, func(gtx layout.Context) layout.Dimensions {
		if state.follow {
			gtx = gtx.Disabled()
		}
		// floats over the transcript so it gets its own plane
		kit.FillCut(gtx.Ops, image.Rectangle{Max: gtx.Constraints.Max}, gtx.Dp(th.Cut.Small), th.R.Stone4)
		return state.jump.Layout(gtx, th.Theme)
	})
}

func (state *viewState) layoutThumb(gtx layout.Context, th *look.Theme, size image.Point) {
	alpha := look.Transition(gtx, &state.thumb, state.scroller.Active(gtx), th.Motion.HoverIn, th.Motion.HoverOut)
	if alpha <= 0 {
		return
	}
	look.DrawThumb(gtx, th, size, state.scroller.Position, len(state.visible)+1, alpha)
}

func (state *viewState) plaque(key string) *kit.Plaque {
	plaque := state.plaques[key]
	if plaque == nil {
		plaque = new(kit.Plaque)
		state.plaques[key] = plaque
	}
	return plaque
}

func (state *viewState) icon(gtx layout.Context, th *look.Theme, key string, icon *widget.Icon, description string) (layout.Dimensions, bool) {
	control := state.icons[key]
	if control == nil {
		control = new(iconControl)
		state.icons[key] = control
	}
	control.action.Icon, control.action.Description = icon, description
	clicked := control.action.Clicked(gtx)
	dimensions := control.tip.Layout(gtx, th, state.overlays, description, func(gtx layout.Context) layout.Dimensions {
		return control.action.Layout(gtx, th, th.R.Fg3)
	})
	return dimensions, clicked
}

func (state *viewState) rise(key string) *look.Rise {
	rise := state.rises[key]
	if rise == nil {
		rise = new(look.Rise)
		state.rises[key] = rise
		// first seen after the view was drawn means it just arrived
		if state.drawn {
			rise.Arrive()
		}
	}
	return rise
}

func (state *viewState) selectable(
	gtx layout.Context,
	th *look.Theme,
	key string,
	style look.TextStyle,
	ink color.NRGBA,
	value string,
) layout.Dimensions {
	selectable := state.selectables[key]
	if selectable == nil {
		selectable = new(widget.Selectable)
		selectable.LineHeight, selectable.LineHeightScale = style.Line, 1
		state.selectables[key] = selectable
	}
	selectable.SetText(value)
	changed := selectable.Update(gtx)
	marking := selectable.SelectionLen() != 0
	state.hold(gtx, key, selectable, changed, marking, selectable.Focused(), selectable.SelectedText, selectable.ClearSelection)
	// a press that marked nothing has no use for the keyboard it took
	if state.settling > 0 && !marking && selectable.Focused() {
		state.settling = 0
		state.act(Action{Kind: ActRelease})
	}
	selection := colorMaterial(gtx.Ops, palette.WithAlpha(th.R.Sel, .6))
	dimensions := selectable.Layout(gtx, th.Shaper, style.Face, style.Size, colorMaterial(gtx.Ops, ink), selection)
	// selectable text does not say what it reads, a label does
	area := clip.Rect(image.Rectangle{Max: dimensions.Size}).Push(gtx.Ops)
	semantic.LabelOp(value).Add(gtx.Ops)
	area.Pop()
	return dimensions
}

func colorMaterial(ops *op.Ops, ink color.NRGBA) op.CallOp {
	record := op.Record(ops)
	paint.ColorOp{Color: ink}.Add(ops)
	return record.Stop()
}

func slab(gtx layout.Context, th *look.Theme, plane color.NRGBA, inset unit.Dp, body layout.Widget) layout.Dimensions {
	record := op.Record(gtx.Ops)
	dimensions := layout.UniformInset(inset).Layout(gtx, body)
	call := record.Stop()
	kit.FillCut(gtx.Ops, image.Rectangle{Max: dimensions.Size}, gtx.Dp(th.Cut.Mid), plane)
	call.Add(gtx.Ops)
	return dimensions
}

func firstLines(value string, count int) (string, bool) {
	lines := strings.Split(strings.TrimRight(value, "\n"), "\n")
	if len(lines) <= count {
		return strings.Join(lines, "\n"), false
	}
	return strings.Join(lines[:count], "\n"), true
}

func lastLines(value string, count int) (string, bool) {
	lines := strings.Split(strings.TrimRight(value, "\n"), "\n")
	if len(lines) <= count {
		return strings.Join(lines, "\n"), false
	}
	return strings.Join(lines[len(lines)-count:], "\n"), true
}

func firstLine(value string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(value), "\n")
	return line
}
