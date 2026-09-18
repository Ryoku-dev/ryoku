package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// newTestStore builds a settings store over a fresh temp shell.json path. The
// file does not exist yet, so the store starts from shipped defaults.
func newTestStore(t *testing.T) *settingsStore {
	t.Helper()
	return newSettingsStore(filepath.Join(t.TempDir(), "shell.json"))
}

// frameGet walks a dotted path into a marshalled frame, failing if a segment is
// missing so an assertion never silently reads a nil.
func frameGet(t *testing.T, frame []byte, path string) any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(frame, &m); err != nil {
		t.Fatalf("frame is not a JSON object: %v", err)
	}
	v, err := getByPath(m, strings.Split(path, "."))
	if err != nil {
		t.Fatalf("frame %q: %v", path, err)
	}
	return v
}

// frameNum reads a numeric leaf out of a frame (JSON numbers decode to float64).
func frameNum(t *testing.T, frame []byte, path string) float64 {
	t.Helper()
	v, ok := frameGet(t, frame, path).(float64)
	if !ok {
		t.Fatalf("frame %q: not a number", path)
	}
	return v
}

func rm(s string) json.RawMessage { return json.RawMessage(s) }

// TestPatchRejectsInvalidSchemaValues checks a strict patch of a bad enum, an
// out-of-range integer, an empty required string, or an unknown enum member is
// rejected and never touches the file (the daemon commits only after a clean
// validate + persist).
func TestPatchRejectsInvalidSchemaValues(t *testing.T) {
	cases := []struct {
		name, path, val, want string
	}{
		{"bad enum mode", "theme.matugen.mode", `"Blue"`, "is not one of"},
		{"bad position", "menus.clock_menu.position", `"Sideways"`, "is not one of"},
		{"bad quick icon", "bars.widgets.quick_settings.icon", `"Ubuntu"`, "is not one of"},
		{"bad bar widget", "bars.top_bar.left_widgets", `["Nope"]`, "is not one of"},
		{"bad menu widget", "menus.clock_menu.widgets", `[{"type":"Bogus"}]`, "is not one of"},
		{"border too high", "theme.attributes.sizing.border_width", `999`, "out of range"},
		{"margin negative", "notifications.popup_window_margins", `-5`, "out of range"},
		{"min height too high", "bars.top_bar.minimum_height", `9000`, "out of range"},
		{"empty theme", "theme.theme", `""`, "must not be empty"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newTestStore(t)
			err := s.patch(c.path, rm(c.val))
			if err == nil {
				t.Fatalf("patch %s=%s: want error, got nil", c.path, c.val)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("patch %s=%s: error %q, want substring %q", c.path, c.val, err, c.want)
			}
			if _, statErr := os.Stat(s.path); !os.IsNotExist(statErr) {
				t.Fatalf("rejected patch wrote the file (stat err=%v); it must stay untouched", statErr)
			}
		})
	}
}

// TestPatchClampsFloatStrengths checks the float knobs clamp into range instead
// of erroring (the shipped newtypes clamp), so an out-of-range float patch
// succeeds with the boundary value.
func TestPatchClampsFloatStrengths(t *testing.T) {
	cases := []struct {
		path, val string
		want      float64
	}{
		{"theme.matugen.contrast", "9", 1},
		{"theme.matugen.contrast", "-9", -1},
		{"theme.attributes.window_opacity", "2", 1},
		{"theme.attributes.window_opacity", "-1", 0},
		{"theme.motion.scale", "9", 3},
		{"theme.motion.scale", "0.1", 0.25},
	}
	for _, c := range cases {
		t.Run(c.path+"="+c.val, func(t *testing.T) {
			s := newTestStore(t)
			if err := s.patch(c.path, rm(c.val)); err != nil {
				t.Fatalf("patch %s=%s: %v", c.path, c.val, err)
			}
			if got := frameNum(t, s.frameLocked(), c.path); got != c.want {
				t.Fatalf("patch %s=%s: clamped to %v, want %v", c.path, c.val, got, c.want)
			}
		})
	}
}

