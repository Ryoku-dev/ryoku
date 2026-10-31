package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const workshopAppID = "431960"

// Matches the renderer's cap on preset dependency chains.
const maxPresetDepth = 32

const (
	workshopWatchDebounce        = time.Second
	workshopFallbackPollInterval = 30 * time.Second
)

type workshopFetchError struct {
	status  string
	message string
}

func (e *workshopFetchError) Error() string {
	return e.message
}

func workshopFetchFailure(status, message string) error {
	return &workshopFetchError{status: status, message: message}
}

type workshopLib struct {
	d     *daemon
	props *wePropStore

	libMu    sync.Mutex
	libStamp string
	libDirs  []string
	rewatch  chan struct{}
}

func newWorkshopLib(d *daemon) *workshopLib {
	w := &workshopLib{d: d, props: newWePropStore(d), rewatch: make(chan struct{}, 1)}
	weScanHook = w.scanEntries
	return w
}

func (w *workshopLib) requestRewatch() {
	select {
	case w.rewatch <- struct{}{}:
	default:
	}
}

func init() {
	watchSetting("paths.", func(d *daemon, key string, _ interface{}) {
		if d.workshop == nil {
			return
		}
		switch key {
		case "paths.steam", "paths.steamWorkshop":
			d.workshop.requestRewatch()
		}
	})
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
	return filepath.Join(w.weLibrary(), "steamapps", "workshop", "content", workshopAppID)
}

// The renderer guesses only the native Steam paths, so a Flatpak Steam or a second
// library would leave scenes without Wallpaper Engine's shared shaders and materials.
func (w *workshopLib) weAssetsDir() string {
	if p := wkString(w.d.setting("paths.steamWeAssets")); p != "" {
		return resolvePath(p)
	}
	dir := filepath.Join(w.weLibrary(), "steamapps", "common", "wallpaper_engine", "assets")
	if dirExists(dir) {
		return dir
	}
	return ""
}

func (w *workshopLib) weLibrary() string {
	root := w.steamRoot()
	vdf, stamp := libraryFoldersFile(root)
	w.libMu.Lock()
	if w.libDirs == nil || stamp != w.libStamp {
		w.libDirs = steamLibraryDirs(root, vdf)
		w.libStamp = stamp
	}
	dirs := w.libDirs
	w.libMu.Unlock()
	return pickWeLibrary(dirs)
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

func previewSource(path string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	return imageExts[ext] || videoExts[ext]
}

func declaredPreview(dir string, doc map[string]interface{}) string {
	file, _ := doc["preview"].(string)
	joined, ok := safeItemJoin(dir, file)
	if !ok || !fileExists(joined) || !previewSource(joined) {
		return ""
	}
	return joined
}

func namedPreview(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(strings.ToLower(name), "preview.") {
			if joined, ok := safeItemJoin(dir, name); ok && fileExists(joined) && previewSource(joined) {
				return joined
			}
		}
	}
	return ""
}

func fallbackPreview(dir string) string {
	var found string
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if path == dir {
				return err
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(entry.Name())) {
		case ".jpg", ".jpeg", ".png":
			found = path
			return fs.SkipAll
		default:
			return nil
		}
	})
	return found
}

func findPreview(itemDir string, project *weProject) string {
	dirs := []string{itemDir}
	docs := []map[string]interface{}{nil}
	if doc, err := readProject(itemDir); err == nil {
		docs[0] = doc
	}
	if project != nil && project.source != itemDir {
		dirs = append(dirs, project.source)
		docs = append(docs, project.document)
	} else if project != nil {
		docs[0] = project.document
	}
	for i, dir := range dirs {
		if preview := declaredPreview(dir, docs[i]); preview != "" {
			return preview
		}
	}
	for _, dir := range dirs {
		if preview := namedPreview(dir); preview != "" {
			return preview
		}
	}
	for _, dir := range dirs {
		if preview := fallbackPreview(dir); preview != "" {
			return preview
		}
	}
	return ""
}

