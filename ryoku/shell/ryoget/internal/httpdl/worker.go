package httpdl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"ryoget/internal/plan"
)

// each bounded request asks for about this long at the connection's speed
const requestSeconds = 2.0

var errIgnoredRange = errors.New("the server ignored the range request")

// a request with no answer at all may be a server turning extra connections away
type connError struct{ err error }

func (e *connError) Error() string { return e.err.Error() }
func (e *connError) Unwrap() error { return e.err }

type writeError struct{ err error }

func (e *writeError) Error() string { return "write: " + e.err.Error() }
func (e *writeError) Unwrap() error { return e.err }

type failKind int

const (
	failTransient failKind = iota
	failFatal
	// the server refused this connection outright
	failRefused
	// a connection that failed before any answer, a refusal when it repeats
	failMaybeRefused
)

func classify(err error) failKind {
	var se *StatusError
	if errors.As(err, &se) {
		switch se.Code {
		case http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusForbidden:
			return failRefused
		}
		if transientStatus(se.Code) {
			return failTransient
		}
		return failFatal
	}
	var we *writeError
	if errors.As(err, &we) || errors.Is(err, errChanged) {
		return failFatal
	}
	var ce *connError
	if errors.As(err, &ce) {
		return failMaybeRefused
	}
	if errors.Is(err, errIgnoredRange) || retryable(err) {
		return failTransient
	}
	return failFatal
}

func retryAfterOf(err error) time.Duration {
	var se *StatusError
	if errors.As(err, &se) {
		return se.RetryAfter
	}
	return 0
}

type stream struct {
	resp    *http.Response
	cancel  context.CancelFunc
	idle    *time.Timer
	stalled atomic.Bool
	// exclusive end of the bytes the body carries, unknownEnd when unknown
	end int64
}

func (s *stream) close() {
	if s == nil {
		return
	}
	s.idle.Stop()
	if s.resp != nil {
		s.resp.Body.Close()
	}
	s.cancel()
}

func (s *stream) read(buf []byte) (int, error) {
	n, err := s.resp.Body.Read(buf)
	if n > 0 {
		s.idle.Reset(idleTimeout)
	}
	if err != nil && err != io.EOF && s.stalled.Load() {
		err = errIdle
	}
	return n, err
}

type exitKind int

const (
	exitIdle exitKind = iota
	exitStopped
	exitRefused
	exitGaveUp
	exitFatal
)

type exit struct {
	kind       exitKind
	err        error
	retryAfter time.Duration
}

type pumped int

const (
	pumpSegDone pumped = iota
	pumpRangeDone
	pumpStopped
)

// ported from prism-download-manager crates/prism-engine/src/segmented/worker.rs (MIT)
type worker struct {
	j         *job
	id        int
	buf       []byte
	reqSize   int64
	connected bool
	// bytes arrived since the last failure, which resets the retry budget
	progressed bool
	streamed   bool
	early      int
	failures   int
}

func (w *worker) setConnected(on bool) {
	if w.connected == on {
		return
	}
	w.connected = on
	if on {
		w.j.connected.Add(1)
		select {
		case w.j.connNote <- struct{}{}:
		default:
		}
	} else {
		w.j.connected.Add(-1)
	}
}

func (w *worker) release(a *assignment) {
	if a != nil {
		w.j.tab.release(a.seg, w.id)
	}
}

func (w *worker) run(first *firstStream) exit {
	defer w.setConnected(false)
	j := w.j
	var cur *assignment
	var cursor int64
	var st *stream
	if first != nil {
		a := first.a
		cur, cursor, st = &a, a.cursor, first.st
	}
	for {
		if j.ctx.Err() != nil {
			st.close()
			w.release(cur)
			return exit{kind: exitStopped}
		}
		if cur == nil {
			a, ok := j.tab.acquire(w.id, true, time.Now())
			if !ok {
				return exit{kind: exitIdle}
			}
			cur, cursor = &a, a.cursor
		}
		if st == nil {
			end := j.tab.endOf(cur.seg)
			if cursor >= end {
				cur = nil
				continue
			}
			reqEnd := end
			if j.ranged && !cur.race && !j.fewRequests.Load() {
				reqEnd = min(end, alignUp(cursor+w.reqSize))
			}
			if !cur.race {
				j.tab.setReqEnd(cur.seg, w.id, reqEnd)
			}
			var err error
			if st, err = j.openRange(cursor, reqEnd); err != nil {
				if ex, done := w.failed(err, cur, &cursor); done {
					return ex
				}
				continue
			}
		}
		res, err := w.pump(st, cur, &cursor)
		st.close()
		st = nil
		switch {
		case err != nil:
			if ex, done := w.failed(err, cur, &cursor); done {
				return ex
			}
		case res == pumpSegDone:
			cur = nil
		case res == pumpStopped:
			w.release(cur)
			return exit{kind: exitStopped}
		}
	}
}

