package shell

import (
	"image"
	"testing"

	"gioui.org/io/key"
)

// the demo agent that waits for leave to edit the settings page
const waitingAgent = "agent/agent-20261110-101500-b82d"

func (s *stage) asking() bool {
	_, open := s.named("Allow once")
	return open
}

// waiting opens the session and lets the request stand long enough to be read
func waiting(t *testing.T) *stage {
	t.Helper()
	stage := newStage(t, image.Pt(1440, 900))
	stage.open(waitingAgent)
	if !stage.asking() {
		t.Fatalf("the demo agent is not waiting for an answer; on screen: %v", stage.names())
	}
	for range 40 {
		stage.frame()
	}
	return stage
}

func (s *stage) answered(how string) {
	s.t.Helper()
	s.await("the request is answered", func() bool { return !s.asking() })
	if _, recorded := s.named(how + ": Edit settings page"); !recorded {
		s.t.Fatalf("the answer is not recorded as %q; on screen: %v", how, s.names())
	}
}

// The composer holds the keyboard while an agent waits, so a key that
// answers the agent has to come through it. Words in it are a message.
func TestEnterAllowsOnceFromAnEmptyComposerAndSendsWordsFromAFullOne(t *testing.T) {
	stage := waiting(t)
	if _, told := stage.named("Enter allows once, Esc refuses"); !told {
		t.Fatalf("the composer does not say which keys answer; on screen: %v", stage.names())
	}
	stage.typeText("use the other file")
	stage.press(key.NameReturn, 0)
	stage.await("the words leave the composer", func() bool { return !stage.sendReady() })
	if !stage.asking() {
		t.Fatal("Enter in a composer with words in it answered the request")
	}
	stage.press(key.NameReturn, 0)
	stage.answered("Allowed once")
}

// Escape used to refuse the request whatever else it was pressed for.
func TestEscapeRefusesOnlyWhenNothingElseIsItsBusiness(t *testing.T) {
	stage := waiting(t)
	stage.typeText("/")
	stage.press(key.NameEscape, 0)
	if !stage.asking() {
		t.Fatal("Escape pressed over typed text refused the request")
	}
	stage.typeOver(1, "")

	line, ok := stage.named("The frame now keeps navigation, search and the reading column in one stable composition.")
	if !ok {
		t.Fatalf("the agent's answer is not on screen: %v", stage.names())
	}
	middle := (line.Desc.Bounds.Min.Y + line.Desc.Bounds.Max.Y) / 2
	stage.drag(image.Pt(line.Desc.Bounds.Min.X+4, middle), image.Pt(line.Desc.Bounds.Min.X+160, middle))
	stage.press(key.NameEscape, 0)
	if !stage.asking() {
		t.Fatal("Escape pressed to let go of marked text refused the request")
	}

	stage.press(key.NameEscape, 0)
	stage.answered("Denied")
}
