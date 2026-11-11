package chat

import (
	"image"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// the field never gets shorter than composerMinimum so the control row keeps its place with or without text
const (
	composerMinimum unit.Dp = 96
	composerLines           = 8
	controlRow      unit.Dp = 32
)

type composerState struct {
	editor widget.Editor
	look   look.Theme

	send   kit.IconPlaque
	morph  look.Morph
	queue  kit.Plaque
	steer  kit.Plaque
	attach look.IconAction
	add    kit.Plaque
	focus  anim.Tween

	attachField kit.TextField
	attachOpen  bool
	attachFocus bool
	images      []string
	remove      map[string]*look.IconAction
	unqueue     map[string]*look.IconAction

	recall recallState
	slash  slashState

	submitted *Submission
	actions   []Action
	stopReady bool
	wantFocus bool
	ready     bool
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
	state.send.Small = true
	state.queue.Kind, state.queue.Small = kit.PlaqueGold, true
	state.steer.Kind, state.steer.Small = kit.PlaqueLine, true
	state.add.Kind, state.add.Small = kit.PlaqueLine, true
	state.attachField.Mono = true
	state.attachField.Editor.SingleLine, state.attachField.Editor.Submit = true, true
	state.remove = make(map[string]*look.IconAction)
	state.unqueue = make(map[string]*look.IconAction)
}

func (state *composerState) hasContent() bool {
	return strings.TrimSpace(state.editor.Text()) != "" || len(state.images) > 0
}

func (state *composerState) layout(gtx layout.Context, th *theme.Theme, options ComposerOptions) layout.Dimensions {
	state.look.Theme = th
	state.editor.LineHeight = th.T.Proof.Line
	if state.wantFocus && !options.Disabled {
		gtx.Execute(key.FocusCmd{Tag: &state.editor})
		state.wantFocus = false
	}
	// the first to ask for a key gets it, so these go before the editor
	state.processSlash(gtx, options)
	state.processRecall(gtx, options)
	state.processAnswer(gtx, options)
	state.processPaste(gtx, options)
	state.processEditorEvents(gtx, options)
	state.processAttach(gtx, options)
	state.processControls(gtx, options)

	if options.Disabled {
		gtx = gtx.Disabled()
		defer paint.PushOpacity(gtx.Ops, .5).Pop()
	}
	if len(options.Queue) == 0 {
		return state.layoutField(gtx, th, options)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return state.layoutQueue(gtx, th, options.Queue) }),
		layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return state.layoutField(gtx, th, options) }),
	)
}

func (state *composerState) layoutField(gtx layout.Context, th *theme.Theme, options ComposerOptions) layout.Dimensions {
	width := gtx.Constraints.Max.X
	top, side, foot, gap := gtx.Dp(13), gtx.Dp(16), gtx.Dp(8), gtx.Dp(8)
	inner := max(0, width-2*side)
	y := top

	record := op.Record(gtx.Ops)
	if len(state.images) > 0 {
		y += state.at(gtx, side, y, inner, func(gtx layout.Context) layout.Dimensions { return state.layoutAttachments(gtx, th) }) + gap
	}
	y += state.at(gtx, side, y, inner, func(gtx layout.Context) layout.Dimensions {
		return state.layoutEditor(gtx, th, options.Placeholder)
	})
	if state.attachOpen && options.CanAttach {
		y += gap + state.at(gtx, side, y+gap, inner, func(gtx layout.Context) layout.Dimensions {
			return state.layoutAttachField(gtx, th)
		})
	}
	row := gtx.Dp(controlRow)
	height := max(gtx.Dp(composerMinimum), y+gap+row+foot)
	// less inset than the text so the bare icons line up with its left edge
	state.at(gtx, foot, height-foot-row, width-2*foot, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = row, row
		return state.layoutControls(gtx, th, options)
	})
	content := record.Stop()

	bounds := image.Rect(0, 0, width, height)
	cut := gtx.Dp(th.Cut.Mid)
	kit.FillCut(gtx.Ops, bounds, cut, th.R.Stone3)
	focused := gtx.Focused(&state.editor) && gtx.Source.Enabled()
	focus := look.Transition(gtx, &state.focus, focused, th.Motion.FocusIn, th.Motion.FocusOut)
	// one tone above the pane is not an edge on a wallpaper palette, so the field keeps a hairline at rest
	kit.StrokeCut(gtx.Ops, bounds, cut, max(1, gtx.Dp(1)), palette.Mix(th.R.Vein, th.R.Fg2, focus))
	content.Add(gtx.Ops)
	state.layoutSlash(gtx, th, options, width)
	return layout.Dimensions{Size: bounds.Max}
}

