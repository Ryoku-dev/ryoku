package chat

import (
	"fmt"
	"image"
	"image/color"
	"path/filepath"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/diffview"
	"ryoku-rashin/internal/protocol"
)

type markdownState struct {
	source string
	blocks []markdownBlock
}

type messageTarget struct {
	pending   bool
	pointerID pointer.ID
}

type viewState struct {
	scroller transcriptScroller
	follow   bool
	ready    bool
	actions  []Action
	visible  []int

	jump        kit.Plaque
	buttons     map[string]*kit.Plaque
	disclosures map[string]*kit.Disclosure
	selectables map[string]*widget.Selectable
	links       map[string]*widget.Clickable
	markdown    map[string]*markdownState
	messages    map[string]*messageTarget
	diffs       map[string]*diffview.View
	menuKey     string
	menuSeq     int64
	menuText    string
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
	state.ready = true
	state.follow = true
	state.scroller.Gap = 0
	state.buttons = make(map[string]*kit.Plaque)
	state.disclosures = make(map[string]*kit.Disclosure)
	state.selectables = make(map[string]*widget.Selectable)
	state.links = make(map[string]*widget.Clickable)
	state.markdown = make(map[string]*markdownState)
	state.messages = make(map[string]*messageTarget)
	state.diffs = make(map[string]*diffview.View)
	state.jump.Kind = kit.PlaqueLine
	state.jump.Label = i18n.T("Jump to latest")
	state.jump.Small = true
}

func (state *viewState) takeActions() []Action {
	actions := state.actions
	state.actions = nil
	return actions
}

func (state *viewState) layout(gtx layout.Context, th *theme.Theme, info protocol.SessionInfo, model *modelState) layout.Dimensions {
	if model == nil {
		model = &modelState{}
	}
	state.visible = state.visible[:0]
	for index := range model.rows {
		row := &model.rows[index]
		if row.kind == rowPermission && row.permission != nil && row.permission.resolved != nil {
			continue
		}
		if row.kind == rowTerminal && model.terminalOwners[row.key] != "" {
			continue
		}
		state.visible = append(state.visible, index)
	}
	busy := model.activeTurn != nil
	count := len(state.visible)
	if busy {
		count++
	}
	if state.follow {
		state.scroller.JumpToEnd()
	}

	return layout.Stack{Alignment: layout.SE}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			dimensions := state.scroller.Layout(gtx, count, func(gtx layout.Context, index int) layout.Dimensions {
				gtx.Constraints.Min.Y = 0
				if index == len(state.visible) {
					return state.layoutBusy(gtx, th)
				}
				return layout.Inset{Top: th.Sp.S2, Bottom: th.Sp.S2, Left: th.Sp.S4, Right: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return state.layoutRow(gtx, th, model, info.Checkpoints, &model.rows[state.visible[index]], state.visible[index])
				})
			})
			if state.scroller.UserScrolled() {
				state.follow = state.scroller.AtEnd()
			}
			return dimensions
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			if state.follow {
				return layout.Dimensions{}
			}
			if state.jump.Clicked(gtx) {
				state.follow = true
				state.scroller.JumpToEnd()
			}
			return layout.Inset{Right: th.Sp.S4, Bottom: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = image.Point{}
				return state.jump.Layout(gtx, th)
			})
		}),
	)
}

func (state *viewState) layoutRow(gtx layout.Context, th *theme.Theme, modelState *modelState, checkpoints bool, row *transcriptRow, index int) layout.Dimensions {
	key := strconv.Itoa(index)
	switch row.kind {
	case rowUser:
		return state.layoutMessage(gtx, th, "user:"+row.key, row.seq, row.user.Text, func(gtx layout.Context) layout.Dimensions {
			return state.layoutUser(gtx, th, row.user, key)
		})
	case rowText:
		return state.layoutMessage(gtx, th, "text:"+row.key, row.seq, row.text, func(gtx layout.Context) layout.Dimensions {
			return state.layoutMarkdown(gtx, th, "text:"+row.key, row.text)
		})
	case rowThought:
		return state.layoutThought(gtx, th, "thought:"+row.key, row.text)
	case rowTool:
		return state.layoutToolGroup(gtx, th, modelState, row.tools, "tool:"+row.key)
	case rowPermission:
		return state.layoutPermission(gtx, th, row.permission)
	case rowPlan:
		return state.layoutPlan(gtx, th, row.plan, "plan:"+key)
	case rowTerminal:
		return state.layoutTerminal(gtx, th, row.terminal, "terminal:"+row.key, false)
	case rowAnswer:
		return state.layoutAnswer(gtx, th, row.answer, "answer:"+key)
	case rowError:
		return state.layoutError(gtx, th, row.err, "error:"+key)
	case rowTurnEnd:
		return state.layoutTurnFooter(gtx, th, row.turn, checkpoints, "turn:"+key)
	default:
		return layout.Dimensions{}
	}
}

