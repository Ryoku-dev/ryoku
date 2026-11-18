package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	wm "ryoku-wm"
)

// Hyprdrive is the compositor plugin that brings the mission-control overview,
// Mosaic and window motion to Ryoku. It is off until the user turns it on, and it
// never touches a shipped Lua module or settings.lua: everything it needs lives
// in one fenced block at the end of ~/.config/hypr/user.lua, which this file
// writes on every apply and removes when the plugin is off. Each bind it adds is
// listed with the Ryoku bind it would take over; a bind that takes one over is
// off until the user accepts it.

const (
	hyprdriveBegin = "-- ryoku-hyprdrive:begin"
	hyprdriveEnd   = "-- ryoku-hyprdrive:end"
)

// hdBind is one bind Hyprdrive offers. A Chords entry is a Hyprland chord the
// block binds to the Lua action at the same index; a bind with no chords (the
// Super tap, a gesture) is a plugin feature that no keybind collides with.
type hdBind struct {
	ID     string
	Label  string
	Detail string
	Chords []string
	Lua    []string
	// Config is the plugin key the switch writes instead of a bind.
	Config string
	// Off ships the shortcut off: it is there to turn on, and one that would
	// replace a Ryoku shortcut still waits for that either way.
	Off bool
}

var hyprdriveBinds = []hdBind{
	{
		ID: "tap-super", Off: true, Label: "Tap Super for the overview",
		Detail: "Press and release Super alone to show every window; Super with any other key stays a shortcut",
		Config: "hyprdrive:tap_super",
	},
	{
		ID: "overview-tab", Label: "Hyprdrive's overview on Super+Tab",
		Detail: "Shows Hyprdrive's overview instead of Ryoku's workspace overview",
		Chords: []string{"SUPER + Tab"},
		Lua:    []string{`hl.plugin.hyprdrive.overview("toggle")`},
	},
	{
		ID: "show-desktop", Off: true, Label: "Show desktop",
		Detail: "Slides every window off the screen, and back on the second press",
		Chords: []string{"SUPER + D"},
		Lua:    []string{`hl.plugin.hyprdrive.show_desktop()`},
	},
	{
		ID: "scroll-desktops", Off: true, Label: "Super and the wheel slide the desktops",
		Detail: "The row of desktops follows the wheel, empty desktops included",
		Chords: []string{"SUPER + mouse_down", "SUPER + mouse_up"},
		Lua:    []string{`hl.plugin.hyprdrive.desktop("next")`, `hl.plugin.hyprdrive.desktop("prev")`},
	},
	{
		ID: "layout-cycle", Label: "Cycle the desktop's layout",
		Detail: "Dwindle, then Scrolling, then Mosaic; each desktop keeps its layout",
		Chords: []string{"SUPER + SHIFT + L"},
		Lua:    []string{`function() hl.exec_cmd("ryoku-wm-hyprland hyprdrive cycle-layout") end`},
	},
	{
		ID: "swipe-overview", Off: true, Label: "Swipe up with three fingers for the overview",
		Detail: "The overview follows the fingers; swiping down closes it again",
	},
}

// Hyprdrive is the store's record of the plugin: whether it is on, the user's
// answer for each bind (absent: on when it takes nothing over), a bind moved to
// another chord, and the plugin keys the settings page set, by colon path
// ("hyprdrive:mosaic:enabled").
type Hyprdrive struct {
	Enabled bool              `json:"enabled"`
	Binds   map[string]bool   `json:"binds,omitempty"`
	Chords  map[string]string `json:"chords,omitempty"`
	Config  map[string]any    `json:"config,omitempty"`
}

