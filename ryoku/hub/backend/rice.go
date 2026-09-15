package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"ryostore/compat"
)

// rice = a named, versioned look document for the whole desktop. it captures a
// curated view of the four user-owned stores under ~/.config/ryoku (hypr.json,
// shell.json, theme.json, launcher.json) plus assets (wallpaper, launcher hero,
// cursor, fonts), so a whole desktop look can be saved, applied, shared, and
// reverted in one click. see docs/superpowers/specs (local).
//
// capture and apply are uniform per-store map overlays gated by a per-store key
// allowlist: a capture pulls only look-defining keys, an apply sets only those
// keys and leaves everything else the recipient has alone (their keybinds,
// input, machine state never move unless a behavior layer is opted into).

const riceSchema = 1

type RiceColor struct {
	Mode      string `json:"mode"`                // "wallpaper" | "fixed"
	Palette   string `json:"palette,omitempty"`   // relative file, when Mode == "fixed"
	ThemeApps *bool  `json:"themeApps,omitempty"` // do external GTK / GUI apps follow this look
}

type RiceAssets struct {
	Wallpaper string   `json:"wallpaper,omitempty"`
	Hero      string   `json:"hero,omitempty"`
	Cursor    string   `json:"cursor,omitempty"`
	Fonts     []string `json:"fonts,omitempty"`
	Fastfetch string   `json:"fastfetch,omitempty"`
	// FastfetchStyle is the bundled config.jsonc: the readout's layout and
	// modules, distinct from the emblem image above.
	FastfetchStyle string `json:"fastfetchStyle,omitempty"`
	// Lock is the active qylock skin slug (e.g. "clockwork/orbital"); the skin
	// installs separately, so a rice travels the choice, not the files.
	Lock string `json:"lock,omitempty"`
}

type Rice struct {
	Schema      int                        `json:"schema"`
	Slug        string                     `json:"slug"`
	Name        string                     `json:"name"`
	Author      string                     `json:"author,omitempty"`
	Blurb       string                     `json:"blurb,omitempty"`
	Tags        []string                   `json:"tags,omitempty"`
	CreatedWith string                     `json:"createdWith,omitempty"`
	Color       RiceColor                  `json:"color"`
	Assets      RiceAssets                 `json:"assets"`
	Look        map[string]map[string]any  `json:"look"`
	Layers      map[string]json.RawMessage `json:"layers,omitempty"`
}

func ryokuConfigDir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "ryoku")
}

func ricesDir() string            { return filepath.Join(ryokuConfigDir(), "rices") }
func shellStorePath() string      { return filepath.Join(ryokuConfigDir(), "shell.json") }
func launcherStorePath() string   { return filepath.Join(ryokuConfigDir(), "launcher.json") }
func widgetsStorePath() string    { return filepath.Join(ryokuConfigDir(), "widgets.json") }
func visualizerStorePath() string { return filepath.Join(ryokuConfigDir(), "visualizer.json") }
func decorStorePath() string      { return filepath.Join(ryokuConfigDir(), "decor.json") }
func brandStorePath() string      { return filepath.Join(ryokuConfigDir(), "brand.json") }
func ryogamiStorePath() string    { return filepath.Join(ryokuConfigDir(), "ryogami.json") }
func ricePath(slug string) string {
	return filepath.Join(ricesDir(), slug, "rice.json")
}

// A rice captures the whole desktop look. hypr.json splits at the top level:
// the look sections travel always, the behavior sections are opt-in layers
// ("all" opts into every one). shell.json and launcher.json capture whole
// except a small denylist of regional / personal / per-machine keys, so a new
// look key travels automatically and a snapshot never silently drops what it
// cannot name. widgets/visualizer/decor hold nothing personal and travel whole;
// brand is identity and travels as an opt-in layer.
var riceHyprLook = []string{"appearance", "cursor", "anim", "plugins", "dwindle", "master"}
var riceHyprLayers = []string{"input", "windowRules", "layerRules", "appOverrides", "keybinds", "autostart", "env", "apps"}

// layers that live outside hypr.json; routed to their own store on apply.
var riceExtraLayers = []string{"brand"}

// riceShellOmit / riceLauncherOmit are the keys held back from the whole-store
// shell / launcher capture: the author's weather city and unit, locale, UI
// language, the units/weather block, and per-monitor scaling never travel, and
// the launcher hero travels as a bundled asset rather than an absolute path.
var riceShellOmit = []string{"weatherLocation", "weatherUnit", "formatLocale", "language", "general", "displays"}
var riceLauncherOmit = []string{"weatherUnit", "heroImage"}

// readJSONMap reads a store file into a generic map; a missing or torn file
// reads as an empty map so an overlay still lands on a fresh key set.
func readJSONMap(path string) map[string]any {
	m := map[string]any{}
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &m); err != nil || m == nil {
			return map[string]any{}
		}
	}
	return m
}

// pick copies the allowlisted keys present in src into a fresh map; a nil
// allowlist copies every key (the whole-store looks).
func pick(src map[string]any, allow []string) map[string]any {
	out := map[string]any{}
	if allow == nil {
		for k, v := range src {
			out[k] = v
		}
		return out
	}
	for _, k := range allow {
		if v, ok := src[k]; ok {
			out[k] = v
		}
	}
	return out
}

// omit copies every key of src except the denied ones into a fresh map: the
// whole-store look, minus the regional / personal / machine keys a rice holds
// back. The inverse of pick's allowlist.
func omit(src map[string]any, deny []string) map[string]any {
	out := map[string]any{}
	for k, v := range src {
		out[k] = v
	}
	for _, k := range deny {
		delete(out, k)
	}
	return out
}

// overlayStore sets the allowlisted keys from src into the store file, leaves
// every other key untouched, and writes atomically. unknown keys in src are
// ignored, so a rice built on an older schema can never inject a retired key;
// a nil allowlist overlays every key (the whole-store looks).
func overlayStore(path string, src map[string]any, allow []string) error {
	cur := readJSONMap(path)
	if allow == nil {
		for k, v := range src {
			cur[k] = v
		}
	} else {
		for _, k := range allow {
			if v, ok := src[k]; ok {
				cur[k] = v
			}
		}
	}
	return atomicWrite(path, mustJSON(cur), 0o644)
}

func extractStore(path string, allow []string) map[string]any {
	return pick(readJSONMap(path), allow)
}

