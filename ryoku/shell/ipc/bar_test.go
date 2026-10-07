package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// catalogueFixture is the shipped widgets.json shape (spec section 2), written
// verbatim so the tests exercise the real catalogue contract without depending
// on BarCore having landed the file.
const catalogueFixture = `[
  { "id": "launcher",   "gid": "G1",  "label": "Launcher",        "gloss": "起動", "category": "Bar",
    "desc": "The mark that opens the launcher and this panel.",
    "settings": [
      { "key": "launcherLogoMode", "type": "choice", "label": "Mark", "options": ["text", "icon"] },
      { "key": "launcherLogoText", "type": "choice", "label": "Wordmark", "options": ["ryoku", "omarchy", "hyprland", "arch", "omacom"] },
      { "key": "launcherLogoIcon", "type": "choice", "label": "Glyph", "options": ["ryoku", "arch", "dragon"] } ] },
  { "id": "workspaces", "gid": "G2",  "label": "Workspaces",      "gloss": "間",   "category": "Bar",
    "settings": [
      { "key": "workspaceMode",  "type": "choice", "label": "Shown", "options": ["active", "5", "10"] },
      { "key": "workspaceStyle", "type": "choice", "label": "Marker", "options": ["default", "numbers", "magic", "kanji", "rings", "aurora", "pacman"] } ] },
  { "id": "status",     "gid": "G3",  "label": "Status",          "visKey": "status",   "desc": "Arch update, tray, notifications" },
  { "id": "memory",     "gid": "G4",  "label": "Memory",          "visKey": "memory" },
  { "id": "cpu",        "gid": "G5",  "label": "CPU",             "visKey": "cpu" },
  { "id": "volume",     "gid": "G6",  "label": "Volume",          "visKey": "volume",
    "settings": [ { "key": "audioBoost", "type": "toggle", "label": "Allow over 100%" } ] },
  { "id": "ai",         "gid": "G7",  "label": "AI usage",        "visKey": "claude",
    "settings": [ { "key": "aiTools", "type": "multi", "label": "Tools", "options": ["claude", "codex", "opencode"] } ] },
  { "id": "clock",      "gid": "G8",  "label": "Clock",           "desc": "weather, clock, date, indicators",
    "settings": [ { "key": "clock12h", "type": "toggle", "label": "12-hour" },
                  { "key": "weatherImperial", "type": "toggle", "label": "Fahrenheit" },
                  { "key": "weather", "type": "toggle", "label": "Weather", "visKey": true } ] },
  { "id": "media",      "gid": "G9",  "label": "Now playing",     "visKey": "mpris",
    "settings": [ { "key": "mprisBarStyle", "type": "choice", "label": "Style", "options": ["default", "full"] } ] },
  { "id": "quick",      "gid": "G10", "label": "Quick",           "visKey": "quick",    "desc": "keep awake, media, theme" },
  { "id": "network",    "gid": "G11", "label": "Network",         "visKey": "network" },
  { "id": "battery",    "gid": "G12", "label": "Battery",         "visKey": "battery" },
  { "id": "brightness", "gid": "G13", "label": "Brightness",      "visKey": "brightness" },
  { "id": "power",      "gid": "G14", "label": "Power",           "visKey": "power" },
  { "id": "bluetooth",  "gid": "G15", "label": "Bluetooth",       "visKey": "bluetooth" },
  { "id": "cputemp",    "gid": "G16", "label": "CPU temperature", "visKey": "cpuTemperature",
    "settings": [ { "key": "barTemperatureSource", "type": "choice", "label": "Sensor", "options": ["cpu", "core", "gpu", "nvme", "memory"] } ] },
  { "id": "gpu",        "gid": "G17", "label": "GPU",             "visKey": "gpu" },
  { "id": "storage",    "gid": "G18", "label": "Storage",         "visKey": "storage" },
  { "id": "layout",     "gid": "G19", "label": "Keyboard layout", "visKey": "layout" }
]`

func testCatalog(t *testing.T) []catalogWidget {
	t.Helper()
	var cat []catalogWidget
	if err := json.Unmarshal([]byte(catalogueFixture), &cat); err != nil {
		t.Fatalf("catalogue fixture does not parse: %v", err)
	}
	return cat
}

