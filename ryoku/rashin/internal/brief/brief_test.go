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

// An agent is told the three things it cannot know by itself: what Ryoku and
// its vault are, what Rashin is, and that the folder it works in is searchable.
func TestAnAgentIsToldAboutRyokuTheVaultRashinAndCodeSearch(t *testing.T) {
	vault := machine(t, true)
	text := Agent(Session{
		Info:     protocol.SessionInfo{Cwd: "/work/app/wt", Worktree: "/work/app/wt", Branch: "fix/login", Project: "p1"},
		Project:  "app",
		Index:    &protocol.IndexInfo{Root: "/work/app", Files: 412, Languages: []protocol.Count{{Name: "cpp", Count: 300}, {Name: "c", Count: 100}, {Name: "bash", Count: 12}}},
		Semantic: true,
	})
	for _, want := range []string{
		"The user did not write this part",
		"Ryoku, a Linux desktop built on Arch",
		"- Folder: /work/app/wt", "- Project: app", "- Branch: fix/login",
		vault + " is this machine's map", "desktop.md", "ryoku owner <path>",
		"Rashin is Ryoku's optional assistant", "Rashin > Settings > Providers",
		"an index of /work/app (412 files: 300 C++, 100 C, 12 shell) with search by meaning built in",
		"when another code index or search skill you carry finds little here",
		"rashin search", "rashin find <name>", "rashin references <name>",
		"End of context from Rashin.",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the brief does not say %q", want)
		}
	}
	if strings.Contains(text, "Worktree:") {
		t.Error("the brief names the worktree again though it is the folder")
	}
	var many []protocol.Count
	for _, name := range []string{"go", "qml", "cpp", "python", "lua", "json", "yaml", "toml"} {
		many = append(many, protocol.Count{Name: name, Count: 2})
	}
	wide := Agent(Session{Info: protocol.SessionInfo{Cwd: "/work/app"}, Index: &protocol.IndexInfo{Root: "/work/app", Files: 16, Languages: many}})
	if !strings.Contains(wide, "(16 files: 2 Go, 2 QML, 2 C++, 2 Python, 2 Lua, 2 JSON, 2 more languages)") {
		t.Errorf("an index of many languages reads: %s", wide[strings.Index(wide, "Code search"):])
	}
}

// What is not there is not claimed: no vault on a machine without one, no
// search over a folder that cannot be indexed, no count for an index still
// being built.
func TestTheBriefClaimsOnlyWhatIsThere(t *testing.T) {
	machine(t, false)
	text := Agent(Session{Info: protocol.SessionInfo{Cwd: "/home/someone"}})
	if strings.Contains(text, "vault") && strings.Contains(text, "this machine's map") {
		t.Error("the brief points at a vault that does not exist")
	}
	if strings.Contains(text, "Code search\n") {
		t.Error("the brief offers a search over a folder that is not indexed")
	}
	building := Agent(Session{Info: protocol.SessionInfo{Cwd: "/work/app"}, Index: &protocol.IndexInfo{Root: "/work/app"}})
	if !strings.Contains(building, "an index of /work/app, and it follows your edits") {
		t.Errorf("an index still being built reads: %s", building[strings.Index(building, "Code search"):])
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
	machine(t, false)
	if text := Vault(400); !strings.Contains(text, "not available yet") {
		t.Fatalf("a machine without a vault reads %q", text)
	}
}
