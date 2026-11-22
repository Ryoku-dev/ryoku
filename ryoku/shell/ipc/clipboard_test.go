package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func clipIDs(s *clipState) []uint64 {
	ids := make([]uint64, len(s.entries))
	for i, e := range s.entries {
		ids[i] = e.ID
	}
	return ids
}

func tinyPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// The history rules are the clipboard's contract: newest-first order, one id per
// distinct content, a repeat promoted to the front reusing its id (no
// duplicate), and a hard 100-entry cap that drops the oldest.
func TestClipHistoryRules(t *testing.T) {
	s := &clipState{}
	for i, h := range []uint64{100, 200, 300} {
		s.pushLocked(&clipEntry{hash: h, Kind: "text", Preview: fmt.Sprintf("e%d", i)})
	}
	if got := clipIDs(s); !reflect.DeepEqual(got, []uint64{3, 2, 1}) {
		t.Fatalf("three pushes, ids front-to-back = %v, want [3 2 1]", got)
	}

	// A repeat of the oldest content promotes to the front and reuses its id.
	s.pushLocked(&clipEntry{hash: 100, Kind: "text", Preview: "dup"})
	if len(s.entries) != 3 {
		t.Fatalf("dedup kept a duplicate: len = %d, want 3", len(s.entries))
	}
	if got := clipIDs(s); !reflect.DeepEqual(got, []uint64{1, 3, 2}) {
		t.Fatalf("after dedup, ids front-to-back = %v, want [1 3 2]", got)
	}
	if s.entries[0].Preview != "dup" {
		t.Errorf("promoted entry not refreshed: preview = %q", s.entries[0].Preview)
	}

	// The cap drops the oldest once a new entry overflows it.
	s = &clipState{}
	n := clipMaxEntries + 5
	for i := range n {
		s.pushLocked(&clipEntry{hash: uint64(1000 + i)})
	}
	if len(s.entries) != clipMaxEntries {
		t.Fatalf("cap not enforced: len = %d, want %d", len(s.entries), clipMaxEntries)
	}
	if s.entries[0].ID != uint64(n) {
		t.Errorf("front id = %d, want %d (newest)", s.entries[0].ID, n)
	}
	if s.entries[len(s.entries)-1].ID != 6 {
		t.Errorf("oldest surviving id = %d, want 6 (ids 1-5 evicted)", s.entries[len(s.entries)-1].ID)
	}
}

// copy_entry promotes an existing entry to the front, keeping its id.
func TestClipPromote(t *testing.T) {
	s := &clipState{}
	for _, h := range []uint64{10, 20, 30} {
		s.pushLocked(&clipEntry{hash: h})
	}
	if e := s.promoteLocked(1); e == nil {
		t.Fatal("promoteLocked(1) = nil, want the id-1 entry")
	}
	if got := clipIDs(s); !reflect.DeepEqual(got, []uint64{1, 3, 2}) {
		t.Fatalf("after promote, ids = %v, want [1 3 2]", got)
	}
	if s.promoteLocked(99) != nil {
		t.Error("promoteLocked of an unknown id should be nil")
	}
}

func clipFrame(t *testing.T, s *clipState) []map[string]any {
	t.Helper()
	s.publish()
	var frame struct {
		Entries []map[string]any `json:"entries"`
	}
	if err := json.Unmarshal(s.topic.last, &frame); err != nil {
		t.Fatalf("decode frame %s: %v", s.topic.last, err)
	}
	return frame.Entries
}

