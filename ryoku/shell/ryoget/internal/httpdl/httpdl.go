package httpdl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ryoget/internal/plan"
)

const (
	chunkSize = 32 << 10
	buckets   = 64
	// a resume without validators compares this much before the frontier
	overlapCheck = 64 << 10
	// SidecarExt follows the .part file it describes
	SidecarExt = ".ryoget"
)

var errChanged = errors.New("the file changed on the server")

// Resumer is implemented by a Progress that wants bytes already on disk
// reported apart from bytes that just arrived, so they do not count as speed.
type Resumer interface{ Resumed(bytes int64) }

// ConnReporter is implemented by a Progress that shows live connections.
type ConnReporter interface{ Conns(n int) }

// Namer is called once with the first answer, before any byte lands, and returns the download path.
type Namer func(info *Info) (string, error)

type Downloader struct {
	Client *http.Client
	Limit  plan.Limiter
	// read when a download starts; nil means 8 connections, 1 MiB segments
	Conns    func() int
	MinSplit func() int64
	// test hook: overrides the retry back-off (a server's Retry-After still wins)
	Backoff func(try int) time.Duration
	// learned per-host connection limits, empty means hosts.json in the state dir
	HostsFile string

	tune *tuning
}

type tuning struct {
	// after a connection gave up, replacements wait this long
	hold time.Duration
	// first wait before trying one more connection than a host allowed
	probeFirst, probeMax time.Duration
	saveEvery            time.Duration
	tick                 time.Duration
	minReq, maxReq       int64
	initial              int
	maxRetries           int
	raceSecs             float64
	raceWarmup           time.Duration
	free                 func(dir string) (int64, error)
}

// ported from prism-download-manager crates/prism-engine/src/config.rs (MIT)
func defaultTuning() tuning {
	return tuning{
		hold:       10 * time.Second,
		probeFirst: 20 * time.Second,
		probeMax:   300 * time.Second,
		saveEvery:  5 * time.Second,
		tick:       250 * time.Millisecond,
		minReq:     4 << 20,
		maxReq:     16 << 20,
		initial:    4,
		maxRetries: 10,
		raceSecs:   3,
		raceWarmup: time.Second,
		free:       statfsFree,
	}
}

