package main

import (
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	wm "ryoku-wm"
)

// matugen.go is the dynamic colour pipeline and the sole renderer of the
// wallpaper palette. When Match wallpaper is on and no fixed named theme is
// selected, a wallpaper change (or a scheme-knob patch) runs matugen against the
// current image with the configured scheme arguments, writes the shell palette
// the Quickshell singletons read, and fans that same palette into the terminal,
// window border, system monitor, Qt, and (when app theming is on) the GTK / GUI
// app suite. Match wallpaper off, or a fixed named theme active, leaves the
// pipeline idle so it never fights the static palette the theme daemon owns.
//
// One knob store, one renderer: ~/.config/ryoku/matugen.json holds mode,
// contrast, prefer, the per-app roster and the app-suite toggle. The
// Hub appearance page writes that file; the daemon reads it and watches it, so a
// knob save retints exactly as a wallpaper change does. There is no second knob
// source and no second renderer.
//
// The installed matugen needs two passes:
//
//  1. Scheme generation:
//     matugen image <img> -t scheme-tonal-spot -m <mode> --contrast <c>
//     --lightness-dark <ld> --lightness-light <ll> --source-color-index <i>
//     --prefer <pref> --json hex --dry-run
//     emits the Material 3 palette on stdout and touches nothing on disk.
//  2. Templating:
//     the parsed palette is carried into `matugen -c <cfg> json <carrier>`, which
//     renders the deployed templates (kitty, the Hyprland border, btop, Qt, and,
//     when app theming is on, the GTK / GUI-app suite) from that one palette.
//
// The shell's own colors.json (the base16 keys the legacy reader consumes plus
// the camelCase Material 3 roles Theme.qml and Tokens.qml resolve) is authored
// here from the same palette, matching the shipped convention where the control
// plane owns colors.json and matugen fans it out.

// matugenKnobs are the scheme controls, read from the one knob store
// (~/.config/ryoku/matugen.json). The token values are matugen's own CLI
// vocabulary, so the constructed argv passes them straight through.
type matugenKnobs struct {
	Mode             string          // "dark" | "light" | "smart"
	Contrast         float64         // -1.0 .. 1.0
	LightnessDark    float64         // affine lightness transform for dark schemes
	LightnessLight   float64         // affine lightness transform for light schemes
	Prefer           string          // source-colour pick, e.g. "saturation"
	SchemeType       string          // matugen -t variant; "" resolves to the default
	SourceColorIndex int             // 0..4, most dominant first
	ThemeRyokuApps   bool            // theme the GTK / GUI app suite (apps.toml)
	Templates        map[string]bool // per-app roster, keyed by group
}

// The value sets matugen accepts. An out-of-schema knob fails the run loudly
// rather than handing matugen a token it would reject.
var matugenPrefers = map[string]bool{
	"darkness": true, "lightness": true, "saturation": true,
	"less-saturation": true, "value": true, "closest-to-fallback": true,
}

// The default scheme Ryoku generates with. A variant picker (matugen.json
// schemeType) lets a user pick another; tonal-spot stays the default because the
// desaturating variants (monochrome, neutral) pull the colour out of a wallpaper.
// The Hub labels those as desaturating instead of hiding the choice.
const matugenSchemeType = "scheme-tonal-spot"

// The variants matugen 4.x accepts for -t. An out-of-set schemeType fails the
// run loudly (the same contract as prefer) rather than feeding matugen a token
// it would reject.
var matugenSchemes = map[string]bool{
	"scheme-tonal-spot": true, "scheme-vibrant": true, "scheme-expressive": true,
	"scheme-fidelity": true, "scheme-content": true, "scheme-fruit-salad": true,
	"scheme-rainbow": true, "scheme-neutral": true, "scheme-monochrome": true,
}

// defaultMatugenKnobs backs a missing or partial matugen.json so a run still
// produces a valid argv. They mirror the shipped matugen.json.
func defaultMatugenKnobs() matugenKnobs {
	return matugenKnobs{
		Mode:           "smart",
		Contrast:       0,
		Prefer:         "saturation",
		SchemeType:     matugenSchemeType,
		ThemeRyokuApps: true,
	}
}

// readMatugenKnobs reads the scheme controls from the one knob store
// (~/.config/ryoku/matugen.json), falling back to the shipped defaults for any
// that are absent so a partial file still produces a valid run.
func readMatugenKnobs() matugenKnobs {
	k := defaultMatugenKnobs()
	b, err := os.ReadFile(matugenKnobsPath())
	if err != nil {
		return k
	}
	var doc struct {
		Mode             *string         `json:"mode"`
		Contrast         *float64        `json:"contrast"`
		LightnessDark    *float64        `json:"lightnessDark"`
		LightnessLight   *float64        `json:"lightnessLight"`
		Prefer           *string         `json:"prefer"`
		SchemeType       *string         `json:"schemeType"`
		SourceColorIndex *int            `json:"sourceColorIndex"`
		ThemeRyokuApps   *bool           `json:"themeRyokuApps"`
		Templates        map[string]bool `json:"templates"`
	}
	if json.Unmarshal(b, &doc) != nil {
		return k
	}
	if doc.Mode != nil && *doc.Mode != "" {
		k.Mode = *doc.Mode
	}
	if doc.SchemeType != nil && *doc.SchemeType != "" {
		k.SchemeType = *doc.SchemeType
	}
	if doc.Contrast != nil {
		k.Contrast = *doc.Contrast
	}
	if doc.LightnessDark != nil {
		k.LightnessDark = *doc.LightnessDark
	}
	if doc.LightnessLight != nil {
		k.LightnessLight = *doc.LightnessLight
	}
	if doc.Prefer != nil && *doc.Prefer != "" {
		k.Prefer = *doc.Prefer
	}
	if doc.SourceColorIndex != nil {
		k.SourceColorIndex = *doc.SourceColorIndex
	}
	if doc.ThemeRyokuApps != nil {
		k.ThemeRyokuApps = *doc.ThemeRyokuApps
	}
	k.Templates = doc.Templates
	return k
}

// matugenArgs builds the phase-1 (scheme generation) argv for the knobs and the
// already-resolved light/dark mode, or an error if a token is out of schema so a
// bad knob fails loudly. --prefer is required: the installed matugen refuses to
// pick a source colour non-interactively without it.
func matugenArgs(img string, k matugenKnobs, mode string) ([]string, error) {
	if mode != "dark" && mode != "light" {
		return nil, fmt.Errorf("unresolved mode %q (want dark|light)", mode)
	}
	if !matugenPrefers[k.Prefer] {
		return nil, fmt.Errorf("unknown prefer %q", k.Prefer)
	}
	scheme := k.SchemeType
	if scheme == "" {
		scheme = matugenSchemeType
	}
	if !matugenSchemes[scheme] {
		return nil, fmt.Errorf("unknown schemeType %q", scheme)
	}
	c := k.Contrast
	if c < -1 {
		c = -1
	} else if c > 1 {
		c = 1
	}
	f := func(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }
	return []string{
		"image", img,
		"-t", scheme,
		"-m", mode,
		"--contrast", f(c),
		"--lightness-dark", f(k.LightnessDark),
		"--lightness-light", f(k.LightnessLight),
		"--source-color-index", strconv.Itoa(k.SourceColorIndex),
		"--prefer", k.Prefer,
		"--json", "hex",
		"--dry-run",
	}, nil
}

// smartMode maps a wallpaper's mean luma to a light/dark scheme: a dark image
// gets a dark scheme, a light image a light one, split at the mid-point. This is
// the "smart" mode's rule; the installed matugen's -m accepts only light|dark,
// so the choice is made here and passed through.
func smartMode(luma float64) string {
	if luma >= 0.5 {
		return "light"
	}
	return "dark"
}

// resolveMode turns the mode knob into a concrete light/dark for matugen: an
// explicit light/dark passes through; "sun" follows the real day/night window
// the weather poll publishes (sun.go), independent of what the wallpaper
// depicts; "smart" (or anything else) follows the wallpaper's luminance. For a
// clip that is the whole run, not the sampled frame: one bright second in a
// dark wallpaper used to turn the desktop white.
func resolveMode(mode, img string) string {
	switch mode {
	case "light", "dark":
		return mode
	case "sun":
		return sunMode(img)
	}
	var luma float64
	var ok bool
	if isVideo(img) {
		luma, ok = videoLuma(img)
	} else {
		luma, ok = matugenImageLuma(img)
	}
	if !ok {
		// Nothing decodable: default to dark, the shell's own signature.
		return "dark"
	}
	return smartMode(luma)
}

