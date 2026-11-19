package bt

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"

	"ryoget/internal/proto"
)

// Stats is one torrent's state as the task row shows it. Size and Done cover
// the selected files; Size is -1 until the metadata is known. Down and Up are
// bytes per second; Uploaded and Ratio carry over restarts.
type Stats struct {
	Size, Done          int64
	Down, Up            int64
	Uploaded            int64
	Peers, Seeds, Conns int
	Ratio               float64
	Pieces              [proto.PieceBuckets]byte
	Complete, Seeding   bool
}

// Torrent is one task's torrent. While paused or after seeding it holds no
// anacrolix torrent at all, so it neither announces nor keeps connections.
type Torrent struct {
	c        *Client
	ih       metainfo.Hash
	dir      string
	magnet   bool
	trackers [][]string
	display  string
	err      atomic.Pointer[errBox]

	mu       sync.Mutex
	tor      *torrent.Torrent
	opened   bool
	store    storage.ClientImpl
	info     *metainfo.Info
	want     []int
	sel      []bool
	paused   bool
	finished bool
	dropped  bool
	complete bool
	hasMeta  bool
	meta     chan struct{}
	done     chan struct{}
	fin      chan struct{}
	side     sidecar
	seedFrom time.Time
	savedAt  time.Time
	last     Stats
	lastDone []int64
	m        meter
}

type errBox struct{ err error }

func (t *Torrent) InfoHash() string { return t.ih.HexString() }

// Err is the storage error that stopped the download, if any.
func (t *Torrent) Err() error {
	if b := t.err.Load(); b != nil {
		return b.err
	}
	return nil
}

func (t *Torrent) setErr(err error) { t.err.Store(&errBox{err}) }

// Metadata is closed once the file list is known and storage is open.
func (t *Torrent) Metadata() <-chan struct{} { return t.meta }

func (t *Torrent) HasMetadata() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.hasMeta
}

// Completed is closed when every selected file is downloaded and verified.
// Select can reopen a completed torrent; ask again after it.
func (t *Torrent) Completed() <-chan struct{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.done
}

// Finished is closed when the torrent stops for good after completing: at
// once without seeding, otherwise when the seed policy is met.
func (t *Torrent) Finished() <-chan struct{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.fin
}

func (t *Torrent) Paused() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.paused
}

func (t *Torrent) Name() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	switch {
	case t.info != nil:
		return t.info.BestName()
	case t.display != "":
		return t.display
	}
	return t.ih.HexString()
}

// Path is the downloaded file, or the folder of a multi-file torrent.
func (t *Torrent) Path() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.info == nil {
		return ""
	}
	return filepath.Join(t.dir, t.info.BestName())
}

func (t *Torrent) Files() []proto.File {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.info == nil {
		return nil
	}
	files := infoFiles(t.info)
	var live []*torrent.File
	if t.tor != nil && t.opened {
		live = t.tor.Files()
	}
	for i := range files {
		files[i].Selected = t.sel[i]
		switch {
		case i < len(live):
			files[i].Done = live[i].BytesCompleted()
		case i < len(t.lastDone):
			files[i].Done = t.lastDone[i]
		}
	}
	return files
}

// Select replaces the file selection. Selecting more files of a completed
// torrent resumes it.
func (t *Torrent) Select(idx []int) error {
	if len(idx) == 0 {
		return invalid("Select at least one file.")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.dropped {
		return errClosed
	}
	if t.info == nil {
		t.want = slices.Clone(idx)
		return nil
	}
	n := len(t.sel)
	sel := make([]bool, n)
	grows := false
	for _, i := range idx {
		if i < 0 || i >= n {
			return invalid("That file is not in this torrent.")
		}
		sel[i] = true
		grows = grows || !t.sel[i]
	}
	t.sel = sel
	t.applyLocked()
	if grows && t.complete {
		now := time.Now()
		t.complete = false
		t.foldSeedLocked(now)
		t.done = make(chan struct{})
		if t.finished {
			t.finished = false
			t.fin = make(chan struct{})
			if !t.paused {
				t.restartLocked()
			}
		}
	}
	return nil
}

// Pause stops the torrent completely; Resume starts it again from the kept
// metainfo and piece completion.
func (t *Torrent) Pause() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.paused || t.dropped {
		return
	}
	t.paused = true
	now := time.Now()
	t.stopLocked(now)
	t.saveLocked(now)
}

func (t *Torrent) Resume() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.paused || t.dropped {
		return
	}
	t.paused = false
	if !t.finished {
		t.restartLocked()
	}
}