// TestLayoutMove exercises the four placement modes and cross-section moves, and
// checks that an anchor absent from the target section is rejected.
func TestLayoutMove(t *testing.T) {
	base := barLayout{Version: 1, Left: []string{"a", "b", "c"}, Center: []string{"m"}, Right: []string{"x", "y"}}
	cases := []struct {
		name         string
		id, section  string
		p            placement
		wantErr      bool
		left, center []string
		right        []string
	}{
		{name: "index within lane", id: "c", section: "left", p: placement{hasIndex: true, index: 0},
			left: []string{"c", "a", "b"}, center: []string{"m"}, right: []string{"x", "y"}},
		{name: "index past end clamps", id: "a", section: "left", p: placement{hasIndex: true, index: 99},
			left: []string{"b", "c", "a"}, center: []string{"m"}, right: []string{"x", "y"}},
		{name: "before anchor", id: "y", section: "left", p: placement{before: "b"},
			left: []string{"a", "y", "b", "c"}, center: []string{"m"}, right: []string{"x"}},
		{name: "after anchor", id: "x", section: "left", p: placement{after: "a"},
			left: []string{"a", "x", "b", "c"}, center: []string{"m"}, right: []string{"y"}},
		{name: "cross section to center", id: "a", section: "center", p: placement{},
			left: []string{"b", "c"}, center: []string{"m", "a"}, right: []string{"x", "y"}},
		{name: "unknown anchor", id: "a", section: "right", p: placement{before: "nope"}, wantErr: true},
		{name: "unknown section", id: "a", section: "middle", p: placement{}, wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := layoutMove(base, c.id, c.section, c.p)
			if c.wantErr {
				if err == nil {
					t.Fatalf("want error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got.Left, c.left) || !reflect.DeepEqual(got.Center, c.center) || !reflect.DeepEqual(got.Right, c.right) {
				t.Fatalf("move = L%v C%v R%v, want L%v C%v R%v", got.Left, got.Center, got.Right, c.left, c.center, c.right)
			}
			// The input must not be mutated (fresh slices only).
			if !reflect.DeepEqual(base.Left, []string{"a", "b", "c"}) {
				t.Fatalf("layoutMove mutated its input: %v", base.Left)
			}
		})
	}
}

// TestNormalizeLayout checks duplicates are dropped, unknown/stale ids are
// dropped, and a known widget missing from the layout is appended to its default
// section (right for a built-in, its manifest section for a plugin).
func TestNormalizeLayout(t *testing.T) {
	known := []knownWidget{
		{id: "a", section: "left"},
		{id: "b", section: "left"},
		{id: "m", section: "center"},
		{id: "x", section: "right"},
		{id: "plug", section: "center"}, // a plugin that asks to land in center
	}
	in := barLayout{
		Left:   []string{"a", "a", "b"}, // duplicate a
		Center: []string{"m", "stale"},  // stale is not known
		Right:  []string{"x"},
		// "plug" omitted entirely -> appended to its default section (center)
	}
	got := normalizeLayout(in, known)
	if !reflect.DeepEqual(got.Left, []string{"a", "b"}) {
		t.Fatalf("left = %v, want [a b]", got.Left)
	}
	if !reflect.DeepEqual(got.Center, []string{"m", "plug"}) {
		t.Fatalf("center = %v, want [m plug] (stale dropped, plug appended to its section)", got.Center)
	}
	if !reflect.DeepEqual(got.Right, []string{"x"}) {
		t.Fatalf("right = %v, want [x]", got.Right)
	}
	if got.Version != 1 {
		t.Fatalf("version = %d, want 1", got.Version)
	}
}

// TestNormalizeLayoutAppendsMissingToRight checks a known built-in the layout
// omits lands at the end of right when its default section is right.
func TestNormalizeLayoutAppendsMissingToRight(t *testing.T) {
	known := []knownWidget{{id: "a", section: "right"}, {id: "b", section: "right"}}
	got := normalizeLayout(barLayout{Right: []string{"a"}}, known)
	if !reflect.DeepEqual(got.Right, []string{"a", "b"}) {
		t.Fatalf("right = %v, want [a b]", got.Right)
	}
}

// TestLayoutDefaults returns the shipped order, filtered to the catalogue ids.
func TestLayoutDefaults(t *testing.T) {
	cat := testCatalog(t)
	got := layoutDefaults(cat)
	want := shippedLayout()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("defaults = %+v, want %+v", got, want)
	}
	// An id the catalogue drops is not placed.
	partial := []catalogWidget{{ID: "launcher"}, {ID: "clock"}}
	gp := layoutDefaults(partial)
	if !reflect.DeepEqual(gp.Left, []string{"launcher"}) || !reflect.DeepEqual(gp.Center, []string{"clock"}) || len(gp.Right) != 0 {
		t.Fatalf("partial defaults = %+v", gp)
	}
}