// TestPatchAcceptsValidSchemaValues checks in-range enum, integer, and list
// patches commit and show up in the next frame.
func TestPatchAcceptsValidSchemaValues(t *testing.T) {
	s := newTestStore(t)
	steps := []struct {
		path, val string
	}{
		{"theme.matugen.mode", `"Light"`},
		{"theme.attributes.sizing.border_width", `15`},
		{"menus.clock_menu.position", `"Right"`},
		{"bars.top_bar.left_widgets", `["Clock","Battery"]`},
	}
	for _, st := range steps {
		if err := s.patch(st.path, rm(st.val)); err != nil {
			t.Fatalf("patch %s=%s: %v", st.path, st.val, err)
		}
	}
	frame := s.frameLocked()
	if got := frameGet(t, frame, "theme.matugen.mode"); got != "Light" {
		t.Fatalf("mode = %v, want Light", got)
	}
	if got := frameNum(t, frame, "theme.attributes.sizing.border_width"); got != 15 {
		t.Fatalf("border_width = %v, want 15", got)
	}
	if got := frameGet(t, frame, "menus.clock_menu.position"); got != "Right" {
		t.Fatalf("position = %v, want Right", got)
	}
	ws, ok := frameGet(t, frame, "bars.top_bar.left_widgets").([]any)
	if !ok || len(ws) != 2 || ws[0] != "Clock" || ws[1] != "Battery" {
		t.Fatalf("left_widgets = %v, want [Clock Battery]", ws)
	}
}

// TestPatchPathResolution checks how a patch resolves its path: an unknown
// schema leaf is rejected (never silently created), a descent through a
// non-object errors, malformed paths error, and a passthrough key is created on
// merge.
func TestPatchPathResolution(t *testing.T) {
	t.Run("errors", func(t *testing.T) {
		cases := []struct {
			name, path, val, want string
		}{
			{"unknown schema leaf", "theme.bogus", `1`, "unknown setting"},
			{"unknown nested leaf", "theme.matugen.nope", `1`, "unknown setting"},
			{"descend through string", "theme.theme.x", `1`, "cannot descend"},
			{"empty path", "", `1`, "empty path"},
			{"empty segment", "theme..mode", `"Dark"`, "empty segment"},
			{"trailing dot", "menus.", `1`, "empty segment"},
			{"missing value", "theme.theme", ``, "missing value"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				s := newTestStore(t)
				err := s.patch(c.path, rm(c.val))
				if err == nil || !strings.Contains(err.Error(), c.want) {
					t.Fatalf("patch %q=%q: error %v, want substring %q", c.path, c.val, err, c.want)
				}
			})
		}
	})
	t.Run("passthrough create", func(t *testing.T) {
		s := newTestStore(t)
		if err := s.patch("frameRadius", rm(`9`)); err != nil {
			t.Fatalf("passthrough leaf: %v", err)
		}
		if err := s.patch("sidebar.width", rm(`340`)); err != nil {
			t.Fatalf("passthrough nested: %v", err)
		}
		frame := s.frameLocked()
		if got := frameNum(t, frame, "frameRadius"); got != 9 {
			t.Fatalf("frameRadius = %v, want 9", got)
		}
		if got := frameNum(t, frame, "sidebar.width"); got != 340 {
			t.Fatalf("sidebar.width = %v, want 340", got)
		}
	})
}

