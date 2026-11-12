package state

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestAppPreferencesPersistTheFrame(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	want := AppPreferences{
		Width: 1612, Height: 1043, ZoomPercent: 110, SidebarFolded: true, SidePanelOpen: true, ReduceMotion: true,
		Colours: ColoursWallpaper, Contrast: ContrastHigh, Tint: TintMedium,
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

func TestAppPreferencesNormalizeLook(t *testing.T) {
	if got := (AppPreferences{}).Normalize(); got.Contrast != ContrastStandard || got.Tint != TintLow {
		t.Fatalf("default look = %q, %q, want standard and low", got.Contrast, got.Tint)
	}
	if got := (AppPreferences{Contrast: "loud", Tint: "heavy"}).Normalize(); got.Contrast != ContrastStandard || got.Tint != TintLow {
		t.Fatalf("unknown words gave %q, %q, want standard and low", got.Contrast, got.Tint)
	}
	if got := (AppPreferences{Contrast: ContrastHigh, Tint: TintOff}).Normalize(); got.Contrast != ContrastHigh || got.Tint != TintOff {
		t.Fatalf("known words became %q, %q", got.Contrast, got.Tint)
	}
}

func TestAppPreferencesOpenInTheRyokuColours(t *testing.T) {
	if got := (AppPreferences{}).Normalize().Colours; got != ColoursRyoku {
		t.Fatalf("default colours = %q, want the ryoku.dev stock", got)
	}
	if got := (AppPreferences{Colours: "sepia"}).Normalize().Colours; got != ColoursRyoku {
		t.Fatalf("an unknown word gave %q, want the ryoku.dev stock", got)
	}
	for _, word := range []string{ColoursRyokuLight, ColoursWallpaper} {
		if got := (AppPreferences{Colours: word}).Normalize().Colours; got != word {
			t.Fatalf("%q became %q", word, got)
		}
	}
}
