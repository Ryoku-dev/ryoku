package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Plugin config emission (the settings.lua plugin blocks and the live-eval config
// push) and the `plugins` verb dispatcher.

// pluginDir is where the package installs the optional plugin .so files.
const pluginDir = "/usr/lib/hyprland/plugins"

// runPlugins dispatches the plugin subsystem verb.
func runPlugins(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("plugins needs list|rebuild|sync|add|remove")
	}
	switch args[0] {
	case "list":
		return printJSON(pluginRosterNow())
	case "rebuild":
		return runPluginRebuild(args[1:])
	case "sync":
		return runPluginSync()
	case "add":
		return runPluginAdd(args[1:])
	case "remove":
		return runPluginRemove(args[1:])
	default:
		return fmt.Errorf("unknown plugins subcommand: %s", args[0])
	}
}

// runPluginSync reconciles the live plugin set to the store after an apply +
// reload: it unloads a plugin left running by hand that the store now disables,
// loads an enabled one a reload did not, and pushes every config. It reconciles
// toward the store, so it needs no before/after diff.
func runPluginSync() error {
	o := loadStore(desktopStorePath())
	if live() {
		loaded := loadedPlugins()
		for _, d := range bundledPlugins {
			if pluginEnabled(o, d.ID) {
				continue
			}
			if _, on := loaded[d.Loaded]; on {
				unloadPluginCopies(d.ID)
			}
		}
		for _, id := range sortedExtraIDs(o.Plugins.Extra) {
			if o.Plugins.Extra[id].Enabled {
				continue
			}
			name := id
			if r, ok := readReceipt(id); ok && r.Loaded != "" {
				name = r.Loaded
			}
			if _, on := loaded[name]; on {
				unloadPluginCopies(id)
			}
		}
		loadEnabledPlugins(o)
		pushEval(genPluginConfig(o))
	}
	return printJSON(map[string]bool{"synced": true})
}

// printJSON writes v to the buffered stdout, compact, one object per line.
func printJSON(v any) error {
	return json.NewEncoder(stdout).Encode(v)
}

// pushEval applies Lua flash-free via hyprctl eval; empty input is a no-op.
func pushEval(lua string) {
	if strings.TrimSpace(lua) == "" {
		return
	}
	_, _ = ctl("eval", lua)
}

// pluginSoPath resolves the copy settings.lua should load for the compositor this
// file is generated for. "" means nothing loadable is installed, so the load is
// left out rather than have Hyprland refuse it on every reload.
func pluginSoPath(id string) string {
	target := liveCompositor().abi
	if !target.ok() {
		target, _ = headersABI()
	}
	c, ok, stale := pickCopy(pluginCopies(id), target)
	if !ok || stale {
		return ""
	}
	return c.Path
}

// pluginBlock is one enabled plugin resolved for emission.
type pluginBlock struct {
	id     string
	name   string // as `plugins list` reports it; the guard's key
	so     string // "" when no copy built for this Hyprland is installed
	config string // the `plugin = { ... }` table, "" when nothing to set
	extra  string // verbatim Lua after the config
}

// luaPluginLoaded is the guard every plugin block runs its config behind:
// hl.plugin.load only declares a path and Hyprland loads the set in a later pass,
// so the config must wait for the plugin to actually be present.
const luaPluginLoaded = `local function ryoku_plugin_loaded(name)
  for _, p in ipairs(hl.get_loaded_plugins()) do
    if p.name == name then return true end
  end
  return false
end

`

const luaPluginStartupSync = `hl.on("hyprland.start", function()
  hl.exec_cmd("ryoku-wm-hyprland plugins sync >/dev/null 2>&1")
end)

`

// genPlugins renders the enabled plugins for settings.lua. Hyprland resolves
// hl.plugin.load declarations after the first config pass; the startup sync
// then loads anything still missing and applies the saved plugin values.
func genPlugins(o Overrides) string {
	var b strings.Builder
	blocks := pluginBlocks(o)
	if len(blocks) > 0 {
		b.WriteString(luaPluginLoaded)
	}
	for _, pb := range blocks {
		if pb.so == "" {
			fmt.Fprintf(&b, "-- %s: no copy built for this Hyprland; rebuild it from Settings > Plugins\n\n", pb.id)
			continue
		}
		b.WriteString(pb.loadLua())
	}
	if len(blocks) > 0 {
		b.WriteString(luaPluginStartupSync)
	}
	b.WriteString(genScrolling(o))
	return b.String()
}