// TestPassthroughMergePreservesKeys checks passthrough and schema writes leave
// every unrelated key intact: the daemon merges one leaf at a time rather than
// rewriting the file from its own schema view.
func TestPatchPersistsPerDisplayShellVisibility(t *testing.T) {
	s := newTestStore(t)

	steps := []struct {
		path string
		val  string
	}{
		{"displays.bar.DP-1", `true`},
		{"displays.bar.DP-2", `false`},
		{"displays.widgets.DP-1", `true`},
		{"displays.widgets.DP-2", `false`},
	}

	for _, step := range steps {
		if err := s.patch(step.path, rm(step.val)); err != nil {
			t.Fatalf("patch %s=%s: %v", step.path, step.val, err)
		}
	}

	frame := s.frameLocked()

	for path, want := range map[string]bool{
		"displays.bar.DP-1":     true,
		"displays.bar.DP-2":     false,
		"displays.widgets.DP-1": true,
		"displays.widgets.DP-2": false,
	} {
		got, ok := frameGet(t, frame, path).(bool)
		if !ok || got != want {
			t.Fatalf("%s = %v, want %v", path, got, want)
		}
	}
}

func TestPassthroughMergePreservesKeys(t *testing.T) {
	s := newTestStore(t)
	for _, st := range []struct{ path, val string }{
		{"frameRadius", `9`},
		{"weatherLocation", `"Tokyo"`},
		{"sidebarWidth", `340`},
	} {
		if err := s.patch(st.path, rm(st.val)); err != nil {
			t.Fatalf("passthrough %s: %v", st.path, err)
		}
	}
	// A schema write must not disturb the passthrough keys.
	if err := s.patch("theme.matugen.mode", rm(`"Light"`)); err != nil {
		t.Fatalf("schema patch: %v", err)
	}
	frame := s.frameLocked()
	if got := frameNum(t, frame, "frameRadius"); got != 9 {
		t.Fatalf("frameRadius lost: %v", got)
	}
	if got := frameGet(t, frame, "weatherLocation"); got != "Tokyo" {
		t.Fatalf("weatherLocation lost: %v", got)
	}
	if got := frameGet(t, frame, "theme.matugen.mode"); got != "Light" {
		t.Fatalf("schema key not written: %v", got)
	}
	// A schema default a nobody touched is still present and correct.
	if got := frameGet(t, frame, "general.temperature_unit"); got != "Metric" {
		t.Fatalf("schema default lost: %v", got)
	}
	// A passthrough write must not disturb the earlier schema write.
	if err := s.patch("frameRadius", rm(`12`)); err != nil {
		t.Fatalf("passthrough re-patch: %v", err)
	}
	frame = s.frameLocked()
	if got := frameGet(t, frame, "theme.matugen.mode"); got != "Light" {
		t.Fatalf("schema key disturbed by passthrough write: %v", got)
	}
	if got := frameGet(t, frame, "weatherLocation"); got != "Tokyo" {
		t.Fatalf("passthrough sibling disturbed: %v", got)
	}
}

// TestAtomicWriteToDisk checks a committed patch lands in the file as valid JSON
// with no leftover temp file, and a rejected patch leaves the on-disk bytes
// exactly as they were.
func TestAtomicWriteToDisk(t *testing.T) {
	s := newTestStore(t)
	if err := s.patch("theme.matugen.mode", rm(`"Light"`)); err != nil {
		t.Fatalf("patch: %v", err)
	}
	b, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if got := frameGet(t, b, "theme.matugen.mode"); got != "Light" {
		t.Fatalf("on-disk mode = %v, want Light", got)
	}
	if len(b) == 0 || b[len(b)-1] != '\n' {
		t.Fatalf("persisted file should end with a newline")
	}
	if _, err := os.Stat(s.path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file left behind (stat err=%v); rename must consume it", err)
	}
	// A rejected patch must not alter the bytes on disk.
	before, _ := os.ReadFile(s.path)
	if err := s.patch("theme.theme", rm(`""`)); err == nil {
		t.Fatalf("empty theme.theme should be rejected")
	}
	after, _ := os.ReadFile(s.path)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("rejected patch changed the file on disk")
	}
}

