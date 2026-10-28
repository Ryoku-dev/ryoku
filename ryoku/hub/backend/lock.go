package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ryoku-hub lock manages installed qylock skins behind Ryoku Settings. Picking
// one swaps the in-session lock preference plus the SDDM greeter theme. Ryostore
// owns upstream browsing and install-only downloads; Hub owns activation.
//
//	ryoku-hub lock list                              installed skins + active one, as JSON
//	ryoku-hub lock set <slug>                        activate an installed skin
//	ryoku-hub lock apply-greeter <slug> <true|false> privileged greeter install
//
// a skin = any folder under the themes dir with a Main.qml. slug = its path
// under that dir (e.g. "clockwork/orbital"), exactly what lock.sh resolves
// against themes_link.

// LockSkin = one selectable skin as the Hub draws it.
type LockSkin struct {
	Slug      string   `json:"slug"`    // path under the themes dir, e.g. "clockwork/orbital"
	Name      string   `json:"name"`    // "Orbital"
	Theme     string   `json:"theme"`   // "Clockwork"
	Summary   string   `json:"summary"` // one line
	Blurb     string   `json:"blurb"`   // a sentence
	Tags      []string `json:"tags"`
	Preview   string   `json:"preview"`   // gif: file://... local, https://... upstream, "" none
	Installed bool     `json:"installed"` // present under qylock themes dir
	Active    bool     `json:"active"`
	SizeKB    int      `json:"sizeKB"` // upstream install weight, 0 when unknown
}

// LockResponse is the installed-only `ryoku-hub lock list` payload.
type LockResponse struct {
	Active string     `json:"active"`
	Skins  []LockSkin `json:"skins"`
	// Error is non-empty when the skins list could not be read (e.g. the themes
	// dir exists but can't be scanned). It lets the page tell "couldn't read the
	// list" from "nothing installed" instead of showing one message for both.
	Error string `json:"error,omitempty"`
}

// lockCurated: hand-written copy for the skins Ryoku ships. anything not in
// here falls back to folder name + metadata.desktop so a stray hand-dropped
// qylock theme still shows up with something readable.
var lockCurated = map[string]LockSkin{
	"clockwork/orbital": {
		Name: "Orbital", Theme: "Clockwork", Tags: []string{"Clockwork"},
		Summary: "An orbital clock that winds up to unlock",
		Blurb:   "Concentric minute and second rings sweep past a bold hour readout; typing your key winds the mechanism before it springs open.",
	},
	"clockwork/tape": {
		Name: "Tape", Theme: "Clockwork", Tags: []string{"Clockwork"},
		Summary: "Time on warm, scrolling film reels",
		Blurb:   "Hours, minutes, and seconds roll past on sprocketed tape behind an amber readout beam, like a frame of film paused at now.",
	},
}

func xdgHome(env, sub string) string {
	if base := os.Getenv(env); base != "" {
		return base
	}
	return filepath.Join(os.Getenv("HOME"), sub)
}

func qylockThemesDir() string {
	return filepath.Join(xdgHome("XDG_DATA_HOME", ".local/share"), "qylock", "themes")
}

func qylockThemePref() string {
	return filepath.Join(xdgHome("XDG_CONFIG_HOME", ".config"), "qylock", "theme")
}

// runLock = `ryoku-hub lock <sub> [arg]` dispatch.
func runLock(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("lock needs list|set|apply-greeter")
	}
	switch args[0] {
	case "list":
		return printJSON(listLockSkins())
	case "set":
		if len(args) < 2 {
			return fmt.Errorf("lock set needs a slug")
		}
		return setLockSkin(args[1])
	case "apply-greeter":
		if len(args) < 3 {
			return fmt.Errorf("lock apply-greeter needs a slug and true or false")
		}
		clock24h, err := strconv.ParseBool(args[2])
		if err != nil {
			return fmt.Errorf("invalid clock format %q", args[2])
		}
		return applyGreeter(args[1], clock24h)
	default:
		return fmt.Errorf("lock needs list|set|apply-greeter")
	}
}

func listLockSkins() LockResponse {
	return listLockSkinsIn(qylockThemesDir(), readLockPref(qylockThemePref()), lockReceiptsDir())
}

