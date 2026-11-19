package daemon

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
)

// accessTeams gives every session the same team access.
type accessTeams struct {
	boundTeams
	access protocol.TeamAccess
}

func (t *accessTeams) Access(protocol.SessionInfo) protocol.TeamAccess { return t.access }

// Every team sandbox hides the places credentials live and every other coding
// agent's own state, whatever the settings say.
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

// Every team task's tool host and its screen cover the host's /run and keep
// host daemons outside, while the tool host drops the system bus address and
// runs through the rashin command's scope.
func TestEverySandboxCoversRunAndRunsThroughTheScope(t *testing.T) {
	isolateDaemon(t)
	shortRuntime(t)
	bin := t.TempDir()
	rashin := filepath.Join(bin, "rashin")
	if err := os.WriteFile(rashin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	d := New(Deps{Teams: &accessTeams{}})
	t.Cleanup(d.closeAgentSockets)
	team, err := d.manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: filepath.Join(paths.TeamsHome(), "one"), Team: "team-1", TeamTask: "task-1", Member: "member-1"})
	if err != nil {
		t.Fatal(err)
	}
	agent, err := d.agentSandbox(team)
	if err != nil {
		t.Fatal(err)
	}
	for name, spec := range map[string]*sandbox.Spec{"agent": agent, "screen": d.screenSandbox(team.ID)} {
		for _, path := range []string{"/run/dbus/system_bus_socket", "/run/docker.sock", "/run/libvirt/libvirt-sock"} {
			if !spec.Hides(path) {
				t.Errorf("the %s sandbox shows %s", name, path)
			}
		}
		if spec.Scope != rashin {
			t.Errorf("the %s sandbox runs through %q, not the rashin command", name, spec.Scope)
		}
	}
	for _, spec := range []*sandbox.Spec{agent} {
		if !slices.Contains(spec.Unset, "DBUS_SYSTEM_BUS_ADDRESS") {
			t.Errorf("the system bus address crosses: %v", spec.Unset)
		}
		for _, path := range []string{"/nix/var/nix/daemon-socket", "/var/lib/incus"} {
			if !spec.Hides(path) {
				t.Errorf("%s shows", path)
			}
		}
	}
}

// A tool host starts with only deliberate environment variables and a private
// home; exported provider keys and the user's actual HOME cannot cross.
func TestTeamToolHostHasPrivateHomeAndCleanEnvironment(t *testing.T) {
	isolateDaemon(t)
	shortRuntime(t)
	t.Setenv("RASHIN_TEST_TOKEN", "secret")
	t.Setenv("LC_MESSAGES", "en_US.UTF-8")
	d := New(Deps{})
	team, err := d.manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: filepath.Join(paths.TeamsHome(), "one"), Team: "team-one", TeamTask: "task-one"})
	if err != nil {
		t.Fatal(err)
	}
	spec, err := d.agentSandbox(team)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.closeAgentSockets)
	home := paths.Resolved(paths.ToolHome("team-one", "task-one"))
	cache := paths.Resolved(paths.ToolCache("team-one"))
	if !spec.ClearEnv || spec.Env["HOME"] != home || spec.Env["XDG_CACHE_HOME"] != cache ||
		spec.Env["XDG_CONFIG_HOME"] != filepath.Join(home, ".config") ||
		spec.Env["XDG_DATA_HOME"] != filepath.Join(home, ".local", "share") ||
		spec.Env["XDG_STATE_HOME"] != filepath.Join(home, ".local", "state") ||
		spec.Env["LC_MESSAGES"] != "en_US.UTF-8" || spec.Env["RASHIN_TEST_TOKEN"] != "" {
		t.Fatalf("unsafe tool host environment: %+v", spec.Env)
	}
	for _, dir := range []string{home, cache} {
		info, err := os.Stat(dir)
		if err != nil || info.Mode().Perm() != 0o700 {
			t.Fatalf("tool home/cache %s permissions: %v, %v", dir, info, err)
		}
		if !slices.Contains(spec.Writable, dir) {
			t.Fatalf("tool host cannot write its private dir %s: %v", dir, spec.Writable)
		}
	}
	gitconfig, err := os.ReadFile(filepath.Join(home, ".gitconfig"))
	if err != nil || !strings.Contains(string(gitconfig), filepath.Join(paths.Home(), ".gitconfig")) {
		t.Fatalf("tool home has no user git identity include: %v, %q", err, gitconfig)
	}
}

func TestTeamToolHomeRefusesSymlinkAtTeamBoundary(t *testing.T) {
	isolateDaemon(t)
	outside := t.TempDir()
	if err := os.MkdirAll(paths.ToolHomes(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(paths.ToolHomes(), "team-one")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := prepareToolHome("team-one", "task-one"); err == nil {
		t.Fatal("a symlinked team home exposed an unrelated directory")
	}
	if _, err := os.Stat(filepath.Join(outside, "task-one")); !os.IsNotExist(err) {
		t.Fatalf("a symlinked team home wrote outside: %v", err)
	}
}
