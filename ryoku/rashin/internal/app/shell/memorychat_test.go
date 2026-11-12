package shell

import (
	"context"
	"fmt"
	"image"
	"strings"
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/semantic"

	"ryoku-rashin/internal/protocol"
)

const memoryAsk = "ask-20261110-094200-a17c"

// The Memory updated line opens what the change did, and Undo there is the
// daemon's undo: the change comes back undone and the line says so.
func TestMemoryUpdatedLineUndoes(t *testing.T) {
	stage := newStage(t, image.Pt(1612, 2600))
	stage.open("ask/" + memoryAsk)
	stage.click("Memory updated")
	if _, ok := stage.named("Rashin remembered"); !ok {
		t.Fatalf("the line opened no popover; on screen: %v", stage.names())
	}
	stage.click("Undo")
	stage.await("the daemon undoes the change", func() bool {
		for _, change := range stage.runtime.store.Snapshot().MemoryChanges {
			if change.Session == memoryAsk && change.Undone {
				return true
			}
		}
		return false
	})
	stage.await("the popover says it was undone", func() bool { _, ok := stage.named("Undone"); return ok })
	if _, ok := stage.named("Redo"); !ok {
		t.Fatalf("an undone change offers no Redo; on screen: %v", stage.names())
	}
	stage.press(key.NameEscape, 0)
	stage.settle()
	if _, ok := stage.named("Memory unchanged"); !ok {
		t.Fatalf("the line does not read undone; on screen: %v", stage.names())
	}
}

// What an answer drew on is listed, and Not useful here tells the daemon to
// leave that memory out of this conversation.
func TestUsedMemoriesCanBeLeftOutHere(t *testing.T) {
	stage := newStage(t, image.Pt(1612, 2600))
	stage.open("ask/" + memoryAsk)
	stage.click("Used 3 memories")
	if _, ok := stage.named("This answer used"); !ok {
		t.Fatalf("the line opened no popover; on screen: %v", stage.names())
	}
	if _, ok := stage.named("Rashin also had the card about you."); !ok {
		t.Fatalf("the popover does not say the card was in the prompt; on screen: %v", stage.names())
	}
	stage.click("Not useful here")
	// the placard is said only once the daemon answered memory.suppress
	stage.await("the daemon took it", func() bool { _, ok := stage.named("Rashin won't use this here again."); return ok })
	if _, ok := stage.named("Not used here again"); !ok {
		t.Fatalf("the row still offers to leave it out; on screen: %v", stage.names())
	}
}

// A temporary conversation is marked, locks its memory choice once it has
// started, and is gone from the list when it is left.
func TestATemporaryAskEndsWhenLeft(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("ask/new")
	stage.press("N", key.ModShortcut|key.ModShift)
	if _, ok := stage.named("Temporary"); !ok {
		t.Fatalf("Ctrl Shift N did not make the conversation temporary; on screen: %v", stage.names())
	}
	if _, ok := stage.named("Nothing here is saved or remembered."); !ok {
		t.Fatalf("the new conversation does not say it is not saved; on screen: %v", stage.names())
	}
	if _, ok := stage.named("Memory: Use only"); !ok {
		t.Fatalf("a temporary conversation does not start from use only; on screen: %v", stage.names())
	}
	if !stage.typedArrives("what is using my disk") {
		t.Fatal("the composer lost the keyboard")
	}
	known := make(map[string]bool)
	for _, info := range stage.runtime.store.Snapshot().Sessions {
		known[info.ID] = true
	}
	var started protocol.SessionInfo
	stage.press(key.NameReturn, 0)
	stage.await("the question starts a temporary conversation", func() bool {
		for _, info := range stage.runtime.store.Snapshot().Sessions {
			if !known[info.ID] {
				started = info
			}
		}
		return started.ID != "" && stage.sessionInfo(started.ID).Title != ""
	})
	if !started.Temporary || started.Memory != protocol.MemoryUseOnly {
		t.Fatalf("the conversation started as temporary %v with memory %q", started.Temporary, started.Memory)
	}
	stage.await("the memory choice locks after the first message", func() bool {
		_, ok := stage.named("Set when this conversation started.")
		return ok
	})
	if _, ok := stage.named("Temporary"); !ok {
		t.Fatalf("the open conversation is not marked temporary; on screen: %v", stage.names())
	}

	// narrowing still works after the first message
	stage.choose("Memory: Use only", "No memory")
	stage.await("the daemon narrows it", func() bool { return stage.sessionInfo(started.ID).Memory == protocol.MemoryNone })

	stage.click("Close this conversation")
	stage.await("leaving ends it", func() bool {
		_, still := stage.runtime.store.Snapshot().Sessions[started.ID]
		return !still
	})
	stage.await("the placard says it closed", func() bool { _, ok := stage.named("Temporary conversation closed."); return ok })
}

