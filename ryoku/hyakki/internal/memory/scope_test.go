package memory

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// run runs git in dir with a fixed identity.
func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com", "GIT_CONFIG_GLOBAL=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestProjectKeys(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "alpha")
	if err := os.MkdirAll(filepath.Join(repo, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, repo, "init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, repo, "add", ".")
	run(t, repo, "commit", "-q", "-m", "first")
	root := run(t, repo, "rev-list", "--max-parents=0", "HEAD")
	if err := os.WriteFile(filepath.Join(repo, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, repo, "add", ".")
	run(t, repo, "commit", "-q", "-m", "second")

	e := Open(Deps{Dir: filepath.Join(base, "vault"), TeamDir: filepath.Join(base, "teams"), DB: filepath.Join(base, "memory.db")})
	defer e.Close()
	scope, kept := e.sessionScope(protocol.SessionInfo{Cwd: filepath.Join(repo, "sub")})
	if scope != protocol.MemoryProjectPrefix+root[:12] || kept {
		t.Fatalf("scope = %q kept %v, root %s", scope, kept, root)
	}
	if name := e.scopeName(scope); name != "alpha" {
		t.Fatalf("name = %q", name)
	}

	moved := filepath.Join(base, "beta")
	if err := os.Rename(repo, moved); err != nil {
		t.Fatal(err)
	}
	fresh := Open(Deps{Dir: filepath.Join(base, "vault2"), TeamDir: filepath.Join(base, "teams2"), DB: filepath.Join(base, "memory2.db")})
	defer fresh.Close()
	if again, _ := fresh.sessionScope(protocol.SessionInfo{Cwd: moved}); again != scope {
		t.Fatalf("after rename %q, before %q", again, scope)
	}

	plain := filepath.Join(base, "plain")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	if scope, _ := e.sessionScope(protocol.SessionInfo{Cwd: plain}); scope != "" {
		t.Fatalf("plain folder scope = %q", scope)
	}

	empty := filepath.Join(base, "empty")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, empty, "init", "-q")
	run(t, empty, "remote", "add", "origin", "https://example.com/empty.git")
	if key := e.keys.key(empty); key != hashHex("https://example.com/empty.git")[:12] {
		t.Fatalf("origin key = %q", key)
	}
}

func TestProjectNamesAndModes(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "work")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, repo, "init", "-q")
	run(t, repo, "commit", "-q", "--allow-empty", "-m", "first")
	names := stubNames{projects: map[string]stubProject{"p1": {root: repo, name: "Ryoku", mode: protocol.ProjectMemoryKept}}}
	e := Open(Deps{Names: names, Dir: filepath.Join(base, "vault"), TeamDir: filepath.Join(base, "teams"), DB: filepath.Join(base, "memory.db")})
	defer e.Close()
	scope, kept := e.sessionScope(protocol.SessionInfo{Project: "p1", Cwd: base})
	if scope == "" || !kept || e.scopeName(scope) != "Ryoku" {
		t.Fatalf("scope %q kept %v name %q", scope, kept, e.scopeName(scope))
	}
	st, err := e.store()
	if err != nil {
		t.Fatal(err)
	}
	mustApply(t, st, Op{Kind: OpAdd, Scope: scope, Text: "Builds with make"})
	path := filepath.Join(base, "vault", "projects", "ryoku-"+strings.TrimPrefix(scope, protocol.MemoryProjectPrefix)+".md")
	data, err := os.ReadFile(path)
	if err != nil || !strings.HasPrefix(string(data), "# Ryoku\n") {
		t.Fatalf("project file %s: %v\n%s", path, err, data)
	}
}

// stubProject is one Work project of stubNames.
type stubProject struct{ root, name, mode string }

// stubNames is Names over fixed projects and teams.
type stubNames struct {
	projects map[string]stubProject
	teams    map[string]protocol.Team
}

func (n stubNames) Project(id string) (string, string, string, bool) {
	p, ok := n.projects[id]
	return p.root, p.name, p.mode, ok
}

func (n stubNames) ProjectAt(root string) (string, string, string, bool) {
	for id, p := range n.projects {
		if filepath.Clean(p.root) == filepath.Clean(root) {
			return id, p.name, p.mode, true
		}
	}
	return "", "", "", false
}

func (n stubNames) Team(id string) (protocol.Team, bool) {
	team, ok := n.teams[id]
	return team, ok
}
