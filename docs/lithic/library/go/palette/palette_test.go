package palette

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

func TestDarkMapping(t *testing.T) {
	p := syntheticPalette(true)
	r := FromPalette(Palette{Roles: p, Dark: true, Source: "wallpaper"}, Look{})
	assertColor(t, "ground", r.Ground, p[tokens.PaletteDark["ground"]])
	assertColor(t, "stone2", r.Stone2, p[tokens.PaletteDark["stone2"]])
	assertColor(t, "slab", r.Slab, p[tokens.PaletteDark["slab"]])
	assertColor(t, "field", r.Field, p[tokens.PaletteDark["field"]])
	assertColor(t, "vein", r.Vein, p[tokens.PaletteDark["vein"]])
	assertColor(t, "fg", r.Fg, p[tokens.PaletteDark["fg"]])
	assertColor(t, "edge", r.Edge, Mix(r.Vein, r.Fg3, tokens.AlphaEdge))
	assertColor(t, "selection", r.Sel, p[tokens.PaletteAccents["sel"]])
	if r.Scrim.A != alphaByte(tokens.AlphaScrimDark) {
		t.Fatalf("dark scrim alpha = %d", r.Scrim.A)
	}
}

func TestLightMapping(t *testing.T) {
	p := syntheticPalette(false)
	r := FromPalette(Palette{Roles: p, Dark: false, Source: "wallpaper"}, Look{})
	assertColor(t, "ground", r.Ground, p[tokens.PaletteLight["ground"]])
	assertColor(t, "stone1", r.Stone1, p[tokens.PaletteLight["stone1"]])
	assertColor(t, "stone4", r.Stone4, p[tokens.PaletteLight["stone4"]])
	assertColor(t, "slab", r.Slab, p[tokens.PaletteLight["slab"]])
	assertColor(t, "field", r.Field, p[tokens.PaletteLight["field"]])
	assertColor(t, "vein", r.Vein, p[tokens.PaletteLight["vein"]])
	if r.Scrim.A != alphaByte(tokens.AlphaScrimLight) {
		t.Fatalf("light scrim alpha = %d", r.Scrim.A)
	}
}

func TestSelectionTakesTheWallpaperColour(t *testing.T) {
	p := syntheticPalette(true)
	toward := color.NRGBA{R: 40, G: 90, B: 150, A: 255}
	p[tokens.PaletteAccents["selMix"]] = toward
	r := FromPalette(Palette{Roles: p, Dark: true, Source: "wallpaper"}, Look{})
	assertColor(t, "selection", r.Sel, Mix(p[tokens.PaletteAccents["sel"]], toward, tokens.AlphaSelMix))
	assertColor(t, "onSel", r.OnSel, p[tokens.PaletteAccents["onSel"]])
	assertColor(t, "onSel2", r.OnSel2, Mix(r.OnSel, r.Sel, tokens.AlphaOnSel2))
}

func TestForcedModesKeepSignatureNeutrals(t *testing.T) {
	p := syntheticPalette(true)
	toward := color.NRGBA{R: 40, G: 90, B: 150, A: 255}
	p[tokens.PaletteAccents["selMix"]] = toward
	for _, test := range []struct {
		name string
		dark bool
	}{
		{name: "dark", dark: true},
		{name: "light", dark: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := FromPalette(Palette{Roles: p, Dark: test.dark, Forced: true, Source: "wallpaper"}, Look{})
			signature := Signature(test.dark)
			assertColor(t, "ground", r.Ground, signature.Ground)
			assertColor(t, "stone2", r.Stone2, signature.Stone2)
			assertColor(t, "slab", r.Slab, signature.Slab)
			assertColor(t, "field", r.Field, signature.Field)
			assertColor(t, "fg", r.Fg, signature.Fg)
			assertColor(t, "edge", r.Edge, signature.Edge)
			assertColor(t, "gold", r.Gold, p[tokens.PaletteAccents["gold"]])
			assertColor(t, "selection", r.Sel, Mix(p[tokens.PaletteAccents["sel"]], toward, tokens.AlphaSelMix))
		})
	}
}

func TestSignatureStandardIsTheTable(t *testing.T) {
	for _, dark := range []bool{true, false} {
		for _, tint := range []Tint{TintOff, TintLow, TintMedium} {
			r := FromPalette(Palette{Dark: dark, Source: "signature"}, Look{Tint: tint})
			if r != Signature(dark) {
				t.Fatalf("dark %v tint %d = %#v, want the signature", dark, tint, r)
			}
		}
	}
}