func (state *viewState) layoutMessage(gtx layout.Context, th *theme.Theme, key string, seq int64, text string, content layout.Widget) layout.Dimensions {
	target := state.messages[key]
	if target == nil {
		target = &messageTarget{}
		state.messages[key] = target
	}
	for {
		raw, ok := gtx.Event(pointer.Filter{Target: target, Kinds: pointer.Press | pointer.Release | pointer.Cancel})
		if !ok {
			break
		}
		pointerEvent := raw.(pointer.Event)
		switch pointerEvent.Kind {
		case pointer.Press:
			if pointerEvent.Buttons&pointer.ButtonSecondary != 0 {
				target.pending = true
				target.pointerID = pointerEvent.PointerID
			}
		case pointer.Release:
			if target.pending && target.pointerID == pointerEvent.PointerID {
				target.pending = false
				state.menuKey, state.menuSeq, state.menuText = key, seq, text
			}
		case pointer.Cancel:
			target.pending = false
		}
	}

	dimensions := content(gtx)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect(image.Rectangle{Max: dimensions.Size}).Push(gtx.Ops)
	event.Op(gtx.Ops, target)
	area.Pop()
	pass.Pop()
	if state.menuKey != key {
		return dimensions
	}
	menuGTX := gtx
	menuGTX.Constraints.Min = image.Point{}
	menuOffset := op.Offset(image.Pt(0, dimensions.Size.Y+gtx.Dp(th.Sp.S2))).Push(gtx.Ops)
	menuDimensions := state.layoutMessageMenu(menuGTX, th, key)
	menuOffset.Pop()
	dimensions.Size.X = max(dimensions.Size.X, menuDimensions.Size.X)
	dimensions.Size.Y += gtx.Dp(th.Sp.S2) + menuDimensions.Size.Y
	return dimensions
}

func (state *viewState) layoutMessageMenu(gtx layout.Context, th *theme.Theme, key string) layout.Dimensions {
	copyButton := state.button(key + ":menu:copy")
	copyButton.Kind = kit.PlaqueQuiet
	copyButton.Label = i18n.T("Copy")
	copyButton.Small = true
	forkButton := state.button(key + ":menu:fork")
	forkButton.Kind = kit.PlaqueLine
	forkButton.Label = i18n.T("Fork from here")
	forkButton.Small = true
	if copyButton.Clicked(gtx) {
		state.actions = append(state.actions, Action{Kind: ActCopy, Value: state.menuText})
		state.menuKey = ""
	}
	if forkButton.Clicked(gtx) {
		state.actions = append(state.actions, Action{Kind: ActFork, Seq: state.menuSeq})
		state.menuKey = ""
	}
	return kit.Slab(gtx, th.R.Stone4, th.Cut.Menu, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(th.Sp.S2).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return copyButton.Layout(gtx, th)
				}),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return forkButton.Layout(gtx, th)
				}),
			)
		})
	})
}

func (state *viewState) layoutBusy(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	return layout.Inset{Top: th.Sp.S3, Bottom: th.Sp.S4, Left: th.Sp.S4, Right: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return (kit.Working{Active: true}).Layout(gtx, th)
			}),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Rigid(kit.Text(th, th.T.Label, th.R.Fg2, i18n.T("Working"))),
		)
	})
}

func (state *viewState) layoutUser(gtx layout.Context, th *theme.Theme, user protocol.UserData, key string) layout.Dimensions {
	return kit.Slab(gtx, th.R.Stone3, th.Cut.Mid, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(th.Sp.S4).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				layout.Rigid(kit.Text(th, th.T.Label, th.R.Fg2, i18n.T("You"))),
			}
			if user.Text != "" {
				children = append(children,
					layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return state.selectable(gtx, th, "user:"+key, th.T.Body, th.R.Fg, user.Text)
					}),
				)
			}
			if len(user.Images) > 0 {
				children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout))
				for imageIndex, attachment := range user.Images {
					name := filepath.Base(attachment.Path)
					if name == "." || name == "" {
						name = attachment.Path
					}
					attachmentKey := fmt.Sprintf("user:%s:image:%d", key, imageIndex)
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return state.selectable(gtx, th, attachmentKey, th.T.MonoSm, th.R.Fg2, name)
					}))
				}
			}
			if user.Terminal != nil {
				terminal := user.Terminal
				children = append(children,
					layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						parts := make([]string, 0, 5)
						if terminal.Cwd != "" {
							parts = append(parts, terminal.Cwd)
						}
						if terminal.Shell != "" {
							parts = append(parts, terminal.Shell)
						}
						if terminal.LastCommand != "" {
							parts = append(parts, "$ "+terminal.LastCommand)
						}
						if terminal.LastStatus != nil {
							parts = append(parts, i18n.Tf("Exit %d", *terminal.LastStatus))
						}
						if terminal.Buffer {
							parts = append(parts, i18n.T("Command plan requested"))
						}
						context := strings.Join(parts, "\n")
						return state.selectable(gtx, th, "user:"+key+":terminal", th.T.MonoSm, th.R.Fg2, context)
					}),
				)
			}
			if user.Handoff != nil {
				children = append(children,
					layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return state.layoutHandoffContext(gtx, th, user.Handoff, "user:"+key+":handoff")
					}),
				)
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

