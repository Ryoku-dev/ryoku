package chat

import (
	"crypto/rand"
	"encoding/hex"
	"image"
	"io"
	"sort"
	"strings"

	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/symbols"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/prompts"
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
	attach kit.IconPlaque
	add    kit.Plaque
	focus  anim.Tween

	attachField kit.TextField
	attachOpen  bool
	attachFocus bool
	images      []string
	remove      map[string]*kit.IconPlaque
	unqueue     map[string]*kit.IconPlaque

	recall recallState
	slash  slashState

	ask       *askComposerState
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
	state.steer.Kind, state.steer.Small = kit.PlaqueTonal, true
	state.add.Kind, state.add.Small = kit.PlaqueTonal, true
	state.attachField.Mono = true
	state.attachField.Editor.SingleLine, state.attachField.Editor.Submit = true, true
	state.remove = make(map[string]*kit.IconPlaque)
	state.unqueue = make(map[string]*kit.IconPlaque)
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
	state.processDraftUndo(gtx, options)
	state.processEditorEvents(gtx, options)
	state.cancelChangedWriting()
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
	if state.askToolsVisible() {
		y += gap + state.at(gtx, side, y+gap, inner, func(gtx layout.Context) layout.Dimensions {
			return state.layoutAskTools(gtx, th)
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
	focused := gtx.Focused(&state.editor) && gtx.Source.Enabled()
	focus := look.Transition(gtx, &state.focus, focused, th.Motion.FocusIn, th.Motion.FocusOut)
	kit.FieldSurface(gtx, th, bounds, gtx.Dp(th.Cut.Mid), th.R.Field, focus, false)
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
	selection := colorMaterial(gtx.Ops, look.SelectedText(&state.look))
	return state.editor.Layout(gtx, th.Shaper, th.T.Proof.Face, th.T.Proof.Size, ink, selection)
}

func (state *composerState) layoutControls(gtx layout.Context, th *theme.Theme, options ComposerOptions) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(options.Controls)+8)
	if options.CanAttach {
		if a := state.ask; a != nil && a.feature("analyze") {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				a.attach.Form, a.attach.Label, a.attach.Icon = look.SelectQuiet, i18n.T("Attach"), look.Icon.Attach
				a.attach.Return = &state.editor
				if a.attachChoices == nil {
					a.attachChoices = []look.SelectOption{{Label: i18n.T("Attach"), Disabled: true}, {Label: i18n.T("Attach a file")}, {Label: i18n.T("Analyze a file…")}}
				}
				dims := a.attach.LayoutOptions(gtx, &state.look, a.hooks.Overlays, a.attachChoices, 0)
				if index, changed := a.attach.Changed(); changed {
					if index == 2 && a.hooks.Analyze != nil {
						a.hooks.Analyze()
					} else if index == 1 && options.Attach != nil {
						options.Attach()
					}
				}
				return dims
			}))
		} else {
			state.attach.Kind, state.attach.Small = kit.PlaqueQuiet, true
			state.attach.Icon, state.attach.Description = look.Icon.Attach, i18n.T("Attach a file")
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return state.attach.Layout(gtx, th)
			}))
		}
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
		state.queue.Kind = kit.PlaqueGold
		if options.Answer.Asked {
			state.queue.Kind = kit.PlaqueTonal
		}
		state.steer.Label, state.queue.Label = i18n.T("Steer"), i18n.T("Queue")
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return state.steer.Layout(gtx, th) }),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return state.queue.Layout(gtx, th) }),
		)
	} else {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return state.layoutSendStop(gtx, th, options.Busy, options.Answer.Asked)
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

func (state *composerState) layoutSendStop(gtx layout.Context, th *theme.Theme, busy, asked bool) layout.Dimensions {
	icon, ink := look.Icon.Send, th.R.OnGold
	state.send.Kind, state.send.Description = kit.PlaqueGold, i18n.T("Send")
	if asked {
		state.send.Kind, ink = kit.PlaqueTonal, th.R.Fg
	}
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
			remove = &kit.IconPlaque{Kind: kit.PlaqueQuiet, Small: true, Icon: look.Icon.Close, Description: i18n.T("Remove from the queue")}
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
		return i18n.Tf("%s, with %s", text, strings.Join(names, ", "))
	}
}