func TestHighContrastSpreadsStonesFromGround(t *testing.T) {
	p := syntheticPalette(true)
	standard := FromPalette(Palette{Roles: p, Dark: true, Source: "wallpaper"}, Look{})
	high := FromPalette(Palette{Roles: p, Dark: true, Source: "wallpaper"}, Look{Contrast: High})
	factor := 1 + float64(tokens.AlphaContrastHigh)
	assertColor(t, "ground", high.Ground, standard.Ground)
	assertColor(t, "fg", high.Fg, standard.Fg)
	assertColor(t, "fg2", high.Fg2, standard.Fg2)
	for _, stone := range []struct {
		name      string
		got, base color.NRGBA
	}{
		{"stone1", high.Stone1, standard.Stone1},
		{"stone2", high.Stone2, standard.Stone2},
		{"stone3", high.Stone3, standard.Stone3},
		{"stone4", high.Stone4, standard.Stone4},
		{"slab", high.Slab, standard.Slab},
		{"field", high.Field, standard.Field},
		{"vein", high.Vein, standard.Vein},
	} {
		assertColor(t, stone.name, stone.got, away(stone.base, standard.Ground, factor))
	}
	assertColor(t, "fg3", high.Fg3, Mix(standard.Fg3, standard.Fg, tokens.AlphaContrastHighFg3))
	assertColor(t, "edge", high.Edge, Mix(high.Vein, high.Fg3, tokens.AlphaEdge))
}

func TestHighContrastKeepsInkReadableOnTheSignature(t *testing.T) {
	for _, dark := range []bool{true, false} {
		r := FromPalette(Palette{Dark: dark, Source: "signature"}, Look{Contrast: High})
		if ratio := ContrastRatio(r.GoldInk, r.Stone2); ratio < float64(tokens.ContrastMinimum) {
			t.Fatalf("dark %v goldInk contrast = %.2f", dark, ratio)
		}
		if ratio := ContrastRatio(r.Jasper, r.Stone2); ratio < float64(tokens.ContrastMinimum) {
			t.Fatalf("dark %v jasper contrast = %.2f", dark, ratio)
		}
	}
}

func TestTintMixesStonesTowardTheTintRole(t *testing.T) {
	p := syntheticPalette(false)
	tint := color.NRGBA{R: 30, G: 120, B: 90, A: 255}
	p[tokens.PaletteAccents["tint"]] = tint
	standard := FromPalette(Palette{Roles: p, Dark: false, Source: "wallpaper"}, Look{})
	for _, test := range []struct {
		tint   Tint
		amount float32
	}{
		{TintLow, tokens.AlphaTintLow},
		{TintMedium, tokens.AlphaTintMedium},
	} {
		r := FromPalette(Palette{Roles: p, Dark: false, Source: "wallpaper"}, Look{Tint: test.tint})
		assertColor(t, "ground", r.Ground, Mix(standard.Ground, tint, test.amount))
		assertColor(t, "stone2", r.Stone2, Mix(standard.Stone2, tint, test.amount))
		assertColor(t, "slab", r.Slab, Mix(standard.Slab, tint, test.amount))
		assertColor(t, "field", r.Field, Mix(standard.Field, tint, test.amount))
		assertColor(t, "vein", r.Vein, standard.Vein)
		assertColor(t, "edge", r.Edge, standard.Edge)
		assertColor(t, "fg", r.Fg, standard.Fg)
	}
}

func TestContrastRunsBeforeTint(t *testing.T) {
	p := syntheticPalette(true)
	tint := color.NRGBA{R: 30, G: 60, B: 120, A: 255}
	p[tokens.PaletteAccents["tint"]] = tint
	high := FromPalette(Palette{Roles: p, Dark: true, Source: "wallpaper"}, Look{Contrast: High})
	both := FromPalette(Palette{Roles: p, Dark: true, Source: "wallpaper"}, Look{Contrast: High, Tint: TintMedium})
	assertColor(t, "stone3", both.Stone3, Mix(high.Stone3, tint, tokens.AlphaTintMedium))
	assertColor(t, "vein", both.Vein, high.Vein)
}

