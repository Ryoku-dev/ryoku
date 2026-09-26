package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-cli/internal/updater"

	wm "ryoku-wm"
)

// swapMaterialize swaps the delivery for the test's own and restores it after.
func swapMaterialize(t *testing.T, fn func() error) {
	t.Helper()
	prev := materializeNow
	materializeNow = fn
	t.Cleanup(func() { materializeNow = prev })
}

// swapSessionLive declares the session live without a live compositor.
func swapSessionLive(t *testing.T, live bool) {
	t.Helper()
	prev := sessionLive
	sessionLive = func() bool { return live }
	t.Cleanup(func() { sessionLive = prev })
}

// swapStartSession keeps a test from restarting the desktop it runs on.
func swapStartSession(t *testing.T, fn func() error) {
	t.Helper()
	prev := startSession
	startSession = fn
	t.Cleanup(func() { startSession = prev })
}

// The reconciler must notice a compositor whose Ryoku tree is not laid down -- a
// package install leaves it bare, and a switch to the other compositor used to
// prune it -- lay it with the same materialize the installer runs, and start a
// live session that already booted without it.

// configTreeFixture: a packaged base that ships the given provider trees, and an
// empty ~/.config, i.e. a box right after `pacman -S`.
func configTreeFixture(t *testing.T, providers ...string) (home, base string) {
	t.Helper()
	home, base = t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Setenv("RYOKU_CONFIG_BASE", base)
	// RYOKU_WM is the seam's own declaration and outranks a live socket in the
	// environment (a developer's session), so each test names the one it means.
	t.Setenv("RYOKU_WM", wm.ProviderNiri)
	for _, name := range providers {
		entry := wm.ConfigEntry(name)
		if entry == "" {
			t.Fatalf("no config entry point for %q", name)
		}
		full := filepath.Join(base, entry)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("-- shipped\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return home, base
}

func laidDown(home, entry string) bool {
	_, err := os.Stat(filepath.Join(home, ".config", entry))
	return err == nil
}

// A checkout box has no packaged base: nothing to compare, nothing to nag about.
func TestReconcileConfigTreeOnACheckoutBoxIsOK(t *testing.T) {
	home, _ := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("RYOKU_CONFIG_BASE", filepath.Join(home, "no-such-base"))
	t.Setenv("RYOKU_WM", wm.ProviderNiri)
	called := false
	swapMaterialize(t, func() error { called = true; return nil })

	if r := reconcileConfigTree(false); r.status != recOK {
		t.Fatalf("status=%s detail=%q, want ok", r.status.label(), r.detail)
	}
	if called {
		t.Fatal("a box with no packaged base must not materialize")
	}
}

// The reported case: a package install, then a session on a bare compositor. The
// check names the live compositor first, and the fix lays the tree and brings the
// session up.
func TestReconcileConfigTreeBareLiveSessionIsFixed(t *testing.T) {
	home, _ := configTreeFixture(t, wm.ProviderNiri, wm.ProviderHyprland)
	t.Setenv("RYOKU_WM", wm.ProviderNiri)
	restarts := 0
	swapMaterialize(t, updater.Materialize)
	swapStartSession(t, func() error { restarts++; return nil })
	swapSessionLive(t, true)

	r := reconcileConfigTree(true)
	if r.status != recWouldFix {
		t.Fatalf("check: status=%s detail=%q, want would-fix", r.status.label(), r.detail)
	}
	if !strings.Contains(r.detail, "niri") || !strings.Contains(r.detail, "hyprland") {
		t.Errorf("check detail should name both trees, got %q", r.detail)
	}
	if laidDown(home, "niri/config.kdl") {
		t.Fatal("check-only must not write")
	}

	if r := reconcileConfigTree(false); r.status != recFixed {
		t.Fatalf("fix: status=%s detail=%q, want fixed", r.status.label(), r.detail)
	}
	if !laidDown(home, "niri/config.kdl") || !laidDown(home, "hypr/hyprland.lua") {
		t.Fatal("both installed trees should be laid down")
	}
	if restarts != 1 {
		t.Fatalf("a live bare session should be brought up once, got %d", restarts)
	}
}

func TestReconcileConfigTreeReportsGuardedSessionStartFailure(t *testing.T) {
	_, _ = configTreeFixture(t, wm.ProviderNiri)
	t.Setenv("RYOKU_WM", wm.ProviderNiri)
	swapMaterialize(t, updater.Materialize)
	swapStartSession(t, func() error { return errors.New("sleep guard unavailable") })
	swapSessionLive(t, true)

	r := reconcileConfigTree(false)
	if r.status != recFailed {
		t.Fatalf("status=%s detail=%q, want failed", r.status.label(), r.detail)
	}
	if !strings.Contains(r.detail, "sleep guard unavailable") {
		t.Fatalf("failure detail did not preserve startup error: %q", r.detail)
	}
}

// Only the switched-away tree is missing: heal it, and leave the live session
// alone (its shell is running).
func TestReconcileConfigTreeHealsTheSwitchedAwayTreeOnly(t *testing.T) {
	home, base := configTreeFixture(t, wm.ProviderNiri, wm.ProviderHyprland)
	t.Setenv("RYOKU_WM", wm.ProviderHyprland)
	if err := os.MkdirAll(filepath.Join(home, ".config", "hypr"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".config", "hypr", "hyprland.lua"), []byte("-- live tree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(base, "niri")); err != nil { // niri's variant is still installed
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(base, "niri"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "niri", "config.kdl"), []byte("-- shipped\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	restarts := 0
	swapMaterialize(t, updater.Materialize)
	swapStartSession(t, func() error { restarts++; return nil })
	swapSessionLive(t, true)

	r := reconcileConfigTree(true)
	if r.status != recWouldFix || !strings.Contains(r.detail, "switching back") {
		t.Fatalf("check: status=%s detail=%q, want would-fix naming the switch-back case", r.status.label(), r.detail)
	}
	if r := reconcileConfigTree(false); r.status != recFixed {
		t.Fatalf("fix: status=%s detail=%q, want fixed", r.status.label(), r.detail)
	}
	if !laidDown(home, "niri/config.kdl") {
		t.Fatal("the switched-away tree should be laid down")
	}
	if restarts != 0 {
		t.Fatal("a live session with its tree in place must not be restarted")
	}
}

// A desktop whose package is not installed is not nagged about: there is no tree
// to lay and the seam knows it.
func TestReconcileConfigTreeIgnoresAnUninstalledDesktop(t *testing.T) {
	configTreeFixture(t, wm.ProviderNiri) // hyprland's variant is not installed
	t.Setenv("RYOKU_WM", wm.ProviderNiri)
	swapMaterialize(t, updater.Materialize)
	swapStartSession(t, func() error { return nil })

	r := reconcileConfigTree(false)
	if r.status != recFixed {
		t.Fatalf("status=%s detail=%q, want fixed (niri only)", r.status.label(), r.detail)
	}
	if strings.Contains(r.detail, "hyprland") {
		t.Fatalf("an uninstalled desktop must not be named, got %q", r.detail)
	}
}

// A tree the base cannot produce is a failure with the real cause, never a silent
// "fixed": the entry point is what proves the layout.
func TestReconcileConfigTreeEntryPointMissingAfterMaterializeFails(t *testing.T) {
	_, base := configTreeFixture(t, wm.ProviderNiri)
	t.Setenv("RYOKU_WM", wm.ProviderNiri)
	// the base claims the dir but ships no entry point
	if err := os.Remove(filepath.Join(base, "niri", "config.kdl")); err != nil {
		t.Fatal(err)
	}
	swapMaterialize(t, updater.Materialize)
	swapStartSession(t, func() error { t.Fatal("a session that is still bare must not be reported as started"); return nil })

	r := reconcileConfigTree(false)
	if r.status != recFailed {
		t.Fatalf("status=%s detail=%q, want failed", r.status.label(), r.detail)
	}
	if !strings.Contains(r.detail, "niri") {
		t.Fatalf("failure should name the compositor, got %q", r.detail)
	}
}
