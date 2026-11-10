package chat

import (
	"encoding/json"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func testEvent(t *testing.T, sequence int64, kind string, data any) protocol.SessionEvent {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return protocol.SessionEvent{Seq: sequence, Kind: kind, Data: raw}
}

func TestReducerOrdersRowsAndIgnoresAppliedSequences(t *testing.T) {
	model := NewModel(protocol.SessionInfo{ID: "ask-test"})
	model.Apply(testEvent(t, 1, protocol.EvUser, protocol.UserData{Text: "Question"}))
	model.Apply(testEvent(t, 2, protocol.EvText, protocol.TextData{Message: "m1", Delta: "First"}))
	model.Apply(testEvent(t, 2, protocol.EvText, protocol.TextData{Message: "m1", Delta: " duplicate"}))
	model.Apply(testEvent(t, 1, protocol.EvError, protocol.ErrorData{Message: "old"}))

	if model.LastSeq() != 2 {
		t.Fatalf("LastSeq = %d, want 2", model.LastSeq())
	}
	if len(model.state.rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(model.state.rows))
	}
	if model.state.rows[0].kind != rowUser || model.state.rows[1].kind != rowText {
		t.Fatalf("row order = %v, %v", model.state.rows[0].kind, model.state.rows[1].kind)
	}
	if model.state.rows[1].text != "First" {
		t.Fatalf("text = %q, duplicate sequence was applied", model.state.rows[1].text)
	}
}

func TestReducerMergesToolsAndAppendsTerminalOutput(t *testing.T) {
	model := NewModel(protocol.SessionInfo{})
	model.Apply(testEvent(t, 1, protocol.EvTool, protocol.ToolData{Call: "c1", Title: "Read", Kind: protocol.ToolRead, Status: protocol.ToolRunning, Input: "a"}))
	model.Apply(testEvent(t, 2, protocol.EvTool, map[string]any{"call": "c1", "status": protocol.ToolDone, "output": "ok", "input": ""}))
	model.Apply(testEvent(t, 3, protocol.EvTerminal, protocol.TerminalData{Terminal: "term1", Command: "printf a", Delta: "a"}))
	exit := 0
	model.Apply(testEvent(t, 4, protocol.EvTerminal, protocol.TerminalData{Terminal: "term1", Delta: "b", Exit: &exit, Done: true}))

	tool := model.state.rows[model.state.tools["c1"]].tool
	if tool.Title != "Read" || tool.Status != protocol.ToolDone || tool.Output != "ok" || tool.Input != "" {
		t.Fatalf("merged tool = %#v", tool)
	}
	terminal := model.state.terminals["term1"]
	if terminal.text != "ab" || !terminal.data.Done || terminal.data.Exit == nil || *terminal.data.Exit != 0 {
		t.Fatalf("terminal = %#v text %q", terminal.data, terminal.text)
	}
}

func TestReducerKeepsPermissionsOpenUntilResolved(t *testing.T) {
	model := NewModel(protocol.SessionInfo{})
	first := protocol.PermissionData{Request: "r1", Title: "First"}
	second := protocol.PermissionData{Request: "r2", Title: "Second"}
	model.Apply(testEvent(t, 1, protocol.EvPermission, first))
	model.Apply(testEvent(t, 2, protocol.EvPermission, second))
	if pending := model.Pending(); len(pending) != 2 || pending[0].Request != "r1" || pending[1].Request != "r2" {
		t.Fatalf("pending order = %#v", pending)
	}
	model.Apply(testEvent(t, 3, protocol.EvPermissionResolved, protocol.PermissionResolvedData{Request: "r1", Option: "once", Outcome: "selected"}))
	if pending := model.Pending(); len(pending) != 1 || pending[0].Request != "r2" {
		t.Fatalf("pending after resolution = %#v", pending)
	}
}

func TestReducerTracksTurnBoundaryAndAttachesAnswer(t *testing.T) {
	model := NewModel(protocol.SessionInfo{})
	model.Apply(testEvent(t, 1, protocol.EvTurnStart, protocol.TurnStartData{Turn: 7}))
	if !model.Busy() {
		t.Fatal("model is not busy after turn_start")
	}
	answer := protocol.AnswerData{Via: "provider/model", NeedsAgent: true, Reason: "Needs a file change"}
	model.Apply(testEvent(t, 2, protocol.EvAnswer, answer))
	turn := model.state.turns[7]
	if turn.answer == nil || turn.answer.Via != answer.Via || turn.answerRow < 0 {
		t.Fatalf("answer was not attached to turn: %#v", turn)
	}
	usage := &protocol.TokenUsage{Input: 10, Output: 4}
	model.Apply(testEvent(t, 3, protocol.EvTurnEnd, protocol.TurnEndData{Turn: 7, Stop: protocol.StopEnd, Usage: usage}))
	if model.Busy() || turn.end == nil || turn.end.Usage.Output != 4 {
		t.Fatalf("turn did not end: busy=%v turn=%#v", model.Busy(), turn)
	}
}

func TestResetRebuildsReplay(t *testing.T) {
	model := NewModel(protocol.SessionInfo{Title: "Old"})
	model.Apply(testEvent(t, 9, protocol.EvUser, protocol.UserData{Text: "Old"}))
	replay := []protocol.SessionEvent{
		testEvent(t, 1, protocol.EvUser, protocol.UserData{Text: "New"}),
		testEvent(t, 2, protocol.EvTitle, protocol.TitleData{Title: "Rebuilt"}),
	}
	model.Reset(protocol.SessionInfo{ID: "ask-new"}, replay)
	if model.LastSeq() != 2 || len(model.state.rows) != 1 || model.Info().Title != "Rebuilt" {
		t.Fatalf("reset state = seq %d rows %d info %#v", model.LastSeq(), len(model.state.rows), model.Info())
	}
}
