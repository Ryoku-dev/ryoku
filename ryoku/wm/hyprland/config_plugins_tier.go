package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// The Hyprland compositor plugins the desktop manages. A plugin is one .so
// Hyprland dlopens, ABI-locked to the exact compositor build. Three tiers can
// hold a copy: the package under pluginDir, a local build under userPluginDir,
// and hyprpm's own tree. Beside every copy Ryoku builds sits a <id>.abi receipt
// so the roster and settings.lua can tell a stale build from a working one
// without loading it.

// pluginDef is one plugin Ryoku bundles.
type pluginDef struct {
	ID      string
	Name    string
	Desc    string
	Repo    string // upstream git URL; "" for a plugin authored in this repo
	Plugin  string // the [table] in the repo's hyprpm.toml
	Local   string // in-tree source dir for a Ryoku-authored plugin
	Package string // the pacman package that ships the .so
	Docs    string
	Sounds  string
	Loaded  string // the name the plugin reports in `hyprctl plugins list`
}

var bundledPlugins = []pluginDef{
	{
		ID: "hyprbars", Name: "Title bars",
		Desc: "A bar with the window's title and close/maximise buttons above every window.",
		Repo: "https://github.com/hyprwm/hyprland-plugins", Plugin: "hyprbars",
		Package: "ryoku-hypr-plugins",
		Docs:    "https://github.com/hyprwm/hyprland-plugins/tree/main/hyprbars",
		Loaded:  "hyprbars",
	},
	{
		ID: "hyprglass", Name: "Glass",
		Desc: "Liquid-glass blur and refraction on windows.",
		Repo: "https://github.com/hyprnux/hyprglass", Plugin: "hyprglass",
		Package: "hyprglass",
		Docs:    "https://github.com/hyprnux/hyprglass#configuration",
		Loaded:  "hyprglass",
	},
	{
		ID: "imgborders", Name: "Image borders",
		Desc: "Tiles an image of your choosing around each window as its border.",
		Repo: "https://codeberg.org/zacoons/imgborders", Plugin: "imgborders",
		Package: "imgborders",
		Docs:    "https://codeberg.org/zacoons/imgborders#configuration",
		Loaded:  "imgborders",
	},
	{
		ID: "dynamic-cursors", Name: "Cursor motion",
		Desc: "Realistic pointer motion (rotate, tilt, stretch) and shake-to-find magnify.",
		Repo: "https://github.com/VirtCode/hypr-dynamic-cursors", Plugin: "dynamic-cursors",
		Package: "hypr-dynamic-cursors",
		Docs:    "https://github.com/VirtCode/hypr-dynamic-cursors#configuration",
		Loaded:  "dynamic-cursors",
	},
	{
		ID: "hyprfocus", Name: "Focus flash",
		Desc: "Flashes, shrinks or slides a window as it takes focus.",
		Repo: "https://github.com/hyprwm/hyprland-plugins", Plugin: "hyprfocus",
		Package: "ryoku-hypr-plugins",
		Docs:    "https://github.com/hyprwm/hyprland-plugins/tree/main/hyprfocus",
		Loaded:  "hyprfocus",
	},
	{
		ID: "keysounds", Name: "Key sounds",
		Desc:   "Plays a keyboard sound on every key press: real switch recordings, or your own samples.",
		Plugin: "keysounds", Local: "ryoku/hyprland/plugins/keysounds",
		Package: "ryoku-keysounds",
		Docs:    "https://github.com/ryoku-dev/ryoku-arch/blob/unstable-dev/ryoku/hyprland/plugins/keysounds/README.md",
		Sounds:  "https://github.com/hainguyents13/mechvibes/tree/main/src/audio",
		Loaded:  "keysounds",
	},
}

