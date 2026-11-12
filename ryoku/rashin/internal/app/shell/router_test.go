package shell

import (
	"image"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"ryoku-rashin/internal/protocol"
)

// stands in for a page that asks for keys of its own, between the router and the end of the frame
type keyboard struct {
	operations op.Ops
	input      input.Router
	router     *Router
	now        time.Time
	after      func(layout.Context)
}

func newKeyboard(router *Router) *keyboard {
	board := &keyboard{router: router, now: time.Unix(100, 0)}
	board.frame()
	return board
}

func (board *keyboard) frame() {
	board.operations.Reset()
	gtx := layout.Context{
		Ops: &board.operations, Source: board.input.Source(), Now: board.now,
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(800, 600)),
	}
	board.router.HandleKeys(gtx, nil)
	if board.after != nil {
		board.after(gtx)
	}
	board.input.Frame(&board.operations)
	board.now = board.now.Add(time.Millisecond)
}

func (board *keyboard) press(name key.Name, modifiers key.Modifiers) {
	board.input.Queue(key.Event{Name: name, Modifiers: modifiers, State: key.Press})
	board.frame()
}

func TestRouterHistoryBackAndForward(t *testing.T) {
	router := NewRouter(protocol.TabAsk, "ask-one", nil)
	router.Navigate(protocol.TabAgent, "agent-one")
	router.Push(Place{Tab: protocol.TabProjects, Project: "hyperion", ProjectView: "changes"})
	if !router.Back() || router.Tab != protocol.TabAgent || router.Session != "agent-one" {
		t.Fatalf("back = %#v", router.Current())
	}
	if !router.Back() || router.Tab != protocol.TabAsk || router.Session != "ask-one" {
		t.Fatalf("second back = %#v", router.Current())
	}
	if router.Back() {
		t.Fatal("back walked before the first place")
	}
	if !router.Forward() || router.Tab != protocol.TabAgent {
		t.Fatalf("forward = %#v", router.Current())
	}
	router.Navigate(protocol.TabIndex, "")
	if router.CanForward() {
		t.Fatal("new navigation did not truncate forward history")
	}
}

func TestReplaceStandsInForThePlaceWithoutGrowingHistory(t *testing.T) {
	router := NewRouter(protocol.TabAsk, "", nil)
	router.Push(Place{Tab: protocol.TabProjects})
	router.Replace(Place{Tab: protocol.TabProjects, Project: "hyperion", ProjectView: "overview"})
	if current := router.Current(); current.Project != "hyperion" {
		t.Fatalf("current place = %#v", current)
	}
	if !router.Back() || router.Tab != protocol.TabAsk {
		t.Fatalf("back from a replaced place went to %#v, want Ask", router.Current())
	}
}

func TestPaneArrivesAfreshOnlyForAnotherDestinationOrProject(t *testing.T) {
	home := Place{Tab: protocol.TabAsk}
	first := Place{Tab: protocol.TabAsk, Session: "ask-one"}
	if home.arrival() != first.arrival() {
		t.Fatal("a first question becoming a session would make the pane arrive again")
	}
	if first.key() == home.key() {
		t.Fatal("a session and its destination's home are told apart nowhere")
	}
	overview := Place{Tab: protocol.TabProjects, Project: "hyperion", ProjectView: "overview"}
	changes := Place{Tab: protocol.TabProjects, Project: "hyperion", ProjectView: "changes"}
	other := Place{Tab: protocol.TabProjects, Project: "ryotunes"}
	if overview.arrival() != changes.arrival() || overview.arrival() == other.arrival() {
		t.Fatal("a project's views must share an arrival that another project does not")
	}
	crew := Place{Tab: protocol.TabTeams, Team: "team-research", TeamView: "overview"}
	spend := Place{Tab: protocol.TabTeams, Team: "team-research", TeamView: "spend"}
	mail := Place{Tab: protocol.TabTeams, Team: "team-mail"}
	if crew.arrival() != spend.arrival() || crew.arrival() == mail.arrival() || crew.key() == mail.key() {
		t.Fatal("a team's views must share an arrival and a key that another team does not")
	}
}

