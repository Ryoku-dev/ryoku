// Package keyboard owns the keyboard layout across every screen that asks for
// one. A layout has to land in four places and the desktop only owns the last:
//
//	initramfs   a copy of /etc/vconsole.conf   the LUKS passphrase prompt
//	X11 config  00-keyboard.conf               the SDDM greeter, so the first login
//	console     /etc/vconsole.conf KEYMAP      the TTYs
//	hypr.json   input.kbLayout                 the desktop, once you are logged in
//
// Two directions matter. Adopting reads the layout the installer was told about
// into the desktop, so a box installed on AZERTY does not come up on QWERTY.
// Applying pushes a chosen layout back out, so the greeter, the TTYs and the
// boot prompt follow the desktop instead of staying on whatever was there.
//
// The initramfs is the one that traps people. mkinitcpio's sd-vconsole hook does
// `add_file /etc/vconsole.conf` at BUILD time, so the passphrase prompt is
// frozen to the keymap current when the image was last generated. Editing
// /etc/vconsole.conf afterwards, by hand or through localectl, cannot reach it.
package keyboard

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"
	i18n "ryoku-i18n"
)

// A keyboard cannot be asked what it is. USB and HID report scancodes and a
// product name, never the legends printed on the keys, so an AZERTY board and a
// QWERTY board are the same device to the kernel, and a compositor's per-device
// layout is only the layout already configured. What the machine does know is
// what the person told the installer, so detection reads that back.

// consoleToXkb maps the console keymap names that differ from their xkb layout
// code. Anything absent passes through, which covers the many that agree
// already (fr, de, es, it, pt, ru...).
var consoleToXkb = map[string]string{
	"uk": "gb", "gb": "gb",
	"fr-latin1": "fr", "fr-pc": "fr", "azerty": "fr",
	"be-latin1": "be",
	"de-latin1": "de", "de_CH-latin1": "ch", "sg": "ch", "sf": "ch",
	"es-cp850": "es", "la-latin1": "latam",
	"it2":       "it",
	"pt-latin1": "pt", "br-abnt2": "br",
	"no-latin1": "no", "nb": "no", "fi-latin1": "fi", "se-latin1": "se",
	"dk-latin1": "dk", "is-latin1": "is",
	"pl2": "pl", "cz-lat2": "cz", "sk-qwertz": "sk",
	"hu101": "hu", "croat": "hr", "slovene": "si",
	"trq": "tr", "trf": "tr",
	"ua-utf": "ua", "ruwin_alt-UTF-8": "ru",
	"gr": "gr", "il": "il",
	"jp106": "jp", "kr": "kr",
	"us-acentos": "us", "dvorak": "us", "colemak": "us",
}

// localeToXkb maps a locale country to the layout its keyboards ship with. Only
// the unambiguous ones: a country whose boards are commonly QWERTY (IE, NL, MT,
// every English locale) is left out so it falls through to us.
var localeToXkb = map[string]string{
	"FR": "fr", "BE": "be", "LU": "fr",
	"DE": "de", "AT": "de", "CH": "ch",
	"ES": "es", "IT": "it", "PT": "pt", "BR": "br",
	"RU": "ru", "UA": "ua", "PL": "pl", "CZ": "cz", "SK": "sk",
	"HU": "hu", "HR": "hr", "SI": "si", "RS": "rs", "BG": "bg",
	"RO": "ro", "GR": "gr", "TR": "tr", "IL": "il",
	"SE": "se", "NO": "no", "DK": "dk", "FI": "fi", "IS": "is", "EE": "ee",
	"LV": "lv", "LT": "lt", "JP": "jp", "KR": "kr",
	"GB": "gb", "MA": "fr", "DZ": "fr", "TN": "fr", "SN": "fr", "CI": "fr",
}

// Detected is one answer plus where it came from, so a report can say why.
type Detected struct {
	Layout string
	Source string
}

// Detect resolves a layout from what the system records, strongest source
// first. An empty Layout means nothing on the box says anything.
func Detect(x11, console, locale string) Detected {
	// 1. an explicit X11 layout is already an xkb code, set on purpose.
	if v := strings.TrimSpace(x11); v != "" {
		if first := strings.SplitN(v, ",", 2)[0]; first != "" {
			return Detected{Layout: first, Source: i18n.T("the X11 keymap")}
		}
	}
	// 2. the console keymap: what was picked during installation.
	if v := ConsoleAsXkb(console); v != "" {
		return Detected{Layout: v, Source: i18n.T("the console keymap")}
	}
	// 3. the locale's country, the weakest of the three. A locale never
	//    outranks a keymap: typing French on a US board is common.
	if c := localeCountry(locale); c != "" {
		if mapped, ok := localeToXkb[c]; ok {
			return Detected{Layout: mapped, Source: i18n.T("the system locale")}
		}
	}
	return Detected{}
}

