package shell

import (
	"image"
	"testing"

	"gioui.org/io/key"
	"gioui.org/io/semantic"

	"ryoku-rashin/internal/app/desktop"
)

// sendReady says whether the composer on screen holds something to send: its
// Send plaque stands down while the text is empty, and while a turn runs the
// plaque is Stop until there are words to queue.
func (s *stage) sendReady() bool {
	if node, ok := s.named("Send"); ok {
		return !node.Desc.Disabled
	}
	_, queues := s.named("Queue")
	return queues
}

// typedArrives types and reports whether the words reached the composer
func (s *stage) typedArrives(text string) bool {
	s.t.Helper()
	s.typeText(text)
	return s.sendReady()
}

// composerText is the composer's own text area, the lowest editor on the page
func (s *stage) composerText() image.Rectangle {
	s.t.Helper()
	var found image.Rectangle
	for _, node := range s.nodes() {
		if node.Desc.Class == semantic.Editor && node.Desc.Bounds.Min.Y > found.Min.Y {
			found = node.Desc.Bounds
		}
	}
	if found.Empty() {
		s.t.Fatal("no text field is on screen")
	}
	return found
}

// withAndWithoutMotion runs a test on the app at rest and on the app with its
// motion on. A page that fades in, a menu that fades out: what happens over
// several frames loses the keyboard in ways one frame never shows.
func withAndWithoutMotion(t *testing.T, test func(t *testing.T, open func() *stage)) {
	size := image.Pt(1440, 900)
	t.Run("still", func(t *testing.T) { test(t, func() *stage { return newStage(t, size) }) })
	t.Run("moving", func(t *testing.T) { test(t, func() *stage { return newMovingStage(t, size) }) })
}

func TestEveryComposerTakesTypingWhenItsPageOpens(t *testing.T) {
	withAndWithoutMotion(t, func(t *testing.T, open func() *stage) {
		stage := open()
		for _, route := range []string{"ask/new", "ask/ask-20261110-094200-a17c", "agent/new", "agent/agent-done"} {
			stage.open(route)
			if !stage.typedArrives("what changed") {
				t.Errorf("%s: typing after the page opened did not reach its composer", route)
			}
		}
	})
}

// With motion on, a page that arrives is faded in over several frames. What it
// asks for in the first of them, the keyboard, must still be granted.
func TestAPageReachedByShortcutTakesTypingWhileItArrives(t *testing.T) {
	stage := newMovingStage(t, image.Pt(1440, 900))
	stage.open("ask/new")
	for name, destination := range map[string]key.Name{"Ask": "1", "Agents": "2"} {
		stage.press(",", key.ModShortcut)
		stage.press(destination, key.ModShortcut)
		if !stage.typedArrives("what changed") {
			t.Errorf("%s: typing after Ctrl %s went nowhere", name, destination)
		}
	}
}

func TestPressingTheComposerPutsTheKeyboardInIt(t *testing.T) {
	for _, route := range []string{"ask/new", "agent/new", "agent/agent-done"} {
		stage := newStage(t, image.Pt(1440, 900))
		stage.open(route)
		// somewhere that takes the keyboard away first
		stage.click("Fold the sidebar  Ctrl B")
		stage.clickAt(centre(stage.composerText()))
		if !stage.typedArrives("what changed") {
			t.Errorf("%s: pressing the composer and typing did not reach it", route)
		}
	}
}

func TestChoosingInTheComposerRowLeavesTheKeyboardInTheText(t *testing.T) {
	withAndWithoutMotion(t, func(t *testing.T, open func() *stage) {
		for route, picks := range map[string][2]string{
			"ask/new":          {"Model: Auto", "Smart"},
			"agent/new":        {"Harness: Codex", "Claude Code"},
			"agent/agent-done": {"Mode: Default", "Plan"},
		} {
			stage := open()
			stage.open(route)
			stage.click(picks[0])
			stage.click(picks[1])
			if !stage.typedArrives("what changed") {
				t.Errorf("%s: after choosing %q the typing went nowhere", route, picks[1])
			}
		}
	})
}

func TestTheSearchFieldGivesTheKeyboardBack(t *testing.T) {
	withAndWithoutMotion(t, func(t *testing.T, open func() *stage) {
		stage := open()
		stage.open("ask/new")
		stage.press("K", key.ModShortcut)
		stage.typeText("zoom")
		if stage.sendReady() {
			t.Fatal("words typed into the search field went to the composer")
		}
		stage.press(key.NameEscape, 0)
		if !stage.typedArrives("what changed") {
			t.Fatal("after Escape closed the search field the typing went nowhere")
		}
	})
}

func TestPressingTheComposerWhileSearchIsOpenKeepsTheKeyboardThere(t *testing.T) {
	withAndWithoutMotion(t, func(t *testing.T, open func() *stage) {
		stage := open()
		stage.open("ask/new")
		stage.press("K", key.ModShortcut)
		stage.clickAt(centre(stage.composerText()))
		if !stage.typedArrives("what changed") {
			t.Fatal("pressing the composer closed the search field and left the keyboard nowhere")
		}
	})
}

func TestAClosedDialogGivesTheKeyboardBack(t *testing.T) {
	withAndWithoutMotion(t, func(t *testing.T, open func() *stage) {
		stage := open()
		stage.open("agent/new")
		stage.click("More options")
		stage.click("Cancel")
		if !stage.typedArrives("what changed") {
			t.Fatal("after the dialog closed the typing went nowhere")
		}
	})
}

func TestTheHarnessChosenForASessionIsTheOneSettingsShows(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("agent/new")
	stage.click("Harness: Codex")
	stage.click("Claude Code")
	stage.await("the choice becomes the setting", func() bool {
		return stage.runtime.store.Snapshot().Settings.Agent.Harness == "claude"
	})
	stage.open("settings/agents")
	if _, shown := stage.named("Claude Code"); !shown {
		t.Fatalf("Settings does not show the harness that was chosen; on screen: %v", stage.names())
	}
}

func TestAttachAsksTheSystemForFilesAndFallsBackToAField(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("ask/new")
	stage.chooser = func(request desktop.ChooseRequest) []string {
		if request.Folder || !request.Multiple {
			t.Errorf("attach asked for %+v, want several files", request)
		}
		return []string{"/tmp/notes.txt"}
	}
	stage.click("Attach a file")
	if _, attached := stage.named("notes.txt"); !attached {
		t.Fatalf("the chosen file is not attached; on screen: %v", stage.names())
	}

	stage.chooser = nil
	editors := 0
	for _, node := range stage.nodes() {
		if node.Desc.Class == semantic.Editor {
			editors++
		}
	}
	stage.click("Attach a file")
	after := 0
	for _, node := range stage.nodes() {
		if node.Desc.Class == semantic.Editor {
			after++
		}
	}
	if after <= editors {
		t.Fatalf("with no chooser on the machine, attach showed no field to type a path into (%d editors, then %d)", editors, after)
	}
}

func TestAddProjectAsksTheSystemForAFolder(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("projects")
	asked := desktop.ChooseRequest{}
	stage.chooser = func(request desktop.ChooseRequest) []string {
		asked = request
		return []string{"/tmp/fresh-project"}
	}
	stage.click("Add project")
	if !asked.Folder {
		t.Fatalf("Add project asked the chooser for %+v, want a folder", asked)
	}
	stage.await("the chosen folder becomes a project", func() bool {
		for _, project := range stage.runtime.store.Snapshot().Projects {
			if project.Root == "/tmp/fresh-project" {
				return true
			}
		}
		return false
	})
}
