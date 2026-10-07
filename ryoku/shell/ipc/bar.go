package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// bar.go is the agent-facing bar/dock control surface on the daemon: the
// `ryoku-shell bar ...` and `ryoku-shell dock ...` verbs. The bar layout lives in
// shell.json (qsbar.layout) as three ordered lists of stable widget ids;
// built-in widgets and installed store plugins are entries of the same list.
// These verbs read the widget catalogue (the one file the shell, the panel and
// this CLI share) and the installed plugins, validate every mutation against
// them, and write through the settings store (the sole writer of shell.json) or,
// for a plugin, through ryoku-plugins-place. Nothing here executes plugin code.
//
// The layout algebra (layoutMove, normalizeLayout, layoutDefaults, barListRows)
// is pure so it can be unit-tested without a running daemon.

// barCLIVerbs are the data-layout bar verbs. They take a verb first, so they do
// not collide with the reveal grammar (`bar <edge> <action>`), which takes an
// edge first; the dispatch switch routes on this set.
var barCLIVerbs = map[string]bool{
	"list": true, "catalog": true, "move": true, "show": true, "hide": true,
	"set": true, "position": true, "form": true, "defaults": true, "settings": true,
}

// barForms are the shell shapes the bar can take (qsbar.barShellStyle).
var barForms = map[string]bool{"full": true, "fit": true, "dock": true, "notch": true, "islands": true}

// dockEdges are the screen edges the dock can anchor to; "auto" tracks the edge
// opposite the bar.
var dockEdges = map[string]bool{"auto": true, "top": true, "bottom": true, "left": true, "right": true}

// shippedOff is the set of catalogue visKeys that ship hidden, mirroring the
// mod* property defaults in the qsbar Theme.qml (the AI pill, power and
// bluetooth). Everything else ships shown. `bar defaults` derives the shipped
// visibility from the catalogue using this set, so a terminal reset matches the
// panel's barLayoutReset exactly.
var shippedOff = map[string]bool{"claude": true, "power": true, "bluetooth": true}

// catalogWidget is one built-in widget row from widgets.json.
type catalogWidget struct {
	ID       string           `json:"id"`
	Gid      string           `json:"gid"`
	Label    string           `json:"label"`
	Gloss    string           `json:"gloss"`
	Category string           `json:"category"`
	Desc     string           `json:"desc"`
	VisKey   string           `json:"visKey"`
	Settings []catalogSetting `json:"settings"`
}

// catalogSetting is one settings row in the catalogue. VisKey here is a bool
// flag (distinct from a widget's string visKey): a row with visKey:true is
// itself a visibility toggle whose key is a qsbar.widgets key (clock's weather),
// so `bar defaults` writes it too.
type catalogSetting struct {
	Key     string   `json:"key"`
	Type    string   `json:"type"`
	Label   string   `json:"label"`
	Options []string `json:"options"`
	Min     *int     `json:"min"`
	Max     *int     `json:"max"`
	VisKey  *bool    `json:"visKey"`
}

// barLayout is the qsbar.layout document: placement only, three ordered id lists.
type barLayout struct {
	Version int      `json:"version"`
	Left    []string `json:"left"`
	Center  []string `json:"center"`
	Right   []string `json:"right"`
}

// knownWidget is one placeable widget id and the section a copy lands in when it
// is missing from the layout (built-ins default to right; a plugin uses its
// manifest defaults.bar.section).
type knownWidget struct {
	id      string
	section string
}

// placement is the requested position for `bar move`: an explicit 0-based index,
// or an anchor to sit before/after within the target section.
type placement struct {
	hasIndex bool
	index    int
	before   string
	after    string
}

// listRow is one `bar list` element: the placement and visibility of a widget.
type listRow struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Kind    string `json:"kind"`
	Section string `json:"section"`
	Index   int    `json:"index"`
	Shown   bool   `json:"shown"`
}

// widgetMeta is the per-id data barListRows needs beyond placement.
type widgetMeta struct {
	Label string
	Kind  string
	Shown bool
}

// pluginInfo is one installed plugin: its id, its directory, and its parsed
// manifest. Accessors read manifest fields defensively so a malformed manifest
// degrades to defaults rather than panicking.
type pluginInfo struct {
	ID       string
	Dir      string
	Manifest map[string]any
}

// pluginState is a plugin's placement in ~/.config/ryoku/plugins.json.
type pluginState struct {
	enabled bool
	host    string
}

// shippedLayout is the default order (contract 2): the order a fresh box gets.
func shippedLayout() barLayout {
	return barLayout{
		Version: 1,
		Left:    []string{"launcher", "workspaces", "status", "cpu", "volume", "memory", "ai"},
		Center:  []string{"clock"},
		Right:   []string{"media", "quick", "network", "power", "battery", "brightness", "cputemp", "storage", "gpu", "bluetooth", "layout"},
	}
}

