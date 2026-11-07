package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func shellsTestIndex(contribution bool) shellIndex {
	return shellIndex{Schema: 1, Active: "base", Fallback: "base", Items: []shellItem{
		{ID: "base", Category: "subshells", Source: "shipped", Dir: "subshells/base", InUse: true, Prepared: true},
		{ID: "extra", Category: "barstyles", Source: "store", Dir: "/unpacked/extra/1", InUse: contribution, Prepared: contribution},
	}}
}

func TestShellCommitDecisionTracksActiveAndRemovedItems(t *testing.T) {
	started := shellsTestIndex(false)
	d := &daemon{settings: &settingsStore{raw: map[string]any{"barStyle": "base"}}}
	d.applyShellIndex(&started)
	d.captureShellSnapshot()
	if d.shellCommitNeeded() {
		t.Fatal("unchanged active item and in-use set requested a restart")
	}
	d.settings.raw["barStyle"] = "other"
	if !d.shellCommitNeeded() {
		t.Fatal("active item change did not request a restart")
	}
	d.settings.raw["barStyle"] = "base"
	added := shellsTestIndex(true)
	d.applyShellIndex(&added)
	if d.shellCommitNeeded() {
		t.Fatal("an item that only became in use requested a restart; it loads live")
	}
	removed := shellsTestIndex(false)
	d.applyShellIndex(&removed)
	if !d.shellCommitNeeded() {
		t.Fatal("an item the shell loaded and no longer uses did not request a restart")
	}
}

// commitTestDaemon wires a shell started with started's in-use set, and stubs
// sync (writing next as shells.json), the hooks and the restart.
func commitTestDaemon(t *testing.T, started shellIndex, next *shellIndex, events *[]string) *daemon {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	oldRunner, oldSync, oldReload := runItemScript, runRyostoreShellsSync, commitReload
	t.Cleanup(func() { runItemScript, runRyostoreShellsSync, commitReload = oldRunner, oldSync, oldReload })
	runItemScript = func(string, []string, ...string) error { return nil }
	runRyostoreShellsSync = func(prune bool) error {
		if prune {
			*events = append(*events, "prune")
		} else {
			*events = append(*events, "sync")
		}
		body, _ := json.Marshal(*next)
		path := shellsIndexPath()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, body, 0o600)
	}
	commitReload = func(*daemon) string {
		*events = append(*events, "reload")
		return "ok"
	}
	d := &daemon{settings: &settingsStore{raw: map[string]any{"barStyle": started.Active}}}
	d.applyShellIndex(&started)
	d.captureShellSnapshot()
	return d
}

func TestCommitRestartsForAContributionLoadedThenDropped(t *testing.T) {
	events := []string{}
	next := shellsTestIndex(false)
	d := commitTestDaemon(t, shellsTestIndex(false), &next, &events)
	added := shellsTestIndex(true)
	d.applyShellIndex(&added)
	removed := shellsTestIndex(false)
	d.applyShellIndex(&removed)
	if got := d.commitShells(); got != "ok" {
		t.Fatalf("commit = %q", got)
	}
	if want := []string{"sync", "reload", "prune"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("commit steps = %v, want %v", events, want)
	}
	events = events[:0]
	if got := d.commitShells(); got != "ok" {
		t.Fatalf("second commit = %q", got)
	}
	if want := []string{"sync", "prune"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("restart did not reset the seen set: steps = %v, want %v", events, want)
	}
}

func TestCommitWithoutChangesStillPrunes(t *testing.T) {
	events := []string{}
	next := shellsTestIndex(false)
	d := commitTestDaemon(t, shellsTestIndex(false), &next, &events)
	if got := d.commitShells(); got != "ok" {
		t.Fatalf("commit = %q", got)
	}
	if want := []string{"sync", "prune"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("commit steps = %v, want %v", events, want)
	}
}