// Every frame entry carries at, its copy time in Unix milliseconds, so the
// palette can show and group by age; an entry with no known time leaves it out.
func TestClipFrameCarriesCopyTime(t *testing.T) {
	copied := time.UnixMilli(1_700_000_000_123)
	s := &clipState{topic: newStateTopic()}
	s.pushLocked(&clipEntry{hash: 1, Kind: "text", Mime: "text/plain", Size: 5, Preview: "hello"})
	s.pushLocked(&clipEntry{
		hash: 2, Kind: "image", Mime: "image/png", Size: 2048, ThumbPath: "/cache/thumb-0000000000000002.png",
		ThumbW: 256, ThumbH: 128, W: 1024, H: 512, Starred: true, ts: copied,
	})

	ents := clipFrame(t, s)
	if len(ents) != 2 {
		t.Fatalf("frame has %d entries, want 2", len(ents))
	}
	raw, _ := json.Marshal(ents[0])
	t.Logf("sample frame entry: %s", raw)
	want := map[string]any{
		"id": 2.0, "kind": "image", "mime": "image/png", "size": 2048.0,
		"thumb": "/cache/thumb-0000000000000002.png", "thumbW": 256.0, "thumbH": 128.0,
		"w": 1024.0, "h": 512.0, "starred": true, "at": float64(copied.UnixMilli()),
	}
	if !reflect.DeepEqual(ents[0], want) {
		t.Errorf("timed entry = %v, want %v", ents[0], want)
	}
	wantUnknown := map[string]any{
		"id": 1.0, "kind": "text", "mime": "text/plain", "size": 5.0, "preview": "hello", "starred": false,
	}
	if !reflect.DeepEqual(ents[1], wantUnknown) {
		t.Errorf("untimed entry = %v, want %v (no at)", ents[1], wantUnknown)
	}

	s.entries[1].ts = time.Unix(0, 0)
	if _, ok := clipFrame(t, s)[1]["at"]; ok {
		t.Error("an entry restored with a zero persisted ts should leave at out")
	}

	s.mu.Lock()
	s.promoteLocked(2)
	s.mu.Unlock()
	after := clipFrame(t, s)
	at, _ := after[0]["at"].(float64)
	if after[0]["id"] != 2.0 || at <= float64(copied.UnixMilli()) {
		t.Errorf("after promote, front entry = %v, want id 2 with at past %d", after[0], copied.UnixMilli())
	}
}

// classifyClip is the preview contract: text keeps a 200-character preview,
// a decodable image gets a thumbnail, an undecodable image falls to binary, and
// any other type is binary.
func TestClassifyClip(t *testing.T) {
	kind, preview, thumb, _, _, _, _ := classifyClip("text/plain", []byte(strings.Repeat("a", 250)))
	if kind != "text" || len([]rune(preview)) != clipTextPreviewLen || thumb != nil {
		t.Errorf("text classify = (%q, %d-rune preview, thumb=%v), want (text, 200, false)", kind, len([]rune(preview)), thumb != nil)
	}

	kind, _, thumb, thumbW, thumbH, w, h := classifyClip("image/png", tinyPNG(t, 8, 4))
	if kind != "image" || thumb == nil || thumbW != 8 || thumbH != 4 || w != 8 || h != 4 {
		t.Errorf("png classify = (%q, thumb=%v, thumb=%dx%d, source=%dx%d), want (image, true, 8x4, 8x4)", kind, thumb != nil, thumbW, thumbH, w, h)
	}

	kind, _, thumb, _, _, _, _ = classifyClip("image/png", []byte("not a png"))
	if kind != "binary" || thumb != nil {
		t.Errorf("undecodable image classify = (%q, thumb=%v), want (binary, false)", kind, thumb != nil)
	}

	if kind, _, _, _, _, _, _ := classifyClip("application/pdf", []byte("%PDF")); kind != "binary" {
		t.Errorf("pdf classify = %q, want binary", kind)
	}
}

// pickBestMime reproduces the reference priority: text types, then image types,
// then the first offered.
func TestPickBestMime(t *testing.T) {
	cases := []struct {
		offered []string
		want    string
	}{
		{[]string{"text/html", "text/plain", "text/plain;charset=utf-8"}, "text/plain;charset=utf-8"},
		{[]string{"text/html", "text/plain"}, "text/plain"},
		{[]string{"image/bmp", "STRING", "TEXT"}, "STRING"},
		{[]string{"image/bmp", "image/png"}, "image/png"},
		{[]string{"image/tiff", "image/jpeg"}, "image/jpeg"},
		{[]string{"application/x-thing"}, "application/x-thing"},
		// A browser's private marker is never stored as an entry of its own, and
		// never wins the fallback against a real type offered beside it. Every
		// chromium/x- type is private, not just the x-internal- family.
		{[]string{"chromium/x-internal-source-rfh-token"}, ""},
		{[]string{"chromium/x-source-url"}, ""},
		{[]string{"chromium/x-web-custom-data"}, ""},
		{[]string{"chromium/x-source-url", "image/png"}, "image/png"},
		{[]string{"chromium/x-source-url", "text/plain"}, "text/plain"},
		{[]string{"chromium/x-source-url", "chromium/x-internal-source-rfh-token"}, ""},
		{[]string{"chromium/x-internal-source-rfh-token", "image/png"}, "image/png"},
		{nil, ""},
	}
	for _, c := range cases {
		if got := pickBestMime(c.offered); got != c.want {
			t.Errorf("pickBestMime(%v) = %q, want %q", c.offered, got, c.want)
		}
	}
}