// hyprdriveDefaults is the plugin's own default for every key its settings page
// shows, so the page reads a value before the user has set one.
func hyprdriveDefaults() map[string]any {
	return map[string]any{
		"hyprdrive:scale":                               0.82,
		"hyprdrive:overview_tile_order":                 "position",
		"hyprdrive:strip":                               true,
		"hyprdrive:strip_cleanup_button":                true,
		"hyprdrive:backdrop":                            "off",
		"hyprdrive:mission_control":                     true,
		"hyprdrive:search:browse_on_click":              false,
		"hyprdrive:search:dictionary":                   false,
		"hyprdrive:search:translate":                    false,
		"hyprdrive:hot_corner:enabled":                  false,
		"hyprdrive:hot_corner:corner":                   "top-left",
		"hyprdrive:hot_corner:delay_ms":                 150.0,
		"hyprdrive:hot_corner:ripple":                   true,
		"hyprdrive:mosaic:enabled":                      true,
		"hyprdrive:mosaic:default":                      true,
		"hyprdrive:mosaic:infinite_canvas":              true,
		"hyprdrive:mosaic:canvas_directions":            "all",
		"hyprdrive:mosaic:super_zoom":                   true,
		"hyprdrive:mosaic:zoom_key":                     "SUPER + ALT",
		"hyprdrive:mosaic:spawn_at_cursor":              true,
		"hyprdrive:mosaic:quiet_new_windows":            true,
		"hyprdrive:mosaic:zoom_labels":                  "hover",
		"hyprdrive:general:fullscreen_desktop":          "free",
		"hyprdrive:drag:hold_to_swap":                   true,
		"hyprdrive:drag:spring_edges":                   true,
		"hyprdrive:general:open_tiled":                  false,
		"hyprdrive:general:scratchpad_bar_exit":         true,
		"hyprdrive:animation:curve":                     "serpantinum",
		"hyprdrive:motion_blur:enabled":                 true,
		"hyprdrive:layout_motion_blur":                  16.0,
		"hyprdrive:animation:desktop_row_bounce":        true,
		"hyprdrive:animation:desktop_jump_through":      true,
		"hyprdrive:binds:scroll_desktops_smooth":        false,
		"hyprdrive:gestures:keep_swipe":                 true,
		"hyprdrive:gestures:swipe_while_dragging":       true,
		"hyprdrive:gesture_detent":                      0.35,
		"hyprdrive:smooth_frames":                       true,
		"hyprdrive:mosaic:edge_push":                    true,
		"hyprdrive:mosaic:edge_push_resistance":         400.0,
		"hyprdrive:animation:canvas_pan_ms":             420.0,
		"hyprdrive:motion_blur:samples":                 16.0,
		"hyprdrive:motion_blur:layout:strength":         1.0,
		"hyprdrive:motion_blur:layout:fade":             "early",
		"hyprdrive:motion_blur:flick:strength":          0.5,
		"hyprdrive:motion_blur:flick:fade":              "early",
		"hyprdrive:motion_blur:swipe:strength":          0.5,
		"hyprdrive:motion_blur:swipe:fade":              "constant",
		"hyprdrive:motion_blur:scratchpad:strength":     0.5,
		"hyprdrive:motion_blur:scratchpad:fade":         "early",
		"hyprdrive:motion_blur:overview:strength":       0.25,
		"hyprdrive:motion_blur:overview:fade":           "early",
		"hyprdrive:motion_blur:desktop_switch:strength": 0.25,
		"hyprdrive:motion_blur:desktop_switch:fade":     "constant",
		"hyprdrive:motion_blur:search_tiles:strength":   0.25,
		"hyprdrive:motion_blur:search_tiles:fade":       "constant",
		"hyprdrive:motion_blur:search_pane:strength":    0.5,
		"hyprdrive:motion_blur:search_pane:fade":        "early",
		"hyprdrive:motion_blur:drag:strength":           1.0,
		"hyprdrive:motion_blur:drag:fade":               "constant",
		"hyprdrive:motion_blur:desktop_slide:strength":  0.5,
		"hyprdrive:motion_blur:desktop_slide:fade":      "early",
		"hyprdrive:motion_blur:wheel_pan:strength":      0.25,
		"hyprdrive:motion_blur:wheel_pan:fade":          "constant",
	}
}

// hdConflict is a Ryoku bind a Hyprdrive chord would take over.
type hdConflict struct {
	Chord string `json:"chord"`
	Label string `json:"label"`
}