func (state *composerState) layoutChip(
	gtx layout.Context,
	th *theme.Theme,
	lead, text string,
	remove *kit.IconPlaque,
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
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return remove.Layout(gtx, th) }),
		)
	})
	call := record.Stop()
	bounds, cut := image.Rect(0, 0, dimensions.Size.X, height), gtx.Dp(th.Cut.Small)
	kit.SlabOutline(gtx, th, bounds, cut)
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
	case keys.Plan:
		return i18n.T("Enter starts building, Esc keeps planning")
	case keys.Allow && keys.Refuse:
		return i18n.T("Enter allows once, Esc refuses")
	case keys.Allow:
		return i18n.T("Enter allows once")
	case keys.Refuse:
		return i18n.T("Esc refuses")
	case keys.Asked:
		return i18n.T("Choose an answer above")
	default:
		return ""
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
	if a := state.ask; a != nil && a.request != "" && text != a.requestDraft {
		state.cancelWriting()
	}
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
	state.undoDraftReset()
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

type AskToolsOptions struct {
	Feature  func(string) bool
	Call     func(string, any, any, func(error))
	Notify   func(string)
	Language string
	Overlays *look.Overlays
	Analyze  func()
}

type promptVariable struct {
	name     string
	field    kit.TextField
	resolved bool
}

type askComposerState struct {
	hooks                                 AskToolsOptions
	library, attach                       look.Select
	writing                               kit.Plaque
	writingMenu                           kit.Menu
	writingMenuSize                       image.Point
	writingChoices                        []kit.MenuItem
	improve                               kit.IconPlaque
	insert, cancel                        kit.Plaque
	prompts                               []protocol.Prompt
	commands                              []protocol.Command
	promptChoices, attachChoices          []look.SelectOption
	loaded, loading, busy, voiceRead      bool
	voice, libraryOpen, slashTyping       bool
	variableFocus                         bool
	picked                                *protocol.Prompt
	variables                             []promptVariable
	variableList                          layout.List
	vars                                  map[string]string
	lastDraft                             string
	undoBefore, undoAfter                 string
	undoReady                             bool
	clipboard                             struct{}
	clipboardPending                      bool
	epoch                                 uint64
	request, requestFeature, requestDraft string
	writingResult                         *protocol.WritingCheckResult
	writingDraft                          string
	issueActions                          []draftIssueActions
	issueList                             look.Scroller
	applyWriting, dismissWriting          kit.Plaque
}

var improvePromptIcon = kit.Icon(symbols.AutoFixHigh)
var writingPromptIcon = kit.Icon(symbols.EditNote)

func (a *askComposerState) feature(id string) bool {
	return a.hooks.Feature != nil && a.hooks.Feature(id)
}

func (a *askComposerState) active() bool {
	return a.feature("prompts") || a.feature("enhance") || a.feature("askwriting") || a.feature("analyze")
}

func (state *composerState) askToolsVisible() bool {
	a := state.ask
	return a != nil && (a.feature("prompts") ||
		strings.TrimSpace(state.editor.Text()) != "" && (a.feature("enhance") || a.feature("askwriting")))
}

