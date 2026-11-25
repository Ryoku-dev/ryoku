package structures

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func customTree() protocol.StructureTemplate {
	return protocol.StructureTemplate{ID: "custom:release", Name: "Release", Shape: "deep", Reporting: protocol.ReportingTree, MinSeats: 3, MaxSeats: 3, DefaultSeats: 3,
		Seats: []protocol.StructureSeat{
			{ID: "checker", Title: "Checker", Rank: "reviewer", Duty: "Verify the change", ReportsToSeat: "builder"},
			{ID: "lead", Title: "Lead", Rank: "lead", Duty: "Hold the goal", Lead: true},
			{ID: "builder", Title: "Builder", Rank: "executor", Duty: "Implement the change", ReportsToSeat: "lead"},
		}}
}

func assertGraph(t *testing.T, members []protocol.Member, reporting string) {
	t.Helper()
	index := make(map[string]protocol.Member)
	for _, m := range members {
		if m.ID == "" {
			t.Fatal("missing member ID")
		}
		if _, exists := index[m.ID]; exists {
			t.Fatalf("duplicate member %s", m.ID)
		}
		index[m.ID] = m
	}
	for _, start := range members {
		seen := make(map[string]bool)
		for m := start; ; {
			if seen[m.ID] {
				t.Fatalf("cycle from %s", start.ID)
			}
			seen[m.ID] = true
			if m.ReportsTo == "" {
				if reporting == protocol.ReportingTree && !m.Lead {
					t.Fatalf("%s fails to reach lead", start.ID)
				}
				break
			}
			next, exists := index[m.ReportsTo]
			if !exists {
				t.Fatalf("dangling manager %s", m.ReportsTo)
			}
			m = next
		}
	}
}

func TestEveryBuiltinSelectableRosterIsConnected(t *testing.T) {
	for _, recipe := range Builtin() {
		t.Run(recipe.ID, func(t *testing.T) {
			for count := recipe.MinSeats; count <= recipe.MaxSeats; count++ {
				members, err := Instantiate(recipe, count)
				if err != nil {
					t.Fatalf("count %d: %v", count, err)
				}
				if len(members) != count {
					t.Fatalf("count %d produced %d", count, len(members))
				}
				assertGraph(t, members, recipe.Reporting)
			}
			if _, err := Instantiate(recipe, recipe.MaxSeats+1); err == nil {
				t.Fatal("accepted oversized roster")
			}
			if _, err := Instantiate(recipe, -1); err == nil {
				t.Fatal("accepted negative count")
			}
			assertGraph(t, recipe.Example.Members, recipe.Reporting)
			ids := make(map[string]bool)
			for _, m := range recipe.Example.Members {
				ids[m.ID] = true
			}
			for _, step := range recipe.Example.Steps {
				for _, id := range step.Members {
					if !ids[id] {
						t.Fatalf("example action references absent member %s", id)
					}
				}
			}
		})
	}
}

func TestCustomGraphAcceptsReorderingRejectsCyclesAndDanglingManagers(t *testing.T) {
	template := customTree()
	members, err := Instantiate(template, 0)
	if err != nil {
		t.Fatal(err)
	}
	assertGraph(t, members, template.Reporting)
	for _, mutate := range []func(*protocol.StructureTemplate){
		func(v *protocol.StructureTemplate) { v.Seats[0].ReportsToSeat = "absent" },
		func(v *protocol.StructureTemplate) { v.Seats[0].ReportsToSeat = "checker" },
		func(v *protocol.StructureTemplate) { v.Seats[2].ReportsToSeat = "checker" },
		func(v *protocol.StructureTemplate) { v.Seats[2].ReportsToSeat = "" },
		func(v *protocol.StructureTemplate) { v.MinSeats = 1 },
	} {
		v := cloneTemplate(template)
		mutate(&v)
		if err := Validate(v); err == nil {
			t.Fatal("accepted invalid graph")
		}
	}
	// The marshal's ID must never collide with an expanded worker prototype.
	v, _ := Lookup("night-parade")
	v.Seats[0].ID = "kitsune-001"
	for i := 1; i < len(v.Seats); i++ {
		v.Seats[i].ReportsToSeat = v.Seats[0].ID
	}
	members, err = Instantiate(v, v.MaxSeats)
	if err != nil {
		t.Fatal(err)
	}
	assertGraph(t, members, v.Reporting)
}

