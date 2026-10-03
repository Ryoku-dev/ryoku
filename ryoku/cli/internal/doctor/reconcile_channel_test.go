package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-cli/internal/sys"
)

func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// A packaged install has no update-channel checkout, so there is nothing to
// reconcile and the step is a no-op.
func TestReconcileUpdateChannelNoCheckout(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("RYOKU_REPO", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Setenv("RYOKU_CHANNEL", "")
	if r := reconcileUpdateChannel(false); r.status != recOK {
		t.Fatalf("packaged box: got %s (%s), want ok", r.status.label(), r.detail)
	}
}

// The core heal: a checkout left on the wrong branch (tracked unstable-dev, but
// sitting on main after a track toggle) is put back on the tracked branch, so
// `ryoku update` stops measuring against the wrong channel.
func TestReconcileUpdateChannelSwitchesToTrackedBranch(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work")
	if out, err := exec.Command("git", "init", "--bare", "-b", "main", origin).CombinedOutput(); err != nil {
		t.Fatalf("init bare: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "clone", origin, work).CombinedOutput(); err != nil {
		t.Fatalf("clone: %v\n%s", err, out)
	}
	gitT(t, work, "config", "user.email", "t@e")
	gitT(t, work, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(work, "f"), []byte("main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitT(t, work, "add", "-A")
	gitT(t, work, "commit", "-m", "main")
	gitT(t, work, "push", "origin", "main")
	gitT(t, work, "checkout", "-b", "unstable-dev")
	if err := os.WriteFile(filepath.Join(work, "f"), []byte("dev\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitT(t, work, "add", "-A")
	gitT(t, work, "commit", "-m", "dev")
	gitT(t, work, "push", "origin", "unstable-dev")
	gitT(t, work, "checkout", "main") // the "separated" state: on main, tracking unstable-dev

	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	if err := os.MkdirAll(filepath.Join(cfg, "environment.d"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "environment.d", "ryoku.conf"), []byte("RYOKU_CHANNEL=unstable-dev\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_REPO", work)

	if r := reconcileUpdateChannel(false); r.status != recFixed {
		t.Fatalf("got %s (%s), want fixed", r.status.label(), r.detail)
	}
	if head := strings.TrimSpace(gitT(t, work, "symbolic-ref", "--short", "HEAD")); head != "unstable-dev" {
		t.Fatalf("checkout on %q, want unstable-dev", head)
	}
}

// mkRyokuArch makes a git work tree at dst whose origin path contains
// "ryoku-arch", so isRyokuArchTree accepts it.
func mkRyokuArch(t *testing.T, root, dst string) {
	t.Helper()
	origin := filepath.Join(root, "ryoku-arch.git")
	if out, err := exec.Command("git", "init", "--bare", "-b", "main", origin).CombinedOutput(); err != nil {
		t.Fatalf("init bare: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "clone", origin, dst).CombinedOutput(); err != nil {
		t.Fatalf("clone: %v\n%s", err, out)
	}
	gitT(t, dst, "config", "user.email", "t@e")
	gitT(t, dst, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dst, "f"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitT(t, dst, "add", "-A")
	gitT(t, dst, "commit", "-m", "c")
}

// A symlinked state dir (the retired dev-switch layout) becomes a real
// directory, migrating whatever the target held.
func TestReconcileRepoPointerDesymlinks(t *testing.T) {
	t.Setenv("RYOKU_REPO", "")
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home")) // no ~/ryoku-arch: isolate the de-symlink
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	state := sys.StateDir()
	target := filepath.Join(root, "devswitch", "state", "ryoku")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "repo"), []byte("/gone\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(state), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, state); err != nil {
		t.Fatal(err)
	}
	if r := reconcileRepoPointer(false); r.status != recFixed {
		t.Fatalf("got %s (%s), want fixed", r.status.label(), r.detail)
	}
	if fi, err := os.Lstat(state); err != nil || fi.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("state dir still a symlink (err=%v)", err)
	}
	if b, err := os.ReadFile(filepath.Join(state, "repo")); err != nil || strings.TrimSpace(string(b)) != "/gone" {
		t.Fatalf("migrated pointer missing (%q, err=%v)", b, err)
	}
}

// A stale pointer beside a healthy ~/ryoku-arch checkout is repointed.
func TestReconcileRepoPointerRepoints(t *testing.T) {
	t.Setenv("RYOKU_REPO", "")
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	state := sys.StateDir()
	if err := os.MkdirAll(state, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "repo"), []byte("/gone\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	track := filepath.Join(home, "ryoku-arch")
	mkRyokuArch(t, root, track)
	// the box still opts into source tracking (a recorded RYOKU_CHANNEL), so a
	// lost pointer self-heals to the clone.
	writeTrackedChannel(t, home, "unstable-dev")
	if r := reconcileRepoPointer(true); r.status != recWouldFix {
		t.Fatalf("checkOnly: got %s, want would-fix", r.status.label())
	}
	if r := reconcileRepoPointer(false); r.status != recFixed {
		t.Fatalf("got %s (%s), want fixed", r.status.label(), r.detail)
	}
	if b, err := os.ReadFile(filepath.Join(state, "repo")); err != nil || strings.TrimSpace(string(b)) != track {
		t.Fatalf("pointer = %q (err=%v), want %s", b, err, track)
	}
}

// A box migrated onto packages carries no tracked channel and left its
// ~/ryoku-arch clone on disk deliberately; the pointer reconciler must not
// re-adopt it, or the doctor would undo the migration.
func TestReconcileRepoPointerLeavesMigratedClone(t *testing.T) {
	t.Setenv("RYOKU_REPO", "")
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config")) // no environment.d channel
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	state := sys.StateDir()
	if err := os.MkdirAll(state, 0o755); err != nil {
		t.Fatal(err)
	}
	track := filepath.Join(home, "ryoku-arch")
	mkRyokuArch(t, root, track) // the clone is present but not tracked
	if r := reconcileRepoPointer(false); r.status != recOK {
		t.Fatalf("got %s (%s), want ok (clone left alone)", r.status.label(), r.detail)
	}
	if _, err := os.Stat(filepath.Join(state, "repo")); !os.IsNotExist(err) {
		t.Fatalf("pointer was recorded (err=%v); the migrated clone must not be re-adopted", err)
	}
}

// writeTrackedChannel records ch in home's environment.d, the source-tracking
// opt-in ResolveRepo's fallback and the pointer reconciler gate on.
func writeTrackedChannel(t *testing.T, home, ch string) {
	t.Helper()
	dir := filepath.Join(home, ".config", "environment.d")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ryoku.conf"), []byte("RYOKU_CHANNEL="+ch+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A real state dir whose pointer is a healthy ryoku-arch checkout is left alone.
func TestReconcileRepoPointerHealthy(t *testing.T) {
	t.Setenv("RYOKU_REPO", "")
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	state := sys.StateDir()
	if err := os.MkdirAll(state, 0o755); err != nil {
		t.Fatal(err)
	}
	track := filepath.Join(home, "ryoku-arch")
	mkRyokuArch(t, root, track)
	if err := os.WriteFile(filepath.Join(state, "repo"), []byte(track+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := reconcileRepoPointer(false); r.status != recOK {
		t.Fatalf("got %s (%s), want ok", r.status.label(), r.detail)
	}
}
