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
    { "type": "custom", "format": "\u001b[38;2;226;52;42m■\u001b[0m \u001b[38;2;143;135;112mRYOKU \u00b7 \u529b \u00b7 a hand-built Arch desktop\u001b[0m" },
    "break",
    { "type": "custom", "format": "\u001b[38;2;226;52;42m\u2500\u2500\u001b[0m \u001b[1;38;2;243;237;225mVITALS\u001b[0m \u001b[38;2;58;46;36m\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u001b[0m" },
    { "type": "cpu", "key": "CPU" },
    { "type": "gpu", "key": "GPU", "detectionMethod": "pci" },
    { "type": "command", "key": "OS", "text": "echo hi" },
    { "type": "colors", "symbol": "circle" }
  ]
}`

func loadSample(t *testing.T) ffModel {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "fastfetch"), 0o755); err != nil {
		t.Fatal(err)
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
	if r := rowByKind(m.Rows, "tagline"); r == nil || r.Text != "RYOKU \u00b7 \u529b \u00b7 a hand-built Arch desktop" {
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
	if rowByKind(m2.Rows, "tagline").Text != "RYOKU \u00b7 \u529b \u00b7 a hand-built Arch desktop" {
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

func TestBuildTaglineFormatMatchesShipped(t *testing.T) {
	got := ffTaglineFormat("226;52;42", "RYOKU \u00b7 \u529b \u00b7 a hand-built Arch desktop")
	want := "\x1b[38;2;226;52;42m\u25a0\x1b[0m \x1b[38;2;143;135;112mRYOKU \u00b7 \u529b \u00b7 a hand-built Arch desktop\x1b[0m"
	if got != want {
		t.Errorf("tagline format:\n got %q\nwant %q", got, want)
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
	palette := `{
  "keys": "1;2;3",
  "title": "4;5;6",
  "muted": "7;8;9",
  "section": "10;11;12",
  "rule": "13;14;15",
  "percent": { "green": "16;17;18", "yellow": "19;20;21", "red": "22;23;24" }
}`
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
		"38;2;1;2;3", "38;2;4;5;6", "38;2;7;8;9", "38;2;10;11;12",
		"38;2;13;14;15", "38;2;16;17;18", "38;2;19;20;21", "38;2;22;23;24",
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
	if strings.Contains(string(fixed), "38;2;1;2;3") {
		t.Fatal("generated wallpaper colours were serialized into the user config")
	}
}

func TestWriteEffectiveFastfetchUsesPersistedPaletteMode(t *testing.T) {
	loadSample(t)
	if err := os.WriteFile(fastfetchPalettePath(), []byte(`{
  "keys":"31;32;33","title":"34;35;36","muted":"37;38;39",
  "section":"40;41;42","rule":"43;44;45",
  "percent":{"green":"46;47;48","yellow":"49;50;51","red":"52;53;54"}
}`), 0o644); err != nil {
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

func TestFastfetchPaletteRejectsOutOfRangeColours(t *testing.T) {
	loadSample(t)
	if err := os.WriteFile(fastfetchPalettePath(), []byte(`{
  "keys":"256;0;0","title":"1;2;3","muted":"1;2;3",
  "section":"1;2;3","rule":"1;2;3",
  "percent":{"green":"1;2;3","yellow":"1;2;3","red":"1;2;3"}
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadFastfetchPalette(); err == nil {
		t.Fatal("out-of-range palette colour was accepted")
	}
}
