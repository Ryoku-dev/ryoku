// Package i18n is Ryoku's translation lookup for Go: the same catalog, the same
// source-string-as-key contract and the same fallback as ryoku/ui/Singletons/I18n.qml,
// so the installer TUI, the shell installer and the ryoku CLI speak the language
// the desktop speaks.
//
// A developer only ever writes English, wrapped where it is displayed:
//
//	title := i18n.T("Pick a disk")
//	line  := i18n.Tf("formatting %s as btrfs", dev)
//
// An untranslated string returns its English self, so a missing catalog never
// blanks the UI. Programs that ship with no filesystem to read from (the
// standalone shell installer, the ISO's TUI) get the catalog compiled in by
// importing ryoku-i18n/catalog; programs that always run on an installed Ryoku
// (the CLI) read /usr/share/ryoku/i18n and stay small.
package i18n

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// SystemDir is where a packaged Ryoku keeps the catalog. RYOKU_I18N_DIR wins,
// so a dev checkout points at ryoku/i18n/catalog without installing anything.
const SystemDir = "/usr/share/ryoku/i18n"

var (
	mu    sync.RWMutex
	lang  = "en"
	table map[string]string
	rtl   bool
)

// Lang is the active language code ("en", "pt_BR", ...).
func Lang() string {
	mu.RLock()
	defer mu.RUnlock()
	return lang
}

// RTL reports whether the active language is written right to left, so a
// terminal UI can flip its alignment.
func RTL() bool {
	mu.RLock()
	defer mu.RUnlock()
	return rtl
}

// T translates one English source string.
func T(s string) string {
	mu.RLock()
	defer mu.RUnlock()
	if v, ok := table[s]; ok && v != "" {
		return v
	}
	return s
}

// Tf translates a format string, then fills it. The placeholders travel with
// the translation (sync.py drops any translation that mangles them), so the
// arguments land in whatever order the target language needs.
func Tf(format string, a ...any) string {
	return fmt.Sprintf(T(format), a...)
}

// Use activates a language. An empty or "auto" code resolves from the
// environment; an unknown one falls back to English. Returns the code in force.
func Use(code string) string {
	code = Resolve(code)
	m, _ := load(code)
	mu.Lock()
	lang, table, rtl = code, m, IsRTL(code)
	mu.Unlock()
	return code
}

// SetFS points the loader at an embedded catalog (ryoku-i18n/catalog), for a
// binary that must translate itself with no files on disk. Call it before Use.
func SetFS(f fs.FS) {
	mu.Lock()
	embedded = f
	mu.Unlock()
}

var embedded fs.FS

// Resolve turns a requested code, a human name ("Polski"), or "auto" into a
// catalog code, consulting the same places the shell does: the explicit
// request, RYOKU_LANG, the desktop's shell.json, then the POSIX locale.
func Resolve(code string) string {
	if c := match(code); c != "" {
		return c
	}
	if c := match(os.Getenv("RYOKU_LANG")); c != "" {
		return c
	}
	if c := match(shellConfigLanguage()); c != "" {
		return c
	}
	for _, v := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if c := match(localeCode(os.Getenv(v))); c != "" {
			return c
		}
	}
	return "en"
}

// match accepts a code ("pt_BR"), a language name ("Polish"), a native name
// ("Polski") or a bare tag ("pt-br"), and returns the catalog code or "".
func match(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "auto") {
		return ""
	}
	norm := strings.ReplaceAll(s, "-", "_")
	for _, l := range Languages() {
		if strings.EqualFold(l.Code, norm) || strings.EqualFold(l.Name, s) || l.Native == s {
			return l.Code
		}
	}
	// a regional tag with no catalog of its own falls back to its base
	// language, so de_AT reads de.json rather than dropping to English.
	if base, _, ok := strings.Cut(norm, "_"); ok {
		for _, l := range Languages() {
			if strings.EqualFold(l.Code, base) {
				return l.Code
			}
		}
	}
	return ""
}

// localeCode trims a POSIX locale ("pt_BR.UTF-8@euro") to its language tag.
func localeCode(v string) string {
	if i := strings.IndexAny(v, ".@"); i >= 0 {
		v = v[:i]
	}
	if v == "C" || v == "POSIX" {
		return ""
	}
	return v
}

// shellConfigLanguage reads the desktop's chosen language out of shell.json.
// Absent (a live ISO, a foreign distro, the greeter) it returns "".
func shellConfigLanguage() string {
	home := os.Getenv("XDG_CONFIG_HOME")
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		home = filepath.Join(h, ".config")
	}
	b, err := os.ReadFile(filepath.Join(home, "ryoku", "shell.json"))
	if err != nil {
		return ""
	}
	var doc struct {
		Language string `json:"language"`
	}
	if json.Unmarshal(b, &doc) != nil {
		return ""
	}
	return doc.Language
}

// load reads one catalog: the user overlay first (a hand fix or a locally
// generated language beats the shipped file), then the shipped catalog from
// RYOKU_I18N_DIR, the system dir, or the embedded copy.
func load(code string) (map[string]string, error) {
	out := map[string]string{}
	if code == "en" {
		return out, nil // English keys are the strings
	}
	name := code + ".json"
	var last error
	for _, dir := range dirs() {
		if m, err := readCatalog(os.DirFS(dir), name); err == nil {
			mergeInto(out, m)
			break
		} else if !os.IsNotExist(err) {
			last = err
		}
	}
	if len(out) == 0 && embedded != nil {
		if m, err := readCatalog(embedded, name); err == nil {
			mergeInto(out, m)
		} else {
			last = err
		}
	}
	if overlay := userOverlayDir(); overlay != "" {
		if m, err := readCatalog(os.DirFS(overlay), name); err == nil {
			mergeInto(out, m) // the overlay wins
		}
	}
	return out, last
}

func dirs() []string {
	var out []string
	if d := os.Getenv("RYOKU_I18N_DIR"); d != "" {
		out = append(out, d)
	}
	return append(out, SystemDir)
}

func userOverlayDir() string {
	home := os.Getenv("XDG_CONFIG_HOME")
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		home = filepath.Join(h, ".config")
	}
	return filepath.Join(home, "ryoku", "i18n")
}

func readCatalog(f fs.FS, name string) (map[string]string, error) {
	b, err := fs.ReadFile(f, name)
	if err != nil {
		return nil, err
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func mergeInto(dst, src map[string]string) {
	for k, v := range src {
		if v != "" {
			dst[k] = v
		}
	}
}
