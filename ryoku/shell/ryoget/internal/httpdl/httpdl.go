package httpdl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ryoget/internal/plan"
)

const (
	maxTries  = 5
	chunkSize = 32 << 10
	buckets   = 64
	// how often the sidecar is rewritten while bytes land
	saveEvery = time.Second
	markEvery = 200 * time.Millisecond
	// SidecarExt follows the .part file it describes
	SidecarExt = ".ryoget"
)

var errChanged = errors.New("the file changed on the server")

// Resumer is implemented by a Progress that wants bytes already on disk
// reported apart from bytes that just arrived, so they do not count as speed.
type Resumer interface{ Resumed(bytes int64) }

// ConnReporter is implemented by a Progress that shows live connections.
type ConnReporter interface{ Conns(n int) }

type Downloader struct {
	Client *http.Client
	Limit  plan.Limiter
	// read when a download starts; nil means 8 connections, 1 MiB segments
	Conns    func() int
	MinSplit func() int64
	// test hook: overrides the retry back-off
	Backoff func(try int) time.Duration
}

type segment struct {
	Start int64 `json:"start"`
	// exclusive; -1 while the size is unknown
	End  int64 `json:"end"`
	Done int64 `json:"done"`

	// bytes counted in Done whose WriteAt has not returned yet
	flight int64
	owned  bool
}

func (s *segment) remaining() int64 {
	if s.End < 0 {
		return 1 << 62
	}
	return s.End - s.Start - s.Done
}

type sidecar struct {
	URL          string    `json:"url"`
	ETag         string    `json:"etag,omitempty"`
	LastModified string    `json:"lastModified,omitempty"`
	Size         int64     `json:"size"`
	Segments     []segment `json:"segments"`
}

type job struct {
	d        *Downloader
	part     plan.Part
	info     *Info
	file     *os.File
	partPath string
	minSplit int64
	// resume from a sidecar written for another URL (size and validators still checked)
	anyURL bool
	p      plan.Progress
	conns  atomic.Int32
	// workers running, and how many the server has shown it tolerates
	alive atomic.Int32
	limit atomic.Int32

	mu   sync.Mutex
	segs []*segment
}

// Fetch implements the engine's PartHTTP fetcher: dst is written exactly. Its
// dst is private to one plan, so a resume accepts a new URL for the same bytes:
// media links are re-resolved on every run and their signed URLs change.
func (d *Downloader) Fetch(ctx context.Context, part plan.Part, dst string, p plan.Progress) error {
	_ = os.Remove(dst)
	_, err := d.fetch(ctx, part, dst, p, true)
	return err
}

// FetchFile downloads into dst+".part" and publishes it under dst, or under
// "dst (n)" when that name is taken by then. It returns the final path.
func (d *Downloader) FetchFile(ctx context.Context, part plan.Part, dst string, p plan.Progress) (string, error) {
	return d.fetch(ctx, part, dst, p, false)
}

func (d *Downloader) fetch(ctx context.Context, part plan.Part, dst string, p plan.Progress, anyURL bool) (string, error) {
	client := d.Client
	if client == nil {
		client = NewClient()
	}
	info, err := Probe(ctx, client, part.URL, part.Headers)
	if err != nil {
		return "", err
	}
	j := &job{d: d, part: part, info: info, partPath: dst + ".part", p: p, minSplit: 1 << 20, anyURL: anyURL}
	if d.MinSplit != nil {
		if v := d.MinSplit(); v > 0 {
			j.minSplit = v
		}
	}
	for restarts := 0; ; restarts++ {
		err = j.run(ctx, client, restarts > 0)
		if !errors.Is(err, errChanged) || restarts >= 2 {
			break
		}
		if info, err = Probe(ctx, client, part.URL, part.Headers); err != nil {
			return "", err
		}
		j.info = info
	}
	if err != nil {
		return "", err
	}
	return Publish(j.partPath, dst)
}

func (j *job) sidecarPath() string { return j.partPath + SidecarExt }

func (j *job) resumable() bool { return j.info.Ranges && j.info.Size > 0 }