// readLockPref: active slug (trimmed). missing file -> "".
func readLockPref(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func clockFormat24h() bool {
	b, err := os.ReadFile(filepath.Join(xdgHome("XDG_CONFIG_HOME", ".config"), "ryoku", "shell.json"))
	if err != nil {
		return false
	}
	var settings struct {
		General struct {
			ClockFormat24H bool `json:"clock_format_24_h"`
		} `json:"general"`
	}
	return json.Unmarshal(b, &settings) == nil && settings.General.ClockFormat24H
}

// listLockSkinsIn scans dir for installed skins, unions in any store product the
// folder scan missed but whose receipt in receiptsDir points at a Main.qml that
// is present, marks whichever slug matches active, and reports a read failure so
// the page can tell "couldn't read the list" from "nothing installed". Split out
// from listLockSkins so a temp tree (and an empty receiptsDir) can drive it from
// tests.
func listLockSkinsIn(dir, active, receiptsDir string) LockResponse {
	slugs, scanErr := scanLockSlugs(dir)
	seen := make(map[string]bool, len(slugs))
	for _, slug := range slugs {
		seen[slug] = true
	}
	// Second source: store receipts. The scan's folder heuristic can miss a
	// receipt-owned tree (installed as/under a symlink, or nested deeper than it
	// walks); the receipt is the authoritative record that the product is
	// installed, so union it in when its Main.qml is actually on disk.
	for _, slug := range receiptLockSlugs(dir, receiptsDir) {
		if !seen[slug] {
			seen[slug] = true
			slugs = append(slugs, slug)
		}
	}
	skins := make([]LockSkin, 0, len(slugs))
	for _, slug := range slugs {
		s := lockSkinFor(dir, slug)
		s.Active = slug == active
		skins = append(skins, s)
	}
	sort.Slice(skins, func(i, j int) bool {
		if skins[i].Theme != skins[j].Theme {
			return skins[i].Theme < skins[j].Theme
		}
		return skins[i].Name < skins[j].Name
	})
	resp := LockResponse{Active: active, Skins: skins}
	if scanErr != nil {
		resp.Error = scanErr.Error()
	}
	return resp
}

// scanLockSlugs walks two levels deep (theme, theme/variant) for any folder that
// has a Main.qml, returning the slug (path under dir). A theme installed as a
// symlink to a directory is followed like a real folder: os.ReadDir yields a
// symlink entry whose IsDir() is false, so without this it would be dropped. A
// non-nil error means dir itself could not be read for a reason other than "does
// not exist"; the caller surfaces that as a listing failure, not an empty tree.
func scanLockSlugs(dir string) ([]string, error) {
	tops, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var slugs []string
	for _, t := range tops {
		if !entryIsDir(dir, t) {
			continue
		}
		top := filepath.Join(dir, t.Name())
		if fileExists(filepath.Join(top, "Main.qml")) {
			slugs = append(slugs, t.Name())
			continue
		}
		subs, _ := os.ReadDir(top)
		for _, s := range subs {
			if entryIsDir(top, s) && fileExists(filepath.Join(top, s.Name(), "Main.qml")) {
				slugs = append(slugs, t.Name()+"/"+s.Name())
			}
		}
	}
	return slugs, nil
}

// entryIsDir reports whether e in parent is a directory, following a symlink to
// its target. os.ReadDir returns a symlinked dir with IsDir()==false, so a theme
// dir that is a symlink (or lives under one) would be skipped without this.
func entryIsDir(parent string, e os.DirEntry) bool {
	if e.IsDir() {
		return true
	}
	if e.Type()&os.ModeSymlink == 0 {
		return false
	}
	info, err := os.Stat(filepath.Join(parent, e.Name()))
	return err == nil && info.IsDir()
}

// lockReceiptsDir: where RyoStore records installed lockscreen products, one
// JSON receipt per product. Resolved from the Hub's XDG_STATE_HOME, matching the
// store's storeStateDir() (stateHome/ryoku/store/<category>).
func lockReceiptsDir() string {
	return filepath.Join(xdgHome("XDG_STATE_HOME", ".local/state"), "ryoku", "store", "lockscreens")
}

// lockReceipt is the slice of a RyoStore receipt the Hub needs to locate an
// installed lockscreen: the destination under the data dir and the files laid
// down, so the one that is Main.qml pins the slug.
type lockReceipt struct {
	Destination string `json:"destination"`
	Files       []struct {
		Destination string `json:"destination"`
	} `json:"files"`
}

// receiptLockSlugs reads the store's lockscreen receipts under receiptsDir and
// returns, for each whose Main.qml is present under dir, the slug lock.sh
// resolves (the directory holding Main.qml, relative to the themes dir). A
// receipt is proof the product was installed; unioning it with the folder scan
// keeps a receipt-owned skin listed even when the scan's heuristic misses it, as
// long as its files are still on disk where the Hub can read them.
func receiptLockSlugs(dir, receiptsDir string) []string {
	if receiptsDir == "" {
		return nil
	}
	entries, err := os.ReadDir(receiptsDir)
	if err != nil {
		return nil
	}
	var slugs []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		slug := receiptLockSlug(filepath.Join(receiptsDir, e.Name()))
		if slug != "" && fileExists(filepath.Join(dir, filepath.FromSlash(slug), "Main.qml")) {
			slugs = append(slugs, slug)
		}
	}
	return slugs
}

