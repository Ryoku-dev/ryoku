package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"syscall"
)

//go:embed schemes/light.json schemes/dark.json schemes/mono.json
var schemesFS embed.FS

// loadScheme returns a curated fixed palette (light or dark) baked into the
// binary: the desktop's "set it light / set it dark" presets.
func loadScheme(mode string) (map[string]string, error) {
	b, err := schemesFS.ReadFile("schemes/" + mode + ".json")
	if err != nil {
		return nil, err
	}
	var m map[string]string
	return m, json.Unmarshal(b, &m)
}

// writePalette authors the shell's own palette (the cache colors.json every
// Quickshell singleton reads) and hands that same palette to matugen, which
// renders every external app config from it.
func writePalette(pal map[string]string) {
	_ = os.MkdirAll(ryokuCacheDir(), 0o755)
	_ = atomicWrite(filepath.Join(ryokuCacheDir(), "colors.json"), mustJSON(pal), 0o644)
	renderApps(pal)
}

// renderApps runs matugen in json (templating-only) mode over the palette, the
// one engine that fans it into the app configs from the templates deployed
// under ~/.config/matugen. config.toml is the core surface (kitty, Hyprland
// borders, btop, Qt) and always renders; apps.toml is the GTK / GUI-app reach,
// rendered only when "Theme apps" is on (else the GTK stylesheets are blanked
// so those apps fall back to stock). Passthrough keeps every colour byte-exact;
// only .hex resolves in this mode, so Qt's ARGB roles read the pre-formatted
// *_argb keys the carrier carries beside the plain colours.
func renderApps(pal map[string]string) {
	cfg := loadMatugenConfig()
	renderActiveTemplates(cfg, pal)
}

func runMatugen(cfg, carrier string) {
	if out, err := exec.Command("matugen", "-c", cfg, "json", carrier).CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "matugen: %v: %s\n", err, out)
	}
}

// themeAppsOn reports whether the palette should reach GTK / GUI apps. A theme
// state without the key (an older theme.json) reads as on, so existing installs
// keep the themed apps they already had.
func themeAppsOn(s themeState) bool { return s.ThemeApps == nil || *s.ThemeApps }

// gtkThemeChoice reports the GTK base-theme choice. Absent (an older theme.json)
// reads as "adw", the libadwaita-consistent GTK3 theme that follows the palette.
func gtkThemeChoice(s themeState) string {
	if s.GtkTheme == "" {
		return "adw"
	}
	return s.GtkTheme
}

// gnomeAccentOn reports whether the GNOME named accent tracks the palette. A
// theme state without the key (an older theme.json) reads as on.
func gnomeAccentOn(s themeState) bool { return s.GnomeAccent == nil || *s.GnomeAccent }

// gtkOff is written to the generated GTK stylesheets when app theming is off, so
// GTK / libadwaita apps drop the Ryoku palette and use their own stock colours.
const gtkOff = "/* Ryoku: app theming is off; apps use their own colours. */\n"

func blankGtk() {
	for _, rel := range []string{"gtk-3.0/gtk.css", "gtk-4.0/gtk.css"} {
		p := filepath.Join(configHome(), rel)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = atomicWrite(p, []byte(gtkOff), 0o644)
	}
}

// currentScheme reports the active palette mode for the UI: light/dark when a
// curated preset is locked, follow when colours track the wallpaper, custom when
// a theme owns its own fixed palette.
func currentScheme() string {
	st := loadThemeState()
	if st.FollowWallpaper {
		return "follow"
	}
	if st.Scheme == "light" || st.Scheme == "dark" || st.Scheme == "mono" {
		return st.Scheme
	}
	return "custom"
}

// selectShellTheme sets shell.json theme.theme through the daemon, the one
// writer of that file. theme.theme is the colour master: the daemon derives
// theme.json's followWallpaper from it on every load and patch, so a scheme
// change that only wrote theme.json (what this file used to do) was undone the
// next time the daemon synced, and picking Wallpaper again in the shell was a
// no-op because theme.theme already said so -- the desktop stuck on the wrong
// palette. Best-effort: with no daemon (a TTY, a box mid-update) the caller's
// theme.json write below still persists the choice and the daemon syncs at
// its next load.
func selectShellTheme(name string) {
	_ = exec.Command("ryoku-shell", "theme", name).Run()
}