func projectSchemeColor(doc map[string]interface{}) (int, int, bool) {
	general, _ := doc["general"].(map[string]interface{})
	properties, _ := general["properties"].(map[string]interface{})
	raw := properties["schemecolor"]
	if object, ok := raw.(map[string]interface{}); ok {
		raw = object["value"]
	}
	value, _ := raw.(string)
	parts := strings.Fields(value)
	if len(parts) < 3 {
		return 0, 0, false
	}
	r, errR := strconv.ParseFloat(parts[0], 64)
	g, errG := strconv.ParseFloat(parts[1], 64)
	b, errB := strconv.ParseFloat(parts[2], 64)
	if errR != nil || errG != nil || errB != nil {
		return 0, 0, false
	}
	r = math.Max(0, math.Min(1, r))
	g = math.Max(0, math.Min(1, g))
	b = math.Max(0, math.Min(1, b))
	high := math.Max(r, math.Max(g, b))
	low := math.Min(r, math.Min(g, b))
	delta := high - low
	lightness := (high + low) / 2
	var saturation float64
	if delta > 0 {
		saturation = delta / (1 - math.Abs(2*lightness-1))
	}
	var hue float64
	switch {
	case delta == 0:
	case high == r:
		hue = 60 * math.Mod((g-b)/delta, 6)
	case high == g:
		hue = 60 * ((b-r)/delta + 2)
	default:
		hue = 60 * ((r-g)/delta + 4)
	}
	if hue < 0 {
		hue += 360
	}
	sat := int(math.Round(saturation * 100))
	return int(hueBucket(uint16(math.Round(hue))%360, uint16(sat))), sat, true
}

func (w *workshopLib) downloadedIDs() []string {
	dir := w.workshopDir()
	entries, err := os.ReadDir(dir)
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
		if _, err := validateProject(filepath.Join(dir, id)); err == nil {
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
		preview := findPreview(itemDir, project)
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
		if hue, sat, ok := projectSchemeColor(project.document); ok {
			e.Hue = hue
			e.Sat = sat
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
	source := preview
	if needsFirstFrame(preview) {
		frame, err := extractFirstFrame(preview)
		if err != nil {
			return fmt.Errorf("first frame: %w", err)
		}
		defer os.Remove(frame)
		source = frame
	}
	tmp := tmpPath(thumb)
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "magick",
		fmt.Sprintf("%s[0]", source),
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

func (w *workshopLib) workshopSignature() string {
	dir := w.workshopDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var parts []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		parts = append(parts, e.Name()+":"+strconv.FormatInt(fileMtime(filepath.Join(dir, e.Name(), "project.json")), 10))
	}
	sort.Strings(parts)
	return strings.Join(parts, "|")
}

type workshopWatch struct {
	fd     int
	root   string
	wds    map[int32]string
	byPath map[string]int32
}

func openWorkshopWatch(root string) (*workshopWatch, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC)
	if err != nil {
		return nil, err
	}
	watch := &workshopWatch{
		fd:     fd,
		root:   filepath.Clean(root),
		wds:    map[int32]string{},
		byPath: map[string]int32{},
	}
	base, rootExists, err := nearestWorkshopAncestor(watch.root)
	if err != nil {
		syscall.Close(fd)
		return nil, err
	}
	if err := watch.add(base); err != nil {
		syscall.Close(fd)
		return nil, err
	}
	if !rootExists {
		return watch, nil
	}
	entries, err := os.ReadDir(watch.root)
	if err != nil {
		syscall.Close(fd)
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			_ = watch.add(filepath.Join(watch.root, entry.Name()))
		}
	}
	return watch, nil
}

func nearestWorkshopAncestor(root string) (string, bool, error) {
	path := root
	for {
		st, err := os.Stat(path)
		if err == nil {
			if !st.IsDir() {
				return "", false, fmt.Errorf("%s is not a directory", path)
			}
			return path, path == root, nil
		}
		if !os.IsNotExist(err) {
			return "", false, err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", false, err
		}
		path = parent
	}
}

func (w *workshopWatch) add(path string) error {
	path = filepath.Clean(path)
	if _, ok := w.byPath[path]; ok {
		return nil
	}
	mask := uint32(syscall.IN_CREATE | syscall.IN_CLOSE_WRITE | syscall.IN_DELETE |
		syscall.IN_MOVED_TO | syscall.IN_MOVED_FROM | syscall.IN_DELETE_SELF | syscall.IN_MOVE_SELF)
	wd, err := syscall.InotifyAddWatch(w.fd, path, mask)
	if err != nil {
		return err
	}
	w.wds[int32(wd)] = path
	w.byPath[path] = int32(wd)
	return nil
}