// Drop removes the torrent and its kept state, and its files when asked.
func (t *Torrent) Drop(deleteData bool) error {
	t.mu.Lock()
	if t.dropped {
		t.mu.Unlock()
		return nil
	}
	t.dropped = true
	if t.tor != nil {
		t.tor.Drop()
		t.tor = nil
	}
	info := t.info
	t.mu.Unlock()
	t.c.remove(t)
	t.c.forgetState(t.ih)
	var err error
	pieces := 0
	if info != nil {
		pieces = info.NumPieces()
		if deleteData {
			err = removeFiles(t.dir, info, func(int) bool { return true }, false)
		}
	}
	t.c.purge(t.ih, pieces)
	return err
}

func (t *Torrent) Stats() Stats {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.tor == nil || !t.opened {
		s := t.last
		if t.info == nil {
			s.Size = -1
		} else if s.Size == 0 {
			s.Size = t.selectedSizeLocked()
		}
		s.Uploaded = t.side.Uploaded
		if t.tor != nil {
			st := t.tor.Stats()
			s.Uploaded += st.BytesWrittenData.Int64()
			s.Conns = st.ActivePeers
			s.Seeds = st.ConnectedSeeders
			s.Peers = s.Conns - s.Seeds
			s.Down, s.Up = t.m.sample(time.Now(), st.BytesReadUsefulData.Int64(), st.BytesWrittenData.Int64())
		}
		return s
	}
	return t.liveStatsLocked(time.Now())
}

func (t *Torrent) liveStatsLocked(now time.Time) (s Stats) {
	tor := t.tor
	st := tor.Stats()
	s.Conns = st.ActivePeers
	s.Seeds = st.ConnectedSeeders
	s.Peers = s.Conns - s.Seeds
	written := st.BytesWrittenData.Int64()
	s.Uploaded = t.side.Uploaded + written
	s.Down, s.Up = t.m.sample(now, st.BytesReadUsefulData.Int64(), written)
	for i, f := range tor.Files() {
		if i < len(t.sel) && t.sel[i] {
			s.Size += f.Length()
			s.Done += f.BytesCompleted()
		}
	}
	runs := tor.PieceStateRuns()
	s.Pieces = buckets(runs, func(p int) float64 {
		size := t.info.Piece(p).Length()
		if size <= 0 {
			return 0
		}
		return 1 - float64(tor.PieceBytesMissing(p))/float64(size)
	})
	s.Complete = t.complete
	s.Seeding = t.complete && !t.finished
	s.Ratio = ratio(s.Uploaded, s.Done)
	return
}

// tick runs once a second from the supervisor: it notices completion and ends
// seeding by the policy. It reports whether the torrent still needs ticking.
func (t *Torrent) tick(now time.Time, policy SeedPolicy) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.dropped || t.paused || t.finished || t.tor == nil || !t.opened {
		return false
	}
	tor := t.tor
	complete := selectedComplete(tor.PieceStateRuns(), tor.Files(), t.sel)
	switch {
	case complete && !t.complete:
		t.complete = true
		close(t.done)
	case !complete && t.complete:
		t.complete = false
		t.foldSeedLocked(now)
		t.done = make(chan struct{})
	}
	if t.complete {
		if t.seedFrom.IsZero() {
			t.seedFrom = now
		}
		if !policy.On || t.seedMetLocked(now, policy) {
			t.finishLocked(now)
			return false
		}
	}
	if now.Sub(t.savedAt) >= 30*time.Second {
		t.saveLocked(now)
	}
	return true
}

func (t *Torrent) seedMetLocked(now time.Time, p SeedPolicy) bool {
	if p.Ratio > 0 {
		up := t.side.Uploaded + uploaded(t.tor)
		if ratio(up, t.selectedSizeLocked()) >= p.Ratio {
			return true
		}
	}
	if p.Minutes > 0 {
		seeded := t.side.Seeded + now.Sub(t.seedFrom).Seconds()
		if seeded >= float64(p.Minutes)*60 {
			return true
		}
	}
	return false
}

func (t *Torrent) finishLocked(now time.Time) {
	t.stopLocked(now)
	t.finished = true
	close(t.fin)
	sel := t.sel
	_ = removeFiles(t.dir, t.info, func(i int) bool { return !sel[i] }, true)
	t.saveLocked(now)
}

// startLocked adds the anacrolix torrent; storage opens once the info is in.
func (t *Torrent) startLocked(spec *torrent.TorrentSpec) error {
	c := t.c
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return errClosed
	}
	c.wg.Add(1)
	c.mu.Unlock()
	tor, err := c.addSpec(spec, t.magnet)
	if err != nil {
		c.wg.Done()
		return err
	}
	t.tor = tor
	t.opened = false
	t.m = meter{}
	tor.SetOnWriteChunkError(func(err error) {
		t.setErr(err)
		tor.DisallowDataDownload()
	})
	go t.awaitInfo(tor)
	return nil
}

