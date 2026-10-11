package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	wm "ryoku-wm"
)

// apply is pure store -> ini, so these exercise the composer directly: the
// layer order the F1 contract pinned, the store's own sections, the switch cost
// list a user reads, and the report shape. The composed file itself is proven
// by byte-pinned merges rather than by running wayfire, since wayfire has no
// validate verb and the nested-session proof lives in the E2E suite.

// wayfireHome points the provider's config and overlay trees at a temp dir and
// pins the state dir, so the palette file and the greeter hand-off cannot leak
// in from the dev box.
func wayfireHome(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	return wayfireConfigDir()
}

// withDefaults points the shipped-defaults layer at a fixture body and restores
// the package path afterwards.
func withDefaults(t *testing.T, body string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "wayfire.ini")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	prev := shareDefaultsPath
	shareDefaultsPath = p
	t.Cleanup(func() { shareDefaultsPath = prev })
}

func writeStore(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "desktop.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func writeSeed(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// mustCompose is compose for the tests that need its bytes: the shipped
// defaults are a fixture by then, so a failure is the test's own setup.
func mustCompose(t *testing.T, s wayfireStore) []byte {
	t.Helper()
	b, err := compose(s)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	return b
}

// capApply runs apply with stdout captured and returns the decoded report.
func capApply(t *testing.T, args ...string) wm.ApplyReport {
	t.Helper()
	var buf bytes.Buffer
	prev := stdout
	stdout = bufio.NewWriter(&buf)
	defer func() { stdout = prev }()
	if err := runApply(args); err != nil {
		t.Fatalf("runApply(%v): %v", args, err)
	}
	stdout.Flush()
	var rep wm.ApplyReport
	if err := json.Unmarshal(buf.Bytes(), &rep); err != nil {
		t.Fatalf("decode report: %v\n%s", err, buf.String())
	}
	return rep
}

func readGen(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

// The composition order is the F1 contract: defaults, store, seeds with
// user.ini last, each layer winning per key. The expected body is exact, so a
// change in ordering or spelling has to be a deliberate edit here.
func TestComposeLastWins(t *testing.T) {
	dir := wayfireHome(t)
	withDefaults(t, "[core]\nvwidth = 3\nplugins = animate blur grid\n\n[decoration]\nborder_size = 8\n")
	writeSeed(t, dir, "keyboard.ini", "[input]\nxkb_layout = us\n")
	writeSeed(t, dir, "user.ini", "[decoration]\nborder_size = 1\n")
	store := writeStore(t, `{"desktop":{"appearance":{"borderSize":4,"activeBorder":"#112233","inactiveBorder":"#445566","borderFollowsPalette":false,"animations":true,"blurEnabled":true},"input":{"kbLayout":"de"}}}`)

	got := string(mustCompose(t, loadStore(store)))
	want := generatedHeader + `
[core]
vwidth = 3
plugins = animate blur grid

[decoration]
border_size = 1
active_color = \#112233FF
inactive_color = \#445566FF

[input]
xkb_layout = us
xkb_variant = 
xkb_options = 
kb_numlock_default_state = false
mouse_cursor_speed = 0
mouse_accel_profile = default
left_handed_mode = false
mouse_natural_scroll = false
mouse_scroll_speed = 1
natural_scroll = true
touchpad_scroll_speed = 1
tap_to_click = true
tap_and_drag = true
click_method = buttonareas
middle_emulation = false
disable_touchpad_while_typing = true
kb_repeat_rate = 25
kb_repeat_delay = 600
cursor_theme = Bibata-Modern-Ice
cursor_size = 24

[vswipe]
enable_vertical = false
enable_horizontal = false
fingers = 3

[vswitch]
duration = 300ms circle
binding_1 = <super> KEY_1 | <super> KEY_KP1
binding_2 = <super> KEY_2 | <super> KEY_KP2
binding_3 = <super> KEY_3 | <super> KEY_KP3
binding_4 = <super> KEY_4 | <super> KEY_KP4
binding_5 = <super> KEY_5 | <super> KEY_KP5
binding_6 = <super> KEY_6 | <super> KEY_KP6
binding_7 = <super> KEY_7 | <super> KEY_KP7
binding_8 = <super> KEY_8 | <super> KEY_KP8
binding_9 = <super> KEY_9 | <super> KEY_KP9
with_win_1 = <super> <alt> KEY_1 | <super> <alt> KEY_KP1
with_win_2 = <super> <alt> KEY_2 | <super> <alt> KEY_KP2
with_win_3 = <super> <alt> KEY_3 | <super> <alt> KEY_KP3
with_win_4 = <super> <alt> KEY_4 | <super> <alt> KEY_KP4
with_win_5 = <super> <alt> KEY_5 | <super> <alt> KEY_KP5
with_win_6 = <super> <alt> KEY_6 | <super> <alt> KEY_KP6
with_win_7 = <super> <alt> KEY_7 | <super> <alt> KEY_KP7
with_win_8 = <super> <alt> KEY_8 | <super> <alt> KEY_KP8
with_win_9 = <super> <alt> KEY_9 | <super> <alt> KEY_KP9
send_win_1 = <super> <shift> KEY_1 | <super> <shift> KEY_KP1
send_win_2 = <super> <shift> KEY_2 | <super> <shift> KEY_KP2
send_win_3 = <super> <shift> KEY_3 | <super> <shift> KEY_KP3
send_win_4 = <super> <shift> KEY_4 | <super> <shift> KEY_KP4
send_win_5 = <super> <shift> KEY_5 | <super> <shift> KEY_KP5
send_win_6 = <super> <shift> KEY_6 | <super> <shift> KEY_KP6
send_win_7 = <super> <shift> KEY_7 | <super> <shift> KEY_KP7
send_win_8 = <super> <shift> KEY_8 | <super> <shift> KEY_KP8
send_win_9 = <super> <shift> KEY_9 | <super> <shift> KEY_KP9

[animate]
duration = 400ms circle

[grid]
duration = 300ms circle

[depthdeck]
enabled = true
scatter = true
card_edge_scatter = true
card_scatter_reshuffle = true
card_peek_min = 24
card_peek_max = 80
animation_ms = 200
layer_1_opacity = 0.85
layer_1_scale = 0.7
layer_2_opacity = 0.7
layer_2_scale = 0.5
max_layers = 8
maximized_front_scale = 0.9

[command]
command_window_close = jq -e '(.barStyle // "qsbar") == "iris" and .inir.closeConfirm.enabled == true' "${XDG_CONFIG_HOME:-$HOME/.config}/ryoku/shell.json" >/dev/null 2>&1 && qs -c shell ipc call closeConfirm trigger || ryoku-wm-wayfire act window.close focused
binding_window_close = <super> KEY_Q | <alt> KEY_F4
command_window_float = ryoku-wm-wayfire act window.float focused
binding_window_float = <super> KEY_A
command_window_preset_height = ryoku-wm-wayfire act window.presetHeight
binding_window_preset_height = <super> <shift> KEY_R
command_column_center = ryoku-wm-wayfire act window.center
binding_column_center = <super> KEY_C
command_focus_left = ryoku-wm-wayfire act window.focusDirection left
binding_focus_left = <super> KEY_LEFT
command_focus_right = ryoku-wm-wayfire act window.focusDirection right
binding_focus_right = <super> KEY_RIGHT
command_focus_up = ryoku-wm-wayfire act window.focusDirection up
binding_focus_up = <super> KEY_UP
command_focus_down = ryoku-wm-wayfire act window.focusDirection down
binding_focus_down = <super> KEY_DOWN
command_column_first = ryoku-wm-wayfire act window.focusEdge left
binding_column_first = <super> KEY_HOME
command_column_last = ryoku-wm-wayfire act window.focusEdge right
binding_column_last = <super> KEY_END
command_move_left = ryoku-wm-wayfire act window.moveBy left
binding_move_left = <super> <shift> KEY_LEFT
command_move_right = ryoku-wm-wayfire act window.moveBy right
binding_move_right = <super> <shift> KEY_RIGHT
command_move_up = ryoku-wm-wayfire act window.moveBy up
binding_move_up = <super> <shift> KEY_UP
command_move_down = ryoku-wm-wayfire act window.moveBy down
binding_move_down = <super> <shift> KEY_DOWN
command_resize_narrower = ryoku-wm-wayfire act window.resizeBy narrower
binding_resize_narrower = <super> <ctrl> KEY_LEFT
command_resize_wider = ryoku-wm-wayfire act window.resizeBy wider
binding_resize_wider = <super> <ctrl> KEY_RIGHT
command_resize_shorter = ryoku-wm-wayfire act window.resizeBy shorter
binding_resize_shorter = <super> <ctrl> KEY_UP
command_resize_taller = ryoku-wm-wayfire act window.resizeBy taller
binding_resize_taller = <super> <ctrl> KEY_DOWN
command_resize_reset_height = ryoku-wm-wayfire act window.presetHeight first
binding_resize_reset_height = <super> <ctrl> KEY_R
command_workspace_prev = ryoku-wm-wayfire act workspace.cycle -1
binding_workspace_prev = <super> KEY_PAGEUP
command_workspace_next = ryoku-wm-wayfire act workspace.cycle 1
binding_workspace_next = <super> KEY_PAGEDOWN
command_workspace_move_window_prev = ryoku-wm-wayfire act window.moveToWorkspaceBy -1
binding_workspace_move_window_prev = <super> <shift> KEY_PAGEUP
command_workspace_move_window_next = ryoku-wm-wayfire act window.moveToWorkspaceBy 1
binding_workspace_move_window_next = <super> <shift> KEY_PAGEDOWN
command_workspace_overview = ryoku-shell overview
binding_workspace_overview = <super> KEY_TAB
command_workspace_overview_desktops = ryoku-shell overview
binding_workspace_overview_desktops = <super> <alt> KEY_TAB
command_display_focus_left = ryoku-wm-wayfire act output.focusDirection left
binding_display_focus_left = <super> <alt> KEY_LEFT
command_display_focus_right = ryoku-wm-wayfire act output.focusDirection right
binding_display_focus_right = <super> <alt> KEY_RIGHT
command_display_focus_up = ryoku-wm-wayfire act output.focusDirection up
binding_display_focus_up = <super> <alt> KEY_UP
command_display_focus_down = ryoku-wm-wayfire act output.focusDirection down
binding_display_focus_down = <super> <alt> KEY_DOWN
command_display_move_window_left = ryoku-wm-wayfire act window.moveToOutputBy left
binding_display_move_window_left = <super> <alt> <shift> KEY_LEFT
command_display_move_window_right = ryoku-wm-wayfire act window.moveToOutputBy right
binding_display_move_window_right = <super> <alt> <shift> KEY_RIGHT
command_display_move_window_up = ryoku-wm-wayfire act window.moveToOutputBy up
binding_display_move_window_up = <super> <alt> <shift> KEY_UP
command_display_move_window_down = ryoku-wm-wayfire act window.moveToOutputBy down
binding_display_move_window_down = <super> <alt> <shift> KEY_DOWN
command_display_move_workspace_left = ryoku-wm-wayfire act workspace.moveToOutputBy left
binding_display_move_workspace_left = <super> <ctrl> <alt> KEY_LEFT
command_display_move_workspace_right = ryoku-wm-wayfire act workspace.moveToOutputBy right
binding_display_move_workspace_right = <super> <ctrl> <alt> KEY_RIGHT
command_display_move_workspace_up = ryoku-wm-wayfire act workspace.moveToOutputBy up
binding_display_move_workspace_up = <super> <ctrl> <alt> KEY_UP
command_display_move_workspace_down = ryoku-wm-wayfire act workspace.moveToOutputBy down
binding_display_move_workspace_down = <super> <ctrl> <alt> KEY_DOWN
command_display_cycle = ryoku-wm-wayfire act output.cycle
binding_display_cycle = <super> KEY_P
command_app_terminal = ryoku-app terminal
binding_app_terminal = <super> KEY_ENTER
command_app_files = ryoku-app files
binding_app_files = <super> KEY_E
command_app_browser = ryoku-app browser
binding_app_browser = <super> KEY_B
command_app_editor = ryoku-app editor
binding_app_editor = <super> KEY_N
command_app_notes = ryoku-app notes
binding_app_notes = <super> KEY_O
command_app_yazi = kitty -e yazi
binding_app_yazi = <super> <alt> KEY_E
command_app_ryotunes = ryotunes
binding_app_ryotunes = <super> KEY_J
command_shell_launcher = ryoku-shell launcher
binding_shell_launcher = <super> KEY_SPACE
command_shell_ask = ryoku-shell ask
binding_shell_ask = <alt> KEY_SPACE
command_shell_rashin = ryoku-summon Rashin flock -n -o /tmp/rashin-app.lock rashin-app
binding_shell_rashin = <super> <alt> KEY_SPACE
command_shell_cheatsheet = pkill -x -f 'qs -c keys' 2>/dev/null || env QML_IMPORT_PATH="$HOME/.local/lib/qt6/qml" QML2_IMPORT_PATH="$HOME/.local/lib/qt6/qml" flock -n -o /tmp/ryoku-keys.lock qs -c keys
binding_shell_cheatsheet = <super> KEY_K
command_shell_lock = ryoku-shell lock
binding_shell_lock = <super> KEY_L
command_shell_quicksettings = ryoku-shell quicksettings
binding_shell_quicksettings = <super> KEY_ESC
command_shell_wallpaper = ryogami wallpaper ui
binding_shell_wallpaper = <super> KEY_W
command_shell_wallpaper_random = ryogami wallpaper random
binding_shell_wallpaper_random = <super> <shift> KEY_W
command_shell_ryovm = ryoku-summon ryovm env QML_IMPORT_PATH="$HOME/.local/lib/qt6/qml" QML2_IMPORT_PATH="$HOME/.local/lib/qt6/qml" flock -n -o /tmp/ryovm.lock qs -c ryovm
binding_shell_ryovm = <super> <shift> KEY_V
command_shell_clipboard = ryoku-shell clipboard
binding_shell_clipboard = <super> KEY_V
command_shell_visualizer = ryoku-shell visualizer
binding_shell_visualizer = <super> KEY_M
command_shell_visualizer_overlay = ryoku-shell visualizer-overlay
binding_shell_visualizer_overlay = <super> <shift> KEY_M
command_shell_visualizer_place = ryoku-shell visualizer-place
binding_shell_visualizer_place = <super> <alt> KEY_M
command_shell_voice = ryoku-shell voice
binding_shell_voice = <super> KEY_GRAVE
command_shell_settings = ryoku-shell hub open
binding_shell_settings = <super> KEY_COMMA
command_shell_screenshot = env QML_IMPORT_PATH="$HOME/.local/lib/qt6/qml" QML2_IMPORT_PATH="$HOME/.local/lib/qt6/qml" flock -n -o /tmp/ryoshot.lock qs -c ryoshot
binding_shell_screenshot = <super> <shift> KEY_S
command_shell_screenshot_print = env QML_IMPORT_PATH="$HOME/.local/lib/qt6/qml" QML2_IMPORT_PATH="$HOME/.local/lib/qt6/qml" flock -n -o /tmp/ryoshot.lock qs -c ryoshot
binding_shell_screenshot_print = KEY_SYSRQ
command_shell_screenshot_monitor = env QML_IMPORT_PATH="$HOME/.local/lib/qt6/qml" QML2_IMPORT_PATH="$HOME/.local/lib/qt6/qml" flock -n -o /tmp/ryoshot.lock env RYOSHOT_MODE=monitor qs -c ryoshot
binding_shell_screenshot_monitor = <shift> KEY_SYSRQ
command_shell_color_picker = hyprpicker -a
binding_shell_color_picker = <super> <shift> KEY_C
command_shell_restart_audio = ryoku-restart-audio
binding_shell_restart_audio = <super> <shift> KEY_A
command_media_volume_up = ryoku-volume up
repeatable_binding_media_volume_up = KEY_VOLUMEUP
command_media_volume_down = ryoku-volume down
repeatable_binding_media_volume_down = KEY_VOLUMEDOWN
command_media_mute = wpctl set-mute @DEFAULT_AUDIO_SINK@ toggle
binding_media_mute = KEY_MUTE
command_media_play = playerctl play-pause
binding_media_play = KEY_PLAY
command_media_next = playerctl next
binding_media_next = KEY_NEXTSONG
command_media_prev = playerctl previous
binding_media_prev = KEY_PREVSONG
command_hardware_brightness_up = ryoku-cmd-brightness +5
repeatable_binding_hardware_brightness_up = KEY_BRIGHTNESSUP
command_hardware_brightness_down = ryoku-cmd-brightness -5
repeatable_binding_hardware_brightness_down = KEY_BRIGHTNESSDOWN
command_hardware_touchpad_toggle = ryoku-wm-wayfire act input.touchpad toggle
binding_hardware_touchpad_toggle = KEY_TOUCHPADTOGGLE
command_hardware_touchpad_on = ryoku-wm-wayfire act input.touchpad on
binding_hardware_touchpad_on = KEY_TOUCHPADON
command_hardware_touchpad_off = ryoku-wm-wayfire act input.touchpad off
binding_hardware_touchpad_off = KEY_TOUCHPADOFF

[wm-actions]
toggle_fullscreen = <super> KEY_F
toggle_sticky = <super> <shift> KEY_P
toggle_maximize = <super> KEY_D

[fast-switcher]
activate_forward = <alt> KEY_TAB

[move]
activate = <super> BTN_LEFT

[resize]
activate = <super> BTN_RIGHT
`
	if got != want {
		t.Errorf("composed body differs from the pinned golden:\n--- want ---\n%s\n--- got ---\n%s", want, got)
	}
}

// The shipped plugin list is written with backslash continuations; the composer
// glues those lines into one value, so the generated file carries the same list
// on a single line and the store's toggles can still carve a plugin out of it.
func TestContinuationSeedsGlued(t *testing.T) {
	dir := wayfireHome(t)
	withDefaults(t, "")
	writeSeed(t, dir, "user.ini", "[core]\nplugins = \\\n  animate \\\n  blur \\\n  zoom\n")
	store := writeStore(t, `{"desktop":{"appearance":{"borderFollowsPalette":false,"animations":false,"blurEnabled":true}}}`)
	body := string(mustCompose(t, loadStore(store)))
	if !strings.Contains(body, "plugins = blur zoom") {
		t.Errorf("a continued plugin list must glue into one line, then lose animate:\n%s", body)
	}
}

// Determinism: two runs over the same layers are byte-equal, which is what
// lets doctor diff the file without flapping.
func TestComposeDeterministic(t *testing.T) {
	wayfireHome(t)
	withDefaults(t, "[core]\nplugins = animate blur\n")
	store := writeStore(t, `{"desktop":{"appearance":{"borderSize":4,"borderFollowsPalette":false}}}`)
	s := loadStore(store)
	if a, b := mustCompose(t, s), mustCompose(t, s); !bytes.Equal(a, b) {
		t.Fatalf("two runs differ:\n%s\n---\n%s", a, b)
	}
}

func TestApplyWritesWayfireIni(t *testing.T) {
	dir := wayfireHome(t)
	withDefaults(t, "[core]\nplugins = animate blur\n")
	store := writeStore(t, `{"desktop":{"appearance":{"borderSize":6,"borderFollowsPalette":false,"animations":true,"blurEnabled":true}}}`)

	rep := capApply(t, store)
	if len(rep.Written) != 1 || rep.Written[0] != filepath.Join(dir, "wayfire.ini") {
		t.Fatalf("written = %v, want the live wayfire.ini", rep.Written)
	}
	if rep.ReloadNeeded {
		t.Fatal("wayfire reloads its own config; ReloadNeeded must be false")
	}
	live := readGen(t, dir, "wayfire.ini")
	if !strings.Contains(live, "border_size = 6") {
		t.Errorf("composed file misses the store:\n%s", live)
	}
	overlay := readGen(t, userEditsWayfireDir(), "wayfire.ini")
	if overlay != live {
		t.Error("the user_edits copy must match the live file byte for byte")
	}
}

func TestPreviewWritesNothing(t *testing.T) {
	dir := wayfireHome(t)
	withDefaults(t, "[core]\nplugins = animate\n")
	store := writeStore(t, `{"desktop":{"appearance":{"borderSize":6,"borderFollowsPalette":false}}}`)
	rep := capApply(t, "--preview", store)
	if len(rep.Written) != 0 {
		t.Fatalf("preview wrote %v", rep.Written)
	}
	if _, err := os.Stat(filepath.Join(dir, "wayfire.ini")); err == nil {
		t.Fatal("preview must leave no file behind")
	}
}

// The switch cost: every desktop.* key without a wayfire spelling gets its own
// line with a wayfire-specific reason, a foreign namespace collapses to one,
// and the honoured keys stay quiet.
func TestUnhonoredNamesLosses(t *testing.T) {
	wayfireHome(t)
	store := writeStore(t, `{
	  "desktop": {
	    "appearance": {"gapsIn": 16, "rounding": 8, "blurSize": 12, "borderSize": 4},
	    "input": {"followMouse": 1, "middleClickPaste": false, "swipeDistance": 300, "kbLayout": "us"},
	    "env": [{"key": "FOO", "value": "bar"}],
	    "windows": {"tameMaximizeOnOpen": true},
	    "apps": {"terminal": "alacritty"},
	    "keybindRebinds": {"SUPER + T": "SUPER + Y"},
	    "unbinds": ["SUPER + Q"],
	    "autostart": [{"command": "sleep 1"}]
	  },
	  "wm": {"niri": {"preferNoCsd": true}}
	}`)
	got := map[string]string{}
	for _, u := range unhonored(store) {
		got[u.Key] = u.Reason
	}
	for key, want := range map[string]string{
		"desktop.appearance.gapsIn":   "wayfire windows float; there are no gaps between them.",
		"desktop.appearance.rounding": "wayfire's decorations are square; there is no corner radius.",
		"desktop.input.followMouse":   "wayfire has no focus-follows-mouse mode.",
		"desktop.env":                 "wayfire takes its environment from the session, not from the store.",
		"desktop.windows":             "wayfire lets apps open themselves maximised; there is no tame-on-open.",
		"desktop.apps":                "wayfire takes its environment from the session, not from the store.",
		"wm.niri":                     "These Niri-only settings have no wayfire equivalent. They stay in the store and return if you switch back.",
	} {
		if got[key] != want {
			t.Errorf("%s = %q, want %q", key, got[key], want)
		}
	}
	if _, ok := got["desktop.appearance.borderSize"]; ok {
		t.Error("an emitted leaf must not be reported")
	}
	for _, key := range []string{"desktop.keybindRebinds", "desktop.unbinds"} {
		if _, ok := got[key]; ok {
			t.Errorf("the rebind tables are honoured by resolveBinds; %s must stay quiet", key)
		}
	}
	for key := range got {
		if strings.HasPrefix(key, "desktop.autostart") {
			t.Errorf("autostart translates whole; it must not be reported: %s", key)
		}
	}
	if _, ok := got["desktop.input.swipeInvert"]; ok {
		// swipeDistance is reported; swipeInvert was not set, so it stays quiet.
		t.Error("an unset leaf must not be reported")
	}
	if got["desktop.input.swipeDistance"] == "" {
		t.Error("swipeDistance has no wayfire spelling and must be reported")
	}
}

// The colour gate, both ways: with the palette following, the recorded wallpaper
// colours land in the file; pinned, the store's own colours do and the palette
// file is ignored.
func TestPaletteGateInCompose(t *testing.T) {
	wayfireHome(t)
	withDefaults(t, "")
	pin := writeStore(t, `{"desktop":{"appearance":{"borderSize":4,"activeBorder":"#111111","inactiveBorder":"#222222","borderFollowsPalette":false}}}`)
	follow := writeStore(t, `{"desktop":{"appearance":{"borderSize":4,"activeBorder":"#111111","inactiveBorder":"#222222","borderFollowsPalette":true}}}`)

	pal := filepath.Join(os.Getenv("XDG_STATE_HOME"), "ryoku", "wayfire-border-palette.json")
	if err := os.MkdirAll(filepath.Dir(pal), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pal, []byte(`{"active":"#aabbcc","inactive":"#ddeeff"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	pinned := string(mustCompose(t, loadStore(pin)))
	if !strings.Contains(pinned, "active_color = \\#111111FF") {
		t.Errorf("a pinned border must ignore the palette:\n%s", pinned)
	}
	following := string(mustCompose(t, loadStore(follow)))
	if !strings.Contains(following, "active_color = \\#AABBCCFF") ||
		!strings.Contains(following, "inactive_color = \\#DDEEFFFF") {
		t.Errorf("a palette-following border must take the recorded colours:\n%s", following)
	}
}

// The two plugin-presence toggles carve their plugin out of the composed list
// and nothing else moves; both on leaves the defaults' list untouched.
func TestPluginsAdjustedByStore(t *testing.T) {
	wayfireHome(t)
	withDefaults(t, "[core]\nplugins =   animate   blur   grid  \n")

	off := writeStore(t, `{"desktop":{"appearance":{"animations":false,"blurEnabled":true,"borderFollowsPalette":false}}}`)
	body := string(mustCompose(t, loadStore(off)))
	if strings.Contains(body, "plugins = animate") || !strings.Contains(body, "plugins = blur grid") {
		t.Errorf("animations off must drop animate only:\n%s", body)
	}

	on := writeStore(t, `{"desktop":{"appearance":{"animations":true,"blurEnabled":true,"borderFollowsPalette":false}}}`)
	body = string(mustCompose(t, loadStore(on)))
	if !strings.Contains(body, "plugins = animate   blur   grid") {
		t.Errorf("both toggles on must leave the defaults list's inner spacing alone:\n%s", body)
	}
}

func TestDefaultsCarryWayfireNamespace(t *testing.T) {
	var buf bytes.Buffer
	prev := stdout
	stdout = bufio.NewWriter(&buf)
	defer func() { stdout = prev }()
	if err := runDefaults(); err != nil {
		t.Fatalf("runDefaults: %v", err)
	}
	stdout.Flush()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &top); err != nil {
		t.Fatalf("decode defaults: %v\n%s", err, buf.String())
	}
	var wmNS map[string]json.RawMessage
	if err := json.Unmarshal(top["wm"], &wmNS); err != nil {
		t.Fatalf("decode wm namespace: %v\n%s", err, buf.String())
	}
	own, ok := wmNS[wm.ProviderWayfire]
	if !ok {
		t.Fatalf("defaults miss wm.wayfire: %s", buf.String())
	}
	var ownLeaves map[string]json.RawMessage
	if err := json.Unmarshal(own, &ownLeaves); err != nil {
		t.Fatalf("decode wm.wayfire: %v", err)
	}
	if _, ok := ownLeaves["depthdeck"]; !ok {
		t.Errorf("wm.wayfire must carry depthdeck")
	}
	var desktop map[string]json.RawMessage
	if err := json.Unmarshal(top["desktop"], &desktop); err != nil {
		t.Fatalf("decode desktop: %v", err)
	}
	if _, ok := desktop["appearance"]; !ok {
		t.Errorf("defaults must carry the neutral appearance baseline")
	}
	if _, ok := desktop["env"]; ok {
		t.Error("wayfire does not model env; the defaults must not offer it")
	}
}

// The rule language: an honoured row renders verbatim, a lost one names the
// part that has no spelling.
func TestWindowRuleRendering(t *testing.T) {
	for _, tc := range []struct {
		rule WindowRule
		want string
	}{
		{WindowRule{Class: "kitty", Action: "maximize"},
			`on created if app_id is "kitty" then maximize`},
		{WindowRule{Title: "Player", Action: "pin"},
			`on created if title is "Player" then set sticky true`},
		{WindowRule{Class: "mpv", Action: "opacity", Value: "0.9"},
			`on created if app_id is "mpv" then set alpha 0.9`},
		{WindowRule{Class: "mail", Action: "workspace", Value: "2 0"},
			`on created if app_id is "mail" then assign_workspace 2 0`},
		{WindowRule{Class: "term", Title: "shell", Action: "maximize"},
			`on created if app_id is "term" & title is "shell" then maximize`},
	} {
		got, ok := wayfireRule(tc.rule)
		if !ok || got != tc.want {
			t.Errorf("wayfireRule(%+v) = %q, %v; want %q", tc.rule, got, ok, tc.want)
		}
	}
	lost := []WindowRule{
		{Class: "x", Action: "float"},
		{Class: "x", Action: "opacity", Value: "2"},
		{Class: "x", Action: "workspace", Value: "next"},
		{Class: "x", Action: "workspace", Value: "3"},
		{Action: "maximize"},
	}
	for _, r := range lost {
		if rule, ok := wayfireRule(r); ok {
			t.Errorf("wayfireRule(%+v) must be lost, got %q", r, rule)
		}
	}
	if _, why := renderRule("x", "", "float", ""); !strings.Contains(why, `no "float" action`) {
		t.Errorf("an unknown action must name it, got %q", why)
	}
	if _, why := renderRule("", "", "maximize", ""); !strings.Contains(why, "app id or title") {
		t.Errorf("an unanchored rule must say so, got %q", why)
	}
	if _, why := renderRule("x", "", "workspace", "3"); !strings.Contains(why, "grid coordinates") {
		t.Errorf("a linear workspace index must name the grid format, got %q", why)
	}
}

// The bind path: an exec chord lands as a command/binding pair in wayfire's
// spelling, a release bind takes the release prefix, and a row wayfire cannot
// express never emits. The catalogue rides the same emission, so the composed
// doc carries both.
func TestBindsEmitted(t *testing.T) {
	s := defaultStore()
	s.Keybinds = []Keybind{
		{Keys: "Super+T", Action: "exec", Value: "alacritty"},
		{Keys: "Super+Shift+Y", Action: "exec", Value: "say hi", Release: true},
		{Keys: "Super+Q", Action: "close"},
		{Keys: "mouse_up", Action: "exec", Value: "wheel"},
	}
	d := &iniDoc{}
	applyBinds(d, s)
	got := map[string]string{}
	for _, sec := range d.sections {
		for _, k := range sec.keys {
			got[sec.name+"/"+k.name] = k.value
		}
	}
	if got["command/command_ryoku_0"] != "alacritty" || got["command/binding_ryoku_0"] != "<super> KEY_T" {
		t.Errorf("exec bind: %v", got)
	}
	if got["command/release_binding_ryoku_1"] != "<super> <shift> KEY_Y" {
		t.Errorf("release bind chord: %v", got)
	}
	if _, ok := got["command/command_ryoku_2"]; ok {
		t.Error("a non-exec bind has no wayfire spelling and must not be emitted")
	}
	if _, ok := got["command/binding_ryoku_3"]; ok {
		t.Error("a wheel chord has no activator spelling and must not be emitted")
	}
	// The catalogue's own row lands beside the customs in the same pass.
	if got["command/command_shell_launcher"] != "ryoku-shell launcher" || got["command/binding_shell_launcher"] != "<super> KEY_SPACE" {
		t.Errorf("catalogue spawn row: %v", got)
	}
}

func TestBindsUnhonored(t *testing.T) {
	wayfireHome(t)
	store := writeStore(t, `{"desktop":{"keybinds":[
	  {"keys":"Super+Q","action":"close"},
	  {"keys":"mouse_up","action":"exec","value":"x"},
	  {"keys":"Super+T","action":"exec","value":""},
	  {"keys":"Super+R","action":"exec","value":"rofi"}
	]}}`)
	got := map[string]string{}
	for _, u := range unhonored(store) {
		got[u.Key] = u.Reason
	}
	if !strings.Contains(got["desktop.keybinds[0]"], `cannot bind the "close" action`) {
		t.Errorf("keybinds[0] = %q", got["desktop.keybinds[0]"])
	}
	if !strings.Contains(got["desktop.keybinds[1]"], "cannot bind the chord") {
		t.Errorf("keybinds[1] = %q", got["desktop.keybinds[1]"])
	}
	if !strings.Contains(got["desktop.keybinds[2]"], "needs a command") {
		t.Errorf("keybinds[2] = %q", got["desktop.keybinds[2]"])
	}
	if _, ok := got["desktop.keybinds[3]"]; ok {
		t.Error("a bind wayfire honours must stay quiet")
	}
}

// The input, cursor, swipe, autostart and depthdeck sections arrive with the
// neutral values in wayfire's spelling.
func TestStoreSectionsEmitted(t *testing.T) {
	wayfireHome(t)
	withDefaults(t, "")
	store := writeStore(t, `{
	  "desktop": {
	    "appearance": {"borderFollowsPalette": false, "animations": true, "blurEnabled": true},
	    "input": {"kbLayout": "br", "sensitivity": 0.4, "accelProfile": "flat",
	              "clickfinger": true, "workspaceSwipe": true, "swipeFingers": 4},
	    "cursor": {"theme": "Adwaita", "size": 32},
	    "autostart": [{"command": "nm-applet"}]
	  },
	  "wm": {"wayfire": {"depthdeck": {"maximizedFrontScale": 0.95, "animationMs": 250}}}
	}`)
	body := string(mustCompose(t, loadStore(store)))
	for _, want := range []string{
		"xkb_layout = br",
		"mouse_cursor_speed = 0.4",
		"mouse_accel_profile = flat",
		"click_method = clickfinger",
		"enable_vertical = true",
		"fingers = 4",
		"cursor_theme = Adwaita",
		"cursor_size = 32",
		"ryoku_00 = nm-applet",
		"maximized_front_scale = 0.95",
		"animation_ms = 250",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("composed file misses %q:\n%s", want, body)
		}
	}
}

// The greeter hand-off rides apply, the same way the sibling providers publish
// it, so login and the session agree on the keypad.
func TestApplyPublishesGreeterNumlock(t *testing.T) {
	wayfireHome(t)
	withDefaults(t, "[core]\nplugins = animate\n")
	file := filepath.Join(t.TempDir(), "greeter-numlock")
	t.Setenv("RYOKU_GREETER_NUMLOCK_FILE", file)
	store := writeStore(t, `{"desktop":{"input":{"numlockByDefault":true}}}`)
	capApply(t, store)
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("apply wrote no greeter numlock hand-off: %v", err)
	}
	if got := strings.TrimSpace(string(b)); got != "on" {
		t.Fatalf("hand-off = %q, want on", got)
	}
}

// The shipped defaults are the session's spine: the plugin list, the autostart
// rows, the baseline's own binds. A compose without them still produces bytes,
// and those bytes boot a bare wayfire with no shell and no depthdeck, which
// looks like a render and is worse than none. apply refuses and the live file
// keeps the last good one.
func TestApplyRefusesWhenTheShippedDefaultsAreMissing(t *testing.T) {
	dir := wayfireHome(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "wayfire.ini"), []byte("# last good render\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prev := shareDefaultsPath
	shareDefaultsPath = filepath.Join(t.TempDir(), "absent.ini")
	t.Cleanup(func() { shareDefaultsPath = prev })

	store := writeStore(t, `{}`)
	if _, err := compose(loadStore(store)); err == nil {
		t.Fatal("compose without its shipped defaults must fail")
	}
	if err := runApply([]string{store}); err == nil {
		t.Fatal("apply without its shipped defaults must fail")
	}
	if got := readGen(t, dir, "wayfire.ini"); got != "# last good render\n" {
		t.Errorf("apply overwrote the live file without its first layer: %q", got)
	}
}
