package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type matugenConfig struct {
	Mode             string          `json:"mode"`             // "dark", "light", "smart"
	Contrast         float64         `json:"contrast"`         // -1.0 to 1.0
	LightnessDark    float64         `json:"lightnessDark"`    // -1.0 to 1.0
	LightnessLight   float64         `json:"lightnessLight"`   // -1.0 to 1.0
	Prefer           string          `json:"prefer"`           // "dominant" or "vibrant"
	SchemeType       string          `json:"schemeType"`       // matugen -t variant
	SourceColorIndex int             `json:"sourceColorIndex"` // 0..4
	ThemeRyokuApps   bool            `json:"themeRyokuApps"`   // theme Ryoku's native shell & apps
	Templates        map[string]bool `json:"templates"`        // app -> bool
}

func hexToRGB(hex string) (int, int, int) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == 6 {
		r, _ := strconv.ParseInt(hex[0:2], 16, 64)
		g, _ := strconv.ParseInt(hex[2:4], 16, 64)
		b, _ := strconv.ParseInt(hex[4:6], 16, 64)
		return int(r), int(g), int(b)
	}
	return 0, 0, 0
}

// light or dark from the surface luma, for templates that pick a default look
func paletteMode(pal map[string]string) string {
	bg := pal["surface"]
	if bg == "" {
		bg = pal["background"]
	}
	r, g, b := hexToRGB(bg)
	if 0.299*float64(r)+0.587*float64(g)+0.114*float64(b) >= 127.5 {
		return "light"
	}
	return "dark"
}

func paletteCarrier(pal map[string]string) map[string]any {
	c := map[string]any{}
	put := func(name, hex string) {
		if hex == "" {
			return
		}
		stripped := strings.TrimPrefix(hex, "#")
		r, g, b := hexToRGB(hex)
		colorObj := map[string]any{
			"hex":          hex,
			"hex_stripped": stripped,
			"red":          strconv.Itoa(r),
			"green":        strconv.Itoa(g),
			"blue":         strconv.Itoa(b),
			"rgb":          fmt.Sprintf("%d, %d, %d", r, g, b),
		}
		entry := map[string]any{
			"default":      colorObj,
			"dark":         colorObj,
			"light":        colorObj,
			"hex":          hex,
			"hex_stripped": stripped,
			"red":          strconv.Itoa(r),
			"green":        strconv.Itoa(g),
			"blue":         strconv.Itoa(b),
			"rgb":          fmt.Sprintf("%d, %d, %d", r, g, b),
		}
		c[name] = entry
	}
	for k, v := range pal {
		put(k, v)
		put(k+"_argb", "#ff"+strings.TrimPrefix(v, "#"))
	}
	// A palette may carry only the base16 slots (the light/dark/mono fixed schemes
	// and rice-locked palettes), but the app templates (discord, steam, zed,
	// ghostty, obs, telegram, heroic, micro, cava, papirus) speak Material 3 role
	// names. Map base16 -> M3 roles so those templates resolve -- otherwise one
	// unresolved role aborts the whole render and every app (Hyprland borders,
	// GTK/Nautilus, kitty, ...) freezes on the last theme. Only fill roles the
	// palette did not already define, so a real M3 palette is never overwritten.
	pick := func(keys ...string) string {
		for _, k := range keys {
			if v := pal[k]; v != "" {
				return v
			}
		}
		return ""
	}
	accent := pick("color4", "foreground")
	bg := pick("background", "color0")
	fg := pick("foreground", "color15", "color7")
	dim := pick("color0", "background")
	muted := pick("color8", "color0")
	subtle := pick("color7", "foreground")
	roles := map[string]string{
		"primary": accent, "on_primary": bg, "primary_container": accent,
		"on_primary_container": fg, "primary_fixed": accent, "primary_fixed_dim": accent,
		"inverse_primary": accent,
		"secondary":       pick("color6", "color4"), "on_secondary": bg,
		"secondary_container": dim, "on_secondary_container": fg,
		"secondary_fixed": pick("color6", "color4"), "secondary_fixed_dim": pick("color6", "color4"),
		"tertiary": pick("color5", "color2"), "on_tertiary": bg,
		"tertiary_container": dim, "on_tertiary_container": fg,
		"on_primary_fixed": bg, "on_primary_fixed_variant": bg,
		"on_secondary_fixed": bg, "on_secondary_fixed_variant": bg,
		"tertiary_fixed": pick("color5", "color2"), "tertiary_fixed_dim": pick("color5", "color2"),
		"on_tertiary_fixed": bg, "on_tertiary_fixed_variant": bg, "surface_tint": accent,
		"surface": bg, "on_surface": fg, "on_surface_variant": subtle, "on_background": fg,
		"surface_dim": dim, "surface_bright": muted, "surface_variant": dim,
		"surface_container": dim, "surface_container_lowest": bg, "surface_container_low": dim,
		"surface_container_high": muted, "surface_container_highest": muted,
		"error": pick("color1", "color9"), "on_error": bg,
		"error_container": pick("color1", "color9"), "on_error_container": fg,
		"outline": muted, "outline_variant": dim,
		"inverse_surface": fg, "inverse_on_surface": bg,
		"scrim": "#000000", "shadow": "#000000",
	}
	for name, hex := range roles {
		if _, ok := c[name]; ok || hex == "" {
			continue
		}
		put(name, hex)
		put(name+"_argb", "#ff"+strings.TrimPrefix(hex, "#"))
	}
	if fg, ok := pal["foreground"]; ok {
		put("cursor", fg)
	} else if onSurf, ok := pal["on_surface"]; ok {
		put("cursor", onSurf)
	}
	return c
}