// layoutDefaults is the shipped order filtered to the ids the catalogue actually
// carries. The shipped order is a fixed contract, not derived from the
// catalogue; the catalogue only prunes an id it has dropped.
func layoutDefaults(cat []catalogWidget) barLayout {
	ids := map[string]bool{}
	for _, w := range cat {
		ids[w.ID] = true
	}
	filter := func(xs []string) []string {
		out := []string{}
		for _, id := range xs {
			if ids[id] {
				out = append(out, id)
			}
		}
		return out
	}
	ship := shippedLayout()
	return barLayout{Version: 1, Left: filter(ship.Left), Center: filter(ship.Center), Right: filter(ship.Right)}
}

// visibilityDefaults derives the shipped qsbar.widgets map from the catalogue:
// every widget visKey and every settings row flagged visKey:true, shown unless
// it is in the ship-off set.
func visibilityDefaults(cat []catalogWidget) map[string]bool {
	out := map[string]bool{}
	for _, w := range cat {
		if w.VisKey != "" {
			out[w.VisKey] = !shippedOff[w.VisKey]
		}
		for _, s := range w.Settings {
			if s.VisKey != nil && *s.VisKey {
				out[s.Key] = !shippedOff[s.Key]
			}
		}
	}
	return out
}

// removeID returns ids with every occurrence of id removed.
func removeID(ids []string, id string) []string {
	out := []string{}
	for _, x := range ids {
		if x != id {
			out = append(out, x)
		}
	}
	return out
}

func indexOf(ids []string, id string) int {
	for i, x := range ids {
		if x == id {
			return i
		}
	}
	return -1
}

// insertAt inserts id into ids at idx (clamped to the ends), returning a fresh
// slice so callers never alias a shared backing array.
func insertAt(ids []string, idx int, id string) []string {
	if idx < 0 {
		idx = 0
	}
	if idx > len(ids) {
		idx = len(ids)
	}
	out := make([]string, 0, len(ids)+1)
	out = append(out, ids[:idx]...)
	out = append(out, id)
	out = append(out, ids[idx:]...)
	return out
}

func validSection(s string) bool { return s == "left" || s == "center" || s == "right" }

// layoutMove pulls id out of wherever it sits and re-inserts it in section at
// the requested position: an explicit index, or before/after an anchor already
// in that section. An anchor that is not in the section is an error.
func layoutMove(l barLayout, id, section string, p placement) (barLayout, error) {
	if !validSection(section) {
		return l, fmt.Errorf("unknown section %q", section)
	}
	out := barLayout{
		Version: l.Version,
		Left:    removeID(l.Left, id),
		Center:  removeID(l.Center, id),
		Right:   removeID(l.Right, id),
	}
	if out.Version == 0 {
		out.Version = 1
	}
	var target []string
	switch section {
	case "left":
		target = out.Left
	case "center":
		target = out.Center
	case "right":
		target = out.Right
	}
	idx := len(target)
	switch {
	case p.before != "":
		i := indexOf(target, p.before)
		if i < 0 {
			return l, fmt.Errorf("unknown anchor %q in section %s", p.before, section)
		}
		idx = i
	case p.after != "":
		i := indexOf(target, p.after)
		if i < 0 {
			return l, fmt.Errorf("unknown anchor %q in section %s", p.after, section)
		}
		idx = i + 1
	case p.hasIndex:
		idx = p.index
	}
	target = insertAt(target, idx, id)
	switch section {
	case "left":
		out.Left = target
	case "center":
		out.Center = target
	case "right":
		out.Right = target
	}
	return out, nil
}

// normalizeLayout reconciles a layout against the known widget set: it keeps the
// first occurrence of each id, drops duplicates and ids that are no longer known
// (a plugin uninstalled since the layout was written), and appends any known
// widget the layout omits to its default section, in known order.
func normalizeLayout(l barLayout, known []knownWidget) barLayout {
	sectionOf := map[string]string{}
	order := []string{}
	for _, k := range known {
		if _, seen := sectionOf[k.id]; !seen {
			order = append(order, k.id)
		}
		sec := k.section
		if !validSection(sec) {
			sec = "right"
		}
		sectionOf[k.id] = sec
	}
	seen := map[string]bool{}
	keep := func(ids []string) []string {
		out := []string{}
		for _, id := range ids {
			if seen[id] {
				continue
			}
			if _, ok := sectionOf[id]; !ok {
				continue
			}
			seen[id] = true
			out = append(out, id)
		}
		return out
	}
	out := barLayout{Version: l.Version, Left: keep(l.Left), Center: keep(l.Center), Right: keep(l.Right)}
	if out.Version == 0 {
		out.Version = 1
	}
	for _, id := range order {
		if seen[id] {
			continue
		}
		seen[id] = true
		switch sectionOf[id] {
		case "left":
			out.Left = append(out.Left, id)
		case "center":
			out.Center = append(out.Center, id)
		default:
			out.Right = append(out.Right, id)
		}
	}
	return out
}

