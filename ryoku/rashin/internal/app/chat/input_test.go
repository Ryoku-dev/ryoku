package chat

import (
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"

	"ryoku-rashin/internal/protocol"
)

type chatFrame struct {
	operations op.Ops
	router     input.Router
	now        time.Time
	theme      *theme.Theme
	size       image.Point
}

func newChatFrame(size image.Point) *chatFrame {
	return &chatFrame{
		now:   time.Unix(100, 0),
		theme: theme.New(palette.Signature(true)),
		size:  size,
	}
}

func (frame *chatFrame) context() layout.Context {
	frame.operations.Reset()
	return layout.Context{
		Ops:         &frame.operations,
		Now:         frame.now,
		Source:      frame.router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(frame.size),
	}
}

func (frame *chatFrame) finish() {
	frame.router.Frame(&frame.operations)
	frame.now = frame.now.Add(time.Millisecond)
}

func TestComposerEnterSubmits(t *testing.T) {
	frame := newChatFrame(image.Pt(720, 240))
	var composer Composer
	composer.SetText("Explain the setting")
	composer.Focus()
	composer.Layout(frame.context(), frame.theme, ComposerOptions{ModelLabel: "Auto"})
	frame.finish()

	frame.router.Queue(key.Event{Name: key.NameReturn, State: key.Press})
	composer.Layout(frame.context(), frame.theme, ComposerOptions{ModelLabel: "Auto"})
	frame.finish()

	submission, ok := composer.Submitted()
	if !ok || submission.Text != "Explain the setting" {
		t.Fatalf("submission = %#v, %v", submission, ok)
	}
}

func TestComposerShiftEnterInsertsNewline(t *testing.T) {
	frame := newChatFrame(image.Pt(720, 240))
	var composer Composer
	composer.SetText("First line")
	composer.Focus()
	composer.Layout(frame.context(), frame.theme, ComposerOptions{ModelLabel: "Auto"})
	frame.finish()

	frame.router.Queue(key.Event{Name: key.NameReturn, Modifiers: key.ModShift, State: key.Press})
	composer.Layout(frame.context(), frame.theme, ComposerOptions{ModelLabel: "Auto"})
	frame.finish()

	if _, ok := composer.Submitted(); ok {
		t.Fatal("Shift+Enter submitted the composer")
	}
	if !strings.Contains(composer.state.editor.Text(), "\n") {
		t.Fatalf("editor text = %q, want a newline", composer.state.editor.Text())
	}
}

func TestPermissionOptionClickProducesAction(t *testing.T) {
	frame := newChatFrame(image.Pt(800, 360))
	model := NewModel(protocol.SessionInfo{})
	permission := protocol.PermissionData{
		Request: "request-1",
		Title:   "Read configuration",
		Input:   "/etc/example.conf",
		Options: []protocol.PermissionOption{
			{ID: "once", Label: "Allow once", Kind: protocol.OptAllowOnce},
			{ID: "deny", Label: "Deny", Kind: protocol.OptRejectOnce},
		},
	}
	model.Apply(testEvent(t, 1, protocol.EvPermission, permission))
	var view View
	view.Layout(frame.context(), frame.theme, model)
	frame.finish()

	button := view.state.buttons["permission:request-1:once"]
	if button == nil {
		t.Fatal("permission control was not laid out")
	}
	button.Clickable.Click()
	view.Layout(frame.context(), frame.theme, model)
	frame.finish()

	actions := view.Actions()
	if len(actions) != 1 || actions[0].Kind != ActPermission || actions[0].Request != "request-1" || actions[0].Option != "once" {
		t.Fatalf("actions = %#v", actions)
	}
}

