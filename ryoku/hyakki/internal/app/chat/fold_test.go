package chat

import (
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestFinishedTurnFoldsOnlyWorkThatShowsMoreThanOneLine(t *testing.T) {
	call := func(id string, agents ...protocol.AgentWork) protocol.ToolData {
		return protocol.ToolData{Call: id, Title: "Read " + id, Kind: protocol.ToolRead, Status: protocol.ToolDone, Agents: agents}
	}
	thought := protocol.TextData{Message: "t", Delta: "Checking the next file."}
	explorer := protocol.AgentWork{ID: "a", Name: "explore", Status: protocol.ToolDone}
	cases := []struct {
		name          string
		work          []any
		ended         bool
		fold          bool
		steps, thinks int
	}{
		{name: "one group of calls is already a fold", work: []any{call("a"), call("b"), call("c")}, ended: true},
		{name: "calls around a thought", work: []any{call("a"), thought, call("b")}, ended: true, fold: true, steps: 2, thinks: 1},
		{name: "a subagent box beside a group", work: []any{call("a", explorer), call("b"), call("c")}, ended: true, fold: true, steps: 3},
		{name: "a turn still working", work: []any{call("a"), thought, call("b")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := NewModel(protocol.SessionInfo{ID: "s"})
			model.Apply(testEvent(t, 1, protocol.EvUser, protocol.UserData{Text: "Go"}))
			model.Apply(testEvent(t, 2, protocol.EvTurnStart, protocol.TurnStartData{Turn: 1}))
			sequence := int64(3)
			for _, item := range tc.work {
				kind := protocol.EvTool
				if _, isThought := item.(protocol.TextData); isThought {
					kind = protocol.EvThought
				}
				model.Apply(testEvent(t, sequence, kind, item))
				sequence++
			}
			if tc.ended {
				model.Apply(testEvent(t, sequence, protocol.EvTurnEnd, protocol.TurnEndData{Turn: 1, Stop: protocol.StopEnd}))
			}
			folds := new(viewState).foldTurns(model.state)
			if !tc.fold {
				if len(folds) != 0 {
					t.Fatalf("folded %#v", folds)
				}
				return
			}
			if len(folds) != 1 {
				t.Fatalf("folds = %#v, want one", folds)
			}
			for _, fold := range folds {
				if fold.steps != tc.steps || fold.thoughts != tc.thinks {
					t.Fatalf("fold counts %d steps and %d thoughts, want %d and %d", fold.steps, fold.thoughts, tc.steps, tc.thinks)
				}
			}
		})
	}
}