func TestShortcutsReachTheirActions(t *testing.T) {
	router := NewRouter(protocol.TabAsk, "", nil)
	var newCount, foldCount, panelCount, zoom int
	router.OnNew = func() { newCount++ }
	router.OnToggleFold = func() { foldCount++ }
	router.OnTogglePanel = func() { panelCount++ }
	router.OnZoom = func(delta int) { zoom = delta }
	board := newKeyboard(router)

	board.press("K", key.ModShortcut)
	if !router.PaletteOpen {
		t.Fatal("Ctrl K did not open search")
	}
	board.frame()
	board.press(key.NameEscape, 0)
	if router.PaletteOpen {
		t.Fatal("Escape did not close search")
	}
	board.press("N", key.ModShortcut)
	board.press("B", key.ModShortcut)
	board.press("\\", key.ModShortcut)
	if newCount != 1 || foldCount != 1 || panelCount != 1 {
		t.Fatalf("shortcut callbacks = new %d, fold %d, panel %d", newCount, foldCount, panelCount)
	}
	for index, tab := range []string{protocol.TabAsk, protocol.TabAgent, protocol.TabProjects, protocol.TabIndex} {
		board.press(key.Name('1'+rune(index)), key.ModShortcut)
		if router.Tab != tab {
			t.Fatalf("Ctrl %d tab = %q, want %q", index+1, router.Tab, tab)
		}
	}
	board.press(",", key.ModShortcut)
	if router.Tab != protocol.TabSettings {
		t.Fatalf("Ctrl comma tab = %q", router.Tab)
	}
	board.press("T", key.ModShortcut)
	if router.Tab != protocol.TabInbox || !router.TeamsMode() {
		t.Fatalf("Ctrl T went to %q, want the Inbox in Teams mode", router.Tab)
	}
	for index, tab := range []string{
		protocol.TabInbox, protocol.TabTeams, protocol.TabDreams, protocol.TabWork, protocol.TabRoutines, protocol.TabVault, protocol.TabActivity,
	} {
		board.press(key.Name('1'+rune(index)), key.ModShortcut)
		if router.Tab != tab {
			t.Fatalf("Ctrl %d in Teams tab = %q, want %q", index+1, router.Tab, tab)
		}
	}
	board.press("T", key.ModShortcut)
	if router.Tab != protocol.TabIndex || router.TeamsMode() {
		t.Fatalf("Ctrl T back went to %q, want Index where Rashin was left", router.Tab)
	}
	board.press("5", key.ModShortcut)
	if router.Tab != protocol.TabIndex {
		t.Fatalf("Ctrl 5 in Rashin went to %q, Rashin has four destinations", router.Tab)
	}
	board.press("+", key.ModShortcut)
	if zoom != 10 {
		t.Fatalf("Ctrl plus zoom delta = %d", zoom)
	}
	board.press("0", key.ModShortcut)
	if zoom != 0 {
		t.Fatalf("Ctrl 0 zoom delta = %d", zoom)
	}
}

func escapes(board *keyboard) *int {
	heard := 0
	board.after = func(gtx layout.Context) {
		for {
			raw, ok := gtx.Event(key.Filter{Name: key.NameEscape})
			if !ok {
				return
			}
			if pressed, ok := raw.(key.Event); ok && pressed.State == key.Press {
				heard++
			}
		}
	}
	board.frame()
	return &heard
}

func TestEscapeIsLeftToThePageUnlessTheFrameHasSomethingToClose(t *testing.T) {
	router := NewRouter(protocol.TabAsk, "", nil)
	sheet := false
	router.CanEscape = func() bool { return sheet }
	router.OnEscape = func() bool { closed := sheet; sheet = false; return closed }
	board := newKeyboard(router)
	heard := escapes(board)

	board.press(key.NameEscape, 0)
	if *heard != 1 {
		t.Fatalf("the page heard %d Escapes with nothing of the frame's open, want 1", *heard)
	}
	sheet = true
	board.frame()
	board.press(key.NameEscape, 0)
	if sheet || *heard != 1 {
		t.Fatalf("Escape with a sheet open: sheet still open %v, page heard %d, want closed and 1", sheet, *heard)
	}
}

func TestOpenDialogKeepsTheFramesShortcutsOut(t *testing.T) {
	router := NewRouter(protocol.TabAsk, "", nil)
	created := 0
	router.OnNew = func() { created++ }
	board := newKeyboard(router)
	heard := escapes(board)

	router.Modal = true
	board.frame()
	board.press("N", key.ModShortcut)
	board.press("2", key.ModShortcut)
	board.press(key.NameEscape, 0)
	if created != 0 || router.Tab != protocol.TabAsk {
		t.Fatalf("shortcuts acted under a dialog: new %d, tab %q", created, router.Tab)
	}
	if *heard != 1 {
		t.Fatalf("the dialog heard %d Escapes, want 1", *heard)
	}
}

func TestEachModeGoesBackToWhereItWasLeft(t *testing.T) {
	router := NewRouter(protocol.TabAgent, "agent-one", nil)
	router.SwitchMode()
	if router.Tab != protocol.TabInbox {
		t.Fatalf("Teams opened first at %q, want the Inbox", router.Tab)
	}
	task := Place{Tab: protocol.TabWork, TeamTask: "task-6f1c2a9e"}
	router.Push(task)
	router.Navigate(protocol.TabSettings, "")
	if !router.TeamsMode() {
		t.Fatal("Settings opened from Teams left Teams mode")
	}
	router.SwitchMode()
	if current := router.Current(); current.Tab != protocol.TabAgent || current.Session != "agent-one" {
		t.Fatalf("back to Rashin went to %#v, want the agent session", current)
	}
	router.SwitchMode()
	if current := router.Current(); current != task {
		t.Fatalf("back to Teams went to %#v, want the task", current)
	}
}
