package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func isolateConfig(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(root, "run"))
}

func rawSettings(t *testing.T, text string) map[string]json.RawMessage {
	t.Helper()
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		t.Fatal(err)
	}
	return raw
}

// Rashin 1's chatAgent named an external harness; only the switch and the
// approvals carry over, and agents start on the router.
func TestMigrateV1CarriesEnabledAndApprovals(t *testing.T) {
	s := MigrateV1(rawSettings(t, `{"enabled":true,"optedOut":false,"chatAgent":"codex","approvals":"auto"}`))
	if !s.Enabled || s.Agent.Approvals != protocol.ApprovalsAuto || s.Agent.Model != protocol.RouteAuto {
		t.Fatalf("migration = %#v", s)
	}
}

func TestMigrateV1RespectsDisabledAndOptedOut(t *testing.T) {
	cases := []string{`{"enabled":false,"optedOut":false}`, `{"enabled":true,"optedOut":true}`}
	for _, input := range cases {
		if MigrateV1(rawSettings(t, input)).Enabled {
			t.Fatalf("migration enabled for %s", input)
		}
	}
}

func TestMergeCannotFlipEnabled(t *testing.T) {
	cur := Defaults()
	cur.Enabled = true
	for _, patch := range []string{`{"enabled":false}`, `null`, `{"enabled":null}`} {
		next, err := Merge(cur, json.RawMessage(patch))
		if err != nil {
			t.Fatal(err)
		}
		if !next.Enabled {
			t.Fatalf("patch %s flipped enabled", patch)
		}
	}
}

func TestSaveMergedPreservesOnDiskEnabled(t *testing.T) {
	isolateConfig(t)
	enabled := Defaults()
	enabled.Enabled = true
	if err := Save(enabled); err != nil {
		t.Fatal(err)
	}
	if err := SetEnabled(false); err != nil {
		t.Fatal(err)
	}
	enabled.Ask.Habits = false
	saved, err := SaveMerged(enabled)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Enabled {
		t.Fatal("settings.set restored enabled")
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Enabled || loaded.Ask.Habits {
		t.Fatalf("saved settings = %#v", loaded)
	}
}

func TestValidateFallsBackFromModelsTheGatewayCannotRoute(t *testing.T) {
	settings := Defaults()
	settings.Ask.Model = "nonsense"
	settings.Agent.Model = "haiku"
	settings.Agent.Options = map[string]string{"": "high"}
	Validate(&settings)
	if settings.Ask.Model != protocol.RouteAuto || settings.Agent.Model != protocol.RouteAuto {
		t.Fatalf("models = %q, %q", settings.Ask.Model, settings.Agent.Model)
	}
	if settings.Agent.Options != nil {
		t.Fatalf("options = %#v", settings.Agent.Options)
	}
}

func TestMergeRejectsModelsTheGatewayCannotRoute(t *testing.T) {
	current := Defaults()
	patches := []string{
		`{"ask":{"model":"nonsense"}}`,
		`{"agent":{"model":"haiku"}}`,
		`{"agent":{"options":{"effort":""}}}`,
	}
	for _, patch := range patches {
		_, err := Merge(current, json.RawMessage(patch))
		var protocolErr *protocol.Error
		if !errors.As(err, &protocolErr) || protocolErr.Code != protocol.CodeBadRequest {
			t.Fatalf("patch %s error = %#v", patch, err)
		}
	}
	next, err := Merge(current, json.RawMessage(`{"agent":{"model":"rashin-models/lfm3-omni-2b","options":{"effort":"low"},"plan":true}}`))
	if err != nil {
		t.Fatal(err)
	}
	if next.Agent.Model != "rashin-models/lfm3-omni-2b" || next.Agent.Options["effort"] != "low" || !next.Agent.Plan {
		t.Fatalf("agent = %#v", next.Agent)
	}
}

// rashin.json stays on version 2 when harnesses go: the keys they used vanish
// on the next save, and a model only a harness understood is not kept.
func TestHarnessSettingsDropOnTheNextSave(t *testing.T) {
	isolateConfig(t)
	old := `{"version":2,"enabled":true,
		"ask":{"model":"auto:fast","approvals":"read-only","harness":"claude"},
		"agent":{"harness":"omp","approvals":"ask","idleStopMinutes":10,"instances":[{"id":"two","command":["omp","acp"]}]},
		"teams":{"defaults":{"harness":"codex","model":"gpt-5.6","approvals":"ask","notify":"all"}}}`
	if err := writeAtomic(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "ryoku", "rashin.json"), []byte(old)); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Enabled || loaded.Ask.Model != protocol.RouteFast || loaded.Agent.Approvals != protocol.ApprovalsAsk {
		t.Fatalf("loaded = %#v", loaded)
	}
	if loaded.Agent.Model != protocol.RouteAuto || loaded.Teams.Defaults.Model != "" {
		t.Fatalf("models = %q, %q", loaded.Agent.Model, loaded.Teams.Defaults.Model)
	}
	if err := Save(loaded); err != nil {
		t.Fatal(err)
	}
	raw, err := readRaw()
	if err != nil {
		t.Fatal(err)
	}
	if string(raw["version"]) != "2" {
		t.Fatalf("version = %s", raw["version"])
	}
	for _, key := range []string{"ask", "agent", "teams"} {
		if text := string(raw[key]); strings.Contains(text, "harness") || strings.Contains(text, "instances") {
			t.Fatalf("%s still holds a harness key: %s", key, text)
		}
	}
}

func TestValidateAcceptsQualifiedNestedModelID(t *testing.T) {
	settings := Defaults()
	settings.Ask.Model = "kilo/nvidia/nemotron:free"
	Validate(&settings)
	if settings.Ask.Model != "kilo/nvidia/nemotron:free" {
		t.Fatalf("model = %q", settings.Ask.Model)
	}
}

func TestLocalAPIIsOffUntilAskedFor(t *testing.T) {
	isolateConfig(t)
	settings, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if settings.API.Enabled || settings.API.Port != protocol.DefaultAPIPort {
		t.Fatalf("a machine that never changed a setting has api = %#v", settings.API)
	}
}

func TestAPISettingsSurviveASave(t *testing.T) {
	isolateConfig(t)
	next, err := Merge(Defaults(), json.RawMessage(`{"api":{"enabled":true,"port":4100}}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(next); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.API.Enabled || loaded.API.Port != 4100 {
		t.Fatalf("api after a save and a load = %#v", loaded.API)
	}
}

func TestAPIPortBelowWhatAUserCanBindIsRefused(t *testing.T) {
	for _, patch := range []string{`{"api":{"port":80}}`, `{"api":{"port":70000}}`} {
		if _, err := Merge(Defaults(), json.RawMessage(patch)); err == nil {
			t.Fatalf("patch %s was accepted", patch)
		}
	}
}
