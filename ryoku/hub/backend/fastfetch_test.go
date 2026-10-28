package main

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// a representative JSONC config: comments, a $schema URL (whose // must survive
// the comment strip), an image logo, and one of every row kind the model knows
// (title, tagline, header, plain module, a module with an extra field, a command
// module, colours), plus a leading break.
const sampleFF = `{
  // editorial dossier
  "$schema": "https://example.com/schema.json",
  "logo": { "type": "kitty-direct", "source": "~/.config/fastfetch/emblem.png", "width": 28, "height": 14, "padding": { "top": 5, "right": 5, "left": 3 } },
  "display": { "color": { "keys": "38;2;226;52;42" }, "separator": "  " },
  "modules": [
    "break",
    { "type": "title", "format": "{user-name}@{host-name}" },
    { "type": "custom", "format": "\u001b[38;2;226;52;42m■\u001b[0m \u001b[38;2;143;135;112mRYOKU \u00b7 \u529b \u00b7 a hand-built {$RYOKU_BASE_OS} desktop\u001b[0m" },
    "break",
    { "type": "custom", "format": "\u001b[38;2;226;52;42m\u2500\u2500\u001b[0m \u001b[1;38;2;243;237;225mVITALS\u001b[0m \u001b[38;2;58;46;36m\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u001b[0m" },
    { "type": "cpu", "key": "CPU" },
    { "type": "gpu", "key": "GPU", "detectionMethod": "pci" },
    { "type": "command", "key": "OS", "text": "echo hi" },
    { "type": "colors", "symbol": "circle" }
  ]
}`

const sampleWallpaperPalette = `{
  "primary":"#1f2021",
  "foreground":"#222324",
  "onSurface":"#222324",
  "onSurfaceVariant":"#252627",
  "outlineVariant":"#2b2c2d",
  "tertiary":"#2e2f30",
  "secondary":"#313233",
  "error":"#343536"
}`

func loadSample(t *testing.T) ffModel {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	t.Setenv("RYOKU_BASE_OS", "Void")
	for _, path := range []string{filepath.Join(dir, "fastfetch"), filepath.Join(dir, "cache", "ryoku")} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "fastfetch", "config.jsonc"), []byte(sampleFF), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := loadFastfetch()
	if err != nil {
		t.Fatalf("loadFastfetch: %v", err)
	}
	return m
}

func rowByKind(rows []ffRow, kind string) *ffRow {
	for i := range rows {
		if rows[i].Kind == kind {
			return &rows[i]
		}
	}
	return nil
}

func TestFastfetchJapaneseGlossPreference(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if !fastfetchJapaneseGlossEnabled() {
		t.Fatal("missing setting should preserve the current Japanese gloss")
	}
	if err := os.MkdirAll(filepath.Join(dir, "ryoku"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ryoku", "shell.json"), []byte(`{"showJapaneseGloss":false}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if fastfetchJapaneseGlossEnabled() {
		t.Fatal("disabled setting was ignored")
	}
	input := []byte(`{\"format\":\"RYOKU · 力 · a hand-built Linux desktop\"}`)
	got := string(applyFastfetchJapaneseGloss(input, false))
	if strings.Contains(got, "力") || !strings.Contains(got, "RYOKU · a hand-built Linux desktop") {
		t.Fatalf("Japanese gloss remained in effective config: %s", got)
	}
}

func TestStripJSONC(t *testing.T) {
	in := `{ "$schema": "https://x/y.json", // trailing
  "a": 1, /* block */ "b": "//not a comment" }`
	out := string(stripJSONC([]byte(in)))
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("stripped output does not parse: %v\n%s", err, out)
	}
	if v["$schema"] != "https://x/y.json" {
		t.Errorf("schema URL mangled: %v", v["$schema"])
	}
	if v["b"] != "//not a comment" {
		t.Errorf("string-internal // was stripped: %v", v["b"])
	}
}

func TestLoadFastfetchModel(t *testing.T) {
	m := loadSample(t)
	if m.Logo.Kind != "image" || m.Logo.Width != 28 || m.Logo.Padding != 3 {
		t.Errorf("logo = %+v, want image/28/3", m.Logo)
	}
	if m.Accent != "226;52;42" {
		t.Errorf("accent = %q, want 226;52;42", m.Accent)
	}
	if r := rowByKind(m.Rows, "tagline"); r == nil || r.Text != "RYOKU \u00b7 \u529b \u00b7 a hand-built Void desktop" {
		t.Errorf("tagline text = %+v", r)
	}
	if r := rowByKind(m.Rows, "header"); r == nil || r.Text != "VITALS" {
		t.Errorf("header label = %+v", r)
	}
	cpu := rowByKind(m.Rows, "module")
	if cpu == nil || cpu.Module != "cpu" || cpu.Key != "CPU" {
		t.Errorf("first module = %+v, want cpu/CPU", cpu)
	}
}