func (c *Composer) AskTools(options *ComposerOptions, hooks AskToolsOptions) {
	state := c.ensureState()
	if hooks.Feature == nil {
		return
	}
	on := hooks.Feature("prompts") || hooks.Feature("enhance") || hooks.Feature("askwriting") || hooks.Feature("analyze")
	if !on && state.ask == nil {
		return
	}
	if state.ask == nil {
		state.ask = &askComposerState{}
	}
	a := state.ask
	a.hooks = hooks
	if a.request != "" && !a.feature(a.requestFeature) {
		state.cancelWriting()
	}
	if !a.feature("prompts") {
		a.library.Close(hooks.Overlays)
		a.picked, a.variables, a.clipboardPending = nil, nil, false
		a.loaded, a.slashTyping = false, false
	}
	if !a.feature("askwriting") {
		a.closeWritingMenu()
		a.writingResult = nil
		a.voiceRead = false
	}
	if !a.feature("analyze") {
		a.attach.Close(hooks.Overlays)
	}
	if !on {
		a.picked, a.variables, a.clipboardPending = nil, nil, false
		a.loaded, a.voiceRead = false, false
		a.epoch++
		return
	}
	_, typing := state.promptQuery()
	if !typing {
		a.lastDraft = state.editor.Text()
	}
	if typing && !a.slashTyping {
		a.loaded = false
	}
	a.slashTyping = typing
	if a.feature("askwriting") && strings.TrimSpace(state.editor.Text()) != "" && !a.voiceRead {
		a.voiceRead = true
		voice := new(protocol.WritingVoice)
		hooks.Call(protocol.MWritingVoiceGet, protocol.WritingVoiceGetParams{}, voice, func(err error) {
			if err == nil {
				a.voice = strings.TrimSpace(voice.Notes) != ""
			}
		})
	}
	if a.feature("prompts") {
		options.Commands = func() []protocol.Command { return a.commands }
		if a.promptChoices == nil {
			a.promptChoices = []look.SelectOption{{Label: i18n.T("Prompt library"), Disabled: true}}
		}
		if !a.loaded && !a.loading {
			a.loading = true
			result := new(protocol.PromptsListResult)
			hooks.Call(protocol.MPromptsList, protocol.PromptsListParams{}, result, func(err error) {
				a.loading = false
				if err != nil {
					a.loaded = true
					hooks.Notify(err.Error())
					return
				}
				a.prompts = result.Prompts
				a.commands = make([]protocol.Command, len(a.prompts))
				a.promptChoices = make([]look.SelectOption, len(a.prompts)+1)
				a.promptChoices[0] = look.SelectOption{Label: i18n.T("Prompt library"), Disabled: true}
				for index, item := range a.prompts {
					a.commands[index] = protocol.Command{Name: item.ID, Description: promptFirstLine(item.Text), Hint: item.Kind}
					a.promptChoices[index+1] = look.SelectOption{Label: item.Name, Detail: promptFirstLine(item.Text)}
				}
				a.loaded = true
			})
		}
	}
}

func promptFirstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return line
}

func (state *composerState) promptQuery() (string, bool) {
	text := state.editor.Text()
	if index := strings.LastIndexByte(text, '\n'); index >= 0 {
		text = text[index+1:]
	}
	return slashQuery(text)
}

func (state *composerState) layoutAskTools(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	a := state.ask
	if a.feature("prompts") {
		state.processPromptClipboard(gtx)
	}
	var tools []layout.Widget
	if a.feature("prompts") {
		tools = append(tools, func(gtx layout.Context) layout.Dimensions {
			a.library.Form, a.library.Label, a.library.Icon = look.SelectQuiet, i18n.T("Prompt library"), look.Icon.Skills
			a.library.Return, a.library.Searched = &state.editor, true
			dims := a.library.LayoutOptions(gtx, &state.look, a.hooks.Overlays, a.promptChoices, 0)
			open := a.library.Opened(a.hooks.Overlays)
			if !open && a.libraryOpen && !a.loading {
				a.loaded = false
			}
			a.libraryOpen = open
			if index, changed := a.library.Changed(); changed && index > 0 && index <= len(a.prompts) {
				state.choosePrompt(gtx, a.prompts[index-1])
			}
			return dims
		})
	}
	if a.feature("enhance") && strings.TrimSpace(state.editor.Text()) != "" {
		tools = append(tools, func(gtx layout.Context) layout.Dimensions {
			a.improve.Kind, a.improve.Small = kit.PlaqueQuiet, true
			a.improve.Icon, a.improve.Description = improvePromptIcon, i18n.T("Improve my prompt")
			a.improve.Busy = a.busy
			if a.improve.Clicked(gtx) {
				state.transformDraft("enhance", nil, i18n.T("Improved, Ctrl+Z to undo"))
			}
			return a.improve.Layout(gtx, th)
		})
	}
	if a.feature("askwriting") && strings.TrimSpace(state.editor.Text()) != "" {
		tools = append(tools, func(gtx layout.Context) layout.Dimensions {
			return state.layoutWritingMenu(gtx, th)
		})
	}
	if a.writingResult != nil && a.writingDraft != state.editor.Text() {
		a.writingResult = nil
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return look.Flow{Gap: 8, LineGap: 8}.Layout(gtx, len(tools), func(gtx layout.Context, index int) layout.Dimensions {
				return tools[index](gtx)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return state.layoutWritingIssues(gtx, th)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if a.picked == nil || !a.feature("prompts") {
				return layout.Dimensions{}
			}
			return state.layoutPromptVariables(gtx, th)
		}),
	)
}

