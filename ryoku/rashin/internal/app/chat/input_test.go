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

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type chatFrame struct {
	operations op.Ops
	router     input.Router
	now        time.Time
	theme      *look.Theme
	overlays   look.Overlays
	size       image.Point
}

func newChatFrame(size image.Point) *chatFrame {
	return &chatFrame{now: time.Unix(100, 0), theme: look.New(0), size: size}
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

func (frame *chatFrame) click(x, y float32) {
	at := f32.Pt(x, y)
	frame.router.Queue(
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: at},
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at},
	)
}

func (frame *chatFrame) press(name key.Name) {
	frame.router.Queue(key.Event{Name: name, State: key.Press})
}

func pressInComposer(t *testing.T, options ComposerOptions, text string, pressed key.Event) (*Composer, Submission, bool) {
	t.Helper()
	frame := newChatFrame(image.Pt(720, 280))
	composer := new(Composer)
	composer.SetText(text)
	composer.Focus()
	composer.Layout(frame.context(), frame.theme, options)
	frame.finish()

	frame.router.Queue(pressed)
	composer.Layout(frame.context(), frame.theme, options)
	frame.finish()
	submission, ok := composer.Submitted()
	return composer, submission, ok
}

func TestComposerEnterSubmits(t *testing.T) {
	_, submission, ok := pressInComposer(t, ComposerOptions{}, "Explain the setting", key.Event{Name: key.NameReturn, State: key.Press})
	if !ok || submission.Text != "Explain the setting" || submission.Steer {
		t.Fatalf("submission = %#v, %v", submission, ok)
	}
}

func TestComposerShiftEnterBreaksTheLine(t *testing.T) {
	pressed := key.Event{Name: key.NameReturn, Modifiers: key.ModShift, State: key.Press}
	composer, _, ok := pressInComposer(t, ComposerOptions{}, "First line", pressed)
	if ok {
		t.Fatal("Shift+Enter submitted the composer")
	}
	if text := composer.state.editor.Text(); !strings.Contains(text, "\n") {
		t.Fatalf("editor text = %q, want a newline", text)
	}
}

func TestComposerCtrlEnterSteersARunningTurn(t *testing.T) {
	pressed := key.Event{Name: key.NameReturn, Modifiers: key.ModCtrl, State: key.Press}
	_, submission, ok := pressInComposer(t, ComposerOptions{Busy: true}, "Use the new constraint", pressed)
	if !ok || submission.Text != "Use the new constraint" || !submission.Steer {
		t.Fatalf("submission = %#v, %v", submission, ok)
	}
}

func TestQueuedPromptCanBeTakenBack(t *testing.T) {
	frame := newChatFrame(image.Pt(720, 320))
	var composer Composer
	options := ComposerOptions{
		Busy:  true,
		Queue: []protocol.QueuedPrompt{{ID: "queue-1", Text: "Run the focused test"}},
	}
	composer.Layout(frame.context(), frame.theme, options)
	frame.finish()

	remove := composer.state.unqueue["queue-1"]
	if remove == nil {
		t.Fatal("the queued prompt has no way to remove it")
	}
	remove.Clickable.Click()
	composer.Layout(frame.context(), frame.theme, options)
	frame.finish()

	actions := composer.Actions()
	if len(actions) != 1 || actions[0].Kind != ActUnqueue || actions[0].Value != "queue-1" {
		t.Fatalf("actions = %#v", actions)
	}
}

func TestSettledComposerAsksForNoFrame(t *testing.T) {
	previous := anim.Reduced()
	anim.SetReduce(false)
	defer anim.SetReduce(previous)

	frame := newChatFrame(image.Pt(760, 240))
	var composer Composer
	composer.SetText("Continue")
	composer.Layout(frame.context(), frame.theme, ComposerOptions{})
	frame.finish()
	// send turns into stop, the icon morphs and then has to rest
	composer.Layout(frame.context(), frame.theme, ComposerOptions{Busy: true})
	frame.finish()
	frame.router.WakeupTime()
	frame.now = frame.now.Add(time.Second)
	composer.Layout(frame.context(), frame.theme, ComposerOptions{Busy: true})
	frame.finish()
	if wake, ok := frame.router.WakeupTime(); ok {
		t.Fatalf("a composer at rest asked for a frame at %v", wake)
	}
}

func TestPressingTheComposerTextFocusesIt(t *testing.T) {
	frame := newChatFrame(image.Pt(720, 96))
	var composer Composer
	draw := func() {
		composer.Layout(frame.context(), frame.theme, ComposerOptions{CanAttach: true})
		frame.finish()
	}
	draw()
	frame.click(200, 24)
	draw()
	draw()
	if !frame.router.Source().Focused(&composer.state.editor) {
		t.Fatal("a press on the text left the composer without the keyboard")
	}
}

