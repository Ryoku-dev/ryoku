package chat

import (
	"image"
	"path/filepath"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

type composerState struct {
	editor widget.Editor

	send   kit.Plaque
	steer  kit.Plaque
	stop   kit.Plaque
	model  kit.Plaque
	attach kit.Plaque
	add    kit.Plaque

	attachField kit.TextField
	attachOpen  bool
	images      []string
	queue       map[string]*kit.Plaque

	submitted  *Submission
	actions    []Action
	stopReady  bool
	modelReady bool
	focus      bool
	ready      bool
}

func (composer *Composer) ensureState() *composerState {
	if composer.state == nil {
		composer.state = &composerState{}
	}
	composer.state.ensure()
	return composer.state
}

func (state *composerState) ensure() {
	if state.ready {
		return
	}
	state.ready = true
	state.editor.Submit = true
	state.editor.InputHint = key.HintText
	state.editor.LineHeightScale = 1
	state.send.Kind = kit.PlaqueGold
	state.send.Label = i18n.T("Send")
	state.steer.Kind = kit.PlaqueLine
	state.steer.Label = i18n.T("Steer")
	state.stop.Kind = kit.PlaqueLine
	state.stop.Label = i18n.T("Stop")
	state.model.Kind = kit.PlaqueQuiet
	state.model.Small = true
	state.attach.Kind = kit.PlaqueQuiet
	state.attach.Label = i18n.T("Attach")
	state.attach.Small = true
	state.add.Kind = kit.PlaqueLine
	state.add.Label = i18n.T("Add")
	state.add.Small = true
	state.attachField.Label = i18n.T("Image path")
	state.attachField.Placeholder = i18n.T("Path to an image")
	state.attachField.Mono = true
	state.attachField.Editor.SingleLine = true
	state.attachField.Editor.Submit = true
	state.queue = make(map[string]*kit.Plaque)
}

func (state *composerState) layout(gtx layout.Context, th *theme.Theme, options ComposerOptions) layout.Dimensions {
	state.editor.LineHeight = th.T.Body.Line
	state.send.Label = i18n.T("Send")
	if options.Busy {
		state.send.Label = i18n.T("Queue")
	}
	state.model.Label = options.ModelLabel
	if state.model.Label == "" {
		state.model.Label = i18n.T("Choose model")
	}
	if state.focus && !options.Disabled {
		gtx.Execute(key.FocusCmd{Tag: &state.editor})
		state.focus = false
	}

	if !options.Disabled {
		if options.Busy {
			for {
				raw, ok := gtx.Event(key.Filter{
					Focus: &state.editor, Required: key.ModCtrl,
					Optional: key.ModShift | key.ModAlt | key.ModSuper,
					Name:     key.NameReturn,
				})
				if !ok {
					break
				}
				event := raw.(key.Event)
				if event.State == key.Press {
					state.submit(state.editor.Text(), true)
				}
			}
		}
		for {
			event, ok := state.editor.Update(gtx)
			if !ok {
				break
			}
			if submitted, ok := event.(widget.SubmitEvent); ok {
				state.submit(submitted.Text, false)
			}
		}
	}
	if state.send.Clicked(gtx) && !options.Disabled {
		state.submit(state.editor.Text(), false)
	}
	if state.steer.Clicked(gtx) && !options.Disabled && options.Busy {
		state.submit(state.editor.Text(), true)
	}
	if state.stop.Clicked(gtx) && !options.Disabled && options.Busy {
		state.stopReady = true
	}
	if state.model.Clicked(gtx) && !options.Disabled {
		state.modelReady = true
	}
	if state.attach.Clicked(gtx) && !options.Disabled && options.CanAttach {
		state.attachOpen = !state.attachOpen
		if state.attachOpen {
			state.attachField.Focus(gtx)
		}
	}

	if state.attachOpen && !options.Disabled && options.CanAttach {
		for {
			event, ok := state.attachField.Editor.Update(gtx)
			if !ok {
				break
			}
			if _, ok := event.(widget.SubmitEvent); ok {
				state.addImage()
			}
		}
		if state.add.Clicked(gtx) {
			state.addImage()
		}
	}

	contentGTX := gtx
	if options.Disabled {
		contentGTX = gtx.Disabled()
	}
	if options.Disabled {
		opacity := paint.PushOpacity(gtx.Ops, .5)
		defer opacity.Pop()
	}
	return kit.Slab(contentGTX, th.R.Stone2, th.Cut.Mid, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(th.Sp.S3).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, 8)
			if len(options.Queue) > 0 {
				children = append(children,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return state.layoutQueue(gtx, th, options.Queue)
					}),
					layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				)
			}
			if len(state.images) > 0 {
				children = append(children,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						items := make([]layout.FlexChild, 0, len(state.images)*2)
						for index, path := range state.images {
							if index > 0 {
								items = append(items, layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout))
							}
							name := filepath.Base(path)
							items = append(items, layout.Rigid(kit.Text(th, th.T.MonoSm, th.R.Fg2, name)))
						}
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, items...)
					}),
					layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				)
			}
			children = append(children,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return state.layoutEditor(gtx, th, options.Placeholder)
						}),
						layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if !options.Busy {
								return state.send.Layout(gtx, th)
							}
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return state.send.Layout(gtx, th)
								}),
								layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return state.steer.Layout(gtx, th)
								}),
							)
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					controls := make([]layout.FlexChild, 0, 5)
					if options.Busy {
						controls = append(controls,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								state.stop.Small = true
								return state.stop.Layout(gtx, th)
							}),
							layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
						)
					}
					if len(options.Controls) == 0 {
						controls = append(controls, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return state.model.Layout(gtx, th)
						}))
					} else {
						for index, control := range options.Controls {
							if index > 0 {
								controls = append(controls, layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout))
							}
							controls = append(controls, layout.Rigid(control))
						}
					}
					if options.CanAttach {
						controls = append(controls,
							layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return state.attach.Layout(gtx, th)
							}),
						)
					}
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, controls...)
				}),
			)
			if state.attachOpen && options.CanAttach {
				children = append(children,
					layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return state.attachField.Layout(gtx, th)
							}),
							layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return state.add.Layout(gtx, th)
							}),
						)
					}),
				)
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

