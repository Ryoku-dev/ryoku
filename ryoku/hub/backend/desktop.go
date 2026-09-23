package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	wm "ryoku-wm"
)

// ryoku-hub desktop is the neutral window-manager settings backend. It owns
// ~/.config/ryoku/desktop.json (the single writer) and drives the active
// compositor through the wm seam: apply authors the compositor config, act does
// live changes. Nothing here knows which compositor is running.
//
//	desktop get                 the store overlaid on the provider defaults, JSON
//	desktop defaults            the provider's default subtree, JSON
//	desktop save <json>         persist the store, then apply + reload live
//	desktop preview <json>      push the draft live without persisting
//	desktop restore             reload the saved config live
//	desktop set-rebind <d> <c>  patch one keybind remap (for non-Hub apps)
//	desktop cursors|layouts     enumerate installed cursor themes / kb layouts
//	desktop variants <layout>   kb variants for a layout
//	desktop plugins <verb>      the compositor plugin subsystem

func desktopStorePath() string { return filepath.Join(ryokuConfigDir(), "desktop.json") }

// hyprConfigDir is the compositor config tree the import layerer still writes a
// user.lua block into; the generated config is the provider's.
func hyprConfigDir() string { return filepath.Join(configHome(), "hypr") }

var (
	wmOnce   sync.Once
	wmClient *wm.Client
)

func desktopClient() *wm.Client {
	wmOnce.Do(func() { wmClient = wm.Open() })
	return wmClient
}

// wmSplit is the store's compositor split, read from the provider's namespaced
// `defaults` once: the wm.<provider> namespace name and the sections that live
// under it. Rice and import address the store by flat section name; this routes
// each exclusive into that namespace so it never collides with a GUI save, and
// leaves everything else under desktop. The split is the provider's to define,
// so it is derived rather than hardcoded here.
var (
	wmSplitOnce sync.Once
	wmSplitNS   string
	wmSplitSet  map[string]bool
)

func wmSplit() (string, map[string]bool) {
	wmSplitOnce.Do(func() {
		wmSplitSet = map[string]bool{}
		b, err := desktopClient().Defaults()
		if err != nil {
			return
		}
		var ns struct {
			WM map[string]map[string]json.RawMessage `json:"wm"`
		}
		if json.Unmarshal(b, &ns) != nil {
			return
		}
		for name, sections := range ns.WM {
			wmSplitNS = name
			for k := range sections {
				wmSplitSet[k] = true
			}
		}
	})
	return wmSplitNS, wmSplitSet
}

func runDesktop(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("desktop needs get|defaults|schema|unhonored|save|preview|restore|set-rebind|cursors|layouts|variants|plugins|scheme|anim-preset")
	}
	switch args[0] {
	case "get":
		return getDesktop()
	case "defaults":
		return forwardDefaults()
	case "schema":
		return forwardSchema()
	case "unhonored":
		return forwardUnhonored()
	case "save":
		if len(args) < 2 {
			return fmt.Errorf("desktop save needs a JSON argument")
		}
		return saveDesktop(args[1])
	case "preview":
		if len(args) < 2 {
			return fmt.Errorf("desktop preview needs a JSON argument")
		}
		return previewDesktop(args[1])
	case "restore":
		restoreDesktop()
		return nil
	case "set-rebind":
		if len(args) < 2 {
			return fmt.Errorf("desktop set-rebind needs a default chord")
		}
		chosen := ""
		if len(args) > 2 {
			chosen = args[2]
		}
		return setRebind(args[1], chosen)
	case "plugins":
		return pluginsDesktop(args[1:])
	case "cursors":
		return printJSON(listCursorThemes())
	case "layouts":
		return printJSON(listKbLayouts())
	case "variants":
		if len(args) < 2 {
			return fmt.Errorf("desktop variants needs a layout code")
		}
		return printJSON(listKbVariants(args[1]))
	case "scheme":
		if len(args) < 2 {
			return printJSON(map[string]string{"scheme": currentScheme()})
		}
		return applyScheme(args[1])
	case "ryoku-theme":
		return applyRyokuTheme()
	case "theme-apps":
		if len(args) < 2 {
			return printJSON(map[string]bool{"themeApps": currentThemeApps()})
		}
		return applyThemeApps(args[1] == "on" || args[1] == "true")
	case "gtk-theme":
		if len(args) < 2 {
			return printJSON(map[string]string{"gtkTheme": currentGtkTheme()})
		}
		return applyGtkTheme(args[1])
	case "gnome-accent":
		if len(args) < 2 {
			return printJSON(map[string]bool{"gnomeAccent": currentGnomeAccent()})
		}
		return applyGnomeAccent(args[1] == "on" || args[1] == "true")
	case "matugen":
		return runMatugenCmd(args[1:])
	case "anim-preset":
		if len(args) < 2 {
			return printJSON(map[string]string{"preset": currentAnimPreset()})
		}
		return applyAnimPreset(args[1])
	default:
		return fmt.Errorf("unknown desktop subcommand: %s", args[0])
	}
}

