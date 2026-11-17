package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The input, cursor and clipboard blocks: the keyboard, pointer and touchpad
// leaves niri's input node covers, the pointer-focus and mod-key niri exclusives,
// plus the cursor theme and the primary-selection toggle.

func writeInput(b *strings.Builder, in Input, n Niri) {
	b.WriteString("input {\n")
	b.WriteString("    keyboard {\n")
	if in.KbLayout != "" || in.KbVariant != "" || in.KbOptions != "" {
		b.WriteString("        xkb {\n")
		if in.KbLayout != "" {
			fmt.Fprintf(b, "            layout %s\n", kdlStr(in.KbLayout))
		}
		if in.KbVariant != "" {
			fmt.Fprintf(b, "            variant %s\n", kdlStr(in.KbVariant))
		}
		if in.KbOptions != "" {
			fmt.Fprintf(b, "            options %s\n", kdlStr(in.KbOptions))
		}
		b.WriteString("        }\n")
	}
	fmt.Fprintf(b, "        repeat-rate %d\n", in.RepeatRate)
	fmt.Fprintf(b, "        repeat-delay %d\n", in.RepeatDelay)
	if in.NumlockByDefault {
		b.WriteString("        numlock\n")
	}
	b.WriteString("    }\n")

	b.WriteString("    touchpad {\n")
	if touchpadDisabled() {
		b.WriteString("        off\n")
	}
	if in.TapToClick {
		b.WriteString("        tap\n")
	}
	if in.NaturalScroll {
		b.WriteString("        natural-scroll\n")
	}
	if in.TouchScrollFactor > 0 && in.TouchScrollFactor != 1 {
		fmt.Fprintf(b, "        scroll-factor %s\n", kdlNum(in.TouchScrollFactor))
	}
	if in.DisableWhileTyping {
		b.WriteString("        dwt\n")
	}
	if in.TapAndDrag {
		b.WriteString("        drag true\n")
	}
	if in.Clickfinger {
		b.WriteString("        click-method \"clickfinger\"\n")
	}
	if in.MiddleEmulation {
		b.WriteString("        middle-emulation\n")
	}
	if in.LeftHanded {
		b.WriteString("        left-handed\n")
	}
	if in.Sensitivity != 0 {
		fmt.Fprintf(b, "        accel-speed %s\n", kdlNum(in.Sensitivity))
	}
	if p := accelProfile(in.AccelProfile); p != "" {
		fmt.Fprintf(b, "        accel-profile %s\n", kdlStr(p))
	}
	b.WriteString("    }\n")

	b.WriteString("    mouse {\n")
	if in.MouseNaturalScroll {
		b.WriteString("        natural-scroll\n")
	}
	if in.MouseScrollFactor > 0 && in.MouseScrollFactor != 1 {
		fmt.Fprintf(b, "        scroll-factor %s\n", kdlNum(in.MouseScrollFactor))
	}
	if in.LeftHanded {
		b.WriteString("        left-handed\n")
	}
	if in.Sensitivity != 0 {
		fmt.Fprintf(b, "        accel-speed %s\n", kdlNum(in.Sensitivity))
	}
	if p := accelProfile(in.AccelProfile); p != "" {
		fmt.Fprintf(b, "        accel-profile %s\n", kdlStr(p))
	}
	b.WriteString("    }\n")

	// niri already sends scrolling to the window under the pointer, so only mode 1 needs the line
	if in.FollowMouse == 1 {
		if n.FocusFollowScroll >= 0 {
			fmt.Fprintf(b, "    focus-follows-mouse max-scroll-amount=%s\n", kdlStr(fmt.Sprintf("%d%%", n.FocusFollowScroll)))
		} else {
			b.WriteString("    focus-follows-mouse\n")
		}
	}
	if m := warpMode(n.WarpMouseToFocus); m != "skip" {
		if m == "" {
			b.WriteString("    warp-mouse-to-focus\n")
		} else {
			fmt.Fprintf(b, "    warp-mouse-to-focus mode=%s\n", kdlStr(m))
		}
	}
	if n.WorkspaceBackForth {
		b.WriteString("    workspace-auto-back-and-forth\n")
	}
	if n.DisablePowerKey {
		b.WriteString("    disable-power-key-handling\n")
	}
	// Super is niri's own default, so only a changed mod key needs a line.
	if k := modKey(n.ModKey); k != "" && k != "Super" {
		fmt.Fprintf(b, "    mod-key %s\n", kdlStr(k))
	}
	if k := modKey(n.ModKeyNested); k != "" && k != "Super" {
		fmt.Fprintf(b, "    mod-key-nested %s\n", kdlStr(k))
	}
	b.WriteString("}\n\n")
}

// accelProfile maps the neutral profile name onto niri's two accepted values;
// anything else omits the line.
func accelProfile(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "flat":
		return "flat"
	case "adaptive":
		return "adaptive"
	}
	return ""
}