// barListRows walks a normalized layout in section order and stamps each id with
// its section, 0-based index within the section, and metadata. An id with no
// metadata is skipped (a normalized layout never contains one).
func barListRows(l barLayout, meta map[string]widgetMeta) []listRow {
	rows := []listRow{}
	add := func(section string, ids []string) {
		for i, id := range ids {
			md, ok := meta[id]
			if !ok {
				continue
			}
			rows = append(rows, listRow{ID: id, Label: md.Label, Kind: md.Kind, Section: section, Index: i, Shown: md.Shown})
		}
	}
	add("left", l.Left)
	add("center", l.Center)
	add("right", l.Right)
	return rows
}

// ── catalogue and plugin loading ────────────────────────────────────────────

// shellCatalogPath resolves the widget catalogue the shell materialised: on a
// deployed box ~/.config/quickshell/shell/..., on a checkout
// RYOKU_SHELL_DIR/quickshell/shell/..., mirroring the daemon's shell-config
// resolution elsewhere.
func shellCatalogPath() string {
	var base string
	if shellDir != "" {
		base = filepath.Join(shellDir, "quickshell", "shell")
	} else {
		configRoot := os.Getenv("XDG_CONFIG_HOME")
		if configRoot == "" {
			home, _ := os.UserHomeDir()
			configRoot = filepath.Join(home, ".config")
		}
		base = filepath.Join(configRoot, "quickshell", "shell")
	}
	return filepath.Join(base, "modules", "bar", "barstyles", "qsbar", "core", "widgets.json")
}

func loadCatalog(path string) ([]catalogWidget, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("catalogue not found at %s", path)
	}
	var cat []catalogWidget
	if err := json.Unmarshal(b, &cat); err != nil {
		return nil, fmt.Errorf("parse catalogue: %v", err)
	}
	return cat, nil
}

// dataHome is $XDG_DATA_HOME (default ~/.local/share).
func dataHome() string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return d
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "share")
}

// pluginsPlaceTool resolves ryoku-plugins-place: the checkout copy under
// RYOKU_SHELL_DIR, else the packaged name on PATH (matching Desktop.qml).
func pluginsPlaceTool() string {
	if shellDir != "" {
		return filepath.Join(shellDir, "quickshell", "plugins", "ryoku-plugins-place")
	}
	return "ryoku-plugins-place"
}

// loadInstalledPlugins reads every plugin manifest from the dev-override dirs
// (RYOSTORE_PLUGINS_DIR, legacy RYOKU_PLUGINS_DIR, colon-separated) and the
// installed root (~/.local/share/ryoku/plugins), first source wins on a
// duplicate id, mirroring discover.sh's source order.
func loadInstalledPlugins() []pluginInfo {
	var dirs []string
	pd := os.Getenv("RYOSTORE_PLUGINS_DIR")
	if pd == "" {
		pd = os.Getenv("RYOKU_PLUGINS_DIR")
	}
	if pd != "" {
		for _, d := range strings.Split(pd, ":") {
			if d != "" {
				dirs = append(dirs, d)
			}
		}
	}
	dirs = append(dirs, filepath.Join(dataHome(), "ryoku", "plugins"))
	seen := map[string]bool{}
	var out []pluginInfo
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			b, err := os.ReadFile(filepath.Join(dir, e.Name(), "manifest.json"))
			if err != nil {
				continue
			}
			var m map[string]any
			if json.Unmarshal(b, &m) != nil {
				continue
			}
			id, _ := m["id"].(string)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, pluginInfo{ID: id, Dir: filepath.Join(dir, e.Name()), Manifest: m})
		}
	}
	return out
}

// barCapablePlugins keeps only the plugins whose manifest declares topbarGlyph.
func barCapablePlugins(all []pluginInfo) []pluginInfo {
	out := []pluginInfo{}
	for _, p := range all {
		if inSet(p.hosts(), "topbarGlyph") {
			out = append(out, p)
		}
	}
	return out
}

// loadPluginsState reads ~/.config/ryoku/plugins.json into enabled/host per id.
func loadPluginsState() map[string]pluginState {
	out := map[string]pluginState{}
	b, err := os.ReadFile(filepath.Join(ryokuConfigDir(), "plugins.json"))
	if err != nil {
		return out
	}
	var raw map[string]any
	if json.Unmarshal(b, &raw) != nil {
		return out
	}
	for id, v := range raw {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		st := pluginState{}
		if e, ok := m["enabled"].(bool); ok {
			st.enabled = e
		}
		if h, ok := m["host"].(string); ok {
			st.host = h
		}
		out[id] = st
	}
	return out
}

func (p pluginInfo) name() string {
	if n, ok := p.Manifest["name"].(string); ok && n != "" {
		return n
	}
	return p.ID
}