// receiptLockSlug turns one receipt file into the slug of the skin it installed,
// or "" if it is not a themes-dir lockscreen. The slug is the directory holding
// Main.qml, taken relative to <data>/qylock/themes.
func receiptLockSlug(receiptPath string) string {
	b, err := os.ReadFile(receiptPath)
	if err != nil {
		return ""
	}
	var r lockReceipt
	if err := json.Unmarshal(b, &r); err != nil {
		return ""
	}
	const prefix = "qylock/themes/"
	base := filepath.ToSlash(r.Destination)
	if !strings.HasPrefix(base, prefix) {
		return ""
	}
	base = strings.TrimPrefix(base, prefix)
	if base == "" {
		return ""
	}
	for _, f := range r.Files {
		d := filepath.ToSlash(f.Destination)
		if d == "Main.qml" {
			return base
		}
		if sub, ok := strings.CutSuffix(d, "/Main.qml"); ok {
			return base + "/" + sub
		}
	}
	return base
}

func lockSkinFor(dir, slug string) LockSkin {
	s := lockSkinMeta(dir, slug)
	s.Installed = true
	// Shipped skins carry preview.gif at the skin root; a RyoStore download lands
	// it under assets/ (its product manifest maps the preview to assets/preview.gif),
	// so a downloaded theme showed no preview until we look there too.
	for _, rel := range [][]string{{"preview.gif"}, {"assets", "preview.gif"}} {
		if p := filepath.Join(append([]string{dir, slug}, rel...)...); fileExists(p) {
			s.Preview = "file://" + p
			break
		}
	}
	return s
}

// lockSkinMeta fills the display copy: curated map first, else a name+tag
// guess from the slug plus, for an installed skin, the summary from
// metadata.desktop. doesn't touch Preview / Installed / Active.
func lockSkinMeta(dir, slug string) LockSkin {
	if c, ok := lockCurated[slug]; ok {
		return LockSkin{Slug: slug, Name: c.Name, Theme: c.Theme, Summary: c.Summary, Blurb: c.Blurb, Tags: c.Tags}
	}
	s := LockSkin{Slug: slug, Name: lockSkinName(slug), Tags: lockSkinTags(slug)}
	if len(s.Tags) > 0 {
		s.Theme = s.Tags[0]
	}
	s.Summary = lockDesktopDescription(filepath.Join(dir, slug, "metadata.desktop"))
	return s
}

// lockSkinName: slug -> display name. take the leaf, swap separators for
// spaces, capitalise each word ("pixel-coffee" -> "Pixel Coffee").
func lockSkinName(slug string) string {
	leaf := slug
	if i := strings.LastIndex(slug, "/"); i >= 0 {
		leaf = slug[i+1:]
	}
	words := strings.Fields(strings.NewReplacer("-", " ", "_", " ").Replace(leaf))
	for i, w := range words {
		words[i] = titleWord(w)
	}
	return strings.Join(words, " ")
}

// lockSkinTags groups the obvious qylock families so siblings share a label.
func lockSkinTags(slug string) []string {
	switch {
	case strings.HasPrefix(slug, "clockwork"):
		return []string{"Clockwork"}
	case strings.HasPrefix(slug, "pixel-"):
		return []string{"Pixel"}
	case strings.HasPrefix(slug, "R1999"):
		return []string{"Reverse 1999"}
	}
	return nil
}

func setLockSkin(slug string) error {
	dir := qylockThemesDir()
	if !fileExists(filepath.Join(dir, slug, "Main.qml")) {
		return fmt.Errorf("unknown lock skin: %s", slug)
	}
	// The session lock is the user's own file and needs no privilege, so it is
	// written first. Escalating first meant a cancelled admin prompt threw the
	// whole pick away: the page snapped back to the old skin even though the
	// in-session half could always have honoured it.
	if err := setLockSkinIn(dir, qylockThemePref(), slug); err != nil {
		return err
	}
	if err := escalateGreeter(slug, clockFormat24h()); err != nil {
		return fmt.Errorf("lock skin set for this session; the sign-in screen keeps its old one: %w", err)
	}
	return nil
}