func TestComposerControlsTakeThePointer(t *testing.T) {
	frame := newChatFrame(image.Pt(720, 96))
	var composer Composer
	var control kit.Click
	asked, pressed := 0, 0
	options := ComposerOptions{
		CanAttach: true,
		Attach:    func() { asked++ },
		Controls: []layout.Widget{func(gtx layout.Context) layout.Dimensions {
			if control.Clicked(gtx) {
				pressed++
			}
			return control.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: image.Pt(60, 28)}
			})
		}},
	}
	draw := func() {
		composer.Layout(frame.context(), frame.theme, options)
		frame.finish()
	}
	composer.SetText("Check the journal")
	draw()

	// the control row is 32 high, 8 above the foot: attach leads it, send ends it
	frame.click(22, 72)
	draw()
	draw()
	if asked != 1 {
		t.Fatalf("attach asked for files %d times, want once", asked)
	}
	frame.click(66, 72)
	draw()
	draw()
	if pressed != 1 {
		t.Fatalf("the page's own control took %d presses, want one", pressed)
	}
	frame.click(696, 72)
	draw()
	draw()
	if submission, ok := composer.Submitted(); !ok || submission.Text != "Check the journal" {
		t.Fatalf("send submitted %#v, %v", submission, ok)
	}
}

func TestAttachOpensThePathFieldWithoutAChooser(t *testing.T) {
	frame := newChatFrame(image.Pt(720, 160))
	var composer Composer
	draw := func() {
		composer.Layout(frame.context(), frame.theme, ComposerOptions{CanAttach: true})
		frame.finish()
	}
	draw()
	frame.click(22, 72)
	draw()
	draw()
	if !composer.state.attachOpen {
		t.Fatal("attach did nothing when no chooser was given")
	}
}

func focusedComposer(frame *chatFrame, composer *Composer, options ComposerOptions) func() {
	draw := func() {
		composer.Layout(frame.context(), frame.theme, options)
		frame.finish()
	}
	composer.Focus()
	draw()
	draw()
	return draw
}

func TestUpRecallsSentPrompts(t *testing.T) {
	frame := newChatFrame(image.Pt(720, 96))
	var composer Composer
	draw := focusedComposer(frame, &composer, ComposerOptions{History: func() []string { return []string{"first question", "second question"} }})
	text := func() string { return composer.state.editor.Text() }

	for _, step := range []struct {
		key  key.Name
		want string
	}{
		{key.NameUpArrow, "second question"},
		{key.NameUpArrow, "first question"},
		{key.NameUpArrow, "first question"},
		{key.NameDownArrow, "second question"},
		{key.NameDownArrow, ""},
	} {
		frame.press(step.key)
		draw()
		if text() != step.want {
			t.Fatalf("after %s the composer holds %q, want %q", step.key, text(), step.want)
		}
	}
}

func TestAnEditedRecallIsADraft(t *testing.T) {
	frame := newChatFrame(image.Pt(720, 96))
	var composer Composer
	draw := focusedComposer(frame, &composer, ComposerOptions{History: func() []string { return []string{"first question", "second question"} }})
	frame.press(key.NameUpArrow)
	draw()
	composer.state.editor.Insert(" again")
	draw()
	frame.press(key.NameUpArrow)
	draw()
	if text := composer.state.editor.Text(); text != "second question again" {
		t.Fatalf("Up replaced an edited prompt with %q", text)
	}
}

func TestSlashCompletesACommand(t *testing.T) {
	commands := []protocol.Command{{Name: "compact"}, {Name: "clear"}, {Name: "model"}}
	frame := newChatFrame(image.Pt(720, 96))
	var composer Composer
	draw := focusedComposer(frame, &composer, ComposerOptions{Commands: func() []protocol.Command { return commands }})
	text := func() string { return composer.state.editor.Text() }

	composer.SetText("/c")
	draw()
	frame.press(key.NameDownArrow)
	draw()
	frame.press(key.NameReturn)
	draw()
	if _, sent := composer.Submitted(); sent || text() != "/clear " {
		t.Fatalf("Enter on the second match left %q, sent %v", text(), sent)
	}

	composer.SetText("/compact")
	draw()
	frame.press(key.NameReturn)
	draw()
	if submission, sent := composer.Submitted(); !sent || submission.Text != "/compact" {
		t.Fatalf("Enter on a command typed in full sent %#v, %v", submission, sent)
	}
}