func loadRice(slug string) (Rice, string, error) {
	var r Rice
	dir := filepath.Join(ricesDir(), slug)
	b, err := os.ReadFile(filepath.Join(dir, "rice.json"))
	if err != nil {
		return r, dir, err
	}
	err = json.Unmarshal(b, &r)
	return r, dir, err
}

func saveRice(r Rice) error {
	return atomicWrite(ricePath(r.Slug), mustJSON(r), 0o644)
}

// listRices returns every user rice, sorted by slug for a stable UI. the
// reserved backup slots (.baseline / .previous) and any dotdir are skipped.
func listRices() []Rice {
	entries, _ := os.ReadDir(ricesDir())
	out := []Rice{}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if r, _, err := loadRice(e.Name()); err == nil {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

func slugify(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
		} else if b.Len() > 0 && !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func currentUser() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return ""
}

// ryokuVersion is the running Ryoku base version (the createdWith tag). Best
// effort: an empty string when the ryoku CLI is not on PATH (e.g. tests).
func ryokuVersion() string {
	out, err := exec.Command("ryoku", "version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.TrimSpace(string(out)), "v")
}

func allowed(k string, allow []string) bool {
	for _, a := range allow {
		if a == k {
			return true
		}
	}
	return false
}

func isFile(p string) bool {
	if p == "" {
		return false
	}
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return atomicWrite(dst, b, 0o644)
}

func stateHome() string {
	if b := os.Getenv("XDG_STATE_HOME"); b != "" {
		return b
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "state")
}

// currentWallpaper reads the path the shell recorded for the live wallpaper.
func currentWallpaper() string {
	b, err := os.ReadFile(filepath.Join(stateHome(), "ryoku-wallpaper"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func launcherHero() string {
	if h, ok := readJSONMap(launcherStorePath())["heroImage"].(string); ok {
		return h
	}
	return ""
}

// isVideo mirrors the shell's wallpaper routing: these extensions play through
// the live wallpaper daemon, everything else is a still.
func isVideo(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".mp4", ".webm", ".mkv", ".mov":
		return true
	}
	return false
}

func liveWallDir() string { return filepath.Join(os.Getenv("HOME"), "Pictures", "livewalls") }

// previewFrameOffset: seconds into a video wallpaper worth screenshotting,
// from the same per-video ryowalls tune the shell samples its palette at, so
// the rice preview shows the frame the user actually tuned the look around.
func previewFrameOffset(video string) string {
	b, err := os.ReadFile(filepath.Join(stateHome(), "ryoku-ryowalls.json"))
	if err != nil {
		return "1"
	}
	var t struct {
		Image string  `json:"image"`
		Frame float64 `json:"frame"`
	}
	if json.Unmarshal(b, &t) == nil && t.Image == video && t.Frame > 0 {
		return strconv.FormatFloat(t.Frame, 'f', 2, 64)
	}
	return "1"
}

// writeRicePreview renders preview.png beside the manifest: the wallpaper as
// it was the moment the rice was saved. an image is scaled down (a tile never
// needs more than ~1280 wide); a video wallpaper contributes its tuned frame,
// since an <Image> cannot paint an mp4. best-effort: without ffmpeg an image
// is copied whole and a video rice falls back to the tile silhouette.
func writeRicePreview(dir, wall string) {
	out := filepath.Join(dir, "preview.png")
	if isVideo(wall) {
		_ = exec.Command("ffmpeg", "-y", "-ss", previewFrameOffset(wall), "-i", wall,
			"-frames:v", "1", "-vf", "scale=1280:-2", out).Run()
		return
	}
	if exec.Command("ffmpeg", "-y", "-i", wall, "-frames:v", "1", "-vf", "scale=1280:-2", out).Run() != nil {
		_ = copyFile(wall, out)
	}
}

// validAssetName gates rice://-referenced files to plain names inside the rice
// folder, so a hostile manifest cannot point an asset copy outside it.
func validAssetName(n string) bool {
	return n != "" && !strings.HasPrefix(n, ".") && !strings.ContainsAny(n, "/\\")
}

// bundleDecorAssets copies each decor's picture into the rice and marks its
// src rice://<asset>, so the decors travel with the rice instead of pointing
// at files that exist only on the author's disk.
func bundleDecorAssets(dir string, decor map[string]any) {
	for key, v := range decor {
		ent, ok := v.(map[string]any)
		if !ok {
			continue
		}
		src, _ := ent["src"].(string)
		p := strings.TrimPrefix(src, "file://")
		if p == "" || !isFile(p) {
			continue
		}
		asset := "decor-" + slugify(key) + filepath.Ext(p)
		if copyFile(p, filepath.Join(dir, asset)) == nil {
			ent["src"] = "rice://" + asset
		}
	}
}

// rehydrateDecorAssets lands rice://-bundled decor pictures under the config
// dir and points the entries at the copies, so an applied rice never
// references the rices folder itself (deleting a rice later must not blank
// the live desktop). a missing or invalid asset clears the picture; the decor
// still applies.
func rehydrateDecorAssets(riceDir, slug string, decor map[string]any) {
	out := filepath.Join(ryokuConfigDir(), "rice-assets", slug)
	for _, v := range decor {
		ent, ok := v.(map[string]any)
		if !ok {
			continue
		}
		src, _ := ent["src"].(string)
		if !strings.HasPrefix(src, "rice://") {
			continue
		}
		name := strings.TrimPrefix(src, "rice://")
		dst := filepath.Join(out, name)
		if validAssetName(name) && isFile(filepath.Join(riceDir, name)) &&
			copyFile(filepath.Join(riceDir, name), dst) == nil {
			ent["src"] = "file://" + dst
		} else {
			ent["src"] = ""
		}
	}
}

// rehydrateRiceAsset copies a rice://-referenced file out of the rice folder
// into rice-assets/<slug>/ and returns where it landed. ok is false when ref is
// not a rice:// reference or the copy fails, so the caller can drop the field.
func rehydrateRiceAsset(riceDir, slug, ref string) (string, bool) {
	if !strings.HasPrefix(ref, "rice://") {
		return "", false
	}
	name := strings.TrimPrefix(ref, "rice://")
	dst := filepath.Join(ryokuConfigDir(), "rice-assets", slug, name)
	if validAssetName(name) && isFile(filepath.Join(riceDir, name)) &&
		copyFile(filepath.Join(riceDir, name), dst) == nil {
		return dst, true
	}
	return "", false
}

// rehydrateBrandAssets is the brand layer's counterpart: markImage and the
// reload-cover asset are bare paths in brand.json, so each copy lands as one.
func rehydrateBrandAssets(riceDir, slug string, brand map[string]any) {
	if src, _ := brand["markImage"].(string); strings.HasPrefix(src, "rice://") {
		if dst, ok := rehydrateRiceAsset(riceDir, slug, src); ok {
			brand["markImage"] = dst
		} else {
			delete(brand, "markImage")
		}
	}
	// reloadCover.path is a bare path nested one level down. A rice:// one is
	// bundled, so land it beside the mark; a foreign absolute path (an older
	// rice, or one authored before covers travelled) resolves to nothing on
	// this box, so drop the block and let the reload fall back to the default
	// cover instead of a broken one.
	if rc, ok := brand["reloadCover"].(map[string]any); ok {
		src, _ := rc["path"].(string)
		if strings.HasPrefix(src, "rice://") {
			if dst, ok := rehydrateRiceAsset(riceDir, slug, src); ok {
				rc["path"] = dst
				brand["reloadCover"] = rc
			} else {
				delete(brand, "reloadCover")
			}
		} else if src != "" && !isFile(src) {
			delete(brand, "reloadCover")
		}
	}
}

// captureRice snapshots the live look (plus the requested behavior layers) into
// a new user rice, bundling the wallpaper, launcher hero, and (for a locked
// palette) the cached colours. cursor and fonts travel by name.
func captureRice(name string, layers []string) (Rice, error) {
	slug := slugify(name)
	if slug == "" {
		return Rice{}, fmt.Errorf("a rice needs a name")
	}
	hy := readHyprSections()
	r := Rice{
		Schema:      riceSchema,
		Slug:        slug,
		Name:        strings.TrimSpace(name),
		Author:      currentUser(),
		CreatedWith: ryokuVersion(),
		Look: map[string]map[string]any{
			"hypr":       pick(hy, riceHyprLook),
			"shell":      omit(readJSONMap(shellStorePath()), riceShellOmit),
			"launcher":   omit(readJSONMap(launcherStorePath()), riceLauncherOmit),
			"widgets":    extractStore(widgetsStorePath(), nil),
			"visualizer": extractStore(visualizerStorePath(), nil),
			"decor":      extractStore(decorStorePath(), nil),
			"theme":      readJSONMap(themeStatePath()),
			"ryogami":    pick(readJSONMap(ryogamiStorePath()), []string{"matugen"}),
		},
	}
	if len(layers) == 1 && layers[0] == "all" {
		layers = append(append([]string{}, riceHyprLayers...), riceExtraLayers...)
	}
	if len(layers) > 0 {
		r.Layers = map[string]json.RawMessage{}
		for _, l := range layers {
			if allowed(l, riceHyprLayers) {
				v, ok := hy[l]
				if !ok || isEmptyLayer(v) {
					continue
				}
				if b, err := json.Marshal(v); err == nil {
					r.Layers[l] = b
				}
				continue
			}
			if l == "brand" {
				if bm := readJSONMap(brandStorePath()); len(bm) > 0 {
					if b, err := json.Marshal(bm); err == nil {
						r.Layers[l] = b
					}
				}
			}
		}
		if len(r.Layers) == 0 {
			r.Layers = nil
		}
	}
	if cur, ok := hy["cursor"].(map[string]any); ok {
		if t, ok := cur["theme"].(string); ok {
			r.Assets.Cursor = t
		}
	}
	st := loadThemeState()
	if st.FollowWallpaper {
		r.Color = RiceColor{Mode: "wallpaper"}
	} else {
		r.Color = RiceColor{Mode: "fixed", Palette: "palette.json"}
	}
	ta := themeAppsOn(st)
	r.Color.ThemeApps = &ta
	if err := saveRice(r); err != nil {
		return r, err
	}
	dir := filepath.Join(ricesDir(), slug)
	if wp := currentWallpaper(); isFile(wp) {
		asset := "wall" + filepath.Ext(wp)
		if copyFile(wp, filepath.Join(dir, asset)) == nil {
			r.Assets.Wallpaper = asset
		}
		// the preview is the wallpaper as it stands right now: the saved
		// look's own specimen, exact for stills and the tuned frame for a
		// live (video) wall.
		writeRicePreview(dir, wp)
	}
	if hero := launcherHero(); isFile(hero) {
		asset := "hero" + filepath.Ext(hero)
		if copyFile(hero, filepath.Join(dir, asset)) == nil {
			r.Assets.Hero = asset
		}
	}
	if r.Color.Mode == "fixed" {
		if copyFile(filepath.Join(ryokuCacheDir(), "colors.json"), filepath.Join(dir, "palette.json")) != nil {
			r.Color.Mode = "wallpaper" // no cached palette: follow the wallpaper instead
			r.Color.Palette = ""
		}
	}
	bundleDecorAssets(dir, r.Look["decor"])
	if raw, ok := r.Layers["brand"]; ok {
		var bm map[string]any
		if json.Unmarshal(raw, &bm) == nil {
			changed := false
			if mi, _ := bm["markImage"].(string); mi != "" && isFile(mi) {
				asset := "brandmark" + filepath.Ext(mi)
				if copyFile(mi, filepath.Join(dir, asset)) == nil {
					bm["markImage"] = "rice://" + asset
					changed = true
				}
			}
			// the custom reload cover is a managed asset on the author's disk;
			// bundle it like the mark so the rice renders it on another box
			// instead of leaving the default cover.
			if rc, ok := bm["reloadCover"].(map[string]any); ok {
				if p, _ := rc["path"].(string); p != "" && isFile(p) {
					asset := "reloadcover" + filepath.Ext(p)
					if copyFile(p, filepath.Join(dir, asset)) == nil {
						rc["path"] = "rice://" + asset
						bm["reloadCover"] = rc
						changed = true
					}
				}
			}
			if changed {
				if b, err := json.Marshal(bm); err == nil {
					r.Layers["brand"] = b
				}
			}
		}
	}
	// fastfetch: the whole readout config travels (its layout and modules), plus
	// the logo image when it is a user emblem rather than the shipped brand mark.
	if copyFile(fastfetchConfigPath(), filepath.Join(dir, "fastfetch.jsonc")) == nil {
		r.Assets.FastfetchStyle = "fastfetch.jsonc"
	}
	if m, err := loadFastfetch(); err == nil && m.Logo.Kind == "image" {
		if src := ffExpandTilde(m.Logo.Source); filepath.Base(src) != "fastfetch-emblem.png" && isFile(src) {
			asset := "emblem" + filepath.Ext(src)
			if copyFile(src, filepath.Join(dir, asset)) == nil {
				r.Assets.Fastfetch = asset
			}
		}
	}
	// lockscreen: the active qylock skin travels by slug; the skin files install
	// separately, so a rice carries the choice, not the theme.
	if slug := readLockPref(qylockThemePref()); slug != "" {
		r.Assets.Lock = slug
	}
	return r, saveRice(r)
}

// isEmptyLayer treats a nil, empty array, or empty object as no config, so a
// full capture never bloats a rice with sections the user never set.
func isEmptyLayer(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	}
	return false
}

// riceRun / riceReload wrap the external effects (wallpaper daemon, cursor,
// compositor reload) so tests can observe an apply without a live session.
var riceRun = func(name string, args ...string) error { return exec.Command(name, args...).Run() }
var riceReload = func() { reloadDesktop() }

func wallpaperDir() string { return filepath.Join(os.Getenv("HOME"), "Pictures", "Wallpapers") }

func setLauncherHero(abs string) {
	m := readJSONMap(launcherStorePath())
	m["heroImage"] = abs
	_ = atomicWrite(launcherStorePath(), mustJSON(m), 0o644)
}

func readPalette(path string) map[string]string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var p map[string]string
	if json.Unmarshal(b, &p) != nil {
		return nil
	}
	return p
}

// the reserved backup slots snapshot the four stores verbatim, so a restore is
// a byte-for-byte revert (not an allowlisted merge). that is what makes
// "restore my original setup" trustworthy.
var backupStores = []string{
	"desktop.json", "shell.json", "launcher.json", "theme.json", "ryogami.json",
	"widgets.json", "visualizer.json", "decor.json", "brand.json", "profile.json",
}

func snapshotStores(slot string) error {
	dir := filepath.Join(ricesDir(), slot)
	for _, f := range backupStores {
		src := filepath.Join(ryokuConfigDir(), f)
		if !isFile(src) {
			continue
		}
		if err := copyFile(src, filepath.Join(dir, f)); err != nil {
			return err
		}
	}
	if wp := currentWallpaper(); wp != "" {
		_ = atomicWrite(filepath.Join(dir, "wallpaper.path"), []byte(wp), 0o644)
	}
	// the fastfetch readout and the qylock skin live outside ~/.config/ryoku, so
	// snapshot them explicitly to keep a revert whole.
	if isFile(fastfetchConfigPath()) {
		_ = copyFile(fastfetchConfigPath(), filepath.Join(dir, "fastfetch.jsonc"))
	}
	if slug := readLockPref(qylockThemePref()); slug != "" {
		_ = atomicWrite(filepath.Join(dir, "lock.slug"), []byte(slug), 0o644)
	}
	return nil
}

// ensureBaseline snapshots the pristine pre-rice setup exactly once; it is never
// overwritten, so the user can always return to how the machine shipped/was.
func ensureBaseline() {
	if !isFile(filepath.Join(ricesDir(), ".baseline", "desktop.json")) {
		_ = snapshotStores(".baseline")
	}
}

// restoreRice reverts to a reserved slot (.baseline = pristine pre-rice,
// .previous = the state before the last apply), then re-sets the wallpaper and
// reloads so the live session matches the reverted files.
func restoreRice(slot string) error {
	dir := filepath.Join(ricesDir(), slot)
	restored := false
	for _, f := range backupStores {
		src := filepath.Join(dir, f)
		if !isFile(src) {
			continue
		}
		if err := copyFile(src, filepath.Join(ryokuConfigDir(), f)); err != nil {
			return err
		}
		restored = true
	}
	if !restored {
		return fmt.Errorf("no backup to restore at %q", slot)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "wallpaper.path")); err == nil && isFile(strings.TrimSpace(string(b))) {
		_ = riceRun("ryogami", "wallpaper", "set", strings.TrimSpace(string(b)))
	} else {
		_ = riceRun("ryogami", "wallpaper", "repaint")
	}
	if src := filepath.Join(dir, "fastfetch.jsonc"); isFile(src) {
		_ = copyFile(src, fastfetchConfigPath())
	}
	if b, err := os.ReadFile(filepath.Join(dir, "lock.slug")); err == nil {
		if slug := strings.TrimSpace(string(b)); slug != "" {
			_ = setLockSkinIn(qylockThemesDir(), qylockThemePref(), slug)
		}
	}
	_, _ = desktopClient().Apply(desktopStorePath())
	riceReload()
	return nil
}

// applyRice merges a rice onto the live stores and reloads. it first snapshots
// the current setup into .previous (and .baseline once) so revert is one click.
func applyRice(slug string, layers []string) error {
	r, dir, err := loadRice(slug)
	if err != nil {
		return err
	}
	_ = snapshotStores(".previous")
	ensureBaseline()

	// a store write failing (disk full, bad perms) must surface: silently
	// applying half a rice reports success over mixed state. .previous (above)
	// is the one-click way back either way.
	if err := overlayHyprSections(r.Look["hypr"], riceHyprLook); err != nil {
		return fmt.Errorf("apply hypr look: %w", err)
	}
	// "all" restores every captured layer, so applying a snapshot brings back the
	// keybinds, window rules, input and brand it saved -- not just the look.
	if len(layers) == 1 && layers[0] == "all" && r.Layers != nil {
		layers = nil
		for l := range r.Layers {
			layers = append(layers, l)
		}
	}
	if len(layers) > 0 && r.Layers != nil {
		sections := map[string]any{}
		for _, l := range layers {
			raw, ok := r.Layers[l]
			if !ok {
				continue
			}
			// brand routes to its own store; everything else is a hypr section.
			if l == "brand" {
				var bm map[string]any
				if json.Unmarshal(raw, &bm) == nil && len(bm) > 0 {
					rehydrateBrandAssets(dir, r.Slug, bm)
					if err := overlayStore(brandStorePath(), bm, nil); err != nil {
						return fmt.Errorf("apply brand: %w", err)
					}
				}
				continue
			}
			var v any
			if json.Unmarshal(raw, &v) == nil {
				sections[l] = v
			}
		}
		if len(sections) > 0 {
			_ = setHyprSections(sections)
		}
	}
	if err := overlayStore(shellStorePath(), r.Look["shell"], nil); err != nil {
		return fmt.Errorf("apply shell look: %w", err)
	}
	if err := overlayStore(launcherStorePath(), r.Look["launcher"], nil); err != nil {
		return fmt.Errorf("apply launcher look: %w", err)
	}
	if len(r.Look["widgets"]) > 0 {
		if err := overlayStore(widgetsStorePath(), r.Look["widgets"], nil); err != nil {
			return fmt.Errorf("apply widgets look: %w", err)
		}
	}
	if len(r.Look["visualizer"]) > 0 {
		if err := overlayStore(visualizerStorePath(), r.Look["visualizer"], nil); err != nil {
			return fmt.Errorf("apply visualizer look: %w", err)
		}
	}
	if len(r.Look["decor"]) > 0 {
		dec := r.Look["decor"]
		rehydrateDecorAssets(dir, r.Slug, dec)
		if err := overlayStore(decorStorePath(), dec, nil); err != nil {
			return fmt.Errorf("apply decor look: %w", err)
		}
	}
	if len(r.Look["ryogami"]) > 0 {
		if err := overlayStore(ryogamiStorePath(), r.Look["ryogami"], nil); err != nil {
			return fmt.Errorf("apply ryogami look: %w", err)
		}
	}

	// a rice built after the toggle carries its app-theming choice; apply it so
	// a shared full-system look reaches (or spares) the recipient's apps the same
	// way it did the author's. an older rice (nil) leaves the recipient's setting.
	// Held aside and re-applied below: the colour step reloads theme.json after
	// the daemon has synced it.
	var themeApps *bool
	if r.Color.ThemeApps != nil {
		ta := *r.Color.ThemeApps
		themeApps = &ta
	}
	if r.Assets.Wallpaper != "" {
		// a video wall lands in the livewalls pool (Super+W cycles it like the
		// shell's own), a still in the wallpapers pool; `wallpaper set` routes
		// either to the right daemon.
		destDir := wallpaperDir()
		if isVideo(r.Assets.Wallpaper) {
			destDir = liveWallDir()
		}
		_ = os.MkdirAll(destDir, 0o755)
		dst := filepath.Join(destDir, r.Slug+filepath.Ext(r.Assets.Wallpaper))
		if copyFile(filepath.Join(dir, r.Assets.Wallpaper), dst) == nil {
			_ = riceRun("ryogami", "wallpaper", "set", dst)
		}
	}
	// Colour is set AFTER the wallpaper: `wallpaper set` re-derives the palette
	// from the new wall and turns follow back on, so a fixed rice must re-assert
	// its palette and follow=false last, or the wallpaper's matugen colours win.
	// The shell's theme.theme is the master theme.json shadows (see
	// selectShellTheme): a fixed rice leaves it off Wallpaper so the daemon's next
	// sync cannot flip follow back on. The theme state itself travels whole
	// (scheme, gtkTheme, gnomeAccent): seed from the rice's captured theme.json
	// and let the wallpaper-vs-fixed choice below own only followWallpaper. An
	// older rice (no theme look) keeps the recipient's state, and Color's
	// themeApps still wins.
	riceThemeState := func() themeState {
		st := loadThemeState()
		if tj := r.Look["theme"]; len(tj) > 0 {
			if b, err := json.Marshal(tj); err == nil {
				_ = json.Unmarshal(b, &st)
			}
		}
		if themeApps != nil {
			st.ThemeApps = themeApps
		}
		return st
	}
	if r.Color.Mode == "fixed" {
		if !staticThemeActive() {
			selectShellTheme("Default")
		}
		st := riceThemeState()
		st.FollowWallpaper = false
		if len(r.Look["theme"]) == 0 {
			st.Scheme = "" // older rice carried no scheme; clear as before
		}
		saveThemeState(st)
		if pal := readPalette(filepath.Join(dir, r.Color.Palette)); pal != nil {
			writePalette(pal)
		}
		_ = riceRun("ryogami", "wallpaper", "repaint")
	} else {
		selectShellTheme("Wallpaper")
		st := riceThemeState()
		st.FollowWallpaper = true
		saveThemeState(st)
	}
	if r.Assets.Hero != "" {
		dst := filepath.Join(ryokuConfigDir(), "rice-hero"+filepath.Ext(r.Assets.Hero))
		if copyFile(filepath.Join(dir, r.Assets.Hero), dst) == nil {
			setLauncherHero(dst)
		}
	}
	if r.Assets.Cursor != "" {
		ns := readJSONMap(desktopStorePath())
		childMap(childMap(ns, "desktop"), "cursor")["theme"] = r.Assets.Cursor
		_ = atomicWrite(desktopStorePath(), mustJSON(ns), 0o644)
		theme, size := effectiveCursor()
		setLiveCursor(theme, size)
	}
	// fastfetch: lay the whole readout config first, then repoint the emblem at
	// the rice's bundled logo (an apply that carries only a style leaves the
	// recipient's emblem, and one that carries only an emblem keeps their style).
	if r.Assets.FastfetchStyle != "" {
		_ = copyFile(filepath.Join(dir, r.Assets.FastfetchStyle), fastfetchConfigPath())
	}
	if r.Assets.Fastfetch != "" {
		_, _ = applyFastfetchEmblem(filepath.Join(dir, r.Assets.Fastfetch))
	}
	// lockscreen: point the session lock at the rice's skin when it is installed;
	// the greeter is left untouched so an apply never pops a privilege prompt, and
	// an unknown skin is a soft miss, not a failure.
	if r.Assets.Lock != "" {
		_ = setLockSkinIn(qylockThemesDir(), qylockThemePref(), r.Assets.Lock)
	}
	// profile hero: copy the bundled image into the profile store and point the
	// hero at it, so the recipient's Profile page wears the rice's face. profile.json
	// is in backupStores, so a restore reverts it.
	if prof := r.Look["profile"]; len(prof) > 0 {
		if hero, ok := prof["hero"].(map[string]any); ok {
			if src, _ := hero["source"].(string); src != "" {
				heroDir := profileHeroDir()
				_ = os.MkdirAll(heroDir, 0o755)
				name := "rice-" + r.Slug + filepath.Ext(src)
				if copyFile(filepath.Join(dir, src), filepath.Join(heroDir, name)) == nil {
					hero["source"] = name
					prof["hero"] = hero
				}
			}
		}
		if err := overlayStore(profileConfigPath(), prof, nil); err != nil {
			return fmt.Errorf("apply profile: %w", err)
		}
	}

	_, _ = desktopClient().Apply(desktopStorePath())
	riceReload()
	return nil
}

// --- active rice + compatibility -------------------------------------------

func activePath() string { return filepath.Join(ricesDir(), ".active") }

func activeRice() string {
	b, _ := os.ReadFile(activePath())
	return strings.TrimSpace(string(b))
}

func setActiveRice(slug string) { _ = atomicWrite(activePath(), []byte(slug), 0o644) }

// reapplyRiceEmblem lands the active rice's fastfetch emblem again when the
// readout is back on the shipped brand mark. Boxes riced before applyRice used
// the user-owned ryoku-logo path had the rice's emblem copied over the shipped
// fastfetch-emblem.png, which every update re-laid; `ryoku doctor` calls this
// so one update after the fix restores the rice's logo. A logo the user imported
// themselves (any other source) is never touched. Returns the stored source, or
// "" when nothing needed doing.
func reapplyRiceEmblem() (string, error) {
	slug := activeRice()
	if slug == "" {
		return "", nil
	}
	r, dir, err := loadRice(slug)
	if err != nil || r.Assets.Fastfetch == "" {
		return "", nil
	}
	if !fastfetchOnShippedEmblem() {
		return "", nil
	}
	return applyFastfetchEmblem(filepath.Join(dir, r.Assets.Fastfetch))
}

// --- the UI-facing list ----------------------------------------------------

// riceListEntry is a rice plus the fields the Rices tab needs: compatibility
// against the running Ryoku, whether it is the applied rice, a preview URL,
// and whether its wallpaper is a live (video) wall.
type riceListEntry struct {
	Rice
	Compat  string `json:"compat"`
	Active  bool   `json:"active"`
	Preview string `json:"preview,omitempty"`
	Live    bool   `json:"live,omitempty"`
}

func listRiceEntries() []riceListEntry {
	active := activeRice()
	out := []riceListEntry{}
	for _, r := range listRices() {
		e := riceListEntry{
			Rice: r, Compat: compat.Rice(r.CreatedWith, ryokuVersion()), Active: r.Slug == active,
			Live: isVideo(r.Assets.Wallpaper),
		}
		dir := filepath.Join(ricesDir(), r.Slug)
		if p := filepath.Join(dir, "preview.png"); isFile(p) {
			e.Preview = "file://" + p
		} else if r.Assets.Wallpaper != "" && !e.Live && isFile(filepath.Join(dir, r.Assets.Wallpaper)) {
			// never hand a video to an <Image>; a live rice without a rendered
			// preview falls back to the tile silhouette.
			e.Preview = "file://" + filepath.Join(dir, r.Assets.Wallpaper)
		}
		out = append(out, e)
	}
	return out
}

// setRiceWallpaper bundles a chosen image or video into a user rice as its
// wallpaper, regenerating the preview so the tile shows the wall it will set.
func setRiceWallpaper(slug, src string) error {
	if !validRiceSlug(slug) {
		return fmt.Errorf("bad rice slug %q", slug)
	}
	if !isFile(src) {
		return fmt.Errorf("no such file: %s", src)
	}
	r, dir, err := loadRice(slug)
	if err != nil {
		return err
	}
	asset := "wall" + filepath.Ext(src)
	if err := copyFile(src, filepath.Join(dir, asset)); err != nil {
		return err
	}
	if r.Assets.Wallpaper != "" && r.Assets.Wallpaper != asset {
		_ = os.Remove(filepath.Join(dir, r.Assets.Wallpaper))
	}
	r.Assets.Wallpaper = asset
	writeRicePreview(dir, src)
	return saveRice(r)
}

// --- files / export --------------------------------------------------------

// riceTouch is one path applying a rice writes, or one asset it bundles.
type riceTouch struct {
	Path     string `json:"path"`
	Kind     string `json:"kind"` // config | output | asset
	Icon     string `json:"icon"`
	Label    string `json:"label"`
	Provided bool   `json:"provided"`
}

// homeRel shortens a path under $HOME to a ~ path for display.
func homeRel(p string) string {
	if h := os.Getenv("HOME"); h != "" && strings.HasPrefix(p, h) {
		return "~" + p[len(h):]
	}
	return p
}

// riceTouches reports every path applying the rice writes (its config stores
// and the outputs regenerated from them) plus the assets it carries, each
// flagged by whether the rice actually provides it.
func riceTouches(r Rice, dir string) []riceTouch {
	cfg := ryokuConfigDir()
	touches := []riceTouch{
		{homeRel(filepath.Join(cfg, "hypr.json")), "config", "window", "Windows: decoration and motion", len(r.Look["hypr"]) > 0},
		{homeRel(filepath.Join(cfg, "shell.json")), "config", "widgets", "Shell: bar skin and modules", len(r.Look["shell"]) > 0},
		{homeRel(filepath.Join(cfg, "theme.json")), "config", "palette", "Colours: palette master", r.Color.Mode != "" || len(r.Look["theme"]) > 0},
		{homeRel(filepath.Join(cfg, "launcher.json")), "config", "rocket", "Launcher: hero and card", len(r.Look["launcher"]) > 0 || r.Assets.Hero != ""},
		{homeRel(filepath.Join(cfg, "widgets.json")), "config", "widgets", "Desktop widgets: clock and calendar", len(r.Look["widgets"]) > 0},
		{homeRel(filepath.Join(cfg, "visualizer.json")), "config", "widgets", "Audio visualiser", len(r.Look["visualizer"]) > 0},
		{homeRel(filepath.Join(cfg, "decor.json")), "config", "image", "Desktop decors (pictures bundled)", len(r.Look["decor"]) > 0},
		{homeRel(filepath.Join(hyprConfigDir(), "settings.lua")), "output", "refresh", "Hyprland settings (regenerated)", true},
	}
	if r.Color.Mode == "fixed" {
		touches = append(touches,
			riceTouch{homeRel(filepath.Join(ryokuCacheDir(), "colors.json")), "output", "refresh", "Colour palette (ryoku cache)", true},
			riceTouch{homeRel(kittyThemePath()), "output", "terminal", "kitty colours", true},
		)
	}
	if r.Color.ThemeApps == nil || *r.Color.ThemeApps {
		touches = append(touches, riceTouch{homeRel(filepath.Join(configHome(), "gtk-3.0", "gtk.css")), "output", "widgets", "GTK / GUI apps (Files, editors)", true})
	}
	if r.Assets.Wallpaper != "" {
		label := "Desktop wallpaper"
		if isVideo(r.Assets.Wallpaper) {
			label = "Desktop wallpaper (live video)"
		}
		touches = append(touches, riceTouch{homeRel(filepath.Join(dir, r.Assets.Wallpaper)), "asset", "wallpaper", label, true})
	}
	if r.Assets.Hero != "" {
		touches = append(touches, riceTouch{homeRel(filepath.Join(dir, r.Assets.Hero)), "asset", "image", "Launcher hero", true})
	}
	if r.Color.Palette != "" {
		touches = append(touches, riceTouch{homeRel(filepath.Join(dir, r.Color.Palette)), "asset", "palette", "Fixed 16-colour palette", isFile(filepath.Join(dir, r.Color.Palette))})
	}
	if r.Assets.Cursor != "" {
		touches = append(touches, riceTouch{r.Assets.Cursor, "asset", "mouse", "Cursor theme", true})
	}
	layerRows := []struct {
		key, icon, label string
	}{
		{"keybinds", "keyboard", "Keybinds"},
		{"input", "mouse", "Input (pointer, keyboard)"},
		{"windowRules", "window", "Window rules"},
		{"layerRules", "widgets", "Layer rules"},
		{"appOverrides", "window", "Per-app overrides"},
		{"autostart", "rocket", "Autostart"},
		{"env", "variable", "Environment"},
		{"brand", "image", "Brand mark and name"},
	}
	for _, lr := range layerRows {
		if _, ok := r.Layers[lr.key]; ok {
			// every hypr layer lands in hypr.json; brand routes to its own store.
			store := "hypr.json"
			if lr.key == "brand" {
				store = "brand.json"
			}
			touches = append(touches, riceTouch{homeRel(filepath.Join(cfg, store)), "config", lr.icon, lr.label, true})
		}
	}
	return touches
}

// riceFiles prints what a rice touches plus its manifest, for the Hub's
// "what it touches" list and config viewer.
func riceFiles(slug string) error {
	r, dir, err := loadRice(slug)
	if err != nil {
		return err
	}
	return printJSON(struct {
		Touches []riceTouch `json:"touches"`
		Config  string      `json:"config"`
	}{
		Touches: riceTouches(r, dir),
		Config:  string(mustJSON(r)),
	})
}

// exportRice extracts a rice into dest/<slug>/: its manifest and assets, a
// readable configs/ breakout of the per-store look, and a short README, so the
// whole setup travels as a plain, inspectable folder.
func exportRice(slug, dest string) (string, error) {
	if !validRiceSlug(slug) {
		return "", fmt.Errorf("bad rice slug %q", slug)
	}
	r, dir, err := loadRice(slug)
	if err != nil {
		return "", err
	}
	if fi, err := os.Stat(dest); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("not a folder: %s", dest)
	}
	out := filepath.Join(dest, slug)
	if err := os.MkdirAll(out, 0o755); err != nil {
		return "", err
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		if err := copyFile(filepath.Join(dir, e.Name()), filepath.Join(out, e.Name())); err != nil {
			return "", err
		}
	}
	cfgDir := filepath.Join(out, "configs")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return "", err
	}
	for store, vals := range r.Look {
		if len(vals) == 0 {
			continue
		}
		if err := atomicWrite(filepath.Join(cfgDir, store+".json"), mustJSON(vals), 0o644); err != nil {
			return "", err
		}
	}
	if err := atomicWrite(filepath.Join(out, "README.txt"), []byte(exportReadme(r)), 0o644); err != nil {
		return "", err
	}
	return out, nil
}