// The first-run card asks once; Turn on memory sets use and learn and marks
// the question answered.
func TestFirstRunCardTurnsMemoryOn(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	patch := []byte(`{"memory":{"asked":false,"use":false,"learn":false}}`)
	if err := stage.runtime.client.Call(context.Background(), protocol.MSettingsSet, protocol.SettingsSetParams{Patch: patch}, new(protocol.Settings)); err != nil {
		t.Fatal(err)
	}
	stage.open("ask/new")
	if _, ok := stage.named("Rashin can remember"); !ok {
		t.Fatalf("no first-run card; on screen: %v", stage.names())
	}
	stage.click("Turn on memory")
	stage.await("memory is on", func() bool {
		memory := stage.settings().Memory
		return memory.Asked && memory.Use && memory.Learn
	})
	stage.await("the card is gone", func() bool { _, ok := stage.named("Rashin can remember"); return !ok })
	if _, ok := stage.named("Memory is on. Rashin starts with your next conversation."); !ok {
		t.Fatalf("no placard; on screen: %v", stage.names())
	}
}

// Deleting a conversation that taught memory forgets what it taught when the
// toggle is on, which it is from the start.
func TestDeletingAnAskForgetsWhatItTaught(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	learned := stage.learnedFrom(memoryAsk)
	stage.open("ask/" + memoryAsk)
	stage.click("More")
	stage.click("Delete")
	toggle := fmt.Sprintf("Also forget the %d memories learned from it.", len(learned))
	stage.await("the dialog counts what was learned", func() bool { _, ok := stage.named(toggle); return ok })
	stage.click("Delete")
	stage.await("the conversation is deleted", func() bool {
		_, still := stage.runtime.store.Snapshot().Sessions[memoryAsk]
		return !still
	})
	if left := stage.learnedFrom(memoryAsk); len(left) != 0 || len(learned) == 0 {
		t.Fatalf("learned %v before and %v still remembered after", learned, left)
	}
}

func (s *stage) learnedFrom(session string) []string {
	s.t.Helper()
	var result protocol.MemoryListResult
	if err := s.runtime.client.Call(context.Background(), protocol.MMemoryList, protocol.MemoryListParams{Session: session}, &result); err != nil {
		s.t.Fatal(err)
	}
	ids := make([]string, 0, len(result.Memories))
	for _, memory := range result.Memories {
		ids = append(ids, memory.ID)
	}
	return ids
}

// The board's composer starts a temporary agent session too, and opening the
// board again ends it.
func TestTheBoardStartsATemporarySession(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("agent")
	stage.click("Temporary conversation  Ctrl Shift N")
	if _, ok := stage.named("Temporary"); !ok {
		t.Fatalf("the board is not marked temporary; on screen: %v", stage.names())
	}
	known := make(map[string]bool)
	for _, info := range stage.runtime.store.Snapshot().Sessions {
		known[info.ID] = true
	}
	stage.typeText("fix the build")
	stage.press(key.NameReturn, 0)
	var started protocol.SessionInfo
	stage.await("the board starts a temporary session", func() bool {
		for _, info := range stage.runtime.store.Snapshot().Sessions {
			if !known[info.ID] {
				started = info
			}
		}
		return started.ID != "" && stage.shell.router.Current().Session == started.ID
	})
	if !started.Temporary {
		t.Fatalf("the session started as %+v", started)
	}
	// the agent's own program may keep a log, so the field promises only what Rashin keeps
	if _, ok := stage.named("Temporary: Rashin keeps and learns nothing. The agent's own program may keep its log."); !ok {
		t.Fatalf("the temporary session does not say what is kept; on screen: %v", stage.names())
	}
	if _, ok := stage.named("Temporary"); !ok {
		t.Fatalf("the session is not marked temporary; on screen: %v", stage.names())
	}
	stage.open("agent")
	stage.await("leaving ends it", func() bool {
		_, still := stage.runtime.store.Snapshot().Sessions[started.ID]
		return !still
	})
}

