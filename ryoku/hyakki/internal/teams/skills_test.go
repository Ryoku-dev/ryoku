package teams

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/skills"
)

func TestMemberSkillsCannotWidenGlobalOrTeamRestrictions(t *testing.T) {
	h := newHarnessWith(t, func(s *protocol.Settings) { s.Skills.Off = []string{"superpowers"} })
	library := skills.New(filepath.Join(t.TempDir(), "skills"))
	h.s.SetSkills(library)
	team := h.newTeam(t, "Docs", nil)
	catalog, err := library.List(nil)
	if err != nil {
		t.Fatal(err)
	}
	check := func(member string, allowed func(string) bool) {
		t.Helper()
		off := skills.Off(h.s.settings(), h.s.SkillsOff(team.ID, member)...)
		for _, set := range catalog.Sets {
			if len(set.Skills) == 0 {
				continue
			}
			p := protocol.SkillParams{ID: set.Skills[0].ID}
			if _, err := library.Get(p, nil); err != nil {
				t.Fatalf("invalid canonical skill fixture %s: %v", p.ID, err)
			}
			_, err := library.Get(p, off)
			if allowed(set.ID) != (err == nil) {
				t.Fatalf("seat %s access to %s: allowed=%t, err=%v", member, p.ID, allowed(set.ID), err)
			}
		}
	}
	save := func(allow *[]string) {
		t.Helper()
		team.Members[0].Skills = allow
		patch, err := json.Marshal(map[string]any{"skillsOff": []string{"openspec"}, "members": team.Members})
		if err != nil {
			t.Fatal(err)
		}
		team = do[protocol.Team](t, h, protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: team.ID, Patch: patch})
	}
	allow := []string{"egm", "openspec", "superpowers"}
	save(&allow)
	check(team.Members[0].ID, func(set string) bool { return set == "egm" })
	save(nil)
	check(team.Members[0].ID, func(set string) bool {
		return set != "openspec" && set != "superpowers" && set != protocol.ManagedSkillSet
	})
	empty := []string{}
	save(&empty)
	check(team.Members[0].ID, func(string) bool { return false })
	check("removed-seat", func(string) bool { return false })
}

func TestTeamBriefHidesManagedSkillsWhileAutolearnIsOff(t *testing.T) {
	h := newHarness(t, 0)
	library := skills.New(filepath.Join(t.TempDir(), "builtin"))
	managed := library.WithManaged(filepath.Join(t.TempDir(), "managed"))
	if _, err := managed.Create("release-check", "Use when preparing a release.", "1. Check the release."); err != nil {
		t.Fatal(err)
	}
	h.s.SetSkills(library)
	team := h.newTeam(t, "Docs", nil)
	ref := h.agent(t, h.newTask(t, team, "Write the release notes.").ID)
	info, _ := h.sessions.Info(ref.Session)
	if brief := h.s.Brief(info); strings.Contains(brief, "release-check") || strings.Contains(brief, managed.Dir()) {
		t.Fatalf("managed skills leaked into the team brief while disabled:\n%s", brief)
	}
}
