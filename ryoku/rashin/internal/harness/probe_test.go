package harness

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
)

// probeSettings sets up one instance of the fake agent, in mode, as the only
// harness, with every Rashin path in fresh folders.
func probeSettings(t *testing.T, mode string) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	writeHarnessSettings(t, []protocol.HarnessInstance{{
		ID:      "fake",
		Name:    "Fake agent",
		Command: []string{os.Args[0], "-test.run=^TestACPHelperProcess$"},
		Env:     map[string]string{"RASHIN_ACP_TEST_MODE": mode},
	}})
}

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
	result, err := registry.Probe(context.Background(), protocol.HarnessProbeParams{ID: "fake", Model: "small", Options: map[string]string{"thinking": "high"}}, base)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Message != "Fake agent started in the sandbox and took Small, thinking High." {
		t.Fatalf("the probe said %+v", result)
	}
	capabilities := result.Harness.Capabilities
	if result.Harness.ID != "fake" || capabilities == nil || len(capabilities.ModelList) != 2 || capabilities.DefaultModel != "big" || len(capabilities.Options) != 1 {
		t.Fatalf("the probe left the harness reading %+v", result.Harness)
	}
	if learned == 0 {
		t.Fatal("the probe's catalog was not announced")
	}
	if _, err := os.Stat(probeDir("fake")); !os.IsNotExist(err) {
		t.Fatalf("the probe folder is still there: %v", err)
	}

	result, err = registry.Probe(context.Background(), protocol.HarnessProbeParams{ID: "fake"}, base)
	if err != nil || !result.OK || result.Message != "Fake agent started in the sandbox on Big." {
		t.Fatalf("a bare probe said %+v, %v", result, err)
	}
}

// An option the harness does not offer fails the probe in words the user
// can act on.
func TestAProbeSaysWhatTheHarnessRefused(t *testing.T) {
	probeSettings(t, "options")
	result, err := New().Probe(context.Background(), protocol.HarnessProbeParams{ID: "fake", Options: map[string]string{"speed": "fast"}}, probeBase(t))
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
	result, err := New().Probe(context.Background(), protocol.HarnessProbeParams{ID: "fake"}, base)
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