// History only accepts complete entries.
func TestClipIngestSizeRules(t *testing.T) {
	s := &clipState{}
	s.ingest("text/plain", nil)
	s.ingest("text/plain", []byte{})
	s.ingest("text/plain", bytes.Repeat([]byte("a"), clipMaxEntryBytes+1))
	if len(s.entries) != 0 {
		t.Fatalf("invalid selections stored: %d entries", len(s.entries))
	}
}

func TestClipCopyFileKeepsCompleteOversizedSelection(t *testing.T) {
	bin := t.TempDir()
	stdinPath := filepath.Join(t.TempDir(), "stdin")
	lengthPath := filepath.Join(t.TempDir(), "length")
	hashPath := filepath.Join(t.TempDir(), "hash")
	writeCommandStub(t, bin, "wl-copy", `cat > "$CLIP_COPY_STDIN"
wc -c < "$CLIP_COPY_STDIN" > "$CLIP_COPY_LENGTH"
sha256sum "$CLIP_COPY_STDIN" > "$CLIP_COPY_HASH"`)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CLIP_COPY_STDIN", stdinPath)
	t.Setenv("CLIP_COPY_LENGTH", lengthPath)
	t.Setenv("CLIP_COPY_HASH", hashPath)

	data := bytes.Repeat([]byte("complete-selection-"), clipMaxEntryBytes/19+2)
	path := filepath.Join(t.TempDir(), "capture.png")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	s := &clipState{}
	if err := s.copyFile("image/png", path); err != nil {
		t.Fatal(err)
	}
	length, err := os.ReadFile(lengthPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(string(length)), fmt.Sprint(len(data)); got != want {
		t.Errorf("wl-copy stdin length = %s, want %s", got, want)
	}
	hash, err := os.ReadFile(hashPath)
	if err != nil {
		t.Fatal(err)
	}
	hashFields := strings.Fields(string(hash))
	if len(hashFields) == 0 {
		t.Fatal("wl-copy did not record a stdin hash")
	}
	if got, want := hashFields[0], fmt.Sprintf("%x", sha256.Sum256(data)); got != want {
		t.Errorf("wl-copy stdin hash = %s, want %s", got, want)
	}
	if len(s.entries) != 0 {
		t.Fatalf("oversized selection stored in history: %d entries", len(s.entries))
	}
}

func TestClipLoadDropsCapLengthImage(t *testing.T) {
	stateDir := t.TempDir()
	dataDir := filepath.Join(stateDir, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	data := bytes.Repeat([]byte{0x89}, clipMaxEntryBytes)
	hash := clipHash(data)
	thumbPath := filepath.Join(cacheDir, "thumb-legacy.png")
	if err := os.WriteFile(thumbPath, []byte("thumb"), 0o600); err != nil {
		t.Fatal(err)
	}
	legacy := &clipState{
		stateDir: stateDir,
		dataDir:  dataDir,
		cacheDir: cacheDir,
		nextID:   1,
		entries: []*clipEntry{{
			ID: 1, Kind: "image", Mime: "image/png", Size: len(data),
			ThumbPath: thumbPath, hash: hash,
		}},
	}
	legacy.writeData(hash, data)
	legacy.persistLocked()

	loaded := &clipState{stateDir: stateDir, dataDir: dataDir, cacheDir: cacheDir}
	loaded.load()
	if len(loaded.entries) != 0 {
		t.Fatalf("loaded %d corrupt image entries, want 0", len(loaded.entries))
	}
	for _, path := range []string{legacy.dataPath(hash), thumbPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("corrupt entry file %q still exists: %v", path, err)
		}
	}
}

func TestTruncateBytesRunes(t *testing.T) {
	if got := truncateBytesRunes([]byte("héllo"), 3); got != "hél" {
		t.Errorf("truncateBytesRunes(héllo, 3) = %q, want hél", got)
	}
	if got := truncateBytesRunes([]byte("hi"), 5); got != "hi" {
		t.Errorf("truncateBytesRunes(hi, 5) = %q, want hi", got)
	}
}

// The history survives a daemon restart: a fresh clipState.load() restores its
// metadata while payload bytes remain in their backing files.
func TestClipPersistence(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	s := &clipState{stateDir: dir, dataDir: dataDir, cacheDir: cache}
	imageData := tinyPNG(t, 1205, 417)
	s.ingest("image/png", imageData)
	imageEntry := s.entries[0]
	if imageEntry.W != 1205 || imageEntry.H != 417 {
		t.Errorf("source image size = %dx%d, want 1205x417", imageEntry.W, imageEntry.H)
	}
	if imageEntry.ThumbW != 512 || imageEntry.ThumbH != 177 {
		t.Errorf("thumbnail size = %dx%d, want 512x177", imageEntry.ThumbW, imageEntry.ThumbH)
	}
	s.ingest("text/plain", []byte("hello"))
	s.ingest("text/plain", []byte("world"))

	s2 := &clipState{stateDir: dir, dataDir: dataDir, cacheDir: cache}
	s2.load()
	if len(s2.entries) != 3 {
		t.Fatalf("reloaded %d entries, want 3", len(s2.entries))
	}
	if s2.entries[0].Preview != "world" {
		t.Errorf("front after reload = %q, want %q", s2.entries[0].Preview, "world")
	}
	if got, err := os.ReadFile(s2.dataPath(s2.entries[0].hash)); err != nil || !bytes.Equal(got, []byte("world")) {
		t.Errorf("reloaded front payload = %q, %v; want %q, nil", got, err, "world")
	}
	reloadedImage := s2.entries[2]
	if reloadedImage.W != 1205 || reloadedImage.H != 417 {
		t.Errorf("reloaded source image size = %dx%d, want 1205x417", reloadedImage.W, reloadedImage.H)
	}
	if reloadedImage.ThumbW != 512 || reloadedImage.ThumbH != 177 {
		t.Errorf("reloaded thumbnail size = %dx%d, want 512x177", reloadedImage.ThumbW, reloadedImage.ThumbH)
	}
	if s2.nextID != s.nextID {
		t.Errorf("reloaded nextID = %d, want %d", s2.nextID, s.nextID)
	}
}

func TestClipCopyReadsPayloadFromDisk(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	copied := filepath.Join(t.TempDir(), "copied")
	writeCommandStub(t, bin, "wl-copy", `cat > "$CLIP_COPY_STDIN"`)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CLIP_COPY_STDIN", copied)

	s := &clipState{stateDir: dir, dataDir: dataDir, cacheDir: t.TempDir()}
	s.ingest("text/plain", []byte("from disk"))
	s.ingest("text/plain", []byte("newer"))
	if err := s.copy(1); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(copied)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("from disk")) {
		t.Errorf("wl-copy received %q, want %q", got, "from disk")
	}
	if ids := clipIDs(s); !reflect.DeepEqual(ids, []uint64{1, 2}) {
		t.Errorf("ids after copy = %v, want [1 2]", ids)
	}
}

