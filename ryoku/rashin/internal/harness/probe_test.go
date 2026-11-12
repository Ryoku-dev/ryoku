package harness

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
)

// probeSettings sets up one instance of the fake agent, in mode, as the only
// harness, with every Rashin path in fresh folders, signed in for teams.
func probeSettings(t *testing.T, mode string) {
	t.Helper()
	// only bubblewrap stays on PATH, as the sandbox runs it by name
	bin := t.TempDir()
	if bwrap, err := exec.LookPath("bwrap"); err == nil {
		if err := os.Symlink(bwrap, filepath.Join(bin, "bwrap")); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	writeHarnessSettings(t, []protocol.HarnessInstance{{
		ID:      "fake",
		Name:    "Fake agent",
		Command: []string{os.Args[0], "-test.run=^TestACPHelperProcess$"},
		Env:     map[string]string{"RASHIN_ACP_TEST_MODE": mode},
	}})
	signInForTeams(t, "fake", probeTeam)
}

// probeTeam is the team the probe tests check a harness for.
const probeTeam = "team-probe"

// probeBase is a team agent's sandbox without a folder, which still runs this
// test binary from its build folder under /tmp; probes that start an agent
// skip where bubblewrap cannot run.
func probeBase(t *testing.T) sandbox.Spec {
	t.Helper()
	if err := sandbox.Available(); err != nil {
		t.Skipf("bubblewrap cannot run here: %v", err)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return sandbox.Spec{
		RuntimeDir: paths.Resolved(t.TempDir()),
		ReadOnly:   []string{filepath.Dir(paths.Resolved(program))},
		Hidden:     []string{paths.Resolved(paths.StateDir())},
	}
}

func refusedWith(err error, code string) bool {
	var refused *protocol.Error
	return errors.As(err, &refused) && refused.Code == code
}

// A probe starts the harness in its sandbox without a prompt, picks the
// model and options, records the catalog and says what it took; its folder
// is gone afterwards.
func TestAProbeStartsTheHarnessAndSaysWhatItTook(t *testing.T) {
	probeSettings(t, "options")
	base := probeBase(t)
	registry := New()
	learned := 0
	registry.OnLearned(func() { learned++ })
	result, err := registry.Probe(context.Background(), protocol.HarnessProbeParams{ID: "fake", Team: probeTeam, Model: "small", Options: map[string]string{"thinking": "high"}}, base)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Message != "Fake agent started in the sandbox and took Small, thinking High." {
		t.Fatalf("the probe said %+v", result)
	}
	capabilities := result.Harness.TeamCapabilities(probeTeam)
	if result.Harness.ID != "fake" || capabilities == nil || len(capabilities.ModelList) != 2 || capabilities.DefaultModel != "big" || len(capabilities.Options) != 1 {
		t.Fatalf("the probe left the harness reading %+v", result.Harness)
	}
	if result.Harness.Capabilities != nil {
		t.Fatalf("a Check in a team's home wrote the user's own list: %+v", result.Harness.Capabilities)
	}
	if learned == 0 {
		t.Fatal("the probe's catalog was not announced")
	}
	if _, err := os.Stat(probeDir("fake")); !os.IsNotExist(err) {
		t.Fatalf("the probe folder is still there: %v", err)
	}

	result, err = registry.Probe(context.Background(), protocol.HarnessProbeParams{ID: "fake", Team: probeTeam}, base)
	if err != nil || !result.OK || result.Message != "Fake agent started in the sandbox on Big." {
		t.Fatalf("a bare probe said %+v, %v", result, err)
	}
	result, err = registry.Probe(context.Background(), protocol.HarnessProbeParams{ID: "fake", Team: "team-elsewhere"}, base)
	if err != nil || result.OK || !strings.Contains(result.Message, "not signed in") {
		t.Fatalf("a team not signed in was checked: %+v, %v", result, err)
	}
}

// A probe without a team reads the user's own list, as the Agents board's
// Read the list does, and leaves every team's list alone; a team's list
// never stands in for the user's.
func TestAProbeWithoutATeamReadsTheUsersOwnList(t *testing.T) {
	probeSettings(t, "options")
	registry := New()
	info, ok := registry.info(context.Background(), "fake")
	if !ok {
		t.Fatal("no fake harness")
	}
	registry.recordCapabilities("fake", info.Path, probeTeam, protocol.HarnessCapabilities{ModelList: []protocol.Choice{{ID: "team-only"}}, DefaultModel: "team-only"})
	if info, _ := registry.info(context.Background(), "fake"); info.Capabilities != nil {
		t.Fatalf("a team's list shows as the user's: %+v", info.Capabilities)
	}
	result, err := registry.Probe(context.Background(), protocol.HarnessProbeParams{ID: "fake"}, sandbox.Spec{})
	if err != nil || !result.OK || result.Message != "Fake agent started on Big." {
		t.Fatalf("the user's probe said %+v, %v", result, err)
	}
	if own := result.Harness.Capabilities; own == nil || len(own.ModelList) != 2 || own.DefaultModel != "big" {
		t.Fatalf("the user's list reads %+v", own)
	}
	if team := result.Harness.TeamCapabilities(probeTeam); team == nil || team.DefaultModel != "team-only" {
		t.Fatalf("the user's probe changed the team's list: %+v", team)
	}
	if other := result.Harness.TeamCapabilities("team-elsewhere"); other != nil {
		t.Fatalf("a team that never ran the harness reads %+v", other)
	}
}

// An option the harness does not offer fails the probe in words the user
// can act on.
func TestAProbeSaysWhatTheHarnessRefused(t *testing.T) {
	probeSettings(t, "options")
	result, err := New().Probe(context.Background(), protocol.HarnessProbeParams{ID: "fake", Team: probeTeam, Options: map[string]string{"speed": "fast"}}, probeBase(t))
	if err != nil {
		t.Fatal(err)
	}
	if result.OK || !strings.HasPrefix(result.Message, "Fake agent did not take speed fast.") {
		t.Fatalf("a refused option read %+v", result)
	}
}

// A harness that never answers is given up on at the probe's time limit.
func TestAProbeGivesUpOnASilentHarness(t *testing.T) {
	probeSettings(t, "silent")
	base := probeBase(t)
	saved := probeTimeout
	probeTimeout = 300 * time.Millisecond
	t.Cleanup(func() { probeTimeout = saved })
	started := time.Now()
	result, err := New().Probe(context.Background(), protocol.HarnessProbeParams{ID: "fake", Team: probeTeam}, base)
	if err != nil {
		t.Fatal(err)
	}
	if result.OK || !strings.Contains(result.Message, "did not answer within") {
		t.Fatalf("a silent harness read %+v", result)
	}
	if waited := time.Since(started); waited > 10*time.Second {
		t.Fatalf("the probe waited %v", waited)
	}
}

// A probe names a harness on this computer, keeps its picks within limits,
// runs one at a time per harness and does not start what is not installed.
func TestAProbeIsRefusedOrAnsweredWithoutStarting(t *testing.T) {
	probeSettings(t, "options")
	registry := New()
	ctx := context.Background()
	if _, err := registry.Probe(ctx, protocol.HarnessProbeParams{ID: "nowhere"}, sandbox.Spec{}); !refusedWith(err, protocol.CodeBadRequest) {
		t.Fatalf("an unknown harness: %v", err)
	}
	if _, err := registry.Probe(ctx, protocol.HarnessProbeParams{ID: "fake", Model: "two words"}, sandbox.Spec{}); !refusedWith(err, protocol.CodeBadRequest) {
		t.Fatalf("a model id with a space: %v", err)
	}
	if !registry.claimProbe("fake") {
		t.Fatal("the first claim was refused")
	}
	if _, err := registry.Probe(ctx, protocol.HarnessProbeParams{ID: "fake"}, sandbox.Spec{}); !refusedWith(err, protocol.CodeConflict) {
		t.Fatalf("a second probe of one harness: %v", err)
	}
	registry.releaseProbe("fake")
	result, err := registry.Probe(ctx, protocol.HarnessProbeParams{ID: "codex"}, sandbox.Spec{})
	if err != nil || result.OK || result.Message != "Codex is not installed." || result.Harness.ID != "codex" {
		t.Fatalf("an uninstalled harness: %+v, %v", result, err)
	}
	if !registry.claimProbe("codex") {
		t.Fatal("a finished probe kept its harness claimed")
	}
}

// An instance id becomes one folder name under the probe folder, whatever
// it contains.
func TestAProbeFolderStaysInsideTheProbeFolder(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	root := filepath.Join(paths.CacheDir(), "probe")
	for _, id := range []string{"work", "../work", "a/b", "..", "."} {
		dir := probeDir(id)
		if filepath.Dir(dir) != root {
			t.Fatalf("probeDir(%q) = %s, outside %s", id, dir, root)
		}
	}
}
