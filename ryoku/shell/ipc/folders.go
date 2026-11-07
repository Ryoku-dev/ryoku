package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// folders.go tints the folder icons to the wallpaper accent. It builds a small
// icon theme that inherits Papirus-Dark and overrides only the folder icons with
// Papirus's matching colour set (root-free, never touches the packaged theme).
// The accent (argument, else the palette's Material primary with color4
// fallback) picks the nearest Papirus folder colour.
//
// The theme name consumers resolve to is fixed (ryoku-folders); only its content
// changes. Each build lands in a temp dir and is published by renaming a symlink
// over the old one, so a reader (a GTK app, or the shell resolving a folder icon)
// sees either the whole old tree or the whole new one, never a theme half-swapped
// from under it (the SIGABRT a fresh timestamped name per change caused, issue
// #47). The previous generation is kept one extra cycle so an in-flight lookup of
// it still resolves, and gtk-update-icon-cache busts GTK's on-disk cache.
//
// The daemon runs it in process on every palette change (refreshFolderIcons);
// `ryoku-shell folders [accent]` runs it client-side at login.

const (
	folderTheme   = "ryoku-folders"  // fixed theme name consumers resolve to
	folderGensDir = ".ryoku-folders" // hidden container: generations + temp builds
)

// folderIconsPapirus is the packaged theme the colour sets come from.
var folderIconsPapirus = "/usr/share/icons/Papirus"

const folderIndexTheme = `[Icon Theme]
Name=Ryoku Folders
Inherits=Papirus-Dark
Directories=places

[places]
Size=64
MinSize=8
MaxSize=512
Type=Scalable
Context=Places
`

// A palette burst inside the daemon runs one build at a time; the file lock
// below serialises against a login run in another process.
var folderTintMu sync.Mutex

type folderTint struct {
	papirus string // Papirus root
	base    string // the user icons dir the theme is published in
	lock    string
	colors  string // the palette colors.json the accent falls back to
}

func defaultFolderTint() folderTint {
	run := os.Getenv("XDG_RUNTIME_DIR")
	if run == "" {
		run = "/tmp"
	}
	return folderTint{
		papirus: folderIconsPapirus,
		base:    filepath.Join(matugenDataHome(), "icons"),
		lock:    filepath.Join(run, "ryoku-folders.lock"),
		colors:  matugenColorsPath(),
	}
}

// runFolders is `ryoku-shell folders [accent]`.
func runFolders(args []string) int {
	accent := ""
	if len(args) > 0 {
		accent = args[0]
	}
	if _, err := defaultFolderTint().apply(accent, true); err != nil {
		fmt.Fprintln(os.Stderr, "ryoku-shell folders:", err)
		return 1
	}
	return 0
}

// apply builds and publishes the theme for accent ("" reads the palette). It
// reports whether a new generation was published. When the live generation
// already holds the chosen colour nothing is rebuilt; ensureSetting still
// re-selects the theme over a shipped default then, so a login run heals an
// icon-theme setting reset behind its back.
func (f folderTint) apply(accent string, ensureSetting bool) (bool, error) {
	src := filepath.Join(f.papirus, "64x64", "places")
	if st, err := os.Stat(src); err != nil || !st.IsDir() {
		return false, nil
	}

	folderTintMu.Lock()
	defer folderTintMu.Unlock()
	defer lockFolderTint(f.lock)()

	if accent == "" {
		accent = paletteAccent(f.colors)
	}
	color := "blue"
	if accent != "" {
		color = folderColor(accent)
		if _, err := os.Stat(filepath.Join(src, "folder-"+color+".svg")); err != nil {
			color = "blue"
		}
	}

	plan, err := planFolderTheme(src, color)
	if err != nil {
		return false, err
	}
	link := filepath.Join(f.base, folderTheme)
	gens := filepath.Join(f.base, folderGensDir)

	if plan.matches(link) {
		if ensureSetting {
			selectFolderTheme(readIconTheme())
		}
		return false, nil
	}

	cur := readIconTheme()
	// The generation the live symlink points at now, kept one more cycle below.
	prev := ""
	if target, err := os.Readlink(link); err == nil && target != "" {
		prev = filepath.Base(target)
	}

	gen := strconv.FormatInt(time.Now().UnixNano(), 10)
	tmp := filepath.Join(gens, "build."+strconv.Itoa(os.Getpid()))
	if err := plan.build(tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return false, err
	}
	// Bake the cache into the tree before publishing so GTK re-reads the new
	// colours instead of the stale on-disk cache; the fixed name alone would not.
	if bin, err := exec.LookPath("gtk-update-icon-cache"); err == nil {
		_ = exec.Command(bin, "-qf", tmp).Run()
	}
	if err := os.Rename(tmp, filepath.Join(gens, gen)); err != nil {
		_ = os.RemoveAll(tmp)
		return false, err
	}

	stage := filepath.Join(f.base, ".ryoku-folders.link."+strconv.Itoa(os.Getpid()))
	_ = os.Remove(stage)
	if err := os.Symlink(folderGensDir+"/"+gen, stage); err != nil {
		return false, err
	}
	if st, err := os.Lstat(link); err == nil && st.Mode()&os.ModeSymlink == 0 {
		_ = os.RemoveAll(link)
	}
	if err := os.Rename(stage, link); err != nil {
		_ = os.Remove(stage)
		return false, err
	}

	selectFolderTheme(cur)
	pruneFolderGens(gens, gen, prev)
	pruneLegacyFolderThemes(f.base, cur)
	return true, nil
}

