package daemon

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
)

// The dreams' home is hidden from every team as the teams' home is, while a
// dream's own folder under it is bound back for its own agents: another team
// cannot read what a dream makes, and a dream cannot read another team.
func TestTheDreamsHomeIsHiddenFromAnotherTeamsSandbox(t *testing.T) {
	isolateDaemon(t)
	shortRuntime(t)
	dreamFolder := filepath.Join(paths.DreamsHome(), "esp32-setup")
	teamFolder := filepath.Join(paths.TeamsHome(), "one")
	for _, dir := range []string{dreamFolder, teamFolder} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	made := filepath.Join(dreamFolder, "index.html")
	kept := filepath.Join(teamFolder, "notes.md")
	for _, file := range []string{made, kept} {
		if err := os.WriteFile(file, []byte("private\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	d := New(Deps{})
	t.Cleanup(d.closeAgentSockets)
	team, err := d.manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: teamFolder, Team: "team-1", TeamTask: "task-1", Member: "member-1"})
	if err != nil {
		t.Fatal(err)
	}
	dream, err := d.manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: dreamFolder, Team: "team-2", TeamTask: "task-2", Member: "member-2"})
	if err != nil {
		t.Fatal(err)
	}
	teamSpec, err := d.agentSandbox(team)
	if err != nil {
		t.Fatal(err)
	}
	dreamSpec, err := d.agentSandbox(dream)
	if err != nil {
		t.Fatalf("a dream's own folder was refused: %v", err)
	}
	if !slices.Contains(teamSpec.Hidden, paths.Resolved(paths.DreamsHome())) || dreamSpec.Folder != paths.Resolved(dreamFolder) {
		t.Fatalf("team hides %v; dream folder %s", teamSpec.Hidden, dreamSpec.Folder)
	}
	if err := sandbox.Available(); err != nil {
		t.Skipf("bubblewrap cannot run here: %v", err)
	}
	probe := func(spec sandbox.Spec, own, other string) string {
		argv, err := sandbox.Command(spec, []string{"/bin/sh", "-c", `cat "$1"; test -e "$2" || echo hidden`, "probe", own, other})
		if err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command(argv[0], argv[1:]...).CombinedOutput()
		if err != nil {
			t.Fatalf("sandbox probe failed: %v\n%s", err, out)
		}
		return strings.TrimSpace(string(out))
	}
	if got := probe(*teamSpec, kept, made); got != "private\nhidden" {
		t.Fatalf("the team's probe said %q", got)
	}
	if got := probe(*dreamSpec, made, kept); got != "private\nhidden" {
		t.Fatalf("the dream's probe said %q", got)
	}
}
