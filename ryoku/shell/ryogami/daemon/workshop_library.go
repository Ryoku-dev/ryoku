package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const workshopAppID = "431960"

// Matches the renderer's cap on preset dependency chains.
const maxPresetDepth = 32

type workshopLib struct {
	d     *daemon
	props *wePropStore
}

func newWorkshopLib(d *daemon) *workshopLib {
	w := &workshopLib{d: d, props: newWePropStore(d)}
	weScanHook = w.scanEntries
	return w
}

func wkString(v interface{}) string {
	s, _ := v.(string)
	return s
}

func wkBool(v interface{}, def bool) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return def
}

func wkInt(v interface{}, def int) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case string:
		if i, err := strconv.Atoi(n); err == nil {
			return i
		}
	}
	return def
}

func (d *daemon) featureSteam() bool {
	return wkBool(d.setting("features.steam"), true)
}

func (w *workshopLib) steamRoot() string {
	if p := wkString(w.d.setting("paths.steam")); p != "" {
		return resolvePath(p)
	}
	return detectSteamRoot(home(), func(p string) bool { return dirExists(p) })
}

func (w *workshopLib) workshopDir() string {
	if p := wkString(w.d.setting("paths.steamWorkshop")); p != "" {
		return resolvePath(p)
	}
	return filepath.Join(w.steamRoot(), "steamapps", "workshop", "content", workshopAppID)
}

func detectSteamRoot(homeDir string, exists func(string) bool) string {
	candidates := []string{
		".local/share/Steam",
		".steam/steam",
		".steam/debian-installation",
		".var/app/com.valvesoftware.Steam/.local/share/Steam",
		"snap/steam/common/.local/share/Steam",
	}
	for _, c := range candidates {
		root := filepath.Join(homeDir, filepath.FromSlash(c))
		if exists(filepath.Join(root, "steamapps")) {
			return root
		}
	}
	return filepath.Join(homeDir, ".local", "share", "Steam")
}

type weProject struct {
	source      string
	directories []string
	document    map[string]interface{}
}

func readProject(dir string) (map[string]interface{}, error) {
	path := filepath.Join(dir, "project.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) > 16*1024*1024 {
		return nil, fmt.Errorf("%s: exceeds 16 MiB", path)
	}
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
	var doc map[string]interface{}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if doc == nil {
		return nil, fmt.Errorf("%s: project must be an object", path)
	}
	return doc, nil
}