// hdBindStatus is one bind as the settings page lists it.
type hdBindStatus struct {
	ID        string       `json:"id"`
	Label     string       `json:"label"`
	Detail    string       `json:"detail"`
	Chords    []string     `json:"chords"`
	Conflicts []hdConflict `json:"conflicts"`
	On        bool         `json:"on"`
	Decided   bool         `json:"decided"`
	// Default is the chords it ships with; Recordable, whether its one chord is a
	// keyboard chord the page can record a new one for
	Default    []string `json:"default"`
	Recordable bool     `json:"recordable"`
}

// hdStatus is `ryoku-wm-hyprland hyprdrive status`.
type hdStatus struct {
	Enabled   bool           `json:"enabled"`
	Installed bool           `json:"installed"`
	Loaded    bool           `json:"loaded"`
	Binds     []hdBindStatus `json:"binds"`
	TakesOver []hdConflict   `json:"takesOver"`
	// Taken is every chord Ryoku binds now, so a chord the page records can be
	// refused before it takes one over
	Taken []hdConflict `json:"taken"`
}

func runHyprdrive(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("hyprdrive needs status|cycle-layout")
	}
	switch args[0] {
	case "status":
		return printJSON(hyprdriveStatus(loadStore(desktopStorePath())))
	case "cycle-layout":
		return hyprdriveCycleLayout()
	default:
		return fmt.Errorf("unknown hyprdrive subcommand: %s", args[0])
	}
}

// chordsOf is a bind's chords with the user's move applied.
func (b hdBind) chordsOf(h Hyprdrive) []string {
	if moved, ok := h.Chords[b.ID]; ok && moved != "" && len(b.Chords) == 1 {
		return []string{moved}
	}
	return b.Chords
}

// ryokuChordLabels maps every chord Ryoku binds right now (after the user's own
// rebinds and custom binds) to what it does, keyed by normalised chord.
func ryokuChordLabels(o Overrides) map[string]string {
	out := map[string]string{}
	for _, row := range buildBindRows(hyprModulesDir(), o) {
		if row.Chord == "" || row.Disabled || strings.Contains(row.Chord, "{") {
			continue
		}
		n := wm.NormChord(row.Chord)
		if _, seen := out[n]; !seen {
			out[n] = row.Label
		}
	}
	return out
}

// conflictsOf is every Ryoku bind this bind's chords would replace.
func (b hdBind) conflictsOf(h Hyprdrive, taken map[string]string) []hdConflict {
	var out []hdConflict
	for _, c := range b.chordsOf(h) {
		if label, ok := taken[wm.NormChord(c)]; ok {
			out = append(out, hdConflict{Chord: c, Label: label})
		}
	}
	return out
}

// bindOn is the user's answer, or the default: on unless it takes a bind over.
func bindOn(h Hyprdrive, b hdBind, conflicts []hdConflict) (on, decided bool) {
	if v, ok := h.Binds[b.ID]; ok {
		return v, true
	}
	return len(conflicts) == 0 && !b.Off, false
}

func hyprdriveStatus(o Overrides) hdStatus {
	h := o.Plugins.Hyprdrive
	taken := ryokuChordLabels(o)
	st := hdStatus{Enabled: h.Enabled, Installed: pluginSoPath("hyprdrive") != "", Binds: []hdBindStatus{}, TakesOver: []hdConflict{}}
	if live() {
		_, st.Loaded = loadedPlugins()["hyprdrive"]
	}
	for _, b := range hyprdriveBinds {
		conflicts := b.conflictsOf(h, taken)
		on, decided := bindOn(h, b, conflicts)
		chords := b.chordsOf(h)
		if chords == nil {
			chords = []string{}
		}
		if conflicts == nil {
			conflicts = []hdConflict{}
		}
		def := b.Chords
		if def == nil {
			def = []string{}
		}
		recordable := len(b.Chords) == 1 && !strings.Contains(strings.ToLower(b.Chords[0]), "mouse")
		st.Binds = append(st.Binds, hdBindStatus{ID: b.ID, Label: b.Label, Detail: b.Detail, Chords: chords, Conflicts: conflicts, On: on, Decided: decided, Default: def, Recordable: recordable})
		if on {
			st.TakesOver = append(st.TakesOver, conflicts...)
		}
	}
	st.Taken = []hdConflict{}
	for chord, label := range taken {
		st.Taken = append(st.Taken, hdConflict{Chord: chord, Label: label})
	}
	sort.Slice(st.Taken, func(i, j int) bool { return st.Taken[i].Chord < st.Taken[j].Chord })
	return st
}