func (p pluginInfo) desc() string {
	s, _ := p.Manifest["description"].(string)
	return s
}

func (p pluginInfo) version() string {
	s, _ := p.Manifest["version"].(string)
	return s
}

func (p pluginInfo) author() string {
	s, _ := p.Manifest["author"].(string)
	return s
}

// official is the manifest's own claim; Ryoku's plugins ship it true, so any
// other bar plugin is a community widget (the panel's Community route).
func (p pluginInfo) official() bool {
	b, _ := p.Manifest["official"].(bool)
	return b
}

func (p pluginInfo) hosts() []string { return toStringSlice(p.Manifest["hosts"]) }

// defaultSection is the plugin's requested landing lane (manifest
// defaults.bar.section), or right when it declares none.
func (p pluginInfo) defaultSection() string {
	d, _ := p.Manifest["defaults"].(map[string]any)
	if d == nil {
		return "right"
	}
	bar, _ := d["bar"].(map[string]any)
	if bar == nil {
		return "right"
	}
	if s, ok := bar["section"].(string); ok && validSection(s) {
		return s
	}
	return "right"
}

func (p pluginInfo) settingsList() []any {
	md, _ := p.Manifest["metadata"].(map[string]any)
	if md == nil {
		return []any{}
	}
	if arr, ok := md["settings"].([]any); ok {
		return arr
	}
	return []any{}
}

func (p pluginInfo) setting(key string) (map[string]any, bool) {
	for _, s := range p.settingsList() {
		if m, ok := s.(map[string]any); ok {
			if k, _ := m["key"].(string); k == key {
				return m, true
			}
		}
	}
	return nil, false
}

func knownWidgets(cat []catalogWidget, plugins []pluginInfo) []knownWidget {
	out := []knownWidget{}
	for _, w := range cat {
		out = append(out, knownWidget{id: w.ID, section: "right"})
	}
	for _, p := range plugins {
		out = append(out, knownWidget{id: p.ID, section: p.defaultSection()})
	}
	return out
}

func findWidget(cat []catalogWidget, id string) (catalogWidget, bool) {
	for _, w := range cat {
		if w.ID == id {
			return w, true
		}
	}
	return catalogWidget{}, false
}

func findSetting(w catalogWidget, key string) (catalogSetting, bool) {
	for _, s := range w.Settings {
		if s.Key == key {
			return s, true
		}
	}
	return catalogSetting{}, false
}

func findPlugin(plugins []pluginInfo, id string) (pluginInfo, bool) {
	for _, p := range plugins {
		if p.ID == id {
			return p, true
		}
	}
	return pluginInfo{}, false
}

// toStringSlice reads a decoded JSON array into a string slice, dropping any
// non-string element.
func toStringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return []string{}
	}
	out := []string{}
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// ── settings frame access ───────────────────────────────────────────────────

// barConfig snapshots the current in-memory shell.json frame (a deep copy under
// the store lock), so a read never races the sole writer.
func (d *daemon) barConfig() map[string]any {
	if d.settings == nil {
		return map[string]any{}
	}
	d.settings.mu.Lock()
	defer d.settings.mu.Unlock()
	return deepCopyMap(d.settings.raw)
}

// parseLayout reads qsbar.layout out of a frame, falling back to the shipped
// order when the box has not written a layout yet (pre-migration).
func parseLayout(frame map[string]any) barLayout {
	qs, _ := frame["qsbar"].(map[string]any)
	if qs == nil {
		return shippedLayout()
	}
	lm, ok := qs["layout"].(map[string]any)
	if !ok {
		return shippedLayout()
	}
	l := barLayout{Version: 1}
	if v, ok := lm["version"].(float64); ok {
		l.Version = int(v)
	}
	l.Left = toStringSlice(lm["left"])
	l.Center = toStringSlice(lm["center"])
	l.Right = toStringSlice(lm["right"])
	return l
}

// parseWidgets reads qsbar.widgets (a visKey -> bool map) out of a frame.
func parseWidgets(frame map[string]any) map[string]bool {
	out := map[string]bool{}
	qs, _ := frame["qsbar"].(map[string]any)
	if qs == nil {
		return out
	}
	w, _ := qs["widgets"].(map[string]any)
	for k, v := range w {
		if b, ok := v.(bool); ok {
			out[k] = b
		}
	}
	return out
}

// ── verb dispatch ───────────────────────────────────────────────────────────

func (d *daemon) barCLI(args []string) string {
	switch args[0] {
	case "list":
		return d.barList(hasJSONFlag(args[1:]))
	case "catalog":
		return d.barCatalog(hasJSONFlag(args[1:]))
	case "move":
		return d.barMove(args[1:])
	case "show":
		return d.barShowHide(args[1:], true)
	case "hide":
		return d.barShowHide(args[1:], false)
	case "set":
		return d.barSet(args[1:])
	case "position":
		return d.barPosition(args[1:])
	case "form":
		return d.barForm(args[1:])
	case "defaults":
		return d.barDefaults()
	case "settings":
		return d.barSettings(args[1:])
	}
	return "err bar: unknown verb " + args[0]
}