// getDesktop returns the stored values overlaid on the provider defaults, so
// every leaf the GUI shows is present even before a save.
func getDesktop() error {
	merged := effectiveStore()
	return printJSON(merged)
}

// effectiveStore is the user's desktop.json deep-merged over the provider
// defaults.
func effectiveStore() map[string]any {
	base := map[string]any{}
	if b, err := desktopClient().Defaults(); err == nil {
		_ = json.Unmarshal(b, &base)
	}
	return jsonMerge(base, readJSONMap(desktopStorePath()))
}

// forwardDefaults hands the provider's default subtree straight through; the Hub
// does not reason about its shape.
func forwardDefaults() error {
	b, err := desktopClient().Defaults()
	if err != nil {
		return err
	}
	os.Stdout.Write(b)
	if len(b) == 0 || b[len(b)-1] != '\n' {
		fmt.Println()
	}
	return nil
}

// forwardSchema hands the active provider's exclusive settings rows through so
// the Hub's window-manager page draws them without knowing which compositor
// authored them. An empty array when no provider answers, so the page stays
// quiet rather than erroring.
func forwardSchema() error {
	if !desktopClient().Available() {
		return printJSON([]json.RawMessage{})
	}
	rows, err := desktopClient().Schema()
	if err != nil {
		return err
	}
	return printJSON(rows)
}

// forwardUnhonored hands the active provider's "cannot honour" list through for
// the window-manager page's cannot-do section: the settings a dry-run apply of
// the current store would drop, the same losses a switch preview shows. Ungated
// and store-driven; empty when no provider answers.
func forwardUnhonored() error {
	if !desktopClient().Available() {
		return printJSON([]wm.Unhonored{})
	}
	rep, err := desktopClient().DryRun(desktopStorePath())
	if err != nil {
		return err
	}
	out := rep.Unhonored
	if out == nil {
		out = []wm.Unhonored{}
	}
	return printJSON(out)
}

// saveDesktop persists the draft the GUI sent and applies it live.
func saveDesktop(raw string) error {
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return fmt.Errorf("parse desktop JSON: %w", err)
	}
	if m == nil {
		return fmt.Errorf("desktop settings must be a JSON object")
	}
	return withDesktopLock(func() error {
		before, _ := effectiveCursor()
		if err := atomicWrite(desktopStorePath(), mustJSON(m), 0o644); err != nil {
			return err
		}
		os.Remove(desktopPreviewPath())
		if err := applyLive(); err != nil {
			return err
		}
		after, _ := effectiveCursor()
		// Enabling the wallpaper-following pointer is the one save that needs
		// more than a reload: recolour to the live accent right away, so the
		// pointer follows matugen from the moment it is turned on rather than
		// showing the packaged fallback until the next palette change. The
		// full build rasterises eleven sizes, so it runs off the save path.
		if after == wm.CursorThemeMaterial && before != after {
			go func() { _ = exec.Command("ryoku-cursor-material-recolor", "--force", "--full").Run() }()
		}
		return nil
	})
}