// hyprdriveBindDefaults is each bind's answer before the user gives one, against
// the binds Ryoku holds now: on, unless it would take one of them over.
func hyprdriveBindDefaults(o Overrides) map[string]bool {
	taken := ryokuChordLabels(o)
	out := make(map[string]bool, len(hyprdriveBinds))
	for _, b := range hyprdriveBinds {
		out[b.ID], _ = bindOn(Hyprdrive{Chords: o.Plugins.Hyprdrive.Chords}, b, b.conflictsOf(o.Plugins.Hyprdrive, taken))
	}
	return out
}

// genHyprdriveBlock is the block's Lua, "" when the plugin is off. The plugin
// is declared, then everything else waits until it is actually loaded: a
// plugin key set before the plugin registers it paints Hyprland's error overlay.
func genHyprdriveBlock(o Overrides) string {
	h := o.Plugins.Hyprdrive
	if !h.Enabled {
		return ""
	}
	so := pluginSoPath("hyprdrive")
	taken := ryokuChordLabels(o)

	var b strings.Builder
	b.WriteString(hyprdriveBegin + "\n")
	b.WriteString("-- Hyprdrive, from Ryoku Settings > Windows > Hyprdrive. Ryoku rewrites everything\n")
	b.WriteString("-- between these two lines; your own lines go above or below them.\n")
	if so == "" {
		b.WriteString("-- Hyprdrive has no copy built for this Hyprland; rebuild it from Ryoku Settings\n")
		b.WriteString("-- > Windows > Window manager.\n")
		b.WriteString(hyprdriveEnd + "\n")
		return b.String()
	}
	fmt.Fprintf(&b, "pcall(hl.plugin.load, %s)\n", luaStr(so))
	b.WriteString("local function ryoku_hyprdrive_loaded()\n")
	b.WriteString("  for _, p in ipairs(hl.get_loaded_plugins()) do\n")
	b.WriteString("    if p.name == \"hyprdrive\" then return true end\n")
	b.WriteString("  end\n")
	b.WriteString("  return false\n")
	b.WriteString("end\n")
	b.WriteString("if ryoku_hyprdrive_loaded() then\n")

	cfg := hyprdriveConfig(h, taken)
	if tree := luaConfigTree(cfg); len(tree) > 0 {
		fmt.Fprintf(&b, "  hl.config({ plugin = %s })\n", renderLuaTable(tree))
	}
	if o.Appearance.Layout == layoutMosaic {
		b.WriteString("  hl.config({ general = { layout = \"mosaic\" } })\n")
	}

	// While the overview is open the plugin enters this submap, so a click with
	// Super held reaches the overview instead of Ryoku's Super+drag.
	b.WriteString("  hl.define_submap(\"hyprdrive\", function()\n")
	b.WriteString("    hl.bind(\"mouse:272\", hl.plugin.hyprdrive.window(\"click\"), { mouse = true, description = \"Overview: select\" })\n")
	b.WriteString("    hl.bind(\"SUPER + mouse:272\", hl.plugin.hyprdrive.window(\"click\"), { mouse = true, description = \"Overview: select\" })\n")
	b.WriteString("  end)\n")

	for _, bd := range hyprdriveBinds {
		on, _ := bindOn(h, bd, bd.conflictsOf(h, taken))
		if !on {
			continue
		}
		if bd.ID == "swipe-overview" {
			b.WriteString("  hl.plugin.hyprdrive.gesture({ fingers = 3, direction = \"up\" })\n")
			b.WriteString("  hl.plugin.hyprdrive.gesture({ fingers = 3, direction = \"down\", action = \"canvas_zoom\" })\n")
			continue
		}
		for i, chord := range bd.chordsOf(h) {
			// A second bind on a chord does not replace the first (both would
			// fire), so the Ryoku bind it takes over is unbound first.
			fmt.Fprintf(&b, "  hl.unbind(%s)\n", luaStr(chord))
			fmt.Fprintf(&b, "  hl.bind(%s, %s, { description = %s })\n", luaStr(chord), bd.Lua[i], luaStr(bd.Label))
		}
	}

	// Each desktop's layout as the layout cycle last left it.
	b.WriteString("  local state = os.getenv(\"XDG_STATE_HOME\") or ((os.getenv(\"HOME\") or \"\") .. \"/.local/state\")\n")
	b.WriteString("  local layouts = io.open(state .. \"/ryoku/hyprdrive-layouts\", \"r\")\n")
	b.WriteString("  if layouts then\n")
	b.WriteString("    for line in layouts:lines() do\n")
	b.WriteString("      local ws, layout = line:match(\"^(-?%d+)%s+(%a+)$\")\n")
	b.WriteString("      if ws then hl.workspace_rule({ workspace = ws, layout = layout }) end\n")
	b.WriteString("    end\n")
	b.WriteString("    layouts:close()\n")
	b.WriteString("  end\n")
	// A reload does not load a plugin it is missing (unloaded by hand, or still
	// loading at login), so the settings and shortcuts above would wait for the
	// next Save. Load it and parse once more; nothing more when it will not load,
	// so this cannot loop.
	b.WriteString("else\n")
	fmt.Fprintf(&b, "  hl.exec_cmd(%s)\n", luaStr(hyprdriveHealCmd(so)))
	b.WriteString("end\n")
	b.WriteString(hyprdriveEnd + "\n")
	return b.String()
}