// sunMode resolves the "sun" mode: light between sunrise and sunset, dark
// outside. The times come from the weather poll (the same location-correct
// data the weather widget shows), so a "daytime" video that samples dark no
// longer forces a dark theme at noon. Until the first frame lands there is no
// window to follow, and the wallpaper's own luminance is the honest fallback:
// the box behaves like "smart" for the minutes it has no sun data.
func sunMode(img string) string {
	sunrise, sunset, ok := daySun.window()
	if !ok {
		return resolveMode("smart", img)
	}
	if isDaytime(time.Now(), sunrise, sunset) {
		return "light"
	}
	return "dark"
}

// videoLuma: a clip's mean luma over its first minute, sampled a frame a second
// at 32x18. ffmpeg's gray conversion carries the same BT.601 weights meanLuma
// uses, so the two agree on what "bright" means.
func videoLuma(video string) (float64, bool) {
	out, err := exec.Command("ffmpeg", "-v", "error", "-t", "60", "-i", video,
		"-vf", "fps=1,scale=32:18", "-f", "rawvideo", "-pix_fmt", "gray", "-").Output()
	if err != nil || len(out) == 0 {
		return 0, false
	}
	var sum int64
	for _, b := range out {
		sum += int64(b)
	}
	return float64(sum) / float64(len(out)) / 255, true
}

// matugenImageLuma returns the wallpaper's mean perceptual luma in 0..1, using
// the shell's luma weights (dither.frag: 0.299/0.587/0.114). It decodes with the
// standard library (png/jpeg/gif) and falls back to a one-pixel ffmpeg downscale
// for formats the stdlib cannot read (webp), so every wallpaper the desktop
// accepts resolves a smart light/dark choice.
func matugenImageLuma(img string) (float64, bool) {
	if f, err := os.Open(img); err == nil {
		m, _, derr := image.Decode(f)
		f.Close()
		if derr == nil {
			return meanLuma(m), true
		}
	}
	return ffmpegLuma(img)
}

