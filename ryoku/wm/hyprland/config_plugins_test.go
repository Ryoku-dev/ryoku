package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageVersionUsesHostPackageSeam(t *testing.T) {
	bin := t.TempDir()
	host := filepath.Join(bin, "ryoku-host")
	if err := os.WriteFile(host, []byte("#!/bin/sh\n[ \"$1 $2 $3\" = 'pkg version hyprglass' ] || exit 2\nprintf '0:1.2.3-4.fc44\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if got := packageVersion("hyprglass"); got != "0:1.2.3-4.fc44" {
		t.Fatalf("packageVersion = %q", got)
	}
}

// The ABI string a plugin is built against must round-trip exactly as Hyprland
// composes it, and a mismatch must name the library that moved.
func TestABIParseAndDiff(t *testing.T) {
	const s = "efb50993780079460b0cbed1363e2166a2de1d9f_aq_0.15_hu_0.14_hg_0.5_hc_0.1_hlg_0.6"
	a := parseABI(s)
	if !a.ok() || a.String() != s {
		t.Fatalf("parseABI round trip: got %q", a.String())
	}
	if parseABI("garbage").ok() {
		t.Fatal("garbage parsed as an ABI")
	}
	built := parseABI("efb50993780079460b0cbed1363e2166a2de1d9f_aq_0.14_hu_0.14_hg_0.5_hc_0.1_hlg_0.6")
	if got, want := abiDiff(built, a), "built for aquamarine 0.14, running 0.15"; got != want {
		t.Fatalf("abiDiff = %q, want %q", got, want)
	}
	if abiDiff(a, a) != "" {
		t.Fatal("identical ABIs reported a diff")
	}
	other := parseABI("1234567890abcdef_aq_0.15_hu_0.14_hg_0.5_hc_0.1_hlg_0.6")
	if got := abiDiff(other, a); !strings.HasPrefix(got, "built for Hyprland 1234567, running efb5099") {
		t.Fatalf("commit diff = %q", got)
	}
}

// version.h is the build target: its macros must yield the same string the
// compositor reports, patch levels dropped the way the plugin API drops them.
func TestParseVersionHeader(t *testing.T) {
	src := `#pragma once
#define GIT_COMMIT_HASH    "efb50993780079460b0cbed1363e2166a2de1d9f"
#define GIT_TAG            "v0.56.2"
#define AQUAMARINE_VERSION "0.15.0"
#define HYPRLANG_VERSION     "0.6.8"
#define HYPRUTILS_VERSION    "0.14.1"
#define HYPRCURSOR_VERSION   "0.1.13"
#define HYPRGRAPHICS_VERSION "0.5.1"
`
	abi, tag := parseVersionHeader(src)
	if got, want := abi.String(), "efb50993780079460b0cbed1363e2166a2de1d9f_aq_0.15_hu_0.14_hg_0.5_hc_0.1_hlg_0.6"; got != want {
		t.Fatalf("headers ABI = %q, want %q", got, want)
	}
	if tag != "0.56.2" {
		t.Fatalf("tag = %q", tag)
	}
	if abi, _ := parseVersionHeader("nothing here"); abi.ok() {
		t.Fatal("a header without a commit parsed as an ABI")
	}
}

// Which copy settings.lua loads decides whether a plugin works after a Hyprland
// bump: a matching receipt wins over tier order, an unreceipted copy is trusted,
// and an all-stale set is reported stale so the load can be left out.
func TestPickCopy(t *testing.T) {
	target := parseABI("aaaa_aq_0.15_hu_0.14_hg_0.5_hc_0.1_hlg_0.6")
	old := parseABI("aaaa_aq_0.14_hu_0.14_hg_0.5_hc_0.1_hlg_0.6")
	user := soCopy{Path: "/home/u/.local/lib/hyprland/plugins/x.so", Tier: "built", ABI: old}
	pkg := soCopy{Path: "/usr/lib/hyprland/plugins/x.so", Tier: "package", ABI: target}
	if c, ok, stale := pickCopy([]soCopy{user, pkg}, target); !ok || stale || c.Tier != "package" {
		t.Fatalf("stale local build was not passed over for the matching package copy: %+v ok=%v stale=%v", c, ok, stale)
	}
	fresh := user
	fresh.ABI = target
	if c, _, _ := pickCopy([]soCopy{fresh, pkg}, target); c.Tier != "built" {
		t.Fatalf("a matching local build should win over the package copy, got %s", c.Tier)
	}
	untagged := soCopy{Path: user.Path, Tier: "built"}
	if c, ok, stale := pickCopy([]soCopy{untagged}, target); !ok || stale || c.Path != user.Path {
		t.Fatal("a copy without a receipt must be trusted")
	}
	if _, ok, stale := pickCopy([]soCopy{user}, target); !ok || !stale {
		t.Fatal("a lone stale copy must be reported stale")
	}
	if _, ok, _ := pickCopy(nil, target); ok {
		t.Fatal("no copies reported ok")
	}
	if c, ok, stale := pickCopy([]soCopy{user, pkg}, hyprABI{}); !ok || stale || c.Tier != "built" {
		t.Fatal("with no target ABI the first copy is taken as before")
	}
}

// Hyprland's refusal names both ABIs; the detail line must turn that into the
// library that moved, and pass any other reason through untouched.
func TestLoadFailureDetail(t *testing.T) {
	err := errors.New("Plugin /x.so could not be loaded: plugin crashed/threw in main: version mismatch, built against: efb5_aq_0.14_hu_0.14_hg_0.5_hc_0.1_hlg_0.6, running compositor: efb5_aq_0.15_hu_0.14_hg_0.5_hc_0.1_hlg_0.6")
	if got, want := loadFailureDetail(err), "built for aquamarine 0.14, running 0.15"; got != want {
		t.Fatalf("detail = %q, want %q", got, want)
	}
	if got := loadFailureDetail(errors.New("Plugin /x.so could not be loaded: dlopen failed")); got != "dlopen failed" {
		t.Fatalf("plain failure detail = %q", got)
	}
}

// A plugin's config keys survive compilation as string literals; the scan must
// return each once, dropping the plugin: prefix and anything that is not a key.
func TestScanPluginKeys(t *testing.T) {
	dir := t.TempDir()
	so := filepath.Join(dir, "x.so")
	blob := strings.Join([]string{
		"ELF junk", "plugin:hyprexpo:columns", "plugin:hyprexpo:gap_size",
		"plugin:hyprexpo:columns", "plugin:", "plugin:noleaf", "plugin:bad key:x", "plugin:hyprexpo:bg_col",
	}, "\x00")
	if err := os.WriteFile(so, []byte(blob), 0o644); err != nil {
		t.Fatal(err)
	}
	got := scanPluginKeys(so)
	want := []string{"hyprexpo:bg_col", "hyprexpo:columns", "hyprexpo:gap_size"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("keys = %v, want %v", got, want)
	}
}

// hyprpm.toml is the recipe: plugin tables in document order (skipping the
// repository table and any table without an output), the commit pins, and the
// pin lookup by Hyprland commit.
func TestParseHyprpmManifest(t *testing.T) {
	m, err := parseHyprpmManifest(`
[repository]
name = "hyprland-plugins"
authors = ["Vaxry"]
commit_pins = [
    ["1111111111111111111111111111111111111111", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"],
    ["2222222222222222222222222222222222222222", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"],
]

[hyprexpo]
description = "An overview"
authors = ["Vaxry"]
output = "hyprexpo/hyprexpo.so"
build = ["make -C hyprexpo all"]
since_hyprland = 6066

[hyprbars]
description = "Title bars"
output = "hyprbars/hyprbars.so"
build = ["make -C hyprbars all"]
`)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "hyprland-plugins" {
		t.Fatalf("name = %q", m.Name)
	}
	if strings.Join(m.Order, ",") != "hyprexpo,hyprbars" {
		t.Fatalf("order = %v", m.Order)
	}
	if p := m.Plugins["hyprexpo"]; p.Output != "hyprexpo/hyprexpo.so" || len(p.Build) != 1 || p.Description != "An overview" {
		t.Fatalf("hyprexpo = %+v", p)
	}
	if got := m.pinFor("2222222222222222222222222222222222222222"); got != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Fatalf("pin = %q", got)
	}
	if m.pinFor("3333") != "" {
		t.Fatal("an unknown Hyprland commit must have no pin")
	}
	if _, err := parseHyprpmManifest("[repository]\nname = \"x\"\n"); err == nil {
		t.Fatal("a manifest without a plugin table must be rejected")
	}
}

