package shell

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"

	"ryoku-rashin/internal/protocol"
)

const (
	choiceTask     = "work/task-colour-piece"
	choiceApproval = "approval-angle"
	choiceQuestion = "Which angle should the piece take?"
	choiceB        = "How the compositors got there"
)

// the approval as the demo daemon now holds it
func (s *stage) approval(id string) protocol.Approval {
	for _, approval := range s.runtime.store.Snapshot().Approvals {
		if approval.ID == id {
			return approval
		}
	}
	return protocol.Approval{}
}

// on a task page the waiting choice takes its letter without a click first
func TestATaskPagesWaitingChoiceTakesItsLetter(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open(choiceTask)
	if _, ok := stage.named(choiceQuestion); !ok {
		t.Fatalf("the task page shows no choice card; on screen: %v", stage.names())
	}
	stage.press("B", 0)
	stage.await("the daemon hears the choice", func() bool {
		approval := stage.approval(choiceApproval)
		return approval.Decision == protocol.DecideChoose && approval.Choice == choiceB
	})
	if _, ok := stage.named("Chose B · " + choiceB); !ok {
		t.Fatalf("the card did not fold to the choice; on screen: %v", stage.names())
	}
}

// a letter typed into the card's note is the note's, never an answer, even when it
// comes right after the press that gave the note the keyboard, before another frame
func TestAFocusedNoteKeepsTheLetters(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open(choiceTask)
	note, ok := stage.named("A note for the team (optional)")
	if !ok {
		t.Fatalf("the choice card has no note; on screen: %v", stage.names())
	}
	at := f32.Pt(float32(note.Desc.Bounds.Min.X+12), float32(note.Desc.Bounds.Min.Y+12))
	stage.now = stage.now.Add(300 * time.Millisecond)
	stage.queue(
		pointer.Event{Kind: pointer.Move, Position: at},
		pointer.Event{Kind: pointer.Press, Buttons: pointer.ButtonPrimary, Position: at},
		pointer.Event{Kind: pointer.Release, Position: at},
	)
	stage.frame()
	stage.router.Queue(key.Event{Name: "B", State: key.Press}, key.EditEvent{Text: "b"})
	stage.settle()
	if approval := stage.approval(choiceApproval); approval.Status != protocol.ApprovalPending {
		t.Fatalf("typing into the note answered the choice: %+v", approval)
	}
	if _, ok := stage.named(choiceQuestion); !ok {
		t.Fatalf("the choice card folded while its note was typed in; on screen: %v", stage.names())
	}
}

// on the Inbox the keys go to the card that holds them, so a letter alone answers nothing
func TestTheInboxAnswersOnlyTheFocusedCard(t *testing.T) {
	// tall enough that the choice card stands in the board without scrolling
	stage := newStage(t, image.Pt(1440, 3000))
	stage.open("inbox")
	var title image.Rectangle
	for _, node := range stage.nodes() {
		// the sidebar names the task's question too; the card is the one in the board
		if nodeName(node) == choiceQuestion && node.Desc.Bounds.Min.X > title.Min.X {
			title = node.Desc.Bounds
		}
	}
	if title.Empty() {
		t.Fatalf("the Inbox shows no choice card; on screen: %v", stage.names())
	}
	stage.press("B", 0)
	stage.settle()
	if approval := stage.approval(choiceApproval); approval.Status != protocol.ApprovalPending {
		t.Fatalf("a letter with no card focused answered the choice: %+v", approval)
	}
	stage.clickAt(title.Min.Add(image.Pt(12, 12)))
	stage.press("2", key.Modifiers(0))
	stage.await("the focused card takes its number", func() bool {
		return stage.approval(choiceApproval).Choice == choiceB
	})
}