// hyprdriveConfig is the plugin keys the block sets: the page's choices, then
// the ones the bind switches own, so a stale page value can never turn a bind
// back on behind the switch.
func hyprdriveConfig(h Hyprdrive, taken map[string]string) map[string]any {
	cfg := map[string]any{}
	for k, v := range h.Config {
		if strings.HasPrefix(k, "hyprdrive:") {
			cfg[k] = v
		}
	}
	for _, bd := range hyprdriveBinds {
		if bd.Config == "" {
			continue
		}
		on, _ := bindOn(h, bd, bd.conflictsOf(h, taken))
		cfg[bd.Config] = on
	}
	return cfg
}

func userLuaPath() string { return filepath.Join(hyprConfigDir(), "user.lua") }

// spliceHyprdrive replaces the fenced block in user.lua's text with block (""
// removes it), leaving every other line exactly as the user wrote it. A block
// whose end marker was deleted by hand runs to the end of the file.
func spliceHyprdrive(text, block string) string {
	start := strings.Index(text, hyprdriveBegin)
	if start < 0 {
		if block == "" {
			return text
		}
		if text != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		if text != "" {
			text += "\n"
		}
		return text + block
	}
	end := len(text)
	if rel := strings.Index(text[start:], hyprdriveEnd); rel >= 0 {
		end = start + rel + len(hyprdriveEnd)
		if end < len(text) && text[end] == '\n' {
			end++
		}
	}
	before, after := text[:start], text[end:]
	if block == "" {
		// the blank line the append put before the block goes with it, and only
		// that one, so a file that ended in a blank line still does
		if strings.HasSuffix(before, "\n\n") {
			before = before[:len(before)-1]
		}
		return before + after
	}
	return before + block + after
}

// writeHyprdriveBlock brings user.lua in line with the store. A user.lua that
// does not exist yet is created only when there is a block to put in it.
func writeHyprdriveBlock(o Overrides) error {
	path := userLuaPath()
	old, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	next := spliceHyprdrive(string(old), genHyprdriveBlock(o))
	if next == string(old) {
		return nil
	}
	return atomicWrite(path, []byte(next), 0o644)
}