// previewDesktop pushes the draft live via the provider without persisting; a
// compositor whose config is file-only simply does nothing.
func previewDesktop(raw string) error {
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return fmt.Errorf("parse desktop JSON: %w", err)
	}
	if m == nil {
		return fmt.Errorf("desktop settings must be a JSON object")
	}
	f, err := os.CreateTemp("", "ryoku-desktop-preview-*.json")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(mustJSON(m)); err != nil {
		f.Close()
		return err
	}
	f.Close()
	if _, err := desktopClient().Preview(tmp); err != nil {
		return err
	}
	// Record the live draft so the shell daemon can land it again after a
	// config reload; save/restore clear it. The owner is the calling Hub, not
	// this short-lived CLI: the Hub spawns it as a child, so the parent is the
	// process whose life the preview depends on. Once that Hub quits, the next
	// reload drops the marker and reverts to disk -- an unsaved quit still
	// means unsaved.
	b, _ := json.Marshal(map[string]any{"pid": os.Getppid(), "draft": m})
	_ = os.MkdirAll(ryokuConfigDir(), 0o755)
	_ = os.WriteFile(desktopPreviewPath(), b, 0o600)
	return nil
}

// desktopPreviewPath is the hand-off between the Hub's live preview and the
// shell daemon's palette reloads. A preview is live-only state: it writes no
// config, so a config-only reload re-reads disk and silently drops it. The
// daemon re-asserts the draft after every such reload while the Hub that owns
// it is alive.
func desktopPreviewPath() string { return filepath.Join(ryokuConfigDir(), ".desktop-preview.json") }

// restoreDesktop reverts the live session to the saved config: a reload resets
// every keyword, and the cursor is re-asserted since it is imperative state a
// reload leaves alone.
func restoreDesktop() {
	os.Remove(desktopPreviewPath())
	c := desktopClient()
	_ = c.Act(wm.ActionConfigReload)
	setLiveCursorFromStore()
}

// setRebind patches one entry of desktop.keybindRebinds under the file lock, then
// applies, so a separate app (ryoshot) can change its launch chord without
// becoming a second writer of the store.
func setRebind(def, chosen string) error {
	def = strings.TrimSpace(def)
	chosen = strings.TrimSpace(chosen)
	if def == "" {
		return fmt.Errorf("set-rebind: empty default chord")
	}
	return withDesktopLock(func() error {
		m := readJSONMap(desktopStorePath())
		d := childMap(m, "desktop")
		rb, _ := d["keybindRebinds"].(map[string]any)
		if rb == nil {
			rb = map[string]any{}
		}
		if chosen == "" || chosen == def {
			delete(rb, def)
		} else {
			rb[def] = chosen
		}
		if len(rb) == 0 {
			delete(d, "keybindRebinds")
		} else {
			d["keybindRebinds"] = rb
		}
		m["desktop"] = d
		if err := atomicWrite(desktopStorePath(), mustJSON(m), 0o644); err != nil {
			return err
		}
		return applyDesktop()
	})
}

// pluginsDesktop forwards to the provider's plugin subsystem. remove reports the
// store key it dropped; the Hub, the single writer, deletes it from desktop.json.
func pluginsDesktop(args []string) error {
	b, err := desktopClient().Plugins(args...)
	if err != nil {
		return err
	}
	if len(args) > 0 && args[0] == "remove" {
		var r struct {
			StoreKey string `json:"storeKey"`
		}
		if json.Unmarshal(b, &r) == nil && r.StoreKey != "" {
			_ = withDesktopLock(func() error {
				m := readJSONMap(desktopStorePath())
				if deleteDottedPath(m, r.StoreKey) {
					if err := atomicWrite(desktopStorePath(), mustJSON(m), 0o644); err != nil {
						return err
					}
					return applyDesktop()
				}
				return nil
			})
		}
	}
	os.Stdout.Write(b)
	if len(b) == 0 || b[len(b)-1] != '\n' {
		fmt.Println()
	}
	return nil
}

// --- provider facade ------------------------------------------------------

// applyDesktop re-authors the compositor config from desktop.json and reloads.
// The common "regenerate live" path for the theme, rice and import writers.
func applyDesktop() error {
	c := desktopClient()
	rep, err := c.Apply(desktopStorePath())
	if err != nil {
		return err
	}
	if rep.ReloadNeeded {
		_ = c.Act(wm.ActionConfigReload)
	}
	return nil
}

// reloadDesktop reloads the compositor config in place.
func reloadDesktop() { _ = desktopClient().Act(wm.ActionConfigReload) }