func hasJSONFlag(args []string) bool {
	for _, a := range args {
		if a == "--json" {
			return true
		}
	}
	return false
}

func (d *daemon) barList(asJSON bool) string {
	cat, err := loadCatalog(shellCatalogPath())
	if err != nil {
		return "err bar: " + err.Error()
	}
	plugins := barCapablePlugins(loadInstalledPlugins())
	frame := d.barConfig()
	known := knownWidgets(cat, plugins)
	layout := normalizeLayout(parseLayout(frame), known)
	widgets := parseWidgets(frame)
	state := loadPluginsState()

	meta := map[string]widgetMeta{}
	for _, w := range cat {
		shown := true
		if w.VisKey != "" {
			if v, ok := widgets[w.VisKey]; ok {
				shown = v
			} else {
				shown = !shippedOff[w.VisKey]
			}
		}
		meta[w.ID] = widgetMeta{Label: w.Label, Kind: "builtin", Shown: shown}
	}
	for _, p := range plugins {
		st := state[p.ID]
		meta[p.ID] = widgetMeta{Label: p.name(), Kind: "plugin", Shown: st.enabled && st.host == "topbarGlyph"}
	}

	rows := barListRows(layout, meta)
	if asJSON {
		b, _ := json.Marshal(rows)
		return string(b)
	}
	var b strings.Builder
	for _, r := range rows {
		shown := "shown"
		if !r.Shown {
			shown = "hidden"
		}
		fmt.Fprintf(&b, "%-6s %2d  %-12s %-8s %-6s  %s\n", r.Section, r.Index, r.ID, r.Kind, shown, r.Label)
	}
	return strings.TrimRight(b.String(), "\n")
}

// catalogSettingsView renders the catalogue settings rows to the contract row
// shape ({key,type,label?,options?,min?,max?}); the visKey flag is internal to
// the catalogue and is not exposed here.
func catalogSettingsView(ss []catalogSetting) []map[string]any {
	out := []map[string]any{}
	for _, s := range ss {
		m := map[string]any{"key": s.Key, "type": s.Type}
		if s.Label != "" {
			m["label"] = s.Label
		}
		if len(s.Options) > 0 {
			m["options"] = s.Options
		}
		if s.Min != nil {
			m["min"] = *s.Min
		}
		if s.Max != nil {
			m["max"] = *s.Max
		}
		out = append(out, m)
	}
	return out
}

func (d *daemon) barCatalog(asJSON bool) string {
	cat, err := loadCatalog(shellCatalogPath())
	if err != nil {
		return "err bar: " + err.Error()
	}
	plugins := barCapablePlugins(loadInstalledPlugins())
	entries := []map[string]any{}
	for _, w := range cat {
		entries = append(entries, map[string]any{
			"id": w.ID, "label": w.Label, "gloss": w.Gloss, "category": w.Category,
			"desc": w.Desc, "kind": "builtin", "official": true, "pluginDir": "",
			"settings": catalogSettingsView(w.Settings),
		})
	}
	for _, p := range plugins {
		entries = append(entries, map[string]any{
			"id": p.ID, "label": p.name(), "gloss": "", "category": "Plugin",
			"desc": p.desc(), "kind": "plugin", "official": p.official(),
			"author": p.author(), "version": p.version(), "pluginDir": p.Dir,
			"hosts": p.hosts(), "settings": p.settingsList(),
		})
	}
	if asJSON {
		b, _ := json.Marshal(entries)
		return string(b)
	}
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "%-12s %-8s %s\n", e["id"], e["kind"], e["label"])
	}
	return strings.TrimRight(b.String(), "\n")
}

func (d *daemon) barMove(args []string) string {
	if len(args) < 1 {
		return "err bar: move needs a widget id"
	}
	id := args[0]
	section := ""
	p := placement{}
	rest := args[1:]
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--section":
			i++
			if i >= len(rest) {
				return "err bar: --section needs a value"
			}
			section = rest[i]
		case "--index":
			i++
			if i >= len(rest) {
				return "err bar: --index needs a value"
			}
			n, err := strconv.Atoi(rest[i])
			if err != nil {
				return "err bar: --index must be an integer"
			}
			p.hasIndex, p.index = true, n
		case "--before":
			i++
			if i >= len(rest) {
				return "err bar: --before needs a value"
			}
			p.before = rest[i]
		case "--after":
			i++
			if i >= len(rest) {
				return "err bar: --after needs a value"
			}
			p.after = rest[i]
		default:
			return "err bar: unknown flag " + rest[i]
		}
	}
	if section == "" {
		return "err bar: move needs --section <left|center|right>"
	}
	if !validSection(section) {
		return "err bar: unknown section " + section
	}
	cat, err := loadCatalog(shellCatalogPath())
	if err != nil {
		return "err bar: " + err.Error()
	}
	plugins := barCapablePlugins(loadInstalledPlugins())
	known := knownWidgets(cat, plugins)
	if !knownContains(known, id) {
		return "err bar: unknown widget " + id
	}
	frame := d.barConfig()
	layout := normalizeLayout(parseLayout(frame), known)
	moved, err := layoutMove(layout, id, section, p)
	if err != nil {
		return "err bar: " + err.Error()
	}
	return d.patchLayout(moved)
}

