package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	wm "ryoku-wm"
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

func TestNotificationDefaultsMatchContract(t *testing.T) {
	store := newTestStore(t)
	if got := frameGet(t, store.frameLocked(), "notifications.show_images"); got != false {
		t.Fatalf("notifications.show_images = %v, want false", got)
	}
}

func TestJapaneseGlossDefaultsOn(t *testing.T) {
	store := newTestStore(t)
	if got := frameGet(t, store.frameLocked(), "showJapaneseGloss"); got != true {
		t.Fatalf("showJapaneseGloss = %v, want true", got)
	}
}

// The shell's own glass and shadows default on, a file written before they
// existed (or holding only one) keeps the other on, and each switches alone.
func TestShellEffectsDefaultOnAndSwitchAlone(t *testing.T) {
	store := newTestStore(t)
	for _, key := range []string{"theme.effects.glass", "theme.effects.shadows"} {
		if got := frameGet(t, store.frameLocked(), key); got != true {
			t.Fatalf("%s = %v, want true", key, got)
		}
	}
	if err := store.patch("theme.effects.glass", rm(`false`)); err != nil {
		t.Fatalf("patch glass off: %v", err)
	}
	if got := frameGet(t, store.frameLocked(), "theme.effects.glass"); got != false {
		t.Fatalf("glass = %v after patching it off", got)
	}
	if got := frameGet(t, store.frameLocked(), "theme.effects.shadows"); got != true {
		t.Fatalf("shadows = %v, want them left on when glass goes off", got)
	}

	path := filepath.Join(t.TempDir(), "shell.json")
	if err := os.WriteFile(path, []byte(`{"theme":{"theme":"Default","effects":{"shadows":false}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s := newSettingsStore(path)
	if !s.cur.Theme.Effects.Glass || s.cur.Theme.Effects.Shadows {
		t.Fatalf("effects = %+v, want glass on by default and the stored shadows off", s.cur.Theme.Effects)
	}
}

func TestPhoneDefaultsMatchContract(t *testing.T) {
	store := newTestStore(t)
	var want any
	if err := json.Unmarshal([]byte(`{
		"enabled": false,
		"desktopNotifications": true,
		"contacts": {"enabled": true, "hideUnnamed": true},
		"scrcpy": {
			"audio": true,
			"useWireless": false,
			"autoWirelessIp": true,
			"wirelessIp": "",
			"wirelessPort": 5555,
			"pinAdbPort": false,
			"maxSize": 0,
			"maxFps": 60,
			"videoBitRate": "8M",
			"turnScreenOff": false,
			"stayAwake": true,
			"showTouches": false,
			"clipboardSync": true,
			"keyboardMode": "uhid",
			"autoResume": true,
			"appMode": {
				"showAppIcons": true,
				"autoUnlock": true,
				"flexDisplay": false,
				"displayWidth": 1280,
				"displayHeight": 960,
				"displayDpi": 240,
				"onSessionEnd": "home"
			},
			"recording": {"folder": ""}
		},
		"webcam": {"cameraFacing": "back", "width": 1280, "height": 720, "fps": 30, "videoDevice": ""},
		"microphone": {"micGain": 100, "monitor": false, "setDefault": false},
		"alarm": {"enabled": true, "pollMinutes": 10, "clockApps": [], "ignorePackages": []}
	}`), &want); err != nil {
		t.Fatal(err)
	}
	if got := frameGet(t, store.frameLocked(), "phone"); !reflect.DeepEqual(got, want) {
		t.Fatalf("phone defaults = %#v, want %#v", got, want)
	}
}

func TestDockEdgeGapValidation(t *testing.T) {
	store := newTestStore(t)
	if err := store.patch("dock", rm(`{"edgeGap":-15}`)); err != nil {
		t.Fatal(err)
	}
	if got := frameNum(t, store.frameLocked(), "dock.edgeGap"); got != -15 {
		t.Fatalf("dock.edgeGap = %v, want -15", got)
	}
	store = newSettingsStore(store.path)
	if got := frameNum(t, store.frameLocked(), "dock.edgeGap"); got != -15 {
		t.Fatalf("reloaded dock.edgeGap = %v, want -15", got)
	}
	if err := store.patch("dock.edgeGap", rm("24")); err != nil {
		t.Fatal(err)
	}
	if got := frameNum(t, store.frameLocked(), "dock.edgeGap"); got != 24 {
		t.Fatalf("dock.edgeGap = %v, want 24", got)
	}
	for _, value := range []string{"-25", "65", "1.5", `"wide"`} {
		if err := store.patch("dock.edgeGap", rm(value)); err == nil {
			t.Fatalf("dock.edgeGap accepted %s", value)
		}
		if got := frameNum(t, store.frameLocked(), "dock.edgeGap"); got != 24 {
			t.Fatalf("rejected patch changed dock.edgeGap to %v", got)
		}
	}
	if err := store.patch("dock", rm(`{"edgeGap":65}`)); err == nil {
		t.Fatal("whole dock patch accepted an out-of-range edge gap")
	}
	if got := frameNum(t, store.frameLocked(), "dock.edgeGap"); got != 24 {
		t.Fatalf("rejected dock object changed dock.edgeGap to %v", got)
	}

	for _, value := range []struct {
		gap  float64
		want int
	}{{80, dockEdgeGapMax}, {-30, dockEdgeGapMin}} {
		raw := map[string]any{"dock": map[string]any{"edgeGap": value.gap}}
		if err := normalizeDockSettings(raw, false); err != nil {
			t.Fatal(err)
		}
		if got := raw["dock"].(map[string]any)["edgeGap"]; got != value.want {
			t.Fatalf("lenient dock.edgeGap = %v, want %d", got, value.want)
		}
	}
}

// TestPatchRejectsInvalidSchemaValues checks bad schema values and retired
// namespaces are rejected without touching the file. The daemon commits only
// after a clean validation and persist.
func TestPatchRejectsInvalidSchemaValues(t *testing.T) {
	cases := []struct {
		name, path, val, want string
	}{
		{"bad enum fit", "wallpaper.content_fit", `"Blue"`, "is not one of"},
		{"retired matugen mode", "theme.matugen.mode", `"Light"`, "unknown setting"},
		{"bad glass type", "theme.effects.glass", `"off"`, "cannot unmarshal"},
		{"unknown effect", "theme.effects.blur", `true`, "unknown setting"},
		{"bad position", "menus.clock_menu.position", `"Sideways"`, "is not one of"},
		{"bad quick icon", "bars.widgets.quick_settings.icon", `"Ubuntu"`, "is not one of"},
		{"bad bar widget", "bars.top_bar.left_widgets", `["Nope"]`, "is not one of"},
		{"bad menu widget", "menus.clock_menu.widgets", `[{"type":"Bogus"}]`, "is not one of"},
		{"border too high", "theme.attributes.sizing.border_width", `999`, "out of range"},
		{"margin negative", "notifications.popup_window_margins", `-5`, "out of range"},
		{"bad image switch", "notifications.show_images", `1`, "cannot unmarshal"},
		{"min height too high", "bars.top_bar.minimum_height", `9000`, "out of range"},
		{"bad ask enabled type", "ask.bubble.enabled", `1`, "cannot unmarshal"},
		{"bad ask screen type", "ask.bubble.screen", `42`, "cannot unmarshal"},
		{"unknown ask key", "ask.bubble.nope", `true`, "unknown setting"},
		{"empty theme", "theme.theme", `""`, "must not be empty"},
		{"bad phone port", "phone.scrcpy.wirelessPort", `0`, "out of range"},
		{"bad phone keyboard mode", "phone.scrcpy.keyboardMode", `"raw"`, "is not one of"},
		{"bad phone session end", "phone.scrcpy.appMode.onSessionEnd", `"quit"`, "is not one of"},
		{"bad phone camera", "phone.webcam.cameraFacing", `"side"`, "is not one of"},
		{"bad phone fps", "phone.webcam.fps", `0`, "out of range"},
		{"bad phone gain", "phone.microphone.micGain", `201`, "out of range"},
		{"retired phone microphone switch", "phone.microphone.enabled", `true`, "unknown setting"},
		{"bad phone alarm interval", "phone.alarm.pollMinutes", `0`, "out of range"},
		{"bad controls position", "controls.position", `"left"`, "is not one of"},
		{"controls gap too high", "controls.edgeGap", `97`, "out of range"},
		{"controls gap is null", "controls.edgeGap", `null`, "must be an integer"},
		{"retired sidebar layout", "sidebars.layout", `"classic"`, "sidebars settings are retired"},
		{"retired sidebar subtree", "sidebars.left.width", `930`, "sidebars settings are retired"},
		{"retired sidebar root", "sidebars", `{"layout":"modern"}`, "sidebars settings are retired"},
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
		{"deckFrost", "2", 1},
		{"deckFrost", "-1", 0},
		{"theme.attributes.window_opacity", "2", 1},
		{"theme.attributes.window_opacity", "-1", 0},
		{"theme.motion.scale", "9", 3},
		{"theme.motion.scale", "0.1", 0.25},
		{"ask.bubble.x", "4", 1},
		{"ask.bubble.x", "-2", 0},
		{"ask.bubble.y", "3", 1},
		{"ask.bubble.y", "-1", 0},
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
		{"wallpaper.content_fit", `"Contain"`},
		{"theme.attributes.sizing.border_width", `15`},
		{"menus.clock_menu.position", `"Right"`},
		{"bars.top_bar.left_widgets", `["Clock","Battery"]`},
		{"ask.bubble.enabled", `true`},
		{"ask.bubble.screen", `"DP-2"`},
		{"phone.enabled", `true`},
		{"phone.desktopNotifications", `false`},
		{"phone.scrcpy.audio", `false`},
		{"phone.scrcpy.maxFps", `90`},
		{"phone.scrcpy.videoBitRate", `"12M"`},
		{"phone.webcam.cameraFacing", `"front"`},
		{"phone.microphone.micGain", `125`},
		{"phone.alarm.clockApps", `["com.google.android.deskclock"]`},
	}
	for _, st := range steps {
		if err := s.patch(st.path, rm(st.val)); err != nil {
			t.Fatalf("patch %s=%s: %v", st.path, st.val, err)
		}
	}
	frame := s.frameLocked()
	if got := frameGet(t, frame, "wallpaper.content_fit"); got != "Contain" {
		t.Fatalf("content_fit = %v, want Contain", got)
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
	if got := frameGet(t, frame, "ask.bubble.enabled"); got != true {
		t.Fatalf("ask.bubble.enabled = %v, want true", got)
	}
	if got := frameGet(t, frame, "ask.bubble.screen"); got != "DP-2" {
		t.Fatalf("ask.bubble.screen = %v, want DP-2", got)
	}
	if got := frameGet(t, frame, "phone.enabled"); got != true {
		t.Fatalf("phone.enabled = %v, want true", got)
	}
	if got := frameGet(t, frame, "phone.scrcpy.audio"); got != false {
		t.Fatalf("phone.scrcpy.audio = %v, want false", got)
	}
	if got := frameNum(t, frame, "phone.scrcpy.maxFps"); got != 90 {
		t.Fatalf("phone.scrcpy.maxFps = %v, want 90", got)
	}
	if got := frameGet(t, frame, "phone.webcam.cameraFacing"); got != "front" {
		t.Fatalf("phone.webcam.cameraFacing = %v, want front", got)
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
			{"unknown nested leaf", "theme.motion.nope", `1`, "unknown setting"},
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
	if err := s.patch("wallpaper.content_fit", rm(`"Contain"`)); err != nil {
		t.Fatalf("schema patch: %v", err)
	}
	frame := s.frameLocked()
	if got := frameNum(t, frame, "frameRadius"); got != 9 {
		t.Fatalf("frameRadius lost: %v", got)
	}
	if got := frameGet(t, frame, "weatherLocation"); got != "Tokyo" {
		t.Fatalf("weatherLocation lost: %v", got)
	}
	if got := frameGet(t, frame, "wallpaper.content_fit"); got != "Contain" {
		t.Fatalf("schema key not written: %v", got)
	}
	// A schema default a nobody touched is still present and correct.
	if got := frameGet(t, frame, "general.weather_location_query.type"); got != "Coordinates" {
		t.Fatalf("schema default lost: %v", got)
	}
	// A passthrough write must not disturb the earlier schema write.
	if err := s.patch("frameRadius", rm(`12`)); err != nil {
		t.Fatalf("passthrough re-patch: %v", err)
	}
	frame = s.frameLocked()
	if got := frameGet(t, frame, "wallpaper.content_fit"); got != "Contain" {
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
	if err := s.patch("wallpaper.content_fit", rm(`"Contain"`)); err != nil {
		t.Fatalf("patch: %v", err)
	}
	b, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if got := frameGet(t, b, "wallpaper.content_fit"); got != "Contain" {
		t.Fatalf("on-disk content_fit = %v, want Contain", got)
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
		// Defaults only: exactly the schema roots, no passthrough.
		if len(s.raw) != len(contractKeys) {
			t.Fatalf("first-load fallback carried %d top-level keys, want %d (schema only)", len(s.raw), len(contractKeys))
		}
		for _, k := range []string{
			"deckFrostEnabled", "deckFrost", "showJapaneseGloss", "general", "theme", "bars", "menus",
			"notifications", "wallpaper", "ask", "controls", "batteryAlerts",
		} {
			if _, ok := s.raw[k]; !ok {
				t.Fatalf("first-load fallback missing schema root %q", k)
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
	if err := s.patch("wallpaper.content_fit", rm(`"Contain"`)); err != nil {
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
	if s.cur.Wallpaper.ContentFit != "Contain" {
		t.Fatalf("malformed reload lost last-good: content_fit = %q", s.cur.Wallpaper.ContentFit)
	}
	if notifies != 1 {
		t.Fatalf("malformed reload pushed a frame (notifies = %d)", notifies)
	}

	// A valid on-disk edit is adopted and pushed.
	if err := os.WriteFile(s.path, []byte(`{"wallpaper":{"content_fit":"Cover"}}`), 0o644); err != nil {
		t.Fatalf("write good: %v", err)
	}
	s.reload()
	if s.cur.Wallpaper.ContentFit != "Cover" {
		t.Fatalf("good reload not adopted: content_fit = %q", s.cur.Wallpaper.ContentFit)
	}
	if notifies != 2 {
		t.Fatalf("good reload did not push (notifies = %d, want 2)", notifies)
	}
}

// TestResetSemantics checks reset restores a schema key to its shipped default
// and errors for unknown or malformed paths.
func TestResetSemantics(t *testing.T) {
	s := newTestStore(t)
	if err := s.patch("wallpaper.content_fit", rm(`"Contain"`)); err != nil {
		t.Fatalf("patch: %v", err)
	}
	if err := s.patch("notifications.popup_window_margins", rm(`50`)); err != nil {
		t.Fatalf("patch: %v", err)
	}
	if err := s.reset("wallpaper.content_fit"); err != nil {
		t.Fatalf("reset content_fit: %v", err)
	}
	if err := s.reset("notifications.popup_window_margins"); err != nil {
		t.Fatalf("reset margins: %v", err)
	}
	frame := s.frameLocked()
	if got := frameGet(t, frame, "wallpaper.content_fit"); got != "Cover" {
		t.Fatalf("reset content_fit = %v, want default Cover", got)
	}
	if got := frameNum(t, frame, "notifications.popup_window_margins"); got != 0 {
		t.Fatalf("reset margins = %v, want default 0", got)
	}

	if err := s.reset("theme.bogus"); err == nil || !strings.Contains(err.Error(), "unknown setting") {
		t.Fatalf("reset unknown: error %v, want unknown setting", err)
	}
	if err := s.reset(""); err == nil || !strings.Contains(err.Error(), "empty path") {
		t.Fatalf("reset empty: error %v, want empty path", err)
	}
}

// TestResetRemovesPassthroughKeys checks reset of a key without a schema removes
// it, so the shell's own default applies as on an install that never wrote it,
// and that a reset followed by a patch restores an object exactly, where a
// merging patch alone would keep sub-keys added since.
func TestResetRemovesPassthroughKeys(t *testing.T) {
	s := newTestStore(t)
	if err := s.patch("frameRadius", rm(`9`)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := s.reset("frameRadius"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if _, ok := s.raw["frameRadius"]; ok {
		t.Fatalf("reset left frameRadius stored")
	}
	if err := s.reset("frameRadius"); err != nil {
		t.Fatalf("reset of a key that is not stored: %v", err)
	}

	if err := s.patch("orpheus", rm(`{"drops":true}`)); err != nil {
		t.Fatalf("seed object: %v", err)
	}
	if err := s.patch("orpheus", rm(`{"glyphs":2}`)); err != nil {
		t.Fatalf("merge object: %v", err)
	}
	if err := s.reset("orpheus.glyphs"); err != nil {
		t.Fatalf("reset nested: %v", err)
	}
	if got := s.raw["orpheus"]; !reflect.DeepEqual(got, map[string]any{"drops": true}) {
		t.Fatalf("nested reset left orpheus = %v", got)
	}
	if err := s.patch("orpheus", rm(`{"glyphs":2}`)); err != nil {
		t.Fatalf("merge again: %v", err)
	}
	if err := s.reset("orpheus"); err != nil {
		t.Fatalf("reset object: %v", err)
	}
	if err := s.patch("orpheus", rm(`{"drops":true}`)); err != nil {
		t.Fatalf("restore object: %v", err)
	}
	if got := s.raw["orpheus"]; !reflect.DeepEqual(got, map[string]any{"drops": true}) {
		t.Fatalf("reset then patch restored orpheus = %v, want exactly the old object", got)
	}

	if err := s.patch("barStyle", rm(`"sumi"`)); err != nil {
		t.Fatalf("seed bar style: %v", err)
	}
	if err := s.patch(barStyleTransactionKey, rm(`{"from":"iris"}`)); err != nil {
		t.Fatalf("seed marker: %v", err)
	}
	if err := s.reset("barStyle"); err != nil {
		t.Fatalf("reset bar style: %v", err)
	}
	if _, ok := s.raw[barStyleTransactionKey]; ok {
		t.Fatalf("resetting the bar style kept the Store removal's marker")
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

func TestControlsNormalization(t *testing.T) {
	s := newTestStore(t)
	value := `{"sections":[{"id":"levels","visible":false},{"id":"unknown","visible":true},{"id":"levels","visible":true}],"hidden":["liveGraph","unknown","liveGraph"]}`
	if err := s.patch("controls", rm(value)); err != nil {
		t.Fatalf("patch controls: %v", err)
	}

	controls := frameGet(t, s.frameLocked(), "controls").(map[string]any)
	sections := controls["sections"].([]any)
	if len(sections) != len(controlSectionIDs) {
		t.Fatalf("normalized sections = %d, want %d: %v", len(sections), len(controlSectionIDs), sections)
	}
	first := sections[0].(map[string]any)
	if first["id"] != "levels" || first["visible"] != false {
		t.Fatalf("first normalized section = %v, want hidden levels", first)
	}
	for _, raw := range sections {
		if raw.(map[string]any)["id"] == "unknown" {
			t.Fatalf("unknown section survived normalization: %v", sections)
		}
	}
	hidden := controls["hidden"].([]any)
	if len(hidden) != 1 || hidden[0] != "liveGraph" {
		t.Fatalf("normalized hidden = %v, want [liveGraph]", hidden)
	}

	if controls["position"] != "top-left" {
		t.Fatalf("missing controls position = %v, want top-left", controls["position"])
	}
	if controls["edgeGap"] != float64(12) {
		t.Fatalf("missing controls edgeGap = %v, want 12", controls["edgeGap"])
	}
}

func TestControlsGeometrySettings(t *testing.T) {
	s := newTestStore(t)
	if got := frameGet(t, s.frameLocked(), "controls.position"); got != "top-left" {
		t.Fatalf("controls.position default = %v, want top-left", got)
	}
	if got := frameNum(t, s.frameLocked(), "controls.edgeGap"); got != 12 {
		t.Fatalf("controls.edgeGap default = %v, want 12", got)
	}
	if err := s.patch("controls.position", rm(`"top-center"`)); err != nil {
		t.Fatalf("patch controls position: %v", err)
	}
	if err := s.patch("controls.edgeGap", rm(`0`)); err != nil {
		t.Fatalf("patch controls edge gap: %v", err)
	}
	if got := frameGet(t, s.frameLocked(), "controls.position"); got != "top-center" {
		t.Fatalf("controls.position = %v, want top-center", got)
	}
	if got := frameNum(t, s.frameLocked(), "controls.edgeGap"); got != 0 {
		t.Fatalf("controls.edgeGap = %v, want 0", got)
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
	if err := os.WriteFile(path, []byte("{\"barStyle\":\"alpha\",\"keep\":true}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := newSettingsStore(path)
	if err := os.WriteFile(path, []byte("{\"barStyle\":\"beta\",\"keep\":true,\"external\":1}\n"), 0o644); err != nil {
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
	if got["barStyle"] != "beta" || got["keep"] != true || got["external"] != float64(1) || got["weatherLocation"] != "Oslo" {
		t.Fatalf("merged settings = %#v", got)
	}
}

func TestBarStyleCommandUsesSettingsStoreAndCommits(t *testing.T) {
	store := newTestStore(t)
	if err := store.patch(barStyleTransactionKey, json.RawMessage(`"remove-alpha"`)); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	oldSync := runRyostoreShellsSync
	t.Cleanup(func() { runRyostoreShellsSync = oldSync })
	runRyostoreShellsSync = func(bool) error {
		index := shellIndex{Schema: 1, Active: "beta", Fallback: "beta", Items: []shellItem{{ID: "beta", Category: "subshells", InUse: true}}}
		body, _ := json.Marshal(index)
		path := shellsIndexPath()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, body, 0o600)
	}
	d := daemon{settings: store}
	if got := d.dispatch("barstyle beta"); got != "ok" {
		t.Fatalf("barstyle command = %q", got)
	}
	if got := frameGet(t, store.frameLocked(), "barStyle"); got != "beta" {
		t.Fatalf("barStyle = %v", got)
	}
	if frameHas(t, store.frameLocked(), barStyleTransactionKey) {
		t.Fatal("explicit barstyle selection retained transaction marker")
	}
	if got := d.dispatch("barstyle ../alpha"); !strings.HasPrefix(got, "err ") {
		t.Fatalf("invalid barstyle command = %q", got)
	}
}

// A Store dock removal marks the design it moved to its fallback; once the user
// edits the dock themselves, recovery must not put the removed design back.
func TestDockEditDropsTheStoreRemovalMarker(t *testing.T) {
	store := newTestStore(t)
	if err := store.patch(dockTransactionKey, json.RawMessage(`"engawa-1"`)); err != nil {
		t.Fatal(err)
	}
	if err := store.patch("weatherLocation", json.RawMessage(`"Oslo"`)); err != nil {
		t.Fatal(err)
	}
	if !frameHas(t, store.frameLocked(), dockTransactionKey) {
		t.Fatal("an unrelated edit dropped the dock marker")
	}
	if err := store.patch("dock", json.RawMessage(`{"design":"ryoku","autohide":false}`)); err != nil {
		t.Fatal(err)
	}
	if frameHas(t, store.frameLocked(), dockTransactionKey) {
		t.Fatal("a dock edit retained the store removal marker")
	}
}

// An overlay design the user picks drops only that design's removal marker; the
// other designs' recoveries stay theirs. Replacing the whole designs object
// drops all three.
func TestDesignEditDropsOnlyItsStoreRemovalMarker(t *testing.T) {
	store := newTestStore(t)
	for _, marker := range designTransactionKeys {
		if err := store.patch(marker, json.RawMessage(`"remove-1"`)); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.patch("designs.osd", json.RawMessage(`"ito"`)); err != nil {
		t.Fatal(err)
	}
	frame := store.frameLocked()
	if frameHas(t, frame, designTransactionKeys["osd"]) {
		t.Fatal("an OSD design edit retained its store removal marker")
	}
	if !frameHas(t, frame, designTransactionKeys["notification"]) || !frameHas(t, frame, designTransactionKeys["sidebar"]) {
		t.Fatal("an OSD design edit dropped another design's marker")
	}
	if err := store.patch("designs", json.RawMessage(`{"osd":"ryoku","notification":"ryoku","sidebar":"ryoku"}`)); err != nil {
		t.Fatal(err)
	}
	for design, marker := range designTransactionKeys {
		if frameHas(t, store.frameLocked(), marker) {
			t.Fatalf("replacing designs retained the %s marker", design)
		}
	}
}

func TestPatchRejectsMalformedConcurrentWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shell.json")
	if err := os.WriteFile(path, []byte("{\"barStyle\":\"alpha\",\"keep\":true}\n"), 0o644); err != nil {
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
	if store.raw["barStyle"] != "alpha" || store.raw["keep"] != true {
		t.Fatalf("last-good settings changed: %#v", store.raw)
	}
}

// TestLoadCorruptFileIsBackedUp: an existing but unparseable shell.json is
// preserved to <path>.corrupt instead of being silently reduced to defaults, so
// a transient read never destroys the user's passthrough keys.
func TestLoadCorruptFileIsBackedUp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shell.json")
	junk := []byte(`{"barStyle":"alpha","customShell":{"shape":"full"}`)
	if err := os.WriteFile(path, junk, 0o644); err != nil {
		t.Fatal(err)
	}
	raw, _, _ := loadSettingsFile(path)
	if _, ok := raw["customShell"]; ok {
		t.Fatalf("unparseable file should fall back to defaults, got %v", raw)
	}
	b, err := os.ReadFile(path + ".corrupt")
	if err != nil {
		t.Fatalf("corrupt file was not backed up: %v", err)
	}
	if !reflect.DeepEqual(b, junk) {
		t.Fatalf(".corrupt backup does not match the original bytes")
	}
}

// App rules reads the active provider's action list and look fields off the
// same settings frame it reads values from, so the frame carries them beside
// caps and deadKeys. A bare store (no provider probed yet) adds no gating keys,
// and a provider that lists none must still emit [] so the Hub never reads a
// null.
func TestFrameCarriesRuleLists(t *testing.T) {
	s := newTestStore(t)
	for _, key := range []string{"windowRuleActions", "appOverrideFields"} {
		if frameHas(t, s.frameLocked(), key) {
			t.Fatalf("bare store leaked %s before a provider answered", key)
		}
	}

	s.caps = map[string]bool{"windowRules": true}
	s.deadKeys = []string{}
	s.windowRuleActions = []string{"float", "tile", "opacity"}
	s.appOverrideFields = []string{"opacity", "rounding"}
	for key, want := range map[string][]string{
		"windowRuleActions": {"float", "tile", "opacity"},
		"appOverrideFields": {"opacity", "rounding"},
	} {
		got, ok := frameGet(t, s.frameLocked(), key).([]any)
		if !ok || len(got) != len(want) {
			t.Fatalf("%s = %v, want %v", key, got, want)
		}
		for i, v := range want {
			if got[i] != v {
				t.Errorf("%s[%d] = %v, want %v", key, i, got[i], v)
			}
		}
	}

	s.windowRuleActions = nil
	s.appOverrideFields = nil
	for _, key := range []string{"windowRuleActions", "appOverrideFields"} {
		empty, ok := frameGet(t, s.frameLocked(), key).([]any)
		if !ok || len(empty) != 0 {
			t.Fatalf("empty %s = %v, want []", key, empty)
		}
	}
}

// A store leaf only another installed provider models is dead, so its row
// hides: Window style's blur behind windows goes where the active window
// manager draws none, and comes back where it does.
func TestDeadSettingKeysFollowTheActiveProvider(t *testing.T) {
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	provider := func(name, defaults string) {
		script := "#!/bin/sh\ncase \"$1\" in\n  caps) printf '%s' '{\"name\":\"" + name + "\"}' ;;\n  defaults) printf '%s' '" + defaults + "' ;;\nesac\n"
		if err := os.WriteFile(filepath.Join(bin, "ryoku-wm-"+name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	names := wm.Providers()
	active, other := names[0], names[1]
	provider(active, `{"desktop":{"appearance":{"blurEnabled":false,"shadowEnabled":true,"rounding":0}}}`)
	provider(other, `{"desktop":{"appearance":{"blurEnabled":true,"gapsIn":4}}}`)

	d := &daemon{wmc: wm.OpenNamed(active)}
	dead := d.deadSettingKeys()
	if !slices.Contains(dead, "desktop.appearance.gapsIn") || slices.Contains(dead, "desktop.appearance.rounding") || slices.Contains(dead, "desktop.appearance.blurEnabled") {
		t.Fatalf("dead = %v, want gapsIn dead, rounding and blurEnabled live", dead)
	}

	provider(active, `{"desktop":{"appearance":{"shadowEnabled":true,"rounding":0}}}`)
	if dead = d.deadSettingKeys(); !slices.Contains(dead, "desktop.appearance.blurEnabled") {
		t.Fatalf("dead = %v, want blurEnabled dead where the window manager draws no blur", dead)
	}
}

// A list a provider models (its layer rules, its animation items) is a key the
// other provider's page must read as dead; dropping lists from the flattening
// left every list-keyed page visible on both compositors.
func TestFlattenLeavesCountsListsAsKeys(t *testing.T) {
	var tree map[string]any
	if err := json.Unmarshal([]byte(`{"desktop":{"appearance":{"rounding":8},"windowRules":[]},"wm":{"x":{"layerRules":[],"anim":{"items":[{"name":"a"}]}}}}`), &tree); err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	flattenLeaves("", tree, got)
	for _, want := range []string{"desktop.appearance.rounding", "desktop.windowRules", "wm.x.layerRules", "wm.x.anim.items"} {
		if !got[want] {
			t.Errorf("missing key %q in %v", want, got)
		}
	}
	if got["wm.x.anim.items.name"] {
		t.Errorf("descended into a list: %v", got)
	}
}

// boolAt is how a subsystem reads a switch the daemon does not model. The
// clipboard's weekly sweep is the caller: a passthrough key has to be readable
// as a boolean, and anything missing or of the wrong type must read as off
// rather than as an error.
func TestSettingsBoolAt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shell.json")
	body := `{"clipboard":{"pruneWeekly":true,"widthPercent":65},"weatherLocation":"Berlin"}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	s := newSettingsStore(path)
	cases := []struct {
		path string
		want bool
	}{
		{"clipboard.pruneWeekly", true},   // a passthrough boolean
		{"clipboard.missing", false},      // absent leaf
		{"clipboard.widthPercent", false}, // present, but not a boolean
		{"clipboard", false},              // descends through the object
		{"weatherLocation", false},        // a string
	}
	for _, c := range cases {
		if got := s.boolAt(c.path); got != c.want {
			t.Errorf("boolAt(%q) = %v, want %v", c.path, got, c.want)
		}
	}
	// A patch through the store is visible to the next read, which is what makes
	// the Hub's toggle live for the sweep.
	if err := s.patch("clipboard.pruneWeekly", json.RawMessage(``+`false`)); err != nil {
		t.Fatalf("patch: %v", err)
	}
	if s.boolAt("clipboard.pruneWeekly") {
		t.Error("boolAt still true after the key was patched off")
	}
}

func startSettingsWatch(t *testing.T, s *settingsStore) {
	t.Helper()
	quit := make(chan struct{})
	ready := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.watch(quit, ready)
	}()
	<-ready
	t.Cleanup(func() {
		close(quit)
		<-done
	})
}

func TestSettingsWatchFollowsRenameReplace(t *testing.T) {
	s := newTestStore(t)
	if err := s.patch("weatherLocation", rm(`"before"`)); err != nil {
		t.Fatal(err)
	}
	changed := make(chan []byte, 1)
	s.onChange = func(frame []byte) { changed <- frame }
	startSettingsWatch(t, s)

	body, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	doc["weatherLocation"] = "after"
	body, err = json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	tmp := s.path + ".editor"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		t.Fatal(err)
	}

	select {
	case frame := <-changed:
		if got := frameGet(t, frame, "weatherLocation"); got != "after" {
			t.Fatalf("weatherLocation = %v, want after", got)
		}
	case <-time.After(time.Second):
		t.Fatal("rename-replaced shell.json did not reload")
	}
}

func TestSettingsWatchFollowsMissingParent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "ryoku", "shell.json")
	s := newSettingsStore(path)
	changed := make(chan []byte, 1)
	s.onChange = func(frame []byte) { changed <- frame }
	startSettingsWatch(t, s)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"weatherLocation":"after"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case frame := <-changed:
		if got := frameGet(t, frame, "weatherLocation"); got != "after" {
			t.Fatalf("weatherLocation = %v, want after", got)
		}
	case <-time.After(time.Second):
		t.Fatal("shell.json under a newly-created parent did not reload")
	}
}

func TestSettingsWatchReloadsAFinishedRewriteThatKeepsItsStat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shell.json")
	s := newSettingsStore(path)
	changed := make(chan []byte, 2)
	s.onChange = func(frame []byte) { changed <- frame }
	startSettingsWatch(t, s)

	if err := os.WriteFile(path, []byte(`{"weatherLocation":"alpha"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-changed:
	case <-time.After(time.Second):
		t.Fatal("initial shell.json did not load")
	}
	// the create and the finished write can each publish; settle first
settle:
	for {
		select {
		case <-changed:
		case <-time.After(50 * time.Millisecond):
			break settle
		}
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	stamp := fi.ModTime()

	// rewrite in place and put the old mtime back before closing: the watcher
	// only sees the finished write, with the stat it already latched
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"weatherLocation":"bravo"}`); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	select {
	case frame := <-changed:
		if got := frameGet(t, frame, "weatherLocation"); got != "bravo" {
			t.Fatalf("weatherLocation = %v, want bravo", got)
		}
	case <-time.After(time.Second):
		t.Fatal("a finished rewrite under the latched stat did not reload")
	}
}
