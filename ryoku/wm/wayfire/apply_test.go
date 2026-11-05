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
// in from the dev box. The runtime dir is emptied too: apply reloads a running
// Wayfire, and socket discovery would otherwise find the session's. The config
// base starts empty, so only a test's own withDefaults supplies a baseline.
func wayfireHome(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("WAYFIRE_SOCKET", "")
	t.Setenv("RYOKU_CONFIG_BASE", t.TempDir())
	withWayfirePlugins(t)
	return wayfireConfigDir()
}

func withWayfirePlugins(t *testing.T, names ...string) {
	t.Helper()
	old := wayfirePluginDirs
	dir := t.TempDir()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, "lib"+name+".so"), []byte("fixture"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	wayfirePluginDirs = []string{dir}
	t.Cleanup(func() { wayfirePluginDirs = old })
}

// withDefaults points the shipped-defaults layer at a fixture body through the
// same config-base override a checkout deploy uses.
func withDefaults(t *testing.T, body string) {
	t.Helper()
	base := t.TempDir()
	p := filepath.Join(base, "wayfire", "wayfire.ini")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_CONFIG_BASE", base)
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
	withWayfirePlugins(t, "ryoku-decor")
	baseline := "[core]\nvwidth = 3\nplugins = animate blur grid\n\n[decoration]\nborder_size = 8\n"
	withDefaults(t, baseline)
	writeSeed(t, dir, "keyboard.ini", "[input]\nxkb_layout = us\n")
	writeSeed(t, dir, "user.ini", "[decoration]\nborder_size = 1\n")
	store := writeStore(t, `{"desktop":{"appearance":{"borderSize":4,"activeBorder":"#112233","inactiveBorder":"#445566","borderFollowsPalette":false,"animations":true,"blurEnabled":true},"input":{"kbLayout":"de"}}}`)

	got := string(mustCompose(t, loadStore(store)))
	want := wm.GeneratedConfigBaselineMarker(wm.ProviderWayfire, []byte(baseline)) + "\n" + generatedHeader + `
[core]
vwidth = 5
plugins = animate blur grid ryoku-decor
vheight = 2
preferred_decoration_mode = client
xwayland = true
focus_buttons = BTN_LEFT | BTN_MIDDLE | BTN_RIGHT
focus_button_with_modifiers = false
focus_buttons_passthrough = true

[decoration]
border_size = 0
title_height = 0
font = sans-serif
font_color = \#FFFFFFFF
font_scale = 0.8
button_order = minimize maximize close
button_scale = 0.7
button_padding = 4
ignore_views = none
forced_views = none
active_color = \#112233FF
inactive_color = \#445566FF

[simple-tile]
tile_by_default = ` + tileByDefaultMatcher() + `
keep_fullscreen_on_adjacent = true
inner_gap_size = 12
outer_horiz_gap_size = 18
outer_vert_gap_size = 18
key_toggle_maximized = <super> KEY_D

[vswitch]
gap = 20
wraparound = false
duration = 250ms circle
binding_1 = <super> KEY_1 | <super> KEY_KP1
binding_2 = <super> KEY_2 | <super> KEY_KP2
binding_3 = <super> KEY_3 | <super> KEY_KP3
binding_4 = <super> KEY_4 | <super> KEY_KP4
binding_5 = <super> KEY_5 | <super> KEY_KP5
binding_6 = <super> KEY_6 | <super> KEY_KP6
binding_7 = <super> KEY_7 | <super> KEY_KP7
binding_8 = <super> KEY_8 | <super> KEY_KP8
binding_9 = <super> KEY_9 | <super> KEY_KP9
binding_10 = <super> KEY_0 | <super> KEY_KP0
with_win_1 = <super> <alt> KEY_1 | <super> <alt> KEY_KP1
with_win_2 = <super> <alt> KEY_2 | <super> <alt> KEY_KP2
with_win_3 = <super> <alt> KEY_3 | <super> <alt> KEY_KP3
with_win_4 = <super> <alt> KEY_4 | <super> <alt> KEY_KP4
with_win_5 = <super> <alt> KEY_5 | <super> <alt> KEY_KP5
with_win_6 = <super> <alt> KEY_6 | <super> <alt> KEY_KP6
with_win_7 = <super> <alt> KEY_7 | <super> <alt> KEY_KP7
with_win_8 = <super> <alt> KEY_8 | <super> <alt> KEY_KP8
with_win_9 = <super> <alt> KEY_9 | <super> <alt> KEY_KP9
with_win_10 = <super> <alt> KEY_0 | <super> <alt> KEY_KP0
send_win_1 = <super> <shift> KEY_1 | <super> <shift> KEY_KP1
send_win_2 = <super> <shift> KEY_2 | <super> <shift> KEY_KP2
send_win_3 = <super> <shift> KEY_3 | <super> <shift> KEY_KP3
send_win_4 = <super> <shift> KEY_4 | <super> <shift> KEY_KP4
send_win_5 = <super> <shift> KEY_5 | <super> <shift> KEY_KP5
send_win_6 = <super> <shift> KEY_6 | <super> <shift> KEY_KP6
send_win_7 = <super> <shift> KEY_7 | <super> <shift> KEY_KP7
send_win_8 = <super> <shift> KEY_8 | <super> <shift> KEY_KP8
send_win_9 = <super> <shift> KEY_9 | <super> <shift> KEY_KP9
send_win_10 = <super> <shift> KEY_0 | <super> <shift> KEY_KP0

[place]
mode = center

[ryoku-decor]
radius = 0
corner_power = 4
border_width = 4
active_color = \#112233FF
inactive_color = \#445566FF
shadow_enabled = true
shadow_radius = 45
shadow_spread = 0
shadow_offset_x = 0
shadow_offset_y = 5
shadow_color = \#00000070
ignore_views = none

[blur]
blur_by_default = type is "toplevel"
method = kawase
saturation = 1
alpha_threshold = 0.5
alpha_exponent = 1
box_offset = 2
box_degrade = 3
box_iterations = 2
gaussian_offset = 2
gaussian_degrade = 3
gaussian_iterations = 2
kawase_offset = 2
kawase_degrade = 3
kawase_iterations = 2
bokeh_offset = 2
bokeh_degrade = 3
bokeh_iterations = 2

[wobbly]
friction = 3
spring_k = 8
grid_resolution = 6

[zoom]
modifier = <super>
speed = 0.01
smoothing_duration = 300ms linear
interpolation_method = 0

[fast-switcher]
inactive_alpha = 0.7
activate = <alt> KEY_TAB

[input]
xkb_layout = us
xkb_variant = 
xkb_options = 
kb_numlock_default_state = false
mouse_cursor_speed = 0
mouse_accel_profile = default
touchpad_cursor_speed = 0
touchpad_accel_profile = default
left_handed_mode = false
mouse_natural_scroll = false
mouse_scroll_speed = 1
natural_scroll = true
touchpad_scroll_speed = 1
tap_to_click = true
tap_and_drag = true
click_method = button-areas
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

[window-rules]
ryoku_builtin_nautilus = on created if app_id is "org.gnome.Nautilus" then set geometry_ppt 4 6 92 88
ryoku_builtin_hub = on created if title is "Ryoku Settings" then maximize
ryoku_builtin_rashin = on created if title is "Rashin" then set geometry_ppt 4 6 92 88
ryoku_builtin_phone_tools = on created if title contains "ryoku-phone-" then set geometry_ppt 31 6 38 88
ryoku_builtin_update = on created if app_id is "dev.ryoku.update" then set geometry_ppt 4 6 92 88
ryoku_builtin_ryostore_title = on created if title is "Ryostore" then set geometry_ppt 4 6 92 88
ryoku_builtin_ryovm = on created if title is "ryovm" then set geometry_ppt 4 6 92 88
ryoku_builtin_ryoport_ssh = on created if app_id is "dev.ryoku.ryoport_ssh" then set geometry_ppt 20 24 60 52
ryoku_builtin_ryostore_app = on created if app_id is "ryostore" then set geometry_ppt 15 17 70 66
ryoku_builtin_rashin_setup = on created if app_id is "ryoku-rashin-setup" then set geometry_ppt 15 17 70 66
ryoku_builtin_looking_glass = on created if app_id is "looking-glass-client" then set geometry_ppt 4 6 92 88
ryoku_builtin_qemu_lower = on created if app_id is "qemu" then set geometry_ppt 4 6 92 88
ryoku_builtin_qemu_upper = on created if app_id is "Qemu" then set geometry_ppt 4 6 92 88
ryoku_builtin_welcome = on created if title is "Welcome to Ryoku" then set geometry_ppt 4 6 92 88
ryoku_builtin_chromium_sharebar = on created if app_id is "" & title contains "is sharing" then assign_workspace 4 1

[animate]
duration = 250ms circle
open_animation = zoom
close_animation = zoom
minimize_animation = squeezimize

[grid]
duration = 250ms circle

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
command_window_preset_width = ryoku-wm-wayfire act window.presetWidth
binding_window_preset_width = <super> KEY_R
command_window_preset_height = ryoku-wm-wayfire act window.presetHeight
binding_window_preset_height = <super> <shift> KEY_R
command_column_maximize = ryoku-wm-wayfire act window.maximize
binding_column_maximize = <super> KEY_D
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
command_workspace_hide_window = ryoku-wm-wayfire act window.hide focused
binding_workspace_hide_window = <super> KEY_H
command_workspace_scratchpad = ryoku-wm-wayfire act workspace.toggleSpecial scratchpad
binding_workspace_scratchpad = <super> <alt> KEY_H
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
command_shell_cheatsheet = ryoku-shell cheatsheet
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
command_shell_settings = ryoku-shell settings
binding_shell_settings = <super> KEY_COMMA
command_shell_stage_editor = ryoku-shell hub open widgets
binding_shell_stage_editor = <super> <shift> KEY_D
command_shell_screenshot = env QML_IMPORT_PATH="$HOME/.local/lib/qt6/qml" QML2_IMPORT_PATH="$HOME/.local/lib/qt6/qml" ryoku-cmd-ryoshot
binding_shell_screenshot = <super> <shift> KEY_S
command_shell_screenshot_print = env QML_IMPORT_PATH="$HOME/.local/lib/qt6/qml" QML2_IMPORT_PATH="$HOME/.local/lib/qt6/qml" ryoku-cmd-ryoshot
binding_shell_screenshot_print = KEY_PRINT
command_shell_screenshot_monitor = env QML_IMPORT_PATH="$HOME/.local/lib/qt6/qml" QML2_IMPORT_PATH="$HOME/.local/lib/qt6/qml" RYOSHOT_MODE=monitor ryoku-cmd-ryoshot
binding_shell_screenshot_monitor = <shift> KEY_PRINT
command_shell_color_picker = ryoku-cmd-color-picker
binding_shell_color_picker = <super> <shift> KEY_C
command_shell_restart_audio = ryoku-restart-audio
binding_shell_restart_audio = <super> <shift> KEY_A
command_media_volume_up = ryoku-volume up
repeatable_binding_media_volume_up = KEY_VOLUMEUP
command_media_volume_down = ryoku-volume down
repeatable_binding_media_volume_down = KEY_VOLUMEDOWN
command_media_mute = wpctl set-mute @DEFAULT_AUDIO_SINK@ toggle
always_binding_media_mute = KEY_MUTE
command_media_mic_mute = wpctl set-mute @DEFAULT_AUDIO_SOURCE@ toggle
always_binding_media_mic_mute = KEY_MICMUTE
command_media_play = playerctl play-pause
always_binding_media_play = KEY_PLAYPAUSE
command_media_next = playerctl next
always_binding_media_next = KEY_NEXTSONG
command_media_prev = playerctl previous
always_binding_media_prev = KEY_PREVIOUSSONG
command_hardware_brightness_up = ryoku-cmd-brightness +5
repeatable_binding_hardware_brightness_up = KEY_BRIGHTNESSUP
command_hardware_brightness_down = ryoku-cmd-brightness -5
repeatable_binding_hardware_brightness_down = KEY_BRIGHTNESSDOWN
command_hardware_touchpad_toggle = ryoku-wm-wayfire act input.touchpad toggle
always_binding_hardware_touchpad_toggle = KEY_TOUCHPAD_TOGGLE
command_hardware_touchpad_on = ryoku-wm-wayfire act input.touchpad on
always_binding_hardware_touchpad_on = KEY_TOUCHPAD_ON
command_hardware_touchpad_off = ryoku-wm-wayfire act input.touchpad off
always_binding_hardware_touchpad_off = KEY_TOUCHPAD_OFF

[wm-actions]
toggle_fullscreen = <super> KEY_F
toggle_sticky = <super> <shift> KEY_P

[shortcuts-inhibit]
break_grab = <super> <shift> KEY_ESC

[move]
activate = <super> BTN_LEFT

[resize]
activate = <super> BTN_RIGHT
`
	if got != want {
		t.Errorf("composed body differs from the pinned golden:\n--- want ---\n%s\n--- got ---\n%s", want, got)
	}
}