func (w *workshopWatch) remove(path string) {
	path = filepath.Clean(path)
	wd, ok := w.byPath[path]
	if !ok {
		return
	}
	_, _ = syscall.InotifyRmWatch(w.fd, uint32(wd))
	delete(w.byPath, path)
	delete(w.wds, wd)
}

func (w *workshopWatch) close() {
	_ = syscall.Close(w.fd)
}

func (w *workshopWatch) rootPresent() bool {
	st, err := os.Stat(w.root)
	return err == nil && st.IsDir()
}

func workshopPathLeadsToRoot(path, root string) bool {
	path = filepath.Clean(path)
	return path == root || strings.HasPrefix(root, path+string(filepath.Separator))
}

type workshopWatchRead struct {
	events []inotifyEvent
	err    error
}

func runWorkshopWatch(w *workshopWatch, debounce time.Duration, initial bool, rewatch <-chan struct{}, changed func()) (bool, error) {
	var timer *time.Timer
	schedule := func() {
		if timer == nil {
			timer = time.AfterFunc(debounce, changed)
			return
		}
		timer.Stop()
		timer.Reset(debounce)
	}
	if initial {
		schedule()
	}
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	reads := make(chan workshopWatchRead, 1)
	done := make(chan struct{})
	defer close(done)
	go func() {
		buf := make([]byte, 64*1024)
		for {
			n, err := syscall.Read(w.fd, buf)
			if err == syscall.EINTR {
				continue
			}
			read := workshopWatchRead{err: err}
			if err == nil {
				read.events = parseInotifyEvents(buf[:n])
			}
			select {
			case reads <- read:
			case <-done:
				return
			}
			if err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-rewatch:
			return true, nil
		case read := <-reads:
			if read.err != nil {
				return false, read.err
			}
			for _, ev := range read.events {
				if ev.mask&syscall.IN_Q_OVERFLOW != 0 {
					return true, nil
				}
				parent, ok := w.wds[ev.wd]
				if !ok {
					continue
				}
				if ev.mask&(syscall.IN_DELETE_SELF|syscall.IN_MOVE_SELF|syscall.IN_IGNORED) != 0 {
					delete(w.byPath, parent)
					delete(w.wds, ev.wd)
					if parent == w.root || workshopPathLeadsToRoot(parent, w.root) {
						return true, nil
					}
					schedule()
					continue
				}
				child := filepath.Join(parent, ev.name)
				if parent != w.root {
					if !w.rootPresent() {
						if ev.mask&syscall.IN_ISDIR != 0 &&
							ev.mask&(syscall.IN_CREATE|syscall.IN_MOVED_TO) != 0 &&
							workshopPathLeadsToRoot(child, w.root) {
							return true, nil
						}
						continue
					}
					if workshopPathLeadsToRoot(parent, w.root) {
						return true, nil
					}
				}
				if parent == w.root && ev.mask&syscall.IN_ISDIR != 0 {
					switch {
					case ev.mask&(syscall.IN_CREATE|syscall.IN_MOVED_TO) != 0:
						_ = w.add(child)
					case ev.mask&(syscall.IN_DELETE|syscall.IN_MOVED_FROM) != 0:
						w.remove(child)
					}
				}
				if parent == w.root || filepath.Dir(parent) == w.root {
					schedule()
				}
			}
		}
	}
}