func (state *viewState) layoutHandoffContext(gtx layout.Context, th *theme.Theme, handoff *protocol.Handoff, key string) layout.Dimensions {
	return kit.Slab(gtx, th.R.Stone2, th.Cut.Small, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(th.Sp.S3).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			source := handoff.Title
			if source == "" {
				source = handoff.From
			}
			title := i18n.T("Handoff")
			if source != "" {
				title = i18n.Tf("From %s", source)
			}
			children := []layout.FlexChild{
				layout.Rigid(kit.Text(th, th.T.Label, th.R.Fg2, title)),
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return state.selectable(gtx, th, key+":question", th.T.Body, th.R.Fg, handoff.Question)
				}),
			}
			if handoff.Answer != "" {
				children = append(children,
					layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return state.selectable(gtx, th, key+":answer", th.T.Body, th.R.Fg2, handoff.Answer)
					}),
				)
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

func (state *viewState) layoutThought(gtx layout.Context, th *theme.Theme, key, value string) layout.Dimensions {
	disclosure := state.disclosure(key)
	toggle := state.button(key + ":toggle")
	toggle.Kind = kit.PlaqueQuiet
	toggle.Label = i18n.T("Thinking")
	toggle.Small = true
	if toggle.Clicked(gtx) {
		disclosure.Open = !disclosure.Open
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return toggle.Layout(gtx, th)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return disclosure.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return state.selectable(gtx, th, key+":text", th.T.Body, th.R.Fg2, value)
				})
			})
		}),
	)
}

func (state *viewState) layoutToolGroup(gtx layout.Context, th *theme.Theme, model *modelState, tools []*protocol.ToolData, key string) layout.Dimensions {
	if len(tools) == 0 {
		return layout.Dimensions{}
	}
	if len(tools) == 1 {
		return state.layoutTool(gtx, th, model, tools[0], key)
	}
	disclosure := state.disclosure(key + ":group")
	toggle := state.button(key + ":group:toggle")
	toggle.Kind = kit.PlaqueQuiet
	toggle.Small = true
	commands, reads, edits, failures := 0, 0, 0, 0
	runningTitle := ""
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		switch tool.Kind {
		case protocol.ToolExecute:
			commands++
		case protocol.ToolRead, protocol.ToolSearch, protocol.ToolFetch:
			reads++
		case protocol.ToolEdit, protocol.ToolDelete, protocol.ToolMove:
			edits++
		}
		if tool.Status == protocol.ToolFailed {
			failures++
		}
		if tool.Status == protocol.ToolRunning || tool.Status == protocol.ToolPending {
			runningTitle = tool.Title
			if runningTitle == "" {
				runningTitle = i18n.T("Tool call")
			}
		}
	}
	parts := make([]string, 0, 4)
	if commands > 0 {
		parts = append(parts, i18n.Tf("%d commands", commands))
	}
	if reads > 0 {
		parts = append(parts, i18n.Tf("%d reads", reads))
	}
	if edits > 0 {
		parts = append(parts, i18n.Tf("%d edits", edits))
	}
	if failures > 0 {
		parts = append(parts, i18n.Tf("%d failures", failures))
	}
	if len(parts) == 0 {
		parts = append(parts, i18n.Tf("%d tool calls", len(tools)))
	}
	toggle.Label = strings.Join(parts, " · ")
	if runningTitle != "" {
		disclosure.Open = true
		toggle.Label += " · " + runningTitle
	}
	if toggle.Clicked(gtx) {
		if runningTitle != "" {
			disclosure.Open = true
		} else {
			disclosure.Open = !disclosure.Open
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return kit.Slab(gtx, th.R.Stone2, th.Cut.Small, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(th.Sp.S2).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return toggle.Layout(gtx, th)
				})
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return disclosure.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, 0, len(tools)*2)
				for index, tool := range tools {
					if index > 0 {
						children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout))
					}
					toolKey := fmt.Sprintf("%s:%d", key, index)
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return state.layoutTool(gtx, th, model, tool, toolKey)
					}))
				}
				return layout.Inset{Top: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
				})
			})
		}),
	)
}

