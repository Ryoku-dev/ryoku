package harness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
)

func TestListUsesTableOrderConfiguredDefaultAndVersionCache(t *testing.T) {
	bin := t.TempDir()
	configHome := t.TempDir()
	t.Setenv("PATH", bin)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	versionFile := filepath.Join(bin, "version")
	if err := os.WriteFile(versionFile, []byte("one\n"), 0600); err != nil {
		t.Fatal(err)
	}
	codex := filepath.Join(bin, "codex-acp")
	writeHarnessScript(t, codex, versionFile)
	writeHarnessScript(t, filepath.Join(bin, "omp"), versionFile)
	settingsDir := filepath.Join(configHome, "ryoku")
	if err := os.MkdirAll(settingsDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(settingsDir, "rashin.json"), []byte(`{"version":2,"agent":{"harness":"omp","approvals":"read-only","idleStopMinutes":10}}`), 0600); err != nil {
		t.Fatal(err)
	}
	registry := New()
	first, err := registry.List(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if first[1].ID != "codex" || !first[1].Installed || first[1].Version != "one" {
		t.Fatalf("codex discovery failed: %+v", first[1])
	}
	if !first[6].Default {
		t.Fatalf("configured installed harness was not default: %+v", first[6])
	}
	if err := os.WriteFile(versionFile, []byte("two\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cached, err := registry.List(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if cached[1].Version != "one" {
		t.Fatalf("version cache ignored an unchanged binary: %q", cached[1].Version)
	}
	stamp := time.Now().Add(3 * time.Second)
	if err := os.Chtimes(codex, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	changed, err := registry.List(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if changed[1].Version != "two" {
		t.Fatalf("version command was not rerun after binary changed: %q", changed[1].Version)
	}
}

func TestConfiguredMissingHarnessFallsBackInTableOrder(t *testing.T) {
	bin := t.TempDir()
	configHome := t.TempDir()
	t.Setenv("PATH", bin)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	writeHarnessScript(t, filepath.Join(bin, "opencode"), filepath.Join(bin, "unused"))
	settingsDir := filepath.Join(configHome, "ryoku")
	if err := os.MkdirAll(settingsDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(settingsDir, "rashin.json"), []byte(`{"version":2,"agent":{"harness":"claude","approvals":"read-only","idleStopMinutes":10}}`), 0600); err != nil {
		t.Fatal(err)
	}
	list, err := New().List(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if !list[2].Default || list[2].ID != "opencode" {
		t.Fatalf("first installed harness was not selected: %+v", list)
	}
}

func TestCustomInstancesListCollisionAndPassEnvironment(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	writeHarnessSettings(t, []protocol.HarnessInstance{
		{
			ID:      "work",
			Name:    "Work account",
			Command: []string{os.Args[0], "-test.run=^TestACPHelperProcess$"},
			Env: map[string]string{
				"RASHIN_ACP_TEST_MODE": "environment",
				"RASHIN_CUSTOM_VALUE":  "from-instance",
			},
		},
		{ID: "codex", Name: "Collision", Command: []string{os.Args[0]}},
	})
	registry := New()
	list, err := registry.List(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	custom := harnessByID(list[len(definitions):], "work")
	executable, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	if custom == nil || !custom.Custom || !custom.Installed || custom.Command[0] != executable {
		t.Fatalf("custom harness was not listed as installed: %+v", custom)
	}
	collision := harnessByID(list[len(definitions):], "codex")
	if collision == nil || collision.Installed || !strings.Contains(collision.Install, "conflicts") {
		t.Fatalf("built-in collision was not explained: %+v", collision)
	}
	agent, err := registry.NewAgent(context.Background(), core.HarnessOptions{
		Harness: "work",
		Cwd:     t.TempDir(),
		Approvals: func() string {
			return protocol.ApprovalsAuto
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = agent.Stop() })
	sink := newTestSink()
	if _, err := agent.Prompt(context.Background(), core.PromptInput{Text: "environment"}, sink); err != nil {
		t.Fatal(err)
	}
	text := sink.waitKind(t, protocol.EvText, 1)[0].data.(protocol.TextData)
	if text.Delta != "from-instance" {
		t.Fatalf("custom environment did not reach agent: %q", text.Delta)
	}
}

func TestCapabilitiesPersistAndListForUnchangedBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	writeHarnessSettings(t, []protocol.HarnessInstance{{
		ID:      "capable",
		Name:    "Capable",
		Command: []string{os.Args[0], "-test.run=^TestACPHelperProcess$"},
		Env:     map[string]string{"RASHIN_ACP_TEST_MODE": "capabilities"},
	}})
	registry := New()
	agent, err := registry.NewAgent(context.Background(), core.HarnessOptions{
		Harness: "capable",
		Cwd:     t.TempDir(),
		Approvals: func() string {
			return protocol.ApprovalsAuto
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	sink := newTestSink()
	if _, err := agent.Prompt(context.Background(), core.PromptInput{Text: "capabilities"}, sink); err != nil {
		t.Fatal(err)
	}
	if err := agent.Stop(); err != nil {
		t.Fatal(err)
	}
	list, err := New().List(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	info := harnessByID(list, "capable")
	if info == nil || info.Capabilities == nil {
		t.Fatalf("persisted capabilities were not listed: %+v", info)
	}
	capabilities := info.Capabilities
	if !capabilities.LoadSession || !capabilities.Images || !capabilities.Audio || !capabilities.Context ||
		!capabilities.Models || len(capabilities.Modes) != 1 || capabilities.Modes[0].ID != "code" {
		t.Fatalf("capabilities were incomplete: %+v", capabilities)
	}
}

func writeHarnessSettings(t *testing.T, instances []protocol.HarnessInstance) {
	t.Helper()
	settings := map[string]any{
		"version": 2,
		"agent": map[string]any{
			"approvals":       protocol.ApprovalsReadOnly,
			"idleStopMinutes": 10,
			"instances":       instances,
		},
	}
	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "ryoku", "rashin.json"))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rashin.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func harnessByID(list []protocol.HarnessInfo, id string) *protocol.HarnessInfo {
	for i := range list {
		if list[i].ID == id {
			return &list[i]
		}
	}
	return nil
}

func writeHarnessScript(t *testing.T, path, versionFile string) {
	t.Helper()
	script := "#!/bin/sh\nif [ \"$1\" = --version ]; then read value < \"" + versionFile + "\" 2>/dev/null || value=unknown; printf %s \"$value\"; fi\n"
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().Add(time.Second)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
}