// loadLua keeps one plugin's failure local while making it visible in the
// compositor log instead of discarding pcall's error value.
func (pb pluginBlock) loadLua() string {
	var b strings.Builder
	b.WriteString("do\n")
	b.WriteString("  local ok, err = pcall(function()\n")
	fmt.Fprintf(&b, "    hl.plugin.load(%s)\n", luaStr(pb.so))
	b.WriteString(pb.configLua("    "))
	b.WriteString("  end)\n")
	b.WriteString("  if not ok then\n")
	fmt.Fprintf(&b, "    print(%s .. tostring(err))\n", luaStr("Ryoku plugin "+pb.id+": "))
	b.WriteString("  end\n")
	b.WriteString("end\n\n")
	return b.String()
}

// genPluginConfig renders only the config of the enabled plugins, for the live
// eval: the preview and the pass after a load, when a plugin just come up still
// holds its defaults.
func genPluginConfig(o Overrides) string {
	var b strings.Builder
	for _, pb := range pluginBlocks(o) {
		if pb.so == "" || pb.config == "" {
			continue
		}
		if b.Len() == 0 {
			b.WriteString(luaPluginLoaded)
		}
		b.WriteString(pb.configLua(""))
	}
	return b.String()
}

// configLua is the guarded config of one block, indented for its context.
func (pb pluginBlock) configLua(indent string) string {
	if pb.config == "" && pb.extra == "" {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%sif ryoku_plugin_loaded(%s) then\n", indent, luaStr(pb.name))
	if pb.config != "" {
		fmt.Fprintf(&b, "%s  hl.config({ plugin = %s })\n", indent, pb.config)
	}
	b.WriteString(pb.extra)
	fmt.Fprintf(&b, "%send\n", indent)
	return b.String()
}

// typedBlock is a bundled plugin's block. ConfigSection is the exact name
// Hyprland exposes to Lua after normalising the plugin's option namespace.
func typedBlock(id string, opts []string, extra string) pluginBlock {
	d, _ := bundledByID(id)
	return pluginBlock{
		id: id, name: d.Loaded, so: pluginSoPath(id),
		config: fmt.Sprintf("{ %s = { %s } }", d.ConfigSection, strings.Join(opts, ", ")),
		extra:  extra,
	}
}

// pluginBlocks resolves every enabled plugin, bundled ones first, then added.
func pluginBlocks(o Overrides) []pluginBlock {
	var out []pluginBlock
	p := o.Plugins

	if dc := p.DynamicCursors; dc.Enabled {
		opts := []string{
			"enabled = true",
			fmt.Sprintf("mode = %s", luaStr(dc.Mode)),
			fmt.Sprintf("shake = { enabled = %t, base = %s }", dc.Shake, luaNum(dc.Magnify)),
		}
		out = append(out, typedBlock("dynamic-cursors", opts, ""))
	}

	if hb := p.Hyprbars; hb.Enabled {
		opts := []string{
			"enabled = true",
			fmt.Sprintf("bar_height = %d", hb.Height),
			fmt.Sprintf("bar_text_size = %d", hb.TextSize),
			fmt.Sprintf("bar_blur = %t", hb.Blur),
		}
		var extra string
		if hb.Buttons {
			extra = "    hl.plugin.hyprbars.add_button({ bg_color = \"rgb(ff5f57)\", fg_color = \"rgb(ffffff)\", size = 12, icon = \"\u00d7\", action = \"hyprctl dispatch killactive\" })\n" +
				"    hl.plugin.hyprbars.add_button({ bg_color = \"rgb(28c840)\", fg_color = \"rgb(ffffff)\", size = 12, icon = \"+\", action = \"hyprctl dispatch fullscreen 1\" })\n"
		}
		out = append(out, typedBlock("hyprbars", opts, extra))
	}

	if ib := p.Imgborders; ib.Enabled {
		opts := []string{
			"enabled = true",
			fmt.Sprintf("image = %s", luaStr(ib.Image)),
			fmt.Sprintf("sizes = %s", luaStr(ib.Sizes)),
			fmt.Sprintf("insets = %s", luaStr(ib.Insets)),
			fmt.Sprintf("scale = %s", luaNum(ib.Scale)),
			fmt.Sprintf("smooth = %t", ib.Smooth),
			fmt.Sprintf("blur = %t", ib.Blur),
		}
		out = append(out, typedBlock("imgborders", opts, ""))
	}

	if hg := p.Hyprglass; hg.Enabled {
		opts := []string{
			"enabled = 1",
			fmt.Sprintf("default_preset = %s", luaStr(hg.Preset)),
			fmt.Sprintf("blur_strength = %s", luaNum(hg.BlurStrength)),
			fmt.Sprintf("glass_opacity = %s", luaNum(hg.Opacity)),
			fmt.Sprintf("tint_color = 0x%s", luaHex8(hg.Tint)),
			fmt.Sprintf("brightness = %s", luaNum(hg.Brightness)),
			fmt.Sprintf("default_theme = %s", luaStr(hg.Theme)),
		}
		out = append(out, typedBlock("hyprglass", opts, ""))
	}

	if hf := p.Hyprfocus; hf.Enabled {
		// the 0.56 plugin renamed its keys: the animation is chosen per input
		// source and bounce became shrink. The store keeps the stable names.
		anim := hf.Mode
		if anim == "bounce" {
			anim = "shrink"
		}
		opts := []string{
			"enable = true",
			fmt.Sprintf("keyboard_focus_animation = %s", luaStr(anim)),
			`mouse_focus_animation = "none"`,
			fmt.Sprintf("fade_opacity = %s", luaNum(hf.Opacity)),
			fmt.Sprintf("shrink_percentage = %s", luaNum(hf.Bounce)),
			fmt.Sprintf("slide_height = %s", luaNum(hf.Slide)),
		}
		out = append(out, typedBlock("hyprfocus", opts, ""))
	}

	if ks := p.Keysounds; ks.Enabled {
		opts := []string{
			"enabled = true",
			fmt.Sprintf("profile = %s", luaStr(ks.Profile)),
			fmt.Sprintf("volume = %s", luaNum(ks.Volume)),
			fmt.Sprintf("release = %t", ks.Release),
		}
		out = append(out, typedBlock("keysounds", opts, ""))
	}

	if bp := p.BordersPlusPlus; bp.Enabled {
		// borders-plus-plus registers no enabled key: the store flag already
		// gates whether the block is emitted and the .so loaded at all.
		opts := []string{
			fmt.Sprintf("add_borders = %d", bp.Count),
			fmt.Sprintf("natural_rounding = %t", bp.NaturalRounding),
			fmt.Sprintf("col = { border_1 = %s }", luaStr(luaRGB(bp.Color))),
			fmt.Sprintf("border_size_1 = %d", bp.Size),
		}
		out = append(out, typedBlock("borders-plus-plus", opts, ""))
	}

	if gr := p.Hyprgrass; gr.Enabled {
		// hyprgrass likewise registers no enabled key.
		opts := []string{
			fmt.Sprintf("sensitivity = %s", luaNum(gr.Sensitivity)),
			fmt.Sprintf("long_press_delay = %d", gr.LongPressDelay),
			fmt.Sprintf("edge_margin = %d", gr.EdgeMargin),
			fmt.Sprintf("resize_on_border_long_press = %t", gr.ResizeOnBorder),
		}
		var extra string
		if gr.ThreeFingerTap {
			// A three-finger tap floats the window under it: the one pointer
			// gesture a touchscreen needs and Hyprland core does not bind.
			extra = "    hl.plugin.hyprgrass.bind({\n" +
				"        pattern = {kind = \"tap\", fingers = 3},\n" +
				"        action = hl.dsp.window.float({ action = \"toggle\" }),\n" +
				"    })\n"
		}
		out = append(out, typedBlock("hyprgrass", opts, extra))
	}

	for _, id := range sortedExtraIDs(p.Extra) {
		if ep := p.Extra[id]; ep.Enabled {
			out = append(out, extraBlock(id, ep.Config))
		}
	}
	return out
}

// genScrolling: the scrolling layout is Hyprland core (0.54+), configured under
// the core `scrolling` category. Its knobs emit only when the layout is selected.
func genScrolling(o Overrides) string {
	if o.Appearance.Layout != "scrolling" {
		return ""
	}
	var b strings.Builder
	hs, dhs := o.Plugins.Hyprscrolling, defaultOverrides().Plugins.Hyprscrolling
	var sc []string
	if hs.ColumnWidth != dhs.ColumnWidth {
		sc = append(sc, fmt.Sprintf("column_width = %s", luaNum(hs.ColumnWidth)))
	}
	if hs.FollowFocus != dhs.FollowFocus {
		sc = append(sc, fmt.Sprintf("follow_focus = %t", hs.FollowFocus))
	}
	if len(sc) > 0 {
		fmt.Fprintf(&b, "hl.config({ scrolling = { %s } })\n\n", strings.Join(sc, ", "))
	}
	// follow focus is inert while the pointer can't move focus (shipped
	// follow_mouse = 2), so let the pointer drive focus too unless the user pinned it.
	if hs.FollowFocus && o.Input.FollowMouse == defaultOverrides().Input.FollowMouse {
		b.WriteString("hl.config({ input = { follow_mouse = 1 } })\n\n")
	}
	return b.String()
}

// extraBlock is an added plugin's block: the config keys the user set, stored as
// full colon paths, as nested tables. The name the plugin reports (the guard's
// key) comes from its receipt once a load revealed it, the id until then.
func extraBlock(id string, cfg map[string]any) pluginBlock {
	name := id
	if r, ok := readReceipt(id); ok && r.Loaded != "" {
		name = r.Loaded
	}
	pb := pluginBlock{id: id, name: name, so: pluginSoPath(id)}
	if tree := luaConfigTree(cfg); len(tree) > 0 {
		pb.config = renderLuaTable(tree)
	}
	return pb
}

// luaConfigTree nests colon-path keys into tables: "a:b:c" -> { a = { b = { c } } }.
func luaConfigTree(cfg map[string]any) map[string]any {
	tree := map[string]any{}
	for _, k := range sortedKeys(cfg) {
		parts := strings.Split(k, ":")
		if len(parts) < 2 {
			continue
		}
		for i := range parts {
			parts[i] = strings.ReplaceAll(parts[i], "-", "_")
		}
		cur := tree
		for _, p := range parts[:len(parts)-1] {
			next, ok := cur[p].(map[string]any)
			if !ok {
				next = map[string]any{}
				cur[p] = next
			}
			cur = next
		}
		cur[parts[len(parts)-1]] = cfg[k]
	}
	return tree
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

var luaIdentRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// renderLuaTable renders a nested map as a Lua table constructor with stable key
// order; a key that is not a bare identifier is bracketed.
func renderLuaTable(t map[string]any) string {
	var parts []string
	for _, k := range sortedKeys(t) {
		key := k
		if !luaIdentRe.MatchString(k) {
			key = "[" + luaStr(k) + "]"
		}
		parts = append(parts, key+" = "+luaValue(t[k]))
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}

// luaValue renders a store value by its JSON type.
func luaValue(v any) string {
	switch x := v.(type) {
	case bool:
		return strconv.FormatBool(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	case string:
		return luaStr(x)
	case map[string]any:
		return renderLuaTable(x)
	}
	return luaStr(fmt.Sprint(v))
}

// luaHex8 sanitises an RRGGBBAA hex string to a bare 8-digit lowercase hex for a
// Lua 0x literal; a malformed value falls back to the hyprglass default tint.
func luaHex8(s string) string {
	s = strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(s)), "0x"), "#")
	if len(s) != 8 {
		return "8899aa22"
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return "8899aa22"
		}
	}
	return s
}