func TestLifecycleRunsDeactivateThenActivateAndSkipsUnchanged(t *testing.T) {
	oldRunner := runItemScript
	t.Cleanup(func() { runItemScript = oldRunner })
	events := []string{}
	runItemScript = func(path string, env []string, args ...string) error {
		events = append(events, filepath.Base(path))
		joined := strings.Join(env, "\n")
		if !strings.Contains(joined, "RYOKU_ITEM_ID=") || !strings.Contains(joined, "RYOKU_ITEM_DIR=") {
			t.Fatal("lifecycle environment is incomplete")
		}
		return nil
	}
	previous := &shellIndex{Active: "alpha", Items: []shellItem{{
		ID: "alpha", Category: "barstyles", Dir: "/items/alpha", Prepared: true,
		Lifecycle: shellLifecycle{Deactivate: "/hooks/deactivate-alpha"},
	}}}
	next := &shellIndex{Active: "beta", Items: []shellItem{{
		ID: "beta", Category: "barstyles", Dir: "/items/beta", Prepared: true,
		Lifecycle: shellLifecycle{Activate: "/hooks/activate-beta"},
	}}}
	runLifecycleTransition(previous, next)
	if want := []string{"deactivate-alpha", "activate-beta"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("lifecycle order = %v, want %v", events, want)
	}
	events = nil
	runLifecycleTransition(next, next)
	if len(events) != 0 {
		t.Fatalf("unchanged item ran lifecycle hooks: %v", events)
	}
}

func TestActiveItemUpdateReactivatesRestartsThenPrunes(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	oldRunner, oldSync, oldReload := runItemScript, runRyostoreShellsSync, commitReload
	t.Cleanup(func() { runItemScript, runRyostoreShellsSync, commitReload = oldRunner, oldSync, oldReload })
	record := func(dir string) shellItem {
		return shellItem{ID: "alpha", Category: "barstyles", Source: "store", Dir: dir, InUse: true, Prepared: true,
			Lifecycle: shellLifecycle{Activate: dir + "/bin/activate", Deactivate: dir + "/bin/deactivate"}}
	}
	oldIndex := &shellIndex{Schema: 1, Active: "alpha", Fallback: "beta", Items: []shellItem{record("/unpacked/alpha/old")}}
	newIndex := shellIndex{Schema: 1, Active: "alpha", Fallback: "beta", Items: []shellItem{record("/unpacked/alpha/new")}}

	events := []string{}
	runItemScript = func(path string, env []string, args ...string) error {
		dir := ""
		for _, value := range env {
			if strings.HasPrefix(value, "RYOKU_ITEM_DIR=") {
				dir = strings.TrimPrefix(value, "RYOKU_ITEM_DIR=")
			}
		}
		events = append(events, filepath.Base(path)+" "+dir)
		return nil
	}
	runRyostoreShellsSync = func(prune bool) error {
		if prune {
			events = append(events, "prune")
		} else {
			events = append(events, "sync")
		}
		body, _ := json.Marshal(newIndex)
		path := shellsIndexPath()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, body, 0o600)
	}
	commitReload = func(*daemon) string {
		events = append(events, "reload")
		return "ok"
	}

	d := &daemon{
		settings:         &settingsStore{raw: map[string]any{"barStyle": "alpha"}},
		shellIndex:       oldIndex,
		shellSnapshot:    snapshotFor("alpha", oldIndex),
		shellSnapshotSet: true,
	}
	if got := d.commitShells(); got != "ok" {
		t.Fatalf("commit = %q", got)
	}
	want := []string{
		"sync",
		"deactivate /unpacked/alpha/old",
		"activate /unpacked/alpha/new",
		"reload",
		"prune",
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("update order = %v, want %v", events, want)
	}
}

func TestPackedActiveItemRunsNoHooks(t *testing.T) {
	oldRunner := runItemScript
	t.Cleanup(func() { runItemScript = oldRunner })
	ran := []string{}
	runItemScript = func(path string, env []string, args ...string) error {
		ran = append(ran, path)
		return nil
	}
	packed := &shellIndex{Active: "alpha", Items: []shellItem{{ID: "alpha", Category: "barstyles",
		Lifecycle: shellLifecycle{Activate: "/hooks/activate"}}}}
	runLifecycleTransition(nil, packed)
	if len(ran) != 0 {
		t.Fatalf("packed item ran hooks: %v", ran)
	}
}