func TestCustomRosterNamesSurviveMemberNormalization(t *testing.T) {
	fixed := protocol.StructureTemplate{
		ID: "custom:colliding-names", Name: "Colliding names", Shape: "flat",
		Reporting: protocol.ReportingPeers, MinSeats: 7, MaxSeats: 7, DefaultSeats: 7,
		Seats: []protocol.StructureSeat{
			{ID: "a", Title: "Worker", Rank: "worker", Duty: "Review first batch"},
			{ID: "b", Title: "Worker", Rank: "worker", Duty: "Review second batch"},
			{ID: "c", Title: "Worker-2", Rank: "worker", Duty: "Review third batch"},
			{ID: "d", Title: "worker", Rank: "worker", Duty: "Review fourth batch"},
			{ID: "e", Title: " Worker ", Rank: "worker", Duty: "Review fifth batch"},
			{ID: "f", Title: " worker-2 ", Rank: "worker", Duty: "Review sixth batch"},
			{ID: "g", Title: "Worker-2-2", Rank: "worker", Duty: "Review seventh batch"},
		},
	}
	variable := protocol.StructureTemplate{
		ID: "custom:colliding-parade", Name: "Colliding parade", Shape: "horde",
		Reporting: protocol.ReportingTree, Variable: true, MinSeats: 2, MaxSeats: 3, DefaultSeats: 3,
		Seats: []protocol.StructureSeat{
			{ID: "marshal", Title: " WORKER-1 ", Rank: "marshal", Duty: "Coordinate batches", Lead: true},
			{ID: "worker", Title: " worker ", Rank: "worker", Duty: "Review a batch", ReportsToSeat: "marshal"},
		},
	}
	for _, tc := range []struct {
		template protocol.StructureTemplate
		want     []string
	}{
		{fixed, []string{"Worker", "Worker-2", "Worker-2-2", "worker-3", "Worker-4", "worker-2-3", "Worker-2-2-2"}},
		{variable, []string{"WORKER-1", "worker-1-2", "worker-2"}},
	} {
		t.Run(tc.template.ID, func(t *testing.T) {
			members, err := Instantiate(tc.template, 0)
			if err != nil {
				t.Fatal(err)
			}
			assertGraph(t, members, tc.template.Reporting)
			got := make([]string, len(members))
			seen := make(map[string]bool)
			for i, member := range members {
				got[i] = member.Name
				trimmed := strings.TrimSpace(member.Name)
				if trimmed == "" || trimmed != member.Name {
					t.Fatalf("unnormalized name %q", member.Name)
				}
				key := strings.ToLower(trimmed)
				if seen[key] {
					t.Fatalf("member normalization would reject duplicate %q", member.Name)
				}
				seen[key] = true
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("names = %v, want %v", got, tc.want)
			}
			again, err := Instantiate(tc.template, 0)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(members, again) {
				t.Fatal("name allocation is not deterministic")
			}
		})
	}
}

