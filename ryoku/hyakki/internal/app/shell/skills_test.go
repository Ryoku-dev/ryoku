package shell

import (
	"context"
	"image"
	"slices"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func (s *stage) skillsOff() []string {
	s.t.Helper()
	var settings protocol.Settings
	if err := s.runtime.client.Call(context.Background(), protocol.MSettingsGet, protocol.Empty{}, &settings); err != nil {
		s.t.Fatal(err)
	}
	return settings.Skills.Off
}

func (s *stage) teamSkillsOff(id string) []string {
	s.t.Helper()
	var result protocol.TeamsResult
	if err := s.runtime.client.Call(context.Background(), protocol.MTeamsList, protocol.Empty{}, &result); err != nil {
		s.t.Fatal(err)
	}
	for _, team := range result.Teams {
		if team.ID == id {
			return team.SkillsOff
		}
	}
	s.t.Fatalf("the daemon has no team %s", id)
	return nil
}

// Settings, Skills turns a set off for every model, reads a skill in the side panel and copies a missing command's install line
func TestSettingsSkillsTurnsASetOffReadsASkillAndCopiesTheInstall(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("settings")
	stage.click("Skills")
	stage.await("the sets arrive", func() bool { _, ok := stage.named("Superpowers for every model"); return ok })

	stage.reveal("OpenSpec for every model")
	stage.click("OpenSpec for every model")
	stage.await("the daemon stores OpenSpec as off", func() bool { return slices.Equal(stage.skillsOff(), []string{"openspec"}) })

	stage.reveal("Copy the openspec install command")
	stage.click("Copy the openspec install command")
	if got := stage.clipboard(); got != "npm install -g @fission-ai/openspec" {
		t.Fatalf("the clipboard holds %q", got)
	}

	stage.reveal("systematic-debugging")
	stage.click("systematic-debugging")
	stage.await("the skill's text in the side panel", func() bool { _, ok := stage.named("Systematic Debugging"); return ok })
	if _, ok := stage.named("Close systematic-debugging"); !ok {
		t.Fatalf("the sheet has no Close: %v", stage.names())
	}
	if slices.ContainsFunc(stage.names(), func(name string) bool { return strings.Contains(name, "description:") }) {
		t.Fatal("the sheet shows the skill's frontmatter")
	}
	stage.click("Close systematic-debugging")
	stage.await("the sheet to close", func() bool { _, ok := stage.named("Systematic Debugging"); return !ok })
}

// a team's Settings turns a set off for its members through the team update
func TestATeamsSettingsTurnsASkillSetOffForItsMembers(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 1000))
	stage.open("teams/team-repo/settings")
	if len(stage.teamSkillsOff("team-repo")) != 0 {
		t.Fatal("the demo team starts with a set off")
	}
	stage.reveal("Spec Kit for this team")
	stage.click("Spec Kit for this team")
	stage.await("the daemon stores Spec Kit as off for the team", func() bool {
		return slices.Equal(stage.teamSkillsOff("team-repo"), []string{"speckit"})
	})
}
