package teams

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// What a former external harness left in the teams' state is cleaned when
// the state opens: a model id only a harness took, in teams and proposals,
// a task blocked on the harness (kept blocked with a reason to retry), its
// notices, and engine spend keyed by harness and model.
func TestWhatAFormerHarnessLeftIsCleanedOnOpen(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "tasks"), 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(name, data string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("state.json", `{
  "teams": [{"id": "team-a", "name": "Ops", "folder": "/tmp/ops", "members": [
    {"id": "m1", "name": "Ana", "lead": true, "harness": "claude", "model": "haiku", "options": {"effort": "high"}},
    {"id": "m2", "name": "Ben", "harness": "codex", "model": "openai/gpt"}
  ]}],
  "routines": [],
  "approvals": [
    {"id": "approval-r", "team": "team-a", "kind": "restore", "title": "Rashin put back a home", "status": "pending", "repeats": 2},
    {"id": "approval-t", "kind": "team", "title": "A team", "status": "pending", "proposal": {"team": {"name": "Desk", "members": [{"name": "Cy", "harness": "claude", "model": "sonnet"}]}}}
  ]
}`)
	write("tasks/task-b.json", `{"id": "task-b", "team": "team-a", "title": "Logs", "status": "blocked", "member": "m1",
  "reason": "This team is not signed in to Claude.", "signIn": "claude", "harness": "claude", "model": "haiku"}`)
	write("tasks/task-c.json", `{"id": "task-c", "team": "team-a", "title": "Disks", "status": "done", "member": "m2", "harness": "codex", "signIn": "", "model": "openai/gpt"}`)
	write("spend.json", `{"2026-11": {"team-a": {"days": {"2026-11-10": {"input": 10, "output": 5}},
  "engines": {"claude\u001fhaiku": 7, "codex\u001fopenai/gpt": 8, "openai/gpt": 1, "claude\u001f": 2}}}}`)

	st, err := openState(dir)
	if err != nil {
		t.Fatal(err)
	}
	team, _ := st.team("team-a")
	if team.Members[0].Model != "" || team.Members[0].Options["effort"] != "high" || team.Members[1].Model != "openai/gpt" {
		t.Fatalf("the members read %+v", team.Members)
	}
	if _, ok := st.approval("approval-r"); ok {
		t.Fatal("a restore notice survived")
	}
	if proposal, _ := st.approval("approval-t"); proposal.Proposal.Team.Members[0].Model != "" {
		t.Fatalf("the proposal keeps the model %q", proposal.Proposal.Team.Members[0].Model)
	}
	blocked, _ := st.task("task-b")
	if blocked.Status != protocol.TaskBlocked || blocked.Model != "" || !strings.Contains(blocked.Reason, "Retry it") {
		t.Fatalf("the blocked task reads %+v", blocked)
	}
	if done, _ := st.task("task-c"); done.Model != "openai/gpt" {
		t.Fatalf("a finished task lost its model: %+v", done)
	}
	for name, gone := range map[string]string{"state.json": `"restore"`, "tasks/task-b.json": `"signIn"`, "tasks/task-c.json": `"harness"`} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), gone) {
			t.Fatalf("%s still holds %s", name, gone)
		}
	}
	stateData, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stateData), `"harness"`) || strings.Contains(string(stateData), `"repeats"`) {
		t.Fatal("state.json retained obsolete engine fields")
	}

	st.spendMu.Lock()
	file, err := st.spendLocked()
	st.spendMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if engines := file["2026-11"]["team-a"].Engines; !maps.Equal(engines, map[string]int64{"openai/gpt": 9}) {
		t.Fatalf("the engines read %v, want the model's 9 tokens alone", engines)
	}
	data, err := os.ReadFile(filepath.Join(dir, "spend.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `\u001f`) {
		t.Fatal("spend.json still keys an engine by harness")
	}
}

func TestPendingFormerDreamStallCardsAreDroppedOnOpen(t *testing.T) {
	dir := t.TempDir()
	raw := `{"approvals":[` +
		`{"id":"old-stall","kind":"action","status":"pending","task":"task-a","action":{"dream":"stall"}},` +
		`{"id":"still-asked","kind":"action","status":"pending","task":"task-a","action":{"dream":"blocked"}},` +
		`{"id":"decided","kind":"action","status":"approved","task":"task-a","action":{"dream":"stall"}}]}`
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := openState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := st.approval("old-stall"); ok {
		t.Fatal("the obsolete pending stall question could still be answered")
	}
	for _, id := range []string{"still-asked", "decided"} {
		if _, ok := st.approval(id); !ok {
			t.Fatalf("unrelated approval %s was removed", id)
		}
	}
	reopened, err := openState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reopened.approval("old-stall"); ok {
		t.Fatal("the obsolete stall survived the migration on disk")
	}
}
