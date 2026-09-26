package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestMatugenArgs pins the argv the pipeline hands matugen for each knob
// combination: the matugen.json tokens pass straight through, the resolved
// light/dark mode is the -m value, lightness / source-index / prefer are wired
// from the knobs, contrast clamps to [-1, 1], and -t is the schemeType variant
// (tonal-spot by default, or another when the matugen.json knob selects one).
func TestMatugenArgs(t *testing.T) {
	cases := []struct {
		name string
		k    matugenKnobs
		mode string
		want []string
	}{
		{
			"dark / saturation / 0.2",
			matugenKnobs{Prefer: "saturation", Contrast: 0.2},
			"dark",
			[]string{"image", "/w.png", "-t", "scheme-tonal-spot", "-m", "dark", "--contrast", "0.20", "--lightness-dark", "0.00", "--lightness-light", "0.00", "--source-color-index", "0", "--prefer", "saturation", "--json", "hex", "--dry-run"},
		},
		{
			"light / value / +0.5 / ld+0.3 / ll-0.2 / idx2",
			matugenKnobs{Prefer: "value", Contrast: 0.5, LightnessDark: 0.3, LightnessLight: -0.2, SourceColorIndex: 2},
			"light",
			[]string{"image", "/w.png", "-t", "scheme-tonal-spot", "-m", "light", "--contrast", "0.50", "--lightness-dark", "0.30", "--lightness-light", "-0.20", "--source-color-index", "2", "--prefer", "value", "--json", "hex", "--dry-run"},
		},
		{
			"contrast clamp high",
			matugenKnobs{Prefer: "darkness", Contrast: 5},
			"dark",
			[]string{"image", "/w.png", "-t", "scheme-tonal-spot", "-m", "dark", "--contrast", "1.00", "--lightness-dark", "0.00", "--lightness-light", "0.00", "--source-color-index", "0", "--prefer", "darkness", "--json", "hex", "--dry-run"},
		},
		{
			"contrast clamp low",
			matugenKnobs{Prefer: "less-saturation", Contrast: -5},
			"light",
			[]string{"image", "/w.png", "-t", "scheme-tonal-spot", "-m", "light", "--contrast", "-1.00", "--lightness-dark", "0.00", "--lightness-light", "0.00", "--source-color-index", "0", "--prefer", "less-saturation", "--json", "hex", "--dry-run"},
		},
		{
			"dark / vibrant variant",
			matugenKnobs{Prefer: "saturation", SchemeType: "scheme-vibrant"},
			"dark",
			[]string{"image", "/w.png", "-t", "scheme-vibrant", "-m", "dark", "--contrast", "0.00", "--lightness-dark", "0.00", "--lightness-light", "0.00", "--source-color-index", "0", "--prefer", "saturation", "--json", "hex", "--dry-run"},
		},
	}
	for _, c := range cases {
		got, err := matugenArgs("/w.png", c.k, c.mode)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", c.name, err)
			continue
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("%s:\n got %v\nwant %v", c.name, got, c.want)
		}
	}
}

// TestMatugenArgsRejectsUnknown: an out-of-schema token is an error, so the
// caller fails loudly instead of feeding matugen a value it would reject. An
// unresolved mode ("smart" reaching matugenArgs) is a caller bug and also fails.
func TestMatugenArgsRejectsUnknown(t *testing.T) {
	cases := []struct {
		name string
		k    matugenKnobs
		mode string
	}{
		{"unresolved smart mode", matugenKnobs{Prefer: "saturation"}, "smart"},
		{"unknown prefer", matugenKnobs{Prefer: "nope"}, "dark"},
		{"unknown schemeType", matugenKnobs{Prefer: "saturation", SchemeType: "scheme-nope"}, "dark"},
	}
	for _, c := range cases {
		if _, err := matugenArgs("/w.png", c.k, c.mode); err == nil {
			t.Errorf("%s: expected error, got nil", c.name)
		}
	}
}

// TestStoredSchemeTypeIsHonored: a matugen.json schemeType now reaches the argv,
// so the Hub's variant picker takes effect on the wallpaper path; an absent key
// still resolves to the tonal-spot default.
func TestStoredSchemeTypeIsHonored(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if err := os.MkdirAll(filepath.Join(home, ".config", "ryoku"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(home, ".config", "ryoku", "matugen.json"),
		`{"schemeType":"scheme-vibrant","mode":"dark","prefer":"saturation"}`)
	args, err := matugenArgs("/w.png", readMatugenKnobs(), "dark")
	if err != nil {
		t.Fatal(err)
	}
	got := ""
	for i, a := range args {
		if a == "-t" && i+1 < len(args) {
			got = args[i+1]
		}
	}
	if got != "scheme-vibrant" {
		t.Fatalf("-t = %q, want scheme-vibrant (a stored schemeType must be honored)", got)
	}
}

// TestReadMatugenKnobs proves the one knob store (matugen.json) is the source:
// a missing file yields the shipped defaults, and a full file maps every field
// (native tokens, lightness, source index, app-suite toggle, and the per-app
// roster) into the knobs the pipeline reads.
func TestReadMatugenKnobs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	ryoku := filepath.Join(home, ".config", "ryoku")
	if err := os.MkdirAll(ryoku, 0o755); err != nil {
		t.Fatal(err)
	}

	// Missing file -> defaults.
	k := readMatugenKnobs()
	if k.Mode != "smart" || k.Prefer != "saturation" || !k.ThemeRyokuApps || k.SchemeType != "scheme-tonal-spot" {
		t.Errorf("defaults: got %+v", k)
	}

	// Full file -> every field mapped.
	writeFile(t, filepath.Join(ryoku, "matugen.json"),
		`{"engine":"matugen","schemeType":"scheme-vibrant","mode":"dark","contrast":0.2,`+
			`"lightnessDark":0.1,"lightnessLight":-0.1,"prefer":"value","sourceColorIndex":3,`+
			`"themeRyokuApps":false,"templates":{"kitty":true,"btop":false,"gtk":true}}`)
	k = readMatugenKnobs()
	if k.Mode != "dark" ||
		k.Contrast != 0.2 || k.LightnessDark != 0.1 || k.LightnessLight != -0.1 ||
		k.Prefer != "value" || k.SchemeType != "scheme-vibrant" || k.SourceColorIndex != 3 || k.ThemeRyokuApps {
		t.Errorf("full file: got %+v", k)
	}
	if k.Templates["kitty"] != true || k.Templates["btop"] != false || k.Templates["gtk"] != true {
		t.Errorf("roster: got %v", k.Templates)
	}
}