func TestPaletteHookUsesActiveIndexItem(t *testing.T) {
	oldRunner := runItemScript
	t.Cleanup(func() { runItemScript = oldRunner })
	called := false
	runItemScript = func(path string, env []string, args ...string) error {
		called = true
		if path != "/hooks/palette" || len(args) != 1 || args[0] != matugenColorsPath() {
			t.Fatalf("palette hook = %q %v", path, args)
		}
		return nil
	}
	d := &daemon{shellIndex: &shellIndex{Active: "alpha", Items: []shellItem{{
		ID: "alpha", Category: "barstyles", Dir: "/items/alpha",
		Lifecycle: shellLifecycle{Palette: "/hooks/palette"},
	}}}}
	d.runActivePaletteHook()
	if !called {
		t.Fatal("active palette hook did not run")
	}
}

func TestRuntimeInstallRunsOnceAndCommandCarriesManagedEnvironment(t *testing.T) {
	oldRunner := runItemScript
	t.Cleanup(func() { runItemScript = oldRunner })
	calls := map[string]int{}
	runItemScript = func(path string, env []string, args ...string) error {
		calls[path]++
		return nil
	}
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	item := shellItem{
		ID: "alpha", Category: "barstyles", Dir: "/items/alpha",
		Runtime: shellRuntime{Check: "/hooks/check", Install: "/hooks/install", Start: "/hooks/start"},
	}
	d := &daemon{shellIndex: &shellIndex{Active: "alpha", Items: []shellItem{item}}}
	for range 2 {
		cmd, run, err := d.desktopCommand()
		if err != nil {
			t.Fatal(err)
		}
		if !run.runtime || cmd.Path != "/hooks/start" {
			t.Fatalf("runtime command = %q, managed = %t", cmd.Path, run.runtime)
		}
		env := strings.Join(cmd.Env, "\n")
		if !strings.Contains(env, "RYOKU_RUNTIME_MANAGED=1") || !strings.Contains(env, "RYOKU_RUNTIME_READY_FILE=") {
			t.Fatal("runtime command is missing managed environment")
		}
		os.Remove(run.ready)
	}
	if calls["/hooks/check"] != 2 || calls["/hooks/install"] != 1 {
		t.Fatalf("runtime calls = %#v", calls)
	}
}

func TestRuntimeCheckCanKeepResidentShell(t *testing.T) {
	oldRunner := runItemScript
	t.Cleanup(func() { runItemScript = oldRunner })
	runItemScript = func(path string, env []string, args ...string) error {
		if path == "/hooks/check" {
			return errors.New("not wanted")
		}
		return nil
	}
	item := shellItem{ID: "alpha", Category: "barstyles", Runtime: shellRuntime{Check: "/hooks/check", Start: "/hooks/start"}}
	d := &daemon{shellIndex: &shellIndex{Active: "alpha", Items: []shellItem{item}}}
	cmd, run, err := d.desktopCommand()
	if err != nil {
		t.Fatal(err)
	}
	if run.runtime || filepath.Base(cmd.Path) != "qs" {
		t.Fatalf("failed runtime check selected %q, managed = %t", cmd.Path, run.runtime)
	}
}

func TestRuntimeBackoffCaps(t *testing.T) {
	if got := nextRuntimeBackoff(time.Second); got != 2*time.Second {
		t.Fatalf("first backoff = %s", got)
	}
	if got := nextRuntimeBackoff(20 * time.Second); got != 30*time.Second {
		t.Fatalf("capped backoff = %s", got)
	}
}

func TestRuntimeFailureFallsBackThroughIndex(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	path := filepath.Join(t.TempDir(), "shell.json")
	if err := os.WriteFile(path, []byte(`{"barStyle":"alpha","keep":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	failed := shellItem{ID: "alpha", Category: "barstyles"}
	fallback := shellItem{ID: "beta", Category: "subshells"}
	d := &daemon{
		settings:   newSettingsStore(path),
		shellIndex: &shellIndex{Schema: 1, Active: "alpha", Fallback: "beta", Items: []shellItem{failed, fallback}},
	}
	oldSync := runRyostoreShellsSync
	t.Cleanup(func() { runRyostoreShellsSync = oldSync })
	runRyostoreShellsSync = func(bool) error {
		index := shellIndex{Schema: 1, Active: "beta", Fallback: "beta", Items: []shellItem{failed, fallback}}
		body, _ := json.Marshal(index)
		file := shellsIndexPath()
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return err
		}
		return os.WriteFile(file, body, 0o600)
	}
	d.fallbackRuntime(&failed, "failed")
	if got := d.activeBarStyle(); got != "beta" {
		t.Fatalf("fallback active item = %q", got)
	}
}