func exportReadme(r Rice) string {
	name := r.Name
	if name == "" {
		name = r.Slug
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Ryoku rice: %s\n", name)
	if r.Author != "" {
		fmt.Fprintf(&b, "By %s\n", r.Author)
	}
	if r.Blurb != "" {
		fmt.Fprintf(&b, "\n%s\n", r.Blurb)
	}
	b.WriteString("\nContents\n")
	b.WriteString("  rice.json    the manifest: the look values and asset names\n")
	b.WriteString("  configs/     the same look broken out per config file, to read\n")
	if r.Color.Palette != "" {
		b.WriteString("  palette.json the fixed 16-colour palette\n")
	}
	if r.Assets.Wallpaper != "" {
		kind := "the desktop wallpaper"
		if isVideo(r.Assets.Wallpaper) {
			kind = "the live desktop wallpaper (video)"
		}
		fmt.Fprintf(&b, "  %s   %s\n", r.Assets.Wallpaper, kind)
	}
	if r.Assets.Hero != "" {
		fmt.Fprintf(&b, "  %s   the launcher hero image\n", r.Assets.Hero)
	}
	if len(r.Look["decor"]) > 0 {
		b.WriteString("  decor-*      the desktop decors' pictures\n")
	}
	fmt.Fprintf(&b, "\nDrop this folder into ~/.config/ryoku/rices/ and apply it from\nRyoku Settings > Appearance > Rices, or `ryoku-hub rice apply %s`.\n", r.Slug)
	return b.String()
}

// runExportRice prints the destination folder for the Hub.
func runExportRice(slug, dest string) error {
	out, err := exportRice(slug, dest)
	if err != nil {
		return err
	}
	return printJSON(map[string]string{"path": out})
}

// --- edit / delete / fork --------------------------------------------------

func validRiceSlug(slug string) bool {
	return slug != "" && !strings.HasPrefix(slug, ".") && !strings.ContainsAny(slug, "/\\")
}

func deleteRice(slug string) error {
	if !validRiceSlug(slug) {
		return fmt.Errorf("bad rice slug %q", slug)
	}
	if slug == activeRice() {
		setActiveRice("")
	}
	return os.RemoveAll(filepath.Join(ricesDir(), slug))
}

// saveRiceJSON persists an edited manifest sent by the Hub's rice editor.
func saveRiceJSON(s string) error {
	var r Rice
	if err := json.Unmarshal([]byte(s), &r); err != nil {
		return fmt.Errorf("parse rice JSON: %w", err)
	}
	if !validRiceSlug(r.Slug) {
		return fmt.Errorf("rice needs a valid slug")
	}
	if r.Schema == 0 {
		r.Schema = riceSchema
	}
	return saveRice(r)
}

// importRice copies an exported rice folder (anything holding a valid
// rice.json) into the user's rices, so a shared look installs from a picked
// folder in one step. the slug is re-derived and de-duped locally, never
// trusted as a path; configs/ and README are the export's reading matter and
// stay behind.
func importRice(src string) (Rice, error) {
	b, err := os.ReadFile(filepath.Join(src, "rice.json"))
	if err != nil {
		return Rice{}, fmt.Errorf("not a rice folder (no rice.json): %s", src)
	}
	var r Rice
	if err := json.Unmarshal(b, &r); err != nil {
		return Rice{}, fmt.Errorf("bad rice.json: %w", err)
	}
	slug := slugify(r.Name)
	if slug == "" {
		slug = slugify(r.Slug)
	}
	if slug == "" {
		return Rice{}, fmt.Errorf("rice.json names no rice")
	}
	base := slug
	for i := 2; isFile(ricePath(slug)); i++ {
		slug = fmt.Sprintf("%s-%d", base, i)
	}
	r.Slug = slug
	ents, err := os.ReadDir(src)
	if err != nil {
		return Rice{}, err
	}
	dir := filepath.Join(ricesDir(), slug)
	for _, e := range ents {
		n := e.Name()
		if e.IsDir() || n == "rice.json" || n == "README.txt" || strings.HasPrefix(n, ".") {
			continue
		}
		if err := copyFile(filepath.Join(src, n), filepath.Join(dir, n)); err != nil {
			return Rice{}, err
		}
	}
	return r, saveRice(r)
}

// forkRice duplicates a rice (and its bundled assets) under a fresh slug, so a
// shipped or installed rice can be tweaked without touching the original.
func forkRice(slug string) (Rice, error) {
	r, dir, err := loadRice(slug)
	if err != nil {
		return r, err
	}
	newSlug := slug + "-copy"
	for i := 2; isFile(ricePath(newSlug)); i++ {
		newSlug = fmt.Sprintf("%s-copy-%d", slug, i)
	}
	newDir := filepath.Join(ricesDir(), newSlug)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() || e.Name() == "rice.json" {
			continue
		}
		_ = copyFile(filepath.Join(dir, e.Name()), filepath.Join(newDir, e.Name()))
	}
	r.Slug = newSlug
	r.Name = strings.TrimSpace(r.Name) + " copy"
	return r, saveRice(r)
}