// keysoundsUserDir is where the user's own key sound profiles go, created on
// demand with a README so "My sounds" opens a folder that explains itself.
func keysoundsUserDir() string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "share")
	}
	dir := filepath.Join(base, "ryoku", "keysounds")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return dir
	}
	readme := filepath.Join(dir, "README")
	if !fileExists(readme) {
		_ = os.WriteFile(readme, []byte(`Your key sound profiles. Each folder here is one profile, picked by name in
Settings > Plugins > Key sounds; a folder named like a shipped profile
replaces it.

From a Mechvibes pack (mechvibes.com, or github.com/hainguyents13/mechvibes
src/audio): unpack it anywhere, then

    ryoku-keysounds-import <pack-dir> --name <profile>

cuts it into a folder here. Or make one by hand: short .wav/.ogg files named

    down-1.wav down-2.wav ...     any key press (one is picked at random)
    up-1.wav ...                  key release
    space-1.wav enter-1.wav backspace-1.wav       those keys
    space-up-1.wav enter-up-1.wav backspace-up-1.wav   their releases

A missing role falls back to down (or up). The shipped profiles are a good
starting point to copy from: /usr/share/ryoku/keysounds.
`), 0o644)
	}
	return dir
}

// keysoundsShippedDir is where the installed profiles are: beside a local build
// of the plugin when there is one, else the package's.
func keysoundsShippedDir() string {
	if local := filepath.Join(userPluginDir(), "keysounds"); fileExists(filepath.Join(local, "cherry-mx-brown")) {
		return local
	}
	return "/usr/share/ryoku/keysounds"
}

func bundledByID(id string) (pluginDef, bool) {
	for _, d := range bundledPlugins {
		if d.ID == id {
			return d, true
		}
	}
	return pluginDef{}, false
}

// sharedRecipeDir is where ryoku-desktop lays the source of a plugin Ryoku
// authors (keysounds), so a packaged box can rebuild it without a checkout.
const sharedRecipeDir = "/usr/share/ryoku/hypr-plugins"

// userPluginDir is the local-build tier: what deploy.sh, Rebuild and Add write.
func userPluginDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "lib", "hyprland", "plugins")
}

// hyprpmDir is hyprpm's own plugin tree, read-only so a plugin the user
// installed by hand still shows on the Plugins page.
func hyprpmDir() string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "hyprpm")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "hyprpm")
}

// hyprABI is Hyprland's plugin ABI string, split: the compositor commit plus the
// major.minor of each library the plugin API leaks through its headers.
type hyprABI struct {
	Commit, Aquamarine, Hyprutils, Hyprgraphics, Hyprcursor, Hyprlang string
}

func (a hyprABI) String() string {
	if a.Commit == "" {
		return ""
	}
	return a.Commit + "_aq_" + a.Aquamarine + "_hu_" + a.Hyprutils + "_hg_" + a.Hyprgraphics + "_hc_" + a.Hyprcursor + "_hlg_" + a.Hyprlang
}

func (a hyprABI) ok() bool { return a.Commit != "" }

var abiRe = regexp.MustCompile(`^([0-9a-f]+)_aq_([^_]+)_hu_([^_]+)_hg_([^_]+)_hc_([^_]+)_hlg_([^_\s]+)$`)

func parseABI(s string) hyprABI {
	m := abiRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return hyprABI{}
	}
	return hyprABI{Commit: m[1], Aquamarine: m[2], Hyprutils: m[3], Hyprgraphics: m[4], Hyprcursor: m[5], Hyprlang: m[6]}
}

// abiDiff names what moved between the ABI a plugin was built for and the one the
// compositor runs, in words a user can act on. Empty when they agree.
func abiDiff(built, running hyprABI) string {
	var parts []string
	pair := func(name, b, r string) {
		if b != r {
			parts = append(parts, fmt.Sprintf("%s %s, running %s", name, b, r))
		}
	}
	if built.Commit != running.Commit {
		parts = append(parts, fmt.Sprintf("Hyprland %s, running %s", shortCommit(built.Commit), shortCommit(running.Commit)))
	}
	pair("aquamarine", built.Aquamarine, running.Aquamarine)
	pair("hyprutils", built.Hyprutils, running.Hyprutils)
	pair("hyprgraphics", built.Hyprgraphics, running.Hyprgraphics)
	pair("hyprcursor", built.Hyprcursor, running.Hyprcursor)
	pair("hyprlang", built.Hyprlang, running.Hyprlang)
	if len(parts) == 0 {
		return ""
	}
	return "built for " + strings.Join(parts, "; ")
}

