package state

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"ryoku-rashin/internal/paths"
)

func TestAppPreferencesPersistTheFrame(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	want := AppPreferences{
		Width: 1612, Height: 1043, ZoomPercent: 110, SidebarFolded: true, SidePanelOpen: true,
		HiddenSteps: map[string][]string{"team-a": {"mail", "routine:r1"}},
	}
	if err := SaveAppPreferences(want); err != nil {
		t.Fatal(err)
	}
	got := LoadAppPreferences()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("preferences = %#v, want %#v", got, want)
	}
}

// A file written while Hyakki kept its own look still opens the window where
// it was; the old look and harness keys are dropped by the next save.
func TestAppPreferencesFromBeforeTheDesktopLookStillOpen(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	path := paths.AppState()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	old := `{"width":1400,"height":900,"zoomPercent":120,"reduceMotion":true,"colours":"wallpaper","contrast":"high","tint":"medium","agentModels":{"codex":"gpt-5.6"}}`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	got := LoadAppPreferences()
	if got.Width != 1400 || got.Height != 900 || got.ZoomPercent != 120 {
		t.Fatalf("an old file opened at %dx%d zoom %d, want 1400x900 zoom 120", got.Width, got.Height, got.ZoomPercent)
	}
	if err := SaveAppPreferences(got); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"width":1400,"height":900,"zoomPercent":120}`; string(saved) != want {
		t.Fatalf("saved %s, want %s", saved, want)
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
