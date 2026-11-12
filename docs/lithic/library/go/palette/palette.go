package palette

import (
	"encoding/hex"
	"encoding/json"
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Mode selects live palette following or forced signature neutrals.
type Mode uint8

const (
	Follow Mode = iota
	Dark
	Light
)

// Palette is the decoded source palette before it is mapped to Lithic roles.
type Palette struct {
	Roles  map[string]color.NRGBA
	Dark   bool
	Forced bool
	Source string
}

var osReleasePath = "/etc/os-release"

// Load resolves the Ryoku palette files using their defined precedence.
func Load(mode Mode) Palette {
	named := nestedRoles(readObject(shellSettingsPath()), "themePalette")
	follow := true
	if object := readObject(themeSettingsPath()); object != nil {
		if value, ok := object["followWallpaper"].(bool); ok {
			follow = value
		}
	}

	roles := make(map[string]color.NRGBA)
	source := "signature"
	if follow || mode != Follow {
		wallpaper := colorRoles(readObject(palettePath()))
		for key, value := range wallpaper {
			roles[key] = value
		}
		if len(wallpaper) != 0 {
			source = "wallpaper"
		}
	}
	for key, value := range named {
		roles[key] = value
	}
	if len(named) != 0 {
		source = "scheme"
	}
	if source == "signature" {
		if accent, ok := editionColor(osReleasePath); ok {
			roles[tokens.PaletteAccents["gold"]] = accent
		}
	}

	forced := mode != Follow
	dark := mode != Light
	if !forced {
		dark = true
		if surface, ok := roles["surface"]; ok {
			luma := (0.299*float64(surface.R) + 0.587*float64(surface.G) + 0.114*float64(surface.B)) / 255
			dark = luma <= 0.5
		}
	}
	return Palette{Roles: roles, Dark: dark, Forced: forced, Source: source}
}

// FromPalette maps a source palette to the complete Lithic role set. Without
// a palette Lithic paints its signature: the ryoku.dev stock (black paper,
// bone ink, the sun as the one action), with an edition's signal in place of
// the sun where os-release names one. A palette only dyes: Lithic owns every
// lightness (the tone ladder for the mode and contrast), and the palette lends
// hue and chroma, neutrals from its surface, ink from its on-surface, the
// action from its primary, the selection from its containers and jasper from
// its error.
func FromPalette(p Palette, look Look) Roles {
	steps := ladder(p.Dark, look.Contrast)
	table := signatureTable(p.Dark, look.Contrast)
	if p.Source != "wallpaper" && p.Source != "scheme" {
		r := signatureNeutrals(table, p)
		r.Sel, r.OnSel, r.OnSel2 = tableColor(table, "sel"), tableColor(table, "onSel"), tableColor(table, "onSel2")
		r.Gold, r.OnGold, r.GoldInk = tableColor(table, "gold"), tableColor(table, "onGold"), tableColor(table, "goldInk")
		r.Jasper = tableColor(table, "jasper")
		if signal, ok := p.Roles[tokens.PaletteAccents["gold"]]; ok {
			r.Gold, r.GoldInk, r.OnGold = signal, signal, InkOn(signal)
		}
		return withMinerals(r, steps)
	}

	// A forced mode keeps the signature's neutrals; following paints them
	// from the palette's surface and on-surface on the ladder.
	r := signatureNeutrals(table, p)
	if !p.Forced {
		fallback := tokens.DyeFallback
		neutral := dye{C: fallback["stoneChroma"], H: fallback["neutralHue"]}
		groundChroma := fallback["groundChroma"]
		ink := dye{C: fallback["inkChroma"], H: fallback["neutralHue"]}
		if surface, ok := p.Roles[tokens.PaletteDye["neutral"]]; ok {
			d := dyeOf(surface)
			factor := tintFactor(look.Tint)
			neutral = dye{C: min(tokens.DyeChroma["stone"], d.C) * factor, H: d.H}
			groundChroma = min(tokens.DyeChroma["ground"], d.C*tokens.DyeBoost["ground"]) * factor
		}
		if onSurface, ok := p.Roles[tokens.PaletteDye["ink"]]; ok {
			d := dyeOf(onSurface)
			ink = dye{C: min(tokens.DyeChroma["ink"], d.C), H: d.H}
		}
		vein := dye{C: neutral.C * tokens.DyeBoost["vein"], H: neutral.H}
		r.Ground = toned(steps["ground"], dye{C: groundChroma, H: neutral.H})
		r.Stone1, r.Stone2 = toned(steps["stone1"], neutral), toned(steps["stone2"], neutral)
		r.Stone3, r.Stone4 = toned(steps["stone3"], neutral), toned(steps["stone4"], neutral)
		r.Slab, r.Field = toned(steps["slab"], neutral), toned(steps["field"], neutral)
		r.Vein, r.Edge = toned(steps["vein"], vein), toned(steps["edge"], vein)
		r.Fg, r.Fg2, r.Fg3 = toned(steps["fg"], ink), toned(steps["fg2"], ink), toned(steps["fg3"], ink)
	}

	fallback := tokens.DyeFallback
	gold := dye{C: fallback["goldChroma"], H: fallback["goldHue"]}
	jasper := dye{C: tokens.DyeChroma["jasper"], H: fallback["jasperHue"]}
	sel := dye{C: fallback["selChroma"], H: fallback["neutralHue"]}
	if value, ok := p.Roles[tokens.PaletteAccents["scrim"]]; ok {
		alpha := tokens.AlphaScrimLight
		if p.Dark {
			alpha = tokens.AlphaScrimDark
		}
		r.Scrim = WithAlpha(value, alpha)
	}
	if primary, ok := p.Roles[tokens.PaletteAccents["gold"]]; ok {
		gold = accent(primary)
	}
	if container, ok := p.Roles[tokens.PaletteAccents["sel"]]; ok {
		if toward, ok := p.Roles[tokens.PaletteAccents["selMix"]]; ok {
			container = Mix(container, toward, tokens.AlphaSelMix)
		}
		d := dyeOf(container)
		sel = dye{C: min(tokens.DyeChroma["sel"], d.C), H: d.H}
	}
	if errorRole, ok := p.Roles[tokens.PaletteAccents["jasper"]]; ok {
		jasper.H = dyeOf(errorRole).H
	}
	r.Gold, r.GoldInk = toned(steps["gold"], gold), toned(steps["goldInk"], gold)
	r.OnGold = toned(steps["onGold"], dye{C: tokens.DyeChroma["onGold"], H: gold.H})
	r.Jasper = toned(steps["jasper"], jasper)
	r.Sel = toned(steps["sel"], sel)
	r.OnSel = toned(steps["onSel"], dye{C: tokens.DyeChroma["onSel"], H: sel.H})
	r.OnSel2 = Mix(r.OnSel, r.Sel, tokens.AlphaOnSel2)
	return withMinerals(r, steps)
}

// signatureNeutrals is the signature table's planes, hairlines and ink.
func signatureNeutrals(table map[string]string, p Palette) Roles {
	return Roles{
		Ground: tableColor(table, "ground"), Stone1: tableColor(table, "stone1"),
		Stone2: tableColor(table, "stone2"), Stone3: tableColor(table, "stone3"),
		Stone4: tableColor(table, "stone4"), Slab: tableColor(table, "slab"),
		Field: tableColor(table, "field"), Vein: tableColor(table, "vein"), Edge: tableColor(table, "edge"),
		Fg: tableColor(table, "fg"), Fg2: tableColor(table, "fg2"), Fg3: tableColor(table, "fg3"),
		Scrim: tableColor(table, "scrim"), Dark: p.Dark, Source: p.Source,
	}
}

func signatureTable(dark bool, contrast Contrast) map[string]string {
	switch {
	case dark && contrast == High:
		return tokens.SignatureDarkHigh
	case dark:
		return tokens.SignatureDark
	case contrast == High:
		return tokens.SignatureLightHigh
	default:
		return tokens.SignatureLight
	}
}

// withMinerals adds the fixed state minerals and the lightness their fields
// take on this ladder.
func withMinerals(r Roles, steps map[string]float64) Roles {
	minerals := tokens.MineralsLight
	if r.Dark {
		minerals = tokens.MineralsDark
	}
	r.Jade, r.Lapis, r.Amber = tableColor(minerals, "jade"), tableColor(minerals, "lapis"), tableColor(minerals, "amber")
	r.mineralTone = steps["mineralField"]
	return r
}

func configHome() string {
	if value := os.Getenv("XDG_CONFIG_HOME"); value != "" {
		return value
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

func cacheHome() string {
	if value := os.Getenv("XDG_CACHE_HOME"); value != "" {
		return value
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache")
}

func shellSettingsPath() string { return filepath.Join(configHome(), "ryoku", "shell.json") }
func themeSettingsPath() string { return filepath.Join(configHome(), "ryoku", "theme.json") }
func palettePath() string       { return filepath.Join(cacheHome(), "ryoku", "colors.json") }

func readObject(path string) map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var object map[string]any
	if json.Unmarshal(data, &object) != nil {
		return nil
	}
	return object
}

func nestedRoles(object map[string]any, key string) map[string]color.NRGBA {
	nested, ok := object[key].(map[string]any)
	if !ok {
		return nil
	}
	return colorRoles(nested)
}

func colorRoles(object map[string]any) map[string]color.NRGBA {
	roles := make(map[string]color.NRGBA)
	for key, raw := range object {
		value, ok := raw.(string)
		if !ok {
			continue
		}
		if parsed, ok := parseColor(value); ok {
			roles[key] = parsed
		}
	}
	return roles
}

func parseColor(value string) (color.NRGBA, bool) {
	if (len(value) != 7 && len(value) != 9) || value[0] != '#' {
		return color.NRGBA{}, false
	}
	decoded, err := hex.DecodeString(value[1:])
	if err != nil {
		return color.NRGBA{}, false
	}
	parsed := color.NRGBA{R: decoded[0], G: decoded[1], B: decoded[2], A: 0xff}
	if len(decoded) == 4 {
		parsed.A = decoded[3]
	}
	return parsed, true
}

func tableColor(table map[string]string, key string) color.NRGBA {
	value, _ := parseColor(table[key])
	return value
}

func editionColor(path string) (color.NRGBA, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return color.NRGBA{}, false
	}
	var id, idLike string
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "ID":
			id = osReleaseValue(value)
		case "ID_LIKE":
			idLike = osReleaseValue(value)
		}
	}
	if value, ok := edition(strings.ToLower(id)); ok {
		return value, true
	}
	for like := range strings.FieldsSeq(strings.ToLower(idLike)) {
		if value, ok := edition(like); ok {
			return value, true
		}
	}
	return color.NRGBA{}, false
}

func osReleaseValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 2 {
		return value
	}
	if value[0] == '"' && value[len(value)-1] == '"' {
		if decoded, err := strconv.Unquote(value); err == nil {
			return decoded
		}
	}
	if value[0] == '\'' && value[len(value)-1] == '\'' {
		return value[1 : len(value)-1]
	}
	return value
}

func edition(id string) (color.NRGBA, bool) {
	value, ok := tokens.Edition[id]
	if !ok {
		return color.NRGBA{}, false
	}
	return parseColor(value)
}
