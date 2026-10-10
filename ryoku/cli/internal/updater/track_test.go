package updater

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"ryoku-cli/internal/sys"
)

// packagedConf points sys.PacmanConf at a fixture whose [ryoku] Server names
// channel (no [ryoku] stanza when channel==""), plus a temp sync dir, so the
// channel writers and readers stay hermetic under a temp HOME and never touch
// /etc or /var.
func packagedConf(t *testing.T, channel string) {
	t.Helper()
	dir := t.TempDir()
	conf := filepath.Join(dir, "pacman.conf")
	body := "[options]\nHoldPkg = pacman\n"
	if channel != "" {
		body += "\n[ryoku]\nSigLevel = Required\nServer = " + sys.ChannelServer(channel) + "\n"
	}
	if err := os.WriteFile(conf, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_PACMAN_CONF", conf)
	t.Setenv("RYOKU_HOST_PKGMGR", "pacman")
	oldConf, oldSync := sys.PacmanConf, sys.PacmanSyncDir
	sys.PacmanConf = conf
	sys.PacmanSyncDir = t.TempDir()
	t.Cleanup(func() { sys.PacmanConf = oldConf; sys.PacmanSyncDir = oldSync })
}

// initRyokuArchClone makes a git work tree at dir whose origin names ryoku-arch,
// so ResolveRepo's fallback and a recorded pointer both accept it.
func initRyokuArchClone(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, dir, "init")
	mustGit(t, dir, "remote", "add", "origin", "https://github.com/ryoku-dev/ryoku-arch.git")
}

// isolateHome points HOME and the XDG roots at fresh temp dirs and clears
// RYOKU_REPO, so a track test never reads or writes this box's real state.
func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("RYOKU_REPO", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Setenv("RYOKU_CHANNEL", "")
	return home
}

// retargetChannel is the transactional pin: it rewrites the [ryoku] Server to
// the channel, runs the move, and reports where the box landed. A packaged box
// has nothing to migrate.
func TestRetargetChannelPinsServer(t *testing.T) {
	isolateHome(t)
	packagedConf(t, "stable")

	moved := 0
	if err := retargetChannel(sys.ChannelTesting, func() error { moved++; return nil }); err != nil {
		t.Fatalf("retarget to testing: %v", err)
	}
	if moved != 1 {
		t.Fatalf("move ran %d times, want 1", moved)
	}
	if got := sys.PackagedChannel(); got != "testing" {
		t.Fatalf("channel = %q, want testing", got)
	}
	if srv := sys.RyokuServer(); srv != sys.ChannelServer("testing") {
		t.Fatalf("server = %q, want %q", srv, sys.ChannelServer("testing"))
	}
}

// The #291 invariant: when the move fails, the previous [ryoku] Server is put
// back exactly and the databases are re-synced, so the pin never drifts from the
// packages. The old boot guard left the pin on the target after a failed
// downgrade -- this test fails on that behaviour.
func TestRetargetChannelRestoresOnFailure(t *testing.T) {
	isolateHome(t)
	packagedConf(t, "stable")
	before := sys.RyokuServer()
	resynced := false
	stubMoveRepoSync(t, func(force bool) error {
		resynced = resynced || force
		return nil
	})

	err := retargetChannel(sys.ChannelTesting, func() error { return fmt.Errorf("downgrade could not satisfy dependencies") })
	if err == nil {
		t.Fatal("expected the failed move to surface an error")
	}
	if got := sys.RyokuServer(); got != before {
		t.Fatalf("Server = %q after a failed move, want the previous %q restored exactly", got, before)
	}
	if got := sys.PackagedChannel(); got != "stable" {
		t.Fatalf("channel = %q after a failed move, want stable", got)
	}
	if !resynced {
		t.Fatal("restore did not re-sync the databases")
	}
}

// tracking a channel on a box whose updates come from a checkout (a recorded pointer plus
// a RYOKU_CHANNEL=unstable-dev env file) migrates it onto packages: both are
// removed so the update path resolves to packages, not the checkout, while the
// clone directory itself is left on disk. The pin is the caller's separate,
// transactional step, so this touches only the checkout.
func TestMigrateOffCheckout(t *testing.T) {
	home := isolateHome(t)
	packagedConf(t, "testing") // a checkout box with a leftover [ryoku] stanza

	clone := filepath.Join(home, "ryoku-arch")
	initRyokuArchClone(t, clone)
	if err := os.MkdirAll(sys.StateDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sys.StateDir(), "repo"), []byte(clone+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	envFile := filepath.Join(sys.ConfigHome(), "environment.d", "ryoku.conf")
	if err := os.MkdirAll(filepath.Dir(envFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envFile, []byte("RYOKU_CHANNEL=unstable-dev\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !sys.SourceTracked() || sys.ResolveRepo() != clone {
		t.Fatalf("precondition: SourceTracked=%v ResolveRepo=%q, want a checkout at %q", sys.SourceTracked(), sys.ResolveRepo(), clone)
	}

	migrated, err := migrateOffCheckout()
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !migrated {
		t.Fatal("expected a migration off the checkout")
	}
	if _, err := os.Stat(filepath.Join(sys.StateDir(), "repo")); !os.IsNotExist(err) {
		t.Fatalf("repo pointer still present (err=%v)", err)
	}
	if sys.TrackedChannel() != "" {
		t.Fatalf("tracked channel still %q", sys.TrackedChannel())
	}
	if got := sys.ResolveRepo(); got != "" {
		t.Fatalf("ResolveRepo = %q, want packages (empty)", got)
	}
	if _, err := os.Stat(clone); err != nil {
		t.Fatalf("clone directory removed: %v", err)
	}
}
