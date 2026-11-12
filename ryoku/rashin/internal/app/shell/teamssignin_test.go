package shell

import (
	"image"
	"slices"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// the harness as the demo daemon now holds it
func (s *stage) harness(id string) protocol.HarnessInfo {
	for _, harness := range s.runtime.store.Snapshot().Harnesses {
		if harness.ID == id {
			return harness
		}
	}
	return protocol.HarnessInfo{}
}

// what the daemon hands back runs in the user's terminal word for word: the sign-in in the team's own home for the harness
func (s *stage) signInLaunched(id, team string, command ...string) {
	s.t.Helper()
	s.await("the terminal opens", func() bool { return len(s.argv) > 0 })
	s.await("the daemon says the sign-in is open", func() bool {
		home, _ := s.harness(id).Teams.HomeOf(team)
		return home.Opening
	})
	argv := s.argv[len(s.argv)-1]
	if len(argv) < 4 || strings.Join(argv[:4], " ") != "ryoku-app terminal -- sh" {
		s.t.Fatalf("the sign-in was started as %q", argv)
	}
	home, _ := s.harness(id).Teams.HomeOf(team)
	if !strings.HasSuffix(home.Home, "/harness-homes/"+id+"/"+team) || !slices.Contains(argv, home.Home) {
		s.t.Fatalf("the sign-in does not run in the team home %q: %q", home.Home, argv)
	}
	if tail := argv[len(argv)-len(command):]; strings.Join(tail, " ") != strings.Join(command, " ") {
		s.t.Fatalf("the sign-in runs %q, not %q", tail, command)
	}
}

// Settings > Teams lists the teams each harness still waits on, and each signs in to its own home
func TestSettingsSignsATeamInToAHarness(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 3600))
	stage.open("settings/teams")
	if _, ok := stage.named("Sign Review account in for Release notes"); ok {
		t.Fatal("a harness Rashin knows no sign-in for offers one")
	}
	for _, name := range []string{
		"3 teams signed in, 1 not", "1 team signed in, 2 not", "Open the setup of Review account for Release notes",
		"Open the setup of Claude Code for Repo watch with News digest's config", "Every harness's own state", "~/Documents/taxes",
	} {
		if _, ok := stage.named(name); !ok {
			t.Fatalf("Settings > Teams does not show %q; on screen: %v", name, stage.names())
		}
	}
	stage.click("Sign Codex in for Repo watch")
	stage.signInLaunched("codex", "team-repo", "codex", "login")
	stage.settle()
	if _, ok := stage.named("Signing in…"); !ok {
		t.Fatalf("the row does not say it is signing in; on screen: %v", stage.names())
	}
}

// a team's own settings open the setup in its home, starting from another team's config when one is signed in
func TestATeamsSettingsOpenItsSetupFromAnotherTeam(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 3600))
	stage.open("teams/team-repo/settings")
	stage.click("Open the setup of Claude Code for Repo watch with News digest's config")
	stage.signInLaunched("claude", "team-repo", "sh")
}

// a task blocked on a sign-in offers it on its Inbox card, for the task's own team
func TestTheInboxSignsInWhatABlockedTaskWaitsOn(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 4400))
	stage.open("inbox")
	stage.click("Sign in for Teams")
	stage.signInLaunched("codex", "team-repo", "codex", "login")
}

// a notice that Rashin put a sign-in back asks nothing and says how often it happened, Got it marks it read
func TestGotItMarksTheRestoreNoticeRead(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 4400))
	stage.open("inbox")
	const title = "Rashin put Research desk's Claude Code sign-in back"
	for _, name := range []string{title, "It happened 2 more times before you read this."} {
		if _, ok := stage.named(name); !ok {
			t.Fatalf("the Inbox does not show %q; on screen: %v", name, stage.names())
		}
	}
	stage.click("Got it")
	stage.await("the daemon marks it read", func() bool {
		approval := stage.approval("approval-restore")
		return approval.Status == protocol.ApprovalApproved && approval.Decision == protocol.DecideOnce
	})
	if _, ok := stage.named("Read: " + title); !ok {
		t.Fatalf("the notice did not fold; on screen: %v", stage.names())
	}
}