func (state *viewState) layoutTool(gtx layout.Context, th *theme.Theme, model *modelState, tool *protocol.ToolData, key string) layout.Dimensions {
	if tool == nil {
		return layout.Dimensions{}
	}
	tone, word := palette.Becoming, i18n.T("Running")
	for _, request := range model.permissionOrder {
		permission := model.permissions[request]
		if permission != nil && permission.resolved == nil && permission.data.Call == tool.Call {
			tone, word = palette.Attention, i18n.T("Needs you")
			break
		}
	}
	if word != i18n.T("Needs you") {
		switch tool.Status {
		case protocol.ToolDone:
			tone, word = palette.Alive, i18n.T("Done")
		case protocol.ToolFailed:
			tone, word = palette.Failed, i18n.T("Failed")
		}
	}
	return kit.Slab(gtx, th.R.Field(tone), th.Cut.Mid, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(th.Sp.S4).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			title := tool.Title
			if title == "" {
				title = i18n.T("Tool call")
			}
			children := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, kit.Text(th, th.T.Row, th.R.Fg, title)),
						layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
						layout.Rigid(kit.Claim(th, tone, word)),
					)
				}),
			}
			if tool.Kind != "" {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return state.selectable(gtx, th, key+":kind", th.T.MonoSm, th.R.Fg3, i18n.T(tool.Kind))
				}))
			}
			if tool.Input != "" {
				children = append(children,
					layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return state.selectable(gtx, th, key+":input", th.T.MonoSm, th.R.Fg2, tool.Input)
					}),
				)
			}
			if tool.Diff != nil {
				children = append(children,
					layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return state.layoutAgentDiff(gtx, th, tool.Diff, key+":diff")
					}),
				)
			}
			if tool.Output != "" {
				children = append(children,
					layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return state.layoutCollapsibleText(gtx, th, key+":output", i18n.T("Output"), tool.Output, 12)
					}),
				)
			}
			if tool.Terminal != "" {
				if terminal := model.terminals[tool.Terminal]; terminal != nil {
					children = append(children,
						layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return state.layoutTerminal(gtx, th, terminal, key+":terminal", true)
						}),
					)
				}
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

func (state *viewState) layoutPermission(gtx layout.Context, th *theme.Theme, permission *permissionState) layout.Dimensions {
	if permission == nil || permission.resolved != nil {
		return layout.Dimensions{}
	}
	data := permission.data
	return kit.Slab(gtx, th.R.Field(palette.Attention), th.Cut.Mid, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(th.Sp.S4).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				layout.Rigid(kit.Claim(th, palette.Attention, i18n.T("Needs you"))),
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(kit.Text(th, th.T.Row, th.R.Fg, data.Title)),
			}
			if data.Input != "" {
				children = append(children,
					layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return state.selectable(gtx, th, "permission:"+data.Request+":input", th.T.MonoSm, th.R.Fg2, data.Input)
					}),
				)
			}
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					controls := make([]layout.FlexChild, 0, len(data.Options)*2)
					for index, option := range data.Options {
						button := state.button("permission:" + data.Request + ":" + option.ID)
						button.Label = i18n.T(option.Label)
						button.Small = true
						switch option.Kind {
						case protocol.OptAllowOnce:
							button.Kind = kit.PlaqueGold
						case protocol.OptAllowAlways:
							button.Kind = kit.PlaqueLine
						default:
							button.Kind = kit.PlaqueDangerLine
						}
						request, optionID := data.Request, option.ID
						if button.Clicked(gtx) {
							state.actions = append(state.actions, Action{Kind: ActPermission, Request: request, Option: optionID})
						}
						if index > 0 {
							controls = append(controls, layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout))
						}
						controls = append(controls, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return button.Layout(gtx, th)
						}))
					}
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, controls...)
				}),
			)
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

