package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// tests reaching matugenApply must never retint the session they run inside
func init() { folderIconsPapirus = filepath.Join(os.DevNull, "papirus") }

func folderFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	places := filepath.Join(root, "64x64", "places")
	if err := os.MkdirAll(places, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{"blue", "red", "green", "grey"} {
		writeFile(t, filepath.Join(places, "folder-"+c+".svg"), "<svg>folder "+c+"</svg>")
		writeFile(t, filepath.Join(places, "folder-"+c+"-music.svg"), "<svg>music "+c+"</svg>")
		writeFile(t, filepath.Join(places, "user-"+c+"-home.svg"), "<svg>home "+c+"</svg>")
		writeFile(t, filepath.Join(places, "user-"+c+"-desktop.svg"), "<svg>desktop "+c+"</svg>")
		symlink(t, "user-"+c+"-desktop.svg", filepath.Join(places, "folder-"+c+"-desktop.svg"))
	}
	writeFile(t, filepath.Join(places, "folder-bluegrey.svg"), "<svg>folder bluegrey</svg>")
	symlink(t, "missing.svg", filepath.Join(places, "folder-red-broken.svg"))
	symlink(t, "folder-blue.svg", filepath.Join(places, "folder.svg"))
	for _, a := range []string{"gtk-directory.svg", "inode-directory.svg", "stock_folder.svg"} {
		symlink(t, "folder.svg", filepath.Join(places, a))
	}
	symlink(t, "user-blue-home.svg", filepath.Join(places, "user-home.svg"))
	return root
}

func symlink(t *testing.T, target, path string) {
	t.Helper()
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
}

type folderEnv struct {
	tint folderTint
	log  string
}

func newFolderEnv(t *testing.T, theme string) folderEnv {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(dir, "argv.log")
	logLine := `printf '%s\n' "${0##*/} $*" >> "` + log + `"`
	writeShim(t, filepath.Join(bin, "gsettings"), logLine+"\n[ \"$1\" = get ] && printf \"'%s'\\n\" '"+theme+"'\nexit 0")
	writeShim(t, filepath.Join(bin, "dconf"), logLine)
	writeShim(t, filepath.Join(bin, "gtk-update-icon-cache"), logLine+"\nprintf cache > \"$2/icon-theme.cache\"")
	t.Setenv("PATH", bin)
	return folderEnv{
		tint: folderTint{
			papirus: folderFixture(t),
			base:    filepath.Join(dir, "data", "icons"),
			lock:    filepath.Join(dir, "ryoku-folders.lock"),
			colors:  filepath.Join(dir, "colors.json"),
		},
		log: log,
	}
}

func (e folderEnv) calls(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(e.log)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(e.log)
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func (e folderEnv) live(t *testing.T) string {
	t.Helper()
	target, err := os.Readlink(filepath.Join(e.tint.base, folderTheme))
	if err != nil {
		t.Fatalf("theme link: %v", err)
	}
	if !strings.HasPrefix(target, folderGensDir+"/") {
		t.Fatalf("theme link target %q is not a generation", target)
	}
	return filepath.Base(target)
}

func (e folderEnv) gens(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(e.tint.base, folderGensDir))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, en := range entries {
		names = append(names, en.Name())
	}
	return names
}

