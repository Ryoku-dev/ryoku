package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
)

// clipboard.go owns the typed clipboard history the daemon streams to QML over
// the "clipboard" state topic. It keeps 100 entries, promotes duplicate content
// in place, limits stored selections to 10 MiB, and creates bounded image
// thumbnails. Copy re-sets the Wayland selection through wl-copy.
//
// The daemon also keeps the selection alive: on Wayland a clipboard belongs to
// the client that set it, so closing that window drops the content. The keeper
// retains every offered type and re-claims the selection when its owner exits.

const (
	clipMaxEntries      = 100       // history cap; oldest drops when a new entry overflows
	clipMaxEntryBytes   = 10 << 20  // 10 MiB per history entry; larger selections are not stored
	clipMaxStarredBytes = 100 << 20 // total disk budget for starred entries
	clipTextPreviewLen  = 200       // text preview length in characters
	clipThumbnailSize   = 512       // image thumbnail bounding box in pixels
	clipPersistEnv      = "RYOKU_CLIP_PERSIST"
)

type clipEntry struct {
	ID         uint64 `json:"id"`
	Kind       string `json:"kind"` // "text" | "image" | "binary"
	Mime       string `json:"mime"`
	Size       int    `json:"size"`
	Preview    string `json:"preview,omitempty"`
	ThumbPath  string `json:"thumb,omitempty"`
	ThumbW     int    `json:"thumbW,omitempty"`
	ThumbH     int    `json:"thumbH,omitempty"`
	W          int    `json:"w,omitempty"`
	H          int    `json:"h,omitempty"`
	Starred    bool   `json:"starred"`
	hash       uint64
	ts         time.Time
	thumbBytes int
}

// at is derived from ts on the way out so the persisted clock stays the only copy time
func (e *clipEntry) MarshalJSON() ([]byte, error) {
	type plain clipEntry
	var at int64
	if ms := e.ts.UnixMilli(); ms > 0 {
		at = ms
	}
	return json.Marshal(struct {
		*plain
		At int64 `json:"at,omitempty"`
	}{(*plain)(e), at})
}

type clipState struct {
	primary  primarySelectionState
	mu       sync.Mutex
	entries  []*clipEntry // newest at front
	nextID   uint64
	topic    *stateTopic
	cacheDir string // on-disk image thumbnails (cache)
	stateDir string // persisted history dir; "" disables on-disk persistence (tests)
	dataDir  string // persisted entry bytes (stateDir/data)
	// lastPrune is when the automatic sweep last ran; zero means it has never
	// run, and the next look starts the clock rather than pruning on the spot.
	lastPrune time.Time
}

