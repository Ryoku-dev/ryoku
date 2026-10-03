package sys

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitInitRyokuArch makes a git work tree at dir whose origin names ryoku-arch,
// so ResolveRepo's fallback and a recorded pointer both accept it.
func gitInitRyokuArch(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("remote", "add", "origin", "https://github.com/ryoku-dev/ryoku-arch.git")
}

// RetireSourceTracking drops the recorded pointer and the tracked channel so the
// update path resolves to packages, while the ~/ryoku-arch clone is left on disk
// (the user's data) and, absent the tracked channel, is not re-adopted.
func TestRetireSourceTrackingMigratesOffCheckout(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("RYOKU_REPO", "")
	t.Setenv("RYOKU_WM", "hyprland") // the retire flow edits hypr/user.lua; Detect must not follow the test host's compositor
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))

	clone := filepath.Join(home, "ryoku-arch")
	gitInitRyokuArch(t, clone)

	if err := os.MkdirAll(StateDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	pointer := filepath.Join(StateDir(), "repo")
	if err := os.WriteFile(pointer, []byte(clone+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	envFile := filepath.Join(ConfigHome(), "environment.d", "ryoku.conf")
	if err := os.MkdirAll(filepath.Dir(envFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envFile, []byte("RYOKU_CHANNEL=unstable-dev\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	luaFile := filepath.Join(ConfigHome(), "hypr", "user.lua")
	if err := os.MkdirAll(filepath.Dir(luaFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(luaFile, []byte("-- user config\nhl.env(\"RYOKU_CHANNEL\", \"unstable-dev\")\nhl.keyword(\"misc:vfr\", true)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !SourceTracked() || ResolveRepo() != clone {
		t.Fatalf("precondition: SourceTracked=%v ResolveRepo=%q, want checkout at %q", SourceTracked(), ResolveRepo(), clone)
	}

	if err := RetireSourceTracking(); err != nil {
		t.Fatalf("RetireSourceTracking: %v", err)
	}

	if _, err := os.Stat(pointer); !os.IsNotExist(err) {
		t.Fatalf("repo pointer still present (err=%v)", err)
	}
	if TrackedChannel() != "" {
		t.Fatalf("tracked channel still %q", TrackedChannel())
	}
	if SourceTracked() {
		t.Fatal("still SourceTracked after retire")
	}
	if got := ResolveRepo(); got != "" {
		t.Fatalf("ResolveRepo = %q, want packages (empty)", got)
	}
	if _, err := os.Stat(clone); err != nil {
		t.Fatalf("clone directory removed: %v", err)
	}
	b, err := os.ReadFile(luaFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "RYOKU_CHANNEL") {
		t.Fatalf("user.lua still carries the channel line:\n%s", b)
	}
	if !strings.Contains(string(b), "hl.keyword") {
		t.Fatalf("user.lua lost the user's other lines:\n%s", b)
	}
}

// ResolveRepo's ~/ryoku-arch fallback is gated on the tracked channel: a
// migrated box (no RYOKU_CHANNEL) never re-adopts the clone it left on disk,
// while a source box (a recorded RYOKU_CHANNEL) still self-heals a lost pointer.
func TestResolveRepoFallbackNeedsTracking(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("RYOKU_REPO", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state")) // no recorded pointer

	clone := filepath.Join(home, "ryoku-arch")
	gitInitRyokuArch(t, clone)

	if got := ResolveRepo(); got != "" {
		t.Fatalf("ResolveRepo without tracking = %q, want empty (clone left deliberately)", got)
	}

	envFile := filepath.Join(ConfigHome(), "environment.d", "ryoku.conf")
	if err := os.MkdirAll(filepath.Dir(envFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envFile, []byte("RYOKU_CHANNEL=unstable-dev\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ResolveRepo(); got != clone {
		t.Fatalf("ResolveRepo with tracking = %q, want %q (self-heal a lost pointer)", got, clone)
	}
}