// ConsoleAsXkb converts a console keymap name to its xkb layout code, so that
// comparing the console against the desktop does not read uk versus gb as
// drift. Empty when the name says nothing.
func ConsoleAsXkb(console string) string {
	v := strings.TrimSpace(console)
	if v == "" {
		return ""
	}
	if mapped, ok := consoleToXkb[v]; ok {
		return mapped
	}
	// a bare two or three letter keymap usually equals its xkb code
	if len(v) <= 3 && !strings.ContainsAny(v, "-_") {
		return v
	}
	// strip a charset suffix ("fr-latin9" -> "fr") as a last try
	if base := strings.SplitN(v, "-", 2)[0]; base != "" && len(base) <= 3 {
		return base
	}
	return ""
}

// localeCountry pulls the country out of a locale ("fr_FR.UTF-8" -> FR).
func localeCountry(locale string) string {
	v := strings.TrimSpace(locale)
	if v == "" {
		return ""
	}
	if i := strings.IndexAny(v, ".@"); i >= 0 {
		v = v[:i]
	}
	parts := strings.SplitN(v, "_", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.ToUpper(parts[1])
}

// ---- reading the four layers -------------------------------------------------

var x11LayoutRe = regexp.MustCompile(`(?i)Option\s+"XkbLayout"\s+"([^"]*)"`)

// Paths are package vars so tests can use an isolated filesystem.
var (
	X11Path      = "/etc/X11/xorg.conf.d/00-keyboard.conf"
	VconsolePath = "/etc/vconsole.conf"
)

// X11Layout is the greeter's layout, from the X11 keyboard config.
func X11Layout() string {
	b, err := os.ReadFile(X11Path)
	if err != nil {
		return ""
	}
	m := x11LayoutRe.FindSubmatch(b)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(string(m[1]))
}

// ConsoleKeymap is KEYMAP from /etc/vconsole.conf. Reads VconsolePath so the
// freshness check below and this agree on which file is meant.
func ConsoleKeymap() string {
	b, err := os.ReadFile(VconsolePath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "KEYMAP="); ok {
			return strings.Trim(strings.TrimSpace(v), `"`)
		}
	}
	return ""
}

// SystemLocale prefers /etc/locale.conf over the caller's environment, so a run
// from an odd shell still reads what the system was installed as.
func SystemLocale() string {
	if b, err := os.ReadFile("/etc/locale.conf"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if v, ok := strings.CutPrefix(strings.TrimSpace(line), "LANG="); ok {
				return strings.Trim(strings.TrimSpace(v), `"`)
			}
		}
	}
	return os.Getenv("LANG")
}

// BootImageGlobs is a package var so a test can point it at a temp dir.
var BootImageGlobs = []string{"/boot/EFI/Linux/*.efi", "/boot/initramfs-*.img"}

// BootImageTime is the newest non-fallback boot image and its mtime. A fallback
// image is regenerated on its own schedule, so it never speaks for freshness.
func BootImageTime() (time.Time, string) {
	var newest time.Time
	var which string
	for _, g := range BootImageGlobs {
		paths, _ := filepath.Glob(g)
		for _, p := range paths {
			if strings.Contains(p, "fallback") {
				continue
			}
			fi, err := os.Stat(p)
			if err != nil {
				continue
			}
			if fi.ModTime().After(newest) {
				newest, which = fi.ModTime(), p
			}
		}
	}
	return newest, which
}

// BootStale reports whether the boot image predates /etc/vconsole.conf, meaning
// the passphrase prompt still carries the keymap baked in when it was built.
// This is the check a plain config comparison misses: every file under /etc can
// agree while the prompt is still wrong.
func BootStale() (bool, string) {
	when, path := BootImageTime()
	if when.IsZero() {
		return false, ""
	}
	fi, err := os.Stat(VconsolePath)
	if err != nil {
		return false, path
	}
	return fi.ModTime().After(when), path
}

// ---- pushing a layout out ----------------------------------------------------

// Layout is one keyboard choice, in xkb terms.
type Layout struct {
	Layout  string
	Variant string
	Options string
}

// Primary is the first layout of a "fr,us" pair, which is the one a login
// screen and a boot prompt need: they have no way to switch.
func (l Layout) Primary() string {
	return strings.TrimSpace(strings.SplitN(l.Layout, ",", 2)[0])
}

var applySystemKeymap = func(layout, variant, options string) int {
	return host.Default().Keymap([]string{"set", layout, variant, options})
}

// ApplySystem writes the layout to the greeter and console through the host
// seam so the same caller works with localectl and Void's rc.conf.
func ApplySystem(l Layout) error {
	p := l.Primary()
	if p == "" {
		return fmt.Errorf(i18n.T("no layout to apply"))
	}
	if code := applySystemKeymap(p, l.Variant, l.Options); code != host.ExitOK {
		return fmt.Errorf(i18n.T("set the greeter and console keymap: ryoku-host exited %d"), code)
	}
	return nil
}

// RebuildBootImage regenerates the initramfs so the passphrase prompt picks up
// the console keymap. Slow (it rebuilds every preset, and a UKI with it) and it
// regenerates what the machine boots from, so callers decide when it runs.
func RebuildBootImage() error {
	if !sys.Has("mkinitcpio") {
		return fmt.Errorf(i18n.T("mkinitcpio not installed"))
	}
	if err := sys.Sudo("mkinitcpio", "-P"); err != nil {
		return fmt.Errorf(i18n.T("rebuild the boot image: %w"), err)
	}
	return nil
}