func (state *composerState) choosePrompt(gtx layout.Context, item protocol.Prompt) {
	a := state.ask
	if a.busy {
		return
	}
	a.epoch++
	a.picked = &item
	a.variables = nil
	content := a.lastDraft
	if _, typing := state.promptQuery(); typing {
		if index := strings.LastIndexByte(state.editor.Text(), '\n'); index >= 0 {
			content = strings.TrimRight(state.editor.Text()[:index], "\n")
		}
	}
	a.vars = map[string]string{"lang": a.hooks.Language, "content": content, "selection": content}
	a.clipboardPending = false
	for _, name := range prompts.Variables(item.Text) {
		if name == "date" || name == "lang" {
			continue
		}
		if (name == "content" || name == "selection") && content != "" {
			continue
		}
		field := promptVariable{name: name}
		field.field.Label = i18n.Tf("Fill %s", name)
		field.field.Editor.SingleLine = name != "content" && name != "selection"
		a.variables = append(a.variables, field)
		if name == "content" || name == "selection" {
			a.clipboardPending = true
		}
	}
	a.variableFocus = len(a.variables) > 0
	if a.clipboardPending {
		a.clipboardTarget(gtx)
		gtx.Execute(clipboard.ReadCmd{Tag: &a.clipboard})
	}
	state.slash.open = false
	state.slash.dismissed = state.editor.Text()
	if len(a.variables) == 0 {
		state.expandPrompt()
	}
}

func (state *composerState) processPromptClipboard(gtx layout.Context) {
	a := state.ask
	a.clipboardTarget(gtx)
	for {
		raw, ok := gtx.Event(transfer.TargetFilter{Target: &a.clipboard, Type: "text/plain"})
		if !ok {
			break
		}
		data, ok := raw.(transfer.DataEvent)
		if !ok {
			continue
		}
		reader := data.Open()
		text, err := io.ReadAll(io.LimitReader(reader, 64<<10))
		reader.Close()
		if err == nil && len(text) > 0 && a.clipboardPending && a.picked != nil {
			unfilled := 0
			for index := range a.variables {
				field := &a.variables[index]
				if field.name == "content" || field.name == "selection" {
					value := field.field.Text()
					if value == "" {
						value = string(text)
					}
					a.vars[field.name], field.resolved = value, true
				} else {
					unfilled++
				}
			}
			a.variableFocus = unfilled > 0
			if unfilled == 0 {
				state.expandPrompt()
			}
		}
		a.clipboardPending = false
	}
}