// TestVisibilityDefaults derives the shipped qsbar.widgets from the catalogue:
// every visKey shown except the ship-off set, plus clock's weather toggle.
func TestVisibilityDefaults(t *testing.T) {
	vis := visibilityDefaults(testCatalog(t))
	off := map[string]bool{"claude": true, "power": true, "bluetooth": true}
	for k, v := range vis {
		if off[k] && v {
			t.Fatalf("%q ships off but is true", k)
		}
		if !off[k] && !v {
			t.Fatalf("%q ships on but is false", k)
		}
	}
	if _, ok := vis["weather"]; !ok {
		t.Fatal("clock's weather visKey:true toggle must appear in the shipped visibility")
	}
	if !vis["weather"] || !vis["status"] || vis["claude"] || vis["power"] || vis["bluetooth"] {
		t.Fatalf("visibility off-set wrong: %+v", vis)
	}
}

// TestBarListRows is the `bar list --json` row builder: it walks the layout in
// section order and stamps section, 0-based index, and metadata.
func TestBarListRows(t *testing.T) {
	l := barLayout{Left: []string{"launcher", "clock"}, Center: []string{}, Right: []string{"plug"}}
	meta := map[string]widgetMeta{
		"launcher": {Label: "Launcher", Kind: "builtin", Shown: true},
		"clock":    {Label: "Clock", Kind: "builtin", Shown: false},
		"plug":     {Label: "My Plugin", Kind: "plugin", Shown: true},
	}
	rows := barListRows(l, meta)
	want := []listRow{
		{ID: "launcher", Label: "Launcher", Kind: "builtin", Section: "left", Index: 0, Shown: true},
		{ID: "clock", Label: "Clock", Kind: "builtin", Section: "left", Index: 1, Shown: false},
		{ID: "plug", Label: "My Plugin", Kind: "plugin", Section: "right", Index: 0, Shown: true},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %+v, want %+v", rows, want)
	}
	// An id with no metadata is skipped (a normalized layout never carries one).
	if got := barListRows(barLayout{Left: []string{"ghost"}}, meta); len(got) != 0 {
		t.Fatalf("unknown id should be skipped, got %+v", got)
	}
}

