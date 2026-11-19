package chat

import (
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestSearchStepsCountDistinctToolKinds(t *testing.T) {
	var progress searchWork
	progress.add([]*protocol.ToolData{
		nil,
		{Kind: protocol.ToolSearch}, {Kind: protocol.ToolSearch},
		{Kind: protocol.ToolFetch}, {Kind: protocol.ToolFetch},
		{Kind: protocol.ToolFetch}, {Kind: protocol.ToolFetch},
		{Kind: protocol.ToolRead}, {Kind: protocol.ToolExecute},
	})
	if progress.searches != 2 || progress.pages != 4 || progress.files != 1 {
		t.Fatalf("wrong tool counts: %#v", progress)
	}
	if got := progress.text(); got != "Searched 2, Read 4 pages, Read 1 file" {
		t.Fatalf("summary = %q", got)
	}
}

func TestSearchStepsOffKeepsSingleWorkRowUnfolded(t *testing.T) {
	turn := &turnState{end: &protocol.TurnEndData{Turn: 1, Stop: protocol.StopEnd}}
	model := &modelState{rows: []transcriptRow{
		{kind: rowUser},
		{kind: rowTool, tools: []*protocol.ToolData{{Kind: protocol.ToolSearch}}},
		{kind: rowTurnEnd, turn: turn},
	}}
	var view View
	view.SetFeatures(func(string) bool { return false })
	if got := view.ensureState().foldTurns(model); len(got) != 0 {
		t.Fatalf("disabled search steps changed folding: %#v", got)
	}
	view.SetFeatures(func(id string) bool { return id == "searchsteps" })
	folds := view.ensureState().foldTurns(model)
	if fold := folds[1]; fold == nil || fold.search.searches != 1 {
		t.Fatalf("completed search did not fold into the work line: %#v", folds)
	}
}
