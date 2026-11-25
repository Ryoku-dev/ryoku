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
	answers AnswerOptions
	compare *compareVisual
	rule    ruleVisual
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
	if state.feature("compare") {
		state.syncComparison(block)
	}
	visual := state.turnVisual(block.turn)
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return state.layoutTurnFootBase(gtx, th, block) }),
	}
	if state.feature("compare") && visual.compare != nil && visual.compare.source == block.info.ID {
		if visual.compare.picking {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: th.Sp.S2, Bottom: th.Sp.S2, Left: th.Sp.S3, Right: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return state.layoutComparePicker(gtx, th, block, visual.compare, "compare:"+block.info.ID+":"+strconv.Itoa(block.turn))
				})
			}))
		}
		if visual.compare.err != "" {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: th.Sp.S2}.Layout(gtx, kit.Text(th.Theme, th.T.Detail, th.R.Fg2, visual.compare.err))
			}))
		}
	}
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return state.layoutRule(gtx, th, block) }))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (state *viewState) layoutTurnFootBase(gtx layout.Context, th *look.Theme, block blockContext) layout.Dimensions {
	turn := block.row.turn
	visual := state.turnVisual(block.turn)
	show := look.Transition(gtx, &visual.show, visual.hovered || block.last || visual.rule.open, th.Motion.HoverIn, th.Motion.HoverOut)
	if show <= 0 {
		return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, gtx.Dp(th.Sp.S6))}
	}
	defer paint.PushOpacity(gtx.Ops, show).Pop()
	gtx.Constraints.Min.Y = gtx.Dp(th.Sp.S6)
	key := "turn:" + strconv.Itoa(block.index)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return state.layoutTurnActions(gtx, th, block, key)
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutTurnUsage(gtx, th, turn)
		}),
	)
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
	children := make([]layout.FlexChild, 0, len(actions)+5)
	for index, item := range actions {
		if index > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dimensions, clicked := state.icon(gtx, th, key+":"+strconv.Itoa(index), item.icon, item.description)
			if clicked {
				state.act(item.action)
			}
			return dimensions
		}))
	}
	addPlaque := func(label, id string, action func()) {
		children = append(children,
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return state.answerPlaque(gtx, th, key+":"+id, label, action)
			}),
		)
	}
	if block.info.Kind == protocol.KindAsk && state.feature("compare") {
		addPlaque(i18n.T("Compare"), "compare", func() { state.openCompare(block) })
	}
	if block.info.Kind == protocol.KindAsk && state.feature("share") {
		addPlaque(i18n.T("Share as a picture"), "share", func() { state.shareAnswer(gtx, th, block) })
	}
	if state.feature(protocol.FeatureRules) && state.call != nil && strings.TrimSpace(answerText(block.model, block.index)) != "" {
		addPlaque(i18n.T("That was wrong, make a rule"), "rule", func() {
			visual := &state.turnVisual(block.turn).rule
			visual.open = !visual.open
			if visual.open {
				visual.complaint.Focus()
			}
		})
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

// the turn's cost and the model that answered, without repeating the provider
func layoutTurnUsage(gtx layout.Context, th *look.Theme, turn *turnState) layout.Dimensions {
	children := make([]layout.FlexChild, 0, 3)
	if words := turnUsage(turn); words != "" {
		children = append(children, layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, words)))
	}
	if turn.answer != nil && turn.answer.Via != "" {
		if len(children) > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout))
		}
		model := turn.answer.Via
		if _, name, qualified := strings.Cut(model, "/"); qualified && name != "" {
			model = name
		}
		children = append(children, layout.Rigid(kit.RawText(th.Theme, th.T.MonoSm, th.R.Fg3, model)))
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
