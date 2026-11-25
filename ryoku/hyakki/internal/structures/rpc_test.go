package structures

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func catalogueRPC(t *testing.T, store *Store, method string, params any) any {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Handle(store, method, raw, nil)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	return result
}

func TestEditedCloneDoesNotResurrectItsSourceExampleAfterRestore(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	cloned := catalogueRPC(t, store, protocol.MTeamsStructureClone, protocol.StructureCloneParams{ID: "pagan-council", NewID: "custom:small-council", Name: "Small council"}).(protocol.StructureTemplate)
	cloned.Seats = cloned.Seats[:5]
	cloned.MinSeats, cloned.MaxSeats, cloned.DefaultSeats = 5, 5, 5
	cloned.Seats[0].DefaultSpirit = ""
	cloned.Seats[0].Title = "My reviewer"
	saved := catalogueRPC(t, store, protocol.MTeamsStructureSave, protocol.StructureSaveParams{Template: cloned, ExpectedRevision: cloned.Revision}).(protocol.StructureTemplate)
	if saved.Example.Title != "" || len(saved.Example.Members) != 0 || len(saved.Example.Steps) != 0 {
		t.Fatal("edited recipe kept the original workflow identities")
	}
	catalogueRPC(t, store, protocol.MTeamsStructureDelete, protocol.StructureRef{ID: saved.ID, ExpectedRevision: saved.Revision})
	store, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	catalogue := catalogueRPC(t, store, protocol.MTeamsStructures, protocol.Empty{}).(protocol.TeamStructuresResult)
	if len(catalogue.Deleted) != 1 {
		t.Fatalf("deleted snapshots = %v", catalogue.Deleted)
	}
	restored := catalogueRPC(t, store, protocol.MTeamsStructureRestore, protocol.StructureRef{ID: saved.ID, ExpectedRevision: catalogue.Deleted[0].Revision}).(protocol.StructureTemplate)
	if !reflect.DeepEqual(restored.Seats, saved.Seats) || !reflect.DeepEqual(restored.Example, saved.Example) {
		t.Fatal("restore replaced edited content with the source recipe")
	}
	draft := catalogueRPC(t, store, protocol.MTeamsStructureDraft, protocol.StructureDraftParams{ID: restored.ID}).(protocol.StructureDraftResult)
	if len(draft.Members) != 5 || draft.Members[0].Name != "My reviewer" || draft.Members[0].Spirit != "" {
		t.Fatalf("draft resurrected source roster: %+v", draft.Members)
	}
	raw, err := json.Marshal(protocol.StructureSaveParams{Template: saved, ExpectedRevision: saved.Revision})
	if err != nil {
		t.Fatal(err)
	}
	_, err = Handle(store, protocol.MTeamsStructureSave, raw, nil)
	if protocol.AsError(err).Code != protocol.CodeConflict {
		t.Fatalf("pre-delete edit = %v", err)
	}
}

func TestCatalogueRejectsInvalidInputAndDoesNotMaskDiskFailures(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{`null`, `[]`, `{"id":"night-parade","count":2,"unknown":true}`, `{"id":"night-parade"} {}`, `{"id":"night-parade","count":-1}`} {
		_, err := Handle(store, protocol.MTeamsStructureDraft, json.RawMessage(raw), nil)
		if err == nil || protocol.AsError(err).Code != protocol.CodeBadRequest {
			t.Fatalf("accepted %s: %v", raw, err)
		}
	}
	changed := 0
	if _, err := Handle(store, protocol.MTeamsStructureDelete, json.RawMessage(`{"id":"night-parade","expectedRevision":1}`), func() { changed++ }); err != nil {
		t.Fatal(err)
	}
	_, err = Handle(store, protocol.MTeamsStructureDraft, json.RawMessage(`{"id":"night-parade"}`), nil)
	if err == nil || protocol.AsError(err).Code != protocol.CodeNotFound {
		t.Fatalf("hidden draft = %v", err)
	}
	if changed != 1 {
		t.Fatalf("successful mutation invalidations = %d", changed)
	}
	if _, err := Handle(store, protocol.MTeamsStructureDelete, json.RawMessage(`{"id":"night-parade","expectedRevision":1}`), func() { changed++ }); err == nil || protocol.AsError(err).Code != protocol.CodeConflict {
		t.Fatalf("stale deletion = %v", err)
	}
	if changed != 1 {
		t.Fatal("failed mutation emitted invalidation")
	}
	if err := os.WriteFile(filepath.Join(dir, "structures.json"), []byte(`{"version":1,"entries":[`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{protocol.MTeamsStructures, protocol.MTeamsStructureSuggest} {
		_, err := Handle(store, method, json.RawMessage(`{}`), nil)
		var wire *protocol.Error
		if err == nil || errors.As(err, &wire) {
			t.Fatalf("disk failure replaced by defaults/client error: %v", err)
		}
	}
}

func TestExplicitCountAdviceNeverExpandsOrShrinksTheRequestedRoster(t *testing.T) {
	for _, count := range []int{1, 2, 10, protocol.TeamsMembersMax} {
		for _, suggestion := range Suggest(protocol.StructureAnswers{Work: "build", Count: count}, Builtin()) {
			if suggestion.SeatCount != count || len(suggestion.Members) != count {
				t.Fatalf("requested %d, suggested %+v", count, suggestion)
			}
		}
	}
	court, _ := Lookup("goetia-court")
	if suggestions := Suggest(protocol.StructureAnswers{Work: "build", Count: 2}, []protocol.StructureTemplate{court}); len(suggestions) != 0 {
		t.Fatal("impossible exact count silently clamped")
	}
	court.Hidden = true
	if suggestions := Suggest(protocol.StructureAnswers{Work: "build"}, []protocol.StructureTemplate{court}); len(suggestions) != 0 {
		t.Fatal("hidden template returned as advice")
	}
}

func TestOversizedExpandedPreviewIsAnActionableErrorNotADroppedConnection(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	recipe, _ := Lookup("night-parade")
	recipe.ID, recipe.Builtin, recipe.Revision = "custom:escaped-parade", false, 0
	recipe.Example.Title = strings.Repeat("x", 4<<20)
	for i := 1; i < len(recipe.Seats); i++ {
		recipe.Seats[i].Duty = strings.Repeat("\x00", 16000)
		recipe.Seats[i].CapabilitiesHint = strings.Repeat("\x00", protocol.MemberCapabilitiesMaxRunes)
	}
	recipe, err = store.Save(recipe, 0)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(protocol.StructureDraftParams{ID: recipe.ID, Count: protocol.TeamsMembersMax})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Handle(store, protocol.MTeamsStructureDraft, raw, nil)
	if result != nil || err == nil || protocol.AsError(err).Code != protocol.CodeBadRequest {
		t.Fatalf("oversized draft did not return an RPC error: %T %v", result, err)
	}
}
