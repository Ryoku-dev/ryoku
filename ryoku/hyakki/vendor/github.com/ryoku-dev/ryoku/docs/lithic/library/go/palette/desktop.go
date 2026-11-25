package palette

import (
	"image/color"
	"maps"
	"os"
	"path/filepath"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/design"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Colours is the desktop's choice of colours (shell.json theme.colours).
type Colours uint8

const (
	// Ryoku is Ryoku's own colours, the signature, light or dark as the desktop is.
	Ryoku Colours = iota
	// Wallpaper is the desktop's colours: the wallpaper's palette or a named scheme.
	Wallpaper
)

// String returns the name shell.json uses.
func (c Colours) String() string {
	if c == Wallpaper {
		return "wallpaper"
	}
	return "ryoku"
}

// Desktop is the look the Ryoku desktop asks for, resolved as Lithic.qml's _load does
// (Lithic.qml:742-794): the design, the colours, light or dark, contrast and tint, reduced
// motion and the kanji gloss, the palette they come from and the roles they make.
type Desktop struct {
	// Design is shell.json theme.design, or LITHIC_DESIGN when it names a design.
	Design design.Style
	// Colours is shell.json theme.colours. Under LITHIC_MODE it is Wallpaper exactly when
	// the forced mode is Follow.
	Colours Colours
	// Mode is Lithic.mode: Follow on the desktop's colours, otherwise Light or Dark as
	// matugen.json's mode says, and, for smart, sun or no word, as colors.json's surface is.
	Mode Mode
	// Look is shell.json theme.contrast and theme.tint (the zero look under LITHIC_MODE).
	Look Look
	// ReduceMotion is shell.json theme.motion.reduce.
	ReduceMotion bool
	// Gloss is shell.json showJapaneseGloss; true when absent.
	Gloss bool
	// Palette is the palette the roles come from. Under Follow it is the desktop's (a host
	// palette from LITHIC_PALETTE_JSON, else colors.json unless theme.json turns
	// followWallpaper off, with themePalette over it); otherwise, and when that is empty, it
	// is the signature, carrying only the edition accent from os-release.
	Palette Palette
	// Roles are the roles Design wears now, RolesFor(Design).
	Roles Roles
}

// Option changes how ReadDesktop and WatchDesktop read the desktop.
type Option func(*readConfig)

type readConfig struct {
	env       bool
	config    string
	cache     string
	osRelease string
	// armed, when set, runs once WatchDesktop's watches are in place (tests)
	armed func()
}

// WithEnv turns the LITHIC_MODE, LITHIC_DESIGN and LITHIC_PALETTE_JSON overrides on or off.
// They are on by default, as in Lithic.qml.
func WithEnv(on bool) Option {
	return func(c *readConfig) { c.env = on }
}

// WithDirs reads the files from other places: config and cache stand for XDG_CONFIG_HOME
// and XDG_CACHE_HOME (the files sit in their ryoku directories) and osRelease for
// /etc/os-release. An empty value keeps the default.
func WithDirs(config, cache, osRelease string) Option {
	return func(c *readConfig) {
		if config != "" {
			c.config = config
		}
		if cache != "" {
			c.cache = cache
		}
		if osRelease != "" {
			c.osRelease = osRelease
		}
	}
}

func newReadConfig(opts []Option) readConfig {
	c := readConfig{env: true}
	for _, opt := range opts {
		opt(&c)
	}
	if c.config == "" {
		c.config = configHome()
	}
	if c.cache == "" {
		c.cache = cacheHome()
	}
	if c.osRelease == "" {
		c.osRelease = osReleasePath
	}
	return c
}

func (c readConfig) getenv(name string) string {
	if !c.env {
		return ""
	}
	return os.Getenv(name)
}

func (c readConfig) shellPath() string   { return filepath.Join(c.config, "ryoku", "shell.json") }
func (c readConfig) themePath() string   { return filepath.Join(c.config, "ryoku", "theme.json") }
func (c readConfig) matugenPath() string { return filepath.Join(c.config, "ryoku", "matugen.json") }
func (c readConfig) colorsPath() string  { return filepath.Join(c.cache, "ryoku", "colors.json") }

// targets are the files a change of which can change the Desktop.
func (c readConfig) targets() []string {
	return []string{c.shellPath(), c.themePath(), c.matugenPath(), c.colorsPath()}
}

// ReadDesktop reads the desktop's look once: shell.json, theme.json and matugen.json from
// the ryoku config directory, colors.json from the ryoku cache directory and os-release.
// Missing or invalid files read as Lithic.qml reads them: Lithic, Ryoku's colours, dark
// unless the palette's surface is light, standard contrast, no tint, motion on, gloss on.
func ReadDesktop(opts ...Option) Desktop {
	return readDesktop(newReadConfig(opts))
}

func readDesktop(c readConfig) Desktop {
	shell := readObject(c.shellPath())
	look, _ := shell["theme"].(map[string]any)
	wallpaper := colorRoles(readObject(c.colorsPath()))

	d := Desktop{Gloss: true}
	if name, ok := look["design"].(string); ok {
		d.Design, _ = design.Parse(name)
	}
	if forced, ok := design.Parse(c.getenv("LITHIC_DESIGN")); ok {
		d.Design = forced
	}
	if motion, ok := look["motion"].(map[string]any); ok {
		d.ReduceMotion, _ = motion["reduce"].(bool)
	}
	if gloss, ok := shell["showJapaneseGloss"].(bool); ok {
		d.Gloss = gloss
	}

	if forced, ok := validMode(c.getenv("LITHIC_MODE")); ok {
		d.Mode = forced
		if forced == Follow {
			d.Colours = Wallpaper
		}
	} else {
		tone := ""
		if knobs := readObject(c.matugenPath()); knobs != nil {
			if value, _ := knobs["mode"].(string); value == "dark" || value == "light" {
				tone = value
			}
		}
		surface, hasSurface := wallpaper["surface"]
		light := tone == "light" || (tone == "" && hasSurface && luma(surface) > 0.5)
		switch {
		case look["colours"] == "wallpaper":
			d.Mode, d.Colours = Follow, Wallpaper
		case light:
			d.Mode = Light
		default:
			d.Mode = Dark
		}
		contrast, _ := look["contrast"].(string)
		tint, _ := look["tint"].(string)
		d.Look = ParseLook(contrast, tint)
	}

	roles := make(map[string]color.NRGBA)
	source := "signature"
	if d.Mode == Follow {
		if host := hostRoles(c.getenv("LITHIC_PALETTE_JSON")); len(host) != 0 {
			roles, source = host, "host"
		} else {
			source = desktopRoles(roles, readObject(c.themePath()), wallpaper, shell)
		}
	}
	if source == "signature" {
		if accent, ok := editionColor(c.osRelease); ok {
			roles[tokens.PaletteAccents["gold"]] = accent
		}
	}
	dark := d.Mode != Light
	if surface, ok := roles["surface"]; ok && d.Mode == Follow {
		dark = luma(surface) <= 0.5
	}
	d.Palette = Palette{Roles: roles, Dark: dark, Source: source}
	d.Roles = d.RolesFor(d.Design)
	return d
}

// RolesFor is Lithic.rolesFor (Lithic.qml:723-736): the roles style s would wear now, for
// the style in use or a preview of another. Ryoku's colours are the signature in every
// style; the desktop's colours are the toned ladder in Lithic and the palette's own
// Material roles (Material) in the round styles.
func (d Desktop) RolesFor(s design.Style) Roles {
	if _, ok := d.Palette.Roles["surface"]; ok && s.Profile().Desktop && d.Mode == Follow {
		return Material(d.Palette, d.Look)
	}
	return rolesFrom(d.Palette, d.Look, d.Palette.Source != "signature")
}

// Equal reports whether two readings ask for the same look.
func (d Desktop) Equal(o Desktop) bool {
	return d.Design == o.Design && d.Colours == o.Colours && d.Mode == o.Mode && d.Look == o.Look &&
		d.ReduceMotion == o.ReduceMotion && d.Gloss == o.Gloss &&
		equalPalette(d.Palette, o.Palette) && d.Roles == o.Roles
}

// hostRoles reads LITHIC_PALETTE_JSON's file as Lithic.qml does: its themePalette when it
// holds one, else the whole object, keeping only colours.
func hostRoles(path string) map[string]color.NRGBA {
	if path == "" {
		return nil
	}
	object := readObject(path)
	if nested, present := object["themePalette"]; present {
		if nested == nil {
			return nil
		}
		if inner, ok := nested.(map[string]any); ok {
			object = inner
		}
	}
	return colorRoles(object)
}

func validMode(name string) (Mode, bool) {
	switch name {
	case "dark":
		return Dark, true
	case "light":
		return Light, true
	case "follow":
		return Follow, true
	}
	return Dark, false
}

// luma is Lithic.qml's _luma on 0..1 channels.
func luma(c color.NRGBA) float64 {
	return (0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)) / 255
}

func equalPalette(a, b Palette) bool {
	return a.Dark == b.Dark && a.Source == b.Source && maps.Equal(a.Roles, b.Roles)
}
