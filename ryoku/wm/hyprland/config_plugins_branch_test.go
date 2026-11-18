package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A plugin that builds a named branch (Hyprdrive: the fork's ryoku branch) reads
// that branch's manifest and builds its tip, even when the default branch's
// manifest pins the running Hyprland to another commit.
func TestBranchPluginBuildsItsBranchNotTheDefault(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	const hl = "0123456789012345678901234567890123456789"
	origin := t.TempDir()
	git := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(origin, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(origin, "init", "-q", "-b", "main")
	write("hyprpm.toml", "[repository]\nname = \"x\"\ncommit_pins = []\n[x]\noutput = \"x.so\"\nbuild = [\"true\"]\n")
	write("which", "main\n")
	git(origin, "add", "-A")
	git(origin, "commit", "-q", "-m", "base")
	base := git(origin, "rev-parse", "HEAD")
	// the default branch pins the running Hyprland to its old commit
	write("hyprpm.toml", "[repository]\nname = \"x\"\ncommit_pins = [\n  [\""+hl+"\", \""+base+"\"],\n]\n[x]\noutput = \"x.so\"\nbuild = [\"true\"]\n")
	git(origin, "commit", "-q", "-am", "pin")
	git(origin, "checkout", "-q", "-b", "ryoku", base)
	write("which", "ryoku\n")
	git(origin, "commit", "-q", "-am", "ryoku")
	want := git(origin, "rev-parse", "HEAD")
	git(origin, "checkout", "-q", "main")

	clone := filepath.Join(t.TempDir(), "clone")
	git(filepath.Dir(clone), "clone", "-q", origin, clone)

	m, err := tipManifest(clone, "ryoku")
	if err != nil {
		t.Fatal(err)
	}
	if m.pinFor(hl) != "" {
		t.Fatal("the ryoku branch's manifest has no pin; the default branch's was read")
	}
	got, err := checkoutFor(clone, m, hl, "ryoku", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("built %s, want the ryoku tip %s", got, want)
	}
	if b, _ := os.ReadFile(filepath.Join(clone, "which")); strings.TrimSpace(string(b)) != "ryoku" {
		t.Fatalf("checked out %q, want the ryoku branch", b)
	}
	// the default branch still builds as before
	if m, err := tipManifest(clone, ""); err != nil || m.pinFor(hl) != base {
		t.Fatalf("the default branch's manifest should pin %s: %v", base, err)
	}
}

// Hyprdrive builds from the fork's ryoku branch, never from Ryoku's own tree.
func TestHyprdriveFollowsTheFork(t *testing.T) {
	d, ok := bundledByID("hyprdrive")
	if !ok {
		t.Fatal("hyprdrive is not bundled")
	}
	if d.Repo != "https://github.com/dodo986/hyprdrive-ryoku" || d.Branch != "ryoku" || d.Local != "" {
		t.Fatalf("hyprdrive builds from repo %q branch %q local %q", d.Repo, d.Branch, d.Local)
	}
}
