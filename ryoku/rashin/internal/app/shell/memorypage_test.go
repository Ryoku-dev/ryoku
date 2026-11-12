package shell

import (
	"context"
	"encoding/json"
	"image"
	"slices"
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/pointer"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/protocol"
)

func (s *stage) memoryPlace() env.MemoryPlace {
	place := s.shell.router.Current()
	return env.MemoryPlace{Filter: place.MemoryFilter, Scope: place.MemoryScope, ID: place.Memory}
}

// a dialog's button is pressable when a control by that name stands enabled inside the dialog titled title
func (s *stage) pressableIn(title, name string) bool {
	s.t.Helper()
	dialog, ok := s.named(title)
	if !ok {
		s.t.Fatalf("no dialog is called %q", title)
	}
	for _, node := range s.nodes() {
		if node.Desc.Label == name && !node.Desc.Disabled && node.Desc.Bounds.In(dialog.Desc.Bounds) {
			return true
		}
	}
	return false
}

func TestTheMemoryPageOpensFromSettingsAndFilters(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("settings/memory")
	stage.click("Open memory")
	if tab := stage.shell.router.Tab; tab != protocol.TabMemory {
		t.Fatalf("Open memory went to %q", tab)
	}
	if _, ok := stage.named("Add memory"); !ok {
		t.Fatalf("the Memory page has no Add memory: %v", stage.names())
	}
	if _, ok := stage.named("Uses fish as their login shell"); !ok {
		t.Fatalf("All does not list what Rashin remembers about you: %v", stage.names())
	}
	stage.click("Suggested")
	stage.await("the Suggested filter", func() bool { return stage.memoryPlace().Filter == env.MemorySuggested })
	stage.await("a suggestion to keep", func() bool { _, ok := stage.named("Keep"); return ok })
	if _, ok := stage.named("Uses fish as their login shell"); ok {
		t.Fatal("Suggested still lists a memory in use")
	}
}

func TestSearchingMemoryKeepsOnlyWhatMatches(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("memory")
	stage.click("Search memory")
	stage.typeText("niri")
	stage.await("the search to drop the fish shell", func() bool {
		_, fish := stage.named("Uses fish as their login shell")
		return !fish
	})
	matched := slices.ContainsFunc(stage.names(), func(name string) bool {
		return name != "Search memory" && strings.Contains(strings.ToLower(name), "niri")
	})
	if !matched {
		t.Fatalf("nothing about niri is listed: %v", stage.names())
	}
}

func TestAMemoryIsEditedAndSaved(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	const id, before, after = "m1a4c7e2", "Uses fish as their login shell", "Uses fish as the login shell everywhere"
	stage.open("memory/about")
	stage.click(before)
	stage.click("Edit")
	if _, ok := stage.named("Save"); !ok {
		t.Fatalf("Edit opened no editor: %v", stage.names())
	}
	stage.typeOver(len([]rune(before)), after)
	stage.click("Save")
	stage.await("the demo daemon to keep the new text", func() bool { return stage.memoryText(id) == after })
	if _, ok := stage.named("Save"); ok {
		t.Fatal("the editor stayed open after Save")
	}
}

func (s *stage) memoryText(id string) string {
	for _, memory := range s.memories(protocol.MemoryListParams{Limit: 1000}) {
		if memory.ID == id {
			return memory.Text
		}
	}
	return ""
}

func TestForgettingAMemoryCanBeUndone(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	const id, text = "m3e8a1c5", "Prefers answers with the command first"
	stage.open("memory/about")
	stage.click(text)
	stage.click("Forget")
	stage.await("the memory to be forgotten", func() bool { return stage.memoryStatus(id) == "" })
	stage.await("its row to leave", func() bool { _, ok := stage.named(text); return !ok })
	stage.click("Undo")
	stage.await("Undo to bring it back", func() bool { return stage.memoryStatus(id) == protocol.MemoryActive })
	stage.await("its row to return", func() bool { _, ok := stage.named(text); return ok })
}

func TestAddMemoryRemembersWhatIsTyped(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	const text = "Prefers tea over coffee in the morning"
	stage.open("memory")
	stage.click("Add memory")
	stage.click("Save")
	if _, ok := stage.named("A memory can't be empty."); !ok {
		t.Fatalf("an empty memory was not refused: %v", stage.names())
	}
	stage.typeText(text)
	stage.click("Save")
	stage.await("the new memory to be remembered", func() bool {
		return hasText(stage.memories(protocol.MemoryListParams{Scope: protocol.MemoryYou}), text)
	})
	stage.await("its row to show", func() bool { _, ok := stage.named(text); return ok })
}