func TestBuildRoundTripStable(t *testing.T) {
	m := loadSample(t)
	b, err := buildFastfetch(m)
	if err != nil {
		t.Fatal(err)
	}
	// the rebuilt config must parse and reload to an equivalent model.
	if err := os.WriteFile(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "fastfetch", "config.jsonc"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	m2, err := loadFastfetch()
	if err != nil {
		t.Fatal(err)
	}
	if len(m2.Rows) != len(m.Rows) {
		t.Fatalf("row count changed: %d -> %d", len(m.Rows), len(m2.Rows))
	}
	if rowByKind(m2.Rows, "tagline").Text != "RYOKU \u00b7 \u529b \u00b7 a hand-built Void desktop" {
		t.Errorf("tagline drifted on round-trip")
	}
	if rowByKind(m2.Rows, "header").Text != "VITALS" {
		t.Errorf("header drifted on round-trip")
	}
	// a module's extra field (gpu detectionMethod) survives the rebuild.
	if !strings.Contains(string(b), "detectionMethod") {
		t.Errorf("gpu detectionMethod dropped on rebuild")
	}
	// the command module's echo text survives.
	if !strings.Contains(string(b), "echo hi") {
		t.Errorf("command module text dropped on rebuild")
	}
}

func TestUnchangedTaglinePreservesBaseOSToken(t *testing.T) {
	m := loadSample(t)
	b, err := buildFastfetch(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), ffBaseOSToken) {
		t.Fatalf("unchanged tagline lost %q:\n%s", ffBaseOSToken, b)
	}

	tagline := rowByKind(m.Rows, "tagline")
	if tagline == nil {
		t.Fatal("tagline row missing")
	}
	tagline.TextEdited = true
	b, err = buildFastfetch(m)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), ffBaseOSToken) {
		t.Fatalf("edited tagline retained %q:\n%s", ffBaseOSToken, b)
	}
	if !strings.Contains(string(b), "a hand-built Void desktop") {
		t.Fatalf("edited tagline did not save displayed text:\n%s", b)
	}
}