func (state *viewState) layoutPlan(gtx layout.Context, th *theme.Theme, entries []protocol.PlanEntry, key string) layout.Dimensions {
	children := []layout.FlexChild{layout.Rigid(kit.Text(th, th.T.Section, th.R.Fg, i18n.T("Plan")))}
	for index, entry := range entries {
		mark := "[ ]"
		status := i18n.T("Pending")
		ink := th.R.Fg2
		switch entry.Status {
		case "in_progress":
			mark, status, ink = "[>]", i18n.T("In progress"), th.R.Lapis
		case "completed":
			mark, status, ink = "[x]", i18n.T("Completed"), th.R.Jade
		}
		entryKey := fmt.Sprintf("%s:%d", key, index)
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
					layout.Rigid(kit.Text(th, th.T.MonoSm, ink, mark)),
					layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
					layout.Rigid(kit.Text(th, th.T.Label, ink, status)),
					layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return state.selectable(gtx, th, entryKey, th.T.Body, th.R.Fg, entry.Content)
					}),
				)
			}),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (state *viewState) layoutTerminal(gtx layout.Context, th *theme.Theme, terminal *terminalState, key string, embedded bool) layout.Dimensions {
	if terminal == nil {
		return layout.Dimensions{}
	}
	body := func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, 4)
		if terminal.data.Command != "" {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return state.selectable(gtx, th, key+":command", th.T.Mono, th.R.Fg, "$ "+terminal.data.Command)
			}))
		}
		if terminal.text != "" {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return state.layoutCollapsibleText(gtx, th, key+":text", i18n.T("Terminal output"), terminal.text, 12)
				}),
			)
		}
		if terminal.data.Done && terminal.data.Exit != nil {
			exit := i18n.Tf("Exit %d", *terminal.data.Exit)
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return state.selectable(gtx, th, key+":exit", th.T.MonoSm, th.R.Fg3, exit)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	if embedded {
		return body(gtx)
	}
	return kit.Slab(gtx, th.R.Stone2, th.Cut.Small, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(th.Sp.S3).Layout(gtx, body)
	})
}

func (state *viewState) layoutCollapsibleText(gtx layout.Context, th *theme.Theme, key, title, value string, lines int) layout.Dimensions {
	disclosure := state.disclosure(key)
	button := state.button(key + ":toggle")
	button.Kind = kit.PlaqueQuiet
	button.Small = true
	button.Label = title
	if button.Clicked(gtx) {
		disclosure.Open = !disclosure.Open
	}
	preview := lastLines(value, lines)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return button.Layout(gtx, th)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if disclosure.Open {
				return disclosure.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return state.selectable(gtx, th, key+":full", th.T.MonoSm, th.R.Fg2, value)
					})
				})
			}
			return layout.Inset{Top: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return state.selectable(gtx, th, key+":preview", th.T.MonoSm, th.R.Fg2, preview)
			})
		}),
	)
}

func lastLines(value string, count int) string {
	lines := strings.Split(value, "\n")
	if len(lines) > count {
		lines = lines[len(lines)-count:]
	}
	return strings.Join(lines, "\n")
}

func (state *viewState) layoutAgentDiff(gtx layout.Context, th *theme.Theme, diff *protocol.Diff, key string) layout.Dimensions {
	view := state.diffs[key]
	if view == nil {
		view = &diffview.View{}
		state.diffs[key] = view
	}
	file := diffview.FromTexts(diff.Path, diff.Old, diff.New)
	dimensions := view.Layout(gtx, th, protocol.DiffResult{
		Files:     []protocol.FileDiff{file},
		Truncated: diff.Truncated,
	}, diffview.Options{FoldOver: 40, Wrap: true})
	for _, action := range view.Actions() {
		if action.Kind == diffview.ActOpen {
			state.actions = append(state.actions, Action{Kind: ActOpenPath, Value: action.Path})
		}
	}
	return dimensions
}

