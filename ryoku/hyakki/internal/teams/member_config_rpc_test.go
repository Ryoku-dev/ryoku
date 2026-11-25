package teams

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"ryoku-rashin/internal/memberconfig"
	"ryoku-rashin/internal/protocol"
)

func configRef(team protocol.Team, index int) protocol.MemberConfigRef {
	return protocol.MemberConfigRef{Team: team.ID, Member: team.Members[index].ID}
}

func configDocument(t *testing.T, h *harness, ref protocol.MemberConfigRef) protocol.MemberConfigDocument {
	t.Helper()
	return do[protocol.MemberConfigDocument](t, h, protocol.MTeamsMemberConfigGet, ref)
}

func configTitle(text, title string) string {
	// Fixture members have an empty title, encoded as title: "".
	return strings.Replace(text, `title: ""`, "title: "+title, 1)
}

func TestMemberConfigConcurrentSavesAndStaleRestoreKeepCommittedHistory(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Concurrent configuration", nil)
	ref := configRef(team, 1)
	doc := configDocument(t, h, ref)
	start := make(chan struct{})
	results := make(chan error, 2)
	var group sync.WaitGroup
	for _, title := range []string{"First", "Second"} {
		group.Add(1)
		go func(title string) {
			defer group.Done()
			<-start
			_, err := h.call(protocol.MTeamsMemberConfigSave, protocol.MemberConfigSaveParams{Team: ref.Team, Member: ref.Member, YAML: configTitle(doc.YAML, title), ExpectedRevision: doc.Revision})
			results <- err
		}(title)
	}
	close(start)
	group.Wait()
	close(results)
	passed, failed := 0, 0
	for err := range results {
		if err == nil {
			passed++
		} else {
			failed++
		}
	}
	if passed != 1 || failed != 1 {
		t.Fatalf("concurrent saves: %d accepted, %d refused", passed, failed)
	}
	current := configDocument(t, h, ref)
	if current.Revision != doc.Revision+1 {
		t.Fatal("concurrent saves did not create exactly one new revision")
	}
	if _, err := h.call(protocol.MTeamsMemberConfigRestore, protocol.MemberConfigRestoreParams{Team: ref.Team, Member: ref.Member, Revision: doc.Revision, ExpectedRevision: doc.Revision}); err == nil {
		t.Fatal("stale restore succeeded")
	}
	if after := configDocument(t, h, ref); after != current {
		t.Fatal("stale restore changed configuration")
	}
	restored := do[protocol.MemberConfigDocument](t, h, protocol.MTeamsMemberConfigRestore, protocol.MemberConfigRestoreParams{Team: ref.Team, Member: ref.Member, Revision: doc.Revision, ExpectedRevision: current.Revision})
	if restored.Revision != current.Revision+1 {
		t.Fatal("restore did not create a new revision")
	}
	history := do[protocol.MemberConfigHistoryResult](t, h, protocol.MTeamsMemberConfigHistory, ref)
	if len(history.Revisions) != 3 || !history.Revisions[0].Current || history.Revisions[2].Revision != doc.Revision {
		t.Fatal("revision history lost a committed state")
	}
}

func TestMemberConfigFormEditsPersistAndRetainBoundedHistory(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Form configuration", nil)
	ref := configRef(team, 1)
	for range protocol.MemberConfigHistoryLimit + 3 {
		team.Members[1].Title += "x"
		team = do[protocol.Team](t, h, protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: team.ID, Patch: mustJSON(t, map[string]any{"members": team.Members})})
	}
	doc := configDocument(t, h, ref)
	history := do[protocol.MemberConfigHistoryResult](t, h, protocol.MTeamsMemberConfigHistory, ref)
	if len(history.Revisions) != protocol.MemberConfigHistoryLimit || history.RetentionLimit != protocol.MemberConfigHistoryLimit {
		t.Fatal("history retention is not explicit and bounded")
	}
	if history.Revisions[0].Revision != doc.Revision || history.Revisions[1].Revision != doc.Revision-1 {
		t.Fatal("current/prior revisions not retained")
	}
	restartHeartbeatHarness(t, h)
	if reopened := configDocument(t, h, ref); reopened != doc {
		t.Fatal("reopen changed persisted configuration")
	}
	if reopened := do[protocol.MemberConfigHistoryResult](t, h, protocol.MTeamsMemberConfigHistory, ref); !reflect.DeepEqual(reopened, history) {
		t.Fatal("reopen changed persisted history")
	}
	if _, err := h.call(protocol.MTeamsMemberConfigGet, protocol.MemberConfigRef{Team: ref.Team, Member: ref.Member, Revision: 1}); err == nil {
		t.Fatal("pruned history manufactured a snapshot")
	}
}