// preflightData is what the capture card shows before a save: wallpaper kind,
// decor count, the non-empty behavior layers, colour mode. the user sees the
// coverage before naming the rice, instead of after.
func preflightData() map[string]any {
	hy := readHyprSections()
	layers := []string{}
	for _, l := range riceHyprLayers {
		if v, ok := hy[l]; ok && !isEmptyLayer(v) {
			layers = append(layers, l)
		}
	}
	if len(readJSONMap(brandStorePath())) > 0 {
		layers = append(layers, "brand")
	}
	decors := 0
	for _, v := range readJSONMap(decorStorePath()) {
		if ent, ok := v.(map[string]any); ok && len(ent) > 0 {
			decors++
		}
	}
	wall := currentWallpaper()
	return map[string]any{
		"wallpaper":  wall != "" && isFile(wall),
		"live":       isVideo(wall),
		"decors":     decors,
		"widgets":    len(readJSONMap(widgetsStorePath())) > 0,
		"visualizer": len(readJSONMap(visualizerStorePath())) > 0,
		"fastfetch":  isFile(fastfetchConfigPath()),
		"lock":       readLockPref(qylockThemePref()) != "",
		"layers":     layers,
		"fixed":      !loadThemeState().FollowWallpaper,
		"themeApps":  themeAppsOn(loadThemeState()),
	}
}

