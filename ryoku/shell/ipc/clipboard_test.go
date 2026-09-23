package main

import (
	"bytes"
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

// classifyClip is the preview contract: text keeps a 200-character preview,
// a decodable image gets a thumbnail, an undecodable image falls to binary, and
// any other type is binary.
func TestClassifyClip(t *testing.T) {
	kind, preview, thumb, _, _ := classifyClip("text/plain", []byte(strings.Repeat("a", 250)))
	if kind != "text" || len([]rune(preview)) != clipTextPreviewLen || thumb != nil {
		t.Errorf("text classify = (%q, %d-rune preview, thumb=%v), want (text, 200, false)", kind, len([]rune(preview)), thumb != nil)
	}

	kind, _, thumb, w, h := classifyClip("image/png", tinyPNG(t, 8, 4))
	if kind != "image" || thumb == nil || w != 8 || h != 4 {
		t.Errorf("png classify = (%q, thumb=%v, %dx%d), want (image, true, 8x4)", kind, thumb != nil, w, h)
	}

	kind, _, thumb, _, _ = classifyClip("image/png", []byte("not a png"))
	if kind != "binary" || thumb != nil {
		t.Errorf("undecodable image classify = (%q, thumb=%v), want (binary, false)", kind, thumb != nil)
	}

	if kind, _, _, _, _ := classifyClip("application/pdf", []byte("%PDF")); kind != "binary" {
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
		// never wins the fallback against a real type offered beside it.
		{[]string{"chromium/x-internal-source-rfh-token"}, ""},
		{[]string{"chromium/x-internal-source-url"}, ""},
		{[]string{"chromium/x-internal-source-rfh-token", "image/png"}, "image/png"},
		{[]string{"chromium/x-internal-source-rfh-token", "text/plain"}, "text/plain"},
		{nil, ""},
	}
	for _, c := range cases {
		if got := pickBestMime(c.offered); got != c.want {
			t.Errorf("pickBestMime(%v) = %q, want %q", c.offered, got, c.want)
		}
	}
}

// Empty data is dropped, and a selection past the entry cap is truncated.
func TestClipIngestSizeRules(t *testing.T) {
	s := &clipState{}
	s.ingest("text/plain", nil)
	s.ingest("text/plain", []byte{})
	if len(s.entries) != 0 {
		t.Fatalf("empty selection stored: %d entries", len(s.entries))
	}

	s.ingest("text/plain", bytes.Repeat([]byte("a"), clipMaxEntryBytes+100))
	if len(s.entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(s.entries))
	}
	if s.entries[0].Size != clipMaxEntryBytes {
		t.Errorf("stored size = %d, want %d (truncated at the 10 MiB cap)", s.entries[0].Size, clipMaxEntryBytes)
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("héllo", 3); got != "hél" {
		t.Errorf("truncateRunes(héllo, 3) = %q, want hél", got)
	}
	if got := truncateRunes("hi", 5); got != "hi" {
		t.Errorf("truncateRunes(hi, 5) = %q, want hi", got)
	}
}

// The history survives a daemon restart: ingested entries are written to the
// state dir and a fresh clipState.load() restores them, order, bytes and nextID.
func TestClipPersistence(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	s := &clipState{stateDir: dir, dataDir: dataDir, cacheDir: cache}
	s.ingest("text/plain", []byte("hello"))
	s.ingest("text/plain", []byte("world"))

	s2 := &clipState{stateDir: dir, dataDir: dataDir, cacheDir: cache}
	s2.load()
	if len(s2.entries) != 2 {
		t.Fatalf("reloaded %d entries, want 2", len(s2.entries))
	}
	if s2.entries[0].Preview != "world" {
		t.Errorf("front after reload = %q, want %q", s2.entries[0].Preview, "world")
	}
	if !bytes.Equal(s2.entries[0].data, []byte("world")) {
		t.Errorf("reloaded front bytes = %q, want %q", s2.entries[0].data, "world")
	}
	if s2.nextID != s.nextID {
		t.Errorf("reloaded nextID = %d, want %d", s2.nextID, s.nextID)
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

// isClipWatcherCmdline must match only our own watcher/helper -- never another
// app's wl-paste or the daemon itself -- since a false match SIGKILLs the pid.
func TestIsClipWatcherCmdline(t *testing.T) {
	const self = "/home/u/.local/bin/ryoku-shell"
	cases := []struct {
		name string
		argv []string
		want bool
	}{
		{"watcher", []string{"wl-paste", "--watch", self, "__clip-ingest"}, true},
		{"helper", []string{self, "__clip-ingest"}, true},
		{"other-app wl-paste", []string{"wl-paste", "--watch", "/usr/bin/cliphist", "store"}, false},
		{"plain wl-paste", []string{"wl-paste", "-n", "-t", "image/png"}, false},
		{"our daemon", []string{self, "daemon"}, false},
		{"our other verb", []string{self, "ipc", "menu"}, false},
		{"ingest marker, foreign binary", []string{"wl-paste", "--watch", "/other/bin", "__clip-ingest"}, false},
		{"empty", nil, false},
	}
	for _, c := range cases {
		if got := isClipWatcherCmdline(c.argv, self); got != c.want {
			t.Errorf("%s: isClipWatcherCmdline(%v) = %v, want %v", c.name, c.argv, got, c.want)
		}
	}
}