func knownContains(known []knownWidget, id string) bool {
	for _, k := range known {
		if k.id == id {
			return true
		}
	}
	return false
}

func (d *daemon) patchLayout(l barLayout) string {
	if d.settings == nil {
		return "err bar: settings not ready"
	}
	if l.Left == nil {
		l.Left = []string{}
	}
	if l.Center == nil {
		l.Center = []string{}
	}
	if l.Right == nil {
		l.Right = []string{}
	}
	if l.Version == 0 {
		l.Version = 1
	}
	b, _ := json.Marshal(l)
	if err := d.settings.patch("qsbar.layout", b); err != nil {
		return "err bar: " + err.Error()
	}
	return "ok"
}

func (d *daemon) barShowHide(args []string, on bool) string {
	if len(args) < 1 {
		return "err bar: expected a widget id"
	}
	id := args[0]
	cat, err := loadCatalog(shellCatalogPath())
	if err != nil {
		return "err bar: " + err.Error()
	}
	if w, ok := findWidget(cat, id); ok {
		if w.VisKey == "" {
			return "err bar: " + id + " is always shown and has no visibility toggle"
		}
		if d.settings == nil {
			return "err bar: settings not ready"
		}
		v, _ := json.Marshal(on)
		if err := d.settings.patch("qsbar.widgets."+w.VisKey, v); err != nil {
			return "err bar: " + err.Error()
		}
		return "ok"
	}
	plugins := barCapablePlugins(loadInstalledPlugins())
	if _, ok := findPlugin(plugins, id); ok {
		return pluginShow(id, on)
	}
	return "err bar: unknown widget " + id
}

// pluginShow enables or disables a plugin on the bar through the shared
// placement writer: enable sets the topbarGlyph host too, so it lands on the bar
// the moment it goes live.
func pluginShow(id string, on bool) string {
	tool := pluginsPlaceTool()
	if on {
		if out, err := runPlace(tool, id, "enabled", "true"); err != nil {
			return "err bar: " + placeErr(out, err)
		}
		if out, err := runPlace(tool, id, "host", "topbarGlyph"); err != nil {
			return "err bar: " + placeErr(out, err)
		}
		return "ok"
	}
	if out, err := runPlace(tool, id, "enabled", "false"); err != nil {
		return "err bar: " + placeErr(out, err)
	}
	return "ok"
}

func runPlace(tool string, args ...string) (string, error) {
	out, err := exec.Command(tool, args...).CombinedOutput()
	return string(out), err
}

func placeErr(out string, err error) string {
	out = strings.TrimSpace(out)
	if out != "" {
		return "ryoku-plugins-place: " + out
	}
	return "ryoku-plugins-place: " + err.Error()
}

func (d *daemon) barSet(args []string) string {
	if len(args) < 3 {
		return "err bar: set needs <id> <key> <value>"
	}
	id, key := args[0], args[1]
	value := strings.Join(args[2:], " ")
	cat, err := loadCatalog(shellCatalogPath())
	if err != nil {
		return "err bar: " + err.Error()
	}
	if w, ok := findWidget(cat, id); ok {
		s, ok := findSetting(w, key)
		if !ok {
			return "err bar: unknown setting " + key + " for " + id
		}
		v, err := validateBuiltinSetting(s, value)
		if err != nil {
			return "err bar: " + err.Error()
		}
		if d.settings == nil {
			return "err bar: settings not ready"
		}
		path := "qsbar." + key
		if key == "clock12h" {
			path = "general.clock_format_24_h"
			var twelve bool
			if err := json.Unmarshal(v, &twelve); err != nil {
				return "err bar: " + err.Error()
			}
			v, _ = json.Marshal(!twelve)
		}
		if err := d.settings.patch(path, v); err != nil {
			return "err bar: " + err.Error()
		}
		return "ok"
	}
	plugins := barCapablePlugins(loadInstalledPlugins())
	if p, ok := findPlugin(plugins, id); ok {
		ms, ok := p.setting(key)
		if !ok {
			return "err bar: unknown setting " + key + " for " + id
		}
		v, err := validatePluginSetting(ms, value)
		if err != nil {
			return "err bar: " + err.Error()
		}
		obj, _ := json.Marshal(map[string]json.RawMessage{key: v})
		if out, err := runPlace(pluginsPlaceTool(), id, "settings", string(obj)); err != nil {
			return "err bar: " + placeErr(out, err)
		}
		return "ok"
	}
	return "err bar: unknown widget " + id
}