// startClipboard registers the clipboard topic and control calls, publishes an
// empty snapshot, and starts the selection watcher.
func (d *daemon) startClipboard() {
	cache := clipCacheDir()
	_ = os.MkdirAll(cache, 0o700)
	state := clipStateDir()
	data := filepath.Join(state, "data")
	_ = os.MkdirAll(data, 0o700)
	s := &clipState{topic: d.registerTopic("clipboard"), cacheDir: cache, stateDir: state, dataDir: data}
	s.load()
	d.clip = s
	s.publish()

	d.registerCall("clipboard.copy", func(raw json.RawMessage) (any, error) {
		var a struct {
			Entry uint64 `json:"entry"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		return nil, s.copy(a.Entry)
	})
	d.registerCall("clipboard.delete", func(raw json.RawMessage) (any, error) {
		var a struct {
			Entry uint64 `json:"entry"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		s.del(a.Entry)
		return nil, nil
	})
	d.registerCall("clipboard.clear", func(json.RawMessage) (any, error) {
		s.clear()
		return nil, nil
	})
	d.registerCall("clipboard.star", func(raw json.RawMessage) (any, error) {
		var a struct {
			Entry   uint64 `json:"entry"`
			Starred bool   `json:"starred"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		return nil, s.star(a.Entry, a.Starred)
	})

	d.registerCall("clipboard.stats", func(json.RawMessage) (any, error) {
		return s.stats(), nil
	})

	go s.restoreSelection()
	go s.watch()
	go s.keepSelection()
	go s.autoPrune(func() bool {
		if d.settings == nil {
			return false
		}
		return d.settings.boolAt("clipboard.pruneWeekly")
	})
}

// clipCacheDir holds the on-disk image thumbnails, kept out of the state frames
// so a change to one entry never re-ships another entry's pixels.
func clipCacheDir() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".cache")
	}
	return filepath.Join(base, "ryoku", "clipboard")
}

// clipStateDir holds the persisted history: the entry index and one file of
// bytes per entry, so the clipboard survives a daemon restart (an in-memory-only
// history lost everything on relaunch).
func clipStateDir() string {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	return filepath.Join(base, "ryoku", "clipboard")
}

func (s *clipState) indexPath() string { return filepath.Join(s.stateDir, "index.json") }
func (s *clipState) dataPath(hash uint64) string {
	return filepath.Join(s.dataDir, fmt.Sprintf("%016x.bin", hash))
}

func (s *clipState) openPayload(hash uint64) (*os.File, error) {
	if s.dataDir == "" {
		return nil, fmt.Errorf("clipboard payload is not persisted")
	}
	return os.Open(s.dataPath(hash))
}

// persistedClip is the on-disk history: entry metadata plus the next id. Each
// entry's bytes live beside it in dataDir keyed by hash and its thumbnail stays
// in the cache dir, so the index stays small and one entry never rewrites another.
type persistedClip struct {
	NextID    uint64           `json:"nextID"`
	LastPrune int64            `json:"lastPrune,omitempty"` // unix seconds; 0 = never
	Entries   []persistedEntry `json:"entries"`
}

type persistedEntry struct {
	ID         uint64 `json:"id"`
	Kind       string `json:"kind"`
	Mime       string `json:"mime"`
	Size       int    `json:"size"`
	Preview    string `json:"preview,omitempty"`
	ThumbPath  string `json:"thumb,omitempty"`
	ThumbW     int    `json:"thumbW,omitempty"`
	ThumbH     int    `json:"thumbH,omitempty"`
	W          int    `json:"w,omitempty"`
	H          int    `json:"h,omitempty"`
	ThumbBytes int    `json:"thumbBytes,omitempty"`
	Starred    bool   `json:"starred,omitempty"`
	Hash       uint64 `json:"hash"`
	TS         int64  `json:"ts"`
}

// writeData persists one entry's bytes once, keyed by content hash. Skipped when
// persistence is off (dataDir empty, as in tests).
func (s *clipState) writeData(hash uint64, data []byte) {
	if s.dataDir == "" {
		return
	}
	p := s.dataPath(hash)
	if _, err := os.Stat(p); err == nil {
		return
	}
	tmp := p + ".tmp"
	if os.WriteFile(tmp, data, 0o600) == nil {
		_ = os.Rename(tmp, p)
	}
}

// persistLocked writes the entry index atomically. The caller holds s.mu.
func (s *clipState) persistLocked() {
	if s.stateDir == "" {
		return
	}
	p := persistedClip{NextID: s.nextID, Entries: make([]persistedEntry, 0, len(s.entries))}
	if !s.lastPrune.IsZero() {
		p.LastPrune = s.lastPrune.Unix()
	}
	for _, e := range s.entries {
		p.Entries = append(p.Entries, persistedEntry{
			ID: e.ID, Kind: e.Kind, Mime: e.Mime, Size: e.Size, Preview: e.Preview,
			ThumbPath: e.ThumbPath, ThumbW: e.ThumbW, ThumbH: e.ThumbH, W: e.W, H: e.H,
			ThumbBytes: e.thumbBytes, Starred: e.Starred, Hash: e.hash, TS: e.ts.UnixNano(),
		})
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return
	}
	tmp := s.indexPath() + ".tmp"
	if os.WriteFile(tmp, raw, 0o600) == nil {
		_ = os.Rename(tmp, s.indexPath())
	}
}

// load restores metadata without loading payload files.
func (s *clipState) load() {
	if s.stateDir == "" {
		return
	}
	raw, err := os.ReadFile(s.indexPath())
	if err != nil {
		return
	}
	var p persistedClip
	if json.Unmarshal(raw, &p) != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID = p.NextID
	if p.LastPrune > 0 {
		s.lastPrune = time.Unix(p.LastPrune, 0)
	}
	keep := make(map[uint64]bool, len(p.Entries))
	thumbs := make(map[string]bool, len(p.Entries))
	changed := false
	var starredBytes int64
	for _, pe := range p.Entries {
		fi, err := os.Stat(s.dataPath(pe.Hash))
		if err != nil || fi.Size() == 0 {
			continue
		}
		if pe.Kind != "text" && fi.Size() == clipMaxEntryBytes {
			_ = os.Remove(s.dataPath(pe.Hash))
			if pe.ThumbPath != "" {
				_ = os.Remove(pe.ThumbPath)
			}
			changed = true
			continue
		}
		e := &clipEntry{
			ID: pe.ID, Kind: pe.Kind, Mime: pe.Mime, Size: pe.Size, Preview: pe.Preview,
			ThumbPath: pe.ThumbPath, ThumbW: pe.ThumbW, ThumbH: pe.ThumbH, W: pe.W, H: pe.H,
			Starred: pe.Starred, hash: pe.Hash, ts: time.Unix(0, pe.TS), thumbBytes: pe.ThumbBytes,
		}
		if e.Size <= 0 {
			e.Size = int(fi.Size())
			changed = true
		}
		if e.ThumbPath != "" {
			fi, err := os.Stat(e.ThumbPath)
			if err != nil {
				e.ThumbPath = ""
				e.thumbBytes = 0
				if e.Kind == "image" {
					e.Kind = "binary"
				}
			} else if e.thumbBytes == 0 {
				e.thumbBytes = int(fi.Size())
			}
		}
		if e.Starred {
			if n := starredBytes + e.storedBytes(); n <= clipMaxStarredBytes {
				starredBytes = n
			} else {
				e.Starred = false
				changed = true
			}
		}
		if e.ID > s.nextID {
			s.nextID = e.ID
		}
		s.entries = append(s.entries, e)
		keep[pe.Hash] = true
		if e.ThumbPath != "" {
			thumbs[filepath.Base(e.ThumbPath)] = true
		}
	}
	if files, err := os.ReadDir(s.dataDir); err == nil {
		for _, f := range files {
			name := f.Name()
			if !strings.HasSuffix(name, ".bin") {
				continue
			}
			if h, err := strconv.ParseUint(strings.TrimSuffix(name, ".bin"), 16, 64); err == nil && !keep[h] {
				_ = os.Remove(filepath.Join(s.dataDir, name))
			}
		}
	}
	// Thumbnails get the same sweep as the bytes they belong to: a thumbnail
	// whose entry is gone is invisible to the user but still occupies the disk,
	// and the storage report would under-count the history by exactly that much.
	if files, err := os.ReadDir(s.cacheDir); err == nil {
		for _, f := range files {
			name := f.Name()
			if strings.HasSuffix(name, ".png") && !thumbs[name] {
				_ = os.Remove(filepath.Join(s.cacheDir, name))
			}
		}
	}
	if changed {
		s.persistLocked()
	}
}

// clipHash keys dedup. The algorithm only needs to be stable within a run, so
// FNV over the raw bytes is enough; the reference uses its own default hasher.
func clipHash(data []byte) uint64 {
	h := fnv.New64a()
	_, _ = h.Write(data)
	return h.Sum64()
}

func truncateBytesRunes(data []byte, n int) string {
	if n <= 0 {
		return ""
	}
	end := 0
	for range n {
		if end == len(data) {
			return string(data)
		}
		_, size := utf8.DecodeRune(data[end:])
		end += size
	}
	if end == len(data) {
		return string(data)
	}
	if utf8.Valid(data[:end]) {
		return string(data[:end])
	}
	return string([]rune(string(data[:end])))
}

// classifyClip derives an entry's kind and preview from its mime and bytes:
// text/* keeps a 200-character preview; image/* renders a thumbnail (or falls to
// binary when it will not decode); everything else is binary. It is pure so the
// rules are unit-tested without touching disk.
func classifyClip(mime string, data []byte) (kind, preview string, thumb []byte, thumbW, thumbH, w, h int) {
	switch {
	case strings.HasPrefix(mime, "text/"):
		return "text", truncateBytesRunes(data, clipTextPreviewLen), nil, 0, 0, 0, 0
	case strings.HasPrefix(mime, "image/"):
		if t, tw, th, sw, sh, ok := thumbnailPNG(data, clipThumbnailSize); ok {
			return "image", "", t, tw, th, sw, sh
		}
		return "binary", "", nil, 0, 0, 0, 0
	default:
		return "binary", "", nil, 0, 0, 0, 0
	}
}

// thumbnailPNG decodes an image and scales it (aspect preserved, never upscaled)
// into a PNG that fits a max*max box, returning the encoded bytes, thumbnail
// size, and source size. ok is false when the bytes are not a decodable image,
// which the caller treats as a binary entry.
func thumbnailPNG(data []byte, max int) (encoded []byte, thumbW, thumbH, sourceW, sourceH int, ok bool) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, 0, 0, false
	}
	b := img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw <= 0 || sh <= 0 {
		return nil, 0, 0, 0, 0, false
	}
	nw, nh := fitBox(sw, sh, max)
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := range nh {
		sy := y * sh / nh
		for x := range nw {
			sx := x * sw / nw
			dst.Set(x, y, img.At(b.Min.X+sx, b.Min.Y+sy))
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, 0, 0, 0, 0, false
	}
	return buf.Bytes(), nw, nh, sw, sh, true
}

// fitBox returns the largest w*h that fits a max*max box at the source aspect
// ratio, never enlarging a source already inside the box.
func fitBox(w, h, max int) (int, int) {
	if w <= max && h <= max {
		return w, h
	}
	if w >= h {
		nh := max * h / w
		if nh < 1 {
			nh = 1
		}
		return max, nh
	}
	nw := max * w / h
	if nw < 1 {
		nw = 1
	}
	return nw, max
}

// buildEntry turns captured bytes into an entry, persisting an image thumbnail
// to the cache dir when one was generated.
func (s *clipState) buildEntry(mime string, data []byte) *clipEntry {
	kind, preview, thumb, thumbW, thumbH, w, h := classifyClip(mime, data)
	e := &clipEntry{
		Kind:    kind,
		Mime:    mime,
		Size:    len(data),
		Preview: preview,
		ThumbW:  thumbW,
		ThumbH:  thumbH,
		W:       w,
		H:       h,
		hash:    clipHash(data),
		ts:      time.Now(),
	}
	if thumb != nil {
		p := filepath.Join(s.cacheDir, fmt.Sprintf("thumb-%016x.png", e.hash))
		if os.WriteFile(p, thumb, 0o600) == nil {
			e.ThumbPath = p
			e.thumbBytes = len(thumb)
		} else {
			// A thumbnail we cannot persist degrades to a binary entry rather
			// than a broken image reference.
			e.Kind, e.ThumbW, e.ThumbH, e.W, e.H = "binary", 0, 0, 0, 0
		}
	}
	return e
}

// pushLocked inserts an entry at the front. A content-hash match promotes the
// existing entry instead of keeping a duplicate: it is removed and re-inserted
// at the front reusing its id and its starred flag, so references and the
// Starred pane stay intact. A fresh entry takes the next id, and an overflow
// past the cap drops the oldest unstarred entry.
func (s *clipState) pushLocked(e *clipEntry) {
	for i, ex := range s.entries {
		if ex.hash == e.hash {
			e.ID = ex.ID
			e.Starred = ex.Starred
			s.entries = append(s.entries[:i], s.entries[i+1:]...)
			s.entries = append([]*clipEntry{e}, s.entries...)
			return
		}
	}
	s.nextID++
	e.ID = s.nextID
	s.entries = append([]*clipEntry{e}, s.entries...)
	if len(s.entries) > clipMaxEntries {
		s.dropOldestUnstarredLocked()
	}
}

// dropOldestUnstarredLocked enforces the cap by evicting the oldest unstarred
// entry and its backing files, leaving starred entries untouched. When every
// entry is starred nothing is dropped, so a deliberately kept item survives an
// overflow that an ordinary entry would have absorbed.
func (s *clipState) dropOldestUnstarredLocked() {
	for i := len(s.entries) - 1; i >= 0; i-- {
		if s.entries[i].Starred {
			continue
		}
		old := s.entries[i]
		s.entries = append(s.entries[:i], s.entries[i+1:]...)
		if old.ThumbPath != "" {
			_ = os.Remove(old.ThumbPath)
		}
		if s.dataDir != "" {
			_ = os.Remove(s.dataPath(old.hash))
		}
		return
	}
}

// promoteLocked moves an existing entry to the front and refreshes its
// timestamp, keeping the same id. Returns nil when the id is unknown.
func (s *clipState) promoteLocked(id uint64) *clipEntry {
	for i, e := range s.entries {
		if e.ID == id {
			e.ts = time.Now()
			s.entries = append(s.entries[:i], s.entries[i+1:]...)
			s.entries = append([]*clipEntry{e}, s.entries...)
			return e
		}
	}
	return nil
}

func (e *clipEntry) storedBytes() int64 {
	return int64(e.Size) + int64(e.thumbBytes)
}

func (s *clipState) starredBytesLocked() int64 {
	var total int64
	for _, e := range s.entries {
		if e.Starred {
			total += e.storedBytes()
		}
	}
	return total
}

// star sets an entry's starred flag while keeping starred storage bounded.
func (s *clipState) star(id uint64, starred bool) error {
	s.mu.Lock()
	var found *clipEntry
	for _, e := range s.entries {
		if e.ID == id {
			found = e
			break
		}
	}
	if found == nil {
		s.mu.Unlock()
		return fmt.Errorf("no clipboard entry %d", id)
	}
	if starred && !found.Starred && s.starredBytesLocked()+found.storedBytes() > clipMaxStarredBytes {
		s.mu.Unlock()
		return fmt.Errorf("starred clipboard limit reached (%d MiB); unstar an item first", clipMaxStarredBytes>>20)
	}
	found.Starred = starred
	s.persistLocked()
	s.mu.Unlock()
	s.publish()
	return nil
}

// ingest builds and stores a complete captured selection, dropping empty data
// and selections past the entry cap.
func (s *clipState) ingest(mime string, data []byte) {
	if len(data) == 0 || len(data) > clipMaxEntryBytes {
		return
	}
	e := s.buildEntry(mime, data)
	s.writeData(e.hash, data)
	s.mu.Lock()
	s.pushLocked(e)
	s.persistLocked()
	s.mu.Unlock()
	s.publish()
}

// copy re-sets the Wayland selection from its persisted payload and promotes it.
func (s *clipState) copy(id uint64) error {
	s.mu.Lock()
	var found *clipEntry
	for _, e := range s.entries {
		if e.ID == id {
			found = e
			break
		}
	}
	if found == nil {
		s.mu.Unlock()
		return fmt.Errorf("no clipboard entry %d", id)
	}
	payload, err := s.openPayload(found.hash)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("open clipboard entry %d: %w", id, err)
	}
	mime := found.Mime
	s.promoteLocked(id)
	s.persistLocked()
	s.mu.Unlock()
	defer payload.Close()

	s.publish()
	cmd := exec.Command("wl-copy", "--type", mime)
	cmd.Stdin = payload
	return cmd.Run()
}

// copyFile streams a file to the selection before adding it to history.
func (s *clipState) copyFile(mime, path string) error {
	payload, err := os.Open(path)
	if err != nil {
		return err
	}
	fi, err := payload.Stat()
	if err != nil {
		payload.Close()
		return err
	}
	if fi.Size() == 0 {
		payload.Close()
		return fmt.Errorf("clip-copy: empty file %s", path)
	}
	cmd := exec.Command("wl-copy", "--type", mime)
	cmd.Stdin = payload
	runErr := cmd.Run()
	closeErr := payload.Close()
	if runErr != nil {
		return runErr
	}
	if closeErr != nil {
		return closeErr
	}
	if fi.Size() <= clipMaxEntryBytes {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		s.ingest(mime, data)
	}
	return nil
}

func (s *clipState) del(id uint64) {
	s.mu.Lock()
	for i, e := range s.entries {
		if e.ID == id {
			if e.ThumbPath != "" {
				_ = os.Remove(e.ThumbPath)
			}
			if s.dataDir != "" {
				_ = os.Remove(s.dataPath(e.hash))
			}
			s.entries = append(s.entries[:i], s.entries[i+1:]...)
			break
		}
	}
	s.persistLocked()
	s.mu.Unlock()
	s.publish()
}

// clear empties the history but keeps starred entries and their backing files,
// so the Starred pane is genuinely safe from a clear.
func (s *clipState) clear() {
	s.mu.Lock()
	kept := s.entries[:0]
	for _, e := range s.entries {
		if e.Starred {
			kept = append(kept, e)
			continue
		}
		if e.ThumbPath != "" {
			_ = os.Remove(e.ThumbPath)
		}
		if s.dataDir != "" {
			_ = os.Remove(s.dataPath(e.hash))
		}
	}
	s.entries = kept
	s.persistLocked()
	s.mu.Unlock()
	s.publish()
}

// publish ships the whole history as one frame. An empty history is an empty
// array, never null, so QML always sees a defined model.
func (s *clipState) publish() {
	if s.topic == nil {
		return
	}
	s.mu.Lock()
	ents := s.entries
	if ents == nil {
		ents = []*clipEntry{}
	}
	frame, err := json.Marshal(map[string]any{"entries": ents})
	s.mu.Unlock()
	if err != nil {
		return
	}
	s.topic.publish(frame)
}

// browserInternalMime reports a selection type that carries no user content.
// Chromium publishes private marker formats alongside the real write, and
// sometimes as a write of its own: the frame/tab token
// (chromium/x-internal-source-rfh-token) and the provenance marker
// (chromium/x-source-url) each landed in the history as junk of their own.
// Every chromium/x- type is a Chromium-private format no other application can
// consume, so the whole prefix is treated as a marker rather than a blocklist
// that has to grow with each Chromium release; a write offering only markers
// is skipped rather than stored as an entry of its own.
func browserInternalMime(t string) bool {
	return strings.HasPrefix(t, "chromium/x-")
}

// pickBestMime chooses which offered type to store, matching the reference
// priority: the text types first, then the image types, then whatever is offered
// first. Types that carry no user content are dropped before the choice, so a
// marker alone yields no type (the caller stores nothing) and a marker listed
// ahead of a real type cannot be picked by the fallback.
func pickBestMime(offered []string) string {
	usable := make([]string, 0, len(offered))
	has := make(map[string]bool, len(offered))
	for _, t := range offered {
		if browserInternalMime(t) {
			continue
		}
		usable = append(usable, t)
		has[t] = true
	}
	for _, t := range []string{"text/plain;charset=utf-8", "text/plain", "UTF8_STRING", "STRING", "TEXT"} {
		if has[t] {
			return t
		}
	}
	for _, t := range []string{"image/png", "image/jpeg", "image/bmp", "image/tiff"} {
		if has[t] {
			return t
		}
	}
	if len(usable) > 0 {
		return usable[0]
	}
	return ""
}

// clipPersistBin is the selection keeper: the one program that makes a copy
// outlive the client that made it.
const clipPersistBin = "wl-clip-persist"

// keepSelection runs the selection keeper for the daemon's life, re-attaching if
// it drops. A Wayland clipboard is owned by the client that set it, so closing
// the window -- or the browser tab -- that copied drops the content and the next
// paste finds nothing; the keeper holds every offered mime type and re-claims the
// selection when its owner exits. Without it installed the copy keeps the old
// lifetime: still in the history, but gone from the selection.
func (s *clipState) keepSelection() {
	reapStrayClipPersist()
	for {
		cmd := exec.Command(clipPersistBin, "--clipboard", "regular")
		// Marker for the stray reaper: it must never kill a keeper the user runs
		// themselves, and it must recognise ours across daemon restarts.
		cmd.Env = append(os.Environ(), clipPersistEnv+"=1")
		// Tie it to us, so a hard daemon exit cannot strand a process holding a
		// data-control slot.
		cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
		if err := cmd.Start(); err != nil {
			return // not installed; the copy keeps the old lifetime
		}
		_ = cmd.Wait()
		time.Sleep(2 * time.Second)
	}
}

// reapStrayClipPersist kills selection keepers stranded by a previous daemon.
// Pdeathsig ties a keeper to the daemon that started it, but a force-killed
// daemon (or a logout that skipped the exit path) can leave one reparented to
// init and holding a data-control slot for the rest of the login. Only a keeper
// carrying our own environment marker is touched.
func reapStrayClipPersist() {
	me := os.Getpid()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == me {
			continue
		}
		raw, err := os.ReadFile("/proc/" + e.Name() + "/cmdline")
		if err != nil {
			continue
		}
		if !isClipPersistCmdline(strings.Split(string(raw), "\x00")) {
			continue
		}
		env, err := os.ReadFile("/proc/" + e.Name() + "/environ")
		if err != nil || !strings.Contains(string(env), clipPersistEnv+"=1") {
			continue
		}
		if p, err := os.FindProcess(pid); err == nil {
			_ = p.Signal(syscall.SIGKILL)
		}
	}
}

// isClipPersistCmdline reports whether argv is a selection keeper, by the program
// name alone: the environment marker decides ownership.
func isClipPersistCmdline(argv []string) bool {
	for _, a := range argv {
		if filepath.Base(a) == clipPersistBin {
			return true
		}
	}
	return false
}

// clipStats is the storage report of the whole history: how much it occupies,
// split into the two things a user can picture (text and images), and how much
// of it is pinned by starring. Bytes are everything the history keeps on disk --
// each entry's bytes plus the thumbnail an image entry generated.
type clipStats struct {
	Items      int   `json:"items"`
	Starred    int   `json:"starred"`
	Bytes      int64 `json:"bytes"`
	TextBytes  int64 `json:"textBytes"`
	ImageBytes int64 `json:"imageBytes"`
	OtherBytes int64 `json:"otherBytes"`
	LastPrune  int64 `json:"lastPrune,omitempty"` // unix seconds; 0 = never
}

// stats walks the history once. Sizes are held in memory, so this is a sum, not
// a filesystem walk.
func (s *clipState) stats() clipStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := clipStats{Items: len(s.entries)}
	if !s.lastPrune.IsZero() {
		st.LastPrune = s.lastPrune.Unix()
	}
	for _, e := range s.entries {
		n := e.storedBytes()
		st.Bytes += n
		if e.Starred {
			st.Starred++
		}
		switch e.Kind {
		case "text":
			st.TextBytes += n
		case "image":
			st.ImageBytes += n
		default:
			st.OtherBytes += n
		}
	}
	return st
}

const (
	// clipPruneEvery is the automatic sweep period: a weekly drop of everything
	// the user has not starred.
	clipPruneEvery = 7 * 24 * time.Hour
	// clipPruneCheck is how often the sweep is re-examined. It is the daemon's
	// own clock, so a short period costs nothing and means switching the setting
	// on starts the week promptly instead of at the next day's boundary.
	clipPruneCheck = 15 * time.Minute
)

// autoPrune drops unstarred history once a week while the setting is on. The
// timestamp rides the persisted index, so a restart, a reboot or a shell reload
// cannot make it prune twice; and turning the setting on starts the clock rather
// than deleting anything on the spot. Starred entries are never touched -- they
// are the user's own list, not a cache.
func (s *clipState) autoPrune(enabled func() bool) {
	for {
		s.pruneIfDue(enabled(), time.Now())
		time.Sleep(clipPruneCheck)
	}
}

// pruneIfDue runs the sweep when it is due and reports whether it pruned. The
// first look after the setting is switched on only starts the clock.
func (s *clipState) pruneIfDue(on bool, now time.Time) bool {
	if !on {
		return false
	}
	s.mu.Lock()
	last := s.lastPrune
	if last.IsZero() {
		s.lastPrune = now
		s.persistLocked()
		s.mu.Unlock()
		return false
	}
	if now.Sub(last) < clipPruneEvery {
		s.mu.Unlock()
		return false
	}
	s.lastPrune = now
	s.mu.Unlock()
	// clear is the prune: it drops every unstarred entry with its files and keeps
	// the starred ones, then persists the new clock.
	s.clear()
	return true
}

// clipCopy is the `clip-copy <mime> <path>` control verb: the daemon reads the
// file, sets the selection, and stores it in history. Capture and ryoshot call
// it so the copy is owned by the persistent daemon, not a short-lived app whose
// exit would clear the selection.
func (d *daemon) clipCopy(cmd string) string {
	if d.clip == nil {
		return "err clipboard not running"
	}
	fields := strings.SplitN(cmd, " ", 3)
	if len(fields) != 3 || fields[1] == "" || fields[2] == "" {
		return "err clip-copy: usage clip-copy <mime> <path>"
	}
	if err := d.clip.copyFile(fields[1], fields[2]); err != nil {
		return "err clip-copy: " + err.Error()
	}
	return "ok"
}
