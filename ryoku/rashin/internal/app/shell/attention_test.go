package shell

import (
	"sort"
	"strings"
	"testing"

	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

func agents(sessions ...protocol.SessionInfo) state.Snapshot {
	snapshot := state.Snapshot{Connection: state.Ready, Sessions: make(map[string]protocol.SessionInfo)}
	for _, info := range sessions {
		info.Kind = protocol.KindAgent
		snapshot.Sessions[info.ID] = info
	}
	return snapshot
}

func titles(watch *attention, snapshot state.Snapshot) string {
	var said []string
	for _, notice := range watch.notices(snapshot) {
		said = append(said, notice.Key+": "+notice.Title)
	}
	sort.Strings(said)
	return strings.Join(said, "; ")
}

func TestAttentionSaysNothingAboutHowThingsStoodAtStart(t *testing.T) {
	watch := new(attention)
	waiting := protocol.SessionInfo{ID: "a", Title: "Add the panel", State: protocol.StateWaiting}
	if said := titles(watch, agents(waiting)); said != "" {
		t.Fatalf("a session that already needed the reader when Rashin started was announced: %s", said)
	}
	if said := titles(watch, agents(waiting)); said != "" {
		t.Fatalf("nothing changed and it said: %s", said)
	}
}

func TestAttentionTellsWhenAnAgentAsksFailsOrFinishes(t *testing.T) {
	watch := new(attention)
	working := func(id, title string) protocol.SessionInfo {
		return protocol.SessionInfo{ID: id, Title: title, State: protocol.StateWorking}
	}
	titles(watch, agents(working("a", "Add the panel"), working("b", "Fix the build"), working("c", "Write the docs")))

	asked := protocol.SessionInfo{ID: "a", Title: "Add the panel", State: protocol.StateWaiting}
	failed := protocol.SessionInfo{ID: "b", Title: "Fix the build", State: protocol.StateFailed, Error: "The harness stopped."}
	finished := protocol.SessionInfo{ID: "c", Title: "Write the docs", State: protocol.StateIdle}
	said := titles(watch, agents(asked, failed, finished))
	if said != "a: Add the panel needs you; b: Fix the build failed; c: Write the docs finished" {
		t.Fatalf("said %q", said)
	}
	// each is told once, not again with every later change
	if said := titles(watch, agents(asked, failed, finished)); said != "" {
		t.Fatalf("the same standing was announced twice: %s", said)
	}
}

func TestAttentionIgnoresQuestions(t *testing.T) {
	watch := new(attention)
	ask := protocol.SessionInfo{ID: "q", Kind: protocol.KindAsk, Title: "What is using my disk?", State: protocol.StateWorking}
	snapshot := state.Snapshot{Connection: state.Ready, Sessions: map[string]protocol.SessionInfo{"q": ask}}
	watch.notices(snapshot)
	ask.State = protocol.StateIdle
	snapshot.Sessions["q"] = ask
	if notices := watch.notices(snapshot); len(notices) != 0 {
		t.Fatalf("an answer the reader is waiting in front of was announced: %+v", notices)
	}
}

func TestAttentionTellsWhatATeamAsksAndReportsAsItsTeamAllows(t *testing.T) {
	watch := new(attention)
	snapshot := state.Snapshot{Connection: state.Ready, Teams: []protocol.Team{
		{ID: "research", Name: "Research desk", Notify: protocol.NotifyAll},
		{ID: "repo", Name: "Repo watch", Notify: protocol.NotifyNeeds},
	}}
	watch.notices(snapshot)
	report := &protocol.TeamReport{Title: "Open pull requests"}
	snapshot.Approvals = []protocol.Approval{{ID: "approval-1", Team: "research", Title: "Send the mail", Status: protocol.ApprovalPending}}
	snapshot.TeamTasks = map[string]protocol.TeamTask{
		"task-1": {ID: "task-1", Team: "repo", Status: protocol.TaskDone, Report: report},
		"task-2": {ID: "task-2", Team: "research", Status: protocol.TaskDone, Report: report},
	}
	said := titles(watch, snapshot)
	if said != "approval:approval-1: Research desk needs you: Send the mail; report:task-2: Research desk sent a report: Open pull requests" {
		t.Fatalf("said %q", said)
	}
	if said := titles(watch, snapshot); said != "" {
		t.Fatalf("the same approval and report were announced twice: %s", said)
	}
}