// Starred state is part of the persisted schema: a starred entry survives a
// daemon restart still starred, and an unstarred one stays unstarred.
func TestClipStarPersistence(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	s := &clipState{stateDir: dir, dataDir: dataDir, cacheDir: cache}
	s.ingest("text/plain", []byte("keep")) // id 1
	s.ingest("text/plain", []byte("junk")) // id 2
	if err := s.star(1, true); err != nil {
		t.Fatalf("star(1, true) = %v", err)
	}

	s2 := &clipState{stateDir: dir, dataDir: dataDir, cacheDir: cache}
	s2.load()
	byPreview := map[string]bool{}
	for _, e := range s2.entries {
		byPreview[e.Preview] = e.Starred
	}
	if !byPreview["keep"] {
		t.Error("starred entry lost its star across restart")
	}
	if byPreview["junk"] {
		t.Error("unstarred entry came back starred across restart")
	}
}

// A repeat of starred content is promoted in place without losing the star, so
// re-copying something already in the Starred pane keeps it safe.
func TestClipStarDuplicatePreserved(t *testing.T) {
	s := &clipState{}
	s.pushLocked(&clipEntry{hash: 100, Kind: "text", Preview: "keep"})
	if err := s.star(1, true); err != nil {
		t.Fatalf("star(1, true) = %v", err)
	}
	s.pushLocked(&clipEntry{hash: 100, Kind: "text", Preview: "keep-again"})
	if len(s.entries) != 1 {
		t.Fatalf("dedup kept a duplicate: len = %d, want 1", len(s.entries))
	}
	if !s.entries[0].Starred {
		t.Error("re-ingesting starred content dropped the star")
	}
	if s.entries[0].ID != 1 {
		t.Errorf("promoted entry id = %d, want 1", s.entries[0].ID)
	}
}