// An added plugin's settings are stored as colon paths with JSON-typed values;
// the Lua must nest them, normalise dashes, keep ints integral, and run behind
// the loaded guard. Nothing set, nothing emitted: the load alone is the block.
func TestPluginBlockConfigLua(t *testing.T) {
	tree := luaConfigTree(map[string]any{
		"hyprexpo:columns":         float64(3),
		"hyprexpo:gap_size":        float64(5.5),
		"hyprexpo:enable-gesture":  true,
		"hyprexpo:gesture:fingers": float64(3),
		"hyprexpo:bg_col":          "rgb(111111)",
	})
	pb := pluginBlock{id: "hyprexpo", name: "hyprexpo", so: "/p/hyprexpo.so", config: renderLuaTable(tree)}
	got := pb.configLua("  ")
	want := "  if ryoku_plugin_loaded(\"hyprexpo\") then\n" +
		"    hl.config({ plugin = { hyprexpo = { bg_col = \"rgb(111111)\", columns = 3, enable_gesture = true, gap_size = 5.5, gesture = { fingers = 3 } } } })\n" +
		"  end\n"
	if got != want {
		t.Fatalf("config:\n got %q\nwant %q", got, want)
	}
	if bare := (pluginBlock{id: "x", name: "x", so: "/p/x.so"}).configLua(""); bare != "" {
		t.Fatalf("a block with nothing to set emitted %q", bare)
	}
	withExtra := pluginBlock{name: "hyprbars", config: "{ hyprbars = { enabled = true } }", extra: "  add()\n"}.configLua("")
	if withExtra != "if ryoku_plugin_loaded(\"hyprbars\") then\n  hl.config({ plugin = { hyprbars = { enabled = true } } })\n  add()\nend\n" {
		t.Fatalf("extra block = %q", withExtra)
	}
}