func defaultMatugenConfig() matugenConfig {
	return matugenConfig{
		Mode:             "smart",
		Contrast:         0.0,
		LightnessDark:    0.0,
		LightnessLight:   0.0,
		Prefer:           "saturation",
		SchemeType:       "scheme-tonal-spot",
		SourceColorIndex: 0,
		ThemeRyokuApps:   true,
		Templates: map[string]bool{
			"btop":      true,
			"qt":        true,
			"gtk":       true,
			"discord":   true,
			"obs":       true,
			"zed":       true,
			"heroic":    true,
			"hyprland":  true,
			"telegram":  true,
			"steam":     true,
			"kitty":     true,
			"cava":      true,
			"fish":      true,
			"yazi":      true,
			"ghostty":   true,
			"micro":     true,
			"papirus":   true,
			"obsidian":  true,
			"zathura":   true,
			"alacritty": true,
			"tmux":      true,
			"sidra":     true,
			"ryotunes":  true,
		},
	}
}

func matugenConfigPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "ryoku", "matugen.json")
}

func steamThemeReady() bool {
	home := os.Getenv("HOME")
	if home == "" {
		return false
	}

	info, err := os.Stat(filepath.Join(home, ".steam", "steam", "steamui"))
	return err == nil && info.IsDir()
}

func loadMatugenConfig() matugenConfig {
	cfg := defaultMatugenConfig()
	b, err := os.ReadFile(matugenConfigPath())
	if err == nil {
		_ = json.Unmarshal(b, &cfg)
	}
	if cfg.Templates == nil {
		cfg.Templates = defaultMatugenConfig().Templates
	}
	return cfg
}

func saveMatugenConfig(cfg matugenConfig) error {
	return atomicWrite(matugenConfigPath(), mustJSON(cfg), 0o644)
}

func runMatugenCmd(args []string) error {
	if len(args) == 0 {
		return printJSON(loadMatugenConfig())
	}
	switch args[0] {
	case "get":
		return printJSON(loadMatugenConfig())
	case "set":
		if len(args) < 2 {
			return fmt.Errorf("matugen set requires JSON argument")
		}
		// Merge onto what is stored, never onto a zero struct: the caller sends the
		// knobs its page owns, and a key it omits (themeRyokuApps, the roster) would
		// otherwise be written as Go's zero value and silently turn app theming off.
		cfg := loadMatugenConfig()
		if err := json.Unmarshal([]byte(args[1]), &cfg); err != nil {
			return err
		}
		// Persist knobs only. The shell daemon watches matugen.json and is the
		// sole renderer of the palette, so it retints on this
		// write; rendering here would fight it and, for matugen, author a
		// divergent colors.json.
		return saveMatugenConfig(cfg)
	case "render-apps":
		pal := readPalette(filepath.Join(ryokuCacheDir(), "colors.json"))
		if pal != nil {
			cfg := loadMatugenConfig()
			renderActiveTemplates(cfg, pal)
		}
		return nil
	default:
		return fmt.Errorf("unknown matugen subcommand %q", args[0])
	}
}