// TestSmartMode is the smart-mode rule at the decision point: a dark image
// (low luma) selects a dark scheme, a light image a light one, split at the
// mid-point (>= 0.5 is light).
func TestSmartMode(t *testing.T) {
	cases := []struct {
		luma float64
		want string
	}{
		{0.00, "dark"}, {0.10, "dark"}, {0.4999, "dark"},
		{0.50, "light"}, {0.90, "light"}, {1.00, "light"},
	}
	for _, c := range cases {
		if got := smartMode(c.luma); got != c.want {
			t.Errorf("smartMode(%.4f) = %q, want %q", c.luma, got, c.want)
		}
	}
}

// TestResolveModeBothBranches drives the whole smart resolution off real images:
// a near-black wallpaper resolves dark, a near-white one light, while an explicit
// light/dark knob passes through regardless of the wallpaper.
func TestResolveModeBothBranches(t *testing.T) {
	home := t.TempDir()
	darkPNG := filepath.Join(home, "dark.png")
	lightPNG := filepath.Join(home, "light.png")
	writeSolidPNG(t, darkPNG, 12)
	writeSolidPNG(t, lightPNG, 240)

	if l := meanLuma(solidRGBA(4, 4, 0, 0, 0)); l > 0.01 {
		t.Errorf("meanLuma(black) = %.4f, want ~0", l)
	}
	if l := meanLuma(solidRGBA(4, 4, 255, 255, 255)); l < 0.99 {
		t.Errorf("meanLuma(white) = %.4f, want ~1", l)
	}

	cases := []struct {
		name, mode, img, want string
	}{
		{"smart + dark wallpaper -> dark", "smart", darkPNG, "dark"},
		{"smart + light wallpaper -> light", "smart", lightPNG, "light"},
		{"explicit dark ignores a light wallpaper", "dark", lightPNG, "dark"},
		{"explicit light ignores a dark wallpaper", "light", darkPNG, "light"},
	}
	for _, c := range cases {
		if got := resolveMode(c.mode, c.img); got != c.want {
			t.Errorf("%s: resolveMode(%q, ...) = %q, want %q", c.name, c.mode, got, c.want)
		}
	}
}

