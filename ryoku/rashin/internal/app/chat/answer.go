package chat

import (
	"image"
	"strconv"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

func (state *viewState) layoutPlan(gtx layout.Context, th *look.Theme, entries []protocol.PlanEntry) layout.Dimensions {
	return slab(gtx, th, th.R.Stone1, 16, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		children := []layout.FlexChild{layout.Rigid(kit.Text(th.Theme, th.T.Title, th.R.Fg, i18n.T("Plan")))}
		line := gtx.Sp(th.T.Body.Line)
		for _, entry := range entries {
			tone, ink := palette.Idle, th.R.Fg
			switch entry.Status {
			case "in_progress":
				tone = palette.Becoming
			case "completed":
				tone, ink = palette.Alive, th.R.Fg3
			}
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					look.MiddleLeft(gtx, 0, line, func(gtx layout.Context) layout.Dimensions {
						return kit.Flake(gtx, th.Theme, tone, 1, 9)
					})
					indent := gtx.Dp(21)
					offset := op.Offset(image.Pt(indent, 0)).Push(gtx.Ops)
					body := gtx
					body.Constraints.Max.X = max(0, gtx.Constraints.Max.X-indent)
					body.Constraints.Min.X = 0
					dimensions := look.Paragraph(th, th.T.Body, ink, entry.Content)(body)
					offset.Pop()
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, max(line, dimensions.Size.Y))}
				}),
			)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (state *viewState) layoutAnswer(gtx layout.Context, th *look.Theme, answer *protocol.AnswerData, key string) layout.Dimensions {
	if answer == nil {
		return layout.Dimensions{}
	}
	children := make([]layout.FlexChild, 0, 6)
	add := func(widget layout.Widget) {
		if len(children) > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout))
		}
		children = append(children, layout.Rigid(widget))
	}
	if len(answer.Actions) > 0 {
		add(func(gtx layout.Context) layout.Dimensions {
			return state.layoutAnswerActions(gtx, th, answer.Actions, key)
		})
	}
	if answer.Plan != nil && len(answer.Plan.Commands) > 0 {
		add(func(gtx layout.Context) layout.Dimensions {
			return state.layoutCommands(gtx, th, answer.Plan, key+":commands")
		})
	}
	if answer.NeedsAgent {
		add(func(gtx layout.Context) layout.Dimensions {
			return state.layoutAgentOffer(gtx, th, answer.Reason, key+":agent")
		})
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (state *viewState) layoutAnswerActions(gtx layout.Context, th *look.Theme, actions []protocol.Action, key string) layout.Dimensions {
	return look.Flow{Gap: 8, LineGap: 8}.Layout(gtx, len(actions), func(gtx layout.Context, index int) layout.Dimensions {
		action := actions[index]
		plaque := state.plaque(key + ":" + strconv.Itoa(index))
		plaque.Kind, plaque.Small, plaque.Label = kit.PlaqueQuiet, true, action.Label
		switch action.Kind {
		case protocol.ActURL:
			plaque.Icon = look.Icon.OpenInNew
		case protocol.ActFile:
			plaque.Icon = look.Icon.File
		case protocol.ActDir:
			plaque.Icon = look.Icon.Folder
		case protocol.ActCmd:
			plaque.Icon = look.Icon.Terminal
		default:
			plaque.Icon = look.Icon.Copy
		}
		if plaque.Clicked(gtx) {
			state.act(answerAction(action))
		}
		return plaque.Layout(gtx, th.Theme)
	})
}

func answerAction(action protocol.Action) Action {
	result := Action{Kind: ActCopy, Value: action.Value}
	switch action.Kind {
	case protocol.ActURL:
		result.Kind = ActOpenURL
	case protocol.ActFile, protocol.ActDir:
		result.Kind = ActOpenPath
	case protocol.ActCmd:
		result.Kind = ActRunCommand
	}
	return result
}

// each command stands on its own slab with how far it reaches, so nothing runs without that being seen
func (state *viewState) layoutCommands(gtx layout.Context, th *look.Theme, plan *protocol.CommandPlan, key string) layout.Dimensions {
	return slab(gtx, th, th.R.Stone1, 16, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		children := []layout.FlexChild{layout.Rigid(kit.Text(th.Theme, th.T.Title, th.R.Fg, i18n.T("Commands you can run")))}
		if plan.Summary != "" {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
				layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg2, plan.Summary)),
			)
		}
		for index, command := range plan.Commands {
			commandKey := key + ":" + strconv.Itoa(index)
			gap := th.Sp.S2
			if index == 0 {
				gap = th.Sp.S3
			}
			children = append(children,
				layout.Rigid(layout.Spacer{Height: gap}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return state.layoutCommand(gtx, th, command, commandKey) }),
			)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (state *viewState) layoutCommand(gtx layout.Context, th *look.Theme, command protocol.PlannedCommand, key string) layout.Dimensions {
	run := state.plaque(key + ":run")
	run.Kind, run.Small, run.Label = kit.PlaqueLine, true, i18n.T("Run in terminal")
	if run.Clicked(gtx) {
		state.act(Action{Kind: ActRunCommand, Value: command.Command})
	}
	record := op.Record(gtx.Ops)
	dimensions := layout.Inset{Top: 13, Bottom: 8, Left: 13, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return state.selectable(gtx, th, key, th.T.Mono, th.R.Fg, command.Command)
			}),
		}
		if command.Why != "" {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
				layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg2, command.Why)),
			)
		}
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, tierClaim(th, command.Tier)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						dimensions, clicked := state.icon(gtx, th, key+":copy", look.Icon.Copy, i18n.T("Copy"))
						if clicked {
							state.act(Action{Kind: ActCopy, Value: command.Command})
						}
						return dimensions
					}),
					layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return run.Layout(gtx, th.Theme) }),
				)
			}),
		)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
	call := record.Stop()
	kit.FillCut(gtx.Ops, image.Rectangle{Max: dimensions.Size}, gtx.Dp(th.Cut.Small), th.R.Stone2)
	call.Add(gtx.Ops)
	return dimensions
}