func TestMemberConfigMasksPreserveDeleteAndNeverRevealValues(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Masked configuration", nil)
	credential := runtimeRecord(t, h, protocol.SessionInfo{Team: team.ID}, protocol.CredentialRuntime)
	team.Members[1].Runtime = &protocol.MemberRuntime{Kind: protocol.MemberRuntimeCommand, Command: []string{"fixture"}, SecretEnv: map[string]string{"PROCESS_KEY": credential.ID}}
	team = do[protocol.Team](t, h, protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: team.ID, Patch: mustJSON(t, map[string]any{"members": team.Members})})
	ref := configRef(team, 1)
	doc := configDocument(t, h, ref)
	if !strings.Contains(doc.YAML, memberconfig.Mask) || strings.Contains(doc.YAML, credential.ID) || strings.Contains(doc.YAML, credential.Name) || strings.Contains(doc.YAML, runtimeFixtureSecret) {
		t.Fatal("document did not mask the vault reference")
	}
	saved := do[protocol.MemberConfigDocument](t, h, protocol.MTeamsMemberConfigSave, protocol.MemberConfigSaveParams{Team: ref.Team, Member: ref.Member, YAML: configTitle(doc.YAML, "Reader"), ExpectedRevision: doc.Revision})
	live, _ := h.s.state.team(team.ID)
	if live.Members[1].Runtime.SecretEnv["PROCESS_KEY"] != credential.ID {
		t.Fatal("unchanged mask lost its exact reference")
	}
	member, err := memberconfig.Decode(saved.YAML, team.ID, ref.Member, saved.Revision, live.Members[1])
	if err != nil {
		t.Fatal(err)
	}
	member.Runtime.SecretEnv = nil
	del, err := memberconfig.Encode(team.ID, memberconfig.Snapshot{Member: member, At: live.Updated}, member.Revision)
	if err != nil {
		t.Fatal(err)
	}
	deleted := do[protocol.MemberConfigDocument](t, h, protocol.MTeamsMemberConfigSave, protocol.MemberConfigSaveParams{Team: ref.Team, Member: ref.Member, YAML: del.YAML, ExpectedRevision: saved.Revision})
	live, _ = h.s.state.team(team.ID)
	if len(live.Members[1].Runtime.SecretEnv) != 0 {
		t.Fatal("deleting the field did not revoke the reference")
	}
	forged := strings.Replace(deleted.YAML, "kind: command", "kind: command\n    secretEnv:\n      PROCESS_KEY: '<vault-reference>'", 1)
	if _, err := h.call(protocol.MTeamsMemberConfigSave, protocol.MemberConfigSaveParams{Team: ref.Team, Member: ref.Member, YAML: forged, ExpectedRevision: deleted.Revision}); err == nil {
		t.Fatal("mask without a current binding succeeded")
	}
	if _, err := h.s.vault.Update(protocol.VaultUpdateParams{ID: credential.ID, Patch: json.RawMessage(`{"teams":[]}`)}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.call(protocol.MTeamsMemberConfigRestore, protocol.MemberConfigRestoreParams{Team: ref.Team, Member: ref.Member, Revision: saved.Revision, ExpectedRevision: deleted.Revision}); err == nil {
		t.Fatal("restore silently revived a revoked grant")
	}
	raw, err := os.ReadFile(filepath.Join(h.dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), runtimeFixtureSecret) {
		t.Fatal("history persisted a vault value")
	}
}

func TestMemberConfigRestoreRevalidatesGraphAndNeverResurrectsDeletedSeats(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Historical graph", func(p *protocol.TeamCreateParams) {
		p.Reporting = protocol.ReportingTree
		p.Members = []protocol.Member{{ID: "root", Name: "Root", Lead: true}, {ID: "manager", Name: "Manager", ReportsTo: "root"}, {ID: "worker", Name: "Worker", ReportsTo: "manager"}}
	})
	ref := configRef(team, 2)
	original := configDocument(t, h, ref)
	removedID := team.Members[1].ID
	team.Members[2].ReportsTo = team.Members[0].ID
	team.Members = append(team.Members[:1:1], team.Members[2])
	team = do[protocol.Team](t, h, protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: team.ID, Patch: mustJSON(t, map[string]any{"members": team.Members})})
	current := configDocument(t, h, ref)
	if _, err := h.call(protocol.MTeamsMemberConfigRestore, protocol.MemberConfigRestoreParams{Team: ref.Team, Member: ref.Member, Revision: original.Revision, ExpectedRevision: current.Revision}); err == nil {
		t.Fatal("restored a missing reporting manager")
	}
	if after := configDocument(t, h, ref); after != current {
		t.Fatal("invalid graph restore altered current configuration")
	}
	if _, err := h.call(protocol.MTeamsMemberConfigRestore, protocol.MemberConfigRestoreParams{Team: ref.Team, Member: removedID, Revision: 1, ExpectedRevision: 1}); err == nil {
		t.Fatal("restored a deleted member")
	}
}