func (t *Torrent) restartLocked() {
	spec := t.specLocked()
	if err := t.startLocked(spec); err != nil {
		t.setErr(err)
	}
}

func (t *Torrent) specLocked() *torrent.TorrentSpec {
	if mi := t.c.loadMetainfo(t.ih); mi != nil {
		if spec, err := specFromMetainfo(mi); err == nil {
			spec.Trackers = appendTier(spec.Trackers, flatten(t.trackers))
			return spec
		}
	}
	spec := &torrent.TorrentSpec{Trackers: t.trackers, DisplayName: t.display}
	spec.InfoHash = t.ih
	return spec
}

func (t *Torrent) awaitInfo(tor *torrent.Torrent) {
	c := t.c
	defer c.wg.Done()
	select {
	case <-tor.GotInfo():
	case <-tor.Closed():
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.tor != tor || t.dropped {
		return
	}
	info := tor.Info()
	if t.info == nil {
		if _, err := os.Stat(c.metainfoPath(t.ih)); err != nil {
			if err := c.saveMetainfo(t.ih, tor.Metainfo()); err != nil {
				c.log.Warn("saving metainfo", "err", err)
			}
		}
		t.info = info
		t.sel = selection(t.want, len(info.UpvertedFiles()))
		t.want = nil
	}
	if isPrivate(info) {
		tor.ModifyTrackers(t.trackers)
	}
	if t.store == nil {
		t.store = storage.NewFileOpts(storage.NewFileClientOpts{
			ClientBaseDir: t.dir,
			// Files in a download folder get opened, moved and deleted by
			// people; a mapping would turn that into SIGBUS.
			DisableMmap:     true,
			PieceCompletion: c.pc,
			Logger:          c.log,
		})
	}
	c.pc.markOpening(t.ih, true)
	err := tor.OpenStorage(context.Background(), t.store)
	c.pc.markOpening(t.ih, false)
	if err != nil {
		select {
		case <-tor.Closed():
		default:
			t.setErr(err)
		}
		return
	}
	t.opened = true
	t.applyLocked()
	if !t.hasMeta {
		t.hasMeta = true
		close(t.meta)
	}
	c.kick()
}

func (t *Torrent) applyLocked() {
	if t.tor == nil || !t.opened {
		return
	}
	for i, f := range t.tor.Files() {
		if i < len(t.sel) && t.sel[i] {
			f.SetPriority(torrent.PiecePriorityNormal)
		} else {
			f.SetPriority(torrent.PiecePriorityNone)
		}
	}
}

// stopLocked keeps a snapshot, folds this session's counters into the kept
// totals and drops the anacrolix torrent.
func (t *Torrent) stopLocked(now time.Time) {
	t.foldSeedLocked(now)
	if t.tor == nil {
		return
	}
	if t.opened {
		t.last = t.liveStatsLocked(now)
		t.lastDone = t.lastDone[:0]
		for _, f := range t.tor.Files() {
			t.lastDone = append(t.lastDone, f.BytesCompleted())
		}
	}
	t.side.Uploaded += uploaded(t.tor)
	t.last.Uploaded = t.side.Uploaded
	t.last.Down, t.last.Up = 0, 0
	t.last.Peers, t.last.Seeds, t.last.Conns = 0, 0, 0
	t.last.Seeding = false
	t.tor.Drop()
	t.tor = nil
	t.opened = false
}

func (t *Torrent) foldSeedLocked(now time.Time) {
	if !t.seedFrom.IsZero() {
		t.side.Seeded += now.Sub(t.seedFrom).Seconds()
		t.seedFrom = time.Time{}
	}
}

func (t *Torrent) saveLocked(now time.Time) {
	if t.dropped {
		return
	}
	s := t.side
	if t.tor != nil {
		s.Uploaded += uploaded(t.tor)
	}
	if !t.seedFrom.IsZero() {
		s.Seeded += now.Sub(t.seedFrom).Seconds()
	}
	if err := t.c.saveSidecar(t.ih, s); err != nil {
		t.c.log.Warn("saving torrent state", "err", err)
	}
	t.savedAt = now
}

// persist saves the counters when the client closes.
func (t *Torrent) persist() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.saveLocked(time.Now())
}

func uploaded(tor *torrent.Torrent) int64 {
	st := tor.Stats()
	return st.BytesWrittenData.Int64()
}

func (t *Torrent) selectedSizeLocked() (n int64) {
	if t.info == nil {
		return 0
	}
	for i, f := range t.info.UpvertedFiles() {
		if i < len(t.sel) && t.sel[i] {
			n += f.Length
		}
	}
	return
}

func selection(want []int, n int) []bool {
	sel := make([]bool, n)
	for i := range sel {
		sel[i] = want == nil
	}
	for _, i := range want {
		if i >= 0 && i < n {
			sel[i] = true
		}
	}
	return sel
}