// meanLuma averages luma over a bounded grid so a 4K wallpaper stays cheap while
// still reflecting the whole frame.
func meanLuma(m image.Image) float64 {
	b := m.Bounds()
	if b.Empty() {
		return 0
	}
	const grid = 64
	sx := max(1, b.Dx()/grid)
	sy := max(1, b.Dy()/grid)
	var sum, n float64
	for y := b.Min.Y; y < b.Max.Y; y += sy {
		for x := b.Min.X; x < b.Max.X; x += sx {
			r, g, bl, _ := m.At(x, y).RGBA()
			sum += 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(bl>>8)
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return (sum / n) / 255
}

// ffmpegLuma downscales the image to a single area-averaged pixel and reads its
// luma, covering formats the stdlib image decoders miss. ffmpeg is already a
// daemon dependency (it samples video wallpapers).
func ffmpegLuma(img string) (float64, bool) {
	out, err := runCommandOutput("ffmpeg", "-v", "error", "-i", img,
		"-vf", "scale=1:1:flags=area", "-frames:v", "1", "-pix_fmt", "rgb24",
		"-f", "rawvideo", "-")
	if err != nil || len(out) < 3 {
		return 0, false
	}
	r, g, b := float64(out[0]), float64(out[1]), float64(out[2])
	return (0.299*r + 0.587*g + 0.114*b) / 255, true
}

// achromaticChroma is the mean-saturation floor a wallpaper must clear to count
// as carrying colour. Below it a picture is grayscale or monochrome, matugen has
// no source hue to extract and falls back to its built-in blue, so the palette
// is neutralized to match the wallpaper instead. Measured: a grayscale wall sits
// near 0, the least saturated real wallpapers near 0.12.
const achromaticChroma = 0.05

// imageAchromatic reports whether the wallpaper carries essentially no colour (a
// grayscale or monochrome picture). Decoded with the standard library,
// ffmpeg-downscaled for the formats it misses (webp); an undecodable image reads
// as chromatic, so a palette is never neutralized on a guess.
func imageAchromatic(img string) bool {
	if f, err := os.Open(img); err == nil {
		m, _, derr := image.Decode(f)
		f.Close()
		if derr == nil {
			return meanChroma(m) < achromaticChroma
		}
	}
	if c, ok := ffmpegChroma(img); ok {
		return c < achromaticChroma
	}
	return false
}

// meanChroma averages HSV saturation over the same bounded grid meanLuma walks,
// skipping near-black cells where hue is meaningless and sensor / JPEG noise
// dominates, so the measure reflects the picture's real colourfulness.
func meanChroma(m image.Image) float64 {
	b := m.Bounds()
	if b.Empty() {
		return 0
	}
	const grid = 64
	sx := max(1, b.Dx()/grid)
	sy := max(1, b.Dy()/grid)
	var sum, n float64
	for y := b.Min.Y; y < b.Max.Y; y += sy {
		for x := b.Min.X; x < b.Max.X; x += sx {
			r, g, bl, _ := m.At(x, y).RGBA()
			mx := max(r>>8, g>>8, bl>>8)
			if mx < 24 { // near-black: no meaningful hue
				continue
			}
			mn := min(r>>8, g>>8, bl>>8)
			sum += float64(mx-mn) / float64(mx)
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / n
}

// ffmpegChroma downscales to a small grid and averages saturation from the raw
// pixels, for the formats the stdlib decoders miss. A one-pixel average (what
// ffmpegLuma reads) washes all colour out, so chroma needs a grid.
func ffmpegChroma(img string) (float64, bool) {
	const n = 32
	out, err := runCommandOutput("ffmpeg", "-v", "error", "-i", img,
		"-vf", fmt.Sprintf("scale=%d:%d:flags=area", n, n),
		"-frames:v", "1", "-pix_fmt", "rgb24", "-f", "rawvideo", "-")
	if err != nil || len(out) < n*n*3 {
		return 0, false
	}
	var sum, cnt float64
	for i := 0; i+2 < len(out); i += 3 {
		mx := max(out[i], out[i+1], out[i+2])
		if mx < 24 {
			continue
		}
		mn := min(out[i], out[i+1], out[i+2])
		sum += float64(mx-mn) / float64(mx)
		cnt++
	}
	if cnt == 0 {
		return 0, true
	}
	return sum / cnt, true
}

// syncFollowWallpaper makes theme.json's followWallpaper track the selected
// scheme. Without it the two disagree: the Appearance page and the sidebar write
// theme.theme, nothing writes followWallpaper, and the live path is gated on
// followWallpaper alone -- so picking the live scheme after anything had turned
// it off (a rice, a fixed palette) selected a scheme that never regenerated.
// theme.theme is the master; this is its shadow. Other keys in the file
// (themeApps) are preserved.
func syncFollowWallpaper(themeName string) {
	path := filepath.Join(ryokuConfigDir(), "theme.json")
	doc := map[string]any{}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &doc)
	}
	// Colours follow the wallpaper by default. Only a named static theme, or an
	// explicit Light / Dark curated lock, pins a fixed palette; the plain base
	// (Default or Wallpaper) always follows, so a fresh desktop -- and any box
	// that lands back on Default -- tracks the wallpaper instead of the shipped
	// brand palette. theme.theme is the master; this is its shadow.
	scheme, _ := doc["scheme"].(string)
	locked := scheme == "light" || scheme == "dark"
	follow := !staticName(themeName) && !locked
	if cur, ok := doc["followWallpaper"].(bool); ok && cur == follow {
		return
	}
	doc["followWallpaper"] = follow
	_ = writeJSONFile(path, doc)
}

// matchWallpaperOn reports whether the colour master follows the wallpaper. The
// key lives in theme.json (the single source the daemon, window borders and shell
// chrome all read), defaulting ON to match the shell's own Config default: a
// fresh box with no theme.json (or a file missing the key) follows the wallpaper.
func matchWallpaperOn() bool {
	b, err := os.ReadFile(filepath.Join(ryokuConfigDir(), "theme.json"))
	if err != nil {
		return true
	}
	s := struct {
		FollowWallpaper *bool `json:"followWallpaper"`
	}{}
	if json.Unmarshal(b, &s) != nil || s.FollowWallpaper == nil {
		return true
	}
	return *s.FollowWallpaper
}

// gtkThemeSetting reads theme.json's gtkTheme knob (contract C3): "adw" (the
// default), "adwaita", or "system". An absent, malformed, or unrecognised value
// reads as "adw" -- the adw-gtk3 base whose rules derive from the accent, so the
// palette actually lands, unlike stock Adwaita GTK3 which hardcodes its colours.
// theme.json is the one control-plane file the daemon reads directly.
func gtkThemeSetting() string {
	b, err := os.ReadFile(filepath.Join(ryokuConfigDir(), "theme.json"))
	if err != nil {
		return "adw"
	}
	s := struct {
		GtkTheme string `json:"gtkTheme"`
	}{}
	if json.Unmarshal(b, &s) != nil {
		return "adw"
	}
	switch s.GtkTheme {
	case "adw", "adwaita", "system":
		return s.GtkTheme
	}
	return "adw"
}

// resolveGtkTheme turns the gtkTheme knob and the resolved light/dark mode into
// the gsettings gtk-theme name the daemon sets, or "" for "system" -- where the
// user owns gtk-theme and Ryoku never writes it. The variant must match the mode
// so a light/dark flip lands on the right stylesheet (adw-gtk3 vs adw-gtk3-dark),
// since the base carries the accent-derived rules the palette rides on.
func resolveGtkTheme(mode string) string {
	dark := mode != "light"
	switch gtkThemeSetting() {
	case "system":
		return ""
	case "adwaita":
		if dark {
			return "Adwaita-dark"
		}
		return "Adwaita"
	default: // "adw"
		if dark {
			return "adw-gtk3-dark"
		}
		return "adw-gtk3"
	}
}

// gnomeAccentOn reports whether Ryoku tracks the palette onto GNOME's named
// accent (contract C3, default true). Off leaves org.gnome.desktop.interface
// accent-color untouched so a user's own choice stands.
func gnomeAccentOn() bool {
	b, err := os.ReadFile(filepath.Join(ryokuConfigDir(), "theme.json"))
	if err != nil {
		return true
	}
	s := struct {
		GnomeAccent *bool `json:"gnomeAccent"`
	}{}
	if json.Unmarshal(b, &s) != nil || s.GnomeAccent == nil {
		return true
	}
	return *s.GnomeAccent
}

// staticThemeName returns the fixed named theme selected in shell.json, or ""
// for the two dynamic variants (Default, Wallpaper) and an absent key. A named
// theme's palette is the catalog's (themePalettes), which the daemon fans into
// both the shell rail and the app templates so the two share one master.
func staticThemeName() string {
	b, err := os.ReadFile(filepath.Join(ryokuConfigDir(), "shell.json"))
	if err != nil {
		return ""
	}
	s := struct {
		Theme struct {
			Theme string `json:"theme"`
		} `json:"theme"`
	}{}
	if json.Unmarshal(b, &s) != nil {
		return ""
	}
	if !staticName(s.Theme.Theme) {
		return ""
	}
	return s.Theme.Theme
}

// staticThemeActive reports whether a fixed named theme is selected. Its palette
// is owned by the theme catalog, and the dynamic wallpaper pipeline stays idle so
// it never fights that static palette; the static render path drives apps instead.
func staticThemeActive() bool { return staticThemeName() != "" }

// staticName reports whether a theme.theme value names a fixed palette, as
// opposed to the two dynamic variants. The one place the distinction is made.
func staticName(name string) bool {
	switch name {
	case "", "Default", "Wallpaper":
		return false
	}
	return true
}

// matugenFollows reports whether the dynamic matugen pipeline owns the palette
// now: Match wallpaper on and no fixed named theme selected.
func matugenFollows() bool {
	return matchWallpaperOn() && !staticThemeActive()
}

// matugenApply runs the whole pipeline for one image: generate the scheme with
// the configured knobs, author the shell palette, fan it into the app templates,
// and nudge the toolkits to repaint. A video wallpaper is sampled to a still
// first, since matugen decodes images only.
func (d *daemon) matugenApply(img string) error {
	source := img
	if isVideo(img) {
		frame := liveFrame(img)
		if frame == "" {
			return fmt.Errorf("matugen: could not sample a still frame from %q", img)
		}
		img = frame
	}
	pal, tones, mode, err := generatePaletteStill(img, source)
	if err != nil {
		return err
	}

	// Author the shell's own palette: the one file every Quickshell singleton
	// reads (base16 for the legacy reader, camelCase Material 3 roles for
	// Theme.qml and Tokens.qml).
	if err := writeJSONFile(matugenColorsPath(), matugenColorsJSON(pal)); err != nil {
		return fmt.Errorf("matugen colors.json: %w", err)
	}

	// Retint the Material pointer now, parallel with the template fan-out below,
	// so it tracks the wallpaper as promptly as the bar instead of waiting on
	// matugen's post_hook (which runs only after every template has rendered). It
	// reads the colors.json just written; a no-op unless the Material cursor is
	// selected, and its own lock makes the post_hook's later run idempotent.
	go func() { _ = runCommand("ryoku-cursor-material-recolor") }()

	// And the tonal ramps behind those roles, from the same run.
	if tones != nil {
		if err := writeJSONFile(matugenTonesPath(), tones); err != nil {
			fmt.Fprintf(os.Stderr, "matugen tones.json: %v\n", err)
		}
	}

	// Fan the same palette into the app suite through matugen's templating pass,
	// gated by the per-app roster and the app-suite toggle.
	matugenRenderTemplates(matugenShellPalette(pal), readMatugenKnobs())

	// Toolkit nudges so running apps re-read the regenerated configs. Hyprland is
	// reloaded by the caller (paintWorker).
	matugenReload(mode)
	return nil
}

// generatePaletteStill runs matugen on a still image with the configured knobs
// and neutralizes an achromatic picture, returning the shell palette, the tonal
// ramps, and the resolved light/dark mode -- the exact values matugenApply
// writes, with no cache writes and no desktop effects. Shared by apply and the
// ryowalls preview so the specimen never diverges from what Set paints. `source`
// is the wallpaper the user picked, a clip for a live one, and it is what the
// smart light/dark reads.
func generatePaletteStill(img, source string) (map[string]string, map[string]map[string]string, string, error) {
	k := readMatugenKnobs()
	mode := resolveMode(k.Mode, source)
	args, err := matugenArgs(img, k, mode)
	if err != nil {
		return nil, nil, "", err
	}
	out, err := runMatugenCapture(args)
	if err != nil {
		return nil, nil, "", err
	}
	pal, err := parseMatugenPalette(out, mode)
	if err != nil {
		return nil, nil, "", err
	}
	tones := parseMatugenTones(out)

	// A wallpaper with essentially no colour gives matugen no source hue to
	// extract, so it falls back to its built-in blue and paints a blue palette
	// onto a black-and-white picture. Strip the invented hue to grays that match
	// the picture, luminance preserved so every contrast still lands.
	if imageAchromatic(img) {
		neutralizePalette(pal)
		neutralizeTones(tones)
	}
	return pal, tones, mode, nil
}

// matugenPreview prints, as JSON, the palette an image WOULD produce on apply --
// the shell colours, the tonal ramps, and the wallpaper's 8x8 L* map -- without
// touching the cache or the desktop. ryowalls calls it so its live preview and
// spectrum specimen show exactly what Set will paint (one generator, no drift).
func matugenPreview(img string) error {
	source := img
	if isVideo(img) {
		if f := liveFrame(img); f != "" {
			img = f
		} else {
			return fmt.Errorf("matugen-preview: could not sample a still from %q", img)
		}
	}
	pal, tones, _, err := generatePaletteStill(img, source)
	if err != nil {
		return err
	}
	out := map[string]any{
		"colors": matugenColorsJSON(pal),
		"tones":  tones,
	}
	// Merge the wallpaper tone map (grid, detail, cols, rows, lstar) through the
	// one builder the on-disk map uses, so the preview and the published file
	// never disagree on a cell.
	if m, ok := wallToneMap(img); ok {
		for k, v := range m {
			out[k] = v
		}
	}
	return json.NewEncoder(os.Stdout).Encode(out)
}

// matugenApplyStatic renders the app templates from a fixed named theme's curated
// palette and nudges the toolkits, so a static theme reaches the same terminal /
// monitor / GUI suite the wallpaper path does. Shell rail and apps then follow one
// master instead of splitting (rail static, apps stuck on the last wallpaper
// render). No wallpaper is involved; the palette is the catalog's.
func (d *daemon) matugenApplyStatic(name string) error {
	pal, ok := lookupThemePalette(name)
	if !ok {
		return fmt.Errorf("no palette for static theme %q", name)
	}
	shell := staticThemePalette(pal)
	// Author the shell palette too, so every colors.json reader (the launcher and
	// the desktop surfaces) tracks a fixed named theme, not only the wallpaper
	// path: the control plane owns colors.json for both palette sources.
	if err := writeJSONFile(matugenColorsPath(), matugenColorsJSON(shell)); err != nil {
		return fmt.Errorf("matugen colors.json (static): %w", err)
	}
	k := readMatugenKnobs()
	matugenRenderTemplates(matugenShellPalette(shell), k)
	matugenReload(staticPaletteMode(shell))
	// A catalog theme has no ramps, so the last wallpaper's are now wrong. The
	// file's absence is the gate: Ink re-lights the theme's own roles instead.
	_ = os.Remove(matugenTonesPath())
	return nil
}

// staticThemePalette converts a catalog theme's camelCase role palette into the
// snake_case flat palette the templating pass consumes, then fills the extended
// Material roles the catalog omits (surface_bright/dim, the *_fixed accent tones,
// inverse_primary) from the curated set, so every template block resolves instead
// of rendering an empty value. Each extended tone collapses onto the nearest
// curated role that stays legible on the theme's surface.
func staticThemePalette(pal map[string]string) map[string]string {
	conv := map[string]string{"background": "background", "onBackground": "on_background"}
	for _, kv := range matugenRoleKeys { // kv = {snake, camel}
		conv[kv[1]] = kv[0]
	}
	out := make(map[string]string, len(pal)+16)
	for camel, hex := range pal {
		if snake, ok := conv[camel]; ok && hex != "" {
			out[snake] = hex
		}
	}
	fill := func(k, from string) {
		if out[k] == "" {
			out[k] = out[from]
		}
	}
	// Extended surfaces approximate the container ramp.
	fill("surface_bright", "surface_container_high")
	fill("surface_dim", "surface_container_low")
	// The catalog carries only the main accent plus its container, so the fixed
	// accent tones collapse onto the legible main accent.
	fill("primary_fixed", "primary")
	fill("primary_fixed_dim", "primary")
	fill("secondary_fixed", "secondary")
	fill("secondary_fixed_dim", "secondary")
	fill("tertiary_fixed", "tertiary")
	fill("tertiary_fixed_dim", "tertiary")
	fill("inverse_primary", "primary")
	// On-fixed inks used as syntax / dim colours: keep them contrasting a surface.
	fill("on_primary_fixed", "on_primary_container")
	fill("on_primary_fixed_variant", "on_surface_variant")
	fill("on_secondary_fixed", "on_secondary_container")
	fill("on_secondary_fixed_variant", "on_surface_variant")
	fill("on_tertiary_fixed", "on_tertiary_container")
	fill("on_tertiary_fixed_variant", "on_surface_variant")
	return out
}

// staticPaletteMode picks light/dark for a static palette from its surface luma,
// so the toolkit reload sets the matching libadwaita colour-scheme preference.
func staticPaletteMode(shell map[string]string) string {
	if l, ok := hexLuma(shell["surface"]); ok && l >= 0.5 {
		return "light"
	}
	return "dark"
}

// hexLuma returns a #rrggbb colour's mean perceptual luma in 0..1 (the shell's
// 0.299/0.587/0.114 weights), or ok=false when the string is not a hex colour.
func hexLuma(hex string) (float64, bool) {
	h := strings.TrimPrefix(hex, "#")
	if len(h) != 6 {
		return 0, false
	}
	v, err := strconv.ParseInt(h, 16, 64)
	if err != nil {
		return 0, false
	}
	r := float64((v >> 16) & 0xff)
	g := float64((v >> 8) & 0xff)
	b := float64(v & 0xff)
	return (0.299*r + 0.587*g + 0.114*b) / 255.0, true
}

// runMatugenCapture runs matugen and returns its stdout, folding stderr into the
// error so a failed scheme generation is diagnosable in the daemon log.
func runMatugenCapture(args []string) ([]byte, error) {
	cmd := exec.Command("matugen", args...)
	var errBuf strings.Builder
	cmd.Stderr = &errBuf
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("matugen %s: %w: %s", args[0], err, strings.TrimSpace(errBuf.String()))
	}
	return out, nil
}

// parseMatugenPalette pulls the flat role -> hex palette out of matugen's --json
// output for the active mode, falling back through default/dark/light so a role
// missing the requested mode still resolves.
func parseMatugenPalette(out []byte, modeKey string) (map[string]string, error) {
	var doc struct {
		Colors map[string]map[string]struct {
			Color string `json:"color"`
			Hex   string `json:"hex"`
		} `json:"colors"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		return nil, fmt.Errorf("matugen json: %w", err)
	}
	if len(doc.Colors) == 0 {
		return nil, fmt.Errorf("matugen output carried no colors")
	}
	order := []string{modeKey, "default", "dark", "light"}
	pal := make(map[string]string, len(doc.Colors))
	for role, buckets := range doc.Colors {
		for _, m := range order {
			e, ok := buckets[m]
			if !ok {
				continue
			}
			if e.Color != "" {
				pal[role] = e.Color
				break
			}
			if e.Hex != "" {
				pal[role] = e.Hex
				break
			}
		}
	}
	if len(pal) == 0 {
		return nil, fmt.Errorf("matugen output had no usable colors")
	}
	return pal, nil
}

// parseMatugenTones pulls the six tonal palettes out of the same --json output
// the roles come from, as ramp -> tone -> hex. matugen computes them on every
// run and the pipeline used to drop them. A role is one tone chosen to read on
// a surface; a surface-less spectrum bar has to pick its own, and taking it off
// the ramp keeps the chroma Material tuned for that tone.
func parseMatugenTones(out []byte) map[string]map[string]string {
	var doc struct {
		Palettes map[string]map[string]struct {
			Color string `json:"color"`
			Hex   string `json:"hex"`
		} `json:"palettes"`
	}
	if json.Unmarshal(out, &doc) != nil || len(doc.Palettes) == 0 {
		return nil
	}
	ramps := make(map[string]map[string]string, len(doc.Palettes))
	for name, tones := range doc.Palettes {
		ramp := make(map[string]string, len(tones))
		for tone, e := range tones {
			switch {
			case e.Color != "":
				ramp[tone] = e.Color
			case e.Hex != "":
				ramp[tone] = e.Hex
			}
		}
		if len(ramp) > 0 {
			ramps[matugenRampName(name)] = ramp
		}
	}
	if len(ramps) == 0 {
		return nil
	}
	return ramps
}

// matugenRampName renders a ramp name in the camelCase the QML side reads.
func matugenRampName(name string) string {
	if name == "neutral_variant" {
		return "neutralVariant"
	}
	return name
}

// neutralizeHex returns the gray with the same relative luminance as hex: the
// (invented) hue removed, the lightness -- and so every contrast the shell
// derives from the palette -- left exactly where it was. A non-#rrggbb value
// passes through untouched.
func neutralizeHex(hex string) string {
	h := strings.TrimPrefix(hex, "#")
	if len(h) != 6 {
		return hex
	}
	v, err := strconv.ParseInt(h, 16, 64)
	if err != nil {
		return hex
	}
	y := relLuminance(uint8(v>>16), uint8(v>>8), uint8(v))
	n := int(math.Round(srgbFromLinear(y) * 255))
	if n < 0 {
		n = 0
	} else if n > 255 {
		n = 255
	}
	return fmt.Sprintf("#%02x%02x%02x", n, n, n)
}

// srgbFromLinear gamma-encodes a linear channel, the inverse of the linearise in
// relLuminance, so a luminance maps back to an 8-bit sRGB gray.
func srgbFromLinear(u float64) float64 {
	if u <= 0.0031308 {
		return u * 12.92
	}
	return 1.055*math.Pow(u, 1.0/2.4) - 0.055
}

// neutralizePalette strips every role to its gray, in place.
func neutralizePalette(pal map[string]string) {
	for k, v := range pal {
		pal[k] = neutralizeHex(v)
	}
}

// neutralizeTones strips every tone of every ramp to its gray, in place; a nil
// map (no ramps were published) is left alone.
func neutralizeTones(tones map[string]map[string]string) {
	for _, ramp := range tones {
		for t, v := range ramp {
			ramp[t] = neutralizeHex(v)
		}
	}
}

// matugenBase16 maps the Material 3 palette onto the sixteen-colour base16 keys
// the legacy reader (and the terminal / Qt templates) consume. The role
// assignment matches the shipped engine so both palette sources read alike.
//
// The accent slots (color5/6/9/13) map to the main accent roles (primary,
// secondary, tertiary, error), never the *_container / inverse_primary roles.
// A container role is a low-emphasis fill whose lightness inverts between modes:
// it clears 3:1 on the dark surface but washes to ~2.6:1 on a light one -- the
// pale-on-pale btop meters and washed terminal accents on a cream palette. The
// main accents are the tones M3 designs to contrast the surface, so they stay
// legible in both modes. color8 (bright black) stays `outline`, a mid gray
// legible on either surface, for muted text and comments -- not surface_variant,
// a background role that painted muted ink invisibly dark-on-dark.
func matugenBase16(pal map[string]string) map[string]string {
	pick := func(k, fallback string) string {
		if v := pal[k]; v != "" {
			return v
		}
		return fallback
	}
	bg := pick("surface", pick("background", "#121212"))
	fg := pick("on_surface", pick("on_background", "#e6e6e6"))
	primary := pick("primary", "#a8c7fa")
	secondary := pick("secondary", "#7cacf8")
	tertiary := pick("tertiary", "#ffb4a9")
	errc := pick("error", "#ffb4ab")
	outline := pick("outline", "#8e918f")
	return map[string]string{
		"background": bg, "foreground": fg, "cursor": fg,
		"color0": bg, "color1": errc, "color2": primary, "color3": tertiary,
		"color4": secondary, "color5": primary,
		"color6": tertiary, "color7": fg,
		"color8": outline, "color9": errc,
		"color10": primary, "color11": tertiary, "color12": secondary,
		"color13": primary, "color14": outline,
		"color15": pick("on_primary_container", fg),
	}
}

// matugenShellPalette is the carrier palette handed to the templating pass: the
// base16 keys plus every raw Material 3 role, so a template resolves whether it
// names base16 slots (kitty, Qt) or Material 3 roles (GTK, discord).
func matugenShellPalette(pal map[string]string) map[string]string {
	m := matugenBase16(pal)
	for k, v := range pal {
		if v != "" {
			m[k] = v
		}
	}
	return m
}

// matugenRoleKeys maps matugen's snake_case Material 3 roles to the camelCase
// keys Theme.qml resolves, so the shell palette JSON keys line up with the 30
// roles (plus shadow and scrim) the pill reads.
var matugenRoleKeys = [][2]string{
	{"surface", "surface"},
	{"surface_variant", "surfaceVariant"},
	{"surface_container_lowest", "surfaceContainerLowest"},
	{"surface_container_low", "surfaceContainerLow"},
	{"surface_container", "surfaceContainer"},
	{"surface_container_high", "surfaceContainerHigh"},
	{"surface_container_highest", "surfaceContainerHighest"},
	{"inverse_surface", "inverseSurface"},
	{"inverse_on_surface", "inverseOnSurface"},
	{"surface_tint", "surfaceTint"},
	{"primary", "primary"},
	{"primary_container", "primaryContainer"},
	{"secondary", "secondary"},
	{"secondary_container", "secondaryContainer"},
	{"tertiary", "tertiary"},
	{"tertiary_container", "tertiaryContainer"},
	{"error", "error"},
	{"error_container", "errorContainer"},
	{"outline", "outline"},
	{"outline_variant", "outlineVariant"},
	{"on_surface", "onSurface"},
	{"on_surface_variant", "onSurfaceVariant"},
	{"on_primary", "onPrimary"},
	{"on_primary_container", "onPrimaryContainer"},
	{"on_secondary", "onSecondary"},
	{"on_secondary_container", "onSecondaryContainer"},
	{"on_tertiary", "onTertiary"},
	{"on_tertiary_container", "onTertiaryContainer"},
	{"on_error", "onError"},
	{"on_error_container", "onErrorContainer"},
	{"shadow", "shadow"},
	{"scrim", "scrim"},
}

// matugenColorsJSON is the shell palette written to colors.json: the base16 keys
// the legacy reader consumes plus the camelCase Material 3 roles Theme.qml and
// Tokens.qml read, both from the one matugen palette.
func matugenColorsJSON(pal map[string]string) map[string]string {
	out := matugenBase16(pal)
	for _, kv := range matugenRoleKeys {
		if v := pal[kv[0]]; v != "" {
			out[kv[1]] = v
		}
	}
	return out
}

// matugenCarrier wraps the flat palette in the nested shape matugen's template
// engine expects (role.default.hex, role.hex, role.rgb, ...), plus an opaque
// _argb variant per colour for templates that need it (Qt).
func matugenCarrier(pal map[string]string) map[string]any {
	colors := make(map[string]any, len(pal)*2)
	put := func(name, hex string) {
		if hex == "" {
			return
		}
		stripped := strings.TrimPrefix(hex, "#")
		var r, g, b int64
		if len(stripped) == 6 {
			r, _ = strconv.ParseInt(stripped[0:2], 16, 0)
			g, _ = strconv.ParseInt(stripped[2:4], 16, 0)
			b, _ = strconv.ParseInt(stripped[4:6], 16, 0)
		}
		h, s, l := rgbToHSL(r, g, b)
		co := map[string]any{
			"hex":          hex,
			"hex_stripped": stripped,
			"red":          strconv.FormatInt(r, 10),
			"green":        strconv.FormatInt(g, 10),
			"blue":         strconv.FormatInt(b, 10),
			"rgb":          fmt.Sprintf("%d, %d, %d", r, g, b),
			"hue":          strconv.Itoa(h),
			"saturation":   strconv.Itoa(s),
			"lightness":    strconv.Itoa(l),
			"hsl":          fmt.Sprintf("hsl(%d, %d%%, %d%%)", h, s, l),
		}
		entry := map[string]any{"default": co, "dark": co, "light": co}
		for key, val := range co {
			entry[key] = val
		}
		colors[name] = entry
	}
	for k, v := range pal {
		put(k, v)
		put(k+"_argb", "#ff"+strings.TrimPrefix(v, "#"))
	}
	return map[string]any{"colors": colors}
}

// rgbToHSL converts an 8-bit sRGB triple to HSL as hue in degrees [0,360) and
// saturation/lightness in whole percent, the units CSS hsl() and Obsidian's
// --accent-h/s/l expect. The carrier is hex/rgb only in matugen json mode, so a
// template that needs the accent as an HSL triple (Obsidian derives its whole
// accent chain from --accent-h/s/l) gets it from here rather than a hook.
func rgbToHSL(r, g, b int64) (int, int, int) {
	rf, gf, bf := float64(r)/255, float64(g)/255, float64(b)/255
	max := math.Max(rf, math.Max(gf, bf))
	min := math.Min(rf, math.Min(gf, bf))
	l := (max + min) / 2
	if max == min {
		return 0, 0, int(math.Round(l * 100)) // achromatic: hue and saturation undefined
	}
	d := max - min
	var s float64
	if l > 0.5 {
		s = d / (2 - max - min)
	} else {
		s = d / (max + min)
	}
	var h float64
	switch max {
	case rf:
		h = (gf - bf) / d
		if gf < bf {
			h += 6
		}
	case gf:
		h = (bf-rf)/d + 2
	default:
		h = (rf-gf)/d + 4
	}
	h /= 6
	return int(math.Round(h * 360)), int(math.Round(s * 100)), int(math.Round(l * 100))
}

// templateGroup maps a matugen template block name to its roster key, so one
// roster toggle (e.g. "gtk", "discord", "qt") governs every block that themes
// that app.
func templateGroup(block string) string {
	switch block {
	case "gtk3", "gtk4":
		return "gtk"
	case "vesktop", "equibop":
		return "discord"
	case "qt6ct", "kde":
		return "qt"
	case "hypr":
		return "hyprland"
	default:
		return block
	}
}

// filterMatugenConfig keeps the [config] preamble and only the [templates.X]
// blocks whose roster group `enabled` reports on, so the rendered set is exactly
// the roster the user opted into. A block runs from its header to the next
// header; its comments and post_hook ride with it.
func filterMatugenConfig(toml string, enabled func(group string) bool) string {
	var out []string
	keep := true // the [config] preamble always renders
	for _, ln := range strings.Split(toml, "\n") {
		t := strings.TrimSpace(ln)
		switch {
		case strings.HasPrefix(t, "[templates."):
			block := strings.TrimSuffix(strings.TrimPrefix(t, "[templates."), "]")
			keep = enabled(templateGroup(block))
		case strings.HasPrefix(t, "[config"):
			keep = true
		}
		if keep {
			out = append(out, ln)
		}
	}
	return strings.Join(out, "\n")
}

// matugenRenderTemplates fans the palette into the deployed templates. The core
// surface (config.toml: kitty, the Hyprland border, btop, Qt) always themes,
// per-app roster gated. The GTK / GUI-app suite (apps.toml) themes only when the
// app-suite toggle is on; off blanks the GTK stylesheets so those apps fall back
// to stock.
func matugenRenderTemplates(shell map[string]string, k matugenKnobs) {
	dir := matugenTemplateDir()
	matugenEnsureDirs()
	carrierPath := filepath.Join(matugenCacheHome(), "ryoku", "matugen-carrier.json")
	if err := writeJSONFile(carrierPath, matugenCarrier(shell)); err != nil {
		fmt.Fprintf(os.Stderr, "matugen carrier: %v\n", err)
		return
	}
	// A roster key the user explicitly turned off stays off; a group ABSENT from
	// their roster is one that shipped after they last saved it, and defaults on.
	// Absent-means-off would have kept every app added from here on dark for
	// everyone who had ever opened the appearance page.
	enabled := func(group string) bool {
		if group == "steam" && !steamThemeReady() {
			return false
		}
		if v, ok := k.Templates[group]; ok {
			return v
		}
		return true
	}
	matugenRenderFiltered(filepath.Join(dir, "config.toml"), carrierPath, enabled)
	// Two switches gate the app suite and both have to agree: the appearance
	// page's per-group roster (themeRyokuApps) and the master "Theme apps" in
	// theme.json. The Hub blanks the stylesheets when the master goes off, so
	// honouring the roster alone here re-rendered them on the next repaint and
	// silently undid it.
	if k.ThemeRyokuApps && themeAppsEnabled() {
		matugenRenderFiltered(filepath.Join(dir, "apps.toml"), carrierPath, enabled)
		// matugen wrote the shared snippet target; poke each vault's symlink so
		// Obsidian's watcher, which never sees the out-of-vault target change,
		// reloads the new palette without a restart.
		if enabled("obsidian") {
			nudgeObsidian()
		}
	} else {
		blankGtk(matugenConfigHome())
	}
}

// nudgeObsidian re-links the palette snippet inside every registered Obsidian
// vault so the vault's file watcher fires and Obsidian reloads the freshly
// rendered CSS. matugen writes one shared snippet target outside every vault
// (~/.config/matugen/generated/obsidian.css); the vault holds a symlink to it,
// and inotify on the vault's snippets directory does not follow the link, so
// Obsidian never learns the target changed. A symlink recreated atomically
// (temp name then rename) fires IN_MOVED_* inside the vault, which the watcher
// does see, and leaves a symlink so the doctor's snippet check stays satisfied.
// Only a vault whose ryoku.css is already a symlink is touched; a regular file
// there is the user's own and left alone. Best-effort throughout: a missing
// registry or a link that will not recreate is skipped, never surfaced, so a
// palette apply never fails on Obsidian's account.
func nudgeObsidian() {
	b, err := os.ReadFile(filepath.Join(matugenConfigHome(), "obsidian", "obsidian.json"))
	if err != nil {
		return
	}
	var doc struct {
		Vaults map[string]struct {
			Path string `json:"path"`
		} `json:"vaults"`
	}
	if json.Unmarshal(b, &doc) != nil {
		return
	}
	generated := filepath.Join(matugenConfigHome(), "matugen", "generated", "obsidian.css")
	for _, v := range doc.Vaults {
		vault := strings.TrimSpace(v.Path)
		if vault == "" {
			continue
		}
		link := filepath.Join(vault, ".obsidian", "snippets", "ryoku.css")
		fi, err := os.Lstat(link)
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			continue // no link yet (doctor not run) or a user's own file
		}
		tmp := link + ".ryoku-tmp"
		_ = os.Remove(tmp)
		if os.Symlink(generated, tmp) != nil {
			continue
		}
		if os.Rename(tmp, link) != nil {
			_ = os.Remove(tmp)
		}
	}
}

// matugenRenderFiltered renders one matugen config with only its roster-enabled
// template blocks. The filtered config is staged in the cache so the shipped
// template map stays the single source of the destinations and post_hooks.
func matugenRenderFiltered(config, carrier string, enabled func(group string) bool) {
	b, err := os.ReadFile(config)
	if err != nil {
		return
	}
	active := filepath.Join(matugenCacheHome(), "ryoku", "active-"+filepath.Base(config))
	if err := os.WriteFile(active, []byte(filterMatugenConfig(string(b), enabled)), 0o644); err != nil {
		return
	}
	matugenRender(active, carrier)
}

// matugenRender runs one templating pass over a config, logging matugen's own
// output on failure. A missing config is skipped, not an error.
func matugenRender(config, carrier string) {
	if _, err := os.Stat(config); err != nil {
		return
	}
	if out, err := exec.Command("matugen", "-c", config, "json", carrier).CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "matugen render %s: %v: %s\n", filepath.Base(config), err, strings.TrimSpace(string(out)))
	}
}

// runCommand / runCommandOutput are the process shims the reload and luminance
// steps go through, so a test can record the invocations without spawning
// anything.
var (
	runCommand = func(name string, args ...string) error {
		return exec.Command(name, args...).Run()
	}
	runCommandOutput = func(name string, args ...string) ([]byte, error) {
		return exec.Command(name, args...).Output()
	}
)

// matugenReload lands the desktop settings that follow a palette: the libadwaita
// colour-scheme preference tracks light/dark, org.gnome accent-color tracks the
// palette's primary, gtk-theme lands the variant for the mode, and SIGUSR1
// reloads kitty (its kitty.conf includes current-theme.conf). The daemon is the
// single writer of the three GTK-facing gsettings keys. Hyprland is reloaded by
// the caller.
//
// What this does NOT do is repaint an already-open GTK app. Measured on a bare
// Wayland session: a running GTK 3 or GTK 4 app picks up neither a rewritten
// ~/.config/gtk-*/gtk.css nor a changed gtk-theme, with or without an
// xsettings-less settings.ini, so it keeps the palette it started with until it
// restarts. The writes below are still worth making: every app launched after
// this point is correct, and they are what an X11 or xsettings-backed session
// needs to follow along.
func matugenReload(mode string) {
	scheme := "prefer-dark"
	if mode == "light" {
		scheme = "prefer-light"
	}
	_ = runCommand("gsettings", "set", "org.gnome.desktop.interface", "color-scheme", scheme)

	applyGnomeAccent()
	applyKdeColors()

	// gtkTheme "system" (name == "") means the user owns gtk-theme, so Ryoku
	// leaves it entirely alone. Otherwise land the variant for this mode.
	if name := resolveGtkTheme(mode); name != "" {
		matugenNudgeGtk(name)
	}
	_ = runCommand("pkill", "-USR1", "-x", "kitty")
	nudgePalette()
}

// nudgePalette tells the shell's bar to re-read the palette just written to
// colors.json. The bar and frame chrome otherwise learn of a palette change only
// through a colors.json file watch, which can miss an atomic-rename replacement
// and strand them on a stale palette until the next change (or a manual
// re-theme). This socket push over the bar's `theme` IpcHandler is the reliable
// path; the file watch stays as a best-effort fallback. Fire-and-forget so a
// restarting or absent bar never stalls the paint worker.
func nudgePalette() {
	go ipcCall("shell", "theme", "reload", "")
}

// applyBorderColors lands the palette's border colours on the live compositor
// when the provider can recolour the border from the palette. It reads the roles
// the caller just wrote to colors.json; the provider owns the colour literal
// format and whether the store has pinned a fixed colour (then the act no-ops).
func (d *daemon) applyBorderColors() {
	if !d.wmc.Can(wm.CapPaletteBorder) {
		return
	}
	active, inactive, ok := paletteBorderColors()
	if !ok {
		return
	}
	_ = d.wmc.Act(wm.ActionBorderColors, active, inactive)
}

// paletteBorderColors reads the active (color4) and inactive (background) border
// roles from the palette written to colors.json.
func paletteBorderColors() (active, inactive string, ok bool) {
	b, err := os.ReadFile(matugenColorsPath())
	if err != nil {
		return "", "", false
	}
	var c struct {
		Color4     string `json:"color4"`
		Background string `json:"background"`
	}
	if json.Unmarshal(b, &c) != nil {
		return "", "", false
	}
	if c.Color4 == "" && c.Background == "" {
		return "", "", false
	}
	return c.Color4, c.Background, true
}

// matugenNudgeGtk lands gtk-theme on `want`, flipping through a placeholder first
// so the key emits a change signal even when the name is unchanged (setting a key
// to its current value emits nothing). The placeholder is a real, always-installed
// theme rather than the empty string: an app launched during the flip window reads
// an empty gtk-theme as no theme at all and renders unstyled, the documented cause
// of libadwaita and Flatpak apps losing their styling on a retint. `want` is always
// a concrete name here; "system" is handled by the caller not calling this at all.
func matugenNudgeGtk(want string) {
	placeholder := "Adwaita"
	if want == placeholder {
		placeholder = "Adwaita-dark"
	}
	_ = runCommand("gsettings", "set", "org.gnome.desktop.interface", "gtk-theme", placeholder)
	_ = runCommand("gsettings", "set", "org.gnome.desktop.interface", "gtk-theme", want)
}

// applyGnomeAccent tracks the palette's primary onto GNOME's nine named accents
// (org.gnome.desktop.interface accent-color), so Flatpaks and apps that read the
// setting rather than our CSS follow the wallpaper. The primary comes from the
// palette the pipeline just authored (~/.cache/ryoku/colors.json, the one file
// every colours reader shares); an unreadable palette or a non-hex primary skips
// the write rather than guessing. Gated off (gnomeAccent false) leaves the
// user's own accent alone.
func applyGnomeAccent() {
	if !gnomeAccentOn() {
		return
	}
	b, err := os.ReadFile(matugenColorsPath())
	if err != nil {
		return
	}
	s := struct {
		Primary string `json:"primary"`
	}{}
	if json.Unmarshal(b, &s) != nil {
		return
	}
	name, ok := nearestGnomeAccent(s.Primary)
	if !ok {
		return
	}
	_ = runCommand("gsettings", "set", "org.gnome.desktop.interface", "accent-color", name)
}

// kdeglobalsPath is the file KColorScheme reads an app's colours from.
func kdeglobalsPath() string {
	return filepath.Join(matugenConfigHome(), "kdeglobals")
}

// matugenKdeColorsPath is the rendered colour groups waiting to be merged.
func matugenKdeColorsPath() string {
	return filepath.Join(matugenCacheHome(), "ryoku", "kdeglobals-colors.conf")
}

// kdeOwnedGroup reports whether a kdeglobals group is one the palette replaces.
// Everything else in the file, the fonts, the icon theme, the widget style and
// the dialog state KDE apps write for themselves, belongs to the user and is
// copied through untouched.
func kdeOwnedGroup(name string) bool {
	return strings.HasPrefix(name, "Colors:") ||
		strings.HasPrefix(name, "ColorEffects:") ||
		name == "WM"
}

// applyKdeColors merges the rendered colour groups into kdeglobals. KDE apps
// (Dolphin, Ark, Gwenview, Kate) resolve their palette through KColorScheme
// rather than the qt6ct one, so without this they paint at Qt's defaults: on a
// dark scheme a white view background under the scheme's light text.
//
// Merged rather than written over, because kdeglobals is shared with the KDE
// apps themselves. A file that does not exist yet is fine and yields a
// colours-only one.
func applyKdeColors() {
	rendered, err := os.ReadFile(matugenKdeColorsPath())
	if err != nil {
		return
	}
	existing, err := os.ReadFile(kdeglobalsPath())
	if err != nil && !os.IsNotExist(err) {
		return
	}

	var out []string
	for _, group := range parseIniGroups(string(existing)) {
		if kdeOwnedGroup(group.name) {
			continue
		}
		if group.name == "General" {
			group.rows = setIniRow(group.rows, "ColorScheme", "Ryoku")
		}
		out = append(out, renderIniGroup(group))
	}
	for _, group := range parseIniGroups(string(rendered)) {
		out = append(out, renderIniGroup(group))
	}

	tmp := kdeglobalsPath() + ".tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(out, "\n")), 0o644); err != nil {
		return
	}
	// Rename so a KDE app reading concurrently never sees half a palette.
	if err := os.Rename(tmp, kdeglobalsPath()); err != nil {
		_ = os.Remove(tmp)
	}
}

type iniGroup struct {
	name string
	rows [][2]string
}

// parseIniGroups keeps group order and exact key spelling. Hand-rolled because
// kdeglobals uses group names like [Colors:Header][Inactive] and case-sensitive
// keys, neither of which a general INI reader round-trips.
func parseIniGroups(s string) []iniGroup {
	var groups []iniGroup
	cur := -1
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case t == "" || strings.HasPrefix(t, "#"):
		case strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]"):
			groups = append(groups, iniGroup{name: t[1 : len(t)-1]})
			cur = len(groups) - 1
		case cur >= 0:
			if k, v, ok := strings.Cut(t, "="); ok {
				groups[cur].rows = append(groups[cur].rows, [2]string{k, v})
			}
		}
	}
	return groups
}

func setIniRow(rows [][2]string, key, value string) [][2]string {
	for i := range rows {
		if rows[i][0] == key {
			rows[i][1] = value
			return rows
		}
	}
	return append(rows, [2]string{key, value})
}

func renderIniGroup(g iniGroup) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s]\n", g.name)
	for _, r := range g.rows {
		fmt.Fprintf(&b, "%s=%s\n", r[0], r[1])
	}
	return b.String()
}

// gnomeNamedAccents are libadwaita's nine named accents (contract C5), the only
// values org.gnome.desktop.interface accent-color accepts. The hex is the source
// of truth; the nearest match converts them to OKLab at compare time.
var gnomeNamedAccents = [][2]string{
	{"blue", "#3584e4"}, {"teal", "#2190a4"}, {"green", "#3a944a"},
	{"yellow", "#c88800"}, {"orange", "#ed5b00"}, {"red", "#e62d42"},
	{"pink", "#d56199"}, {"purple", "#9141ac"}, {"slate", "#6f8396"},
}

// gnomeAccentNeutralChroma is the OKLab chroma below which a primary is treated
// as neutral. It sits above a near-grey and comfortably below every real accent
// (slate, the least saturated, is ~0.037), so only a genuinely colourless
// primary trips it.
const gnomeAccentNeutralChroma = 0.03

// nearestGnomeAccent maps a #rrggbb primary to the nearest of GNOME's nine named
// accents. The enum names are hues, so the match is by OKLab hue angle: a raw
// a/b Euclidean distance instead pulls a desaturated hue toward a lower-chroma
// neighbour (a light salmon, hue-wise clearly orange, lands on the dark gold
// "yellow"), which is not what the name means. A near-neutral primary has no
// reliable hue and maps to slate, GNOME's own neutral accent -- which is also
// the right answer for an achromatic wallpaper's neutralized (gray) palette. ok
// is false when the primary is not a hex colour, so the caller skips the write.
func nearestGnomeAccent(hex string) (string, bool) {
	_, pa, pb, ok := oklab(hex)
	if !ok {
		return "", false
	}
	if math.Hypot(pa, pb) < gnomeAccentNeutralChroma {
		return "slate", true
	}
	phue := math.Atan2(pb, pa)
	best, bestDist := "", math.MaxFloat64
	for _, acc := range gnomeNamedAccents {
		_, aa, ab, _ := oklab(acc[1])
		if d := hueDistance(phue, math.Atan2(ab, aa)); d < bestDist {
			bestDist, best = d, acc[0]
		}
	}
	return best, true
}

// hueDistance is the absolute angular gap between two hue angles in radians,
// wrapped into [0, pi] so opposite sides of the wheel measure short-way round.
func hueDistance(a, b float64) float64 {
	d := math.Abs(a - b)
	if d > math.Pi {
		d = 2*math.Pi - d
	}
	return d
}

// oklab converts a #rrggbb colour to OKLab (L, a, b), ok=false for a non-hex
// value. a/b are the chroma plane the accent match compares in; L is ignored
// there (the accent names are hues, not lightnesses).
func oklab(hex string) (l, a, b float64, ok bool) {
	h := strings.TrimPrefix(hex, "#")
	if len(h) != 6 {
		return 0, 0, 0, false
	}
	v, err := strconv.ParseInt(h, 16, 64)
	if err != nil {
		return 0, 0, 0, false
	}
	lr := srgbToLinear(float64((v>>16)&0xff) / 255.0)
	lg := srgbToLinear(float64((v>>8)&0xff) / 255.0)
	lb := srgbToLinear(float64(v&0xff) / 255.0)
	lms0 := 0.4122214708*lr + 0.5363325363*lg + 0.0514459929*lb
	lms1 := 0.2119034982*lr + 0.6806995451*lg + 0.1073969566*lb
	lms2 := 0.0883024619*lr + 0.2817188376*lg + 0.6299787005*lb
	c0 := math.Cbrt(lms0)
	c1 := math.Cbrt(lms1)
	c2 := math.Cbrt(lms2)
	l = 0.2104542553*c0 + 0.7936177850*c1 - 0.0040720468*c2
	a = 1.9779984951*c0 - 2.4285922050*c1 + 0.4505937099*c2
	b = 0.0259040371*c0 + 0.7827717662*c1 - 0.8086757660*c2
	return l, a, b, true
}

// srgbToLinear removes the sRGB gamma from one channel, the linear input OKLab
// needs (the inverse of srgbFromLinear).
func srgbToLinear(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// matugenThemeSig fingerprints the settings frame's theme keys the pipeline
// depends on (the active theme name). The settings topic nudges the paint worker
// whenever this changes, so switching to or from a fixed named theme retunes the
// desktop. The scheme knobs live in matugen.json (watched separately), so they
// are not part of this signature.
func matugenThemeSig(frame []byte) string {
	var doc struct {
		Theme struct {
			Theme string `json:"theme"`
		} `json:"theme"`
	}
	if json.Unmarshal(frame, &doc) != nil {
		return ""
	}
	return doc.Theme.Theme
}

// fontSig fingerprints the frame's system-font key, so a change to it re-applies
// the font to the toolkits the same way a theme change retunes them.
func fontSig(frame []byte) string {
	var doc struct {
		FontFamily string  `json:"fontFamily"`
		FontSize   float64 `json:"fontSize"`
	}
	if json.Unmarshal(frame, &doc) != nil {
		return ""
	}
	return doc.FontFamily + "\x1f" + strconv.FormatFloat(doc.FontSize, 'f', -1, 64)
}

// applyFont pushes the chosen fonts to the toolkits without a logout: gsettings
// font-name / monospace-font-name are read live by running GTK apps, the qt6ct
// general font is rewritten for Qt's next launch, and the terminal font lands in
// a kitty include reloaded via SIGUSR1. Empty keys resolve to the shipped faces.
func applyFont(frame []byte) {
	family, mono, size := fontChoice(frame)
	sz := strconv.Itoa(size)
	_ = runCommand("gsettings", "set", "org.gnome.desktop.interface", "font-name", family+" "+sz)
	_ = runCommand("gsettings", "set", "org.gnome.desktop.interface", "document-font-name", family+" "+sz)
	_ = runCommand("gsettings", "set", "org.gnome.desktop.interface", "monospace-font-name", mono+" "+sz)
	writeQt6ctFont(family, size)
	writeKittyFont(mono, size)
	_ = runCommand("pkill", "-USR1", "-x", "kitty")
}

// fontChoice resolves the system font (family), the monospace face that follows
// it, and the base size, each with a shipped fallback. One key drives both: an
// empty family keeps the proportional UI default and a monospace terminal one.
func fontChoice(frame []byte) (family, mono string, size int) {
	var doc struct {
		FontFamily string  `json:"fontFamily"`
		FontSize   float64 `json:"fontSize"`
	}
	_ = json.Unmarshal(frame, &doc)
	if family = strings.TrimSpace(doc.FontFamily); family == "" {
		family = "Space Grotesk"
		mono = "SpaceMono Nerd Font"
	} else {
		mono = family
	}
	if size = int(doc.FontSize); size <= 0 {
		size = 11
	}
	return
}

// writeQt6ctFont swaps the family and size in qt6ct's general font line, keeping
// the style fields, so Qt apps under qt6ct render in the same face on next
// launch. Best-effort: a missing file or an unexpected shape is left untouched.
func writeQt6ctFont(family string, size int) {
	path := filepath.Join(matugenConfigHome(), "qt6ct", "qt6ct.conf")
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(string(b), "\n")
	for i, ln := range lines {
		if !strings.HasPrefix(ln, "general=") {
			continue
		}
		rest := strings.Trim(strings.TrimPrefix(ln, "general="), "\"")
		parts := strings.SplitN(rest, ",", 3)
		if len(parts) < 3 {
			return
		}
		lines[i] = "general=\"" + family + "," + strconv.Itoa(size) + "," + parts[2] + "\""
		_ = os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
		return
	}
}

// writeKittyFont lands the mono face + size in a daemon-owned kitty include, so a
// SIGUSR1 reload retypes the terminal without touching the shipped kitty.conf
// (which carries `include current-font.conf`).
func writeKittyFont(mono string, size int) {
	dir := filepath.Join(matugenConfigHome(), "kitty")
	if _, err := os.Stat(dir); err != nil {
		return
	}
	body := "font_family " + mono + "\nfont_size " + strconv.Itoa(size) + "\n"
	_ = os.WriteFile(filepath.Join(dir, "current-font.conf"), []byte(body), 0o644)
}

// watchSunMode retints the desktop when the day/night edge crosses while the
// mode knob is "sun": resolveMode only runs on a theme pass, and nothing else
// wakes one at sunrise/sunset. The check compares the resolved value, so a
// re-observed sun window that does not flip light/dark costs nothing. The
// cadence matches the night light schedule's: sun times are whole minutes and
// a few minutes of drift at the edge is invisible next to twilight.
func (d *daemon) watchSunMode() {
	last := ""
	for range time.Tick(nlTickEvery) {
		if readMatugenKnobs().Mode != "sun" {
			last = ""
			continue
		}
		sunrise, sunset, ok := daySun.window()
		if !ok {
			continue // no window yet: the smart fallback only flips on a real theme pass
		}
		got := "dark"
		if isDaytime(time.Now(), sunrise, sunset) {
			got = "light"
		}
		if last == "" {
			last = got
			continue
		}
		if got != last {
			last = got
			d.scheduleTheme()
		}
	}
}

// watchMatugenKnobs retints the desktop whenever the knob store changes, so a Hub
// appearance save (which writes matugen.json and renders nothing itself) takes
// hold at once, exactly as a wallpaper change does. scheduleTheme coalesces, so
// a burst of rapid saves themes once. A poll is simpler and more robust across
// the Hub's rename-and-replace write than an inotify watch, at negligible idle
// cost. Runs for the life of the daemon.
func (d *daemon) watchMatugenKnobs() {
	path := matugenKnobsPath()
	var last time.Time
	if fi, err := os.Stat(path); err == nil {
		last = fi.ModTime()
	}
	for range time.Tick(500 * time.Millisecond) {
		fi, err := os.Stat(path)
		if err != nil {
			continue
		}
		if m := fi.ModTime(); !m.Equal(last) {
			last = m
			d.scheduleTheme()
		}
	}
}

// --- paths -----------------------------------------------------------------

func matugenConfigHome() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return d
	}
	return filepath.Join(os.Getenv("HOME"), ".config")
}

func matugenCacheHome() string {
	if d := os.Getenv("XDG_CACHE_HOME"); d != "" {
		return d
	}
	return filepath.Join(os.Getenv("HOME"), ".cache")
}

func matugenDataHome() string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return d
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "share")
}

func matugenTemplateDir() string { return filepath.Join(matugenConfigHome(), "matugen") }

// matugenKnobsPath is the one knob store the Hub appearance page and the daemon
// share.
func matugenKnobsPath() string {
	return filepath.Join(ryokuConfigDir(), "matugen.json")
}

// matugenColorsPath is the shell palette every Quickshell singleton watches. It
// lives under the ryoku cache dir; a single reader convention serves the matugen
// palette source the shell singletons watch.
func matugenColorsPath() string {
	return filepath.Join(matugenCacheHome(), "ryoku", "colors.json")
}

// matugenTonesPath is the tonal-ramp file beside it, for the surfaces that must
// choose a tone rather than consume one. Absent under a fixed named theme.
func matugenTonesPath() string {
	return filepath.Join(matugenCacheHome(), "ryoku", "tones.json")
}

// matugenEnsureDirs pre-creates the template output directories so matugen's own
// "folder doesn't exist" warnings stay out of the daemon log.
func matugenEnsureDirs() {
	cfg := matugenConfigHome()
	cache := matugenCacheHome()
	data := matugenDataHome()
	home := os.Getenv("HOME")
	for _, d := range []string{
		filepath.Join(cache, "ryoku"),
		filepath.Join(cache, "matugen"),
		filepath.Join(cfg, "kitty"),
		filepath.Join(cfg, "btop", "themes"),
		filepath.Join(cfg, "qt6ct", "colors"),
		filepath.Join(cfg, "gtk-3.0"),
		filepath.Join(cfg, "gtk-4.0"),
		filepath.Join(cfg, "vesktop", "themes"),
		filepath.Join(cfg, "equibop", "themes"),
		filepath.Join(cfg, "obs-studio", "themes"),
		filepath.Join(cfg, "zed", "themes"),
		filepath.Join(cfg, "heroic", "store", "styles"),
		filepath.Join(cfg, "cava"),
		filepath.Join(cfg, "fish", "conf.d"),
		filepath.Join(cfg, "yazi"),
		filepath.Join(cfg, "ghostty"),
		filepath.Join(cfg, "micro", "colorschemes"),
		filepath.Join(cfg, "matugen", "generated"),
		filepath.Join(cfg, "zathura"),
		filepath.Join(cfg, "alacritty"),
		filepath.Join(cfg, "tmux"),
		filepath.Join(cfg, "ryotunes", "skins", "matugen"),
		filepath.Join(data, "TelegramDesktop", "tdata"),
	} {
		_ = os.MkdirAll(d, 0o755)
	}
	if steamThemeReady() {
		_ = os.MkdirAll(filepath.Join(home, ".steam", "steam", "steamui", "skins", "Material-Theme", "css", "main", "colors"), 0o755)
	}
}

func steamThemeReady() bool {
	home := os.Getenv("HOME")
	if home == "" {
		return false
	}

	info, err := os.Stat(filepath.Join(home, ".steam", "steam", "steamui"))
	return err == nil && info.IsDir()
}

// writeJSONFile writes v as indented JSON atomically (temp file then rename), so
// a subscriber watching the file never reads a half-written palette.
func writeJSONFile(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