func (state *composerState) layoutPromptVariables(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	a := state.ask
	if a.variableFocus {
		for index := range a.variables {
			if !a.variables[index].resolved {
				gtx.Execute(key.FocusCmd{Tag: &a.variables[index].field.Editor})
				break
			}
		}
		a.variableFocus = false
	}
	children := []layout.FlexChild{
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(kit.Text(th, th.T.Detail, th.R.Fg2, i18n.Tf("Fill in %s", a.picked.Name))),
	}
	children = append(children,
		layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			a.variableList.Axis = layout.Vertical
			gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(240))
			return a.variableList.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
				return look.Flow{Gap: 8, LineGap: 8}.Layout(gtx, len(a.variables), func(gtx layout.Context, index int) layout.Dimensions {
					field := &a.variables[index]
					if field.resolved {
						return layout.Dimensions{}
					}
					width := gtx.Constraints.Max.X
					if field.name != "content" && field.name != "selection" {
						width = min(width, gtx.Dp(220))
					}
					gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
					gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(110))
					return field.field.Layout(gtx, th)
				})
			})
		}),
	)
	a.insert.Kind, a.insert.Small, a.insert.Label, a.insert.Busy = kit.PlaqueTonal, true, i18n.T("Use prompt"), a.busy
	a.cancel.Kind, a.cancel.Small, a.cancel.Label = kit.PlaqueQuiet, true, i18n.T("Cancel")
	if a.insert.Clicked(gtx) {
		for index := range a.variables {
			variable := &a.variables[index]
			if !variable.resolved {
				a.vars[variable.name] = variable.field.Text()
			}
		}
		state.expandPrompt()
	}
	if a.cancel.Clicked(gtx) {
		a.picked, a.variables, a.clipboardPending = nil, nil, false
		a.epoch++
		state.wantFocus = true
	}
	children = append(children,
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.insert.Layout(gtx, th) }),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.cancel.Layout(gtx, th) }),
			)
		}),
	)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (state *composerState) expandPrompt() {
	a := state.ask
	if a.busy || a.picked == nil || !a.feature("prompts") {
		return
	}
	before, epoch := state.editor.Text(), a.epoch
	result := new(protocol.PromptExpandResult)
	a.busy = true
	a.hooks.Call(protocol.MPromptsExpand, protocol.PromptExpandParams{ID: a.picked.ID, Vars: a.vars}, result, func(err error) {
		a.busy = false
		if err != nil {
			a.hooks.Notify(err.Error())
			return
		}
		if epoch != a.epoch || !a.feature("prompts") || state.editor.Text() != before {
			a.hooks.Notify(i18n.T("Your draft changed, choose the prompt again"))
			return
		}
		state.replaceDraft(result.Text, i18n.T("Prompt inserted, Ctrl+Z to undo"))
		a.picked, a.variables, a.clipboardPending = nil, nil, false
	})
}

func (state *composerState) transformDraft(action string, options map[string]string, notice string) {
	a := state.ask
	feature := "askwriting"
	if action == "enhance" {
		feature = "enhance"
	}
	if a.busy || !a.feature(feature) {
		return
	}
	before, epoch := state.editor.Text(), a.epoch
	if strings.TrimSpace(before) == "" {
		return
	}
	result := new(protocol.WritingTransformResult)
	request := state.beginWriting(feature, before)
	if request == "" {
		return
	}
	a.hooks.Call(protocol.MWritingTransform, protocol.WritingTransformParams{Request: request, Text: before, Action: action, Options: options, Language: a.hooks.Language}, result, func(err error) {
		if !state.finishWriting(request, err) {
			return
		}
		if epoch != a.epoch || !a.feature(feature) || state.editor.Text() != before {
			a.hooks.Notify(i18n.T("Your draft changed, try the writing tool again"))
			return
		}
		state.replaceDraft(result.Text, notice)
	})
}

func (state *composerState) checkDraft() {
	a := state.ask
	if a.busy || !a.feature("askwriting") {
		return
	}
	before, epoch := state.editor.Text(), a.epoch
	if strings.TrimSpace(before) == "" {
		return
	}
	result := new(protocol.WritingCheckResult)
	request := state.beginWriting("askwriting", before)
	if request == "" {
		return
	}
	a.hooks.Call(protocol.MWritingCheck, protocol.WritingCheckParams{Request: request, Text: before, Kinds: []string{"grammar"}, Language: a.hooks.Language}, result, func(err error) {
		if !state.finishWriting(request, err) {
			return
		}
		if epoch != a.epoch || !a.feature("askwriting") || state.editor.Text() != before {
			a.hooks.Notify(i18n.T("Your draft changed, check the writing again"))
			return
		}
		a.reviewWriting(before, result)
	})
}

func applyDraftFixes(text string, issues []protocol.WritingIssue) string {
	if len(issues) == 0 {
		return text
	}
	offsets := make([]int, 0, len(text)+1)
	for at, r := range text {
		offsets = append(offsets, at)
		if r > 0xffff {
			offsets = append(offsets, -1)
		}
	}
	offsets = append(offsets, len(text))
	sort.SliceStable(issues, func(i, j int) bool { return issues[i].Start < issues[j].Start })
	var out strings.Builder
	out.Grow(len(text))
	cursor := 0
	for _, issue := range issues {
		if len(issue.Replacements) == 0 || issue.Start < 0 || issue.End < issue.Start || issue.End >= len(offsets) {
			continue
		}
		start, end := offsets[issue.Start], offsets[issue.End]
		if start < cursor || end < 0 {
			continue
		}
		out.WriteString(text[cursor:start])
		out.WriteString(issue.Replacements[0])
		cursor = end
	}
	out.WriteString(text[cursor:])
	return out.String()
}