func shortCommit(c string) string {
	if len(c) > 7 {
		return c[:7]
	}
	return c
}

// stripPatch mirrors the plugin API's version trimming: "0.15.0" -> "0.15".
func stripPatch(v string) string {
	if i := strings.LastIndex(v, "."); i > 0 {
		return v[:i]
	}
	return v
}

// compositorInfo is what the running Hyprland says about itself.
type compositorInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	ABI     string `json:"abi"`
	Live    bool   `json:"live"`
	Headers string `json:"headers"`
	abi     hyprABI
}

// liveCompositor asks the running compositor for its version and ABI hash, once
// per process.
func liveCompositor() compositorInfo {
	liveOnce.Do(func() {
		out, err := ctl("version", "-j")
		if err != nil {
			return
		}
		var v struct {
			Version string `json:"version"`
			Commit  string `json:"commit"`
			ABIHash string `json:"abiHash"`
		}
		if json.Unmarshal(out, &v) != nil {
			return
		}
		abi := parseABI(v.ABIHash)
		liveCached = compositorInfo{Version: v.Version, Commit: v.Commit, ABI: abi.String(), Live: abi.ok(), abi: abi}
	})
	return liveCached
}

var (
	liveOnce   sync.Once
	liveCached compositorInfo
)

// headersVersionPath locates Hyprland's version.h, the file a plugin build bakes
// its ABI from.
func headersVersionPath() string {
	if out, err := exec.Command("pkg-config", "--variable=prefix", "hyprland").Output(); err == nil {
		if p := strings.TrimSpace(string(out)); p != "" {
			return filepath.Join(p, "hyprland", "src", "version.h")
		}
	}
	return "/usr/include/hyprland/src/version.h"
}

// headersABI computes the ABI a build against the installed Hyprland headers
// produces, the build target: a receipt equal to it means rebuilding changes
// nothing; a running compositor equal to it means the build will load.
func headersABI() (hyprABI, string) {
	b, err := os.ReadFile(headersVersionPath())
	if err != nil {
		return hyprABI{}, ""
	}
	return parseVersionHeader(string(b))
}

var versionDefineRe = regexp.MustCompile(`(?m)^#define\s+(\w+)\s+"([^"]*)"`)

// parseVersionHeader reads version.h into an ABI plus the Hyprland tag.
func parseVersionHeader(src string) (hyprABI, string) {
	m := map[string]string{}
	for _, x := range versionDefineRe.FindAllStringSubmatch(src, -1) {
		m[x[1]] = x[2]
	}
	if m["GIT_COMMIT_HASH"] == "" {
		return hyprABI{}, ""
	}
	return hyprABI{
		Commit:       m["GIT_COMMIT_HASH"],
		Aquamarine:   stripPatch(m["AQUAMARINE_VERSION"]),
		Hyprutils:    stripPatch(m["HYPRUTILS_VERSION"]),
		Hyprgraphics: stripPatch(m["HYPRGRAPHICS_VERSION"]),
		Hyprcursor:   stripPatch(m["HYPRCURSOR_VERSION"]),
		Hyprlang:     stripPatch(m["HYPRLANG_VERSION"]),
	}, strings.TrimPrefix(m["GIT_TAG"], "v")
}

// abiSidecar is the receipt path beside a .so: <dir>/<name>.abi.
func abiSidecar(so string) string {
	return strings.TrimSuffix(so, ".so") + ".abi"
}

