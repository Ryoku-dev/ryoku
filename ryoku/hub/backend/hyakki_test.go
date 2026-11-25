package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRecordProvisionedWritesEachNameOnce(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := provisionedPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("yazi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := recordProvisioned("hyakki"); err != nil {
			t.Fatal(err)
		}
	}
	if body, err := os.ReadFile(path); err != nil || string(body) != "yazi\nhyakki\n" {
		t.Fatalf("ledger = %q, %v", body, err)
	}
}

type hyakkiFixture struct {
	installed, enabled bool
	migratedEnabled    *bool
	supervisor         string
	units              map[string]bool
	failure            map[string]error
	applyFailure       error
}

func useHyakkiFixture(t *testing.T, installed, enabled bool) *hyakkiFixture {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	oldLook, oldCLI, oldHost := lookPathHyakki, runHyakkiCLI, runHyakkiHost
	oldInstall, oldRemove, oldApply := installHyakkiPackage, removeHyakkiPackage, applyHyakkiDesktop
	t.Cleanup(func() {
		lookPathHyakki, runHyakkiCLI, runHyakkiHost = oldLook, oldCLI, oldHost
		installHyakkiPackage, removeHyakkiPackage, applyHyakkiDesktop = oldInstall, oldRemove, oldApply
	})
	f := &hyakkiFixture{installed: installed, enabled: enabled, supervisor: "systemd", units: map[string]bool{}, failure: map[string]error{}}
	lookPathHyakki = func(string) (string, error) {
		if f.installed {
			return "/usr/bin/hyakki", nil
		}
		return "", os.ErrNotExist
	}
	runHyakkiCLI = func(args ...string) ([]byte, error) {
		switch args[0] {
		case "status":
			return json.Marshal(map[string]bool{"enabled": f.enabled})
		case "on":
			f.enabled = true
		case "off":
			f.enabled = false
		default:
			t.Fatalf("unexpected native command: %v", args)
		}
		return nil, f.failure[args[0]]
	}
	runHyakkiHost = func(args ...string) ([]byte, error) {
		if len(args) == 2 && args[0] == "hyakki" && args[1] == "migrate" {
			if f.failure["migrate"] == nil && f.migratedEnabled != nil {
				f.enabled = *f.migratedEnabled
				f.migratedEnabled = nil
			}
			return nil, f.failure["migrate"]
		}
		if len(args) == 1 && args[0] == "init" {
			return []byte(f.supervisor + "\n"), nil
		}
		if len(args) != 5 || args[0] != "svc" || args[1] != "--user" || args[3] != "--now" {
			t.Fatalf("unexpected host operation: %v", args)
		}
		if err := f.failure[args[2]+" "+args[4]]; err != nil {
			return nil, err
		}
		f.units[args[4]] = args[2] == "enable"
		return nil, nil
	}
	installHyakkiPackage = func() error {
		if err := f.failure["install"]; err != nil {
			return err
		}
		f.installed = true
		return nil
	}
	removeHyakkiPackage = func() error {
		if err := f.failure["remove"]; err != nil {
			return err
		}
		f.installed = false
		return nil
	}
	applyHyakkiDesktop = func() error {
		err := f.applyFailure
		f.applyFailure = nil
		return err
	}
	return f
}

func TestHyakkiOnFailureRestoresPackagePreferenceAndLedger(t *testing.T) {
	for _, before := range []struct{ installed, enabled, legacyEnabled bool }{
		{false, false, false}, {true, false, false}, {true, true, false}, {true, false, true},
	} {
		t.Run(stringBool(before.installed)+stringBool(before.enabled)+stringBool(before.legacyEnabled), func(t *testing.T) {
			f := useHyakkiFixture(t, before.installed, before.enabled)
			if before.legacyEnabled {
				f.migratedEnabled = &before.legacyEnabled
			}
			cause := errors.New("desktop refused")
			f.applyFailure = cause
			if err := hyakkiOn(); !errors.Is(err, cause) {
				t.Fatalf("missing failure: %v", err)
			}
			if f.installed != before.installed || f.enabled != (before.enabled || before.legacyEnabled) {
				t.Fatalf("failed enable changed original state: %+v", f)
			}
			if _, err := os.Stat(provisionedPath()); !os.IsNotExist(err) {
				t.Fatalf("failed enable left a provisioned ledger: %v", err)
			}
		})
	}
}

func stringBool(value bool) string {
	if value {
		return "on-"
	}
	return "off-"
}

func TestHyakkiRefusedRemovalPreservesMigratedPreference(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(stringBool(enabled), func(t *testing.T) {
			f := useHyakkiFixture(t, true, false)
			f.migratedEnabled = &enabled
			cause := errors.New("removal refused")
			f.failure["remove"] = cause
			if err := hyakkiOff(); !errors.Is(err, cause) {
				t.Fatalf("missing removal failure: %v", err)
			}
			if !f.installed || f.enabled != enabled {
				t.Fatalf("refused removal changed preference: %+v", f)
			}
		})
	}
}

func TestHyakkiMigrationFailureLeavesExistingInstallAlone(t *testing.T) {
	for _, on := range []bool{false, true} {
		t.Run(stringBool(on), func(t *testing.T) {
			f := useHyakkiFixture(t, true, true)
			f.failure["migrate"] = errors.New("old app owns its data")
			var err error
			if on {
				err = hyakkiOn()
			} else {
				err = hyakkiOff()
			}
			if err == nil || !f.installed || !f.enabled {
				t.Fatalf("migration refusal changed app state: %+v, %v", f, err)
			}
		})
	}
}

func TestHyakkiRunitLifecycleLeavesOptionalCapabilitiesDormant(t *testing.T) {
	f := useHyakkiFixture(t, false, false)
	f.supervisor = "runit"
	if err := hyakkiOn(); err != nil {
		t.Fatal(err)
	}
	if !f.installed || !f.enabled || !f.units["hyakki-usage"] || f.units["oni"] || f.units["oni-routines"] {
		t.Fatalf("enable armed unrelated workers: %+v", f)
	}
	f.units["oni"], f.units["oni-routines"] = true, true
	if err := hyakkiOff(); err != nil {
		t.Fatal(err)
	}
	if f.installed || f.enabled || f.units["oni"] || f.units["oni-routines"] || f.units["hyakki-usage"] {
		t.Fatalf("disable left app services armed: %+v", f)
	}
}

func TestHyakkiStopFailureStillRemovesThePackage(t *testing.T) {
	f := useHyakkiFixture(t, true, true)
	f.failure["off"] = errors.New("daemon did not stop cleanly")
	if err := hyakkiOff(); err == nil || f.installed {
		t.Fatalf("stop failure hid incomplete teardown or kept package: %+v, %v", f, err)
	}
}