func TestToggleModuleDropsIt(t *testing.T) {
	m := loadSample(t)
	// disable the cpu module.
	for i := range m.Rows {
		if m.Rows[i].Kind == "module" && m.Rows[i].Module == "cpu" {
			m.Rows[i].Enabled = false
		}
	}
	b, err := buildFastfetch(m)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Modules []json.RawMessage `json:"modules"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	for _, rm := range doc.Modules {
		if strings.Contains(string(rm), `"cpu"`) {
			t.Errorf("disabled cpu module still present: %s", rm)
		}
	}
}

func TestAccentDrivesTemplates(t *testing.T) {
	m := loadSample(t)
	m.Accent = "10;20;30"
	b, err := buildFastfetch(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "38;2;10;20;30") {
		t.Errorf("custom lines did not pick up the new accent")
	}
	// the accent must also land in display.color.keys so it colours the key
	// labels and reloads (get reads the accent back from there).
	if err := os.WriteFile(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "fastfetch", "config.jsonc"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	m2, err := loadFastfetch()
	if err != nil {
		t.Fatal(err)
	}
	if m2.Accent != "10;20;30" {
		t.Errorf("accent did not round-trip: got %q", m2.Accent)
	}
}

// The Hub preview draws the emblem from the model, so every padding the config
// carries has to survive the round trip. Losing padding.top here is what made a
// saved readout put the emblem five rows below where the preview showed it.
func TestFastfetchLogoPaddingRoundTrips(t *testing.T) {
	m := loadSample(t)
	if m.Logo.Padding != 3 || m.Logo.PaddingRight != 5 || m.Logo.PaddingTop != 5 {
		t.Fatalf("logo padding = %d/%d/%d, want left 3, right 5, top 5",
			m.Logo.Padding, m.Logo.PaddingRight, m.Logo.PaddingTop)
	}
	m.Logo.Padding, m.Logo.PaddingRight, m.Logo.PaddingTop = 2, 4, 0
	built, err := buildFastfetch(m)
	if err != nil {
		t.Fatalf("buildFastfetch: %v", err)
	}
	var out struct {
		Logo struct {
			Padding map[string]int `json:"padding"`
		} `json:"logo"`
	}
	if err := json.Unmarshal(built, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := map[string]int{"left": 2, "right": 4, "top": 0}
	for side, value := range want {
		if out.Logo.Padding[side] != value {
			t.Errorf("padding.%s = %d, want %d", side, out.Logo.Padding[side], value)
		}
	}
}

// writeTestEmblem lays down a small opaque PNG with a bright half and a dark half,
// so the bake has tones on both sides of the Bayer threshold and both palette
// entries get used.
func writeTestEmblem(t *testing.T, path string) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := range 16 {
		for x := range 16 {
			if x < 8 {
				img.SetNRGBA(x, y, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
			} else {
				img.SetNRGBA(x, y, color.NRGBA{A: 0xff})
			}
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

func TestDitherLogoBakesTwoColourThenIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "ryoku-logo.png")
	writeTestEmblem(t, src)

	baked, err := ditherFastfetchLogo("on", src)
	if err != nil {
		t.Fatalf("dither on: %v", err)
	}
	if baked != src+ffDitherSuffix {
		t.Fatalf("baked path = %q, want %q", baked, src+ffDitherSuffix)
	}

	// the bake is a genuine 1-bit indexed PNG: a two-entry palette of transparent
	// ground and bone ink, and nothing else.
	f, err := os.Open(baked)
	if err != nil {
		t.Fatalf("open baked: %v", err)
	}
	defer f.Close()
	decoded, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode baked: %v", err)
	}
	pal, ok := decoded.(*image.Paletted)
	if !ok {
		t.Fatalf("baked image is %T, want *image.Paletted (1-bit indexed)", decoded)
	}
	if len(pal.Palette) != 2 {
		t.Fatalf("palette has %d colours, want 2", len(pal.Palette))
	}
	var sawGround, sawBone bool
	wantR, wantG, wantB, wantA := ffBone.RGBA()
	for _, c := range pal.Palette {
		r, g, b, a := c.RGBA()
		if a == 0 {
			sawGround = true
		} else if r == wantR && g == wantG && b == wantB && a == wantA {
			sawBone = true
		} else {
			t.Errorf("unexpected palette colour rgba=%d,%d,%d,%d", r, g, b, a)
		}
	}
	if !sawGround || !sawBone {
		t.Errorf("palette missing a colour: ground=%v bone=%v", sawGround, sawBone)
	}

	// a second bake sees the baked source and returns it untouched: no double bake,
	// so no baked-of-a-baked file appears.
	again, err := ditherFastfetchLogo("on", baked)
	if err != nil {
		t.Fatalf("dither on (second): %v", err)
	}
	if again != baked {
		t.Fatalf("second bake returned %q, want the baked path %q unchanged", again, baked)
	}
	if _, err := os.Stat(baked + ffDitherSuffix); !os.IsNotExist(err) {
		t.Errorf("second bake wrote a double-baked file %q", baked+ffDitherSuffix)
	}

	// off returns the original source, which the bake never modified.
	restored, err := ditherFastfetchLogo("off", baked)
	if err != nil {
		t.Fatalf("dither off: %v", err)
	}
	if restored != src {
		t.Fatalf("dither off = %q, want the original %q", restored, src)
	}
}

func TestWallpaperPaletteBuildsEffectiveConfigWithoutRewritingUserConfig(t *testing.T) {
	model := loadSample(t)
	configPath := fastfetchConfigPath()
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	palette := sampleWallpaperPalette
	if err := os.WriteFile(fastfetchPalettePath(), []byte(palette), 0o644); err != nil {
		t.Fatal(err)
	}
	model.Palette = ffPaletteWallpaper
	built, err := buildFastfetchEffective(model)
	if err != nil {
		t.Fatalf("buildFastfetchEffective: %v", err)
	}
	text := string(built)
	for _, want := range []string{
		"38;2;31;32;33", "38;2;34;35;36", "38;2;37;38;39",
		"38;2;43;44;45", "38;2;46;47;48", "38;2;49;50;51", "38;2;52;53;54",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("effective config missing %q:\n%s", want, text)
		}
	}
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("building the effective config rewrote config.jsonc")
	}
	fixed, err := buildFastfetch(model)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(fixed), "38;2;31;32;33") {
		t.Fatal("generated wallpaper colours were serialized into the user config")
	}
}

func TestWriteEffectiveFastfetchUsesPersistedPaletteMode(t *testing.T) {
	loadSample(t)
	if err := os.WriteFile(fastfetchPalettePath(), []byte(sampleWallpaperPalette), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := setFastfetchPaletteMode(ffPaletteWallpaper); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "effective.json")
	if err := writeEffectiveFastfetch(out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "38;2;31;32;33") {
		t.Fatalf("effective config did not use persisted wallpaper mode:\n%s", b)
	}
}

func TestWallpaperEffectiveUsesLivePaletteAndRecolorRoles(t *testing.T) {
	loadSample(t)
	config := `// ryoku:recolor keys=primary title=foreground output=onSurface
{
  "logo": {"type":"none"},
  "display":{"color":{"keys":"38;2;226;52;42","title":"38;2;243;237;225","output":"#cec7c4"}},
  "modules":[]
}`
	if err := os.WriteFile(fastfetchConfigPath(), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	palette := `{
  "primary":"#f3b7c1","foreground":"#eae0e0","onSurface":"#d0c0b0",
  "onSurfaceVariant":"#a09080","outlineVariant":"#706050",
  "tertiary":"#405060","secondary":"#607080","error":"#901020"
}`
	if err := os.WriteFile(fastfetchPalettePath(), []byte(palette), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := setFastfetchPaletteMode(ffPaletteWallpaper); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "effective.json")
	if err := writeEffectiveFastfetch(out); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"keys": "38;2;243;183;193"`,
		`"title": "38;2;234;224;224"`,
		`"output": "#d0c0b0"`,
	} {
		if !strings.Contains(string(first), want) {
			t.Fatalf("effective config did not resolve %s from the live palette:\n%s", want, first)
		}
	}
	if strings.Contains(string(first), "226;52;42") {
		t.Fatalf("effective config retained the saved accent:\n%s", first)
	}

	changed := `{
  "primary":"#123456","foreground":"#abcdef","onSurface":"#102030",
  "onSurfaceVariant":"#203040","outlineVariant":"#304050",
  "tertiary":"#405060","secondary":"#506070","error":"#607080"
}`
	if err := os.WriteFile(fastfetchPalettePath(), []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	next := time.Now().Add(time.Second)
	if err := os.Chtimes(fastfetchPalettePath(), next, next); err != nil {
		t.Fatal(err)
	}
	if err := writeEffectiveFastfetch(out); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"keys": "38;2;18;52;86"`,
		`"title": "38;2;171;205;239"`,
		`"output": "#102030"`,
	} {
		if !strings.Contains(string(second), want) {
			t.Fatalf("palette change did not refresh %s:\n%s", want, second)
		}
	}
	if strings.Contains(string(second), "243;183;193") {
		t.Fatalf("effective config kept the previous wallpaper accent:\n%s", second)
	}
}

// A rice's config colours its separators and title text through the named
// display.color.title / display.color.separator slots, which fastfetch honours
// directly. The wallpaper pass must recolour the ones present and leave a config
// without them untouched.
func TestApplyPaletteToDisplayRecoloursNamedSlots(t *testing.T) {
	var palette ffPalette
	palette.Keys = "1;2;3"
	palette.Title = "4;5;6"
	palette.Muted = "7;8;9"
	palette.Percent.Green = "16;17;18"
	palette.Percent.Yellow = "19;20;21"
	palette.Percent.Red = "22;23;24"

	in := json.RawMessage(`{"color":{"keys":"38;2;226;52;42","title":"#f3ede1","separator":"#8f8770"},"key":{"width":9}}`)
	got := string(ffApplyPaletteToDisplay(in, palette))
	for _, want := range []string{
		`"keys":"38;2;1;2;3"`, `"title":"38;2;4;5;6"`, `"separator":"38;2;7;8;9"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("recoloured display missing %s:\n%s", want, got)
		}
	}

	// A config with no named title/separator slots keeps only keys recoloured;
	// the pass must not invent slots it does not have.
	bare := json.RawMessage(`{"color":{"keys":"38;2;226;52;42"}}`)
	gotBare := string(ffApplyPaletteToDisplay(bare, palette))
	if strings.Contains(gotBare, `"title"`) || strings.Contains(gotBare, `"separator"`) {
		t.Errorf("bare display gained slots it did not have:\n%s", gotBare)
	}
}