// applyScheme sets the desktop palette mode. follow re-derives from the current
// wallpaper (the reset); light/dark lock a curated preset that survives wallpaper
// changes (themePaletteLocked keeps it). Reused by the Appearance control.
func applyScheme(mode string) error {
	// Mono and the Ryoku-default brand palette are retired as fixed looks: the
	// desktop follows the wallpaper by default now, so a mono request lands on
	// the follow path instead of pinning a palette that never tracks the wall.
	if mode == "mono" {
		mode = "follow"
	}
	switch mode {
	case "follow":
		selectShellTheme("Wallpaper")
		st := loadThemeState()
		st.Scheme = ""
		st.FollowWallpaper = true
		if err := saveThemeState(st); err != nil {
			return err
		}
		// borders read the master: regen so they follow the wallpaper again.
		// Best-effort: a box with no provider still persists the choice above.
		_, _ = desktopClient().Apply(desktopStorePath())
		// the daemon derives (honouring the per-image tune); no re-animation.
		// Explicit, not left to the theme patch: when theme.theme already read
		// Wallpaper the patch is a no-op and nothing else would repaint.
		_ = exec.Command("ryogami", "wallpaper", "repaint").Run()
	case "light", "dark":
		pal, err := loadScheme(mode)
		if err != nil {
			return err
		}
		// Default is the shell's compiled base palette (the MONO card); the
		// curated light/dark presets lock the apps and idle the wallpaper
		// pipeline the same way. Anything but Wallpaper keeps followWallpaper
		// off across daemon restarts.
		selectShellTheme("Default")
		st := loadThemeState()
		st.Scheme = mode
		st.FollowWallpaper = false
		if err := saveThemeState(st); err != nil {
			return err
		}
		// borders read the master: regen so the fixed border colours pin now.
		_, _ = desktopClient().Apply(desktopStorePath())
		writePalette(pal)
		// The desktop's GTK settings (colour-scheme preference, the theme name
		// for this mode, the accent) are the daemon's to write: it owns the
		// resolver, and a second writer here would drift from it the moment the
		// GTK theme preference changes. A curated scheme idles the paint worker,
		// so ask explicitly rather than waiting for a repaint that never comes.
		gtkMode := "dark"
		if mode == "light" {
			gtkMode = "light"
		}
		_ = exec.Command("ryoku-shell", "gtk", "apply", gtkMode).Run()
	default:
		return fmt.Errorf("unknown scheme %q (want follow|light|dark)", mode)
	}
	reloadDesktop()
	_ = exec.Command("pkill", "-USR1", "-x", "kitty").Run()
	return nil
}

// currentThemeApps reports the app-theming toggle for the UI.
func currentThemeApps() bool { return themeAppsOn(loadThemeState()) }

// currentGtkTheme reports the GTK base-theme choice for the UI.
func currentGtkTheme() string { return gtkThemeChoice(loadThemeState()) }

// currentGnomeAccent reports the GNOME-accent sync toggle for the UI.
func currentGnomeAccent() bool { return gnomeAccentOn(loadThemeState()) }

// applyThemeApps sets whether the palette reaches GTK / GUI apps and re-fans the
// live palette at once, so the toggle takes hold without a wallpaper change or a
// scheme flip. renderApps honours the new flag (renders the GTK templates, or
// blanks them). An already-open GTK app keeps the colours it started with: it
// re-reads neither the stylesheet nor the theme name on this session, so the
// toggle shows up in the apps you open next.
func applyThemeApps(on bool) error {
	st := loadThemeState()
	st.ThemeApps = &on
	if err := saveThemeState(st); err != nil {
		return err
	}
	if pal := readPalette(filepath.Join(ryokuCacheDir(), "colors.json")); pal != nil {
		renderApps(pal)
	} else if !on {
		blankGtk()
	}
	repaintPalette()
	return nil
}