func TestDecorPluginPresent(t *testing.T) {
	wayfireHome(t)
	withWayfirePlugins(t, "ryoku-decor")
	withDefaults(t, "[core]\nplugins = decoration autostart\n\n[decoration]\nborder_size = 9\n")
	store := writeStore(t, `{
	  "desktop":{"appearance":{
	    "rounding":18,"roundingPower":6,"borderSize":7,
	    "activeBorder":"#102030","inactiveBorder":"#405060","borderFollowsPalette":false,
	    "shadowEnabled":true,"shadowRange":31,"shadowColor":"#102030",
	    "shadowOffsetX":-4,"shadowOffsetY":9,"shadowSpread":3
	  }},
	  "wm":{"wayfire":{"decoration":{"ignoreViews":"app_id is \"no-frame\""}}}
	}`)

	body := string(mustCompose(t, loadStore(store)))
	for _, want := range []string{
		"plugins = decoration ryoku-decor autostart",
		"[decoration]\nborder_size = 0",
		"[ryoku-decor]\nradius = 18",
		"corner_power = 6",
		"border_width = 7",
		"active_color = \\#102030FF",
		"inactive_color = \\#405060FF",
		"shadow_enabled = true",
		"shadow_radius = 31",
		"shadow_spread = 3",
		"shadow_offset_x = -4",
		"shadow_offset_y = 9",
		"shadow_color = \\#10203070",
		`ignore_views = app_id is "no-frame"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("composed decor config misses %q:\n%s", want, body)
		}
	}
	for _, u := range unhonored(store) {
		switch strings.TrimPrefix(u.Key, "desktop.appearance.") {
		case "rounding", "roundingPower", "shadowEnabled", "shadowRange",
			"shadowColor", "shadowOffsetX", "shadowOffsetY", "shadowSpread":
			t.Errorf("installed decor plugin must honor %s: %s", u.Key, u.Reason)
		}
	}
}

func TestDecorPluginAbsent(t *testing.T) {
	wayfireHome(t)
	withDefaults(t, "[core]\nplugins = decoration ryoku-decor autostart\n\n[decoration]\nborder_size = 9\n\n[ryoku-decor]\nradius = 50\n")
	store := writeStore(t, `{"desktop":{"appearance":{
	  "rounding":18,"roundingPower":6,"borderSize":7,
	  "shadowEnabled":true,"shadowRange":31,"shadowColor":"#102030",
	  "shadowOffsetX":-4,"shadowOffsetY":9,"shadowSpread":3
	}}}`)

	body := string(mustCompose(t, loadStore(store)))
	if !strings.Contains(body, "plugins = decoration autostart") {
		t.Errorf("missing plugin was not removed from core.plugins:\n%s", body)
	}
	if strings.Contains(body, "[ryoku-decor]") {
		t.Errorf("missing plugin left its option section in the composed file:\n%s", body)
	}
	if !strings.Contains(body, "[decoration]\nborder_size = 7") {
		t.Errorf("stock decoration border was not retained:\n%s", body)
	}

	const reason = "The Ryoku window decor plugin is missing; it ships with ryoku-desktop-wayfire."
	got := map[string]string{}
	for _, u := range unhonored(store) {
		got[u.Key] = u.Reason
	}
	for _, key := range []string{
		"rounding", "roundingPower", "shadowEnabled", "shadowRange",
		"shadowColor", "shadowOffsetX", "shadowOffsetY", "shadowSpread",
	} {
		full := "desktop.appearance." + key
		if got[full] != reason {
			t.Errorf("%s reason = %q, want %q", full, got[full], reason)
		}
	}
}

func TestComposeReadsBaselineFromConfigBaseAndMarksIt(t *testing.T) {
	wayfireHome(t)
	baseline := []byte("[core]\nplugins = animate\n\n[ryoku-probe]\nsource = config-base\n")
	withDefaults(t, string(baseline))

	body := mustCompose(t, defaultStore())
	mark := wm.GeneratedConfigBaselineMarker(wm.ProviderWayfire, baseline)
	if !bytes.HasPrefix(body, []byte(mark+"\n")) {
		t.Fatalf("composed file starts with %q, want baseline mark %q", body[:min(len(body), len(mark)+1)], mark)
	}
	if !bytes.Contains(body, []byte("[ryoku-probe]\nsource = config-base")) {
		t.Fatalf("compose did not preserve the baseline probe under RYOKU_CONFIG_BASE:\n%s", body)
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
	    "appearance": {
	      "gapsIn": 16, "gapsOut": 20, "activeOpacity": 0.9,
	      "inactiveOpacity": 0.8, "rounding": 8, "blurSize": 12,
	      "shadowEnabled": true, "shadowColor": "#000000",
	      "shadowOffsetX": 1, "shadowOffsetY": 5, "shadowRange": 45,
	      "shadowSpread": 2, "borderSize": 4
	    },
	    "input": {"followMouse": 1, "middleClickPaste": false, "swipeDistance": 300, "kbLayout": "us"},
	    "cursor": {"theme": "Adwaita", "size": 32, "hideOnKeyPress": true, "inactiveTimeout": 5},
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
	for key, contains := range map[string]string{
		"desktop.appearance.inactiveOpacity": "cannot change it when focus moves",
		"desktop.appearance.rounding":        "Ryoku window decor plugin is missing",
		"desktop.appearance.shadowColor":     "Ryoku window decor plugin is missing",
		"desktop.appearance.shadowEnabled":   "Ryoku window decor plugin is missing",
		"desktop.appearance.shadowOffsetX":   "Ryoku window decor plugin is missing",
		"desktop.appearance.shadowOffsetY":   "Ryoku window decor plugin is missing",
		"desktop.appearance.shadowRange":     "Ryoku window decor plugin is missing",
		"desktop.appearance.shadowSpread":    "Ryoku window decor plugin is missing",
		"desktop.input.followMouse":          "no focus-follows-pointer mode",
		"desktop.input.middleClickPaste":     "no primary-selection switch",
		"desktop.cursor.hideOnKeyPress":      "no inactivity timeout or keypress-hide option",
		"desktop.cursor.inactiveTimeout":     "no inactivity timeout or keypress-hide option",
		"desktop.windows":                    "no tame-on-open",
		"wm.niri":                            "These Niri-only settings have no wayfire equivalent",
	} {
		if !strings.Contains(got[key], contains) {
			t.Errorf("%s = %q, want it to contain %q", key, got[key], contains)
		}
	}
	for _, key := range []string{
		"desktop.appearance.gapsIn", "desktop.appearance.gapsOut",
		"desktop.appearance.activeOpacity", "desktop.appearance.borderSize",
		"desktop.appearance.blurSize",
		"desktop.input.kbLayout", "desktop.cursor.theme", "desktop.cursor.size",
		"desktop.env", "desktop.apps",
		"desktop.keybindRebinds", "desktop.unbinds",
	} {
		if _, ok := got[key]; ok {
			t.Errorf("an emitted leaf must not be reported: %s", key)
		}
	}
	for key := range got {
		if strings.HasPrefix(key, "desktop.autostart") {
			t.Errorf("autostart translates whole; it must not be reported: %s", key)
		}
	}
	if _, ok := got["desktop.input.swipeInvert"]; ok {
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

// The three plugin-presence toggles carve their plugin and inactive options
// section out of the composed file; all on leaves the defaults' list untouched.
func TestPluginsAdjustedByStore(t *testing.T) {
	wayfireHome(t)
	withDefaults(t, "[core]\nplugins =   animate   blur   grid   wobbly  \n")

	off := writeStore(t, `{"desktop":{"appearance":{"animations":false,"blurEnabled":true,"wobblyWindows":true,"borderFollowsPalette":false}}}`)
	body := string(mustCompose(t, loadStore(off)))
	if strings.Contains(body, "plugins = animate") || !strings.Contains(body, "plugins = blur grid wobbly") ||
		strings.Contains(body, "[animate]\n") {
		t.Errorf("animations off must drop animate and its options:\n%s", body)
	}

	blurOff := writeStore(t, `{"desktop":{"appearance":{"animations":true,"blurEnabled":false,"wobblyWindows":true,"borderFollowsPalette":false}}}`)
	body = string(mustCompose(t, loadStore(blurOff)))
	if !strings.Contains(body, "plugins = animate grid wobbly") || strings.Contains(body, "[blur]\n") {
		t.Errorf("blur off must drop blur and its options:\n%s", body)
	}

	wobbleOff := writeStore(t, `{"desktop":{"appearance":{"animations":true,"blurEnabled":true,"wobblyWindows":false,"borderFollowsPalette":false}}}`)
	body = string(mustCompose(t, loadStore(wobbleOff)))
	if strings.Contains(body, "plugins = animate blur grid wobbly") ||
		!strings.Contains(body, "plugins = animate blur grid") || strings.Contains(body, "[wobbly]\n") {
		t.Errorf("wobbly windows off must drop wobbly and its options:\n%s", body)
	}

	on := writeStore(t, `{"desktop":{"appearance":{"animations":true,"blurEnabled":true,"wobblyWindows":true,"borderFollowsPalette":false}}}`)
	body = string(mustCompose(t, loadStore(on)))
	if !strings.Contains(body, "plugins = animate   blur   grid   wobbly") {
		t.Errorf("all toggles on must leave the defaults list's inner spacing alone:\n%s", body)
	}
}

func TestPerformanceDefaultsDisableBlurAndUseFastMotion(t *testing.T) {
	s := defaultStore()
	if s.Appearance.BlurEnabled {
		t.Fatal("Wayfire blur must be opt-in")
	}
	for name, anim := range map[string]AnimKind{
		"workspace": s.Wayfire.Animation.WorkspaceSwitch,
		"window":    s.Wayfire.Animation.Window,
		"snap":      s.Wayfire.Animation.Snap,
	} {
		if anim.DurationMs != 250 {
			t.Errorf("%s default duration = %dms, want 250ms", name, anim.DurationMs)
		}
	}
	d := parseIni([]byte("[core]\nplugins = animate blur grid simple-tile\n"))
	adjustPlugins(&d, s)
	plugins, _ := d.get("core", "plugins")
	if strings.Contains(plugins, "blur") || !strings.Contains(plugins, "animate") {
		t.Errorf("default plugins = %q, want animate without blur", plugins)
	}
	rendered := renderStoreIni(s)
	for section, want := range map[string]string{
		"vswitch": "250ms circle",
		"animate": "250ms circle",
		"grid":    "250ms circle",
	} {
		if got, _ := rendered.get(section, "duration"); got != want {
			t.Errorf("%s duration = %q, want %q", section, got, want)
		}
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
	for _, key := range []string{
		"animation", "tiling", "workspace", "decoration", "blur", "wobbly",
		"zoom", "fastSwitcher", "focus", "xwayland", "placement", "depthdeck",
	} {
		if _, ok := ownLeaves[key]; !ok {
			t.Errorf("wm.wayfire must carry %s", key)
		}
	}
	var desktop map[string]json.RawMessage
	if err := json.Unmarshal(top["desktop"], &desktop); err != nil {
		t.Fatalf("decode desktop: %v", err)
	}
	for _, key := range []string{"appearance", "input", "cursor", "env", "apps"} {
		if _, ok := desktop[key]; !ok {
			t.Errorf("defaults must carry neutral desktop.%s", key)
		}
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
			`on created if title is "Player" then sticky`},
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
		{Class: "x", Action: "workspace", Value: "5 0"},
		{Class: "x", Action: "workspace", Value: "0 2"},
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
	    "appearance": {
	      "gapsIn": 14, "gapsOut": 21, "activeOpacity": 0.86,
	      "borderFollowsPalette": false, "animations": true, "blurEnabled": true
	    },
	    "input": {
	      "kbLayout": "br", "kbVariant": "nodeadkeys", "kbOptions": "ctrl:nocaps",
	      "repeatRate": 31, "repeatDelay": 275,
	      "sensitivity": 0.4, "accelProfile": "flat", "leftHanded": true,
	      "mouseNaturalScroll": true, "mouseScrollFactor": 1.5,
	      "naturalScroll": false, "touchScrollFactor": 1.25,
	      "tapToClick": false, "tapAndDrag": false, "clickfinger": true,
	      "middleEmulation": true, "disableWhileTyping": false,
	      "workspaceSwipe": true, "swipeFingers": 4
	    },
	    "cursor": {"theme": "Adwaita", "size": 32},
	    "env": [
	      {"key": "QT_SCALE_FACTOR", "value": "1.25"},
	      {"key": "QUOTE", "value": "a'b"},
	      {"key": "BAD-NAME", "value": "ignored"}
	    ],
	    "apps": {"browser": "zen-browser", "terminal": "kitty"},
	    "autostart": [{"command": "nm-applet"}]
	  },
	  "wm": {"wayfire": {
	    "animation": {"openStyle": "fire", "closeStyle": "spin", "minimizeStyle": "zoom"},
	    "depthdeck": {"maximizedFrontScale": 0.95, "animationMs": 250}
	  }}
	}`)
	body := string(mustCompose(t, loadStore(store)))
	for _, want := range []string{
		"vwidth = 5",
		"vheight = 2",
		"tile_by_default = " + tileByDefaultMatcher(),
		"inner_gap_size = 14",
		"outer_horiz_gap_size = 21",
		"outer_vert_gap_size = 21",
		"xkb_layout = br",
		"xkb_variant = nodeadkeys",
		"xkb_options = ctrl:nocaps",
		"kb_repeat_rate = 31",
		"kb_repeat_delay = 275",
		"mouse_cursor_speed = 0.4",
		"mouse_accel_profile = flat",
		"touchpad_cursor_speed = 0.4",
		"touchpad_accel_profile = flat",
		"left_handed_mode = true",
		"mouse_natural_scroll = true",
		"mouse_scroll_speed = 1.5",
		"natural_scroll = false",
		"touchpad_scroll_speed = 1.25",
		"tap_to_click = false",
		"tap_and_drag = false",
		"middle_emulation = true",
		"disable_touchpad_while_typing = false",
		"click_method = clickfinger",
		"enable_vertical = true",
		"fingers = 4",
		"cursor_theme = Adwaita",
		"cursor_size = 32",
		`ryoku_00 = env 'QT_SCALE_FACTOR=1.25' 'QUOTE=a'"'"'b' 'BROWSER=zen-browser' 'TERMINAL=kitty' nm-applet`,
		`command_app_terminal = env 'QT_SCALE_FACTOR=1.25' 'QUOTE=a'"'"'b' 'BROWSER=zen-browser' 'TERMINAL=kitty' ryoku-app terminal`,
		`ryoku_active_opacity = on created if type is "toplevel" then set alpha 0.86`,
		"open_animation = fire",
		"close_animation = spin",
		"minimize_animation = zoom",
		"maximized_front_scale = 0.95",
		"animation_ms = 250",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("composed file misses %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "BAD-NAME") {
		t.Errorf("an invalid environment name reached the generated commands:\n%s", body)
	}
}

// These are the Wayfire 0.11 metadata options behind the provider-only Hub
// rows. Each stored value must land on its exact section/key, while the neutral
// workspace-gap and blur controls feed the matching Wayfire tuning.
func TestWindowManagerSettingsEmitted(t *testing.T) {
	s := defaultStore()
	s.Appearance.GapsWorkspaces = 37
	s.Appearance.BlurSize = 6
	s.Appearance.BlurPasses = 5
	s.Wayfire.Tiling = WayfireTiling{TileByDefault: "none", KeepFullscreenOnAdjacent: false}
	s.Wayfire.Workspace = WayfireWorkspace{Columns: 4, Rows: 3, Wraparound: "on_end"}
	s.Wayfire.Placement.Mode = "pointer"
	s.Wayfire.Decoration = WayfireDecoration{
		Mode: "server", TitleHeight: 22, Font: "Cantarell",
		FontColor: "#123456", FontScale: 0.6, ButtonOrder: "close minimize",
		ButtonScale: 0.5, ButtonPadding: 8,
		IgnoreViews: `app_id is "plain"`, ForcedViews: `app_id is "framed"`,
	}
	s.Wayfire.Blur = WayfireBlur{
		Method: "gaussian", Saturation: 1.4, BlurByDefault: `app_id is "glass"`,
		Degrade: 4, AlphaThreshold: 0.3, AlphaExponent: 2,
	}
	s.Wayfire.Wobbly = WayfireWobbly{Friction: 2.5, SpringK: 9, GridResolution: 8}
	s.Wayfire.Zoom = WayfireZoom{
		Modifier: "<alt>", Speed: 0.02, SmoothingMs: 450, InterpolationMethod: "1",
	}
	s.Wayfire.FastSwitcher.InactiveOpacity = 0.55
	s.Wayfire.Focus = WayfireFocus{Buttons: "BTN_LEFT", WithModifiers: true, PassThrough: false}
	s.Wayfire.XWayland.Mode = "lazy"

	d := renderStoreIni(s)
	wants := []struct {
		section, key, value string
	}{
		{"core", "vwidth", "4"},
		{"core", "vheight", "3"},
		{"core", "preferred_decoration_mode", "server"},
		{"core", "xwayland", "lazy"},
		{"core", "focus_buttons", "BTN_LEFT"},
		{"core", "focus_button_with_modifiers", "true"},
		{"core", "focus_buttons_passthrough", "false"},
		{"simple-tile", "tile_by_default", "none"},
		{"simple-tile", "keep_fullscreen_on_adjacent", "false"},
		{"vswitch", "gap", "37"},
		{"vswitch", "wraparound", "on_end"},
		{"place", "mode", "pointer"},
		{"decoration", "title_height", "22"},
		{"decoration", "font", "Cantarell"},
		{"decoration", "font_color", "#123456FF"},
		{"decoration", "font_scale", "0.6"},
		{"decoration", "button_order", "close minimize"},
		{"decoration", "button_scale", "0.5"},
		{"decoration", "button_padding", "8"},
		{"decoration", "ignore_views", `app_id is "plain"`},
		{"decoration", "forced_views", `app_id is "framed"`},
		{"blur", "blur_by_default", `app_id is "glass"`},
		{"blur", "method", "gaussian"},
		{"blur", "saturation", "1.4"},
		{"blur", "alpha_threshold", "0.3"},
		{"blur", "alpha_exponent", "2"},
		{"blur", "box_offset", "6"},
		{"blur", "box_degrade", "4"},
		{"blur", "box_iterations", "5"},
		{"blur", "gaussian_offset", "6"},
		{"blur", "gaussian_degrade", "4"},
		{"blur", "gaussian_iterations", "5"},
		{"blur", "kawase_offset", "6"},
		{"blur", "kawase_degrade", "4"},
		{"blur", "kawase_iterations", "5"},
		{"blur", "bokeh_offset", "6"},
		{"blur", "bokeh_degrade", "4"},
		{"blur", "bokeh_iterations", "5"},
		{"wobbly", "friction", "2.5"},
		{"wobbly", "spring_k", "9"},
		{"wobbly", "grid_resolution", "8"},
		{"zoom", "modifier", "<alt>"},
		{"zoom", "speed", "0.02"},
		{"zoom", "smoothing_duration", "450ms linear"},
		{"zoom", "interpolation_method", "1"},
		{"fast-switcher", "inactive_alpha", "0.55"},
	}
	for _, want := range wants {
		if got, ok := d.get(want.section, want.key); !ok || got != want.value {
			t.Errorf("[%s] %s = %q, want %q", want.section, want.key, got, want.value)
		}
	}
}

func TestWindowAnimStyleRejectsUnknownValues(t *testing.T) {
	for _, tc := range []struct {
		style    string
		minimize bool
		want     string
	}{
		{style: "fire", want: "fire"},
		{style: "squeezimize", minimize: true, want: "squeezimize"},
		{style: "bogus", want: "zoom"},
		{style: "fire", minimize: true, want: "squeezimize"},
	} {
		if got := windowAnimStyle(tc.style, tc.minimize); got != tc.want {
			t.Errorf("windowAnimStyle(%q, %t) = %q, want %q", tc.style, tc.minimize, got, tc.want)
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
	t.Setenv("RYOKU_CONFIG_BASE", t.TempDir())

	store := writeStore(t, `{}`)
	if _, err := compose(loadStore(store)); err == nil {
		t.Fatal("compose without its shipped defaults must fail")
	} else if !strings.Contains(err.Error(), filepath.Join(os.Getenv("RYOKU_CONFIG_BASE"), "wayfire", "wayfire.ini")) {
		t.Fatalf("missing-baseline error does not name the resolved path: %v", err)
	}
	if err := runApply([]string{store}); err == nil {
		t.Fatal("apply without its shipped defaults must fail")
	}
	if got := readGen(t, dir, "wayfire.ini"); got != "# last good render\n" {
		t.Errorf("apply overwrote the live file without its first layer: %q", got)
	}
}

func TestApplyWritesCustomSettingsWindowRule(t *testing.T) {
	dir := wayfireHome(t)
	withDefaults(t, "[core]\nplugins = window-rules place simple-tile\n")
	store := writeStore(t, `{"wm":{"wayfire":{"settingsWindow":{"mode":"custom","width":0.5,"height":0.5}}}}`)
	rep := capApply(t, store)
	if len(rep.Written) != 1 {
		t.Fatalf("written = %v, want wayfire.ini", rep.Written)
	}
	config := readGen(t, dir, "wayfire.ini")
	want := `ryoku_builtin_hub = on created if title is "Ryoku Settings" then set geometry_ppt 25 25 50 50`
	if !strings.Contains(config, want) {
		t.Fatalf("custom settings window rule missing %q:\n%s", want, config)
	}
}