// validateBuiltinSetting checks a value against a catalogue setting row and
// returns the JSON to persist at qsbar.<key>.
func validateBuiltinSetting(s catalogSetting, raw string) (json.RawMessage, error) {
	switch s.Type {
	case "choice":
		if !inSet(s.Options, raw) {
			return nil, fmt.Errorf("%q is not one of: %s", raw, strings.Join(s.Options, ", "))
		}
		b, _ := json.Marshal(raw)
		return b, nil
	case "toggle":
		bv, ok := parseBoolArg(raw)
		if !ok {
			return nil, fmt.Errorf("%q is not a boolean (use on/off or true/false)", raw)
		}
		b, _ := json.Marshal(bv)
		return b, nil
	case "multi":
		parts := splitCSV(raw)
		for _, p := range parts {
			if !inSet(s.Options, p) {
				return nil, fmt.Errorf("%q is not one of: %s", p, strings.Join(s.Options, ", "))
			}
		}
		b, _ := json.Marshal(parts)
		return b, nil
	case "int":
		n, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return nil, fmt.Errorf("%q is not an integer", raw)
		}
		if s.Min != nil && n < *s.Min {
			return nil, fmt.Errorf("%d is below the minimum %d", n, *s.Min)
		}
		if s.Max != nil && n > *s.Max {
			return nil, fmt.Errorf("%d is above the maximum %d", n, *s.Max)
		}
		b, _ := json.Marshal(n)
		return b, nil
	case "text":
		b, _ := json.Marshal(raw)
		return b, nil
	}
	return nil, fmt.Errorf("unsupported setting type %q", s.Type)
}

// validatePluginSetting checks a value against a plugin manifest setting schema
// and returns the JSON to merge into plugins.json[id].settings.
func validatePluginSetting(ms map[string]any, raw string) (json.RawMessage, error) {
	typ, _ := ms["type"].(string)
	switch typ {
	case "toggle":
		bv, ok := parseBoolArg(raw)
		if !ok {
			return nil, fmt.Errorf("%q is not a boolean (use on/off or true/false)", raw)
		}
		b, _ := json.Marshal(bv)
		return b, nil
	case "choice":
		opts := pluginChoiceValues(ms["options"])
		if !inSet(opts, raw) {
			return nil, fmt.Errorf("%q is not one of: %s", raw, strings.Join(opts, ", "))
		}
		b, _ := json.Marshal(raw)
		return b, nil
	case "slider":
		f, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not a number", raw)
		}
		if mn, ok := numField(ms, "min"); ok && f < mn {
			return nil, fmt.Errorf("%g is below the minimum %g", f, mn)
		}
		if mx, ok := numField(ms, "max"); ok && f > mx {
			return nil, fmt.Errorf("%g is above the maximum %g", f, mx)
		}
		b, _ := json.Marshal(f)
		return b, nil
	case "int":
		n, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return nil, fmt.Errorf("%q is not an integer", raw)
		}
		b, _ := json.Marshal(n)
		return b, nil
	default:
		// text, image, colour and any future free-form type carry the string.
		b, _ := json.Marshal(raw)
		return b, nil
	}
}

// pluginChoiceValues reads the choosable values from a manifest choice setting's
// options, which are either {value,label} objects or plain strings.
func pluginChoiceValues(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return []string{}
	}
	out := []string{}
	for _, e := range arr {
		switch t := e.(type) {
		case string:
			out = append(out, t)
		case map[string]any:
			if val, ok := t["value"].(string); ok {
				out = append(out, val)
			}
		}
	}
	return out
}

func numField(m map[string]any, key string) (float64, bool) {
	f, ok := m[key].(float64)
	return f, ok
}

func parseBoolArg(s string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "on", "yes", "1":
		return true, true
	case "false", "off", "no", "0":
		return false, true
	}
	return false, false
}

