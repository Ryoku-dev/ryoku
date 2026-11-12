package palette

import (
	"encoding/json"
	"fmt"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

func TestSignatureIsTheTable(t *testing.T) {
	for _, test := range []struct {
		name     string
		dark     bool
		contrast Contrast
		table    map[string]string
	}{
		{"dark", true, Standard, tokens.SignatureDark},
		{"dark high", true, High, tokens.SignatureDarkHigh},
		{"light", false, Standard, tokens.SignatureLight},
		{"light high", false, High, tokens.SignatureLightHigh},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := FromPalette(Palette{Dark: test.dark, Source: "signature"}, Look{Contrast: test.contrast})
			for name, got := range named(r) {
				if name == "scrim" {
					continue
				}
				want, ok := parseColor(test.table[name])
				if !ok {
					t.Fatalf("signature has no %s", name)
				}
				assertColor(t, name, got, want)
			}
			tinted := FromPalette(Palette{Dark: test.dark, Source: "signature"}, Look{Contrast: test.contrast, Tint: TintMedium})
			if tinted != r {
				t.Fatal("a tint changed the signature")
			}
		})
	}
	if Signature(true) != FromPalette(Palette{Dark: true, Source: "signature"}, Look{}) {
		t.Fatal("Signature(true) is not the standard dark table")
	}
}

func TestEditionColourTakesOnlyTheAction(t *testing.T) {
	signal := color.NRGBA{R: 110, G: 165, B: 207, A: 255}
	r := FromPalette(Palette{Dark: true, Source: "signature", Roles: map[string]color.NRGBA{tokens.PaletteAccents["gold"]: signal}}, Look{})
	signature := Signature(true)
	assertColor(t, "gold", r.Gold, signal)
	assertColor(t, "goldInk", r.GoldInk, signal)
	assertColor(t, "onGold", r.OnGold, InkOn(signal))
	assertColor(t, "sel", r.Sel, signature.Sel)
	assertColor(t, "ground", r.Ground, signature.Ground)
}

func TestThePaletteDyesAndTheLadderSetsLightness(t *testing.T) {
	blue := wallpaper(250, 0.05, 0.30)
	brown := wallpaper(60, 0.03, 0.12)
	a := FromPalette(Palette{Roles: blue, Dark: true, Source: "wallpaper"}, Look{})
	b := FromPalette(Palette{Roles: brown, Dark: true, Source: "wallpaper"}, Look{})
	steps := tokens.ToneDark
	for name, got := range named(a) {
		step, ok := steps[name]
		if !ok || name == "scrim" {
			continue
		}
		if l := lightness(got); math.Abs(l-step) > 0.01 {
			t.Errorf("%s lightness = %.3f, want the ladder's %.3f", name, l, step)
		}
		if l := lightness(named(b)[name]); math.Abs(l-step) > 0.01 {
			t.Errorf("%s on another wallpaper = %.3f, want the ladder's %.3f", name, l, step)
		}
	}
	if hue := dyeOf(a.Gold).H; angle(hue, 250) > 3 {
		t.Errorf("gold hue = %.1f, want the primary's 250", hue)
	}
	if hue := dyeOf(b.Gold).H; angle(hue, 60) > 3 {
		t.Errorf("gold hue = %.1f, want the primary's 60", hue)
	}
}

func TestDarkAndLightAreTheSignatureWhileAWallpaperPaletteExists(t *testing.T) {
	config, cache := t.TempDir(), t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_CACHE_HOME", cache)
	useOSRelease(t, "arch")
	colors := map[string]any{}
	for key, value := range wallpaper(150, 0.06, 0.2) {
		colors[key] = hexColor(value.R, value.G, value.B)
	}
	writeJSON(t, filepath.Join(cache, "ryoku", "colors.json"), colors)
	writeJSON(t, filepath.Join(config, "ryoku", "theme.json"), map[string]any{"followWallpaper": true})
	for _, test := range []struct {
		mode Mode
		dark bool
	}{{Dark, true}, {Light, false}} {
		loaded := Load(test.mode)
		if loaded.Source != "signature" || loaded.Dark != test.dark || len(loaded.Roles) != 0 {
			t.Fatalf("Load(%v) = %+v, want the bare %v signature", test.mode, loaded, test.mode)
		}
		if FromPalette(loaded, Look{}) != Signature(test.dark) {
			t.Fatalf("Load(%v) does not paint the signature", test.mode)
		}
	}
	if loaded := Load(Follow); loaded.Source != "wallpaper" {
		t.Fatalf("Load(Follow) source = %q, want wallpaper", loaded.Source)
	}
}