func TestWhatAskSeesShowsTheCard(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("memory")
	stage.click("What Ask sees")
	stage.await("the card's sheet", func() bool { _, ok := stage.named("Rebuild"); return ok })
	card := stage.runtime.store.Snapshot().Memory.Card.Text
	if card == "" {
		t.Fatal("the demo has no card to show")
	}
	if !slices.ContainsFunc(stage.names(), func(name string) bool { return name == card }) {
		t.Fatalf("the sheet does not show the card: %v", stage.names())
	}
	stage.click("Close What Ask sees")
	if _, ok := stage.named("Rebuild"); ok {
		t.Fatal("Close left the sheet open")
	}
}

func TestForgetEverythingWaitsForTheWord(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("memory")
	stage.click("More")
	stage.click("Forget everything")
	const title = "Forget everything?"
	if stage.pressableIn(title, "Forget everything") {
		t.Fatal("Forget everything was ready before forget was typed")
	}
	stage.click("Type forget to confirm")
	stage.typeText("forg")
	if stage.pressableIn(title, "Forget everything") {
		t.Fatal("Forget everything was ready before the whole word was typed")
	}
	stage.typeOver(4, "forget")
	if !stage.pressableIn(title, "Forget everything") {
		t.Fatalf("Forget everything stayed off after forget was typed: %v", stage.names())
	}
	stage.click("Forget everything")
	stage.await("everything to be forgotten", func() bool {
		return len(stage.memories(protocol.MemoryListParams{})) == 0 && len(stage.memories(protocol.MemoryListParams{Status: protocol.MemorySuggested})) == 0
	})
	stage.await("the empty page", func() bool { _, ok := stage.named("Nothing remembered yet"); return ok })
}

func TestImportPutsSuggestionsUnderSuggested(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	before := len(stage.memories(protocol.MemoryListParams{Status: protocol.MemorySuggested}))
	stage.open("memory")
	stage.click("More")
	stage.click("Import from another assistant")
	// the field's named area reaches to its tallest, so it is pressed near its top where its box always stands
	field, ok := stage.named("Its answer")
	if !ok {
		t.Fatalf("the import dialog has no field for the answer: %v", stage.names())
	}
	stage.clickAt(field.Desc.Bounds.Min.Add(image.Pt(24, 16)))
	stage.typeText("Lives in Lisbon\nPlays the cello on weekends")
	stage.click("Import")
	stage.await("two suggestions", func() bool {
		return len(stage.memories(protocol.MemoryListParams{Status: protocol.MemorySuggested})) == before+2
	})
	stage.click("Show")
	stage.await("Suggested to open", func() bool {
		return stage.shell.router.Tab == protocol.TabMemory && stage.memoryPlace().Filter == env.MemorySuggested
	})
	stage.await("the imported suggestion to be listed", func() bool { _, ok := stage.named("Plays the cello on weekends"); return ok })
}

func (s *stage) setMemory(patch map[string]any) {
	s.t.Helper()
	raw, _ := json.Marshal(map[string]any{"memory": patch})
	if err := s.runtime.client.Call(context.Background(), protocol.MSettingsSet, protocol.SettingsSetParams{Patch: raw}, new(json.RawMessage)); err != nil {
		s.t.Fatal(err)
	}
	s.settle()
}

// reveal wheels the page until the control called name stands inside the window
func (s *stage) reveal(name string) {
	s.t.Helper()
	at := f32.Pt(float32(s.size.X)*2/3, float32(s.size.Y)/2)
	for range 80 {
		if node, ok := s.named(name); ok && node.Desc.Bounds.Max.Y < s.size.Y-40 && node.Desc.Bounds.Min.Y > 60 {
			return
		}
		s.queue(pointer.Event{Kind: pointer.Move, Position: at}, pointer.Event{Kind: pointer.Scroll, Position: at, Scroll: f32.Pt(0, 60)})
		s.settle()
	}
	s.t.Fatalf("%q never came into view", name)
}

func (s *stage) pinned(id string) bool {
	s.t.Helper()
	var memory protocol.Memory
	if err := s.runtime.client.Call(context.Background(), protocol.MMemoryGet, protocol.MemoryRefParams{ID: id}, &memory); err != nil {
		s.t.Fatal(err)
	}
	return memory.Pinned
}

func TestAClickedRowTakesItsKeys(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	const id, text = "m3e8a1c5", "Prefers answers with the command first"
	stage.open("memory/about")
	stage.click(text)
	stage.press("P", 0)
	stage.await("P to pin the clicked row", func() bool { return stage.pinned(id) })
}

func TestABlockedLineIsListedAndCanBeForgotten(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	blocked := stage.memories(protocol.MemoryListParams{Status: protocol.MemoryBlocked})
	if len(blocked) == 0 {
		t.Fatal("the demo has no blocked line")
	}
	stage.open("memory")
	text := env.PlainMemory(blocked[0].Text)
	stage.reveal(text)
	stage.click(text)
	stage.click("Forget")
	stage.await("the blocked line to be forgotten", func() bool { return stage.memoryStatus(blocked[0].ID) == "" })
}