func splitCSV(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (d *daemon) barPosition(args []string) string {
	if len(args) != 1 || (args[0] != "top" && args[0] != "bottom") {
		return "err bar: position expects top or bottom"
	}
	if d.settings == nil {
		return "err bar: settings not ready"
	}
	v, _ := json.Marshal(args[0])
	if err := d.settings.patch("qsbar.barPosition", v); err != nil {
		return "err bar: " + err.Error()
	}
	return "ok"
}

func (d *daemon) barForm(args []string) string {
	if len(args) != 1 || !barForms[args[0]] {
		return "err bar: form expects full|fit|dock|notch|islands"
	}
	if d.settings == nil {
		return "err bar: settings not ready"
	}
	v, _ := json.Marshal(args[0])
	if err := d.settings.patch("qsbar.barShellStyle", v); err != nil {
		return "err bar: " + err.Error()
	}
	return "ok"
}

func (d *daemon) barDefaults() string {
	cat, err := loadCatalog(shellCatalogPath())
	if err != nil {
		return "err bar: " + err.Error()
	}
	if d.settings == nil {
		return "err bar: settings not ready"
	}
	lb, _ := json.Marshal(layoutDefaults(cat))
	if err := d.settings.patch("qsbar.layout", lb); err != nil {
		return "err bar: " + err.Error()
	}
	vb, _ := json.Marshal(visibilityDefaults(cat))
	if err := d.settings.patch("qsbar.widgets", vb); err != nil {
		return "err bar: " + err.Error()
	}
	return "ok"
}

// barSettings raises QS Bar Settings on the active monitor through the qsbar
// IPC the shell exposes; an empty route opens the default route.
func (d *daemon) barSettings(args []string) string {
	route := ""
	if len(args) >= 1 {
		route = args[0]
	}
	argv := append(qsSelect("shell"), "ipc", "call", "qsbar", "settings", route)
	go func() { _ = exec.Command("qs", argv...).Run() }()
	return "ok"
}

// ── dock ────────────────────────────────────────────────────────────────────

// defaultDock is the shipped dock store, mirroring services/Config.qml so a
// partial dock object in shell.json never drops a field a mutation did not set.
func defaultDock() map[string]any {
	return map[string]any{
		"enabled": false, "edge": "auto", "autohide": true, "pinned": []string{},
		"magnify": true, "frost": true, "shadow": true, "labels": true, "media": false,
	}
}

// currentDock reads the dock store, preserving any unknown fields and filling in
// missing shipped defaults, so writing it back is always a complete object.
func (d *daemon) currentDock() map[string]any {
	out := map[string]any{}
	if dock, ok := d.barConfig()["dock"].(map[string]any); ok {
		for k, v := range dock {
			out[k] = v
		}
	}
	for k, v := range defaultDock() {
		if _, ok := out[k]; !ok {
			out[k] = v
		}
	}
	return out
}

func (d *daemon) dockCLI(args []string) string {
	if len(args) < 1 {
		return "err dock: expected show|hide|edge|autohide|pin|unpin|list"
	}
	switch args[0] {
	case "show":
		return d.dockSet("enabled", true)
	case "hide":
		return d.dockSet("enabled", false)
	case "edge":
		if len(args) != 2 || !dockEdges[args[1]] {
			return "err dock: edge expects auto|top|bottom|left|right"
		}
		return d.dockSet("edge", args[1])
	case "autohide":
		if len(args) != 2 || (args[1] != "on" && args[1] != "off") {
			return "err dock: autohide expects on|off"
		}
		return d.dockSet("autohide", args[1] == "on")
	case "pin":
		if len(args) != 2 {
			return "err dock: pin needs an app id"
		}
		return d.dockPin(args[1], true)
	case "unpin":
		if len(args) != 2 {
			return "err dock: unpin needs an app id"
		}
		return d.dockPin(args[1], false)
	case "list":
		return d.dockList(hasJSONFlag(args[1:]))
	}
	return "err dock: unknown verb " + args[0]
}

func (d *daemon) dockSet(key string, value any) string {
	if d.settings == nil {
		return "err dock: settings not ready"
	}
	dock := d.currentDock()
	dock[key] = value
	b, _ := json.Marshal(dock)
	if err := d.settings.patch("dock", b); err != nil {
		return "err dock: " + err.Error()
	}
	return "ok"
}

func (d *daemon) dockPin(app string, add bool) string {
	if d.settings == nil {
		return "err dock: settings not ready"
	}
	dock := d.currentDock()
	pinned := toStringSlice(dock["pinned"])
	if add {
		if inSet(pinned, app) {
			return "ok"
		}
		pinned = append(pinned, app)
	} else {
		if !inSet(pinned, app) {
			return "err dock: " + app + " is not pinned"
		}
		pinned = removeID(pinned, app)
	}
	dock["pinned"] = pinned
	b, _ := json.Marshal(dock)
	if err := d.settings.patch("dock", b); err != nil {
		return "err dock: " + err.Error()
	}
	return "ok"
}

func (d *daemon) dockList(asJSON bool) string {
	dock := d.currentDock()
	view := map[string]any{
		"enabled":  dock["enabled"],
		"edge":     dock["edge"],
		"autohide": dock["autohide"],
		"pinned":   toStringSlice(dock["pinned"]),
	}
	if asJSON {
		b, _ := json.Marshal(view)
		return string(b)
	}
	return fmt.Sprintf("enabled: %v\nedge: %v\nautohide: %v\npinned: %s",
		view["enabled"], view["edge"], view["autohide"], strings.Join(toStringSlice(view["pinned"]), ", "))
}
