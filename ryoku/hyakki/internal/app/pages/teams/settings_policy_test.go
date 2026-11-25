package teams

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

func settingsTree() protocol.Team {
	return protocol.Team{ID: "team", Reporting: protocol.ReportingTree, Members: []protocol.Member{
		{ID: "root", Name: "Root", Lead: true, Revision: 4},
		{ID: "manager", Name: "Manager", ReportsTo: "root", Revision: 7},
		{ID: "worker", Name: "Worker", ReportsTo: "manager", Revision: 2},
		{ID: "reviewer", Name: "Reviewer", ReportsTo: "manager", Revision: 3},
	}}
}

func TestSettingsRootReplacementDoesNotMutateSavedGraph(t *testing.T) {
	before := settingsTree()
	draft := editableTeam(before)
	replaceRoot(&draft, "worker")
	if err := protocol.ValidateReporting(draft); err != nil {
		t.Fatal(err)
	}
	if graphRoot(before) != "root" || before.Members[0].ReportsTo != "" || before.Members[2].ReportsTo != "manager" {
		t.Fatal("editing leaked into the saved graph")
	}
	if graphRoot(draft) != "worker" || draft.Members[0].ReportsTo != "worker" || draft.Members[2].ReportsTo != "" {
		t.Fatalf("replacement lost reporting: %+v", draft.Members)
	}
	for i, m := range draft.Members {
		if m.Revision != before.Members[i].Revision {
			t.Fatal("a draft invented canonical revisions")
		}
	}
	if review := graphChanges(before, draft); !strings.Contains(review, "Root") || !strings.Contains(review, "Worker") {
		t.Fatal("root replacement review omitted affected spirits")
	}
}

func TestSettingsDragRejectsCycleWithoutChangingDraft(t *testing.T) {
	d := reportingDraft{team: settingsTree()}
	before := editableTeam(d.team)
	d.reparent(1, 2)
	if d.problem == "" || !reflect.DeepEqual(d.team, before) {
		t.Fatal("invalid drag changed the graph or did not report a cycle")
	}
	d.reparent(2, 0)
	if d.problem != "" || d.team.Members[2].ReportsTo != "root" || d.team.Members[3].ReportsTo != "manager" {
		t.Fatal("valid drag did not move exactly the selected reporting line")
	}
}

func TestSettingsPeersNeverAcquireAnInventedManagerOrLeader(t *testing.T) {
	tm := protocol.Team{ID: "peers", Reporting: protocol.ReportingPeers, Members: []protocol.Member{{ID: "one", Name: "One"}, {ID: "two", Name: "Two"}}}
	d := reportingDraft{team: editableTeam(tm)}
	d.reparent(1, 0)
	if d.problem == "" || !reflect.DeepEqual(d.team, tm) {
		t.Fatal("drag assigned a manager in peer mode")
	}
	remove := dropMember{team: tm, member: tm.Members[0]}
	remaining := remove.removal()
	if err := protocol.ValidateReporting(remaining); err != nil {
		t.Fatal(err)
	}
	if graphRoot(remaining) != "" || len(remaining.Members) != 1 || remaining.Members[0].ID != "two" {
		t.Fatal("peer removal promoted or respawned a spirit")
	}
}

func TestSettingsManagerRemovalPromotesReviewedDirectReport(t *testing.T) {
	tm := settingsTree()
	d := dropMember{team: tm, member: tm.Members[1], replacement: "worker"}
	after := d.removal()
	if err := protocol.ValidateReporting(after); err != nil {
		t.Fatal(err)
	}
	worker, _ := protocol.MemberByID(after.Members, "worker")
	reviewer, _ := protocol.MemberByID(after.Members, "reviewer")
	if worker.ReportsTo != "root" || reviewer.ReportsTo != "worker" {
		t.Fatal("reviewed replacement introduced a self-manager or lost direct reports")
	}
	if _, exists := protocol.MemberByID(after.Members, "manager"); exists {
		t.Fatal("removed manager was recreated")
	}
}

