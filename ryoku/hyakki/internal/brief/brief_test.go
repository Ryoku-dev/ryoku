package brief

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func machine(t *testing.T, vault bool) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	dir := filepath.Join(root, "data", "ryoku", "vault")
	if vault {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		for name, body := range map[string]string{"AGENTS.md": "Read desktop.md first.", "desktop.md": strings.Repeat("bar: ~/.config/quickshell\n", 40)} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	return dir
}

// An agent is told where it works and that the folder is indexed, with the
// index's size and languages.
func TestAnAgentIsToldItsFolderAndItsCodeIndex(t *testing.T) {
	machine(t, true)
	text := Agent(Session{
		Info:     protocol.SessionInfo{Cwd: "/work/app/wt", Worktree: "/work/app/wt", Branch: "fix/login", Project: "p1"},
		Project:  "app",
		Index:    &protocol.IndexInfo{Root: "/work/app", Files: 412, Languages: []protocol.Count{{Name: "cpp", Count: 300}, {Name: "c", Count: 100}, {Name: "bash", Count: 12}}},
		Semantic: true,
		Screen:   true,
	})
	for _, want := range []string{
		"- Folder: /work/app/wt", "- Project: app", "- Branch: fix/login",
		"an index of /work/app (412 files: 300 C++, 100 C, 12 shell) with search by meaning built in",
		"a hidden desktop of your own",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the brief does not say %q", want)
		}
	}
	for _, gone := range []string{"Worktree:", "The user did not write this part", "MCP"} {
		if strings.Contains(text, gone) {
			t.Errorf("the brief still says %q", gone)
		}
	}
	var many []protocol.Count
	for _, name := range []string{"go", "qml", "cpp", "python", "lua", "json", "yaml", "toml"} {
		many = append(many, protocol.Count{Name: name, Count: 2})
	}
	wide := Agent(Session{Info: protocol.SessionInfo{Cwd: "/work/app"}, Index: &protocol.IndexInfo{Root: "/work/app", Files: 16, Languages: many}})
	if !strings.Contains(wide, "(16 files: 2 Go, 2 QML, 2 C++, 2 Python, 2 Lua, 2 JSON, 2 more languages)") {
		t.Errorf("an index of many languages reads: %s", wide)
	}
}

// What is not there is not claimed: no index over a folder that cannot be
// indexed, no count for an index still being built, no screen not offered.
func TestTheBriefClaimsOnlyWhatIsThere(t *testing.T) {
	machine(t, false)
	text := Agent(Session{Info: protocol.SessionInfo{Cwd: "/home/someone"}})
	if strings.Contains(text, "Code index") || strings.Contains(text, "Screen:") || strings.Contains(text, "- Reach:") {
		t.Errorf("a plain session's brief reads %q", text)
	}
	building := Agent(Session{Info: protocol.SessionInfo{Cwd: "/work/app"}, Index: &protocol.IndexInfo{Root: "/work/app"}})
	if !strings.Contains(building, "an index of /work/app, and it follows your edits") {
		t.Errorf("an index still being built reads: %s", building)
	}
}

// A team task's agent runs in a sandbox that hides the vault; it is told
// what it can change and what looks empty, and its team's brief follows.
func TestATeamAgentIsToldWhatItsSandboxReaches(t *testing.T) {
	machine(t, true)
	text := Agent(Session{
		Info:     protocol.SessionInfo{Cwd: "/teams/digest", Team: "team-1", TeamTask: "task-1"},
		Writable: []string{"/home/me/Projects/site"},
		Team:     "Your team is Digest.",
	})
	for _, want := range []string{
		"- Folder: /teams/digest",
		"You can change only this folder and /home/me/Projects/site;",
		"the Ryoku vault, the credential vault, other teams' folders and the paths Settings > Teams hides look empty",
		"code search reaches only the indexes inside the folders you can change",
		"Your team is Digest.",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the team brief does not say %q", want)
		}
	}
}

func TestTheVaultTextIsCutToItsBudget(t *testing.T) {
	machine(t, true)
	text := Vault(400)
	if !strings.Contains(text, "## AGENTS.md\nRead desktop.md first.") || !strings.Contains(text, "## desktop.md") || !strings.Contains(text, "[trimmed]") {
		t.Fatalf("the vault text reads %q", text)
	}
	if len(text) > 480 {
		t.Fatalf("the vault text is %d bytes for a budget of 400", len(text))
	}
}
