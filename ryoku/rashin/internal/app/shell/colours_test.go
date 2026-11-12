package shell

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"

	"ryoku-rashin/internal/app/state"
)

// Rashin opens in the ryoku.dev stock even with a wallpaper palette on the
// desktop, follows the wallpaper only once the reader picks it, and goes back
// to the very same stock. The stock used to take its accents from the
// wallpaper, and the palette was watched whatever the colours were.
func TestColoursOpenInTheStockAndFollowTheWallpaperOnlyWhenPicked(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stock := stage.runtime.Theme().R
	cache := filepath.Join(os.Getenv("XDG_CACHE_HOME"), "ryoku")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	wallpaper := `{"surface":"#0f2418","onSurface":"#d6f2e0","primary":"#3cff8a","primaryContainer":"#1d5c3a","onPrimaryContainer":"#c4ffd8","error":"#ff5a4f"}`
	if err := os.WriteFile(filepath.Join(cache, "colors.json"), []byte(wallpaper), 0o644); err != nil {
		t.Fatal(err)
	}

	stage.open("settings/appearance")
	if stage.runtime.colours() != state.ColoursRyoku || stage.runtime.unwatch != nil {
		t.Fatalf("Rashin opened in %q, watching the palette: %v", stage.runtime.colours(), stage.runtime.unwatch != nil)
	}
	if _, shown := stage.named("Medium"); shown {
		t.Fatal("Tint is offered on the stock, where it does nothing")
	}

	stage.click("Wallpaper")
	if stage.shell.preferences.Colours != state.ColoursWallpaper || stage.runtime.colours() != state.ColoursWallpaper {
		t.Fatalf("the pick stored %q and paints %q", stage.shell.preferences.Colours, stage.runtime.colours())
	}
	if stage.runtime.unwatch == nil {
		t.Fatal("following the wallpaper does not watch the palette")
	}
	if stage.runtime.Theme().R.Gold == stock.Gold {
		t.Fatal("the wallpaper's colours did not reach the window")
	}
	if _, shown := stage.named("Medium"); !shown {
		t.Fatalf("Tint is not offered with the wallpaper's colours; on screen: %v", stage.names())
	}

	stage.click("Ryoku")
	if stage.runtime.unwatch != nil {
		t.Fatal("the palette is still watched on the stock")
	}
	if got := stage.runtime.Theme().R; got != stock {
		t.Fatalf("back on Ryoku the window paints gold %v, want the stock's %v", got.Gold, stock.Gold)
	}
}

func TestColoursWordsPaintTheirModes(t *testing.T) {
	for word, mode := range map[string]palette.Mode{
		state.ColoursRyoku: palette.Dark, state.ColoursRyokuLight: palette.Light, state.ColoursWallpaper: palette.Follow, "sepia": palette.Dark,
	} {
		if got := coloursMode(word); got != mode {
			t.Fatalf("%q paints mode %v, want %v", word, got, mode)
		}
	}
	for _, word := range []string{state.ColoursRyoku, state.ColoursRyokuLight, state.ColoursWallpaper} {
		if got := modeColours(coloursMode(word)); got != word {
			t.Fatalf("%q reads back as %q", word, got)
		}
	}
}
