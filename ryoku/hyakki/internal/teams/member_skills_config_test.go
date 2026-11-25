package teams

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ryoku-rashin/internal/memberconfig"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/skills"
)

func TestMemberSkillPoliciesSurviveConfigFormRestoreAndReopen(t *testing.T) {
	h := newHarnessWith(t, func(settings *protocol.Settings) {
		settings.Skills.Off = []string{"superpowers"}
	})
	library := skills.New(filepath.Join(t.TempDir(), "skills"))
	h.s.SetSkills(library)
	catalog, err := library.List(nil)
	if err != nil {
		t.Fatal(err)
	}
	var nilAllowlist []string
	emptyAllowlist := []string{}
	subset := []string{"egm", "openspec", "superpowers"}
	team := h.newTeam(t, "Skill policy configuration", func(p *protocol.TeamCreateParams) {
		p.Reporting = protocol.ReportingTree
		p.Members = []protocol.Member{
			{ID: "root", Name: "Denied nil", Lead: true, Skills: &nilAllowlist, Instructions: "Keep these instructions.", Rank: "King", Persona: "reviewer", Tools: &protocol.TeamTools{}, Trust: protocol.ApprovalsReadOnly},
			{ID: "denied", Name: "Denied empty", ReportsTo: "root", Skills: &emptyAllowlist},
			{ID: "inherited", Name: "Inherited", ReportsTo: "root"},
			{ID: "subset", Name: "Subset", ReportsTo: "root", Skills: &subset},
		}
	})
	if nilAllowlist != nil || len(emptyAllowlist) != 0 || !reflect.DeepEqual(subset, []string{"egm", "openspec", "superpowers"}) {
		t.Fatal("creating a team mutated the caller's skill policy")
	}
	credential := runtimeRecord(t, h, protocol.SessionInfo{Team: team.ID}, protocol.CredentialRuntime)
	team.Members[0].Runtime = &protocol.MemberRuntime{
		Kind: protocol.MemberRuntimeCommand, Command: []string{"fixture", "--review"},
		Env: map[string]string{"PUBLIC_MODE": "review"}, SecretEnv: map[string]string{"PROCESS_KEY": credential.ID},
	}
	team = do[protocol.Team](t, h, protocol.MTeamsUpdate, protocol.TeamUpdateParams{
		ID: team.ID, Patch: mustJSON(t, map[string]any{"skillsOff": []string{"openspec"}, "members": team.Members}),
	})
	baseline := cloneTeam(team)
	expected := cloneTeam(team)
	check := func(stage string) {
		t.Helper()
		live, exists := h.s.state.team(team.ID)
		if !exists || !reflect.DeepEqual(live.Members, expected.Members) {
			t.Fatalf("%s changed a member field beyond the requested edit: got %#v, want %#v", stage, live.Members, expected.Members)
		}
		if live.Tools != baseline.Tools || live.Approvals != baseline.Approvals || live.Budget != baseline.Budget ||
			!reflect.DeepEqual(live.Access, baseline.Access) || !reflect.DeepEqual(live.SkillsOff, baseline.SkillsOff) || !reflect.DeepEqual(live.Memory, baseline.Memory) {
			t.Fatalf("%s changed unrelated team grants or policy", stage)
		}
		for _, member := range live.Members {
			off := skills.Off(h.s.settings(), h.s.SkillsOff(team.ID, member.ID)...)
			for _, set := range catalog.Sets {
				if len(set.Skills) == 0 {
					continue
				}
				params := protocol.SkillParams{ID: set.Skills[0].ID}
				if _, err := library.Get(params, nil); err != nil {
					t.Fatalf("invalid skill fixture %s: %v", params.ID, err)
				}
				allowed := set.ID != "openspec" && set.ID != "superpowers" && set.ID != protocol.ManagedSkillSet
				if member.Skills != nil {
					granted := false
					for _, id := range *member.Skills {
						granted = granted || id == set.ID
					}
					allowed = allowed && granted
				}
				_, err := library.Get(params, off)
				if allowed != (err == nil) {
					t.Fatalf("%s: member %s access to %s: allowed=%t, err=%v", stage, member.ID, params.ID, allowed, err)
				}
			}
		}
	}
	check("whole-roster creation and update")
	for _, index := range []int{0, 1} {
		if expected.Members[index].Skills == nil || *expected.Members[index].Skills == nil || len(*expected.Members[index].Skills) != 0 {
			t.Fatal("normalization did not retain an explicit non-nil empty allowlist")
		}
	}
	if expected.Members[2].Skills != nil || expected.Members[3].Skills == nil || !reflect.DeepEqual(*expected.Members[3].Skills, subset) {
		t.Fatal("inherited and nonempty policies lost their distinct representations")
	}

	ref := configRef(team, 0)
	original := configDocument(t, h, ref)
	if !strings.Contains(original.YAML, "skills: []") || !strings.Contains(original.YAML, memberconfig.Mask) || strings.Contains(original.YAML, credential.ID) {
		t.Fatal("configuration did not preserve denial while masking the runtime reference")
	}
	saved := do[protocol.MemberConfigDocument](t, h, protocol.MTeamsMemberConfigSave, protocol.MemberConfigSaveParams{
		Team: ref.Team, Member: ref.Member, YAML: configTitle(original.YAML, "Reviewer"), ExpectedRevision: original.Revision,
	})
	expected.Members[0].Title = "Reviewer"
	expected.Members[0].Revision++
	if saved.Revision != expected.Members[0].Revision {
		t.Fatal("title save did not advance exactly one revision")
	}
	check("Get/title Save")
	subsetRef := configRef(team, 3)
	subsetDocument := configDocument(t, h, subsetRef)
	do[protocol.MemberConfigDocument](t, h, protocol.MTeamsMemberConfigSave, protocol.MemberConfigSaveParams{
		Team: subsetRef.Team, Member: subsetRef.Member, YAML: configTitle(subsetDocument.YAML, "Subset YAML title"), ExpectedRevision: subsetDocument.Revision,
	})
	expected.Members[3].Title = "Subset YAML title"
	expected.Members[3].Revision++
	check("nonempty allowlist YAML title Save")

	form := cloneTeam(expected)
	form.Members[0].Skills = &subset
	form.Members[0].Title = "Form title"
	form.Members[3].Title = "Unrelated edited seat"
	form = do[protocol.Team](t, h, protocol.MTeamsUpdate, protocol.TeamUpdateParams{
		ID: team.ID, Patch: mustJSON(t, map[string]any{"members": form.Members}),
	})
	expected.Members[0].Skills = &subset
	expected.Members[0].Title = "Form title"
	expected.Members[0].Revision++
	expected.Members[3].Title = "Unrelated edited seat"
	expected.Members[3].Revision++
	check("ordinary whole-roster form update")

	preview := do[protocol.MemberConfigDocument](t, h, protocol.MTeamsMemberConfigGet, protocol.MemberConfigRef{
		Team: ref.Team, Member: ref.Member, Revision: original.Revision,
	})
	if preview.YAML != original.YAML || preview.Revision != original.Revision || preview.CurrentRevision != expected.Members[0].Revision {
		t.Fatal("historical preview did not retain the original denied configuration")
	}
	restored := do[protocol.MemberConfigDocument](t, h, protocol.MTeamsMemberConfigRestore, protocol.MemberConfigRestoreParams{
		Team: ref.Team, Member: ref.Member, Revision: original.Revision, ExpectedRevision: expected.Members[0].Revision,
	})
	revision := expected.Members[0].Revision + 1
	expected.Members[0] = baseline.Members[0]
	expected.Members[0].Revision = revision
	if restored.Revision != revision {
		t.Fatal("restore did not create a new revision")
	}
	check("historical restore with another denied member")

	history := do[protocol.MemberConfigHistoryResult](t, h, protocol.MTeamsMemberConfigHistory, ref)
	restartHeartbeatHarness(t, h)
	h.s.SetSkills(library)
	check("state reopen")
	if reopened := configDocument(t, h, ref); reopened != restored {
		t.Fatal("state reopen changed the restored configuration")
	}
	if reopened := do[protocol.MemberConfigHistoryResult](t, h, protocol.MTeamsMemberConfigHistory, ref); !reflect.DeepEqual(reopened, history) {
		t.Fatal("state reopen changed retained revisions")
	}
	if reopened := do[protocol.MemberConfigDocument](t, h, protocol.MTeamsMemberConfigGet, protocol.MemberConfigRef{Team: ref.Team, Member: ref.Member, Revision: original.Revision}); reopened.YAML != preview.YAML || reopened.Revision != original.Revision || reopened.CurrentRevision != restored.Revision {
		t.Fatal("state reopen changed the historical denied configuration")
	}
}