// warpMode maps the stored warp-mouse-to-focus choice onto niri's node. "skip"
// means emit nothing (the feature stays off); "" means the bare node (niri's
// separate-axis centering); the two center words carry a mode property.
func warpMode(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "separate", "on":
		return ""
	case "center-xy":
		return "center-xy"
	case "center-xy-always":
		return "center-xy-always"
	}
	return "skip"
}

// modKey guards the stored mod key against the four niri accepts here; an unknown
// value omits the line so niri keeps its own Super default.
func modKey(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "super":
		return "Super"
	case "alt":
		return "Alt"
	case "ctrl", "control":
		return "Ctrl"
	case "shift":
		return "Shift"
	}
	return ""
}

// writeClipboard turns off niri's primary-selection buffer when the user has
// disabled middle-click paste, the neutral input toggle both providers share.
// niri keeps primary selection unless told otherwise, so the block only appears
// to switch it off.
func writeClipboard(b *strings.Builder, in Input) {
	if in.MiddleClickPaste {
		return
	}
	b.WriteString("clipboard {\n    disable-primary\n}\n\n")
}

func writeCursor(b *strings.Builder, c Cursor) {
	theme := c.Theme
	if alias := cursorReassertTheme(); alias != "" {
		theme = alias
	}
	b.WriteString("cursor {\n")
	if theme != "" {
		fmt.Fprintf(b, "    xcursor-theme %s\n", kdlStr(theme))
	}
	if c.Size > 0 {
		fmt.Fprintf(b, "    xcursor-size %d\n", c.Size)
	}
	if c.InactiveTimeout > 0 {
		fmt.Fprintf(b, "    hide-after-inactive-ms %d\n", c.InactiveTimeout*1000)
	}
	if c.HideOnKeyPress {
		b.WriteString("    hide-when-typing\n")
	}
	b.WriteString("}\n\n")
}

// The cursor re-assert forces niri to re-read recoloured images without ever
// changing the rendered size. niri reloads the cursor only when the config's
// cursor block changes value, and a recolour keeps the theme name, so an
// identical tree is a no-op. Nudging xcursor-size would change the rendered
// size (niri loads size x output scale, so at a fractional scale a neighbouring
// nominal size resolves to a different image), which the user sees as the
// pointer growing/shrinking on every palette change.
//
// Instead the theme NAME alternates between the real theme and a persistent
// mirror directory holding the same cursors. Both directories always fully
// exist, so niri never fails to resolve the named theme and never falls back to
// its built-in 64px cursor (the size jump a transient symlink alias caused while
// it was created/removed mid-reload). The size is untouched, and both themes
// carry the same images, so the alternation is invisible. The state file records
// which theme the config currently names; writeCursor emits it over the store
// theme, and each re-assert flips it after refreshing the mirror.
func cursorReassertThemePath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	return filepath.Join(dir, "ryoku", "cursor-reassert-theme")
}

func writeCursorReassertTheme(name string) error {
	return atomicWrite(cursorReassertThemePath(), []byte(name), 0o644)
}

func clearCursorReassertTheme() error {
	err := os.Remove(cursorReassertThemePath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// cursorReassertTheme reads the theme the config currently names; empty means
// the store theme (no re-assert has flipped it yet).
func cursorReassertTheme() string {
	b, err := os.ReadFile(cursorReassertThemePath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// cursorIconsDir is the user icon root the recolour tool installs into, which
// shadows /usr/share/icons in the xcursor search order.
func cursorIconsDir() string {
	return filepath.Join(os.Getenv("HOME"), ".local", "share", "icons")
}

// cursorMirrorName is the persistent second theme directory the re-assert
// alternates with. It is a real copy, never a symlink and never removed, so the
// config can always name a fully-present theme.
func cursorMirrorName(theme string) string { return theme + "-ryoku-alt" }

// syncCursorMirror copies the real theme's cursors into the mirror directory so
// both name the same recoloured images. The mirror persists across re-asserts;
// only its contents are refreshed. A copy (not a symlink) means niri resolves it
// even mid-recolour, when the real theme's cursors/ is briefly being replaced.
func syncCursorMirror(theme string) error {
	src := filepath.Join(cursorIconsDir(), theme)
	dst := filepath.Join(cursorIconsDir(), cursorMirrorName(theme))
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, part := range []string{"index.theme", "cursors"} {
		s := filepath.Join(src, part)
		d := filepath.Join(dst, part)
		fi, err := os.Stat(s)
		if err != nil {
			continue
		}
		if fi.IsDir() {
			if err := os.RemoveAll(d); err != nil {
				return err
			}
			if err := copyDir(s, d); err != nil {
				return err
			}
		} else {
			b, err := os.ReadFile(s)
			if err != nil {
				return err
			}
			if err := os.WriteFile(d, b, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			_ = os.Remove(target)
			return os.Symlink(link, target)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, info.Mode().Perm())
	})
}