func (w *worker) pump(st *stream, a *assignment, cursor *int64) (pumped, error) {
	j := w.j
	started, from := time.Now(), *cursor
	for {
		n, rerr := st.read(w.buf)
		if n > 0 {
			w.setConnected(true)
			if err := plan.Wait(j.ctx, j.d.Limit, n); err != nil {
				if j.ctx.Err() != nil {
					return pumpStopped, nil
				}
				return 0, err
			}
			j.wmu.RLock()
			accept, progressed, stop := j.tab.advance(a.seg, w.id, *cursor, int64(n))
			var werr error
			if accept > 0 {
				_, werr = j.file.WriteAt(w.buf[:accept], *cursor)
			}
			j.wmu.RUnlock()
			if werr != nil {
				j.writeFailed.Store(true)
				if errors.Is(werr, syscall.ENOSPC) {
					return 0, &writeError{noSpace(j.tab.remaining(), 0)}
				}
				return 0, &writeError{werr}
			}
			*cursor += accept
			if progressed > 0 {
				j.p.Add(progressed)
				w.streamed, w.early = true, 0
				// a stream without ranges restarts from zero, only bytes past its best run are progress
				if j.ranged || *cursor > j.high.Load() {
					w.progressed = true
					j.high.Store(max(j.high.Load(), *cursor))
				}
			}
			if stop {
				if *cursor >= st.end {
					// reading the end of the answer puts the connection back in the pool
					_, _ = st.read(w.buf)
					w.tune(*cursor-from, time.Since(started))
				}
				return pumpSegDone, nil
			}
		}
		switch {
		case rerr == io.EOF:
			if st.end == unknownEnd {
				j.tab.completeUnknown(a.seg)
				return pumpSegDone, nil
			}
			if *cursor >= st.end {
				w.tune(*cursor-from, time.Since(started))
				if *cursor >= j.tab.endOf(a.seg) {
					return pumpSegDone, nil
				}
				return pumpRangeDone, nil
			}
			return 0, io.ErrUnexpectedEOF
		case rerr != nil:
			if j.ctx.Err() != nil {
				return pumpStopped, nil
			}
			if errors.Is(rerr, errIdle) {
				return 0, rerr
			}
			return 0, fmt.Errorf("read: %w", rerr)
		}
	}
}

func (w *worker) tune(bytes int64, took time.Duration) {
	if bytes <= 0 || took <= 0 {
		return
	}
	target := int64(float64(bytes) / took.Seconds() * requestSeconds)
	w.reqSize = min(max(target, w.j.tn.minReq), w.j.tn.maxReq)
}

func (w *worker) failed(err error, a *assignment, cursor *int64) (exit, bool) {
	j := w.j
	if j.ctx.Err() != nil {
		w.release(a)
		return exit{kind: exitStopped}, true
	}
	var se *StatusError
	if errors.As(err, &se) && (se.Code == http.StatusTooManyRequests || se.Code == http.StatusServiceUnavailable) {
		j.fewRequests.Store(true)
	}
	switch kind := classify(err); kind {
	case failFatal:
		w.release(a)
		return exit{kind: exitFatal, err: err}, true
	case failRefused, failMaybeRefused:
		if a.race {
			// a refused racer lowers the limit like any refused connection, or it would be respawned at once
			w.release(a)
			return exit{kind: exitRefused, err: err, retryAfter: retryAfterOf(err)}, true
		}
		others := int(j.connected.Load())
		if w.connected {
			others--
		}
		signal := kind == failRefused
		if !signal {
			// a dropped connection is ordinary, a fresh one failing twice is a refusal
			w.early++
			signal = !w.streamed && w.early >= 2
		}
		if signal && others > 0 {
			w.setConnected(false)
			w.release(a)
			return exit{kind: exitRefused, err: err, retryAfter: retryAfterOf(err)}, true
		}
		if !retryable(err) {
			w.release(a)
			return exit{kind: exitFatal, err: err}, true
		}
	}
	w.setConnected(false)
	immediate := w.progressed
	if w.progressed {
		w.failures, w.progressed = 0, false
	}
	w.failures++
	if w.failures > j.tn.maxRetries {
		w.release(a)
		return exit{kind: exitGaveUp, err: err}, true
	}
	if !j.ranged && *cursor > 0 {
		j.p.Add(-j.tab.resetSingle())
		*cursor = 0
	}
	if !j.sleep(j.retryDelay(err, w.failures, immediate)) {
		w.release(a)
		return exit{kind: exitStopped}, true
	}
	return exit{}, false
}