func TestMemberSkillNullAndAbsentInputRemainInheritance(t *testing.T) {
	for _, policy := range []string{"null", "absent"} {
		t.Run(policy, func(t *testing.T) {
			h := newHarness(t, 0)
			library := skills.New(filepath.Join(t.TempDir(), "skills"))
			h.s.SetSkills(library)
			var denied []string
			team := h.newTeam(t, "Explicit skill inheritance", func(p *protocol.TeamCreateParams) {
				p.Members[0].Skills = &denied
			})
			member := team.Members[0]
			fields := make(map[string]json.RawMessage)
			if err := json.Unmarshal(mustJSON(t, member), &fields); err != nil {
				t.Fatal(err)
			}
			if policy == "null" {
				fields["skills"] = json.RawMessage("null")
			} else {
				delete(fields, "skills")
			}
			members := []json.RawMessage{mustJSON(t, fields), mustJSON(t, team.Members[1])}
			team = do[protocol.Team](t, h, protocol.MTeamsUpdate, protocol.TeamUpdateParams{
				ID: team.ID, Patch: mustJSON(t, map[string]any{"members": members}),
			})
			if team.Members[0].Skills != nil || team.Members[0].Revision != member.Revision+1 {
				t.Fatal("explicit inheritance input did not retain standard JSON semantics")
			}
			ref := configRef(team, 0)
			document := configDocument(t, h, ref)
			text := configTitle(document.YAML, "Inherited title")
			if policy == "null" {
				text = strings.Replace(text, "member:\n", "member:\n  skills: null\n", 1)
			}
			saved := do[protocol.MemberConfigDocument](t, h, protocol.MTeamsMemberConfigSave, protocol.MemberConfigSaveParams{
				Team: ref.Team, Member: ref.Member, YAML: text, ExpectedRevision: document.Revision,
			})
			restartHeartbeatHarness(t, h)
			h.s.SetSkills(library)
			live, _ := h.s.state.team(team.ID)
			if live.Members[0].Skills != nil || live.Members[0].Title != "Inherited title" || live.Members[0].Revision != saved.Revision {
				t.Fatal("YAML title save or reopen changed inheritance")
			}
			params := protocol.SkillParams{ID: "egm"}
			catalog, err := library.List(nil)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, set := range catalog.Sets {
				if set.ID == "egm" && len(set.Skills) != 0 {
					params.ID = set.Skills[0].ID
					found = true
				}
			}
			if !found {
				t.Fatal("missing egm skill fixture")
			}
			if _, err := library.Get(params, skills.Off(h.s.settings(), h.s.SkillsOff(team.ID, member.ID)...)); err != nil {
				t.Fatalf("inherited skill remains denied: %v", err)
			}
		})
	}
}