func (j *job) load() bool {
	if !j.resumable() {
		return false
	}
	b, err := os.ReadFile(j.sidecarPath())
	if err != nil {
		return false
	}
	var sc sidecar
	if json.Unmarshal(b, &sc) != nil || (sc.URL != j.part.URL && !j.anyURL) || sc.Size != j.info.Size || len(sc.Segments) == 0 {
		return false
	}
	switch {
	case sc.ETag != "" && j.info.ETag != "":
		if sc.ETag != j.info.ETag {
			return false
		}
	case sc.LastModified != "" && j.info.LastModified != "":
		if sc.LastModified != j.info.LastModified {
			return false
		}
	case sc.ETag != "" || sc.LastModified != "" || j.info.ETag != "" || j.info.LastModified != "":
		return false
	}
	if fi, err := os.Stat(j.partPath); err != nil || !fi.Mode().IsRegular() {
		return false
	}
	j.segs = j.segs[:0]
	for i := range sc.Segments {
		s := sc.Segments[i]
		if s.Start < 0 || s.End > sc.Size || s.Done < 0 || s.Start+s.Done > s.End {
			return false
		}
		j.segs = append(j.segs, &s)
	}
	return true
}

func (j *job) save() {
	if !j.resumable() {
		return
	}
	sc := sidecar{URL: j.part.URL, ETag: j.info.ETag, LastModified: j.info.LastModified, Size: j.info.Size}
	j.mu.Lock()
	for _, s := range j.segs {
		sc.Segments = append(sc.Segments, segment{Start: s.Start, End: s.End, Done: s.Done - s.flight})
	}
	j.mu.Unlock()
	b, _ := json.Marshal(sc)
	tmp := j.sidecarPath() + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		_ = os.Rename(tmp, j.sidecarPath())
	}
}

func (j *job) fresh() {
	size := j.info.Size
	n := int64(1)
	if j.resumable() {
		n = j.workers(size)
	}
	j.segs = j.segs[:0]
	if size < 0 {
		j.segs = append(j.segs, &segment{End: -1})
		return
	}
	step := size / n
	for i := range n {
		s := &segment{Start: i * step, End: (i + 1) * step}
		if i == n-1 {
			s.End = size
		}
		j.segs = append(j.segs, s)
	}
}

func (j *job) workers(remaining int64) int64 {
	conns := int64(8)
	if j.d.Conns != nil {
		if v := j.d.Conns(); v > 0 {
			conns = int64(v)
		}
	}
	if !j.resumable() {
		return 1
	}
	n := (remaining + j.minSplit - 1) / j.minSplit
	return max(1, min(conns, n))
}