func tree(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || path == dir || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		if d.Type()&os.ModeSymlink != 0 {
			target, _ := os.Readlink(path)
			out = append(out, rel+" -> "+target)
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if rel != "index.theme" {
			out = append(out, rel+": "+string(data))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(out)
	return out
}

func mustApply(t *testing.T, tint folderTint, accent string, ensure bool) bool {
	t.Helper()
	changed, err := tint.apply(accent, ensure)
	if err != nil {
		t.Fatalf("apply(%q): %v", accent, err)
	}
	return changed
}

func TestFolderTintPublishesTree(t *testing.T) {
	e := newFolderEnv(t, "Papirus-Dark")
	if !mustApply(t, e.tint, "#e03030", false) {
		t.Fatal("first run published nothing")
	}
	gen := e.live(t)
	dir := filepath.Join(e.tint.base, folderGensDir, gen)
	want := []string{
		"icon-theme.cache: cache",
		"places/folder-desktop.svg: <svg>desktop red</svg>",
		"places/folder-music.svg: <svg>music red</svg>",
		"places/folder.svg: <svg>folder red</svg>",
		"places/gtk-directory.svg -> folder.svg",
		"places/inode-directory.svg -> folder.svg",
		"places/stock_folder.svg -> folder.svg",
		"places/user-desktop.svg: <svg>desktop red</svg>",
		"places/user-home.svg: <svg>home red</svg>",
	}
	if got := tree(t, dir); !slices.Equal(got, want) {
		t.Errorf("published tree:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if idx, _ := os.ReadFile(filepath.Join(dir, "index.theme")); string(idx) != folderIndexTheme {
		t.Errorf("index.theme = %q", idx)
	}
	for _, f := range []string{"places/folder.svg", "places/user-home.svg"} {
		if st, err := os.Lstat(filepath.Join(dir, f)); err != nil || !st.Mode().IsRegular() {
			t.Errorf("%s is not a regular copy: %v", f, err)
		}
	}
	wantCalls := []string{
		"gsettings get org.gnome.desktop.interface icon-theme",
		"gtk-update-icon-cache -qf " + filepath.Join(e.tint.base, folderGensDir, fmt.Sprintf("build.%d", os.Getpid())),
		"gsettings set org.gnome.desktop.interface icon-theme ryoku-folders",
	}
	if got := e.calls(t); !slices.Equal(got, wantCalls) {
		t.Errorf("calls:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(wantCalls, "\n"))
	}
	if g := e.gens(t); !slices.Equal(g, []string{gen}) {
		t.Errorf("generations = %v, want [%s]", g, gen)
	}
	if _, err := os.Lstat(filepath.Join(e.tint.base, ".ryoku-folders.link."+fmt.Sprint(os.Getpid()))); !os.IsNotExist(err) {
		t.Errorf("staging link left behind: %v", err)
	}
}

func TestFolderTintSameColourIsNoOp(t *testing.T) {
	e := newFolderEnv(t, "ryoku-folders")
	mustApply(t, e.tint, "#e03030", false)
	gen := e.live(t)
	e.calls(t)

	// different accent, same folder colour
	if mustApply(t, e.tint, "#ff0000", false) {
		t.Error("same colour published a new generation")
	}
	if e.live(t) != gen {
		t.Error("live generation moved")
	}
	if got := e.calls(t); len(got) != 0 {
		t.Errorf("no-op run spawned %v", got)
	}

	if mustApply(t, e.tint, "#ff0000", true) {
		t.Error("login run on the same colour published a new generation")
	}
	if got := e.calls(t); !slices.Equal(got, []string{"gsettings get org.gnome.desktop.interface icon-theme"}) {
		t.Errorf("login no-op calls = %v", got)
	}

	// a damaged tree gets rebuilt even on the same colour
	if err := os.Remove(filepath.Join(e.tint.base, folderGensDir, gen, "places", "user-home.svg")); err != nil {
		t.Fatal(err)
	}
	if !mustApply(t, e.tint, "#ff0000", false) {
		t.Error("damaged tree was not rebuilt")
	}
}

func TestFolderTintKeepsOnePreviousGeneration(t *testing.T) {
	// the setting still names a legacy dir, that one stays until the next run
	e := newFolderEnv(t, "ryoku-folders-42")
	base := e.tint.base
	for _, d := range []string{"ryoku-folders-1700000000", "ryoku-folders-17x", "ryoku-folders-42", filepath.Join(folderGensDir, "build.1")} {
		if err := os.MkdirAll(filepath.Join(base, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	mustApply(t, e.tint, "#e03030", false)
	first := e.live(t)
	mustApply(t, e.tint, "#30e030", false)
	second := e.live(t)
	if got, want := e.gens(t), []string{first, second}; !slices.Equal(got, want) {
		t.Errorf("after two runs generations = %v, want %v", got, want)
	}
	mustApply(t, e.tint, "#888888", false)
	third := e.live(t)
	if got, want := e.gens(t), []string{second, third}; !slices.Equal(got, want) {
		t.Errorf("after three runs generations = %v, want %v", got, want)
	}
	if got := tree(t, filepath.Join(base, folderGensDir, third)); !slices.Contains(got, "places/folder.svg: <svg>folder grey</svg>") {
		t.Errorf("grey accent tree = %v", got)
	}
	for name, kept := range map[string]bool{
		"ryoku-folders-1700000000": false,
		"ryoku-folders-17x":        true,
		"ryoku-folders-42":         true,
	} {
		if _, err := os.Stat(filepath.Join(base, name)); (err == nil) != kept {
			t.Errorf("%s kept=%v, want %v", name, err == nil, kept)
		}
	}
}

func TestFolderTintReplacesRealDirectory(t *testing.T) {
	e := newFolderEnv(t, "")
	if err := os.MkdirAll(filepath.Join(e.tint.base, folderTheme, "places"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustApply(t, e.tint, "#2050e0", false)
	e.live(t)
	got := e.calls(t)
	if !slices.Contains(got, "dconf read /org/gnome/desktop/interface/icon-theme") {
		t.Errorf("empty gsettings value did not fall back to dconf: %v", got)
	}
	if !slices.Contains(got, "gsettings set org.gnome.desktop.interface icon-theme ryoku-folders") {
		t.Errorf("unset theme not selected: %v", got)
	}
}

func TestFolderTintLeavesUserTheme(t *testing.T) {
	e := newFolderEnv(t, "Tela-circle")
	mustApply(t, e.tint, "#e03030", true)
	for _, c := range e.calls(t) {
		if strings.Contains(c, " set ") || strings.Contains(c, " write ") {
			t.Errorf("user-picked theme overwritten: %s", c)
		}
	}
}

func TestFolderTintAccentSource(t *testing.T) {
	e := newFolderEnv(t, "ryoku-folders")
	cases := []struct{ colors, want string }{
		{"{\n  \"color4\": \"#20e020\",\n  \"primary\": \"#e02020\"\n}\n", "red"},
		{"{\n  \"color4\": \"#20e020\",\n  \"onPrimary\": \"#e02020\"\n}\n", "green"},
		{`{"primary": "#e02020", "x": {"primary": "#2020e0"}}`, "blue"},
		{"{\n  \"primary\": \"e02020\"\n}\n", "blue"},
		{"{\n  \"primary\": \"#20e0e5\"\n}\n", "blue"}, // teal has no set in the fixture
		{"", "blue"},
	}
	for _, c := range cases {
		writeFile(t, e.tint.colors, c.colors)
		mustApply(t, e.tint, "", false)
		got := tree(t, filepath.Join(e.tint.base, folderGensDir, e.live(t)))
		if !slices.Contains(got, "places/folder.svg: <svg>folder "+c.want+"</svg>") {
			t.Errorf("colors.json %q: tree %v, want %s", c.colors, got, c.want)
		}
	}
	writeFile(t, e.tint.colors, `{"primary": "#e02020"}`)
	mustApply(t, e.tint, "#20e020", false)
	if got := tree(t, filepath.Join(e.tint.base, folderGensDir, e.live(t))); !slices.Contains(got, "places/folder.svg: <svg>folder green</svg>") {
		t.Errorf("explicit accent ignored: %v", got)
	}
}

func TestFolderTintWithoutPapirusDoesNothing(t *testing.T) {
	e := newFolderEnv(t, "Papirus")
	e.tint.papirus = filepath.Join(t.TempDir(), "none")
	if mustApply(t, e.tint, "#e03030", true) {
		t.Error("published without Papirus")
	}
	if _, err := os.Lstat(e.tint.base); !os.IsNotExist(err) {
		t.Errorf("icons dir touched: %v", err)
	}
	if got := e.calls(t); len(got) != 0 {
		t.Errorf("spawned %v", got)
	}
}

// the formula the old shell helper used, verbatim
const folderColorAwk = `{ c=$0
    r=strtonum("0x" substr(c,2,2))/255; g=strtonum("0x" substr(c,4,2))/255; b=strtonum("0x" substr(c,6,2))/255
    mx=(r>g?(r>b?r:b):(g>b?g:b)); mn=(r<g?(r<b?r:b):(g<b?g:b)); d=mx-mn
    if(d==0) h=0; else if(mx==r) h=(g-b)/d; else if(mx==g) h=(b-r)/d+2; else h=(r-g)/d+4
    h*=60; if(h<0) h+=360; s=(mx==0?0:d/mx)
    print (s<0.10)?"grey":(h<15||h>=345)?"red":(h<45)?"orange":(h<70)?"yellow":(h<165)?"green":(h<195)?"teal":(h<215)?"cyan":(h<255)?"blue":(h<290)?"violet":(h<330)?"magenta":"pink"
}`

var folderColorTable = map[string]string{
	"#ff0000": "red", "#ff3f00": "red", "#ff4000": "orange", "#ffbf00": "orange",
	"#ffc000": "yellow", "#c0ff00": "green", "#00ff40": "green", "#00ffc0": "teal",
	"#00ffff": "teal", "#00d0ff": "teal", "#00a0ff": "cyan", "#0000ff": "blue",
	"#8000ff": "violet", "#ff00ff": "magenta", "#ff00c0": "magenta", "#ff0040": "pink",
	"#000000": "grey", "#ffffff": "grey", "#808080": "grey", "#e6e0e0": "grey",
	"#e6d0d0": "grey", "#e6c8c8": "red", "#FF8800": "orange", "": "grey", "zz": "grey",
	"#abc": "red", "ff0000": "red",
}

func TestFolderColorTable(t *testing.T) {
	for accent, want := range folderColorTable {
		if got := folderColor(accent); got != want {
			t.Errorf("folderColor(%q) = %s, want %s", accent, got, want)
		}
	}
}

func TestFolderColorMatchesAwk(t *testing.T) {
	awk, err := exec.LookPath("/usr/bin/gawk")
	if err != nil {
		t.Skip("gawk not installed")
	}
	var inputs []string
	for accent := range folderColorTable {
		inputs = append(inputs, accent)
	}
	inputs = append(inputs, "#1a1a1b", "#e6c9c9")
	for r := 0; r < 256; r += 15 {
		for g := 0; g < 256; g += 15 {
			for b := 0; b < 256; b += 15 {
				inputs = append(inputs, fmt.Sprintf("#%02x%02x%02x", r, g, b))
			}
		}
	}
	for h := range 360 {
		inputs = append(inputs, hueHex(float64(h), 1), hueHex(float64(h), 0.11), hueHex(float64(h)+0.5, 0.6))
	}
	cmd := exec.Command(awk, folderColorAwk)
	cmd.Stdin = strings.NewReader(strings.Join(inputs, "\n") + "\n")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("awk: %v", err)
	}
	got := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(got) != len(inputs) {
		t.Fatalf("awk printed %d lines for %d inputs", len(got), len(inputs))
	}
	for i, in := range inputs {
		if c := folderColor(in); c != got[i] {
			t.Errorf("folderColor(%q) = %s, awk says %s", in, c, got[i])
		}
	}
}

func hueHex(h, s float64) string {
	c := s
	x := c * (1 - abs(mod(h/60, 2)-1))
	var r, g, b float64
	switch {
	case h < 60:
		r, g = c, x
	case h < 120:
		r, g = x, c
	case h < 180:
		g, b = c, x
	case h < 240:
		g, b = x, c
	case h < 300:
		r, b = x, c
	default:
		r, b = c, x
	}
	m := 1 - c
	return fmt.Sprintf("#%02x%02x%02x", int((r+m)*255+0.5), int((g+m)*255+0.5), int((b+m)*255+0.5))
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func mod(a, b float64) float64 {
	for a >= b {
		a -= b
	}
	return a
}