// a dropped connection that was making progress reopens at once
func (j *job) retryDelay(err error, failures int, immediate bool) time.Duration {
	if ra := retryAfterOf(err); ra > 0 {
		return ra
	}
	if immediate {
		return 0
	}
	if j.d.Backoff != nil {
		return j.d.Backoff(failures)
	}
	return backoffDelay(failures-1, jitter())
}

func (j *job) sleep(d time.Duration) bool {
	if d <= 0 {
		return j.ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-j.ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func (j *job) source() (string, bool) {
	j.srcMu.Lock()
	defer j.srcMu.Unlock()
	if j.final != "" {
		return j.final, true
	}
	return j.part.URL, false
}

// a signed redirect target that expired (403/404/410) is dropped once for the original link
// ported from prism-download-manager crates/prism-engine/src/http/fetcher.rs (MIT)
func (j *job) send(from, to int64, ifRange string, ranged bool) (*stream, error) {
	for tries := 0; ; tries++ {
		raw, viaFinal := j.source()
		ctx, cancel := context.WithCancel(j.ctx)
		st := &stream{cancel: cancel}
		st.idle = time.AfterFunc(idleTimeout, func() { st.stalled.Store(true); cancel() })
		req, err := newRequest(ctx, http.MethodGet, raw, j.part.Headers)
		if err != nil {
			st.close()
			return nil, err
		}
		if viaFinal && !sameSite(j.part.URL, raw) {
			// net/http drops these on a redirect to another host, a direct request must too
			for _, k := range []string{"Authorization", "Www-Authenticate", "Cookie", "Cookie2"} {
				req.Header.Del(k)
			}
		}
		req.Header.Set("Accept-Encoding", "identity")
		if ranged {
			last := ""
			if to != unknownEnd {
				last = strconv.FormatInt(to-1, 10)
			}
			req.Header.Set("Range", "bytes="+strconv.FormatInt(from, 10)+"-"+last)
			if ifRange != "" {
				req.Header.Set("If-Range", ifRange)
			}
		}
		resp, err := j.client.Do(req)
		if err != nil {
			stalled := st.stalled.Load()
			st.close()
			if stalled {
				return nil, errIdle
			}
			if j.ctx.Err() != nil {
				return nil, j.ctx.Err()
			}
			return nil, &connError{err}
		}
		st.resp = resp
		switch resp.StatusCode {
		case http.StatusForbidden, http.StatusNotFound, http.StatusGone:
			if viaFinal && tries == 0 {
				st.close()
				j.srcMu.Lock()
				if j.final == raw {
					j.final = ""
				}
				j.srcMu.Unlock()
				continue
			}
		}
		if got := resp.Request.URL.String(); !viaFinal && got != j.part.URL && resp.StatusCode < 300 {
			j.srcMu.Lock()
			j.final = got
			j.srcMu.Unlock()
		}
		return st, nil
	}
}

func (j *job) openRange(from, to int64) (*stream, error) {
	st, err := j.send(from, to, j.validator, j.ranged)
	if err != nil {
		return nil, err
	}
	resp := st.resp
	switch {
	case j.ranged && resp.StatusCode == http.StatusPartialContent:
		start, last, total, ok := contentRange(resp.Header.Get("Content-Range"))
		if !ok || start != from || last < start || (total >= 0 && total != j.info.Size) {
			st.close()
			return nil, errChanged
		}
		// a server that ignores If-Range still shows a changed file in its ETag
		if had, got := j.info.ETag, resp.Header.Get("ETag"); strongETag(had) && strongETag(got) && had != got {
			st.close()
			return nil, errChanged
		}
		st.end = last + 1
	case j.ranged && resp.StatusCode == http.StatusOK:
		st.close()
		if j.validator != "" {
			return nil, errChanged
		}
		return nil, errIgnoredRange
	case !j.ranged && resp.StatusCode == http.StatusOK:
		if n := bodySize(resp); j.info.Size >= 0 && n >= 0 && n != j.info.Size {
			st.close()
			return nil, errChanged
		}
		st.end = j.info.Size
		if st.end < 0 {
			st.end = unknownEnd
		}
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable:
		st.close()
		return nil, errChanged
	default:
		se := statusError(resp)
		st.close()
		return nil, se
	}
	return st, nil
}

func strongETag(e string) bool { return e != "" && !strings.HasPrefix(e, "W/") }

// sameSite follows net/http's redirect rule: credentials go to the same host or a subdomain of it.
func sameSite(orig, dest string) bool {
	o, err1 := url.Parse(orig)
	d, err2 := url.Parse(dest)
	if err1 != nil || err2 != nil {
		return false
	}
	oh, dh := strings.ToLower(o.Hostname()), strings.ToLower(d.Hostname())
	return dh == oh || strings.HasSuffix(dh, "."+oh)
}