// renderActiveTemplates generates matugen configs for enabled templates
func renderActiveTemplates(cfg matugenConfig, pal map[string]string) {
	matugenDir := filepath.Join(configHome(), "matugen")
	cacheDir := filepath.Join(cacheHome(), "ryoku")
	_ = os.MkdirAll(cacheDir, 0o755)

	// Ensure all target output directories exist
	home := os.Getenv("HOME")
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(home, ".local", "share")
	}

	targetDirs := []string{
		filepath.Join(configHome(), "kitty"),
		filepath.Join(configHome(), "qt6ct", "colors"),
		filepath.Join(configHome(), "qt5ct", "colors"),
		filepath.Join(configHome(), "gtk-3.0"),
		filepath.Join(configHome(), "vesktop", "themes"),
		filepath.Join(configHome(), "equibop", "themes"),
		filepath.Join(configHome(), "obs-studio", "themes"),
		filepath.Join(configHome(), "zed", "themes"),
		filepath.Join(configHome(), "heroic", "store", "styles"),
		filepath.Join(dataHome, "TelegramDesktop", "tdata"),
		filepath.Join(configHome(), "cava"),
		filepath.Join(configHome(), "fish", "conf.d"),
		filepath.Join(configHome(), "yazi"),
		filepath.Join(configHome(), "ghostty"),
		filepath.Join(configHome(), "micro", "colorschemes"),
		filepath.Join(cacheHome(), "matugen"),
	}
	for _, d := range targetDirs {
		_ = os.MkdirAll(d, 0o755)
	}
	if steamThemeReady() {
		_ = os.MkdirAll(filepath.Join(home, ".steam", "steam", "steamui", "skins", "Material-Theme", "css", "main", "colors"), 0o755)
	}

	carrierPath := filepath.Join(cacheDir, "matugen-carrier.json")
	carrier := map[string]any{"colors": paletteCarrier(pal), "mode": paletteMode(pal)}
	if err := atomicWrite(carrierPath, mustJSON(carrier), 0o644); err != nil {
		return
	}
	// Always render core surface
	runMatugen(filepath.Join(matugenDir, "config.toml"), carrierPath)

	// Filter apps.toml entries based on cfg.Templates
	appsTomlPath := filepath.Join(matugenDir, "apps.toml")
	b, err := os.ReadFile(appsTomlPath)
	if err != nil {
		return
	}

	// Write filtered apps.toml to cache and execute
	content := string(b)
	sections := strings.Split(content, "\n[")
	var activeSections []string

	for i, sec := range sections {
		secStr := sec
		if i > 0 {
			secStr = "[" + sec
		}
		trimmed := strings.TrimSpace(secStr)
		if strings.HasPrefix(trimmed, "[config]") || strings.HasPrefix(trimmed, "[config\n") {
			activeSections = append(activeSections, secStr)
			continue
		}
		if strings.HasPrefix(trimmed, "[templates.") {
			headerEnd := strings.Index(trimmed, "]")
			if headerEnd > 11 {
				appName := trimmed[11:headerEnd]
				groupKey := appName
				switch appName {
				case "gtk3", "gtk4":
					groupKey = "gtk"
				case "vesktop", "equibop":
					groupKey = "discord"
				}
				if appName == "steam" && !steamThemeReady() {
					continue
				}
				if enabled, ok := cfg.Templates[groupKey]; !ok || enabled {
					activeSections = append(activeSections, secStr)
				}
			}
		}
	}

	activeAppsToml := filepath.Join(cacheDir, "active-apps.toml")
	_ = os.WriteFile(activeAppsToml, []byte(strings.Join(activeSections, "\n")), 0o644)

	runMatugen(activeAppsToml, carrierPath)
}