func tierClaim(th *look.Theme, tier string) layout.Widget {
	tone, word := palette.Idle, i18n.T("Reads")
	switch tier {
	case protocol.TierWrite:
		tone, word = palette.Attention, i18n.T("Changes files")
	case protocol.TierSystem:
		tone, word = palette.Attention, i18n.T("Needs root")
	case protocol.TierDanger:
		tone, word = palette.Failed, i18n.T("Destructive")
	}
	return kit.Claim(th.Theme, tone, word)
}

func (state *viewState) layoutAgentOffer(gtx layout.Context, th *look.Theme, reason, key string) layout.Dimensions {
	plaque := state.plaque(key)
	plaque.Kind, plaque.Small = kit.PlaqueLine, true
	plaque.Icon, plaque.Label = look.Icon.Agents, i18n.T("Move to Agent")
	if plaque.Clicked(gtx) {
		state.act(Action{Kind: ActHandoff})
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return plaque.Layout(gtx, th.Theme) }),
		layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
		layout.Flexed(1, kit.Text(th.Theme, th.T.Caption, th.R.Fg2, reason)),
	)
}

func (state *viewState) layoutError(gtx layout.Context, th *look.Theme, block blockContext, key string) layout.Dimensions {
	failure, turn := block.row.err, block.row.turn
	retry := state.plaque(key + ":retry")
	retry.Kind, retry.Small, retry.Label = kit.PlaqueLine, true, i18n.T("Try again")
	if retry.Clicked(gtx) {
		number := 0
		if turn != nil {
			number = turn.number
		}
		state.act(Action{Kind: ActRetry, Turn: number, Value: questionText(block.model, block.index)})
	}
	gtx.Constraints.Min = image.Point{}
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(560))
	record := op.Record(gtx.Ops)
	dimensions := layout.UniformInset(16).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(kit.Claim(th.Theme, palette.Failed, i18n.T("Failed"))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return state.selectable(gtx, th, key, th.T.Body, th.R.Fg, failure.Message)
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return retry.Layout(gtx, th.Theme) }),
		)
	})
	call := record.Stop()
	kit.FillCut(gtx.Ops, image.Rectangle{Max: dimensions.Size}, gtx.Dp(th.Cut.Mid), th.R.Field(palette.Failed))
	call.Add(gtx.Ops)
	return dimensions
}