func readABI(so string) hyprABI {
	b, err := os.ReadFile(abiSidecar(so))
	if err != nil {
		return hyprABI{}
	}
	return parseABI(string(b))
}

// soCopy is one .so on disk with the tier it came from and its receipt.
type soCopy struct {
	Path string
	Tier string // "built" | "package" | "hyprpm"
	ABI  hyprABI
}

// pluginCopies lists every copy of a plugin, local build first.
func pluginCopies(id string) []soCopy {
	var out []soCopy
	for _, c := range []soCopy{
		{Path: filepath.Join(userPluginDir(), id+".so"), Tier: "built"},
		{Path: filepath.Join(pluginDir, id+".so"), Tier: "package"},
	} {
		if fi, err := os.Stat(c.Path); err == nil && !fi.IsDir() {
			c.ABI = readABI(c.Path)
			out = append(out, c)
		}
	}
	for _, p := range hyprpmCopies() {
		if strings.TrimSuffix(filepath.Base(p), ".so") == id {
			out = append(out, soCopy{Path: p, Tier: "hyprpm"})
		}
	}
	return out
}

// hyprpmCopies finds the .so files hyprpm built: <hyprpmDir>/<repo>/<name>.so.
func hyprpmCopies() []string {
	m, _ := filepath.Glob(filepath.Join(hyprpmDir(), "*", "*.so"))
	sort.Strings(m)
	return m
}

// pickCopy chooses the copy settings.lua should load for a target ABI: the first
// whose receipt matches, else the first with no receipt. stale is true when every
// copy carries a receipt and none matches.
func pickCopy(copies []soCopy, target hyprABI) (c soCopy, ok bool, stale bool) {
	if len(copies) == 0 {
		return soCopy{}, false, false
	}
	if !target.ok() {
		return copies[0], true, false
	}
	for _, c := range copies {
		if c.ABI.ok() && c.ABI == target {
			return c, true, false
		}
	}
	for _, c := range copies {
		if !c.ABI.ok() {
			return c, true, false
		}
	}
	return copies[0], true, true
}

type loadedPlugin struct {
	Name    string `json:"name"`
	Author  string `json:"author"`
	Version string `json:"version"`
}

// loadedPlugins is `hyprctl plugins list -j`: the plugins the compositor holds
// right now, by the name each reports.
func loadedPlugins() map[string]loadedPlugin {
	out, err := ctl("plugins", "list", "-j")
	m := map[string]loadedPlugin{}
	if err != nil {
		return m
	}
	var list []loadedPlugin
	if json.Unmarshal(out, &list) != nil {
		return m
	}
	for _, p := range list {
		m[p.Name] = p
	}
	return m
}

// hyprctlPluginLoad asks the compositor to load a .so; the response text is
// Hyprland's own verdict on a version mismatch.
func hyprctlPluginLoad(path string) error {
	out, err := ctl("plugin", "load", path)
	s := strings.TrimSpace(string(out))
	if err != nil {
		if s != "" && s != "ok" {
			return fmt.Errorf("%s", s)
		}
		return err
	}
	if s != "ok" && s != "" {
		return fmt.Errorf("%s", s)
	}
	return nil
}

func hyprctlPluginUnload(path string) {
	_, _ = ctl("plugin", "unload", path)
}

// unloadPluginCopies drops every copy of a plugin the compositor might hold. A
// reload only unloads a plugin it loaded from config itself; one the roster probe
// or loadEnabledPlugins loaded by hand stays, so a Save that turns one off has to
// say so explicitly.
func unloadPluginCopies(id string) {
	for _, c := range pluginCopies(id) {
		hyprctlPluginUnload(c.Path)
	}
}