func TestFastfetchPaletteRejectsOutOfRangeColours(t *testing.T) {
	loadSample(t)
	if err := os.WriteFile(fastfetchPalettePath(), []byte(`{"primary":"not-a-colour"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadFastfetchPalette(); err == nil {
		t.Fatal("invalid palette colour was accepted")
	}
}

// The readout follows the wallpaper by default: with no state file pinning a
// mode, a valid matugen palette makes the mode wallpaper. A box that has never
// rendered a palette (first boot) falls back to the fixed brand palette.
func TestFastfetchPaletteDefaultsToWallpaperWhenStateAbsent(t *testing.T) {
	loadSample(t)
	if err := os.WriteFile(fastfetchPalettePath(), []byte(sampleWallpaperPalette), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readFastfetchPaletteMode(); got != ffPaletteWallpaper {
		t.Fatalf("default palette mode = %q, want %q", got, ffPaletteWallpaper)
	}
}

func TestFastfetchPaletteFallsBackToFixedWithoutPalette(t *testing.T) {
	loadSample(t)
	if got := readFastfetchPaletteMode(); got != ffPaletteFixed {
		t.Fatalf("palette mode with no palette file = %q, want %q", got, ffPaletteFixed)
	}
}

// End to end: a fresh box (no state file, palette rendered) gets a wallpaper-tinted
// effective config, so the terminal readout matches the theme without the user
// ever choosing a mode.
func TestWriteEffectiveFastfetchDefaultsToWallpaperWithoutStateFile(t *testing.T) {
	loadSample(t)
	if err := os.WriteFile(fastfetchPalettePath(), []byte(sampleWallpaperPalette), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "effective.json")
	if err := writeEffectiveFastfetch(out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "38;2;31;32;33") {
		t.Fatalf("default effective config did not follow the wallpaper palette:\n%s", b)
	}
	if strings.Contains(string(b), "38;2;226;52;42") {
		t.Fatalf("default effective config still carries the fixed brand red:\n%s", b)
	}
}

// In fixed mode the effective config is the user's own file byte-for-byte, so a
// retint never rewrites their layout or comments.
func TestWriteEffectiveFastfetchFixedCopiesConfigVerbatim(t *testing.T) {
	loadSample(t)
	out := filepath.Join(t.TempDir(), "effective.json")
	if err := writeEffectiveFastfetch(out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != sampleFF {
		t.Fatalf("fixed mode rewrote the config instead of copying it verbatim:\n%s", b)
	}
}

// A mode transition must rebuild the cache even though the config file itself
// never changes. The bug this pins: a state file that is *removed* (returning to
// the wallpaper default) has no mtime to compare, so an mtime-only freshness
// check served a stale fixed-mode cache. The resolved-mode sidecar makes the
// unset visible.
func TestWriteEffectiveFastfetchRebuildsOnModeTransition(t *testing.T) {
	loadSample(t)
	if err := os.WriteFile(fastfetchPalettePath(), []byte(sampleWallpaperPalette), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "effective.json")

	// Pin a fixed palette: the cache copies the brand-red config verbatim.
	if err := setFastfetchPaletteMode(ffPaletteFixed); err != nil {
		t.Fatal(err)
	}
	if err := writeEffectiveFastfetch(out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "38;2;226;52;42") {
		t.Fatalf("fixed cache lost the brand red:\n%s", b)
	}

	// Remove the pin (back to the wallpaper default) without touching the config.
	// The cache must rebuild tinted, not serve the stale fixed copy.
	if err := os.Remove(fastfetchPaletteStatePath()); err != nil {
		t.Fatal(err)
	}
	if err := writeEffectiveFastfetch(out); err != nil {
		t.Fatal(err)
	}
	b, err = os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "38;2;31;32;33") {
		t.Fatalf("cache did not rebuild after the mode was unset:\n%s", b)
	}
	if strings.Contains(string(b), "38;2;226;52;42") {
		t.Fatalf("cache kept stale fixed-mode brand red after unset:\n%s", b)
	}
}

// A rice lays its readout verbatim, so a tagline captured with a distribution
// baked in told every Void box it was "a hand-built Arch desktop".
func TestShippedRiceTaglinesNameTheRunningDistribution(t *testing.T) {
	readouts, err := filepath.Glob(filepath.Join("..", "..", "assets", "rices", "*", "fastfetch.jsonc"))
	if err != nil || len(readouts) == 0 {
		t.Fatalf("no shipped rice readouts found: %v", err)
	}
	for _, readout := range readouts {
		data, err := os.ReadFile(readout)
		if err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", dir)
		t.Setenv("RYOKU_BASE_OS", "Void")
		if err := os.MkdirAll(filepath.Join(dir, "fastfetch"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "fastfetch", "config.jsonc"), data, 0o644); err != nil {
			t.Fatal(err)
		}
		m, err := loadFastfetch()
		if err != nil {
			t.Fatalf("%s: %v", readout, err)
		}
		if r := rowByKind(m.Rows, "tagline"); r == nil || !strings.Contains(r.Text, "a hand-built Void desktop") {
			t.Errorf("%s tagline on Void = %+v", readout, r)
		}
	}
}
