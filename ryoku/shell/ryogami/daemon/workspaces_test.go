package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	wm "ryoku-wm"
)

func workspaceTestManager(t *testing.T) (*daemon, *workspaceManager, string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	cache := filepath.Join(root, "cache")
	walls := filepath.Join(root, "walls")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(walls, 0o755); err != nil {
		t.Fatal(err)
	}
	d := &daemon{surface: newWallSurface(), video: newVideoPlayer(), store: openStore(cache), events: newEventHub()}
	d.cfg.Paths.Cache = cache
	d.cfg.Paths.Wallpaper = walls
	m := newWorkspaceManager(d)
	d.workspaces = m
	return d, m, root
}

func TestWorkspaceWallpaperPrecedence(t *testing.T) {
	d, manager, root := workspaceTestManager(t)
	broadcast := filepath.Join(root, "broadcast.png")
	output := filepath.Join(root, "output.png")
	workspace := filepath.Join(root, "workspace.png")
	fallback := filepath.Join(d.cfg.Paths.Wallpaper, "fallback.png")
	for _, path := range []string{broadcast, output, workspace, fallback} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	saveJSON(filepath.Join(d.cfg.Paths.Cache, "outputs.json"), map[string]map[string]interface{}{
		"*":      {"type": "static", "path": broadcast},
		"HDMI-1": {"type": "static", "path": output},
	})
	target := workspaceTarget{Provider: "test", ID: "2", Name: "Writing", Output: "HDMI-1"}
	manager.state.Assignments[workspaceKey(target.Provider, target.ID)] = workspaceWall{
		Provider: target.Provider, ID: target.ID, Name: target.Name, Output: target.Output,
		Type: "static", Path: workspace,
	}

	if got, assigned := manager.resolve(target, true); !assigned || got.Path != workspace {
		t.Fatalf("workspace precedence = (%q, %v), want %q", got.Path, assigned, workspace)
	}
	manager.unassign(target)
	if got, assigned := manager.resolve(target, true); assigned || got.Path != output {
		t.Fatalf("output precedence = (%q, %v), want %q", got.Path, assigned, output)
	}
	state := map[string]map[string]interface{}{"*": {"type": "static", "path": broadcast}}
	saveJSON(filepath.Join(d.cfg.Paths.Cache, "outputs.json"), state)
	if got, assigned := manager.resolve(target, true); assigned || got.Path != broadcast {
		t.Fatalf("broadcast precedence = (%q, %v), want %q", got.Path, assigned, broadcast)
	}
	saveJSON(filepath.Join(d.cfg.Paths.Cache, "outputs.json"), map[string]map[string]interface{}{})
	if got, assigned := manager.resolve(target, true); assigned || got.Path != fallback {
		t.Fatalf("default precedence = (%q, %v), want %q", got.Path, assigned, fallback)
	}
}

func TestWorkspaceKeyUsesIDThenMigratesByName(t *testing.T) {
	_, manager, root := workspaceTestManager(t)
	oldPath := filepath.Join(root, "old.png")
	exactPath := filepath.Join(root, "exact.png")
	old := workspaceWall{Provider: "test", ID: "17", Name: "Mail", Output: "eDP-1", Type: "static", Path: oldPath}
	exact := workspaceWall{Provider: "test", ID: "18", Name: "Mail", Output: "DP-1", Type: "static", Path: exactPath}
	manager.state.Assignments[workspaceKey("test", "17")] = old
	manager.state.Assignments[workspaceKey("test", "18")] = exact

	target := workspaceTarget{Provider: "test", ID: "18", Name: "Renamed", Output: "eDP-1"}
	if got, ok := manager.resolve(target, true); !ok || got.Path != exactPath {
		t.Fatalf("stable id did not win over name fallback: %+v, %v", got, ok)
	}

	delete(manager.state.Assignments, workspaceKey("test", "18"))
	target = workspaceTarget{Provider: "test", ID: "41", Name: "Mail", Output: "eDP-1"}
	if got, ok := manager.resolve(target, true); !ok || got.Path != oldPath {
		t.Fatalf("name fallback did not recover recreated workspace: %+v, %v", got, ok)
	}
	if _, oldStillPresent := manager.state.Assignments[workspaceKey("test", "17")]; oldStillPresent {
		t.Fatal("name fallback left the stale workspace id behind")
	}
	if migrated := manager.state.Assignments[workspaceKey("test", "41")]; migrated.ID != "41" || migrated.Output != "eDP-1" {
		t.Fatalf("migrated assignment = %+v", migrated)
	}
}