func TestASchemeAppliesOnlyWhenFollowing(t *testing.T) {
	config, cache := t.TempDir(), t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_CACHE_HOME", cache)
	useOSRelease(t, "arch")
	writeJSON(t, filepath.Join(config, "ryoku", "shell.json"), map[string]any{"themePalette": map[string]any{"primary": hexColor(9, 8, 7)}})
	if loaded := Load(Dark); loaded.Source != "signature" {
		t.Fatalf("Load(Dark) source = %q, want signature", loaded.Source)
	}
	if loaded := Load(Follow); loaded.Source != "scheme" {
		t.Fatalf("Load(Follow) source = %q, want scheme", loaded.Source)
	}
}

func TestUnfollowedWallpaperFallsBackToTheSignature(t *testing.T) {
	config, cache := t.TempDir(), t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_CACHE_HOME", cache)
	useOSRelease(t, "arch")
	writeJSON(t, filepath.Join(cache, "ryoku", "colors.json"), map[string]any{"primary": hexColor(1, 2, 3), "surface": hexColor(250, 250, 250)})
	writeJSON(t, filepath.Join(config, "ryoku", "theme.json"), map[string]any{"followWallpaper": false})
	loaded := Load(Follow)
	if loaded.Source != "signature" || !loaded.Dark {
		t.Fatalf("Load(Follow) = %+v, want the dark signature", loaded)
	}
}

func TestFollowTakesItsModeFromTheSurface(t *testing.T) {
	config, cache := t.TempDir(), t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_CACHE_HOME", cache)
	writeJSON(t, filepath.Join(cache, "ryoku", "colors.json"), map[string]any{"primary": hexColor(1, 2, 3), "surface": hexColor(250, 250, 250)})
	if loaded := Load(Follow); loaded.Source != "wallpaper" || loaded.Dark {
		t.Fatalf("Load(Follow) = %+v, want a light wallpaper", loaded)
	}
}

func TestTintMovesColourNeverLightness(t *testing.T) {
	roles := wallpaper(30, 0.06, 0.2)
	off := FromPalette(Palette{Roles: roles, Dark: true, Source: "wallpaper"}, Look{})
	low := FromPalette(Palette{Roles: roles, Dark: true, Source: "wallpaper"}, Look{Tint: TintLow})
	medium := FromPalette(Palette{Roles: roles, Dark: true, Source: "wallpaper"}, Look{Tint: TintMedium})
	for _, plane := range []struct {
		name             string
		off, low, medium color.NRGBA
	}{
		{"ground", off.Ground, low.Ground, medium.Ground},
		{"stone2", off.Stone2, low.Stone2, medium.Stone2},
		{"field", off.Field, low.Field, medium.Field},
	} {
		if d := math.Abs(lightness(plane.medium) - lightness(plane.off)); d > 0.005 {
			t.Errorf("%s lightness moved by %.3f under a tint", plane.name, d)
		}
		if !(chroma(plane.off) < chroma(plane.low) && chroma(plane.low) < chroma(plane.medium)) {
			t.Errorf("%s chroma off %.4f, low %.4f, medium %.4f, want it to rise", plane.name, chroma(plane.off), chroma(plane.low), chroma(plane.medium))
		}
	}
	assertColor(t, "fg under a tint", medium.Fg, off.Fg)
}

