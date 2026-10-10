package updater

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-cli/internal/host"
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

func captureUpdaterOutput(t *testing.T, run func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()
	run()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestTrackPinsPacmanToUnstableBuild(t *testing.T) {
	home := isolateHome(t)
	packagedConf(t, sys.ChannelStable)
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "pacman"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	oldIntent := sys.ChannelIntentFile
	sys.ChannelIntentFile = filepath.Join(t.TempDir(), "channel-intent")
	t.Cleanup(func() { sys.ChannelIntentFile = oldIntent })
	oldUpdate := runChannelUpdate
	moves := 0
	runChannelUpdate = func() error {
		moves++
		return nil
	}
	t.Cleanup(func() { runChannelUpdate = oldUpdate })

	output := captureUpdaterOutput(t, func() {
		if err := Track(olderUnstableBuild); err != nil {
			t.Fatal(err)
		}
	})
	if moves != 1 || packagedChannel() != olderUnstableBuild {
		t.Fatalf("moves = %d, channel = %q", moves, packagedChannel())
	}
	if intent := sys.ReadChannelIntent(); intent != olderUnstableBuild {
		t.Fatalf("recorded intent = %q, want %q", intent, olderUnstableBuild)
	}
	if !strings.Contains(output, "`ryoku track unstable` follows unstable builds again") {
		t.Fatalf("track output did not explain how to resume unstable: %q", output)
	}
}

func TestReleaseChoicesIncludeUnstableLedgerOnlyForUnstablePins(t *testing.T) {
	stableTag := "v0.75.3-beta.20"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/releases/index.json":
			fmt.Fprintf(w, `{"latest":%q,"releases":[{"tag":%q,"name":"Onogoro","date":"2026-09-03T20:00:00Z"}]}`, stableTag, stableTag)
		case "/channels/testing/index.json":
			fmt.Fprintf(w, `{"latest":%q,"releases":[{"tag":%q,"name":"Onogoro","date":"2026-10-10T12:00:00Z"}]}`, newerUnstableBuild, newerUnstableBuild)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	tests := []struct {
		name, channel, running string
		wantUnstable           bool
	}{
		{name: "stable channel", channel: sys.ChannelStable, running: stableTag},
		{name: "stable release pin", channel: stableTag, running: stableTag},
		{name: "unstable channel", channel: sys.ChannelTesting, running: newerUnstableBuild, wantUnstable: true},
		{name: "unstable build pin", channel: newerUnstableBuild, running: newerUnstableBuild, wantUnstable: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isolateHome(t)
			packagedConf(t, test.channel)
			t.Setenv("RYOKU_RELEASE_BASE", server.URL)
			if err := repoSetURL(host.RepoURLFor(host.Pacman, test.channel)); err != nil {
				t.Fatal(err)
			}
			releaseFile := filepath.Join(t.TempDir(), "ryoku-release")
			if err := os.WriteFile(releaseFile, []byte("RELEASE="+test.running+"\nNAME=Onogoro\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			oldRelease := sys.ReleaseFile
			sys.ReleaseFile = releaseFile
			t.Cleanup(func() { sys.ReleaseFile = oldRelease })

			output := captureUpdaterOutput(t, printReleaseChoices)
			hasUnstable := strings.Contains(output, "UNSTABLE BUILDS")
			if hasUnstable != test.wantUnstable {
				t.Fatalf("unstable block present = %v, want %v:\n%s", hasUnstable, test.wantUnstable, output)
			}
			if test.wantUnstable {
				if !strings.Contains(output, "* "+newerUnstableBuild) {
					t.Fatalf("running unstable build was not marked:\n%s", output)
				}
				if !strings.Contains(output, "rollback --to <build>") || !strings.Contains(output, "track unstable") {
					t.Fatalf("unstable hints missing:\n%s", output)
				}
			}
		})
	}
}

func TestUnstableLedgerReportsEmptyAndUnreachable(t *testing.T) {
	tests := []struct {
		name, body, want string
		status           int
	}{
		{name: "empty", body: `{"latest":"","releases":[]}`, status: http.StatusOK, want: "no frozen unstable builds are published yet"},
		{name: "unreachable", status: http.StatusServiceUnavailable, want: "unstable build ledger is unreachable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isolateHome(t)
			packagedConf(t, sys.ChannelTesting)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()
			t.Setenv("RYOKU_RELEASE_BASE", server.URL)

			output := captureUpdaterOutput(t, printUnstableBuilds)
			if !strings.Contains(output, test.want) {
				t.Fatalf("output = %q, want %q", output, test.want)
			}
		})
	}
}

func TestPackagedStatusUsesUnstableBuildName(t *testing.T) {
	isolateHome(t)
	packagedConf(t, olderUnstableBuild)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"release":%q,"name":"Onogoro"}`, olderUnstableBuild)
	}))
	defer server.Close()
	t.Setenv("RYOKU_RELEASE_BASE", server.URL)
	if err := repoSetURL(host.RepoURLFor(host.Pacman, olderUnstableBuild)); err != nil {
		t.Fatal(err)
	}
	releaseFile := filepath.Join(t.TempDir(), "ryoku-release")
	if err := os.WriteFile(releaseFile, []byte("RELEASE="+olderUnstableBuild+"\nNAME=Onogoro\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldRelease := sys.ReleaseFile
	sys.ReleaseFile = releaseFile
	t.Cleanup(func() { sys.ReleaseFile = oldRelease })

	status := packagedStatus("", "")
	if status.Channel != olderUnstableBuild {
		t.Fatalf("status channel = %q, want build name %q", status.Channel, olderUnstableBuild)
	}
	if status.ChannelPinStale {
		t.Fatal("a pin matching the installed unstable build was reported stale")
	}
}

func TestPackagedStatusDetectsStaleUnstableBuildPin(t *testing.T) {
	isolateHome(t)
	packagedConf(t, olderUnstableBuild)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"release":%q,"name":"Onogoro"}`, olderUnstableBuild)
	}))
	defer server.Close()
	t.Setenv("RYOKU_RELEASE_BASE", server.URL)
	if err := repoSetURL(host.RepoURLFor(host.Pacman, olderUnstableBuild)); err != nil {
		t.Fatal(err)
	}
	releaseFile := filepath.Join(t.TempDir(), "ryoku-release")
	if err := os.WriteFile(releaseFile, []byte("RELEASE="+newerUnstableBuild+"\nNAME=Onogoro\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldRelease, oldIntent := sys.ReleaseFile, sys.ChannelIntentFile
	sys.ReleaseFile = releaseFile
	sys.ChannelIntentFile = filepath.Join(t.TempDir(), "channel-intent")
	if err := os.WriteFile(sys.ChannelIntentFile, []byte(sys.ChannelTesting+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sys.ReleaseFile = oldRelease
		sys.ChannelIntentFile = oldIntent
	})

	status := packagedStatus("", "")
	if !status.ChannelPinStale || status.RecoverChannel != sys.ChannelTesting {
		t.Fatalf("stale pin status = %+v", status)
	}
}