func TestGoldInkAndJasperMeetMinimumContrast(t *testing.T) {
	p := syntheticPalette(true)
	p[tokens.PaletteAccents["gold"]] = color.NRGBA{R: 35, G: 35, B: 35, A: 255}
	p[tokens.PaletteAccents["jasper"]] = color.NRGBA{R: 42, G: 30, B: 30, A: 255}
	r := FromPalette(Palette{Roles: p, Dark: true, Source: "wallpaper"}, Look{})
	if ratio := ContrastRatio(r.GoldInk, r.Stone2); ratio < float64(tokens.ContrastMinimum) {
		t.Fatalf("goldInk contrast = %.2f", ratio)
	}
	if ratio := ContrastRatio(r.Jasper, r.Stone2); ratio < float64(tokens.ContrastMinimum) {
		t.Fatalf("jasper contrast = %.2f", ratio)
	}
}

func TestInkOnPicksReadableInk(t *testing.T) {
	if got := InkOn(color.NRGBA{R: 245, G: 245, B: 245, A: 255}); got != (color.NRGBA{A: 255}) {
		t.Fatalf("light background ink = %#v", got)
	}
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	if got := InkOn(color.NRGBA{R: 12, G: 12, B: 12, A: 255}); got != white {
		t.Fatalf("dark background ink = %#v", got)
	}
}

func TestNamedSchemeWinsOverWallpaper(t *testing.T) {
	config, cache := t.TempDir(), t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_CACHE_HOME", cache)
	writeJSON(t, filepath.Join(cache, "ryoku", "colors.json"), map[string]any{"primary": hexColor(1, 2, 3), "surface": hexColor(5, 5, 5)})
	writeJSON(t, filepath.Join(config, "ryoku", "shell.json"), map[string]any{"themePalette": map[string]any{"primary": hexColor(9, 8, 7)}})
	loaded := Load(Follow)
	if loaded.Source != "scheme" {
		t.Fatalf("source = %q", loaded.Source)
	}
	assertColor(t, "named primary", loaded.Roles["primary"], color.NRGBA{R: 9, G: 8, B: 7, A: 255})
}

func TestEditionAccentOnSignatureFallback(t *testing.T) {
	config, cache := t.TempDir(), t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_CACHE_HOME", cache)
	path := filepath.Join(t.TempDir(), "os-release")
	if err := os.WriteFile(path, []byte("ID=fedora\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	previous := osReleasePath
	osReleasePath = path
	t.Cleanup(func() { osReleasePath = previous })
	loaded := Load(Follow)
	if loaded.Source != "signature" {
		t.Fatalf("source = %q", loaded.Source)
	}
	want, ok := parseColor(tokens.Edition["fedora"])
	if !ok {
		t.Fatal("fedora token is not a colour")
	}
	assertColor(t, "edition gold", FromPalette(loaded, Look{}).Gold, want)
}

func syntheticPalette(dark bool) map[string]color.NRGBA {
	mapping := tokens.PaletteLight
	if dark {
		mapping = tokens.PaletteDark
	}
	result := make(map[string]color.NRGBA)
	index := uint8(20)
	for _, key := range mapping {
		result[key] = color.NRGBA{R: index, G: index + 1, B: index + 2, A: 255}
		index += 7
	}
	result[tokens.PaletteAccents["gold"]] = color.NRGBA{R: 201, G: 168, B: 97, A: 255}
	result[tokens.PaletteAccents["onGold"]] = color.NRGBA{R: 20, G: 15, B: 9, A: 255}
	result[tokens.PaletteAccents["sel"]] = color.NRGBA{R: 210, G: 208, B: 202, A: 255}
	result[tokens.PaletteAccents["onSel"]] = color.NRGBA{R: 25, G: 25, B: 24, A: 255}
	result[tokens.PaletteAccents["jasper"]] = color.NRGBA{R: 220, G: 70, B: 55, A: 255}
	result[tokens.PaletteAccents["scrim"]] = color.NRGBA{R: 1, G: 1, B: 1, A: 255}
	return result
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func hexColor(r, g, b uint8) string { return fmt.Sprintf("#%02x%02x%02x", r, g, b) }

func alphaByte(alpha float32) uint8 {
	return uint8(float32(255)*alpha + 0.5)
}

func assertColor(t *testing.T, name string, got, want color.NRGBA) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %#v, want %#v", name, got, want)
	}
}