func selectedComplete(runs torrent.PieceStateRuns, files []*torrent.File, sel []bool) bool {
	starts := make([]int, len(runs))
	next := 0
	for i, r := range runs {
		starts[i] = next
		next += r.Length
	}
	for i, f := range files {
		if i >= len(sel) || !sel[i] {
			continue
		}
		begin, end := f.BeginPieceIndex(), f.EndPieceIndex()
		if begin >= end {
			continue
		}
		r, _ := slices.BinarySearch(starts, begin+1)
		for r--; r < len(runs) && starts[r] < end; r++ {
			if !runs[r].Complete {
				return false
			}
		}
	}
	return true
}

// buckets folds the piece map into the task row's 64 fill levels. With fewer
// pieces than buckets each piece spans several buckets.
func buckets(runs torrent.PieceStateRuns, partial func(piece int) float64) (out [proto.PieceBuckets]byte) {
	n := 0
	for _, r := range runs {
		n += r.Length
	}
	if n == 0 {
		return
	}
	const k = proto.PieceBuckets
	run, runStart := 0, 0
	for b := range k {
		lo := b * n / k
		hi := max((b+1)*n/k, lo+1)
		for run < len(runs) && runStart+runs[run].Length <= lo {
			runStart += runs[run].Length
			run++
		}
		var have float64
		for r, rs := run, runStart; r < len(runs) && rs < hi; r++ {
			a, z := max(lo, rs), min(hi, rs+runs[r].Length)
			switch {
			case runs[r].Complete:
				have += float64(z - a)
			case runs[r].Partial:
				for p := a; p < z; p++ {
					have += partial(p)
				}
			}
			rs += runs[r].Length
		}
		out[b] = byte(math.Round(255 * min(1, have/float64(hi-lo))))
	}
	return
}

func ratio(up, done int64) float64 {
	if done <= 0 {
		return 0
	}
	return float64(up) / float64(done)
}

// meter turns byte counters into a smoothed rate.
type meter struct {
	at       time.Time
	r, w     int64
	down, up float64
}

func (m *meter) sample(now time.Time, r, w int64) (int64, int64) {
	if m.at.IsZero() {
		m.at, m.r, m.w = now, r, w
		return 0, 0
	}
	if dt := now.Sub(m.at).Seconds(); dt >= 0.25 {
		a := math.Exp(-dt)
		m.down = m.down*a + float64(r-m.r)/dt*(1-a)
		m.up = m.up*a + float64(w-m.w)/dt*(1-a)
		m.at, m.r, m.w = now, r, w
	}
	return int64(m.down + 0.5), int64(m.up + 0.5)
}

func specInfo(spec *torrent.TorrentSpec) (*metainfo.Info, error) {
	mi := metainfo.MetaInfo{InfoBytes: spec.InfoBytes}
	info, err := mi.UnmarshalInfo()
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func infoFiles(info *metainfo.Info) []proto.File {
	fis := info.UpvertedFiles()
	out := make([]proto.File, len(fis))
	for i, fi := range fis {
		path := info.BestName()
		if info.IsDir() {
			path = strings.Join(fi.BestPath(), "/")
		}
		out[i] = proto.File{Path: path, Size: fi.Length, Selected: true}
	}
	return out
}

// removeFiles deletes the chosen files of a torrent (only their .part files
// when partsOnly) and then the folders that became empty, never leaving dir.
func removeFiles(dir string, info *metainfo.Info, pick func(int) bool, partsOnly bool) error {
	if info == nil {
		return nil
	}
	root := filepath.Clean(dir)
	var firstErr error
	parents := map[string]bool{}
	for i, fi := range info.UpvertedFiles() {
		if !pick(i) {
			continue
		}
		parts := []string{root}
		if name := info.BestName(); name != metainfo.NoName {
			parts = append(parts, name)
		}
		p := filepath.Join(append(parts, fi.BestPath()...)...)
		if rel, err := filepath.Rel(root, p); err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			continue
		}
		names := []string{p + ".part"}
		if !partsOnly {
			names = append(names, p)
		}
		for _, name := range names {
			if err := os.Remove(name); err != nil && !os.IsNotExist(err) && firstErr == nil {
				firstErr = err
			}
		}
		for d := filepath.Dir(p); d != root && strings.HasPrefix(d, root+string(filepath.Separator)); d = filepath.Dir(d) {
			parents[d] = true
		}
	}
	dirs := make([]string, 0, len(parents))
	for d := range parents {
		dirs = append(dirs, d)
	}
	slices.SortFunc(dirs, func(a, b string) int { return len(b) - len(a) })
	for _, d := range dirs {
		_ = os.Remove(d)
	}
	return firstErr
}