func (j *job) run(ctx context.Context, client *http.Client, restart bool) error {
	resumed := !restart && j.load()
	flags := os.O_RDWR | os.O_CREATE
	if !resumed {
		flags |= os.O_TRUNC
		_ = os.Remove(j.sidecarPath())
		j.fresh()
	}
	f, err := os.OpenFile(j.partPath, flags, 0o644)
	if err != nil {
		return err
	}
	j.file = f
	if j.info.Size > 0 {
		if err := f.Truncate(j.info.Size); err != nil {
			f.Close()
			return err
		}
	}
	j.p.SetTotal(j.info.Size)
	var have, left int64
	for _, s := range j.segs {
		have += s.Done
		if s.End >= 0 {
			left += s.remaining()
		}
	}
	if have > 0 {
		if r, ok := j.p.(Resumer); ok {
			r.Resumed(have)
		} else {
			j.p.Add(have)
		}
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stopTick := make(chan struct{})
	tickDone := make(chan struct{})
	go j.tick(stopTick, tickDone)

	j.limit.Store(int32(j.workers(left)))
	var first error
	// a round ends early only when a server turned connections away and the
	// workers that backed off left segments behind for a smaller round
	for round := 0; round < maxTries; round++ {
		n := int64(j.limit.Load())
		if j.info.Size == 0 {
			n = 0
		}
		var wg sync.WaitGroup
		var once sync.Once
		for range n {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := j.worker(runCtx, client); err != nil {
					once.Do(func() { first = err; cancel() })
				}
			}()
		}
		wg.Wait()
		if first != nil || ctx.Err() != nil || j.complete() {
			break
		}
	}
	close(stopTick)
	<-tickDone
	if first == nil && ctx.Err() != nil {
		first = ctx.Err()
	}
	if first == nil && !j.complete() {
		first = io.ErrUnexpectedEOF
	}
	if first != nil {
		if !errors.Is(first, errChanged) {
			j.save()
		}
		f.Close()
		if errors.Is(first, errChanged) {
			j.p.Add(-j.doneBytes())
		}
		return first
	}
	if j.info.Size < 0 {
		got := j.doneBytes()
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

func (j *job) doneBytes() int64 {
	j.mu.Lock()
	defer j.mu.Unlock()
	var n int64
	for _, s := range j.segs {
		n += s.Done
	}
	return n
}

func (j *job) complete() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, s := range j.segs {
		if s.End < 0 || s.remaining() > 0 {
			return false
		}
	}
	return true
}

// next hands a worker an unowned segment, or splits the largest one still in
// flight so a connection that finished early keeps pulling its weight.
func (j *job) next() *segment {
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, s := range j.segs {
		if !s.owned && s.remaining() > 0 {
			s.owned = true
			return s
		}
	}
	if !j.resumable() {
		return nil
	}
	var best *segment
	for _, s := range j.segs {
		if s.owned && s.End >= 0 && (best == nil || s.remaining() > best.remaining()) {
			best = s
		}
	}
	if best == nil || best.remaining() < 2*j.minSplit {
		return nil
	}
	mid := best.Start + best.Done + best.remaining()/2
	ns := &segment{Start: mid, End: best.End, owned: true}
	best.End = mid
	j.segs = append(j.segs, ns)
	return ns
}

func (j *job) worker(ctx context.Context, client *http.Client) error {
	j.alive.Add(1)
	left := false
	defer func() {
		if !left {
			j.alive.Add(-1)
		}
	}()
	for {
		s := j.next()
		if s == nil {
			return nil
		}
		err := j.segment(ctx, client, s, &left)
		j.mu.Lock()
		s.owned = false
		j.mu.Unlock()
		if left {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func throttled(err error) bool {
	var se *StatusError
	return errors.As(err, &se) && (se.Code == http.StatusTooManyRequests || se.Code == http.StatusServiceUnavailable)
}

// segment fetches one segment with retries. A connection the server turns
// away while others still flow leaves (left = true): the download carries on
// with fewer connections instead of failing.
func (j *job) segment(ctx context.Context, client *http.Client, s *segment, left *bool) error {
	tries := 0
	for {
		got, err := j.fetch(ctx, client, s)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !retryable(err) {
			return err
		}
		if throttled(err) {
			if rest := j.alive.Add(-1); rest >= 1 {
				*left = true
				j.limit.Store(rest)
				return nil
			}
			j.alive.Add(1)
		}
		if got > 0 {
			tries = 0
		}
		tries++
		if tries >= maxTries {
			return err
		}
		wait := min(time.Duration(1<<tries)*250*time.Millisecond, 8*time.Second)
		var se *StatusError
		if errors.As(err, &se) && se.RetryAfter > 0 {
			wait = min(se.RetryAfter, 30*time.Second)
		}
		if j.d.Backoff != nil {
			wait = j.d.Backoff(tries)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
}

// validator is what If-Range compares; a weak ETag may not be used there.
func (j *job) validator() string {
	if e := j.info.ETag; e != "" && !strings.HasPrefix(e, "W/") {
		return e
	}
	return j.info.LastModified
}

func (j *job) fetch(ctx context.Context, client *http.Client, s *segment) (int64, error) {
	j.mu.Lock()
	if !j.resumable() && s.Done > 0 {
		// a stream without ranges can only start over
		j.p.Add(-s.Done)
		s.Done = 0
	}
	pos, end := s.Start+s.Done, s.End
	j.mu.Unlock()
	if end >= 0 && pos >= end {
		return 0, nil
	}
	reqCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stalled := atomic.Bool{}
	idle := time.AfterFunc(idleTimeout, func() { stalled.Store(true); cancel() })
	defer idle.Stop()

	req, err := newRequest(reqCtx, http.MethodGet, j.part.URL, j.part.Headers)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept-Encoding", "identity")
	ranged := j.resumable()
	if ranged {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(pos, 10)+"-"+strconv.FormatInt(end-1, 10))
		if v := j.validator(); v != "" {
			req.Header.Set("If-Range", v)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		if stalled.Load() {
			return 0, errIdle
		}
		return 0, err
	}
	defer resp.Body.Close()
	switch {
	case ranged && resp.StatusCode == http.StatusPartialContent:
		start, _, total, ok := contentRange(resp.Header.Get("Content-Range"))
		if !ok || start != pos || (total >= 0 && total != j.info.Size) {
			return 0, errChanged
		}
	case ranged && resp.StatusCode == http.StatusOK:
		return 0, errChanged
	case !ranged && resp.StatusCode == http.StatusOK:
	default:
		return 0, statusError(resp)
	}
	j.conns.Add(1)
	defer j.conns.Add(-1)

	buf := make([]byte, chunkSize)
	var got int64
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			idle.Reset(idleTimeout)
			if err := plan.Wait(ctx, j.d.Limit, n); err != nil {
				return got, err
			}
			j.mu.Lock()
			at := s.Start + s.Done
			allow := int64(n)
			if s.End >= 0 {
				allow = min(allow, s.End-at)
			}
			s.Done += allow
			s.flight += allow
			j.mu.Unlock()
			if allow > 0 {
				_, werr := j.file.WriteAt(buf[:allow], at)
				j.mu.Lock()
				s.flight -= allow
				if werr != nil {
					s.Done -= allow
				}
				j.mu.Unlock()
				if werr != nil {
					return got, werr
				}
				got += allow
				j.p.Add(allow)
			}
			j.mu.Lock()
			finished := s.End >= 0 && s.Start+s.Done >= s.End
			j.mu.Unlock()
			if finished {
				return got, nil
			}
		}
		if rerr == io.EOF {
			j.mu.Lock()
			defer j.mu.Unlock()
			if s.End < 0 {
				s.End = s.Start + s.Done
				return got, nil
			}
			if s.Start+s.Done >= s.End {
				return got, nil
			}
			return got, io.ErrUnexpectedEOF
		}
		if rerr != nil {
			if stalled.Load() {
				return got, errIdle
			}
			if ctx.Err() != nil {
				return got, ctx.Err()
			}
			return got, fmt.Errorf("read: %w", rerr)
		}
	}
}

func (j *job) tick(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	mark := time.NewTicker(markEvery)
	defer mark.Stop()
	lastSave := time.Now()
	lastConns := -1
	for {
		select {
		case <-stop:
			if cr, ok := j.p.(ConnReporter); ok {
				cr.Conns(0)
			}
			return
		case <-mark.C:
		}
		j.mark(false)
		if cr, ok := j.p.(ConnReporter); ok {
			if c := int(j.conns.Load()); c != lastConns {
				lastConns = c
				cr.Conns(c)
			}
		}
		if time.Since(lastSave) >= saveEvery {
			lastSave = time.Now()
			j.save()
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
	j.mu.Lock()
	type span struct{ a, b int64 }
	spans := make([]span, 0, len(j.segs))
	for _, s := range j.segs {
		spans = append(spans, span{s.Start, s.Start + s.Done - s.flight})
	}
	j.mu.Unlock()
	for b := range buckets {
		lo := int64(b) * size / buckets
		hi := int64(b+1) * size / buckets
		if hi <= lo {
			j.p.Mark(b, buckets, 1)
			continue
		}
		var cov int64
		for _, sp := range spans {
			a, e := max(sp.a, lo), min(sp.b, hi)
			if e > a {
				cov += e - a
			}
		}
		j.p.Mark(b, buckets, float64(cov)/float64(hi-lo))
	}
}