func TestStoreRestartsAndRejectsLostUpdatesAcrossHandles(t *testing.T) {
	dir := t.TempDir()
	a, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	created, err := a.Save(customTree(), 0)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, store := range []*Store{a, b} {
		wg.Add(1)
		go func(store *Store) {
			defer wg.Done()
			edit := cloneTemplate(created)
			edit.Name = "Revised release"
			_, err := store.Save(edit, created.Revision)
			results <- err
		}(store)
	}
	wg.Wait()
	close(results)
	success, stale := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrStaleRevision) {
			stale++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || stale != 1 {
		t.Fatalf("success=%d stale=%d", success, stale)
	}
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	list, err := reopened.List()
	if err != nil {
		t.Fatal(err)
	}
	var found protocol.StructureTemplate
	for _, v := range list {
		if v.ID == created.ID {
			found = v
		}
	}
	if found.Revision != created.Revision+1 || found.Name != "Revised release" {
		t.Fatalf("edit did not survive restart: %+v", found)
	}
	assertGraph(t, mustInstantiate(t, found), found.Reporting)
}

func mustInstantiate(t *testing.T, v protocol.StructureTemplate) []protocol.Member {
	t.Helper()
	members, err := Instantiate(v, 0)
	if err != nil {
		t.Fatal(err)
	}
	return members
}

func TestDeleteRestorePreventsABAAndRetainsDiscoverableSnapshot(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.Save(customTree(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(v.ID, v.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(customTree(), 0); !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("deleted ID recreated: %v", err)
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	deleted, err := s.ListDeleted()
	if err != nil || len(deleted) != 1 {
		t.Fatalf("missing restored tombstone: %v %v", deleted, err)
	}
	if _, err := s.Restore(v.ID, v.Revision); !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("stale restore accepted: %v", err)
	}
	restored, err := s.Restore(v.ID, deleted[0].Revision)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored.Seats, v.Seats) || restored.Revision <= deleted[0].Revision || restored.Hidden {
		t.Fatal("restore lost content or monotonic revision")
	}
	if _, err := s.Save(v, v.Revision); !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("pre-delete editor overwrote restore: %v", err)
	}
}

func TestDefaultVisibilityAndClonePersistWithoutMutatingDefaults(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	v, _ := Lookup("goetia-court")
	if _, err := s.Save(v, v.Revision); !errors.Is(err, ErrImmutable) {
		t.Fatalf("default saved: %v", err)
	}
	if err := s.Hide(v.ID, v.Revision); err != nil {
		t.Fatal(err)
	}
	cloned, err := s.Clone(v.ID, "custom:my-court", "My court")
	if err != nil {
		t.Fatal(err)
	}
	cloned.Seats[0].Duty = "A changed role"
	cloned.Example.Members[0].Name = "A local name"
	if _, err := s.Save(cloned, cloned.Revision); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	var hidden protocol.StructureTemplate
	for _, template := range list {
		if template.ID == v.ID {
			hidden = template
		}
	}
	if !hidden.Hidden || !reflect.DeepEqual(hidden.Seats, v.Seats) {
		t.Fatal("hiding/cloning corrupted default")
	}
	if _, err := s.Restore(v.ID, hidden.Revision); err != nil {
		t.Fatal(err)
	}
	// Caller-owned returned slices must never mutate future catalog reads.
	v.Seats[0].Duty = "corruption"
	v.Example.Steps[0].Members[0] = "missing"
	fresh, _ := Lookup(v.ID)
	assertGraph(t, mustInstantiate(t, fresh), fresh.Reporting)
	if fresh.Seats[0].Duty == "corruption" || fresh.Example.Steps[0].Members[0] == "missing" {
		t.Fatal("catalog aliases caller values")
	}
}

func TestStoreFailsClosedOnCorruptionAndFailedCommit(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	// An unusable on-disk state must fail saving without inventing a revision.
	if err := os.Mkdir(filepath.Join(dir, "structures.json"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(customTree(), 0); err == nil {
		t.Fatal("reported successful persistence over a directory")
	}
	if err := os.Remove(filepath.Join(dir, "structures.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(customTree(), 0); err != nil {
		t.Fatalf("failed save leaked an in-memory revision: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "structures.json"), []byte(`{"version":1,"entries":[`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err == nil {
		t.Fatal("silently replaced corrupted state")
	}
	if _, err := s.List(); err == nil {
		t.Fatal("cached successful state concealed disk corruption")
	}
}