// lockFolderTint takes the cross-process lock; a lock file that cannot be
// opened leaves the run unserialised rather than skipping it.
func lockFolderTint(path string) func() {
	fh, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o666)
	if err != nil {
		return func() {}
	}
	for {
		err = syscall.Flock(int(fh.Fd()), syscall.LOCK_EX)
		if !errors.Is(err, syscall.EINTR) {
			break
		}
	}
	return func() { _ = fh.Close() }
}

var (
	accentPrimaryRe = regexp.MustCompile(`^.*"primary"[[:space:]]*:[[:space:]]*"(#[0-9A-Fa-f]{6})".*$`)
	accentColor4Re  = regexp.MustCompile(`^.*"color4"[[:space:]]*:[[:space:]]*"(#[0-9A-Fa-f]{6})".*$`)
)

// paletteAccent reads the Material primary from colors.json, else color4: the
// first line holding the key wins, the last occurrence on that line.
func paletteAccent(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	for _, re := range []*regexp.Regexp{accentPrimaryRe, accentColor4Re} {
		for _, line := range lines {
			if m := re.FindStringSubmatch(line); m != nil {
				return m[1]
			}
		}
	}
	return ""
}

// folderColor maps an accent to the nearest Papirus folder colour by hue.
// Near-greyscale accents (saturation under 0.10) stay grey; anything with
// perceptible hue tints, so a muted wallpaper still picks a folder colour.
func folderColor(accent string) string {
	r := accentChannel(accent, 1) / 255
	g := accentChannel(accent, 3) / 255
	b := accentChannel(accent, 5) / 255
	mx := max(r, g, b)
	mn := min(r, g, b)
	d := mx - mn
	var h float64
	switch {
	case d == 0:
		h = 0
	case mx == r:
		h = (g - b) / d
	case mx == g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	s := 0.0
	if mx != 0 {
		s = d / mx
	}
	switch {
	case s < 0.10:
		return "grey"
	case h < 15 || h >= 345:
		return "red"
	case h < 45:
		return "orange"
	case h < 70:
		return "yellow"
	case h < 165:
		return "green"
	case h < 195:
		return "teal"
	case h < 215:
		return "cyan"
	case h < 255:
		return "blue"
	case h < 290:
		return "violet"
	case h < 330:
		return "magenta"
	}
	return "pink"
}

// accentChannel parses the two characters at byte offset off as hex, reading
// leading hex digits only, so malformed input degrades to 0 per channel.
func accentChannel(accent string, off int) float64 {
	if off >= len(accent) {
		return 0
	}
	digits := accent[off:min(off+2, len(accent))]
	v := 0
	for _, c := range []byte(digits) {
		n, ok := hexDigit(c)
		if !ok {
			break
		}
		v = v*16 + n
	}
	return float64(v)
}

func hexDigit(c byte) (int, bool) {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0'), true
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10, true
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10, true
	}
	return 0, false
}

// folderPlan is one colour's theme content: the Papirus files copied under
// their colourless names, and the standard directory names Papirus maps to
// folder.svg, kept as local aliases so lookup never falls through to Papirus's
// blue originals.
type folderPlan struct {
	copies  map[string]string // dest name -> source path
	order   []string          // dest names in copy order
	aliases []string
}

func planFolderTheme(src, color string) (folderPlan, error) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return folderPlan{}, err
	}
	p := folderPlan{copies: map[string]string{}}
	add := func(name string) {
		path := filepath.Join(src, name)
		st, err := os.Stat(path)
		if err != nil || !st.Mode().IsRegular() {
			return
		}
		d := strings.Replace(name, "-"+color+"-", "-", 1)
		d = strings.Replace(d, "-"+color+".", ".", 1)
		if _, seen := p.copies[d]; !seen {
			p.order = append(p.order, d)
		}
		p.copies[d] = path
	}
	if _, err := os.Stat(filepath.Join(src, "folder-"+color+".svg")); err == nil {
		add("folder-" + color + ".svg")
	}
	for _, prefix := range []string{"folder-" + color + "-", "user-" + color + "-"} {
		for _, e := range entries {
			n := e.Name()
			if len(n) >= len(prefix)+4 && strings.HasPrefix(n, prefix) && strings.HasSuffix(n, ".svg") {
				add(n)
			}
		}
	}
	for _, e := range entries {
		n := e.Name()
		if e.Type()&os.ModeSymlink == 0 || strings.HasPrefix(n, ".") || !strings.HasSuffix(n, ".svg") {
			continue
		}
		if t, err := os.Readlink(filepath.Join(src, n)); err != nil || t != "folder.svg" {
			continue
		}
		if _, taken := p.copies[n]; !taken {
			p.aliases = append(p.aliases, n)
		}
	}
	return p, nil
}

