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
	if strings.Join(called, ",") != "status --json,off,remove,desktop" {
		t.Fatalf("calls = %v", called)
	}
}

// fakeRashin swaps every seam rashinOn/rashinOff use for a recorder. installed
// is the package state; fail maps a recorded call to the error it returns.
func fakeRashin(t *testing.T, installed *bool, enabled bool, fail map[string]error) *[]string {
	t.Helper()
	oldLook, oldRun := lookPathRashin, runRashinCLI
	oldInstall, oldRemove, oldApply := installRashinPackage, removeRashinPackage, applyRashinDesktop
	t.Cleanup(func() {
		lookPathRashin, runRashinCLI = oldLook, oldRun
		installRashinPackage, removeRashinPackage, applyRashinDesktop = oldInstall, oldRemove, oldApply
	})
	called := &[]string{}
	record := func(call string) error {
		*called = append(*called, call)
		return fail[call]
	}
	lookPathRashin = func(string) (string, error) {
		if *installed {
			return "/usr/bin/rashin", nil
		}
		return "", errors.New("not found")
	}
	runRashinCLI = func(args ...string) ([]byte, error) {
		call := strings.Join(args, " ")
		if err := record(call); err != nil {
			return []byte(err.Error()), err
		}
		if call == "status --json" {
			if enabled {
				return []byte(`{"enabled":true}`), nil
			}
			return []byte(`{"enabled":false}`), nil
		}
		return nil, nil
	}
	installRashinPackage = func() error {
		if err := record("install"); err != nil {
			return err
		}
		*installed = true
		return nil
	}
	removeRashinPackage = func() error {
		if err := record("remove"); err != nil {
			return err
		}
		*installed = false
		return nil
	}
	applyRashinDesktop = func() error { return record("desktop") }
	return called
}

// A failed last step of `on` undoes the install, the ledger line and the
// switch, and says nothing is left, so Retry starts from where the user was.
func TestRashinOnFailureRollsBackEverything(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	installed := false
	called := fakeRashin(t, &installed, false, nil)
	// only the first desktop pass fails; the rollback's own pass goes through
	applyRashinDesktop = func() error {
		*called = append(*called, "desktop")
		if len(*called) == 3 {
			return errors.New("compositor refused")
		}
		return nil
	}

	err := rashinOn()
	if err == nil || !strings.Contains(err.Error(), "compositor refused") || !strings.Contains(err.Error(), "left as it was") {
		t.Fatalf("error = %v, want the cause and that nothing is left", err)
	}
	if got := strings.Join(*called, ","); got != "install,on,desktop,off,remove,desktop" {
		t.Fatalf("calls = %s", got)
	}
	if installed {
		t.Fatal("the package installed for this switch is still installed")
	}
	if _, err := os.Stat(filepath.Join(stateHome(), "ryoku", "provisioned")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("provisioned ledger left behind: %v", err)
	}
}

// What a rollback cannot undo is named, so the page never claims a clean state.
func TestRashinOnFailureNamesWhatIsLeft(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	installed := false
	called := fakeRashin(t, &installed, false, map[string]error{
		"on":     errors.New("socket unit missing"),
		"remove": errors.New("the password prompt was dismissed"),
	})

	err := rashinOn()
	if err == nil || !strings.Contains(err.Error(), "socket unit missing") ||
		!strings.Contains(err.Error(), "ryoku-rashin is still installed (the password prompt was dismissed)") {
		t.Fatalf("error = %v, want the cause and the package left installed", err)
	}
	if got := strings.Join(*called, ","); got != "install,on,off,remove" {
		t.Fatalf("calls = %s", got)
	}
}

// A refused removal leaves Rashin installed, so it goes back on rather than
// staying installed and silently switched off.
func TestRashinOffRefusedRemovalTurnsItBackOn(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	installed := true
	called := fakeRashin(t, &installed, true, map[string]error{"remove": errors.New("the password prompt was dismissed")})

	err := rashinOff()
	if err == nil || !strings.Contains(err.Error(), "still installed and on, as before") {
		t.Fatalf("error = %v, want the state left named", err)
	}
	if got := strings.Join(*called, ","); got != "status --json,off,remove,on" {
		t.Fatalf("calls = %s", got)
	}
}

// Rashin that was installed but off stays off when the removal is refused.
func TestRashinOffRefusedRemovalKeepsAnOffRashinOff(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	installed := true
	called := fakeRashin(t, &installed, false, map[string]error{"remove": errors.New("the password prompt was dismissed")})

	err := rashinOff()
	if err == nil || !strings.Contains(err.Error(), "still installed and off, as before") {
		t.Fatalf("error = %v, want the state left named", err)
	}
	if got := strings.Join(*called, ","); got != "status --json,off,remove" {
		t.Fatalf("calls = %s", got)
	}
}