// A clip's smart light/dark comes from its whole run, not from the frame the
// palette was sampled at: a dark wallpaper with one bright second in it used to
// turn the desktop white. A clip that cannot be read lands on dark.
func TestResolveModeVideoReadsTheClip(t *testing.T) {
	bin := t.TempDir()
	// fake ffmpeg: raw gray bytes per clip, and a failure for the third.
	body := `case "$*" in
  *white.mp4*) head -c 128 /dev/zero | tr '\0' '\377' ;;
  *black.mp4*) head -c 128 /dev/zero ;;
  *) exit 1 ;;
esac`
	if err := os.WriteFile(filepath.Join(bin, "ffmpeg"), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	cases := []struct{ name, clip, want string }{
		{"bright clip", "/clips/white.mp4", "light"},
		{"dark clip", "/clips/black.mp4", "dark"},
		{"unreadable clip", "/clips/broken.mp4", "dark"},
	}
	for _, c := range cases {
		if got := resolveMode("smart", c.clip); got != c.want {
			t.Errorf("%s: resolveMode(smart, %q) = %q, want %q", c.name, c.clip, got, c.want)
		}
	}
	if got := resolveMode("dark", "/clips/white.mp4"); got != "dark" {
		t.Errorf("an explicit dark knob must ignore a bright clip, got %q", got)
	}
}

// TestMatugenBase16Color8IsLegibleForeground defends the dark-on-dark fix:
// base16 color8 (the "bright black" muted ink Tokens reads as inkMuted/inkFaint)
// must map to `outline`, a legible mid-lightness foreground role, NOT to
// `surface_variant`, a dark background role that painted muted ink invisibly.
func TestMatugenBase16Color8IsLegibleForeground(t *testing.T) {
	pal := map[string]string{
		"surface": "#13140d", "on_surface": "#e4e3d7",
		"outline": "#909283", "surface_variant": "#46483c",
		"primary": "#bdce80", "secondary": "#c4caa9", "tertiary": "#a1d0c5",
		"error": "#ffb4ab",
	}
	b16 := matugenBase16(pal)
	if b16["color8"] != "#909283" {
		t.Errorf("color8 = %q, want outline #909283 (legible muted ink)", b16["color8"])
	}
	if b16["color8"] == "#46483c" {
		t.Errorf("color8 must not map to surface_variant #46483c (dark-on-dark defect)")
	}
	// Sanity on the surrounding ramp: surface is the dark ground, foreground the
	// light ink.
	if b16["color0"] != "#13140d" || b16["background"] != "#13140d" {
		t.Errorf("color0/background = %q/%q, want surface #13140d", b16["color0"], b16["background"])
	}
	if b16["color7"] != "#e4e3d7" || b16["foreground"] != "#e4e3d7" {
		t.Errorf("color7/foreground = %q/%q, want on_surface #e4e3d7", b16["color7"], b16["foreground"])
	}
}

// TestTemplateGroup pins the block -> roster-key mapping so one toggle governs
// every block that themes an app (both GTK stylesheets, both discord clients,
// the two Qt toolkits, the Hyprland border).
func TestTemplateGroup(t *testing.T) {
	cases := map[string]string{
		"gtk3": "gtk", "gtk4": "gtk",
		"vesktop": "discord", "equibop": "discord",
		"qt6ct": "qt", "kde": "qt", "hypr": "hyprland",
		"kitty": "kitty", "btop": "btop", "papirus": "papirus", "cava": "cava",
	}
	for block, want := range cases {
		if got := templateGroup(block); got != want {
			t.Errorf("templateGroup(%q) = %q, want %q", block, got, want)
		}
	}
}

// TestApplyKdeColors is the merge contract: the colour groups come from the
// render, and everything else in kdeglobals is the user's and survives. The
// fixture carries the two shapes a general INI reader gets wrong, the
// [Colors:Header][Inactive] group name and case-sensitive keys, because
// mangling either would silently drop a user's settings.
func TestApplyKdeColors(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	mustWrite := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mustWrite(kdeglobalsPath(), `[General]
ColorScheme=SomethingElse
font=Comic Sans,11

[Icons]
Theme=breeze-dark

[KDE]
widgetStyle=Darkly

[Colors:View]
BackgroundNormal=#ffffff

[Colors:Header][Inactive]
BackgroundNormal=#ffffff
`)
	mustWrite(matugenKdeColorsPath(), `[Colors:View]
BackgroundNormal=#101010
BackgroundAlternate=#181818

[Colors:Header][Inactive]
BackgroundNormal=#101010

[WM]
activeBackground=#202020
`)

	applyKdeColors()

	b, err := os.ReadFile(kdeglobalsPath())
	if err != nil {
		t.Fatal(err)
	}
	out := string(b)

	for _, want := range []string{
		"font=Comic Sans,11", // the user's font
		"Theme=breeze-dark",  // the user's icon theme
		"widgetStyle=Darkly", // the user's widget style
		"ColorScheme=Ryoku",  // the one General key the palette owns
		"BackgroundNormal=#101010",
		"BackgroundAlternate=#181818",
		"[Colors:Header][Inactive]",
		"activeBackground=#202020",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("kdeglobals missing %q; got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "#ffffff") {
		t.Errorf("stale colour survived the merge; got:\n%s", out)
	}
	if strings.Contains(out, "ColorScheme=SomethingElse") {
		t.Errorf("stale scheme name survived the merge; got:\n%s", out)
	}
}

// TestApplyKdeColorsWithoutKdeglobals covers the machine that has never run a
// KDE app: no kdeglobals yet, so the merge writes a colours-only one instead of
// skipping.
func TestApplyKdeColorsWithoutKdeglobals(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	path := matugenKdeColorsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[Colors:View]\nBackgroundNormal=#101010\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(kdeglobalsPath()), 0o755); err != nil {
		t.Fatal(err)
	}

	applyKdeColors()

	b, err := os.ReadFile(kdeglobalsPath())
	if err != nil {
		t.Fatalf("kdeglobals not created: %v", err)
	}
	if !strings.Contains(string(b), "BackgroundNormal=#101010") {
		t.Errorf("colours not written; got:\n%s", b)
	}
}

// TestApplyKdeColorsForeignSectionSurvives proves the merge only claims the
// colour groups: a section Ryoku knows nothing about, and the user's font,
// come through byte for byte while the palette lands.
func TestApplyKdeColorsForeignSectionSurvives(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	mustWrite := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mustWrite(kdeglobalsPath(), `[General]
font=Comic Sans,11

[SomeSection]
key=value

[Colors:View]
BackgroundNormal=#ffffff
`)
	mustWrite(matugenKdeColorsPath(), `[Colors:View]
BackgroundNormal=#101010
`)

	applyKdeColors()

	b, err := os.ReadFile(kdeglobalsPath())
	if err != nil {
		t.Fatal(err)
	}
	out := string(b)

	for _, want := range []string{
		"[SomeSection]",
		"key=value",
		"font=Comic Sans,11",
		"BackgroundNormal=#101010",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("kdeglobals missing %q; got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "#ffffff") {
		t.Errorf("stale colour survived the merge; got:\n%s", out)
	}
}

// TestFilterMatugenConfig is the roster gate: the [config] preamble always
// renders, an enabled block renders with its comments and post_hook, and a
// disabled block (or a whole group like gtk) is dropped entirely, so the render
// set is exactly the enabled roster.
func TestFilterMatugenConfig(t *testing.T) {
	const core = `[config]
# preamble kept always

[templates.kitty]
input_path = "~/a"
output_path = "~/b"

[templates.hypr]
input_path = "~/c"
output_path = "~/d"

[templates.qt6ct]
input_path = "~/e"
output_path = "~/f"
post_hook = "recolor-folders"
`
	roster := map[string]bool{"kitty": true, "hyprland": false, "qt": true}
	out := filterMatugenConfig(core, func(g string) bool { return roster[g] })

	if !strings.Contains(out, "[config]") || !strings.Contains(out, "preamble kept always") {
		t.Errorf("[config] preamble dropped:\n%s", out)
	}
	if !strings.Contains(out, "[templates.kitty]") {
		t.Errorf("kitty enabled but dropped:\n%s", out)
	}
	if strings.Contains(out, "[templates.hypr]") {
		t.Errorf("hyprland disabled but rendered:\n%s", out)
	}
	if !strings.Contains(out, "[templates.qt6ct]") || !strings.Contains(out, `post_hook = "recolor-folders"`) {
		t.Errorf("qt enabled but block or its post_hook dropped:\n%s", out)
	}

	const apps = `[config]
[templates.gtk3]
output_path = "~/g3"
[templates.gtk4]
output_path = "~/g4"
[templates.vesktop]
output_path = "~/v"
[templates.kde]
output_path = "~/kde"
`
	on := map[string]bool{"gtk": false, "discord": true, "qt": true}
	got := filterMatugenConfig(apps, func(g string) bool { return on[g] })
	if strings.Contains(got, "[templates.gtk3]") || strings.Contains(got, "[templates.gtk4]") {
		t.Errorf("gtk disabled but a gtk block rendered:\n%s", got)
	}
	if !strings.Contains(got, "[templates.vesktop]") {
		t.Errorf("discord enabled but vesktop dropped:\n%s", got)
	}
	if !strings.Contains(got, "[templates.kde]") {
		t.Errorf("qt enabled but the kde block dropped:\n%s", got)
	}
}

// TestMatugenReload proves the daemon owns the three GTK-facing gsettings keys
// on a retint, through the process shim so the test never touches the desktop.
// For (dark, adw, accent on): color-scheme tracks the mode, accent-color tracks
// the palette's primary, gtk-theme lands on the dark adw-gtk3 variant flipped
// through a real placeholder (never the empty string), and kitty gets SIGUSR1.
// The light run re-resolves the light variant.
func TestMatugenReload(t *testing.T) {
	got := gtkReloadEnv(t,
		`{"gtkTheme":"adw","gnomeAccent":true}`,
		`{"primary":"#ffb59b"}`)
	has := func(want ...string) bool {
		for _, c := range *got {
			if slices.Equal(c, want) {
				return true
			}
		}
		return false
	}

	matugenReload("dark")
	if !has("gsettings", "set", "org.gnome.desktop.interface", "color-scheme", "prefer-dark") {
		t.Errorf("dark: color-scheme prefer-dark not set; got %v", *got)
	}
	if !has("gsettings", "set", "org.gnome.desktop.interface", "accent-color", "orange") {
		t.Errorf("dark: accent-color not tracked to the salmon primary's orange; got %v", *got)
	}
	if !has("gsettings", "set", "org.gnome.desktop.interface", "gtk-theme", "Adwaita") ||
		!has("gsettings", "set", "org.gnome.desktop.interface", "gtk-theme", "adw-gtk3-dark") {
		t.Errorf("dark: gtk-theme not flipped through a placeholder to adw-gtk3-dark; got %v", *got)
	}
	if has("gsettings", "set", "org.gnome.desktop.interface", "gtk-theme", "") {
		t.Errorf("dark: nudge passed an empty gtk-theme; got %v", *got)
	}
	if !has("pkill", "-USR1", "-x", "kitty") {
		t.Errorf("dark: kitty SIGUSR1 not sent; got %v", *got)
	}

	*got = nil
	matugenReload("light")
	if !has("gsettings", "set", "org.gnome.desktop.interface", "color-scheme", "prefer-light") {
		t.Errorf("light: color-scheme prefer-light not set; got %v", *got)
	}
	if !has("gsettings", "set", "org.gnome.desktop.interface", "gtk-theme", "adw-gtk3") {
		t.Errorf("light: gtk-theme not re-resolved to the light adw-gtk3 variant; got %v", *got)
	}
}

// TestMatugenReloadSystemNoGtkTheme proves gtkTheme "system" hands gtk-theme back
// to the user: the reload writes no gtk-theme at all (not even a nudge flip), and
// with gnomeAccent off it writes no accent-color either, while color-scheme and
// kitty still fire.
func TestMatugenReloadSystemNoGtkTheme(t *testing.T) {
	got := gtkReloadEnv(t,
		`{"gtkTheme":"system","gnomeAccent":false}`,
		`{"primary":"#3584e4"}`)
	matugenReload("light")
	for _, c := range *got {
		if len(c) >= 5 && c[0] == "gsettings" && c[4] == "gtk-theme" {
			t.Errorf("system: wrote gtk-theme %v", c)
		}
		if len(c) >= 5 && c[0] == "gsettings" && c[4] == "accent-color" {
			t.Errorf("accent off: wrote accent-color %v", c)
		}
	}
	has := func(want ...string) bool {
		for _, c := range *got {
			if slices.Equal(c, want) {
				return true
			}
		}
		return false
	}
	if !has("gsettings", "set", "org.gnome.desktop.interface", "color-scheme", "prefer-light") {
		t.Errorf("system: color-scheme still owned by the daemon; got %v", *got)
	}
	if !has("pkill", "-USR1", "-x", "kitty") {
		t.Errorf("system: kitty SIGUSR1 not sent; got %v", *got)
	}
}

// TestResolveGtkTheme pins contract C3's gtkTheme -> gsettings name mapping by
// mode: adw (the default, and what an absent or unknown value reads as) resolves
// the adw-gtk3 variants, adwaita the stock Adwaita variants, and system resolves
// "" so the daemon never writes gtk-theme.
func TestResolveGtkTheme(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	ryoku := filepath.Join(home, ".config", "ryoku")
	if err := os.MkdirAll(ryoku, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(ryoku, "theme.json")
	cases := []struct {
		theme       string // "" means write no theme.json at all
		dark, light string
	}{
		{`{"gtkTheme":"adw"}`, "adw-gtk3-dark", "adw-gtk3"},
		{`{"gtkTheme":"adwaita"}`, "Adwaita-dark", "Adwaita"},
		{`{"gtkTheme":"system"}`, "", ""},
		{`{"gtkTheme":"nonsense"}`, "adw-gtk3-dark", "adw-gtk3"},
		{`{}`, "adw-gtk3-dark", "adw-gtk3"},
		{"", "adw-gtk3-dark", "adw-gtk3"},
	}
	for _, c := range cases {
		if c.theme == "" {
			_ = os.Remove(path)
		} else {
			writeFile(t, path, c.theme)
		}
		if got := resolveGtkTheme("dark"); got != c.dark {
			t.Errorf("%s dark: resolveGtkTheme = %q, want %q", c.theme, got, c.dark)
		}
		if got := resolveGtkTheme("light"); got != c.light {
			t.Errorf("%s light: resolveGtkTheme = %q, want %q", c.theme, got, c.light)
		}
	}
}

// TestNearestGnomeAccent proves the primary -> named-accent match is by hue with
// a neutral fallback (contract C5): a warm salmon reads as orange (not the dark
// gold "yellow" a raw a/b distance would pick), clear blue and purple hit their
// own names, a near-grey has no reliable hue and lands on slate (GNOME's neutral
// accent), and every named accent maps to itself. A non-hex value returns ok
// false so the caller skips the write.
func TestNearestGnomeAccent(t *testing.T) {
	probes := []struct{ hex, want string }{
		{"#ffb59b", "orange"},
		{"#3584e4", "blue"},
		{"#9141ac", "purple"},
		{"#8a8f98", "slate"},
	}
	for _, p := range probes {
		got, ok := nearestGnomeAccent(p.hex)
		if !ok || got != p.want {
			t.Errorf("nearestGnomeAccent(%s) = %q,%v; want %q,true", p.hex, got, ok, p.want)
		}
	}
	for _, acc := range gnomeNamedAccents {
		if got, ok := nearestGnomeAccent(acc[1]); !ok || got != acc[0] {
			t.Errorf("accent %s (%s) did not map to itself: got %q,%v", acc[0], acc[1], got, ok)
		}
	}
	if _, ok := nearestGnomeAccent("not-a-colour"); ok {
		t.Errorf("non-hex primary should return ok=false")
	}
}

// TestApplyFont: the system font pushes to GTK live via gsettings font-name and
// falls back to the shipped face when empty; fontSig reads the frame's key.
func TestApplyFont(t *testing.T) {
	var got [][]string
	orig := runCommand
	t.Cleanup(func() { runCommand = orig })
	runCommand = func(name string, args ...string) error {
		got = append(got, append([]string{name}, args...))
		return nil
	}
	has := func(want ...string) bool {
		for _, c := range got {
			if slices.Equal(c, want) {
				return true
			}
		}
		return false
	}

	applyFont([]byte(`{"fontFamily":"Maple Mono NF","fontSize":13}`))
	if !has("gsettings", "set", "org.gnome.desktop.interface", "font-name", "Maple Mono NF 13") {
		t.Errorf("font-name not set to Maple Mono NF 13; got %v", got)
	}
	if !has("gsettings", "set", "org.gnome.desktop.interface", "monospace-font-name", "Maple Mono NF 13") {
		t.Errorf("the one system font must also drive monospace-font-name; got %v", got)
	}

	got = nil
	applyFont([]byte(`{}`))
	if !has("gsettings", "set", "org.gnome.desktop.interface", "font-name", "Space Grotesk 11") {
		t.Errorf("empty font did not fall back to Space Grotesk 11; got %v", got)
	}
	if !has("gsettings", "set", "org.gnome.desktop.interface", "monospace-font-name", "SpaceMono Nerd Font 11") {
		t.Errorf("empty mono did not fall back to SpaceMono Nerd Font 11; got %v", got)
	}

	if fontSig([]byte(`{"fontFamily":"Fira Sans","fontSize":12}`)) != "Fira Sans\x1f12" {
		t.Errorf("fontSig did not compose family/size")
	}
	if fontSig([]byte(`{}`)) != "\x1f0" {
		t.Errorf("fontSig should be the empty composite when keys are absent")
	}
}

// TestMatugenFollows is the trigger rule: the dynamic pipeline runs only when
// Match wallpaper is on and no fixed named theme is selected. Match off, or a
// static theme active, leaves it idle so it never fights the theme daemon.
func TestMatugenFollows(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	ryoku := filepath.Join(home, ".config", "ryoku")
	if err := os.MkdirAll(ryoku, 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		follow bool
		theme  string
		want   bool
	}{
		{"match on + wallpaper theme -> run", true, "Wallpaper", true},
		{"match on + default theme -> run", true, "Default", true},
		{"match on + no theme key -> run", true, "", true},
		{"match off -> idle", false, "Wallpaper", false},
		{"match on + static named theme -> idle", true, "Nord Dark", false},
	}
	for _, c := range cases {
		if err := os.WriteFile(filepath.Join(ryoku, "theme.json"), []byte(fmt.Sprintf(`{"followWallpaper":%v}`, c.follow)), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ryoku, "shell.json"), []byte(fmt.Sprintf(`{"theme":{"theme":%q}}`, c.theme)), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := matugenFollows(); got != c.want {
			t.Errorf("%s: matugenFollows() = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestMatugenApplyInvokesBinary drives matugenApply against a PATH-shimmed
// matugen that records its argv and emits a canned palette. It proves the knobs
// come from matugen.json, that smart mode resolves the -m value from the
// wallpaper's luminance (both branches), that the mode selects the colour
// bucket, and that the shell palette lands with the base16 keys plus the
// camelCase Material 3 roles Theme.qml reads. gsettings and pkill are shimmed so
// the test never touches the desktop.
func TestMatugenApplyInvokesBinary(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	bin := t.TempDir()
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	logFile := filepath.Join(home, "matugen.argv")
	jsonDark := filepath.Join(home, "dark.json")
	jsonLight := filepath.Join(home, "light.json")
	writeFile(t, jsonDark, fakeMatugenJSON("#0a0a0a", "#0b0b0b", "#0c0c0c"))
	writeFile(t, jsonLight, fakeMatugenJSON("#f0f0f0", "#f1f1f1", "#f2f2f2"))

	// The shim records argv, then for `image` emits the palette matching the
	// requested mode (-m dark|light picks the bucket), else no-ops (render pass).
	shim := `printf '%s\n' "$*" >> "` + logFile + `"
mode=dark
prev=
for a in "$@"; do case "$prev" in -m) mode="$a";; esac; prev="$a"; done
case "$1" in
  image) if [ "$mode" = light ]; then cat "` + jsonLight + `"; else cat "` + jsonDark + `"; fi ;;
esac`
	writeShim(t, filepath.Join(bin, "matugen"), shim)
	writeShim(t, filepath.Join(bin, "gsettings"), ":")
	writeShim(t, filepath.Join(bin, "pkill"), ":")

	// Deploy a minimal matugen config so the render pass has configs to run.
	mgDir := filepath.Join(home, ".config", "matugen")
	if err := os.MkdirAll(mgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(mgDir, "config.toml"), "[config]\n")
	writeFile(t, filepath.Join(mgDir, "apps.toml"), "[config]\n")

	ryoku := filepath.Join(home, ".config", "ryoku")
	if err := os.MkdirAll(ryoku, 0o755); err != nil {
		t.Fatal(err)
	}
	darkWall := filepath.Join(home, "dark-wall.png")
	lightWall := filepath.Join(home, "light-wall.png")
	writeSolidPNG(t, darkWall, 12)
	writeSolidPNG(t, lightWall, 240)
	colorsPath := filepath.Join(home, ".cache", "ryoku", "colors.json")

	combos := []struct {
		name, mode, prefer, img string
		contrast                float64
		wantArgs                string
		wantSurface             string // colors.json surface for the resolved bucket
	}{
		{
			"smart + dark wallpaper resolves -m dark", "smart", "saturation", darkWall, 0,
			"image " + darkWall + " -t scheme-tonal-spot -m dark --contrast 0.00 --lightness-dark 0.00 --lightness-light 0.00 --source-color-index 0 --prefer saturation --json hex --dry-run",
			"#0a0a0a",
		},
		{
			"smart + light wallpaper resolves -m light", "smart", "saturation", lightWall, 0,
			"image " + lightWall + " -t scheme-tonal-spot -m light --contrast 0.00 --lightness-dark 0.00 --lightness-light 0.00 --source-color-index 0 --prefer saturation --json hex --dry-run",
			"#f2f2f2",
		},
		{
			"explicit light / value / +0.5", "light", "value", darkWall, 0.5,
			"image " + darkWall + " -t scheme-tonal-spot -m light --contrast 0.50 --lightness-dark 0.00 --lightness-light 0.00 --source-color-index 0 --prefer value --json hex --dry-run",
			"#f2f2f2",
		},
	}
	for _, c := range combos {
		_ = os.Remove(logFile)
		_ = os.Remove(colorsPath)
		writeFile(t, filepath.Join(ryoku, "matugen.json"),
			fmt.Sprintf(`{"engine":"matugen","mode":%q,"prefer":%q,"contrast":%v,"themeRyokuApps":true}`,
				c.mode, c.prefer, c.contrast))

		if err := (&daemon{}).matugenApply(c.img); err != nil {
			t.Fatalf("%s: matugenApply: %v", c.name, err)
		}

		logged, err := os.ReadFile(logFile)
		if err != nil {
			t.Fatalf("%s: read argv log: %v", c.name, err)
		}
		if !strings.Contains(string(logged), c.wantArgs) {
			t.Errorf("%s: argv log missing expected image call\n want line: %s\n got:\n%s", c.name, c.wantArgs, logged)
		}

		var pal map[string]string
		b, err := os.ReadFile(colorsPath)
		if err != nil {
			t.Fatalf("%s: colors.json not written: %v", c.name, err)
		}
		if err := json.Unmarshal(b, &pal); err != nil {
			t.Fatalf("%s: colors.json parse: %v", c.name, err)
		}
		for _, k := range []string{"background", "foreground", "color0", "color4", "color8"} {
			if pal[k] == "" {
				t.Errorf("%s: colors.json missing base16 key %q", c.name, k)
			}
		}
		for _, k := range []string{"surface", "onSurface", "primary", "onPrimary", "surfaceContainerHigh", "outline", "shadow", "scrim"} {
			if pal[k] == "" {
				t.Errorf("%s: colors.json missing Material 3 role %q", c.name, k)
			}
		}
		if pal["surface"] != c.wantSurface {
			t.Errorf("%s: surface = %q, want %q (mode bucket not selected)", c.name, pal["surface"], c.wantSurface)
		}
	}
}

// TestMatugenColorsJSONAlignsWithThemeQml pins the shell palette key set to the
// 30 Material colour roles (plus shadow and scrim) Theme.qml resolves, so the
// produced palette always lines up with what the pill reads.
func TestMatugenColorsJSONAlignsWithThemeQml(t *testing.T) {
	pal := map[string]string{}
	for _, kv := range matugenRoleKeys {
		pal[kv[0]] = "#123456"
	}
	out := matugenColorsJSON(pal)
	want := []string{
		"surface", "surfaceVariant", "surfaceContainerLowest", "surfaceContainerLow",
		"surfaceContainer", "surfaceContainerHigh", "surfaceContainerHighest",
		"inverseSurface", "inverseOnSurface", "surfaceTint", "primary", "primaryContainer",
		"secondary", "secondaryContainer", "tertiary", "tertiaryContainer", "error",
		"errorContainer", "outline", "outlineVariant", "onSurface", "onSurfaceVariant",
		"onPrimary", "onPrimaryContainer", "onSecondary", "onSecondaryContainer",
		"onTertiary", "onTertiaryContainer", "onError", "onErrorContainer", "shadow", "scrim",
	}
	for _, k := range want {
		if out[k] == "" {
			t.Errorf("colors.json missing role %q Theme.qml reads", k)
		}
	}
}

// TestMatugenIsolatedDrive drives the real matugenApply in an isolated HOME with
// the shipped templates deployed and a recording-wrapper shim in front of the
// real matugen. It proves the argv matches the knob mapping for smart mode, the
// shell palette lands where Theme.qml watches with the Material 3 roles, the GTK
// stylesheets are written, and -- the roster gate -- an enabled template renders
// while a disabled one does not. Skipped where matugen is absent so the hermetic
// suite stays green everywhere.
func TestMatugenIsolatedDrive(t *testing.T) {
	const realMatugen = "/usr/bin/matugen"
	if _, err := os.Stat(realMatugen); err != nil {
		t.Skip("real matugen not installed; skipping end-to-end drive")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	if err := os.MkdirAll(filepath.Join(home, ".config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(filepath.Join(home, ".config", "matugen"), os.DirFS("../matugen")); err != nil {
		t.Fatalf("deploy templates: %v", err)
	}

	bin := t.TempDir()
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	logFile := filepath.Join(home, "matugen.argv")
	writeShim(t, filepath.Join(bin, "matugen"), `printf '%s\n' "$*" >> "`+logFile+`"; exec `+realMatugen+` "$@"`)
	writeShim(t, filepath.Join(bin, "gsettings"), ":")
	writeShim(t, filepath.Join(bin, "pkill"), ":")
	writeShim(t, filepath.Join(bin, "ryoku-cmd-folders"), ":")
	writeShim(t, filepath.Join(bin, "papirus-folders"), ":")

	img := filepath.Join(home, "wall.png")
	writePNG(t, img) // a dark-ish gradient; smart resolves dark
	ryoku := filepath.Join(home, ".config", "ryoku")
	if err := os.MkdirAll(ryoku, 0o755); err != nil {
		t.Fatal(err)
	}
	// Roster: kitty + gtk on, btop off. Smart mode + the shipped knobs.
	writeFile(t, filepath.Join(ryoku, "matugen.json"),
		`{"engine":"matugen","schemeType":"scheme-tonal-spot","mode":"smart","prefer":"saturation","contrast":0.2,`+
			`"themeRyokuApps":true,"templates":{"kitty":true,"hyprland":true,"gtk":true,"qt":true,"btop":false}}`)

	colorsPath := filepath.Join(home, ".cache", "ryoku", "colors.json")
	kittyOut := filepath.Join(home, ".config", "kitty", "current-theme.conf")
	btopOut := filepath.Join(home, ".config", "btop", "themes", "ryoku.theme")
	gtk4 := filepath.Join(home, ".config", "gtk-4.0", "gtk.css")
	gtk3 := filepath.Join(home, ".config", "gtk-3.0", "gtk.css")
	themeRoles := make([]string, len(matugenRoleKeys))
	for i, kv := range matugenRoleKeys {
		themeRoles[i] = kv[1]
	}

	if err := (&daemon{}).matugenApply(img); err != nil {
		t.Fatalf("matugenApply: %v", err)
	}

	argv, _ := os.ReadFile(logFile)
	t.Logf("matugen argv seen by shim:\n%s", strings.TrimSpace(string(argv)))
	wantArgs := "image " + img + " -t scheme-tonal-spot -m dark --contrast 0.20 --lightness-dark 0.00 --lightness-light 0.00 --source-color-index 0 --prefer saturation --json hex --dry-run"
	if !strings.Contains(string(argv), wantArgs) {
		t.Errorf("argv missing expected image call\n want: %s", wantArgs)
	}

	b, err := os.ReadFile(colorsPath)
	if err != nil {
		t.Fatalf("colors.json not written where Theme.qml watches (%s): %v", colorsPath, err)
	}
	var pal map[string]string
	if err := json.Unmarshal(b, &pal); err != nil {
		t.Fatalf("colors.json parse: %v", err)
	}
	missing := 0
	for _, k := range themeRoles {
		if pal[k] == "" {
			missing++
			t.Errorf("colors.json missing role %q", k)
		}
	}
	t.Logf("colors.json at %s: %d keys, %d/%d Theme.qml roles present; surface=%s primary=%s onSurface=%s color8(outline)=%s",
		colorsPath, len(pal), len(themeRoles)-missing, len(themeRoles), pal["surface"], pal["primary"], pal["onSurface"], pal["color8"])

	// Render set = enabled roster: kitty and GTK enabled -> written; btop
	// disabled -> not written.
	for _, g := range []string{gtk4, gtk3, kittyOut} {
		if fi, err := os.Stat(g); err != nil {
			t.Errorf("enabled template output %s not written: %v", g, err)
		} else {
			t.Logf("wrote %s (%d bytes)", g, fi.Size())
		}
	}
	if _, err := os.Stat(btopOut); err == nil {
		t.Errorf("btop disabled in roster but %s was rendered", btopOut)
	} else {
		t.Logf("btop disabled -> %s correctly absent", btopOut)
	}
}

// --- helpers ---------------------------------------------------------------

func writeShim(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// gtkReloadEnv sets an isolated HOME carrying a theme.json (the gtkTheme /
// gnomeAccent knobs) and a colors.json (the palette's primary), then shims
// runCommand to record every gsettings / pkill invocation so the GTK-facing
// reload is observable without touching the desktop. An empty json argument
// writes no file, exercising the absent-file defaults.
func gtkReloadEnv(t *testing.T, themeJSON, colorsJSON string) *[][]string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	cfg := filepath.Join(home, ".config", "ryoku")
	cache := filepath.Join(home, ".cache", "ryoku")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	if themeJSON != "" {
		writeFile(t, filepath.Join(cfg, "theme.json"), themeJSON)
	}
	if colorsJSON != "" {
		writeFile(t, filepath.Join(cache, "colors.json"), colorsJSON)
	}
	var got [][]string
	origRun, origOut := runCommand, runCommandOutput
	t.Cleanup(func() { runCommand, runCommandOutput = origRun, origOut })
	runCommand = func(name string, args ...string) error {
		got = append(got, append([]string{name}, args...))
		return nil
	}
	runCommandOutput = func(name string, args ...string) ([]byte, error) {
		return []byte("'Adwaita-dark'\n"), nil
	}
	return &got
}

// fakeMatugenJSON renders a matugen --json hex document covering every role the
// pipeline reads, each with a distinct dark/default/light colour so mode
// selection is observable.
func fakeMatugenJSON(dark, def, light string) string {
	roles := []string{
		"surface", "surface_variant", "surface_container_lowest", "surface_container_low",
		"surface_container", "surface_container_high", "surface_container_highest",
		"inverse_surface", "inverse_on_surface", "surface_tint", "primary", "primary_container",
		"secondary", "secondary_container", "tertiary", "tertiary_container", "error",
		"error_container", "outline", "outline_variant", "on_surface", "on_surface_variant",
		"on_primary", "on_primary_container", "on_secondary", "on_secondary_container",
		"on_tertiary", "on_tertiary_container", "on_error", "on_error_container", "shadow",
		"scrim", "background", "on_background", "inverse_primary",
	}
	colors := map[string]any{}
	for _, r := range roles {
		colors[r] = map[string]any{
			"dark":    map[string]any{"color": dark},
			"default": map[string]any{"color": def},
			"light":   map[string]any{"color": light},
		}
	}
	b, _ := json.Marshal(map[string]any{"colors": colors})
	return string(b)
}

// solidRGBA builds a w*h image of one colour, for the luminance helpers.
func solidRGBA(w, h int, r, g, b uint8) *image.RGBA {
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			im.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	return im
}

// writeSolidPNG writes a solid-gray PNG so smart mode resolves deterministically
// (a low gray -> dark, a high gray -> light).
func writeSolidPNG(t *testing.T, path string, gray uint8) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, solidRGBA(16, 16, gray, gray, gray)); err != nil {
		t.Fatal(err)
	}
}

// writePNG writes a small multi-colour gradient PNG so matugen has several source
// colours to choose between (it needs --prefer to pick one non-interactively).
func writePNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			img.Set(x, y, color.RGBA{R: uint8(x * 4), G: uint8(y * 4), B: uint8((x + y) * 2), A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// TestSyncFollowWallpaper pins theme.theme as the single colour master and its
// shadow key: colours follow the wallpaper by default, so the plain base
// (Default or Wallpaper) turns theme.json's followWallpaper ON, while a named
// static theme -- or an explicit Light/Dark curated lock -- turns it off. Other
// keys in the file are somebody else's and must survive.
func TestSyncFollowWallpaper(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	ryoku := filepath.Join(home, ".config", "ryoku")
	if err := os.MkdirAll(ryoku, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(ryoku, "theme.json")
	writeFile(t, path, `{"followWallpaper":false,"themeApps":true}`)

	read := func() map[string]any {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}

	syncFollowWallpaper("Wallpaper")
	got := read()
	if got["followWallpaper"] != true {
		t.Errorf("live scheme: followWallpaper = %v, want true", got["followWallpaper"])
	}
	if got["themeApps"] != true {
		t.Errorf("themeApps was dropped: %v", got)
	}

	syncFollowWallpaper("Dracula")
	if got := read(); got["followWallpaper"] != false {
		t.Errorf("named theme: followWallpaper = %v, want false", got["followWallpaper"])
	}

	// Default is the plain base and now follows the wallpaper: with no static
	// theme and no Light/Dark lock, the shadow key stays on.
	syncFollowWallpaper("Default")
	if got := read(); got["followWallpaper"] != true {
		t.Errorf("Default: followWallpaper = %v, want true (follows by default)", got["followWallpaper"])
	}

	// An explicit Light/Dark curated lock still pins a fixed palette.
	writeFile(t, path, `{"scheme":"dark","themeApps":true}`)
	syncFollowWallpaper("Default")
	if got := read(); got["followWallpaper"] != false {
		t.Errorf("Default + dark lock: followWallpaper = %v, want false", got["followWallpaper"])
	}
}

// TestRosterDefaultsNewAppsOn: a group missing from a saved roster is one that
// shipped later, so it renders; only an explicit false keeps an app stock.
func TestRosterDefaultsNewAppsOn(t *testing.T) {
	toml := "[config]\n\n[templates.kitty]\ni = 1\n\n[templates.fish]\ni = 2\n\n[templates.btop]\ni = 3\n"
	roster := map[string]bool{"kitty": true, "btop": false} // saved before fish existed
	got := filterMatugenConfig(toml, func(g string) bool {
		if v, ok := roster[g]; ok {
			return v
		}
		return true
	})
	if !strings.Contains(got, "[templates.fish]") {
		t.Error("a group absent from the roster must render; fish was dropped")
	}
	if !strings.Contains(got, "[templates.kitty]") {
		t.Error("an enabled group must render")
	}
	if strings.Contains(got, "[templates.btop]") {
		t.Error("an explicitly disabled group must not render")
	}
}

// TestNeutralizeHex proves the achromatic fallback strips hue while holding
// lightness: the result is a true gray (all channels equal), black and white map
// to themselves, luminance ordering survives (a darker colour yields a darker
// gray), and a non-hex value passes through.
func TestNeutralizeHex(t *testing.T) {
	grayVal := func(hex string) int {
		out := neutralizeHex(hex)
		var r, g, b int
		if _, err := fmt.Sscanf(out, "#%02x%02x%02x", &r, &g, &b); err != nil {
			t.Fatalf("neutralizeHex(%q) = %q: %v", hex, out, err)
		}
		if r != g || g != b {
			t.Fatalf("neutralizeHex(%q) = %q, not gray", hex, out)
		}
		return r
	}
	if v := grayVal("#000000"); v != 0 {
		t.Errorf("neutralizeHex(black) gray = %d, want 0", v)
	}
	if v := grayVal("#ffffff"); v != 255 {
		t.Errorf("neutralizeHex(white) gray = %d, want 255", v)
	}
	// matugen's invented blue primary (#005ac1) must land below its own light
	// tone (#adc6ff): luminance is preserved, so the darker colour stays darker.
	if dark, light := grayVal("#005ac1"), grayVal("#adc6ff"); dark >= light {
		t.Errorf("neutralizeHex ordering: #005ac1 -> %d not < #adc6ff -> %d", dark, light)
	}
	if got := neutralizeHex("nope"); got != "nope" {
		t.Errorf("neutralizeHex passthrough = %q, want unchanged", got)
	}
}

// TestMeanChromaSeparatesGrayFromColour is the achromatic gate itself: a solid
// gray reads well below the threshold, a saturated colour well above it, so only
// a colourless wallpaper trips neutralization.
func TestMeanChromaSeparatesGrayFromColour(t *testing.T) {
	if c := meanChroma(solidRGBA(16, 16, 128, 128, 128)); c >= achromaticChroma {
		t.Errorf("meanChroma(gray) = %.3f, want < %.3f", c, achromaticChroma)
	}
	if c := meanChroma(solidRGBA(16, 16, 200, 40, 40)); c <= achromaticChroma {
		t.Errorf("meanChroma(red) = %.3f, want > %.3f", c, achromaticChroma)
	}
}

// TestMatugenApplyNeutralizesAchromaticWallpaper is the whole fix end to end: an
// achromatic wallpaper drives matugen's default-blue fallback, so the daemon
// strips the invented hue before authoring the palette. A grayscale wall yields a
// gray colors.json and gray tones.json; a saturated wall keeps the generated
// colour, proving the gate is scoped to colourless pictures.
func TestMatugenApplyNeutralizesAchromaticWallpaper(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	bin := t.TempDir()
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	// The shim always emits matugen's blue fallback -- both the roles and the
	// tonal ramps -- exactly as it does for a picture it cannot seed from.
	blue := filepath.Join(home, "blue.json")
	writeFile(t, blue, blueMatugenJSON())
	writeShim(t, filepath.Join(bin, "matugen"), `case "$1" in image) cat "`+blue+`";; esac`)
	writeShim(t, filepath.Join(bin, "gsettings"), ":")
	writeShim(t, filepath.Join(bin, "pkill"), ":")

	mgDir := filepath.Join(home, ".config", "matugen")
	if err := os.MkdirAll(mgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(mgDir, "config.toml"), "[config]\n")
	writeFile(t, filepath.Join(mgDir, "apps.toml"), "[config]\n")

	ryoku := filepath.Join(home, ".config", "ryoku")
	if err := os.MkdirAll(ryoku, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(ryoku, "matugen.json"), `{"mode":"dark","prefer":"saturation","themeRyokuApps":true}`)

	colorsPath := filepath.Join(home, ".cache", "ryoku", "colors.json")
	tonesPath := filepath.Join(home, ".cache", "ryoku", "tones.json")

	gray := filepath.Join(home, "gray.png")
	writeSolidPNG(t, gray, 30) // achromatic + dark
	colour := filepath.Join(home, "colour.png")
	writeSolidColourPNG(t, colour, 200, 40, 40) // saturated red

	isGray := func(hex string) bool {
		var r, g, b int
		if _, err := fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b); err != nil {
			return false
		}
		return r == g && g == b
	}
	readPalette := func() map[string]string {
		b, err := os.ReadFile(colorsPath)
		if err != nil {
			t.Fatalf("colors.json: %v", err)
		}
		var m map[string]string
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("colors.json parse: %v", err)
		}
		return m
	}

	// Achromatic: colors.json and tones.json are stripped to gray.
	if err := (&daemon{}).matugenApply(gray); err != nil {
		t.Fatalf("matugenApply(gray): %v", err)
	}
	if p := readPalette()["primary"]; !isGray(p) {
		t.Errorf("achromatic wall: colors.json primary = %q, want gray", p)
	}
	var tones map[string]map[string]string
	tb, err := os.ReadFile(tonesPath)
	if err != nil {
		t.Fatalf("tones.json: %v", err)
	}
	if err := json.Unmarshal(tb, &tones); err != nil {
		t.Fatalf("tones.json parse: %v", err)
	}
	if got := tones["primary"]["80"]; !isGray(got) {
		t.Errorf("achromatic wall: tones primary/80 = %q, want gray", got)
	}

	// Chromatic: the generated colour survives untouched.
	if err := (&daemon{}).matugenApply(colour); err != nil {
		t.Fatalf("matugenApply(colour): %v", err)
	}
	if p := readPalette()["primary"]; isGray(p) {
		t.Errorf("chromatic wall: colors.json primary = %q, want the generated colour kept", p)
	}
}

// blueMatugenJSON is matugen's output when it seeds from its built-in blue
// fallback: every role carries the blue the wallpaper never had, and the tonal
// ramps do too, so the daemon writes both a colors.json and a tones.json to
// check.
func blueMatugenJSON() string {
	var doc map[string]any
	_ = json.Unmarshal([]byte(fakeMatugenJSON("#adc6ff", "#adc6ff", "#445e91")), &doc)
	doc["palettes"] = map[string]any{
		"primary":   map[string]any{"40": map[string]any{"color": "#005ac1"}, "80": map[string]any{"color": "#adc6ff"}},
		"secondary": map[string]any{"80": map[string]any{"color": "#bfc6dc"}},
	}
	b, _ := json.Marshal(doc)
	return string(b)
}

// writeSolidColourPNG writes a solid saturated PNG, a wallpaper the achromatic
// gate must leave alone.
func writeSolidColourPNG(t *testing.T, path string, r, g, b uint8) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, solidRGBA(16, 16, r, g, b)); err != nil {
		t.Fatal(err)
	}
}

// paletteBorderColors maps the palette roles the shell owns onto the border:
// color4 is the active border, background the inactive. The provider owns the
// colour literal format, so only that role mapping is asserted here.
func TestPaletteBorderColors(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	colorsPath := matugenColorsPath()
	if err := os.MkdirAll(filepath.Dir(colorsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := paletteBorderColors(); ok {
		t.Fatal("paletteBorderColors ok with no colors.json, want not ok")
	}
	if err := os.WriteFile(colorsPath, []byte(`{"color4":"#12ab34","background":"#010203"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	active, inactive, ok := paletteBorderColors()
	if !ok || active != "#12ab34" || inactive != "#010203" {
		t.Fatalf("paletteBorderColors() = (%q,%q,%v), want (#12ab34,#010203,true)", active, inactive, ok)
	}
}

// The "sun" mode follows the real day/night window, not the wallpaper: a dark
// wallpaper inside the window resolves light, outside it resolves dark, and a
// box with no observed window falls back to the wallpaper's own luminance (the
// smart rule), never a guess.
func TestResolveModeSunFollowsTheWindow(t *testing.T) {
	home := t.TempDir()
	darkPNG := filepath.Join(home, "dark.png")
	writeSolidPNG(t, darkPNG, 12)

	withWindow := func(offset, span time.Duration, fn func()) {
		t.Helper()
		prev := daySun
		now := time.Now()
		daySun = &sunState{sunrise: now.Add(offset), sunset: now.Add(offset + span), observed: true}
		defer func() { daySun = prev }()
		fn()
	}
	t.Run("inside the window is light on a dark wallpaper", func(t *testing.T) {
		withWindow(-time.Hour, 2*time.Hour, func() {
			if got := resolveMode("sun", darkPNG); got != "light" {
				t.Errorf("resolveMode(sun) inside the window = %q, want light", got)
			}
		})
	})
	t.Run("outside the window is dark on a light wallpaper", func(t *testing.T) {
		lightPNG := filepath.Join(home, "light.png")
		writeSolidPNG(t, lightPNG, 240)
		withWindow(time.Hour, 2*time.Hour, func() {
			if got := resolveMode("sun", lightPNG); got != "dark" {
				t.Errorf("resolveMode(sun) outside the window = %q, want dark", got)
			}
		})
	})
	t.Run("no window falls back to the wallpaper", func(t *testing.T) {
		prev := daySun
		daySun = &sunState{}
		defer func() { daySun = prev }()
		if got := resolveMode("sun", darkPNG); got != "dark" {
			t.Errorf("resolveMode(sun) without a window = %q, want the smart fallback dark", got)
		}
	})
}