// syncHyprdriveLive loads or unloads the plugin to match the store before the
// caller reloads: hl.plugin.load in a config is only honoured when the config
// is parsed, and the keys the block sets exist only once the plugin is in.
func syncHyprdriveLive(o Overrides) {
	if !live() {
		return
	}
	_, loaded := loadedPlugins()["hyprdrive"]
	switch {
	case o.Plugins.Hyprdrive.Enabled && !loaded:
		if so := pluginSoPath("hyprdrive"); so != "" {
			_ = hyprctlPluginLoad(so)
		}
	case !o.Plugins.Hyprdrive.Enabled && loaded:
		unloadPluginCopies("hyprdrive")
	}
}

func hyprdriveLayoutsPath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	return filepath.Join(dir, "ryoku", "hyprdrive-layouts")
}

// nextLayout is the cycle Dwindle -> Scrolling -> Mosaic -> Dwindle; anything
// else (master) goes to Dwindle.
func nextLayout(current string) string {
	switch current {
	case "dwindle":
		return "scrolling"
	case "scrolling":
		return "mosaic"
	default:
		return "dwindle"
	}
}

// hyprdriveCycleLayout moves the focused desktop to the next layout, live, and
// records it so the block replays it at every parse.
func hyprdriveCycleLayout() error {
	out, err := ctl("activeworkspace", "-j")
	if err != nil {
		return err
	}
	var ws struct {
		ID          int    `json:"id"`
		TiledLayout string `json:"tiledLayout"`
	}
	if err := json.Unmarshal(out, &ws); err != nil {
		return fmt.Errorf("activeworkspace: %w", err)
	}
	layout := nextLayout(ws.TiledLayout)
	if err := evalLua(fmt.Sprintf("hl.workspace_rule({ workspace = %s, layout = %s })", luaStr(fmt.Sprint(ws.ID)), luaStr(layout))); err != nil {
		return err
	}
	if err := recordLayout(hyprdriveLayoutsPath(), ws.ID, layout); err != nil {
		return err
	}
	return printJSON(map[string]any{"workspace": ws.ID, "layout": layout})
}

// recordLayout rewrites the layouts file with this desktop's line replaced, one
// "<workspace> <layout>" line each, in workspace order.
func recordLayout(path string, id int, layout string) error {
	lines := map[int]string{}
	if b, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			var n int
			var l string
			if _, err := fmt.Sscanf(line, "%d %s", &n, &l); err == nil {
				lines[n] = l
			}
		}
	}
	lines[id] = layout
	ids := make([]int, 0, len(lines))
	for n := range lines {
		ids = append(ids, n)
	}
	sort.Ints(ids)
	var b strings.Builder
	for _, n := range ids {
		fmt.Fprintf(&b, "%d %s\n", n, lines[n])
	}
	return atomicWrite(path, []byte(b.String()), 0o644)
}

// layoutMosaic is Hyprdrive's layout, a choice beside Split, Main and stack and
// Scrolling while Hyprdrive is on.
const layoutMosaic = "mosaic"

// coreLayout is the layout settings.lua may name. It runs before the Hyprdrive
// block loads the plugin, so Mosaic is Split there and the block switches to
// Mosaic once the plugin is in; with Hyprdrive off, Split is what stays.
func coreLayout(layout string) string {
	if layout == layoutMosaic {
		return "dwindle"
	}
	return layout
}

// mosaicLiveLua puts a Mosaic choice on screen at once, and only while the
// plugin is loaded: a layout Hyprland does not know is a config error.
func mosaicLiveLua(o Overrides) string {
	if o.Appearance.Layout != layoutMosaic || !o.Plugins.Hyprdrive.Enabled {
		return ""
	}
	return `for _, p in ipairs(hl.get_loaded_plugins()) do
  if p.name == "hyprdrive" then hl.config({ general = { layout = "mosaic" } }) end
end
`
}

// hyprdriveHealCmd loads the plugin when the compositor does not hold it and
// reloads once it does, so the block's guarded half runs.
func hyprdriveHealCmd(so string) string {
	held := `hyprctl plugin list | grep -q "^Plugin hyprdrive "`
	return "sh -c '" + "sleep 1; " + held + " || hyprctl plugin load \"" + so + "\" >/dev/null 2>&1; " + held + " && hyprctl reload >/dev/null'"
}