func (state *viewState) layoutAnswer(gtx layout.Context, th *theme.Theme, answer *protocol.AnswerData, key string) layout.Dimensions {
	if answer == nil {
		return layout.Dimensions{}
	}
	children := make([]layout.FlexChild, 0)
	if len(answer.Actions) > 0 {
		children = append(children, layout.Rigid(kit.Text(th, th.T.Label, th.R.Fg2, i18n.T("Actions"))))
		for index, answerAction := range answer.Actions {
			button := state.button(fmt.Sprintf("%s:action:%d", key, index))
			button.Kind = kit.PlaqueQuiet
			button.Small = true
			button.Label = i18n.T(answerAction.Label)
			if button.Label == "" {
				button.Label = answerAction.Value
			}
			if button.Clicked(gtx) {
				kind := ActCopy
				switch answerAction.Kind {
				case protocol.ActFile, protocol.ActDir:
					kind = ActOpenPath
				case protocol.ActURL:
					kind = ActOpenURL
				case protocol.ActCmd:
					kind = ActRunCommand
				}
				state.actions = append(state.actions, Action{Kind: kind, Value: answerAction.Value})
			}
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return button.Layout(gtx, th)
				}),
			)
		}
	}
	if answer.Plan != nil {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(kit.Text(th, th.T.Section, th.R.Fg, i18n.T("Command plan"))),
		)
		if answer.Plan.Summary != "" {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return state.selectable(gtx, th, key+":summary", th.T.Body, th.R.Fg2, answer.Plan.Summary)
			}))
		}
		for index, command := range answer.Plan.Commands {
			commandKey := fmt.Sprintf("%s:command:%d", key, index)
			button := state.button(commandKey + ":run")
			button.Kind = kit.PlaqueLine
			button.Small = true
			button.Label = i18n.T("Run")
			if button.Clicked(gtx) {
				state.actions = append(state.actions, Action{Kind: ActRunCommand, Value: command.Command})
			}
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return kit.Slab(gtx, th.R.Stone2, th.Cut.Small, func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(th.Sp.S3).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return state.selectable(gtx, th, commandKey+":value", th.T.MonoSm, th.R.Fg, command.Command)
								}),
								layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
										layout.Flexed(1, kit.Text(th, th.T.Label, tierInk(th, command.Tier), tierWord(command.Tier))),
										layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return button.Layout(gtx, th)
										}),
									)
								}),
							)
						})
					})
				}),
			)
		}
	}
	if answer.NeedsAgent {
		button := state.button(key + ":handoff")
		button.Kind = kit.PlaqueLine
		button.Label = i18n.T("Move to Agent")
		if button.Clicked(gtx) {
			state.actions = append(state.actions, Action{Kind: ActHandoff, Value: answer.Reason})
		}
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return kit.Slab(gtx, th.R.Field(palette.Attention), th.Cut.Mid, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(th.Sp.S4).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(kit.Claim(th, palette.Attention, i18n.T("Needs agent"))),
							layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return state.selectable(gtx, th, key+":reason", th.T.Body, th.R.Fg, answer.Reason)
							}),
							layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return button.Layout(gtx, th)
							}),
						)
					})
				})
			}),
		)
	}
	if len(children) == 0 {
		return layout.Dimensions{}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func tierInk(th *theme.Theme, tier string) color.NRGBA {
	switch tier {
	case protocol.TierDanger:
		return th.R.Jasper
	case protocol.TierSystem:
		return th.R.Amber
	case protocol.TierWrite:
		return th.R.Lapis
	default:
		return th.R.Fg2
	}
}
func tierWord(tier string) string {
	switch tier {
	case protocol.TierRead:
		return i18n.T("Read")
	case protocol.TierWrite:
		return i18n.T("Write")
	case protocol.TierSystem:
		return i18n.T("System")
	case protocol.TierDanger:
		return i18n.T("Danger")
	default:
		return i18n.T(tier)
	}
}

func (state *viewState) layoutError(gtx layout.Context, th *theme.Theme, problem protocol.ErrorData, key string) layout.Dimensions {
	return kit.Slab(gtx, th.R.Field(palette.Failed), th.Cut.Mid, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(th.Sp.S4).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(kit.Claim(th, palette.Failed, i18n.T("Failed"))),
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return state.selectable(gtx, th, key, th.T.Body, th.R.Fg, problem.Message)
				}),
			)
		})
	})
}

func (state *viewState) layoutTurnFooter(gtx layout.Context, th *theme.Theme, turn *turnState, checkpoints bool, key string) layout.Dimensions {
	if turn == nil || turn.end == nil {
		return layout.Dimensions{}
	}
	parts := make([]string, 0, 3)
	if turn.answer != nil && turn.answer.Via != "" {
		parts = append(parts, turn.answer.Via)
	}
	if turn.end.Usage != nil {
		usage := i18n.Tf("%d input · %d output tokens", turn.end.Usage.Input, turn.end.Usage.Output)
		if turn.end.Usage.Estimated {
			usage += " " + i18n.T("estimated")
		}
		parts = append(parts, usage)
	}
	if turn.end.Stop != "" && turn.end.Stop != protocol.StopEnd {
		parts = append(parts, i18n.T("Stopped")+": "+stopWord(turn.end.Stop))
	}
	if !checkpoints {
		return state.selectable(gtx, th, key, th.T.MonoSm, th.R.Fg3, strings.Join(parts, " · "))
	}
	changes := state.button(key + ":changes")
	changes.Kind = kit.PlaqueLine
	changes.Label = i18n.T("Changes")
	changes.Small = true
	revert := state.button(key + ":revert")
	revert.Kind = kit.PlaqueDangerLine
	revert.Label = i18n.T("Revert files")
	revert.Small = true
	if changes.Clicked(gtx) {
		state.actions = append(state.actions, Action{Kind: ActTurnChanges, Turn: turn.number})
	}
	if revert.Clicked(gtx) {
		state.actions = append(state.actions, Action{Kind: ActRevertTurn, Turn: turn.number})
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return state.selectable(gtx, th, key, th.T.MonoSm, th.R.Fg3, strings.Join(parts, " · "))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return changes.Layout(gtx, th)
		}),
		layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return revert.Layout(gtx, th)
		}),
	)
}
func stopWord(reason string) string {
	switch reason {
	case protocol.StopCancelled:
		return i18n.T("Cancelled")
	case protocol.StopError:
		return i18n.T("Error")
	case protocol.StopMaxTokens:
		return i18n.T("Token limit")
	case protocol.StopRefusal:
		return i18n.T("Refused")
	case protocol.StopMaxRounds:
		return i18n.T("Request limit")
	default:
		return i18n.T(reason)
	}
}

