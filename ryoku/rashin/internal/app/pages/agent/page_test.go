package agent

import (
	"testing"

	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

func TestGroupSessionsUsesNeedsWorkingDoneIdleOrder(t *testing.T) {
	store := state.New(nil)
	sessions := []protocol.SessionInfo{
		{ID: "needs", Kind: protocol.KindAgent, State: protocol.StateIdle, Project: "p", Worktree: "/w", Task: "gate"},
		{ID: "working", Kind: protocol.KindAgent, State: protocol.StateWorking},
		{ID: "done", Kind: protocol.KindAgent, State: protocol.StateIdle, Unread: true},
		{ID: "idle", Kind: protocol.KindAgent, State: protocol.StateStopped},
	}
	store.ApplySnapshot(protocol.Snapshot{Sessions: sessions, Tasks: []protocol.TaskInfo{{ID: "gate", Project: "p", Status: protocol.TaskNeedsYou}}, Projects: []protocol.ProjectInfo{{ID: "p"}}})
	groups := groupSessions(store.Snapshot())
	if len(groups) != 4 {
		t.Fatalf("groups = %d, want 4", len(groups))
	}
	want := []struct{ id, session string }{{groupNeeds, "needs"}, {groupWorking, "working"}, {groupDone, "done"}, {groupIdle, "idle"}}
	for index, expected := range want {
		if groups[index].id != expected.id || len(groups[index].sessions) != 1 || groups[index].sessions[0] != expected.session {
			t.Fatalf("group %d = %#v, want %s/%s", index, groups[index], expected.id, expected.session)
		}
	}
}
