package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const starshipTestLayout = `add_newline = false
palette = "mythos"
format = "$directory$character"

[palettes.mythos]
dir_bg = "#111111"
dir_fg = "#eeeeee"
crimson = "#ff0000"

[directory]
style = "fg:dir_fg bg:dir_bg"

[character]
success_symbol = "[λ](fg:dir_fg)"
error_symbol = "[λ](fg:crimson)"
`

func setupStarshipTest(t *testing.T) string {
	t.Helper()
	config := t.TempDir()
	layouts := filepath.Join(config, "layouts")
	if err := os.MkdirAll(layouts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(layouts, "pill.toml"), []byte(starshipTestLayout), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("RYOKU_STARSHIP_LAYOUTS", layouts)
	return config
}

func TestStarshipFixedLayoutIsCopiedExactly(t *testing.T) {
	config := setupStarshipTest(t)
	state := starshipState{Layout: "pill", Palette: "fixed"}
	if err := applyStarship(state, true); err != nil {
		t.Fatalf("applyStarship: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(config, "starship.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != starshipTestLayout {
		t.Fatalf("fixed layout changed while copying:\n%s", got)
	}
	loaded := readStarshipState()
	if loaded != state {
		t.Fatalf("state = %+v, want %+v", loaded, state)
	}
}

func TestStarshipWallpaperPaletteComposesWithoutChangingLayout(t *testing.T) {
	config := setupStarshipTest(t)
	palettePath := filepath.Join(config, "starship", "ryoku-colors.toml")
	if err := os.MkdirAll(filepath.Dir(palettePath), 0o755); err != nil {
		t.Fatal(err)
	}
	palette := `[palettes.wallpaper]
dir_bg = "#222222"
dir_fg = "#abcdef"
crimson = "#fedcba"
`
	if err := os.WriteFile(palettePath, []byte(palette), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyStarship(starshipState{Layout: "pill", Palette: "wallpaper"}, true); err != nil {
		t.Fatalf("applyStarship: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(config, "starship.toml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, want := range []string{`palette = "wallpaper"`, `[palettes.wallpaper]`, `dir_fg = "#abcdef"`, `[directory]`, `success_symbol = "[λ](fg:dir_fg)"`} {
		if !strings.Contains(text, want) {
			t.Errorf("composed config missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, `[palettes.mythos]`) || strings.Contains(text, `#111111`) {
		t.Errorf("fixed palette leaked into wallpaper config:\n%s", text)
	}
	original, err := os.ReadFile(filepath.Join(os.Getenv("RYOKU_STARSHIP_LAYOUTS"), "pill.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(original) != starshipTestLayout {
		t.Fatal("source layout was modified")
	}
}

func TestStarshipRejectsUnknownAndTraversalLayouts(t *testing.T) {
	setupStarshipTest(t)
	for _, id := range []string{"missing", "../pill", "Pill"} {
		if _, err := readStarshipLayout(id); err == nil {
			t.Errorf("readStarshipLayout(%q) succeeded", id)
		}
	}
}

func TestListStarshipLayoutsIsSortedAndFiltersOtherFiles(t *testing.T) {
	setupStarshipTest(t)
	dir := os.Getenv("RYOKU_STARSHIP_LAYOUTS")
	if err := os.WriteFile(filepath.Join(dir, "lean.toml"), []byte(starshipTestLayout), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore"), 0o644); err != nil {
		t.Fatal(err)
	}
	layouts, err := listStarshipLayouts()
	if err != nil {
		t.Fatal(err)
	}
	if len(layouts) != 2 || layouts[0].ID != "lean" || layouts[1].ID != "pill" {
		t.Fatalf("layouts = %+v", layouts)
	}
}

func TestStarshipRefreshTracksWallpaperOnlyInWallpaperMode(t *testing.T) {
	config := setupStarshipTest(t)
	palettePath := filepath.Join(config, "starship", "ryoku-colors.toml")
	if err := os.MkdirAll(filepath.Dir(palettePath), 0o755); err != nil {
		t.Fatal(err)
	}
	writePalette := func(color string) {
		t.Helper()
		body := "[palettes.wallpaper]\ndir_bg = \"#222222\"\ndir_fg = \"" + color + "\"\ncrimson = \"#ff0000\"\n"
		if err := os.WriteFile(palettePath, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writePalette("#aaaaaa")
	if err := applyStarship(starshipState{Layout: "pill", Palette: "wallpaper"}, true); err != nil {
		t.Fatal(err)
	}
	writePalette("#bbbbbb")
	if err := runStarship([]string{"refresh"}); err != nil {
		t.Fatal(err)
	}
	active, err := os.ReadFile(filepath.Join(config, "starship.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(active), "#bbbbbb") {
		t.Fatalf("wallpaper refresh did not update the active palette:\n%s", active)
	}

	if err := applyStarship(starshipState{Layout: "pill", Palette: "fixed"}, true); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config, "starship.toml"), []byte("user edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writePalette("#cccccc")
	if err := runStarship([]string{"refresh"}); err != nil {
		t.Fatal(err)
	}
	active, err = os.ReadFile(filepath.Join(config, "starship.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(active) != "user edit\n" {
		t.Fatalf("fixed-mode refresh rewrote the active config: %q", active)
	}
}