func TestScrolledUpViewDoesNotFollowNewEvents(t *testing.T) {
	frame := newChatFrame(image.Pt(480, 180))
	model := NewModel(protocol.SessionInfo{})
	for sequence := int64(1); sequence <= 24; sequence++ {
		model.Apply(testEvent(t, sequence, protocol.EvUser, protocol.UserData{Text: "A transcript row with enough text to occupy space."}))
	}
	var view View
	view.Layout(frame.context(), frame.theme, model)
	frame.finish()

	frame.router.Queue(
		pointer.Event{
			Kind: pointer.Move, Source: pointer.Mouse, PointerID: 1,
			Position: f32.Pt(120, 80),
		},
		pointer.Event{
			Kind: pointer.Scroll, Source: pointer.Mouse, PointerID: 1,
			Position: f32.Pt(120, 80), Scroll: f32.Pt(0, -120),
		},
	)
	view.Layout(frame.context(), frame.theme, model)
	frame.finish()
	if view.state.follow {
		t.Fatal("scrolling up did not suspend follow mode")
	}
	before := view.state.scroller.Position
	model.Apply(testEvent(t, 25, protocol.EvText, protocol.TextData{Message: "latest", Delta: "Newest answer"}))
	view.Layout(frame.context(), frame.theme, model)
	frame.finish()
	after := view.state.scroller.Position
	if after.First != before.First || after.Offset != before.Offset {
		t.Fatalf("scrolled-up position moved from %#v to %#v", before, after)
	}
}

func TestComposerCtrlEnterSteersWhileBusy(t *testing.T) {
	frame := newChatFrame(image.Pt(720, 280))
	var composer Composer
	composer.SetText("Use the new constraint")
	composer.Focus()
	options := ComposerOptions{Busy: true, ModelLabel: "Auto"}
	composer.Layout(frame.context(), frame.theme, options)
	frame.finish()

	frame.router.Queue(key.Event{Name: key.NameReturn, Modifiers: key.ModCtrl, State: key.Press})
	composer.Layout(frame.context(), frame.theme, options)
	frame.finish()

	submission, ok := composer.Submitted()
	if !ok || submission.Text != "Use the new constraint" || !submission.Steer {
		t.Fatalf("submission = %#v, %v", submission, ok)
	}
}

func TestQueueChipRaisesUnqueueAction(t *testing.T) {
	frame := newChatFrame(image.Pt(720, 320))
	var composer Composer
	options := ComposerOptions{
		Busy:  true,
		Queue: []protocol.QueuedPrompt{{ID: "queue-1", Text: "Run the focused test"}},
	}
	composer.Layout(frame.context(), frame.theme, options)
	frame.finish()

	button := composer.state.queue["queue-1"]
	if button == nil {
		t.Fatal("queued prompt remove control was not laid out")
	}
	button.Clickable.Click()
	composer.Layout(frame.context(), frame.theme, options)
	frame.finish()

	actions := composer.Actions()
	if len(actions) != 1 || actions[0].Kind != ActUnqueue || actions[0].Value != "queue-1" {
		t.Fatalf("actions = %#v", actions)
	}
}

func TestComposerUsesCallerControlsInsteadOfModelChip(t *testing.T) {
	frame := newChatFrame(image.Pt(720, 280))
	var composer Composer
	laidOut := 0
	control := func(gtx layout.Context) layout.Dimensions {
		laidOut++
		return layout.Dimensions{Size: image.Pt(48, 24)}
	}
	composer.Layout(frame.context(), frame.theme, ComposerOptions{
		ModelLabel: "Built-in",
		Controls:   []layout.Widget{control, control},
	})
	frame.finish()
	if laidOut != 2 {
		t.Fatalf("caller controls laid out %d times, want 2", laidOut)
	}
}