// Undo stands once, in the popover's foot, because the daemon undoes a change
// whole; each row keeps its own Edit.
func TestAChangeHasOneUndo(t *testing.T) {
	stage := newStage(t, image.Pt(1612, 2600))
	stage.open("ask/" + memoryAsk)
	// the learning pass is the later line, with two facts in it
	var lines []input.SemanticNode
	for _, node := range stage.nodes() {
		// a line's words label an area that runs on past the window, the line itself is the control
		if node.Desc.Label == "Memory updated" && len(node.Children) > 0 {
			lines = append(lines, node)
		}
	}
	if len(lines) < 2 {
		t.Fatalf("expected two Memory updated lines; on screen: %v", stage.names())
	}
	last := lines[0]
	for _, node := range lines {
		if node.Desc.Bounds.Min.Y > last.Desc.Bounds.Min.Y {
			last = node
		}
	}
	stage.clickAt(centre(last.Desc.Bounds))
	stage.settle()
	if undo, edit := stage.count("Undo"), stage.count("Edit"); undo != 1 || edit != 2 {
		t.Fatalf("the popover has %d Undo and %d Edit; on screen: %v", undo, edit, stage.names())
	}
}

// A fact's code spans are set in mono like the Memory page sets them, never
// with their backticks.
func TestPopoverFactsShowNoBackticks(t *testing.T) {
	stage := newStage(t, image.Pt(1612, 2600))
	stage.open("ask/" + memoryAsk)
	stage.click("Used 3 memories")
	if _, ok := stage.named("This answer used"); !ok {
		t.Fatalf("the line opened no popover; on screen: %v", stage.names())
	}
	for _, name := range stage.names() {
		if strings.Contains(name, "`") {
			t.Fatalf("a backtick reached the screen: %q", name)
		}
	}
	if _, ok := stage.named("iwlwifi"); !ok {
		t.Fatalf("the literal words are not drawn; on screen: %v", stage.names())
	}
}

// buttons only: a plaque's words label the area under it too
func (s *stage) count(name string) int {
	count := 0
	for _, node := range s.nodes() {
		if node.Desc.Class == semantic.Button && (node.Desc.Label == name || node.Desc.Description == name) {
			count++
		}
	}
	return count
}

// Open memory takes the reader to the Memory page, and the popover it was
// pressed in is gone from over it.
func TestOpenMemoryClosesThePopover(t *testing.T) {
	stage := newStage(t, image.Pt(1612, 2600))
	stage.open("ask/" + memoryAsk)
	stage.click("Memory updated")
	if _, ok := stage.named("Rashin remembered"); !ok {
		t.Fatalf("the line opened no popover; on screen: %v", stage.names())
	}
	stage.click("Open memory")
	stage.await("the Memory page opens", func() bool { return stage.shell.router.Current().Tab == protocol.TabMemory })
	stage.settle()
	if _, ok := stage.named("Rashin remembered"); ok {
		t.Fatalf("the popover still stands over the Memory page; on screen: %v", stage.names())
	}
}

// An answer that drew only on the card says so, and its popover offers What
// Ask sees, which opens the Memory page with the card beside it.
func TestAnAnswerThatUsedOnlyTheCardSaysSo(t *testing.T) {
	stage := newStage(t, image.Pt(1612, 2600))
	stage.open("ask/ask-20261109-161200-prev")
	if _, ok := stage.named("Used 0 memories"); ok {
		t.Fatal("the line counts no memories")
	}
	stage.click("Used the card about you")
	stage.click("What Ask sees")
	stage.await("the card opens", func() bool {
		_, ok := stage.named("Close What Ask sees")
		return ok && stage.shell.router.Current().Tab == protocol.TabMemory
	})
	if _, ok := stage.named("This answer used"); ok {
		t.Fatalf("the popover still stands over the Memory page; on screen: %v", stage.names())
	}
}