// A session starts with no plugins loaded, so the guarded config beside a load
// declaration cannot apply its values on the first config pass. The startup
// callback must run after that declaration and reconcile the saved values.
func TestGenPluginsSyncsSavedValuesAtStartup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	plugin := filepath.Join(home, ".local", "lib", "hyprland", "plugins", "hyprglass.so")
	if err := os.MkdirAll(filepath.Dir(plugin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plugin, nil, 0o755); err != nil {
		t.Fatal(err)
	}

	o := defaultOverrides()
	o.Plugins.Hyprglass.Enabled = true
	got := genPlugins(o)
	loadAt := strings.Index(got, `hl.plugin.load("`)
	configAt := strings.Index(got, `if ryoku_plugin_loaded("hyprglass")`)
	syncAt := strings.Index(got, `hl.on("hyprland.start"`)
	if loadAt < 0 || configAt < 0 || syncAt < 0 {
		t.Fatalf("generated plugin startup block is incomplete:\n%s", got)
	}
	if !(loadAt < configAt && configAt < syncAt) {
		t.Fatalf("plugin startup order is load=%d config=%d sync=%d:\n%s", loadAt, configAt, syncAt, got)
	}
	if strings.Count(got, "ryoku-wm-hyprland plugins sync") != 1 {
		t.Fatalf("startup sync count is not one:\n%s", got)
	}
}

// Hyprland turns ':' into '.' and '-' into '_' when it registers plugin
// options with the Lua config provider. The section names must follow that Lua
// namespace, not the plugin filename.
func TestBundledPluginLuaNamespaces(t *testing.T) {
	for id, want := range map[string]string{
		"dynamic-cursors":   "dynamic_cursors",
		"borders-plus-plus": "borders_plus_plus",
		"hyprglass":         "hyprglass",
		"hyprgrass":         "hyprgrass",
		"imgborders":        "imgborders",
	} {
		d, ok := bundledByID(id)
		if !ok {
			t.Fatalf("%s missing from bundled plugins", id)
		}
		if d.ConfigSection != want {
			t.Errorf("%s config section = %q, want %q", id, d.ConfigSection, want)
		}
	}
}

