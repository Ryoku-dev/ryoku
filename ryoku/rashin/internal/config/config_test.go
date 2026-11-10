package config

import (
	"encoding/json"
	"errors"
	"path/filepath"
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

func TestMigrateV1CarriesEnabledHarnessAndApprovals(t *testing.T) {
	s := MigrateV1(rawSettings(t, `{"enabled":true,"optedOut":false,"chatAgent":"codex","approvals":"auto"}`))
	if !s.Enabled || s.Agent.Harness != "codex" || s.Agent.Approvals != protocol.ApprovalsAuto {
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

func TestValidateNormalizesModelAndDropsInvalidInstances(t *testing.T) {
	settings := Defaults()
	settings.Ask.Model = "nonsense"
	settings.Agent.Instances = []protocol.HarnessInstance{
		{ID: "", Command: []string{"agent"}},
		{ID: "empty"},
		{ID: "one", Command: []string{"agent"}},
		{ID: "one", Command: []string{"other"}},
	}
	Validate(&settings)
	if settings.Ask.Model != protocol.RouteAuto {
		t.Fatalf("model = %q", settings.Ask.Model)
	}
	if len(settings.Agent.Instances) != 1 || settings.Agent.Instances[0].ID != "one" {
		t.Fatalf("instances = %#v", settings.Agent.Instances)
	}
}

func TestMergeRejectsInvalidModelsAndInstances(t *testing.T) {
	current := Defaults()
	patches := []string{
		`{"ask":{"model":"nonsense"}}`,
		`{"agent":{"instances":[{"id":"","command":["agent"]}]}}`,
		`{"agent":{"instances":[{"id":"one","command":[]}]}}`,
		`{"agent":{"instances":[{"id":"one","command":["a"]},{"id":"one","command":["b"]}]}}`,
	}
	for _, patch := range patches {
		_, err := Merge(current, json.RawMessage(patch))
		var protocolErr *protocol.Error
		if !errors.As(err, &protocolErr) || protocolErr.Code != protocol.CodeBadRequest {
			t.Fatalf("patch %s error = %#v", patch, err)
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