// applyGtkTheme records the GTK base-theme choice and asks the daemon to
// re-apply. The daemon owns gtk-theme (C4): a repaint re-runs the palette
// pipeline, which resolves the choice into a gtk-theme name for the current mode
// (adw -> adw-gtk3[-dark], adwaita -> Adwaita[-dark], system -> left untouched),
// writes it and nudges running apps. The hub writes no gsettings itself, and the
// generated stylesheets are unchanged by the choice, so there is nothing to
// re-render here.
func applyGtkTheme(mode string) error {
	switch mode {
	case "adw", "adwaita", "system":
	default:
		return fmt.Errorf("unknown gtk theme %q (want adw|adwaita|system)", mode)
	}
	st := loadThemeState()
	st.GtkTheme = mode
	if err := saveThemeState(st); err != nil {
		return err
	}
	repaintPalette()
	return nil
}

// applyGnomeAccent records whether the desktop accent-color tracks the palette
// and asks the daemon to re-apply. The daemon owns accent-color (C4): on a
// repaint it reads the palette primary and, when this is on, writes the nearest
// named GNOME accent so apps reading the system setting follow along.
func applyGnomeAccent(on bool) error {
	st := loadThemeState()
	st.GnomeAccent = &on
	if err := saveThemeState(st); err != nil {
		return err
	}
	repaintPalette()
	return nil
}

// repaintPalette asks the daemon to re-run the palette pipeline in place, with
// no re-animation: the single seam that re-resolves and writes the toolkit
// settings the daemon owns (gtk-theme, color-scheme, accent-color) from
// theme.json. Best-effort, like applyScheme's own repaint -- the persisted
// theme.json is the durable truth, and a box with no live daemon picks the
// choice up at the next login; the setters lean on this only for the live nudge.
func repaintPalette() {
	_ = exec.Command("ryogami", "wallpaper", "repaint").Run()
}

// themeState persists the palette master: whether colours follow the wallpaper
// and, when they don't, which curated scheme is locked. Lives at
// ~/.config/ryoku/theme.json.
type themeState struct {
	FollowWallpaper bool   `json:"followWallpaper"`
	Scheme          string `json:"scheme"`
	ThemeApps       *bool  `json:"themeApps,omitempty"`
	// GtkTheme is the GTK base-theme choice: "adw" (default; the
	// libadwaita-consistent GTK3 theme that follows the palette), "adwaita" (the
	// stock GNOME look) or "system" (Ryoku never writes gtk-theme). Absent reads
	// as "adw".
	GtkTheme string `json:"gtkTheme,omitempty"`
	// GnomeAccent, when on (the default), syncs org.gnome.desktop.interface
	// accent-color to the nearest named accent so Flatpak and GNOME apps that
	// read the system setting follow the palette too. Absent reads as on.
	GnomeAccent *bool `json:"gnomeAccent,omitempty"`
}

func themeStatePath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "ryoku", "theme.json")
}

// loadThemeState defaults to following the wallpaper on a missing or blank file
// (the shipped default look); an existing file (a user who locked a scheme or
// turned follow off) wins.
func loadThemeState() themeState {
	s := themeState{FollowWallpaper: true, Scheme: "mono"}
	if b, err := os.ReadFile(themeStatePath()); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	return s
}

// the theme.json fields Ryogami owns; the hub sets them through `ryogami settings set`
var themeSettingKeys = map[string]string{
	"themeApps":   "theme.themeApps",
	"gtkTheme":    "theme.gtkTheme",
	"gnomeAccent": "theme.gnomeAccent",
}

func saveThemeState(s themeState) error {
	var fields map[string]any
	if err := json.Unmarshal(mustJSON(s), &fields); err != nil {
		return err
	}
	return writeThemeFields(fields)
}

// writeThemeFields lays the hub's fields into theme.json and hands the daemon's to it
// in one batch, so every key has one writer; a nil field is dropped or reset
func writeThemeFields(fields map[string]any) error {
	var before fileSnapshot
	var daemon map[string]any
	err := withThemeLock(func() (err error) {
		if before, err = snapshotFiles(themeStatePath()); err != nil {
			return err
		}
		daemon, err = writeHubThemeFields(fields)
		return err
	})
	if err != nil {
		return err
	}
	if len(daemon) == 0 {
		return nil
	}
	// after the lock is released: the daemon takes the same lock for its own write
	if err := riceRun("ryogami", "settings", "set", string(mustJSON(daemon))); err != nil {
		// a refused batch must not leave the hub's half of it in theme.json
		_ = withThemeLock(before.restore)
		return fmt.Errorf("theme settings: %w", err)
	}
	return nil
}

