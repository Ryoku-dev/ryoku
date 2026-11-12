package chat

import (
	"image"
	"strconv"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type turnVisual struct {
	hovered bool
	show    anim.Tween
}

func (state *viewState) turnVisual(turn int) *turnVisual {
	visual := state.turns[turn]
	if visual == nil {
		visual = new(turnVisual)
		state.turns[turn] = visual
	}
	return visual
}

// every row of a turn shares one tag so the pointer stays inside the turn from row to row
func (state *viewState) trackTurn(gtx layout.Context, turn int, widget layout.Widget) layout.Dimensions {
	dimensions := widget(gtx)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect(image.Rectangle{Max: dimensions.Size}).Push(gtx.Ops)
	event.Op(gtx.Ops, state.turnVisual(turn))
	area.Pop()
	pass.Pop()
	return dimensions
}

func (state *viewState) updateTurnHover(gtx layout.Context) {
	for _, visual := range state.turns {
		for {
			raw, ok := gtx.Event(pointer.Filter{Target: visual, Kinds: pointer.Enter | pointer.Leave | pointer.Cancel})
			if !ok {
				break
			}
			if pointed, ok := raw.(pointer.Event); ok {
				visual.hovered = pointed.Kind == pointer.Enter
			}
		}
	}
}

// the last turn always shows it, earlier turns only under the pointer
func (state *viewState) layoutTurnFoot(gtx layout.Context, th *look.Theme, block blockContext) layout.Dimensions {
	turn := block.row.turn
	if turn == nil || turn.end == nil {
		return layout.Dimensions{}
	}
	visual := state.turnVisual(block.turn)
	show := look.Transition(gtx, &visual.show, visual.hovered || block.last, th.Motion.HoverIn, th.Motion.HoverOut)
	height := gtx.Dp(28)
	size := image.Pt(gtx.Constraints.Max.X, height)
	if show <= 0 {
		return layout.Dimensions{Size: size}
	}
	defer paint.PushOpacity(gtx.Ops, show).Pop()
	gtx.Constraints = layout.Exact(size)
	key := "turn:" + strconv.Itoa(block.index)
	layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// inset so the glyph, not its box, stands on the column's edge
			return layout.Inset{Left: -5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return state.layoutTurnActions(gtx, th, block, key)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layoutTurnUsage(gtx, th, turn)
			})
		}),
	)
	return layout.Dimensions{Size: size}
}

type turnAction struct {
	icon        *widget.Icon
	description string
	action      Action
}

func (state *viewState) layoutTurnActions(gtx layout.Context, th *look.Theme, block blockContext, key string) layout.Dimensions {
	number := block.row.turn.number
	actions := []turnAction{{look.Icon.Copy, i18n.T("Copy answer"), Action{Kind: ActCopy, Value: answerText(block.model, block.index)}}}
	switch {
	case block.info.Kind != protocol.KindAgent:
		actions = append(actions, turnAction{look.Icon.Retry, i18n.T("Ask again"), retryAction(block.model, block.index, number)})
	case block.info.Checkpoints:
		actions = append(actions,
			turnAction{look.Icon.Diff, i18n.T("See what this turn changed"), Action{Kind: ActTurnChanges, Turn: number}},
			turnAction{look.Icon.Undo, i18n.T("Revert files to before this turn"), Action{Kind: ActRevertTurn, Turn: number}},
		)
	}
	if block.info.Kind == protocol.KindAgent {
		actions = append(actions, turnAction{look.Icon.Fork, i18n.T("Fork from here"), Action{Kind: ActFork, Seq: block.row.seq}})
	}
	children := make([]layout.FlexChild, 0, len(actions))
	for index, item := range actions {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dimensions, clicked := state.icon(gtx, th, key+":"+strconv.Itoa(index), item.icon, item.description)
			if clicked {
				state.act(item.action)
			}
			return dimensions
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

func answerText(model *modelState, index int) string {
	var parts []string
	for at := index - 1; at >= 0; at-- {
		row := model.rows[at]
		if row.kind == rowUser || row.kind == rowTurnEnd {
			break
		}
		if row.kind == rowText && strings.TrimSpace(row.text) != "" {
			parts = append([]string{row.text}, parts...)
		}
	}
	return strings.Join(parts, "\n\n")
}

// asking again without what was attached would ask about nothing
func retryAction(model *modelState, index, turn int) Action {
	action := Action{Kind: ActRetry, Turn: turn}
	for at := min(index, len(model.rows)-1); at >= 0; at-- {
		if row := model.rows[at]; row.kind == rowUser {
			// a turn that was a command is asked again by running it again
			if row.user.Run != "" {
				return Action{Kind: ActRunHere, Value: row.user.Run}
			}
			action.Value = row.user.Text
			for _, attached := range row.user.Images {
				action.Attached = append(action.Attached, attached.Path)
			}
			break
		}
	}
	return action
}

// how the turn ended and what it spent in words, then the model that answered,
// a literal id in mono set apart by space
func layoutTurnUsage(gtx layout.Context, th *look.Theme, turn *turnState) layout.Dimensions {
	children := make([]layout.FlexChild, 0, 3)
	if words := turnUsage(turn); words != "" {
		children = append(children, layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, words)))
	}
	if turn.answer != nil && turn.answer.Via != "" {
		if len(children) > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout))
		}
		children = append(children, layout.Rigid(kit.RawText(th.Theme, th.T.MonoSm, th.R.Fg3, turn.answer.Via)))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx, children...)
}

// "Stopped, 18,200 in, 1,400 out"
func turnUsage(turn *turnState) string {
	var stop, spent string
	// half an answer reads like a whole one unless the line under it says it is not
	if turn.end != nil {
		switch turn.end.Stop {
		case protocol.StopCancelled:
			stop = i18n.T("Stopped")
		case protocol.StopMaxTokens:
			stop = i18n.T("Cut off at the length limit")
		}
	}
	if turn.end != nil && turn.end.Usage != nil {
		spent = i18n.Tf("%s in, %s out", look.Grouped(turn.end.Usage.Input), look.Grouped(turn.end.Usage.Output))
	}
	return look.Clauses(stop, spent)
}
