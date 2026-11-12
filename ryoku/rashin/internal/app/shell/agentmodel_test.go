package shell

import (
	"image"
	"strings"
	"testing"

	"gioui.org/io/key"

	"ryoku-rashin/internal/protocol"
)

// startedOn is the agent session the board started with model, if any
func (s *stage) startedOn(harness, model string) (protocol.SessionInfo, bool) {
	for _, info := range s.runtime.store.Snapshot().Sessions {
		if strings.HasPrefix(info.ID, "agent-demo-") && info.Harness == harness && info.Model == model {
			return info, true
		}
	}
	return protocol.SessionInfo{}, false
}

// The model a session starts on is picked on the board before the first
// message: the pick goes out with session.new, each harness keeps its own,
// and the session opens on it.
func TestTheBoardPicksTheModelASessionStartsOn(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("agent")
	stage.choose("Model: Harness default (GPT-5.6 Sol)", "GPT-5.6 mini")
	stage.await("the pick is kept for its harness", func() bool { return stage.shell.preferences.AgentModels["codex"] == "gpt-5.6-mini" })
	if _, ok := stage.named("Model: GPT-5.6 mini"); !ok {
		t.Fatalf("the board does not show the pick; on screen: %v", stage.names())
	}
	// the facts under the composer name the model the session starts on, not the harness's own
	if _, ok := stage.named("GPT-5.6 mini"); !ok {
		t.Fatalf("the facts line does not name the pick; on screen: %v", stage.names())
	}
	if _, ok := stage.named("gpt-5.6-sol"); ok {
		t.Fatalf("the facts line still names the harness's own model; on screen: %v", stage.names())
	}
	if _, ok := stage.named("High"); !ok {
		t.Fatalf("the facts line lost the other facts; on screen: %v", stage.names())
	}

	stage.choose("Harness: Codex", "Claude Code")
	stage.await("the harness moves", func() bool { return stage.settings().Agent.Harness == "claude" })
	if _, ok := stage.named("Model: Harness default (Opus 5.5)"); !ok {
		t.Fatalf("another harness took the first one's pick; on screen: %v", stage.names())
	}
	stage.choose("Harness: Claude Code", "Codex")
	stage.await("the harness moves back", func() bool { return stage.settings().Agent.Harness == "codex" })
	if _, ok := stage.named("Model: GPT-5.6 mini"); !ok {
		t.Fatalf("the harness forgot its pick; on screen: %v", stage.names())
	}

	stage.typeText("fix the build")
	stage.press(key.NameReturn, 0)
	var info protocol.SessionInfo
	stage.await("session.new carries the pick", func() bool {
		var ok bool
		info, ok = stage.startedOn("codex", "gpt-5.6-mini")
		return ok
	})
	stage.await("the session opens on the pick", func() bool {
		_, ok := stage.named("Model: GPT-5.6 mini")
		return ok && stage.shell.router.Current().Session == info.ID
	})
}

// A harness that never ran has no list: Read the list starts it once and the
// board offers what it read.
func TestTheBoardReadsTheListOfAHarnessThatNeverRan(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("agent")
	stage.choose("Harness: Codex", "Review account")
	stage.await("the harness moves", func() bool { return stage.settings().Agent.Harness == "custom-review" })
	if stage.anyNamed("Model: ") != "" {
		t.Fatalf("a harness with no list shows a model select; on screen: %v", stage.names())
	}
	stage.click("Read the list")
	stage.await("the list is read", func() bool { _, ok := stage.named("Model: Harness default (GPT-5.6 Sol)"); return ok })
	stage.choose("Model: Harness default (GPT-5.6 Sol)", "GPT-5.6")
	stage.await("the pick is kept for its harness", func() bool { return stage.shell.preferences.AgentModels["custom-review"] == "gpt-5.6" })
}
