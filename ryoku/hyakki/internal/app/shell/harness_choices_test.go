package shell

import (
	"context"
	"image"
	"strings"
	"testing"

	"gioui.org/io/key"
	"gioui.org/io/semantic"

	"ryoku-rashin/internal/protocol"
)

// a new session has its engine and options before any prompt is sent
func (s *stage) freshSession() protocol.SessionInfo {
	s.t.Helper()
	var info protocol.SessionInfo
	params := protocol.SessionNewParams{Kind: protocol.KindAgent, Model: "rashin-models/lfm3-omni-2b", Cwd: "/home/nero/Work/hyperion"}
	if err := s.runtime.client.Call(context.Background(), protocol.MSessionNew, params, &info); err != nil {
		s.t.Fatal(err)
	}
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
func (s *stage) pickerButton(name string) bool {
	for _, node := range s.nodes() {
		if node.Desc.Class == semantic.Button && node.Desc.Label == name {
			return true
		}
	}
	return false
}

func TestOpeningASessionShowsItsEngineWithoutStartingIt(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	info := stage.freshSession()
	stage.open("agent/" + info.ID)
	stage.await("the model list loads", func() bool { return stage.anyNamed("Model: LFM3 Omni 2B") != "" })
	if stage.sessionInfo(info.ID).Alive {
		t.Fatal("opening a session started the engine without a prompt")
	}
	if !stage.typedArrives("fix the build") {
		t.Error("the composer lost the keyboard while showing the model")
	}
}

func TestAModelIsFoundByTypingItsName(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	info := stage.freshSession()
	stage.open("agent/" + info.ID)
	stage.await("the model list loads", func() bool { return stage.anyNamed("Model: LFM3 Omni 2B") != "" })
	stage.click("Model: LFM3 Omni 2B")
	if !stage.pickerButton("LFM3 Omni 2B") {
		t.Fatalf("the model list did not open; on screen: %v", stage.names())
	}
	stage.typeText("qwen3-coder")
	if stage.pickerButton("LFM3 Omni 2B") {
		t.Fatal("typing did not narrow the list")
	}
	if stage.anyNamed("Qwen3 Coder") == "" {
		t.Fatalf("the searched model is not listed; on screen: %v", stage.names())
	}
	stage.press(key.NameReturn, 0)
	stage.await("the model is set", func() bool { return stage.sessionInfo(info.ID).Model == "openrouter/qwen/qwen3-coder:free" })
	if stage.anyNamed("Model: Qwen3 Coder") == "" {
		t.Fatalf("the composer does not show the model picked; on screen: %v", stage.names())
	}
}

func TestANewSessionOptionsAreVisibleInTheComposer(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("agent/agent-done")
	if stage.anyNamed("Asking: ") == "" {
		t.Fatalf("the composer did not show approval mode; on screen: %v", stage.names())
	}
}

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

func TestReadingASessionDoesNotStartItsEngine(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	info := stage.freshSession()
	stage.open("agent/" + info.ID)
	stage.open("agent")
	stage.open("agent/" + info.ID)
	if stage.sessionInfo(info.ID).Alive {
		t.Fatal("revisiting a session started the engine without a prompt")
	}
}