func TestToolGroupFoldsAndExpands(t *testing.T) {
	frame := newChatFrame(image.Pt(800, 640))
	model := NewModel(protocol.SessionInfo{})
	model.Apply(testEvent(t, 1, protocol.EvText, protocol.TextData{Message: "before", Delta: "I will inspect this."}))
	model.Apply(testEvent(t, 2, protocol.EvTool, protocol.ToolData{Call: "one", Title: "Read file", Kind: protocol.ToolRead, Status: protocol.ToolDone}))
	model.Apply(testEvent(t, 3, protocol.EvTool, protocol.ToolData{Call: "two", Title: "Run test", Kind: protocol.ToolExecute, Status: protocol.ToolDone}))
	model.Apply(testEvent(t, 4, protocol.EvTool, protocol.ToolData{Call: "three", Title: "Edit file", Kind: protocol.ToolEdit, Status: protocol.ToolDone}))
	model.Apply(testEvent(t, 5, protocol.EvText, protocol.TextData{Message: "after", Delta: "The change is ready."}))

	if len(model.state.rows) != 3 || len(model.state.rows[1].tools) != 3 {
		t.Fatalf("rows = %#v", model.state.rows)
	}
	var view View
	view.Layout(frame.context(), frame.theme, model)
	frame.finish()

	disclosure := view.state.disclosures["tool:one:group"]
	toggle := view.state.buttons["tool:one:group:toggle"]
	if disclosure == nil || toggle == nil {
		t.Fatal("tool group controls were not laid out")
	}
	if disclosure.Open {
		t.Fatal("finished tool group started expanded")
	}
	toggle.Clickable.Click()
	view.Layout(frame.context(), frame.theme, model)
	frame.finish()
	if !disclosure.Open {
		t.Fatal("tool group did not expand")
	}
}

func TestMessageForkMenuRaisesForkAtSequence(t *testing.T) {
	frame := newChatFrame(image.Pt(800, 360))
	model := NewModel(protocol.SessionInfo{})
	model.Apply(testEvent(t, 17, protocol.EvUser, protocol.UserData{Text: "Keep this context"}))
	var view View
	render := func() {
		view.ensureState().layoutRow(frame.context(), frame.theme, model.state, model.info.Checkpoints, &model.state.rows[0], 0)
		frame.finish()
	}
	render()

	point := f32.Pt(80, 40)
	frame.router.Queue(pointer.Event{
		Kind: pointer.Press, Source: pointer.Mouse, PointerID: 4,
		Buttons: pointer.ButtonSecondary, Position: point,
	})
	render()
	frame.router.Queue(pointer.Event{
		Kind: pointer.Release, Source: pointer.Mouse, PointerID: 4,
		Position: point,
	})
	render()

	button := view.state.buttons["user:17:menu:fork"]
	if view.state.menuKey != "user:17" || button == nil {
		t.Fatalf("fork menu did not open: key %q button %#v", view.state.menuKey, button)
	}
	button.Clickable.Click()
	render()

	actions := view.Actions()
	if len(actions) != 1 || actions[0].Kind != ActFork || actions[0].Seq != 17 {
		t.Fatalf("actions = %#v", actions)
	}
}

func TestTurnFooterRaisesCheckpointActions(t *testing.T) {
	frame := newChatFrame(image.Pt(800, 360))
	model := NewModel(protocol.SessionInfo{Checkpoints: true})
	model.Apply(testEvent(t, 1, protocol.EvTurnStart, protocol.TurnStartData{Turn: 3}))
	model.Apply(testEvent(t, 2, protocol.EvTurnEnd, protocol.TurnEndData{Turn: 3, Stop: protocol.StopEnd}))
	var view View
	view.Layout(frame.context(), frame.theme, model)
	frame.finish()

	changes := view.state.buttons["turn:0:changes"]
	revert := view.state.buttons["turn:0:revert"]
	if changes == nil || revert == nil {
		t.Fatal("checkpoint controls were not laid out")
	}
	changes.Clickable.Click()
	revert.Clickable.Click()
	view.Layout(frame.context(), frame.theme, model)
	frame.finish()
	actions := view.Actions()
	if len(actions) != 2 ||
		actions[0].Kind != ActTurnChanges || actions[0].Turn != 3 ||
		actions[1].Kind != ActRevertTurn || actions[1].Turn != 3 {
		t.Fatalf("actions = %#v", actions)
	}
}
