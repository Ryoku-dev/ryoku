package chat

import (
	"image"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func shownCalls(m *Model) []string {
	var calls []string
	for _, row := range m.state.rows {
		if row.kind != rowTool {
			continue
		}
		for _, tool := range row.tools {
			calls = append(calls, tool.Call)
		}
	}
	return calls
}

// A memory call stands as a tool line until the line that tells it arrives
// in its turn, and a failed one stays: no line will ever tell it.
func TestAMemoryCallStandsUntilItsLineArrives(t *testing.T) {
	m := NewModel(protocol.SessionInfo{ID: "agent-1", Kind: protocol.KindAgent})
	m.Apply(testEvent(t, 1, protocol.EvTurnStart, protocol.TurnStartData{Turn: 1}))
	m.Apply(testEvent(t, 2, protocol.EvTool, protocol.ToolData{Call: "search", Title: "mcp__rashin_session_search", Status: protocol.ToolDone}))
	if calls := shownCalls(m); len(calls) != 1 {
		t.Fatalf("a search with no line yet shows %v", calls)
	}
	m.Apply(testEvent(t, 3, protocol.EvMemory, protocol.MemoryEventData{Kind: protocol.MemoryLooked, Sessions: []protocol.MemoryRef{{Session: "ask-1"}}}))
	if calls := shownCalls(m); len(calls) != 0 {
		t.Fatalf("the looked line did not take the search's place: %v", calls)
	}
	m.Apply(testEvent(t, 4, protocol.EvTool, protocol.ToolData{Call: "remember", Title: "mcp__rashin_remember", Status: protocol.ToolFailed}))
	m.Apply(testEvent(t, 5, protocol.EvMemory, protocol.MemoryEventData{Kind: protocol.MemoryRefused, Reason: protocol.MemoryRefusedOff}))
	if calls := shownCalls(m); len(calls) != 1 || calls[0] != "remember" {
		t.Fatalf("a failed remember was hidden: %v", calls)
	}
}

// One view shows every conversation of a page, so two conversations with a
// memory line at the same place keep their popovers apart.
func TestMemoryLinesOfTwoConversationsStayApart(t *testing.T) {
	refused := protocol.MemoryEventData{Kind: protocol.MemoryRefused, Reason: protocol.MemoryRefusedSensitive, Text: "a fact"}
	c := newConversation(image.Pt(800, 600))
	for _, id := range []string{"ask-a", "ask-b"} {
		c.model = NewModel(protocol.SessionInfo{ID: id, Kind: protocol.KindAsk})
		c.model.Apply(testEvent(t, 1, protocol.EvMemory, refused))
		c.layout()
	}
	if count := len(c.view.state.memory); count != 2 {
		t.Fatalf("two conversations share %d memory line states", count)
	}
}

// An engine that did not answer in time is said in the daemon's words, never
// as "Used 0 memories".
func TestAUsedLineWithNothingSaysWhy(t *testing.T) {
	words := "Memory did not answer in time, so this answer draws on none of it."
	line := memoryLine(&memoryVisual{data: protocol.MemoryEventData{Kind: protocol.MemoryUsed, Text: words}})
	if line.label != words || line.pops {
		t.Fatalf("the line reads %q and opens a popover: %v", line.label, line.pops)
	}
}