// applyLive is the save path: author + reload, reconcile the plugin set, and
// re-assert the cursor (imperative state a reload does not restore).
func applyLive() error {
	if err := applyDesktop(); err != nil {
		return err
	}
	_, _ = desktopClient().Plugins("sync")
	setLiveCursorFromStore()
	return nil
}

func setLiveCursor(theme string, size int) {
	if strings.TrimSpace(theme) == "" {
		return
	}
	_ = desktopClient().Act(wm.ActionCursorSet, theme, strconv.Itoa(size))
}

func setLiveCursorFromStore() {
	theme, size := effectiveCursor()
	setLiveCursor(theme, size)
}

// effectiveCursor reads the cursor theme and size the store resolves to, over
// the provider defaults.
func effectiveCursor() (string, int) {
	m := effectiveStore()
	d, _ := m["desktop"].(map[string]any)
	c, _ := d["cursor"].(map[string]any)
	theme, _ := c["theme"].(string)
	size := 24
	if s, ok := c["size"].(float64); ok {
		size = int(s)
	}
	return wm.ResolveCursorTheme(theme), size
}

// staticThemeActive reports whether shell.json names a fixed catalog palette
// (not the dynamic Default/Wallpaper variants). Rice reads it to decide whether
// a fixed-colour rice must switch the shell off Wallpaper.
func staticThemeActive() bool {
	b, err := os.ReadFile(shellStorePath())
	if err != nil {
		return false
	}
	var s struct {
		Theme struct {
			Theme string `json:"theme"`
		} `json:"theme"`
	}
	if json.Unmarshal(b, &s) != nil {
		return false
	}
	switch s.Theme.Theme {
	case "", "Default", "Wallpaper":
		return false
	}
	return true
}

// --- store plumbing -------------------------------------------------------

// jsonMerge deep-merges over onto base: maps recurse, everything else (arrays,
// scalars) replaces. Neither input is mutated.
func jsonMerge(base, over map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range over {
		if bv, ok := out[k].(map[string]any); ok {
			if ov, ok := v.(map[string]any); ok {
				out[k] = jsonMerge(bv, ov)
				continue
			}
		}
		out[k] = v
	}
	return out
}

// childMap returns m[key] as a map, creating it when absent or the wrong type.
func childMap(m map[string]any, key string) map[string]any {
	if c, ok := m[key].(map[string]any); ok {
		return c
	}
	c := map[string]any{}
	m[key] = c
	return c
}

// deleteDottedPath removes the leaf at a dotted path (e.g.
// "wm.hyprland.plugins.extra.foo"), reporting whether anything was there.
func deleteDottedPath(m map[string]any, path string) bool {
	parts := strings.Split(path, ".")
	cur := m
	for _, p := range parts[:len(parts)-1] {
		next, ok := cur[p].(map[string]any)
		if !ok {
			return false
		}
		cur = next
	}
	leaf := parts[len(parts)-1]
	if _, ok := cur[leaf]; !ok {
		return false
	}
	delete(cur, leaf)
	return true
}

// readHyprSections reads desktop.json as a flat section map, merging desktop.*
// and every wm.<provider>.* namespace (agnostic, like the provider's reader).
func readHyprSections() map[string]any {
	ns := readJSONMap(desktopStorePath())
	flat := map[string]any{}
	if d, ok := ns["desktop"].(map[string]any); ok {
		for k, v := range d {
			flat[k] = v
		}
	}
	if w, ok := ns["wm"].(map[string]any); ok {
		for _, sub := range w {
			if m, ok := sub.(map[string]any); ok {
				for k, v := range m {
					flat[k] = v
				}
			}
		}
	}
	return flat
}

// putHyprSection files one flat section into its namespace within ns: an
// exclusive into wm.<provider>, everything else into desktop.
func putHyprSection(ns map[string]any, section string, v any) {
	if name, set := wmSplit(); name != "" && set[section] {
		childMap(childMap(ns, "wm"), name)[section] = v
		return
	}
	childMap(ns, "desktop")[section] = v
}