// star flips the flag both ways and reports an error for an unknown id.
func TestClipStarToggleAndUnknown(t *testing.T) {
	s := &clipState{}
	s.pushLocked(&clipEntry{hash: 1, Kind: "text", Preview: "a"})
	if err := s.star(1, true); err != nil {
		t.Fatalf("star(1, true) = %v", err)
	}
	if !s.entries[0].Starred {
		t.Fatal("star did not set the flag")
	}
	if err := s.star(1, false); err != nil {
		t.Fatalf("star(1, false) = %v", err)
	}
	if s.entries[0].Starred {
		t.Error("unstar did not clear the flag")
	}
	if err := s.star(999, true); err == nil {
		t.Error("star of an unknown id returned no error")
	}
}

func TestClipStarredByteBudget(t *testing.T) {
	s := &clipState{}
	s.pushLocked(&clipEntry{hash: 1, Size: 60 << 20})
	s.pushLocked(&clipEntry{hash: 2, Size: 50 << 20})
	if err := s.star(1, true); err != nil {
		t.Fatalf("star first entry: %v", err)
	}
	err := s.star(2, true)
	if err == nil || !strings.Contains(err.Error(), "unstar an item first") {
		t.Fatalf("star past budget = %v, want a clear limit error", err)
	}
	if s.entries[0].Starred {
		t.Fatal("entry was starred past the byte budget")
	}
	if err := s.star(1, false); err != nil {
		t.Fatal(err)
	}
	if err := s.star(2, true); err != nil {
		t.Fatalf("star after freeing budget: %v", err)
	}
}

func TestClipLoadKeepsNewestStarsWithinBudget(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	s := &clipState{stateDir: dir, dataDir: dataDir}
	for i := 1; i <= 3; i++ {
		data := []byte{byte(i)}
		hash := clipHash(data)
		s.writeData(hash, data)
		s.entries = append([]*clipEntry{{
			ID: uint64(i), Kind: "binary", Mime: "application/octet-stream",
			Size: 40 << 20, Starred: true, hash: hash, ts: time.Unix(int64(i), 0),
		}}, s.entries...)
		s.nextID = uint64(i)
	}
	s.persistLocked()

	loaded := &clipState{stateDir: dir, dataDir: dataDir}
	loaded.load()
	if len(loaded.entries) != 3 {
		t.Fatalf("loaded %d entries, want 3", len(loaded.entries))
	}
	if !loaded.entries[0].Starred || !loaded.entries[1].Starred || loaded.entries[2].Starred {
		t.Errorf("starred flags newest-first = [%v %v %v], want [true true false]",
			loaded.entries[0].Starred, loaded.entries[1].Starred, loaded.entries[2].Starred)
	}
	for _, e := range loaded.entries {
		if _, err := os.Stat(loaded.dataPath(e.hash)); err != nil {
			t.Errorf("load removed payload for entry %d: %v", e.ID, err)
		}
	}
}

// clear empties the history but the Starred pane is safe: starred entries and
// their backing files stay, unstarred entries and their files go.
func TestClipClearPreservesStarred(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	s := &clipState{stateDir: dir, dataDir: dataDir, cacheDir: cache}
	s.ingest("text/plain", []byte("keep")) // id 1
	s.ingest("text/plain", []byte("drop")) // id 2
	var keepHash, dropHash uint64
	for _, e := range s.entries {
		switch e.Preview {
		case "keep":
			keepHash = e.hash
		case "drop":
			dropHash = e.hash
		}
	}
	if err := s.star(1, true); err != nil {
		t.Fatalf("star(1, true) = %v", err)
	}

	s.clear()
	if len(s.entries) != 1 || s.entries[0].Preview != "keep" {
		t.Fatalf("after clear entries = %v, want just the starred keep", clipIDs(s))
	}
	if !s.entries[0].Starred {
		t.Error("clear stripped the star from the surviving entry")
	}
	if _, err := os.Stat(s.dataPath(keepHash)); err != nil {
		t.Errorf("clear removed the starred entry's backing file: %v", err)
	}
	if _, err := os.Stat(s.dataPath(dropHash)); !os.IsNotExist(err) {
		t.Errorf("clear left the unstarred entry's backing file behind")
	}
}

