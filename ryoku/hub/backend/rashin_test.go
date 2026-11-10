package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The ledger records a name once, so repeated switching never pads the file,
// and a name already there from the installer is left where it is.
func TestRecordProvisionedWritesEachNameOnce(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	ledger := filepath.Join(state, "ryoku", "provisioned")
	if err := os.MkdirAll(filepath.Dir(ledger), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ledger, []byte("yazi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := recordProvisioned("ryoku-rashin"); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "yazi\nryoku-rashin\n" {
		t.Fatalf("ledger = %q", got)
	}
}

func TestRecordProvisionedCreatesTheLedger(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	if err := recordProvisioned("ryoku-rashin"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(state, "ryoku", "provisioned"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "ryoku-rashin\n" {
		t.Fatalf("ledger = %q", got)
	}
}

func TestRashinOnInstallsRecordsAndEnables(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	oldLook, oldRun := lookPathRashin, runRashinCLI
	oldInstall, oldApply := installRashinPackage, applyRashinDesktop
	t.Cleanup(func() {
		lookPathRashin, runRashinCLI = oldLook, oldRun
		installRashinPackage, applyRashinDesktop = oldInstall, oldApply
	})

	installed := false
	called := []string{}
	lookPathRashin = func(name string) (string, error) {
		if installed {
			return "/usr/bin/rashin", nil
		}
		return "", errors.New("not found")
	}
	installRashinPackage = func() error {
		called = append(called, "install")
		installed = true
		return nil
	}
	runRashinCLI = func(args ...string) ([]byte, error) {
		called = append(called, strings.Join(args, " "))
		return nil, nil
	}
	applyRashinDesktop = func() error {
		called = append(called, "desktop")
		return nil
	}

	if err := rashinOn(); err != nil {
		t.Fatal(err)
	}
	if strings.Join(called, ",") != "install,on,desktop" {
		t.Fatalf("calls = %v", called)
	}
	if _, err := os.Stat(filepath.Join(stateHome(), "ryoku", "provisioned")); err != nil {
		t.Fatal(err)
	}
}

func TestRashinOffRemovesPackageAfterStopFailure(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	oldLook, oldRun := lookPathRashin, runRashinCLI
	oldRemove, oldApply := removeRashinPackage, applyRashinDesktop
	t.Cleanup(func() {
		lookPathRashin, runRashinCLI = oldLook, oldRun
		removeRashinPackage, applyRashinDesktop = oldRemove, oldApply
	})

	called := []string{}
	lookPathRashin = func(name string) (string, error) { return "/usr/bin/rashin", nil }
	runRashinCLI = func(args ...string) ([]byte, error) {
		called = append(called, strings.Join(args, " "))
		return []byte("daemon did not answer"), errors.New("exit 1")
	}
	removeRashinPackage = func() error {
		called = append(called, "remove")
		return nil
	}
	applyRashinDesktop = func() error {
		called = append(called, "desktop")
		return nil
	}

	err := rashinOff()
	if err == nil || !strings.Contains(err.Error(), "daemon did not answer") {
		t.Fatalf("error = %v", err)
	}
	if strings.Join(called, ",") != "off,remove,desktop" {
		t.Fatalf("calls = %v", called)
	}
}
