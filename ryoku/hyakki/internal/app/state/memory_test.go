package state

import (
	"encoding/json"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// The setup line reads the latest step and keeps the result after the last
// one, so "Done" or the error does not vanish on the next frame; a fresh
// snapshot starts it over.
func TestMemorySetupProgressStaysUntilASnapshotReplacesIt(t *testing.T) {
	store := New(nil)
	setup := func(progress protocol.MemorySetupProgress) {
		t.Helper()
		data, err := json.Marshal(progress)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Apply(protocol.Event{Event: protocol.EvtMemorySetup, Data: data}); err != nil {
			t.Fatal(err)
		}
	}
	if store.Snapshot().MemorySetup != nil {
		t.Fatal("a new store has setup progress")
	}
	before := store.Snapshot().MemoryRevision
	setup(protocol.MemorySetupProgress{Engine: protocol.MemoryHindsight, Step: "Fetching models", Part: 2, Parts: 4})
	got := store.Snapshot()
	if got.MemorySetup == nil || got.MemorySetup.Part != 2 || got.MemorySetup.Step != "Fetching models" {
		t.Fatalf("setup = %+v, want part 2", got.MemorySetup)
	}
	if got.MemoryRevision == before {
		t.Fatal("setup progress did not move the memory revision")
	}
	setup(protocol.MemorySetupProgress{Engine: protocol.MemoryHindsight, Part: 4, Parts: 4, Done: true, Error: "uv is missing"})
	for range 2 {
		if done := store.Snapshot().MemorySetup; done == nil || !done.Done || done.Error != "uv is missing" {
			t.Fatalf("finished setup = %+v, want it kept with its error", done)
		}
	}
	store.ApplySnapshot(protocol.Snapshot{})
	if store.Snapshot().MemorySetup != nil {
		t.Fatal("a snapshot kept the old setup progress")
	}
}