func TestSettingsDetailsPreserveInheritanceVersusExplicitEmptyPolicy(t *testing.T) {
	var form memberDetails
	m := protocol.Member{ID: "spirit", Name: "Spirit", Revision: 8}
	form.set(m)
	if err := form.apply(&m); err != nil {
		t.Fatal(err)
	}
	if m.Tools != nil || m.Skills != nil || m.Budget != nil || m.Runtime != nil || m.Heartbeat != nil || m.Revision != 8 {
		t.Fatal("merely opening details materialized inherited policies")
	}
	form.toolsOn.Value, form.skillsOn.Value, form.heartbeatOn.Value = true, true, true
	for i := range form.wakes {
		form.wakes[i].Value = false
	}
	if err := form.apply(&m); err != nil {
		t.Fatal(err)
	}
	if m.Tools == nil || m.Skills == nil || len(*m.Skills) != 0 || m.Heartbeat == nil || len(m.Heartbeat.Modes) != 0 {
		t.Fatal("explicit empty restrictions were collapsed into inheritance")
	}
}

func TestSettingsRuntimeArgumentVectorAndVaultReferencesRoundTrip(t *testing.T) {
	original := protocol.Member{ID: "spirit", Name: "Spirit", Revision: 9, Runtime: &protocol.MemberRuntime{Kind: protocol.MemberRuntimeCommand, Command: []string{"tool", "a b", "", "--literal=$HOME"}, Env: map[string]string{"PUBLIC": "not a key"}, SecretEnv: map[string]string{"TOKEN": "existing-vault-entry"}}}
	var form memberDetails
	form.set(original)
	got := original
	if err := form.apply(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Runtime, original.Runtime) {
		t.Fatal("runtime form split argv, lost a vault reference or changed public environment")
	}
	form.argv.SetText("tool --not-a-json-array")
	if err := form.apply(&got); err == nil {
		t.Fatal("a shell fragment was accepted as argv")
	}
}

func TestSettingsRefreshKeepsDirtyFieldsAndAdvancesCleanBaselines(t *testing.T) {
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	first := protocol.Team{ID: "team", Name: "Original", Purpose: "Goal", Folder: "/work", Updated: now, Budget: protocol.TeamBudget{MonthTokens: 100000, TaskTurns: 10}}
	var s settingsTab
	s.fill(first)
	s.name.SetText("Local draft")
	s.limit.Value = 200000
	remote := first
	remote.Name, remote.Budget.MonthTokens, remote.Updated = "Remote edit", 300000, now.Add(time.Second)
	s.fill(remote)
	if s.name.Text() != "Local draft" || s.limit.Value != 200000 {
		t.Fatal("an asynchronous refresh discarded dirty form edits")
	}
	if !s.identityUpdated.Equal(now) || !s.budgetUpdated.Equal(now) {
		t.Fatal("dirty fields were rebound to a newer save authority")
	}
	committed := remote
	committed.Name, committed.Budget.MonthTokens, committed.Updated = "Local draft", 200000, now.Add(2*time.Second)
	s.fill(committed)
	if !s.identityUpdated.Equal(committed.Updated) || !s.budgetUpdated.Equal(committed.Updated) {
		t.Fatal("successful edits left the next save bound to a stale baseline")
	}
	next := committed
	next.Name, next.Updated = "Later saved name", now.Add(3*time.Second)
	s.fill(next)
	if s.name.Text() != next.Name {
		t.Fatal("clean fields stopped following canonical state after a successful save")
	}
}

func TestIdentitySaveRecognizesAbsoluteAndHomeRelativeFolder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	first := protocol.Team{ID: "folder-team", Name: "Folder team", Purpose: "Keep reviewed settings", Folder: home + "/old", Updated: now}
	var form settingsTab
	form.fill(first)
	form.folder.SetText(home + "/new")
	committed := first
	committed.Folder, committed.Updated = home+"/new", now.Add(time.Second)
	form.fill(committed)
	form.name.SetText("Next independent edit")
	if !form.identityUpdated.Equal(committed.Updated) {
		t.Fatal("successful absolute folder save left the next edit bound to a stale version")
	}
	if form.identity().name != "Next independent edit" || form.identity().folder != "~/new" {
		t.Fatal("recognizing the saved folder lost the next edit or its canonical path")
	}
	remote := committed
	remote.Name, remote.Updated = "A concurrent change", now.Add(2*time.Second)
	form.fill(remote)
	if form.identity().name != "Next independent edit" || !form.identityUpdated.Equal(committed.Updated) {
		t.Fatal("concurrent refresh discarded or rebound an unsaved follow-up edit")
	}
}