func TestMemberConfigInvalidDocumentsAndDiskFailureAreAtomic(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Atomic configuration", nil)
	ref := configRef(team, 1)
	doc := configDocument(t, h, ref)
	for _, text := range []string{
		strings.Replace(doc.YAML, "team: "+team.ID, "team: other", 1),
		strings.Replace(doc.YAML, "id: "+ref.Member, "id: other", 1),
		doc.YAML + "---\nversion: 1\n",
		doc.YAML + "unknownField: denied\n",
		strings.Replace(doc.YAML, "version: 1", "version: 2", 1),
		strings.Replace(doc.YAML, "version: 1", "version: &anchor 1", 1),
		doc.YAML + "version: 1\n",
	} {
		if _, err := h.call(protocol.MTeamsMemberConfigSave, protocol.MemberConfigSaveParams{Team: ref.Team, Member: ref.Member, YAML: text, ExpectedRevision: doc.Revision}); err == nil {
			t.Fatal("invalid document was accepted")
		}
	}
	beforeHistory := do[protocol.MemberConfigHistoryResult](t, h, protocol.MTeamsMemberConfigHistory, ref)
	// A nonempty directory at the atomic replacement target deterministically
	// refuses rename, including when tests run with elevated privileges.
	path := filepath.Join(h.dir, "state.json")
	backup := filepath.Join(h.dir, "saved-state.json")
	if err := os.Rename(path, backup); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "block"), []byte("block"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, saveErr := h.call(protocol.MTeamsMemberConfigSave, protocol.MemberConfigSaveParams{Team: ref.Team, Member: ref.Member, YAML: configTitle(doc.YAML, "MustNotCommit"), ExpectedRevision: doc.Revision})
	if saveErr == nil {
		t.Fatal("failed disk transaction reported success")
	}
	if _, err := h.call(protocol.MTeamsMemberConfigRestore, protocol.MemberConfigRestoreParams{Team: ref.Team, Member: ref.Member, Revision: doc.Revision, ExpectedRevision: doc.Revision}); err == nil {
		t.Fatal("failed disk restore reported success")
	}
	if after := configDocument(t, h, ref); after != doc {
		t.Fatal("failed disk transaction changed canonical state")
	}
	if after := do[protocol.MemberConfigHistoryResult](t, h, protocol.MTeamsMemberConfigHistory, ref); !reflect.DeepEqual(after, beforeHistory) {
		t.Fatal("failed disk transaction changed history")
	}
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(backup, path); err != nil {
		t.Fatal(err)
	}
	restartHeartbeatHarness(t, h)
	if after := configDocument(t, h, ref); after != doc {
		t.Fatal("failed transaction survived reopen")
	}
	for _, method := range []string{protocol.MTeamsMemberConfigGet, protocol.MTeamsMemberConfigSave, protocol.MTeamsMemberConfigHistory, protocol.MTeamsMemberConfigRestore} {
		ctx := context.WithValue(context.Background(), agentSession{}, "forged-agent")
		if _, err := h.s.Handle(ctx, method, json.RawMessage(`{}`)); err == nil {
			t.Fatal("agent callback accessed owner configuration")
		}
	}
}

func TestMemberConfigConcurrentRestoresCreateExactlyOneNewRevision(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Concurrent restore", nil)
	ref := configRef(team, 1)
	doc := configDocument(t, h, ref)
	start := make(chan struct{})
	results := make(chan error, 2)
	var group sync.WaitGroup
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			_, err := h.call(protocol.MTeamsMemberConfigRestore, protocol.MemberConfigRestoreParams{Team: ref.Team, Member: ref.Member, Revision: doc.Revision, ExpectedRevision: doc.Revision})
			results <- err
		}()
	}
	close(start)
	group.Wait()
	close(results)
	accepted := 0
	for err := range results {
		if err == nil {
			accepted++
		}
	}
	if accepted != 1 {
		t.Fatalf("accepted %d concurrent restores", accepted)
	}
	current := configDocument(t, h, ref)
	history := do[protocol.MemberConfigHistoryResult](t, h, protocol.MTeamsMemberConfigHistory, ref)
	if current.Revision != doc.Revision+1 || len(history.Revisions) != 2 {
		t.Fatal("concurrent restore lost monotonic atomic history")
	}
}

func TestMemberConfigParserBoundsAndNestedShapeKeepCurrentDocument(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Bounded YAML", nil)
	ref := configRef(team, 1)
	doc := configDocument(t, h, ref)
	for _, text := range []string{
		strings.Repeat(" ", memberconfig.MaxBytes+1),
		strings.Replace(doc.YAML, `title: ""`, "title: "+strings.Repeat("x", 65537), 1),
		strings.Replace(doc.YAML, `title: ""`, "title: !secret denied", 1),
		strings.Replace(doc.YAML, `title: ""`, "title: [wrong, shape]", 1),
		strings.Replace(doc.YAML, `title: ""`, "title: &named denied\n  persona: *named", 1),
		strings.Replace(doc.YAML, `title: ""`, "title: ["+strings.Repeat("[", 33)+"0"+strings.Repeat("]", 33)+"]", 1),
	} {
		if _, err := h.call(protocol.MTeamsMemberConfigSave, protocol.MemberConfigSaveParams{Team: ref.Team, Member: ref.Member, YAML: text, ExpectedRevision: doc.Revision}); err == nil {
			t.Fatal("unsupported/oversized YAML was accepted")
		}
	}
	if current := configDocument(t, h, ref); current != doc {
		t.Fatal("parser refusal changed configuration")
	}
}