func (w *workshopLib) watch() {
	if !w.d.featureSteam() {
		return
	}
	last := w.workshopSignature()
	watchedRoot := ""
	pathDirty := false
	var stateMu sync.Mutex
	changed := func() {
		if !w.d.featureSteam() {
			return
		}
		resolved, err := filepath.Abs(w.workshopDir())
		if err == nil {
			stateMu.Lock()
			pathChanged := watchedRoot != "" && filepath.Clean(resolved) != watchedRoot
			stateMu.Unlock()
			if pathChanged {
				w.requestRewatch()
				return
			}
		}
		sig := w.workshopSignature()
		stateMu.Lock()
		force := pathDirty
		pathDirty = false
		if sig == last && !force {
			stateMu.Unlock()
			return
		}
		last = sig
		stateMu.Unlock()
		w.d.rescan(false)
		w.d.broadcast("ryogami.workshop.changed", map[string]interface{}{})
	}
	initial := false
	fallbackLogged := false
	for {
		watcher, err := openWorkshopWatch(w.workshopDir())
		if err != nil {
			if !fallbackLogged {
				fmt.Fprintf(os.Stderr, "ryogami: workshop watch unavailable, polling every %s: %v\n", workshopFallbackPollInterval, err)
				fallbackLogged = true
			}
			w.pollWorkshop(&last, &stateMu, changed)
			initial = true
			continue
		}
		fallbackLogged = false
		current := w.workshopSignature()
		stateMu.Lock()
		if watchedRoot != "" && watchedRoot != watcher.root {
			pathDirty = true
			initial = true
		}
		watchedRoot = watcher.root
		if current != last {
			initial = true
		}
		stateMu.Unlock()
		rebuild, err := runWorkshopWatch(watcher, workshopWatchDebounce, initial, w.rewatch, changed)
		watcher.close()
		if err != nil {
			if !fallbackLogged {
				fmt.Fprintf(os.Stderr, "ryogami: workshop watch unavailable, polling every %s: %v\n", workshopFallbackPollInterval, err)
				fallbackLogged = true
			}
			w.pollWorkshop(&last, &stateMu, changed)
			initial = true
			continue
		}
		initial = rebuild
	}
}

func (w *workshopLib) pollWorkshop(last *string, stateMu *sync.Mutex, changed func()) {
	ticker := time.NewTicker(workshopFallbackPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-w.rewatch:
			return
		case <-ticker.C:
		}
		if !w.d.featureSteam() {
			continue
		}
		stateMu.Lock()
		before := *last
		stateMu.Unlock()
		if w.workshopSignature() != before {
			changed()
		}
	}
}

func (d *daemon) allowWorkshopDownloadEvent(id, status string, now time.Time) bool {
	d.workshopProgressMu.Lock()
	defer d.workshopProgressMu.Unlock()
	if d.workshopProgress == nil {
		d.workshopProgress = map[string]progressEventState{}
	}
	last, has := d.workshopProgress[id]
	if status != "downloading" {
		delete(d.workshopProgress, id)
		return true
	}
	if !has || last.status != status || now.Sub(last.at) >= progressMinInterval {
		d.workshopProgress[id] = progressEventState{status: status, at: now}
		return true
	}
	return false
}

func (d *daemon) emitWorkshopDownload(id, status string, progress float64, message, path string) {
	if !d.allowWorkshopDownloadEvent(id, status, time.Now()) {
		return
	}
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
	if d.tasks == nil {
		return
	}
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
	fetch := func(item string) error {
		if useSteamcmd {
			return w.steamcmdDownload(item, id)
		}
		return w.steamworksDownload(item, id)
	}
	if err := w.installChain(id, fetch); err != nil {
		status := "error"
		var fetchErr *workshopFetchError
		if errors.As(err, &fetchErr) && fetchErr.status != "" {
			status = fetchErr.status
		}
		d.emitWorkshopDownload(id, status, 0, err.Error(), "")
		return
	}
	d.emitWorkshopDownload(id, "done", 1.0, "", filepath.Join(w.workshopDir(), id))
	d.rescan(true)
	d.broadcast("ryogami.workshop.changed", map[string]interface{}{})
}

func (w *workshopLib) installChain(id string, fetch func(item string) error) error {
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
		if derr != nil || incomplete {
			if err := fetch(cur); err != nil {
				return err
			}
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
	if info, err := os.Lstat(target); err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return
		}
		link, readErr := os.Readlink(target)
		if readErr == nil && !filepath.IsAbs(link) {
			link = filepath.Join(filepath.Dir(target), link)
		}
		if readErr == nil && dirExists(link) {
			return
		}
		_ = os.Remove(target)
	}
	if actual == "" || !dirExists(actual) || actual == target {
		return
	}
	_ = os.MkdirAll(weDir, 0o755)
	_ = os.Symlink(actual, target)
}
