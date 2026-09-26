package doctor

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReconcileZen(t *testing.T) {
	// The signed theme xpi is absent here, so no extension is installed;
	// TestZenPolicyThemeExtension covers the theme injection.
	origXPI := zenThemeXPI
	zenThemeXPI = filepath.Join(t.TempDir(), "no-theme.xpi")
	defer func() { zenThemeXPI = origXPI }()

	// No Zen install anywhere is a clean no-op, so an update never touches a
	// user who does not have Zen.
	if r := reconcileZenInto([]string{filepath.Join(t.TempDir(), "absent")}, false); r.status != recOK {
		t.Fatalf("absent Zen: status %v, want ok", r.status)
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "application.ini"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(root, "distribution", "policies.json")

	// Check-only reports the pending apply and writes nothing.
	if r := reconcileZenInto([]string{root}, true); r.status != recWouldFix {
		t.Fatalf("check-only: status %v, want todo", r.status)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatal("check-only wrote the policy; it must only report")
	}

	// A packages-owned policies.json is already in place with the Zen packager's
	// own keys; the reconciler must keep them, not overwrite the file.
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	packager := []byte(`{"policies":{"DisableAppUpdate":true,"DefaultSerialGuardSetting":3}}`)
	if err := os.WriteFile(dst, packager, 0o644); err != nil {
		t.Fatal(err)
	}

	// Apply merges the Ryoku policy onto the packager's file.
	if r := reconcileZenInto([]string{root}, false); r.status != recFixed {
		t.Fatalf("apply: status %v, want fixed", r.status)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("policy not written: %v", err)
	}
	var doc struct {
		Policies struct {
			DisableAppUpdate          bool           `json:"DisableAppUpdate"`
			DefaultSerialGuardSetting int            `json:"DefaultSerialGuardSetting"`
			DisableTelemetry          bool           `json:"DisableTelemetry"`
			ExtensionSettings         map[string]any `json:"ExtensionSettings"`
			Preferences               map[string]any `json:"Preferences"`
		} `json:"policies"`
	}
	if err := json.Unmarshal(got, &doc); err != nil {
		t.Fatalf("policy is not valid JSON: %v", err)
	}
	// The packager's own keys survive the merge.
	if !doc.Policies.DisableAppUpdate || doc.Policies.DefaultSerialGuardSetting != 3 {
		t.Fatal("merge discarded the Zen packager's own policies")
	}
	// Ryoku's own prefs are applied.
	if !doc.Policies.DisableTelemetry || len(doc.Policies.Preferences) == 0 {
		t.Fatal("Ryoku policy keys were not applied")
	}
	// No signed theme xpi here, so no extensions are installed at all: the two
	// privacy extensions are gone, and the theme is only added with its xpi.
	if len(doc.Policies.ExtensionSettings) != 0 {
		t.Fatalf("unexpected extensions installed: %v", doc.Policies.ExtensionSettings)
	}

	// A second run changes nothing (idempotent).
	if r := reconcileZenInto([]string{root}, false); r.status != recOK {
		t.Fatalf("second apply: status %v, want ok (idempotent)", r.status)
	}

	// A drifted policy is reconverged.
	if err := os.WriteFile(dst, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := reconcileZenInto([]string{root}, false); r.status != recFixed {
		t.Fatalf("stale rewrite: status %v, want fixed", r.status)
	}
}

func TestReconcileZenIgnoresLauncherDirectory(t *testing.T) {
	root := t.TempDir()
	launcher := filepath.Join(root, "zen-browser")
	if err := os.WriteFile(launcher, []byte("#!/bin/sh\nexec /opt/zen/zen-bin \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if r := reconcileZenInto([]string{root}, false); r.status != recOK {
		t.Fatalf("launcher directory: status %v, want ok", r.status)
	}
	if _, err := os.Stat(filepath.Join(root, "distribution")); !os.IsNotExist(err) {
		t.Fatal("launcher directory was modified as if it were a Zen install")
	}
}

func TestZenPolicyThemeExtension(t *testing.T) {
	orig := zenThemeXPI
	defer func() { zenThemeXPI = orig }()

	// No signed xpi: the palette-follow theme extension is omitted.
	zenThemeXPI = filepath.Join(t.TempDir(), "absent.xpi")
	if bytes.Contains(zenPolicyBytes(), []byte("ryoku-theme@ryoku.arch")) {
		t.Fatal("theme extension present without a signed xpi")
	}

	// Signed xpi present: it is added to ExtensionSettings from the local file.
	xpi := filepath.Join(t.TempDir(), "ryoku-theme.xpi")
	if err := os.WriteFile(xpi, []byte("PK\x03\x04"), 0o644); err != nil {
		t.Fatal(err)
	}
	zenThemeXPI = xpi
	var doc struct {
		Policies struct {
			ExtensionSettings map[string]struct {
				InstallationMode string `json:"installation_mode"`
				InstallURL       string `json:"install_url"`
			} `json:"ExtensionSettings"`
		} `json:"policies"`
	}
	if err := json.Unmarshal(zenPolicyBytes(), &doc); err != nil {
		t.Fatalf("policy with theme ext is not valid JSON: %v", err)
	}
	e, ok := doc.Policies.ExtensionSettings["ryoku-theme@ryoku.arch"]
	if !ok {
		t.Fatal("theme extension missing when the signed xpi is present")
	}
	if e.InstallURL != "file://"+xpi {
		t.Fatalf("theme install_url = %q, want file://%s", e.InstallURL, xpi)
	}
	// The theme is the only extension Ryoku installs; the two privacy extensions
	// that used to ship here are gone.
	if len(doc.Policies.ExtensionSettings) != 1 {
		t.Fatalf("unexpected extensions alongside the theme: %v", doc.Policies.ExtensionSettings)
	}
}