func (state *composerState) at(gtx layout.Context, x, y, width int, widget layout.Widget) int {
	offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
	child := gtx
	child.Constraints = layout.Constraints{Min: image.Pt(width, 0), Max: image.Pt(width, gtx.Constraints.Max.Y)}
	dimensions := widget(child)
	offset.Pop()
	return dimensions.Size.Y
}

func (state *composerState) layoutEditor(gtx layout.Context, th *theme.Theme, placeholder string) layout.Dimensions {
	line := max(1, gtx.Sp(th.T.Proof.Line))
	gtx.Constraints.Min.Y = line
	gtx.Constraints.Max.Y = line * composerLines
	if state.editor.Len() == 0 && placeholder != "" {
		kit.Label{Style: th.T.Proof, Color: th.R.Fg3, MaxLines: 1}.Layout(gtx, th, placeholder)
	}
	defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
	ink := colorMaterial(gtx.Ops, th.R.Fg)
	selection := colorMaterial(gtx.Ops, palette.WithAlpha(th.R.Sel, .6))
	return state.editor.Layout(gtx, th.Shaper, th.T.Proof.Face, th.T.Proof.Size, ink, selection)
}

func (state *composerState) layoutControls(gtx layout.Context, th *theme.Theme, options ComposerOptions) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(options.Controls)+8)
	if options.CanAttach {
		state.attach.Icon, state.attach.Description = look.Icon.Attach, i18n.T("Attach a file")
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return state.attach.Layout(gtx, &state.look, th.R.Fg2)
		}))
	}
	if len(options.Controls) > 0 {
		// Send and the queue plaques are laid out after these, so their room is held back here
		reserve := gtx.Dp(48)
		if options.Busy && state.hasContent() {
			reserve = gtx.Dp(180)
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = max(0, gtx.Constraints.Max.X-reserve)
			return shareRow(gtx, options.Controls)
		}))
	}
	children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		return state.layoutHint(gtx, th, options)
	}))
	if options.Busy && state.hasContent() {
		state.steer.Label, state.queue.Label = i18n.T("Steer"), i18n.T("Queue")
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return state.steer.Layout(gtx, th) }),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return state.queue.Layout(gtx, th) }),
		)
	} else {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return state.layoutSendStop(gtx, th, options.Busy)
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

// in a row too narrow for every control the widest give way first, each cutting its own label
func shareRow(gtx layout.Context, controls []layout.Widget) layout.Dimensions {
	widths := make([]int, len(controls))
	total := 0
	measure := gtx.Disabled()
	measure.Constraints.Min = image.Point{}
	for index, control := range controls {
		record := op.Record(gtx.Ops)
		widths[index] = control(measure).Size.X
		record.Stop()
		total += widths[index]
	}
	for limit := gtx.Constraints.Max.X; total > limit; {
		// the widest, how many are that wide, and the next width down
		widest, below, count := 0, 0, 0
		for _, width := range widths {
			widest = max(widest, width)
		}
		for _, width := range widths {
			if width == widest {
				count++
			} else {
				below = max(below, width)
			}
		}
		step := min(widest-below, (total-limit+count-1)/count)
		if step <= 0 {
			break
		}
		for index, width := range widths {
			if width == widest {
				widths[index] -= step
				total -= step
			}
		}
	}
	children := make([]layout.FlexChild, len(controls))
	for index, control := range controls {
		children[index] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = widths[index]
			return control(gtx)
		})
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

func (state *composerState) layoutHint(gtx layout.Context, th *theme.Theme, options ComposerOptions) layout.Dimensions {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Constraints.Min.Y)
	hint := ""
	switch {
	case gtx.Focused(&state.editor) && !state.hasContent() && options.Answer != (AnswerKeys{}):
		hint = answerHint(options.Answer)
	case options.Busy && state.hasContent():
		hint = i18n.T("Enter queues, Ctrl Enter steers")
	case gtx.Focused(&state.editor) && state.editor.Len() == 0 && !options.Busy:
		hint = i18n.T("Enter to send, Shift Enter for a new line")
	}
	if hint == "" || size.X < gtx.Dp(220) {
		return layout.Dimensions{Size: size}
	}
	gtx.Constraints = layout.Exact(size)
	return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Right: th.Sp.S3}.Layout(gtx, kit.RawText(th, th.T.Caption, th.R.Fg3, hint))
	})
}

