package sys

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// box lays an account home: an optional environment.d channel and an optional
// recorded checkout on branch (a linked worktree when worktree is set).
func box(t *testing.T, channel, branch string, worktree bool) string {
	t.Helper()
	home := t.TempDir()
	if channel != "" {
		writeFile(t, filepath.Join(home, ".config", "environment.d", "ryoku.conf"), "PATH=/x\nRYOKU_CHANNEL="+channel+"\n")
	}
	if branch != "" {
		repo := filepath.Join(home, "ryoku-arch")
		gitDir := filepath.Join(repo, ".git")
		if worktree {
			gitDir = filepath.Join(home, "main-clone", ".git", "worktrees", "ryoku-arch")
			writeFile(t, filepath.Join(repo, ".git"), "gitdir: "+gitDir+"\n")
		}
		writeFile(t, filepath.Join(gitDir, "HEAD"), "ref: refs/heads/"+branch+"\n")
		writeFile(t, filepath.Join(home, ".local", "state", "ryoku", "repo"), repo+"\n")
	}
	return home
}

// The user's report and the root helper decide the line with the same rule,
// and the session's RYOKU_CHANNEL never takes part (pkexec does not pass it).
func TestSourceLineUserAndRootAgree(t *testing.T) {
	cases := []struct {
		name            string
		channel, branch string
		worktree        bool
		want            string
	}{
		{"repo-tracked hyperion box, no environment.d", "", "hyperion", false, "hyperion"},
		{"repo-tracked hyperion worktree", "", "hyperion", true, "hyperion"},
		{"switched box: environment.d wins over the checkout", "hyperion", "unstable-dev", false, "hyperion"},
		{"unstable checkout", "", "unstable-dev", false, "unstable-dev"},
		{"environment.d only", "hyperion", "", false, "hyperion"},
		{"stable packaged box", "", "", false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := box(t, c.channel, c.branch, c.worktree)
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
			t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
			t.Setenv("RYOKU_CHANNEL", "main")
			t.Setenv("RYOKU_REPO", "")
			user, root := SourceLine(), SourceLineAt(home)
			if user != c.want || root != c.want {
				t.Fatalf("user %q, root %q, want %q", user, root, c.want)
			}
		})
	}
}

func TestSourceLineIgnoresDetachedAndJunk(t *testing.T) {
	home := box(t, "", "x", false)
	writeFile(t, filepath.Join(home, "ryoku-arch", ".git", "HEAD"), "ced944d1b0000000000000000000000000000000\n")
	if got := SourceLineAt(home); got != "" {
		t.Fatalf("detached HEAD gave %q", got)
	}
	if got := SourceLineAt(""); got != "" {
		t.Fatalf("empty home gave %q", got)
	}
	big := box(t, "", "", false)
	writeFile(t, filepath.Join(big, ".config", "environment.d", "ryoku.conf"), string(make([]byte, 8<<10))+"\nRYOKU_CHANNEL=hyperion\n")
	if got := SourceLineAt(big); got != "" {
		t.Fatalf("read past the 4 KiB bound: %q", got)
	}
}