func (state *viewState) layoutMarkdown(gtx layout.Context, th *theme.Theme, key, source string) layout.Dimensions {
	document := state.markdown[key]
	if document == nil {
		document = &markdownState{}
		state.markdown[key] = document
	}
	if document.source != source {
		document.source = source
		document.blocks = parseMarkdown(source)
	}
	children := make([]layout.FlexChild, 0, len(document.blocks)*2)
	for index := range document.blocks {
		if index > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout))
		}
		block := &document.blocks[index]
		blockKey := fmt.Sprintf("%s:block:%d", key, index)
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return state.layoutMarkdownBlock(gtx, th, blockKey, block)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (state *viewState) layoutMarkdownBlock(gtx layout.Context, th *theme.Theme, key string, block *markdownBlock) layout.Dimensions {
	switch block.kind {
	case markdownHeading:
		style := th.T.Section
		if block.level == 1 {
			style = th.T.Title
		}
		return state.selectable(gtx, th, key, style, th.R.Fg, block.text)
	case markdownList:
		children := make([]layout.FlexChild, 0, len(block.items))
		for index, item := range block.items {
			prefix := "-"
			if item.ordered {
				prefix = strconv.Itoa(item.number) + "."
			}
			itemKey := fmt.Sprintf("%s:item:%d", key, index)
			indent := unit.Dp(float32(th.Sp.S4) * float32(item.depth))
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: indent}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
						layout.Rigid(kit.Text(th, th.T.MonoSm, th.R.Fg2, prefix)),
						layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return state.layoutInline(gtx, th, itemKey, item.text, th.T.Body)
						}),
					)
				})
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	case markdownQuote:
		return kit.Slab(gtx, th.R.Stone2, th.Cut.Small, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: th.Sp.S3, Bottom: th.Sp.S3, Left: th.Sp.S4, Right: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return state.selectable(gtx, th, key, th.T.Body, th.R.Fg2, block.text)
			})
		})
	case markdownCode:
		copyButton := state.button(key + ":copy")
		copyButton.Kind = kit.PlaqueQuiet
		copyButton.Small = true
		copyButton.Label = i18n.T("Copy")
		if copyButton.Clicked(gtx) {
			state.actions = append(state.actions, Action{Kind: ActCopy, Value: block.text})
		}
		language := block.language
		if language == "" {
			language = i18n.T("Code")
		}
		return kit.Slab(gtx, th.R.Stone2, th.Cut.Small, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(th.Sp.S3).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
							layout.Flexed(1, kit.Text(th, th.T.MonoSm, th.R.Fg3, language)),
							layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return copyButton.Layout(gtx, th)
							}),
						)
					}),
					layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return state.selectable(gtx, th, key+":code", th.T.Mono, th.R.Fg, block.text)
					}),
				)
			})
		})
	case markdownTable:
		return state.layoutTable(gtx, th, key, block)
	default:
		return state.layoutInline(gtx, th, key, block.text, th.T.Body)
	}
}

