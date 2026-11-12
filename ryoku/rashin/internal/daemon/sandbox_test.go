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

// Every team sandbox hides the places credentials live and every harness's
// own state, whatever the settings say: the user's list only adds to them,
// and an empty one hides exactly what Rashin always hides.
func TestTeamSandboxesAlwaysHideSecretsAndTheSettingsListAddsToThem(t *testing.T) {
	isolateDaemon(t)
	d := &Daemon{}
	for _, hidden := range [][]string{nil, {}, {"~/Private", "/srv/secrets"}} {
		d.settings.Teams.Hidden = hidden
		spec := d.sandboxBase()
		for _, path := range []string{
			"~/.ssh", "~/.gnupg", "~/.config/gh", "~/.hermes", "~/.codex", "~/.claude", "~/.omp", "~/.config/chromium",
			"~/.config/vivaldi", "~/.librewolf", "~/.config/vesktop", "~/.config/Signal", "~/.local/share/TelegramDesktop",
			"~/.bash_history", "~/.zsh_history", "~/.local/share/fish/fish_history", "~/.aws", "~/.config/gcloud",
			"~/.vault-token", "~/.config/rclone", "~/.npmrc", "~/.pypirc", "~/.cargo/credentials.toml", "~/.thunderbird",
			"~/.local/share/keyrings", "~/.local/state/ryoku",
		} {
			if !slices.Contains(spec.Hidden, paths.Resolved(paths.Expand(path))) {
				t.Fatalf("with the list %v the sandbox does not hide %s", hidden, path)
			}
		}
		for _, private := range []string{paths.StateDir(), paths.CacheDir(), paths.TeamsHome(), paths.VaultDir()} {
			if !slices.Contains(spec.Hidden, paths.Resolved(private)) {
				t.Fatalf("the sandbox does not hide %s", private)
			}
		}
	}
	if spec := d.sandboxBase(); !slices.Contains(spec.Hidden, paths.Resolved(filepath.Join(paths.Home(), "Private"))) || !slices.Contains(spec.Hidden, paths.Resolved("/srv/secrets")) {
		t.Fatalf("the user's list is not hidden: %v", spec.Hidden)
	}
	d.settings.Teams.Hidden = []string{}
	if spec := d.sandboxBase(); !slices.Equal(spec.Hidden, alwaysHidden()) {
		t.Fatalf("an empty list hides other than what Rashin always hides: %v", spec.Hidden)
	}
}

// A team folder inside what Rashin always hides is refused, as binding it
// would uncover it; one under the teams' home is a team's own.
func TestATeamFolderInsideAHiddenPlaceIsRefused(t *testing.T) {
	isolateDaemon(t)
	shortRuntime(t)
	d := New(Deps{})
	for _, folder := range []string{filepath.Join(paths.Home(), ".ssh"), filepath.Join(paths.Home(), ".omp", "agent"), filepath.Join(paths.StateDir(), "x")} {
		info, err := d.manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: folder, Team: "team-1", TeamTask: "task-1", Member: "member-1"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.agentSandbox(info); err == nil {
			t.Fatalf("a team folder in %s was given a sandbox", folder)
		}
		if _, err := os.Stat(folder); !os.IsNotExist(err) {
			t.Fatalf("the refused folder %s was made", folder)
		}
	}
	t.Cleanup(d.closeAgentSockets)
}

// A team's own access widens and narrows its agents' sandbox: its hidden
// paths join the list, and its writable folders become writable when they
// exist and do not reach into what Rashin always hides.
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
	gh := filepath.Join(home, ".config", "gh")
	omp := filepath.Join(home, ".omp", "agent")
	for _, dir := range []string{ownCache, otherTeam, state, gh, omp} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	d := &Daemon{}
	d.settings.Teams.Hidden = []string{}
	spec := d.sandboxBase()
	spec.Folder = paths.Resolved(own)
	spec = withAccess(spec, protocol.TeamAccess{
		Writable: []string{shared, filepath.Join(home, "Gone"), otherTeam, state, gh, omp, ownCache},
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
