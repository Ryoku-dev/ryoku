package daemon

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
	"ryoku-rashin/internal/skills"
)

// skillTeams leaves its team's members without the sets in off.
type skillTeams struct {
	boundTeams
	off []string
}

func (t *skillTeams) SkillsOff(string, string) []string { return t.off }

func (t *skillTeams) Brief(protocol.SessionInfo) string { return "" }

func (t *skillTeams) ScreenGranted(protocol.SessionInfo) bool { return false }

// skillsDaemon has every set but egm on for every model, and its team
// leaves out openspec.
func skillsDaemon(t *testing.T) (*Daemon, protocol.SessionInfo) {
	t.Helper()
	isolateDaemon(t)
	shortRuntime(t)
	d := New(Deps{Teams: &skillTeams{boundTeams: boundTeams{handled: make(chan string, 8)}, off: []string{"openspec"}}, Skills: skills.New(paths.SkillsDir())})
	d.settings.Skills.Off = []string{"egm"}
	team, err := d.manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: filepath.Join(paths.TeamsHome(), "one"), Team: "team-1", TeamTask: "task-1", Member: "member-1"})
	if err != nil {
		t.Fatal(err)
	}
	return d, team
}

func onSets(result protocol.SkillsResult) []string {
	var on []string
	for _, set := range result.Sets {
		if set.On {
			on = append(on, set.ID)
		}
	}
	return on
}

func TestATeamToolHostReadsOnlyItsTeamsSkills(t *testing.T) {
	d, team := skillsDaemon(t)
	ctx := context.Background()
	host := d.ToolHost(team)

	var listed protocol.SkillsResult
	if err := host.Call(ctx, protocol.MSkillsList, protocol.Empty{}, &listed); err != nil {
		t.Fatal(err)
	}
	if on := onSets(listed); !slices.Equal(on, []string{"superpowers", "speckit"}) {
		t.Fatalf("the team's tools list %v as on", on)
	}
	var found protocol.SkillsSearchResult
	if err := host.Call(ctx, protocol.MSkillsSearch, protocol.SkillsSearchParams{Query: "propose a change with openspec"}, &found); err != nil {
		t.Fatal(err)
	}
	for _, skill := range found.Skills {
		if skill.Set == "openspec" || skill.Set == "egm" {
			t.Fatalf("the team's search found %s", skill.ID)
		}
	}
	for _, off := range []string{"openspec/openspec-propose", "egm/egm-draft"} {
		if err := host.Call(ctx, protocol.MSkillsGet, protocol.SkillParams{ID: off}, nil); err == nil {
			t.Fatalf("the team read %s", off)
		}
	}
	var read protocol.SkillContent
	if err := host.Call(ctx, protocol.MSkillsGet, protocol.SkillParams{ID: "superpowers/test-driven-development"}, &read); err != nil || !strings.HasPrefix(read.Text, "---\nname: test-driven-development") {
		t.Fatalf("reading a skill in the team's tools: %v", err)
	}

	// anyone else gets every set that is on for every model
	everyone, err := d.skillsCall(&connection{}, protocol.MSkillsList, nil)
	if err != nil {
		t.Fatal(err)
	}
	if on := onSets(everyone.(protocol.SkillsResult)); !slices.Equal(on, []string{"superpowers", "openspec", "speckit"}) {
		t.Fatalf("the main socket lists %v as on", on)
	}
}

// The sandbox shows the team's sets read-only at the paths its brief names,
// inside Rashin's data dir, which it otherwise hides, and keeps the sets the
// team is not given hidden.
func TestATeamSandboxReadsItsSkillsAndCannotChangeThem(t *testing.T) {
	if err := sandbox.Available(); err != nil {
		t.Skipf("bubblewrap cannot run here: %v", err)
	}
	d, team := skillsDaemon(t)
	spec, err := d.agentSandbox(team)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.closeAgentSockets)
	result, _ := d.sessionSkills(team)
	skill := filepath.Join(result.Root, "superpowers", "skills", "test-driven-development", "SKILL.md")
	hidden := filepath.Join(result.Root, "openspec", "skills", "openspec-propose", "SKILL.md")
	if !paths.Within(paths.Resolved(paths.DataDir()), paths.Resolved(skill)) || !slices.Contains(spec.Hidden, paths.Resolved(paths.LegacyVaultDir())) {
		t.Fatalf("the skills are not inside the hidden data dir: %s, %v", skill, spec.Hidden)
	}
	private := []string{
		filepath.Join(paths.DataDir(), "private-asset.txt"),
		filepath.Join(filepath.Dir(paths.ConfigFile()), "private-settings.txt"),
	}
	for _, path := range private {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("private sandbox canary"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	argv, err := sandbox.Command(*spec, []string{"/bin/sh", "-c", `
		head -n 2 "$1"
		{ echo changed >> "$1"; } 2>/dev/null || echo refused
		touch "$(dirname "$1")/new" 2>/dev/null || echo refused
		test -e "$2" || echo hidden
		test ! -e "$3" && test ! -e "$4" || echo private-visible
	`, "probe", skill, hidden, private[0], private[1]})
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(argv[0], argv[1:]...).CombinedOutput()
	if err != nil {
		t.Fatalf("sandbox probe failed: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "---\nname: test-driven-development\nrefused\nrefused\nhidden" {
		t.Fatalf("probe said %q", got)
	}
	if body, _ := os.ReadFile(skill); strings.Contains(string(body), "changed") {
		t.Fatal("the sandbox changed a skill")
	}
}