// Overflow evicts the oldest unstarred entry, never a starred one, while an
// unstarred candidate remains; when only starred entries are left it keeps them
// rather than dropping something the user marked safe.
func TestClipOverflowSpareStarred(t *testing.T) {
	s := &clipState{}
	for i := range clipMaxEntries {
		s.pushLocked(&clipEntry{hash: uint64(1 + i)})
	}
	if err := s.star(1, true); err != nil { // id 1 is the oldest, at the back
		t.Fatalf("star(1, true) = %v", err)
	}
	for i := range 5 {
		s.pushLocked(&clipEntry{hash: uint64(1000 + i)})
	}
	if len(s.entries) != clipMaxEntries {
		t.Fatalf("cap not held with an unstarred candidate present: len = %d, want %d", len(s.entries), clipMaxEntries)
	}
	starredPresent := false
	for _, e := range s.entries {
		if e.ID == 1 {
			starredPresent = true
		}
	}
	if !starredPresent {
		t.Error("overflow evicted the starred entry instead of an unstarred one")
	}

	// Star every remaining entry, then overflow again: with only starred entries
	// already present, none of them may be evicted. The freshly-copied unstarred
	// entry is the candidate that yields to hold the cap.
	prevIDs := clipIDs(s)
	for _, e := range s.entries {
		e.Starred = true
	}
	s.pushLocked(&clipEntry{hash: 9999})
	present := map[uint64]bool{}
	for _, e := range s.entries {
		present[e.ID] = true
	}
	for _, id := range prevIDs {
		if !present[id] {
			t.Errorf("overflow evicted starred entry %d with no other unstarred candidate", id)
		}
	}
}

func TestClipEvictionRemovesPayload(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	s := &clipState{stateDir: dir, dataDir: dataDir, cacheDir: t.TempDir()}
	for i := range clipMaxEntries {
		s.ingest("text/plain", []byte(fmt.Sprintf("entry-%d", i)))
	}
	oldest := s.entries[len(s.entries)-1]
	oldestPath := s.dataPath(oldest.hash)
	s.ingest("text/plain", []byte("overflow"))
	if len(s.entries) != clipMaxEntries {
		t.Fatalf("entries after overflow = %d, want %d", len(s.entries), clipMaxEntries)
	}
	if _, err := os.Stat(oldestPath); !os.IsNotExist(err) {
		t.Errorf("evicted payload still exists: %v", err)
	}
}

// The storage report is what the Hub shows: every entry's bytes plus the
// thumbnail an image carries, split into text, images and the rest, with the
// pinned count -- and it must survive a reload (the sizes are persisted).
func TestClipStats(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	s := &clipState{stateDir: dir, dataDir: dataDir, cacheDir: cache}
	s.ingest("text/plain", []byte("hello")) // 5 bytes of text
	s.ingest("image/png", tinyPNG(t, 8, 8)) // image bytes + a thumbnail
	if err := s.star(1, true); err != nil { // star the text entry
		t.Fatalf("star(1, true) = %v", err)
	}

	st := s.stats()
	if st.Items != 2 || st.Starred != 1 {
		t.Fatalf("stats = %+v, want 2 items and 1 starred", st)
	}
	if st.TextBytes != 5 {
		t.Errorf("textBytes = %d, want 5", st.TextBytes)
	}
	if st.ImageBytes <= int64(len(tinyPNG(t, 8, 8))) {
		t.Errorf("imageBytes = %d, want more than the image alone (thumbnail included)", st.ImageBytes)
	}
	if st.Bytes != st.TextBytes+st.ImageBytes+st.OtherBytes {
		t.Errorf("bytes %d != the three parts %d+%d+%d", st.Bytes, st.TextBytes, st.ImageBytes, st.OtherBytes)
	}

	// The same numbers after a restart: thumbnail sizes ride the index instead of
	// being re-measured into a different total.
	s2 := &clipState{stateDir: dir, dataDir: dataDir, cacheDir: cache}
	s2.load()
	if got := s2.stats(); got != st {
		t.Errorf("stats after reload = %+v, want %+v", got, st)
	}
}