func (state *composerState) layoutQueue(gtx layout.Context, th *theme.Theme, queue []protocol.QueuedPrompt) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(queue)*2)
	for index, prompt := range queue {
		if index > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout))
		}
		button := state.queue[prompt.ID]
		if button == nil {
			button = &kit.Plaque{Kind: kit.PlaqueQuiet, Label: i18n.T("Remove"), Small: true}
			state.queue[prompt.ID] = button
		}
		if button.Clicked(gtx) {
			state.actions = append(state.actions, Action{Kind: ActUnqueue, Value: prompt.ID})
		}
		text := prompt.Text
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return kit.Slab(gtx, th.R.Stone3, th.Cut.Small, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(th.Sp.S2).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, kit.Text(th, th.T.Body, th.R.Fg, text)),
						layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return button.Layout(gtx, th)
						}),
					)
				})
			})
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (state *composerState) layoutEditor(gtx layout.Context, th *theme.Theme, placeholder string) layout.Dimensions {
	lineHeight := max(1, gtx.Sp(th.T.Body.Line))
	minimum := lineHeight + 2*gtx.Dp(th.Sp.S2)
	maximum := lineHeight*8 + 2*gtx.Dp(th.Sp.S2)
	gtx.Constraints.Min.Y = minimum
	gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, maximum)

	return kit.Slab(gtx, th.R.Stone3, th.Cut.Mid, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(th.Sp.S2).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Stack{Alignment: layout.NW}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					if state.editor.Text() != "" || placeholder == "" {
						return layout.Dimensions{Size: gtx.Constraints.Min}
					}
					return kit.Label{Style: th.T.Body, Color: th.R.Fg3}.Layout(gtx, th, placeholder)
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					material := colorMaterial(gtx.Ops, th.R.Fg)
					selection := colorMaterial(gtx.Ops, th.R.Sel)
					clipper := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
					dimensions := state.editor.Layout(gtx, th.Shaper, th.T.Body.Face, th.T.Body.Size, material, selection)
					clipper.Pop()
					return dimensions
				}),
			)
		})
	})
}

func (state *composerState) submit(text string, steer bool) {
	if strings.TrimSpace(text) == "" && len(state.images) == 0 {
		return
	}
	state.submitted = &Submission{Text: text, Images: append([]string(nil), state.images...), Steer: steer}
	state.editor.SetText("")
	state.images = state.images[:0]
}

func (state *composerState) addImage() {
	path := strings.TrimSpace(state.attachField.Text())
	if path == "" {
		return
	}
	state.images = append(state.images, path)
	state.attachField.SetText("")
	state.attachOpen = false
}

func (state *composerState) submittedValue() (Submission, bool) {
	if state.submitted == nil {
		return Submission{}, false
	}
	submission := *state.submitted
	state.submitted = nil
	return submission, true
}

func (state *composerState) takeActions() []Action {
	actions := state.actions
	state.actions = nil
	return actions
}

func (state *composerState) takeStop() bool {
	ready := state.stopReady
	state.stopReady = false
	return ready
}

func (state *composerState) takeModel() bool {
	ready := state.modelReady
	state.modelReady = false
	return ready
}