// loadEnabledPlugins loads every enabled plugin the compositor does not hold
// after a reload. Returns true when something was loaded, so the caller can push
// the config once more.
func loadEnabledPlugins(o Overrides) bool {
	loaded := loadedPlugins()
	var want []pluginDef
	for _, d := range bundledPlugins {
		if pluginEnabled(o, d.ID) {
			want = append(want, d)
		}
	}
	for _, id := range sortedExtraIDs(o.Plugins.Extra) {
		if !o.Plugins.Extra[id].Enabled {
			continue
		}
		d := pluginDef{ID: id, Loaded: id}
		if r, ok := readReceipt(id); ok {
			d = receiptDef(r)
		}
		want = append(want, d)
	}
	any := false
	for _, d := range want {
		if _, ok := loaded[d.Loaded]; ok {
			continue
		}
		if so := pluginSoPath(d.ID); so != "" && hyprctlPluginLoad(so) == nil {
			any = true
		}
	}
	return any
}

var mismatchRe = regexp.MustCompile(`built against: (\S+), running compositor: (\S+)`)

// loadFailureDetail turns Hyprland's load error into the line the Hub shows.
func loadFailureDetail(err error) string {
	s := err.Error()
	if m := mismatchRe.FindStringSubmatch(s); m != nil {
		if d := abiDiff(parseABI(m[1]), parseABI(m[2])); d != "" {
			return d
		}
	}
	if i := strings.Index(s, "could not be loaded: "); i >= 0 {
		s = s[i+len("could not be loaded: "):]
	}
	return strings.TrimSpace(s)
}

// detectedSetting is one config value a plugin registers, found by name in its
// .so and typed by the compositor once loaded. Key drops the `plugin:` prefix.
type detectedSetting struct {
	Key     string `json:"key"`
	Type    string `json:"type,omitempty"`
	Default any    `json:"default,omitempty"`
}

var pluginKeyRe = regexp.MustCompile(`^plugin:[A-Za-z0-9_-]+(?::[A-Za-z0-9_-]+)+$`)

// scanPluginKeys pulls the config keys a plugin registers out of its .so: the
// names are string literals, so they survive compilation verbatim.
func scanPluginKeys(so string) []string {
	b, err := os.ReadFile(so)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var keys []string
	for _, s := range bytes.Split(b, []byte{0}) {
		if len(s) < 8 || !bytes.HasPrefix(s, []byte("plugin:")) {
			continue
		}
		k := string(s)
		if !pluginKeyRe.MatchString(k) || seen[k] {
			continue
		}
		seen[k] = true
		keys = append(keys, strings.TrimPrefix(k, "plugin:"))
	}
	sort.Strings(keys)
	return keys
}

// typeSetting asks the compositor what one plugin option is: its type and the
// value it currently holds.
func typeSetting(key string) (string, any, bool) {
	out, err := ctl("-j", "getoption", "plugin:"+key)
	if err != nil {
		return "", nil, false
	}
	var v map[string]any
	if json.Unmarshal(out, &v) != nil {
		return "", nil, false
	}
	for _, t := range []string{"bool", "int", "float", "str", "color", "custom"} {
		if val, ok := v[t]; ok {
			return t, val, true
		}
	}
	return "", nil, false
}

// detectSettings merges the keys in a .so with whatever the compositor can type
// right now, keeping earlier typing when the plugin is not loaded.
func detectSettings(so string, loaded bool, prior []detectedSetting) []detectedSetting {
	known := map[string]detectedSetting{}
	for _, s := range prior {
		known[s.Key] = s
	}
	var out []detectedSetting
	for _, k := range scanPluginKeys(so) {
		s := detectedSetting{Key: k}
		if p, ok := known[k]; ok {
			s = p
		}
		if loaded {
			if t, def, ok := typeSetting(k); ok {
				s.Type = t
				if s.Default == nil {
					s.Default = def
				}
			}
		}
		out = append(out, s)
	}
	return out
}