func TestEscapeShutsTheCommandList(t *testing.T) {
	frame := newChatFrame(image.Pt(720, 96))
	var composer Composer
	commands := func() []protocol.Command { return []protocol.Command{{Name: "compact"}} }
	draw := focusedComposer(frame, &composer, ComposerOptions{Commands: commands})
	composer.SetText("/co")
	draw()
	frame.press(key.NameEscape)
	draw()
	frame.press(key.NameReturn)
	draw()
	if submission, sent := composer.Submitted(); !sent || submission.Text != "/co" {
		t.Fatalf("after Escape, Enter sent %#v, %v; want the text as typed", submission, sent)
	}
}

// The composer has the keyboard while an agent waits for an answer, so it is
// the composer that has to tell an answer from a message. Only an empty one
// can: words mean the reader is writing, and Enter sends them.
func TestEnterAndEscapeAnswerOnlyFromAnEmptyComposer(t *testing.T) {
	both := ComposerOptions{Busy: true, Answer: AnswerKeys{Allow: true, Refuse: true}}
	kinds := func(composer *Composer) []ActionKind {
		var kinds []ActionKind
		for _, action := range composer.Actions() {
			kinds = append(kinds, action.Kind)
		}
		return kinds
	}
	enter, escape := key.Event{Name: key.NameReturn, State: key.Press}, key.Event{Name: key.NameEscape, State: key.Press}

	composer, _, sent := pressInComposer(t, both, "", enter)
	if raised := kinds(composer); sent || len(raised) != 1 || raised[0] != ActAllow {
		t.Fatalf("Enter in an empty composer raised %v, sent %v", raised, sent)
	}
	composer, _, _ = pressInComposer(t, both, "", escape)
	if raised := kinds(composer); len(raised) != 1 || raised[0] != ActRefuse {
		t.Fatalf("Escape in an empty composer raised %v", raised)
	}
	composer, submission, sent := pressInComposer(t, both, "no, use the other file", enter)
	if raised := kinds(composer); !sent || submission.Text != "no, use the other file" || len(raised) != 0 {
		t.Fatalf("Enter with words sent %v %q and raised %v", sent, submission.Text, raised)
	}
	composer, _, _ = pressInComposer(t, both, "no, use the other file", escape)
	if raised := kinds(composer); len(raised) != 0 || composer.state.editor.Text() == "" {
		t.Fatalf("Escape with words raised %v and left %q", raised, composer.state.editor.Text())
	}
	composer, _, _ = pressInComposer(t, both, "", key.Event{Name: key.NameReturn, Modifiers: key.ModShift, State: key.Press})
	if raised := kinds(composer); len(raised) != 0 {
		t.Fatalf("Shift+Enter raised %v", raised)
	}
	composer, _, _ = pressInComposer(t, ComposerOptions{Busy: true}, "", enter)
	if raised := kinds(composer); len(raised) != 0 {
		t.Fatalf("Enter with no request waiting raised %v", raised)
	}
}

func TestQuotingKeepsWhatIsAlreadyWrittenAndLeavesRoomToAnswer(t *testing.T) {
	var composer Composer
	composer.Quote("rashin api on\nprints the address")
	if text := composer.state.editor.Text(); text != "> rashin api on\n> prints the address\n\n" {
		t.Fatalf("a quotation went into an empty field as %q", text)
	}
	composer.SetText("About the port:")
	composer.Quote("  the port is 3600  ")
	if text := composer.state.editor.Text(); text != "About the port:\n\n> the port is 3600\n\n" {
		t.Fatalf("a quotation went under a draft as %q", text)
	}
}

// When a turn ends the plaque turns from Stop back to Send. With nothing
// typed it is disabled, and a disabled control used to drop the request for
// the next frame: the turn was over and the plaque still showed Stop until
// the pointer happened to move.
func TestTheSendPlaqueFinishesTurningBackFromStop(t *testing.T) {
	frame := newChatFrame(image.Pt(720, 280))
	composer := new(Composer)
	composer.Layout(frame.context(), frame.theme, ComposerOptions{Busy: true})
	frame.finish()
	frame.router.WakeupTime()

	composer.Layout(frame.context(), frame.theme, ComposerOptions{})
	frame.finish()
	if anim.Reduced() {
		t.Skip("motion is reduced here, so the plaque turns at once")
	}
	if _, wakes := frame.router.WakeupTime(); !wakes {
		t.Fatal("the plaque began to turn back and asked for no further frame")
	}
}
