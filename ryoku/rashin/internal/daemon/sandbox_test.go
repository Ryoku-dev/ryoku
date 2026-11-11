package daemon

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// accessTeams gives every session the same team access.
type accessTeams struct {
	boundTeams
	access protocol.TeamAccess
}

func (t *accessTeams) Access(protocol.SessionInfo) protocol.TeamAccess { return t.access }

// With no list of its own, a team agent sees the built-in private paths as
// empty, expanded from "~/"; a list the user set replaces them, and an empty
// one hides nothing beyond Rashin's own dirs.
func TestTeamSandboxesHideTheSettingsList(t *testing.T) {
	isolateDaemon(t)
	d := &Daemon{}
	d.settings.Teams.Hidden = nil
	spec := d.sandboxBase()
	for _, path := range []string{"~/.ssh", "~/.gnupg", "~/.config/chromium", "~/.local/share/keyrings"} {
		want := paths.Resolved(filepath.Join(paths.Home(), path[2:]))
		if !slices.Contains(spec.Hidden, want) {
			t.Fatalf("the default list does not hide %s: %v", want, spec.Hidden)
		}
	}
	for _, private := range []string{paths.StateDir(), paths.CacheDir(), paths.TeamsHome(), paths.VaultDir()} {
		if !slices.Contains(spec.Hidden, paths.Resolved(private)) {
			t.Fatalf("the sandbox does not hide %s", private)
		}
	}

	d.settings.Teams.Hidden = []string{"~/Private", "/srv/secrets"}
	spec = d.sandboxBase()
	if !slices.Contains(spec.Hidden, paths.Resolved(filepath.Join(paths.Home(), "Private"))) || !slices.Contains(spec.Hidden, paths.Resolved("/srv/secrets")) {
		t.Fatalf("the user's list is not hidden: %v", spec.Hidden)
	}
	if slices.Contains(spec.Hidden, paths.Resolved(filepath.Join(paths.Home(), ".ssh"))) {
		t.Fatalf("a list the user set still hides the defaults: %v", spec.Hidden)
	}

	d.settings.Teams.Hidden = []string{}
	spec = d.sandboxBase()
	if len(spec.Hidden) != len(privateDirs()) {
		t.Fatalf("an empty list hides more than Rashin's dirs: %v", spec.Hidden)
	}
}

// A team's own access widens and narrows its agents' sandbox: its hidden
// paths join the list, and its writable folders become writable when they
// exist and do not reach into Rashin's private dirs.
func TestTeamAccessWidensAndNarrowsTheSandbox(t *testing.T) {
	isolateDaemon(t)
	home := paths.Home()
	shared := filepath.Join(home, "Projects", "site")
	if err := os.MkdirAll(shared, 0o700); err != nil {
		t.Fatal(err)
	}
	own := filepath.Join(paths.TeamsHome(), "one")
	ownCache := filepath.Join(own, "cache")
	otherTeam := filepath.Join(paths.TeamsHome(), "two")
	state := paths.StateDir()
	for _, dir := range []string{ownCache, otherTeam, state} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	d := &Daemon{}
	d.settings.Teams.Hidden = []string{}
	spec := d.sandboxBase()
	spec.Folder = paths.Resolved(own)
	spec = withAccess(spec, protocol.TeamAccess{
		Writable: []string{shared, filepath.Join(home, "Gone"), otherTeam, state, ownCache},
		Hidden:   []string{filepath.Join(home, "Projects", "site", ".env")},
	})
	want := []string{paths.Resolved(shared), paths.Resolved(ownCache)}
	if !slices.Equal(spec.Writable, want) {
		t.Fatalf("writable = %v, want %v", spec.Writable, want)
	}
	if !slices.Contains(spec.Hidden, paths.Resolved(filepath.Join(shared, ".env"))) {
		t.Fatalf("the team's hidden path is not hidden: %v", spec.Hidden)
	}
}

// The agent's sandbox and the apps on its screen both take the team's access;
// the apps only what it hides.
func TestAgentAndScreenSandboxesTakeTheTeamsAccess(t *testing.T) {
	isolateDaemon(t)
	shortRuntime(t)
	shared := filepath.Join(paths.Home(), "Projects")
	if err := os.MkdirAll(shared, 0o700); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(paths.Home(), "Notes")
	crew := &accessTeams{access: protocol.TeamAccess{Writable: []string{shared}, Hidden: []string{secret}}}
	d := New(Deps{Teams: crew})
	team, err := d.manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: filepath.Join(paths.TeamsHome(), "one"), Team: "team-1", TeamTask: "task-1", Member: "member-1"})
	if err != nil {
		t.Fatal(err)
	}
	spec, err := d.agentSandbox(team)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.closeAgentSockets)
	if !slices.Contains(spec.Writable, paths.Resolved(shared)) || !slices.Contains(spec.Hidden, paths.Resolved(secret)) {
		t.Fatalf("the agent's sandbox lacks the team's access: %+v", spec)
	}
	screen := d.screenSandbox(team.ID)
	if screen == nil || !slices.Contains(screen.Hidden, paths.Resolved(secret)) || slices.Contains(screen.Writable, paths.Resolved(shared)) {
		t.Fatalf("the screen's sandbox = %+v", screen)
	}
}