func projectDependency(doc map[string]interface{}) (string, bool, error) {
	raw, has := doc["dependency"]
	if !has {
		if _, hasPreset := doc["preset"]; hasPreset {
			return "", false, fmt.Errorf("preset has no dependency")
		}
		return "", false, nil
	}
	var id string
	switch v := raw.(type) {
	case string:
		id = v
	case float64:
		id = strconv.FormatInt(int64(v), 10)
	default:
		return "", false, fmt.Errorf("invalid dependency id")
	}
	if id == "" || !allDigits(id) || id == "0" {
		return "", false, fmt.Errorf("invalid dependency id")
	}
	if _, ok := doc["preset"].(map[string]interface{}); !ok {
		return "", false, fmt.Errorf("preset settings must be an object")
	}
	return id, true, nil
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

func resolveProject(dir string) (*weProject, error) {
	library := filepath.Dir(dir)
	source := dir
	var directories []string
	visited := map[string]bool{}
	var presets []map[string]interface{}
	var document map[string]interface{}
	for {
		canon, err := wkCanonPath(source)
		if err != nil {
			return nil, err
		}
		source = canon
		if visited[source] {
			return nil, fmt.Errorf("preset dependency cycle")
		}
		visited[source] = true
		if len(directories) >= maxPresetDepth {
			return nil, fmt.Errorf("preset dependency chain is too deep")
		}
		directories = append(directories, source)
		doc, err := readProject(source)
		if err != nil {
			return nil, err
		}
		id, has, err := projectDependency(doc)
		if err != nil {
			return nil, err
		}
		if !has {
			document = doc
			break
		}
		presets = append(presets, mapOf(doc["preset"]))
		linked := filepath.Join(library, id)
		sibling := filepath.Join(filepath.Dir(source), id)
		if fileExists(filepath.Join(linked, "project.json")) {
			source = linked
		} else {
			source = sibling
		}
		if !fileExists(filepath.Join(source, "project.json")) {
			return nil, fmt.Errorf("preset requires Workshop item %s; download it first", id)
		}
	}
	applyPresets(document, presets)
	return &weProject{source: source, directories: directories, document: document}, nil
}

// Later-declared presets win, so they are applied in reverse.
func applyPresets(document map[string]interface{}, presets []map[string]interface{}) {
	props := mapOf(mapOf(document["general"])["properties"])
	if len(props) == 0 {
		return
	}
	for i := len(presets) - 1; i >= 0; i-- {
		for name, value := range presets[i] {
			entry, ok := props[name].(map[string]interface{})
			if !ok {
				continue
			}
			entry["value"] = value
		}
	}
}

func mapOf(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return map[string]interface{}{}
}

func (p *weProject) declarations() map[string]interface{} {
	return mapOf(mapOf(p.document["general"])["properties"])
}

func (p *weProject) scenePackage() (string, error) {
	for _, name := range []string{"scene.pkg", "gifscene.pkg"} {
		if candidate := filepath.Join(p.source, name); fileExists(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("scene package is missing in %s", p.source)
}

func projectType(doc map[string]interface{}) string {
	s, _ := doc["type"].(string)
	return strings.ToLower(s)
}

// Web and application projects are rejected: the renderer plays only scenes and videos.
func validateProject(dir string) (string, error) {
	project, err := resolveProject(dir)
	if err != nil {
		return "", err
	}
	switch projectType(project.document) {
	case "scene":
		if _, err := project.scenePackage(); err != nil {
			return "", err
		}
		return "scene", nil
	case "video":
		file, _ := project.document["file"].(string)
		joined, ok := safeItemJoin(project.source, file)
		if !ok || !fileExists(joined) {
			return "", fmt.Errorf("video file is missing or unsafe")
		}
		return "video", nil
	default:
		return "", fmt.Errorf("unsupported project type %q", projectType(project.document))
	}
}

// Refuses any path that escapes the item directory.
func safeItemJoin(itemDir, file string) (string, bool) {
	if file == "" {
		return "", false
	}
	rel := filepath.FromSlash(file)
	if filepath.IsAbs(rel) {
		return "", false
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if part == ".." {
			return "", false
		}
	}
	root, err := wkCanonPath(itemDir)
	if err != nil {
		return "", false
	}
	joined, err := wkCanonPath(filepath.Join(itemDir, rel))
	if err != nil {
		return "", false
	}
	if joined != root && !strings.HasPrefix(joined, root+string(os.PathSeparator)) {
		return "", false
	}
	return joined, true
}

// An absent path falls back to a clean absolute form so membership can be tested first.
func wkCanonPath(p string) (string, error) {
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return filepath.Abs(resolved)
	}
	return filepath.Abs(p)
}

func validWeID(id string) bool {
	return id != "" &&
		!strings.HasPrefix(id, "-") &&
		!strings.Contains(id, "/") &&
		!strings.Contains(id, "\\") &&
		!strings.Contains(id, "..") &&
		id != "."
}

func readProjectTitle(itemDir string) string {
	doc, err := readProject(itemDir)
	if err != nil {
		return ""
	}
	title, _ := doc["title"].(string)
	return strings.TrimSpace(title)
}

func projectTags(doc map[string]interface{}) string {
	arr, ok := doc["tags"].([]interface{})
	if !ok {
		return ""
	}
	var tags []string
	for _, v := range arr {
		if s, ok := v.(string); ok && s != "" {
			tags = append(tags, s)
		}
	}
	return strings.Join(tags, ", ")
}

func findPreview(itemDir string, doc map[string]interface{}) string {
	if file, ok := doc["preview"].(string); ok {
		if joined, ok := safeItemJoin(itemDir, file); ok && fileExists(joined) {
			return joined
		}
	}
	entries, err := os.ReadDir(itemDir)
	if err != nil {
		return ""
	}
	var names []string
	for _, e := range entries {
		if strings.HasPrefix(strings.ToLower(e.Name()), "preview.") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		if joined, ok := safeItemJoin(itemDir, name); ok && fileExists(joined) {
			return joined
		}
	}
	return ""
}

func (w *workshopLib) downloadedIDs() []string {
	entries, err := os.ReadDir(w.workshopDir())
	if err != nil {
		return nil
	}
	var ids []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		if !validWeID(id) {
			continue
		}
		if _, err := validateProject(filepath.Join(w.workshopDir(), id)); err == nil {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func weThumbPaths(cacheDir, id string) (thumb, thumbSm string) {
	thumb = filepath.Join(cacheDir, "wallpaper", "thumbs", "we--"+id+".webp")
	thumbSm = filepath.Join(cacheDir, "wallpaper", "thumbs-sm", "we--"+id+".webp")
	return thumb, thumbSm
}

func (w *workshopLib) scanEntries(cacheDir string, prior map[string]Entry, onItem func(Entry)) map[string]Entry {
	if !w.d.featureSteam() {
		return nil
	}
	dir := w.workshopDir()
	result := map[string]Entry{}
	for _, id := range w.downloadedIDs() {
		itemDir := filepath.Join(dir, id)
		key := "we:" + id
		mtime := fileMtime(filepath.Join(itemDir, "project.json"))
		thumb, thumbSm := weThumbPaths(cacheDir, id)
		if p, ok := prior[key]; ok && p.Mtime == mtime && fileExists(p.Thumb) {
			if p.Path == "" {
				p.Path = itemDir
			}
			result[key] = p
			continue
		}
		project, err := resolveProject(itemDir)
		if err != nil {
			continue
		}
		weType, err := validateProject(itemDir)
		if err != nil {
			continue
		}
		preview := findPreview(itemDir, project.document)
		e := Entry{
			Key:     key,
			Name:    firstNonEmpty(readProjectTitle(itemDir), id),
			Type:    "we",
			WeID:    id,
			WeType:  weType,
			Tags:    projectTags(project.document),
			Preview: preview,
			Path:    itemDir,
			Mtime:   mtime,
		}
		if p, ok := prior[key]; ok {
			e.Favourite = p.Favourite
			e.ApplyCount = p.ApplyCount
		}
		if weType == "video" {
			if file, ok := project.document["file"].(string); ok {
				if joined, ok := safeItemJoin(project.source, file); ok {
					e.VideoFile = joined
					if fi, err := os.Stat(joined); err == nil {
						e.Filesize = fi.Size()
					}
				}
			}
		}
		if preview != "" {
			if err := generateWeThumb(preview, thumb, thumbSm); err != nil {
				fmt.Fprintf(os.Stderr, "ryogami: we thumb failed for %s: %v\n", id, err)
			} else {
				e.Thumb = thumb
				e.ThumbSm = thumbSm
				hue, sat, richness := extractColors(thumb)
				e.Hue = int(hueBucket(hue, sat))
				e.Sat = int(sat)
				e.Richness = int(richness)
			}
		}
		result[key] = e
		onItem(e)
	}
	return result
}

func generateWeThumb(preview, thumb, thumbSm string) error {
	if err := os.MkdirAll(filepath.Dir(thumb), 0o755); err != nil {
		return err
	}
	tmp := tmpPath(thumb)
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "magick",
		fmt.Sprintf("%s[0]", preview),
		"-resize", fmt.Sprintf("%dx%d^", thumbW, thumbH),
		"-gravity", "center",
		"-extent", fmt.Sprintf("%dx%d", thumbW, thumbH),
		"-quality", "85", tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("magick: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if err := os.Rename(tmp, thumb); err != nil {
		return err
	}
	return genSmallThumb(thumb, thumbSm)
}

func (w *workshopLib) downloadedSet() map[string]bool {
	set := map[string]bool{}
	for _, id := range w.downloadedIDs() {
		set[id] = true
	}
	return set
}

func (d *daemon) emitWorkshopDownload(id, status string, progress float64, message, path string) {
	data := map[string]interface{}{"id": id, "status": status}
	if progress > 0 {
		data["progress"] = progress
	}
	if message != "" {
		data["message"] = message
	}
	if path != "" {
		data["path"] = path
	}
	d.broadcast("ryogami.workshop.download", data)
	// Steam and steamcmd own the transfer, so the chip reports it but cannot stop it.
	task := "download:we-" + id
	switch status {
	case "downloading":
		d.tasks.startOnce(task, "download", "Download", 100)
		d.tasks.progress(task, int(progress*100), 100, message)
	case "done":
		d.tasks.finish(task, taskCompleted, 0, "")
	default:
		d.tasks.finish(task, taskFailed, 0, message)
	}
}

// With no backend to fetch the item, open_in_steam hands over a steam:// URL; without
// Steam at all, no_steam points at the item's Workshop page instead.
func (w *workshopLib) download(id string) (status, openURL string) {
	d := w.d
	dir := w.workshopDir()
	if _, err := validateProject(filepath.Join(dir, id)); err == nil {
		go d.rescan(true)
		d.emitWorkshopDownload(id, "done", 1.0, "", filepath.Join(dir, id))
		return "exists", ""
	}
	backend := wkString(d.setting("steam.backend"))
	switch {
	case backend == "steamcmd" && steamcmdInstalled():
		go w.runDownload(id, true)
		return "started", ""
	case steamHelperPresent():
		go w.runDownload(id, false)
		return "started", ""
	case steamcmdInstalled():
		go w.runDownload(id, true)
		return "started", ""
	case !steamInstalled():
		return "no_steam", "https://steamcommunity.com/sharedfiles/filedetails/?id=" + id
	default:
		return "open_in_steam", "steam://url/CommunityFilePage/" + id
	}
}

var flatpakSystemApps = "/var/lib/flatpak/app"

// A native or Snap Steam puts steam on PATH; a Flatpak one only leaves its app directory.
func steamInstalled() bool {
	if _, err := exec.LookPath("steam"); err == nil {
		return true
	}
	for _, apps := range []string{flatpakSystemApps, filepath.Join(home(), ".local", "share", "flatpak", "app")} {
		if dirExists(filepath.Join(apps, "com.valvesoftware.Steam")) {
			return true
		}
	}
	return false
}

func (w *workshopLib) runDownload(id string, useSteamcmd bool) {
	d := w.d
	d.emitWorkshopDownload(id, "downloading", 0, "", "")
	fetch := func(item string) bool {
		if useSteamcmd {
			return w.steamcmdDownload(item, id)
		}
		return w.steamworksDownload(item, id)
	}
	if err := w.installChain(id, fetch); err != nil {
		d.emitWorkshopDownload(id, "error", 0, err.Error(), "")
		return
	}
	d.emitWorkshopDownload(id, "done", 1.0, "", filepath.Join(w.workshopDir(), id))
	d.rescan(true)
	d.broadcast("ryogami.workshop.changed", map[string]interface{}{})
}

func (w *workshopLib) installChain(id string, fetch func(item string) bool) error {
	dir := w.workshopDir()
	cur := id
	visited := map[string]bool{}
	for {
		if visited[cur] {
			return fmt.Errorf("preset dependency cycle at %s", cur)
		}
		visited[cur] = true
		if len(visited) > maxPresetDepth {
			return fmt.Errorf("preset dependency chain is too deep")
		}
		item := filepath.Join(dir, cur)
		doc, derr := readProject(item)
		incomplete := false
		if derr == nil {
			if t := projectType(doc); t == "scene" || t == "video" {
				if _, verr := validateProject(item); verr != nil {
					incomplete = true
				}
			}
		}
		if (derr != nil || incomplete) && !fetch(cur) {
			return fmt.Errorf("download failed for Workshop item %s", cur)
		}
		doc, err := readProject(item)
		if err != nil {
			return err
		}
		parent, has, err := projectDependency(doc)
		if err != nil {
			return err
		}
		if !has {
			break
		}
		if actual, err := wkCanonPath(item); err == nil {
			sibling := filepath.Join(filepath.Dir(actual), parent)
			if fileExists(filepath.Join(sibling, "project.json")) {
				reconcileWeItem(dir, parent, sibling)
			}
		}
		cur = parent
	}
	_, err := validateProject(filepath.Join(dir, id))
	return err
}

func reconcileWeItem(weDir, id, actual string) {
	target := filepath.Join(weDir, id)
	if _, err := os.Lstat(target); err == nil {
		return
	}
	if actual == "" || !dirExists(actual) || actual == target {
		return
	}
	_ = os.MkdirAll(weDir, 0o755)
	_ = os.Symlink(actual, target)
}
