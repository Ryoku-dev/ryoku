package state

import (
	"path/filepath"
	"testing"
)

func TestAppPreferencesPersistTheFrame(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	want := AppPreferences{
		Width: 1612, Height: 1043, ZoomPercent: 110, SidebarFolded: true, SidePanelOpen: true, ReduceMotion: true,
	}
	if err := SaveAppPreferences(want); err != nil {
		t.Fatal(err)
	}
	got := LoadAppPreferences()
	if got != want {
		t.Fatalf("preferences = %#v, want %#v", got, want)
	}
}

func TestAppPreferencesNormalizeZoom(t *testing.T) {
	if got := (AppPreferences{}).Normalize().ZoomPercent; got != DefaultZoom {
		t.Fatalf("default zoom = %d", got)
	}
	if got := (AppPreferences{ZoomPercent: 20}).Normalize().ZoomPercent; got != MinZoom {
		t.Fatalf("minimum zoom = %d", got)
	}
	if got := (AppPreferences{ZoomPercent: 900}).Normalize().ZoomPercent; got != MaxZoom {
		t.Fatalf("maximum zoom = %d", got)
	}
}
