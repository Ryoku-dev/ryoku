package shell

import (
	"context"
	"image"
	"strings"
	"testing"

	"gioui.org/io/key"

	"ryoku-rashin/internal/protocol"
)

// freshSession is an agent session nothing was ever asked in: its harness
// never ran, so nothing is known yet about what it will use.
func (s *stage) freshSession() protocol.SessionInfo {
	s.t.Helper()
	var info protocol.SessionInfo
	params := protocol.SessionNewParams{Kind: protocol.KindAgent, Harness: "codex", Cwd: "/home/nero/Work/hyperion"}
	if err := s.runtime.client.Call(context.Background(), protocol.MSessionNew, params, &info); err != nil {
		s.t.Fatal(err)
	}
	// the list hears of a session when it is attached, and a place it has not heard of opens as the board
	if err := s.runtime.client.Attach(context.Background(), info.ID); err != nil {
		s.t.Fatal(err)
	}
	return info
}

func (s *stage) anyNamed(prefix string) string {
	for _, name := range s.names() {
		if strings.HasPrefix(name, prefix) {
			return name
		}
	}
	return ""
}

// A session shows the model, mode and options its harness will really use as
// soon as it is opened, not a row of "Default" until the first prompt: opening
// it brings the harness up to ask.
func TestOpeningASessionShowsWhatItsHarnessWillUse(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	info := stage.freshSession()
	stage.open("agent/" + info.ID)
	stage.await("the harness says what it runs", func() bool { return stage.anyNamed("Model: ") != "" })
	for _, want := range []string{"Model: Opus", "Mode: Default", "Thinking: Auto"} {
		if _, ok := stage.named(want); !ok {
			t.Errorf("the composer does not show %q; on screen: %v", want, stage.names())
		}
	}
	if !stage.typedArrives("fix the build") {
		t.Error("the composer lost the keyboard while its harness came up")
	}
}

// A harness offers hundreds of models. The list is searched by typing, the
// arrow keys and Enter pick from it, and the keyboard goes back to the prompt.
func TestAModelIsFoundByTypingItsName(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	info := stage.freshSession()
	stage.open("agent/" + info.ID)
	stage.await("the harness says what it runs", func() bool { return stage.anyNamed("Model: ") != "" })

	stage.click("Model: Opus")
	if _, ok := stage.named("Sonnet"); !ok {
		t.Fatalf("the model list did not open; on screen: %v", stage.names())
	}
	stage.typeText("openai lu")
	if _, ok := stage.named("Sonnet"); ok {
		t.Fatal("typing did not narrow the list")
	}
	if _, ok := stage.named("Luna"); !ok {
		t.Fatalf("the model typed for is not listed; on screen: %v", stage.names())
	}
	stage.press(key.NameReturn, 0)
	stage.await("the model is set", func() bool { return stage.sessionInfo(info.ID).Model == "openai/luna" })
	if _, ok := stage.named("Model: Luna"); !ok {
		t.Fatalf("the composer does not show the model picked; on screen: %v", stage.names())
	}
	if !stage.typedArrives("fix the build") {
		t.Error("picking a model did not hand the keyboard back to the composer")
	}
}

// Whatever else the harness offers is a select of its own, set through the
// harness, which says the new value back.
func TestAHarnessOptionIsSetFromTheComposer(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("agent/agent-done")
	stage.choose("Thinking: Auto", "High")
	stage.await("the option moves", func() bool { _, ok := stage.named("Thinking: High"); return ok })
}

// A list too short to search is still walked with the arrow keys.
func TestAShortListIsWalkedWithTheKeyboard(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("ask/new")
	stage.click("Asking: Read only")
	stage.press(key.NameDownArrow, 0)
	stage.press(key.NameReturn, 0)
	stage.await("the next choice is taken", func() bool { return stage.settings().Ask.Approvals == protocol.ApprovalsAsk })
	if _, ok := stage.named("Asking: Ask first"); !ok {
		t.Fatalf("the composer does not show the choice; on screen: %v", stage.names())
	}
}

// Looking at a session must not leave its harness running: one brought up
// only to say what it uses is stopped again when the session is left with
// nothing asked of it.
func TestAHarnessBroughtUpToBeLookedAtIsStoppedOnLeaving(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	info := stage.freshSession()
	stage.open("agent/" + info.ID)
	stage.await("the harness comes up", func() bool { return stage.sessionInfo(info.ID).Alive })
	stage.open("agent")
	stage.await("the harness is stopped", func() bool { return !stage.sessionInfo(info.ID).Alive })

	// what it said stays known, so coming back shows it without starting anything
	stage.open("agent/" + info.ID)
	if _, ok := stage.named("Model: Opus"); !ok {
		t.Fatalf("the session forgot what its harness uses; on screen: %v", stage.names())
	}
	if stage.sessionInfo(info.ID).Alive {
		t.Fatal("coming back to a session started its harness again though nothing new could be learned")
	}
}
