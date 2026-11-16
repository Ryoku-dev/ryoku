package palette

import (
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/design"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// home is a temporary desktop: a config and cache tree and an os-release naming no edition.
type home struct {
	config, cache, osRelease string
}

func newHome(t *testing.T) home {
	t.Helper()
	h := home{config: t.TempDir(), cache: t.TempDir(), osRelease: filepath.Join(t.TempDir(), "os-release")}
	if err := os.WriteFile(h.osRelease, []byte("ID=plain\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return h
}

func (h home) options(env bool) []Option {
	return []Option{WithEnv(env), WithDirs(h.config, h.cache, h.osRelease)}
}

func (h home) read(t *testing.T) Desktop {
	t.Helper()
	return ReadDesktop(h.options(false)...)
}

func (h home) shell(t *testing.T, value any) {
	writeJSON(t, filepath.Join(h.config, "ryoku", "shell.json"), value)
}

func (h home) theme(t *testing.T, value any) {
	writeJSON(t, filepath.Join(h.config, "ryoku", "theme.json"), value)
}

func (h home) matugen(t *testing.T, mode string) {
	writeJSON(t, filepath.Join(h.config, "ryoku", "matugen.json"), map[string]any{"mode": mode})
}

func (h home) colors(t *testing.T, value any) {
	writeJSON(t, filepath.Join(h.cache, "ryoku", "colors.json"), value)
}

// materialRoles is a Material palette as colors.json carries it, as hex strings.
func materialRoles(dark bool) map[string]any {
	if dark {
		return map[string]any{
			"surface": "#101418", "surfaceContainerLow": "#191c20", "surfaceContainer": "#1d2024",
			"surfaceContainerHigh": "#272a2f", "surfaceContainerHighest": "#32353a",
			"outlineVariant": "#42474e", "outline": "#8c9199", "onSurface": "#e0e2e8",
			"onSurfaceVariant": "#c2c7cf", "primary": "#9dcbfc", "onPrimary": "#003355",
			"tertiary": "#d4bee6", "onTertiary": "#392a49", "error": "#ffb4ab",
			"secondaryContainer": "#3a4857", "onSecondaryContainer": "#d6e4f7",
			"primaryContainer": "#134a74", "scrim": "#000000",
		}
	}
	return map[string]any{
		"surface": "#f6faf6", "surfaceContainerLow": "#f0f5f0", "surfaceContainer": "#eaefea",
		"surfaceContainerHigh": "#e4e9e5", "surfaceContainerHighest": "#dfe4df",
		"outlineVariant": "#bfc9c2", "outline": "#707973", "onSurface": "#171d1a",
		"onSurfaceVariant": "#404943", "primary": "#2c6a56", "onPrimary": "#ffffff",
		"tertiary": "#3d6373", "onTertiary": "#ffffff", "error": "#ba1a1a",
		"secondaryContainer": "#cee9dc", "onSecondaryContainer": "#082018",
		"primaryContainer": "#b1f0d7", "scrim": "#000000",
	}
}

func hexRoles(t *testing.T, object map[string]any) map[string]color.NRGBA {
	t.Helper()
	roles := colorRoles(object)
	if len(roles) != len(object) {
		t.Fatal("fixture holds a value that is not a colour")
	}
	return roles
}

func TestReadDesktopWithoutFiles(t *testing.T) {
	d := newHome(t).read(t)
	want := Desktop{Gloss: true, Palette: Palette{Roles: map[string]color.NRGBA{}, Dark: true, Source: "signature"}, Roles: Signature(true)}
	if !d.Equal(want) {
		t.Fatalf("empty desktop = %+v", d)
	}
	if d.Design != design.Lithic || d.Colours != Ryoku || d.Mode != Dark || d.Look != (Look{}) || d.ReduceMotion || !d.Gloss {
		t.Fatalf("defaults = %+v", d)
	}
}

// TestLightOrDark mirrors Lithic.qml:776-778: matugen's dark or light wins; smart, sun,
// another word or no file leave it to colors.json's surface, even on Ryoku's colours.
func TestLightOrDark(t *testing.T) {
	for _, surface := range []struct {
		name string
		dark bool
	}{{"dark surface", true}, {"light surface", false}} {
		for _, test := range []struct {
			matugen string
			want    Mode
		}{
			{"dark", Dark},
			{"light", Light},
			{"smart", map[bool]Mode{true: Dark, false: Light}[surface.dark]},
			{"sun", map[bool]Mode{true: Dark, false: Light}[surface.dark]},
			{"", map[bool]Mode{true: Dark, false: Light}[surface.dark]},
		} {
			h := newHome(t)
			h.colors(t, materialRoles(surface.dark))
			if test.matugen != "" {
				h.matugen(t, test.matugen)
			}
			d := h.read(t)
			if d.Mode != test.want {
				t.Errorf("%s, matugen %q: mode = %v, want %v", surface.name, test.matugen, d.Mode, test.want)
			}
			if d.Roles != Signature(test.want == Dark) {
				t.Errorf("%s, matugen %q: Ryoku's colours are not the signature", surface.name, test.matugen)
			}
		}
	}
}

func TestNoSurfaceIsDark(t *testing.T) {
	h := newHome(t)
	h.matugen(t, "smart")
	h.colors(t, map[string]any{"primary": "#9dcbfc"})
	if d := h.read(t); d.Mode != Dark {
		t.Fatalf("mode = %v", d.Mode)
	}
}

func TestShellSettings(t *testing.T) {
	h := newHome(t)
	h.shell(t, map[string]any{
		"showJapaneseGloss": false,
		"theme": map[string]any{
			"design": "adwaita", "colours": "ryoku", "contrast": "high", "tint": "medium",
			"motion": map[string]any{"reduce": true, "scale": 1.5},
		},
	})
	d := h.read(t)
	if d.Design != design.Adwaita || d.Colours != Ryoku || d.Mode != Dark || !d.ReduceMotion || d.Gloss {
		t.Fatalf("desktop = %+v", d)
	}
	if d.Look != (Look{Contrast: High, Tint: TintMedium}) {
		t.Fatalf("look = %+v", d.Look)
	}
	if d.Roles != FromPalette(Palette{Dark: true, Source: "signature"}, d.Look) {
		t.Fatal("adwaita on Ryoku's colours is not the high-contrast signature")
	}

	h.shell(t, map[string]any{"theme": map[string]any{"design": "bauhaus", "contrast": "max", "tint": "loud", "motion": "no"}})
	d = h.read(t)
	if d.Design != design.Lithic || d.Look != (Look{}) || d.ReduceMotion || !d.Gloss {
		t.Fatalf("invalid values = %+v", d)
	}
}

func TestWallpaperColoursFollow(t *testing.T) {
	h := newHome(t)
	h.colors(t, materialRoles(true))
	h.shell(t, map[string]any{"theme": map[string]any{"colours": "wallpaper", "design": "lithic", "tint": "low"}})
	d := h.read(t)
	if d.Mode != Follow || d.Colours != Wallpaper || d.Palette.Source != "wallpaper" || !d.Palette.Dark {
		t.Fatalf("desktop = %+v", d)
	}
	wallpaperPalette := Palette{Roles: hexRoles(t, materialRoles(true)), Dark: true, Source: "wallpaper"}
	if d.Roles != FromPalette(wallpaperPalette, Look{Tint: TintLow}) {
		t.Fatal("Lithic on the wallpaper is not the toned ladder")
	}
	for _, style := range []design.Style{design.Material, design.Adwaita, design.Fluent} {
		if d.RolesFor(style) != Material(wallpaperPalette, Look{Tint: TintLow}) {
			t.Fatalf("%s on the wallpaper does not wear the Material roles", style)
		}
	}

	// matugen's word does not matter while following: the surface decides
	h.matugen(t, "light")
	if d := h.read(t); d.Mode != Follow || !d.Palette.Dark {
		t.Fatalf("matugen light while following = %+v", d)
	}
}

func TestDesignWearsTheDesktopPalette(t *testing.T) {
	h := newHome(t)
	h.colors(t, materialRoles(false))
	h.shell(t, map[string]any{"theme": map[string]any{"colours": "wallpaper", "design": "material"}})
	d := h.read(t)
	if d.Design != design.Material || d.Palette.Dark {
		t.Fatalf("desktop = %+v", d)
	}
	roles := hexRoles(t, materialRoles(false))
	for name, pair := range map[string][2]color.NRGBA{
		"ground":      {d.Roles.Ground, roles["surface"]},
		"stone1":      {d.Roles.Stone1, roles["surfaceContainerLow"]},
		"stone2":      {d.Roles.Stone2, roles["surface"]},
		"stone3":      {d.Roles.Stone3, roles["surfaceContainer"]},
		"stone4":      {d.Roles.Stone4, roles["surfaceContainerHigh"]},
		"slab":        {d.Roles.Slab, roles["surface"]},
		"field":       {d.Roles.Field, roles["surfaceContainerHighest"]},
		"vein":        {d.Roles.Vein, roles["outlineVariant"]},
		"edge":        {d.Roles.Edge, roles["outline"]},
		"fg":          {d.Roles.Fg, roles["onSurface"]},
		"fg2":         {d.Roles.Fg2, roles["onSurfaceVariant"]},
		"fg3":         {d.Roles.Fg3, Mix(roles["onSurfaceVariant"], roles["surface"], 0.22)},
		"sel":         {d.Roles.Sel, roles["primary"]},
		"onSel":       {d.Roles.OnSel, roles["onPrimary"]},
		"onSel2":      {d.Roles.OnSel2, Mix(roles["onPrimary"], roles["primary"], tokens.AlphaOnSel2)},
		"gold":        {d.Roles.Gold, roles["tertiary"]},
		"goldInk":     {d.Roles.GoldInk, roles["tertiary"]},
		"onGold":      {d.Roles.OnGold, roles["onTertiary"]},
		"jasper":      {d.Roles.Jasper, roles["error"]},
		"container":   {d.Roles.Container, roles["secondaryContainer"]},
		"onContainer": {d.Roles.OnContainer, roles["onSecondaryContainer"]},
	} {
		assertColor(t, name, pair[0], pair[1])
	}
	if d.RolesFor(design.Lithic) != FromPalette(d.Palette, Look{}) {
		t.Fatal("a Lithic preview is not the toned ladder")
	}
}

func TestMaterialKeepsTheLadderForMissingRoles(t *testing.T) {
	partial := Palette{Roles: map[string]color.NRGBA{
		"surface": {R: 0x10, G: 0x14, B: 0x18, A: 0xff},
		"primary": {R: 0x9d, G: 0xcb, B: 0xfc, A: 0xff},
		"error":   {R: 0xff, G: 0xb4, B: 0xab, A: 0xff},
	}, Dark: true, Source: "wallpaper"}
	ladder := FromPalette(partial, Look{Contrast: High})
	r := Material(partial, Look{Contrast: High})
	assertColor(t, "ground", r.Ground, partial.Roles["surface"])
	assertColor(t, "stone1 keeps the ladder", r.Stone1, ladder.Stone1)
	assertColor(t, "fg keeps the ladder", r.Fg, ladder.Fg)
	assertColor(t, "fg3 keeps the ladder", r.Fg3, ladder.Fg3)
	assertColor(t, "onSel keeps the ladder", r.OnSel, ladder.OnSel)
	assertColor(t, "onSel2 mixes the new selection", r.OnSel2, Mix(ladder.OnSel, partial.Roles["primary"], tokens.AlphaOnSel2))
	// the container comes from the ladder's selection: _withMinerals runs before the primary
	assertColor(t, "container", r.Container, WithAlpha(ladder.Sel, 0.16))
	assertColor(t, "onContainer", r.OnContainer, ladder.Fg)
	assertColor(t, "scrim", r.Scrim, ladder.Scrim)
	// the failed field keeps the toned jasper's hue, as Lithic.qml computes the fields first
	assertColor(t, "failed field", r.MineralField(Failed), ladder.MineralField(Failed))
	assertColor(t, "alive field", r.MineralField(Alive), ladder.MineralField(Alive))
}

func TestContainerOnTheLadder(t *testing.T) {
	for _, dark := range []bool{true, false} {
		r := Signature(dark)
		alpha := map[bool]float32{true: 0.16, false: 0.14}[dark]
		assertColor(t, "container", r.Container, WithAlpha(r.Sel, alpha))
		assertColor(t, "onContainer", r.OnContainer, r.Fg)
	}
}

func TestUnfollowedWallpaper(t *testing.T) {
	h := newHome(t)
	h.colors(t, materialRoles(true))
	h.theme(t, map[string]any{"followWallpaper": false})
	h.shell(t, map[string]any{"theme": map[string]any{"colours": "wallpaper", "design": "fluent"}})
	d := h.read(t)
	if d.Mode != Follow || d.Palette.Source != "signature" || len(d.Palette.Roles) != 0 {
		t.Fatalf("desktop = %+v", d)
	}
	// no surface to wear: Fluent falls back to Lithic's ladder, the signature
	if d.Roles != Signature(true) {
		t.Fatal("fluent with the wallpaper unfollowed is not the signature")
	}
}

func TestSchemeOverWallpaper(t *testing.T) {
	h := newHome(t)
	h.colors(t, materialRoles(true))
	h.shell(t, map[string]any{
		"theme":        map[string]any{"colours": "wallpaper", "design": "material"},
		"themePalette": map[string]any{"primary": "#8a3f6b", "surface": "#fbf8fa", "bogus": "red"},
	})
	d := h.read(t)
	if d.Palette.Source != "scheme" || d.Palette.Dark {
		t.Fatalf("palette = %+v", d.Palette)
	}
	assertColor(t, "scheme primary", d.Roles.Sel, color.NRGBA{R: 0x8a, G: 0x3f, B: 0x6b, A: 0xff})
	assertColor(t, "scheme surface", d.Roles.Ground, color.NRGBA{R: 0xfb, G: 0xf8, B: 0xfa, A: 0xff})
	assertColor(t, "wallpaper outline under the scheme", d.Roles.Edge, color.NRGBA{R: 0x8c, G: 0x91, B: 0x99, A: 0xff})
	if _, ok := d.Palette.Roles["bogus"]; ok {
		t.Fatal("a non-colour reached the palette")
	}
}

func TestEditionAccentOnRyokuColours(t *testing.T) {
	h := newHome(t)
	if err := os.WriteFile(h.osRelease, []byte("ID=\"fedora\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	h.shell(t, map[string]any{"theme": map[string]any{"design": "material"}})
	want, _ := parseColor(tokens.Edition["fedora"])
	d := h.read(t)
	assertColor(t, "edition gold", d.Roles.Gold, want)
	assertColor(t, "edition gold as Lithic", d.RolesFor(design.Lithic).Gold, want)
}

func TestEnvironmentOverrides(t *testing.T) {
	h := newHome(t)
	h.colors(t, materialRoles(true))
	h.shell(t, map[string]any{"theme": map[string]any{"design": "material", "colours": "wallpaper", "contrast": "high", "tint": "low"}})

	t.Setenv("LITHIC_MODE", "light")
	t.Setenv("LITHIC_DESIGN", "fluent")
	d := ReadDesktop(h.options(true)...)
	if d.Mode != Light || d.Colours != Ryoku || d.Look != (Look{}) || d.Design != design.Fluent {
		t.Fatalf("forced = %+v", d)
	}
	if d.Roles != Signature(false) {
		t.Fatal("forced light is not the light signature")
	}

	t.Setenv("LITHIC_MODE", "follow")
	t.Setenv("LITHIC_DESIGN", "nope")
	d = ReadDesktop(h.options(true)...)
	if d.Mode != Follow || d.Colours != Wallpaper || d.Design != design.Material || d.Look != (Look{}) {
		t.Fatalf("forced follow = %+v", d)
	}

	t.Setenv("LITHIC_MODE", "sepia")
	if d := ReadDesktop(h.options(true)...); d.Mode != Follow || d.Look != (Look{Contrast: High, Tint: TintLow}) {
		t.Fatalf("an invalid LITHIC_MODE forced something: %+v", d)
	}

	t.Setenv("LITHIC_MODE", "light")
	t.Setenv("LITHIC_DESIGN", "adwaita")
	if d := ReadDesktop(h.options(false)...); d.Mode != Follow || d.Design != design.Material {
		t.Fatalf("WithEnv(false) honoured the environment: %+v", d)
	}
}

func TestHostPalette(t *testing.T) {
	h := newHome(t)
	h.colors(t, materialRoles(true))
	h.shell(t, map[string]any{"theme": map[string]any{"colours": "wallpaper", "design": "adwaita"}})
	host := filepath.Join(t.TempDir(), "host.json")
	writeJSON(t, host, map[string]any{"themePalette": materialRoles(false)})
	t.Setenv("LITHIC_PALETTE_JSON", host)
	t.Setenv("LITHIC_MODE", "")
	t.Setenv("LITHIC_DESIGN", "")
	d := ReadDesktop(h.options(true)...)
	if d.Palette.Source != "host" || d.Palette.Dark {
		t.Fatalf("palette = %+v", d.Palette)
	}
	hostPalette := Palette{Roles: hexRoles(t, materialRoles(false)), Dark: false, Source: "host"}
	if d.Roles != Material(hostPalette, Look{}) {
		t.Fatal("adwaita does not wear the host's roles")
	}
	if got, want := d.RolesFor(design.Lithic), rolesFrom(hostPalette, Look{}, true); got != want {
		t.Fatal("Lithic does not dye from the host's roles")
	}

	// a whole-object host file without themePalette counts too
	writeJSON(t, host, map[string]any{"surface": "#101418", "primary": "#9dcbfc"})
	if d := ReadDesktop(h.options(true)...); d.Palette.Source != "host" || len(d.Palette.Roles) != 2 {
		t.Fatalf("whole-object host = %+v", d.Palette)
	}
}

func TestPlanesAreLithicQML(t *testing.T) {
	r := Material(Palette{Roles: hexRoles(t, materialRoles(true)), Dark: true, Source: "wallpaper"}, Look{})
	clear := color.NRGBA{}
	for _, test := range []struct {
		style design.Style
		want  Planes
	}{
		{design.Lithic, Planes{r.Ground, r.Stone2, r.Vein, r.Slab, r.Vein}},
		{design.Material, Planes{r.Ground, r.Stone1, clear, r.Stone3, clear}},
		{design.Adwaita, Planes{r.Stone3, r.Ground, clear, r.Stone4, clear}},
		{design.Fluent, Planes{r.Ground, r.Stone3, r.Vein, r.Stone4, WithAlpha(r.Vein, 0.7)}},
	} {
		if got := r.Planes(test.style); got != test.want {
			t.Errorf("%s planes = %+v, want %+v", test.style, got, test.want)
		}
	}
	if got := r.Planes(design.Fluent).CardStroke.A; got != 179 {
		t.Errorf("fluent card stroke alpha = %d, want 179", got)
	}
}

func TestDesktopEqual(t *testing.T) {
	h := newHome(t)
	h.colors(t, materialRoles(true))
	h.shell(t, map[string]any{"theme": map[string]any{"colours": "wallpaper"}})
	a, b := h.read(t), h.read(t)
	if !a.Equal(b) {
		t.Fatal("two readings of the same files differ")
	}
	for name, change := range map[string]func(*Desktop){
		"design":  func(d *Desktop) { d.Design = design.Fluent },
		"colours": func(d *Desktop) { d.Colours = Ryoku },
		"mode":    func(d *Desktop) { d.Mode = Dark },
		"look":    func(d *Desktop) { d.Look.Tint = TintLow },
		"reduce":  func(d *Desktop) { d.ReduceMotion = true },
		"gloss":   func(d *Desktop) { d.Gloss = false },
		"source":  func(d *Desktop) { d.Palette.Source = "scheme" },
		"roles":   func(d *Desktop) { d.Roles.Sel.R++ },
		"palette": func(d *Desktop) {
			d.Palette.Roles = map[string]color.NRGBA{"surface": {A: 255}}
		},
	} {
		c := h.read(t)
		change(&c)
		if a.Equal(c) {
			t.Errorf("a %s change reads as equal", name)
		}
	}
}

func TestColoursString(t *testing.T) {
	if Ryoku.String() != "ryoku" || Wallpaper.String() != "wallpaper" {
		t.Fatal("colours names")
	}
}
