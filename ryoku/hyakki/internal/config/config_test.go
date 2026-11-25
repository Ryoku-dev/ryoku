package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ryoku-rashin/internal/paths"
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

func TestEnableIfUnsetPreservesPersistedConsent(t *testing.T) {
	cases := []struct {
		name, document string
		enabled        bool
	}{
		{"new installation", "", true},
		{"saved off", `{"version":2,"enabled":false,"unknown":{"keep":"opaque"}}`, false},
		{"current on ignores legacy opt-out", `{"version":2,"enabled":true,"optedOut":true}`, true},
		{"legacy opt-out", `{"enabled":true,"optedOut":true}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isolateConfig(t)
			path := paths.ConfigFile()
			if tc.document != "" {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(tc.document), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			enabled, err := EnableIfUnset(func() error { return nil })
			if err != nil || enabled != tc.enabled {
				t.Fatalf("conditional activation = %v, %v; want %v", enabled, err, tc.enabled)
			}
			settings, err := Load()
			if err != nil || settings.Enabled != tc.enabled {
				t.Fatalf("persisted consent = %v, %v; want %v", settings.Enabled, err, tc.enabled)
			}
			if tc.document != "" {
				after, err := os.ReadFile(path)
				if err != nil || string(after) != tc.document {
					t.Fatalf("existing configuration changed: %q, %v", after, err)
				}
			}
		})
	}
}

func TestEnableIfUnsetReadsConsentAfterWaitingForConfigLock(t *testing.T) {
	isolateConfig(t)
	type result struct {
		enabled bool
		err     error
	}
	started := make(chan struct{})
	done := make(chan result, 1)
	if err := withConfigLock(func() error {
		go func() {
			close(started)
			enabled, err := EnableIfUnset(func() error { return nil })
			done <- result{enabled, err}
		}()
		<-started
		settings := Defaults()
		settings.Enabled = false
		return saveUnlocked(settings)
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-done:
		if got.err != nil || got.enabled {
			t.Fatalf("pending activation replaced off consent: %v, %v", got.enabled, got.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("pending activation did not finish after the config transaction")
	}
	settings, err := Load()
	if err != nil || settings.Enabled {
		t.Fatalf("off consent was not retained: %v, %v", settings.Enabled, err)
	}
}

func TestEnableIfUnsetRejectsInvalidSettingsWithoutReplacingThem(t *testing.T) {
	for _, document := range []string{"null\n", `{"version":2,"enabled":`} {
		t.Run(document, func(t *testing.T) {
			isolateConfig(t)
			path := paths.ConfigFile()
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(document), 0o600); err != nil {
				t.Fatal(err)
			}
			if enabled, err := EnableIfUnset(func() error { return nil }); err == nil || enabled {
				t.Fatalf("invalid settings accepted: %v, %v", enabled, err)
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != document {
				t.Fatalf("invalid configuration replaced: %q, %v", after, err)
			}
		})
	}
}

func TestEnableIfUnsetKeepsFailedActivationRetrySubordinateToOff(t *testing.T) {
	isolateConfig(t)
	failure := errors.New("activation failed")
	activate := func() error { return failure }
	enabled, err := EnableIfUnset(activate)
	if !enabled || !errors.Is(err, failure) {
		t.Fatalf("partial activation = %v, %v", enabled, err)
	}
	settings, err := Load()
	if err != nil || !settings.Enabled {
		t.Fatalf("partial activation lost its saved intent: %v, %v", settings.Enabled, err)
	}
	if err := SetEnabled(false); err != nil {
		t.Fatal(err)
	}
	enabled, err = EnableIfUnset(activate)
	if enabled || err != nil {
		t.Fatalf("retry overrode off or attempted activation: %v, %v", enabled, err)
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

// A fresh install, and a file saved before the key was written, sign in at
// Ryoku's own server with nothing typed.
func TestFreshInstallUsesRyokuAccountServer(t *testing.T) {
	isolateConfig(t)
	if DefaultAccountServer != "https://ai.ryoku.dev" {
		t.Fatalf("DefaultAccountServer = %q", DefaultAccountServer)
	}
	fresh, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Providers.Account != DefaultAccountServer {
		t.Fatalf("fresh install account server = %q, want %q", fresh.Providers.Account, DefaultAccountServer)
	}
	older := fromRaw(rawSettings(t, `{"version":2,"providers":{"models":{"idleMinutes":5}}}`))
	if older.Providers.Account != DefaultAccountServer {
		t.Fatalf("a file without the key gave %q, want %q", older.Providers.Account, DefaultAccountServer)
	}
}

// Turning the account service off is written down, so the default does not
// come back on the next load; null in a patch returns to the default.
func TestClearedAccountServerStaysClearedAcrossReloads(t *testing.T) {
	isolateConfig(t)
	cleared, err := Merge(Defaults(), json.RawMessage(`{"providers":{"account":""}}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SaveMerged(cleared); err != nil {
		t.Fatal(err)
	}
	raw, err := readRaw()
	if err != nil {
		t.Fatal(err)
	}
	var providers map[string]json.RawMessage
	if err := json.Unmarshal(raw["providers"], &providers); err != nil {
		t.Fatal(err)
	}
	if string(providers["account"]) != `""` {
		t.Fatalf("the file keeps account as %s, want an explicit empty string", providers["account"])
	}
	for i := range 2 {
		loaded, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if loaded.Providers.Account != "" {
			t.Fatalf("load %d brought the account server back as %q", i, loaded.Providers.Account)
		}
		if err := Save(loaded); err != nil {
			t.Fatal(err)
		}
	}
	reset, err := Merge(cleared, json.RawMessage(`{"providers":{"account":null}}`))
	if err != nil {
		t.Fatal(err)
	}
	if reset.Providers.Account != DefaultAccountServer {
		t.Fatalf("a null account gave %q, want the default", reset.Providers.Account)
	}
}

// A server a user saved before the default existed is theirs and stays.
func TestSavedCustomAccountServerSurvivesUpgrade(t *testing.T) {
	isolateConfig(t)
	const custom = "https://accounts.example.com"
	loaded := fromRaw(rawSettings(t, `{"version":2,"providers":{"account":"`+custom+`"}}`))
	if loaded.Providers.Account != custom {
		t.Fatalf("custom server loaded as %q", loaded.Providers.Account)
	}
	if err := Save(loaded); err != nil {
		t.Fatal(err)
	}
	again, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if again.Providers.Account != custom {
		t.Fatalf("custom server after a save and a load = %q", again.Providers.Account)
	}
}