// The weekly sweep drops unstarred history and keeps the starred entries, and it
// runs at most once per period: the clock rides the index, so a restart or a
// second look does not prune again.
func TestClipAutoPrune(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	s := &clipState{stateDir: dir, dataDir: dataDir, cacheDir: cache}
	s.ingest("text/plain", []byte("junk")) // id 1
	s.ingest("text/plain", []byte("keep")) // id 2
	if err := s.star(2, true); err != nil {
		t.Fatalf("star(2, true) = %v", err)
	}

	now := time.Now()
	// Off: nothing happens, not even the clock starting.
	if s.pruneIfDue(false, now) {
		t.Error("pruned while the setting was off")
	}
	// First look after switching on starts the clock rather than deleting.
	if s.pruneIfDue(true, now) {
		t.Error("pruned on the first look instead of starting the clock")
	}
	if len(s.entries) != 2 {
		t.Fatalf("entries = %d after the clock started, want 2", len(s.entries))
	}
	// Still inside the week.
	if s.pruneIfDue(true, now.Add(clipPruneEvery-time.Minute)) {
		t.Error("pruned before the week was up")
	}
	if len(s.entries) != 2 {
		t.Fatalf("entries = %d before the week was up, want 2", len(s.entries))
	}
	// Due: everything unstarred goes, the starred entry stays.
	if !s.pruneIfDue(true, now.Add(clipPruneEvery)) {
		t.Error("did not prune when the week was up")
	}
	if len(s.entries) != 1 || s.entries[0].Preview != "keep" {
		t.Fatalf("after the sweep = %d entries (%v), want just %q", len(s.entries), clipIDs(s), "keep")
	}
	// And the new clock survives a restart, so the next sweep waits another week.
	s2 := &clipState{stateDir: dir, dataDir: dataDir, cacheDir: cache}
	s2.load()
	if s2.pruneIfDue(true, now.Add(clipPruneEvery+time.Minute)) {
		t.Error("pruned again immediately after the last sweep")
	}
}

// A thumbnail whose entry is gone is invisible to the user but still holds the
// disk, and it would make the storage report under-count the history. The load
// sweep takes it, and leaves the live entry's thumbnail alone.
func TestClipSweepsOrphanThumbnails(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	s := &clipState{stateDir: dir, dataDir: dataDir, cacheDir: cache}
	s.ingest("image/png", tinyPNG(t, 8, 8))
	if len(s.entries) != 1 || s.entries[0].ThumbPath == "" {
		t.Fatal("expected one image entry with a thumbnail")
	}
	live := s.entries[0].ThumbPath
	orphan := filepath.Join(cache, "thumb-00000000deadbeef.png")
	if err := os.WriteFile(orphan, []byte("stale thumbnail bytes"), 0o600); err != nil {
		t.Fatal(err)
	}

	s2 := &clipState{stateDir: dir, dataDir: dataDir, cacheDir: cache}
	s2.load()
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Error("orphan thumbnail survived the load sweep")
	}
	if _, err := os.Stat(live); err != nil {
		t.Errorf("the live entry's thumbnail was swept: %v", err)
	}
}

// The keeper is recognised by program name and owned by its environment marker,
// so a user's own wl-clip-persist is never reaped and ours always is.
func TestIsClipPersistCmdline(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want bool
	}{
		{"our keeper", []string{"wl-clip-persist", "--clipboard", "regular"}, true},
		{"keeper by path", []string{"/usr/bin/wl-clip-persist", "--clipboard", "regular"}, true},
		{"the watcher is not a keeper", []string{"wl-paste", "--watch", "/home/u/.local/bin/ryoku-shell", "__clip-ingest"}, false},
		{"the daemon is not a keeper", []string{"/home/u/.local/bin/ryoku-shell", "daemon"}, false},
		{"empty", nil, false},
	}
	for _, c := range cases {
		if got := isClipPersistCmdline(c.argv); got != c.want {
			t.Errorf("%s: isClipPersistCmdline(%v) = %v, want %v", c.name, c.argv, got, c.want)
		}
	}
}