// pluginReceipt records a local build: where it came from, which commit, when,
// and the settings detected.
type pluginReceipt struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Desc     string            `json:"desc,omitempty"`
	Repo     string            `json:"repo,omitempty"`
	Local    string            `json:"local,omitempty"`
	Plugin   string            `json:"plugin"`
	Commit   string            `json:"commit,omitempty"`
	Hyprland string            `json:"hyprland,omitempty"`
	BuiltAt  string            `json:"builtAt"`
	Loaded   string            `json:"loadedName,omitempty"`
	Settings []detectedSetting `json:"settings"`
}

func receiptPath(id string) string {
	return filepath.Join(userPluginDir(), id+".json")
}

func readReceipt(id string) (pluginReceipt, bool) {
	b, err := os.ReadFile(receiptPath(id))
	if err != nil {
		return pluginReceipt{}, false
	}
	var r pluginReceipt
	if json.Unmarshal(b, &r) != nil || r.ID == "" {
		return pluginReceipt{}, false
	}
	return r, true
}

func writeReceipt(r pluginReceipt) error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(receiptPath(r.ID), append(b, '\n'), 0o644)
}

// localReceipts lists every receipt in the user tier, sorted by id.
func localReceipts() []pluginReceipt {
	m, _ := filepath.Glob(filepath.Join(userPluginDir(), "*.json"))
	sort.Strings(m)
	var out []pluginReceipt
	for _, p := range m {
		id := strings.TrimSuffix(filepath.Base(p), ".json")
		if r, ok := readReceipt(id); ok {
			out = append(out, r)
		}
	}
	return out
}

// pluginInfo is one row of `plugins list`.
type pluginInfo struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Desc          string            `json:"desc"`
	Docs          string            `json:"docs"`
	Sounds        string            `json:"sounds,omitempty"`
	UserSounds    string            `json:"userSounds,omitempty"`
	ShippedSounds string            `json:"shippedSounds,omitempty"`
	Repo          string            `json:"repo"`
	Package       string            `json:"package,omitempty"`
	Bundled       bool              `json:"bundled"`
	Source        string            `json:"source"` // package | built | hyprpm | none
	Path          string            `json:"path"`
	Installed     bool              `json:"installed"`
	Version       string            `json:"version"`
	BuiltAt       string            `json:"builtAt,omitempty"`
	ABI           string            `json:"abi"`
	Enabled       bool              `json:"enabled"`
	Loaded        bool              `json:"loaded"`
	Status        string            `json:"status"` // running | ready | stale | missing | failed
	Detail        string            `json:"detail"`
	Current       bool              `json:"current"`
	Rebuildable   bool              `json:"rebuildable"`
	Removable     bool              `json:"removable"`
	Settings      []detectedSetting `json:"settings"`
}

// toolchainInfo says whether this box can build a plugin at all.
type toolchainInfo struct {
	OK      bool     `json:"ok"`
	Missing []string `json:"missing"`
}

func toolchain() toolchainInfo {
	missing := []string{}
	for _, bin := range []string{"git", "make", "c++", "pkg-config", "cmake"} {
		if _, err := exec.LookPath(bin); err != nil {
			missing = append(missing, bin)
		}
	}
	if exec.Command("pkg-config", "--exists", "hyprland").Run() != nil {
		missing = append(missing, "hyprland headers")
	}
	return toolchainInfo{OK: len(missing) == 0, Missing: missing}
}

type pluginRoster struct {
	Compositor compositorInfo `json:"compositor"`
	Toolchain  toolchainInfo  `json:"toolchain"`
	Plugins    []pluginInfo   `json:"plugins"`
}

// pluginEnabled reads a plugin's toggle from the store.
func pluginEnabled(o Overrides, id string) bool {
	switch id {
	case "hyprbars":
		return o.Plugins.Hyprbars.Enabled
	case "hyprglass":
		return o.Plugins.Hyprglass.Enabled
	case "imgborders":
		return o.Plugins.Imgborders.Enabled
	case "dynamic-cursors":
		return o.Plugins.DynamicCursors.Enabled
	case "hyprfocus":
		return o.Plugins.Hyprfocus.Enabled
	case "keysounds":
		return o.Plugins.Keysounds.Enabled
	}
	return o.Plugins.Extra[id].Enabled
}