func TestPluginLoadLuaReportsErrors(t *testing.T) {
	pb := pluginBlock{
		id: "dynamic-cursors", name: "dynamic-cursors", so: "/p/dynamic-cursors.so",
		config: "{ dynamic_cursors = { mode = \"rotate\", shake = { enabled = false } } }",
	}
	got := pb.loadLua()
	want := "do\n" +
		"  local ok, err = pcall(function()\n" +
		"    hl.plugin.load(\"/p/dynamic-cursors.so\")\n" +
		"    if ryoku_plugin_loaded(\"dynamic-cursors\") then\n" +
		"      hl.config({ plugin = { dynamic_cursors = { mode = \"rotate\", shake = { enabled = false } } } })\n" +
		"    end\n" +
		"  end)\n" +
		"  if not ok then\n" +
		"    print(\"Ryoku plugin dynamic-cursors: \" .. tostring(err))\n" +
		"  end\n" +
		"end\n\n"
	if got != want {
		t.Fatalf("load block:\n got %q\nwant %q", got, want)
	}
}

// Every bundled plugin's toggle has to reach the roster through the store, or a
// plugin the user enabled would never be loaded by the status probe.
func TestPluginEnabledCoversBundled(t *testing.T) {
	o := defaultOverrides()
	o.Plugins.DynamicCursors.Enabled = true
	o.Plugins.Hyprbars.Enabled = true
	o.Plugins.Imgborders.Enabled = true
	o.Plugins.Hyprglass.Enabled = true
	o.Plugins.Hyprfocus.Enabled = true
	o.Plugins.Keysounds.Enabled = true
	o.Plugins.BordersPlusPlus.Enabled = true
	o.Plugins.Hyprgrass.Enabled = true
	for _, d := range bundledPlugins {
		if !pluginEnabled(o, d.ID) {
			t.Fatalf("%s enabled in the store but not seen by pluginEnabled", d.ID)
		}
	}
}

func TestEnabledPluginReportsUnreachableCompositor(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".local", "lib", "hyprland", "plugins")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	const id = "offline-test"
	so := filepath.Join(dir, id+".so")
	if err := os.WriteFile(so, []byte("plugin:offline-test:enabled"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := parseABI("aaaa_aq_0.15_hu_0.14_hg_0.5_hc_0.1_hlg_0.6")
	if err := os.WriteFile(filepath.Join(dir, id+".abi"), []byte(target.String()+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := defaultOverrides()
	o.Plugins.Extra = map[string]ExtraPlugin{id: {Enabled: true}}
	info := describePlugin(
		pluginDef{ID: id, Name: "Offline test", Loaded: id},
		o, target, map[string]loadedPlugin{}, false,
	)
	if info.Status != "unreachable" || !strings.Contains(info.Detail, "not reachable") {
		t.Fatalf("offline status = %q, detail = %q", info.Status, info.Detail)
	}
}

func TestRepoSlug(t *testing.T) {
	cases := map[string]string{
		"https://github.com/hyprwm/hyprland-plugins":      "github.com-hyprwm-hyprland-plugins",
		"https://github.com/hyprwm/hyprland-plugins.git/": "github.com-hyprwm-hyprland-plugins",
		"git@codeberg.org:zacoons/imgborders.git":         "codeberg.org-zacoons-imgborders",
	}
	for in, want := range cases {
		if got := repoSlug(in); got != want {
			t.Fatalf("repoSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

// The NVIDIA caveats ride on the plugin cards: the cursor plugin and the
// glass plugin each carry a known cost on NVIDIA, and both must be surfaced
// whether or not the plugin is currently loaded. Any other plugin, and any
// non-NVIDIA machine, carries no note.
func TestPluginNvidiaNote(t *testing.T) {
	if got := pluginNvidiaNote("hyprbars", true, false); got != "" {
		t.Fatalf("note on non-NVIDIA: %q", got)
	}
	if got := pluginNvidiaNote("hyprbars", true, true); got != "" {
		t.Fatalf("note on an unremarkable plugin: %q", got)
	}
	for _, c := range []struct {
		id   string
		want string
	}{
		{"dynamic-cursors", "software"},
		{"hyprglass", "blur"},
	} {
		loaded := pluginNvidiaNote(c.id, true, true)
		unloaded := pluginNvidiaNote(c.id, false, true)
		if !strings.Contains(loaded, c.want) {
			t.Errorf("%s loaded note = %q, want it to mention %q", c.id, loaded, c.want)
		}
		if unloaded == "" || unloaded == loaded {
			t.Errorf("%s unloaded note = %q, want a distinct warning", c.id, unloaded)
		}
	}
}
