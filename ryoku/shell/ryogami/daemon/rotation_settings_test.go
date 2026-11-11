package main

import (
	"path/filepath"
	"testing"
)

func rotationRunning(d *daemon) bool {
	running, _ := d.random.status()["running"].(bool)
	return running
}

// "Favourites only" is on by default, and a library with no favourites used to
// leave rotation reporting active while it never changed the wallpaper (#738).
func TestRandomPickWithoutFavouritesUsesTheWholePool(t *testing.T) {
	var cfg config
	cfg.Paths.Wallpaper = t.TempDir()
	cfg.Paths.Cache = t.TempDir()
	d := &daemon{cfg: cfg, store: openStore(cfg.Paths.Cache)}
	d.store.entries["a.png"] = Entry{Key: "a.png", Name: "a.png", Type: "static"}
	d.store.entries["b.png"] = Entry{Key: "b.png", Name: "b.png", Type: "static"}

	if got := d.pickRandom([]string{"static"}, true); got == "" {
		t.Fatal("favourites only with no favourites must still pick from the library")
	}

	d.store.entries["b.png"] = Entry{Key: "b.png", Name: "b.png", Type: "static", Favourite: 1}
	for range 20 {
		if got := filepath.Base(d.pickRandom([]string{"static"}, true)); got != "b.png" {
			t.Fatalf("with a favourite present the pool stays favourites only, picked %s", got)
		}
	}
}

// The picker's auto-rotate button used to start an in-memory loop that a daemon
// restart dropped. It now flips the saved switch the start hook reads.
func TestAutoRotateToggleResumesAfterRestart(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	start := func() *daemon {
		d := &daemon{events: newEventHub(), random: newRandomRotation()}
		d.playlists = newPlaylistManager(filepath.Join(root, "cache"), d)
		d.syncRandomRotation()
		return d
	}

	d := start()
	if rotationRunning(d) {
		t.Fatal("rotation must start off")
	}
	if _, err := d.runAction("randomRotate", nil); err != nil {
		t.Fatal(err)
	}
	if !rotationRunning(d) {
		t.Fatal("the toggle must start rotation now")
	}
	d.flushSettingsNow()
	d.random.stop()

	d = start()
	if !rotationRunning(d) {
		t.Fatal("rotation turned on from the picker must resume after a restart")
	}
	if _, err := d.runAction("randomRotate", nil); err != nil {
		t.Fatal(err)
	}
	if rotationRunning(d) || d.rotationActive() {
		t.Fatal("the toggle must stop rotation now")
	}
	d.flushSettingsNow()

	if d = start(); rotationRunning(d) {
		t.Fatal("rotation turned off from the picker must stay off after a restart")
	}
}
