package chat

import (
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"

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