// packageVersion is what pacman holds for a package, "" when not installed.
func packageVersion(pkg string) string {
	if pkg == "" {
		return ""
	}
	out, err := exec.Command("pacman", "-Q", pkg).Output()
	if err != nil {
		return ""
	}
	f := strings.Fields(string(out))
	if len(f) < 2 {
		return ""
	}
	return f[1]
}

// pluginRosterNow builds the full roster against the live compositor. An enabled,
// installed, not-stale, not-loaded plugin is loaded here, which is what a reload
// would do, so its row carries the compositor's own verdict.
func pluginRosterNow() pluginRoster {
	o := loadStore(desktopStorePath())
	comp := liveCompositor()
	hdr, hdrTag := headersABI()
	comp.Headers = hdr.String()
	if !comp.Live && hdr.ok() {
		comp.Version, comp.Commit = hdrTag, hdr.Commit
	}
	target := comp.abi
	if !target.ok() {
		target = hdr
	}
	loaded := loadedPlugins()
	roster := pluginRoster{Compositor: comp, Toolchain: toolchain()}

	seen := map[string]bool{}
	for _, d := range bundledPlugins {
		seen[d.ID] = true
		roster.Plugins = append(roster.Plugins, describePlugin(d, o, target, loaded))
	}
	for _, r := range localReceipts() {
		if seen[r.ID] {
			continue
		}
		seen[r.ID] = true
		roster.Plugins = append(roster.Plugins, describePlugin(receiptDef(r), o, target, loaded))
	}
	for _, p := range hyprpmCopies() {
		id := strings.TrimSuffix(filepath.Base(p), ".so")
		if seen[id] {
			continue
		}
		seen[id] = true
		d := pluginDef{ID: id, Name: id, Plugin: id, Loaded: id, Desc: "Installed by hyprpm from " + filepath.Base(filepath.Dir(p)) + "."}
		roster.Plugins = append(roster.Plugins, describePlugin(d, o, target, loaded))
	}
	return roster
}

// receiptDef turns a local build's receipt into the def the roster describes.
func receiptDef(r pluginReceipt) pluginDef {
	name := r.Name
	if name == "" {
		name = r.ID
	}
	loadedName := r.Loaded
	if loadedName == "" {
		loadedName = r.ID
	}
	return pluginDef{ID: r.ID, Name: name, Desc: r.Desc, Repo: r.Repo, Plugin: r.Plugin, Local: r.Local, Docs: r.Repo, Loaded: loadedName}
}

