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

// FromPalette maps a source palette to the complete Lithic role set.
func FromPalette(p Palette) Roles {
	r := Signature(p.Dark)
	r.Source = p.Source
	if p.Source != "wallpaper" && p.Source != "scheme" {
		accent, ok := p.Roles[tokens.PaletteAccents[tokens.PaletteToned["goldInk"]]]
		if !ok {
			return r
		}
		r.Gold = accent
		r.OnGold = InkOn(accent)
		r.GoldInk = contrastTone(accent, r.Stone2, p.Dark)
		return r
	}

	role := func(name string, fallback color.NRGBA) color.NRGBA {
		if value, ok := p.Roles[name]; ok {
			return value
		}
		return fallback
	}
	neutrals := tokens.PaletteLight
	if p.Dark {
		neutrals = tokens.PaletteDark
	}
	if !p.Forced {
		r.Ground = role(neutrals["ground"], r.Ground)
		r.Stone1 = role(neutrals["stone1"], r.Stone1)
		r.Stone2 = role(neutrals["stone2"], r.Stone2)
		r.Stone3 = role(neutrals["stone3"], r.Stone3)
		r.Stone4 = role(neutrals["stone4"], r.Stone4)
		r.Vein = role(neutrals["vein"], r.Vein)
		r.Fg = role(neutrals["fg"], r.Fg)
		r.Fg2 = role(neutrals["fg2"], r.Fg2)
		r.Fg3 = role(neutrals["fg3"], r.Fg3)
		if scrim, ok := p.Roles[tokens.PaletteAccents["scrim"]]; ok {
			alpha := tokens.AlphaScrimLight
			if p.Dark {
				alpha = tokens.AlphaScrimDark
			}
			r.Scrim = WithAlpha(scrim, alpha)
		}
	}

	r.Gold = role(tokens.PaletteAccents[tokens.PaletteToned["goldInk"]], r.Gold)
	r.OnGold = role(tokens.PaletteAccents["onGold"], r.OnGold)
	r.GoldInk = contrastTone(r.Gold, r.Stone2, p.Dark)
	r.Sel = role(tokens.PaletteAccents["sel"], r.Sel)
	r.OnSel = role(tokens.PaletteAccents["onSel"], r.OnSel)
	r.OnSel2 = Mix(r.OnSel, r.Sel, tokens.AlphaOnSel2)
	r.Jasper = contrastTone(role(tokens.PaletteAccents[tokens.PaletteToned["jasper"]], r.Jasper), r.Stone2, p.Dark)
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