func (state *composerState) layoutSendStop(gtx layout.Context, th *theme.Theme, busy bool) layout.Dimensions {
	icon, ink := look.Icon.Send, th.R.OnGold
	state.send.Kind, state.send.Description = kit.PlaqueGold, i18n.T("Send")
	if busy {
		icon, ink = look.Icon.Stop, th.R.Fg2
		state.send.Kind, state.send.Description = kit.PlaqueQuiet, i18n.T("Stop")
	}
	// the morph keeps the live context: a disabled one drops its request for the next frame, and the plaque stood on Stop after the turn
	live := gtx
	if !busy && !state.hasContent() {
		gtx = gtx.Disabled()
	}
	// drawn bare so the icon can morph between send and stop
	state.send.Icon = nil
	dimensions := state.send.Layout(gtx, th)
	glyph := gtx.Dp(18)
	offset := op.Offset(image.Pt((dimensions.Size.X-glyph)/2, (dimensions.Size.Y-glyph)/2)).Push(gtx.Ops)
	if !gtx.Source.Enabled() {
		ink = palette.WithAlpha(ink, .5)
	}
	state.morph.Layout(live, &state.look, icon, ink, 18)
	offset.Pop()
	return dimensions
}

func (state *composerState) layoutQueue(gtx layout.Context, th *theme.Theme, queue []protocol.QueuedPrompt) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(queue)*2)
	for index, prompt := range queue {
		if index > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout))
		}
		remove := state.unqueue[prompt.ID]
		if remove == nil {
			remove = &look.IconAction{Icon: look.Icon.Close, Description: i18n.T("Remove from the queue")}
			state.unqueue[prompt.ID] = remove
		}
		if remove.Clicked(gtx) {
			state.actions = append(state.actions, Action{Kind: ActUnqueue, Value: prompt.ID})
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return state.layoutChip(gtx, th, i18n.T("Queued"), queuedLabel(prompt), remove, false)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// what waits is named by its words, with what is attached to them, or by the attachments alone
func queuedLabel(prompt protocol.QueuedPrompt) string {
	names := make([]string, len(prompt.Images))
	for index, attached := range prompt.Images {
		names[index] = attachmentName(attached.Path)
	}
	switch text := strings.TrimSpace(prompt.Text); {
	case len(names) == 0:
		return text
	case text == "":
		return strings.Join(names, ", ")
	default:
		return text + " · " + strings.Join(names, ", ")
	}
}

func (state *composerState) layoutChip(
	gtx layout.Context,
	th *theme.Theme,
	lead, text string,
	remove *look.IconAction,
	hug bool,
) layout.Dimensions {
	height := gtx.Dp(32)
	record := op.Record(gtx.Ops)
	row := gtx
	row.Constraints.Min.Y, row.Constraints.Max.Y = height, height
	if hug {
		row.Constraints.Min.X = 0
		row.Constraints.Max.X = min(row.Constraints.Max.X, gtx.Dp(260))
	}
	dimensions := layout.Inset{Left: th.Sp.S3, Right: 2}.Layout(row, func(gtx layout.Context) layout.Dimensions {
		label := func(gtx layout.Context) layout.Dimensions {
			style, ink := th.T.Caption, th.R.Fg2
			if hug {
				style = th.T.MonoSm
			}
			return kit.RawText(th, style, ink, text)(gtx)
		}
		body := layout.Flexed(1, label)
		if hug {
			body = layout.Rigid(label)
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if lead == "" {
					return layout.Dimensions{}
				}
				return layout.Inset{Right: th.Sp.S2}.Layout(gtx, kit.RawText(th, th.T.Caption, th.R.Fg3, lead))
			}),
			body,
			layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return remove.Layout(gtx, &state.look, th.R.Fg3) }),
		)
	})
	call := record.Stop()
	kit.FillCut(gtx.Ops, image.Rect(0, 0, dimensions.Size.X, height), gtx.Dp(th.Cut.Small), th.R.Stone1)
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(dimensions.Size.X, height)}
}