// build writes the plan into dir as a complete theme tree.
func (p folderPlan) build(dir string) error {
	_ = os.RemoveAll(dir)
	places := filepath.Join(dir, "places")
	if err := os.MkdirAll(places, 0o777); err != nil {
		return err
	}
	for _, d := range p.order {
		if err := copyFollowing(p.copies[d], filepath.Join(places, d)); err != nil {
			return err
		}
	}
	for _, a := range p.aliases {
		if err := os.Symlink("folder.svg", filepath.Join(places, a)); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(dir, "index.theme"), []byte(folderIndexTheme), 0o666)
}

// matches reports whether the generation link points at already holds exactly
// this plan, so a palette change that keeps the folder colour rebuilds nothing.
func (p folderPlan) matches(link string) bool {
	target, err := os.Readlink(link)
	if err != nil {
		return false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(link), target)
	}
	if idx, err := os.ReadFile(filepath.Join(target, "index.theme")); err != nil || string(idx) != folderIndexTheme {
		return false
	}
	places := filepath.Join(target, "places")
	entries, err := os.ReadDir(places)
	if err != nil || len(entries) != len(p.order)+len(p.aliases) {
		return false
	}
	for _, a := range p.aliases {
		if t, err := os.Readlink(filepath.Join(places, a)); err != nil || t != "folder.svg" {
			return false
		}
	}
	for _, d := range p.order {
		got, err := os.ReadFile(filepath.Join(places, d))
		if err != nil {
			return false
		}
		want, err := os.ReadFile(p.copies[d])
		if err != nil || !bytes.Equal(got, want) {
			return false
		}
	}
	return true
}

// copyFollowing copies src's content (through symlinks) to a fresh dst.
func copyFollowing(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	st, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, st.Mode().Perm())
}

// readIconTheme reads the current icon-theme setting. Minimal Wayland sessions
// can have dconf but no GSettings schemas.
func readIconTheme() string {
	cur := ""
	if bin, err := exec.LookPath("gsettings"); err == nil {
		out, _ := exec.Command(bin, "get", "org.gnome.desktop.interface", "icon-theme").Output()
		cur = strings.ReplaceAll(strings.TrimRight(string(out), "\n"), "'", "")
	}
	if cur == "" {
		if bin, err := exec.LookPath("dconf"); err == nil {
			out, _ := exec.Command(bin, "read", "/org/gnome/desktop/interface/icon-theme").Output()
			cur = strings.ReplaceAll(strings.TrimRight(string(out), "\n"), "'", "")
		}
	}
	return cur
}

// selectFolderTheme points consumers at the fixed name only over the shipped
// defaults (or a stale generation name), and only when it is not already set,
// so palette changes never churn the setting and a theme the user picked stays
// theirs.
func selectFolderTheme(cur string) {
	switch cur {
	case folderTheme:
		return
	case "", "Papirus", "Papirus-Dark", "Papirus-Light", "Adwaita", "hicolor":
	default:
		if !strings.HasPrefix(cur, folderTheme+"-") {
			return
		}
	}
	if bin, err := exec.LookPath("gsettings"); err == nil &&
		exec.Command(bin, "set", "org.gnome.desktop.interface", "icon-theme", folderTheme).Run() == nil {
		return
	}
	if bin, err := exec.LookPath("dconf"); err == nil {
		_ = exec.Command(bin, "write", "/org/gnome/desktop/interface/icon-theme", "'"+folderTheme+"'").Run()
	}
}

// pruneFolderGens keeps only the live and immediately previous generations; a
// reader that resolved the old target before the swap still finds it.
func pruneFolderGens(gens, gen, prev string) {
	entries, err := os.ReadDir(gens)
	if err != nil {
		return
	}
	for _, e := range entries {
		n := e.Name()
		if strings.HasPrefix(n, ".") || n == gen || n == prev {
			continue
		}
		path := filepath.Join(gens, n)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		_ = os.RemoveAll(path)
	}
}

// pruneLegacyFolderThemes removes the per-change ryoku-folders-<digits> dirs the
// old naming scheme leaked, never the still-active one (dropped next run).
func pruneLegacyFolderThemes(base, cur string) {
	entries, err := os.ReadDir(base)
	if err != nil {
		return
	}
	prefix := folderTheme + "-"
	for _, e := range entries {
		n := e.Name()
		if !strings.HasPrefix(n, prefix) || n == cur {
			continue
		}
		if strings.Trim(n[len(prefix):], "0123456789") != "" {
			continue
		}
		path := filepath.Join(base, n)
		if st, err := os.Stat(path); err != nil || !st.IsDir() {
			continue
		}
		_ = os.RemoveAll(path)
	}
}
