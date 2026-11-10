package chat

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"

	"ryoku-rashin/internal/protocol"
)

type conversation struct {
	frame *chatFrame
	model *Model
	view  View
	// when the pointer last did something, which is what tells a second press from a new one
	clock time.Duration
}

func newConversation(size image.Point) *conversation {
	return &conversation{frame: newChatFrame(size), model: NewModel(protocol.SessionInfo{})}
}

func (c *conversation) layout() {
	c.view.Layout(c.frame.context(), c.frame.theme, &c.frame.overlays, c.model)
	c.frame.finish()
}

func (c *conversation) fill(t *testing.T, rows int) {
	t.Helper()
	for sequence := int64(1); sequence <= int64(rows); sequence++ {
		c.model.Apply(testEvent(t, sequence, protocol.EvUser, protocol.UserData{Text: "A transcript row with enough text to occupy space."}))
	}
}

func (c *conversation) scrollUp() {
	at := f32.Pt(120, 80)
	c.frame.router.Queue(
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, PointerID: 1, Position: at},
		pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, PointerID: 1, Position: at, Scroll: f32.Pt(0, -120)},
	)
	c.layout()
}

func permissionRequest(id string) protocol.PermissionData {
	return protocol.PermissionData{
		Request: id, Title: "Edit the settings page", Input: "internal/app/pages/settings/page.go",
		Options: []protocol.PermissionOption{
			{ID: "once", Label: "Allow once", Kind: protocol.OptAllowOnce},
			{ID: "always", Label: "Always allow", Kind: protocol.OptAllowAlways},
			{ID: "deny", Label: "Deny", Kind: protocol.OptRejectOnce},
		},
	}
}

func assertPermissionAction(t *testing.T, actions []Action, request, option string) {
	t.Helper()
	if len(actions) != 1 || actions[0].Kind != ActPermission || actions[0].Request != request || actions[0].Option != option {
		t.Fatalf("permission actions = %#v, want %s answered %q", actions, request, option)
	}
}

func TestLongTranscriptOpensAtItsEnd(t *testing.T) {
	c := newConversation(image.Pt(480, 180))
	c.fill(t, 24)
	c.layout()
	position := c.view.state.scroller.Position
	if position.BeforeEnd || position.First == 0 {
		t.Fatalf("a transcript longer than its room opened at %#v, want its end", position)
	}
}

func TestShortTranscriptStandsAtTheTop(t *testing.T) {
	c := newConversation(image.Pt(480, 600))
	c.fill(t, 2)
	c.layout()
	c.layout()
	position := c.view.state.scroller.Position
	if position.First != 0 || position.Offset != 0 || c.view.Scrolled() {
		t.Fatalf("a transcript shorter than its room moved to %#v", position)
	}
}

// The room a transcript has can be small for a moment, as while the composer
// travels down the page when a question opens. A transcript that then fits
// used to hang from the end of its room with a gap above it.
func TestATranscriptThatFitsAgainGoesBackToTheTop(t *testing.T) {
	c := newConversation(image.Pt(480, 120))
	c.fill(t, 3)
	c.layout()
	c.layout()
	c.frame.size = image.Pt(480, 900)
	c.layout()
	c.layout()
	top := c.frame.size.Y
	for _, node := range c.frame.router.AppendSemantics(nil) {
		if node.Desc.Label == "A transcript row with enough text to occupy space." {
			top = min(top, node.Desc.Bounds.Min.Y)
		}
	}
	if top > 60 {
		t.Fatalf("with room for all of it the transcript starts %d down the page", top)
	}
}

func TestScrolledUpTranscriptStaysPutUntilAskedBack(t *testing.T) {
	c := newConversation(image.Pt(480, 180))
	c.fill(t, 24)
	c.layout()
	c.scrollUp()
	if c.view.state.follow {
		t.Fatal("scrolling up did not stop the transcript following its end")
	}
	before := c.view.state.scroller.Position
	c.model.Apply(testEvent(t, 25, protocol.EvText, protocol.TextData{Message: "latest", Delta: "Newest answer"}))
	c.layout()
	if after := c.view.state.scroller.Position; after.First != before.First || after.Offset != before.Offset {
		t.Fatalf("a new event moved the reader from %#v to %#v", before, after)
	}

	c.view.state.jump.Clickable.Click()
	c.layout()
	c.layout()
	if !c.view.state.follow || c.view.state.scroller.Position.BeforeEnd {
		t.Fatalf("going to the end did not restore following: %#v", c.view.state.scroller.Position)
	}
}

func TestPermissionOptionPressAnswersTheRequest(t *testing.T) {
	c := newConversation(image.Pt(800, 420))
	c.model.Apply(testEvent(t, 1, protocol.EvPermission, permissionRequest("request-1")))
	c.layout()
	option := c.view.state.plaques["permission:request-1:always"]
	if option == nil {
		t.Fatal("an open request offered no way to answer it")
	}
	option.Clickable.Click()
	c.layout()
	assertPermissionAction(t, c.view.Actions(), "request-1", "always")
}

// A key can consent for the reader, so it only counts for a request that is
// on screen and has been long enough to read.
func TestAKeyAnswersARequestOnlyOnceItCouldBeRead(t *testing.T) {
	c := newConversation(image.Pt(760, 420))
	if keys := c.view.Asking(c.model); keys != (AnswerKeys{}) {
		t.Fatalf("a transcript with no request takes answer keys: %+v", keys)
	}
	c.model.Apply(testEvent(t, 1, protocol.EvPermission, permissionRequest("request-keys")))
	if keys := c.view.Asking(c.model); keys != (AnswerKeys{}) {
		t.Fatalf("a request that was never drawn takes answer keys: %+v", keys)
	}
	c.layout()
	if keys := c.view.Asking(c.model); !keys.Allow || !keys.Refuse {
		t.Fatalf("a request on screen takes %+v", keys)
	}
	if answer, ok := c.view.Answer(c.model, ActAllow, c.frame.now); ok {
		t.Fatalf("a key pressed as the request appeared answered it: %#v", answer)
	}
	c.frame.now = c.frame.now.Add(answerAfter)
	c.layout()
	allow, _ := c.view.Answer(c.model, ActAllow, c.frame.now)
	assertPermissionAction(t, []Action{allow}, "request-keys", "once")
	refuse, _ := c.view.Answer(c.model, ActRefuse, c.frame.now)
	assertPermissionAction(t, []Action{refuse}, "request-keys", "deny")
}