type sidecar struct {
	URL          string `json:"url"`
	ETag         string `json:"etag,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
	Size         int64  `json:"size"`
	Segments     []span `json:"segments"`
}

type firstStream struct {
	a  assignment
	st *stream
}

func (f *firstStream) close() {
	if f != nil {
		f.st.close()
	}
}

type job struct {
	d      *Downloader
	tn     tuning
	client *http.Client
	part   plan.Part
	p      plan.Progress
	host   string
	conns  int

	info      *Info
	ranged    bool
	validator string
	minSplit  int64
	partPath  string
	file      *os.File
	tab       *table

	ctx    context.Context
	cancel context.CancelFunc

	// writers hold it shared from accounting to WriteAt, a sidecar snapshot holds it alone
	wmu         sync.RWMutex
	writeFailed atomic.Bool

	srcMu sync.Mutex
	final string

	fewRequests atomic.Bool
	// furthest a stream without ranges ever got
	high      atomic.Int64
	connected atomic.Int32
	connNote  chan struct{}
	exits     chan exit
}

// Fetch writes dst exactly, a resume checks the bytes and not the URL since media links are re-signed every run.
func (d *Downloader) Fetch(ctx context.Context, part plan.Part, dst string, p plan.Progress) error {
	_ = os.Remove(dst)
	_, err := d.fetch(ctx, part, dst, nil, p)
	return err
}

// FetchFile publishes under dst or "dst (n)", with dst empty the namer picks it from the first answer.
func (d *Downloader) FetchFile(ctx context.Context, part plan.Part, dst string, name Namer, p plan.Progress) (string, error) {
	return d.fetch(ctx, part, dst, name, p)
}

func (d *Downloader) tuning() tuning {
	if d.tune != nil {
		return *d.tune
	}
	return defaultTuning()
}

func (d *Downloader) fetch(ctx context.Context, part plan.Part, dst string, name Namer, p plan.Progress) (string, error) {
	if err := checkURL(part.URL); err != nil {
		return "", err
	}
	if dst == "" && name == nil {
		return "", errors.New("httpdl: no destination")
	}
	client := d.Client
	if client == nil {
		client = NewClient()
	}
	named := name == nil
	for restarts := 0; ; restarts++ {
		j := d.newJob(ctx, client, part, p)
		err := j.download(&dst, name, &named, restarts > 0)
		j.cancel()
		if errors.Is(err, errChanged) && restarts < 2 {
			continue
		}
		if err != nil {
			return "", err
		}
		return Publish(dst+".part", dst)
	}
}

func (d *Downloader) newJob(ctx context.Context, client *http.Client, part plan.Part, p plan.Progress) *job {
	j := &job{d: d, tn: d.tuning(), client: client, part: part, p: p, conns: 8, minSplit: 1 << 20,
		connNote: make(chan struct{}, 1), exits: make(chan exit)}
	if d.Conns != nil {
		if v := d.Conns(); v > 0 {
			j.conns = v
		}
	}
	if d.MinSplit != nil {
		if v := d.MinSplit(); v > 0 {
			j.minSplit = v
		}
	}
	if u, err := url.Parse(part.URL); err == nil {
		j.host = strings.ToLower(u.Host)
	}
	j.ctx, j.cancel = context.WithCancel(ctx)
	return j
}

func (j *job) sidecarPath() string { return j.partPath + SidecarExt }

// a weak ETag may not be used in If-Range
func ifRange(etag, lastModified string) string {
	if strongETag(etag) {
		return etag
	}
	return lastModified
}

func (j *job) download(dst *string, name Namer, named *bool, fresh bool) error {
	var first *firstStream
	resumed := false
	if *dst != "" && !fresh {
		j.partPath = *dst + ".part"
		if sc := j.loadSidecar(); sc != nil {
			f, err := j.resume(sc)
			switch {
			case err == nil:
				first, resumed = f, true
			case !errors.Is(err, errChanged):
				return j.stopErr(err)
			}
		}
	}
	if !resumed {
		f, err := j.probe()
		if err != nil {
			return j.stopErr(err)
		}
		first = f
	}
	if !*named {
		*named = true
		path, err := name(j.info)
		if err != nil {
			first.close()
			return err
		}
		if *dst == "" {
			*dst = path
		}
	}
	j.partPath = *dst + ".part"
	dir := filepath.Dir(j.partPath)
	var f *os.File
	var err error
	if resumed {
		// fallocate already reserved the file, only blocks it does not hold yet need room
		if err = checkSpace(j.tn.free, dir, min(j.tab.remaining(), j.info.Size-allocated(j.partPath))); err == nil {
			f, err = os.OpenFile(j.partPath, os.O_RDWR, 0o644)
		}
	} else {
		// a stale .part from before a change still holds its blocks
		_ = os.Remove(j.sidecarPath())
		_ = os.Remove(j.partPath)
		if err = checkSpace(j.tn.free, dir, j.info.Size); err == nil {
			f, err = os.OpenFile(j.partPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
		}
		if err == nil {
			if err = allocate(f, j.info.Size); err != nil {
				f.Close()
			}
		}
	}
	if err != nil {
		first.close()
		return err
	}
	j.file = f
	j.p.SetTotal(j.info.Size)
	if have := j.tab.downloaded.Load(); have > 0 {
		if r, ok := j.p.(Resumer); ok {
			r.Resumed(have)
		} else {
			j.p.Add(have)
		}
	}
	err = j.supervise(first)
	if err != nil {
		if errors.Is(err, errChanged) {
			j.p.Add(-j.tab.downloaded.Load())
			_ = os.Remove(j.sidecarPath())
		} else {
			j.save()
		}
		f.Close()
		return err
	}
	if j.info.Size < 0 {
		got := j.tab.downloaded.Load()
		if err := f.Truncate(got); err != nil {
			f.Close()
			return err
		}
		j.p.SetTotal(got)
	}
	j.mark(true)
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	_ = os.Remove(j.sidecarPath())
	return nil
}

func (j *job) stopErr(err error) error {
	if j.ctx.Err() != nil {
		return j.ctx.Err()
	}
	return err
}

// retrying gives requests made before any connection runs the same budget a connection gets.
func (j *job) retrying(fn func() error) error {
	failures := 0
	for {
		err := fn()
		if err == nil {
			return nil
		}
		if j.ctx.Err() != nil {
			return j.ctx.Err()
		}
		var ce *connError
		if errors.Is(err, errChanged) || (!retryable(err) && !errors.As(err, &ce)) {
			return err
		}
		failures++
		if failures > j.tn.maxRetries {
			return err
		}
		if !j.sleep(j.retryDelay(err, failures, false)) {
			return j.ctx.Err()
		}
	}
}

// probe needs no HEAD or one-byte GET: the first ranged answer names the file and its body is the first stream.
// ported from prism-download-manager crates/prism-engine/src/http/fetcher.rs (MIT)
func (j *job) probe() (*firstStream, error) {
	var st *stream
	ranged := true
	err := j.retrying(func() error {
		for {
			s, err := j.send(0, j.tn.minReq, "", ranged)
			if err != nil {
				return err
			}
			resp := s.resp
			switch {
			case resp.StatusCode == http.StatusPartialContent:
				start, last, total, ok := contentRange(resp.Header.Get("Content-Range"))
				if !ok || start != 0 || total <= 0 || last < 0 || last >= total {
					s.close()
					ranged = false
					continue
				}
				j.info = infoFrom(resp, j.part.URL)
				j.info.Size, j.info.Ranges = total, true
				s.end = last + 1
			case resp.StatusCode == http.StatusOK:
				j.info = infoFrom(resp, j.part.URL)
				j.info.Size = bodySize(resp)
				s.end = j.info.Size
				if s.end < 0 {
					s.end = unknownEnd
				}
			case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable && ranged:
				_, _, total, ok := contentRange(resp.Header.Get("Content-Range"))
				if !ok || total != 0 {
					s.close()
					ranged = false
					continue
				}
				j.info = infoFrom(resp, j.part.URL)
				j.info.Size = 0
				s.close()
				return nil
			default:
				se := statusError(resp)
				s.close()
				return se
			}
			st = s
			return nil
		}
	})
	if err != nil {
		return nil, err
	}
	j.ranged = j.info.Ranges
	j.validator = ifRange(j.info.ETag, j.info.LastModified)
	j.tab = newTable(j.info.Size, j.minSplit, j.ranged)
	j.tab.raceSecs, j.tab.warmup = j.tn.raceSecs, j.tn.raceWarmup
	if st == nil || j.info.Size == 0 {
		st.close()
		return nil, nil
	}
	return &firstStream{a: j.tab.claim(j.tab.segs[0], 1, st.end), st: st}, nil
}

func (j *job) loadSidecar() *sidecar {
	b, err := os.ReadFile(j.sidecarPath())
	if err != nil {
		return nil
	}
	var sc sidecar
	ok := json.Unmarshal(b, &sc) == nil && partitions(sc.Segments, sc.Size)
	if ok {
		fi, err := os.Stat(j.partPath)
		ok = err == nil && fi.Mode().IsRegular() && fi.Size() == sc.Size
	}
	have := false
	for _, s := range sc.Segments {
		have = have || s.Done > 0
	}
	if !ok || !have {
		_ = os.Remove(j.sidecarPath())
		return nil
	}
	return &sc
}

// resume trusts size, validators and bytes, not the link, which a refresh may have changed.
func (j *job) resume(sc *sidecar) (first *firstStream, err error) {
	defer func() {
		// a server that answers 200 to every range now cannot continue this partial
		if errors.Is(err, errIgnoredRange) {
			err = errChanged
		}
	}()
	j.info = &Info{URL: j.part.URL, Size: sc.Size, Ranges: true, ETag: sc.ETag, LastModified: sc.LastModified}
	j.ranged = true
	j.validator = ifRange(sc.ETag, sc.LastModified)
	j.tab = tableFrom(sc.Segments, sc.Size, j.minSplit)
	j.tab.raceSecs, j.tab.warmup = j.tn.raceSecs, j.tn.raceWarmup
	pending := j.tab.firstPending()
	if j.validator == "" || pending == nil {
		return nil, j.verifyOverlap()
	}
	reqEnd := min(pending.end, alignUp(pending.pos+j.tn.minReq))
	a := j.tab.claim(pending, 1, reqEnd)
	var st *stream
	err = j.retrying(func() error {
		s, err := j.openRange(a.cursor, reqEnd)
		st = s
		return err
	})
	if err != nil {
		j.tab.release(pending, 1)
		return nil, err
	}
	j.learn(st.resp)
	return &firstStream{a: a, st: st}, nil
}

// without validators, the bytes before a frontier are the only proof the file is the same
// ported from prism-download-manager crates/prism-engine/src/http/download.rs (MIT)
func (j *job) verifyOverlap() error {
	s := j.tab.longest()
	if s == nil {
		return errChanged
	}
	n := min(s.pos-s.start, overlapCheck)
	from := s.pos - n
	local := make([]byte, n)
	f, err := os.Open(j.partPath)
	if err != nil {
		return errChanged
	}
	_, err = f.ReadAt(local, from)
	f.Close()
	if err != nil {
		return errChanged
	}
	remote := make([]byte, n)
	return j.retrying(func() error {
		st, err := j.openRange(from, s.pos)
		if err != nil {
			return err
		}
		defer st.close()
		if _, err := io.ReadFull(st.resp.Body, remote); err != nil {
			return io.ErrUnexpectedEOF
		}
		j.learn(st.resp)
		if !bytes.Equal(remote, local) {
			return errChanged
		}
		return nil
	})
}

func (j *job) learn(resp *http.Response) {
	got := infoFrom(resp, j.part.URL)
	j.info.URL, j.info.Name, j.info.Mime = got.URL, got.Name, got.Mime
	if j.info.ETag == "" {
		j.info.ETag = got.ETag
	}
	if j.info.LastModified == "" {
		j.info.LastModified = got.LastModified
	}
}

// the sidecar may only claim bytes already synced to disk
// ported from prism-download-manager crates/prism-engine/src/segmented/writer.rs (MIT)
func (j *job) save() {
	if !j.ranged || j.info.Size <= 0 || j.writeFailed.Load() || j.file == nil {
		return
	}
	j.wmu.Lock()
	spans := j.tab.spans()
	j.wmu.Unlock()
	if datasync(j.file) != nil {
		return
	}
	b, err := json.Marshal(sidecar{URL: j.part.URL, ETag: j.info.ETag, LastModified: j.info.LastModified, Size: j.info.Size, Segments: spans})
	if err != nil {
		return
	}
	tmp := j.sidecarPath() + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return
	}
	_, werr := f.Write(b)
	serr := f.Sync()
	if cerr := f.Close(); werr == nil && serr == nil && cerr == nil {
		_ = os.Rename(tmp, j.sidecarPath())
	}
}

// one connection giving up does not fail the download, only a replacement round without progress does
// ported from prism-download-manager crates/prism-engine/src/segmented/supervisor.rs (MIT)
func (j *job) supervise(first *firstStream) error {
	requested := j.conns
	if !j.tab.splittable {
		requested = 1
	}
	hosts := j.d.hostsFile()
	learned := 0
	if j.tab.splittable && j.host != "" {
		learned = learnedLimit(hosts, j.host)
	}
	ramp := min(max(1, j.tn.initial), requested)
	probeBackoff := j.tn.probeFirst
	var probeUpAt, holdUntil time.Time
	if learned > 0 {
		probeUpAt = time.Now().Add(probeBackoff)
	}
	// bytes in when the last connection gave up, -1 before that
	heldAt := int64(-1)
	var lastErr, fatal error
	done := false
	active, nextID := 0, 1
	spawn := func(f *firstStream) {
		w := &worker{j: j, id: nextID, buf: make([]byte, chunkSize), reqSize: j.tn.minReq}
		nextID++
		active++
		go func() { j.exits <- w.run(f) }()
	}
	if first != nil {
		spawn(first)
	}
	tick := time.NewTicker(j.tn.tick)
	defer tick.Stop()
	lastSave := time.Now()
	lastConns := -1
	cr, _ := j.p.(ConnReporter)
	defer func() {
		if cr != nil {
			cr.Conns(0)
		}
	}()
	for {
		now := time.Now()
		limit := requested
		if learned > 0 {
			limit = min(limit, learned)
		}
		limit = max(1, limit)
		if !done && j.tab.allDone() {
			done = true
			j.cancel()
		}
		stopping := j.ctx.Err() != nil
		if !stopping && !now.Before(holdUntil) {
			holdUntil = time.Time{}
			for {
				for active < min(ramp, limit) && j.tab.hasWork(true, now) {
					spawn(nil)
				}
				// the rest open once every connection already started delivers bytes
				if ramp >= limit || int(j.connected.Load()) < max(1, min(ramp, active)) {
					break
				}
				ramp = limit
			}
		}
		if active == 0 {
			switch {
			case fatal != nil:
				return fatal
			case done:
				return nil
			case stopping:
				return j.ctx.Err()
			case holdUntil.IsZero():
				if lastErr == nil {
					lastErr = io.ErrUnexpectedEOF
				}
				return lastErr
			}
		}
		select {
		case ex := <-j.exits:
			active--
			switch ex.kind {
			case exitRefused:
				if working := max(1, int(j.connected.Load())); working < limit {
					learned = working
					recordLimit(hosts, j.host, working)
				}
				// a host that asked to wait (Retry-After) stays down that long
				wait := max(ex.retryAfter, probeBackoff)
				probeUpAt = time.Now().Add(wait)
				probeBackoff = min(2*probeBackoff, j.tn.probeMax)
			case exitGaveUp:
				lastErr = ex.err
				if active == 0 && !j.tab.allDone() {
					got := j.tab.downloaded.Load()
					if heldAt >= 0 && got == heldAt {
						return lastErr
					}
					heldAt = got
				}
				holdUntil = time.Now().Add(j.tn.hold)
			case exitFatal:
				if fatal == nil {
					fatal = ex.err
				}
				j.cancel()
			}
		case <-tick.C:
			now := time.Now()
			j.tab.tick(now)
			j.mark(false)
			if cr != nil {
				if c := int(j.connected.Load()); c != lastConns {
					lastConns = c
					cr.Conns(c)
				}
			}
			if now.Sub(lastSave) >= j.tn.saveEvery {
				lastSave = now
				j.save()
			}
			if learned > 0 && learned < requested && !probeUpAt.IsZero() && !now.Before(probeUpAt) {
				learned++
				recordLimit(hosts, j.host, learned)
				probeUpAt = now.Add(probeBackoff)
			}
		case <-j.connNote:
		}
	}
}

func (j *job) mark(all bool) {
	size := j.info.Size
	if size <= 0 {
		if all {
			for b := range buckets {
				j.p.Mark(b, buckets, 1)
			}
		}
		return
	}
	spans := j.tab.spans()
	for b := range buckets {
		lo := int64(b) * size / buckets
		hi := int64(b+1) * size / buckets
		if hi <= lo {
			j.p.Mark(b, buckets, 1)
			continue
		}
		var cov int64
		for _, sp := range spans {
			a, e := max(sp.Start, lo), min(sp.Start+sp.Done, hi)
			if e > a {
				cov += e - a
			}
		}
		j.p.Mark(b, buckets, float64(cov)/float64(hi-lo))
	}
}