// TestLoadClampsOutOfRangeIntegers checks a hand-edited file with an
// out-of-range integer still loads, clamped, rather than being rejected as
// malformed (load is lenient where a patch is strict).
func TestLoadClampsOutOfRangeIntegers(t *testing.T) {
	cases := []struct {
		name, body, path string
		want             int
	}{
		{"border high", `{"theme":{"attributes":{"sizing":{"border_width":999}}}}`, "theme.attributes.sizing.border_width", 20},
		{"border low", `{"theme":{"attributes":{"sizing":{"border_width":-5}}}}`, "theme.attributes.sizing.border_width", 0},
		{"margin high", `{"notifications":{"popup_window_margins":5000}}`, "notifications.popup_window_margins", 1000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "shell.json")
			if err := os.WriteFile(path, []byte(c.body), 0o644); err != nil {
				t.Fatalf("seed file: %v", err)
			}
			s := newSettingsStore(path)
			if got := frameNum(t, s.frameLocked(), c.path); got != float64(c.want) {
				t.Fatalf("%s: loaded %v, want clamped %d", c.path, got, c.want)
			}
		})
	}
}

// TestLoadCoercesRetiredTheme checks a file whose saved theme was retired from
// the catalog still loads: on a lenient load the unknown theme.theme coerces to
// "Wallpaper" (not the shipped "Default"), and the rest of the schema survives
// -- the whole file must not reset to defaults over one dropped enum value.
func TestLoadCoercesRetiredTheme(t *testing.T) {
	// Isolate from any installed theme library so "Nightfox" is genuinely unknown.
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "shell.json")
	body := `{"theme":{"theme":"Nightfox","attributes":{"sizing":{"border_width":5}}}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	s := newSettingsStore(path)
	if got := s.cur.Theme.Theme; got != "Wallpaper" {
		t.Fatalf("retired theme.theme = %q, want coerced to Wallpaper", got)
	}
	if got := s.cur.Theme.Attributes.Sizing.BorderWidth; got != 5 {
		t.Fatalf("border_width = %d after coerce, want 5 preserved (schema must not reset)", got)
	}
}

// TestMalformedFileFirstLoad checks the first-load fallback: an unparseable file
// yields shipped defaults with no passthrough keys, while a parseable file whose
// schema does not validate keeps its passthrough keys and resets only the schema.
func TestMalformedFileFirstLoad(t *testing.T) {
	t.Run("unparseable resets to defaults", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "shell.json")
		if err := os.WriteFile(path, []byte("not json {{{"), 0o644); err != nil {
			t.Fatalf("seed: %v", err)
		}
		s := newSettingsStore(path)
		if !reflect.DeepEqual(s.cur, defaultSettings()) {
			t.Fatalf("unparseable file did not fall back to defaults")
		}
		// Defaults only: exactly the six schema namespaces, no passthrough.
		if len(s.raw) != 6 {
			t.Fatalf("first-load fallback carried %d top-level keys, want 6 (schema only)", len(s.raw))
		}
		for _, k := range []string{"general", "theme", "bars", "menus", "notifications", "wallpaper"} {
			if _, ok := s.raw[k]; !ok {
				t.Fatalf("first-load fallback missing schema namespace %q", k)
			}
		}
	})
	t.Run("bad schema keeps passthrough resets schema", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "shell.json")
		body := `{"theme":{"theme":""},"frameRadius":42,"weatherLocation":"X"}`
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("seed: %v", err)
		}
		s := newSettingsStore(path)
		if s.cur.Theme.Theme != "Default" {
			t.Fatalf("bad schema not reset: theme.theme = %q", s.cur.Theme.Theme)
		}
		frame := s.frameLocked()
		if got := frameNum(t, frame, "frameRadius"); got != 42 {
			t.Fatalf("passthrough frameRadius dropped: %v", got)
		}
		if got := frameGet(t, frame, "weatherLocation"); got != "X" {
			t.Fatalf("passthrough weatherLocation dropped: %v", got)
		}
		if got := frameGet(t, frame, "theme.theme"); got != "Default" {
			t.Fatalf("schema not reset on frame: theme.theme = %v", got)
		}
	})
}

// TestReloadKeepsLastGood checks the reload path: a malformed external edit is
// ignored (last-good stays, no push), and a valid external edit is adopted and
// pushed.
func TestReloadKeepsLastGood(t *testing.T) {
	s := newTestStore(t)
	var notifies int
	s.onChange = func([]byte) { notifies++ }
	if err := s.patch("theme.matugen.mode", rm(`"Light"`)); err != nil {
		t.Fatalf("seed patch: %v", err)
	}
	if notifies != 1 {
		t.Fatalf("patch notifies = %d, want 1", notifies)
	}

	// A malformed on-disk edit is ignored: last-good state, no new frame.
	if err := os.WriteFile(s.path, []byte("garbage }{"), 0o644); err != nil {
		t.Fatalf("write garbage: %v", err)
	}
	s.reload()
	if s.cur.Theme.Matugen.Mode != "Light" {
		t.Fatalf("malformed reload lost last-good: mode = %q", s.cur.Theme.Matugen.Mode)
	}
	if notifies != 1 {
		t.Fatalf("malformed reload pushed a frame (notifies = %d)", notifies)
	}

	// A valid on-disk edit is adopted and pushed.
	if err := os.WriteFile(s.path, []byte(`{"theme":{"matugen":{"mode":"Dark"}}}`), 0o644); err != nil {
		t.Fatalf("write good: %v", err)
	}
	s.reload()
	if s.cur.Theme.Matugen.Mode != "Dark" {
		t.Fatalf("good reload not adopted: mode = %q", s.cur.Theme.Matugen.Mode)
	}
	if notifies != 2 {
		t.Fatalf("good reload did not push (notifies = %d, want 2)", notifies)
	}
}

// TestResetSemantics checks reset restores a schema key to its shipped default,
// errors for a passthrough key (no shipped default), and errors for unknown or
// malformed paths.
func TestResetSemantics(t *testing.T) {
	s := newTestStore(t)
	if err := s.patch("theme.matugen.mode", rm(`"Light"`)); err != nil {
		t.Fatalf("patch: %v", err)
	}
	if err := s.patch("notifications.popup_window_margins", rm(`50`)); err != nil {
		t.Fatalf("patch: %v", err)
	}
	if err := s.reset("theme.matugen.mode"); err != nil {
		t.Fatalf("reset mode: %v", err)
	}
	if err := s.reset("notifications.popup_window_margins"); err != nil {
		t.Fatalf("reset margins: %v", err)
	}
	frame := s.frameLocked()
	if got := frameGet(t, frame, "theme.matugen.mode"); got != "Dark" {
		t.Fatalf("reset mode = %v, want default Dark", got)
	}
	if got := frameNum(t, frame, "notifications.popup_window_margins"); got != 0 {
		t.Fatalf("reset margins = %v, want default 0", got)
	}

	// Resetting a passthrough key has no shipped default to restore.
	if err := s.patch("frameRadius", rm(`9`)); err != nil {
		t.Fatalf("seed passthrough: %v", err)
	}
	if err := s.reset("frameRadius"); err == nil || !strings.Contains(err.Error(), "passthrough key has no shipped default") {
		t.Fatalf("reset passthrough: error %v, want passthrough refusal", err)
	}
	if err := s.reset("theme.bogus"); err == nil || !strings.Contains(err.Error(), "unknown setting") {
		t.Fatalf("reset unknown: error %v, want unknown setting", err)
	}
	if err := s.reset(""); err == nil || !strings.Contains(err.Error(), "empty path") {
		t.Fatalf("reset empty: error %v, want empty path", err)
	}
}

// TestResetRestoresDefaultWidgetList checks reset works on a compound schema
// leaf (a menu widget list), not just scalars: it returns the whole shipped
// default value.
func TestResetRestoresDefaultWidgetList(t *testing.T) {
	s := newTestStore(t)
	if err := s.patch("menus.clipboard_menu.widgets", rm(`[{"type":"Clock"}]`)); err != nil {
		t.Fatalf("patch widgets: %v", err)
	}
	if err := s.reset("menus.clipboard_menu.widgets"); err != nil {
		t.Fatalf("reset widgets: %v", err)
	}
	ws, ok := frameGet(t, s.frameLocked(), "menus.clipboard_menu.widgets").([]any)
	if !ok || len(ws) != 1 {
		t.Fatalf("reset widget list = %v, want the shipped one-item default", ws)
	}
	w, _ := ws[0].(map[string]any)
	if w["type"] != "Clipboard" {
		t.Fatalf("reset widget list default = %v, want [{type:Clipboard}]", ws)
	}
}

// TestDefaultsValidate guards the shipped default against drift: it must pass
// its own strict validator, or a fresh install would boot on an invalid file.
func TestDefaultsValidate(t *testing.T) {
	if err := defaultSettings().normalize(true); err != nil {
		t.Fatalf("shipped defaults do not validate: %v", err)
	}
}

// TestFrameBarsPatchPreservesSubtrees enforces the subtree-preservation
// invariant: a whole-object passthrough patch that omits a subtree can never
// drop it from the store, while a subtree the patch does carry is replaced
// wholesale (so edits and removals still apply). This is the single chokepoint
// that guarantees no writer -- Bar Studio staging, save, revert, the dock-pinning
// path, or a hand edit -- can wipe a frameBars config subtree.
func TestFrameBarsPatchPreservesSubtrees(t *testing.T) {
	s := newTestStore(t)
	full := `{"version":1,"style":"slate-frame",` +
		`"rails":{"left":{"enabled":true,"size":48}},` +
		`"menus":{"wallpaper":{"anchor":"bottom-left","minWidth":1200},"clock":{"anchor":"left"}},` +
		`"surfaces":{"stash":{"anchor":"left"}},` +
		`"dock":{"pinned":["firefox"]}}`
	if err := s.patch("frameBars", rm(full)); err != nil {
		t.Fatalf("seed frameBars: %v", err)
	}

	// A partial write that carries only the rails subtree (thickness bumped),
	// omitting menus, surfaces, dock, style and version entirely.
	if err := s.patch("frameBars", rm(`{"rails":{"left":{"enabled":true,"size":64}}}`)); err != nil {
		t.Fatalf("partial frameBars: %v", err)
	}
	frame := s.frameLocked()

	// The carried subtree is updated.
	if got := frameNum(t, frame, "frameBars.rails.left.size"); got != 64 {
		t.Fatalf("rails.left.size = %v, want 64", got)
	}
	// Every omitted subtree survives with its exact contents (frameGet fails the
	// test if any segment is missing, so these calls assert presence directly).
	if got := frameGet(t, frame, "frameBars.menus.wallpaper.anchor"); got != "bottom-left" {
		t.Fatalf("menus subtree dropped by a partial write: %v", got)
	}
	if got := frameGet(t, frame, "frameBars.surfaces.stash.anchor"); got != "left" {
		t.Fatalf("surfaces subtree dropped: %v", got)
	}
	if got := frameGet(t, frame, "frameBars.dock.pinned"); got == nil {
		t.Fatalf("dock subtree dropped")
	}
	if got := frameGet(t, frame, "frameBars.style"); got != "slate-frame" {
		t.Fatalf("style subtree dropped: %v", got)
	}
	if got := frameNum(t, frame, "frameBars.version"); got != 1 {
		t.Fatalf("version dropped: %v", got)
	}

	// A subtree the patch DOES carry is replaced wholesale, so a removed member
	// (retiring the clock menu, say) actually goes away rather than lingering
	// behind a deep merge.
	if err := s.patch("frameBars", rm(`{"menus":{"wallpaper":{"anchor":"bottom-left","minWidth":1200}}}`)); err != nil {
		t.Fatalf("menus replace: %v", err)
	}
	frame = s.frameLocked()
	menus, ok := frameGet(t, frame, "frameBars.menus").(map[string]any)
	if !ok {
		t.Fatalf("menus not an object")
	}
	if _, present := menus["clock"]; present {
		t.Fatalf("carried menus subtree was merged, not replaced: clock lingered")
	}
	if _, present := menus["wallpaper"]; !present {
		t.Fatalf("wallpaper lost on menus replace")
	}
}

func TestPatchMergesAConcurrentSynchronizedWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shell.json")
	if err := os.WriteFile(path, []byte("{\"barStyle\":\"obi\",\"keep\":true}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := newSettingsStore(path)
	if err := os.WriteFile(path, []byte("{\"barStyle\":\"sumi\",\"keep\":true,\"external\":1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.patch("weatherLocation", json.RawMessage(`"Oslo"`)); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got["barStyle"] != "sumi" || got["keep"] != true || got["external"] != float64(1) || got["weatherLocation"] != "Oslo" {
		t.Fatalf("merged settings = %#v", got)
	}
}

func TestBarStyleCommandUsesSettingsStore(t *testing.T) {
	store := newTestStore(t)
	if err := store.patch(barStyleTransactionKey, json.RawMessage(`"remove-obi"`)); err != nil {
		t.Fatal(err)
	}
	d := daemon{settings: store}
	if got := d.dispatch("barstyle sumi"); got != "ok" {
		t.Fatalf("barstyle command = %q", got)
	}
	if got := frameGet(t, store.frameLocked(), "barStyle"); got != "sumi" {
		t.Fatalf("barStyle = %v", got)
	}
	if frameHas(t, store.frameLocked(), barStyleTransactionKey) {
		t.Fatal("explicit barstyle selection retained transaction marker")
	}
	if got := d.dispatch("barstyle ../obi"); !strings.HasPrefix(got, "err ") {
		t.Fatalf("invalid barstyle command = %q", got)
	}
}

func TestPatchRejectsMalformedConcurrentWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shell.json")
	if err := os.WriteFile(path, []byte("{\"barStyle\":\"obi\",\"keep\":true}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := newSettingsStore(path)
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.patch("weatherLocation", json.RawMessage(`"Oslo"`)); err == nil {
		t.Fatal("patch accepted malformed concurrent settings")
	}
	if raw, err := os.ReadFile(path); err != nil || string(raw) != "{" {
		t.Fatalf("malformed settings overwritten: %q, err=%v", raw, err)
	}
	if store.raw["barStyle"] != "obi" || store.raw["keep"] != true {
		t.Fatalf("last-good settings changed: %#v", store.raw)
	}
}

// TestLoadCorruptFileIsBackedUp: an existing but unparseable shell.json is
// preserved to <path>.corrupt instead of being silently reduced to defaults, so
// a transient or corrupt read never destroys the user's passthrough keys (qsbar
// and the other native look knobs) unrecoverably.
func TestLoadCorruptFileIsBackedUp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shell.json")
	junk := []byte(`{"barStyle":"qsbar","qsbar":{"barShellStyle":"full"}`) // truncated: unparseable
	if err := os.WriteFile(path, junk, 0o644); err != nil {
		t.Fatal(err)
	}
	raw, _, _ := loadSettingsFile(path)
	if _, ok := raw["qsbar"]; ok {
		t.Fatalf("unparseable file should fall back to defaults (no qsbar), got %v", raw)
	}
	b, err := os.ReadFile(path + ".corrupt")
	if err != nil {
		t.Fatalf("corrupt file was not backed up: %v", err)
	}
	if !reflect.DeepEqual(b, junk) {
		t.Fatalf(".corrupt backup does not match the original bytes")
	}
}