// Panes and slabs lie on the ground's own tone at either contrast; what steps
// up from it (a field, a hover, an overlay, a hairline) steps further at high.
func TestHighContrastSpreadsWhatStepsUp(t *testing.T) {
	roles := wallpaper(200, 0.05, 0.2)
	standard := FromPalette(Palette{Roles: roles, Dark: true, Source: "wallpaper"}, Look{})
	high := FromPalette(Palette{Roles: roles, Dark: true, Source: "wallpaper"}, Look{Contrast: High})
	for _, r := range []Roles{standard, high} {
		for name, plane := range map[string]color.NRGBA{"stone1": r.Stone1, "stone2": r.Stone2, "slab": r.Slab} {
			if d := math.Abs(lightness(plane) - lightness(r.Ground)); d > 0.01 {
				t.Fatalf("%s sits %.3f off the ground, want it on the ground's tone", name, d)
			}
		}
	}
	for name, step := range map[string]func(Roles) color.NRGBA{
		"field":  func(r Roles) color.NRGBA { return r.Field },
		"stone3": func(r Roles) color.NRGBA { return r.Stone3 },
		"stone4": func(r Roles) color.NRGBA { return r.Stone4 },
		"vein":   func(r Roles) color.NRGBA { return r.Vein },
	} {
		gap := func(r Roles) float64 { return lightness(step(r)) - lightness(r.Ground) }
		if gap(high) <= gap(standard) {
			t.Fatalf("%s sits %.3f above the ground at high contrast, %.3f at standard", name, gap(high), gap(standard))
		}
	}
	if ContrastRatio(high.Fg3, high.Stone2) <= ContrastRatio(standard.Fg3, standard.Stone2) {
		t.Fatal("high contrast did not lift fg3")
	}
}

// Text must stay readable on whatever wallpaper a reader picks, so the floors
// are checked around the whole hue circle at every tint, not on one palette.
func TestEveryHueKeepsTheContrastFloors(t *testing.T) {
	for _, dark := range []bool{true, false} {
		for _, contrast := range []Contrast{Standard, High} {
			floor := 4.6
			if contrast == High {
				floor = 5.0
			}
			check := func(t *testing.T, r Roles, what string) {
				t.Helper()
				planes := map[string]color.NRGBA{"stone1": r.Stone1, "stone2": r.Stone2, "slab": r.Slab}
				inks := map[string]struct {
					c     color.NRGBA
					floor float64
				}{
					"goldInk": {r.GoldInk, floor}, "jasper": {r.Jasper, floor}, "fg3": {r.Fg3, floor}, "fg2": {r.Fg2, floor},
					"jade": {r.Jade, 4.6}, "lapis": {r.Lapis, 4.6}, "amber": {r.Amber, 4.6},
				}
				for ink, want := range inks {
					for plane, background := range planes {
						if ratio := ContrastRatio(want.c, background); ratio < want.floor {
							t.Fatalf("%s: %s on %s = %.2f, want %.1f", what, ink, plane, ratio, want.floor)
						}
					}
				}
				if ratio := ContrastRatio(r.Fg2, r.Field); ratio < floor {
					t.Fatalf("%s: fg2 on field = %.2f, want %.1f", what, ratio, floor)
				}
				for plane, background := range map[string]color.NRGBA{"stone1": r.Stone1, "stone2": r.Stone2, "stone3": r.Stone3, "stone4": r.Stone4, "slab": r.Slab, "field": r.Field} {
					if ratio := ContrastRatio(r.Sel, background); ratio < 3 {
						t.Fatalf("%s: sel on %s = %.2f, want 3", what, plane, ratio)
					}
				}
				if ratio := ContrastRatio(r.OnSel, r.Sel); ratio < 4.5 {
					t.Fatalf("%s: onSel on sel = %.2f, want 4.5", what, ratio)
				}
				if ratio := ContrastRatio(r.OnSel2, r.Sel); ratio < 4.5 {
					t.Fatalf("%s: onSel2 on sel = %.2f, want 4.5", what, ratio)
				}
			}
			name := fmt.Sprintf("dark %v contrast %d", dark, contrast)
			t.Run(name, func(t *testing.T) {
				check(t, FromPalette(Palette{Dark: dark, Source: "signature"}, Look{Contrast: contrast}), "signature")
				for _, tint := range []Tint{TintOff, TintLow, TintMedium} {
					for hue := range 360 {
						r := FromPalette(Palette{Roles: wallpaper(float64(hue), 0.12, 0.2), Dark: dark, Source: "wallpaper"}, Look{Contrast: contrast, Tint: tint})
						check(t, r, fmt.Sprintf("hue %d tint %d", hue, tint))
					}
				}
			})
		}
	}
}

func TestScrimTakesTheModesAlpha(t *testing.T) {
	roles := wallpaper(0, 0.02, 0.1)
	if r := FromPalette(Palette{Roles: roles, Dark: true, Source: "wallpaper"}, Look{}); r.Scrim.A != alphaByte(tokens.AlphaScrimDark) {
		t.Fatalf("dark scrim alpha = %d", r.Scrim.A)
	}
	if r := FromPalette(Palette{Roles: roles, Dark: false, Source: "wallpaper"}, Look{}); r.Scrim.A != alphaByte(tokens.AlphaScrimLight) {
		t.Fatalf("light scrim alpha = %d", r.Scrim.A)
	}
}