func (state *viewState) layoutTable(gtx layout.Context, th *theme.Theme, key string, block *markdownBlock) layout.Dimensions {
	rows := make([][]string, 0, len(block.rows)+1)
	rows = append(rows, block.head)
	rows = append(rows, block.rows...)
	children := make([]layout.FlexChild, 0, len(rows))
	for rowIndex, row := range rows {
		rowKey := fmt.Sprintf("%s:row:%d", key, rowIndex)
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			cells := make([]layout.FlexChild, 0, len(row))
			for column, value := range row {
				cellKey := fmt.Sprintf("%s:cell:%d", rowKey, column)
				style := th.T.Body
				if rowIndex == 0 {
					style = th.T.Label
				}
				cells = append(cells, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(th.Sp.S2).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return state.selectable(gtx, th, cellKey, style, th.R.Fg, value)
					})
				}))
			}
			return kit.Slab(gtx, th.R.Stone2, 0, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, cells...)
			})
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (state *viewState) layoutInline(gtx layout.Context, th *theme.Theme, key, source string, base theme.TextStyle) layout.Dimensions {
	type piece struct {
		span inlineSpan
		text string
		key  string
	}
	pieces := make([]piece, 0)
	for spanIndex, span := range parseInline(source) {
		words := splitWords(span.text)
		for wordIndex, word := range words {
			pieces = append(pieces, piece{span: span, text: word, key: fmt.Sprintf("%s:%d:%d", key, spanIndex, wordIndex)})
		}
	}
	calls := make([]op.CallOp, len(pieces))
	sizes := make([]image.Point, len(pieces))
	positions := make([]image.Point, len(pieces))
	available := max(0, gtx.Constraints.Max.X)
	x, y, rowHeight, width := 0, 0, 0, 0
	for index, piece := range pieces {
		child := gtx
		child.Constraints.Min = image.Point{}
		child.Constraints.Max.X = available
		record := op.Record(gtx.Ops)
		dimensions := state.layoutInlinePiece(child, th, piece.key, piece.text, piece.span, base)
		calls[index] = record.Stop()
		sizes[index] = dimensions.Size
		if x > 0 && x+dimensions.Size.X > available {
			x = 0
			y += rowHeight
			rowHeight = 0
		}
		positions[index] = image.Pt(x, y)
		x += dimensions.Size.X
		width = max(width, x)
		rowHeight = max(rowHeight, dimensions.Size.Y)
	}
	for index, call := range calls {
		offset := op.Offset(positions[index]).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
	}
	return layout.Dimensions{Size: gtx.Constraints.Constrain(image.Pt(width, y+rowHeight))}
}

func splitWords(value string) []string {
	if value == "" {
		return nil
	}
	words := make([]string, 0, strings.Count(value, " ")+1)
	start := 0
	for index, r := range value {
		if unicodeSpace(r) {
			if index > start {
				words = append(words, value[start:index])
			}
			words = append(words, value[index:index+1])
			start = index + 1
		}
	}
	if start < len(value) {
		words = append(words, value[start:])
	}
	return words
}

func unicodeSpace(value rune) bool {
	return value == ' ' || value == '\t' || value == '\n'
}

func (state *viewState) layoutInlinePiece(gtx layout.Context, th *theme.Theme, key, value string, span inlineSpan, base theme.TextStyle) layout.Dimensions {
	style := base
	ink := th.R.Fg
	switch span.kind {
	case inlineEmphasis:
		style.Face.Style = font.Italic
	case inlineStrong:
		style.Face.Weight = font.Bold
	case inlineCode:
		style = th.T.Mono
		ink = th.R.Fg2
	case inlineLink:
		ink = th.R.GoldInk
		link := state.links[key]
		if link == nil {
			link = new(widget.Clickable)
			state.links[key] = link
		}
		if link.Clicked(gtx) {
			state.actions = append(state.actions, Action{Kind: ActOpenURL, Value: span.url})
		}
		return link.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			pass := pointer.PassOp{}.Push(gtx.Ops)
			dimensions := state.selectable(gtx, th, key+":text", style, ink, value)
			pass.Pop()
			return dimensions
		})
	}
	return state.selectable(gtx, th, key, style, ink, value)
}

func (state *viewState) selectable(gtx layout.Context, th *theme.Theme, key string, style theme.TextStyle, ink color.NRGBA, value string) layout.Dimensions {
	selectable := state.selectables[key]
	if selectable == nil {
		selectable = new(widget.Selectable)
		selectable.LineHeight = style.Line
		selectable.LineHeightScale = 1
		state.selectables[key] = selectable
	}
	selectable.SetText(value)
	textMaterial := colorMaterial(gtx.Ops, ink)
	selectionMaterial := colorMaterial(gtx.Ops, th.R.Sel)
	return selectable.Layout(gtx, th.Shaper, style.Face, style.Size, textMaterial, selectionMaterial)
}

func colorMaterial(ops *op.Ops, ink color.NRGBA) op.CallOp {
	record := op.Record(ops)
	paint.ColorOp{Color: ink}.Add(ops)
	return record.Stop()
}

func (state *viewState) button(key string) *kit.Plaque {
	button := state.buttons[key]
	if button == nil {
		button = new(kit.Plaque)
		state.buttons[key] = button
	}
	return button
}

func (state *viewState) disclosure(key string) *kit.Disclosure {
	disclosure := state.disclosures[key]
	if disclosure == nil {
		disclosure = new(kit.Disclosure)
		state.disclosures[key] = disclosure
	}
	return disclosure
}
