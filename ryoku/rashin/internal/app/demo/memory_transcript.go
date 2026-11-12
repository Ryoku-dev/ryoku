package demo

import (
	"encoding/json"
	"slices"

	"ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// the build question shows every memory line a conversation can carry: what the answer drew on and read, what was told and left out, what learning kept afterwards, and the narrowing that followed
func memoryTranscript(events []protocol.SessionEvent) []protocol.SessionEvent {
	memory := makeMemoryFixtures()
	used := func(ids ...string) []protocol.MemoryItem {
		items := make([]protocol.MemoryItem, 0, len(ids))
		for _, id := range ids {
			if index := memory.index(id); index >= 0 {
				items = append(items, memoryItem(memory.memories[index], ""))
			}
		}
		return items
	}
	looked := []protocol.MemoryRef{
		{Session: "ask-20261109-161200-prev", Kind: protocol.KindAsk, Title: i18n.T("How do I switch to niri?")},
		{Session: "ask-20261107-112500-week", Kind: protocol.KindAsk, Title: i18n.T("Where are provider keys stored?")},
	}
	events = insertAfter(events, protocol.EvTurnStart,
		memoryEvent(protocol.MemoryEventData{Kind: protocol.MemoryUsed, Used: used("m1a4c7e2", "m3e8a1c5", "m6c3e0f1"), Card: true}),
		memoryEvent(protocol.MemoryEventData{Kind: protocol.MemoryLooked, Sessions: looked}),
	)
	events = insertAfter(events, protocol.EvAnswer,
		memoryEvent(protocol.MemoryEventData{Kind: protocol.MemoryUpdated, Change: memory.change("chg-demo-told")}),
		memoryEvent(protocol.MemoryEventData{
			Kind: protocol.MemoryRefused, Reason: protocol.MemoryRefusedSensitive,
			Text: i18n.T("Has a doctor's appointment about migraines on Friday"),
		}),
	)
	return insertAfter(events, protocol.EvTurnEnd,
		memoryEvent(protocol.MemoryEventData{Kind: protocol.MemoryUpdated, Change: memory.change("chg-demo-learned")}),
		memoryEvent(protocol.MemoryEventData{Kind: protocol.MemoryNarrowed, Memory: protocol.MemoryUseOnly}),
	)
}

// the niri question drew on the card about the reader and on no single memory, and was read again by the build question
func cardTranscript(events []protocol.SessionEvent) []protocol.SessionEvent {
	return insertAfter(events, protocol.EvTurnStart, memoryEvent(protocol.MemoryEventData{Kind: protocol.MemoryUsed, Card: true}))
}

// the agent remembered through Rashin's tools: its call gives way to the memory line
func agentMemoryTranscript(events []protocol.SessionEvent) []protocol.SessionEvent {
	memory := makeMemoryFixtures()
	change := memory.change("chg-demo-agent")
	if change == nil {
		return events
	}
	call := protocol.ToolData{
		Call: "remember-1", Title: "mcp__rashin_remember", Kind: protocol.ToolOther, Status: protocol.ToolDone,
		Input: change.Items[0].Text,
	}
	return insertAfter(events, protocol.EvTool,
		protocol.SessionEvent{Kind: protocol.EvTool, Data: marshal(call)},
		memoryEvent(protocol.MemoryEventData{Kind: protocol.MemoryUpdated, Change: change}),
	)
}

func (m *memoryFixtures) change(id string) *protocol.MemoryChange {
	for _, recorded := range m.changes {
		if recorded.change.ID == id {
			change := recorded.change
			return &change
		}
	}
	return nil
}

func memoryEvent(data protocol.MemoryEventData) protocol.SessionEvent {
	return protocol.SessionEvent{Kind: protocol.EvMemory, Data: marshal(data)}
}

func marshal(value any) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}

// added after the first event of kind, and every event numbered again in order, at the time its place gives it
func insertAfter(events []protocol.SessionEvent, kind string, added ...protocol.SessionEvent) []protocol.SessionEvent {
	at := slices.IndexFunc(events, func(event protocol.SessionEvent) bool { return event.Kind == kind })
	if at < 0 {
		return events
	}
	events = slices.Insert(slices.Clone(events), at+1, added...)
	for index := range events {
		events[index].Seq = int64(index + 1)
		if events[index].TS.IsZero() {
			events[index].TS = events[index-1].TS
		}
	}
	return events
}