func (state *composerState) replaceDraft(text, notice string) {
	a := state.ask
	if text == state.editor.Text() {
		a.hooks.Notify(i18n.T("No changes needed"))
		return
	}
	a.undoBefore, a.undoAfter = state.editor.Text(), text
	a.undoReady = true
	state.setText(text)
	state.wantFocus = true
	a.hooks.Notify(notice)
}

func (state *composerState) undoDraftReset() {
	state.cancelWriting()
	if a := state.ask; a != nil {
		a.undoBefore, a.undoAfter = "", ""
		a.undoReady = false
		a.picked, a.variables, a.clipboardPending = nil, nil, false
		a.lastDraft = state.editor.Text()
		a.epoch++
	}
}

func (state *composerState) processDraftUndo(gtx layout.Context, options ComposerOptions) {
	a := state.ask
	if a == nil || !a.active() || options.Disabled || !a.undoReady || state.editor.Text() != a.undoAfter {
		return
	}
	for {
		raw, ok := gtx.Event(key.Filter{Focus: &state.editor, Name: "Z", Required: key.ModShortcut})
		if !ok {
			return
		}
		if pressed, ok := raw.(key.Event); ok && pressed.State == key.Press {
			state.setText(a.undoBefore)
			a.undoBefore, a.undoAfter = "", ""
			a.undoReady = false
			a.hooks.Notify(i18n.T("Draft restored"))
		}
	}
}

func (c *Composer) ReplaceDraft(text, notice string) {
	state := c.ensureState()
	if state.ask != nil && state.ask.active() {
		state.replaceDraft(text, notice)
	} else {
		state.setText(text)
	}
}

func (c *Composer) RefreshAskTools() {
	if a := c.ensureState().ask; a != nil {
		a.loaded, a.voiceRead = false, false
	}
}

func (state *composerState) beginWriting(feature, draft string) string {
	a := state.ask
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		a.hooks.Notify(err.Error())
		return ""
	}
	request := "composer-" + hex.EncodeToString(id[:])
	a.request, a.requestFeature, a.requestDraft = request, feature, draft
	a.busy = true
	return request
}

func (state *composerState) finishWriting(request string, err error) bool {
	a := state.ask
	if a == nil || a.request != request {
		return false
	}
	if err != nil {
		state.cancelWriting()
		a.hooks.Notify(err.Error())
		return false
	}
	a.request, a.requestFeature, a.requestDraft = "", "", ""
	a.busy = false
	return true
}

func (state *composerState) cancelChangedWriting() {
	if a := state.ask; a != nil && a.request != "" && state.editor.Text() != a.requestDraft {
		state.cancelWriting()
	}
}

func (state *composerState) cancelWriting() {
	a := state.ask
	if a == nil || a.request == "" {
		return
	}
	request := a.request
	a.request, a.requestFeature, a.requestDraft = "", "", ""
	a.busy = false
	a.epoch++
	a.hooks.Call(protocol.MWritingCancel, protocol.WritingCancelParams{Request: request}, new(protocol.Empty), nil)
}

func (c *Composer) WritingRequest() string {
	if c.state != nil && c.state.ask != nil {
		return c.state.ask.request
	}
	return ""
}

func (c *Composer) CancelAskTools() {
	if c.state == nil || c.state.ask == nil {
		return
	}
	state, a := c.state, c.state.ask
	state.cancelWriting()
	a.epoch++
	a.picked, a.variables, a.clipboardPending = nil, nil, false
	a.library.Close(a.hooks.Overlays)
	a.closeWritingMenu()
	a.attach.Close(a.hooks.Overlays)
	state.slash.open = false
}

func (a *askComposerState) clipboardTarget(gtx layout.Context) {
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &a.clipboard)
	pass.Pop()
}