// overlayHyprSections overlays the allowlisted flat sections from src into
// desktop.json, each in its namespace, leaving every other key untouched.
func overlayHyprSections(src map[string]any, allow []string) error {
	ns := readJSONMap(desktopStorePath())
	set := func(k string, v any) { putHyprSection(ns, k, v) }
	if allow == nil {
		for k, v := range src {
			set(k, v)
		}
	} else {
		for _, k := range allow {
			if v, ok := src[k]; ok {
				set(k, v)
			}
		}
	}
	return atomicWrite(desktopStorePath(), mustJSON(ns), 0o644)
}

// setHyprSections files each named flat section into its namespace and writes.
func setHyprSections(sections map[string]any) error {
	ns := readJSONMap(desktopStorePath())
	for k, v := range sections {
		putHyprSection(ns, k, v)
	}
	return atomicWrite(desktopStorePath(), mustJSON(ns), 0o644)
}

// withDesktopLock serializes read-modify-write of desktop.json against the GUI
// save, so a concurrent write cannot drop the other's change.
func withDesktopLock(fn func() error) error {
	_ = os.MkdirAll(ryokuConfigDir(), 0o755)
	f, err := os.OpenFile(filepath.Join(ryokuConfigDir(), ".desktop.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fn()
	}
	defer f.Close()
	if syscall.Flock(int(f.Fd()), syscall.LOCK_EX) == nil {
		defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	}
	return fn()
}

func printJSON(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	os.Stdout.Write(b)
	fmt.Println()
	return nil
}

func atomicWrite(path string, b []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(b); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Chmod(mode); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// anim-preset: the active animation personality. A plain file the animation
// module reads at load; set writes it and reloads.
var animPresets = map[string]bool{
	"ryoku": true, "dusky": true, "bounce": true, "fade": true, "fast": true,
	"mechanical": true, "minimal": true, "rage": true, "slowmotion": true,
	"air": true, "hallucination": true, "exaggerated": true, "disable": true,
}

func animPresetPath() string { return filepath.Join(ryokuConfigDir(), "anim-preset") }

func currentAnimPreset() string {
	b, err := os.ReadFile(animPresetPath())
	if err != nil {
		return "ryoku"
	}
	name := strings.TrimSpace(string(b))
	if !animPresets[name] {
		return "ryoku"
	}
	return name
}

func applyAnimPreset(name string) error {
	if !animPresets[name] {
		return fmt.Errorf("unknown animation preset %q", name)
	}
	if err := atomicWrite(animPresetPath(), []byte(name+"\n"), 0o644); err != nil {
		return err
	}
	reloadDesktop()
	return nil
}

// freshCaps probes the provider without the shared client's process-lifetime
// cache. The import paths run once per invocation and must see the provider that
// matches the current environment, not whatever the first probe in this process
// happened to resolve.
func freshCaps() (wm.Caps, error) {
	d := wm.Detect()
	if d.Name == "" {
		return wm.Caps{}, wm.ErrNoProvider
	}
	return wm.OpenNamed(d.Name).Caps()
}

// generatedConfigWrites lists the provider-authored config files with their
// CURRENT contents, so a caller can hand them to backup() and have an undo
// restore them exactly. The write itself is a no-op; the point is the backup.
// A file that does not exist yet is included with no content, which records an
// empty backup and makes undo remove it.
func generatedConfigWrites() []pendingWrite {
	caps, err := freshCaps()
	if err != nil {
		return nil
	}
	base := configHome()
	out := make([]pendingWrite, 0, len(caps.GeneratedFiles))
	for _, rel := range caps.GeneratedFiles {
		path := filepath.Join(base, rel)
		// An absent file is recorded with no content on purpose: backup() then
		// stores an empty backup and undo removes it, which is the pre-import
		// state. applyDesktop authors the real content immediately after.
		data, _ := os.ReadFile(path)
		out = append(out, pendingWrite{path: path, content: data})
	}
	return out
}

// clearGeneratedConfig removes what the provider's apply authors. Used when a
// restore leaves no store: the emitted config is a pure function of the store,
// so an absent store means absent config, not defaults.
func clearGeneratedConfig() {
	caps, err := freshCaps()
	if err != nil {
		return
	}
	base := configHome()
	for _, rel := range caps.GeneratedFiles {
		_ = os.Remove(filepath.Join(base, rel))
	}
}