func TestWorkspaceSwitchDebouncesToOneReveal(t *testing.T) {
	calls := make(chan string, 4)
	manager := &workspaceManager{
		active: map[string]workspaceTarget{}, pending: map[string]*pendingWorkspaceSwitch{}, debounce: 15 * time.Millisecond,
	}
	manager.apply = func(target workspaceTarget, mode string) {
		calls <- target.ID + ":" + mode
	}
	frame := func(id string) []wm.Workspace {
		return []wm.Workspace{{ID: id, Name: id, Output: "eDP-1", Active: true}}
	}
	manager.updateActiveForProvider("test", frame("1"))
	if got := <-calls; got != "1:init" {
		t.Fatalf("initial call = %q, want 1:init", got)
	}
	manager.updateActiveForProvider("test", frame("2"))
	manager.updateActiveForProvider("test", frame("3"))
	for _, want := range []string{"2:init", "3:set"} {
		select {
		case got := <-calls:
			if got != want {
				t.Fatalf("debounced call = %q, want %q", got, want)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("timed out waiting for %q", want)
		}
	}
	select {
	case got := <-calls:
		t.Fatalf("unexpected extra call %q", got)
	case <-time.After(30 * time.Millisecond):
	}
}

func TestWorkspaceStateRestoresWithoutAnimation(t *testing.T) {
	d, manager, root := workspaceTestManager(t)
	path := filepath.Join(root, "restored.png")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := workspaceTarget{Provider: "test", ID: "7", Name: "Seven", Output: "eDP-1"}
	if _, err := manager.assign(target, workspaceWall{Type: "static", Path: path, Volume: 75}); err != nil {
		t.Fatal(err)
	}
	reloaded := newWorkspaceManager(d)
	if reloaded.state.Version != workspaceStateVersion {
		t.Fatalf("state version = %d, want %d", reloaded.state.Version, workspaceStateVersion)
	}
	wall, ok := reloaded.resolve(target, true)
	if !ok || wall.Path != path || wall.Volume != 75 {
		t.Fatalf("restored assignment = %+v, %v", wall, ok)
	}
	called := make(chan string, 1)
	reloaded.apply = func(_ workspaceTarget, mode string) { called <- mode }
	reloaded.updateActiveForProvider("test", []wm.Workspace{{ID: "7", Name: "Seven", Output: "eDP-1", Active: true}})
	select {
	case mode := <-called:
		if mode != "init" {
			t.Fatalf("initial restore mode = %q, want init", mode)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("initial workspace frame did not restore its wallpaper")
	}
}

func TestHotplugRestoreTouchesOnlyAddedOutput(t *testing.T) {
	d, _, root := workspaceTestManager(t)
	shared := filepath.Join(root, "shared.png")
	settled := filepath.Join(root, "settled.png")
	for _, path := range []string{shared, settled} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	saveJSON(filepath.Join(d.cfg.Paths.Cache, "outputs.json"), map[string]map[string]interface{}{
		"*": {"type": "static", "path": shared},
	})
	d.surface.showOutput("DP-2", settled, "cover", nil, false, false, videoClip{})

	if want, applied := d.restoreOutputTargets([]string{"DP-1"}); want != 1 || applied != 1 {
		t.Fatalf("targeted restore = %d/%d, want 1/1", want, applied)
	}
	frame := d.surface.snapshot()
	if got := frame.Outputs["DP-1"].Path; got != shared {
		t.Fatalf("added output path = %q, want %q", got, shared)
	}
	if got := frame.Outputs["DP-2"].Path; got != settled {
		t.Fatalf("settled output changed to %q, want %q", got, settled)
	}
}

// Switching between workspaces that resolve to the wallpaper already on the
// output must leave the frame alone; only a different wallpaper repaints.
func TestWorkspaceSwitchToSameWallpaperDoesNotRepaint(t *testing.T) {
	d, manager, root := workspaceTestManager(t)
	shared := filepath.Join(root, "shared.png")
	own := filepath.Join(root, "own.png")
	for _, path := range []string{shared, own} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	saveJSON(filepath.Join(d.cfg.Paths.Cache, "outputs.json"), map[string]map[string]interface{}{
		"*": {"type": "static", "path": shared},
	})
	one := workspaceTarget{Provider: "test", ID: "1", Name: "1", Output: "eDP-1"}
	two := workspaceTarget{Provider: "test", ID: "2", Name: "2", Output: "eDP-1"}
	three := workspaceTarget{Provider: "test", ID: "3", Name: "3", Output: "eDP-1"}
	manager.state.Assignments[workspaceKey("test", "3")] = workspaceWall{
		Provider: "test", ID: "3", Name: "3", Output: "eDP-1", Type: "static", Path: own,
	}
	revision := func() int64 { return d.surface.snapshot().Outputs["eDP-1"].Revision }

	if err := d.applyWorkspaceTarget(one, "set"); err != nil {
		t.Fatal(err)
	}
	painted := revision()
	if painted == 0 {
		t.Fatal("first workspace did not paint its wallpaper")
	}
	if err := d.applyWorkspaceTarget(two, "set"); err != nil {
		t.Fatal(err)
	}
	if got := revision(); got != painted {
		t.Fatalf("same wallpaper repainted on switch: revision %d -> %d", painted, got)
	}
	if err := d.applyWorkspaceTarget(three, "set"); err != nil {
		t.Fatal(err)
	}
	if frame := d.surface.snapshot().Outputs["eDP-1"]; frame.Path != own || frame.Revision == painted {
		t.Fatalf("assigned workspace did not repaint: %+v", frame)
	}
	if err := d.applyWorkspaceTarget(one, "set"); err != nil {
		t.Fatal(err)
	}
	if got := d.surface.snapshot().Outputs["eDP-1"].Path; got != shared {
		t.Fatalf("returning to an unassigned workspace kept %q, want %q", got, shared)
	}
}