func ricePreflight() error { return printJSON(preflightData()) }

// --- dispatch --------------------------------------------------------------

func runRice(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("rice needs list|preflight|capture|apply|restore|save|fork|delete|import|publish|setwall|files|export|emblem")
	}
	switch args[0] {
	case "preflight":
		return ricePreflight()
	case "list":
		return printJSON(listRiceEntries())
	case "capture":
		if len(args) < 2 {
			return fmt.Errorf("rice capture needs a name")
		}
		r, err := captureRice(args[1], args[2:])
		if err != nil {
			return err
		}
		return printJSON(r)
	case "apply":
		if len(args) < 2 {
			return fmt.Errorf("rice apply needs a slug")
		}
		if err := applyRice(args[1], args[2:]); err != nil {
			return err
		}
		setActiveRice(args[1])
		return nil
	case "emblem":
		p, err := reapplyRiceEmblem()
		if err != nil {
			return err
		}
		if p != "" {
			fmt.Println(p)
		}
		return nil
	case "restore":
		slot := ".baseline"
		if len(args) >= 2 && args[1] == "previous" {
			slot = ".previous"
		}
		if err := restoreRice(slot); err != nil {
			return err
		}
		setActiveRice("")
		return nil
	case "save":
		if len(args) < 2 {
			return fmt.Errorf("rice save needs a JSON argument")
		}
		return saveRiceJSON(args[1])
	case "fork":
		if len(args) < 2 {
			return fmt.Errorf("rice fork needs a slug")
		}
		nr, err := forkRice(args[1])
		if err != nil {
			return err
		}
		return printJSON(nr)
	case "delete":
		if len(args) < 2 {
			return fmt.Errorf("rice delete needs a slug")
		}
		return deleteRice(args[1])
	case "import":
		if len(args) < 2 {
			return fmt.Errorf("rice import needs a folder")
		}
		nr, err := importRice(args[1])
		if err != nil {
			return err
		}
		return printJSON(nr)
	case "publish":
		if len(args) < 3 {
			return fmt.Errorf("rice publish needs a slug and a store path")
		}
		return publishRice(args[1], args[2])
	case "setwall":
		if len(args) < 3 {
			return fmt.Errorf("rice setwall needs a slug and an image path")
		}
		return setRiceWallpaper(args[1], args[2])
	case "files":
		if len(args) < 2 {
			return fmt.Errorf("rice files needs a slug")
		}
		return riceFiles(args[1])
	case "export":
		if len(args) < 3 {
			return fmt.Errorf("rice export needs a slug and a destination folder")
		}
		return runExportRice(args[1], args[2])
	default:
		return fmt.Errorf("unknown rice subcommand: %s", args[0])
	}
}