func TestAnsweredPermissionStaysAsARecord(t *testing.T) {
	c := newConversation(image.Pt(760, 420))
	c.model.Apply(testEvent(t, 1, protocol.EvPermission, permissionRequest("request-fold")))
	c.layout()
	c.model.Apply(testEvent(t, 2, protocol.EvPermissionResolved, protocol.PermissionResolvedData{
		Request: "request-fold", Option: "once", Outcome: "selected", By: "user",
	}))
	c.frame.now = c.frame.now.Add(answerAfter)
	c.layout()
	visible := c.view.state.visible
	if len(visible) != 1 || c.model.state.rows[visible[0]].kind != rowPermission {
		t.Fatalf("an answered request left the transcript: visible rows %v", visible)
	}
	if answer, ok := c.view.Answer(c.model, ActAllow, c.frame.now); ok {
		t.Fatalf("a key answered a request that was already answered: %#v", answer)
	}
}

// Asking again without what was attached would ask about nothing.
func TestAskingAgainTakesWhatWasAttached(t *testing.T) {
	c := newConversation(image.Pt(760, 420))
	asked := protocol.UserData{Text: "What is this?", Images: []protocol.Attachment{{Path: "/home/reader/shot.png"}, {Path: "/home/reader/build.log"}}}
	c.model.Apply(testEvent(t, 1, protocol.EvUser, asked))
	c.model.Apply(testEvent(t, 2, protocol.EvText, protocol.TextData{Message: "m", Delta: "A terminal."}))
	again := retryAction(c.model.state, len(c.model.state.rows)-1, 1)
	if again.Kind != ActRetry || again.Value != asked.Text || len(again.Attached) != 2 || again.Attached[1] != "/home/reader/build.log" {
		t.Fatalf("asking again raised %#v", again)
	}
}

func TestStepsFoldToOneLineWhenTheLastFinishes(t *testing.T) {
	c := newConversation(image.Pt(760, 500))
	step := func(sequence int64, call, status string) {
		c.model.Apply(testEvent(t, sequence, protocol.EvTool, protocol.ToolData{Call: call, Title: call, Kind: protocol.ToolRead, Status: status}))
	}
	step(1, "read", protocol.ToolDone)
	step(2, "edit", protocol.ToolRunning)
	c.layout()
	if group := c.view.state.work["tool:read:group"]; group != nil {
		t.Fatal("steps were folded while one of them was still running")
	}
	step(3, "edit", protocol.ToolDone)
	c.layout()
	group := c.view.state.work["tool:read:group"]
	if group == nil || group.open {
		t.Fatalf("finished steps did not fold to one line: %#v", group)
	}
	group.click.Clickable.Click()
	c.layout()
	if !group.open {
		t.Fatal("the folded steps did not unfold when pressed")
	}
}

func TestTailMarkNeverRepeatsWhatARowAlreadyShows(t *testing.T) {
	model := NewModel(protocol.SessionInfo{})
	expect := func(when string, shown bool, label string) {
		t.Helper()
		if gotShown, gotLabel := tailMark(model.state); gotShown != shown || gotLabel != label {
			t.Fatalf("%s: tail mark = %v %q, want %v %q", when, gotShown, gotLabel, shown, label)
		}
	}
	model.Apply(testEvent(t, 1, protocol.EvUser, protocol.UserData{Text: "Why is my disk full?"}))
	model.Apply(testEvent(t, 2, protocol.EvTurnStart, protocol.TurnStartData{Turn: 1}))
	expect("after the question", true, "Thinking")
	model.Apply(testEvent(t, 3, protocol.EvThought, protocol.TextData{Message: "thought", Delta: "Reading the disk."}))
	expect("while a thought arrives", false, "")
	model.Apply(testEvent(t, 4, protocol.EvTool, protocol.ToolData{Call: "df", Title: "Read disk usage", Status: protocol.ToolRunning}))
	expect("while a step runs", false, "")
	model.Apply(testEvent(t, 5, protocol.EvTool, protocol.ToolData{Call: "df", Status: protocol.ToolDone}))
	expect("between steps", true, "Thinking")
	model.Apply(testEvent(t, 6, protocol.EvText, protocol.TextData{Message: "answer", Delta: "Your disk is"}))
	expect("while words arrive", true, "")
}

func TestSettledQuestionAsksForNoFrame(t *testing.T) {
	previous := anim.Reduced()
	anim.SetReduce(false)
	defer anim.SetReduce(previous)

	c := newConversation(image.Pt(760, 420))
	c.layout()
	c.model.Apply(testEvent(t, 1, protocol.EvUser, protocol.UserData{Text: "How does following work?"}))
	c.layout()
	// the question rises in, which asks for frames, then it has to come to rest
	if _, moving := c.frame.router.WakeupTime(); !moving {
		t.Fatal("a question that just arrived did not move")
	}
	c.frame.now = c.frame.now.Add(time.Second)
	c.layout()
	if wake, ok := c.frame.router.WakeupTime(); ok {
		t.Fatalf("a transcript at rest asked for a frame at %v", wake)
	}
}
