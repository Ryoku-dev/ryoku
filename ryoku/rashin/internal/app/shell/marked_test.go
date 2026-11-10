package shell

import (
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
)

// the demo question whose answer has a paragraph with strong text in it
const styledAsk = "ask/ask-20261110-094200-a17c"

const styledParagraph = "The semantic index is rebuilding after the palette update. The cache remains safe while this runs."

// across is a drag over the first words of the text that reads as given
func (s *stage) across(text string) {
	s.t.Helper()
	node, ok := s.named(text)
	if !ok {
		s.t.Fatalf("no text on screen reads %q; there is: %v", text, s.names())
	}
	bounds := node.Desc.Bounds
	top := bounds.Min.Y + 10
	s.drag(image.Pt(bounds.Min.X+3, top), image.Pt(bounds.Min.X+170, top))
}

func TestWordsInAStyledParagraphCanBeMarkedAndCopied(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 2400))
	stage.open(styledAsk)
	stage.across(styledParagraph)
	stage.press("C", key.ModShortcut)
	copied := stage.clipboard()
	if len(copied) < 8 || !strings.HasPrefix(styledParagraph, copied) {
		t.Fatalf("a drag over the paragraph and Ctrl+C copied %q", copied)
	}
}

// Marked text holds the keyboard so Ctrl+C reaches it. Nothing else in the
// transcript may: a press on an answer is not a reason to click the composer
// again before typing.
func TestTheTranscriptKeepsTheKeyboardOnlyWhileTextIsMarked(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 2400))
	stage.open(styledAsk)
	for _, text := range []string{styledParagraph, "CPU use will settle when the project index finishes."} {
		node, ok := stage.named(text)
		if !ok {
			t.Fatalf("no text on screen reads %q; there is: %v", text, stage.names())
		}
		stage.clickAt(image.Pt(node.Desc.Bounds.Min.X+40, node.Desc.Bounds.Min.Y+10))
		if !stage.typedArrives("and then") {
			t.Fatalf("after a press on %q typing did not reach the composer", text)
		}
		stage.typeOver(8, "")
	}

	stage.across(styledParagraph)
	if stage.typedArrives("lost") {
		t.Fatal("typing reached the composer while text was marked for copying")
	}
	stage.press(key.NameEscape, 0)
	if !stage.typedArrives("and then") {
		t.Fatal("after Escape let go of the marked text, typing did not reach the composer")
	}
}

// The words marked in an answer are usually what the next message is about.
func TestMarkedWordsCanBeQuotedInTheReply(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 2400))
	stage.open(styledAsk)
	stage.across(styledParagraph)
	node, _ := stage.named(styledParagraph)
	at := f32.Pt(float32(node.Desc.Bounds.Min.X+40), float32(node.Desc.Bounds.Min.Y+10))
	stage.now = stage.now.Add(time.Second)
	stage.queue(
		pointer.Event{Kind: pointer.Press, Buttons: pointer.ButtonSecondary, Position: at},
		pointer.Event{Kind: pointer.Release, Position: at},
	)
	stage.settle()
	stage.click("Quote in the reply")
	if !stage.sendReady() {
		t.Fatalf("after quoting, the composer holds nothing to send; on screen: %v", stage.names())
	}
	if !stage.typedArrives("what does this mean?") {
		t.Fatal("after quoting, typing did not reach the composer")
	}
}