// setLockSkinIn writes slug to the active-lock pref file. an unknown slug is
// rejected here so a typo can't quietly disable the lock.
func setLockSkinIn(dir, pref, slug string) error {
	if !fileExists(filepath.Join(dir, slug, "Main.qml")) {
		return fmt.Errorf("unknown lock skin: %s", slug)
	}
	if err := os.MkdirAll(filepath.Dir(pref), 0o755); err != nil {
		return err
	}
	return atomicWrite(pref, []byte(slug+"\n"), 0o644)
}

const greeterTheme = "ryoku"

func greeterThemeInstaller() string {
	if v := os.Getenv("RYOKU_GREETER_THEME_INSTALLER"); v != "" {
		return v
	}
	return "/usr/share/ryoku/lockscreen/qylock/sddm-host/install-greeter-theme"
}

func sddmThemesDir() string {
	if v := os.Getenv("RYOKU_SDDM_THEMES_DIR"); v != "" {
		return v
	}
	return "/usr/share/sddm/themes"
}

func sddmConfPath() string {
	if v := os.Getenv("RYOKU_SDDM_CONF"); v != "" {
		return v
	}
	return "/etc/sddm.conf.d/99-ryoku.conf"
}

// validSlug: kill empty / absolute / traversing slugs before they hit a
// filesystem path or a privileged copy.
func validSlug(slug string) error {
	if slug == "" || strings.HasPrefix(slug, "/") || strings.Contains(slug, "..") {
		return fmt.Errorf("invalid skin slug: %q", slug)
	}
	return nil
}

// escalateGreeter re-execs this binary under pkexec to install the skin as the
// SDDM greeter (it has to write /usr/share/sddm + /etc). pkexec pops the
// graphical polkit prompt; cancel -> error.
func escalateGreeter(slug string, clock24h bool) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command("pkexec", self, "lock", "apply-greeter", slug, strconv.FormatBool(clock24h))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sign-in screen needs admin authentication: %w", err)
	}
	return nil
}

// applyGreeter = the privileged half, run as root by pkexec. installs the
// skin the invoking user picked as the greeter. source resolves from the
// invoking user's home, never from a caller-supplied path. don't change that.
func applyGreeter(slug string, clock24h bool) error {
	src := os.Getenv("RYOKU_QYLOCK_THEMES")
	if src == "" {
		src = invokingUserThemes()
	}
	return installGreeter(src, sddmThemesDir(), sddmConfPath(), slug, clock24h)
}

// invokingUserThemes: qylock themes dir of whoever invoked us via pkexec
// (PKEXEC_UID) or sudo (SUDO_UID), so root reads the right home.
func invokingUserThemes() string {
	uid := os.Getenv("PKEXEC_UID")
	if uid == "" {
		uid = os.Getenv("SUDO_UID")
	}
	if uid != "" {
		if u, err := user.LookupId(uid); err == nil {
			return filepath.Join(u.HomeDir, ".local", "share", "qylock", "themes")
		}
	}
	return qylockThemesDir()
}

// installGreeter installs srcThemes/slug beneath the shared SDDM host and
// points the greeter config at it. The host owns cursor fallback, focus and the
// global clock preference, so optional skins receive the same login-screen
// behavior as the built-in skin.
func installGreeter(srcThemes, themesDir, confPath, slug string, clock24h bool) error {
	if err := validSlug(slug); err != nil {
		return err
	}
	src := filepath.Join(srcThemes, slug)
	if !fileExists(filepath.Join(src, "Main.qml")) {
		return fmt.Errorf("not an installed skin: %s", slug)
	}
	dst := filepath.Join(themesDir, greeterTheme)
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		return err
	}
	if out, err := exec.Command(
		greeterThemeInstaller(), src, dst, strconv.FormatBool(clock24h),
	).CombinedOutput(); err != nil {
		return fmt.Errorf("install greeter theme: %v: %s", err, out)
	}
	if err := os.MkdirAll(filepath.Dir(confPath), 0o755); err != nil {
		return err
	}
	return atomicWrite(confPath, []byte("[Theme]\nCurrent="+greeterTheme+"\n"), 0o644)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// titleWord: upper-case the first rune. "orbital" -> "Orbital".
func titleWord(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// lockDesktopDescription: pull Description= from a freedesktop-style file.
// best effort, any error -> "".
func lockDesktopDescription(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "Description="); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
