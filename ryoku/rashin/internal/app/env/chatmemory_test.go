package env

import (
	"testing"

	"ryoku-rashin/internal/app/chat"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// A frame that brought nothing new to the transcript looks through none of
// it: the memory lines are only read again when its sequence moves.
func TestMemoryLinesCostNothingOnAQuietFrame(t *testing.T) {
	model := chat.NewModel(protocol.SessionInfo{ID: "ask-1", Kind: protocol.KindAsk})
	model.Apply(protocol.SessionEvent{Seq: 1, Kind: protocol.EvMemory, Data: []byte(`{"kind":"updated","change":{"id":"c1","items":[]}}`)})
	change := protocol.MemoryChange{ID: "c1"}
	snapshot := state.Snapshot{Memory: &protocol.MemoryStatus{}, MemoryChanges: map[string]protocol.MemoryChange{"c1": change}}
	var lines MemoryLines
	var view chat.View
	lines.Sync(nil, snapshot, &view, model)
	if allocations := testing.AllocsPerRun(50, func() { lines.Sync(nil, snapshot, &view, model) }); allocations != 0 {
		t.Fatalf("a quiet frame allocated %v times", allocations)
	}
}