// TestValidateBuiltinSetting covers each catalogue setting type and its
// rejections: an option out of the list, a non-bool for a toggle, and a
// value that is not an integer.
func TestValidateBuiltinSetting(t *testing.T) {
	min, max := 0, 10
	cases := []struct {
		name    string
		s       catalogSetting
		raw     string
		want    string // marshalled JSON on success
		wantErr bool
	}{
		{name: "choice ok", s: catalogSetting{Type: "choice", Options: []string{"text", "icon"}}, raw: "icon", want: `"icon"`},
		{name: "choice not in list", s: catalogSetting{Type: "choice", Options: []string{"text", "icon"}}, raw: "video", wantErr: true},
		{name: "toggle on", s: catalogSetting{Type: "toggle"}, raw: "on", want: `true`},
		{name: "toggle false", s: catalogSetting{Type: "toggle"}, raw: "false", want: `false`},
		{name: "toggle bad", s: catalogSetting{Type: "toggle"}, raw: "maybe", wantErr: true},
		{name: "multi ok", s: catalogSetting{Type: "multi", Options: []string{"claude", "codex", "opencode"}}, raw: "claude,codex", want: `["claude","codex"]`},
		{name: "multi bad member", s: catalogSetting{Type: "multi", Options: []string{"claude"}}, raw: "claude,gemini", wantErr: true},
		{name: "int ok", s: catalogSetting{Type: "int", Min: &min, Max: &max}, raw: "5", want: `5`},
		{name: "int too high", s: catalogSetting{Type: "int", Min: &min, Max: &max}, raw: "99", wantErr: true},
		{name: "int not a number", s: catalogSetting{Type: "int"}, raw: "five", wantErr: true},
		{name: "text ok", s: catalogSetting{Type: "text"}, raw: "hi there", want: `"hi there"`},
		{name: "unknown type", s: catalogSetting{Type: "colour"}, raw: "x", wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := validateBuiltinSetting(c.s, c.raw)
			if c.wantErr {
				if err == nil {
					t.Fatalf("want error, got %s", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got) != c.want {
				t.Fatalf("value = %s, want %s", got, c.want)
			}
		})
	}
}

// TestValidatePluginSetting covers the manifest setting schema shapes: choice
// values are {value,label} objects, slider clamps to min/max, toggle rejects a
// non-bool.
func TestValidatePluginSetting(t *testing.T) {
	choice := map[string]any{"type": "choice", "options": []any{
		map[string]any{"value": "red", "label": "Red"},
		map[string]any{"value": "blue", "label": "Blue"},
	}}
	slider := map[string]any{"type": "slider", "min": 0.0, "max": 1.0}
	cases := []struct {
		name    string
		ms      map[string]any
		raw     string
		want    string
		wantErr bool
	}{
		{name: "choice ok", ms: choice, raw: "blue", want: `"blue"`},
		{name: "choice not in list", ms: choice, raw: "green", wantErr: true},
		{name: "toggle ok", ms: map[string]any{"type": "toggle"}, raw: "on", want: `true`},
		{name: "toggle bad", ms: map[string]any{"type": "toggle"}, raw: "x", wantErr: true},
		{name: "slider ok", ms: slider, raw: "0.5", want: `0.5`},
		{name: "slider too high", ms: slider, raw: "2", wantErr: true},
		{name: "text passthrough", ms: map[string]any{"type": "text"}, raw: "hello", want: `"hello"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := validatePluginSetting(c.ms, c.raw)
			if c.wantErr {
				if err == nil {
					t.Fatalf("want error, got %s", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got) != c.want {
				t.Fatalf("value = %s, want %s", got, c.want)
			}
		})
	}
}

// barTestDaemon wires a daemon with a settings store over a temp shell.json and
// points the catalogue/plugin/config lookups at temp dirs, so the verbs run
// end-to-end without a live shell.
func barTestDaemon(t *testing.T) *daemon {
	t.Helper()
	root := t.TempDir()

	old := shellDir
	shellDir = root
	t.Cleanup(func() { shellDir = old })

	catPath := filepath.Join(root, "quickshell", "shell", "modules", "bar", "barstyles", "qsbar", "core", "widgets.json")
	if err := os.MkdirAll(filepath.Dir(catPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catPath, []byte(catalogueFixture), 0o644); err != nil {
		t.Fatal(err)
	}

	// Isolate plugin discovery and plugins.json from the developer's real dirs.
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("RYOSTORE_PLUGINS_DIR", "")
	t.Setenv("RYOKU_PLUGINS_DIR", "")

	d := &daemon{}
	d.settings = newSettingsStore(filepath.Join(root, "shell.json"))
	return d
}

func (d *daemon) frame(t *testing.T) map[string]any {
	t.Helper()
	d.settings.mu.Lock()
	defer d.settings.mu.Unlock()
	return deepCopyMap(d.settings.raw)
}

// TestBarVerbsThroughStore drives the whole verb set through dispatch and checks
// the settings frame reflects every mutation.
func TestBarVerbsThroughStore(t *testing.T) {
	d := barTestDaemon(t)

	// A fresh box with no layout lists the shipped order.
	out := d.dispatch("bar list --json")
	var rows []listRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("bar list --json did not parse: %v (%s)", err, out)
	}
	if rows[0].ID != "launcher" || rows[0].Section != "left" || rows[0].Index != 0 {
		t.Fatalf("first row = %+v, want launcher left 0", rows[0])
	}
	// The AI pill ships hidden.
	for _, r := range rows {
		if r.ID == "ai" && r.Shown {
			t.Fatal("ai must ship hidden")
		}
	}

	// Move the clock into the left lane at the head.
	if got := d.dispatch("bar move clock --section left --index 0"); got != "ok" {
		t.Fatalf("bar move: %s", got)
	}
	frame := d.frame(t)
	layout := parseLayout(frame)
	if layout.Left[0] != "clock" {
		t.Fatalf("clock did not move to left head: %v", layout.Left)
	}
	if inSet(layout.Center, "clock") {
		t.Fatalf("clock still in center: %v", layout.Center)
	}

	// Show/hide a built-in flips its visKey.
	if got := d.dispatch("bar show power"); got != "ok" {
		t.Fatalf("bar show power: %s", got)
	}
	if !parseWidgets(d.frame(t))["power"] {
		t.Fatal("bar show power did not set qsbar.widgets.power")
	}
	if got := d.dispatch("bar hide status"); got != "ok" {
		t.Fatalf("bar hide status: %s", got)
	}
	if parseWidgets(d.frame(t))["status"] {
		t.Fatal("bar hide status did not clear qsbar.widgets.status")
	}

	// A built-in without a visKey cannot be toggled.
	if got := d.dispatch("bar hide launcher"); !strings.HasPrefix(got, "err bar") {
		t.Fatalf("bar hide launcher should error, got %s", got)
	}

	// bar set validates against the catalogue and patches qsbar.<key>.
	if got := d.dispatch("bar set launcher launcherLogoText arch"); got != "ok" {
		t.Fatalf("bar set: %s", got)
	}
	if qs, _ := d.frame(t)["qsbar"].(map[string]any); qs["launcherLogoText"] != "arch" {
		t.Fatalf("qsbar.launcherLogoText = %v, want arch", qs["launcherLogoText"])
	}
	if got := d.dispatch("bar set launcher launcherLogoText nope"); !strings.HasPrefix(got, "err bar") {
		t.Fatalf("bad option should error, got %s", got)
	}
	if got := d.dispatch("bar set volume audioBoost on"); got != "ok" {
		t.Fatalf("bar set toggle: %s", got)
	}
	if qs, _ := d.frame(t)["qsbar"].(map[string]any); qs["audioBoost"] != true {
		t.Fatalf("qsbar.audioBoost = %v, want true", qs["audioBoost"])
	}

	// Position and form patch their presentation keys.
	if got := d.dispatch("bar position bottom"); got != "ok" {
		t.Fatalf("bar position: %s", got)
	}
	if got := d.dispatch("bar form islands"); got != "ok" {
		t.Fatalf("bar form: %s", got)
	}
	qs, _ := d.frame(t)["qsbar"].(map[string]any)
	if qs["barPosition"] != "bottom" || qs["barShellStyle"] != "islands" {
		t.Fatalf("position/form not persisted: %+v", qs)
	}
	if got := d.dispatch("bar form sideways"); !strings.HasPrefix(got, "err bar") {
		t.Fatalf("bad form should error, got %s", got)
	}

	// Defaults restore the shipped order and visibility.
	if got := d.dispatch("bar defaults"); got != "ok" {
		t.Fatalf("bar defaults: %s", got)
	}
	frame = d.frame(t)
	if parseLayout(frame).Center[0] != "clock" {
		t.Fatal("defaults did not restore clock to center")
	}
	w := parseWidgets(frame)
	if w["claude"] || w["power"] || w["bluetooth"] {
		t.Fatalf("defaults did not ship the off-set off: %+v", w)
	}
	if !w["status"] || !w["weather"] {
		t.Fatalf("defaults did not ship the on-set on: %+v", w)
	}
}

// TestDockVerbsThroughStore drives the dock verbs and checks the top-level dock
// object round-trips, keeping the shipped fields the CLI does not touch.
func TestDockVerbsThroughStore(t *testing.T) {
	d := barTestDaemon(t)

	if got := d.dispatch("dock show"); got != "ok" {
		t.Fatalf("dock show: %s", got)
	}
	if got := d.dispatch("dock edge left"); got != "ok" {
		t.Fatalf("dock edge: %s", got)
	}
	if got := d.dispatch("dock autohide off"); got != "ok" {
		t.Fatalf("dock autohide: %s", got)
	}
	if got := d.dispatch("dock pin org.kitty"); got != "ok" {
		t.Fatalf("dock pin: %s", got)
	}
	if got := d.dispatch("dock pin org.kitty"); got != "ok" {
		t.Fatalf("re-pin should be idempotent: %s", got)
	}

	out := d.dispatch("dock list --json")
	var view map[string]any
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("dock list --json did not parse: %v (%s)", err, out)
	}
	if view["enabled"] != true || view["edge"] != "left" || view["autohide"] != false {
		t.Fatalf("dock list = %+v", view)
	}
	pinned := toStringSlice(view["pinned"])
	if len(pinned) != 1 || pinned[0] != "org.kitty" {
		t.Fatalf("pinned = %v, want [org.kitty]", pinned)
	}

	// The shipped fields the CLI never sets survive the write.
	dock, _ := d.frame(t)["dock"].(map[string]any)
	if dock["magnify"] != true || dock["frost"] != true {
		t.Fatalf("dock write dropped untouched fields: %+v", dock)
	}

	if got := d.dispatch("dock unpin org.kitty"); got != "ok" {
		t.Fatalf("dock unpin: %s", got)
	}
	if got := d.dispatch("dock unpin org.kitty"); !strings.HasPrefix(got, "err dock") {
		t.Fatalf("unpinning a missing app should error, got %s", got)
	}
	if got := d.dispatch("dock edge sideways"); !strings.HasPrefix(got, "err dock") {
		t.Fatalf("bad edge should error, got %s", got)
	}
}

// TestBarCatalogWithPlugin lists a topbar-capable plugin in the catalogue and
// reports it shown when plugins.json enables it on the bar.
func TestBarCatalogWithPlugin(t *testing.T) {
	d := barTestDaemon(t)
	root := shellDir

	pluginsDir := filepath.Join(root, "extra-plugins")
	manDir := filepath.Join(pluginsDir, "myplug")
	if err := os.MkdirAll(manDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"id":"myplug","name":"My Plugin","version":"2.1.0",
		"hosts":["topbarGlyph","desktopWidget"],
		"defaults":{"bar":{"section":"center"}},
		"metadata":{"settings":[{"key":"accent","type":"choice","label":"Accent",
			"options":[{"value":"red","label":"Red"},{"value":"blue","label":"Blue"}],"default":"red"}]}}`
	if err := os.WriteFile(filepath.Join(manDir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOSTORE_PLUGINS_DIR", pluginsDir)

	// Enable it on the bar through plugins.json.
	pj := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "ryoku")
	if err := os.MkdirAll(pj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pj, "plugins.json"), []byte(`{"myplug":{"enabled":true,"host":"topbarGlyph"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	out := d.dispatch("bar catalog --json")
	var entries []map[string]any
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("bar catalog --json did not parse: %v (%s)", err, out)
	}
	var found map[string]any
	for _, e := range entries {
		if e["id"] == "myplug" {
			found = e
		}
	}
	if found == nil {
		t.Fatal("plugin missing from catalogue")
	}
	if found["kind"] != "plugin" {
		t.Fatalf("plugin kind = %v", found["kind"])
	}
	if hosts := toStringSlice(found["hosts"]); !inSet(hosts, "topbarGlyph") {
		t.Fatalf("plugin hosts = %v", hosts)
	}
	if found["pluginDir"] != manDir {
		t.Fatalf("pluginDir = %v, want %v", found["pluginDir"], manDir)
	}

	// It lands in center (its manifest default section) and reports shown.
	out = d.dispatch("bar list --json")
	var rows []listRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("bar list --json did not parse: %v", err)
	}
	var pr *listRow
	for i := range rows {
		if rows[i].ID == "myplug" {
			pr = &rows[i]
		}
	}
	if pr == nil {
		t.Fatal("plugin not listed")
	}
	if pr.Section != "center" || !pr.Shown || pr.Kind != "plugin" {
		t.Fatalf("plugin row = %+v, want center/shown/plugin", *pr)
	}
}

func TestBarClockFormatUsesGlobalPreference(t *testing.T) {
	d := barTestDaemon(t)
	for _, tc := range []struct {
		value  string
		want24 bool
	}{
		{"off", true}, {"on", false},
	} {
		if got := d.dispatch("bar set clock clock12h " + tc.value); got != "ok" {
			t.Fatalf("bar set clock: %s", got)
		}
		frame := d.frame(t)
		general := frame["general"].(map[string]any)
		if general["clock_format_24_h"] != tc.want24 {
			t.Fatalf("global clock format = %v, want %v", general["clock_format_24_h"], tc.want24)
		}
		reloaded := newSettingsStore(d.settings.path)
		if reloaded.raw["general"].(map[string]any)["clock_format_24_h"] != tc.want24 {
			t.Fatal("clock format did not survive a store reload")
		}
	}
}