// withThemeLock runs fn under the lock the daemon takes for theme.json
func withThemeLock(fn func() error) error {
	path := themeStatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	return fn()
}

// writeHubThemeFields writes the hub's fields into theme.json and returns the daemon's changes; the caller holds withThemeLock
func writeHubThemeFields(fields map[string]any) (map[string]any, error) {
	path := themeStatePath()
	cur := readJSONMap(path)
	daemon := map[string]any{}
	for field, v := range fields {
		if key, owned := themeSettingKeys[field]; owned {
			if !reflect.DeepEqual(cur[field], v) {
				daemon[key] = v
			}
			continue
		}
		if v == nil {
			delete(cur, field)
		} else {
			cur[field] = v
		}
	}
	return daemon, atomicWrite(path, mustJSON(cur), 0o644)
}

func ryokuCacheDir() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".cache")
	}
	return filepath.Join(base, "ryoku")
}

func kittyThemePath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "kitty", "current-theme.conf")
}

func cacheHome() string {
	if b := os.Getenv("XDG_CACHE_HOME"); b != "" {
		return b
	}
	return filepath.Join(os.Getenv("HOME"), ".cache")
}

func mustJSON(v any) []byte {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return []byte("{}")
	}
	return b
}

// applyRyokuTheme resets the desktop to the Ryoku signature in one move: the
// frame-bars style, square corners everywhere, Geologica type across the
// shell and apps, and the grainy-mono palette. Wired to the Appearance and Rices pages.
func applyRyokuTheme() error {
	// Merge so sizing, weather and preserved sidebar content choices survive.
	frameBars, ok := readJSONMap(shellStorePath())["frameBars"].(map[string]any)
	if !ok {
		frameBars = map[string]any{}
	}
	frameBars["style"] = "ryoku-frame"
	// The terminal family is an independent user choice and is intentionally
	// absent from this UI-theme patch.
	mergeShellJSON(map[string]any{
		"frameBars":   frameBars,
		"roundness":   0,
		"frameRadius": 0,
		"osdRadius":   0,
		"fontFamily":  "Geologica",
	})
	// square window corners: pin the appearance override the provider reads.
	_ = withDesktopLock(func() error {
		ns := readJSONMap(desktopStorePath())
		childMap(childMap(ns, "desktop"), "appearance")["rounding"] = 0
		return atomicWrite(desktopStorePath(), mustJSON(ns), 0o644)
	})
	// GTK type now; the Hyprland autostart pins it on the next login.
	_ = exec.Command("gsettings", "set", "org.gnome.desktop.interface", "font-name", "Geologica 11").Run()
	// clear any active-rice marker: the signature is a fresh look, not a rice,
	// so the Rices page must not keep showing the last rice as applied.
	setActiveRice("")
	// the Ryoku mark: the 力 glyph, no custom logo, tinted to the accent, so the
	// signature brand reads as Ryoku (the desktop name is left as the user set it).
	mergeBrandJSON(map[string]any{"markText": "力", "markImage": "", "markTint": true})
	// colours follow the wallpaper (mono is retired) + regen the border lua,
	// reload hypr and kitty.
	return applyScheme("follow")
}

// mergeShellJSON overlays keys onto shell.json, mergeBrandJSON onto brand.json;
// both preserve every key already present, and the shell hot-reloads on write.
func mergeShellJSON(keys map[string]any) { mergeStore("shell.json", keys) }
func mergeBrandJSON(keys map[string]any) { mergeStore("brand.json", keys) }
func mergeStore(name string, keys map[string]any) {
	p := filepath.Join(configHome(), "ryoku", name)
	m := map[string]any{}
	if b, err := os.ReadFile(p); err == nil {
		_ = json.Unmarshal(b, &m)
	}
	for k, v := range keys {
		m[k] = v
	}
	_ = atomicWrite(p, mustJSON(m), 0o644)
}
