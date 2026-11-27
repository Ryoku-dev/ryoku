package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

var nineRoleKeys = []string{
	"primary", "primaryText", "surface", "surfaceText", "surfaceVariant",
	"surfaceContainer", "background", "outline", "tertiary",
}

var nineToColorKey = map[string]string{
	"primary":          "primary",
	"primaryText":      "onPrimary",
	"surface":          "surface",
	"surfaceText":      "onSurface",
	"surfaceVariant":   "surfaceVariant",
	"surfaceContainer": "surfaceContainer",
	"background":       "background",
	"outline":          "outline",
	"tertiary":         "tertiary",
}

// Fire-and-forget so a slow or absent shell never blocks the caller.
func shellTheme(name string) {
	if name == "" {
		return
	}
	go func() { _ = exec.Command("ryoku-shell", "theme", name).Run() }()
}

// shellColours writes the palette family through the shell's own verb. The
// setting store is the sole writer of shell.json, so we never touch that file
// from here. A value already in place is a no-op, which keeps the two colour
// controls from writing each other in a loop.
func shellColours(value string) {
	if value == "" {
		return
	}
	if cur := shellColoursName(); cur == value {
		return
	}
	_ = exec.Command("ryoku-shell", "colours", value).Run()
}

func shellColoursName() string {
	b, err := os.ReadFile(filepath.Join(ryokuConfigDir(), "shell.json"))
	if err != nil {
		return ""
	}
	var s struct {
		Theme struct {
			Colours string `json:"colours"`
		} `json:"theme"`
	}
	if json.Unmarshal(b, &s) != nil {
		return ""
	}
	return s.Theme.Colours
}

func setThemeFollow(follow bool) {
	path := filepath.Join(ryokuConfigDir(), "theme.json")
	doc := map[string]interface{}{}
	loadJSON(path, &doc)
	if cur, ok := doc["followWallpaper"].(bool); ok && cur == follow {
		return
	}
	doc["followWallpaper"] = follow
	saveJSON(path, doc)
}

func shellThemeName() string {
	b, err := os.ReadFile(filepath.Join(ryokuConfigDir(), "shell.json"))
	if err != nil {
		return ""
	}
	var s struct {
		Theme struct {
			Theme string `json:"theme"`
		} `json:"theme"`
	}
	if json.Unmarshal(b, &s) != nil {
		return ""
	}
	return s.Theme.Theme
}

// Defaults on, matching the shell's own default.
func followWallpaperOn() bool {
	b, err := os.ReadFile(filepath.Join(ryokuConfigDir(), "theme.json"))
	if err != nil {
		return true
	}
	var s struct {
		FollowWallpaper *bool `json:"followWallpaper"`
	}
	if json.Unmarshal(b, &s) != nil || s.FollowWallpaper == nil {
		return true
	}
	return *s.FollowWallpaper
}

func themeColorsPath() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		base = filepath.Join(home(), ".cache")
	}
	return filepath.Join(base, "ryoku", "colors.json")
}

func themesDir() string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		base = filepath.Join(home(), ".local", "share")
	}
	return filepath.Join(base, "ryoku", "themes")
}

// Atomic so a watcher never reads a half-written scheme.
func themeWriteFile(path string, v interface{}) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func themeHexRGB(hex string) (r, g, b uint8, ok bool) {
	s := strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(s) != 6 {
		return 0, 0, 0, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return uint8(v >> 16), uint8(v >> 8), uint8(v), true
}

func themeIsHex(s string) bool {
	_, _, _, ok := themeHexRGB(s)
	return ok
}

func themeContrastInk(hex string) string {
	r, g, b, ok := themeHexRGB(hex)
	if !ok {
		return "#000000"
	}
	luma := (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 255.0
	if luma > 0.5 {
		return "#000000"
	}
	return "#ffffff"
}

func themeMixHex(a, b string, t float64) string {
	ar, ag, ab, aok := themeHexRGB(a)
	br, bg, bb, bok := themeHexRGB(b)
	if !aok {
		return b
	}
	if !bok {
		return a
	}
	mix := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t) }
	return fmt.Sprintf("#%02x%02x%02x", mix(ar, br), mix(ag, bg), mix(ab, bb))
}

func firstHex(vals ...string) string {
	for _, v := range vals {
		if themeIsHex(v) {
			return v
		}
	}
	return ""
}

func nineFromColors(c map[string]string) map[string]string {
	out := make(map[string]string, len(nineRoleKeys))
	for _, role := range nineRoleKeys {
		out[role] = firstHex(c[nineToColorKey[role]])
	}
	if !themeIsHex(out["surfaceContainer"]) {
		out["surfaceContainer"] = firstHex(c["surfaceContainerHigh"], c["surface"])
	}
	if !themeIsHex(out["background"]) {
		out["background"] = firstHex(c["surface"])
	}
	if !themeIsHex(out["tertiary"]) {
		out["tertiary"] = firstHex(c["secondary"], c["primary"])
	}
	return out
}

func nineFromSw(sw []string) map[string]string {
	if len(sw) < 7 {
		return nil
	}
	surface, onSurface, primary := sw[0], sw[1], sw[2]
	tertiary, outline := sw[4], sw[6]
	return map[string]string{
		"primary":          primary,
		"primaryText":      themeContrastInk(primary),
		"surface":          surface,
		"surfaceText":      onSurface,
		"surfaceVariant":   themeMixHex(surface, onSurface, 0.16),
		"surfaceContainer": themeMixHex(surface, onSurface, 0.08),
		"background":       surface,
		"outline":          outline,
		"tertiary":         tertiary,
	}
}

// Only the custom-seed path uses this: it starts from a colour, not an image.
func parseSeedPalette(out []byte, mode string) (map[string]string, error) {
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
	order := []string{mode, "default", "dark", "light"}
	pal := map[string]string{}
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

func (d *daemon) buildCustomSeedTheme(seeds string) (string, error) {
	seed := ""
	for _, part := range strings.Split(seeds, ",") {
		if themeIsHex(part) {
			seed = "#" + strings.TrimPrefix(strings.TrimSpace(part), "#")
			break
		}
	}
	if seed == "" {
		return "", fmt.Errorf("no valid hex seed in %q", seeds)
	}
	scheme := d.settingString("matugen.schemeType")
	if scheme == "" {
		scheme = "scheme-tonal-spot"
	}
	dark, err := matugenSeedRoles(seed, scheme, "dark")
	if err != nil {
		return "", err
	}
	light, err := matugenSeedRoles(seed, scheme, "light")
	if err != nil {
		return "", err
	}
	if err := saveThemeScheme("custom", "Custom seeds", dark, light); err != nil {
		return "", err
	}
	return "custom", nil
}

func matugenSeedRoles(seed, scheme, mode string) (map[string]string, error) {
	args := []string{"color", "hex", seed, "-t", scheme, "-m", mode, "--json", "hex", "--dry-run"}
	out, err := exec.Command("matugen", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("matugen color hex (%s): %w", mode, err)
	}
	pal, err := parseSeedPalette(out, mode)
	if err != nil {
		return nil, err
	}
	nine := map[string]string{
		"primary":          pal["primary"],
		"primaryText":      pal["on_primary"],
		"surface":          pal["surface"],
		"surfaceText":      pal["on_surface"],
		"surfaceVariant":   pal["surface_variant"],
		"surfaceContainer": firstHex(pal["surface_container"], pal["surface"]),
		"background":       firstHex(pal["background"], pal["surface"]),
		"outline":          pal["outline"],
		"tertiary":         firstHex(pal["tertiary"], pal["secondary"], pal["primary"]),
	}
	return nine, nil
}