func TestAnUnreachableEngineIsNamedAndTriedAgain(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	if err := stage.runtime.client.Call(context.Background(), protocol.MMemoryForgetAll, protocol.MemoryForgetAllParams{}, new(protocol.MemoryChange)); err != nil {
		t.Fatal(err)
	}
	stage.settle()
	status := *stage.runtime.store.Snapshot().Memory
	status.Engine, status.EngineState, status.EngineDetail = protocol.MemoryHindsight, protocol.EngineUnreachable, "connection refused"
	stage.runtime.store.SetMemoryStatus(status)
	stage.open("memory")
	if _, ok := stage.named("Can't reach Hindsight"); !ok {
		t.Fatalf("the empty state does not name the engine in use: %v", stage.names())
	}
	stage.click("Try again")
	stage.await("the status to be asked for again", func() bool {
		return stage.runtime.store.Snapshot().Memory.EngineState == protocol.EngineReady
	})
}

func TestTheHonchoKeyWaitsForAnAddress(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("settings/memory")
	stage.reveal("Honcho")
	stage.click("Honcho")
	stage.reveal("Key")
	stage.click("Key")
	stage.typeText("not-a-real-key")
	if node, ok := stage.named("Keep in the vault"); !ok || !node.Desc.Disabled {
		t.Fatal("Keep in the vault is offered before an address is saved")
	}
}

func TestALocalServerDoesNotLeaveThisComputer(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.setMemory(map[string]any{"honcho": map[string]any{"url": "http://127.0.0.1:8000"}})
	stage.open("settings/memory")
	stage.reveal("Honcho")
	stage.click("Honcho")
	if _, ok := stage.named("Leaves this computer"); ok {
		t.Fatal("a server on this computer is said to leave it")
	}
	stage.setMemory(map[string]any{"honcho": map[string]any{"url": "https://memory.example.org"}})
	if _, ok := stage.named("Leaves this computer"); !ok {
		t.Fatal("a remote server is not said to leave this computer")
	}
}

func TestARemoteHindsightOffersNoSetup(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.setMemory(map[string]any{"hindsight": map[string]any{"local": false, "url": "https://hindsight.example.org"}})
	stage.open("settings/memory")
	stage.reveal("Hindsight")
	stage.click("Hindsight")
	if _, ok := stage.named("Set up on this computer"); ok {
		t.Fatal("a remote Hindsight offers to be set up on this computer")
	}
}

func TestMovingMemoryChangesTheEngineInSettings(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("settings/memory")
	stage.reveal("Hindsight")
	stage.click("Hindsight")
	stage.click("Move memory to Hindsight")
	stage.click("Move")
	stage.await("settings to name the new engine", func() bool {
		return stage.runtime.store.Snapshot().Settings.Memory.Engine == protocol.MemoryHindsight
	})
}

func TestClosingWhatAskSeesPutsThePanelBack(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("memory")
	if stage.shell.preferences.SidePanelOpen {
		t.Fatal("the side panel starts open")
	}
	stage.click("What Ask sees")
	stage.click("Close What Ask sees")
	if stage.shell.preferences.SidePanelOpen {
		t.Fatal("closing the sheet left the side panel open")
	}
}

func TestOpeningMemoryWithTheCardShowsWhatAskSees(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("ask/new")
	stage.shell.openMemory(env.MemoryPlace{Card: true})
	stage.settle()
	if _, ok := stage.named("Close What Ask sees"); !ok {
		t.Fatalf("the Memory page opened without What Ask sees: %v", stage.names())
	}
}

func TestOpeningAMemoryShowsItsEditor(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	// the oldest memory about you stands below the fold, with the projects and teams still under it
	memories := stage.memories(protocol.MemoryListParams{Scope: protocol.MemoryYou, Sort: protocol.MemorySortOldest, Limit: 1})
	stage.open("ask/new")
	stage.shell.openMemory(env.MemoryPlace{ID: memories[0].ID})
	stage.await("the editor to open", func() bool { _, ok := stage.named("Forget this memory"); return ok })
	field, ok := stage.named("Memory")
	save, _ := stage.named("Save")
	if !ok || save.Desc.Bounds.Max.Y > stage.size.Y || save.Desc.Bounds.Min.Y < 0 {
		t.Fatalf("the editor is not in view: field %v, save %v", field.Desc.Bounds, save.Desc.Bounds)
	}
	if top := field.Desc.Bounds.Min.Y; top > stage.size.Y/3 {
		var all []image.Rectangle
		for _, node := range stage.nodes() {
			if node.Desc.Label == "Memory" {
				all = append(all, node.Desc.Bounds)
			}
		}
		t.Fatalf("the editor starts at %d, below the top third of %d; save %v, memory nodes %v", top, stage.size.Y, save.Desc.Bounds, all)
	}
}
