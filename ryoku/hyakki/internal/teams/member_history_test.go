package teams

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestLegacyMemberHistoryMigratesOnceAndCorruptHistoryIsRefused(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Legacy configuration", nil)
	ref := configRef(team, 1)
	doc := configDocument(t, h, ref)
	if err := h.s.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(h.dir, "state.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var legacy map[string]json.RawMessage
	if err := json.Unmarshal(raw, &legacy); err != nil {
		t.Fatal(err)
	}
	delete(legacy, "memberHistoryVersion")
	delete(legacy, "memberHistories")
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := openState(h.dir)
	if err != nil {
		t.Fatal(err)
	}
	member, entries, err := st.memberSnapshots(ref.Team, ref.Member)
	if err != nil || len(entries) != 1 || member.Revision != doc.Revision {
		t.Fatal("legacy migration fabricated or lost a revision")
	}
	reopened, err := openState(h.dir)
	if err != nil {
		t.Fatal(err)
	}
	_, again, err := reopened.memberSnapshots(ref.Team, ref.Member)
	if err != nil || !reflect.DeepEqual(entries, again) {
		t.Fatal("migration repeated or changed snapshots on reopen")
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var file stateFile
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	file.MemberHistories[ref.Team][ref.Member][0].Member.Title = "corrupt snapshot"
	data, err = json.Marshal(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := openState(h.dir); err == nil {
		t.Fatal("corrupt history was silently replaced by manufactured history")
	}
}

func TestGraphRemovalBindsOriginalTeamEvenWhenSurvivingMembersUnchanged(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Conflicting removal", nil)
	original := cloneTeam(team)
	team.Members[1].Title = "Concurrent edit"
	updated := do[protocol.Team](t, h, protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: team.ID, Patch: mustJSON(t, map[string]any{"members": team.Members})})
	_, err := h.call(protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: team.ID, ExpectedUpdated: original.Updated, Patch: mustJSON(t, map[string]any{"members": original.Members[:1]})})
	if err == nil {
		t.Fatal("removal discarded a concurrent edit to the removed member")
	}
	live, _ := h.s.state.team(team.ID)
	if !reflect.DeepEqual(live.Members, updated.Members) {
		t.Fatal("conflicting removal changed canonical roster")
	}
}

func TestMemberConfigFirstCreateFailureDoesNotKeepTeamOrHistory(t *testing.T) {
	h := newHarness(t, 0)
	path := filepath.Join(h.dir, "state.json")
	if _, err := os.Stat(path); err == nil {
		t.Fatal("fixture unexpectedly has a committed team state")
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "block"), []byte("block"), 0o600); err != nil {
		t.Fatal(err)
	}
	params := protocol.TeamCreateParams{Name: "First create", Folder: filepath.Join(h.dir, "folders", "first-create"), Members: []protocol.Member{{Name: "Root", Lead: true}}}
	if _, err := h.call(protocol.MTeamsCreate, params); err == nil {
		t.Fatal("first create ignored failed disk commit")
	}
	if teams := do[protocol.TeamsResult](t, h, protocol.MTeamsList, protocol.Empty{}); len(teams.Teams) != 0 {
		t.Fatal("failed first create kept a canonical team")
	}
	if len(h.s.state.memberHistories) != 0 {
		t.Fatal("failed first create kept configuration history")
	}
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	restartHeartbeatHarness(t, h)
	if teams := do[protocol.TeamsResult](t, h, protocol.MTeamsList, protocol.Empty{}); len(teams.Teams) != 0 {
		t.Fatal("failed first create resurrected on reopen")
	}
	team := do[protocol.Team](t, h, protocol.MTeamsCreate, params)
	doc := configDocument(t, h, configRef(team, 0))
	if doc.Revision != 1 {
		t.Fatal("successful first create did not start a single revision")
	}
}
