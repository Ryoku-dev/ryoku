package demo

import (
	"encoding/json"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func memoryCall[T any](t *testing.T, s *server, method string, params any) T {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.callMemory(method, raw)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	typed, ok := result.(T)
	if !ok {
		t.Fatalf("%s answered %T", method, result)
	}
	return typed
}

func memoryList(t *testing.T, s *server, params protocol.MemoryListParams) protocol.MemoryListResult {
	return memoryCall[protocol.MemoryListResult](t, s, protocol.MMemoryList, params)
}

func TestMemoryListCounts(t *testing.T) {
	s := &server{data: makeFixtures()}
	status := memoryCall[protocol.MemoryStatus](t, s, protocol.MMemoryStatus, nil)
	if got := memoryList(t, s, protocol.MemoryListParams{}).Total; got != status.Counts.All || got < 30 {
		t.Fatalf("active %d, counts %+v", got, status.Counts)
	}
	for scope, want := range map[string]int{protocol.MemoryYou: status.Counts.You, protocol.MemoryProjects: status.Counts.Projects, protocol.MemoryTeams: status.Counts.Teams} {
		if got := memoryList(t, s, protocol.MemoryListParams{Scope: scope}).Total; got != want {
			t.Fatalf("%s: %d, want %d", scope, got, want)
		}
	}
	for state, want := range map[string]int{protocol.MemorySuggested: 5, protocol.MemoryReplaced: 3, protocol.MemoryBlocked: 1} {
		if got := memoryList(t, s, protocol.MemoryListParams{Status: state}).Total; got != want {
			t.Fatalf("%s: %d, want %d", state, got, want)
		}
	}
	if got := memoryList(t, s, protocol.MemoryListParams{Session: demoMemorySession}).Total; got != 3 {
		t.Fatalf("learned in %s: %d, want 3", demoMemorySession, got)
	}
	if days := status.Days; len(days) != 30 || days[29].InUse != status.Counts.All || days[29].Day != "2026-11-10" {
		t.Fatalf("days end %+v", days[len(days)-1])
	}
	if status.Card.From == 0 || len([]rune(status.Card.Text)) > protocol.MemoryCardMax {
		t.Fatalf("card %d facts, %d characters", status.Card.From, len([]rune(status.Card.Text)))
	}
}

func TestMemoryAddThenGet(t *testing.T) {
	s := &server{data: makeFixtures()}
	change := memoryCall[protocol.MemoryChange](t, s, protocol.MMemoryAdd, protocol.MemoryAddParams{Scope: protocol.MemoryYou, Text: "Keeps a spare charger in the car"})
	if len(change.Items) != 1 || change.Items[0].Op != protocol.MemoryOpRemembered {
		t.Fatalf("change %+v", change)
	}
	got := memoryCall[protocol.Memory](t, s, protocol.MMemoryGet, protocol.MemoryRefParams{ID: change.Items[0].ID})
	if got.Text != "Keeps a spare charger in the car" || got.Status != protocol.MemoryActive || got.Source.Kind != protocol.MemoryFromYou {
		t.Fatalf("got %+v", got)
	}
	if _, err := s.callMemory(protocol.MMemoryAdd, json.RawMessage(`{"scope":"you","text":"my password is hunter2"}`)); err == nil {
		t.Fatal("a secret was kept")
	}
	if len(s.takeMemoryEvents()) != 1 {
		t.Fatal("the add was not announced once")
	}
}

func TestMemoryEditKeepsVersion(t *testing.T) {
	s := &server{data: makeFixtures()}
	text := "Uses fish 4 as their login shell"
	memoryCall[protocol.MemoryChange](t, s, protocol.MMemoryEdit, protocol.MemoryEditParams{ID: "m1a4c7e2", Text: &text})
	got := memoryCall[protocol.Memory](t, s, protocol.MMemoryGet, protocol.MemoryRefParams{ID: "m1a4c7e2"})
	if got.Text != text || len(got.Versions) != 1 || got.Versions[0].Text != "Uses fish as their login shell" || got.Versions[0].By != "you" {
		t.Fatalf("got %+v", got)
	}
}

func TestMemoryForgetThenUndo(t *testing.T) {
	s := &server{data: makeFixtures()}
	change := memoryCall[protocol.MemoryChange](t, s, protocol.MMemoryForget, protocol.MemoryForgetParams{IDs: []string{"m5a7b2d9"}})
	if _, err := s.callMemory(protocol.MMemoryGet, json.RawMessage(`{"id":"m5a7b2d9"}`)); err == nil {
		t.Fatal("forgotten memory still there")
	}
	undone := memoryCall[protocol.MemoryChange](t, s, protocol.MMemoryUndo, protocol.MemoryChangeParams{Change: change.ID})
	if !undone.Undone {
		t.Fatal("undo did not mark the change")
	}
	got := memoryCall[protocol.Memory](t, s, protocol.MMemoryGet, protocol.MemoryRefParams{ID: "m5a7b2d9"})
	if len(got.Versions) != 2 || !got.Pinned {
		t.Fatalf("undo brought back %+v", got)
	}
	memoryCall[protocol.MemoryChange](t, s, protocol.MMemoryRedo, protocol.MemoryChangeParams{Change: change.ID})
	if _, err := s.callMemory(protocol.MMemoryGet, json.RawMessage(`{"id":"m5a7b2d9"}`)); err == nil {
		t.Fatal("redo left the memory")
	}
	learned := memoryCall[protocol.MemoryChange](t, s, protocol.MMemoryUndo, protocol.MemoryChangeParams{Change: "chg-demo-learned"})
	if !learned.Undone || memoryCall[protocol.Memory](t, s, protocol.MMemoryGet, protocol.MemoryRefParams{ID: "m7c2e9a4"}).Status != protocol.MemoryActive {
		t.Fatal("undoing the learning pass left the mainline kernel replaced")
	}
	changes := memoryCall[protocol.MemoryChangesResult](t, s, protocol.MMemoryChanges, protocol.MemoryChangesParams{IDs: []string{"chg-demo-told", "chg-demo-learned", "gone"}})
	if len(changes.Changes) != 2 || !changes.Changes[1].Undone {
		t.Fatalf("changes %+v", changes.Changes)
	}
	memoryCall[protocol.MemoryChange](t, s, protocol.MMemoryUndo, protocol.MemoryChangeParams{Change: "chg-demo-agent"})
	if _, err := s.callMemory(protocol.MMemoryGet, json.RawMessage(`{"id":"m5b9e3f6"}`)); err == nil {
		t.Fatal("undoing the agent's remember kept the memory")
	}
	memoryCall[protocol.MemoryChange](t, s, protocol.MMemoryRedo, protocol.MemoryChangeParams{Change: "chg-demo-agent"})
	if got := memoryList(t, s, protocol.MemoryListParams{Session: "agent-20261110-101500-b82d", Scope: demoMemoryHyperion, Query: "fixed clock"}); got.Total != 1 {
		t.Fatalf("redo found %d", got.Total)
	}
}

func TestMemoryKeepSuggestion(t *testing.T) {
	s := &server{data: makeFixtures()}
	memoryCall[protocol.MemoryChange](t, s, protocol.MMemoryKeep, protocol.MemoryIDsParams{IDs: []string{"m1e4a7b5"}})
	if got := memoryCall[protocol.Memory](t, s, protocol.MMemoryGet, protocol.MemoryRefParams{ID: "m1e4a7b5"}); got.Status != protocol.MemoryActive {
		t.Fatalf("kept suggestion is %s", got.Status)
	}
	if got := memoryList(t, s, protocol.MemoryListParams{Status: protocol.MemorySuggested}).Total; got != 4 {
		t.Fatalf("suggestions left %d", got)
	}
}

func TestMemoryImportSuggests(t *testing.T) {
	s := &server{data: makeFixtures()}
	change := memoryCall[protocol.MemoryChange](t, s, protocol.MMemoryImport, protocol.MemoryImportParams{Text: "- Likes tea\n\n2. Lives in Leeds\n* sk-abcdefghijklmnop\n", From: "ChatGPT"})
	if change.Origin != protocol.MemoryByImport || len(change.Items) != 2 || change.Items[0].Text != "Likes tea" || change.Items[1].Text != "Lives in Leeds" {
		t.Fatalf("change %+v", change)
	}
	got := memoryCall[protocol.Memory](t, s, protocol.MMemoryGet, protocol.MemoryRefParams{ID: change.Items[1].ID})
	if got.Status != protocol.MemorySuggested || got.Source.Kind != protocol.MemoryFromImport || got.Source.From != "ChatGPT" {
		t.Fatalf("got %+v", got)
	}
}

func TestMemoryForgetAll(t *testing.T) {
	s := &server{data: makeFixtures()}
	memoryCall[protocol.MemoryChange](t, s, protocol.MMemoryForgetAll, protocol.MemoryForgetAllParams{Scope: demoMemoryRyotunes})
	if got := memoryList(t, s, protocol.MemoryListParams{Scope: demoMemoryRyotunes}).Total; got != 0 {
		t.Fatalf("ryotunes kept %d", got)
	}
	memoryCall[protocol.MemoryChange](t, s, protocol.MMemoryForgetAll, protocol.MemoryForgetAllParams{})
	status := memoryCall[protocol.MemoryStatus](t, s, protocol.MMemoryStatus, nil)
	if counts := status.Counts; counts.All+counts.Suggested+counts.Replaced+counts.Blocked != 0 {
		t.Fatalf("left %+v", counts)
	}
}

func TestMemorySettingsAnnounce(t *testing.T) {
	s := &server{data: makeFixtures()}
	s.takeMemoryEvents()
	s.data.snapshot.Settings.Memory.Learn = false
	events := s.takeMemoryEvents()
	if len(events) != 1 || events[0].Event != protocol.EvtMemoryChanged {
		t.Fatalf("events %+v", events)
	}
	memoryCall[protocol.Empty](t, s, protocol.MMemoryEngineSetup, protocol.MemoryEngineParams{Engine: protocol.MemoryHindsight})
	if events := s.takeMemoryEvents(); len(events) != 4 || events[3].Event != protocol.EvtMemorySetup {
		t.Fatalf("setup events %d", len(events))
	}
}