// with nothing typed these answer the waiting request, and are taken before the editor swallows Enter
func (state *composerState) processAnswer(gtx layout.Context, options ComposerOptions) {
	if options.Disabled || state.hasContent() {
		return
	}
	answer := func(name key.Name, kind ActionKind) {
		for {
			raw, ok := gtx.Event(key.Filter{Focus: &state.editor, Name: name})
			if !ok {
				return
			}
			if pressed, ok := raw.(key.Event); ok && pressed.State == key.Press {
				state.actions = append(state.actions, Action{Kind: kind})
			}
		}
	}
	if options.Answer.Allow {
		answer(key.NameReturn, ActAllow)
	}
	if options.Answer.Refuse {
		answer(key.NameEscape, ActRefuse)
	}
}

func answerHint(keys AnswerKeys) string {
	switch {
	case keys.Allow && keys.Refuse:
		return i18n.T("Enter allows once, Esc refuses")
	case keys.Allow:
		return i18n.T("Enter allows once")
	default:
		return i18n.T("Esc refuses")
	}
}

func (state *composerState) processEditorEvents(gtx layout.Context, options ComposerOptions) {
	if options.Disabled {
		return
	}
	if options.Busy {
		for {
			raw, ok := gtx.Event(key.Filter{
				Focus: &state.editor, Required: key.ModCtrl,
				Optional: key.ModShift | key.ModAlt | key.ModSuper, Name: key.NameReturn,
			})
			if !ok {
				break
			}
			if pressed, ok := raw.(key.Event); ok && pressed.State == key.Press {
				state.submit(state.editor.Text(), true)
			}
		}
	}
	for {
		raw, ok := state.editor.Update(gtx)
		if !ok {
			break
		}
		if submitted, ok := raw.(widget.SubmitEvent); ok {
			state.submit(submitted.Text, false)
		}
	}
}

func (state *composerState) processControls(gtx layout.Context, options ComposerOptions) {
	enabled := !options.Disabled
	if state.send.Clicked(gtx) && enabled {
		if options.Busy && !state.hasContent() {
			state.stopReady = true
		} else {
			state.submit(state.editor.Text(), false)
		}
	}
	if state.queue.Clicked(gtx) && enabled {
		state.submit(state.editor.Text(), false)
	}
	if state.steer.Clicked(gtx) && enabled {
		state.submit(state.editor.Text(), true)
	}
}

func (state *composerState) setText(text string) {
	state.editor.SetText(text)
	end := state.editor.Len()
	state.editor.SetCaret(end, end)
}

// what is already written stays above the quotation, so quoting twice builds one reply
func (state *composerState) quote(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	draft := strings.TrimRight(state.editor.Text(), "\n")
	if draft != "" {
		draft += "\n\n"
	}
	state.setText(draft + "> " + strings.ReplaceAll(text, "\n", "\n> ") + "\n\n")
	state.wantFocus = true
}

func (state *composerState) setDraft(draft Draft) {
	state.setText(draft.Text)
	state.images = append(state.images[:0], draft.Attachments...)
	clear(state.remove)
	state.attachOpen = false
	state.recall = recallState{}
	state.slash = slashState{}
}

func (state *composerState) submit(text string, steer bool) {
	if strings.TrimSpace(text) == "" && len(state.images) == 0 {
		return
	}
	state.submitted = &Submission{Text: text, Images: append([]string(nil), state.images...), Steer: steer}
	state.setDraft(Draft{})
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
