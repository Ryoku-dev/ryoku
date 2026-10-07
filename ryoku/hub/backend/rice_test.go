package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeDesktopStore lays flat hypr sections into the namespaced desktop.json the
// store now uses, so a test can keep writing the sections it cares about.
func writeDesktopStore(t *testing.T, flat string) {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(flat), &m); err != nil {
		t.Fatal(err)
	}
	if err := setHyprSections(m); err != nil {
		t.Fatal(err)
	}
}

// A snapshot captures shell.json whole and applies it whole: omit drops only
// the denylisted personal / regional keys (so a new look key travels
// automatically), and a nil-allowlist overlay sets every captured key while
// leaving a key the rice never carried untouched.
func TestShellCaptureOmitsPersonalAndOverlaysWhole(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	p := shellStorePath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{"barStyle":"noctalia","frameBars":{"style":"slate-frame"},"weatherLocation":"Oslo","fontScale":1.3}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// capture: the whole look travels, the personal key is held back.
	look := omit(readJSONMap(p), riceShellOmit)
	if look["barStyle"] != "noctalia" || look["fontScale"] == nil {
		t.Fatalf("omit dropped a look key: %v", look)
	}
	if _, ok := look["weatherLocation"]; ok {
		t.Fatal("omit leaked a personal key")
	}

	// apply onto a recipient: every captured key lands, and a personal key the
	// rice never carried survives.
	recipient := filepath.Join(dir, "recipient.json")
	if err := os.WriteFile(recipient, []byte(`{"barStyle":"caelestia","weatherLocation":"Berlin"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := overlayStore(recipient, look, nil); err != nil {
		t.Fatal(err)
	}
	got := readJSONMap(recipient)
	if got["barStyle"] != "noctalia" {
		t.Fatalf("overlay did not set barStyle: %v", got["barStyle"])
	}
	if got["frameBars"].(map[string]any)["style"] != "slate-frame" {
		t.Fatalf("overlay did not carry frameBars: %v", got["frameBars"])
	}
	if got["weatherLocation"] != "Berlin" {
		t.Fatalf("overlay clobbered a key the rice never carried: %v", got["weatherLocation"])
	}
}

// a rice round-trips through save/load, and listRices skips the reserved backup
// slots so they never appear as browsable rices.
func TestSaveLoadListRice(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("RYOKU_SYSTEM_RICES_DIR", t.TempDir())

	r := Rice{
		Schema: riceSchema, Slug: "demo", Name: "Demo", CreatedWith: "0.6.8",
		Color: RiceColor{Mode: "wallpaper"},
		Look:  map[string]map[string]any{"shell": {"frameBars": map[string]any{"style": "ryoku-frame"}}},
	}
	if err := saveRice(r); err != nil {
		t.Fatal(err)
	}
	got, _, err := loadRice("demo")
	if err != nil {
		t.Fatal(err)
	}
	if got.Look["shell"]["frameBars"].(map[string]any)["style"] != "ryoku-frame" {
		t.Fatalf("round-trip lost look: %v", got.Look)
	}

	if err := os.MkdirAll(filepath.Join(ricesDir(), ".baseline"), 0o755); err != nil {
		t.Fatal(err)
	}
	ls := listRices()
	if len(ls) != 1 || ls[0].Slug != "demo" {
		t.Fatalf("listRices = %v, want [demo] (reserved slot skipped)", ls)
	}
}

func writeRiceManifest(t *testing.T, dir string, r Rice) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rice.json"), mustJSON(r), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSystemRicesListWithUserOverride(t *testing.T) {
	config := t.TempDir()
	system := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("RYOKU_SYSTEM_RICES_DIR", system)

	writeRiceManifest(t, filepath.Join(system, "default"), Rice{
		Schema: riceSchema, Slug: "default", Name: "Packaged", Look: map[string]map[string]any{},
	})
	writeRiceManifest(t, filepath.Join(system, "system-only"), Rice{
		Schema: riceSchema, Slug: "system-only", Name: "System only", Look: map[string]map[string]any{},
	})
	writeRiceManifest(t, filepath.Join(ricesDir(), "default"), Rice{
		Schema: riceSchema, Slug: "default", Name: "User", Look: map[string]map[string]any{},
	})

	got := listRices()
	if len(got) != 2 {
		t.Fatalf("listRices returned %d entries: %v", len(got), got)
	}
	if got[0].Slug != "default" || got[0].Name != "User" {
		t.Fatalf("user rice did not override packaged slug: %+v", got[0])
	}
	if got[1].Slug != "system-only" {
		t.Fatalf("packaged-only rice missing: %+v", got)
	}
}

func TestApplyingSystemRiceCopiesItToUserStore(t *testing.T) {
	home := t.TempDir()
	system := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(home, "run"))
	t.Setenv("RYOKU_SYSTEM_RICES_DIR", system)

	r := Rice{
		Schema: riceSchema, Slug: "default", Name: "Default",
		Color: RiceColor{Mode: "wallpaper"},
		Assets: RiceAssets{
			FastfetchStyle: "fastfetch.jsonc",
		},
		Look: map[string]map[string]any{
			"hypr":     {},
			"shell":    {"barStyle": "qsbar"},
			"launcher": {},
		},
	}
	systemDir := filepath.Join(system, "default")
	writeRiceManifest(t, systemDir, r)
	if err := os.WriteFile(filepath.Join(systemDir, "fastfetch.jsonc"), []byte(`{"modules":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var calls []string
	origRun, origReload := riceRun, riceReload
	riceRun = func(name string, args ...string) error {
		calls = append(calls, strings.Join(append([]string{name}, args...), " "))
		return nil
	}
	riceReload = func() {}
	t.Cleanup(func() { riceRun, riceReload = origRun, origReload })

	if err := applyRice("default", nil); err != nil {
		t.Fatal(err)
	}
	userDir := filepath.Join(ricesDir(), "default")
	if !isFile(filepath.Join(userDir, "rice.json")) || !isFile(filepath.Join(userDir, "fastfetch.jsonc")) {
		t.Fatalf("packaged rice was not copied into %s", userDir)
	}
	for _, call := range calls {
		if strings.Contains(call, "wallpaper set") {
			t.Fatalf("wallpaper-free packaged rice changed the wallpaper: %v", calls)
		}
	}
}

func TestRiceInputLayerPreservesInstallerKeyboard(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(home, "run"))
	t.Setenv("RYOKU_SYSTEM_RICES_DIR", t.TempDir())
	writeDesktopStore(t, `{"input":{"kbLayout":"gb","kbVariant":"intl","kbOptions":"caps:escape","naturalScroll":false}}`)
	r := Rice{
		Schema: riceSchema, Slug: "portable", Name: "Portable",
		Color: RiceColor{Mode: "wallpaper"},
		Look: map[string]map[string]any{
			"hypr": {}, "shell": {}, "launcher": {},
		},
		Layers: map[string]json.RawMessage{
			"input": json.RawMessage(`{"naturalScroll":true,"tapToClick":true}`),
		},
	}
	if err := saveRice(r); err != nil {
		t.Fatal(err)
	}

	origRun, origReload := riceRun, riceReload
	riceRun = func(string, ...string) error { return nil }
	riceReload = func() {}
	t.Cleanup(func() { riceRun, riceReload = origRun, origReload })

	if err := applyRice("portable", []string{"input"}); err != nil {
		t.Fatal(err)
	}
	input := readHyprSections()["input"].(map[string]any)
	if input["kbLayout"] != "gb" || input["kbVariant"] != "intl" || input["kbOptions"] != "caps:escape" {
		t.Fatalf("portable input layer clobbered installer keyboard fields: %v", input)
	}
	if input["naturalScroll"] != true || input["tapToClick"] != true {
		t.Fatalf("portable input settings did not apply: %v", input)
	}
}

func TestRiceWindowSectionsSelectActiveProvider(t *testing.T) {
	r := Rice{Look: map[string]map[string]any{
		"hypr": {
			"appearance": map[string]any{"rounding": float64(8)},
			"cursor":     map[string]any{"size": float64(24)},
		},
		"wm": {
			"first": map[string]any{
				"appearance": map[string]any{"blurEnabled": false},
				"anim":       map[string]any{"preset": "first"},
			},
			"second": map[string]any{
				"appearance": map[string]any{"blurEnabled": true},
				"frame":      "second",
			},
		},
	}}

	first := riceWindowSections(r, "first")
	firstAppearance := first["appearance"].(map[string]any)
	if firstAppearance["rounding"] != float64(8) || firstAppearance["blurEnabled"] != false {
		t.Fatalf("shared and active sections did not merge: %v", first)
	}
	if _, ok := first["frame"]; ok {
		t.Fatalf("inactive provider section leaked: %v", first)
	}

	second := riceWindowSections(r, "second")
	if second["frame"] != "second" {
		t.Fatalf("active provider section missing: %v", second)
	}
	if _, ok := second["anim"]; ok {
		t.Fatalf("other provider animation leaked: %v", second)
	}
	if second["cursor"].(map[string]any)["size"] != float64(24) {
		t.Fatalf("shared section missing for second provider: %v", second)
	}
}

func TestRiceWindowSectionsLegacyManifestIsUnchanged(t *testing.T) {
	shared := map[string]any{
		"appearance": map[string]any{"rounding": float64(8)},
		"anim":       map[string]any{"speed": float64(4)},
	}
	r := Rice{Look: map[string]map[string]any{"hypr": shared}}
	if got := riceWindowSections(r, "anything"); !reflect.DeepEqual(got, shared) {
		t.Fatalf("legacy rice changed: got %v want %v", got, shared)
	}
}

// captureRice pulls the whole shell / launcher look (minus the personal /
// regional denylist), the hypr look sections, and the theme master into look,
// routes behavior keys to layers only when opted in, and records the cursor by
// name. personal keys (weatherLocation) never travel; a plain look key does.
func TestRiceCapture(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "ryoku"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(p, body string) {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeDesktopStore(t, `{"appearance":{"rounding":10},"cursor":{"theme":"Bibata-Modern-Ice","size":24},"input":{"sensitivity":0.2}}`)
	write(shellStorePath(), `{"frameBars":{"style":"ryoku-frame"},"weatherLocation":"Oslo","sidebarWidth":360}`)
	write(launcherStorePath(), `{"heroStrength":0.5,"showWeather":true}`)
	write(themeStatePath(), `{"followWallpaper":true}`)

	r, err := captureRice("My Rice", nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Slug != "my-rice" {
		t.Fatalf("slug = %q, want my-rice", r.Slug)
	}
	if r.Look["hypr"]["appearance"] == nil {
		t.Fatal("appearance look missing")
	}
	if _, ok := r.Look["hypr"]["input"]; ok {
		t.Fatal("input leaked into look (it is a behavior layer)")
	}
	if frameBars, ok := r.Look["shell"]["frameBars"].(map[string]any); !ok || frameBars["style"] != "ryoku-frame" {
		t.Fatalf("shell frameBars = %v, want ryoku-frame", r.Look["shell"]["frameBars"])
	}
	if _, ok := r.Look["shell"]["weatherLocation"]; ok {
		t.Fatal("personal key weatherLocation captured")
	}
	if _, ok := r.Look["shell"]["sidebarWidth"]; !ok {
		t.Fatal("a plain look key must travel in the whole-store snapshot")
	}
	if r.Assets.Cursor != "Bibata-Modern-Ice" {
		t.Fatalf("cursor = %q", r.Assets.Cursor)
	}
	if r.Color.Mode != "wallpaper" {
		t.Fatalf("colour mode = %q, want wallpaper", r.Color.Mode)
	}
	if r.Layers != nil {
		t.Fatalf("no layers requested but got %v", r.Layers)
	}

	r2, err := captureRice("With Layer", []string{"input"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r2.Layers["input"]; !ok {
		t.Fatalf("input layer not captured: %v", r2.Layers)
	}
}

// applyRice overlays the captured look keys onto the live stores (a key the
// rice never carried survives), flips the colour master for a fixed rice and
// writes its palette, and reloads. restoreRice(".baseline") then reverts every
// store to the pristine pre-apply snapshot.
func TestRiceApplyMergesAndRestoreReverts(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	t.Setenv("HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "ryoku"), 0o755); err != nil {
		t.Fatal(err)
	}

	var calls []string
	origRun, origReload := riceRun, riceReload
	riceRun = func(name string, args ...string) error { calls = append(calls, name); return nil }
	riceReload = func() { calls = append(calls, "reload") }
	t.Cleanup(func() { riceRun, riceReload = origRun, origReload })

	w := func(p, body string) {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeDesktopStore(t, `{"appearance":{"rounding":2},"cursor":{"theme":"Bibata-Modern-Ice","size":24}}`)
	w(shellStorePath(), `{"frameBars":{"style":"slate-frame","rails":{"top":{"size":44}}},"weatherLocation":"Oslo","sidebarWidth":360}`)
	w(launcherStorePath(), `{"heroStrength":0.6}`)
	w(themeStatePath(), `{"followWallpaper":true}`)

	if err := os.MkdirAll(filepath.Join(ricesDir(), "cool"), 0o755); err != nil {
		t.Fatal(err)
	}
	w(filepath.Join(ricesDir(), "cool", "palette.json"), `{"background":"#101010","color4":"#ff8800","foreground":"#eeeeee"}`)
	w(ricePath("cool"), `{"schema":1,"slug":"cool","name":"Cool","color":{"mode":"fixed","palette":"palette.json"},"look":{"hypr":{"appearance":{"rounding":18}},"shell":{"frameBars":{"style":"ryoku-frame","rails":{"top":{"size":38}}}},"launcher":{}}}`)
	if err := applyRice("cool", nil); err != nil {
		t.Fatal(err)
	}
	shell := readJSONMap(shellStorePath())
	frameBars := shell["frameBars"].(map[string]any)
	if frameBars["style"] != "ryoku-frame" {
		t.Fatalf("apply did not set frameBars.style: %v", frameBars["style"])
	}
	if shell["sidebarWidth"] != float64(360) {
		t.Fatal("apply clobbered a personal sidebar value")
	}
	ap := readHyprSections()["appearance"].(map[string]any)
	if ap["rounding"].(float64) != 18 {
		t.Fatalf("apply rounding = %v, want 18", ap["rounding"])
	}
	if loadThemeState().FollowWallpaper {
		t.Fatal("a fixed rice must turn off followWallpaper")
	}
	if !isFile(filepath.Join(dir, "cache", "ryoku", "colors.json")) {
		t.Fatal("fixed palette not written to the ryoku cache")
	}
	reloaded := false
	for _, c := range calls {
		if c == "reload" {
			reloaded = true
		}
	}
	if !reloaded {
		t.Fatalf("apply did not reload; calls = %v", calls)
	}

	if err := restoreRice(".baseline"); err != nil {
		t.Fatal(err)
	}
	shell2 := readJSONMap(shellStorePath())
	frameBars2 := shell2["frameBars"].(map[string]any)
	if frameBars2["style"] != "slate-frame" {
		t.Fatalf("restore did not revert frameBars.style: %v", frameBars2["style"])
	}
	if !loadThemeState().FollowWallpaper {
		t.Fatal("restore did not revert followWallpaper")
	}
}

// themeApps travels with a rice: capture records the author's app-theming
// choice, apply sets it on the recipient, and an older rice (no themeApps key)
// leaves the recipient's own setting alone. themeAppsOn defaults to on when the
// key is absent, so existing installs keep their themed apps.
func TestThemeAppsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	t.Setenv("HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "ryoku"), 0o755); err != nil {
		t.Fatal(err)
	}
	origRun, origReload := riceRun, riceReload
	riceRun = func(name string, args ...string) error { return nil }
	riceReload = func() {}
	t.Cleanup(func() { riceRun, riceReload = origRun, origReload })
	w := func(p, body string) {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if !themeAppsOn(loadThemeState()) {
		t.Fatal("themeApps absent should read as on")
	}

	// capture with the toggle off records the choice on the rice.
	writeDesktopStore(t, `{"appearance":{"rounding":2}}`)
	w(shellStorePath(), `{"frameBars":{"style":"slate-frame"}}`)
	w(launcherStorePath(), `{}`)
	w(themeStatePath(), `{"followWallpaper":true,"themeApps":false}`)
	r, err := captureRice("Bare", nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Color.ThemeApps == nil || *r.Color.ThemeApps {
		t.Fatalf("capture did not record themeApps=false: %v", r.Color.ThemeApps)
	}

	// a recipient with the toggle on adopts the rice's off.
	w(themeStatePath(), `{"followWallpaper":true,"themeApps":true}`)
	if err := applyRice("bare", nil); err != nil {
		t.Fatal(err)
	}
	if themeAppsOn(loadThemeState()) {
		t.Fatal("apply did not carry themeApps=false onto the recipient")
	}

	// an older rice without the key must not clear the recipient's setting.
	w(themeStatePath(), `{"followWallpaper":true,"themeApps":true}`)
	if err := os.MkdirAll(filepath.Join(ricesDir(), "legacy"), 0o755); err != nil {
		t.Fatal(err)
	}
	w(ricePath("legacy"), `{"schema":1,"slug":"legacy","name":"Legacy","color":{"mode":"wallpaper"},"look":{"hypr":{},"shell":{},"launcher":{}}}`)
	if err := applyRice("legacy", nil); err != nil {
		t.Fatal(err)
	}
	if !themeAppsOn(loadThemeState()) {
		t.Fatal("a rice without themeApps must not clear the recipient's setting")
	}
}

// riceTouches flags which config stores a rice provides and includes the fixed
// palette outputs and bundled assets; exportRice lays the manifest, a per-store
// configs breakout, and a README into a plain folder.
func TestRiceTouchesAndExport(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	r := Rice{
		Schema: riceSchema, Slug: "demo", Name: "Demo", Author: "Ryoku Team",
		CreatedWith: "0.6.8",
		Color:       RiceColor{Mode: "fixed", Palette: "palette.json"},
		Assets:      RiceAssets{Wallpaper: "wall.png"},
		Look:        map[string]map[string]any{"hypr": {"rounding": 3.0}, "shell": {"frameBars": map[string]any{"style": "ryoku-frame"}}},
	}
	rdir := filepath.Join(ricesDir(), "demo")
	if err := os.MkdirAll(rdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := saveRice(r); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rdir, "palette.json"), []byte(`{"color0":"#000"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rdir, "wall.png"), []byte("img"), 0o644); err != nil {
		t.Fatal(err)
	}

	touches := riceTouches(r, rdir)
	find := func(sub string) *riceTouch {
		for i := range touches {
			if strings.Contains(touches[i].Path, sub) || strings.Contains(touches[i].Label, sub) {
				return &touches[i]
			}
		}
		return nil
	}
	if h := find("hypr.json"); h == nil || !h.Provided {
		t.Fatalf("hypr.json not reported as provided: %+v", h)
	}
	if l := find("launcher.json"); l == nil || l.Provided {
		t.Fatalf("launcher.json should be present but not provided: %+v", l)
	}
	if find("colors.json") == nil {
		t.Fatal("fixed rice should touch the ryoku palette cache")
	}
	if find("Desktop wallpaper") == nil {
		t.Fatal("bundled wallpaper not reported")
	}

	dest := t.TempDir()
	out, err := exportRice("demo", dest)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"rice.json", "palette.json", "wall.png", "README.txt", filepath.Join("configs", "hypr.json"), filepath.Join("configs", "shell.json")} {
		if !isFile(filepath.Join(out, f)) {
			t.Fatalf("export missing %s", f)
		}
	}
}

// captureRice with "all" pulls every non-empty behavior layer (keybinds
// included) while skipping empty sections, and riceTouches then lists them so
// the setup is fully installable and removable.
func TestCaptureAllLayersAndTouches(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "ryoku"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeDesktopStore(t, `{"appearance":{"rounding":0},"keybinds":[{"keys":"SUPER + T","action":"exec","value":"kitty"}],"windowRules":[{"class":"Spotify","action":"float"}],"input":[]}`)
	if err := os.WriteFile(themeStatePath(), []byte(`{"followWallpaper":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := captureRice("Full", []string{"all"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Layers["keybinds"]; !ok {
		t.Fatalf("keybinds not captured by full capture: %v", r.Layers)
	}
	if _, ok := r.Layers["windowRules"]; !ok {
		t.Fatalf("windowRules not captured by full capture: %v", r.Layers)
	}
	if _, ok := r.Layers["input"]; ok {
		t.Fatal("empty input layer should be skipped, not captured")
	}

	touches := riceTouches(r, filepath.Join(ricesDir(), r.Slug))
	hasKeybinds := false
	for _, tc := range touches {
		if tc.Label == "Keybinds" {
			hasKeybinds = true
		}
	}
	if !hasKeybinds {
		t.Fatal("touches list should include the captured Keybinds config")
	}
}

// a rice carrying a square shell shape and a keybinds layer installs both (the
// frame, island and chrome go square alongside the windows, and the keybind
// lands), and restore removes them, so every config round-trips end to end.
func TestSquareShellAndKeybindRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	t.Setenv("HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "ryoku"), 0o755); err != nil {
		t.Fatal(err)
	}
	origRun, origReload := riceRun, riceReload
	riceRun = func(name string, args ...string) error { return nil }
	riceReload = func() {}
	t.Cleanup(func() { riceRun, riceReload = origRun, origReload })

	w := func(p, body string) {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeDesktopStore(t, `{"appearance":{"rounding":12},"keybinds":[]}`)
	w(shellStorePath(), `{"frameBars":{"style":"slate-frame"},"frameRadius":9,"roundness":10,"osdRadius":12}`)
	w(launcherStorePath(), `{}`)
	w(themeStatePath(), `{"followWallpaper":true}`)

	if err := os.MkdirAll(filepath.Join(ricesDir(), "sq"), 0o755); err != nil {
		t.Fatal(err)
	}
	w(ricePath("sq"), `{"schema":1,"slug":"sq","name":"Square","color":{"mode":"wallpaper"},"look":{"hypr":{"appearance":{"rounding":0}},"shell":{"frameRadius":0,"roundness":0,"osdRadius":0},"launcher":{}},"layers":{"keybinds":[{"keys":"SUPER + T","action":"exec","value":"kitty"}]}}`)

	if err := applyRice("sq", []string{"keybinds"}); err != nil {
		t.Fatal(err)
	}
	shell := readJSONMap(shellStorePath())
	for _, k := range []string{"frameRadius", "roundness", "osdRadius"} {
		if v, ok := shell[k].(float64); !ok || v != 0 {
			t.Fatalf("square rice did not set shell %s to 0: %v", k, shell[k])
		}
	}
	ap := readHyprSections()["appearance"].(map[string]any)
	if ap["rounding"].(float64) != 0 {
		t.Fatalf("square rice did not set window rounding to 0: %v", ap["rounding"])
	}
	if kb, _ := readHyprSections()["keybinds"].([]any); len(kb) != 1 {
		t.Fatalf("keybinds layer not installed: %v", readHyprSections()["keybinds"])
	}

	if err := restoreRice(".baseline"); err != nil {
		t.Fatal(err)
	}
	shell2 := readJSONMap(shellStorePath())
	if shell2["frameRadius"].(float64) != 9 {
		t.Fatalf("restore did not revert the frame shape: %v", shell2)
	}
	if kb2, _ := readHyprSections()["keybinds"].([]any); len(kb2) != 0 {
		t.Fatalf("restore did not remove the installed keybind: %v", readHyprSections()["keybinds"])
	}
}

// the widened snapshot: widgets/visualizer/decor travel as whole-store look,
// a decor's picture is bundled and its src rewritten to rice://, the brand
// layer bundles its mark, and a live (video) wallpaper is recorded without
// ever being handed to the tile <Image> as a preview.
func TestCaptureNewStoresDecorsAndLiveWall(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	t.Setenv("HOME", dir)
	t.Setenv("RYOKU_SYSTEM_RICES_DIR", t.TempDir())
	for _, d := range []string{filepath.Join(dir, "ryoku"), filepath.Join(dir, "state"), filepath.Join(dir, "pics")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	w := func(p, body string) {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	decorPic := filepath.Join(dir, "pics", "cat.png")
	mark := filepath.Join(dir, "pics", "mark.png")
	clip := filepath.Join(dir, "pics", "wave.mp4")
	w(decorPic, "PNG")
	w(mark, "PNG")
	w(clip, "MP4")
	writeDesktopStore(t, `{"appearance":{"rounding":8}}`)
	w(shellStorePath(), `{"frameBars":{"style":"ryoku-frame"}}`)
	w(launcherStorePath(), `{"bgBlur":0.4,"radius":22}`)
	w(themeStatePath(), `{"followWallpaper":true}`)
	w(widgetsStorePath(), `{"clock24h":true,"clockScale":1.2}`)
	w(visualizerStorePath(), `{"bars":116,"style":"bars"}`)
	w(decorStorePath(), `{"input.touchpad":{"shot":6,"zoom":1,"src":"file://`+decorPic+`"},"input.map":{"shot":10,"src":""}}`)
	w(brandStorePath(), `{"name":"Berserk","markText":"力","markImage":"`+mark+`"}`)
	w(filepath.Join(dir, "state", "ryoku-wallpaper"), clip+"\n")

	r, err := captureRice("Wave", []string{"all"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Look["widgets"]["clock24h"] != true {
		t.Fatalf("widgets look missing: %v", r.Look["widgets"])
	}
	if r.Look["visualizer"]["style"] != "bars" {
		t.Fatalf("visualizer look missing: %v", r.Look["visualizer"])
	}
	if r.Look["launcher"]["bgBlur"] != 0.4 {
		t.Fatalf("launcher bgBlur not captured: %v", r.Look["launcher"])
	}
	tp := r.Look["decor"]["input.touchpad"].(map[string]any)
	src, _ := tp["src"].(string)
	if !strings.HasPrefix(src, "rice://decor-") {
		t.Fatalf("decor src not bundled: %q", src)
	}
	if !isFile(filepath.Join(ricesDir(), "wave", strings.TrimPrefix(src, "rice://"))) {
		t.Fatal("bundled decor picture missing from the rice folder")
	}
	raw, ok := r.Layers["brand"]
	if !ok {
		t.Fatalf("brand layer not captured with all: %v", r.Layers)
	}
	if !strings.Contains(string(raw), "rice://brandmark") {
		t.Fatalf("brand mark not bundled: %s", raw)
	}
	if !isFile(filepath.Join(ricesDir(), "wave", "brandmark.png")) {
		t.Fatal("bundled brand mark missing from the rice folder")
	}
	if r.Assets.Wallpaper != "wall.mp4" {
		t.Fatalf("wallpaper asset = %q, want wall.mp4", r.Assets.Wallpaper)
	}
	es := listRiceEntries()
	if len(es) != 1 || !es[0].Live {
		t.Fatalf("live flag not set: %+v", es)
	}
	if strings.HasSuffix(es[0].Preview, ".mp4") {
		t.Fatalf("a video must never be the tile preview: %q", es[0].Preview)
	}

	// preflight mirrors the same coverage before the save.
	pf := preflightData()
	if pf["live"] != true || pf["decors"] != 2 || pf["widgets"] != true {
		t.Fatalf("preflight = %v", pf)
	}
}

// applying a rice with the widened look writes the new stores, lands decor
// and brand pictures under rice-assets (never pointing into the rices dir),
// routes a video wallpaper to the livewalls pool, and .previous snapshots the
// new stores so restore reverts them.
func TestApplyNewStoresBrandAndVideoWall(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	t.Setenv("HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "ryoku"), 0o755); err != nil {
		t.Fatal(err)
	}
	var calls []string
	origRun, origReload := riceRun, riceReload
	riceRun = func(name string, args ...string) error {
		calls = append(calls, name+" "+strings.Join(args, " "))
		return nil
	}
	riceReload = func() {}
	t.Cleanup(func() { riceRun, riceReload = origRun, origReload })

	w := func(p, body string) {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeDesktopStore(t, `{"appearance":{"rounding":2}}`)
	w(shellStorePath(), `{"frameBars":{"style":"slate-frame"}}`)
	w(launcherStorePath(), `{}`)
	w(themeStatePath(), `{"followWallpaper":true}`)
	w(widgetsStorePath(), `{"clock24h":false}`)
	w(visualizerStorePath(), `{"bars":32}`)
	w(decorStorePath(), `{}`)
	w(brandStorePath(), `{"name":"Old"}`)

	rdir := filepath.Join(ricesDir(), "wave")
	if err := os.MkdirAll(rdir, 0o755); err != nil {
		t.Fatal(err)
	}
	w(filepath.Join(rdir, "wall.mp4"), "MP4")
	w(filepath.Join(rdir, "decor-input-touchpad.png"), "PNG")
	w(filepath.Join(rdir, "brandmark.png"), "PNG")
	w(ricePath("wave"), `{"schema":1,"slug":"wave","name":"Wave",
	  "color":{"mode":"wallpaper"},
	  "assets":{"wallpaper":"wall.mp4"},
	  "look":{"hypr":{"appearance":{"rounding":18}},"shell":{"frameBars":{"style":"ryoku-frame"}},"launcher":{},
	          "widgets":{"clock24h":true},"visualizer":{"bars":116},
	          "decor":{"input.touchpad":{"shot":6,"src":"rice://decor-input-touchpad.png"}}},
	  "layers":{"brand":{"name":"Berserk","markImage":"rice://brandmark.png"}}}`)

	if err := applyRice("wave", []string{"brand"}); err != nil {
		t.Fatal(err)
	}
	if readJSONMap(widgetsStorePath())["clock24h"] != true {
		t.Fatal("widgets look not applied")
	}
	if readJSONMap(visualizerStorePath())["bars"].(float64) != 116 {
		t.Fatal("visualizer look not applied")
	}
	dec := readJSONMap(decorStorePath())["input.touchpad"].(map[string]any)
	src, _ := dec["src"].(string)
	if !strings.HasPrefix(src, "file://"+filepath.Join(dir, "ryoku", "rice-assets", "wave")) {
		t.Fatalf("decor src not rehydrated under rice-assets: %q", src)
	}
	if !isFile(strings.TrimPrefix(src, "file://")) {
		t.Fatal("rehydrated decor picture missing on disk")
	}
	br := readJSONMap(brandStorePath())
	if br["name"] != "Berserk" {
		t.Fatalf("brand layer not applied: %v", br)
	}
	mi, _ := br["markImage"].(string)
	if !strings.HasPrefix(mi, filepath.Join(dir, "ryoku", "rice-assets", "wave")) || !isFile(mi) {
		t.Fatalf("brand mark not rehydrated: %q", mi)
	}
	wantWall := filepath.Join(dir, "Pictures", "livewalls", "wave.mp4")
	found := false
	for _, c := range calls {
		if c == "ryogami wallpaper set "+wantWall {
			found = true
		}
	}
	if !found || !isFile(wantWall) {
		t.Fatalf("video wall not routed to livewalls; calls=%v", calls)
	}
	if !isFile(filepath.Join(ricesDir(), ".previous", "widgets.json")) {
		t.Fatal(".previous snapshot missing widgets.json")
	}

	// the byte-for-byte restore brings the new stores back too.
	if err := restoreRice(".previous"); err != nil {
		t.Fatal(err)
	}
	if readJSONMap(widgetsStorePath())["clock24h"] != false {
		t.Fatal("restore did not revert widgets.json")
	}
	if readJSONMap(brandStorePath())["name"] != "Old" {
		t.Fatal("restore did not revert brand.json")
	}
}

// setRiceWallpaper accepts a video, swaps out the previous asset file, and an
// import lands an exported folder as a fresh, de-duped local rice without its
// reading matter (configs/, README).
func TestSetWallVideoAndImport(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	t.Setenv("HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "ryoku"), 0o755); err != nil {
		t.Fatal(err)
	}
	w := func(p, body string) {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(ricesDir(), "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	w(ricePath("demo"), `{"schema":1,"slug":"demo","name":"Demo","color":{"mode":"wallpaper"},"assets":{"wallpaper":"wall.png"},"look":{}}`)
	w(filepath.Join(ricesDir(), "demo", "wall.png"), "PNG")
	clip := filepath.Join(dir, "clip.mp4")
	w(clip, "MP4")
	if err := setRiceWallpaper("demo", clip); err != nil {
		t.Fatal(err)
	}
	r, _, err := loadRice("demo")
	if err != nil {
		t.Fatal(err)
	}
	if r.Assets.Wallpaper != "wall.mp4" {
		t.Fatalf("wallpaper = %q, want wall.mp4", r.Assets.Wallpaper)
	}
	if isFile(filepath.Join(ricesDir(), "demo", "wall.png")) {
		t.Fatal("stale wall.png left behind after a video setwall")
	}

	// an exported folder: manifest + assets + reading matter.
	exp := filepath.Join(dir, "shared")
	if err := os.MkdirAll(filepath.Join(exp, "configs"), 0o755); err != nil {
		t.Fatal(err)
	}
	w(filepath.Join(exp, "rice.json"), `{"schema":1,"slug":"demo","name":"Demo","color":{"mode":"wallpaper"},"look":{"shell":{"frameBars":{"style":"ryoku-frame"}}}}`)
	w(filepath.Join(exp, "wall.png"), "PNG")
	w(filepath.Join(exp, "README.txt"), "hi")
	w(filepath.Join(exp, "configs", "shell.json"), `{}`)

	nr, err := importRice(exp)
	if err != nil {
		t.Fatal(err)
	}
	if nr.Slug != "demo-2" {
		t.Fatalf("import slug = %q, want demo-2 (de-duped)", nr.Slug)
	}
	idir := filepath.Join(ricesDir(), "demo-2")
	if !isFile(filepath.Join(idir, "wall.png")) {
		t.Fatal("import dropped the wallpaper asset")
	}
	if isFile(filepath.Join(idir, "README.txt")) || isFile(filepath.Join(idir, "configs", "shell.json")) {
		t.Fatal("import copied the export's reading matter")
	}
	if _, _, err := loadRice("demo-2"); err != nil {
		t.Fatalf("imported rice does not load: %v", err)
	}
}

func TestRunRiceRejectsStoreCommands(t *testing.T) {
	for _, command := range []string{"catalog", "install"} {
		err := runRice([]string{command})
		if err == nil || !strings.Contains(err.Error(), "unknown rice subcommand") {
			t.Fatalf("runRice(%q) = %v, want unknown rice subcommand", command, err)
		}
	}
}

// The widened snapshot carries the whole desktop identity: the theme master
// (named scheme, GTK choice), the wallpaper daemon's matugen scheme, the
// fastfetch readout with its emblem, and the lock skin. capture bundles each and
// apply lands them all on a recipient whose own machine tuning is left alone.
func TestCaptureAppliesThemeFastfetchAndLock(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	t.Setenv("HOME", dir)
	origRun, origReload := riceRun, riceReload
	riceRun = func(name string, args ...string) error { return nil }
	riceReload = func() {}
	t.Cleanup(func() { riceRun, riceReload = origRun, origReload })

	w := func(p, body string) {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// a lock skin the author has installed (a folder with Main.qml); the rice
	// travels its slug, not the files.
	slug := "clockwork/orbital"
	w(filepath.Join(qylockThemesDir(), slug, "Main.qml"), "import QtQuick")
	w(qylockThemePref(), slug+"\n")

	writeDesktopStore(t, `{"appearance":{"rounding":6},"dwindle":{"mfact":0.6}}`)
	w(shellStorePath(), `{"barStyle":"qsbar"}`)
	w(launcherStorePath(), `{}`)
	w(themeStatePath(), `{"followWallpaper":false,"scheme":"mono","gtkTheme":"adwaita","themeApps":true}`)
	w(ryogamiStorePath(), `{"matugen":{"mode":"dark","schemeType":"scheme-fidelity"},"resource_tier":"high"}`)
	emblem := filepath.Join(dir, "logo.png")
	w(emblem, "PNG")
	w(fastfetchConfigPath(), `{"logo":{"type":"kitty-direct","source":"`+emblem+`"},"display":{"key":{"width":9}}}`)
	w(filepath.Join(dir, "cache", "ryoku", "colors.json"), `{"background":"#101010"}`)

	r, err := captureRice("Identity", []string{"all"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Look["theme"]["scheme"] != "mono" || r.Look["theme"]["gtkTheme"] != "adwaita" {
		t.Fatalf("theme master not captured: %v", r.Look["theme"])
	}
	if mat, ok := r.Look["ryogami"]["matugen"].(map[string]any); !ok || mat["schemeType"] != "scheme-fidelity" {
		t.Fatalf("ryogami matugen scheme not captured: %v", r.Look["ryogami"])
	}
	if _, ok := r.Look["ryogami"]["resource_tier"]; ok {
		t.Fatal("machine tuning (resource_tier) must not travel")
	}
	rdir := filepath.Join(ricesDir(), r.Slug)
	if r.Assets.FastfetchStyle != "fastfetch.jsonc" || !isFile(filepath.Join(rdir, "fastfetch.jsonc")) {
		t.Fatalf("fastfetch style not bundled: %q", r.Assets.FastfetchStyle)
	}
	if r.Assets.Fastfetch == "" || !isFile(filepath.Join(rdir, r.Assets.Fastfetch)) {
		t.Fatalf("fastfetch emblem not bundled: %q", r.Assets.Fastfetch)
	}
	if r.Assets.Lock != slug {
		t.Fatalf("lock slug = %q, want %q", r.Assets.Lock, slug)
	}

	// a recipient with different everything adopts the rice's identity.
	w(themeStatePath(), `{"followWallpaper":true,"scheme":"light","gtkTheme":"adw"}`)
	w(ryogamiStorePath(), `{"matugen":{"mode":"light","schemeType":"scheme-tonal-spot"},"resource_tier":"low"}`)
	w(fastfetchConfigPath(), `{"logo":{"type":"builtin"}}`)
	w(qylockThemePref(), "other/skin\n")

	if err := applyRice(r.Slug, nil); err != nil {
		t.Fatal(err)
	}
	st := loadThemeState()
	if st.Scheme != "mono" || gtkThemeChoice(st) != "adwaita" {
		t.Fatalf("apply did not carry the theme master: %+v", st)
	}
	if readJSONMap(ryogamiStorePath())["matugen"].(map[string]any)["schemeType"] != "scheme-fidelity" {
		t.Fatal("apply did not carry the matugen scheme")
	}
	if readJSONMap(ryogamiStorePath())["resource_tier"] != "low" {
		t.Fatal("apply must not overwrite machine tuning")
	}
	if ff, _ := loadFastfetch(); ff.Logo.Kind != "image" {
		t.Fatalf("apply did not restore the fastfetch emblem: %+v", ff.Logo)
	}
	if readLockPref(qylockThemePref()) != slug {
		t.Fatalf("apply did not set the lock skin: %q", readLockPref(qylockThemePref()))
	}
}

// A rice with the brand layer bundles the custom reload-cover asset the same
// way it bundles the mark: the file is copied into the rice folder and the
// path rewritten to rice://, so the cover travels instead of pointing at a
// file that exists only on the author's disk.
func TestCaptureBundlesReloadCover(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	t.Setenv("HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "ryoku"), 0o755); err != nil {
		t.Fatal(err)
	}
	cover := filepath.Join(dir, "cover.gif")
	if err := os.WriteFile(cover, []byte("GIF89a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brandStorePath(), []byte(`{"name":"Berserk","reloadCover":{"path":"`+cover+`","name":"cover.gif","kind":"animated","bytes":6,"enabled":true}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := captureRice("Cov", []string{"brand"})
	if err != nil {
		t.Fatal(err)
	}
	raw, ok := r.Layers["brand"]
	if !ok {
		t.Fatalf("brand layer not captured: %v", r.Layers)
	}
	var bm map[string]any
	if err := json.Unmarshal(raw, &bm); err != nil {
		t.Fatal(err)
	}
	rc, _ := bm["reloadCover"].(map[string]any)
	if rc == nil {
		t.Fatalf("reloadCover not in captured brand layer: %s", raw)
	}
	if rc["path"] != "rice://reloadcover.gif" {
		t.Fatalf("reloadCover path not bundled to rice://: %v", rc["path"])
	}
	if rc["kind"] != "animated" || rc["enabled"] != true {
		t.Fatalf("reloadCover metadata not preserved: %v", rc)
	}
	if !isFile(filepath.Join(ricesDir(), "cov", "reloadcover.gif")) {
		t.Fatal("bundled reload cover missing from the rice folder")
	}
}

// Applying a rice lands the bundled reload cover under rice-assets and rewrites
// the path to that absolute location (so the renderer's "file://"+path resolves
// on this box); a rice:// asset that is missing, or a foreign absolute path
// that does not resolve here, drops the block so the reload falls back to the
// default cover cleanly instead of a broken one.
func TestRehydrateReloadCoverAsset(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "ryoku"), 0o755); err != nil {
		t.Fatal(err)
	}
	riceDir := filepath.Join(dir, "rice")
	if err := os.MkdirAll(riceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(riceDir, "reloadcover.gif"), []byte("GIF89a"), 0o644); err != nil {
		t.Fatal(err)
	}
	present := filepath.Join(dir, "present.gif")
	if err := os.WriteFile(present, []byte("GIF89a"), 0o644); err != nil {
		t.Fatal(err)
	}

	// rice:// reference: copied under rice-assets, path rewritten to it.
	bm := map[string]any{"reloadCover": map[string]any{"path": "rice://reloadcover.gif", "kind": "animated"}}
	rehydrateBrandAssets(riceDir, "cov", bm)
	rc, _ := bm["reloadCover"].(map[string]any)
	if rc == nil {
		t.Fatalf("reloadCover dropped for a valid rice:// asset: %v", bm)
	}
	got, _ := rc["path"].(string)
	wantPrefix := filepath.Join(dir, "ryoku", "rice-assets", "cov")
	if !strings.HasPrefix(got, wantPrefix) || !isFile(got) {
		t.Fatalf("reloadCover not rehydrated under rice-assets: %q", got)
	}

	// rice:// reference whose file is not in the rice folder: dropped.
	bm = map[string]any{"reloadCover": map[string]any{"path": "rice://missing.gif", "kind": "animated"}}
	rehydrateBrandAssets(riceDir, "cov", bm)
	if _, ok := bm["reloadCover"]; ok {
		t.Fatalf("missing rice:// cover not dropped: %v", bm)
	}

	// foreign absolute path that does not resolve on this box: dropped.
	bm = map[string]any{"reloadCover": map[string]any{"path": "/home/someone-else/.local/share/ryoku/reload-cover/x.gif", "kind": "animated"}}
	rehydrateBrandAssets(riceDir, "cov", bm)
	if _, ok := bm["reloadCover"]; ok {
		t.Fatalf("dangling absolute cover not dropped: %v", bm)
	}

	// an absolute path that does resolve here (e.g. a same-box re-apply) is kept.
	bm = map[string]any{"reloadCover": map[string]any{"path": present, "kind": "animated"}}
	rehydrateBrandAssets(riceDir, "cov", bm)
	rc, _ = bm["reloadCover"].(map[string]any)
	if rc == nil || rc["path"] != present {
		t.Fatalf("resolvable absolute cover not kept: %v", bm)
	}
}

// A rice is the whole visual desktop: the plugin roster (per-widget settings
// and placements), the widget stage, the palette engine, and the Profile
// plate's decor all travel, and the picker's look sections arrive as a
// allowlist that holds back the author's hardware (gpuDevice, per-monitor
// clip state) and accounts.
func TestRiceCaptureCarriesEveryVisualStore(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "ryoku"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(p, body string) {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeDesktopStore(t, `{"appearance":{"rounding":2}}`)
	write(shellStorePath(), `{}`)
	write(launcherStorePath(), `{}`)
	write(themeStatePath(), `{"followWallpaper":true}`)
	write(pluginsStorePath(), `{"mjw-calc":{"enabled":true,"host":"desktopWidget","placement":{"desktopWidget":{"x":120,"y":80}},"settings":{"digits":6}}}`)
	write(stageStorePath(), `{"edge":0.15,"quality":"fine"}`)
	write(matugenStorePath(), `{"schemeType":"scheme-vibrant","themeRyokuApps":true}`)
	write(ryogamiStorePath(), `{"transition":{"shader":"morph"},"performance":{"gpuDevice":"uuid:deadbeef"},"monitor":"eDP-1","integrations":[{"name":"wallhaven"}]}`)

	r, err := captureRice("Whole", nil)
	if err != nil {
		t.Fatal(err)
	}
	plugins := r.Look["plugins"]
	calc, _ := plugins["mjw-calc"].(map[string]any)
	if calc == nil {
		t.Fatalf("plugin roster not captured: %v", plugins)
	}
	if s, _ := calc["settings"].(map[string]any); s["digits"] != float64(6) {
		t.Fatalf("per-widget plugin settings lost: %v", calc["settings"])
	}
	if r.Look["stage"]["quality"] != "fine" {
		t.Fatalf("stage look not captured: %v", r.Look["stage"])
	}
	if r.Look["matugen"]["schemeType"] != "scheme-vibrant" {
		t.Fatalf("palette engine not captured: %v", r.Look["matugen"])
	}
	ryog := r.Look["ryogami"]
	if ryog["transition"] == nil {
		t.Fatalf("picker transition not captured: %v", ryog)
	}
	if _, ok := ryog["performance"]; ok {
		t.Fatal("the author's gpuDevice must not travel")
	}
	if _, ok := ryog["monitor"]; ok {
		t.Fatal("the author's per-monitor clip state must not travel")
	}
	if _, ok := ryog["integrations"]; ok {
		t.Fatal("external accounts must not travel")
	}
}

// The profile plate's custom hero travels as a bundled asset and lands on the
// recipient's profile store; the shipped-art case needs no bundle because the
// name resolves against ~/Pictures/ryodecors on every box.
func TestRiceProfileHeroRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	origRun, origReload := riceRun, riceReload
	riceRun = func(string, ...string) error { return nil }
	riceReload = func() {}
	t.Cleanup(func() { riceRun, riceReload = origRun, origReload })

	if err := os.MkdirAll(profileHeroDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileHeroDir(), "myface.png"), []byte("img"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeDesktopStore(t, `{"appearance":{"rounding":2}}`)
	if err := os.WriteFile(profileConfigPath(), []byte(`{"hero":{"kind":"custom","source":"myface.png","zoom":1.4}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := captureRice("Face", nil)
	if err != nil {
		t.Fatal(err)
	}
	hero, _ := r.Look["profile"]["hero"].(map[string]any)
	if hero["source"] != "profilehero.png" {
		t.Fatalf("custom hero not rebundled as an asset: %v", hero)
	}
	if !isFile(filepath.Join(ricesDir(), "face", "profilehero.png")) {
		t.Fatal("hero image not bundled into the rice")
	}

	// a recipient applies it: the image lands in their profile store and the
	// manifest points at the copy.
	if err := os.Remove(profileConfigPath()); err != nil {
		t.Fatal(err)
	}
	if err := applyRice("face", nil); err != nil {
		t.Fatal(err)
	}
	got := readJSONMap(profileConfigPath())
	gh, _ := got["hero"].(map[string]any)
	if gh["kind"] != "custom" || gh["source"] != "rice-face.png" {
		t.Fatalf("apply did not land the hero: %v", gh)
	}
	if !isFile(filepath.Join(profileHeroDir(), "rice-face.png")) {
		t.Fatal("applied hero image missing from the profile store")
	}
}

// The folder field is free text: a leading ~ is the home, a bare name sits
// under it, and a folder that does not exist yet is created. Refusing all
// three was the top export failure people reported.
func TestExpandUserDest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cases := []struct{ in, want string }{
		{"~/rice-out", filepath.Join(home, "rice-out")},
		{"~", filepath.Join(home, "Rices")},
		{"", filepath.Join(home, "Rices")},
		{"rice-out", filepath.Join(home, "rice-out")},
		{"/abs/path", "/abs/path"},
		{"file://" + filepath.Join(home, "x"), filepath.Join(home, "x")},
	}
	for _, c := range cases {
		if got := expandUserDest(c.in); got != c.want {
			t.Errorf("expandUserDest(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// exportRice creates a destination that does not exist yet and accepts the ~
// form, so typing "~/rice-share" into the folder field works.
func TestRiceExportCreatesMissingDest(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	r := Rice{Schema: riceSchema, Slug: "demo", Name: "Demo",
		Look: map[string]map[string]any{"shell": {"frameBars": map[string]any{"style": "ryoku-frame"}}}}
	if err := saveRice(r); err != nil {
		t.Fatal(err)
	}
	out, err := exportRice("demo", "~/rice-share")
	if err != nil {
		t.Fatalf("export to a missing ~ folder: %v", err)
	}
	if want := filepath.Join(dir, "rice-share", "demo"); out != want {
		t.Fatalf("exported to %q, want %q", out, want)
	}
	if !isFile(filepath.Join(out, "rice.json")) {
		t.Fatal("manifest missing from the export")
	}
}
