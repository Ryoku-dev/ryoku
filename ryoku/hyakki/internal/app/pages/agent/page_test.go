package agent

import (
	"testing"

	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

func TestSessionsAreGroupedByWhatTheyWantFromTheReader(t *testing.T) {
	store := state.New(nil)
	store.ApplySnapshot(protocol.Snapshot{
		Projects: []protocol.ProjectInfo{{ID: "p"}},
		Tasks:    []protocol.TaskInfo{{ID: "gate", Project: "p", Status: protocol.TaskNeedsYou}},
		Sessions: []protocol.SessionInfo{
			{ID: "decision", Kind: protocol.KindAgent, State: protocol.StateIdle, Project: "p", Worktree: "/w", Task: "gate"},
			{ID: "permission", Kind: protocol.KindAgent, State: protocol.StateWorking, Pending: 1},
			{ID: "failed", Kind: protocol.KindAgent, State: protocol.StateFailed},
			{ID: "working", Kind: protocol.KindAgent, State: protocol.StateWorking},
			{ID: "done", Kind: protocol.KindAgent, State: protocol.StateIdle, Unread: true},
			{ID: "idle", Kind: protocol.KindAgent, State: protocol.StateStopped},
			{ID: "ask", Kind: protocol.KindAsk, State: protocol.StateWorking},
		},
	})
	groups := groupSessions(store.Snapshot())
	want := map[group][]string{
		groupNeeds:   {"decision", "permission", "failed"},
		groupWorking: {"working"},
		groupDone:    {"done"},
		groupIdle:    {"idle"},
	}
	for at, ids := range want {
		got := make(map[string]bool, len(groups[at]))
		for _, info := range groups[at] {
			got[info.ID] = true
		}
		if len(got) != len(ids) {
			t.Fatalf("%s holds %v, want %v", at.label(), got, ids)
		}
		for _, id := range ids {
			if !got[id] {
				t.Fatalf("%s holds %v, want %v", at.label(), got, ids)
			}
		}
	}
}

func TestASessionBeingOpenedIsWaitedForUntilItIsListed(t *testing.T) {
	page := New()
	if page.lost(false) {
		t.Fatal("the board was treated as a lost session")
	}
	page.selected, page.opening = "agent-new", "agent-new"
	if page.lost(false) {
		t.Fatal("a session that was just made was dropped before it was listed")
	}
	if page.lost(true) || page.opening != "" {
		t.Fatal("a listed session must end the wait")
	}
	if !page.lost(false) {
		t.Fatal("a session that left the list after being shown was kept open")
	}
}