func TestMineralFieldTakesItsMineralsHue(t *testing.T) {
	r := FromPalette(Palette{Roles: wallpaper(300, 0.08, 0.2), Dark: true, Source: "wallpaper"}, Look{})
	for _, test := range []struct {
		tone    Tone
		mineral color.NRGBA
	}{
		{Alive, r.Jade}, {Becoming, r.Lapis}, {Attention, r.Amber}, {Failed, r.Jasper},
	} {
		field := r.MineralField(test.tone)
		if l := lightness(field); math.Abs(l-tokens.ToneDark["mineralField"]) > 0.01 {
			t.Errorf("tone %d field lightness = %.3f", test.tone, l)
		}
		if a := angle(dyeOf(field).H, dyeOf(test.mineral).H); a > 8 {
			t.Errorf("tone %d field hue is %.1f degrees off its mineral", test.tone, a)
		}
	}
	assertColor(t, "idle field", r.MineralField(Idle), r.Stone3)
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

func TestEditionAccentOnSignatureInEveryMode(t *testing.T) {
	config, cache := t.TempDir(), t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_CACHE_HOME", cache)
	useOSRelease(t, "fedora")
	want, ok := parseColor(tokens.Edition["fedora"])
	if !ok {
		t.Fatal("fedora token is not a colour")
	}
	for _, mode := range []Mode{Dark, Light, Follow} {
		loaded := Load(mode)
		if loaded.Source != "signature" {
			t.Fatalf("Load(%v) source = %q", mode, loaded.Source)
		}
		assertColor(t, mode.String()+" edition gold", FromPalette(loaded, Look{}).Gold, want)
	}
}

// useOSRelease points the edition lookup at an os-release naming id.
func useOSRelease(t *testing.T, id string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "os-release")
	if err := os.WriteFile(path, []byte("ID="+id+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	previous := osReleasePath
	osReleasePath = path
	t.Cleanup(func() { osReleasePath = previous })
}

// wallpaper is a Material-style palette whose every role sits at one hue.
func wallpaper(hue, surfaceChroma, accentChroma float64) map[string]color.NRGBA {
	surface := toned(0.2, dye{C: surfaceChroma, H: hue})
	accent := toned(0.75, dye{C: accentChroma, H: hue})
	container := toned(0.35, dye{C: accentChroma / 2, H: hue})
	return map[string]color.NRGBA{
		tokens.PaletteDye["neutral"]:    surface,
		tokens.PaletteDye["ink"]:        toned(0.9, dye{C: surfaceChroma / 2, H: hue}),
		tokens.PaletteAccents["gold"]:   accent,
		tokens.PaletteAccents["jasper"]: toned(0.65, dye{C: accentChroma, H: hue}),
		tokens.PaletteAccents["sel"]:    container,
		tokens.PaletteAccents["selMix"]: toned(0.4, dye{C: accentChroma / 2, H: hue}),
		tokens.PaletteAccents["scrim"]:  {A: 255},
	}
}

func named(r Roles) map[string]color.NRGBA {
	return map[string]color.NRGBA{
		"ground": r.Ground, "stone1": r.Stone1, "stone2": r.Stone2, "stone3": r.Stone3, "stone4": r.Stone4,
		"slab": r.Slab, "field": r.Field, "vein": r.Vein, "edge": r.Edge, "fg": r.Fg, "fg2": r.Fg2, "fg3": r.Fg3,
		"sel": r.Sel, "onSel": r.OnSel, "onSel2": r.OnSel2, "gold": r.Gold, "onGold": r.OnGold, "goldInk": r.GoldInk,
		"jasper": r.Jasper, "scrim": r.Scrim,
	}
}

func lightness(c color.NRGBA) float64 {
	linear := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	r, g, b := linear(c.R), linear(c.G), linear(c.B)
	l := math.Cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*b)
	m := math.Cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*b)
	s := math.Cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*b)
	return 0.2104542553*l + 0.7936177850*m - 0.0040720468*s
}

func chroma(c color.NRGBA) float64 { return dyeOf(c).C }

func angle(a, b float64) float64 {
	d := math.Mod(math.Abs(a-b), 360)
	return math.Min(d, 360-d)
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