func describePlugin(d pluginDef, o Overrides, target hyprABI, loaded map[string]loadedPlugin) pluginInfo {
	_, bundled := bundledByID(d.ID)
	info := pluginInfo{
		ID: d.ID, Name: d.Name, Desc: d.Desc, Docs: d.Docs, Sounds: d.Sounds, Repo: d.Repo, Package: d.Package,
		Bundled: bundled, Source: "none", Enabled: pluginEnabled(o, d.ID),
		Rebuildable: d.Repo != "" || d.Local != "",
		Removable:   !bundled,
		Settings:    []detectedSetting{},
	}
	if d.Local != "" {
		if src, ok := localSourceDir(d, checkoutRoot("")); ok {
			if readme := filepath.Join(src, "README.md"); fileExists(readme) {
				info.Docs = readme
			}
		}
	}
	if d.ID == "keysounds" {
		info.UserSounds = keysoundsUserDir()
		info.ShippedSounds = keysoundsShippedDir()
	}
	receipt, hasReceipt := readReceipt(d.ID)
	copies := pluginCopies(d.ID)
	c, ok, stale := pickCopy(copies, target)
	if !ok {
		info.Status, info.Detail = "missing", "Not installed on this machine."
		if d.Package != "" {
			info.Detail = "Not installed: the " + d.Package + " package is missing."
		}
		return info
	}
	info.Installed, info.Path, info.Source, info.ABI = true, c.Path, c.Tier, c.ABI.String()
	if hdr, _ := headersABI(); hdr.ok() {
		need, _ := needsBuild(d, hdr, checkoutRoot(""))
		info.Current = !need
	}
	switch c.Tier {
	case "package":
		info.Version = packageVersion(d.Package)
	case "built":
		if hasReceipt {
			info.Version, info.BuiltAt = shortCommit(receipt.Commit), receipt.BuiltAt
			if receipt.Hyprland != "" {
				info.Version = strings.TrimSpace(info.Version + " for Hyprland " + receipt.Hyprland)
			}
		}
	case "hyprpm":
		info.Version = "hyprpm"
	}

	lp, isLoaded := loaded[d.Loaded]
	if isLoaded && lp.Version != "" {
		info.Version = strings.TrimSpace(info.Version + " (reports " + lp.Version + ")")
	}
	info.Loaded = isLoaded

	if !bundled {
		info.Settings = detectSettings(c.Path, isLoaded, receipt.Settings)
		if hasReceipt && isLoaded && !settingsEqual(receipt.Settings, info.Settings) {
			receipt.Settings = info.Settings
			_ = writeReceipt(receipt)
		}
	}

	switch {
	case isLoaded:
		info.Status, info.Detail = "running", "Loaded in the running compositor."
	case stale:
		info.Status, info.Detail = "stale", abiDiff(c.ABI, target)+". Rebuild it for this Hyprland."
	case info.Enabled && target.ok():
		if err := hyprctlPluginLoad(c.Path); err != nil {
			info.Status, info.Detail = "failed", loadFailureDetail(err)
			if mismatchRe.MatchString(err.Error()) {
				info.Status = "stale"
				info.Detail += ". Rebuild it for this Hyprland."
			}
		} else {
			info.Loaded = true
			info.Status, info.Detail = "running", "Loaded in the running compositor."
			pushEval(genPluginConfig(o))
			if !bundled {
				for name := range loadedPlugins() {
					if _, seen := loaded[name]; !seen {
						receipt.Loaded = name
					}
				}
				info.Settings = detectSettings(c.Path, true, info.Settings)
				if hasReceipt {
					receipt.Settings = info.Settings
					_ = writeReceipt(receipt)
				}
			}
		}
	case info.Enabled:
		info.Status, info.Detail = "ready", "Enabled; loads when the compositor starts."
	default:
		info.Status, info.Detail = "ready", "Installed for this Hyprland. Turn it on and Save to load it."
		if !c.ABI.ok() {
			info.Detail = "Installed. Turn it on and Save to load it."
		}
	}
	if d.ID == "dynamic-cursors" {
		if note := dynamicCursorsNote(info.Loaded, nvidiaGPU()); note != "" {
			info.Detail += " " + note
		}
	}
	return info
}

// dynamicCursorsNote is the honest caveat for the one plugin with a known
// cost on NVIDIA: it forces software cursor rendering, which reports say
// stutters when the pointer crosses monitors (#257, #259). Surfacing it on
// the plugin's own card beats shipping the surprise silently.
func dynamicCursorsNote(loaded, nvidia bool) string {
	if !nvidia {
		return ""
	}
	if loaded {
		return "On NVIDIA this plugin renders the cursor in software, which can stutter when the pointer crosses monitors: turn it off if the desktop feels laggy."
	}
	return "On NVIDIA this plugin forces software cursor rendering and can stutter across monitors."
}

// nvidiaGPU reports an active NVIDIA driver by its device nodes, the same
// signal ryoku-gpu-detect reads; no subprocess.
func nvidiaGPU() bool {
	for _, dev := range []string{"/dev/nvidia0", "/dev/nvidiactl"} {
		if _, err := os.Stat(dev); err == nil {
			return true
		}
	}
	return false
}

func settingsEqual(a, b []detectedSetting) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}
