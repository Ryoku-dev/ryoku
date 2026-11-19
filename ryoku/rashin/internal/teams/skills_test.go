package teams

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/skills"
)

func TestATeamIsBriefedOnItsOwnSkillSets(t *testing.T) {
	h := newHarnessWith(t, func(s *protocol.Settings) { s.Skills.Off = []string{"superpowers"} })
	h.s.SetSkills(skills.New(filepath.Join(t.TempDir(), "skills")))
	team := h.newTeam(t, "Docs", nil)
	team = do[protocol.Team](t, h, protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: team.ID, Patch: json.RawMessage(`{"skillsOff":["openspec","openspec",""]}`)})
	if !slices.Equal(team.SkillsOff, []string{"openspec"}) || !slices.Equal(h.s.SkillsOff(team.ID), []string{"openspec"}) {
		t.Fatalf("the team's sets off = %v", team.SkillsOff)
	}
	if !slices.ContainsFunc(h.ledger(t), func(a protocol.Activity) bool {
		return a.Action == "team.update" && strings.Contains(a.Detail, "Took the skill set openspec from it.")
	}) {
		t.Fatal("turning a set off for the team is not in Activity")
	}

	task := h.newTask(t, team, "Write the design for the new sync.")
	ref := h.agent(t, task.ID)
	info, _ := h.sessions.Info(ref.Session)
	lead := h.s.Brief(info)
	_, section, found := strings.Cut(lead, "\nSkills\n")
	if !found {
		t.Fatalf("no Skills section in the lead's brief:\n%s", lead)
	}
	for _, want := range []string{"Spec Kit (speckit)", "Elephant and Goldfish (egm)", "egm-{", "/<set>/skills/<name>/SKILL.md", "As the lead: designs and reviews go best with Elephant and Goldfish", "features of many steps with Spec Kit."} {
		if !strings.Contains(section, want) {
			t.Errorf("the lead's Skills section lacks %q:\n%s", want, section)
		}
	}
	for _, unwanted := range []string{"Superpowers", "OpenSpec", "openspec-"} {
		if strings.Contains(section, unwanted) {
			t.Errorf("the lead is told of %s, which is off:\n%s", unwanted, section)
		}
	}

	for _, m := range team.Members {
		if !m.Lead {
			info.Member = m.ID
		}
	}
	member := h.s.Brief(info)
	if !strings.Contains(member, "\nSkills\n") || strings.Contains(member, "As the lead") {
		t.Fatalf("a member's brief:\n%s", member)
	}
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
